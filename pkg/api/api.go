// Package api serves a wits repository over HTTP as JSON, for clients that are
// not a terminal — the Angular interface in wits-ui first.
//
// It is another caller of pkg/workspace, nothing more. Every request opens the
// repository afresh, so what it answers is what wits status would say at that
// moment; every write goes through the recorder, under the journal's lock, with
// the same checks the commands and the terminal interface make. Figures keep
// the scope they have everywhere else — the fill, the shelf, the jar — and the
// field names say which.
//
// It is health data, so the server answers this machine only: requests must
// name a loopback host, which keeps a web page elsewhere from reaching it
// through DNS rebinding, and writes must be JSON, which a page on another
// origin cannot send without a preflight this server never grants.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Server answers for one repository.
type Server struct {
	repo *repo.Repo
	ui   string
}

// New returns a server for the repository, serving the built interface from
// ui when it is not empty.
func New(r *repo.Repo, ui string) *Server {
	// The journal is created on first use; creating it here, before any
	// request, keeps two first requests from racing to create it.
	r.Journal()
	return &Server{repo: r, ui: ui}
}

// Handler routes the API, and the interface when there is one.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/overview", s.read(overviewOf))
	mux.HandleFunc("GET /api/journal", s.journal)
	mux.HandleFunc("GET /api/jars", s.read(jarsOf))
	mux.HandleFunc("GET /api/cycles", s.read(cyclesOf))
	mux.HandleFunc("GET /api/devices", s.read(devicesOf))
	mux.HandleFunc("GET /api/days", s.read(daysOf))
	mux.HandleFunc("GET /api/effects", s.read(effectsOf))
	mux.HandleFunc("POST /api/rate", s.rate)
	mux.HandleFunc("POST /api/buy", s.write(buy))
	mux.HandleFunc("POST /api/grind", s.write(grind))
	mux.HandleFunc("POST /api/sesh", s.write(sesh))
	mux.HandleFunc("POST /api/weigh", s.write(weigh))
	mux.HandleFunc("POST /api/undo", s.write(undo))
	mux.HandleFunc("POST /api/flush", s.flush)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		fail(w, http.StatusNotFound, errors.New("no such endpoint"))
	})
	if s.ui != "" {
		mux.Handle("/", spa(s.ui))
	}
	return local(mux)
}

// local refuses any request that does not name this machine as its host.
func local(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		if !loopback(host) {
			fail(w, http.StatusForbidden, errors.New("wits-server answers this machine only"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loopback reports whether a host name or address is this machine.
func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// spa serves the built interface, and its index for any path that is not a
// file, so a reload on a client-side route still finds the app.
func spa(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}

// read answers a GET from a fresh snapshot of the repository.
func (s *Server) read(view func(*workspace.Workspace) any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ws, err := workspace.Read(s.repo)
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		reply(w, http.StatusOK, view(ws))
	}
}

// writable opens a fresh workspace for a write, refusing anything but a
// JSON body of a sensible size; it answers the refusal itself.
func (s *Server) writable(w http.ResponseWriter, r *http.Request) (*workspace.Workspace, bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, http.StatusUnsupportedMediaType, errors.New("send JSON"))
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	ws, err := workspace.Read(s.repo)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return nil, false
	}
	return ws, true
}

// write decodes a JSON body and records it through a fresh workspace's
// recorder, answering with the entry it wrote.
func (s *Server) write(do func(*workspace.Workspace, *http.Request) (journal.Event, string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, ok := s.writable(w, r)
		if !ok {
			return
		}
		e, message, err := do(ws, r)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		reply(w, http.StatusCreated, written{Entry: entryOf(e, ws, nil), Message: message})
	}
}

// flush records stashes as consumed — the named ones, or every one holding
// something when none is named — as one batch.
func (s *Server) flush(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.writable(w, r)
	if !ok {
		return
	}
	var req struct {
		Products []string `json:"products"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	events, err := ws.Recorder.Flush(req.Products)
	if errors.Is(err, record.ErrNothingToReconcile) {
		fail(w, http.StatusBadRequest, errors.New("there is nothing in those stashes to flush"))
		return
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	out := flushed{Entries: make([]entry, 0, len(events))}
	var grams journal.Centigrams
	for _, e := range events {
		out.Entries = append(out.Entries, entryOf(e, ws, nil))
		grams += journal.CG(e.Grams)
	}
	out.Message = fmt.Sprintf("flushed %d %s: %.2f g recorded as consumed", len(events),
		map[bool]string{true: "stash", false: "stashes"}[len(events) == 1], grams.Grams())
	reply(w, http.StatusCreated, out)
}

// flushed is the answer to a flush: every adjustment it wrote.
type flushed struct {
	Entries []entry `json:"entries"`
	Message string  `json:"message"`
}

// rated is a session's standing rating, with the session it rates.
type rated struct {
	Hash    string                    `json:"hash"`
	At      time.Time                 `json:"at"`
	Scores  map[effects.Dimension]int `json:"scores"`
	Note    string                    `json:"note,omitempty"`
	Session entry                     `json:"session"`
}

// effectLog is the effect log as a client shows it: what can be rated, what
// has been, what is waiting, and what it adds up to.
type effectLog struct {
	Dimensions []effects.Dimension `json:"dimensions"`
	Rated      []rated             `json:"rated"`
	Unrated    []entry             `json:"unrated"`
	Insights   effects.Insights    `json:"insights"`
}

// effectsOf answers the effect log: every session's standing rating, newest
// session first; the latest sessions not yet rated; and the insights.
func effectsOf(ws *workspace.Workspace) any {
	latest := effects.Latest(ws.Ratings)
	undone := record.Reverted(ws.State.Events)
	out := effectLog{Dimensions: effects.Dimensions, Rated: []rated{}, Unrated: []entry{}}
	for i := len(ws.State.Events) - 1; i >= 0; i-- {
		e := ws.State.Events[i]
		if e.Type != journal.Sesh || undone[e.Hash] || e.OccurredAt.After(ws.OpenedAt) {
			continue
		}
		if r, ok := latest[e.Hash]; ok {
			out.Rated = append(out.Rated, rated{Hash: r.Hash, At: r.At, Scores: r.Scores, Note: r.Note,
				Session: entryOf(e, ws, nil)})
		} else if len(out.Unrated) < 12 {
			out.Unrated = append(out.Unrated, entryOf(e, ws, nil))
		}
	}
	out.Insights = effects.Analyse(ws.Ratings, ws.State.Applied(), ws.ProductName)
	return out
}

// rate records a rating of a session.
func (s *Server) rate(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.writable(w, r)
	if !ok {
		return
	}
	var req struct {
		Session string                    `json:"session"`
		Scores  map[effects.Dimension]int `json:"scores"`
		Note    string                    `json:"note"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	rating, err := ws.Recorder.Rate(req.Session, req.Scores, req.Note, time.Now())
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	e, _ := ws.Recorder.Find(rating.Session)
	reply(w, http.StatusCreated, map[string]any{
		"rating":  rated{Hash: rating.Hash, At: rating.At, Scores: rating.Scores, Note: rating.Note, Session: entryOf(e, ws, nil)},
		"message": fmt.Sprintf("rated the session of %.2f g of %s", e.Grams, ws.ProductName(e.Product)),
	})
}

// written is the answer to a write.
type written struct {
	Entry   entry  `json:"entry"`
	Message string `json:"message"`
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, status int, err error) {
	reply(w, status, map[string]string{"error": err.Error()})
}

// overview is the front page: the fill in progress, the shelf around it, and
// what is on its way.
type overview struct {
	Now      time.Time `json:"now"`
	ChainOK  bool      `json:"chain_ok"`
	Fill     *fill     `json:"fill"`
	Shelf    shelf     `json:"shelf"`
	Upcoming []entry   `json:"upcoming"`
}

// fill is the cycle in progress, in the fill's scope: its own grams.
type fill struct {
	Number       int           `json:"number"`
	Start        time.Time     `json:"start"`
	Day          int           `json:"day"`
	Dispensed    float64       `json:"dispensed"`
	Remaining    float64       `json:"remaining"`
	RemainingPct float64       `json:"remaining_pct"`
	PerActiveDay float64       `json:"per_active_day"`
	DaysWorth    float64       `json:"days_worth"`
	Carried      carried       `json:"carried"`
	Products     []fillProduct `json:"products"`
	// ShelfProjection is the shelf's, not the fill's: everything standing
	// when the fill arrived, less every grind since, and on at the rate.
	ShelfProjection projection `json:"shelf_projection"`
}

// projection is the shelf's grams day by day since the fill arrived, and on
// to the day they run out at the observed rate.
type projection struct {
	History   []point    `json:"history"`
	Projected []point    `json:"projected"`
	EmptyOn   *time.Time `json:"empty_on,omitempty"`
}

// point is grams on a day.
type point struct {
	Date  string  `json:"date"`
	Grams float64 `json:"grams"`
}

// carried is what older fills still hold on the shelf: the seam between the
// fill's scope and the shelf's.
type carried struct {
	Grams  float64 `json:"grams"`
	Jars   int     `json:"jars"`
	Cycles int     `json:"cycles"`
}

// fillProduct is one product's row of the fill: sealed, ground into the stash
// and collected as AVB, against what the fill dispensed of it.
type fillProduct struct {
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	Dispensed float64 `json:"dispensed"`
	Sealed    float64 `json:"sealed"`
	Stash     float64 `json:"stash"`
	AVB       float64 `json:"avb"`
}

// shelf is every jar standing today, whichever fill dispensed it.
type shelf struct {
	Storage float64 `json:"storage"`
	Stash   float64 `json:"stash"`
	AVB     float64 `json:"avb"`
	Jars    int     `json:"jars"`
}

func overviewOf(ws *workspace.Workspace) any {
	out := overview{Now: ws.OpenedAt, ChainOK: ws.Chain == nil, Fill: fillOf(ws), Upcoming: []entry{}}
	var storage, stash, avb journal.Centigrams
	for _, b := range ws.State.Balances {
		storage += journal.CG(b.Storage)
		stash += journal.CG(b.Stash)
		avb += journal.CG(b.AVB)
		if b.Storage > 0 || b.Stash > 0 {
			out.Shelf.Jars++
		}
	}
	out.Shelf.Storage, out.Shelf.Stash, out.Shelf.AVB = storage.Grams(), stash.Grams(), avb.Grams()
	undone := record.Reverted(ws.State.Events)
	for _, e := range ws.State.Upcoming() {
		if !undone[e.Hash] {
			out.Upcoming = append(out.Upcoming, entryOf(e, ws, undone))
		}
	}
	return out
}

func fillOf(ws *workspace.Workspace) *fill {
	c := ws.Cycle()
	if c == nil {
		return nil
	}
	stats := ledger.Summarise(c.Events)
	remaining := ws.State.FillOnShelf(c)
	f := &fill{
		Number:       c.Seq + 1,
		Start:        c.Start,
		Day:          int(ws.OpenedAt.Sub(c.Start).Hours()/24) + 1,
		Dispensed:    float64(c.Purchased),
		Remaining:    float64(remaining),
		PerActiveDay: stats.PerActiveDay,
		DaysWorth:    stats.DaysWorth(float64(remaining)),
		Products:     []fillProduct{},
	}
	if c.Purchased > 0 {
		f.RemainingPct = float64(remaining / c.Purchased)
	}
	f.ShelfProjection = projectionOf(c, ws.OpenedAt, stats.PerActiveDay)
	grams, jars, cycles := ws.State.CarriedOnShelf(c)
	f.Carried = carried{Grams: float64(grams), Jars: jars, Cycles: cycles}
	for _, slug := range c.Products {
		stash, avb := ws.State.FillGround(c, slug)
		f.Products = append(f.Products, fillProduct{
			Slug: slug, Name: ws.ProductName(slug),
			Dispensed: float64(c.PurchasedOf(slug)), Sealed: float64(ws.State.ShareOf(c, slug)),
			Stash: float64(stash), AVB: float64(avb),
		})
	}
	return f
}

// projectionOf is the terminal's supply projection, as data: the shelf's grams
// on each day of the fill so far, then on at the rate until they run out —
// ninety days at most.
func projectionOf(c *ledger.Cycle, now time.Time, rate float64) projection {
	perDay := map[string]journal.Centigrams{}
	for _, e := range c.Events {
		if e.Type == journal.Grind {
			perDay[e.OccurredAt.Format(time.DateOnly)] += journal.CG(e.Grams)
		}
	}
	out := projection{History: []point{}, Projected: []point{}}
	held := journal.CG(float64(c.Held()))
	day := c.Start
	for ; !day.After(now); day = day.AddDate(0, 0, 1) {
		held -= perDay[day.Format(time.DateOnly)]
		out.History = append(out.History, point{day.Format(time.DateOnly), max(held.Grams(), 0)})
	}
	if rate <= 0 || held <= 0 {
		return out
	}
	level := held.Grams()
	last := now
	for i := 0; i < 90 && level > 0; i++ {
		level = max(level-rate, 0)
		last = now.AddDate(0, 0, i+1)
		out.Projected = append(out.Projected, point{last.Format(time.DateOnly), ledger.Round(level)})
	}
	out.EmptyOn = &last
	return out
}

// day is what was ground and put through a device on one calendar day.
type day struct {
	Date   string  `json:"date"`
	Ground float64 `json:"ground"`
	Seshed float64 `json:"seshed"`
}

// daysOf totals every day with an entry, oldest first, for the calendars.
func daysOf(ws *workspace.Workspace) any {
	ground, seshed := map[string]journal.Centigrams{}, map[string]journal.Centigrams{}
	seen := map[string]bool{}
	var dates []string
	for _, e := range ws.State.Applied() {
		d := e.OccurredAt.Format(time.DateOnly)
		if !seen[d] {
			seen[d] = true
			dates = append(dates, d)
		}
		switch e.Type {
		case journal.Grind:
			ground[d] += journal.CG(e.Grams)
		case journal.Sesh:
			seshed[d] += journal.CG(e.Grams)
		}
	}
	sort.Strings(dates)
	out := make([]day, 0, len(dates))
	for _, d := range dates {
		out = append(out, day{Date: d, Ground: ground[d].Grams(), Seshed: seshed[d].Grams()})
	}
	return out
}

// entry is one journal entry as a client shows it.
type entry struct {
	Hash        string    `json:"hash"`
	Type        string    `json:"type"`
	OccurredAt  time.Time `json:"occurred_at"`
	RecordedAt  time.Time `json:"recorded_at"`
	Product     string    `json:"product,omitempty"`
	ProductName string    `json:"product_name,omitempty"`
	Grams       float64   `json:"grams"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Device      string    `json:"device,omitempty"`
	Temperature int       `json:"temperature,omitempty"`
	Note        string    `json:"note,omitempty"`
	Reverts     string    `json:"reverts,omitempty"`
	Corrected   bool      `json:"corrected,omitempty"`
	Upcoming    bool      `json:"upcoming,omitempty"`
}

func entryOf(e journal.Event, ws *workspace.Workspace, undone map[string]bool) entry {
	return entry{
		Hash: e.Hash, Type: string(e.Type), OccurredAt: e.OccurredAt, RecordedAt: e.RecordedAt,
		Product: e.Product, ProductName: ws.ProductName(e.Product), Grams: e.Grams,
		From: string(e.From), To: string(e.To), Device: e.Device, Temperature: e.Temperature,
		Note: e.Note, Reverts: e.Reverts, Corrected: undone[e.Hash] && e.Reverts == "",
		Upcoming: e.OccurredAt.After(ws.OpenedAt),
	}
}

// journal answers the entries newest first: limit caps how many, type keeps
// one kind, and corrected pairs are left out unless all is set — the way the
// terminal's journal reads.
func (s *Server) journal(w http.ResponseWriter, r *http.Request) {
	ws, err := workspace.Read(s.repo)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	q := r.URL.Query()
	limit := 200
	if _, err := fmt.Sscan(q.Get("limit"), &limit); q.Get("limit") != "" && (err != nil || limit < 1) {
		fail(w, http.StatusBadRequest, errors.New("limit must be a positive number"))
		return
	}
	undone := record.Reverted(ws.State.Events)
	out := []entry{}
	for i := len(ws.State.Events) - 1; i >= 0 && len(out) < limit; i-- {
		e := ws.State.Events[i]
		if q.Get("type") != "" && string(e.Type) != q.Get("type") {
			continue
		}
		if q.Get("all") == "" && undone[e.Hash] {
			continue
		}
		out = append(out, entryOf(e, ws, undone))
	}
	reply(w, http.StatusOK, map[string]any{"entries": out, "total": len(ws.State.Events)})
}

// jar is one product across its lifetime — the jar's scope.
type jar struct {
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Manufacturer string    `json:"manufacturer,omitempty"`
	Cultivar     string    `json:"cultivar,omitempty"`
	THC          float64   `json:"thc,omitempty"`
	CBD          float64   `json:"cbd,omitempty"`
	Storage      float64   `json:"storage"`
	Stash        float64   `json:"stash"`
	AVB          float64   `json:"avb"`
	Bought       float64   `json:"bought"`
	Ground       float64   `json:"ground"`
	Fills        int       `json:"fills"`
	LastSeen     time.Time `json:"last_seen"`
}

func jarsOf(ws *workspace.Workspace) any {
	bought, ground := map[string]journal.Centigrams{}, map[string]journal.Centigrams{}
	fills, seen := map[string]int{}, map[string]time.Time{}
	for _, e := range ws.State.Applied() {
		switch e.Type {
		case journal.Purchase:
			bought[e.Product] += journal.CG(e.Grams)
			fills[e.Product]++
		case journal.Grind:
			ground[e.Product] += journal.CG(e.Grams)
		}
		if e.OccurredAt.After(seen[e.Product]) {
			seen[e.Product] = e.OccurredAt
		}
	}
	out := []jar{}
	for _, slug := range ws.State.Products() {
		if slug == "" {
			continue
		}
		b := ws.State.Balances[slug]
		j := jar{Slug: slug, Name: ws.ProductName(slug), Storage: b.Storage, Stash: b.Stash, AVB: b.AVB,
			Bought: bought[slug].Grams(), Ground: ground[slug].Grams(), Fills: fills[slug], LastSeen: seen[slug]}
		describe(&j, ws.Products, slug)
		out = append(out, j)
	}
	sort.SliceStable(out, func(i, k int) bool { return out[i].LastSeen.After(out[k].LastSeen) })
	return out
}

// describe adds what the catalog knows about a product.
func describe(j *jar, products *catalog.Catalog, slug string) {
	if p := products.Exact(slug); p != nil {
		j.Manufacturer, j.Cultivar, j.THC, j.CBD = p.Manufacturer, p.Cultivar, p.THC, p.CBD
	}
}

// cycle is one fill in the history.
type cycle struct {
	Number    int       `json:"number"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end,omitzero"`
	Open      bool      `json:"open"`
	Dispensed float64   `json:"dispensed"`
	// GroundDuring is every grind during the fill's tenure, from whichever jar:
	// the shelf's grinding over the period, not the fill's.
	GroundDuring float64 `json:"ground_during"`
	// Remaining is the fill's own grams still sealed.
	Remaining float64  `json:"remaining"`
	Products  []string `json:"products"`
}

func cyclesOf(ws *workspace.Workspace) any {
	out := make([]cycle, 0, len(ws.State.Cycles))
	for i := len(ws.State.Cycles) - 1; i >= 0; i-- {
		c := ws.State.Cycles[i]
		out = append(out, cycle{Number: c.Seq + 1, Start: c.Start, End: c.End, Open: c.Open(),
			Dispensed: float64(c.Purchased), GroundDuring: float64(c.Ground),
			Remaining: float64(ws.State.FillOnShelf(&c)), Products: c.Products})
	}
	return out
}

// device is one vaporizer and what has gone through it.
type device struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind,omitempty"`
	MinTemp     int     `json:"min_temp,omitempty"`
	MaxTemp     int     `json:"max_temp,omitempty"`
	DefaultTemp int     `json:"default_temp,omitempty"`
	Sessions    int     `json:"sessions"`
	Grams       float64 `json:"grams"`
}

func devicesOf(ws *workspace.Workspace) any {
	sessions, grams := map[string]int{}, map[string]journal.Centigrams{}
	for _, e := range ws.State.Applied() {
		if e.Type == journal.Sesh && e.Device != "" {
			sessions[e.Device]++
			grams[e.Device] += journal.CG(e.Grams)
		}
	}
	out := []device{}
	for _, d := range ws.Devices.Devices {
		out = append(out, device{Slug: d.Slug, Name: d.Name, Kind: d.Kind, MinTemp: d.MinTemp,
			MaxTemp: d.MaxTemp, DefaultTemp: d.DefaultTemp, Sessions: sessions[d.Slug], Grams: grams[d.Slug].Grams()})
	}
	return out
}

// decode reads a request body into v, refusing fields it does not know.
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("unreadable request: %w", err)
	}
	return nil
}

// when reads the date an entry happened, as the commands' --date does: empty
// is now; a date, a date and a time, or RFC 3339 otherwise.
func when(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	for _, layout := range []string{time.DateOnly, "2006-01-02 15:04", "2006-01-02T15:04", time.RFC3339} {
		if at, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return at, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q is not a date, expected YYYY-MM-DD", s)
}

type buyRequest struct {
	Name  string  `json:"name"`
	Slug  string  `json:"slug"`
	Grams float64 `json:"grams"`
	Date  string  `json:"date"`
}

func buy(ws *workspace.Workspace, r *http.Request) (journal.Event, string, error) {
	var req buyRequest
	if err := decode(r, &req); err != nil {
		return journal.Event{}, "", err
	}
	at, err := when(req.Date)
	if err != nil {
		return journal.Event{}, "", err
	}
	e, p, added, err := ws.Recorder.Buy(strings.TrimSpace(req.Name), strings.TrimSpace(req.Slug), req.Grams, at)
	if err != nil {
		return e, "", err
	}
	msg := fmt.Sprintf("picked up %.2f g of %s", e.Grams, p.Name)
	if added {
		msg = fmt.Sprintf("new product %s, as %s — picked up %.2f g", p.Name, p.Slug, e.Grams)
	}
	return e, msg, nil
}

type grindRequest struct {
	Product string  `json:"product"`
	Grams   float64 `json:"grams"`
	Date    string  `json:"date"`
}

func grind(ws *workspace.Workspace, r *http.Request) (journal.Event, string, error) {
	var req grindRequest
	if err := decode(r, &req); err != nil {
		return journal.Event{}, "", err
	}
	at, err := when(req.Date)
	if err != nil {
		return journal.Event{}, "", err
	}
	e, err := ws.Recorder.Grind(req.Product, req.Grams, at)
	if err != nil {
		return e, "", err
	}
	return e, fmt.Sprintf("ground %.2f g of %s", e.Grams, ws.ProductName(e.Product)), nil
}

type seshRequest struct {
	Product     string  `json:"product"`
	Grams       float64 `json:"grams"`
	Device      string  `json:"device"`
	Temperature int     `json:"temperature"`
	Note        string  `json:"note"`
	Date        string  `json:"date"`
}

func sesh(ws *workspace.Workspace, r *http.Request) (journal.Event, string, error) {
	var req seshRequest
	if err := decode(r, &req); err != nil {
		return journal.Event{}, "", err
	}
	at, err := when(req.Date)
	if err != nil {
		return journal.Event{}, "", err
	}
	e, err := ws.Recorder.Session(req.Product, req.Grams, at, req.Device, req.Temperature, strings.TrimSpace(req.Note))
	if err != nil {
		return e, "", err
	}
	return e, fmt.Sprintf("a session of %.2f g of %s", e.Grams, ws.ProductName(e.Product)), nil
}

type weighRequest struct {
	Product string  `json:"product"`
	Account string  `json:"account"`
	Weighed float64 `json:"weighed"`
	Note    string  `json:"note"`
}

func weigh(ws *workspace.Workspace, r *http.Request) (journal.Event, string, error) {
	var req weighRequest
	if err := decode(r, &req); err != nil {
		return journal.Event{}, "", err
	}
	e, err := ws.Recorder.Reconcile(req.Product, journal.Account(req.Account), req.Weighed, strings.TrimSpace(req.Note))
	if err != nil {
		return e, "", err
	}
	verb := "found"
	if e.From != journal.External {
		verb = "lost"
	}
	return e, fmt.Sprintf("%s %.2f g of %s in %s", verb, e.Grams, ws.ProductName(e.Product), req.Account), nil
}

type undoRequest struct {
	Hash   string `json:"hash"`
	Reason string `json:"reason"`
}

func undo(ws *workspace.Workspace, r *http.Request) (journal.Event, string, error) {
	var req undoRequest
	if err := decode(r, &req); err != nil {
		return journal.Event{}, "", err
	}
	e, err := ws.Recorder.Revert(req.Hash, strings.TrimSpace(req.Reason))
	if err != nil {
		return e, "", err
	}
	what := "entry"
	if original, err := ws.Recorder.Find(e.Reverts); err == nil {
		what = string(original.Type)
	}
	return e, fmt.Sprintf("undid the %s of %.2f g of %s", what, e.Grams, ws.ProductName(e.Product)), nil
}

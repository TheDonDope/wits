// Package api serves a wits repository over HTTP as JSON, for clients that are
// not a terminal — the Angular interface in wits-ui first.
//
// It is another caller of pkg/workspace, nothing more. Every request answers
// from the repository as it stands — read again whenever a file of it has
// changed, so what it answers is what wits status would say at that moment;
// every write goes through the recorder, under the journal's lock, with
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
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/research"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/version"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Server answers for one repository at a time. On a computer the interface
// can open another (desk.go); the phone has the one.
type Server struct {
	ui string

	// The repository, and the stamp of its files when it was last read: every
	// GET read and verified the whole journal, 12 ms here and ten times that
	// on a phone, for a journal that had not changed since the last. mu guards
	// all three, since opening another repository changes them together.
	mu     sync.Mutex
	repo   *repo.Repo
	cached *workspace.Workspace
	stamp  string

	// Where a terpene lookup asks, and what asks: the web, unless a test
	// stands in for it.
	sites  research.Sites
	client *http.Client
}

// New returns a server for the repository, serving the built interface from
// ui when it is not empty. On a computer, the interface can have it serve
// another (desk.go).
func New(r *repo.Repo, ui string) *Server {
	// The journal is created on first use; creating it here, before any
	// request, keeps two first requests from racing to create it.
	r.Journal()
	return &Server{repo: r, ui: ui, sites: sitesFor(runtime.GOOS), client: &http.Client{Timeout: researchTimeout}}
}

// sitesFor is where a terpene lookup asks: the web itself from a computer,
// and through a reader from the phone's engine, whose requests go out as the
// page's own and may read only sites that allow it.
func sitesFor(goos string) research.Sites {
	if goos == "js" {
		return research.Phone
	}
	return research.Web
}

// Handler routes the API, and the interface when there is one.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/overview", s.read(overviewOf))
	mux.HandleFunc("GET /api/journal", s.journal)
	mux.HandleFunc("GET /api/jars", s.read(jarsOf))
	mux.HandleFunc("GET /api/jars/{slug}", s.jarAt)
	mux.HandleFunc("GET /api/jars/{slug}/research", s.research)
	mux.HandleFunc("PUT /api/jars/{slug}/terpenes", s.terpenes)
	mux.HandleFunc("PUT /api/jars/{slug}/rating", s.rateStrain)
	mux.HandleFunc("GET /api/cycles", s.read(cyclesOf))
	mux.HandleFunc("GET /api/devices", s.read(devicesOf))
	mux.HandleFunc("POST /api/devices", s.addDevice)
	mux.HandleFunc("PUT /api/devices/{slug}", s.editDevice)
	mux.HandleFunc("GET /api/days", s.read(daysOf))
	mux.HandleFunc("GET /api/effects", s.read(effectsOf))
	mux.HandleFunc("POST /api/rate", s.rate)
	mux.HandleFunc("POST /api/label", s.label)
	mux.HandleFunc("POST /api/invoice", s.readInvoice)
	mux.HandleFunc("POST /api/fill", s.batch(recordFill))
	mux.HandleFunc("POST /api/grind", s.write(grind))
	mux.HandleFunc("POST /api/sesh", s.write(sesh))
	mux.HandleFunc("POST /api/weigh", s.write(weigh))
	mux.HandleFunc("GET /api/weigh/forgotten", s.forgottenAt)
	mux.HandleFunc("POST /api/weigh/forgotten", s.batch(weighForgotten))
	mux.HandleFunc("POST /api/undo", s.write(undo))
	mux.HandleFunc("POST /api/flush", s.flush)
	mux.HandleFunc("GET /api/avb", s.read(avbOf))
	mux.HandleFunc("GET /api/costs", s.read(costsOf))
	mux.HandleFunc("GET /api/unpriced", s.unpriced)
	mux.HandleFunc("POST /api/price", s.price)
	mux.HandleFunc("POST /api/price/fill", s.priceFill)
	mux.HandleFunc("GET /api/report.pdf", s.report)
	mux.HandleFunc("GET /api/carry", s.carry)
	mux.HandleFunc("GET /api/pack", s.pack)
	mux.HandleFunc("GET /api/review", s.year)
	mux.HandleFunc("GET /api/today", s.today)
	mux.HandleFunc("GET /api/strains", s.read(strainsOf))
	mux.HandleFunc("GET /api/terpenes", s.read(terpenesOf))
	mux.HandleFunc("POST /api/avb/collect", s.batch(collect))
	mux.HandleFunc("POST /api/avb/use", s.batch(useAVB))
	s.desk(mux)
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

// current is the repository being served.
func (s *Server) current() *repo.Repo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo
}

// Repo is the repository being served: the phone's engine syncs the one the
// page has open, and remembers it for the next time the page loads.
func (s *Server) Repo() *repo.Repo { return s.current() }

// serve makes r the repository being served, and forgets the last one's
// snapshot. A request already under way finishes with the one it began on.
func (s *Server) serve(r *repo.Repo) {
	r.Journal()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.repo, s.cached, s.stamp = r, nil, ""
}

// snapshot is the repository as it stands: the one read last, when nothing
// has been written since — its files' sizes and times unchanged — and read
// afresh when anything has. A snapshot is only read from; the time it
// measures against is now, whichever read it came from. Writes never use one:
// they read the journal under its lock, as the recorder does.
func (s *Server) snapshot() (*workspace.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stamp := workspace.Stamp(s.repo)
	if s.cached == nil || stamp != s.stamp {
		ws, err := workspace.Read(s.repo)
		if err != nil {
			return nil, err
		}
		s.cached, s.stamp = ws, stamp
	}
	ws := *s.cached
	ws.OpenedAt = time.Now()
	return &ws, nil
}

// read answers a GET from a snapshot of the repository.
func (s *Server) read(view func(*workspace.Workspace) any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ws, err := s.snapshot()
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
	if !jsonBody(w, r) {
		return nil, false
	}
	ws, err := workspace.Read(s.current())
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return nil, false
	}
	return ws, true
}

// jsonBody refuses a request whose body is not JSON of a sensible size, and
// answers the refusal itself.
func jsonBody(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, http.StatusUnsupportedMediaType, errors.New("send JSON"))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	return true
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

// batch is write for a recorder call that writes an entry a product.
func (s *Server) batch(do func(*workspace.Workspace, *http.Request) ([]journal.Event, string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, ok := s.writable(w, r)
		if !ok {
			return
		}
		events, message, err := do(ws, r)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		out := flushed{Entries: make([]entry, 0, len(events)), Message: message}
		for _, e := range events {
			out.Entries = append(out.Entries, entryOf(e, ws, nil))
		}
		reply(w, http.StatusCreated, out)
	}
}

type avbRequest struct {
	Grams  float64 `json:"grams"`
	Device string  `json:"device"`
	Date   string  `json:"date"`
	Note   string  `json:"note"`
}

// collect records the AVB weighed out of a device.
func collect(ws *workspace.Workspace, r *http.Request) ([]journal.Event, string, error) {
	var req avbRequest
	if err := decode(r, &req); err != nil {
		return nil, "", err
	}
	at, err := when(req.Date)
	if err != nil {
		return nil, "", err
	}
	events, err := ws.Recorder.CollectAVB(strings.TrimSpace(req.Device), req.Grams, at, strings.TrimSpace(req.Note))
	if err != nil {
		return nil, "", err
	}
	y := ledger.Yields(ws.Recorder.State().Events)
	last := y[len(y)-1]
	return events, fmt.Sprintf("collected %.2f g of AVB from %.2f g: %.0f%% back", last.Collected, last.From, last.Ratio()*100), nil
}

// useAVB records AVB used up.
func useAVB(ws *workspace.Workspace, r *http.Request) ([]journal.Event, string, error) {
	var req avbRequest
	if err := decode(r, &req); err != nil {
		return nil, "", err
	}
	at, err := when(req.Date)
	if err != nil {
		return nil, "", err
	}
	events, err := ws.Recorder.UseAVB(req.Grams, at, strings.TrimSpace(req.Note))
	if err != nil {
		return nil, "", err
	}
	return events, fmt.Sprintf("used %.2f g of AVB", req.Grams), nil
}

// avb is the AVB jar, what is waiting to be collected, and every yield.
type avb struct {
	Held    float64        `json:"held"`
	Waiting []waiting      `json:"waiting"`
	Yields  []yieldOfEntry `json:"yields"`
}

// waiting is a device's sessions since it was last emptied.
type waiting struct {
	Device   string  `json:"device"`
	Grams    float64 `json:"grams"`
	Sessions int     `json:"sessions"`
}

// yieldOfEntry is one collection, as ledger.Yield.
type yieldOfEntry struct {
	At        time.Time `json:"at"`
	Device    string    `json:"device"`
	Collected float64   `json:"collected"`
	From      float64   `json:"from"`
	Sessions  int       `json:"sessions"`
	Ratio     float64   `json:"ratio"`
}

func avbOf(ws *workspace.Workspace) any {
	out := avb{Waiting: []waiting{}, Yields: []yieldOfEntry{}}
	var held journal.Centigrams
	for _, b := range ws.State.Balances {
		held += journal.CG(b.AVB)
	}
	out.Held = held.Grams()
	for device, sessions := range ledger.Uncollected(ws.State.Events, ws.OpenedAt) {
		var g journal.Centigrams
		for _, e := range sessions {
			g += journal.CG(e.Grams)
		}
		out.Waiting = append(out.Waiting, waiting{Device: device, Grams: g.Grams(), Sessions: len(sessions)})
	}
	sort.Slice(out.Waiting, func(i, j int) bool { return out.Waiting[i].Device < out.Waiting[j].Device })
	for _, y := range ledger.Yields(ws.State.Events) {
		out.Yields = append(out.Yields, yieldOfEntry{At: y.At, Device: y.Device, Collected: y.Collected,
			From: y.From, Sessions: y.Sessions, Ratio: y.Ratio()})
	}
	return out
}

// costs is prices.Costs as a client reads it, cents as cents.
type costs struct {
	Cents       int64      `json:"cents"`
	Priced      float64    `json:"priced"`
	PerGram     float64    `json:"per_gram"`
	Unpriced    int        `json:"unpriced"`
	Helped      int        `json:"helped"`
	PerHelped   float64    `json:"per_helped"`
	Fills       []costFill `json:"fills"`
	Months      []month    `json:"months"`
	HelpedNoPay int        `json:"helped_unpriced"`
	insights
}

type costFill struct {
	Number    int       `json:"number"`
	Start     time.Time `json:"start"`
	Dispensed float64   `json:"dispensed"`
	Priced    float64   `json:"priced"`
	Cents     int64     `json:"cents"`
	PerGram   float64   `json:"per_gram"`
	Unpriced  int       `json:"unpriced"`
}

type month struct {
	Month string  `json:"month"`
	Cents int64   `json:"cents"`
	Grams float64 `json:"grams"`
}

func costsOf(ws *workspace.Workspace) any {
	c := prices.Analyse(ws.State, prices.Latest(ws.Prices), ws.Ratings)
	out := costs{Cents: c.Cents, Priced: c.Priced, PerGram: c.PerGram(), Unpriced: c.Unpriced, Helped: c.Helped,
		PerHelped: c.PerHelped(), HelpedNoPay: c.HelpedNoCost, Fills: []costFill{}, Months: []month{}}
	for _, f := range c.Fills {
		out.Fills = append(out.Fills, costFill{Number: f.Seq + 1, Start: f.Start, Dispensed: f.Dispensed,
			Priced: f.Priced, Cents: f.Cents, PerGram: f.PerGram(), Unpriced: f.Unpriced})
	}
	for _, m := range c.Months {
		out.Months = append(out.Months, month(m))
	}
	out.insights = insightsOf(ws)
	return out
}

// reading is what a pharmacy label says about a fill, as the buy form is
// filled from it: the product line, or the name of the product it already is.
type reading struct {
	Name  string  `json:"name"`
	THC   float64 `json:"thc,omitempty"`
	CBD   float64 `json:"cbd,omitempty"`
	Grams float64 `json:"grams,omitempty"`
	// Slug is the product already in the catalog the label names; Name is
	// then that product's, so the fill adds nothing.
	Slug string `json:"slug,omitempty"`
}

// label reads the text of a photographed label — the OCR ran on the device
// that took the photograph — into the product and grams of a fill. It records
// nothing: the form it fills is checked first.
func (s *Server) label(w http.ResponseWriter, r *http.Request) {
	if !jsonBody(w, r) {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	l := catalog.ReadLabel(req.Text)
	out := reading{Name: l.Name, THC: l.THC, CBD: l.CBD, Grams: l.Grams.Grams()}
	if p := ws.Products.Matching(l.Name); p != nil {
		out.Name, out.Slug = p.Name, p.Slug
	}
	reply(w, http.StatusOK, out)
}

// strain is one product on the map of the strains, as a client reads it:
// placed by its terpenes, when it has any written down.
type strain struct {
	Slug      string             `json:"slug"`
	Name      string             `json:"name"`
	X         float64            `json:"x"`
	Y         float64            `json:"y"`
	Placed    bool               `json:"placed"`
	THC       float64            `json:"thc"`
	CBD       float64            `json:"cbd"`
	Terpenes  []string           `json:"terpenes"`
	Aroma     map[string]float64 `json:"aroma"`
	Lead      string             `json:"lead,omitempty"`
	Taste     string             `json:"taste,omitempty"`
	Dispensed float64            `json:"dispensed"`
	Overall   float64            `json:"overall,omitempty"`
	Ratings   int                `json:"ratings"`
	Like      []strainLike       `json:"like"`
}

// strainLike is a product like another, how alike, and what they share.
type strainLike struct {
	Slug   string   `json:"slug"`
	Score  float64  `json:"score"`
	Shared []string `json:"shared"`
}

func strainsOf(ws *workspace.Workspace) any {
	all := report.Strains(ws)
	out := make([]strain, 0, len(all))
	for _, s := range all {
		row := strain{Slug: s.Slug, Name: s.Name, X: s.X, Y: s.Y, Placed: s.Placed, THC: s.THC, CBD: s.CBD,
			Terpenes: append([]string{}, s.Terpenes...), Aroma: map[string]float64{}, Lead: string(s.Lead), Taste: s.Taste,
			Dispensed: s.Dispensed, Overall: s.Overall, Ratings: s.Ratings, Like: []strainLike{}}
		for f, v := range s.Aroma {
			row.Aroma[string(f)] = v
		}
		for _, l := range s.Like {
			row.Like = append(row.Like, strainLike{Slug: l.Slug, Score: l.Score, Shared: append([]string{}, l.Shared...)})
		}
		out = append(out, row)
	}
	return out
}

// terpene is one terpene the shelf names, as a client reads it.
type terpene struct {
	Name         string   `json:"name"`
	Flavors      []string `json:"flavors"`
	Family       string   `json:"family,omitempty"`
	Notes        string   `json:"notes,omitempty"`
	Effects      []string `json:"effects"`
	BoilingPoint int      `json:"boiling_point,omitempty"`
	Products     int      `json:"products"`
	Leads        int      `json:"leads"`
	Overall      float64  `json:"overall,omitempty"`
	Rated        int      `json:"rated"`
}

// terpenesOf is every terpene the shelf's products name, the most named
// first: what each tastes of, and what their sessions were rated.
func terpenesOf(ws *workspace.Workspace) any {
	all := report.Terpenes(report.Strains(ws))
	out := make([]terpene, 0, len(all))
	for _, t := range all {
		out = append(out, terpene{Name: t.Name, Flavors: t.Flavors, Family: string(t.Family), Notes: t.Notes,
			Effects: t.Effects, BoilingPoint: t.BoilingPoint, Products: t.Products, Leads: t.Leads, Overall: t.Overall,
			Rated: t.Rated})
	}
	return out
}

// carry is a signed statement of what is held, with its QR code: the
// products on the shelf, or every jar holding anything with ?all=1. The
// sealing key is made the first time, as wits carry makes it.
func (s *Server) carry(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	items := report.Holdings(ws, r.URL.Query().Get("all") == "1")
	events, err := ws.Journal().Events()
	if err == nil && (len(items) == 0 || len(events) == 0) {
		err = errors.New("nothing is held: there is nothing to show")
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	key, _, err := seal.Key(ws.Repo.SealKeyPath())
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	c := seal.MakeCarry(key, events[len(events)-1].Hash, len(events), ws.OpenedAt, items)
	svg, err := report.QR(c.URL())
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	out := struct {
		Statement   string      `json:"statement"`
		URL         string      `json:"url"`
		Fingerprint string      `json:"fingerprint"`
		QR          string      `json:"qr"`
		Items       []carryItem `json:"items"`
	}{c.String(), c.URL(), seal.Fingerprint(c.Key), svg, make([]carryItem, 0, len(items))}
	for _, it := range items {
		ci := carryItem{Name: it.Name, Sealed: it.Sealed, Ground: it.Ground, AVB: it.AVB, Fills: []carryFill{}}
		for _, f := range it.Fills {
			ci.Fills = append(ci.Fills, carryFill(f))
		}
		out.Items = append(out.Items, ci)
	}
	reply(w, http.StatusOK, out)
}

// carryItem is one product held, as a client reads it.
type carryItem struct {
	Name   string      `json:"name"`
	Sealed float64     `json:"sealed"`
	Ground float64     `json:"ground"`
	AVB    float64     `json:"avb"`
	Fills  []carryFill `json:"fills"`
}

// carryFill is one fill a product held came from.
type carryFill struct {
	Day   string  `json:"day"`
	Grams float64 `json:"grams"`
}

// report is the page for the prescriber, as a PDF: the days ?from= to ?to=,
// both included, the last 90 by default.
func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	y, m, d := ws.OpenedAt.Date()
	to := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	from := to.AddDate(0, 0, -89)
	for name, at := range map[string]*time.Time{"from": &from, "to": &to} {
		if v := r.URL.Query().Get(name); v != "" {
			if *at, err = time.ParseInLocation(time.DateOnly, v, time.Local); err != nil {
				fail(w, http.StatusBadRequest, fmt.Errorf("%s %q is not a date, expected YYYY-MM-DD", name, v))
				return
			}
		}
	}
	if to.Before(from) {
		fail(w, http.StatusBadRequest, errors.New("the report ends before it starts"))
		return
	}
	var buf strings.Builder
	if err := report.PDF(&buf, report.Build(ws, from, to, version.String())); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="wits-report-`+to.Format(time.DateOnly)+`.pdf"`)
	_, _ = io.WriteString(w, buf.String())
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
	undone := ledger.Undone(ws.State.Events)
	out := effectLog{Dimensions: effects.Dimensions, Rated: []rated{}, Unrated: []entry{}}
	for i := len(ws.State.Events) - 1; i >= 0; i-- {
		e := ws.State.Events[i]
		if e.Type != journal.Sesh || undone[e.Hash] {
			continue
		}
		if r, ok := latest[e.Hash]; ok {
			out.Rated = append(out.Rated, rated{Hash: r.Hash, At: r.At, Scores: r.Scores, Note: r.Note,
				Session: entryOf(e, ws, nil)})
		} else if len(out.Unrated) < 12 && e.Note != record.ForgottenNote {
			// A session worked out at the scale was not remembered when it
			// happened; how it went is not remembered either.
			out.Unrated = append(out.Unrated, entryOf(e, ws, nil))
		}
	}
	out.Insights = effects.Analyse(ws.Ratings, ws.State.Events, ws.ProductName)
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

// overview is the front page: the fill in progress, and the shelf around it.
type overview struct {
	Now     time.Time `json:"now"`
	ChainOK bool      `json:"chain_ok"`
	Fill    *fill     `json:"fill"`
	// Fills is every fill still holding grams of its own, newest first: the
	// fill in progress, then each older one not yet emptied. Each speaks for
	// itself — its own grams, its own rate.
	Fills []*fill `json:"fills"`
	Shelf shelf   `json:"shelf"`
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
	// Refill is when to order the next prescription: the projection's empty
	// day, less the pharmacy's lead time.
	Refill *refill `json:"refill,omitempty"`
}

// refill is ledger.Refill as a client reads it.
type refill struct {
	OrderBy  string `json:"order_by"`
	EmptyOn  string `json:"empty_on"`
	LeadDays int    `json:"lead_days"`
	Overdue  bool   `json:"overdue"`
}

// projection is the shelf's grams day by day since the fill arrived, and on
// to the day they run out at the observed rate.
type projection struct {
	History   []point `json:"history"`
	Projected []point `json:"projected"`
	EmptyOn   string  `json:"empty_on,omitempty"`
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
	out := overview{Now: ws.OpenedAt, ChainOK: ws.Chain == nil, Fill: fillOf(ws), Fills: []*fill{}}
	if out.Fill != nil {
		out.Fills = append(out.Fills, out.Fill)
		current := ws.Cycle()
		for i := len(ws.State.Cycles) - 1; i >= 0; i-- {
			if c := &ws.State.Cycles[i]; c.Seq != current.Seq && c.Open() && !c.Start.After(ws.OpenedAt) {
				out.Fills = append(out.Fills, cycleFill(ws, c))
			}
		}
	}
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
	return out
}

func fillOf(ws *workspace.Workspace) *fill {
	c := ws.Cycle()
	if c == nil {
		return nil
	}
	f := cycleFill(ws, c)
	f.ShelfProjection = projectionOf(c, ws.OpenedAt)
	if plan, ok := ledger.PlanRefill(c, ws.OpenedAt, ws.Repo.Config.Lead()); ok {
		f.Refill = &refill{OrderBy: plan.OrderBy.Format(time.DateOnly), EmptyOn: plan.EmptyOn.Format(time.DateOnly),
			LeadDays: plan.Lead, Overdue: plan.Overdue(ws.OpenedAt)}
	}
	return f
}

// cycleFill is one fill in its own scope: its grams, its rate, its jars.
// The shelf's projection and the refill belong to the fill in progress, and
// fillOf adds them.
func cycleFill(ws *workspace.Workspace, c *ledger.Cycle) *fill {
	stats := ledger.Summarise(c.Standing)
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
		// Every fill answers a projection, empty for all but the one in
		// progress, so a client reads one shape.
		ShelfProjection: projection{History: []point{}, Projected: []point{}},
	}
	if c.Purchased > 0 {
		f.RemainingPct = float64(remaining / c.Purchased)
	}
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

// projectionOf is the ledger's projection of the shelf, as data: the shelf's
// grams on each day of the fill so far, then on at the fill's rate to the day
// the refill planner says it runs out.
func projectionOf(c *ledger.Cycle, now time.Time) projection {
	p := ledger.Project(c, now)
	out := projection{History: []point{}, Projected: []point{}}
	for _, l := range p.History {
		out.History = append(out.History, point{l.Day.Format(time.DateOnly), float64(l.Shelf)})
	}
	for _, l := range p.Projected {
		out.Projected = append(out.Projected, point{l.Day.Format(time.DateOnly), float64(l.Shelf)})
	}
	if !p.EmptyOn.IsZero() {
		out.EmptyOn = p.EmptyOn.Format(time.DateOnly)
	}
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
	for _, e := range ws.State.Standing {
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
	Mode        string    `json:"mode,omitempty"`
	Heater      string    `json:"heater,omitempty"`
	Setting     string    `json:"setting,omitempty"`
	Timer       int       `json:"timer,omitempty"`
	Note        string    `json:"note,omitempty"`
	Reverts     string    `json:"reverts,omitempty"`
	Corrected   bool      `json:"corrected,omitempty"`
}

func entryOf(e journal.Event, ws *workspace.Workspace, undone map[string]bool) entry {
	return entry{
		Hash: e.Hash, Type: string(e.Type), OccurredAt: e.OccurredAt, RecordedAt: e.RecordedAt,
		Product: e.Product, ProductName: ws.ProductName(e.Product), Grams: e.Grams,
		From: string(e.From), To: string(e.To), Device: e.Device, Temperature: e.Temperature,
		Mode: e.Mode, Heater: e.Heater, Setting: e.Setting, Timer: e.Timer, Note: e.Note, Reverts: e.Reverts, Corrected: undone[e.Hash] && e.Reverts == "",
	}
}

// journal answers the entries newest first: limit caps how many, type keeps
// one kind, and corrected pairs are left out unless all is set — the way the
// terminal's journal reads.
func (s *Server) journal(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
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
	undone := ledger.Undone(ws.State.Events)
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
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Cultivar     string   `json:"cultivar,omitempty"`
	THC          float64  `json:"thc,omitempty"`
	CBD          float64  `json:"cbd,omitempty"`
	Terpenes     []string `json:"terpenes"`
	// Rating is the strain rated as a whole, apart from any one session.
	Rating   *strainRating `json:"rating,omitempty"`
	Storage  float64       `json:"storage"`
	Stash    float64       `json:"stash"`
	AVB      float64       `json:"avb"`
	Bought   float64       `json:"bought"`
	Ground   float64       `json:"ground"`
	Fills    int           `json:"fills"`
	LastSeen time.Time     `json:"last_seen"`
	// LastUsed is when it was last ground or had in a session: what a person
	// remembers it by, where LastSeen counts any entry, a cleared stash too.
	LastUsed time.Time `json:"last_used,omitzero"`
}

func jarsOf(ws *workspace.Workspace) any {
	bought, ground := map[string]journal.Centigrams{}, map[string]journal.Centigrams{}
	fills, seen, used := map[string]int{}, map[string]time.Time{}, map[string]time.Time{}
	for _, e := range ws.State.Standing {
		switch e.Type {
		case journal.Purchase:
			bought[e.Product] += journal.CG(e.Grams)
			fills[e.Product]++
		case journal.Grind:
			ground[e.Product] += journal.CG(e.Grams)
		}
		if (e.Type == journal.Grind || e.Type == journal.Sesh) && e.OccurredAt.After(used[e.Product]) {
			used[e.Product] = e.OccurredAt
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
			Bought: bought[slug].Grams(), Ground: ground[slug].Grams(), Fills: fills[slug], LastSeen: seen[slug],
			LastUsed: used[slug], Terpenes: []string{}}
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
		j.Terpenes = append(j.Terpenes, p.Terpenes...)
		j.Rating = ratingOf(p.Rating)
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
	Mode        string  `json:"mode"`
	Heater      string  `json:"heater"`
	Setting     string  `json:"setting"`
	Timer       int     `json:"timer"`
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
	e, err := ws.Recorder.SessionWith(record.Sesh{Product: req.Product, Grams: req.Grams, At: at, Device: req.Device,
		Temp: req.Temperature, Mode: req.Mode, Heater: req.Heater, Setting: req.Setting, Timer: req.Timer, Note: strings.TrimSpace(req.Note)})
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

// Package api serves a repository over HTTP; this file serves one jar in
// full — what the catalog knows of the product, every fill of it with its
// pickups and prices, its grinding month by month, and its sessions — and
// writes down its terpenes, looked up on the web or typed in.
package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/research"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// jarDetail is one product across its life — the jar's scope — and each
// fill of it, in that fill's scope.
type jarDetail struct {
	jar
	Country   string    `json:"country,omitempty"`
	Radiated  bool      `json:"radiated,omitempty"`
	Strain    string    `json:"strain"`
	Terpenes  []string  `json:"terpenes"`
	FirstSeen time.Time `json:"first_seen"`
	// Seshed is what went through a device from the stash, logged as
	// sessions; Flushed is what was cleared from the stash unlogged.
	Seshed   float64    `json:"seshed"`
	Flushed  float64    `json:"flushed"`
	Fills    []jarFill  `json:"fill_list"`
	Price    jarPrice   `json:"price"`
	Months   []jarMonth `json:"months"`
	Sessions jarSesh    `json:"sessions"`
	Like     []named    `json:"like"`
}

// jarFill is one fill of the product — the fill's scope: what it dispensed
// of it, what of that is still sealed, and the pickups that brought it.
type jarFill struct {
	Number    int       `json:"number"`
	Start     time.Time `json:"start"`
	Open      bool      `json:"open"`
	Dispensed float64   `json:"dispensed"`
	Sealed    float64   `json:"sealed"`
	Pickups   []pickup  `json:"pickups"`
}

// pickup is one purchase of the product, with what it cost when that is
// written down.
type pickup struct {
	Hash    string    `json:"hash"`
	At      time.Time `json:"at"`
	Grams   float64   `json:"grams"`
	Cents   int64     `json:"cents,omitempty"`
	PerGram float64   `json:"per_gram,omitempty"`
	Unknown bool      `json:"unknown,omitempty"`
	Priced  bool      `json:"priced"`
	Seller  string    `json:"seller,omitempty"`
	Note    string    `json:"note,omitempty"`
}

// jarPrice is what the jar's priced pickups cost together.
type jarPrice struct {
	Cents    int64   `json:"cents"`
	Priced   float64 `json:"priced"`
	PerGram  float64 `json:"per_gram"`
	Unpriced int     `json:"unpriced"`
}

// jarMonth is what of the jar was ground and put through a device in a month.
type jarMonth struct {
	Month  string  `json:"month"`
	Ground float64 `json:"ground"`
	Seshed float64 `json:"seshed"`
}

// jarSesh is the jar's sessions: how many, on which devices and at what
// temperatures, and how they were rated — each dimension's average.
type jarSesh struct {
	Count   int                           `json:"count"`
	Grams   float64                       `json:"grams"`
	First   time.Time                     `json:"first,omitzero"`
	Last    time.Time                     `json:"last,omitzero"`
	Devices []jarDevice                   `json:"devices"`
	MinTemp int                           `json:"min_temp,omitempty"`
	MaxTemp int                           `json:"max_temp,omitempty"`
	Rated   int                           `json:"rated"`
	Scores  map[effects.Dimension]float64 `json:"scores"`
	Recent  []rated                       `json:"recent"`
}

// jarDevice is the sessions of the jar one device had.
type jarDevice struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	Sessions int     `json:"sessions"`
	Grams    float64 `json:"grams"`
}

// named is a product by its slug and name.
type named struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// errNoJar is a slug the repository knows nothing of.
var errNoJar = errors.New("no product by that slug")

// jarAt answers one jar in full.
func (s *Server) jarAt(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	out, ok := jarOf(ws, r.PathValue("slug"))
	if !ok {
		fail(w, http.StatusNotFound, errNoJar)
		return
	}
	reply(w, http.StatusOK, out)
}

// jarOf is one product in full, or false when the journal never names it.
func jarOf(ws *workspace.Workspace, slug string) (*jarDetail, bool) {
	var j *jar
	for _, each := range jarsOf(ws).([]jar) {
		if each.Slug == slug {
			j = &each
			break
		}
	}
	if j == nil {
		return nil, false
	}
	out := &jarDetail{jar: *j, Terpenes: []string{}, Like: []named{}}
	if p := ws.Products.Exact(slug); p != nil {
		out.Country, out.Radiated = p.Country, p.Radiated
		out.Terpenes = append(out.Terpenes, p.Terpenes...)
		_, out.Strain = prices.StrainOf(productOf(p), producers(ws))
	}
	out.Fills, out.Price = jarFills(ws, slug)
	out.Months, out.FirstSeen, out.Seshed, out.Flushed = monthsOf(ws, slug)
	out.Sessions = seshOf(ws, slug)
	out.Like = likeOf(ws, slug)
	return out, true
}

// productOf is what the price breakdowns read of a product.
func productOf(p *catalog.Product) prices.Product {
	return prices.Product{Name: p.Name, Maker: p.Manufacturer, Cultivar: p.Cultivar, THC: p.THC, CBD: p.CBD}
}

// producers is the words every producer in the catalog is named by.
func producers(ws *workspace.Workspace) map[string]bool {
	all := make([]prices.Product, 0, len(ws.Products.Products))
	for _, p := range ws.Products.Products {
		all = append(all, productOf(p))
	}
	return prices.Producers(all)
}

// jarFills is every fill of the product, newest first, each with its pickups
// and their prices, and what the priced ones cost together.
func jarFills(ws *workspace.Workspace, slug string) ([]jarFill, jarPrice) {
	latest := prices.Latest(ws.Prices)
	out := []jarFill{}
	var total jarPrice
	var priced journal.Centigrams
	for i := len(ws.State.Cycles) - 1; i >= 0; i-- {
		c := &ws.State.Cycles[i]
		if !slices.Contains(c.Products, slug) {
			continue
		}
		f := jarFill{Number: c.Seq + 1, Start: c.Start, Open: c.Open(), Dispensed: float64(c.PurchasedOf(slug)),
			Sealed: float64(ws.State.ShareOf(c, slug)), Pickups: []pickup{}}
		for _, e := range c.Standing {
			if e.Type != journal.Purchase || e.Product != slug {
				continue
			}
			p := pickupOf(e, latest)
			if p.Priced {
				total.Cents += p.Cents
				priced += journal.CG(p.Grams)
			} else if !p.Unknown {
				total.Unpriced++
			}
			f.Pickups = append(f.Pickups, p)
		}
		out = append(out, f)
	}
	total.Priced = priced.Grams()
	if total.Priced > 0 {
		total.PerGram = float64(total.Cents) / total.Priced
	}
	return out, total
}

// pickupOf is a purchase with its standing price, if it has one.
func pickupOf(e journal.Event, latest map[string]prices.Price) pickup {
	p := pickup{Hash: e.Hash, At: e.OccurredAt, Grams: e.Grams}
	if price, ok := latest[e.Hash]; ok {
		p.Unknown, p.Seller, p.Note = price.Unknown, price.Seller, price.Note
		if !price.Unknown {
			p.Priced, p.Cents = true, price.Cents
			if e.Grams > 0 {
				p.PerGram = float64(price.Cents) / e.Grams
			}
		}
	}
	return p
}

// monthsOf is what of the product was ground and seshed each month, oldest
// first; when the journal first names it; and what left its stash, as
// sessions and as clearings.
func monthsOf(ws *workspace.Workspace, slug string) ([]jarMonth, time.Time, float64, float64) {
	ground, seshed := map[string]journal.Centigrams{}, map[string]journal.Centigrams{}
	var first time.Time
	var sesh, flushed journal.Centigrams
	for _, e := range ws.State.Standing {
		if e.Product != slug {
			continue
		}
		if first.IsZero() || e.OccurredAt.Before(first) {
			first = e.OccurredAt
		}
		m := e.OccurredAt.Format("2006-01")
		switch {
		case e.Type == journal.Grind:
			ground[m] += journal.CG(e.Grams)
		case e.Type == journal.Sesh:
			seshed[m] += journal.CG(e.Grams)
			sesh += journal.CG(e.Grams)
		case e.Type == journal.Adjust && e.From == journal.Stash && e.To == journal.Consumed:
			flushed += journal.CG(e.Grams)
		}
	}
	months := make([]string, 0, len(ground)+len(seshed))
	for m := range ground {
		months = append(months, m)
	}
	for m := range seshed {
		months = append(months, m)
	}
	slices.Sort(months)
	out := []jarMonth{}
	for _, m := range slices.Compact(months) {
		out = append(out, jarMonth{Month: m, Ground: ground[m].Grams(), Seshed: seshed[m].Grams()})
	}
	return out, first, sesh.Grams(), flushed.Grams()
}

// recentSessions is how many of the jar's latest sessions are listed.
const recentSessions = 8

// seshOf is the jar's sessions: by device, the temperatures they ran at,
// and their ratings, the latest of each session's standing.
func seshOf(ws *workspace.Workspace, slug string) jarSesh {
	out := jarSesh{Devices: []jarDevice{}, Scores: map[effects.Dimension]float64{}, Recent: []rated{}}
	latest := effects.Latest(ws.Ratings)
	devices := map[string]*jarDevice{}
	sums, counts := map[effects.Dimension]int{}, map[effects.Dimension]int{}
	var grams journal.Centigrams
	for _, e := range ws.State.Standing {
		if e.Type != journal.Sesh || e.Product != slug {
			continue
		}
		out.Count++
		grams += journal.CG(e.Grams)
		if out.First.IsZero() {
			out.First = e.OccurredAt
		}
		out.Last = e.OccurredAt
		tally(devices, ws, e)
		temps(&out, e.Temperature)
		if r, ok := latest[e.Hash]; ok {
			out.Rated++
			for d, v := range r.Scores {
				sums[d] += v
				counts[d]++
			}
		}
	}
	out.Grams = grams.Grams()
	for d, n := range counts {
		out.Scores[d] = ledger.Round(float64(sums[d]) / float64(n))
	}
	for _, d := range devices {
		out.Devices = append(out.Devices, *d)
	}
	sort.Slice(out.Devices, func(i, k int) bool { return out.Devices[i].Sessions > out.Devices[k].Sessions })
	out.Recent = recentOf(ws, slug, latest)
	return out
}

// tally counts a session on the device it was taken with.
func tally(devices map[string]*jarDevice, ws *workspace.Workspace, e journal.Event) {
	d := devices[e.Device]
	if d == nil {
		d = &jarDevice{Slug: e.Device, Name: e.Device}
		for _, dev := range ws.Devices.Devices {
			if dev.Slug == e.Device {
				d.Name = dev.Name
			}
		}
		if e.Device == "" {
			d.Name = "no device named"
		}
		devices[e.Device] = d
	}
	d.Sessions++
	d.Grams = ledger.Round(d.Grams + e.Grams)
}

// temps widens the range of temperatures the sessions ran at.
func temps(out *jarSesh, t int) {
	if t <= 0 {
		return
	}
	if out.MinTemp == 0 || t < out.MinTemp {
		out.MinTemp = t
	}
	out.MaxTemp = max(out.MaxTemp, t)
}

// recentOf is the jar's latest sessions, newest first, each with its rating
// when it has one.
func recentOf(ws *workspace.Workspace, slug string, latest map[string]effects.Rating) []rated {
	out := []rated{}
	for i := len(ws.State.Standing) - 1; i >= 0 && len(out) < recentSessions; i-- {
		e := ws.State.Standing[i]
		if e.Type != journal.Sesh || e.Product != slug {
			continue
		}
		row := rated{Session: entryOf(e, ws, nil)}
		if r, ok := latest[e.Hash]; ok {
			row.Hash, row.At, row.Scores, row.Note = r.Hash, r.At, r.Scores, r.Note
		}
		out = append(out, row)
	}
	return out
}

// likeOf is the products most like this one, as the map of the strains
// reckons it.
func likeOf(ws *workspace.Workspace, slug string) []named {
	out := []named{}
	for _, s := range report.Strains(ws) {
		if s.Slug != slug {
			continue
		}
		for _, other := range s.Like {
			out = append(out, named{Slug: other, Name: ws.ProductName(other)})
		}
	}
	return out
}

// researchTimeout is how long a lookup waits on the web, every site together.
const researchTimeout = 25 * time.Second

// research looks the product's terpenes up on the web. It writes nothing;
// the client offers what it found, to be kept or not.
func (s *Server) research(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	p := ws.Products.Exact(r.PathValue("slug"))
	if p == nil {
		fail(w, http.StatusNotFound, errNoJar)
		return
	}
	_, strain := prices.StrainOf(productOf(p), producers(ws))
	ctx, cancel := context.WithTimeout(r.Context(), researchTimeout)
	defer cancel()
	found, err := research.Look(ctx, s.client, s.sites, research.Query{Name: p.Name, Strain: strain})
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	reply(w, http.StatusOK, found)
}

// terpenes writes down a product's terpenes, dominant first; none clears them.
func (s *Server) terpenes(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.writable(w, r)
	if !ok {
		return
	}
	var req struct {
		Terpenes []string `json:"terpenes"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	slug := r.PathValue("slug")
	if ws.Products.Exact(slug) == nil {
		fail(w, http.StatusNotFound, errNoJar)
		return
	}
	terps := catalog.ParseTerpenes(strings.Join(req.Terpenes, ","))
	p, err := ws.Recorder.Terpenes(slug, terps)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	message := "terpenes of " + p.Name + " cleared"
	if len(terps) > 0 {
		message = p.Name + ": " + strings.Join(terps, ", ")
	}
	reply(w, http.StatusCreated, map[string]any{"message": message, "terpenes": append([]string{}, terps...)})
}

// Package api serves a repository over HTTP; this file prices purchases,
// one at a time or a whole fill at once, and lists those still without.
package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// unpricedEntry is a purchase without a price, with the fill it came in:
// purchases within ledger.CycleGap of each other are one pickup, priced
// together on one prescription.
type unpricedEntry struct {
	entry
	Fill      int       `json:"fill"`
	FillStart time.Time `json:"fill_start"`
}

// unpriced is the purchases without a price, for reconcile purchases: those
// still in storage, or every one with ?all=1.
func (s *Server) unpriced(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	events, err := ws.Recorder.Unpriced(r.URL.Query().Get("all") == "1")
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	// A purchase opens or joins exactly one cycle, so the first cycle that
	// holds it is its fill.
	fill := map[string]int{}
	for _, c := range ws.State.Cycles {
		for _, e := range c.Events {
			if _, seen := fill[e.Hash]; !seen && e.Type == journal.Purchase {
				fill[e.Hash] = c.Seq
			}
		}
	}
	out := make([]unpricedEntry, 0, len(events))
	for _, e := range events {
		seq := fill[e.Hash]
		out = append(out, unpricedEntry{entry: entryOf(e, ws, nil), Fill: seq + 1, FillStart: ws.State.Cycles[seq].Start})
	}
	reply(w, http.StatusOK, out)
}

// price writes down what a purchase cost.
func (s *Server) price(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.writable(w, r)
	if !ok {
		return
	}
	var req struct {
		Purchase string `json:"purchase"`
		Euros    string `json:"euros"`
		Note     string `json:"note"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	cents, err := prices.ParseEuros(req.Euros)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	p, err := ws.Recorder.Price(req.Purchase, cents, req.Note, time.Now())
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	e, _ := ws.Recorder.Find(p.Purchase)
	reply(w, http.StatusCreated, map[string]string{"message": fmt.Sprintf("%s for %.2f g of %s",
		prices.Format(cents), e.Grams, ws.ProductName(e.Product))})
}

// errNothingPriced is a fill sent with no price on any of its lines.
var errNothingPriced = errors.New("no price given; nothing to write down")

// priceLine is one purchase of a fill being priced: a price a gram, or its
// invoice's line total, or none to be had (Unknown); neither, and it keeps
// no price.
type priceLine struct {
	Purchase string `json:"purchase"`
	PerGram  string `json:"per_gram"`
	Total    string `json:"total"`
	Unknown  bool   `json:"unknown"`
	Note     string `json:"note"`
}

// priceFill writes down a fill's prices: for each of its purchases a price a
// gram, VAT added when asked for, or the line's total off its invoice, which
// includes VAT — worked out by lineCents, as a fill recorded with its prices
// is, on the grams the journal holds — or that none is to be had, so it is
// asked about no more. A line left blank keeps no price; any bad line
// refuses the fill.
func (s *Server) priceFill(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.writable(w, r)
	if !ok {
		return
	}
	var req struct {
		Lines  []priceLine `json:"lines"`
		VAT    bool        `json:"vat"`
		Seller string      `json:"seller"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	paid, grams, err := fillPrices(ws, req.Lines, req.VAT)
	for i := range paid {
		paid[i].Seller = req.Seller
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if _, err := ws.Recorder.PriceAll(paid, time.Now()); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w, http.StatusCreated, map[string]string{"message": pricedMessage(paid, grams)})
}

// pricedMessage says what a fill's pricing wrote: what the priced ones cost,
// and how many have none to be had.
func pricedMessage(paid []prices.Price, grams float64) string {
	var cents int64
	priced, unknown := 0, 0
	for _, p := range paid {
		if p.Unknown {
			unknown++
			continue
		}
		cents += p.Cents
		priced++
	}
	var parts []string
	if priced > 0 {
		parts = append(parts, fmt.Sprintf("%s for %.2f g, %s", prices.Format(cents), grams, count(priced, "purchase")))
	}
	if unknown > 0 {
		parts = append(parts, count(unknown, "purchase")+" with no price to be had, asked about no more")
	}
	return strings.Join(parts, "; ")
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// fillPrices is what each line of a fill cost, on the grams the journal
// holds for its purchase, and the grams of those with a price.
func fillPrices(ws *workspace.Workspace, lines []priceLine, vat bool) ([]prices.Price, float64, error) {
	var paid []prices.Price
	var grams journal.Centigrams
	for _, l := range lines {
		e, err := ws.Recorder.Find(l.Purchase)
		if err != nil {
			return nil, 0, err
		}
		if l.Unknown {
			paid = append(paid, prices.Price{Purchase: e.Hash, Unknown: true, Note: l.Note})
			continue
		}
		c, err := lineCents(fillLine{Grams: e.Grams, PerGram: l.PerGram, Total: l.Total}, vat)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", ws.ProductName(e.Product), err)
		}
		if c == 0 {
			continue
		}
		paid = append(paid, prices.Price{Purchase: e.Hash, Cents: c, Note: l.Note})
		grams += journal.CG(e.Grams)
	}
	if len(paid) == 0 {
		return nil, 0, errNothingPriced
	}
	return paid, grams.Grams(), nil
}

// Package api serves a repository over HTTP; this file finds the fill an
// invoice is for among those already recorded, so a pickup is priced from its
// invoice rather than recorded a second time.
package api

import (
	"math"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// invoiceWindow is how far apart an invoice's date and a pickup's may be and
// still be one fill. The invoice is dated the day it was ordered, the fill
// the day it was picked up: a day or two apart, rarely more than a week.
const invoiceWindow = 7 * 24 * time.Hour

// invoiceMatch is a recorded fill that holds an invoice's strains near its
// date. Days is how many days apart they are, 0 the same day; Lines is, for
// each line of the invoice, the purchase it is, or "" when the fill has none.
type invoiceMatch struct {
	Fill   int       `json:"fill"`
	Start  time.Time `json:"start"`
	Days   int       `json:"days"`
	Lines  []string  `json:"lines"`
	Priced int       `json:"priced"`
}

// matchInvoice is the fill the invoice is for: of the pickups within
// invoiceWindow of its date, the one holding most of its strains, the nearer
// on a tie. None holding any of them: nil, and the invoice is a new fill.
func matchInvoice(ws *workspace.Workspace, date time.Time, lines []invoiceLine) *invoiceMatch {
	standing := map[string]bool{}
	for _, e := range ws.State.Standing {
		standing[e.Hash] = true
	}
	var best *invoiceMatch
	bestHits := 0
	for i := range ws.State.Cycles {
		c := &ws.State.Cycles[i]
		if math.Abs(float64(c.Start.Sub(date))) > float64(invoiceWindow) {
			continue
		}
		m, hits := fillMatch(c, date, lines, standing)
		if hits > bestHits || (hits == bestHits && best != nil && m.Days < best.Days) {
			best, bestHits = m, hits
		}
	}
	if best == nil || bestHits == 0 {
		return nil
	}
	latest := prices.Latest(ws.Prices)
	for _, h := range best.Lines {
		if _, ok := latest[h]; ok && h != "" {
			best.Priced++
		}
	}
	return best
}

// fillMatch is how a fill's standing purchases answer an invoice's lines,
// and how many of the lines it holds.
func fillMatch(c *ledger.Cycle, date time.Time, lines []invoiceLine, standing map[string]bool) (*invoiceMatch, int) {
	bought := map[string]string{} // product → purchase
	for _, e := range c.Events {
		if e.Type == journal.Purchase && standing[e.Hash] {
			bought[e.Product] = e.Hash
		}
	}
	m := &invoiceMatch{Fill: c.Seq + 1, Start: c.Start, Days: daysApart(c.Start, date), Lines: make([]string, len(lines))}
	hits := 0
	for i, l := range lines {
		if h := bought[l.Slug]; l.Slug != "" && h != "" {
			m.Lines[i] = h
			hits++
		}
	}
	return m, hits
}

// daysApart counts the calendar days between two moments, either way round.
func daysApart(a, b time.Time) int {
	day := func(t time.Time) time.Time {
		y, m, d := t.In(time.Local).Date()
		return time.Date(y, m, d, 12, 0, 0, 0, time.Local)
	}
	return int(math.Abs(math.Round(day(a).Sub(day(b)).Hours() / 24)))
}

// Package api serves a repository over HTTP; this file finds the fill an
// invoice is for among those already recorded, so a pickup is priced from its
// invoice rather than recorded a second time.
package api

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// invoiceDays is how many days apart an invoice's date and a pickup's may be
// and still be one fill. The invoice is dated the day it was ordered, the
// fill the day it was picked up: the same day or a few apart, a week when the
// pharmacy ships, and once, of 37 invoices, 13 days. A fill must hold the
// invoice's strains to match, and of fills that match alike the nearer wins,
// so the next fill along, a month off, does not take it.
const invoiceDays = 14

// invoiceMatch is a recorded fill that holds an invoice's strains near its
// date. Days is how many days apart they are, 0 the same day; Lines is, for
// each line of the invoice, the purchase it is, or "" when the fill has none.
type invoiceMatch struct {
	Fill   int       `json:"fill"`
	Start  time.Time `json:"start"`
	Days   int       `json:"days"`
	Lines  []string  `json:"lines"`
	Priced int       `json:"priced"`
	score  int
}

// matchInvoice is the fill the invoice is for: of the pickups within
// invoiceDays of its date, the one holding most of its strains, then the
// better match, then the nearer. None holding any of them: nil, and the
// invoice is a new fill.
func matchInvoice(ws *workspace.Workspace, date time.Time, lines []invoiceLine) *invoiceMatch {
	standing := map[string]bool{}
	for _, e := range ws.State.Standing {
		standing[e.Hash] = true
	}
	var best *invoiceMatch
	bestHits := 0
	for i := range ws.State.Cycles {
		c := &ws.State.Cycles[i]
		if daysApart(c.Start, date) > invoiceDays {
			continue
		}
		m, hits := fillMatch(ws, c, date, lines, standing)
		if hits > bestHits || (hits == bestHits && best != nil && better(m, best)) {
			best, bestHits = m, hits
		}
	}
	if best == nil || bestHits == 0 {
		return nil
	}
	latest := prices.Latest(ws.Prices)
	for _, h := range best.Lines {
		if p, ok := latest[h]; ok && h != "" && !p.Unknown {
			best.Priced++
		}
	}
	return best
}

func better(a, b *invoiceMatch) bool {
	return a.score > b.score || (a.score == b.score && a.Days < b.Days)
}

// pair is one invoice line against one of a fill's purchases, scored.
type pair struct {
	line, buy, score int
}

// fillMatch is how a fill's standing purchases answer an invoice's lines,
// each line given the purchase it shares most with, one purchase a line, and
// how many of the lines it holds.
func fillMatch(ws *workspace.Workspace, c *ledger.Cycle, date time.Time, lines []invoiceLine, standing map[string]bool) (*invoiceMatch, int) {
	var bought []journal.Event
	for _, e := range c.Events {
		if e.Type == journal.Purchase && standing[e.Hash] {
			bought = append(bought, e)
		}
	}
	var pairs []pair
	for i, l := range lines {
		for j, e := range bought {
			if s := likeness(l, e, ws.ProductName(e.Product)); s > 0 {
				pairs = append(pairs, pair{i, j, s})
			}
		}
	}
	sort.SliceStable(pairs, func(a, b int) bool { return pairs[a].score > pairs[b].score })
	m := &invoiceMatch{Fill: c.Seq + 1, Start: c.Start, Days: daysApart(c.Start, date), Lines: make([]string, len(lines))}
	taken := map[int]bool{}
	hits := 0
	for _, p := range pairs {
		if m.Lines[p.line] != "" || taken[p.buy] {
			continue
		}
		m.Lines[p.line], taken[p.buy] = bought[p.buy].Hash, true
		m.score += p.score
		hits++
	}
	return m, hits
}

// likeness scores how surely an invoice's line is a purchase: the same jar
// outright; else the strain's words they share, more when the strength and
// the grams agree. A strength both print and print differently is another
// product, whatever the name shares — 14/1 White Widow is not 16/1.
func likeness(l invoiceLine, e journal.Event, name string) int {
	if l.Slug != "" && l.Slug == e.Product {
		return 1000
	}
	words, strength := tokens(l.Printed)
	theirs, theirStrength := tokens(name)
	if strength != "" && theirStrength != "" && strength != theirStrength {
		return 0
	}
	shared := 0
	for w := range words {
		if theirs[w] {
			shared++
		}
	}
	if shared == 0 {
		return 0
	}
	score := shared * 10
	if strength != "" && strength == theirStrength {
		score += 5
	}
	if math.Abs(l.Grams-e.Grams) < 0.005 {
		score += 3
	}
	return score
}

var (
	word     = regexp.MustCompile(`[a-zäöüß]{3,}`)
	strength = regexp.MustCompile(`(\d+)\s*[/:]\s*0*(\d+)`)
)

// tokens is a name's words, lowercased and singular ("Cookies" is a
// cookie), and its strength as THC/CBD: "TRUU GT 25:01" is 25/1.
func tokens(name string) (map[string]bool, string) {
	lower := strings.ToLower(name)
	words := map[string]bool{}
	for _, w := range word.FindAllString(lower, -1) {
		if len(w) > 4 {
			w = strings.TrimSuffix(w, "s")
		}
		words[w] = true
	}
	var s string
	if m := strength.FindStringSubmatch(lower); m != nil {
		s = m[1] + "/" + m[2]
	}
	return words, s
}

// daysApart counts the calendar days between two moments, either way round.
func daysApart(a, b time.Time) int {
	day := func(t time.Time) time.Time {
		y, m, d := t.In(time.Local).Date()
		return time.Date(y, m, d, 12, 0, 0, 0, time.Local)
	}
	return int(math.Abs(math.Round(day(a).Sub(day(b)).Hours() / 24)))
}

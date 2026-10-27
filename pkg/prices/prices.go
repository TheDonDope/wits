// Package prices is what each fill cost: the price paid for a purchase,
// recorded beside the journal rather than in it.
//
// A purchase is never edited in place, and a price is not a transfer of grams,
// so it is not a journal entry either. It keeps the journal's disciplines the
// way the effect log does: append-only, written under a lock, hash-chained,
// each hash checked against the line as stored. A price names its purchase by
// the purchase's hash, and pricing a purchase again supersedes the earlier
// price, which stays in the log. Amounts are euro cents, the currency every
// prescription here has been paid in.
package prices

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/chain"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// ErrBrokenChain is returned when the log's hash chain does not verify.
var ErrBrokenChain = errors.New("price log hash chain is broken")

// Price is what was paid for one purchase. Field order is significant: the
// hash is taken over the JSON encoding.
//
// Unknown is a price that is not to be had — no invoice kept, nothing
// remembered — written down so the purchase stops being asked about. It
// costs nothing in the sums; the purchase stays without a price, and a price
// given later supersedes it. Left out when false, so the records written
// before it hash as they did.
//
// Seller is the pharmacy the price was paid to, as its invoice names it;
// empty when it was typed in, and then left out, for the same reason.
type Price struct {
	Seq      int       `json:"seq"`
	Purchase string    `json:"purchase"`
	At       time.Time `json:"at"`
	Cents    int64     `json:"cents"`
	Unknown  bool      `json:"unknown,omitempty"`
	Seller   string    `json:"seller,omitempty"`
	Note     string    `json:"note,omitempty"`
	Prev     string    `json:"prev"`
	Hash     string    `json:"hash"`
}

// Euros is the price in euros.
func (p Price) Euros() float64 { return float64(p.Cents) / 100 }

// Validate reports whether a price is well formed: a purchase named, and an
// amount paid — or none, for a price not to be had.
func (p Price) Validate() error {
	if p.Purchase == "" {
		return errors.New("a price names the purchase it was paid for")
	}
	if p.Unknown {
		if p.Cents != 0 {
			return errors.New("a price not to be had has no amount")
		}
		return nil
	}
	if p.Cents <= 0 {
		return fmt.Errorf("a price is more than nothing, got %s", Format(p.Cents))
	}
	return nil
}

// ParseEuros reads an amount of euros as typed — "120.50", "120,50 €",
// "€120" — into cents.
func ParseEuros(s string) (int64, error) {
	t := strings.TrimSpace(strings.NewReplacer("€", "", "EUR", "", "eur", "").Replace(s))
	t = strings.ReplaceAll(strings.TrimSpace(t), ",", ".")
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || f <= 0 || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%q is not an amount of euros", s)
	}
	cents := math.Round(f * 100)
	if math.Abs(f*100-cents) > 1e-6 {
		return 0, fmt.Errorf("%q is finer than a cent", s)
	}
	return int64(cents), nil
}

// Format writes cents as euros.
func Format(cents int64) string {
	return fmt.Sprintf("%.2f €", float64(cents)/100)
}

// Log is the price log: a chain of prices (pkg/chain).
type Log = chain.Log[Price, *Price]

// Open returns the price log at path; it is created on the first append.
func Open(path string) *Log {
	return chain.Open[Price](path, "price log", "price", ErrBrokenChain)
}

// Chain seals prices one onto the next from the start of a log, as appending
// them to an empty one would: what a bundle's prices hash to once restored.
func Chain(prices []Price) ([]Price, error) { return chain.Chain[Price](prices) }

// Link is the price's place in the log.
func (p *Price) Link() chain.Link {
	return chain.Link{Seq: p.Seq, At: p.At, Prev: p.Prev, Hash: p.Hash}
}

// SetLink places the price in the log.
func (p *Price) SetLink(l chain.Link) { p.Seq, p.At, p.Prev, p.Hash = l.Seq, l.At, l.Prev, l.Hash }

// Latest returns each purchase's newest price: a purchase priced again costs
// what it was priced at last.
func Latest(prices []Price) map[string]Price {
	out := make(map[string]Price, len(prices))
	for _, p := range prices {
		out[p.Purchase] = p
	}
	return out
}

// Fill is what one prescription fill cost: its purchases, the grams of them
// that have a price, and what those cost.
type Fill struct {
	Seq       int
	Start     time.Time
	Dispensed float64
	Priced    float64
	Cents     int64
	Unpriced  int
}

// PerGram is the fill's cost in cents per gram, over the grams with a price.
func (f Fill) PerGram() float64 { return perGram(f.Cents, f.Priced) }

// Month is what was paid in one calendar month, by when it was dispensed.
type Month struct {
	Month string
	Cents int64
	Grams float64
}

// Costs is what the prescriptions cost: by fill, by month, and per session
// that helped — rated 4 or more overall, at its product's price per gram.
type Costs struct {
	Fills        []Fill
	Months       []Month
	Cents        int64
	Priced       float64
	Unpriced     int
	Helped       int
	HelpedCents  int64
	HelpedNoCost int
}

// PerGram is everything paid over every gram with a price.
func (c Costs) PerGram() float64 { return perGram(c.Cents, c.Priced) }

// PerHelped is what a session that helped cost, on average.
func (c Costs) PerHelped() float64 {
	if c.Helped == 0 {
		return 0
	}
	return float64(c.HelpedCents) / float64(c.Helped)
}

func perGram(cents int64, grams float64) float64 {
	if grams <= 0 {
		return 0
	}
	return float64(cents) / grams
}

// helps is the overall rating from which a session counts as one that helped.
const helps = 4

// Analyse is what the purchases cost, by the price each was last given. An
// undone purchase cost nothing; a purchase without a price is counted, not
// guessed at.
func Analyse(state *ledger.State, latest map[string]Price, ratings []effects.Rating) Costs {
	var out Costs
	byMonth := map[string]*Month{}
	byProduct := map[string][2]float64{} // cents, priced grams
	standing := map[string]bool{}
	for _, e := range state.Standing {
		standing[e.Hash] = true
	}
	for _, c := range state.Cycles {
		f := Fill{Seq: c.Seq, Start: c.Start}
		for _, e := range c.Events {
			if e.Type != journal.Purchase || !standing[e.Hash] {
				continue
			}
			f.Dispensed = ledger.Round(f.Dispensed + e.Grams)
			p, ok := latest[e.Hash]
			if !ok || p.Unknown {
				f.Unpriced++
				continue
			}
			f.Priced = ledger.Round(f.Priced + e.Grams)
			f.Cents += p.Cents
			month := e.OccurredAt.Format("2006-01")
			m := byMonth[month]
			if m == nil {
				m = &Month{Month: month}
				byMonth[month] = m
			}
			m.Cents += p.Cents
			m.Grams = ledger.Round(m.Grams + e.Grams)
			pg := byProduct[e.Product]
			byProduct[e.Product] = [2]float64{pg[0] + float64(p.Cents), pg[1] + e.Grams}
		}
		out.Fills = append(out.Fills, f)
		out.Cents += f.Cents
		out.Priced = ledger.Round(out.Priced + f.Priced)
		out.Unpriced += f.Unpriced
	}
	for _, m := range byMonth {
		out.Months = append(out.Months, *m)
	}
	slices.SortFunc(out.Months, func(a, b Month) int { return strings.Compare(a.Month, b.Month) })
	helped(&out, state, byProduct, ratings)
	return out
}

// helped counts the sessions rated as having helped, and what their grams cost
// at their product's price per gram.
func helped(out *Costs, state *ledger.State, byProduct map[string][2]float64, ratings []effects.Rating) {
	rated := effects.Latest(ratings)
	var cents float64
	for _, e := range state.Standing {
		r, ok := rated[e.Hash]
		if e.Type != journal.Sesh || !ok || r.Scores[effects.Overall] < helps {
			continue
		}
		pg := byProduct[e.Product]
		if pg[1] <= 0 {
			out.HelpedNoCost++
			continue
		}
		out.Helped++
		cents += e.Grams * pg[0] / pg[1]
	}
	out.HelpedCents = int64(math.Round(cents))
}

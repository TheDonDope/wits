// Package prices keeps what purchases cost; this file breaks the prices down:
// every priced purchase, and the purchases grouped by strain, by producer,
// by pharmacy and by year, each group with its price a gram from first to
// last.
package prices

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// Product is what the breakdowns need to know of a purchase's product.
type Product struct {
	Name     string
	Maker    string
	Cultivar string
	THC, CBD float64
}

// Bought is one purchase with its price: what every breakdown is made of.
type Bought struct {
	Hash       string
	At         time.Time
	Fill       int // the fill's position, from 0
	Product    string
	Name       string
	Maker      string
	Strain     string // the cultivar, as StrainOf keys it
	StrainName string
	THC, CBD   float64
	Grams      float64
	Cents      int64
	Seller     string
}

// PerGram is the purchase's price a gram, in cents.
func (b Bought) PerGram() float64 { return perGram(b.Cents, b.Grams) }

// Group is purchases taken together — a strain's, a producer's, a
// pharmacy's, a year's — with what they cost, and the price a gram of the
// cheapest, the dearest, the first and the last of them.
type Group struct {
	Key, Name             string
	Products              []string
	Purchases             int
	Grams                 float64
	Cents                 int64
	Min, Max, First, Last float64 // cents a gram
	FirstAt, LastAt       time.Time
}

// PerGram is the group's price a gram, over all its grams.
func (g Group) PerGram() float64 { return perGram(g.Cents, g.Grams) }

// Insights is the purchases with a price, oldest first, and their groups.
type Insights struct {
	Bought  []Bought
	Strains []Group
	Makers  []Group
	Sellers []Group
	Years   []Group
}

// Insight breaks down the purchases that have a price, by the price each was
// last given. What a product is comes from product; an undone purchase, or
// one with no price to be had, is in none of it.
func Insight(state *ledger.State, latest map[string]Price, product func(slug string) Product) Insights {
	standing := map[string]bool{}
	for _, e := range state.Standing {
		standing[e.Hash] = true
	}
	type found struct {
		e    journal.Event
		fill int
		p    Price
	}
	var priced []found
	var known []Product
	for _, c := range state.Cycles {
		for _, e := range c.Events {
			p, ok := latest[e.Hash]
			if e.Type != journal.Purchase || !standing[e.Hash] || !ok || p.Unknown {
				continue
			}
			priced = append(priced, found{e, c.Seq, p})
			known = append(known, product(e.Product))
		}
	}
	producers := Producers(known)
	var out Insights
	for i, f := range priced {
		out.Bought = append(out.Bought, bought(f.e, f.fill, f.p, known[i], producers))
	}
	slices.SortStableFunc(out.Bought, func(a, b Bought) int { return a.At.Compare(b.At) })
	out.Strains = groups(out.Bought, func(b Bought) (string, string) { return b.Strain, b.StrainName })
	out.Makers = groups(out.Bought, func(b Bought) (string, string) { return strings.ToLower(b.Maker), b.Maker })
	out.Sellers = groups(out.Bought, func(b Bought) (string, string) { return b.Seller, b.Seller })
	out.Years = groups(out.Bought, func(b Bought) (string, string) { y := b.At.Format("2006"); return y, y })
	slices.SortFunc(out.Years, func(a, b Group) int { return strings.Compare(a.Key, b.Key) })
	return out
}

func bought(e journal.Event, fill int, p Price, about Product, producers map[string]bool) Bought {
	key, name := StrainOf(about, producers)
	return Bought{Hash: e.Hash, At: e.OccurredAt, Fill: fill, Product: e.Product, Name: about.Name,
		Maker: about.Maker, Strain: key, StrainName: name, THC: about.THC, CBD: about.CBD,
		Grams: e.Grams, Cents: p.Cents, Seller: p.Seller}
}

// groups gathers purchases (oldest first) by a key, most spent on first.
func groups(all []Bought, by func(Bought) (key, name string)) []Group {
	byKey := map[string]*Group{}
	var order []string
	for _, b := range all {
		key, name := by(b)
		g := byKey[key]
		if g == nil {
			g = &Group{Key: key, Name: name, FirstAt: b.At, First: b.PerGram(), Min: b.PerGram()}
			byKey[key] = g
			order = append(order, key)
		}
		add(g, b)
	}
	out := make([]Group, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	slices.SortStableFunc(out, func(a, b Group) int { return cmp.Compare(b.Cents, a.Cents) })
	return out
}

func add(g *Group, b Bought) {
	pg := b.PerGram()
	g.Purchases++
	g.Grams = ledger.Round(g.Grams + b.Grams)
	g.Cents += b.Cents
	g.Min, g.Max = min(g.Min, pg), max(g.Max, pg)
	g.Last, g.LastAt = pg, b.At
	if !slices.Contains(g.Products, b.Name) {
		g.Products = append(g.Products, b.Name)
	}
}

var (
	// A pharmacy's short code in a cultivar's name: "CA", "BUL", "DNK", "T01".
	code   = regexp.MustCompile(`^[\p{Lu}\d]{1,3}$`)
	digits = regexp.MustCompile(`^\d+$`)
	// A strength written into a name, THC/CBD: "25/1", "25:01".
	ratio = regexp.MustCompile(`\d+\s*[/:]\s*\d+`)
	// What a producer's name is padded with, which names no producer.
	padding = map[string]bool{"quantity": true, "pharma": true, "gmbh": true}
)

// Producers is the words producers are named by, from the products that say
// both producer and cultivar: what a product without a cultivar is stripped
// of, so its name says only its strain.
func Producers(products []Product) map[string]bool {
	out := map[string]bool{}
	for _, p := range products {
		if p.Cultivar == "" {
			continue
		}
		for _, w := range strings.Fields(strings.ToLower(p.Maker)) {
			out[w] = true
		}
	}
	return out
}

// StrainOf is the cultivar a product is of, keyed so that the same strain
// keys alike under the names it is sold by — "CA MAC: MAC1", "MAC 1" and
// "MAC1+"; "T01 Hindu Kush" and "Hindu Kush"; "Lemon Cookie" and "Lemon
// Cookies" — and named for showing. MAC 1 and MAC 3 stay apart: a number
// after a short name is part of it. The short codes pharmacies put in a name
// are left out, unless the name is codes alone ("CA GG"), and so are the
// producer's words. A product with no cultivar is keyed by its name, less
// the words of every producer known (producers).
func StrainOf(p Product, producers map[string]bool) (key, name string) {
	source, maker := p.Cultivar, map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(p.Maker)) {
		maker[w] = true
	}
	if source == "" {
		source, maker = p.Name, producers
	}
	tokens := strainTokens(source)
	long := slices.ContainsFunc(tokens, func(t string) bool { return !code.MatchString(t) && len([]rune(t)) >= 4 })
	var kept, words []string
	for _, t := range tokens {
		w := strings.ToLower(t)
		if (long && code.MatchString(t)) || digits.MatchString(t) || maker[w] || padding[w] || len([]rune(w)) < 2 {
			continue
		}
		kept = append(kept, t)
		if len([]rune(w)) > 4 {
			w = strings.TrimSuffix(w, "s")
		}
		words = append(words, w)
	}
	slices.Sort(words)
	words = slices.Compact(words)
	name = strings.Join(kept, " ")
	if name == "" {
		name = p.Name
	}
	return strings.Join(words, " "), name
}

// strainTokens splits a name into its words, a number joined to the short
// name before it ("MAC 1" is MAC1) and a plus dropped ("MAC1+" is MAC1).
func strainTokens(s string) []string {
	var out []string
	s = ratio.ReplaceAllString(s, " ")
	for _, t := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if n := len(out); n > 0 && digits.MatchString(t) && code.MatchString(out[n-1]) && !digits.MatchString(out[n-1]) {
			out[n-1] += t
			continue
		}
		out = append(out, t)
	}
	return out
}

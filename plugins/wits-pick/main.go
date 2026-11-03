// Package main is wits-pick, a plugin run as `wits pick`: tonight's jar,
// chosen from what is on the shelf by how the sessions you rated went —
// the jar's own, and those of every strain sharing its terpenes.
package main

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/TheDonDope/wits/plugins/internal/wits"
)

func main() {
	dim := flag.String("for", "overall", "the score to pick by: overall, pain, sleep, calm, mood, focus or appetite")
	top := flag.Int("top", 3, "how many jars to name")
	least := flag.Int("min", 3, "the rated sessions a terpene needs before it counts")
	flag.Parse()
	var sessions []wits.Session
	var products []wits.Product
	if err := wits.Get(&sessions, "sessions"); err != nil {
		wits.Fail(err)
	}
	if err := wits.Get(&products, "products", "--all"); err != nil {
		wits.Fail(err)
	}
	render(os.Stdout, rank(sessions, products, *dim, *least), *dim, *top)
}

// tally is a sum of scores and how many there were.
type tally struct {
	sum float64
	n   int
}

func (t *tally) add(v float64) { t.sum, t.n = t.sum+v, t.n+1 }
func (t tally) mean() float64  { return t.sum / float64(t.n) }

// pick is one jar on the shelf and why it stands where it does.
type pick struct {
	product wits.Product
	score   float64
	// own is the jar's own sessions, against the mean; matched its
	// terpenes', each against the mean.
	own     tally
	matched []terpene
}

// terpene is how the sessions of strains carrying it went, against the mean.
type terpene struct {
	name string
	lift float64
}

// ranking is the picks, best first, and the mean they are measured from.
type ranking struct {
	picks []pick
	mean  float64
	rated int
}

// rank scores every jar holding something: how its own rated sessions
// went, above or below the mean, plus how the sessions of strains sharing
// its terpenes went, the same way.
func rank(sessions []wits.Session, products []wits.Product, dim string, least int) ranking {
	terps := map[string][]string{}
	for _, p := range products {
		terps[p.Slug] = p.Terpenes
	}
	var all tally
	own, byTerp := map[string]*tally{}, map[string]*tally{}
	for _, s := range sessions {
		v, ok := s.Scores[dim]
		if !ok {
			continue
		}
		all.add(float64(v))
		at(own, s.Product).add(float64(v))
		for _, t := range terps[s.Product] {
			at(byTerp, t).add(float64(v))
		}
	}
	r := ranking{rated: all.n}
	if all.n == 0 {
		return r
	}
	r.mean = all.mean()
	for _, p := range products {
		if p.Storage+p.Stash <= 0 {
			continue
		}
		if pk, ok := score(p, own[p.Slug], byTerp, r.mean, least); ok {
			r.picks = append(r.picks, pk)
		}
	}
	slices.SortStableFunc(r.picks, func(a, b pick) int { return cmp.Compare(b.score, a.score) })
	return r
}

// at is the tally under a key, made the first time it is asked for.
func at(m map[string]*tally, k string) *tally {
	if m[k] == nil {
		m[k] = &tally{}
	}
	return m[k]
}

// score is one jar's pick, when there is anything to go on: its own
// sessions, or terpenes rated often enough.
func score(p wits.Product, own *tally, byTerp map[string]*tally, mean float64, least int) (pick, bool) {
	pk := pick{product: p}
	var lift float64
	for _, t := range p.Terpenes {
		if tt := byTerp[t]; tt != nil && tt.n >= least {
			pk.matched = append(pk.matched, terpene{name: t, lift: tt.mean() - mean})
			lift += tt.mean() - mean
		}
	}
	if len(pk.matched) > 0 {
		pk.score = lift / float64(len(pk.matched))
	}
	if own != nil {
		pk.own = *own
		pk.score += own.mean() - mean
	}
	return pk, pk.own.n > 0 || len(pk.matched) > 0
}

// render names the best jars and why.
func render(out io.Writer, r ranking, dim string, top int) {
	if r.rated == 0 {
		fmt.Fprintf(out, "No session is rated for %s yet: `wits rate session %s=4` after the next one.\n", dim, dim)
		return
	}
	if len(r.picks) == 0 {
		fmt.Fprintln(out, "Nothing on the shelf has been rated, or shares terpenes with what has.")
		return
	}
	fmt.Fprintf(out, "Tonight, for %s (%d rated sessions, %.1f of 5 on average):\n", dim, r.rated, r.mean)
	for i, pk := range r.picks[:min(top, len(r.picks))] {
		p := pk.product
		fmt.Fprintf(out, "\n%d. %s (%s): %+.2f\n", i+1, p.Name, p.Slug, pk.score)
		fmt.Fprintf(out, "   %.2f g ground, %.2f g sealed\n", p.Stash, p.Storage)
		for _, why := range reasons(pk) {
			fmt.Fprintf(out, "   %s\n", why)
		}
	}
}

// reasons says what a pick stands on.
func reasons(pk pick) []string {
	var out []string
	if pk.own.n > 0 {
		out = append(out, fmt.Sprintf("you rated it %.1f over %d sessions", pk.own.mean(), pk.own.n))
	}
	if len(pk.matched) > 0 {
		var ts []string
		for _, t := range pk.matched {
			ts = append(ts, fmt.Sprintf("%s %+.2f", t.name, t.lift))
		}
		out = append(out, "its terpenes: "+strings.Join(ts, ", "))
	}
	return out
}

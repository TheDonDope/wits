// Package report holds the terpenes on the shelf here: each with what it
// tastes of, how many products name it and lead with it, and what the
// sessions of those products were rated.
package report

import (
	"cmp"
	"slices"

	can "github.com/TheDonDope/wits/pkg/cannabis"
)

// Terpene is one terpene the shelf's products name: what it is and tastes
// of, when wits knows it, and what was had of it.
type Terpene struct {
	Name         string
	Flavors      []string
	Family       can.Family
	Notes        string
	Effects      []string
	BoilingPoint int // 0 when wits has none for it
	// Products name it, and Leads of them name it first.
	Products, Leads int
	// Overall is the mean overall rating of those products' sessions, each
	// product counted by its rated sessions and by how high in its profile
	// the terpene stands — a trace counts less than the lead — over Rated
	// sessions in all.
	Overall float64
	Rated   int
}

// Terpenes is every terpene the strains name, the most named first, with
// what their sessions were rated.
func Terpenes(strains []Strain) []Terpene {
	by := map[string]*Terpene{}
	sums, weights := map[string]float64{}, map[string]float64{}
	for _, s := range strains {
		for i, name := range s.Terpenes {
			k := can.Key(name)
			t := by[k]
			if t == nil {
				t = terpeneOf(name)
				by[k] = t
			}
			t.Products++
			if i == 0 {
				t.Leads++
			}
			if s.Ratings > 0 {
				w := float64(s.Ratings) / float64(i+1)
				sums[k] += w * s.Overall
				weights[k] += w
				t.Rated += s.Ratings
			}
		}
	}
	out := make([]Terpene, 0, len(by))
	for k, t := range by {
		if weights[k] > 0 {
			t.Overall = sums[k] / weights[k]
		}
		out = append(out, *t)
	}
	slices.SortFunc(out, func(a, b Terpene) int {
		if c := cmp.Compare(b.Products, a.Products); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

// terpeneOf is a terpene as the shelf names it, with what wits knows of it.
func terpeneOf(name string) *Terpene {
	t := &Terpene{Name: name, Flavors: []string{}, Effects: []string{}}
	known := can.Lookup(name)
	if known == nil {
		return t
	}
	t.Flavors = append(t.Flavors, known.Flavors...)
	t.Effects = append(t.Effects, known.Effects...)
	t.Notes, t.BoilingPoint = known.Notes, known.BoilingPoint
	for _, f := range known.Flavors {
		if fam := can.FamilyOf(f); fam != "" {
			t.Family = fam
			break
		}
	}
	return t
}

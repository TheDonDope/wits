// Package report holds the map of the strains here: every product placed by
// how alike their names, cannabinoids and terpenes are, and rated by the
// effect log.
package report

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Strain is one product on the map of the strains: where its profile puts it,
// what the effect log says it did, and the products most like it.
type Strain struct {
	Slug, Name string
	X, Y       float64
	THC, CBD   float64
	Terpenes   []string
	Dispensed  float64
	// Overall is the mean overall rating of its sessions, over Ratings of
	// them; with none, there is no colour to give it.
	Overall float64
	Ratings int
	Like    []string
}

// Strains places every product dispensed by its profile: across by its THC,
// which can be read off the map, and up and down by the rest of it — the
// words of the cultivar's name ("Wedding Cake", "MAC1", "Lemon") and the
// terpenes where both have them, dominant first — laid on one line by
// classical multidimensional scaling, so alike sits at one height. Two axes
// of scaling alone were tried first: with no terpenes written down most
// products are as unlike as each other, and the map crowded them in a corner
// and flung the rest across it. What each is most like is the whole profile,
// the cannabinoids included.
func Strains(ws *workspace.Workspace) []Strain {
	dispensed := map[string]float64{}
	for _, e := range ws.State.Standing {
		if e.Type == journal.Purchase {
			dispensed[e.Product] = ledger.Round(dispensed[e.Product] + e.Grams)
		}
	}
	var out []Strain
	for _, p := range ws.Products.Products {
		if dispensed[p.Slug] > 0 {
			out = append(out, Strain{Slug: p.Slug, Name: p.Name, THC: p.THC, CBD: p.CBD, Terpenes: p.Terpenes,
				Dispensed: dispensed[p.Slug]})
		}
	}
	slices.SortFunc(out, func(a, b Strain) int { return strings.Compare(a.Slug, b.Slug) })
	words := make([]map[string]bool, len(out))
	for i, s := range out {
		p, _ := ws.Products.Find(s.Slug)
		words[i] = cultivarWords(p.Cultivar, p.Name)
	}
	d := make([][]float64, len(out))
	name := make([][]float64, len(out))
	for i := range out {
		d[i], name[i] = make([]float64, len(out)), make([]float64, len(out))
		for j := range out {
			d[i][j] = distance(out[i], out[j], words[i], words[j])
			name[i][j] = distance(Strain{Slug: out[i].Slug, Terpenes: out[i].Terpenes},
				Strain{Slug: out[j].Slug, Terpenes: out[j].Terpenes}, words[i], words[j])
		}
	}
	ys := scale(name)
	for i := range out {
		out[i].X, out[i].Y = out[i].THC, ys[i]
		out[i].Like = nearest(out, d[i], i, 3)
	}
	rate(out, ws)
	return out
}

// cultivarWords is what a cultivar is called, word by word: lowercase,
// without its digits — "MAC1+" and "MAC 1" are one family — and without the
// codes of two letters or fewer a label puts before the name.
func cultivarWords(cultivar, name string) map[string]bool {
	if cultivar == "" {
		cultivar = name
	}
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(cultivar), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len([]rune(w)) > 2 {
			out[w] = true
		}
	}
	return out
}

// distance is how unlike two profiles are, 0 for the same: the name's words
// and the terpenes weigh most, then the cannabinoids.
func distance(a, b Strain, wa, wb map[string]bool) float64 {
	if a.Slug == b.Slug {
		return 0
	}
	sq := func(x float64) float64 { return x * x }
	d := 1.0 * sq(1-jaccard(wa, wb))
	d += 0.6 * sq((a.THC-b.THC)/20)
	d += 0.6 * sq((a.CBD-b.CBD)/20)
	// Not knowing is halfway, not alike: otherwise two products whose
	// terpenes were never written down would be nearer than two whose
	// written profiles match.
	like := 0.5
	if len(a.Terpenes) > 0 && len(b.Terpenes) > 0 {
		like = terpeneLikeness(a.Terpenes, b.Terpenes)
	}
	d += 1.2 * sq(1-like)
	return math.Sqrt(d)
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	both := 0
	for w := range a {
		if b[w] {
			both++
		}
	}
	return float64(both) / float64(len(a)+len(b)-both)
}

// terpeneLikeness is how alike two terpene lists are, the dominant ones
// weighing most: each terpene weighs 1/(its place), and the likeness is the
// weighted Jaccard — the weight they share over the weight of either.
func terpeneLikeness(a, b []string) float64 {
	weights := func(ts []string) map[string]float64 {
		w := map[string]float64{}
		for i, t := range ts {
			k := catalog.TerpeneKey(t)
			w[k] = math.Max(w[k], 1/float64(i+1))
		}
		return w
	}
	wa, wb := weights(a), weights(b)
	var shared, either float64
	for k, x := range wa {
		shared += math.Min(x, wb[k])
		either += math.Max(x, wb[k])
	}
	for k, y := range wb {
		if _, ok := wa[k]; !ok {
			either += y
		}
	}
	if either == 0 {
		return 0
	}
	return shared / either
}

// scale lays a distance matrix on a line: classical multidimensional
// scaling, the largest eigenvector of the double-centred squared distances,
// found by power iteration — fifty products is nothing to it.
func scale(d [][]float64) []float64 {
	n := len(d)
	b := make([][]float64, n)
	rows, all := make([]float64, n), 0.0
	for i := range d {
		for j := range d[i] {
			rows[i] += d[i][j] * d[i][j] / float64(n)
		}
		all += rows[i] / float64(n)
	}
	for i := range d {
		b[i] = make([]float64, n)
		for j := range d[i] {
			b[i][j] = -0.5 * (d[i][j]*d[i][j] - rows[i] - rows[j] + all)
		}
	}
	first, l1 := eigen(b)
	out := make([]float64, n)
	for i := range n {
		out[i] = first[i] * math.Sqrt(math.Max(l1, 0))
	}
	return out
}

// eigen is the largest eigenvector of a symmetric matrix, and its
// eigenvalue, by power iteration.
func eigen(m [][]float64) ([]float64, float64) {
	n := len(m)
	v := make([]float64, n)
	for i := range v {
		v[i] = 1 + float64(i%7)/7 // any start that is not orthogonal to the answer
	}
	var lambda float64
	for range 300 {
		w := make([]float64, n)
		for i := range m {
			for j := range m[i] {
				w[i] += m[i][j] * v[j]
			}
		}
		norm := 0.0
		for _, x := range w {
			norm += x * x
		}
		if norm = math.Sqrt(norm); norm == 0 {
			return v, 0
		}
		for i := range w {
			w[i] /= norm
		}
		v, lambda = w, norm
	}
	return v, lambda
}

// nearest is the k products nearest the i-th, by slug.
func nearest(all []Strain, d []float64, i, k int) []string {
	order := make([]int, 0, len(all))
	for j := range all {
		if j != i {
			order = append(order, j)
		}
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(d[a], d[b]) })
	var out []string
	for _, j := range order[:min(k, len(order))] {
		out = append(out, all[j].Slug)
	}
	return out
}

// rate gives each product the mean overall rating of its sessions.
func rate(out []Strain, ws *workspace.Workspace) {
	insights := effects.Analyse(ws.Ratings, ws.State.Events, ws.ProductName)
	by := map[string]effects.Group{}
	for _, g := range insights.ByProduct {
		by[g.Key] = g
	}
	for i := range out {
		if g, ok := by[out[i].Slug]; ok && g.Counts[effects.Overall] > 0 {
			out[i].Overall, out[i].Ratings = g.Means[effects.Overall], g.Counts[effects.Overall]
		}
	}
}

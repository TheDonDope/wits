// Package report holds the map of the strains here: every product placed by
// its terpene profile and how it tastes, what each is most like and why, and
// what the effect log says it did.
package report

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"unicode"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Strain is one product on the map of the strains: where its terpenes put
// it, how it tastes, what the effect log says it did, and the products most
// like it.
type Strain struct {
	Slug, Name string
	// X and Y place it among the others by its terpene profile; Placed is
	// false for a product with none written down, which has no place.
	X, Y     float64
	Placed   bool
	THC, CBD float64
	Terpenes []string
	// Aroma is its taste family by family, the strongest 1; Lead the
	// strongest; Taste its three strongest flavours in words.
	Aroma     map[can.Family]float64
	Lead      can.Family
	Taste     string
	Dispensed float64
	// Overall is the mean overall rating of its sessions, over Ratings of
	// them; with none, there is no colour to give it.
	Overall float64
	Ratings int
	Like    []Likeness
}

// Likeness is another product, how alike the two are, 0 to 1, and the
// terpenes they share, in this one's order.
type Likeness struct {
	Slug   string
	Score  float64
	Shared []string
}

// likeMost is how many of the most alike each product keeps.
const likeMost = 5

// Strains places every product dispensed by its terpene profile, and finds
// what each is most like.
//
// Alike is mostly the profile: which terpenes, the dominant ones counting
// most and the rare ones more than those nearly every jar has — Limonene is
// in most, Terpinolene in few, and two that share it share something — and
// how the two taste, family by family, so a citrus-led profile is nearer
// another citrus-led one whatever terpene makes it so. The cultivar's name
// counts a little, the cannabinoids hardly: at 20 to 30 % THC they say almost
// nothing of what a strain is like. A product with no terpenes written down
// is compared by name and cannabinoids alone, and counts as half as alike at
// best, so a known profile is always preferred.
//
// The map lays the profiles out in two dimensions by classical
// multidimensional scaling of how unalike they are: alike sits together.
func Strains(ws *workspace.Workspace) []Strain {
	out := dispensedStrains(ws)
	words := make([]map[string]bool, len(out))
	for i, s := range out {
		p := ws.Products.Exact(s.Slug)
		words[i] = cultivarWords(p.Cultivar, p.Name)
	}
	vectors := terpeneVectors(out)
	sim := make([][]float64, len(out))
	for i := range out {
		sim[i] = make([]float64, len(out))
		for j := range out {
			sim[i][j] = similarity(out[i], out[j], vectors[i], vectors[j], words[i], words[j])
		}
	}
	for i := range out {
		out[i].Like = likes(out, sim[i], i)
	}
	place(out, vectors)
	rate(out, ws)
	return out
}

// dispensedStrains is every product dispensed, by slug, with its grams and
// its taste.
func dispensedStrains(ws *workspace.Workspace) []Strain {
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
				Dispensed: dispensed[p.Slug], Aroma: can.Aroma(p.Terpenes), Lead: can.Lead(p.Terpenes),
				Taste: can.Taste(p.Terpenes)})
		}
	}
	slices.SortFunc(out, func(a, b Strain) int { return strings.Compare(a.Slug, b.Slug) })
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

// terpeneVectors is each product's terpenes as weights, by key: the dominant
// one most, each after it less, and each times how rare it is among the
// profiles — the more of them name a terpene, the less naming it tells.
func terpeneVectors(all []Strain) []map[string]float64 {
	df := map[string]int{}
	profiles := 0
	for _, s := range all {
		if len(s.Terpenes) > 0 {
			profiles++
		}
		for k := range keys(s.Terpenes) {
			df[k]++
		}
	}
	out := make([]map[string]float64, len(all))
	for i, s := range all {
		out[i] = map[string]float64{}
		for j, t := range s.Terpenes {
			k := can.Key(t)
			if _, seen := out[i][k]; seen {
				continue
			}
			idf := math.Log(float64(profiles+1)/float64(df[k]+1)) + 1
			out[i][k] = idf / math.Sqrt(float64(j+1))
		}
	}
	return out
}

// keys is the set of a profile's terpene keys.
func keys(terpenes []string) map[string]bool {
	out := map[string]bool{}
	for _, t := range terpenes {
		out[can.Key(t)] = true
	}
	return out
}

// similarity is how alike two products are, 0 to 1: see Strains.
func similarity(a, b Strain, va, vb map[string]float64, wa, wb map[string]bool) float64 {
	if a.Slug == b.Slug {
		return 1
	}
	cannabinoids := 1 - math.Min(1, (math.Abs(a.THC-b.THC)+math.Abs(a.CBD-b.CBD))/15)
	name := jaccard(wa, wb)
	if len(va) == 0 || len(vb) == 0 {
		return 0.5 * (0.8*name + 0.2*cannabinoids)
	}
	return 0.85*profileSimilarity(a, b, va, vb) + 0.1*name + 0.05*cannabinoids
}

// profileSimilarity is how alike two terpene profiles are: mostly the
// terpenes themselves, partly how they taste.
func profileSimilarity(a, b Strain, va, vb map[string]float64) float64 {
	return 0.7*cosine(va, vb) + 0.3*cosine(families(a.Aroma), families(b.Aroma))
}

// families is an aroma by family name, to compare as a vector.
func families(a map[can.Family]float64) map[string]float64 {
	out := make(map[string]float64, len(a))
	for f, v := range a {
		out[string(f)] = v
	}
	return out
}

// cosine is how alike two weightings are, 0 to 1.
func cosine(a, b map[string]float64) float64 {
	var dot, na, nb float64
	for k, x := range a {
		dot += x * b[k]
		na += x * x
	}
	for _, y := range b {
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
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

// likes is the products most like the i-th, most alike first, each with the
// terpenes the two share.
func likes(all []Strain, sim []float64, i int) []Likeness {
	order := make([]int, 0, len(all))
	for j := range all {
		if j != i {
			order = append(order, j)
		}
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(sim[b], sim[a]) })
	out := []Likeness{}
	for _, j := range order[:min(likeMost, len(order))] {
		out = append(out, Likeness{Slug: all[j].Slug, Score: math.Round(sim[j]*100) / 100,
			Shared: shared(all[i].Terpenes, all[j].Terpenes)})
	}
	return out
}

// shared is the terpenes of a that b names too, in a's order.
func shared(a, b []string) []string {
	in := keys(b)
	out := []string{}
	for _, t := range a {
		if in[can.Key(t)] {
			out = append(out, t)
		}
	}
	return out
}

// place lays the products with a profile out in two dimensions, by how
// unalike their profiles are; those without one are not placed.
func place(all []Strain, vectors []map[string]float64) {
	var idx []int
	for i := range all {
		if len(vectors[i]) > 0 {
			idx = append(idx, i)
		}
	}
	if len(idx) < 3 {
		return
	}
	d := make([][]float64, len(idx))
	for a, i := range idx {
		d[a] = make([]float64, len(idx))
		for b, j := range idx {
			if a != b {
				d[a][b] = 1 - profileSimilarity(all[i], all[j], vectors[i], vectors[j])
			}
		}
	}
	xs, ys := scale(d)
	for a, i := range idx {
		all[i].X, all[i].Y, all[i].Placed = xs[a], ys[a], true
	}
}

// scale lays a distance matrix in a plane: classical multidimensional
// scaling, the two largest eigenvectors of the double-centred squared
// distances, found by power iteration — fifty products is nothing to it.
func scale(d [][]float64) ([]float64, []float64) {
	b := centred(d)
	first, l1 := eigen(b)
	for i := range b {
		for j := range b[i] {
			b[i][j] -= l1 * first[i] * first[j]
		}
	}
	// The second axis is the next largest eigenvalue, not the next largest
	// in size: shifted by the first, every eigenvalue is at least 0, and the
	// one power iteration finds is the largest.
	for i := range b {
		b[i][i] += l1
	}
	second, l2 := eigen(b)
	l2 -= l1
	xs, ys := make([]float64, len(d)), make([]float64, len(d))
	for i := range d {
		xs[i] = first[i] * math.Sqrt(math.Max(l1, 0))
		ys[i] = second[i] * math.Sqrt(math.Max(l2, 0))
	}
	return xs, ys
}

// centred is the double-centred matrix of squared distances, halved and
// negated: what classical scaling takes the eigenvectors of.
func centred(d [][]float64) [][]float64 {
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
	return b
}

// eigen is the eigenvector of a symmetric matrix with the eigenvalue largest
// in size, and that eigenvalue, sign and all, by power iteration.
func eigen(m [][]float64) ([]float64, float64) {
	n := len(m)
	v := make([]float64, n)
	for i := range v {
		v[i] = 1 + float64(i%7)/7 // any start that is not orthogonal to the answer
	}
	lambda := 0.0
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
		v = w
	}
	// The Rayleigh quotient: the eigenvalue with its sign, which the norm drops.
	for i := range m {
		for j := range m[i] {
			lambda += v[i] * m[i][j] * v[j]
		}
	}
	return v, lambda
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

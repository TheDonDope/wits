// Package cannabis holds what terpenes taste of here: the key two labels
// agree on for one terpene, the minor terpenes a lookup finds that the table
// of boiling points does not hold, the eight families their flavours fall
// into, and a profile's taste put in words.
package cannabis

import (
	"slices"
	"sort"
	"strings"
	"unicode"
)

// Minor are terpenes a profile names that wits has no boiling point for:
// found by the web lookups, and tasted, but not on the vaporizer's scale.
var Minor = []*Terpene{
	{Name: "Ocimene", Flavors: []string{"sweet", "herbal", "wood", "citrus"}, Notes: "Also in mint, basil, parsley and orchids"},
	{Name: "Farnesene", Flavors: []string{"green apple", "wood", "sweet"}, Notes: "The scent of green apple skin; also in chamomile and turmeric"},
	{Name: "Valencene", Flavors: []string{"orange", "citrus", "sweet", "wood"}, Notes: "Named for the Valencia orange; also in grapefruit"},
	{Name: "Guaiol", Flavors: []string{"pine", "rose", "wood"}, Notes: "An alcohol of guaiacum wood and cypress pine"},
	{Name: "Fenchol", Flavors: []string{"camphor", "pine", "lemon"}, Notes: "Also in basil and fennel"},
	{Name: "Terpinene", Flavors: []string{"wood", "citrus", "herbal"}, Notes: "Also in tea tree, cumin and cardamom"},
	{Name: "Bergamotene", Flavors: []string{"bergamot", "wood", "tea"}, Notes: "Also in bergamot, carrot and lime"},
	{Name: "Geranyl acetate", Flavors: []string{"rose", "fruit", "sweet"}, Notes: "Also in citronella and lemongrass"},
}

// prefixes are the words a terpene's name carries that do not name it:
// which isomer, and how it is spelled out.
var prefixes = map[string]bool{"alpha": true, "beta": true, "gamma": true, "delta": true, "trans": true, "cis": true,
	"para": true, "α": true, "β": true, "γ": true, "δ": true}

// Key is a terpene's name as two labels agree on it: its longest word, less
// isomer and spelling prefixes, lowercase, without the e German leaves off —
// "β-Myrcen", "myrcene" and "Trans-Caryophyllen" key as one, "Δ-3-Carene"
// as "Carene".
func Key(name string) string {
	var best string
	for _, w := range strings.FieldsFunc(strings.ToLower(strings.TrimSpace(name)), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if !prefixes[w] && len([]rune(w)) > 1 && len(w) > len(best) {
			best = w
		}
	}
	return strings.TrimSuffix(best, "e")
}

// keyed is every terpene wits can taste, by Key: the table of boiling points,
// then the minor ones.
var keyed = func() map[string]*Terpene {
	out := map[string]*Terpene{}
	for _, t := range Terpenes {
		out[Key(t.Name)] = t
	}
	for _, t := range Minor {
		out[Key(t.Name)] = t
	}
	return out
}()

// Lookup is the terpene a name means, or nil when wits does not know it.
func Lookup(name string) *Terpene { return keyed[Key(name)] }

// Family is what a flavour is like, broadly: what a taste wheel's spokes are.
type Family string

// The eight families, in the order a wheel goes round: from the bright to
// the deep.
const (
	Citrus Family = "citrus"
	Fruity Family = "fruity"
	Floral Family = "floral"
	Sweet  Family = "sweet"
	Herbal Family = "herbal"
	Woody  Family = "woody"
	Earthy Family = "earthy"
	Spicy  Family = "spicy"
)

// Families are the eight, in the order a wheel goes round.
var Families = []Family{Citrus, Fruity, Floral, Sweet, Herbal, Woody, Earthy, Spicy}

// families says which family each flavour word falls in.
var families = map[string]Family{
	"citrus": Citrus, "lemon": Citrus, "orange": Citrus, "lime": Citrus, "bergamot": Citrus,
	"apple": Fruity, "green apple": Fruity, "fruit": Fruity,
	"floral": Floral, "lavender": Floral, "rose": Floral, "chamomile": Floral,
	"sweet": Sweet, "balsamic": Sweet,
	"herbal": Herbal, "mint": Herbal, "camphor": Herbal, "cool": Herbal, "sage": Herbal, "rosemary": Herbal, "tea": Herbal, "grassy": Herbal,
	"pine": Woody, "wood": Woody, "fir": Woody, "cedar": Woody,
	"earth": Earthy, "earthy": Earthy, "musk": Earthy, "hops": Earthy, "damp": Earthy,
	"pepper": Spicy, "spicy": Spicy,
}

// FamilyOf is the family a flavour word falls in, or "" for one wits does
// not place.
func FamilyOf(flavor string) Family { return families[strings.ToLower(flavor)] }

// rank is how much the i-th terpene of a profile counts: the dominant one
// most, each after it less, as a label lists them.
func rank(i int) float64 { return 1 / float64(i+1) }

// Aroma is a profile's taste, family by family: each terpene's weight — the
// dominant one most — shared among its flavours, the first of them most, and
// scaled so the strongest family is 1. Terpenes wits cannot taste add nothing.
func Aroma(terpenes []string) map[Family]float64 {
	out := map[Family]float64{}
	for i, name := range terpenes {
		t := Lookup(name)
		if t == nil {
			continue
		}
		for j, f := range t.Flavors {
			if fam := FamilyOf(f); fam != "" {
				out[fam] += rank(i) * rank(j)
			}
		}
	}
	top := 0.0
	for _, v := range out {
		top = max(top, v)
	}
	for k := range out {
		out[k] /= top
	}
	return out
}

// Lead is a profile's strongest family, or "" when wits can taste none of it.
func Lead(terpenes []string) Family {
	a := Aroma(terpenes)
	best := Family("")
	for _, f := range Families {
		if a[f] > a[best] {
			best = f
		}
	}
	return best
}

// Taste puts a profile in words: its three strongest flavours, each from a
// family of its own — "Lemon, pepper and musk". Empty when wits can taste none.
func Taste(terpenes []string) string {
	weight := map[string]float64{}
	for i, name := range terpenes {
		if t := Lookup(name); t != nil {
			for j, f := range t.Flavors {
				if FamilyOf(f) != "" {
					weight[f] += rank(i) * rank(j)
				}
			}
		}
	}
	words := make([]string, 0, len(weight))
	for w := range weight {
		words = append(words, w)
	}
	sort.Slice(words, func(i, j int) bool {
		if weight[words[i]] != weight[words[j]] {
			return weight[words[i]] > weight[words[j]]
		}
		return words[i] < words[j]
	})
	var picked []string
	var seen []Family
	for _, w := range words {
		if f := FamilyOf(w); !slices.Contains(seen, f) && len(picked) < 3 {
			seen = append(seen, f)
			picked = append(picked, w)
		}
	}
	return sentence(picked)
}

// nouns are the flavour words that are adjectives, as a taste says them.
var nouns = map[string]string{"herbal": "herbs", "floral": "flowers", "spicy": "spice", "sweet": "sweetness",
	"earthy": "earth", "cool": "coolness", "grassy": "grass", "damp": "damp earth", "balsamic": "balsam"}

// sentence joins words as a list is spoken, adjectives as nouns, the first
// capitalised.
func sentence(words []string) string {
	for i, w := range words {
		if n, ok := nouns[w]; ok {
			words[i] = n
		}
	}
	switch len(words) {
	case 0:
		return ""
	case 1:
		return capital(words[0])
	}
	return capital(strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1])
}

func capital(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

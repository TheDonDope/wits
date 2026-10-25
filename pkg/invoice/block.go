// Package invoice reads a pharmacy's invoice; this file reads the layout
// that spreads a product over several rows: its name with its PZN, a row
// with its grams and prices, and a description that names its maker,
// strength and strain.
package invoice

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// "Cannamedical Hybrid Ultra DK - PZN: 18661848" opens a product.
	pzn = regexp.MustCompile(`^(.+?)\s+-\s+PZN:?\s*\d+\s*$`)
	// The row of grams and prices, the description's text before it or not:
	// "Cannamedical - THC: 28%, CBD: 1% - Blüte - Hybrid - 5g 12.50€ 62.50€".
	amounts = regexp.MustCompile(`^(.*?)\s*(\d+(?:[.,]\d+)?)\s*g\s+` + euro + `\s+` + euro + `\s*$`)
	thc     = regexp.MustCompile(`THC:?\s*(\d+)\s*%`)
	cbd     = regexp.MustCompile(`CBD:?\s*<?\s*(\d+)\s*%`)
	ratio   = regexp.MustCompile(`\d+[/:]\d+`)
	// The kinds a description names before the strain.
	kinds = map[string]bool{"blüte": true, "hybrid": true, "sativa": true, "indica": true}
)

// block is a product being read over its rows.
type block struct {
	head                  string
	desc                  []string
	grams, perGram, total string
}

// takes reports whether a line is this product's: its description, or its
// row of grams and prices while it has none.
func (b *block) takes(line string) bool {
	return !strings.Contains(line, "€") || (b.grams == "" && amounts.MatchString(line))
}

func (b *block) add(line string) {
	if m := amounts.FindStringSubmatch(line); m != nil && b.grams == "" {
		b.grams, b.perGram, b.total = m[2], m[3], m[4]
		line = m[1]
	}
	if line != "" {
		b.desc = append(b.desc, line)
	}
}

// close adds the product read so far, if it had its grams.
func (inv *Invoice) close(b *block) {
	if b != nil && b.grams != "" {
		inv.add(b.name(), b.grams, b.perGram, b.total)
	}
}

// name is the product as its rows give it: the name by its PZN, with the
// strength and the strain from the description where the name lacks them —
// "Pedanios 22/1" and "… - Blüte - Sativa - Ghost Train Haze" make
// "Pedanios 22/1 Ghost Train Haze".
func (b *block) name() string {
	desc := strings.Join(b.desc, " ")
	name := b.head
	if t := thc.FindStringSubmatch(desc); t != nil && !ratio.MatchString(name) {
		c := "1"
		if m := cbd.FindStringSubmatch(desc); m != nil {
			c = m[1]
		}
		name += fmt.Sprintf(" %s/%s", t[1], c)
	}
	if s := strain(desc); s != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(s)) {
		name += " " + s
	}
	return name
}

// strain is what a description names after the kind of flower it is.
func strain(desc string) string {
	parts := strings.Split(desc, " - ")
	for i := len(parts) - 1; i >= 0; i-- {
		if kinds[strings.ToLower(strings.TrimSpace(strings.TrimSuffix(parts[i], " -")))] {
			return strings.TrimSpace(strings.Join(parts[i+1:], " - "))
		}
	}
	return ""
}

// Package review holds the year's findings in words here: the lines `wits
// review` prints under its bars and the terminal interface's Year page draws,
// written once so the two cannot say a figure differently.
package review

import (
	"fmt"
	"math"
	"strings"

	"github.com/TheDonDope/wits/pkg/prices"
)

// Findings is the year's other figures as label and line, those it has: the
// strains it was most, the busiest day, the longest break, what was rated
// best, the device and terpene of the year, and what it cost.
func (r Review) Findings() [][2]string {
	var out [][2]string
	add := func(label, line string) { out = append(out, [2]string{label, line}) }
	for i, s := range r.Strains[:min(3, len(r.Strains))] {
		label := ""
		if i == 0 {
			label = "Strains"
		}
		add(label, fmt.Sprintf("%d  %s, %.2f g (%.0f %%)", i+1, s.Name, s.Ground, s.Share*100))
	}
	if r.Busiest.Ground > 0 {
		add("Busiest day", fmt.Sprintf("%s, %.2f g ground", r.Busiest.Day.Format("Mon 2 January"), r.Busiest.Ground))
	}
	if b := r.Break; b != nil {
		add("Longest break", fmt.Sprintf("%s, %s to %s (%s in all)", plural(b.Days, "day"),
			b.From.Format("2 January"), b.To.Format("2 January"), plural(b.Breaks, "break")))
	}
	if b := r.Best; b != nil {
		add("Best temperature", fmt.Sprintf("%s, %.1f overall over %s", b.Label, b.Mean, plural(b.Count, "rating")))
	}
	if l := r.Liked; l != nil {
		add("Rated best", fmt.Sprintf("%s, %.1f overall over %s", l.Name, l.Mean, plural(l.Count, "rating")))
	}
	if d := r.Device; d != nil {
		add("Most used", fmt.Sprintf("%s, %s and %.2f g", d.Name, plural(d.Sessions, "session"), d.Seshed))
	}
	if t := r.Terpene; t != nil {
		add("Terpene of the year", fmt.Sprintf("%s — %s, %.0f %% of the grams ground", t.Name,
			strings.Join(t.Flavors, ", "), t.Share*100))
	}
	return append(out, r.costLines()...)
}

// costLines is what the year cost, when any of it had a price.
func (r Review) costLines() [][2]string {
	if r.Priced <= 0 {
		return nil
	}
	euro := func(c float64) string { return prices.Format(int64(math.Round(c))) }
	out := [][2]string{{"Paid", fmt.Sprintf("%s for %.2f g with a price, %s a gram", prices.Format(r.Paid), r.Priced, euro(r.PerGram()))}}
	if d := r.Dearest; d != nil {
		out = append(out, [2]string{"Dearest gram", fmt.Sprintf("%s, %s on %s", euro(d.PerGram), d.Name, d.At.Format("2 January"))})
	}
	if c := r.Cheapest; c != nil {
		out = append(out, [2]string{"Cheapest gram", fmt.Sprintf("%s, %s on %s", euro(c.PerGram), c.Name, c.At.Format("2 January"))})
	}
	return out
}

// plural is a count with its noun, the noun taking an s past one.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

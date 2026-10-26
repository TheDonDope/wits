// Package commands holds `wits review` here: the year on one card.
package commands

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/review"
	"github.com/spf13/cobra"
)

// Review is the `wits review` command.
var Review = &cobra.Command{
	Use:   "review [year]",
	Short: "The year on one card: the grams, the strains, the longest break, the best temperature",
	Long: "The year in wits: what was ground and on how many days, month by month,\n" +
		"the strains, the longest break, the temperature rated best, the strain\n" +
		"rated best, the device most used, the terpene most of the grams led with,\n" +
		"and what it cost — the dearest gram and the cheapest. Every figure is the\n" +
		"year's own, to today in the year still running. This year unless a year\n" +
		"is given; the browser's Insights → Year draws the same.",
	Example: "  wits review\n  wits review 2025",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		years := review.Years(s.Workspace)
		if len(years) == 0 {
			return errors.New("the journal is empty: there is no year to look back on")
		}
		year := s.OpenedAt.Year()
		if len(args) == 1 {
			if year, err = strconv.Atoi(args[0]); err != nil {
				return fmt.Errorf("%q is not a year", args[0])
			}
		}
		r := review.Year(s.Workspace, year)
		if r.Ground == 0 && r.Sessions == 0 && r.Dispensed == 0 {
			return fmt.Errorf("nothing was recorded in %d; the journal has %s", year, yearList(years))
		}
		writeReview(cmd.OutOrStdout(), r)
		return nil
	},
}

func yearList(years []int) string {
	parts := make([]string, len(years))
	for i, y := range years {
		parts[i] = strconv.Itoa(y)
	}
	return strings.Join(parts, ", ")
}

// writeReview draws the card: a headline in a frame, the months as bars, and
// the year's findings one a line.
func writeReview(out io.Writer, r review.Review) {
	title := fmt.Sprintf("%d in wits", r.Year)
	if r.Running {
		title += fmt.Sprintf(", to %s", r.To.Format("2 January"))
	}
	frame(out, title, fmt.Sprintf("%.2f g ground on %d of %d days · %.2f g an active day",
		r.Ground, r.GrindDays, r.Days, r.PerGrindDay),
		fmt.Sprintf("%s · %s · %s", plural(r.Pickups, "prescription"), plural(r.Sessions, "session"), plural(r.Products, "strain")))
	fmt.Fprintln(out)
	months(out, r)
	fmt.Fprintln(out)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, l := range findings(r) {
		fmt.Fprintf(w, "  %s\t%s\n", l[0], l[1])
	}
	w.Flush()
}

// frame draws lines in a rounded box, the first of them bold where a
// terminal shows bold.
func frame(out io.Writer, lines ...string) {
	width := 0
	for _, l := range lines {
		width = max(width, utf8.RuneCountInString(l))
	}
	fmt.Fprintf(out, "╭%s╮\n", strings.Repeat("─", width+4))
	for _, l := range lines {
		fmt.Fprintf(out, "│  %s%s  │\n", l, strings.Repeat(" ", width-utf8.RuneCountInString(l)))
	}
	fmt.Fprintf(out, "╰%s╯\n", strings.Repeat("─", width+4))
}

var monthNames = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// months draws the grams ground each month as a bar, the busiest marked.
func months(out io.Writer, r review.Review) {
	top := 0.0
	for _, g := range r.Months {
		top = max(top, g)
	}
	last := 12
	if r.Running {
		last = int(r.To.Month())
	}
	for i := range last {
		g := r.Months[i]
		mark := ""
		if g == top && g > 0 {
			mark = "  ← the most"
		}
		fmt.Fprintf(out, "  %s  %-32s %6.2f g%s\n", monthNames[i], bar(g, top, 32), g, mark)
	}
}

// bar is a value as a bar of eighths, width cells long at the top.
func bar(v, top float64, width int) string {
	if top <= 0 || v <= 0 {
		return ""
	}
	eighths := int(math.Round(v / top * float64(width*8)))
	return strings.Repeat("█", eighths/8) + []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}[eighths%8]
}

// findings is the year's other figures, label and line, those it has.
func findings(r review.Review) [][2]string {
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
	return append(out, costLines(r)...)
}

// costLines is what the year cost, when any of it had a price.
func costLines(r review.Review) [][2]string {
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

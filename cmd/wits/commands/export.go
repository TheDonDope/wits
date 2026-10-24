package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/version"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOut    string
	exportAll    bool
	exportFrom   string
	exportTo     string
	exportSite   string
)

// Export is the `wits export` command.
var Export = &cobra.Command{
	Use:   "export",
	Short: "Write the journal out as plain Markdown",
	Long: "Write the journal out in a format that is readable without Wits:\n" +
		"printable for an appointment, diffable in git, and still legible if this\n" +
		"program is ever abandoned. Exports the current cycle by default.\n\n" +
		"With --from and --to, one document for that range of days — for an\n" +
		"appointment. With --site, a folder for GitHub Pages: an index and a page\n" +
		"for every fill, linked to each other.",
	Example: "  wits export > cycle.md\n" +
		"  wits export --all --out history.md\n" +
		"  wits export --from 2026-08-01 --to 2026-10-31 --out appointment.md\n" +
		"  wits export --site docs/",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if exportFormat != "markdown" && exportFormat != "md" {
			return fmt.Errorf("unknown format %q, only markdown is supported so far", exportFormat)
		}
		s, err := open()
		if err != nil {
			return err
		}
		window, err := parseWindow(exportFrom, exportTo, s.OpenedAt)
		if err != nil {
			return err
		}
		if exportSite != "" {
			cycles := window.cycles(s.State.Cycles)
			if err := writeSite(exportSite, cycles, s); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote an index and %s to %s\n", plural(len(cycles), "fill page"), exportSite)
			return nil
		}
		return toFile(cmd, exportOut, func(out io.Writer) string {
			if window.set() {
				writeRange(out, window, s)
				return window.String()
			}
			cycles := exportCycles(s)
			writeMarkdown(out, cycles, s.Products, s.State)
			return plural(len(cycles), "cycle")
		})
	},
}

// toFile runs write against the --out file, or standard output without one,
// and says what it wrote where.
func toFile(cmd *cobra.Command, path string, write func(io.Writer) string) error {
	if path == "" {
		write(cmd.OutOrStdout())
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	what := write(f)
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s to %s\n", what, path)
	return nil
}

// exportCycles is what a plain export covers: every cycle with --all, else the
// one in progress, else the last.
func exportCycles(s *session) []ledger.Cycle {
	cycles := s.State.Cycles
	if exportAll {
		return cycles
	}
	if current := s.Cycle(); current != nil {
		return []ledger.Cycle{*current}
	}
	if n := len(cycles); n > 0 {
		return cycles[n-1:]
	}
	return nil
}

// window is a range of whole days, both ends included; a zero end is open.
type window struct{ from, to time.Time }

// parseWindow reads --from and --to. Either may be left out: the range then
// runs from the first entry, or to today.
func parseWindow(from, to string, now time.Time) (window, error) {
	var w window
	var err error
	if from != "" {
		if w.from, err = time.ParseInLocation(time.DateOnly, from, time.Local); err != nil {
			return w, fmt.Errorf("--from %q is not a date, expected YYYY-MM-DD", from)
		}
	}
	if to != "" {
		if w.to, err = time.ParseInLocation(time.DateOnly, to, time.Local); err != nil {
			return w, fmt.Errorf("--to %q is not a date, expected YYYY-MM-DD", to)
		}
	} else if from != "" {
		w.to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	}
	if !w.from.IsZero() && w.to.Before(w.from) {
		return w, fmt.Errorf("--to %s is before --from %s", to, from)
	}
	return w, nil
}

func (w window) set() bool { return !w.from.IsZero() || !w.to.IsZero() }

// holds reports whether a moment falls on a day of the range.
func (w window) holds(t time.Time) bool {
	return (w.from.IsZero() || !t.Before(w.from)) && (w.to.IsZero() || t.Before(w.to.AddDate(0, 0, 1)))
}

// cycles are those that overlap the range, all of them when none is set.
func (w window) cycles(all []ledger.Cycle) []ledger.Cycle {
	if !w.set() {
		return all
	}
	var out []ledger.Cycle
	for _, c := range all {
		ended := !c.Open() && !w.from.IsZero() && c.End.Before(w.from)
		if !ended && (w.to.IsZero() || c.Start.Before(w.to.AddDate(0, 0, 1))) {
			out = append(out, c)
		}
	}
	return out
}

func (w window) String() string {
	from, to := "the first entry", "today"
	if !w.from.IsZero() {
		from = w.from.Format(time.DateOnly)
	}
	if !w.to.IsZero() {
		to = w.to.Format(time.DateOnly)
	}
	return from + " to " + to
}

// writeMarkdown renders cycles as a Markdown document.
func writeMarkdown(out io.Writer, cycles []ledger.Cycle, products *catalog.Catalog, state *ledger.State) {
	fmt.Fprintf(out, "# Wits journal\n\nExported %s.\n", time.Now().Format(time.DateOnly))

	if len(cycles) == 0 {
		fmt.Fprintln(out, "\nNothing recorded yet.")
		return
	}
	for _, c := range cycles {
		fmt.Fprintf(out, "\n## Cycle from %s%s\n\n", c.Start.Format(time.DateOnly), ending(c))
		writeCycle(out, c, products, state, "###")
	}
}

// ending says whether a cycle is still open, or when it emptied.
func ending(c ledger.Cycle) string {
	if c.Open() {
		return " (open)"
	}
	return ", emptied " + c.End.Format(time.DateOnly)
}

// writeCycle is one cycle's figures, products and entries, its sections
// headed at the given level.
func writeCycle(out io.Writer, c ledger.Cycle, products *catalog.Catalog, state *ledger.State, level string) {
	stats := ledger.Summarise(c.Events)
	fmt.Fprintln(out, "| | |")
	fmt.Fprintln(out, "| --- | --- |")
	fmt.Fprintf(out, "| Purchased | %.2f g |\n", c.Purchased)
	if c.Carried > 0 {
		fmt.Fprintf(out, "| On the shelf already | %.2f g |\n", c.Carried)
	}
	fmt.Fprintf(out, "| Ground meanwhile, from any jar | %.2f g |\n", c.Ground)
	if c.Open() {
		remaining := state.FillOnShelf(&c)
		fmt.Fprintf(out, "| Remaining of the fill | %.2f g (%s) |\n",
			remaining, percent(remaining, c.Purchased))
	}
	fmt.Fprintf(out, "| Days elapsed | %d |\n", stats.ElapsedDays)
	fmt.Fprintf(out, "| Days with an entry | %d |\n", stats.ActiveDays)
	fmt.Fprintf(out, "| Per active day | %.2f g |\n", stats.PerActiveDay)
	fmt.Fprintf(out, "| Median per day | %.2f g |\n", stats.MedianPerDay)

	fmt.Fprintf(out, "\n%s Products\n\n", level)
	for _, slug := range c.Products {
		fmt.Fprintf(out, "- `%s` — %s\n", slug, nameOf(products, slug))
	}
	fmt.Fprintf(out, "\n%s Events\n\n", level)
	writeEntries(out, c.Events, products)
}

// writeEntries is the table of entries, an undone one and its correction
// marked so.
func writeEntries(out io.Writer, events []journal.Event, products *catalog.Catalog) {
	undone := record.Reverted(events)
	fmt.Fprintln(out, "| Date | Event | Amount | Product | Device | Note |")
	fmt.Fprintln(out, "| --- | --- | ---: | --- | --- | --- |")
	for _, e := range events {
		note := e.Note
		if undone[e.Hash] {
			note = strings.TrimSpace("(undone) " + note)
		}
		fmt.Fprintf(out, "| %s | %s | %.2f g | %s | %s | %s |\n",
			e.OccurredAt.Format(time.DateOnly), e.Type, e.Grams, nameOf(products, e.Product), e.Device, cell(note))
	}
}

// nameOf is a product's name, or its handle when the catalog lacks it.
func nameOf(products *catalog.Catalog, slug string) string {
	if p, err := products.Find(slug); err == nil && slug != "" {
		return p.Name
	}
	return slug
}

// cell keeps text from breaking a Markdown table.
func cell(s string) string { return strings.ReplaceAll(s, "|", "\\|") }

// standing is the entries that stand: an undone entry and the correction that
// undid it cancel out, and neither belongs in a total.
func standing(events []journal.Event) []journal.Event {
	undone := record.Reverted(events)
	var out []journal.Event
	for _, e := range events {
		if !undone[e.Hash] && e.Reverts == "" {
			out = append(out, e)
		}
	}
	return out
}

// writeRange is one document for a range of days: what was dispensed, ground
// and used in it, by product, what the sessions in it were rated, and every
// entry. Every figure is the range's own — it says so — so none of it reads
// as a fill's or the shelf's.
func writeRange(out io.Writer, w window, s *session) {
	var events []journal.Event
	for _, e := range s.State.Events {
		if w.holds(e.OccurredAt) {
			events = append(events, e)
		}
	}
	fmt.Fprintf(out, "# Wits journal, %s\n\nExported %s. Every figure below is for these days only.\n",
		w, time.Now().Format(time.DateOnly))
	if len(events) == 0 {
		fmt.Fprintln(out, "\nNothing recorded in this range.")
		return
	}
	kept := standing(events)
	writeRangeSummary(out, w, kept)
	writeByProduct(out, kept, s.Products)
	writeRatings(out, kept, s)
	fmt.Fprint(out, "\n## Entries\n\n")
	writeEntries(out, events, s.Products)
}

// writeRangeSummary is the range's totals.
func writeRangeSummary(out io.Writer, w window, events []journal.Event) {
	stats := ledger.Summarise(events)
	first, last := w.from, w.to
	if first.IsZero() {
		first = events[0].OccurredAt
	}
	if last.IsZero() {
		last = events[len(events)-1].OccurredAt
	}
	days := int(last.Sub(first).Hours()/24) + 1
	var dispensed, seshed journal.Centigrams
	fills, sessions := 0, 0
	for _, e := range events {
		switch e.Type {
		case journal.Purchase:
			dispensed += journal.CG(e.Grams)
			fills++
		case journal.Sesh:
			seshed += journal.CG(e.Grams)
			sessions++
		}
	}
	fmt.Fprint(out, "\n## Summary\n\n| | |\n| --- | --- |\n")
	fmt.Fprintf(out, "| Days | %d |\n", days)
	fmt.Fprintf(out, "| Days with grinding | %d |\n", stats.ActiveDays)
	fmt.Fprintf(out, "| Ground | %.2f g |\n", stats.Ground)
	fmt.Fprintf(out, "| Per day with grinding | %.2f g |\n", stats.PerActiveDay)
	fmt.Fprintf(out, "| Median per day with grinding | %.2f g |\n", stats.MedianPerDay)
	fmt.Fprintf(out, "| Per calendar day | %.2f g |\n", ledger.Round(stats.Ground/float64(days)))
	fmt.Fprintf(out, "| Dispensed | %.2f g in %s |\n", dispensed.Grams(), plural(fills, "purchase"))
	fmt.Fprintf(out, "| Through a vaporizer | %.2f g in %s |\n", seshed.Grams(), plural(sessions, "session"))
}

// writeByProduct is what each product went through in the range.
func writeByProduct(out io.Writer, events []journal.Event, products *catalog.Catalog) {
	type use struct{ dispensed, ground, seshed journal.Centigrams }
	by := map[string]*use{}
	var order []string
	for _, e := range events {
		if e.Product == "" {
			continue
		}
		u := by[e.Product]
		if u == nil {
			u = &use{}
			by[e.Product] = u
			order = append(order, e.Product)
		}
		switch e.Type {
		case journal.Purchase:
			u.dispensed += journal.CG(e.Grams)
		case journal.Grind:
			u.ground += journal.CG(e.Grams)
		case journal.Sesh:
			u.seshed += journal.CG(e.Grams)
		}
	}
	fmt.Fprint(out, "\n## By product\n\n| Product | Dispensed | Ground | Through a vaporizer |\n| --- | ---: | ---: | ---: |\n")
	for _, slug := range order {
		u := by[slug]
		fmt.Fprintf(out, "| %s | %.2f g | %.2f g | %.2f g |\n",
			cell(nameOf(products, slug)), u.dispensed.Grams(), u.ground.Grams(), u.seshed.Grams())
	}
}

// writeRatings is what the range's sessions were rated, by product, when any
// were; each mean with the count it stands on.
func writeRatings(out io.Writer, events []journal.Event, s *session) {
	insights := effects.Analyse(s.Ratings, events, s.ProductName)
	if insights.Rated == 0 {
		return
	}
	fmt.Fprintf(out, "\n## How the sessions went\n\n%s rated, from 1 (not at all) to 5 (very well). ",
		plural(insights.Rated, "session"))
	fmt.Fprint(out, "Each mean is followed by how many ratings it stands on.\n\n")
	fmt.Fprint(out, "| Product |")
	for _, d := range effects.Dimensions {
		fmt.Fprintf(out, " %s |", d)
	}
	fmt.Fprint(out, "\n| --- |"+strings.Repeat(" ---: |", len(effects.Dimensions))+"\n")
	for _, g := range insights.ByProduct {
		fmt.Fprintf(out, "| %s |", cell(g.Label))
		for _, d := range effects.Dimensions {
			if m, ok := g.Means[d]; ok {
				fmt.Fprintf(out, " %.1f (%d) |", m, g.Counts[d])
			} else {
				fmt.Fprint(out, " — |")
			}
		}
		fmt.Fprintln(out)
	}
}

// writeSite writes a folder GitHub Pages can serve as it is: an index of every
// fill, and a page for each, linked to the index and to its neighbours. The
// links are to the Markdown files, which Pages turns into links between pages
// and which GitHub follows when the folder is only browsed.
func writeSite(dir string, cycles []ledger.Cycle, s *session) error {
	if err := os.MkdirAll(filepath.Join(dir, "fills"), 0700); err != nil {
		return err
	}
	width := len(strconv.Itoa(len(s.State.Cycles)))
	page := func(c ledger.Cycle) string { return fmt.Sprintf("%0*d.md", width, c.Seq+1) }
	var index strings.Builder
	writeIndex(&index, cycles, page)
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(index.String()), 0600); err != nil {
		return err
	}
	for i, c := range cycles {
		var b strings.Builder
		fmt.Fprintf(&b, "# Fill %d\n\nDispensed %s%s.\n\n", c.Seq+1, c.Start.Format(time.DateOnly), ending(c))
		nav := []string{"[All fills](../index.md)"}
		if i > 0 {
			nav = append([]string{fmt.Sprintf("[← Fill %d](%s)", cycles[i-1].Seq+1, page(cycles[i-1]))}, nav...)
		}
		if i < len(cycles)-1 {
			nav = append(nav, fmt.Sprintf("[Fill %d →](%s)", cycles[i+1].Seq+1, page(cycles[i+1])))
		}
		fmt.Fprintf(&b, "%s\n\n", strings.Join(nav, " · "))
		writeCycle(&b, c, s.Products, s.State, "##")
		if err := os.WriteFile(filepath.Join(dir, "fills", page(c)), []byte(b.String()), 0600); err != nil {
			return err
		}
	}
	return nil
}

// writeIndex is the site's front page: every fill in a table, newest first,
// each linking to its page, and the totals across them.
func writeIndex(out io.Writer, cycles []ledger.Cycle, page func(ledger.Cycle) string) {
	fmt.Fprintf(out, "# Wits journal\n\nExported %s. %s.\n", time.Now().Format(time.DateOnly), plural(len(cycles), "fill"))
	if len(cycles) == 0 {
		return
	}
	fmt.Fprint(out, "\n| Fill | Dispensed | Purchased | Ground meanwhile, any jar | Per active day | |\n")
	fmt.Fprint(out, "| --- | --- | ---: | ---: | ---: | --- |\n")
	var events []journal.Event
	for i := len(cycles) - 1; i >= 0; i-- {
		c := cycles[i]
		stats := ledger.Summarise(c.Events)
		state := "open"
		if !c.Open() {
			state = "emptied " + c.End.Format(time.DateOnly)
		}
		fmt.Fprintf(out, "| [%d](fills/%s) | %s | %.2f g | %.2f g | %.2f g | %s |\n",
			c.Seq+1, page(c), c.Start.Format(time.DateOnly), c.Purchased, c.Ground, stats.PerActiveDay, state)
	}
	for _, c := range cycles {
		events = append(events, c.Events...)
	}
	stats := ledger.Summarise(standing(events))
	fmt.Fprintf(out, "\nAcross them: %.2f g ground on %s, %.2f g a day with grinding.\n",
		stats.Ground, plural(stats.ActiveDays, "day"), stats.PerActiveDay)
}

var (
	reportFrom, reportTo, reportOut string
	reportSeal                      bool
)

// Report is the `wits report` command.
var Report = &cobra.Command{
	Use:   "report",
	Short: "Write one page for the prescriber, as a PDF",
	Long: "Write one page for the prescriber: the days asked for, grams a day, which\n" +
		"strains, what the sessions did by the effect log, and what is left of the\n" +
		"fill in progress, with when the next prescription is due. The last 90 days\n" +
		"unless --from and --to say otherwise. With --seal, the journal's head is\n" +
		"sealed first and the page names the seal, so the page can be checked\n" +
		"against the journal it was made from.",
	Example: "  wits report\n" +
		"  wits report --from 2026-08-01 --to 2026-10-31 --out appointment.pdf",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		w, err := reportWindow(s.OpenedAt)
		if err != nil {
			return err
		}
		if reportSeal {
			if _, _, err := sealHead(cmd, s); err != nil {
				return err
			}
		}
		out := reportOut
		if out == "" {
			out = "wits-report-" + w.to.Format(time.DateOnly) + ".pdf"
		}
		return toFile(cmd, out, func(f io.Writer) string {
			if err := report.PDF(f, report.Build(s.Workspace, w.from, w.to, version.String())); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "🚨 %v\n", err)
			}
			return "a report for " + w.String()
		})
	},
}

// reportWindow is the days a report covers: those asked for, filled in to
// the 90 days before today, or to today.
func reportWindow(now time.Time) (window, error) {
	w, err := parseWindow(reportFrom, reportTo, now)
	if err != nil {
		return w, err
	}
	if w.to.IsZero() {
		w.to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	}
	if w.from.IsZero() {
		w.from = w.to.AddDate(0, 0, -89)
	}
	return w, nil
}

func init() {
	Report.Flags().StringVar(&reportFrom, "from", "", "the first day, YYYY-MM-DD (default 90 days ago)")
	Report.Flags().StringVar(&reportTo, "to", "", "the last day, YYYY-MM-DD (default today)")
	Report.Flags().StringVar(&reportOut, "out", "", "the file to write (default wits-report-<last day>.pdf)")
	Report.Flags().BoolVar(&reportSeal, "seal", false, "seal the journal's head first, and name the seal on the page")
	Export.Flags().StringVar(&exportFormat, "format", "markdown", "output format")
	Export.Flags().StringVar(&exportOut, "out", "", "write to this file instead of stdout")
	Export.Flags().BoolVar(&exportAll, "all", false, "export every cycle, not just the current one")
	Export.Flags().StringVar(&exportFrom, "from", "", "the first day of a range, YYYY-MM-DD")
	Export.Flags().StringVar(&exportTo, "to", "", "the last day of a range, YYYY-MM-DD")
	Export.Flags().StringVar(&exportSite, "site", "", "write a site for GitHub Pages into this folder")
}

package commands

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/ledger"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOut    string
	exportAll    bool
)

// Export is the `wits export` command.
var Export = &cobra.Command{
	Use:   "export",
	Short: "Write the journal out as plain Markdown",
	Long: "Write the journal out in a format that is readable without Wits:\n" +
		"printable for an appointment, diffable in git, and still legible if this\n" +
		"program is ever abandoned. Exports the current cycle by default.",
	Example: "  wits export > cycle.md\n" +
		"  wits export --all --out history.md",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if exportFormat != "markdown" && exportFormat != "md" {
			return fmt.Errorf("unknown format %q, only markdown is supported so far", exportFormat)
		}
		s, err := open()
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		if exportOut != "" {
			f, err := os.OpenFile(exportOut, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			defer f.Close()
			out = f
		}

		cycles := s.state.Cycles
		if !exportAll {
			if current := s.state.CurrentCycle(); current != nil {
				cycles = []ledger.Cycle{*current}
			} else if n := len(cycles); n > 0 {
				cycles = cycles[n-1:]
			}
		}
		writeMarkdown(out, cycles, s.catalog)

		if exportOut != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %d cycle(s) to %s\n", len(cycles), exportOut)
		}
		return nil
	},
}

// writeMarkdown renders cycles as a Markdown document.
func writeMarkdown(out io.Writer, cycles []ledger.Cycle, products *catalog.Catalog) {
	fmt.Fprintf(out, "# Wits journal\n\nExported %s.\n", time.Now().Format(time.DateOnly))

	if len(cycles) == 0 {
		fmt.Fprintln(out, "\nNothing recorded yet.")
		return
	}

	for _, c := range cycles {
		fmt.Fprintf(out, "\n## Cycle from %s", c.Start.Format(time.DateOnly))
		if c.Open() {
			fmt.Fprintf(out, " (open)\n\n")
		} else {
			fmt.Fprintf(out, " to %s\n\n", c.End.Format(time.DateOnly))
		}

		stats := ledger.Summarise(c.Events)
		fmt.Fprintln(out, "| | |")
		fmt.Fprintln(out, "| --- | --- |")
		fmt.Fprintf(out, "| Purchased | %.2f g |\n", c.Purchased)
		fmt.Fprintf(out, "| Ground | %.2f g |\n", c.Ground)
		fmt.Fprintf(out, "| Remaining | %.2f g (%.0f%%) |\n", c.Remaining(), c.RemainingPct()*100)
		fmt.Fprintf(out, "| Days elapsed | %d |\n", stats.ElapsedDays)
		fmt.Fprintf(out, "| Days with an entry | %d |\n", stats.ActiveDays)
		fmt.Fprintf(out, "| Per active day | %.2f g |\n", stats.PerActiveDay)
		fmt.Fprintf(out, "| Median per day | %.2f g |\n", stats.MedianPerDay)

		fmt.Fprint(out, "\n### Products\n\n")
		for _, slug := range c.Products {
			name := slug
			if p, err := products.Find(slug); err == nil {
				name = p.Name
			}
			fmt.Fprintf(out, "- `%s` — %s\n", slug, name)
		}

		fmt.Fprint(out, "\n### Events\n\n")
		fmt.Fprintln(out, "| Date | Event | Amount | Product | Device | Note |")
		fmt.Fprintln(out, "| --- | --- | ---: | --- | --- | --- |")
		for _, e := range c.Events {
			fmt.Fprintf(out, "| %s | %s | %.2f g | %s | %s | %s |\n",
				e.OccurredAt.Format(time.DateOnly), e.Type, e.Grams, e.Product, e.Device, e.Note)
		}
	}
}

func init() {
	Export.Flags().StringVar(&exportFormat, "format", "markdown", "output format")
	Export.Flags().StringVar(&exportOut, "out", "", "write to this file instead of stdout")
	Export.Flags().BoolVar(&exportAll, "all", false, "export every cycle, not just the current one")
}

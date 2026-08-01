package commands

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits-tui/pkg/importer"
	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/spf13/cobra"
)

var importCommit bool

// Import is the `wits import` command.
var Import = &cobra.Command{
	Use:   "import <file.xlsx>",
	Short: "Import a tracking spreadsheet into the journal",
	Long: "Read a tracking spreadsheet and convert each worksheet into a\n" +
		"prescription fill and the daily grinds that followed it.\n\n" +
		"Nothing is written without --commit. The default is a dry run that\n" +
		"reports what would be imported and anything about the spreadsheet that\n" +
		"does not add up, so it can be checked before four years of history are\n" +
		"committed to the journal.",
	Example: "  wits import \"Tracking 2022.xlsx\"\n" +
		"  wits import \"Tracking 2022.xlsx\" --commit",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		result, err := importer.Import(args[0])
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		writeImportReport(out, result)

		if !importCommit {
			fmt.Fprintf(out, "\nDry run. Nothing was written. Re-run with --commit to import.\n")
			return nil
		}

		if existing := len(s.state.Events); existing > 0 {
			return fmt.Errorf("the journal already holds %d events; import into an empty repository so the history cannot be double counted", existing)
		}

		for _, p := range result.Products {
			if err := s.catalog.Add(p); err != nil {
				return err
			}
		}
		if err := s.catalog.Save(s.repo.ProductsPath()); err != nil {
			return err
		}
		for _, e := range result.Events {
			if _, err := s.journal.Append(e); err != nil {
				return fmt.Errorf("importing the event of %s: %w", e.OccurredAt.Format(time.DateOnly), err)
			}
		}
		fmt.Fprintf(out, "\nImported %d products and %d events.\n", len(result.Products), len(result.Events))
		return nil
	},
}

// writeImportReport describes what an import found.
func writeImportReport(out io.Writer, result *importer.Result) {
	purchased, ground := result.Grams()

	fmt.Fprintf(out, "%d worksheets, %d products, %d events\n",
		len(result.Sheets), len(result.Products), len(result.Events))
	fmt.Fprintf(out, "%.2fg dispensed, %.2fg ground, %.2fg unaccounted for\n",
		purchased, ground, purchased-ground)

	var purchases, grinds int
	var first, last time.Time
	for _, e := range result.Events {
		switch e.Type {
		case journal.Purchase:
			purchases++
		case journal.Grind:
			grinds++
		}
		if first.IsZero() || e.OccurredAt.Before(first) {
			first = e.OccurredAt
		}
		if e.OccurredAt.After(last) {
			last = e.OccurredAt
		}
	}
	if !first.IsZero() {
		fmt.Fprintf(out, "%d fills and %d grinds, from %s to %s\n",
			purchases, grinds, first.Format(time.DateOnly), last.Format(time.DateOnly))
	}

	anomalies := result.Anomalies()
	if len(anomalies) == 0 {
		fmt.Fprintln(out, "\nNothing in the spreadsheet looks wrong.")
		return
	}
	fmt.Fprintf(out, "\n%d thing(s) worth checking in the spreadsheet:\n\n", len(anomalies))
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, a := range anomalies {
		fmt.Fprintf(w, "  %s\n", a)
	}
	w.Flush()
}

func init() {
	Import.Flags().BoolVar(&importCommit, "commit", false, "write the events to the journal")
}

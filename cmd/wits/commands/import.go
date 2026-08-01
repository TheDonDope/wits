package commands

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/importer"
	"github.com/TheDonDope/wits/pkg/journal"
)

var importCommit bool

// Import is the `wits import` command.
var Import = &cobra.Command{
	Use:   "import <file.xlsx>",
	Short: "Import a tracking spreadsheet into the journal",
	Long: "Read a tracking spreadsheet and turn each worksheet into a prescription\n" +
		"fill and the daily grinds that followed it.\n\n" +
		"Nothing is written without --commit. The default is a dry run reporting\n" +
		"what would be imported and anything about the spreadsheet that does not\n" +
		"add up, so years of history can be checked before they are recorded.",
	Example: "  wits import \"Tracking.xlsx\"\n" +
		"  wits import \"Tracking.xlsx\" --commit",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		result, err := importer.Read(args[0])
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		writeReport(out, result)

		if !importCommit {
			fmt.Fprintf(out, "\nDry run. Nothing was written. Re-run with --commit to import.\n")
			return nil
		}
		if err := importer.Commit(s.Repo, result); err != nil {
			return err
		}
		fmt.Fprintf(out, "\nImported %d products and %d entries.\n",
			len(result.Products), len(result.Events))
		return nil
	},
}

// writeReport describes what an import found, and what it could not make sense
// of. The unaccounted figure is the one worth reading: it is the grams the
// spreadsheet dispensed but never wrote down using.
func writeReport(out io.Writer, result *importer.Result) {
	purchased, ground := result.Grams()
	counts := result.Counts()
	first, last := result.Span()

	fmt.Fprintf(out, "%d worksheets, %d products, %d entries\n",
		len(result.Sheets), len(result.Products), len(result.Events))
	fmt.Fprintf(out, "%.2f g dispensed, %.2f g ground, %.2f g unaccounted for\n",
		purchased, ground, purchased-ground)
	if !first.IsZero() {
		fmt.Fprintf(out, "%d fills and %d grinds, from %s to %s\n",
			counts[journal.Purchase], counts[journal.Grind],
			first.Format(time.DateOnly), last.Format(time.DateOnly))
	}

	if len(result.Merged) > 0 {
		fmt.Fprintf(out, "\n%d product(s) were named more than one way and become one:\n\n", len(result.Merged))
		for _, m := range result.Merged {
			fmt.Fprintf(out, "  %s\n", m.Slug)
			for _, n := range m.Names {
				fmt.Fprintf(out, "      %s\n", n)
			}
		}
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
	Import.Flags().BoolVar(&importCommit, "commit", false, "write the entries to the journal")
}

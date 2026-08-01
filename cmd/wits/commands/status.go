package commands

import (
	"fmt"
	"io"
	"math"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/spf13/cobra"
)

// Status is the `wits status` command.
var Status = &cobra.Command{
	Use:   "status",
	Short: "Show what is left and how long it will last",
	Long: "Show the working state derived from the journal: how much of each\n" +
		"product is in storage and in its tin, how far through the current cycle\n" +
		"you are, and how long the remainder will last at the observed rate.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		writeStatus(cmd.OutOrStdout(), s.State)
		return nil
	},
}

// writeStatus renders the state as a table.
func writeStatus(out io.Writer, state *ledger.State) {
	cycle := state.CurrentCycle()
	if cycle == nil {
		fmt.Fprintln(out, "No cycle in progress. Storage is empty.")
		if len(state.Cycles) > 0 {
			last := state.Cycles[len(state.Cycles)-1]
			fmt.Fprintf(out, "The last cycle ran %s to %s.\n",
				last.Start.Format(time.DateOnly), last.End.Format(time.DateOnly))
		}
		fmt.Fprintln(out, "\nRecord your next fill with `wits buy`.")
		return
	}

	stats := ledger.Summarise(cycle.Events)
	fmt.Fprintf(out, "On cycle %d, opened %s (day %d)\n\n",
		len(state.Cycles), cycle.Start.Format(time.DateOnly), daysSince(cycle.Start))

	// Only the products of this fill. Four years of history holds every product
	// ever dispensed, and listing all of them buries the three that are on the
	// shelf right now.
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PRODUCT\tSTORAGE\tSTASH\tAVB\tLEFT")
	for _, product := range cycle.Products {
		b := state.Balances[product]
		if b == nil {
			continue
		}
		fmt.Fprintf(w, "%s\t%.2fg\t%.2fg\t%.2fg\t%s\n",
			product, b.Storage, b.Stash, b.AVB, percent(b.Storage, purchasedOf(cycle, product, state)))
	}
	fmt.Fprintf(w, "\t\t\t\t\n")
	fmt.Fprintf(w, "total\t%.2fg\t\t\t%s\n", cycle.Remaining(), percent(cycle.Remaining(), cycle.Purchased))
	w.Flush()

	fmt.Fprintf(out, "\n%.2fg of %.2fg left over %s, %d of them with an entry\n",
		cycle.Remaining(), cycle.Purchased, plural(stats.ElapsedDays, "day"), stats.ActiveDays)
	if stats.PerActiveDay > 0 {
		fmt.Fprintf(out, "%.2fg per active day, %.2fg median, %.2fg per elapsed day\n",
			stats.PerActiveDay, stats.MedianPerDay, stats.PerElapsedDay)
		fmt.Fprintf(out, "About %s left at that rate\n",
			plural(int(math.Round(stats.DaysLeft(cycle.Remaining()))), "day"))
	} else {
		fmt.Fprintln(out, "Nothing ground yet this cycle, so there is no rate to extrapolate from")
	}
}

// purchasedOf returns how many grams of a product the cycle started with, so
// that a per-product percentage has something to be a percentage of.
func purchasedOf(cycle *ledger.Cycle, product string, state *ledger.State) float64 {
	var grams float64
	for _, e := range cycle.Events {
		if e.Product == product && e.Type == "purchase" {
			grams += e.Grams
		}
	}
	if grams == 0 {
		return state.Held(product)
	}
	return grams
}

// percent formats a share as a percentage, or a dash when there is nothing to
// compare against.
func percent(have, of float64) string {
	if of <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", have/of*100)
}

// daysSince returns the number of days from t until today, counting today.
func daysSince(t time.Time) int {
	return int(time.Since(t).Hours()/24) + 1
}

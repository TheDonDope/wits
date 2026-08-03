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
		"product is in storage and in its stash, how far through the current cycle\n" +
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
		// The cycle's own share of the jar, against what its fill dispensed.
		// A jar refilled before it was empty holds an older cycle's grams
		// too, and those stand on the older cycle's account.
		share := state.ShareOf(cycle, product)
		fmt.Fprintf(w, "%s\t%.2fg\t%.2fg\t%.2fg\t%s\n",
			product, share, b.Stash, b.AVB, percent(share, cycle.PurchasedOf(product)))
	}
	remaining := state.FillOnShelf(cycle)
	fmt.Fprintf(w, "\t\t\t\t\n")
	fmt.Fprintf(w, "total\t%.2fg\t\t\t%s\n", remaining, percent(remaining, cycle.Purchased))
	w.Flush()

	fmt.Fprintf(out, "\n%.2fg of %.2fg left over %s, %d of them with an entry\n",
		remaining, cycle.Purchased, plural(stats.ElapsedDays, "day"), stats.ActiveDays)
	if carried, jars, open := state.CarriedOnShelf(cycle); carried > 0 {
		fmt.Fprintf(out, "%.2fg more in %s, %s still open\n",
			carried, plural(jars, "older jar"), plural(open, "earlier cycle"))
	}
	if stats.PerActiveDay > 0 {
		fmt.Fprintf(out, "%.2fg per active day, %.2fg median, %.2fg per elapsed day\n",
			stats.PerActiveDay, stats.MedianPerDay, stats.PerElapsedDay)
		fmt.Fprintf(out, "About %s left at that rate\n",
			plural(int(math.Round(stats.DaysLeft(remaining))), "day"))
	} else {
		fmt.Fprintln(out, "Nothing ground yet this cycle, so there is no rate to extrapolate from")
	}
}

// percent formats a share as a percentage, or a dash when there is nothing to
// compare against.
func percent(have, of float64) string {
	if of <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", have/of*100)
}

// daysSince returns the number of days from t until today, counting today. It
// counts calendar days, the way the interface does, so the two never disagree
// about what day of the cycle it is around midnight.
func daysSince(t time.Time) int {
	now := time.Now()
	ty, tm, td := t.Date()
	ny, nm, nd := now.Date()
	from := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	to := time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC)
	return int(to.Sub(from).Hours()/24) + 1
}

package commands

import (
	"fmt"
	"io"
	"math"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
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
		warnChain(cmd.ErrOrStderr(), s.Workspace)
		warnClashes(cmd.ErrOrStderr(), s.State)
		writeStatus(cmd.OutOrStdout(), s.State, s.Cycle())
		if plan, ok := ledger.PlanRefill(s.Cycle(), s.OpenedAt, s.Repo.Config.Lead()); ok {
			fmt.Fprintln(cmd.OutOrStdout(), orderLine(plan, s.OpenedAt))
		}
		return nil
	},
}

// warnChain says so when the journal's hash chain does not verify.
func warnChain(out io.Writer, ws *workspace.Workspace) {
	if ws.Chain != nil {
		fmt.Fprintln(out, "⚠ the journal was changed outside wits; wits fsck says where")
	}
}

// warnClashes names every entry that took grams that were not in its jar —
// the same last grams recorded on two devices, once pulled together. The jar
// takes nothing more but an undo until one of them is reverted.
func warnClashes(out io.Writer, state *ledger.State) {
	for _, short := range ledger.Shortfalls(state.Events) {
		fmt.Fprintf(out, "🚨 %s %s: wits revert it, or the entry it clashes with\n", shortHash(short.Event.Hash), short)
	}
}

// writeStatus renders the state as a table.
func writeStatus(out io.Writer, state *ledger.State, cycle *ledger.Cycle) {
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

	stats := ledger.Summarise(cycle.Standing)
	fmt.Fprintf(out, "On cycle %d, opened %s (day %d)\n\n",
		cycle.Seq+1, cycle.Start.Format(time.DateOnly), daysSince(cycle.Start))

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
		// The stash and AVB the same way: as far as they are this fill's.
		share := state.ShareOf(cycle, product)
		stash, avb := state.FillGround(cycle, product)
		fmt.Fprintf(w, "%s\t%.2fg\t%.2fg\t%.2fg\t%s\n",
			product, share, stash, avb, percent(share, cycle.PurchasedOf(product)))
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
		fmt.Fprintf(out, "About %s' worth at that rate\n",
			plural(int(math.Round(stats.DaysWorth(float64(remaining)))), "day"))
	} else {
		fmt.Fprintln(out, "Nothing ground yet this cycle, so there is no rate to extrapolate from")
	}
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

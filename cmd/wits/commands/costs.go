// Package commands holds `wits costs` here: what the prescriptions cost.
package commands

import (
	"fmt"
	"io"
	"math"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/spf13/cobra"
)

var costsAll bool

func init() {
	Costs.Flags().BoolVar(&costsAll, "all", false, "every fill and month, not only the last twelve")
}

// Costs is the `wits costs` command.
var Costs = &cobra.Command{
	Use:   "costs",
	Short: "Show what the prescriptions cost",
	Long: "Show what was paid: in all and per gram, by fill and by month, and what a\n" +
		"session that helped cost — rated 4 or more overall, at its product's\n" +
		"price per gram. Only purchases with a price count; how many have none\n" +
		"is said, not guessed at.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		writeCosts(cmd.OutOrStdout(), prices.Analyse(s.State, prices.Latest(s.Prices), s.Ratings), costsAll)
		return nil
	},
}

// writeCosts is the costs, the newest fills and months first.
func writeCosts(out io.Writer, c prices.Costs, all bool) {
	if c.Priced == 0 {
		fmt.Fprintf(out, "No purchase has a price yet; %s without one. `wits reconcile purchases --all` walks them.\n",
			plural(c.Unpriced, "purchase"))
		return
	}
	fmt.Fprintf(out, "%s for %.2fg with a price, %s a gram", prices.Format(c.Cents), c.Priced,
		prices.Format(int64(math.Round(c.PerGram()))))
	if c.Unpriced > 0 {
		fmt.Fprintf(out, "; %s without one", plural(c.Unpriced, "purchase"))
	}
	fmt.Fprintln(out)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nFILL\tDISPENSED\tGRAMS\tPAID\tA GRAM\t")
	for i, shown := len(c.Fills)-1, 0; i >= 0 && (all || shown < 12); i-- {
		f := c.Fills[i]
		paid, per := "-", "-"
		if f.Priced > 0 {
			paid, per = prices.Format(f.Cents), prices.Format(int64(math.Round(f.PerGram())))
		}
		note := ""
		if f.Unpriced > 0 {
			note = plural(f.Unpriced, "purchase") + " without a price"
		}
		fmt.Fprintf(w, "%d\t%s\t%.2fg\t%s\t%s\t%s\n", f.Seq+1, f.Start.Format(time.DateOnly), f.Dispensed, paid, per, note)
		shown++
	}
	fmt.Fprintln(w, "\nMONTH\tPAID\tGRAMS\t\t\t")
	for i, shown := len(c.Months)-1, 0; i >= 0 && (all || shown < 12); i-- {
		m := c.Months[i]
		fmt.Fprintf(w, "%s\t%s\t%.2fg\t\t\t\n", m.Month, prices.Format(m.Cents), m.Grams)
		shown++
	}
	w.Flush()
	if c.Helped > 0 {
		fmt.Fprintf(out, "\n%s that helped (4 or more overall), %s each on average.\n",
			plural(c.Helped, "session"), prices.Format(int64(math.Round(c.PerHelped()))))
	}
}

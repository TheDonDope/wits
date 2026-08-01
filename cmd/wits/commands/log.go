package commands

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/spf13/cobra"
)

var (
	logOneline bool
	logProduct string
	logCurrent bool
	logLimit   int
)

// Log is the `wits log` command.
var Log = &cobra.Command{
	Use:   "log",
	Short: "Show the journal, newest first",
	Long: "Show the events in the journal, newest first, the way `git log` shows\n" +
		"commits. Nothing here can be edited: a mistake is corrected by\n" +
		"appending a compensating event.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}

		events := s.state.Events
		if logCurrent {
			if cycle := s.state.CurrentCycle(); cycle != nil {
				events = cycle.Events
			} else {
				events = nil
			}
		}
		if logProduct != "" {
			product, err := s.catalog.Find(logProduct)
			if err != nil {
				return err
			}
			var filtered []journal.Event
			for _, e := range events {
				if e.Product == product.Slug {
					filtered = append(filtered, e)
				}
			}
			events = filtered
		}

		out := cmd.OutOrStdout()
		if len(events) == 0 {
			fmt.Fprintln(out, "No events yet.")
			return nil
		}

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		shown := 0
		for i := len(events) - 1; i >= 0; i-- {
			if logLimit > 0 && shown == logLimit {
				break
			}
			e := events[i]
			if logOneline {
				fmt.Fprintf(w, "%s\t%s\t%s\t%.2fg\t%s\n",
					shortHash(e.Hash), e.OccurredAt.Format(time.DateOnly), e.Type, e.Grams, e.Product)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%-11s\t%.2fg\t%s\t%s -> %s\n",
					shortHash(e.Hash), e.OccurredAt.Format(time.DateOnly), e.Type, e.Grams, e.Product, e.From, e.To)
			}
			shown++
		}
		return w.Flush()
	},
}

func init() {
	Log.Flags().BoolVar(&logOneline, "oneline", false, "one compact line per event")
	Log.Flags().StringVar(&logProduct, "product", "", "only events for this product")
	Log.Flags().BoolVar(&logCurrent, "current", false, "only events in the current cycle")
	Log.Flags().IntVarP(&logLimit, "max-count", "n", 0, "show at most this many events")
}

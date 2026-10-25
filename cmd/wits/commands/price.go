// Package commands holds `wits price` here: what a purchase cost.
package commands

import (
	"fmt"
	"math"
	"time"

	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/spf13/cobra"
)

var priceNote string

func init() {
	Price.Flags().StringVar(&priceNote, "note", "", "a note")
}

// Price is the `wits price` command.
var Price = &cobra.Command{
	Use:   "price <purchase> <euros>",
	Short: "Write down what a purchase cost",
	Long: "Write down what was paid for a purchase, named by its hash as wits log\n" +
		"shows it. The purchase is not edited: the price goes in a log of its own\n" +
		"beside the journal, and giving one again replaces it, the log keeping both.\n" +
		"`wits reconcile purchases` walks every purchase without one.",
	Example: "  wits price 3f9a2c1 120,50",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		cents, err := prices.ParseEuros(args[1])
		if err != nil {
			return err
		}
		p, err := s.Recorder.Price(args[0], cents, priceNote, time.Now())
		if err != nil {
			return err
		}
		e, _ := s.Recorder.Find(p.Purchase)
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s for %.2fg of %s on %s, %s a gram\n", shortHash(p.Hash),
			prices.Format(cents), e.Grams, e.Product, e.OccurredAt.Format(time.DateOnly),
			prices.Format(int64(math.Round(float64(cents)/e.Grams))))
		return nil
	},
}

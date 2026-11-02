// Package commands holds `wits price` here: what a purchase cost, one at a
// time or every purchase still without one, asked in turn.
package commands

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
)

var (
	priceNote string
	priceAll  bool
)

func init() {
	Price.Flags().StringVar(&priceNote, "note", "", "a note")
	Price.Flags().BoolVar(&priceAll, "all", false, "asking: every purchase without a price, not only those still in storage")
}

// Price is the `wits price` command.
var Price = &cobra.Command{
	Use:   "price [<rev> <euros>]",
	Short: "Write down what a purchase cost",
	Long: "Write down what was paid for a purchase, named by its hash as wits log\n" +
		"shows it, or as HEAD~n. The purchase is not edited: the price goes in a\n" +
		"log of its own beside the journal, and giving one again replaces it, the\n" +
		"log keeping both.\n\n" +
		"On its own it asks, in turn, for every purchase still in storage without\n" +
		"a price; --all asks for every purchase without one, however long ago it\n" +
		"was emptied. A blank answer skips one.",
	Example: "  wits price HEAD 120,50\n" +
		"  wits price 3f9a2c1 120,50\n" +
		"  wits price --all",
	Args:              cobra.MatchAll(cobra.MaximumNArgs(2), notOne("a purchase and what it cost")),
	Annotations:       rehearsable(),
	ValidArgsFunction: completeRev,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			return askUnpriced(cmd, s)
		}
		cents, err := prices.ParseEuros(args[1])
		if err != nil {
			return err
		}
		e, err := resolve(s, args[0])
		if err != nil {
			return err
		}
		p, err := s.Recorder.Price(e.Hash, cents, priceNote, time.Now())
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s for %.2fg of %s on %s, %s a gram\n", shortHash(p.Hash),
			prices.Format(cents), e.Grams, e.Product, e.OccurredAt.Format(time.DateOnly),
			prices.Format(int64(math.Round(float64(cents)/e.Grams))))
		return nil
	},
}

// notOne refuses a single argument where a command takes none or a pair.
func notOne(pair string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) == 1 {
			return fmt.Errorf("give %s, or nothing to be asked", pair)
		}
		return nil
	}
}

// askUnpriced walks the purchases without a price, one question each, and
// writes down what was paid. A blank answer skips one; nothing is written
// until every question is answered.
func askUnpriced(cmd *cobra.Command, s *session) error {
	unpriced, err := s.Recorder.Unpriced(priceAll)
	if err != nil {
		return err
	}
	if len(unpriced) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Every purchase has its price.")
		return nil
	}
	if !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("asking needs a terminal; use `wits price <rev> <euros>`")
	}
	answers, err := askPrices(s, unpriced)
	if err != nil {
		return err
	}
	return applyPrices(cmd.OutOrStdout(), s, unpriced, answers)
}

// askPrices asks what each purchase cost, a question a purchase.
func askPrices(s *session, purchases []journal.Event) ([]string, error) {
	answers := make([]string, len(purchases))
	groups := make([]*huh.Group, 0, len(purchases))
	for i, e := range purchases {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("%s · %.2f g of %s", e.OccurredAt.Format(time.DateOnly), e.Grams, s.ProductName(e.Product))).
				Description(fmt.Sprintf("%d of %d. What was paid, in euros. Blank to skip.", i+1, len(purchases))).
				Value(&answers[i]).
				Validate(optionalEuros),
		))
	}
	return answers, formErr(huh.NewForm(groups...).Run())
}

// optionalEuros accepts a blank answer or an amount of euros.
func optionalEuros(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	_, err := prices.ParseEuros(s)
	return err
}

// applyPrices writes down the prices answered, and says what it wrote.
func applyPrices(out io.Writer, s *session, purchases []journal.Event, answers []string) error {
	written := 0
	for i, e := range purchases {
		if strings.TrimSpace(answers[i]) == "" {
			continue
		}
		cents, err := prices.ParseEuros(answers[i])
		if err != nil {
			return err
		}
		p, err := s.Recorder.Price(e.Hash, cents, "", time.Now())
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "[%s] %s for %.2fg of %s, %s a gram\n", shortHash(p.Hash), prices.Format(cents),
			e.Grams, e.Product, prices.Format(int64(math.Round(float64(cents)/e.Grams))))
		written++
	}
	fmt.Fprintf(out, "Priced %s; %d left without one.\n", plural(written, "purchase"), len(purchases)-written)
	return nil
}

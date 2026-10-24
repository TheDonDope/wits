package commands

import (
	"fmt"
	"io"
	"math"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/prices"
)

var (
	buyDate string
	buySlug string
)

// Buy is the `wits buy` command.
var Buy = &cobra.Command{
	Use:   "buy <product> <amount>",
	Short: "Record a prescription fill into storage",
	Long: "Record a prescription fill, moving grams into storage.\n\n" +
		"A product that is not in the catalog yet is added to it. The name is\n" +
		"parsed for a manufacturer, a THC/CBD ratio and a cultivar where it\n" +
		"follows the usual convention; anything it gets wrong can be corrected in\n" +
		".wits/products.yml.",
	Example: "  wits buy \"Enua 22/1 Wedding Cake\" 20g\n" +
		"  wits buy \"Cannamedical 28/1 Lemon Cookie\" 10g --slug lemon\n" +
		"  wits buy \"Cantourage 25/1 MAC1+\" 20g --date 2026-07-09",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		grams, err := parseGrams(args[1])
		if err != nil {
			return err
		}
		at, err := parseDate(buyDate)
		if err != nil {
			return err
		}
		e, product, added, err := s.Recorder.Buy(args[0], buySlug, grams, at)
		if err != nil {
			return err
		}
		if added {
			// The slug is what every later command refers to, so it is said
			// plainly rather than left to be discovered in products.yml.
			fmt.Fprintf(cmd.OutOrStdout(), "New product %s — refer to it as %s\n",
				product.Name, product.Slug)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] purchase %.2fg %s into storage\n",
			shortHash(e.Hash), e.Grams, product.Slug)
		if buyPrice == "" {
			return nil
		}
		// The purchase stands either way: a price that fails can be given
		// with wits price, and the purchase is not undone for it.
		cents, err := prices.ParseEuros(buyPrice)
		if err == nil {
			_, err = s.Recorder.Price(e.Hash, cents, "", at)
		}
		if err != nil {
			return fmt.Errorf("the purchase is recorded, its price is not (%w); give it with `wits price %s <euros>`",
				err, shortHash(e.Hash))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "paid %s, %s a gram\n", prices.Format(cents),
			prices.Format(int64(math.Round(float64(cents)/e.Grams))))
		return nil
	},
}

func init() {
	Buy.Flags().StringVar(&buyDate, "date", "", "the date the fill happened, defaults to now")
	Buy.Flags().StringVar(&buySlug, "slug", "", "what to call it from now on; made up if not given")
	Buy.Flags().StringVar(&buyPrice, "price", "", "what was paid, in euros")
	Price.Flags().StringVar(&priceNote, "note", "", "a note")
	Costs.Flags().BoolVar(&costsAll, "all", false, "every fill and month, not only the last twelve")
}

var (
	buyPrice  string
	priceNote string
	costsAll  bool
)

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

package commands

import (
	"fmt"
	"math"

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
}

var buyPrice string

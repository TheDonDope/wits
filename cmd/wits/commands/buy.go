package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var buyDate string

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
		"  wits buy \"Cannamedical 28/1 Lemon Cookie\" 10g --date 2026-07-09",
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
		e, product, added, err := s.recorder.Buy(args[0], grams, at)
		if err != nil {
			return err
		}
		if added {
			fmt.Fprintf(cmd.OutOrStdout(), "New product %s (%s)\n", product.Slug, product.Name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] purchase %.2fg %s into storage\n",
			shortHash(e.Hash), e.Grams, product.Slug)
		return nil
	},
}

func init() {
	Buy.Flags().StringVar(&buyDate, "date", "", "the date the fill happened, defaults to now")
}

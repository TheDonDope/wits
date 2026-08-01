package commands

import (
	"fmt"

	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/spf13/cobra"
)

var grindDate string

// Grind is the `wits grind` command.
var Grind = &cobra.Command{
	Use:   "grind <product> <amount>",
	Short: "Move ground product from storage into the stash",
	Long: "Record grinding product for the day, moving grams from storage into\n" +
		"that product's tin.\n\n" +
		"The product can be given as a slug or as any unambiguous part of its\n" +
		"name, so a daily entry stays short.",
	Example: "  wits grind wedding-cake 0.75\n" +
		"  wits grind lemon 1.2 --date 2026-07-29",
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
		at, err := parseDate(grindDate)
		if err != nil {
			return err
		}
		e, err := s.recorder.Grind(args[0], grams, at)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] grind %.2fg %s, %.2fg left in storage\n",
			shortHash(e.Hash), e.Grams, e.Product, s.recorder.Available(e.Product, journal.Storage))
		return nil
	},
}

func init() {
	Grind.Flags().StringVar(&grindDate, "date", "", "the date it was ground, defaults to now")
}

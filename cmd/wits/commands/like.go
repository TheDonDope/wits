// Package commands holds `wits like` here: the products most like one, by name, cannabinoids and terpenes.
package commands

import (
	"fmt"
	"slices"

	"github.com/TheDonDope/wits/pkg/report"
	"github.com/spf13/cobra"
)

// Like is the `wits like` command.
var Like = &cobra.Command{
	Use:   "like <product>",
	Short: "The products most like one, by name, cannabinoids and terpenes",
	Long: "Find the products whose profiles are nearest one's: the words of the\n" +
		"cultivar's name, the THC and CBD, and the terpenes where both have them\n" +
		"written down (`wits describe --terpenes`). The browser's Effects page draws\n" +
		"all of them as a map, coloured by what the effect log says each did.",
	Example: "  wits like wedding",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		p, err := s.Products.Find(args[0])
		if err != nil {
			return err
		}
		strains := report.Strains(s.Workspace)
		i := slices.IndexFunc(strains, func(x report.Strain) bool { return x.Slug == p.Slug })
		if i < 0 {
			return fmt.Errorf("%s was never dispensed, so it is not on the map", p.Name)
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Like %s:\n", p.Name)
		for _, slug := range strains[i].Like {
			j := slices.IndexFunc(strains, func(x report.Strain) bool { return x.Slug == slug })
			fmt.Fprintf(out, "  %s%s\n", strains[j].Name, rated(strains[j]))
		}
		return nil
	},
}

// rated says what the effect log says a strain did, when it says anything.
func rated(s report.Strain) string {
	if s.Ratings == 0 {
		return ""
	}
	return fmt.Sprintf(" — rated %.1f overall, over %s", s.Overall, plural(s.Ratings, "session"))
}

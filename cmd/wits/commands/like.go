// Package commands holds `wits like` here: the products most like one, by name, cannabinoids and terpenes.
package commands

import (
	"fmt"
	"slices"
	"strings"

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
		fmt.Fprintf(out, "Like %s%s:\n", p.Name, tastes(strains[i]))
		for _, l := range strains[i].Like {
			j := slices.IndexFunc(strains, func(x report.Strain) bool { return x.Slug == l.Slug })
			fmt.Fprintf(out, "  %3.0f %%  %s%s%s\n", l.Score*100, strains[j].Name, sharing(l), rated(strains[j]))
		}
		return nil
	},
}

// tastes says what a strain tastes of, when its terpenes say.
func tastes(s report.Strain) string {
	if s.Taste == "" {
		return ""
	}
	return " (" + strings.ToLower(s.Taste) + ")"
}

// sharing names the terpenes two strains share, when they share any.
func sharing(l report.Likeness) string {
	if len(l.Shared) == 0 {
		return ""
	}
	return " — shares " + strings.Join(l.Shared, ", ")
}

// rated says what the effect log says a strain did, when it says anything.
func rated(s report.Strain) string {
	if s.Ratings == 0 {
		return ""
	}
	return fmt.Sprintf(" — rated %.1f overall, over %s", s.Overall, plural(s.Ratings, "session"))
}

// Package commands: `wits describe`, a product shown, or its terpenes written down.

package commands

import (
	"fmt"
	"strings"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/spf13/cobra"
)

var describeTerpenes string

// Describe is the `wits describe` command.
var Describe = &cobra.Command{
	Use:   "describe <product>",
	Short: "Show a product, or write down its terpenes",
	Long: "Show what is known about a product. --terpenes writes down its terpenes as\n" +
		"the label lists them, dominant first, which is what places it on the map of\n" +
		"the strains and finds what is like it: `wits like <product>`.",
	Example: "  wits describe wedding --terpenes \"Myrcen, Limonen, Caryophyllen\"",
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
		if cmd.Flags().Changed("terpenes") {
			if p, err = s.Recorder.Terpenes(p.Slug, catalog.ParseTerpenes(describeTerpenes)); err != nil {
				return err
			}
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%s (%s)\n", p.Name, p.Slug)
		if p.THC > 0 || p.CBD > 0 {
			fmt.Fprintf(out, "THC %g %%, CBD %g %%\n", p.THC, p.CBD)
		}
		terpenes := "none written down yet"
		if len(p.Terpenes) > 0 {
			terpenes = strings.Join(p.Terpenes, ", ")
		}
		fmt.Fprintf(out, "Terpenes: %s\n", terpenes)
		return nil
	},
}

func init() {
	Describe.Flags().StringVar(&describeTerpenes, "terpenes", "", "its terpenes, dominant first, by commas")
}

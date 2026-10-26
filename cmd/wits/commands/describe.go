// Package commands holds `wits describe` here: a product shown, its terpenes written down, or the strain rated as a whole.
package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/spf13/cobra"
)

var (
	describeTerpenes string
	describeRating   string
	describeAgain    string
	describeSay      string
	describeUnrate   bool
)

// Describe is the `wits describe` command.
var Describe = &cobra.Command{
	Use:   "describe <product>",
	Short: "Show a product, write down its terpenes, or rate the strain",
	Long: "Show what is known about a product. --terpenes writes down its terpenes as\n" +
		"the label lists them, dominant first, which is what places it on the map of\n" +
		"the strains and finds what is like it: `wits like <product>`.\n\n" +
		"--rating rates the strain as a whole, apart from any one session (`wits rate`\n" +
		"rates those): overall, taste and aroma, 1 to 5. --again says whether you would\n" +
		"have it again, --say adds a line, and --unrate takes the rating away. Rated\n" +
		"again, the newer rating stands.",
	Example: "  wits describe wedding --terpenes \"Myrcen, Limonen, Caryophyllen\"\n" +
		"  wits describe wedding --rating \"overall=4 taste=5\" --again yes --say \"lemon, then pepper\"",
	Args: cobra.ExactArgs(1),
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
		if p, err = rateFromFlags(cmd, s, p); err != nil {
			return err
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
		fmt.Fprintf(out, "Rated: %s\n", ratedLine(p.Rating))
		return nil
	},
}

// rateFromFlags rates the strain, or takes its rating away, as the flags ask;
// with none of them, it leaves the product as it is.
func rateFromFlags(cmd *cobra.Command, s *session, p *catalog.Product) (*catalog.Product, error) {
	if describeUnrate {
		return s.Recorder.RateStrain(p.Slug, nil, time.Now())
	}
	if !cmd.Flags().Changed("rating") && !cmd.Flags().Changed("again") && !cmd.Flags().Changed("say") {
		return p, nil
	}
	r, err := catalog.ParseRating(describeRating)
	if err != nil {
		return nil, err
	}
	r.Again, r.Note = catalog.Again(strings.ToLower(describeAgain)), describeSay
	return s.Recorder.RateStrain(p.Slug, &r, time.Now())
}

// ratedLine says how a strain was rated as a whole, and when.
func ratedLine(r *catalog.StrainRating) string {
	if r == nil {
		return "not yet (--rating)"
	}
	out := r.Summary()
	if r.Note != "" {
		out = strings.TrimPrefix(out+" — "+r.Note, " — ")
	}
	return fmt.Sprintf("%s (%s)", out, r.At.Format("2 Jan 2006"))
}

func init() {
	Describe.Flags().StringVar(&describeTerpenes, "terpenes", "", "its terpenes, dominant first, by commas")
	Describe.Flags().StringVar(&describeRating, "rating", "", "rate the strain: overall, taste and aroma, 1 to 5, as \"overall=4 taste=5\"")
	Describe.Flags().StringVar(&describeAgain, "again", "", "whether you would have it again: yes or no")
	Describe.Flags().StringVar(&describeSay, "say", "", "a line about the strain, with its rating")
	Describe.Flags().BoolVar(&describeUnrate, "unrate", false, "take the strain's rating away")
}

// Package commands holds the products here: listed, described in full with
// the strains most like each, and their labels corrected.
package commands

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/report"
)

var (
	editTerpenes string
	editName     string
	editMaker    string
	editCultivar string
	editTHC      float64
	editCBD      float64
)

var getProducts = &cobra.Command{
	Use:     "products",
	Aliases: []string{"product", "jars", "jar"},
	Short:   "The jars: what each holds sealed, ground and given as AVB",
	Long: "The products, a row a jar: what is sealed in storage and ground in its\n" +
		"stash. Only those holding something, unless --all. -o wide adds the\n" +
		"label and what went through a device.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		var rows []productRow
		for _, p := range s.Products.Products {
			r := productRowOf(s, p)
			if getAll || r.Storage+r.Stash > 0 {
				rows = append(rows, r)
			}
		}
		return list(cmd.OutOrStdout(), rows, listing[productRow]{
			head: []string{"SLUG", "NAME", "STORAGE", "STASH"},
			wide: []string{"THC/CBD", "MAKER", "AVB", "CONSUMED", "TERPENES"},
			cells: func(r productRow) ([]string, []string) {
				return []string{r.Slug, r.Name, gramsCell(r.Storage), gramsCell(r.Stash)},
					[]string{fmt.Sprintf("%g/%g", r.THC, r.CBD), orDash(r.Maker), gramsCell(r.AVB),
						gramsCell(r.Consumed), orDash(strings.Join(r.Terpenes, ", "))}
			},
		})
	},
}

// productRow is one product as get lists it, and -o json prints it.
type productRow struct {
	Slug     string   `json:"slug"`
	Name     string   `json:"name"`
	Maker    string   `json:"maker,omitempty"`
	Cultivar string   `json:"cultivar,omitempty"`
	THC      float64  `json:"thc"`
	CBD      float64  `json:"cbd"`
	Storage  float64  `json:"storage"`
	Stash    float64  `json:"stash"`
	AVB      float64  `json:"avb"`
	Consumed float64  `json:"consumed"`
	Terpenes []string `json:"terpenes,omitempty"`
}

func productRowOf(s *session, p *catalog.Product) productRow {
	r := productRow{Slug: p.Slug, Name: p.Name, Maker: p.Manufacturer, Cultivar: p.Cultivar,
		THC: p.THC, CBD: p.CBD, Terpenes: p.Terpenes}
	if b := s.State.Balances[p.Slug]; b != nil {
		r.Storage, r.Stash, r.AVB, r.Consumed = b.Storage, b.Stash, b.AVB, b.Consumed
	}
	return r
}

var describeProduct = &cobra.Command{
	Use:     "product <product>",
	Aliases: []string{"products", "jar"},
	Short:   "A product in full: its label, its jar, its terpenes, its rating, and what is like it",
	Long: "Tell what is known about a product: the label, what its jar holds, its\n" +
		"terpenes, how the strain was rated (wits rate strain), and the products\n" +
		"most like it by name, cannabinoids and terpenes. The browser's Effects page\n" +
		"draws all of them as a map.",
	Example:           "  wits describe product wedding",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProduct(""),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		p, err := s.Products.Find(args[0])
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		writeProduct(out, s, p)
		writeLike(out, s, p)
		return nil
	},
}

var editProduct = &cobra.Command{
	Use:     "product <product>",
	Aliases: []string{"products", "jar"},
	Short:   "Correct what the label says, or write down the terpenes",
	Long: "--name, --maker, --cultivar, --thc and --cbd correct what the label\n" +
		"says; the slug its entries name it by stays. --terpenes writes down its\n" +
		"terpenes as the label lists them, dominant first, which is what places\n" +
		"it on the map of the strains and finds what is like it.",
	Example: "  wits edit product wedding --name \"Enua 22/1 Wedding Cake\" --thc 22 --cbd 1\n" +
		"  wits edit product wedding --terpenes \"Myrcen, Limonen, Caryophyllen\"",
	Args:              cobra.ExactArgs(1),
	Annotations:       rehearsable(),
	ValidArgsFunction: completeProduct(""),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		p, err := s.Products.Find(args[0])
		if err != nil {
			return err
		}
		edited, err := detailsFromFlags(cmd, s, p)
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("terpenes") {
			if edited, err = s.Recorder.Terpenes(p.Slug, catalog.ParseTerpenes(editTerpenes)); err != nil {
				return err
			}
		} else if edited == p {
			return fmt.Errorf("nothing to change: say what, with --name, --maker, --cultivar, --thc, --cbd or --terpenes")
		}
		writeProduct(cmd.OutOrStdout(), s, edited)
		return nil
	},
}

// writeProduct tells a product: its label, its jar, its terpenes, its rating.
func writeProduct(out io.Writer, s *session, p *catalog.Product) {
	fmt.Fprintf(out, "%s (%s)\n", p.Name, p.Slug)
	if made := strings.Trim(p.Manufacturer+" · "+p.Cultivar, " ·"); made != "" {
		fmt.Fprintln(out, made)
	}
	if p.THC > 0 || p.CBD > 0 {
		fmt.Fprintf(out, "THC %g %%, CBD %g %%\n", p.THC, p.CBD)
	}
	if b := s.State.Balances[p.Slug]; b != nil {
		fmt.Fprintf(out, "Jar: %.2f g sealed, %.2f g ground, %.2f g given as AVB, %.2f g consumed\n",
			b.Storage, b.Stash, b.AVB, b.Consumed)
	}
	terpenes := "none written down yet (wits edit product --terpenes)"
	if len(p.Terpenes) > 0 {
		terpenes = strings.Join(p.Terpenes, ", ")
	}
	fmt.Fprintf(out, "Terpenes: %s\n", terpenes)
	fmt.Fprintf(out, "Rated: %s\n", ratedLine(p.Rating))
}

// writeLike lists the products nearest one's profile, when it is on the map.
func writeLike(out io.Writer, s *session, p *catalog.Product) {
	strains := report.Strains(s.Workspace)
	i := slices.IndexFunc(strains, func(x report.Strain) bool { return x.Slug == p.Slug })
	if i < 0 || len(strains[i].Like) == 0 {
		return
	}
	fmt.Fprintf(out, "\nLike it%s:\n", tastes(strains[i]))
	for _, l := range strains[i].Like {
		j := slices.IndexFunc(strains, func(x report.Strain) bool { return x.Slug == l.Slug })
		fmt.Fprintf(out, "  %3.0f %%  %s%s%s\n", l.Score*100, strains[j].Name, sharing(l), rated(strains[j]))
	}
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

// detailsFromFlags corrects what the label says, as far as the flags say it,
// keeping the rest of the product as it is; with none of them, it leaves the
// product alone.
func detailsFromFlags(cmd *cobra.Command, s *session, p *catalog.Product) (*catalog.Product, error) {
	next, changed := *p, false
	for _, f := range []struct {
		name string
		set  func()
	}{
		{"name", func() { next.Name = editName }},
		{"maker", func() { next.Manufacturer = editMaker }},
		{"cultivar", func() { next.Cultivar = editCultivar }},
		{"thc", func() { next.THC = editTHC }},
		{"cbd", func() { next.CBD = editCBD }},
	} {
		if cmd.Flags().Changed(f.name) {
			f.set()
			changed = true
		}
	}
	if !changed {
		return p, nil
	}
	return s.Recorder.Describe(p.Slug, next)
}

// ratedLine says how a strain was rated as a whole, and when.
func ratedLine(r *catalog.StrainRating) string {
	if r == nil {
		return "not yet (wits rate strain)"
	}
	out := r.Summary()
	if r.Note != "" {
		out = strings.TrimPrefix(out+" — "+r.Note, " — ")
	}
	return fmt.Sprintf("%s (%s)", out, r.At.Format("2 Jan 2006"))
}

func init() {
	f := editProduct.Flags()
	f.StringVar(&editName, "name", "", "what the product is called, as it should read")
	f.StringVar(&editMaker, "maker", "", "who makes it")
	f.StringVar(&editCultivar, "cultivar", "", "the strain it is grown from")
	f.Float64Var(&editTHC, "thc", 0, "its THC, in per cent")
	f.Float64Var(&editCBD, "cbd", 0, "its CBD, in per cent")
	f.StringVar(&editTerpenes, "terpenes", "", "its terpenes, dominant first, by commas")
	Get.AddCommand(getProducts)
	Describe.AddCommand(describeProduct)
	Edit.AddCommand(editProduct)
}

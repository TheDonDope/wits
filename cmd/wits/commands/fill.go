// Package commands holds the fills and the journal's entries as get lists
// them here: every prescription fill, every session, every entry.
package commands

import (
	"cmp"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

var getFills = &cobra.Command{
	Use:     "fills",
	Aliases: []string{"fill", "cycles", "cycle"},
	Short:   "Every prescription fill: when it opened, what it dispensed, what of it is left",
	Long: "The fills, a row each, oldest first: one prescription filled, opened by\n" +
		"its purchases and closed when its own jars are empty. LEFT is the fill's\n" +
		"own grams still in storage, not the shelf's.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		rows := make([]fillRow, 0, len(s.State.Cycles))
		for i := range s.State.Cycles {
			rows = append(rows, fillRowOf(s, &s.State.Cycles[i]))
		}
		return list(cmd.OutOrStdout(), rows, listing[fillRow]{
			head: []string{"FILL", "OPENED", "DISPENSED", "LEFT", "STATE"},
			wide: []string{"CLOSED", "PRODUCTS"},
			cells: func(r fillRow) ([]string, []string) {
				state, closed := "open", "-"
				if r.Closed != nil {
					state, closed = "closed", r.Closed.Format(time.DateOnly)
				}
				return []string{strconv.Itoa(r.Fill), r.Opened.Format(time.DateOnly), gramsCell(r.Dispensed), gramsCell(r.Left), state},
					[]string{closed, fmt.Sprint(len(r.Products))}
			},
		})
	},
}

// fillRow is one fill as get lists it, and -o json prints it. Dispensed and
// Left are the fill's own grams: the scope of the fill, never the shelf's.
type fillRow struct {
	Fill      int        `json:"fill"`
	Opened    time.Time  `json:"opened"`
	Closed    *time.Time `json:"closed,omitempty"`
	Dispensed float64    `json:"dispensed"`
	Left      float64    `json:"left"`
	Products  []string   `json:"products"`
}

func fillRowOf(s *session, c *ledger.Cycle) fillRow {
	r := fillRow{Fill: c.Seq + 1, Opened: c.Start, Dispensed: float64(c.Purchased),
		Left: float64(s.State.FillOnShelf(c)), Products: c.Products}
	if !c.Open() {
		r.Closed = &c.End
	}
	return r
}

var describeFill = &cobra.Command{
	Use:     "fill [<number>]",
	Aliases: []string{"fills", "cycle"},
	Short:   "One fill in full: its products and what of each is left; without a number, the latest",
	Example: "  wits describe fill\n  wits describe fill 30",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		if len(s.State.Cycles) == 0 {
			return fmt.Errorf("no fill yet: wits buy records one")
		}
		c := &s.State.Cycles[len(s.State.Cycles)-1]
		if len(args) == 1 {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 || n > len(s.State.Cycles) {
				return fmt.Errorf("%q is not a fill; they are numbered 1 to %d", args[0], len(s.State.Cycles))
			}
			c = &s.State.Cycles[n-1]
		}
		out := cmd.OutOrStdout()
		r := fillRowOf(s, c)
		state := "open"
		if r.Closed != nil {
			state = "closed on " + r.Closed.Format("2 Jan 2006")
		}
		fmt.Fprintf(out, "Fill %d, opened %s, %s\n", r.Fill, r.Opened.Format("2 Jan 2006"), state)
		fmt.Fprintf(out, "Dispensed %s, %s of it left in storage\n", gramsCell(r.Dispensed), gramsCell(r.Left))
		if c.Carried > 0 {
			fmt.Fprintf(out, "Carried in %s from earlier fills\n", gramsCell(float64(c.Carried)))
		}
		fmt.Fprintln(out)
		for _, slug := range c.Products {
			fmt.Fprintf(out, "  %-40s %s left of this fill's\n", s.ProductName(slug), gramsCell(float64(s.State.ShareOf(c, slug))))
		}
		return nil
	},
}

var getSessions = &cobra.Command{
	Use:     "sessions",
	Aliases: []string{"session", "sesh"},
	Short:   "Every session that stands, oldest first: what, how much, with what, how hot",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		ratings := effects.Latest(s.Ratings)
		var rows []sessionRow
		for _, e := range s.State.Standing {
			if e.Type != journal.Sesh {
				continue
			}
			r := sessionRow{Hash: e.Hash, At: e.OccurredAt, Product: e.Product, Grams: e.Grams,
				Device: e.Device, Temperature: e.Temperature, Note: e.Note}
			if rating, ok := ratings[e.Hash]; ok {
				r.Rated = effects.Summary(rating.Scores)
			}
			rows = append(rows, r)
		}
		return list(cmd.OutOrStdout(), rows, listing[sessionRow]{
			head: []string{"ENTRY", "WHEN", "PRODUCT", "GRAMS", "DEVICE", "TEMP"},
			wide: []string{"RATED", "NOTE"},
			cells: func(r sessionRow) ([]string, []string) {
				temp := "-"
				if r.Temperature > 0 {
					temp = fmt.Sprintf("%d°C", r.Temperature)
				}
				return []string{shortHash(r.Hash), r.At.Format("2006-01-02 15:04"), r.Product, gramsCell(r.Grams), orDash(r.Device), temp},
					[]string{orDash(r.Rated), orDash(r.Note)}
			},
		})
	},
}

// sessionRow is one session as get lists it, and -o json prints it.
type sessionRow struct {
	Hash        string    `json:"hash"`
	At          time.Time `json:"at"`
	Product     string    `json:"product"`
	Grams       float64   `json:"grams"`
	Device      string    `json:"device,omitempty"`
	Temperature int       `json:"temperature,omitempty"`
	Rated       string    `json:"rated,omitempty"`
	Note        string    `json:"note,omitempty"`
}

var getEntries = &cobra.Command{
	Use:     "entries",
	Aliases: []string{"entry", "events"},
	Short:   "Every entry of the journal, in the order things happened",
	Long: "Every entry, undone ones and their undoing included, oldest first, as\n" +
		"the replay folds them. -o json prints each entry whole, as the journal\n" +
		"holds it; wits log reads the same entries newest first.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		return list(cmd.OutOrStdout(), s.State.Events, listing[journal.Event]{
			head: []string{"ENTRY", "WHEN", "TYPE", "GRAMS", "PRODUCT"},
			wide: []string{"FROM", "TO", "NOTE"},
			cells: func(e journal.Event) ([]string, []string) {
				return []string{shortHash(e.Hash), e.OccurredAt.Format("2006-01-02 15:04"), string(e.Type), gramsCell(e.Grams), orDash(e.Product)},
					[]string{string(e.From), string(e.To), orDash(cmp.Or(e.Note, reverting(e)))}
			},
		})
	},
}

// reverting says which entry an undo undoes.
func reverting(e journal.Event) string {
	if e.Reverts == "" {
		return ""
	}
	return "undoes " + shortHash(e.Reverts)
}

func init() {
	Get.AddCommand(getFills, getSessions, getEntries)
	Describe.AddCommand(describeFill)
}

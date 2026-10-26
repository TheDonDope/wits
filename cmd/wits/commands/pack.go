// Package commands holds `wits pack` here: what to take on a trip, signed.
package commands

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/TheDonDope/wits/pkg/pack"
	"github.com/spf13/cobra"
)

var (
	packDays   int
	packMargin int
	packOldest bool
	packOut    string
)

// Pack is the `wits pack` command.
var Pack = &cobra.Command{
	Use:   "pack",
	Short: "What to take on a trip, and a signed statement of exactly that",
	Long: "The grams for --days away at the rate the refill planner runs on, with\n" +
		"--margin per cent on top (20 unless given), rounded up to a tenth of a gram.\n" +
		"They are drawn from the jars on the shelf evenly, every jar giving about the\n" +
		"same grams so the trip has a mix and the jars go down together, or with\n" +
		"--oldest from the oldest fill's jars first; each jar's ground grams before\n" +
		"its sealed ones. Nothing is recorded: grind what the plan says to, as usual.\n\n" +
		"The plan is signed as a statement of what is carried, for exactly the grams\n" +
		"packed rather than the whole shelf; --out writes its QR code (.svg, .png) or\n" +
		"the statement (.txt), as wits carry does.",
	Example: "  wits pack --days 10\n  wits pack --days 4 --margin 0 --oldest --out trip.svg",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		mode := pack.Even
		if packOldest {
			mode = pack.Oldest
		}
		p, err := pack.Make(s.Workspace, packDays, packMargin, mode)
		if err != nil {
			return err
		}
		writePack(cmd.OutOrStdout(), p)
		c, err := signCarry(cmd, s, p.Statement())
		if err != nil {
			return err
		}
		if packOut == "" {
			return nil
		}
		data, err := carryFile(c, packOut)
		if err != nil {
			return err
		}
		if err := os.WriteFile(packOut, data, 0600); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s, signed for the %.2f g packed\n", packOut, p.Packed)
		return nil
	},
}

// writePack says what to take, from which jar, and how long it lasts.
func writePack(out io.Writer, p pack.Plan) {
	if p.Short {
		fmt.Fprintf(out, "%s at %.2f g a day wants %.2f g, and the shelf holds %.2f g: take it all, %.1f days' worth.\n\n",
			plural(p.Days, "day"), p.Rate, p.Need, p.Shelf, p.Packed/p.Rate)
	} else {
		fmt.Fprintf(out, "Take %.2f g: %s at %.2f g a day, with %d %% to spare.\n\n", p.Packed, plural(p.Days, "day"), p.Rate, p.Margin)
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PRODUCT\tGROUND\tSEALED\tTAKE\tDAYS\tLEFT")
	for _, l := range p.Lines {
		left := l.HeldStash + l.HeldSealed - l.Taken()
		fmt.Fprintf(w, "%s\t%.2fg\t%.2fg\t%.2fg\t%.1f\t%.2fg\n", l.Name, l.Stash, l.Sealed, l.Taken(), l.Days, max(left, 0))
	}
	w.Flush()
	how := "evenly, about the same from every jar"
	if p.Mode == pack.Oldest {
		how = "oldest fill first"
	}
	fmt.Fprintf(out, "\nDrawn %s, ground before sealed.", how)
	if sealed(p) {
		fmt.Fprint(out, " Grind the sealed grams before you go.")
	}
	fmt.Fprintln(out)
}

// sealed reports whether the plan takes any sealed grams, which want grinding.
func sealed(p pack.Plan) bool {
	for _, l := range p.Lines {
		if l.Sealed > 0 {
			return true
		}
	}
	return false
}

func init() {
	Pack.Flags().IntVar(&packDays, "days", 7, "the days away")
	Pack.Flags().IntVar(&packMargin, "margin", pack.DefaultMargin, "per cent to take on top")
	Pack.Flags().BoolVar(&packOldest, "oldest", false, "the oldest fill's jars first, rather than evenly")
	Pack.Flags().StringVar(&packOut, "out", "", "write the QR code (.svg, .png) or the statement (.txt)")
}

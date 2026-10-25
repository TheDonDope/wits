// Package commands holds `wits avb` here: what comes out of the vaporizer, weighed, and used.
package commands

import (
	"cmp"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/spf13/cobra"
)

var (
	avbDevice string
	avbDate   string
	avbNote   string
)

// AVB is the `wits avb` command: the AVB jar, and what each emptying of a
// vaporizer gave back.
var AVB = &cobra.Command{
	Use:   "avb",
	Short: "Show the AVB jar, and what each vaporizer gave back",
	Long: "Already vaped bud is what is left of a session once it has been through a\n" +
		"device. `avb collect` records what is weighed out when a device is emptied;\n" +
		"`avb use` records it drawn down for butter or a tincture. On its own, this\n" +
		"shows what the jar holds and every collection's yield: the share of what\n" +
		"went through the device that came back out.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		writeAVB(cmd.OutOrStdout(), s)
		return nil
	},
}

// writeAVB is the jar and its yields, newest first.
func writeAVB(out io.Writer, s *session) {
	var held journal.Centigrams
	for _, b := range s.State.Balances {
		held += journal.CG(b.AVB)
	}
	fmt.Fprintf(out, "%.2fg in the AVB jar\n", held.Grams())
	yields := ledger.Yields(s.State.Events)
	if len(yields) == 0 {
		fmt.Fprintln(out, "\nNothing collected yet: `wits avb collect <grams>` when a device is emptied.")
		return
	}
	fmt.Fprintln(out)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "COLLECTED\tDEVICE\tAVB\tFROM\tYIELD")
	for i := len(yields) - 1; i >= 0; i-- {
		y := yields[i]
		fmt.Fprintf(w, "%s\t%s\t%.2fg\t%.2fg in %s\t%.0f%%\n", y.At.Format(time.DateOnly),
			cmp.Or(y.Device, "-"), y.Collected, y.From, plural(y.Sessions, "session"), y.Ratio()*100)
	}
	w.Flush()
}

var avbCollect = &cobra.Command{
	Use:   "collect <grams>",
	Short: "Record the AVB weighed out of a device",
	Long: "Record what was weighed out of a device when it was emptied. It came from\n" +
		"the sessions put through it since it was last emptied, and is shared\n" +
		"between their products in proportion to what each put in. --device names\n" +
		"the device when sessions are waiting on more than one.",
	Example: "  wits avb collect 0.85 --device mighty",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, grams, at, err := avbArgs(args[0])
		if err != nil {
			return err
		}
		stored, err := s.Recorder.CollectAVB(avbDevice, grams, at, avbNote)
		if err != nil {
			return err
		}
		y := ledger.Yields(s.Recorder.State().Events)
		last := y[len(y)-1]
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] collected %.2fg of AVB from %.2fg in %s: %.0f%% back\n",
			shortHash(stored[0].Hash), last.Collected, last.From, plural(last.Sessions, "session"), last.Ratio()*100)
		return nil
	},
}

var avbUse = &cobra.Command{
	Use:     "use <grams>",
	Short:   "Record AVB used up",
	Long:    "Record AVB drawn down — for butter, a tincture — from the jar as a whole.",
	Example: "  wits avb use 5 --note butter",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, grams, at, err := avbArgs(args[0])
		if err != nil {
			return err
		}
		stored, err := s.Recorder.UseAVB(grams, at, avbNote)
		if err != nil {
			return err
		}
		var left journal.Centigrams
		for _, b := range s.Recorder.State().Balances {
			left += journal.CG(b.AVB)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] used %.2fg of AVB, %.2fg left in the jar\n",
			shortHash(stored[0].Hash), grams, left.Grams())
		return nil
	},
}

// avbArgs opens the repository and reads an amount and the --date.
func avbArgs(amount string) (*session, float64, time.Time, error) {
	s, err := open()
	if err != nil {
		return nil, 0, time.Time{}, err
	}
	grams, err := parseGrams(amount)
	if err != nil {
		return nil, 0, time.Time{}, err
	}
	at, err := parseDate(avbDate)
	return s, grams, at, err
}

func init() {
	AVB.AddCommand(avbCollect, avbUse)
	avbCollect.Flags().StringVar(&avbDevice, "device", "", "the device emptied")
	for _, c := range []*cobra.Command{avbCollect, avbUse} {
		c.Flags().StringVar(&avbDate, "date", "", "when, YYYY-MM-DD (default now)")
		c.Flags().StringVar(&avbNote, "note", "", "a note")
	}
}

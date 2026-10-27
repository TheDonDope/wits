// Package commands holds `wits amend` here: an entry's amount corrected, by
// undoing it and recording it again as it should have been.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var amendNote string

// Amend is the `wits amend` command.
var Amend = &cobra.Command{
	Use:   "amend <entry> <grams>",
	Short: "Correct an entry's amount, keeping the original",
	Long: "Correct the amount of an earlier entry.\n\n" +
		"Nothing is edited in place: the entry is undone, as `wits revert` undoes it,\n" +
		"and recorded again with the amount it should have had, both in one write.\n" +
		"The journal keeps the original, the undo and the correction. The new amount\n" +
		"is checked as any entry is: a jar gives no more than it held.\n\n" +
		"The entry is named by its hash, abbreviated as `wits log` shows it.",
	Example: "  wits amend 8297238 0.75\n" +
		"  wits amend 8297238 0.75 --note \"misread the scale\"",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeEntry,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		grams, err := parseGrams(args[1])
		if err != nil {
			return err
		}
		original, err := s.Recorder.Find(args[0])
		if err != nil {
			return err
		}
		both, err := s.Recorder.Amendment(original.Hash, grams, amendNote)
		if err != nil {
			return err
		}
		e := both[1]
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] amended %s %s from %s: %.2fg, not %.2fg\n",
			shortHash(e.Hash), e.Type, s.ProductName(e.Product),
			e.OccurredAt.Format("2006-01-02"), e.Grams, original.Grams)
		return nil
	},
}

func init() {
	Amend.Flags().StringVar(&amendNote, "note", "", "why the amount was wrong; the entry keeps its own note otherwise")
}

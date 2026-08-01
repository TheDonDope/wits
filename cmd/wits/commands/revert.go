package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var revertReason string

// Revert is the `wits revert` command.
var Revert = &cobra.Command{
	Use:   "revert <entry>",
	Short: "Undo an entry by recording a correction",
	Long: "Undo an earlier entry.\n\n" +
		"Nothing is removed. The journal is append-only and hash chained, which is\n" +
		"what lets a bundle be verified against the repository it came from, so an\n" +
		"entry is undone by moving the same grams back the way they came. Both the\n" +
		"original and the correction stay in the log.\n\n" +
		"The entry is named by its hash, abbreviated as `wits log` shows it.",
	Example: "  wits revert 8297238\n" +
		"  wits revert 8297238 --reason \"weighed the jar, not the herb\"",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		original, err := s.recorder.Find(args[0])
		if err != nil {
			return err
		}
		e, err := s.recorder.Revert(args[0], revertReason)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] undid %s %.2fg %s from %s\n",
			shortHash(e.Hash), original.Type, original.Grams, original.Product,
			original.OccurredAt.Format("2006-01-02"))
		return nil
	},
}

func init() {
	Revert.Flags().StringVar(&revertReason, "reason", "", "why the entry is being undone")
}

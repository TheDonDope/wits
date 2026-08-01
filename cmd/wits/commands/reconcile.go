package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/journal"
)

var (
	reconcileStash  bool
	reconcileAVB    bool
	reconcileReason string
	reconcileDryRun bool
)

// Reconcile is the `wits reconcile` command.
var Reconcile = &cobra.Command{
	Use:   "reconcile <product> <weight>",
	Short: "Make an account agree with the scale",
	Long: "Record the difference between what the ledger believes an account holds\n" +
		"and what it actually weighs.\n\n" +
		"Ledgers drift: a little is spilled, a session goes unlogged, a scale is\n" +
		"read wrong. The past is not edited to hide it, because nobody knows which\n" +
		"entry was wrong. Instead the difference is recorded as an adjustment, and\n" +
		"the account agrees with the jar again.\n\n" +
		"Storage is reconciled by default; --stash weighs the tin and --avb the\n" +
		"already vaped bud.",
	Example: "  wits reconcile wedding-cake 17.6\n" +
		"  wits reconcile wedding-cake 1.75 --stash --reason \"spilled on the desk\"\n" +
		"  wits reconcile wedding-cake 17.6 --dry-run",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		weighed, err := parseGrams(args[1])
		if err != nil {
			return err
		}
		account, err := reconcileAccount()
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		if reconcileDryRun {
			expected, difference, err := s.Recorder.Difference(args[0], account, weighed)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "%s holds %.2fg by the ledger and %.2fg on the scale: %+.2fg\n",
				account, expected, weighed, difference)
			if difference == 0 {
				fmt.Fprintln(out, "Nothing to reconcile.")
				return nil
			}
			fmt.Fprintln(out, "\nDry run. Nothing was written. Re-run without --dry-run to record it.")
			return nil
		}

		e, err := s.Recorder.Reconcile(args[0], account, weighed, reconcileReason)
		if err != nil {
			return err
		}
		direction := "into"
		if e.To == journal.External {
			direction = "out of"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] adjusted %.2fg %s %s, now %.2fg\n",
			shortHash(e.Hash), e.Grams, direction, account,
			s.Recorder.Available(e.Product, account))
		return nil
	},
}

// reconcileAccount reads which account the flags select.
func reconcileAccount() (journal.Account, error) {
	switch {
	case reconcileStash && reconcileAVB:
		return "", fmt.Errorf("weigh one account at a time: --stash or --avb")
	case reconcileStash:
		return journal.Stash, nil
	case reconcileAVB:
		return journal.AVB, nil
	default:
		return journal.Storage, nil
	}
}

func init() {
	Reconcile.Flags().BoolVar(&reconcileStash, "stash", false, "weigh the tin rather than storage")
	Reconcile.Flags().BoolVar(&reconcileAVB, "avb", false, "weigh the already vaped bud")
	Reconcile.Flags().StringVar(&reconcileReason, "reason", "", "why the amounts differ")
	Reconcile.Flags().BoolVar(&reconcileDryRun, "dry-run", false, "show the difference without recording it")
}

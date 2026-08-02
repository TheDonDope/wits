package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/record"
)

var (
	reconcileStash  bool
	reconcileAVB    bool
	reconcileReason string
	reconcileDryRun bool
)

// accountNames maps the words the command line accepts to the accounts that
// can be put on a scale. Consumed is absent for the same reason it cannot be
// reconciled: there is no jar of it to weigh.
var accountNames = map[string]journal.Account{
	"storage": journal.Storage,
	"stash":   journal.Stash,
	"avb":     journal.AVB,
}

// Reconcile is the `wits reconcile` command.
var Reconcile = &cobra.Command{
	Use:   "reconcile [account] [product] [weight]",
	Short: "Make an account agree with the scale",
	Long: "Record the difference between what the ledger believes an account holds\n" +
		"and what it actually weighs.\n\n" +
		"Ledgers drift: a little is spilled, a session goes unlogged, a scale is\n" +
		"read wrong. The past is not edited to hide it, because nobody knows which\n" +
		"entry was wrong. Instead the difference is recorded as an adjustment, and\n" +
		"the account agrees with the jar again.\n\n" +
		"Run without arguments it is interactive: pick storage or the stash, tick\n" +
		"the jars to weigh, and each is asked for in turn. Naming just the account\n" +
		"skips the first question. The full form records one jar directly.",
	Example: "  wits reconcile\n" +
		"  wits reconcile stash\n" +
		"  wits reconcile stash wcake-221 1.75\n" +
		"  wits reconcile storage wcake-221 17.6 --dry-run\n" +
		"  wits reconcile wedding-cake 17.6",
	Args:              cobra.MaximumNArgs(3),
	ValidArgsFunction: completeReconcile,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}

		flagged, err := flaggedAccount()
		if err != nil {
			return err
		}

		switch len(args) {
		case 0:
			return reconcileInteractively(cmd, s, flagged)
		case 1:
			account, ok := accountNames[args[0]]
			if !ok {
				return fmt.Errorf("%q is not an account; weigh storage, stash or avb — or give <product> <weight>", args[0])
			}
			if flagged != "" {
				return fmt.Errorf("the account is already named in the arguments; drop the flag")
			}
			return reconcileInteractively(cmd, s, account)
		case 2:
			if _, ok := accountNames[args[0]]; ok {
				return fmt.Errorf("which product? try `wits reconcile %s <product> <weight>`", args[0])
			}
			// The original form: product and weight, the account in the flags.
			if flagged == "" {
				flagged = journal.Storage
			}
			return reconcileOne(cmd, s, flagged, args[0], args[1])
		default:
			account, ok := accountNames[args[0]]
			if !ok {
				return fmt.Errorf("%q is not an account; weigh storage, stash or avb", args[0])
			}
			if flagged != "" {
				return fmt.Errorf("the account is already named in the arguments; drop the flag")
			}
			return reconcileOne(cmd, s, account, args[1], args[2])
		}
	},
}

// flaggedAccount reads which account the flags select, or "" for none.
func flaggedAccount() (journal.Account, error) {
	switch {
	case reconcileStash && reconcileAVB:
		return "", fmt.Errorf("weigh one account at a time: --stash or --avb")
	case reconcileStash:
		return journal.Stash, nil
	case reconcileAVB:
		return journal.AVB, nil
	default:
		return "", nil
	}
}

// reconcileOne records a single jar against the scale, which is the shorthand
// for the daily case of one suspicious balance.
func reconcileOne(cmd *cobra.Command, s *session, account journal.Account, ref, weight string) error {
	weighed, err := parseWeight(weight)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if reconcileDryRun {
		expected, difference, err := s.Recorder.Difference(ref, account, weighed)
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

	e, err := s.Recorder.Reconcile(ref, account, weighed, reconcileReason)
	if err != nil {
		return err
	}
	writeAdjustment(out, s, e, account)
	return nil
}

// writeAdjustment reports one recorded adjustment the way the journal will
// remember it.
func writeAdjustment(out io.Writer, s *session, e journal.Event, account journal.Account) {
	direction := "into"
	if e.To == journal.External {
		direction = "out of"
	}
	fmt.Fprintf(out, "[%s] adjusted %.2fg %s %s of %s, now %.2fg\n",
		shortHash(e.Hash), e.Grams, direction, account, e.Product,
		s.Recorder.Available(e.Product, account))
}

// jar is one product's balance in the account being weighed.
type jar struct {
	slug     string
	expected float64
}

// jarsOf lists the products with something in the account, alphabetically, so
// a session at the scale visits them in a stable order.
func jarsOf(s *session, account journal.Account) []jar {
	var jars []jar
	for _, slug := range s.State.Products() {
		if held := s.Recorder.Available(slug, account); held > 0 {
			jars = append(jars, jar{slug: slug, expected: held})
		}
	}
	return jars
}

// reconcileInteractively walks the jars of an account past the scale: pick the
// account if it was not named, tick the jars to weigh — all of them by
// default — and each asks for its reading in turn. A blank reading skips a
// jar, because walking past one is not the same as claiming it is empty.
//
// Nothing is written until every question is answered: abandoning the forms
// halfway records nothing, which is the only honest meaning of escape.
func reconcileInteractively(cmd *cobra.Command, s *session, account journal.Account) error {
	if !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("reconcile without a product is interactive and needs a terminal; " +
			"use `wits reconcile <account> <product> <weight>`")
	}
	if account == "" {
		if err := askAccount(s, &account); err != nil {
			return err
		}
	}
	jars := jarsOf(s, account)
	if len(jars) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Nothing in %s to weigh.\n", account)
		return nil
	}

	selected, expected, err := pickJars(s, account, jars)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing selected, nothing written.")
		return nil
	}

	readings, err := askReadings(s, selected, expected)
	if err != nil {
		return err
	}
	return applyReadings(cmd.OutOrStdout(), s, account, selected, readings)
}

// pickJars offers the jars as a checklist, every one of them ticked, and
// returns what stayed ticked along with each jar's ledger figure.
func pickJars(s *session, account journal.Account, jars []jar) ([]string, map[string]float64, error) {
	selected := make([]string, 0, len(jars))
	expected := map[string]float64{}
	options := make([]huh.Option[string], 0, len(jars))
	for _, j := range jars {
		selected = append(selected, j.slug)
		expected[j.slug] = j.expected
		options = append(options, huh.NewOption(
			fmt.Sprintf("%s — ledger says %.2f g", s.ProductName(j.slug), j.expected), j.slug))
	}
	// The height is the option count plus the chrome; left to its default the
	// inline form shows a one-row viewport, which reads as a single jar.
	err := formErr(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title(fmt.Sprintf("Weigh which of %s", account)).
			Description("Every jar is ticked; untick what stays on the shelf.").
			Options(options...).
			Height(min(len(options), 12) + 3).
			Value(&selected),
	)).Run())
	return selected, expected, err
}

// askReadings asks for each jar's scale reading, one question per jar in its
// own group, so the scale session reads as jar after jar rather than a wall
// of fields.
func askReadings(s *session, selected []string, expected map[string]float64) ([]string, error) {
	readings := make([]string, len(selected))
	groups := make([]*huh.Group, 0, len(selected))
	for i, slug := range selected {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title(s.ProductName(slug)).
				Description(fmt.Sprintf("On the scale, in grams — ledger says %.2f g. Blank to skip.", expected[slug])).
				Value(&readings[i]).
				Validate(optionalGrams),
		))
	}
	return readings, formErr(huh.NewForm(groups...).Run())
}

// askAccount asks which account is on the scale. AVB is only offered when
// something is actually held there.
func askAccount(s *session, account *journal.Account) error {
	choice := string(journal.Storage)
	options := []huh.Option[string]{
		huh.NewOption("Storage — sealed product", string(journal.Storage)),
		huh.NewOption("The stash — ground product", string(journal.Stash)),
	}
	if len(jarsOf(s, journal.AVB)) > 0 {
		options = append(options, huh.NewOption("AVB — already vaped bud", string(journal.AVB)))
	}
	if err := formErr(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Weigh which account").Options(options...).
			Height(len(options) + 2).Value(&choice),
	)).Run()); err != nil {
		return err
	}
	*account = journal.Account(choice)
	return nil
}

// applyReadings records the collected weights, or reports what they would
// change under --dry-run. Jars left blank are skipped, and a jar that already
// matches is said to match rather than silently passed over.
func applyReadings(out io.Writer, s *session, account journal.Account, slugs, readings []string) error {
	recorded := 0
	for i, slug := range slugs {
		reading := strings.TrimSpace(readings[i])
		if reading == "" {
			continue
		}
		weighed, err := parseWeight(reading)
		if err != nil {
			return err
		}
		if reconcileDryRun {
			expected, difference, err := s.Recorder.Difference(slug, account, weighed)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "%s: %.2fg by the ledger, %.2fg on the scale: %+.2fg\n",
				slug, expected, weighed, difference)
			recorded++
			continue
		}
		e, err := s.Recorder.Reconcile(slug, account, weighed, reconcileReason)
		if errors.Is(err, record.ErrNothingToReconcile) {
			fmt.Fprintf(out, "%s: the ledger already matches the scale (%.2f g)\n", slug, weighed)
			continue
		}
		if err != nil {
			return err
		}
		writeAdjustment(out, s, e, account)
		recorded++
	}
	if recorded == 0 {
		fmt.Fprintln(out, "Every jar was skipped or already matched; nothing to record.")
	} else if reconcileDryRun {
		fmt.Fprintln(out, "\nDry run. Nothing was written. Re-run without --dry-run to record it.")
	}
	return nil
}

// optionalGrams accepts a weight or nothing at all, since a blank reading
// means a jar was skipped.
func optionalGrams(v string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	_, err := parseWeight(v)
	return err
}

// parseWeight reads a scale reading written as "17.6", "17.6g" or "17,6 g".
// Unlike an amount, zero is allowed: an empty jar is a real reading, and
// recording it is the point of weighing.
func parseWeight(v string) (float64, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(v)), "g"))
	weighed, err := strconv.ParseFloat(strings.Replace(trimmed, ",", ".", 1), 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a weight in grams", v)
	}
	if weighed < 0 {
		return 0, fmt.Errorf("a weight cannot be negative, got %v", weighed)
	}
	return weighed, nil
}

// formErr rewords a form that could not run — usually because there is no
// terminal to ask on — into advice that still gets the job done.
func formErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return fmt.Errorf("cancelled; nothing was written")
	}
	return fmt.Errorf("cannot ask interactively (%w); use `wits reconcile <account> <product> <weight>`", err)
}

// completeReconcile offers the accounts first, then the jars of the account
// already named. The old <product> <weight> form still completes through the
// products offered beside the accounts.
func completeReconcile(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		out := []string{
			"storage\tweigh the sealed product",
			"stash\tweigh the ground product",
			"avb\tweigh the already vaped bud",
		}
		if products, _ := completeProduct("")(nil, nil, prefix); products != nil {
			out = append(out, products...)
		}
		var kept []string
		for _, o := range out {
			if strings.HasPrefix(o, prefix) {
				kept = append(kept, o)
			}
		}
		return kept, cobra.ShellCompDirectiveNoFileComp
	case 1:
		account, ok := accountNames[args[0]]
		if !ok {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		s, err := open()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []string
		for _, j := range jarsOf(s, account) {
			if strings.HasPrefix(j.slug, prefix) {
				out = append(out, fmt.Sprintf("%s\t%.2f g · %s", j.slug, j.expected, s.ProductName(j.slug)))
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func init() {
	Reconcile.Flags().BoolVar(&reconcileStash, "stash", false, "weigh the stash rather than storage")
	Reconcile.Flags().BoolVar(&reconcileAVB, "avb", false, "weigh the already vaped bud")
	Reconcile.Flags().StringVar(&reconcileReason, "reason", "", "why the amounts differ")
	Reconcile.Flags().BoolVar(&reconcileDryRun, "dry-run", false, "show the difference without recording it")
}

// Package commands implements the command line of Wits, in two grammars.
//
// The journal is written the way git commits: a repository, an append-only
// log, short verbs run as things happen (buy, grind, sesh, weigh), and log,
// show, revert and amend over it, naming entries by hash or as HEAD~n.
// Everything else is read the way kubectl reads a cluster: get lists a kind
// of thing, describe tells one in full, edit, create and delete change the
// catalogs. Branching has no meaning for a prescription and is absent.
package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// session is the workspace a command runs against, named for what a command
// does with it.
type session struct {
	*workspace.Workspace
}

// open reads the repository containing the working directory. Under
// --dry-run its recorder rehearses: every entry built and checked as it
// would be, and none written.
func open() (*session, error) {
	ws, err := workspace.Here()
	if err != nil {
		return nil, err
	}
	if dryRun {
		ws.Recorder = ws.Recorder.Dry()
	}
	return &session{Workspace: ws}, nil
}

// left is what an entry leaves in the account it draws on: the jar as it
// stands, or, rehearsed, as the entry would leave it — a rehearsal writes
// nothing, so the jar has not moved yet.
func left(s *session, e journal.Event, from journal.Account) float64 {
	have := s.Recorder.Available(e.Product, from)
	if dryRun {
		return (journal.CG(have) - journal.CG(e.Grams)).Grams()
	}
	return have
}

// parseGrams reads an amount written as "0.75", "0.75g" or "0,75 g".
func parseGrams(s string) (float64, error) {
	grams, err := ledger.ParseGrams(s)
	if err != nil {
		return 0, err
	}
	if grams <= 0 {
		return 0, fmt.Errorf("amount must be positive, got %v", grams)
	}
	return grams, nil
}

// parseDate reads a --date flag. An empty value means now, so that the common
// case of logging as you go needs no flag at all.
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	for _, layout := range []string{time.DateOnly, "2006-01-02 15:04", time.RFC3339} {
		if at, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return at, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q is not a date, expected YYYY-MM-DD", s)
}

// shortHash abbreviates an event hash the way git abbreviates a commit. A
// rehearsed entry has none: it was never chained, so it says so.
func shortHash(h string) string {
	if h == "" {
		return "rehearsed"
	}
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

// plural renders a count with its noun. The first day of a cycle is the one a
// new repository spends all of its time in, so "left over 1 days" is the
// reading most likely to be someone's first.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// completeProduct offers the products that hold something in the given account,
// so tab completion on a grind offers what can actually be ground and not four
// years of empty jars.
//
// Each slug is offered with its display name after a tab, which is the shape a
// shell shows as a description beside the candidate.
func completeProduct(account journal.Account) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		s, err := open()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []string
		for _, slug := range s.State.Products() {
			if !strings.HasPrefix(slug, prefix) {
				continue
			}
			have := s.Recorder.Available(slug, account)
			if account != "" && have <= 0 {
				continue
			}
			out = append(out, fmt.Sprintf("%s\t%.2f g · %s", slug, have, s.ProductName(slug)))
		}
		sort.Strings(out)
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeEntry offers the recent entries by their abbreviated hash, for the
// commands that take one.
func completeEntry(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	s, err := open()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	events := s.State.Events
	var out []string
	for i := len(events) - 1; i >= 0 && len(out) < 40; i-- {
		e := events[i]
		short := shortHash(e.Hash)
		if !strings.HasPrefix(short, prefix) {
			continue
		}
		out = append(out, fmt.Sprintf("%s\t%s %.2f g %s on %s",
			short, e.Type, e.Grams, e.Product, e.OccurredAt.Format(time.DateOnly)))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// entries counts entries, which plural would spell "entrys".
func entries(n int) string {
	if n == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", n)
}

// percent formats a share as a percentage, or a dash when there is nothing to
// compare against.
//
// Both sides are of one scope — a fill's remaining over the same fill's
// dispensed — and the type parameter holds them to it: a fill's grams over
// the shelf's does not compile.
func percent[T ~float64](have, of T) string {
	if of <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", have/of*100)
}

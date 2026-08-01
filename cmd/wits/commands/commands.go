// Package commands implements the git-shaped command surface of Wits.
//
// Only the parts of git's vocabulary that have a real referent here are
// borrowed: a repository, an append-only log of commits, and a working state
// derived from it. Branching and merging have no meaning for a prescription
// and are deliberately absent.
package commands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// session is the workspace a command runs against, named for what a command
// does with it.
type session struct {
	*workspace.Workspace
}

// open reads the repository containing the working directory.
func open() (*session, error) {
	ws, err := workspace.Here()
	if err != nil {
		return nil, err
	}
	return &session{Workspace: ws}, nil
}

// parseGrams reads an amount written as "0.75", "0.75g" or "0,75 g".
func parseGrams(s string) (float64, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(s)), "g"))
	grams, err := strconv.ParseFloat(strings.Replace(trimmed, ",", ".", 1), 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount in grams", s)
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

// shortHash abbreviates an event hash the way git abbreviates a commit.
func shortHash(h string) string {
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

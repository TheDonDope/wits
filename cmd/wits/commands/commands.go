// Package commands implements the git-shaped command surface of Wits.
//
// Only the parts of git's vocabulary that have a real referent here are
// borrowed: a repository, an append-only log of commits, and a working state
// derived from it. Branching and merging have no meaning for a prescription
// and are deliberately absent.
package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

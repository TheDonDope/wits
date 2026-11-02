// Package commands holds the naming of entries here: by hash, abbreviated
// or not, or counted back from the newest the way git counts commits.
package commands

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/journal"
)

// resolve finds the entry a revision names: a hash or its first letters, as
// wits log prints them, or HEAD — the entry written last — with ~n for the
// nth before it, or ^ for the one before. @ is HEAD, as in git.
//
// Counted in the order entries were written, the journal's own: the entry
// written last is HEAD even when it is backdated behind others.
func resolve(s *session, rev string) (journal.Event, error) {
	back, ok, err := countBack(rev)
	if err != nil {
		return journal.Event{}, err
	}
	if !ok {
		return s.Recorder.Find(rev)
	}
	written := slices.Clone(s.State.Events)
	slices.SortFunc(written, func(a, b journal.Event) int { return b.Seq - a.Seq })
	if len(written) == 0 {
		return journal.Event{}, fmt.Errorf("no entries yet")
	}
	if back >= len(written) {
		return journal.Event{}, fmt.Errorf("%s is before the first entry; the journal holds %s", rev, entries(len(written)))
	}
	return written[back], nil
}

// countBack reads HEAD, HEAD~n, HEAD^^ and their @ forms as a count back
// from the newest entry; ok is false for anything else, which is a hash.
func countBack(rev string) (int, bool, error) {
	rest, ok := strings.CutPrefix(rev, "HEAD")
	if !ok {
		if rest, ok = strings.CutPrefix(rev, "@"); !ok {
			if rev != "" {
				return 0, false, nil
			}
		}
	}
	back := 0
	for rest != "" {
		switch {
		case rest[0] == '^':
			back++
			rest = rest[1:]
		case rest[0] == '~':
			digits := strings.TrimLeft(rest[1:], "0123456789")
			n := 1
			if count := rest[1 : len(rest)-len(digits)]; count != "" {
				var err error
				if n, err = strconv.Atoi(count); err != nil {
					return 0, false, fmt.Errorf("%q is not a revision", rev)
				}
			}
			back += n
			rest = digits
		default:
			return 0, false, fmt.Errorf("%q is not a revision; name an entry by its hash, or as HEAD~n", rev)
		}
	}
	return back, true, nil
}

// completeRev offers HEAD and the recent entries, for the commands that
// take one.
func completeRev(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	out, directive := completeEntry(cmd, args, prefix)
	if len(args) == 0 && strings.HasPrefix("HEAD", prefix) {
		out = append([]string{"HEAD\tthe entry written last"}, out...)
	}
	return out, directive
}

// Package commands holds `wits rate` here: what a session did, rated afterwards.
package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/spf13/cobra"
)

var rateNote string

// Rate is the `wits rate` command.
var Rate = &cobra.Command{
	Use:   "rate <session|last> <dimension=score>...",
	Short: "Rate what a session did",
	Long: "Rate a session afterwards, 1 (not at all) to 5 (very well), on any of:\n" +
		"overall, pain, sleep, calm, mood, focus, appetite. Rate it again and the\n" +
		"newer rating stands; the effect log keeps both.\n\n" +
		"The session is named by its hash, as wits log shows it, or as last.",
	Example: "  wits rate last overall=4 sleep=5\n" +
		"  wits rate 8297238 pain=4 calm=3 --note \"took the edge off\"",
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		session := args[0]
		if session == "last" {
			if session = lastSession(s.State.Standing); session == "" {
				return fmt.Errorf("no session to rate yet")
			}
		}
		scores, err := parseScores(args[1:])
		if err != nil {
			return err
		}
		r, err := s.Recorder.Rate(session, scores, rateNote, time.Now())
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] rated session %s: %s\n",
			shortHash(r.Hash), shortHash(r.Session), effects.Summary(r.Scores))
		return nil
	},
}

func init() {
	Rate.Flags().StringVar(&rateNote, "note", "", "a line about how it went")
}

// lastSession is the newest session that stands, by when it happened: the
// last session among the fold's standing entries.
func lastSession(standing []journal.Event) string {
	for i := len(standing) - 1; i >= 0; i-- {
		if standing[i].Type == journal.Sesh {
			return standing[i].Hash
		}
	}
	return ""
}

// parseScores reads dimension=score pairs.
func parseScores(args []string) (map[effects.Dimension]int, error) {
	scores := map[effects.Dimension]int{}
	for _, a := range args {
		d, v, ok := strings.Cut(a, "=")
		n, err := strconv.Atoi(v)
		if !ok || err != nil {
			return nil, fmt.Errorf("%q is not a score; write it as dimension=1 to 5, like sleep=4", a)
		}
		scores[effects.Dimension(strings.ToLower(d))] = n
	}
	return scores, nil
}

// Package commands holds `wits rate` here: what a session did, rated
// afterwards, and the strain rated as a whole.
package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
)

var (
	rateNote  string
	rateAgain string
	rateClear bool
)

// Rate is the `wits rate` command, which only groups: a session is rated,
// or a strain.
var Rate = &cobra.Command{
	Use:   "rate <session|strain>",
	Short: "Rate what a session did, or a strain as a whole",
	Args:  cobra.NoArgs,
}

var rateSession = &cobra.Command{
	Use:     "session [<rev>] <dimension=score>...",
	Aliases: []string{"sesh"},
	Short:   "Rate what a session did, 1 to 5; without a session, the last",
	Long: "Rate a session afterwards, 1 (not at all) to 5 (very well), on any of:\n" +
		"overall, pain, sleep, calm, mood, focus, appetite. Rate it again and the\n" +
		"newer rating stands; the effect log keeps both.\n\n" +
		"The session is named by its hash, as wits log shows it, or as HEAD~n;\n" +
		"named not at all, it is the last session that stands.",
	Example: "  wits rate session overall=4 sleep=5\n" +
		"  wits rate session 8297238 pain=4 calm=3 --note \"took the edge off\"",
	Args:              cobra.MinimumNArgs(1),
	Annotations:       rehearsable(),
	ValidArgsFunction: completeRev,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		session := lastSession(s.State.Standing)
		if !strings.Contains(args[0], "=") {
			e, err := resolve(s, args[0])
			if err != nil {
				return err
			}
			session, args = e.Hash, args[1:]
		}
		if session == "" {
			return fmt.Errorf("no session to rate yet")
		}
		if len(args) == 0 {
			return fmt.Errorf("rate it on something: overall=4, sleep=5")
		}
		scores, err := parseScores(args)
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

var rateStrain = &cobra.Command{
	Use:     "strain <product> [<dimension=score>...]",
	Aliases: []string{"product"},
	Short:   "Rate a strain as a whole: overall, taste and aroma, 1 to 5",
	Long: "Rate the strain as a whole, apart from any one session: overall, taste\n" +
		"and aroma, 1 to 5. --again says whether you would have it again, --note\n" +
		"adds a line, and --clear takes the rating away. Rated again, the newer\n" +
		"rating stands.",
	Example: "  wits rate strain wedding overall=4 taste=5 --again yes --note \"lemon, then pepper\"\n" +
		"  wits rate strain wedding --clear",
	Args:              cobra.MinimumNArgs(1),
	Annotations:       rehearsable(),
	ValidArgsFunction: completeProduct(""),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		p, err := s.Products.Find(args[0])
		if err != nil {
			return err
		}
		var rating *catalog.StrainRating
		if !rateClear {
			r, err := catalog.ParseRating(strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			r.Again, r.Note = catalog.Again(strings.ToLower(rateAgain)), rateNote
			rating = &r
		}
		if p, err = s.Recorder.RateStrain(p.Slug, rating, time.Now()); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", p.Name, ratedLine(p.Rating))
		return nil
	},
}

func init() {
	rateSession.Flags().StringVar(&rateNote, "note", "", "a line about how it went")
	rateStrain.Flags().StringVar(&rateNote, "note", "", "a line about the strain")
	rateStrain.Flags().StringVar(&rateAgain, "again", "", "whether you would have it again: yes or no")
	rateStrain.Flags().BoolVar(&rateClear, "clear", false, "take the strain's rating away")
	Rate.AddCommand(rateSession, rateStrain)
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

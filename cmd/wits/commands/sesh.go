package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/spf13/cobra"
)

var (
	seshDate   string
	seshDevice string
	seshTemp   int
	seshNote   string
)

// Sesh is the `wits sesh` command.
var Sesh = &cobra.Command{
	Use:     "sesh <product> <amount>",
	Aliases: []string{"session"},
	Short:   "Record a session, drawing from the stash",
	Long: "Record a session: ground product comes out of that product's stash and\n" +
		"goes through a device. What is left of it becomes AVB, which is weighed\n" +
		"and credited separately.\n\n" +
		"With a temperature, this also reports which compounds that setting is\n" +
		"hot enough to release, and warns when it is hot enough to produce\n" +
		"benzene.",
	Example: "  wits sesh wedding-cake 0.3 --device volcano --temp 185\n" +
		"  wits sesh lemon 0.2 --date 2026-07-29",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeProduct(journal.Stash),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		grams, err := parseGrams(args[1])
		if err != nil {
			return err
		}
		at, err := parseDate(seshDate)
		if err != nil {
			return err
		}
		e, err := s.Recorder.Session(args[0], grams, at, seshDevice, seshTemp, seshNote)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "[%s] sesh %.2fg %s, %.2fg left in the stash\n",
			shortHash(e.Hash), e.Grams, e.Product, s.Recorder.Available(e.Product, journal.Stash))
		if e.Temperature > 0 {
			writeReleased(out, e.Temperature)
		}
		return nil
	},
}

// writeReleased reports what a temperature is hot enough to volatilise.
func writeReleased(out io.Writer, celsius int) {
	released := catalog.ReleasedAt(celsius)
	if len(released) == 0 {
		fmt.Fprintf(out, "At %d°C nothing has reached its boiling point yet.\n", celsius)
		return
	}
	names := make([]string, 0, len(released))
	for _, r := range released {
		names = append(names, fmt.Sprintf("%s (%d°C)", r.Name, r.BoilingPoint))
	}
	fmt.Fprintf(out, "At %d°C this releases %s.\n", celsius, strings.Join(names, ", "))

	for _, h := range catalog.Hazards(celsius) {
		fmt.Fprintf(out, "⚠️  %d°C is at or above the %d°C boiling point of %s (%s).\n",
			celsius, h.BoilingPoint, h.Name, strings.Join(h.Effects, ", "))
	}
}

func init() {
	Sesh.Flags().StringVar(&seshDate, "date", "", "when the session was, defaults to now")
	Sesh.Flags().StringVar(&seshDevice, "device", "", "the device used")
	Sesh.Flags().IntVar(&seshTemp, "temp", 0, "the temperature in degrees Celsius")
	Sesh.Flags().StringVar(&seshNote, "note", "", "a note to keep with the entry")
}

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
			if session = lastSession(s.State.Events); session == "" {
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
			shortHash(r.Hash), shortHash(r.Session), describeScores(r.Scores))
		return nil
	},
}

func init() {
	Rate.Flags().StringVar(&rateNote, "note", "", "a line about how it went")
}

// lastSession is the newest session that stands, by when it happened.
func lastSession(events []journal.Event) string {
	undone := map[string]bool{}
	for _, e := range events {
		if e.Reverts != "" {
			undone[e.Reverts] = true
		}
	}
	for i := len(events) - 1; i >= 0; i-- {
		if e := events[i]; e.Type == journal.Sesh && !undone[e.Hash] {
			return e.Hash
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

// describeScores writes scores in the order the dimensions are asked.
func describeScores(scores map[effects.Dimension]int) string {
	var out []string
	for _, d := range effects.Dimensions {
		if v, ok := scores[d]; ok {
			out = append(out, fmt.Sprintf("%s %d", d, v))
		}
	}
	return strings.Join(out, ", ")
}

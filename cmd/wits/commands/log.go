package commands

import (
	"fmt"
	"io"
	"slices"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/spf13/cobra"
)

var (
	logOneline bool
	logProduct string
	logCurrent bool
	logLimit   int
)

// Log is the `wits log` command.
var Log = &cobra.Command{
	Use:   "log",
	Short: "Show the journal, newest first",
	Long: "Show the events in the journal, newest first, the way `git log` shows\n" +
		"commits. Nothing here can be edited: a mistake is corrected by\n" +
		"appending a compensating event.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}

		events := s.State.Events
		if logCurrent {
			if cycle := s.Cycle(); cycle != nil {
				events = cycle.Events
			} else {
				events = nil
			}
		}
		if logProduct != "" {
			product, err := s.Products.Find(logProduct)
			if err != nil {
				return err
			}
			var filtered []journal.Event
			for _, e := range events {
				if e.Product == product.Slug {
					filtered = append(filtered, e)
				}
			}
			events = filtered
		}

		out := cmd.OutOrStdout()
		if len(events) == 0 {
			fmt.Fprintln(out, "No events yet.")
			return nil
		}

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		shown := 0
		for i := len(events) - 1; i >= 0; i-- {
			if logLimit > 0 && shown == logLimit {
				break
			}
			e := events[i]
			if logOneline {
				fmt.Fprintf(w, "%s\t%s\t%s\t%.2fg\t%s\n",
					shortHash(e.Hash), e.OccurredAt.Format(time.DateOnly), e.Type, e.Grams, e.Product)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%-11s\t%.2fg\t%s\t%s -> %s\n",
					shortHash(e.Hash), e.OccurredAt.Format(time.DateOnly), e.Type, e.Grams, e.Product, e.From, e.To)
			}
			shown++
		}
		return w.Flush()
	},
}

func init() {
	Log.Flags().BoolVar(&logOneline, "oneline", false, "one compact line per event")
	Log.Flags().StringVar(&logProduct, "product", "", "only events for this product")
	Log.Flags().BoolVar(&logCurrent, "current", false, "only events in the current cycle")
	Log.Flags().IntVarP(&logLimit, "max-count", "n", 0, "show at most this many events")
}

// Show is the `wits show` command.
var Show = &cobra.Command{
	Use:   "show [entry]",
	Short: "Show one entry in full",
	Long: "Show one entry the way `git show` shows a commit: its place in the chain,\n" +
		"when it happened and when it was written down, what it moved and what\n" +
		"each account held before and after, the fill it fell in, what corrects\n" +
		"it or what it corrects, and — for a session — how it was rated.\n\n" +
		"The entry is named by its hash, as wits log shows it. With none, or\n" +
		"with last, the entry written most recently.",
	Example: "  wits show\n  wits show 8297238",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		e, err := entryNamed(s, args)
		if err != nil {
			return err
		}
		show(cmd.OutOrStdout(), s, e)
		return nil
	},
}

// entryNamed finds the entry an argument names, or the newest one written.
func entryNamed(s *session, args []string) (journal.Event, error) {
	if len(args) == 1 && args[0] != "last" {
		return s.Recorder.Find(args[0])
	}
	var newest *journal.Event
	for i, e := range s.State.Events {
		if newest == nil || e.Seq > newest.Seq {
			newest = &s.State.Events[i]
		}
	}
	if newest == nil {
		return journal.Event{}, fmt.Errorf("no entries yet")
	}
	return *newest, nil
}

// show writes one entry in full.
func show(out io.Writer, s *session, e journal.Event) {
	line := func(label, format string, a ...any) {
		fmt.Fprintf(out, "%-11s %s\n", label, fmt.Sprintf(format, a...))
	}
	fmt.Fprintf(out, "%s %s\n", e.Type, e.Hash)
	prev := "none — the first entry"
	if e.Prev != "" {
		prev = shortHash(e.Prev)
	}
	fmt.Fprintf(out, "entry %d, chained onto %s\n\n", e.Seq, prev)

	line("Occurred", "%s", e.OccurredAt.Format("Mon 02 Jan 2006 15:04"))
	line("Recorded", "%s%s", e.RecordedAt.Format("Mon 02 Jan 2006 15:04"), lateness(e))
	if e.Product != "" {
		line("Product", "%s (%s)", s.ProductName(e.Product), e.Product)
	}
	line("Moved", "%.2f g, %s → %s", e.Grams, e.From, e.To)
	// The jar's figures, the product's whole lifetime — not the fill's share,
	// which is what wits status shows — so the label says so.
	for i, m := range movement(s.State.Events, e) {
		label := ""
		if i == 0 {
			label = "In the jar"
		}
		line(label, "%s", m)
	}
	if e.Device != "" || e.Temperature > 0 {
		line("Device", "%s", deviceLine(s, e))
	}
	if e.Note != "" {
		line("Note", "%s", e.Note)
	}
	showContext(s, e, line)
}

// showContext writes what surrounds an entry: its fill, its corrections, and
// its rating.
func showContext(s *session, e journal.Event, line func(string, string, ...any)) {
	for _, c := range s.State.Cycles {
		if slices.ContainsFunc(c.Events, func(x journal.Event) bool { return x.Hash == e.Hash }) {
			line("Fill", "%d, opened %s", c.Seq+1, c.Start.Format(time.DateOnly))
		}
	}
	if e.Reverts != "" {
		if orig, err := s.Recorder.Find(e.Reverts); err == nil {
			line("Undoes", "%s, %s %.2f g of %s on %s", shortHash(orig.Hash), orig.Type, orig.Grams,
				s.ProductName(orig.Product), orig.OccurredAt.Format(time.DateOnly))
		} else {
			line("Undoes", "%s", shortHash(e.Reverts))
		}
	}
	if fix := s.Recorder.RevertOf(e.Hash); fix != nil {
		line("Undone", "by %s on %s", shortHash(fix.Hash), fix.OccurredAt.Format(time.DateOnly))
	}
	if r, ok := effects.Latest(s.Ratings)[e.Hash]; ok {
		rated := effects.Summary(r.Scores)
		if r.Note != "" {
			rated += fmt.Sprintf(" — %q", r.Note)
		}
		line("Rated", "%s", rated)
		if n := countRatings(s.Ratings, e.Hash); n > 1 {
			line("", "  rated %s; the newest stands", plural(n, "time"))
		}
	} else if e.Type == journal.Sesh && s.Recorder.RevertOf(e.Hash) == nil {
		line("Rated", "not yet — wits rate %s overall=…", shortHash(e.Hash))
	}
}

// lateness says how long after the fact an entry was written down, when that
// was more than an hour.
func lateness(e journal.Event) string {
	late := e.RecordedAt.Sub(e.OccurredAt)
	switch {
	case late >= 48*time.Hour:
		return fmt.Sprintf(" — %d days later", int(late.Hours()/24))
	case late >= time.Hour:
		return fmt.Sprintf(" — %d hours later", int(late.Hours()))
	case late <= -time.Hour:
		return " — before it happened"
	}
	return ""
}

// movement is what each account the entry touched held of its product just
// before it and just after, in the order the fold replays: the jar's scope.
func movement(events []journal.Event, e journal.Event) []string {
	i := slices.IndexFunc(events, func(x journal.Event) bool { return x.Hash == e.Hash })
	if i < 0 || e.Product == "" {
		return nil
	}
	before := ledger.Fold(events[:i]).Balances[e.Product]
	after := ledger.Fold(events[:i+1]).Balances[e.Product]
	if before == nil {
		before = &ledger.Balance{}
	}
	if after == nil {
		return nil
	}
	var out []string
	for _, account := range []journal.Account{e.From, e.To} {
		if account == journal.External {
			continue
		}
		out = append(out, fmt.Sprintf("%-8s %6.2f g → %6.2f g", account, before.In(account), after.In(account)))
	}
	return out
}

// deviceLine names the device a session went through, and how hot.
func deviceLine(s *session, e journal.Event) string {
	name := e.Device
	if s.Devices != nil {
		if d, err := s.Devices.Find(e.Device); err == nil {
			name = d.Name
		}
	}
	switch {
	case name == "":
		return fmt.Sprintf("%d°C", e.Temperature)
	case e.Temperature > 0:
		return fmt.Sprintf("%s at %d°C", name, e.Temperature)
	}
	return name
}

func countRatings(ratings []effects.Rating, session string) int {
	n := 0
	for _, r := range ratings {
		if r.Session == session {
			n++
		}
	}
	return n
}

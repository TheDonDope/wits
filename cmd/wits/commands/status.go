package commands

import (
	"fmt"
	"io"
	"math"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/workspace"
	"github.com/spf13/cobra"
)

// Status is the `wits status` command.
var Status = &cobra.Command{
	Use:   "status",
	Short: "Show what is left and how long it will last",
	Long: "Show the working state derived from the journal: how much of each\n" +
		"product is in storage and in its stash, how far through the current cycle\n" +
		"you are, and how long the remainder will last at the observed rate.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		warnChain(cmd.ErrOrStderr(), s.Workspace)
		writeStatus(cmd.OutOrStdout(), s.State, s.Cycle())
		writeUpcoming(cmd.OutOrStdout(), s.State)
		return nil
	},
}

// warnChain says so when the journal's hash chain does not verify.
func warnChain(out io.Writer, ws *workspace.Workspace) {
	if ws.Chain != nil {
		fmt.Fprintln(out, "⚠ the journal was changed outside wits; wits fsck says where")
	}
}

// Fsck is the `wits fsck` command.
var Fsck = &cobra.Command{
	Use:   "fsck",
	Short: "Check the journal and the catalogs for damage",
	Long: "Check that the journal's hash chain verifies, that no entry takes grams\n" +
		"that were not in the jar at the time, that every entry names a product\n" +
		"and device the catalogs know, that every correction corrects something,\n" +
		"and that every gram that came in is either held or went out.\n\n" +
		"Nothing is changed. A problem is reported, and the command fails.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		problems := fsck(s.Workspace)
		fmt.Fprintf(out, "%s, %s, %s\n", entries(len(s.State.Events)),
			plural(len(s.Products.Products), "product"), plural(len(s.Devices.Devices), "device"))
		fmt.Fprintln(out, massLine(s.State))
		if len(problems) == 0 {
			fmt.Fprintln(out, "✓ the chain verifies, and nothing is out of place")
			return nil
		}
		for _, p := range problems {
			fmt.Fprintln(out, "✗ "+p)
		}
		return fmt.Errorf("%s found", plural(len(problems), "problem"))
	},
}

// fsck lists what is wrong with a repository, chain first.
func fsck(ws *workspace.Workspace) []string {
	var problems []string
	if ws.Chain != nil {
		problems = append(problems, ws.Chain.Error())
	}
	problems = append(problems, effectProblems(ws)...)
	// Written past the recorder — imported, restored, edited by hand — an
	// entry can take grams that were not in the jar at its moment.
	for _, short := range ledger.Shortfalls(ws.State.Events) {
		problems = append(problems, shortHash(short.Event.Hash)+" "+short.String())
	}
	// Recorded before amounts were held to centigrams: shown and bundled as
	// the nearest centigram, so a restore would not give back this journal.
	for _, e := range ws.State.Events {
		if !journal.Whole(e.Grams) {
			problems = append(problems, fmt.Sprintf("%s is %v g, finer than a centigram; a bundle keeps %.2f g",
				shortHash(e.Hash), e.Grams, e.Grams))
		}
	}
	return append(problems, dangling(ws)...)
}

// effectProblems is what is wrong with the effect log: a chain that does not
// verify, and ratings of anything that is not a session the journal holds.
func effectProblems(ws *workspace.Workspace) []string {
	var problems []string
	if ws.EffectsChain != nil {
		problems = append(problems, ws.EffectsChain.Error())
	}
	sessions := map[string]bool{}
	for _, e := range ws.State.Events {
		if e.Type == journal.Sesh {
			sessions[e.Hash] = true
		}
	}
	for _, r := range ws.Ratings {
		if !sessions[r.Session] {
			problems = append(problems, fmt.Sprintf("rating %s rates %s, which is not a session in the journal",
				shortHash(r.Hash), shortHash(r.Session)))
		}
	}
	return problems
}

// dangling lists entries that point at something that is not there: a
// product or device missing from its catalog — one line for each, however
// many entries name it — or a correction of nothing.
func dangling(ws *workspace.Workspace) []string {
	var problems []string
	hashes := map[string]bool{}
	for _, e := range ws.State.Events {
		hashes[e.Hash] = true
	}
	products, devices := map[string]int{}, map[string]int{}
	for _, e := range ws.State.Events {
		if e.Product != "" && ws.Products.Exact(e.Product) == nil {
			products[e.Product]++
		}
		if _, err := ws.Devices.Find(e.Device); e.Device != "" && err != nil {
			devices[e.Device]++
		}
		if e.Reverts != "" && !hashes[e.Reverts] {
			problems = append(problems, fmt.Sprintf("%s corrects %s, which is not in the journal", shortHash(e.Hash), shortHash(e.Reverts)))
		}
	}
	problems = append(problems, missing(products, "product")...)
	return append(problems, missing(devices, "device")...)
}

// missing describes the slugs entries name that a catalog lacks.
func missing(named map[string]int, catalog string) []string {
	slugs := make([]string, 0, len(named))
	for slug := range named {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	out := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		out = append(out, fmt.Sprintf("%s is named by %s but is not in the %s catalog",
			slug, entries(named[slug]), catalog))
	}
	return out
}

// entries counts entries, which plural would spell "entrys".
func entries(n int) string {
	if n == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", n)
}

// massLine adds the grams up straight from the entries, apart from the fold:
// what came in from outside, what went out, and where the rest is — in the
// jars, or through a vaporizer and not yet weighed as AVB. The first less the
// second must be the rest, or a gram has gone missing. It used to call all of
// the rest "held", vaped grams included.
func massLine(state *ledger.State) string {
	var in, out, jars, vaped journal.Centigrams
	for _, e := range state.Applied() {
		if e.From == journal.External && e.To != journal.External {
			in += journal.CG(e.Grams)
		}
		if e.To == journal.External && e.From != journal.External {
			out += journal.CG(e.Grams)
		}
	}
	for _, b := range state.Balances {
		jars += journal.CG(b.Storage) + journal.CG(b.Stash) + journal.CG(b.AVB)
		vaped += journal.CG(b.Consumed)
	}
	line := fmt.Sprintf("%.2f g came in, %.2f g went out; %.2f g in the jars", in.Grams(), out.Grams(), jars.Grams())
	if vaped != 0 {
		line += fmt.Sprintf(", %.2f g through a vaporizer", vaped.Grams())
	}
	if missing := in - out - jars - vaped; missing != 0 {
		return line + fmt.Sprintf(" — %.2f g unaccounted for", missing.Grams())
	}
	return line + " — every gram accounted for"
}

// writeStatus renders the state as a table.
func writeStatus(out io.Writer, state *ledger.State, cycle *ledger.Cycle) {
	if cycle == nil {
		fmt.Fprintln(out, "No cycle in progress. Storage is empty.")
		if len(state.Cycles) > 0 {
			last := state.Cycles[len(state.Cycles)-1]
			fmt.Fprintf(out, "The last cycle ran %s to %s.\n",
				last.Start.Format(time.DateOnly), last.End.Format(time.DateOnly))
		}
		fmt.Fprintln(out, "\nRecord your next fill with `wits buy`.")
		return
	}

	stats := ledger.Summarise(cycle.Events)
	fmt.Fprintf(out, "On cycle %d, opened %s (day %d)\n\n",
		cycle.Seq+1, cycle.Start.Format(time.DateOnly), daysSince(cycle.Start))

	// Only the products of this fill. Four years of history holds every product
	// ever dispensed, and listing all of them buries the three that are on the
	// shelf right now.
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PRODUCT\tSTORAGE\tSTASH\tAVB\tLEFT")
	for _, product := range cycle.Products {
		b := state.Balances[product]
		if b == nil {
			continue
		}
		// The cycle's own share of the jar, against what its fill dispensed.
		// A jar refilled before it was empty holds an older cycle's grams
		// too, and those stand on the older cycle's account.
		// The stash and AVB the same way: as far as they are this fill's.
		share := state.ShareOf(cycle, product)
		stash, avb := state.FillGround(cycle, product)
		fmt.Fprintf(w, "%s\t%.2fg\t%.2fg\t%.2fg\t%s\n",
			product, share, stash, avb, percent(share, cycle.PurchasedOf(product)))
	}
	remaining := state.FillOnShelf(cycle)
	fmt.Fprintf(w, "\t\t\t\t\n")
	fmt.Fprintf(w, "total\t%.2fg\t\t\t%s\n", remaining, percent(remaining, cycle.Purchased))
	w.Flush()

	fmt.Fprintf(out, "\n%.2fg of %.2fg left over %s, %d of them with an entry\n",
		remaining, cycle.Purchased, plural(stats.ElapsedDays, "day"), stats.ActiveDays)
	if carried, jars, open := state.CarriedOnShelf(cycle); carried > 0 {
		fmt.Fprintf(out, "%.2fg more in %s, %s still open\n",
			carried, plural(jars, "older jar"), plural(open, "earlier cycle"))
	}
	if stats.PerActiveDay > 0 {
		fmt.Fprintf(out, "%.2fg per active day, %.2fg median, %.2fg per elapsed day\n",
			stats.PerActiveDay, stats.MedianPerDay, stats.PerElapsedDay)
		fmt.Fprintf(out, "About %s' worth at that rate\n",
			plural(int(math.Round(stats.DaysWorth(float64(remaining)))), "day"))
	} else {
		fmt.Fprintln(out, "Nothing ground yet this cycle, so there is no rate to extrapolate from")
	}
}

// writeUpcoming lists the fills dated ahead, which the figures above leave
// out until they arrive — less any undone, the way the journal leaves out a
// corrected entry and its correction.
func writeUpcoming(out io.Writer, state *ledger.State) {
	undone := record.Reverted(state.Events)
	for _, e := range state.Upcoming() {
		if e.Type == journal.Purchase && !undone[e.Hash] {
			fmt.Fprintf(out, "%.2fg of %s arriving %s, not counted until then\n",
				e.Grams, e.Product, e.OccurredAt.Format(time.DateOnly))
		}
	}
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

// daysSince returns the number of days from t until today, counting today. It
// counts calendar days, the way the interface does, so the two never disagree
// about what day of the cycle it is around midnight.
func daysSince(t time.Time) int {
	now := time.Now()
	ty, tm, td := t.Date()
	ny, nm, nd := now.Date()
	from := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	to := time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC)
	return int(to.Sub(from).Hours()/24) + 1
}

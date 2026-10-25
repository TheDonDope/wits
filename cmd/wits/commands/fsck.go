// Package commands: `wits fsck`, the journal, its logs and the catalogs checked for damage.

package commands

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/workspace"
	"github.com/spf13/cobra"
)

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
	problems = append(problems, ahead(ws)...)
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

// ahead lists entries dated after the repository was opened. The recorder
// refuses them now; a journal written before it did may still hold one, and
// the fold counts it as though it had happened.
func ahead(ws *workspace.Workspace) []string {
	var out []string
	for _, e := range ws.State.Events {
		if e.OccurredAt.After(ws.OpenedAt) {
			out = append(out, fmt.Sprintf("%s, %s %.2f g of %s, is dated %s — it is counted as though it had happened",
				shortHash(e.Hash), e.Type, e.Grams, e.Product, e.OccurredAt.Format(time.DateOnly)))
		}
	}
	return out
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
	problems = append(problems, priceProblems(ws)...)
	return append(problems, sealProblems(ws)...)
}

// sealProblems is any kept seal that does not verify, that seals a head the
// journal does not hold where it says, or whose timestamp is of something
// else.
func sealProblems(ws *workspace.Workspace) []string {
	kept, err := seal.Store{Dir: ws.Repo.SealsDir()}.All()
	if err != nil {
		return []string{err.Error()}
	}
	events, err := ws.Journal().Events()
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	for _, f := range kept {
		s, err := seal.Parse(f.Text)
		if err == nil {
			err = s.Verify()
		}
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("seal %s: %v", f.Name, err))
		case s.Entries < 1 || s.Entries > len(events) || events[s.Entries-1].Hash != s.Head:
			problems = append(problems, fmt.Sprintf("seal %s seals %s at entry %d, which the journal does not hold there",
				f.Name, shortHash(s.Head), s.Entries))
		case len(f.OTS) > 0:
			if p, err := seal.Describe(f.OTS); err != nil || !bytes.Equal(p.Digest, digestOf(s)) {
				problems = append(problems, fmt.Sprintf("seal %s: its timestamp is not of it", f.Name))
			}
		}
	}
	return problems
}

func digestOf(s seal.Seal) []byte {
	d := s.Digest()
	return d[:]
}

// priceProblems is the price log's chain, and any price that names no
// purchase in the journal.
func priceProblems(ws *workspace.Workspace) []string {
	var problems []string
	if ws.PricesChain != nil {
		problems = append(problems, ws.PricesChain.Error())
	}
	purchases := map[string]bool{}
	for _, e := range ws.State.Events {
		if e.Type == journal.Purchase {
			purchases[e.Hash] = true
		}
	}
	for _, p := range ws.Prices {
		if !purchases[p.Purchase] {
			problems = append(problems, fmt.Sprintf("price %s is for %s, which is not a purchase in the journal",
				shortHash(p.Hash), shortHash(p.Purchase)))
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

// massLine adds the grams up straight from the entries, apart from the fold:
// what came in from outside, what went out, and where the rest is — in the
// jars, or through a vaporizer and not yet weighed as AVB. The first less the
// second must be the rest, or a gram has gone missing. It used to call all of
// the rest "held", vaped grams included.
func massLine(state *ledger.State) string {
	var in, out, jars, vaped journal.Centigrams
	for _, e := range state.Events {
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

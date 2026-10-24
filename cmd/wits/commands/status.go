package commands

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/seal"
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
		warnClashes(cmd.ErrOrStderr(), s.State)
		writeStatus(cmd.OutOrStdout(), s.State, s.Cycle())
		if plan, ok := ledger.PlanRefill(s.Cycle(), s.OpenedAt, s.Repo.Config.Lead()); ok {
			fmt.Fprintln(cmd.OutOrStdout(), orderLine(plan, s.OpenedAt))
		}
		return nil
	},
}

var (
	refillLead int
	refillICS  string
)

func init() {
	Refill.Flags().IntVar(&refillLead, "lead", repo.DefaultLead, "days the pharmacy takes; kept for next time")
	Refill.Flags().StringVar(&refillICS, "ics", "", "write a calendar reminder to this file")
}

// Refill is the `wits refill` command.
var Refill = &cobra.Command{
	Use:   "refill",
	Short: "Say when to order the next prescription",
	Long: "Say when the shelf runs out at the fill's rate — the supply projection's\n" +
		"empty day — and so, less the pharmacy's lead time, when to order the next\n" +
		"prescription. --lead sets the lead time and keeps it; it starts at 3 days.\n" +
		"--ics writes a calendar reminder for the day to order by.",
	Example: "  wits refill\n" +
		"  wits refill --lead 5 --ics refill.ics",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("lead") {
			if refillLead < 0 {
				return fmt.Errorf("--lead %d: a lead time is a number of days, 0 or more", refillLead)
			}
			if err := s.Repo.SetLead(refillLead); err != nil {
				return err
			}
		}
		plan, ok := ledger.PlanRefill(s.Cycle(), s.OpenedAt, s.Repo.Config.Lead())
		if !ok {
			return errors.New("no fill in progress with grinding to go by: nothing to project")
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%.2f g on the shelf, at %.2f g a day with grinding: empty on %s.\n",
			plan.Shelf, plan.Rate, plan.EmptyOn.Format("Mon 2 January"))
		fmt.Fprintln(out, orderLine(plan, s.OpenedAt))
		if refillICS == "" {
			return nil
		}
		if err := os.WriteFile(refillICS, []byte(calendar(plan, s.OpenedAt)), 0600); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Wrote a reminder for %s to %s\n", plan.OrderBy.Format(time.DateOnly), refillICS)
		return nil
	},
}

// orderLine says when to order, or that the day has passed.
func orderLine(plan ledger.Refill, now time.Time) string {
	lead := plural(plan.Lead, "day")
	if plan.Overdue(now) {
		return fmt.Sprintf("Order the next prescription now: with %s for the pharmacy, %s has passed.",
			lead, plan.OrderBy.Format("Mon 2 January"))
	}
	return fmt.Sprintf("Order the next prescription by %s, with %s for the pharmacy.",
		plan.OrderBy.Format("Mon 2 January"), lead)
}

// calendar is a reminder for the day to order by, as an iCalendar file: an
// all-day event with an alarm at nine that morning. Its UID names the day, so
// importing a later reminder for the same day replaces it.
func calendar(plan ledger.Refill, now time.Time) string {
	const day = "20060102"
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//wits//refill//EN", "CALSCALE:GREGORIAN",
		"BEGIN:VEVENT",
		"UID:refill-" + plan.OrderBy.Format(day) + "@wits",
		"DTSTAMP:" + now.UTC().Format("20060102T150405Z"),
		"DTSTART;VALUE=DATE:" + plan.OrderBy.Format(day),
		"DTEND;VALUE=DATE:" + plan.OrderBy.AddDate(0, 0, 1).Format(day),
		"SUMMARY:Order the next prescription",
		fmt.Sprintf("DESCRIPTION:The shelf runs out on %s at %.2f g a day\\, and the pharmacy takes %s.",
			plan.EmptyOn.Format("2 January"), plan.Rate, plural(plan.Lead, "day")),
		"BEGIN:VALARM", "ACTION:DISPLAY", "DESCRIPTION:Order the next prescription", "TRIGGER:PT9H", "END:VALARM",
		"END:VEVENT", "END:VCALENDAR",
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

// warnChain says so when the journal's hash chain does not verify.
func warnChain(out io.Writer, ws *workspace.Workspace) {
	if ws.Chain != nil {
		fmt.Fprintln(out, "⚠ the journal was changed outside wits; wits fsck says where")
	}
}

// warnClashes names every entry that took grams that were not in its jar —
// the same last grams recorded on two devices, once pulled together. The jar
// takes nothing more but an undo until one of them is reverted.
func warnClashes(out io.Writer, state *ledger.State) {
	for _, short := range ledger.Shortfalls(state.Events) {
		fmt.Fprintf(out, "🚨 %s %s: wits revert it, or the entry it clashes with\n", shortHash(short.Event.Hash), short)
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

var (
	sealTimestamp bool
	sealUpgrade   bool
	sealKey       bool
)

// Seal is the `wits seal` command.
var Seal = &cobra.Command{
	Use:   "seal",
	Short: "Sign the journal's head, so others can check what it held",
	Long: "Sign the hash of the journal's last entry with a key of your own. The hash\n" +
		"covers every entry before it, so a seal fixes the whole journal up to that\n" +
		"point. The key is made the first time, in .wits/seal.key; hand out its\n" +
		"fingerprint (--key), never the file.\n\n" +
		"--timestamp also sends the seal's SHA-256 — that hash and nothing else —\n" +
		"to the public OpenTimestamps calendars, which commit it into Bitcoin within\n" +
		"a few hours. --upgrade asks them for the finished proof. `ots verify` and\n" +
		"opentimestamps.org check the .ots file without wits.",
	Example: "  wits seal --timestamp\n  wits seal --upgrade\n  wits verify .wits/seals/2026-10-24-f81bbd2cc893.seal",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		switch {
		case sealKey:
			return showSealKey(cmd.OutOrStdout(), s)
		case sealUpgrade:
			return upgradeSeals(cmd, s)
		}
		f, made, err := sealHead(cmd, s)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if made {
			fmt.Fprintf(out, "Sealed the journal's %s, head %s, as %s.seal\n", entries(f.Entries), shortHash(f.Head), f.Name())
		} else {
			fmt.Fprintf(out, "The head %s is sealed already, as %s.seal\n", shortHash(f.Head), f.Name())
		}
		if sealTimestamp {
			return timestampSeal(cmd, s, f)
		}
		return nil
	},
}

// sealHead seals the journal's head, or finds the seal it already has.
func sealHead(cmd *cobra.Command, s *session) (seal.Seal, bool, error) {
	events, err := s.Journal().Events()
	if err != nil {
		return seal.Seal{}, false, err
	}
	if len(events) == 0 {
		return seal.Seal{}, false, errors.New("the journal is empty: there is nothing to seal")
	}
	head := events[len(events)-1].Hash
	st := seal.Store{Dir: s.Repo.SealsDir()}
	kept, err := st.All()
	if err != nil {
		return seal.Seal{}, false, err
	}
	for _, f := range kept {
		if old, err := seal.Parse(f.Text); err == nil && old.Head == head {
			return old, false, nil
		}
	}
	key, made, err := seal.Key(s.Repo.SealKeyPath())
	if err != nil {
		return seal.Seal{}, false, err
	}
	if made {
		fmt.Fprintf(cmd.ErrOrStderr(), "Made a sealing key in %s; its fingerprint is %s\n",
			s.Repo.SealKeyPath(), seal.Fingerprint(key.Public().(ed25519.PublicKey)))
	}
	f := seal.Make(key, head, len(events), time.Now())
	return f, true, st.Save(seal.File{Name: f.Name(), Text: f.String()})
}

// timestampSeal sends the seal's hash to the calendars and keeps the proof.
func timestampSeal(cmd *cobra.Command, s *session, f seal.Seal) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 45*time.Second)
	defer cancel()
	ots, n, err := seal.Stamp(ctx, &http.Client{Timeout: 20 * time.Second}, f.Digest(), seal.Calendars)
	if err != nil {
		return err
	}
	if err := (seal.Store{Dir: s.Repo.SealsDir()}).Save(seal.File{Name: f.Name(), Text: f.String(), OTS: ots}); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Sent the seal's SHA-256, and nothing else, to %s; the proof is %s.ots, pending until it is in a block.\n",
		plural(n, "calendar"), f.Name())
	return nil
}

// upgradeSeals asks the calendars for the finished proof of every pending seal.
func upgradeSeals(cmd *cobra.Command, s *session) error {
	st := seal.Store{Dir: s.Repo.SealsDir()}
	kept, err := st.All()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 20 * time.Second}
	out := cmd.OutOrStdout()
	asked := 0
	for _, f := range kept {
		if p, err := seal.Describe(f.OTS); err != nil || len(p.Pending) == 0 {
			continue
		}
		asked++
		upgraded, changed, err := seal.Upgrade(ctx, client, f.OTS)
		if err != nil {
			return err
		}
		if !changed {
			fmt.Fprintf(out, "%s: still pending\n", f.Name)
			continue
		}
		f.OTS = upgraded
		if err := st.Save(f); err != nil {
			return err
		}
		p, _ := seal.Describe(upgraded)
		fmt.Fprintf(out, "%s: in Bitcoin block %d\n", f.Name, p.Blocks[0])
	}
	if asked == 0 {
		fmt.Fprintln(out, "No seal is waiting for its proof.")
	}
	return nil
}

// showSealKey prints the public half of the sealing key.
func showSealKey(out io.Writer, s *session) error {
	key, err := seal.LoadKey(s.Repo.SealKeyPath())
	if err != nil {
		return err
	}
	pub := key.Public().(ed25519.PublicKey)
	fmt.Fprintf(out, "%s\ned25519:%s\n", seal.Fingerprint(pub), base64.StdEncoding.EncodeToString(pub))
	return nil
}

// Verify is the `wits verify` command.
var Verify = &cobra.Command{
	Use:   "verify <file.seal>",
	Short: "Check a seal: its signature, the journal it seals, and its timestamp",
	Long: "Check that a seal's signature is over its lines, made with the key it\n" +
		"names; that the journal here holds the head it seals, at the same place;\n" +
		"and, when its .ots proof is beside it, what that proof attests. Run outside\n" +
		"a repository, only the seal itself and its proof are checked.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		f, err := seal.Parse(string(text))
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if err := f.Verify(); err != nil {
			return err
		}
		fmt.Fprintf(out, "✓ signed by %s on %s\n", seal.Fingerprint(f.Key), f.At.Local().Format("2 January 2006, 15:04"))
		if err := verifyHead(out, f); err != nil {
			return err
		}
		return verifyProof(out, f, strings.TrimSuffix(args[0], ".seal")+".ots")
	},
}

// verifyHead says whether the journal here holds the sealed head where the
// seal says it is.
func verifyHead(out io.Writer, f seal.Seal) error {
	s, err := open()
	if err != nil {
		fmt.Fprintf(out, "· not in a repository: the head %s, after %s, is not checked\n", shortHash(f.Head), entries(f.Entries))
		return nil
	}
	events, err := s.Journal().Events()
	if err != nil {
		return err
	}
	if f.Entries < 1 || f.Entries > len(events) || events[f.Entries-1].Hash != f.Head {
		return fmt.Errorf("the journal here does not hold the sealed head %s at entry %d", shortHash(f.Head), f.Entries)
	}
	fmt.Fprintf(out, "✓ the journal here holds what was sealed: %s, up to %s\n", entries(f.Entries), shortHash(f.Head))
	return nil
}

// verifyProof says what the seal's timestamp attests, when it has one.
func verifyProof(out io.Writer, f seal.Seal, path string) error {
	ots, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Fprintln(out, "· no timestamp beside it: `wits seal --timestamp` makes one")
		return nil
	}
	if err != nil {
		return err
	}
	p, err := seal.Describe(ots)
	if err != nil {
		return err
	}
	if d := f.Digest(); !bytes.Equal(p.Digest, d[:]) {
		return errors.New("the timestamp is of something else, not this seal")
	}
	if len(p.Blocks) > 0 {
		fmt.Fprintf(out, "✓ timestamped in Bitcoin block %d; `ots verify` or opentimestamps.org check it against the chain\n", p.Blocks[0])
		return nil
	}
	fmt.Fprintf(out, "· timestamp pending at %s; `wits seal --upgrade` asks for it again\n", plural(len(p.Pending), "calendar"))
	return nil
}

func init() {
	Seal.Flags().BoolVar(&sealTimestamp, "timestamp", false, "also timestamp it with OpenTimestamps (sends its SHA-256 to the calendars)")
	Seal.Flags().BoolVar(&sealUpgrade, "upgrade", false, "ask the calendars for the finished proofs")
	Seal.Flags().BoolVar(&sealKey, "key", false, "show the sealing key's public half and fingerprint")
}

package commands

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheDonDope/wits/pkg/repo"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// run executes a command in dir and returns its combined output.
func run(t *testing.T, dir string, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	t.Chdir(dir)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	defer cmd.SetArgs(nil)

	err := cmd.Execute()
	resetFlags(cmd)
	return out.String(), err
}

// resetFlags puts every flag of a command and its subcommands back to its
// default. The commands bind their flags to package variables, which outlive
// a run: a --date given to one buy used to date every buy after it in the
// same test binary, and a test could pass on a date it never asked for.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

// repository initialises a repository in a temp directory and returns its path.
func repository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := run(t, dir, Init, ".")
	require.NoError(t, err)
	return dir
}

func TestInitCommand(t *testing.T) {
	t.Run("CreatesARepository", func(t *testing.T) {
		dir := t.TempDir()

		out, err := run(t, dir, Init, ".")

		require.NoError(t, err)
		assert.Contains(t, out, "Initialised empty Wits repository", "Should say what it did")
		assert.DirExists(t, dir+"/.wits", "Should create the repository")
	})

	t.Run("RefusesToInitialiseTwice", func(t *testing.T) {
		dir := repository(t)

		_, err := run(t, dir, Init, ".")

		assert.Error(t, err, "Should not clobber an existing repository")
	})
}

func TestBuyAndGrind(t *testing.T) {
	t.Run("RecordsAFillAndAGrind", func(t *testing.T) {
		dir := repository(t)

		out, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
		require.NoError(t, err)
		assert.Contains(t, out, "refer to it as wcake-221\n",
			"Should register the product and say what to call it, ratio and all")

		out, err = run(t, dir, Grind, "wedding", "0.75", "--date", "2026-07-02")
		require.NoError(t, err)
		assert.Contains(t, out, "19.25g left in storage", "Should report the new balance")
	})

	t.Run("RefusesToGrindMoreThanIsThere", func(t *testing.T) {
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "1g")
		require.NoError(t, err)

		_, err = run(t, dir, Grind, "wedding", "5")

		assert.ErrorContains(t, err, "cannot take", "Should not let the balance go negative")
	})

	t.Run("RefusesAnUnknownProduct", func(t *testing.T) {
		dir := repository(t)

		_, err := run(t, dir, Grind, "blueberry", "1")

		assert.Error(t, err, "Should not grind a product it has never seen")
	})

	t.Run("RejectsANonsenseAmount", func(t *testing.T) {
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g")
		require.NoError(t, err)

		_, err = run(t, dir, Grind, "wedding", "lots")

		assert.ErrorContains(t, err, "not an amount", "Should reject an amount it cannot read")
	})

	t.Run("OutsideARepository", func(t *testing.T) {
		_, err := run(t, t.TempDir(), Status)

		assert.ErrorContains(t, err, "not a wits repository", "Should say there is no repository")
	})
}

func TestStatusCommand(t *testing.T) {
	t.Run("ReportsTheCurrentCycle", func(t *testing.T) {
		dir := repository(t)
		yesterday := time.Now().AddDate(0, 0, -1).Format(time.DateOnly)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", yesterday)
		require.NoError(t, err)
		_, err = run(t, dir, Grind, "wedding", "2", "--date", yesterday)
		require.NoError(t, err)

		out, err := run(t, dir, Status)

		require.NoError(t, err)
		assert.Contains(t, out, "On cycle 1", "Should report the cycle")
		assert.Contains(t, out, "18.00g", "Should report what is left in storage")
		assert.Contains(t, out, "days' worth at that rate", "Should estimate the supply")
	})

	// A cycle opened today is what every new repository shows, so its wording is
	// the first anyone reads.
	t.Run("CountsTheFirstDayInTheSingular", func(t *testing.T) {
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g")
		require.NoError(t, err)
		_, err = run(t, dir, Grind, "wedding", "0.75")
		require.NoError(t, err)

		out, err := run(t, dir, Status)

		require.NoError(t, err)
		assert.Contains(t, out, "left over 1 day,", "Should not report a cycle as being 1 days old")
		assert.NotContains(t, out, "1 days", "Should not report any count of one in the plural")
	})

	t.Run("EmptyRepository", func(t *testing.T) {
		out, err := run(t, repository(t), Status)

		require.NoError(t, err)
		assert.Contains(t, out, "No cycle in progress", "Should not pretend there is a cycle")
	})
}

func TestLogCommand(t *testing.T) {
	dir := repository(t)
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
	require.NoError(t, err)
	_, err = run(t, dir, Grind, "wedding", "0.75", "--date", "2026-07-02")
	require.NoError(t, err)

	t.Run("NewestFirst", func(t *testing.T) {
		out, err := run(t, dir, Log, "--oneline")

		require.NoError(t, err)
		assert.Less(t, indexOf(out, "grind"), indexOf(out, "purchase"), "Should show the newest event first")
	})

	t.Run("FiltersByProduct", func(t *testing.T) {
		out, err := run(t, dir, Log, "--product", "wedding")

		require.NoError(t, err)
		assert.Contains(t, out, "wcake", "Should show the product's events")
	})
}

func TestExportCommand(t *testing.T) {
	t.Run("Markdown", func(t *testing.T) {
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
		require.NoError(t, err)
		_, err = run(t, dir, Grind, "wedding", "0.75", "--date", "2026-07-02")
		require.NoError(t, err)

		out, err := run(t, dir, Export)

		require.NoError(t, err)
		assert.Contains(t, out, "# Wits journal", "Should have a title")
		assert.Contains(t, out, "| Purchased | 20.00 g |", "Should summarise the cycle")
		assert.Contains(t, out, "| 2026-07-02 | grind | 0.75 g |", "Should list the events")
	})

	t.Run("RejectsAnUnknownFormat", func(t *testing.T) {
		dir := repository(t)

		_, err := run(t, dir, Export, "--format", "pdf")

		assert.ErrorContains(t, err, "unknown format", "Should not silently write the wrong thing")
	})

	// twoFills is a repository with a fill in July and another in August,
	// each with one grind.
	twoFills := func(t *testing.T) string {
		t.Helper()
		dir := repository(t)
		for _, args := range [][]string{
			{"Enua 22/1 Wedding Cake", "10g", "--date", "2026-07-01"},
			{"Enua 22/1 Wedding Cake", "15g", "--date", "2026-08-15"},
		} {
			_, err := run(t, dir, Buy, args...)
			require.NoError(t, err)
		}
		for _, date := range []string{"2026-07-02", "2026-08-16"} {
			_, err := run(t, dir, Grind, "wedding", "0.5", "--date", date)
			require.NoError(t, err)
		}
		return dir
	}

	t.Run("ARangeHoldsOnlyItsDays", func(t *testing.T) {
		dir := twoFills(t)

		out, err := run(t, dir, Export, "--from", "2026-08-01", "--to", "2026-08-31")

		require.NoError(t, err)
		assert.Contains(t, out, "# Wits journal, 2026-08-01 to 2026-08-31", "Should name the range")
		assert.Contains(t, out, "| Days | 31 |", "Should count both ends")
		assert.Contains(t, out, "| Dispensed | 15.00 g in 1 purchase |", "Should total the range's purchases only")
		assert.Contains(t, out, "| Ground | 0.50 g |", "Should total the range's grinds only")
		assert.NotContains(t, out, "2026-07-02", "Should leave out entries before the range")
	})

	t.Run("RefusesABackwardRange", func(t *testing.T) {
		dir := repository(t)

		_, err := run(t, dir, Export, "--from", "2026-08-31", "--to", "2026-08-01")

		assert.ErrorContains(t, err, "is before", "Should refuse a range that ends before it starts")
	})

	t.Run("RefusesAMalformedDate", func(t *testing.T) {
		dir := repository(t)

		_, err := run(t, dir, Export, "--from", "August")

		assert.ErrorContains(t, err, "not a date", "Should say what it expected")
	})

	t.Run("ASiteLinksItsPages", func(t *testing.T) {
		dir := twoFills(t)
		site := filepath.Join(dir, "docs")

		_, err := run(t, dir, Export, "--site", site)

		require.NoError(t, err)
		index, err := os.ReadFile(filepath.Join(site, "index.md"))
		require.NoError(t, err)
		assert.Contains(t, string(index), "[1](fills/1.md)", "Should link the first fill")
		assert.Contains(t, string(index), "[2](fills/2.md)", "Should link the second fill")
		assert.Less(t, indexOf(string(index), "fills/2.md"), indexOf(string(index), "fills/1.md"), "Should list the newest fill first")

		first, err := os.ReadFile(filepath.Join(site, "fills", "1.md"))
		require.NoError(t, err)
		assert.Contains(t, string(first), "[All fills](../index.md)", "Should link back to the index")
		assert.Contains(t, string(first), "[Fill 2 →](2.md)", "Should link to the next fill")
		assert.NotContains(t, string(first), "← Fill", "Should have no fill before the first")
	})
}

func TestParseGrams(t *testing.T) {
	for in, want := range map[string]float64{
		"0.75": 0.75, "0.75g": 0.75, "0,75 g": 0.75, "20G": 20, " 1.5 ": 1.5,
	} {
		t.Run(in, func(t *testing.T) {
			got, err := parseGrams(in)
			require.NoError(t, err)
			assert.Equal(t, want, got, "Should read the amount")
		})
	}

	for _, in := range []string{"", "lots", "-1", "0", "g"} {
		t.Run("Rejects/"+in, func(t *testing.T) {
			_, err := parseGrams(in)
			assert.Error(t, err, "Should reject %q", in)
		})
	}
}

func TestParseDate(t *testing.T) {
	t.Run("EmptyIsNow", func(t *testing.T) {
		at, err := parseDate("")
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), at, time.Minute, "Should default to now")
	})

	t.Run("DateOnly", func(t *testing.T) {
		at, err := parseDate("2026-07-29")
		require.NoError(t, err)
		assert.Equal(t, 2026, at.Year(), "Should read the year")
		assert.Equal(t, time.July, at.Month(), "Should read the month")
		assert.Equal(t, 29, at.Day(), "Should read the day")
	})

	t.Run("Nonsense", func(t *testing.T) {
		_, err := parseDate("last tuesday")
		assert.Error(t, err, "Should reject a date it cannot read")
	})
}

// indexOf returns the position of sub in s, or -1.
func indexOf(s, sub string) int {
	return bytes.Index([]byte(s), []byte(sub))
}

func TestHomeCommand(t *testing.T) {
	t.Run("OutsideARepository", func(t *testing.T) {
		// It must fail on discovery, before it reaches for a terminal: a error
		// about a missing repository is useful, one about /dev/tty is not.
		_, err := run(t, t.TempDir(), Home)

		assert.ErrorContains(t, err, "not a wits repository",
			"Should say there is no repository rather than fail opening a terminal")
	})

	t.Run("FindsTheRepositoryTheSameWayEveryOtherCommandDoes", func(t *testing.T) {
		dir := repository(t)
		nested := filepath.Join(dir, "notes")
		require.NoError(t, os.MkdirAll(nested, 0700))

		// From a subdirectory it gets past discovery and fails only for want of a
		// terminal, which is as far as a test can drive an interactive program.
		_, err := run(t, nested, Home)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "not a wits repository",
			"Should have walked up to the repository")
		assert.Contains(t, err.Error(), "running the interface",
			"and should have got as far as launching")
	})
}

func TestReconcileCommand(t *testing.T) {
	stocked := func(t *testing.T) string {
		t.Helper()
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g")
		require.NoError(t, err)
		_, err = run(t, dir, Grind, "wedding", "2")
		require.NoError(t, err)
		return dir
	}

	t.Run("RecordsTheDifference", func(t *testing.T) {
		dir := stocked(t)

		out, err := run(t, dir, Reconcile, "wedding", "17.6")

		require.NoError(t, err)
		assert.Contains(t, out, "0.40g", "Should record the difference, not the weight")
		assert.Contains(t, out, "out of storage", "Should say which way the grams went")
		assert.Contains(t, out, "now 17.60g", "and what the account holds now")
	})

	t.Run("DryRunWritesNothing", func(t *testing.T) {
		dir := stocked(t)

		out, err := run(t, dir, Reconcile, "wedding", "17.6", "--dry-run")

		require.NoError(t, err)
		assert.Contains(t, out, "-0.40g", "Should show the signed difference")
		assert.Contains(t, out, "Dry run", "Should say it wrote nothing")

		status, err := run(t, dir, Status)
		require.NoError(t, err)
		assert.Contains(t, status, "18.00g", "and storage should be untouched")
	})

	t.Run("TheStash", func(t *testing.T) {
		dir := stocked(t)

		out, err := run(t, dir, Reconcile, "wedding", "1.75", "--stash")

		require.NoError(t, err)
		assert.Contains(t, out, "out of stash", "Should weigh the stash when asked to")
	})

	t.Run("NothingToReconcile", func(t *testing.T) {
		dir := stocked(t)

		_, err := run(t, dir, Reconcile, "wedding", "18")

		assert.ErrorContains(t, err, "already matches the scale",
			"Should say so rather than record a zero-gram adjustment")
	})

	t.Run("RefusesTwoAccountsAtOnce", func(t *testing.T) {
		dir := stocked(t)
		defer func() { reconcileStash, reconcileAVB = false, false }()

		_, err := run(t, dir, Reconcile, "wedding", "1", "--stash", "--avb")

		assert.ErrorContains(t, err, "one account at a time",
			"Should refuse rather than silently pick one")
	})

	t.Run("AccountFirstShorthand", func(t *testing.T) {
		dir := stocked(t)

		out, err := run(t, dir, Reconcile, "stash", "wedding", "1.75")

		require.NoError(t, err)
		assert.Contains(t, out, "out of stash", "Should weigh the account named first")
		assert.Contains(t, out, "wcake-221", "Should name the jar it adjusted")
	})

	t.Run("AnEmptyJarIsARealReading", func(t *testing.T) {
		dir := stocked(t)

		out, err := run(t, dir, Reconcile, "stash", "wedding", "0")

		require.NoError(t, err)
		assert.Contains(t, out, "now 0.00g", "Should record a jar weighed at zero rather than refuse it")
	})

	t.Run("RefusesAnUnknownAccount", func(t *testing.T) {
		dir := stocked(t)

		_, err := run(t, dir, Reconcile, "shelf", "wedding", "1")

		assert.ErrorContains(t, err, "not an account", "Should name the accounts that exist")
	})

	t.Run("AccountAndFlagTogetherAreRefused", func(t *testing.T) {
		dir := stocked(t)

		_, err := run(t, dir, Reconcile, "stash", "wedding", "1.75", "--stash")

		assert.ErrorContains(t, err, "drop the flag", "Should not let the two forms disagree")
	})

	t.Run("AccountAndWeightWithoutAProduct", func(t *testing.T) {
		dir := stocked(t)

		_, err := run(t, dir, Reconcile, "stash", "1.75")

		assert.ErrorContains(t, err, "which product", "Should say what is missing")
	})

	t.Run("InteractiveNeedsATerminal", func(t *testing.T) {
		dir := stocked(t)

		_, err := run(t, dir, Reconcile)

		assert.ErrorContains(t, err, "needs a terminal",
			"Should refuse under a pipe rather than hang on a form nobody can see")
	})

	t.Run("AppliesCollectedReadings", func(t *testing.T) {
		// The interactive flow past the forms: what was typed gets applied,
		// blanks skip, and a jar that matches is said to match.
		dir := stocked(t)
		t.Chdir(dir)
		s, err := open()
		require.NoError(t, err)

		var out bytes.Buffer
		require.NoError(t, applyReadings(&out, s, journal.Stash,
			[]string{"wcake-221"}, []string{"1.75"}))
		assert.Contains(t, out.String(), "out of stash of wcake-221", "Should record a typed reading")

		out.Reset()
		require.NoError(t, applyReadings(&out, s, journal.Stash,
			[]string{"wcake-221"}, []string{""}))
		assert.Contains(t, out.String(), "nothing to record", "Should treat a blank as a skip")

		out.Reset()
		require.NoError(t, applyReadings(&out, s, journal.Stash,
			[]string{"wcake-221"}, []string{"1.75"}))
		assert.Contains(t, out.String(), "already matches", "Should say when the scale agrees")
	})
}

func TestBundleKeepsTheRecordedOrder(t *testing.T) {
	dir := repository(t)
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
	require.NoError(t, err)
	_, err = run(t, dir, Grind, "wedding", "1", "--date", "2026-07-02")
	require.NoError(t, err)
	// Typed in last, dated first: the fold reads it first, the chain last.
	_, err = run(t, dir, Buy, "Enua 22/1 Wedding Cake", "5g", "--date", "2026-05-01")
	require.NoError(t, err)

	file := filepath.Join(t.TempDir(), "history.wits")
	_, err = run(t, dir, Bundle, "--out", file)
	require.NoError(t, err)

	again := repository(t)
	_, err = run(t, again, Restore, file)
	require.NoError(t, err, "A backdated entry should not break the chain on the way back")

	want, err := os.ReadFile(filepath.Join(dir, ".wits", "journal.ndjson"))
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(again, ".wits", "journal.ndjson"))
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got), "The restored journal should be byte for byte the original")
}

func TestFsckCommand(t *testing.T) {
	t.Run("ACleanRepository", func(t *testing.T) {
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
		require.NoError(t, err)
		_, err = run(t, dir, Grind, "wedding", "1", "--date", "2026-07-02")
		require.NoError(t, err)

		out, err := run(t, dir, Fsck)

		require.NoError(t, err)
		assert.Contains(t, out, "2 entries, 1 product, 0 devices")
		assert.Contains(t, out, "20.00 g came in, 0.00 g went out; 20.00 g in the jars — every gram accounted for")
		assert.Contains(t, out, "✓ the chain verifies")
	})

	t.Run("ReportsWhatIsWrongAndFails", func(t *testing.T) {
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "1g", "--date", "2026-07-01")
		require.NoError(t, err)
		r, err := repo.Discover(dir)
		require.NoError(t, err)
		// Written past the recorder, the way an old import or a hand edit
		// could: more ground than there was, and a product nobody defined.
		for _, e := range []journal.Event{
			{Type: journal.Grind, Product: "wcake-221", Grams: 5, OccurredAt: time.Date(2026, 7, 2, 0, 0, 0, 0, time.Local)},
			{Type: journal.Purchase, Product: "ghost", Grams: 2, OccurredAt: time.Date(2026, 7, 3, 0, 0, 0, 0, time.Local)},
		} {
			_, err := r.Journal().Append(e)
			require.NoError(t, err)
		}
		path := r.JournalPath()
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, bytes.Replace(data, []byte(`"grams":1,`), []byte(`"grams":1.0,`), 1), 0600))

		out, err := run(t, dir, Fsck)

		assert.EqualError(t, err, "3 problems found")
		assert.Contains(t, out, "grind 5.00 g of wcake-221 took more than storage held — 1.00 g",
			"An entry written past the recorder that took what was not there")
		assert.Contains(t, out, "✗ journal hash chain is broken: event 1")
		assert.Contains(t, out, "✗ ghost is named by 1 entry but is not in the product catalog")

		status, err := run(t, dir, Status)
		require.NoError(t, err)
		assert.Contains(t, status, "⚠ the journal was changed outside wits", "status should say so too")
	})

	t.Run("AnAmountFinerThanACentigram", func(t *testing.T) {
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
		require.NoError(t, err)
		r, err := repo.Discover(dir)
		require.NoError(t, err)
		// What an older wits wrote when given 0.333: a line with a valid
		// hash, which this one would refuse to append.
		path := r.JournalPath()
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		events, err := r.Journal().Events()
		require.NoError(t, err)
		old := events[0]
		old.Seq, old.Prev, old.Type, old.From, old.To, old.Grams = 2, old.Hash, journal.Grind, journal.Storage, journal.Stash, 0.333
		old.Hash, err = journal.Hash(old, old.Prev)
		require.NoError(t, err)
		line, err := journal.Marshal(old)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, append(append(data, line...), '\n'), 0600))

		out, err := run(t, dir, Fsck)

		assert.EqualError(t, err, "1 problem found")
		assert.Contains(t, out, "is 0.333 g, finer than a centigram; a bundle keeps 0.33 g")
	})
}

func TestBuyRefusesADateAhead(t *testing.T) {
	dir := repository(t)
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "2g", "--date", "2099-01-01")
	assert.ErrorContains(t, err, "purchase is dated 2099-01-01 00:00, after now: an entry is recorded once it has happened")

	// One written before the recorder refused them is found by fsck.
	r, err := repo.Discover(dir)
	require.NoError(t, err)
	_, err = r.Journal().Append(journal.Event{Type: journal.Purchase, Product: "wcake", Grams: 2,
		OccurredAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.Local)})
	require.NoError(t, err)
	out, err := run(t, dir, Fsck)
	assert.Error(t, err)
	assert.Contains(t, out, "purchase 2.00 g of wcake, is dated 2099-01-01")
}

func TestRestoreWarnsOfWhatWasNotThere(t *testing.T) {
	dir := repository(t)
	r, err := repo.Discover(dir)
	require.NoError(t, err)
	at := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	for _, e := range []journal.Event{
		{Type: journal.Purchase, Product: "wcake", Grams: 1, OccurredAt: at},
		{Type: journal.Grind, Product: "wcake", Grams: 5, OccurredAt: at.AddDate(0, 0, 1)},
	} {
		_, err := r.Journal().Append(e)
		require.NoError(t, err)
	}
	file := filepath.Join(t.TempDir(), "history.wits")
	_, err = run(t, dir, Bundle, "--out", file)
	require.NoError(t, err)

	out, err := run(t, repository(t), Restore, file)

	require.NoError(t, err, "A backup gives back what it holds")
	assert.Contains(t, out, "⚠ 1 entry takes grams that were not in the jar at the time; wits fsck lists them")
}

func TestRateCommand(t *testing.T) {
	dir := repository(t)
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
	require.NoError(t, err)
	_, err = run(t, dir, Grind, "wedding", "1", "--date", "2026-07-02")
	require.NoError(t, err)

	_, err = run(t, dir, Rate, "last", "overall=4")
	assert.ErrorContains(t, err, "no session to rate yet")

	_, err = run(t, dir, Sesh, "wedding", "0.3", "--date", "2026-07-02")
	require.NoError(t, err)
	out, err := run(t, dir, Rate, "last", "overall=4", "Sleep=5", "--note", "slept")
	require.NoError(t, err)
	assert.Contains(t, out, "overall 4, sleep 5")

	_, err = run(t, dir, Rate, "last", "sleep")
	assert.ErrorContains(t, err, "dimension=1 to 5")

	fsck, err := run(t, dir, Fsck)
	require.NoError(t, err, fsck)
}

func TestShowCommand(t *testing.T) {
	dir := repository(t)
	_, err := run(t, dir, Show)
	assert.ErrorContains(t, err, "no entries yet")

	_, err = run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
	require.NoError(t, err)
	_, err = run(t, dir, Grind, "wedding", "2", "--date", "2026-07-02")
	require.NoError(t, err)
	_, err = run(t, dir, Sesh, "wedding", "0.3", "--date", "2026-07-02", "--note", "evening")
	require.NoError(t, err)
	_, err = run(t, dir, Rate, "last", "overall=4", "sleep=5")
	require.NoError(t, err)

	out, err := run(t, dir, Show)
	require.NoError(t, err)
	assert.Contains(t, out, "sesh ", "the newest entry by default")
	assert.Contains(t, out, "entry 3, chained onto")
	assert.Contains(t, out, "Enua 22/1 Wedding Cake (")
	assert.Contains(t, out, "0.30 g, stash → consumed")
	assert.Regexp(t, `In the jar\s+stash\s+2\.00 g →\s+1\.70 g`, out, "the jar before and after")
	assert.Regexp(t, `consumed\s+0\.00 g →\s+0\.30 g`, out)
	assert.Contains(t, out, "Note        evening")
	assert.Contains(t, out, "Fill        1, opened 2026-07-01")
	assert.Contains(t, out, "Rated       overall 4, sleep 5")
	assert.Contains(t, out, "days later", "it was written down long after")

	grind := lastOf(t, dir, journal.Grind)
	_, err = run(t, dir, Revert, grind[:7])
	require.Error(t, err, "the session still needs the grind's grams")

	sesh := lastOf(t, dir, journal.Sesh)
	_, err = run(t, dir, Revert, sesh[:7])
	require.NoError(t, err)
	out, err = run(t, dir, Show, sesh[:7])
	require.NoError(t, err)
	assert.Contains(t, out, "Undone      by ")
	out, err = run(t, dir, Show, "last")
	require.NoError(t, err)
	assert.Contains(t, out, "Undoes      "+sesh[:7]+", sesh 0.30 g of Enua 22/1 Wedding Cake on 2026-07-02")
}

// lastOf is the hash of the newest entry of a type in a repository.
func lastOf(t *testing.T, dir string, typ journal.Type) string {
	t.Helper()
	r, err := repo.Discover(dir)
	require.NoError(t, err)
	events, err := r.Journal().Events()
	require.NoError(t, err)
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == typ {
			return events[i].Hash
		}
	}
	t.Fatalf("no %s entry", typ)
	return ""
}

func TestSyncCommands(t *testing.T) {
	share := t.TempDir()
	deck, phone := repository(t), repository(t)
	_, err := run(t, deck, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
	require.NoError(t, err)

	_, err = run(t, deck, Remote)
	assert.ErrorContains(t, err, "no remote is set", "Should say how to set one")

	out, err := run(t, deck, Remote, "add", share)
	require.NoError(t, err)
	assert.Contains(t, out, "bring it to every other device", "Should say what to do with the key")

	out, err = run(t, deck, Push)
	require.NoError(t, err)
	assert.Contains(t, out, "Pushed 1 entry and 0 ratings.")

	_, err = run(t, phone, Remote, "add", share, "--key", filepath.Join(deck, ".wits", "remote.key"))
	require.NoError(t, err)
	out, err = run(t, phone, Pull)
	require.NoError(t, err)
	assert.Contains(t, out, "Pulled 1 entry and 0 ratings.")
	assert.Contains(t, out, "Took the remote's wcake-221.", "Should name the product it brought")

	_, err = run(t, phone, Grind, "wedding", "1", "--date", "2026-07-02")
	require.NoError(t, err)
	_, err = run(t, phone, Push)
	require.NoError(t, err)
	_, err = run(t, deck, Grind, "wedding", "2", "--date", "2026-07-03")
	require.NoError(t, err)

	_, err = run(t, deck, Push)
	assert.ErrorContains(t, err, "wits pull first", "Should not write over the phone's grind")
	out, err = run(t, deck, Pull)
	require.NoError(t, err)
	assert.Contains(t, out, "Chained 1 entry of this device's on top, with new hashes.")

	out, err = run(t, deck, Pull)
	require.NoError(t, err)
	assert.Contains(t, out, "Already up to date.")
}

func TestRefillCommand(t *testing.T) {
	dir := repository(t)
	today := time.Now()
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", today.AddDate(0, 0, -2).Format(time.DateOnly))
	require.NoError(t, err)
	_, err = run(t, dir, Grind, "wedding", "2", "--date", today.AddDate(0, 0, -1).Format(time.DateOnly))
	require.NoError(t, err)
	orderBy := today.AddDate(0, 0, 9-3)

	out, err := run(t, dir, Refill)

	require.NoError(t, err)
	assert.Contains(t, out, "18.00 g on the shelf, at 2.00 g a day", "Should say what it projects from")
	assert.Contains(t, out, "Order the next prescription by "+orderBy.Format("Mon 2 January")+", with 3 days for the pharmacy.")

	ics := filepath.Join(t.TempDir(), "refill.ics")
	out, err = run(t, dir, Refill, "--lead", "5", "--ics", ics)
	require.NoError(t, err)
	assert.Contains(t, out, "with 5 days for the pharmacy", "Should use the lead time given")
	cal, err := os.ReadFile(ics)
	require.NoError(t, err)
	assert.Contains(t, string(cal), "DTSTART;VALUE=DATE:"+today.AddDate(0, 0, 9-5).Format("20060102")+"\r\n",
		"Should remind on the day to order by, in iCalendar's line endings")

	out, err = run(t, dir, Status)
	require.NoError(t, err)
	assert.Contains(t, out, "with 5 days for the pharmacy", "Should keep the lead time, and status should say it")

	_, err = run(t, dir, Refill, "--lead", "-1")
	assert.ErrorContains(t, err, "0 or more")
}

func TestPriceCommands(t *testing.T) {
	dir := repository(t)
	out, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01", "--price", "170,00")
	require.NoError(t, err)
	assert.Contains(t, out, "paid 170.00 €, 8.50 € a gram", "Should price it as it is bought")
	_, err = run(t, dir, Buy, "Enua 22/1 Wedding Cake", "10g", "--date", "2026-07-02")
	require.NoError(t, err)

	out, err = run(t, dir, Costs)
	require.NoError(t, err)
	assert.Contains(t, out, "170.00 € for 20.00g with a price, 8.50 € a gram; 1 purchase without one")

	// The interactive walk past the form: what was answered gets written.
	s, err := workspace.Open(dir)
	require.NoError(t, err)
	unpriced, err := s.Recorder.Unpriced(false)
	require.NoError(t, err)
	require.Len(t, unpriced, 1)
	var buf bytes.Buffer
	require.NoError(t, applyPrices(&buf, &session{Workspace: s}, unpriced, []string{"90"}))
	assert.Contains(t, buf.String(), "90.00 € for 10.00g of wcake-221, 9.00 € a gram")
	assert.Contains(t, buf.String(), "Priced 1 purchase; 0 left without one.")

	out, err = run(t, dir, Costs)
	require.NoError(t, err)
	assert.Contains(t, out, "260.00 € for 30.00g with a price, 8.67 € a gram\n", "Should count both now, and none without")

	_, err = run(t, dir, Price, unpriced[0].Hash[:7], "abc")
	assert.ErrorContains(t, err, "not an amount of euros")
	out, err = run(t, dir, Fsck)
	require.NoError(t, err, out)
}

func TestReportCommand(t *testing.T) {
	dir := repository(t)
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", time.Now().AddDate(0, 0, -3).Format(time.DateOnly))
	require.NoError(t, err)
	pdf := filepath.Join(t.TempDir(), "report.pdf")

	out, err := run(t, dir, Report, "--out", pdf)

	require.NoError(t, err)
	assert.Contains(t, out, "Wrote a report for ", "Should say what it wrote")
	data, err := os.ReadFile(pdf)
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(data, []byte("%PDF-")), "Should write a PDF")

	_, err = run(t, dir, Report, "--from", "2026-10-31", "--to", "2026-10-01", "--out", pdf)
	assert.ErrorContains(t, err, "is before")
}

func TestSealCommands(t *testing.T) {
	dir := repository(t)
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
	require.NoError(t, err)

	out, err := run(t, dir, Seal)
	require.NoError(t, err)
	assert.Contains(t, out, "Made a sealing key", "Should make a key the first time")
	assert.Contains(t, out, "Sealed the journal's 1 entry")
	out, err = run(t, dir, Seal)
	require.NoError(t, err)
	assert.Contains(t, out, "is sealed already", "Should not seal one head twice")

	seals, err := filepath.Glob(filepath.Join(dir, ".wits", "seals", "*.seal"))
	require.NoError(t, err)
	require.Len(t, seals, 1)
	_, err = run(t, dir, Grind, "wedding", "1", "--date", "2026-07-02")
	require.NoError(t, err)
	out, err = run(t, dir, Verify, seals[0])
	require.NoError(t, err)
	assert.Contains(t, out, "holds what was sealed: 1 entry, up to", "A journal grown since still holds what was sealed")
	out, err = run(t, dir, Fsck)
	require.NoError(t, err, out)

	other := repository(t)
	_, err = run(t, other, Buy, "Enua 22/1 Wedding Cake", "21g", "--date", "2026-07-01")
	require.NoError(t, err)
	_, err = run(t, other, Verify, seals[0])
	assert.ErrorContains(t, err, "does not hold the sealed head", "Should notice another journal")

	text, err := os.ReadFile(seals[0])
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(seals[0], []byte(strings.Replace(string(text), "entries 1", "entries 2", 1)), 0600))
	_, err = run(t, dir, Verify, seals[0])
	assert.ErrorContains(t, err, "signature does not match", "Should refuse a seal changed after signing")
	out, err = run(t, dir, Fsck)
	assert.Error(t, err)
	assert.Contains(t, out, "signature does not match", "fsck should name it too")
}

// fakeCalendar answers like an OpenTimestamps calendar: a promise for a
// digest, and a Bitcoin block once asked again.
func fakeCalendar(t *testing.T) *httptest.Server {
	t.Helper()
	varbytes := func(b *bytes.Buffer, p []byte) {
		b.WriteByte(byte(len(p)))
		b.Write(p)
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b bytes.Buffer
		switch {
		case r.URL.Path == "/digest":
			b.Write([]byte{0x08, 0x00, 0x83, 0xdf, 0xe3, 0x0d, 0x2e, 0xf9, 0x0c, 0x8e}) // sha256, pending
			var uri bytes.Buffer
			varbytes(&uri, []byte(srv.URL))
			varbytes(&b, uri.Bytes())
		case strings.HasPrefix(r.URL.Path, "/timestamp/"):
			b.Write([]byte{0x08, 0x00, 0x05, 0x88, 0x96, 0x0d, 0x73, 0xd7, 0x19, 0x01}) // sha256, bitcoin
			varbytes(&b, []byte{0xb4, 0xd6, 0x37})                                      // block 912180
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSealTimestamp(t *testing.T) {
	srv := fakeCalendar(t)
	saved := seal.Calendars
	seal.Calendars = []string{srv.URL}
	t.Cleanup(func() { seal.Calendars = saved })
	dir := repository(t)
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
	require.NoError(t, err)

	out, err := run(t, dir, Seal, "--timestamp")
	require.NoError(t, err)
	assert.Contains(t, out, "Sent the seal's SHA-256, and nothing else, to 1 calendar")
	seals, err := filepath.Glob(filepath.Join(dir, ".wits", "seals", "*.seal"))
	require.NoError(t, err)
	require.Len(t, seals, 1)
	out, err = run(t, dir, Verify, seals[0])
	require.NoError(t, err)
	assert.Contains(t, out, "timestamp pending at 1 calendar")

	out, err = run(t, dir, Seal, "--upgrade")
	require.NoError(t, err)
	assert.Contains(t, out, "in Bitcoin block 912180")
	out, err = run(t, dir, Verify, seals[0])
	require.NoError(t, err)
	assert.Contains(t, out, "timestamped in Bitcoin block 912180")
	out, err = run(t, dir, Seal, "--upgrade")
	require.NoError(t, err)
	assert.Contains(t, out, "No seal is waiting for its proof.")
	out, err = run(t, dir, Seal, "--key")
	require.NoError(t, err)
	assert.Contains(t, out, "SHA256:")
	out, err = run(t, dir, Fsck)
	require.NoError(t, err, out)
}

func TestCarryCommand(t *testing.T) {
	dir := repository(t)
	_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01")
	require.NoError(t, err)
	_, err = run(t, dir, Grind, "wedding", "2", "--date", "2026-07-02")
	require.NoError(t, err)
	out := t.TempDir()

	text, err := run(t, dir, Carry, "--out", filepath.Join(out, "carry.txt"))
	require.NoError(t, err)
	assert.Contains(t, text, "18.00g  2.00g   0.00g  20.00g on 2026-07-01", "Should list what is held against its fill")
	for _, name := range []string{"carry.svg", "carry.png"} {
		_, err = run(t, dir, Carry, "--out", filepath.Join(out, name))
		require.NoError(t, err, name)
		assert.FileExists(t, filepath.Join(out, name))
	}
	_, err = run(t, dir, Carry, "--out", filepath.Join(out, "carry.pdf"))
	assert.ErrorContains(t, err, ".svg or .png")

	verified, err := run(t, dir, Verify, filepath.Join(out, "carry.txt"))
	require.NoError(t, err)
	assert.Contains(t, verified, "✓ signed by SHA256:")
	assert.Contains(t, verified, "holds what was sealed", "Should check it against the journal here")
}

func TestDescribeAndLike(t *testing.T) {
	dir := repository(t)
	for _, name := range []string{"Enua 22/1 Wedding Cake", "Khiron 20/1 Wedding Cake", "Sumo 25/1 Papaya Bomb"} {
		_, err := run(t, dir, Buy, name, "10g", "--date", "2026-07-01")
		require.NoError(t, err)
	}

	out, err := run(t, dir, Describe, "Enua", "--terpenes", "Caryophyllen, Limonen")
	require.NoError(t, err)
	assert.Contains(t, out, "Terpenes: Caryophyllen, Limonen")
	out, err = run(t, dir, Describe, "Khiron")
	require.NoError(t, err)
	assert.Contains(t, out, "Terpenes: none written down yet")

	out, err = run(t, dir, Like, "Enua")
	require.NoError(t, err)
	assert.Contains(t, out, "Like Enua 22/1 Wedding Cake:\n  Khiron 20/1 Wedding Cake\n")
}

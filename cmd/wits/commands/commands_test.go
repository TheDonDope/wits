package commands

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/spf13/cobra"
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
	return out.String(), err
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
		assert.Contains(t, out, "days left at that rate", "Should estimate the supply")
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
		defer run(t, dir, Export, "--format", "markdown") // restore the flag default

		_, err := run(t, dir, Export, "--format", "pdf")

		assert.ErrorContains(t, err, "unknown format", "Should not silently write the wrong thing")
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
		defer func() { reconcileDryRun = false }()

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
		defer func() { reconcileStash = false }()

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
		defer func() { reconcileStash = false }()

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

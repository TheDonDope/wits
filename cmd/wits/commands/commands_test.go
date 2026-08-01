package commands

import (
	"bytes"
	"io"
	"log"
	"os"
	"testing"
	"time"

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
		assert.Contains(t, out, "New product enua-wedding-cake", "Should register the product")

		out, err = run(t, dir, Grind, "wedding", "0.75", "--date", "2026-07-02")
		require.NoError(t, err)
		assert.Contains(t, out, "19.25g left in storage", "Should report the new balance")
	})

	t.Run("RefusesToGrindMoreThanIsThere", func(t *testing.T) {
		dir := repository(t)
		_, err := run(t, dir, Buy, "Enua 22/1 Wedding Cake", "1g")
		require.NoError(t, err)

		_, err = run(t, dir, Grind, "wedding", "5")

		assert.ErrorContains(t, err, "cannot grind", "Should not let the balance go negative")
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
		assert.Contains(t, out, "enua-wedding-cake", "Should show the product's events")
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

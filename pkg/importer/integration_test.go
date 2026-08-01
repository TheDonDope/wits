package importer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/repo"
)

// workbook is the real tracking spreadsheet, kept in the repository so that the
// importer is tested against the thing it was written for and not only against
// fixtures built to suit it.
//
// Fixtures agree with whatever the code believes. This does not: it has stale
// dropdown labels, a sheet dated a year early, a row logged against no product
// at all, and two products spelled more than one way. Every figure asserted here
// was reconciled against the workbook read independently of this package.
const workbook = "../../assets/Tracking.2022.cleaned.xlsx"

// realWorkbook returns the path, skipping if it is not present.
func realWorkbook(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(workbook); err != nil {
		t.Skipf("%s not present", workbook)
	}
	return workbook
}

func TestImportsTheRealWorkbook(t *testing.T) {
	result, err := Read(realWorkbook(t))
	require.NoError(t, err)

	t.Run("Totals", func(t *testing.T) {
		purchased, ground := result.Grams()

		assert.Len(t, result.Sheets, 29, "one worksheet is one prescription cycle")
		assert.Len(t, result.Products, 48, "48 distinct products, after two pairs merge on their slug")
		assert.InDelta(t, 1184.01, purchased, 0.005, "grams dispensed, per the sheet headers")
		assert.InDelta(t, 1116.97, ground, 0.005, "grams the daily rows account for")

		counts := result.Counts()
		assert.Equal(t, 85, counts[journal.Purchase], "one fill per product per sheet")
		assert.Equal(t, 1284, counts[journal.Grind],
			"1286 rows carry an amount; two are logged against no product and are left out")
	})

	t.Run("Span", func(t *testing.T) {
		first, last := result.Span()

		assert.Equal(t, "2023-10-09", first.Format(time.DateOnly), "the first fill")
		assert.False(t, last.Before(first), "and the last entry after it")
	})

	t.Run("StaleLabelsResolveToTheRightProducts", func(t *testing.T) {
		// 2026-07 heads its columns Ice Cream Cake, Citrus Slap and MAC1+, while
		// every row in it still says WW, FLA or MAC1+. The formulas bind the
		// labels to the header rows, which is the only reason the sheet adds up,
		// and the only reason this comes out right.
		var sheet *Sheet
		for _, s := range result.Sheets {
			if s.Name == "2026-07" {
				sheet = s
			}
		}
		require.NotNil(t, sheet, "2026-07 should have been read")

		slugs := make([]string, 0, len(sheet.Products))
		for _, p := range sheet.Products {
			slugs = append(slugs, p.Slug)
		}
		assert.Equal(t,
			[]string{"420-evolution-ice-cream-cake", "enua-citrus-slap", "cantourage-mac1"},
			slugs, "the header names the products, not the dropdown")
		assert.Positive(t, sheet.Products[0].Ground, "and grams reached the first of them")
	})

	t.Run("ReportsWhatDoesNotAddUp", func(t *testing.T) {
		anomalies := result.Anomalies()
		require.Len(t, anomalies, 3, "no more and no fewer than the three known problems")

		assert.Contains(t, anomalies[0], "2025-03")
		assert.Contains(t, anomalies[0], "fall in 2024", "a sheet dated a year early")
		assert.Contains(t, anomalies[1], "2025-06")
		assert.Contains(t, anomalies[1], "typo", "a 0.01 g fill")
		assert.Contains(t, anomalies[2], "2025-10")
		assert.Contains(t, anomalies[2], "2.40 g", "grams logged against no product")
	})

	t.Run("ReportsProductsMergedByTheirSlug", func(t *testing.T) {
		// The slug drops the THC ratio, so one cultivar from one maker at two
		// potencies becomes one product. Said out loud rather than done quietly.
		require.Len(t, result.Merged, 2)
		assert.Equal(t, "420-evolution-ca-mac-mac1", result.Merged[0].Slug)
		assert.Len(t, result.Merged[0].Names, 2)
		assert.Equal(t, "all-nations-lemon-tartz", result.Merged[1].Slug)
		assert.Len(t, result.Merged[1].Names, 2)
	})

	t.Run("EntriesAreChronological", func(t *testing.T) {
		for i := 1; i < len(result.Events); i++ {
			require.False(t, result.Events[i].OccurredAt.Before(result.Events[i-1].OccurredAt),
				"entry %d is out of order, so the journal would not read as a chronology", i)
		}
	})
}

func TestCommittingTheRealWorkbook(t *testing.T) {
	result, err := Read(realWorkbook(t))
	require.NoError(t, err)

	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, Commit(r, result))

	events, err := r.Journal().Events()
	require.NoError(t, err)

	t.Run("EverythingLands", func(t *testing.T) {
		assert.Len(t, events, 1369, "every entry reaches the journal")
		assert.NoError(t, r.Journal().Verify(), "and the hash chain verifies over all of it")
	})

	t.Run("TheFoldMatchesTheSheets", func(t *testing.T) {
		state := ledger.Fold(events)

		assert.Len(t, state.Cycles, 29, "29 worksheets become 29 cycles")

		// Mass is conserved: what was dispensed is either still in storage, in a
		// tin, or has gone through a device.
		var held, ground float64
		for _, b := range state.Balances {
			held += b.Storage
			ground += b.Stash
		}
		_, sheetGround := result.Grams()
		assert.InDelta(t, sheetGround, ground, 0.005,
			"everything ground is in a tin, since the sheets record no sessions")
		assert.InDelta(t, 1184.01, held+ground, 0.005,
			"and storage plus tins equals what was dispensed")
	})

	t.Run("ImportingAgainIsRefused", func(t *testing.T) {
		err := Commit(r, result)

		assert.ErrorIs(t, err, ErrNotEmpty, "a second import would double every gram")
		after, readErr := r.Journal().Events()
		require.NoError(t, readErr)
		assert.Len(t, after, 1369, "and nothing was added")
	})
}

func TestReadingTheWorkbookLeavesItAlone(t *testing.T) {
	path := realWorkbook(t)
	before, err := os.Stat(path)
	require.NoError(t, err)

	_, err = Read(path)
	require.NoError(t, err)

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime(), "the workbook is read, never written")
	assert.Equal(t, before.Size(), after.Size(), "and not resized")
}

// TestWorkbookIsWhereTheDocumentationSaysItIs guards the path itself: the tests
// above skip when it is missing, so a moved file would quietly stop testing
// anything at all.
func TestWorkbookIsWhereTheDocumentationSaysItIs(t *testing.T) {
	_, err := os.Stat(filepath.Clean(workbook))

	assert.NoError(t, err,
		"%s is committed and the importer tests read it; moving it silently skips them", workbook)
}

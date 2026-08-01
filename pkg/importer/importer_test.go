package importer

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/TheDonDope/wits-tui/pkg/ledger"
	"github.com/TheDonDope/wits-tui/pkg/repo"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// header is one product in a worksheet header.
type header struct {
	name  string
	grams float64
}

// entry is one row of a worksheet's daily table.
type entry struct {
	day   int // days into July 2026
	label string
	grams float64
}

// spec describes a worksheet to build.
type spec struct {
	sheet    string
	products []header
	// labels are what each balance column matches, in header order. They are
	// deliberately separate from the product names: that is the whole point.
	labels  []string
	entries []entry
}

// build writes a workbook laid out like the tracking spreadsheet, including the
// running-balance formulas the importer reads its bindings from.
func build(t *testing.T, specs ...spec) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	for _, s := range specs {
		index, err := f.NewSheet(s.sheet)
		require.NoError(t, err)
		f.SetActiveSheet(index)

		for i, p := range s.products {
			require.NoError(t, f.SetCellStr(s.sheet, cell(1, i+1), p.name))
			require.NoError(t, f.SetCellFloat(s.sheet, cell(2, i+1), p.grams, 2, 64))
		}

		head := len(s.products) + 2
		for col, title := range []string{"Date", "Strain", "Amount"} {
			require.NoError(t, f.SetCellStr(s.sheet, cell(col+1, head), title))
		}

		for i, e := range s.entries {
			r := head + 1 + i
			require.NoError(t, f.SetCellFloat(s.sheet, cell(1, r), serial(e.day), 0, 64))
			require.NoError(t, f.SetCellStr(s.sheet, cell(2, r), e.label))
			require.NoError(t, f.SetCellFloat(s.sheet, cell(3, r), e.grams, 2, 64))

			for slot, label := range s.labels {
				col := firstBalanceColumn + slot*2
				prev := fmt.Sprintf("%s%d", column(col), r-1)
				if i == 0 {
					prev = fmt.Sprintf("B%d", slot+1)
				}
				require.NoError(t, f.SetCellFormula(s.sheet, cell(col, r),
					fmt.Sprintf(`IF(B%d="%s",%s-C%d,%s)`, r, label, prev, r, prev)))
			}
		}
	}
	f.DeleteSheet("Sheet1")

	path := filepath.Join(t.TempDir(), "tracking.xlsx")
	require.NoError(t, f.SaveAs(path))
	return path
}

func cell(col, row int) string { ref, _ := excelize.CoordinatesToCellName(col, row); return ref }
func column(col int) string    { name, _ := excelize.ColumnNumberToName(col); return name }
func serial(day int) float64   { return 46203 + float64(day) }

// twoProducts is a small, well-formed cycle.
func twoProducts() spec {
	return spec{
		sheet:    "2026-07",
		products: []header{{"Enua 22/1 Wedding Cake (g)", 20}, {"Cannamedical 28/1 Lemon Cookie (g)", 10}},
		labels:   []string{"WC", "LC"},
		entries: []entry{
			{1, "WC", 0.75},
			{1, "LC", 1.25},
			{2, "WC", 0.50},
		},
	}
}

func TestRead(t *testing.T) {
	t.Run("FillsAndGrinds", func(t *testing.T) {
		result, err := Read(build(t, twoProducts()))
		require.NoError(t, err)

		require.Len(t, result.Sheets, 1)
		assert.Len(t, result.Products, 2, "Should register both products")

		purchased, ground := result.Grams()
		assert.Equal(t, 30.0, purchased, "Should read the dispensed amounts")
		assert.Equal(t, 2.5, ground, "Should total the daily entries")

		counts := result.Counts()
		assert.Equal(t, 2, counts[journal.Purchase], "Should record one fill per product")
		assert.Equal(t, 3, counts[journal.Grind], "Should record every row with an amount")
		assert.Empty(t, result.Anomalies(), "Should find nothing wrong with a clean sheet")
	})

	t.Run("ResolvesProductsByPositionNotByLabel", func(t *testing.T) {
		// The labels are stale: they read WW and FLA while the header names
		// Wedding Cake and Lemon Cookie. The formulas still bind WW to the first
		// header row, and that is what makes the spreadsheet's arithmetic right.
		s := twoProducts()
		s.labels = []string{"WW", "FLA"}
		s.entries = []entry{{1, "WW", 2}, {1, "FLA", 3}}

		result, err := Read(build(t, s))
		require.NoError(t, err)

		byProduct := map[string]float64{}
		for _, e := range result.Events {
			if e.Type == journal.Grind {
				byProduct[e.Product] += e.Grams
			}
		}
		assert.Equal(t, 2.0, byProduct["enua-wedding-cake"], "WW should credit the first header product")
		assert.Equal(t, 3.0, byProduct["cannamedical-lemon-cookie"], "FLA should credit the second")
		assert.Empty(t, result.Anomalies(),
			"A stale label is not a problem in itself, because the sheet still adds up")
	})

	t.Run("ReportsRowsNoBalanceColumnClaims", func(t *testing.T) {
		s := twoProducts()
		s.entries = append(s.entries, entry{3, "", 2.4})

		result, err := Read(build(t, s))
		require.NoError(t, err)

		_, ground := result.Grams()
		assert.Equal(t, 2.5, ground, "Should not attribute grams the sheet never subtracted")
		require.Len(t, result.Anomalies(), 1)
		assert.Contains(t, result.Anomalies()[0], "2.40 g", "Should report the orphaned amount")
	})

	t.Run("ReportsGroundExceedingDispensed", func(t *testing.T) {
		s := twoProducts()
		s.entries = append(s.entries, entry{3, "LC", 99})

		result, err := Read(build(t, s))
		require.NoError(t, err)

		require.NotEmpty(t, result.Anomalies())
		assert.Contains(t, result.Anomalies()[0], "but only", "Should notice more went out than came in")
	})

	t.Run("ReportsATypoDispensedAmount", func(t *testing.T) {
		s := twoProducts()
		s.products[1].grams = 0.01

		result, err := Read(build(t, s))
		require.NoError(t, err)

		// A 0.01 g fill also trips the ground-exceeds-dispensed check, so the
		// typo can be anywhere in the list.
		assert.Contains(t, strings.Join(result.Anomalies(), "\n"), "looks like a typo",
			"Should flag an implausible fill")
	})

	t.Run("ReportsASheetDatedInTheWrongYear", func(t *testing.T) {
		s := twoProducts()
		s.sheet = "2027-07"

		result, err := Read(build(t, s))
		require.NoError(t, err)

		require.Len(t, result.Anomalies(), 1)
		assert.Contains(t, result.Anomalies()[0], "named for 2027", "Should notice the year does not match")
	})

	t.Run("AllowsACycleToBeginBeforeTheMonthItIsNamedFor", func(t *testing.T) {
		// A fill collected on 30 June belongs to the July sheet, and that is not
		// an error. The year is judged by where most entries fall.
		s := twoProducts()
		s.entries = append([]entry{{-9, "WC", 1}}, s.entries...)

		result, err := Read(build(t, s))
		require.NoError(t, err)

		assert.Empty(t, result.Anomalies(), "Should not flag a cycle that starts early")
	})

	t.Run("ReportsProductsMergedByName", func(t *testing.T) {
		a := twoProducts()
		b := twoProducts()
		b.sheet = "2026-08"
		// The same cultivar from the same maker at a different potency: the slug
		// drops the ratio, so these become one product.
		b.products[0] = header{"Enua 25/1 Wedding Cake (g)", 20}

		result, err := Read(build(t, a, b))
		require.NoError(t, err)

		require.Len(t, result.Merged, 1, "Should notice the two spellings became one product")
		assert.Equal(t, "enua-wedding-cake", result.Merged[0].Slug)
		assert.Len(t, result.Merged[0].Names, 2, "and should list both spellings")
	})

	t.Run("EventsAreChronological", func(t *testing.T) {
		s := twoProducts()
		s.entries = []entry{{3, "WC", 1}, {1, "WC", 1}, {2, "WC", 1}}

		result, err := Read(build(t, s))
		require.NoError(t, err)

		for i := 1; i < len(result.Events); i++ {
			assert.False(t, result.Events[i].OccurredAt.Before(result.Events[i-1].OccurredAt),
				"Should be in date order, so the journal reads as a chronology")
		}
	})

	t.Run("SkipsSheetsThatAreNotTrackingSheets", func(t *testing.T) {
		f := excelize.NewFile()
		_, err := f.NewSheet("Notes")
		require.NoError(t, err)
		require.NoError(t, f.SetCellStr("Notes", "A1", "just some notes"))
		path := filepath.Join(t.TempDir(), "w.xlsx")
		require.NoError(t, f.SaveAs(path))
		require.NoError(t, f.Close())

		result, err := Read(path)
		require.NoError(t, err)
		assert.Empty(t, result.Sheets, "Should ignore a sheet with no daily table")
	})

	t.Run("MissingFile", func(t *testing.T) {
		_, err := Read(filepath.Join(t.TempDir(), "nope.xlsx"))
		assert.Error(t, err, "Should report a workbook it cannot open")
	})
}

func TestCommit(t *testing.T) {
	t.Run("WritesProductsAndEntries", func(t *testing.T) {
		result, err := Read(build(t, twoProducts()))
		require.NoError(t, err)
		r, err := repo.Init(t.TempDir())
		require.NoError(t, err)

		require.NoError(t, Commit(r, result))

		events, err := r.Journal().Events()
		require.NoError(t, err)
		assert.Len(t, events, 5, "Should have written every entry")
		assert.NoError(t, r.Journal().Verify(), "and the chain should verify")

		// The sheet grinds 0.75 and 0.50 of the Wedding Cake, out of 20 g.
		state := ledger.Fold(events)
		assert.Equal(t, 18.75, state.Balances["enua-wedding-cake"].Storage, "Storage should match the sheet")
		assert.Equal(t, 1.25, state.Balances["enua-wedding-cake"].Stash, "and so should the tin")
	})

	t.Run("RefusesARepositoryThatAlreadyHasEntries", func(t *testing.T) {
		result, err := Read(build(t, twoProducts()))
		require.NoError(t, err)
		r, err := repo.Init(t.TempDir())
		require.NoError(t, err)
		require.NoError(t, Commit(r, result))

		err = Commit(r, result)

		assert.ErrorIs(t, err, ErrNotEmpty,
			"Should refuse to import twice, which would double every gram")
		events, err := r.Journal().Events()
		require.NoError(t, err)
		assert.Len(t, events, 5, "and should not have added anything")
	})

	t.Run("ReadingWritesNothing", func(t *testing.T) {
		path := build(t, twoProducts())
		before, err := os.Stat(path)
		require.NoError(t, err)

		_, err = Read(path)
		require.NoError(t, err)

		after, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, before.ModTime(), after.ModTime(), "Should not touch the workbook it reads")
	})
}

func TestSpan(t *testing.T) {
	t.Run("FirstAndLastEntry", func(t *testing.T) {
		s := twoProducts()
		s.entries = []entry{{5, "WC", 1}, {1, "WC", 1}, {9, "WC", 1}}

		result, err := Read(build(t, s))
		require.NoError(t, err)

		first, last := result.Span()
		assert.Equal(t, 1, int(last.Sub(first).Hours()/24/8), "Should span the eight days between them")
		assert.True(t, first.Before(last), "Should report them in order")
	})

	t.Run("EmptyResult", func(t *testing.T) {
		first, last := (&Result{}).Span()

		assert.True(t, first.IsZero(), "Should have no first entry")
		assert.True(t, last.IsZero(), "Should have no last entry")
	})
}

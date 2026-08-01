package importer

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// sheetSpec describes a worksheet to build for a test.
type sheetSpec struct {
	name string
	// products are the header rows: display name and grams dispensed.
	products []struct {
		name  string
		grams float64
	}
	// labels are the strain labels the balance columns bind to, in header order.
	labels []string
	// rows are the daily entries: date serial, label, grams.
	rows []struct {
		serial float64
		label  string
		grams  float64
	}
}

// build writes a workbook laid out like the tracking spreadsheet, including the
// running-balance formulas the importer reads its product bindings from.
func build(t *testing.T, spec sheetSpec) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	name := spec.name
	index, err := f.NewSheet(name)
	require.NoError(t, err)
	f.SetActiveSheet(index)

	for i, p := range spec.products {
		require.NoError(t, f.SetCellStr(name, cell(1, i+1), p.name))
		require.NoError(t, f.SetCellFloat(name, cell(2, i+1), p.grams, 2, 64))
	}

	header := len(spec.products) + 2
	for col, title := range []string{"Date", "Strain", "Amount"} {
		require.NoError(t, f.SetCellStr(name, cell(col+1, header), title))
	}

	for i, row := range spec.rows {
		r := header + 1 + i
		require.NoError(t, f.SetCellFloat(name, cell(1, r), row.serial, 0, 64))
		require.NoError(t, f.SetCellStr(name, cell(2, r), row.label))
		require.NoError(t, f.SetCellFloat(name, cell(3, r), row.grams, 2, 64))

		// One pair of balance columns per product, each binding a label to a
		// header row, exactly as the real spreadsheet does.
		for slot, label := range spec.labels {
			col := 4 + slot*2
			prev := fmt.Sprintf("%s%d", column(col), r-1)
			if i == 0 {
				prev = fmt.Sprintf("B%d", slot+1)
			}
			formula := fmt.Sprintf(`IF(B%d="%s",%s-C%d,%s)`, r, label, prev, r, prev)
			require.NoError(t, f.SetCellFormula(name, cell(col, r), formula))
		}
	}

	path := filepath.Join(t.TempDir(), "tracking.xlsx")
	require.NoError(t, f.SaveAs(path))
	return path
}

// cell returns an A1 reference for a one-based column and row.
func cell(col, row int) string {
	ref, _ := excelize.CoordinatesToCellName(col, row)
	return ref
}

// column returns the letter for a one-based column.
func column(col int) string {
	name, _ := excelize.ColumnNumberToName(col)
	return name
}

// serial is the spreadsheet serial number for the nth day of July 2026.
func serial(day int) float64 { return 46203 + float64(day) }

func TestImport(t *testing.T) {
	t.Run("ReadsFillsAndGrinds", func(t *testing.T) {
		path := build(t, sheetSpec{
			name: "2026-07",
			products: []struct {
				name  string
				grams float64
			}{{"Enua 22/1 Wedding Cake (g)", 20}, {"Cannamedical 28/1 Lemon Cookie (g)", 10}},
			labels: []string{"WC", "LC"},
			rows: []struct {
				serial float64
				label  string
				grams  float64
			}{
				{serial(1), "WC", 0.75},
				{serial(1), "LC", 1.25},
				{serial(2), "WC", 0.5},
			},
		})

		result, err := Import(path)
		require.NoError(t, err)

		require.Len(t, result.Sheets, 1)
		assert.Len(t, result.Products, 2, "Should register both products")

		purchased, ground := result.Grams()
		assert.Equal(t, 30.0, purchased, "Should read the dispensed amounts")
		assert.Equal(t, 2.5, ground, "Should total the daily entries")

		var purchases, grinds int
		for _, e := range result.Events {
			switch e.Type {
			case journal.Purchase:
				purchases++
			case journal.Grind:
				grinds++
			}
		}
		assert.Equal(t, 2, purchases, "Should record one fill per product")
		assert.Equal(t, 3, grinds, "Should record every entry with an amount")
		assert.Empty(t, result.Anomalies(), "Should find nothing wrong with a clean sheet")
	})

	t.Run("ResolvesProductsByPositionNotByLabelText", func(t *testing.T) {
		// The dropdown labels are stale: they say WW and FLA while the header
		// names Wedding Cake and Lemon Cookie. The formulas still bind WW to the
		// first header row, which is what makes the sheet's arithmetic right.
		path := build(t, sheetSpec{
			name: "2026-07",
			products: []struct {
				name  string
				grams float64
			}{{"Enua 22/1 Wedding Cake (g)", 20}, {"Cannamedical 28/1 Lemon Cookie (g)", 10}},
			labels: []string{"WW", "FLA"},
			rows: []struct {
				serial float64
				label  string
				grams  float64
			}{
				{serial(1), "WW", 2},
				{serial(1), "FLA", 3},
			},
		})

		result, err := Import(path)
		require.NoError(t, err)

		byProduct := map[string]float64{}
		for _, e := range result.Events {
			if e.Type == journal.Grind {
				byProduct[e.Product] += e.Grams
			}
		}
		assert.Equal(t, 2.0, byProduct["enua-wedding-cake"], "Should credit WW to the first header product")
		assert.Equal(t, 3.0, byProduct["cannamedical-lemon-cookie"], "Should credit FLA to the second header product")
		assert.Empty(t, result.Anomalies(), "Should not treat a stale label as a problem, because the sheet still adds up")
	})

	t.Run("ReportsEntriesNoBalanceColumnClaims", func(t *testing.T) {
		path := build(t, sheetSpec{
			name: "2026-07",
			products: []struct {
				name  string
				grams float64
			}{{"Enua 22/1 Wedding Cake (g)", 20}},
			labels: []string{"WC"},
			rows: []struct {
				serial float64
				label  string
				grams  float64
			}{
				{serial(1), "WC", 1},
				{serial(2), "", 2.4}, // no strain picked, so nothing was subtracted
			},
		})

		result, err := Import(path)
		require.NoError(t, err)

		_, ground := result.Grams()
		assert.Equal(t, 1.0, ground, "Should not attribute grams the sheet never subtracted")
		require.Len(t, result.Anomalies(), 1)
		assert.Contains(t, result.Anomalies()[0], "2.40g", "Should report the orphaned amount")
	})

	t.Run("ReportsASheetDatedInTheWrongYear", func(t *testing.T) {
		path := build(t, sheetSpec{
			name: "2027-07",
			products: []struct {
				name  string
				grams float64
			}{{"Enua 22/1 Wedding Cake (g)", 20}},
			labels: []string{"WC"},
			rows: []struct {
				serial float64
				label  string
				grams  float64
			}{{serial(1), "WC", 1}},
		})

		result, err := Import(path)
		require.NoError(t, err)

		require.Len(t, result.Anomalies(), 1)
		assert.Contains(t, result.Anomalies()[0], "named for 2027", "Should notice the year does not match")
	})

	t.Run("AllowsACycleToStartBeforeTheMonthItIsNamedFor", func(t *testing.T) {
		// A fill collected on 30 December belongs to the January sheet, and that
		// is not an error.
		path := build(t, sheetSpec{
			name: "2026-07",
			products: []struct {
				name  string
				grams float64
			}{{"Enua 22/1 Wedding Cake (g)", 20}},
			labels: []string{"WC"},
			rows: []struct {
				serial float64
				label  string
				grams  float64
			}{
				{serial(-2), "WC", 1}, // late June
				{serial(1), "WC", 1},
				{serial(2), "WC", 1},
			},
		})

		result, err := Import(path)
		require.NoError(t, err)

		assert.Empty(t, result.Anomalies(), "Should judge the year by where most entries fall")
	})

	t.Run("EventsAreChronological", func(t *testing.T) {
		path := build(t, sheetSpec{
			name: "2026-07",
			products: []struct {
				name  string
				grams float64
			}{{"Enua 22/1 Wedding Cake (g)", 20}},
			labels: []string{"WC"},
			rows: []struct {
				serial float64
				label  string
				grams  float64
			}{
				{serial(3), "WC", 1},
				{serial(1), "WC", 1},
				{serial(2), "WC", 1},
			},
		})

		result, err := Import(path)
		require.NoError(t, err)

		for i := 1; i < len(result.Events); i++ {
			assert.False(t, result.Events[i].OccurredAt.Before(result.Events[i-1].OccurredAt),
				"Should be in date order so the journal reads as a chronology")
		}
	})

	t.Run("MissingFile", func(t *testing.T) {
		_, err := Import(filepath.Join(t.TempDir(), "nope.xlsx"))
		assert.Error(t, err, "Should report a workbook it cannot open")
	})
}

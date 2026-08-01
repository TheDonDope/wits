package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// workbook writes a small tracking spreadsheet, laid out like the real one:
// products in the header, a daily table below, and the running-balance formulas
// that bind a strain label to a header row.
func workbook(t *testing.T, orphan bool) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "2026-07"
	index, err := f.NewSheet(sheet)
	require.NoError(t, err)
	f.SetActiveSheet(index)

	require.NoError(t, f.SetCellStr(sheet, "A1", "Enua 22/1 Wedding Cake (g)"))
	require.NoError(t, f.SetCellFloat(sheet, "B1", 20, 2, 64))
	require.NoError(t, f.SetCellStr(sheet, "A2", "Cannamedical 28/1 Lemon Cookie (g)"))
	require.NoError(t, f.SetCellFloat(sheet, "B2", 10, 2, 64))
	for col, title := range []string{"A4", "B4", "C4"} {
		require.NoError(t, f.SetCellStr(sheet, title, []string{"Date", "Strain", "Amount"}[col]))
	}

	rows := []struct {
		label string
		grams float64
	}{{"WC", 0.75}, {"LC", 1.25}}
	if orphan {
		rows = append(rows, struct {
			label string
			grams float64
		}{"", 2.40})
	}
	for i, r := range rows {
		row := 5 + i
		require.NoError(t, f.SetCellFloat(sheet, fmt.Sprintf("A%d", row), float64(46204+i), 0, 64))
		require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("B%d", row), r.label))
		require.NoError(t, f.SetCellFloat(sheet, fmt.Sprintf("C%d", row), r.grams, 2, 64))
		for slot, label := range []string{"WC", "LC"} {
			col := []string{"D", "F"}[slot]
			prev := fmt.Sprintf("%s%d", col, row-1)
			if i == 0 {
				prev = fmt.Sprintf("B%d", slot+1)
			}
			require.NoError(t, f.SetCellFormula(sheet, fmt.Sprintf("%s%d", col, row),
				fmt.Sprintf(`IF(B%d="%s",%s-C%d,%s)`, row, label, prev, row, prev)))
		}
	}
	f.DeleteSheet("Sheet1")

	path := filepath.Join(t.TempDir(), "tracking.xlsx")
	require.NoError(t, f.SaveAs(path))
	return path
}

func TestImportCommand(t *testing.T) {
	t.Run("DryRunReportsAndWritesNothing", func(t *testing.T) {
		dir := repository(t)

		out, err := run(t, dir, Import, workbook(t, false))

		require.NoError(t, err)
		assert.Contains(t, out, "1 worksheets, 2 products, 4 entries", "Should summarise what it found")
		assert.Contains(t, out, "30.00 g dispensed", "Should total the fills")
		assert.Contains(t, out, "2.00 g ground", "Should total the entries")
		assert.Contains(t, out, "2026-07", "Should report the span")
		assert.Contains(t, out, "Nothing in the spreadsheet looks wrong", "Should say so when it is clean")
		assert.Contains(t, out, "Dry run", "Should say it wrote nothing")

		// Straight at the file rather than through another command: the flags
		// those bind are package level and leak between tests.
		_, err = os.Stat(filepath.Join(dir, ".wits", "journal.ndjson"))
		assert.True(t, os.IsNotExist(err), "and should really have written nothing")
	})

	t.Run("ReportsWhatDoesNotAddUp", func(t *testing.T) {
		out, err := run(t, repository(t), Import, workbook(t, true))

		require.NoError(t, err)
		assert.Contains(t, out, "worth checking", "Should flag the orphaned row")
		assert.Contains(t, out, "2.40 g", "Should say how much went unattributed")
		assert.Contains(t, out, "unaccounted for", "Should report the gap between dispensed and ground")
	})

	t.Run("CommitWrites", func(t *testing.T) {
		dir := repository(t)
		defer func() { importCommit = false }()

		out, err := run(t, dir, Import, workbook(t, false), "--commit")

		require.NoError(t, err)
		assert.Contains(t, out, "Imported 2 products and 4 entries", "Should say what it wrote")

		status, err := run(t, dir, Status)
		require.NoError(t, err)
		assert.Contains(t, status, "28.00g", "The ledger should reflect the import")
	})

	t.Run("RefusesToImportTwice", func(t *testing.T) {
		dir := repository(t)
		defer func() { importCommit = false }()
		path := workbook(t, false)
		_, err := run(t, dir, Import, path, "--commit")
		require.NoError(t, err)

		_, err = run(t, dir, Import, path, "--commit")

		assert.ErrorContains(t, err, "already holds entries",
			"Should refuse, since a second import would double every gram")
	})

	t.Run("MissingFile", func(t *testing.T) {
		_, err := run(t, repository(t), Import, filepath.Join(t.TempDir(), "nope.xlsx"))
		assert.Error(t, err, "Should report a workbook it cannot open")
	})
}

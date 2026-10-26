// Package importer reads a tracking spreadsheet into entries; this file is
// the spreadsheet as the importer reads it — each sheet's rows as written,
// and the formulas of the row that binds the balance columns to their
// products — so nothing but xlsx.go needs the library that reads a .xlsx
// file. The phone's engine is built without it (-tags noxlsx), 450 KB
// gzipped lighter; a module of its own, loaded only to import, reads the
// file into a Workbook there.
package importer

import (
	"fmt"
	"strconv"
	"time"
)

// Workbook is a spreadsheet as the importer reads it.
type Workbook struct {
	Sheets []WorkSheet `json:"sheets"`
}

// WorkSheet is one sheet: its rows, every cell as its raw value, and the
// formulas of the row the balance columns are bound in, by cell name.
type WorkSheet struct {
	Name     string            `json:"name"`
	Rows     [][]string        `json:"rows"`
	Formulas map[string]string `json:"formulas"`
}

// BalanceRow is the row, numbered from 1, whose formulas bind the balance
// columns to the products, in a sheet whose daily table begins at header (0
// based); or 0 for a sheet without one.
func BalanceRow(rows [][]string) int {
	if h := headerRow(rows); h >= 0 {
		return h + 2
	}
	return 0
}

// BalanceCells is the cells of a row that can hold a balance formula.
func BalanceCells(row int) []string {
	cells := make([]string, 0, lastBalanceColumn-firstBalanceColumn+1)
	for col := firstBalanceColumn; col <= lastBalanceColumn; col++ {
		cells = append(cells, CellName(col, row))
	}
	return cells
}

// CellName is a cell's name, "D7", from its column and row, both from 1.
func CellName(col, row int) string {
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return name + strconv.Itoa(row)
}

// excel1900 is day 0 of a workbook's dates, after Excel's 1900 leap-year slip.
var excel1900 = time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC)

// excelDate is a date as a workbook stores it — days since 1899-12-30, the
// time of day as a fraction — rounded to the second, as the .xlsx reader
// rounds it. Days before March 1900 count differently in Excel and are no
// tracking sheet's.
func excelDate(serial float64) (time.Time, error) {
	whole := int(serial)
	if serial < 0 || whole <= 61 {
		return time.Time{}, fmt.Errorf("%v is not a date a tracking sheet holds", serial)
	}
	part := serial - float64(whole) + 1e-9
	at := excel1900.AddDate(0, 0, whole).Add(time.Duration(float64(24*time.Hour) * part))
	if at.Nanosecond()/1e6 > 500 {
		return at.Round(time.Second), nil
	}
	return at.Truncate(time.Second), nil
}

// ReadWorkbook parses a workbook already read, from a .xlsx file or from
// the phone's reader. It writes nothing, anywhere.
func ReadWorkbook(w *Workbook) (*Result, error) {
	return read(w)
}

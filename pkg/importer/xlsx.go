//go:build !noxlsx

// Package importer reads a tracking spreadsheet into entries; this file reads
// a .xlsx file into a Workbook, with the one library that can.
package importer

import (
	"io"

	"github.com/xuri/excelize/v2"
)

// Read parses the workbook at path. It opens the file read-only and writes
// nothing, anywhere.
func Read(path string) (*Result, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	w, err := workbookOf(f)
	if err != nil {
		return nil, err
	}
	return read(w)
}

// ReadFrom parses a workbook from a stream — one uploaded through the browser,
// which has no path. It writes nothing, anywhere.
func ReadFrom(in io.Reader) (*Result, error) {
	w, err := Load(in)
	if err != nil {
		return nil, err
	}
	return read(w)
}

// Load reads a .xlsx file into a Workbook, parsing nothing: what the phone's
// reader hands its engine.
func Load(in io.Reader) (*Workbook, error) {
	f, err := excelize.OpenReader(in)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return workbookOf(f)
}

// workbookOf is an open .xlsx file's sheets: their rows as raw values, and
// the formulas of the row the balance columns are bound in.
func workbookOf(f *excelize.File) (*Workbook, error) {
	w := &Workbook{}
	for _, name := range f.GetSheetList() {
		rows, err := f.GetRows(name, excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, err
		}
		sheet := WorkSheet{Name: name, Rows: rows, Formulas: map[string]string{}}
		if row := BalanceRow(rows); row > 0 {
			for _, cell := range BalanceCells(row) {
				if formula, err := f.GetCellFormula(name, cell); err == nil && formula != "" {
					sheet.Formulas[cell] = formula
				}
			}
		}
		w.Sheets = append(w.Sheets, sheet)
	}
	return w, nil
}

//go:build noxlsx

// Package importer reads a tracking spreadsheet into entries; this file
// stands in for xlsx.go where the .xlsx reader is left out — the phone's
// engine, which is handed a Workbook its reader made instead.
package importer

import (
	"errors"
	"io"
)

// ErrNoReader is why a .xlsx file is refused where its reader is left out.
var ErrNoReader = errors.New("this build reads a workbook only once it is read into a Workbook")

// Read is refused here; see ReadWorkbook.
func Read(string) (*Result, error) { return nil, ErrNoReader }

// ReadFrom is refused here; see ReadWorkbook.
func ReadFrom(io.Reader) (*Result, error) { return nil, ErrNoReader }

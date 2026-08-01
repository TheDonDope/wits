// Package importer reads the tracking spreadsheet Wits replaces and turns it
// into journal events.
//
// One worksheet is one prescription cycle. The header names the products and
// the grams dispensed; the table below it holds one row per product per day
// with the grams ground that day.
//
// Products are resolved by position, not by the text in the strain column.
// Those labels are dropdown values that were not always renamed when the
// products changed, so by the later sheets they are stale: a sheet whose header
// reads "Wedding Cake" may still say "WW" in every row. What makes the
// spreadsheet correct anyway is that the running-balance columns bind each
// label to a header row by formula:
//
//	=IF(B6="WW", B1-C6, B1)
//
// The first balance column subtracts from the first header product whatever
// matches its label, whatever that label happens to say. The importer reads
// that binding out of the formulas, so it inherits the spreadsheet's own
// arithmetic rather than second-guessing it from names.
package importer

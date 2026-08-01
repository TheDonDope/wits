// Package importer reads the tracking spreadsheet Wits grew out of and turns it
// into journal entries.
//
// One worksheet is one prescription cycle. The header names the products and the
// grams dispensed; the table below holds one row per product per day with the
// grams ground that day.
//
// # Products are resolved by position, not by name
//
// The strain column holds dropdown values that were not always renamed when the
// products changed, so in later sheets they are stale: a sheet whose header
// reads "Ice Cream Cake" may say "WW" in every row. What keeps the spreadsheet
// correct anyway is that each running-balance column binds a label to a header
// row by formula:
//
//	=IF(B6="WW", B1-C6, B1)
//
// The first balance column subtracts whatever matches its label from the first
// header product, whatever that label happens to say. Reading the binding out of
// the formulas inherits the spreadsheet's own arithmetic instead of
// second-guessing it from names, and it is self-checking: if a formula's target
// ever disagrees with its column's position, that is reported rather than
// guessed at.
//
// # Nothing is written without being asked
//
// Import returns what it found and changes nothing. The caller decides whether
// to commit it, which is what makes a dry run the default and lets years of
// history be checked before it is recorded.
package importer

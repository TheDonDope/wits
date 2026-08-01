// Package ledger derives current state by replaying the journal.
//
// Nothing here is persisted. Balances, cycles and statistics are all folds over
// the event log, which is why a mistake is fixed by appending a correction
// rather than by editing a stored number.
package ledger

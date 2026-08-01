// Package journal implements the append-only event log that backs Wits.
//
// Every change to the inventory is an immutable, hash-chained event appended to
// a newline-delimited JSON file. Nothing is ever edited or removed: a mistake is
// corrected by appending a compensating event, the way a git revert adds a
// commit rather than rewriting one. Current balances are never stored, they are
// derived by replaying the log.
package journal

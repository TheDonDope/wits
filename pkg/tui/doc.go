// Package tui is the terminal interface to a Wits repository.
//
// It is a reader over the ledger. Screens render what the fold produces and
// nothing else: no screen holds a balance of its own, and none of them writes
// to the journal except through the same service the commands use. That is
// what keeps the numbers on screen and the numbers in `wits status` the same
// numbers.
//
// Built on Bubble Tea v2. The charts are drawn here rather than pulled in,
// because the charting libraries available still target the v1 line, and mixing
// the two would put two renderers and two colour-profile detectors in one
// binary — which shows up as inconsistent colour on screen.
package tui

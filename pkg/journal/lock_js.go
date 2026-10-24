//go:build js

// Package journal is the append-only, hash-chained log; doc.go says the rest.
// This file is its lock in a browser, built only there, which is why it says
// so again: a linter reading one file at a time cannot see doc.go.
package journal

import "os"

// lockFile has nothing to wait for in a browser: the page runs one copy of
// wits, on one thread, so no other writer can reach its files.
func lockFile(*os.File) error { return nil }

// unlockFile releases what lockFile did not take.
func unlockFile(*os.File) error { return nil }

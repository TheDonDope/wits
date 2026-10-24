//go:build js

package journal

import "os"

// lockFile has nothing to wait for in a browser: the page runs one copy of
// wits, on one thread, so no other writer can reach its files.
func lockFile(*os.File) error { return nil }

// unlockFile releases what lockFile did not take.
func unlockFile(*os.File) error { return nil }

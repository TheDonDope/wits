//go:build windows

package journal

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes an exclusive lock on an open file, waiting for any other
// holder to finish.
func lockFile(f *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped)
}

// unlockFile releases the lock.
func unlockFile(f *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlapped)
}

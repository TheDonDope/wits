//go:build !windows

package journal

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock on an open file, waiting for any
// other holder to finish.
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlockFile releases the lock.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

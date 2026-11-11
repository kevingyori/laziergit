//go:build !windows

package reviewbridge

import (
	"golang.org/x/sys/unix"
	"os"
)

func lockStoreFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX) }
func unlockStoreFile(f *os.File)     { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }

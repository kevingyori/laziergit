package reviewbridge

import (
	"os"
	"path/filepath"
)

func withStoreLock(dir string, fn func() error) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "review.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := lockStoreFile(f); err != nil {
		return err
	}
	defer unlockStoreFile(f)
	return fn()
}

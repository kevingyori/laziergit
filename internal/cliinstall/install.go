// Package cliinstall installs the command that launches the bundled app.
package cliinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Destination = "/usr/local/bin/laziergit"

// BundleExecutable resolves a launcher symlink and rejects temporary app copies.
func BundleExecutable(executable string) (string, error) {
	executable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	macos := filepath.Dir(executable)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" || filepath.Ext(bundle) != ".app" {
		return "", fmt.Errorf("install the command from the packaged laziergit app")
	}
	if strings.HasPrefix(executable, "/Volumes/") || strings.Contains(executable, "/AppTranslocation/") {
		return "", fmt.Errorf("move laziergit to Applications, then reopen it before installing the command")
	}
	return executable, nil
}

// Install is idempotent and never replaces an existing file, directory or link.
// Symlink creates the exact destination atomically, including after elevation.
func Install(executable, destination string) error {
	if target, err := os.Readlink(destination); err == nil && target == executable {
		return nil
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("%s already exists; remove it manually if you want to replace it", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	return os.Symlink(executable, destination)
}

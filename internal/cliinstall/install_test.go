package cliinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstall(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "app with 'quotes'.app", "Contents", "MacOS", "laziergit")
	if err := os.MkdirAll(filepath.Dir(executable), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("app"), 0755); err != nil {
		t.Fatal(err)
	}
	executable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "bin", "laziergit")
	for range 2 {
		if err := Install(executable, destination); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := BundleExecutable(destination)
	if err != nil || resolved != executable {
		t.Fatalf("resolved launcher = %q, %v", resolved, err)
	}
}

func TestInstallPreservesExistingCommand(t *testing.T) {
	for _, kind := range []string{"file", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "laziergit")
			var err error
			switch kind {
			case "file":
				err = os.WriteFile(destination, []byte("existing"), 0755)
			case "directory":
				err = os.Mkdir(destination, 0755)
			case "symlink":
				err = os.Symlink("missing-command", destination)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, _ := os.Lstat(destination)
			if err := Install("/some/app", destination); err == nil {
				t.Fatal("replaced an existing command")
			}
			after, err := os.Lstat(destination)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("existing command changed: %v", err)
			}
		})
	}
}

func TestRejectUnbundledExecutable(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BundleExecutable(executable); err == nil {
		t.Fatal("accepted an executable outside an app bundle")
	}
}

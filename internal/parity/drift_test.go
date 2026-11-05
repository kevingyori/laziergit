package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestForksMatchUpstream fails when the lazygit test a fork was copied from
// has changed (usually after bumping lazygit). Read lazygit's change, update
// the fork if needed, then run: go run ./internal/parity/cmd/parityfork -accept <name>
func TestForksMatchUpstream(t *testing.T) {
	forks, err := Forks(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(forks) == 0 {
		return
	}
	dir, version, err := Lazygit()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range forks {
		up := UpstreamPath(dir, f.Name)
		h, err := Hash(up)
		if os.IsNotExist(err) {
			t.Errorf("%s: lazygit %s no longer has this test; delete the fork or keep it as our own test outside %s", f.Name, version, ForksDir)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if h != f.Hash {
			old := filepath.Join(filepath.Dir(dir), "lazygit@"+f.Version, "pkg", "integration", "tests", filepath.FromSlash(f.Name)+".go")
			t.Errorf("%s: upstream changed since lazygit %s (now %s). See what lazygit changed:\n"+
				"\tgo mod download github.com/jesseduffield/lazygit@%s  # if not cached\n\tdiff -u %s %s\n"+
				"then update internal/parity/%s and run: go run ./internal/parity/cmd/parityfork -accept %s",
				f.Name, f.Version, version, f.Version, old, up, filepath.ToSlash(f.Path), f.Name)
		}
	}
}

// TestRegistryUpToDate keeps forks.go in step with the fork files.
func TestRegistryUpToDate(t *testing.T) {
	forks, err := Forks(".")
	if err != nil {
		t.Fatal(err)
	}
	want, err := Registry(forks)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(ForksDir, "forks.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s/forks.go is stale; run: go run ./internal/parity/cmd/parityfork -gen", ForksDir)
	}
}

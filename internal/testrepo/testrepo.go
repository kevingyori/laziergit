// Package testrepo builds small git repositories for tests.
package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// New creates a repo with two commits on main, a feature branch, one stash,
// a modified a.go and an untracked new.txt. It points CONFIG_DIR at a temp
// dir so the real lazygit config is never read or written.
func New(t testing.TB) string {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("CONFIG_DIR", configDir)
	// lazygit's test config: no intro popup on first open.
	if err := os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("disableStartupPopups: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "t@t",
		"GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "t@t", "GIT_CONFIG_NOSYSTEM": "1"} {
		t.Setenv(k, v)
	}
	dir := t.TempDir()
	// Keep tests off the network: no gh means no GitHub pull-request lookups.
	t.Setenv("GH_PATH", filepath.Join(dir, ".no-gh"))
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	Git(t, dir, "init", "-q", "-b", "main")
	Write(t, dir, "a.go", "package a\n")
	Git(t, dir, "add", ".")
	Git(t, dir, "commit", "-qm", "first")
	Write(t, dir, "a.go", "package a\n\nfunc A() {}\n")
	Git(t, dir, "commit", "-qam", "second")
	Git(t, dir, "branch", "feature")
	Write(t, dir, "a.go", "package a\n\nfunc A() { println() }\n")
	Git(t, dir, "stash", "-q")
	Write(t, dir, "a.go", "package a\n\nfunc A() { return }\n")
	Write(t, dir, "new.txt", "hello\n")
	return dir
}

func Git(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func Write(t testing.TB, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

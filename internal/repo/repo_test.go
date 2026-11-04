package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture creates a small repo with history, a branch, a stash, and dirty files.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, body string) {
		os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
	}
	run("init", "-q", "-b", "main")
	write("a.go", "package a\n")
	run("add", ".")
	run("commit", "-qm", "first")
	write("a.go", "package a\n\nfunc A() {}\n")
	run("commit", "-qam", "second")
	run("branch", "feature")
	write("a.go", "package a\n\nfunc A() { println() }\n")
	run("stash", "-q")
	write("a.go", "package a\n\nfunc A() { return }\n")
	write("new.txt", "hello\n")
	return dir
}

func TestOpenAndLoad(t *testing.T) {
	t.Setenv("CONFIG_DIR", t.TempDir()) // don't touch the real lazygit config
	r, err := Open(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	s := r.Load()
	if len(s.Errors) > 0 {
		t.Fatalf("errors: %v", s.Errors)
	}
	if len(s.Files) != 2 || len(s.Branches) != 2 || len(s.Commits) != 2 || len(s.Stash) != 1 {
		t.Fatalf("files=%d branches=%d commits=%d stash=%d", len(s.Files), len(s.Branches), len(s.Commits), len(s.Stash))
	}
	if s.Head == nil || s.Head.Name != "main" {
		t.Fatalf("head %+v", s.Head)
	}
	var a = s.Files[0]
	if a.Path != "a.go" {
		a = s.Files[1]
	}
	if d := r.FileDiff(a); !strings.Contains(d, "+func A() { return }") {
		t.Fatalf("diff:\n%s", d)
	}
	if d := r.CommitDiff(s.Commits[0].Hash()); !strings.Contains(d, "second") {
		t.Fatalf("show:\n%s", d)
	}
	if err := r.ToggleStaged(a); err != nil {
		t.Fatal(err)
	}
	s = r.Load()
	for _, f := range s.Files {
		if f.Path == "a.go" && (!f.HasStagedChanges || f.HasUnstagedChanges) {
			t.Fatalf("a.go not staged: %+v", f)
		}
	}
}

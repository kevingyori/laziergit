package repo

import (
	"strings"
	"testing"

	"laziergit/internal/testrepo"
)

func TestOpenAndLoad(t *testing.T) {
	r, err := Open(testrepo.New(t))
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
	if d := r.CommitDiff(s.Commits[0], 0); !strings.Contains(d, "second") {
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

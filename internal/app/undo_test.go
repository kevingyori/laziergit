package app

import (
	"laziergit/internal/testrepo"
	"strings"
	"testing"
)

func TestUndoCommitAndCheckout(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	before := git(t, dir, "rev-parse", "HEAD^")
	a, tt := openApp(t, dir)
	press(tt, "z")
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/undo.png")
	}
	press(tt, "enter")
	if head := git(t, dir, "rev-parse", "HEAD"); head != before {
		t.Fatalf("head %s wanted %s", head, before)
	}
	if st := git(t, dir, "diff", "--cached", "--name-only"); !strings.Contains(st, "a.go") {
		t.Fatal("undo lost staged changes")
	}
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "checkout", "feature")
	a.Refresh()
	tt.Frame()
	press(tt, "z")
	press(tt, "enter")
	if branch := strings.TrimSpace(git(t, dir, "branch", "--show-current")); branch != "main" {
		t.Fatal(branch)
	}
}

func TestUndoRebase(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	before := git(t, dir, "rev-parse", "HEAD")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "f")
	press(tt, "enter")
	press(tt, "z")
	press(tt, "enter")
	if head := git(t, dir, "rev-parse", "HEAD"); head != before {
		t.Fatalf("head %s wanted %s; overlay %T", head, before, a.topOverlay())
	}
}

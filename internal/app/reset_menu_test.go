package app

import (
	"strings"
	"testing"

	"laziergit/internal/testrepo"
)

func TestResetMenuOnCommit(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "j")
	want := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD^"))
	press(tt, "g")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || !strings.HasPrefix(m.title, a.repo.Common.Tr.ResetTo) || len(m.items) != 3 {
		t.Fatalf("reset menu: %#v", a.topOverlay())
	}
	press(tt, "s")
	if got := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD")); got != want {
		t.Fatalf("HEAD = %s, want %s", got, want)
	}
}

func TestWorkspaceResetMenu(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "junk.txt", "x\n")
	a, tt := openApp(t, dir)
	press(tt, "2")
	press(tt, "D")
	if _, ok := a.topOverlay().(*menuOverlay); !ok {
		t.Fatalf("workspace reset menu: %T", a.topOverlay())
	}
	press(tt, "c")
	if got := git(t, dir, "status", "--porcelain", "--", "junk.txt"); strings.Contains(got, "junk") {
		t.Fatalf("untracked file kept: %q", got)
	}
}

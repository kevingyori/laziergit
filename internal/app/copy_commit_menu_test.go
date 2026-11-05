package app

import (
	"testing"

	"laziergit/internal/testrepo"
)

func TestCopyCommitAttributeMenu(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	var got string
	a.copyText = func(s string) error { got = s; return nil }
	press(tt, "4")
	press(tt, "y")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || m.title != a.repo.Common.Tr.Actions.CopyCommitAttributeToClipboard || len(m.items) != 8 {
		t.Fatalf("copy menu: %#v", a.topOverlay())
	}
	press(tt, "s")
	want := git(t, dir, "log", "-1", "--format=%s")
	if got != want[:len(want)-1] {
		t.Fatalf("copied %q, want %q", got, want)
	}
	if a.status != a.repo.Common.Tr.CommitSubjectCopiedToClipboard {
		t.Fatalf("toast = %q", a.status)
	}
}

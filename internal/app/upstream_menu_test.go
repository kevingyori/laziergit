package app

import (
	"strings"
	"testing"
	"time"

	"laziergit/internal/testrepo"
)

func TestUpstreamOptionsMenuDisabledItems(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	press(tt, "3")
	press(tt, "u")
	m, ok := a.topOverlay().(*menuOverlay)
	for i := 0; i < 50 && !ok; i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
		m, ok = a.topOverlay().(*menuOverlay)
	}
	if !ok || m.title != a.repo.Common.Tr.BranchUpstreamOptionsTitle || len(m.items) != 6 {
		t.Fatalf("upstream menu: %#v", a.topOverlay())
	}
	// A branch without an upstream can't be unset or reset onto.
	press(tt, "g")
	if !strings.HasPrefix(a.status, a.repo.Common.Tr.DisabledMenuItemPrefix) {
		t.Fatalf("status = %q", a.status)
	}
}

func TestUpstreamDivergenceOpensSelectedBranch(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "remote", "add", "origin", "https://example.com/test.git")
	git(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD~1")
	git(t, dir, "branch", "--set-upstream-to=origin/main")
	a, tt := openApp(t, dir)
	press(tt, "3")
	press(tt, "u")
	press(tt, "enter")
	if a.refBrowse == nil || a.refBrowse.ref == nil || a.refBrowse.divergenceFrom != "refs/remotes/origin/main" {
		t.Fatalf("divergence not opened: %+v", a.refBrowse)
	}
	if len(a.refBrowse.commits) != 1 || a.refBrowse.commits[0].Hash() != a.snap.Head.CommitHash {
		t.Fatalf("wanted the one local-only commit, got %+v", a.refBrowse.commits)
	}
	press(tt, "esc")
	if a.refBrowse != nil {
		t.Fatal("escape did not return to branches")
	}
}

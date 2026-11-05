package app

import (
	"laziergit/internal/testrepo"
	"testing"
)

func TestRefBrowseNavigation(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "remote", "add", "origin", "https://example.com/test.git")
	git(t, dir, "update-ref", "refs/remotes/origin/zeta", "HEAD")
	git(t, dir, "update-ref", "refs/remotes/origin/alpha", "HEAD~1")
	git(t, dir, "tag", "v1", "HEAD~1")
	a, tt := openApp(t, dir)
	press(tt, "3")
	press(tt, "j")
	parent := a.cursor[panelBranches]
	press(tt, "enter")
	if a.scope() != scopeRefCommits || len(a.refBrowse.commits) == 0 {
		t.Fatalf("branch commits: %+v", a.refBrowse)
	}
	press(tt, "C")
	if len(a.copiedCommits) != 1 {
		t.Fatal("copy from ref")
	}
	press(tt, "esc")
	if a.refBrowse != nil || a.cursor[panelBranches] != parent {
		t.Fatal("branch selection not restored")
	}
	press(tt, "]")
	press(tt, "enter")
	if a.scope() != scopeRemoteBranches || len(a.refBrowse.branches) != 2 {
		t.Fatalf("remote branches: %+v", a.refBrowse)
	}
	press(tt, "j")
	press(tt, "enter")
	if a.scope() != scopeRefCommits || a.refBrowse.ref.RefName() != a.refBrowse.branches[1].FullName() {
		t.Fatal("remote commits")
	}
	press(tt, "esc")
	if a.scope() != scopeRemoteBranches || a.cursor[panelBranches] != 1 {
		t.Fatal("remote selection not restored")
	}
	press(tt, "esc")
	if a.refBrowse != nil || a.cursor[panelBranches] != 0 {
		t.Fatal("remote parent not restored")
	}
	press(tt, "]")
	press(tt, "enter")
	if a.scope() != scopeRefCommits || a.refBrowse.ref.RefName() != "v1" {
		t.Fatal("tag commits")
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/ref-commits.png")
	}
	press(tt, "esc")
	press(tt, "enter")
	if a.cursor[panelBranches] != 0 {
		t.Fatal("sub commits selection not reset")
	}
}

package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/testrepo"
)

func TestBranchTabsKeepTheirSelection(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "branch", "extra-one")
	git(t, dir, "branch", "extra-two")
	a, tt := openApp(t, dir)
	press(tt, "3")
	press(tt, "j")
	press(tt, "j")
	want := a.cursor[panelBranches]
	press(tt, "]")
	press(tt, "[")
	if a.branchTab != 0 || a.cursor[panelBranches] != want {
		t.Fatalf("tab %d cursor %d, want 0 and %d", a.branchTab, a.cursor[panelBranches], want)
	}
}

func TestCopySelectedBranchName(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	var copied []string
	a.copyText = func(s string) error { copied = append(copied, s); return nil }
	press(tt, "3")
	tt.Key(ui.Ctrl, ui.KeyO)
	name := a.snap.Branches[0].Name
	if len(copied) != 1 || copied[0] != name || a.status != "'"+name+"' "+a.repo.Common.Tr.CopiedToClipboard {
		t.Fatalf("copied %q, status %q", copied, a.status)
	}
}

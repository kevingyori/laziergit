package app

import (
	"testing"

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

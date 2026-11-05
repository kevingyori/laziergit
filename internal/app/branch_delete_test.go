package app

import (
	"strings"
	"testing"
)

func TestDeleteBranchMenuAndForceConfirmation(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "checkout", "-b", "unmerged")
	commitFile(t, dir, "unmerged.txt", "change", "unmerged")
	git(t, dir, "checkout", "main")
	a, tt := openApp(t, dir)
	press(tt, "3")
	press(tt, "d")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || m.items[0].disabled != a.repo.Common.Tr.CantDeleteCheckOutBranch {
		t.Fatalf("checked-out branch menu: %#v", a.topOverlay())
	}
	press(tt, "enter")
	if a.topOverlay() != m {
		t.Fatal("disabled deletion closed the menu")
	}
	press(tt, "esc")
	pressUntil(t, tt, "j", func() bool { return a.selectedBranch().Name == "unmerged" })
	press(tt, "d")
	press(tt, "enter")
	c, ok := a.topOverlay().(*confirmOverlay)
	if !ok || c.title != a.repo.Common.Tr.ForceDeleteBranchTitle {
		t.Fatalf("expected force confirmation, got %#v", a.topOverlay())
	}
	press(tt, "esc")
	if !strings.Contains(git(t, dir, "branch", "--list", "unmerged"), "unmerged") {
		t.Fatal("cancel deleted branch")
	}
}

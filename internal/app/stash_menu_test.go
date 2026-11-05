package app

import (
	"strings"
	"testing"

	"laziergit/internal/testrepo"
)

func TestStashOptionsMenu(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	press(tt, "2")
	press(tt, "S")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || m.title != a.repo.Common.Tr.StashOptions || len(m.items) != 5 {
		t.Fatalf("stash options: %#v", a.topOverlay())
	}
	press(tt, "U")
	if _, ok := a.topOverlay().(*promptOverlay); !ok {
		t.Fatalf("stash message prompt: %T", a.topOverlay())
	}
	tt.Type("everything")
	press(tt, "enter")
	if got := git(t, dir, "stash", "list"); !strings.Contains(got, "everything") {
		t.Fatalf("stash list: %q", got)
	}
}

package app

import (
	"strings"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"laziergit/internal/testrepo"
)

func TestCreateFixupCommitMenu(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	testrepo.Write(t, dir, "fix.txt", "fix\n")
	git(t, dir, "add", "fix.txt")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "j")
	press(tt, "F")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || m.title != a.repo.Common.Tr.CreateFixupCommit || len(m.items) != 3 {
		t.Fatalf("fixup menu: %#v", a.topOverlay())
	}
	press(tt, "f")
	pressUntil(t, tt, "", func() bool {
		return strings.HasPrefix(git(t, dir, "log", "-1", "--format=%s"), "fixup! ")
	})
}

func TestFixupMenuOnCommit(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "f")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || m.title != a.repo.Common.Tr.Fixup || len(m.items) != 2 {
		t.Fatalf("fixup menu: %#v", a.topOverlay())
	}
}

func TestSquashFixupSelectionBelowBranchBoundary(t *testing.T) {
	commits := []*models.Commit{
		{Name: "fixup! target"}, {Name: "target"}, {Name: "older"},
		{Name: "fixup! outside"}, {Name: "outside"},
	}
	if got := countSquashableAbove(commits, 4, 1); got != 1 {
		t.Fatalf("removed %d commits, want only the fixup inside the rebase", got)
	}
}

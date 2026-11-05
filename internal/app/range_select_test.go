package app

import (
	"laziergit/internal/testrepo"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilteredRangeStagesVisibleFilesAcrossDirectories(t *testing.T) {
	dir := cleanRepo(t)
	for _, name := range []string{"alpha/keep.txt", "alpha/hidden.txt", "beta/keep.txt", "beta/hidden.txt"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		testrepo.Write(t, dir, name, name+"\n")
	}
	a, tt := openApp(t, dir)
	a.repo.Config.Gui.ShowFileTree = true
	a.newFileTrees()
	a.Refresh()
	tt.Frame()
	typeSearch(t, a, "keep")
	selectFile(t, a, "beta/keep.txt")
	press(tt, "v")
	pressUntil(t, tt, "k", func() bool { return a.selectedFileNode() != nil && a.selectedFileNode().GetPath() == "alpha" })
	press(tt, "space")
	if got := git(t, dir, "diff", "--cached", "--name-only"); got != "alpha/keep.txt\nbeta/keep.txt\n" {
		t.Fatalf("range staging touched the wrong files: %q", got)
	}
	press(tt, "space")
	if got := git(t, dir, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("range unstaging left files staged: %q", got)
	}
}

func TestRangeGuardAndEscapeBeforeFilter(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "apple-one", "1")
	testrepo.Write(t, dir, "apple-two", "2")
	a.Refresh()
	tt.Frame()
	typeSearch(t, a, "apple")
	press(tt, "v")
	press(tt, "j")
	press(tt, "enter")
	if a.staging != nil || !strings.Contains(a.status, a.repo.Common.Tr.RangeSelectNotSupported) {
		t.Fatalf("single-item action on a range: staging %v, status %q", a.staging, a.status)
	}
	press(tt, "esc")
	if a.selectingRange(panelFiles) || !a.files.IsFiltering() {
		t.Fatal("first escape should cancel only the range")
	}
	press(tt, "esc")
	if a.files.IsFiltering() {
		t.Fatal("second escape should cancel the filter")
	}
}

func TestDropSelectedCommitRange(t *testing.T) {
	dir := cleanRepo(t)
	commitFile(t, dir, "one", "one", "range one")
	commitFile(t, dir, "two", "two", "range two")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "v")
	press(tt, "j")
	press(tt, "d")
	press(tt, "enter")
	if a.topOverlay() != nil {
		t.Fatalf("drop failed: %#v", a.topOverlay())
	}
	if got := strings.TrimSpace(git(t, dir, "log", "-1", "--format=%s")); got != "second" {
		t.Fatalf("range drop left a selected commit: %q", got)
	}
	if got := strings.TrimSpace(git(t, dir, "rev-list", "--count", "HEAD")); got != "2" {
		t.Fatalf("wrong number of commits after range drop: %q", got)
	}
}

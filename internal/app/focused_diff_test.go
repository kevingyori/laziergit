package app

import (
	"laziergit/internal/testrepo"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFocusedDirectoryDiffStagesRangeAcrossFiles(t *testing.T) {
	dir := cleanRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"nested/one.txt", "nested/two words.txt"} {
		commitFile(t, dir, name, "old\n", "add "+name)
		testrepo.Write(t, dir, name, "new\n")
	}
	testrepo.Write(t, dir, "outside.txt", "leave alone\n")
	a, tt := openApp(t, dir)
	a.repo.Config.Gui.ShowFileTree = true
	a.repo.Config.Gui.UseHunkModeInDiffView = false
	a.newFileTrees()
	a.Refresh()
	tt.Frame()
	selectFile(t, a, "nested")
	press(tt, "0")
	if a.staging == nil || len(a.staging.sections) != 2 {
		t.Fatalf("directory diff: %+v", a.staging)
	}
	press(tt, "v")
	press(tt, ">")
	press(tt, "space")
	if got := git(t, dir, "diff", "--cached", "--name-only"); got != "nested/one.txt\nnested/two words.txt\n" {
		t.Fatalf("wrong staged files: %q", got)
	}
	if got := git(t, dir, "diff", "--", "nested"); got != "" {
		t.Fatalf("directory changes left unstaged: %q", got)
	}
}

func TestFocusedCommitSearchBuildsPatchWithoutChangingRepository(t *testing.T) {
	dir := cleanRepo(t)
	commitFile(t, dir, "needle.txt", "needle one\nspacer\nneedle two\n", "search fixture")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "0")
	if a.staging == nil || a.staging.worktree {
		t.Fatal("expected a focused commit diff")
	}
	typeSearch(t, a, "needle two")
	s := a.staging
	if s.cursor < 0 || !strings.Contains(s.lines[s.cursor].Content, "needle two") || s.mode != selectLine {
		t.Fatalf("search did not select its line: %+v", s)
	}
	press(tt, "space")
	if a.customPatch == nil {
		t.Fatal("commit selection did not build a patch")
	}
	if got := git(t, dir, "status", "--porcelain"); got != "" {
		t.Fatalf("building a patch changed repository: %q", got)
	}
	press(tt, "esc") // cancel search first
	press(tt, "esc") // leave focused diff
	if a.staging != nil || a.focus != panelCommits {
		t.Fatal("escape did not return to commit list")
	}
}

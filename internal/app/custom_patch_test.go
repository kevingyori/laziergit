package app

import (
	"github.com/egoist/mygo/ui"
	"laziergit/internal/testrepo"
	"strings"
	"testing"
)

func patchFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := cleanRepo(t)
	commitFile(t, dir, "file.txt", "one\ntwo\nthree\nfour\nfive\n", "base")
	commitFile(t, dir, "file.txt", "ONE\ntwo\nTHREE\nfour\nfive\n", "changes")
	return dir, git(t, dir, "rev-parse", "HEAD")
}
func TestCustomPatchTogglePreviewAndReset(t *testing.T) {
	dir, head := patchFixture(t)
	a, tt := openApp(t, dir)
	a.repo.Config.Gui.UseHunkModeInDiffView = false
	press(tt, "4")
	press(tt, "enter")
	press(tt, "0")
	press(tt, "space")
	if a.customPatch == nil || !strings.Contains(a.customPatch.preview, "-one") || strings.Contains(a.customPatch.preview, "+ONE") {
		t.Fatalf("preview: %+v; status %s", a.customPatch, a.status)
	}
	if !strings.Contains(a.staging.lines[a.staging.cursor].Content, "+ONE") {
		t.Fatal("toggle did not advance")
	}
	press(tt, "space")
	if !strings.Contains(a.customPatch.preview, "+ONE") {
		t.Fatal("addition missing")
	}
	if out := shotDir(); out != "" {
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			tt.Frame()
			suffix := "light"
			if dark {
				suffix = "dark"
			}
			shot(t, tt, out+"/custom-patch-"+suffix+".png")
		}
	}
	if git(t, dir, "rev-parse", "HEAD") != head || git(t, dir, "status", "--porcelain") != "" {
		t.Fatal("building patch changed repository")
	}
	press(tt, "tab")
	if !a.staging.staged {
		t.Fatal("preview not focused")
	}
	press(tt, "space")
	if strings.Contains(a.customPatch.preview, "-one") {
		t.Fatal("preview removal did not remove selected deletion")
	}
	tt.Key(ui.Ctrl, ui.KeyP)
	tt.Frame()
	press(tt, "c")
	if a.customPatch != nil || a.staging == nil || a.staging.staged {
		t.Fatal("reset did not return to source")
	}
}
func TestCustomPatchMovesPartialCommitIntoIndex(t *testing.T) {
	dir, _ := patchFixture(t)
	a, tt := openApp(t, dir)
	a.repo.Config.Gui.UseHunkModeInDiffView = false
	press(tt, "4")
	press(tt, "enter")
	press(tt, "0")
	press(tt, "v")
	press(tt, "j")
	press(tt, "space")
	tt.Key(ui.Ctrl, ui.KeyP)
	tt.Frame()
	press(tt, "i")
	if a.customPatch != nil {
		t.Fatalf("patch not consumed; status %s; overlay %T", a.status, a.topOverlay())
	}
	if got := git(t, dir, "show", "HEAD:file.txt"); got != "one\ntwo\nTHREE\nfour\nfive\n" {
		t.Fatalf("commit content %q", got)
	}
	if got := git(t, dir, "diff", "--cached"); !strings.Contains(got, "+ONE") || strings.Contains(got, "+THREE") {
		t.Fatalf("wrong index patch %s", got)
	}
	if got := git(t, dir, "diff"); got != "" {
		t.Fatalf("unstaged %s", got)
	}
	if a.staging == nil || strings.Contains(a.staging.text, "+ONE") || !strings.Contains(a.staging.text, "+THREE") {
		t.Fatal("focused commit diff did not refresh after rewrite")
	}
	press(tt, "2")
	selectFile(t, a, "file.txt")
	tt.Frame()
	if a.mainVisible() || !a.secondaryVisible() || !strings.Contains(a.secondaryText, "+ONE") {
		t.Fatal("staged-only diff is not in secondary pane")
	}
	press(tt, "0")
	press(tt, "a")
	press(tt, "space")
	if a.staging == nil || a.staging.staged || !strings.Contains(a.staging.text, "+ONE") {
		t.Fatal("unstaging last hunk did not follow into the unstaged pane")
	}
}
func TestFocusedSelectionSurvivesContextChanges(t *testing.T) {
	dir := cleanRepo(t)
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, string(rune('A'+i)))
	}
	commitFile(t, dir, "file.txt", strings.Join(lines, "\n")+"\n", "base")
	lines[5] = "changed six"
	lines[25] = "changed twenty six"
	testrepo.Write(t, dir, "file.txt", strings.Join(lines, "\n")+"\n")
	a, tt := openApp(t, dir)
	a.repo.Config.Gui.UseHunkModeInDiffView = false
	press(tt, "0")
	press(tt, "l")
	press(tt, "v")
	press(tt, "j")
	press(tt, "j")
	s := a.staging
	lo, hi := s.selection()
	selected := s.lines[lo].Content + "|" + s.lines[hi].Content
	press(tt, "}")
	lo, hi = s.selection()
	if got := s.lines[lo].Content + "|" + s.lines[hi].Content; got != selected {
		t.Fatalf("selection changed %q -> %q", selected, got)
	}
}

func TestCustomPatchMovesToNewCommit(t *testing.T) {
	for _, before := range []bool{false, true} {
		name := "after"
		if before {
			name = "before"
		}
		t.Run(name, func(t *testing.T) {
			dir, _ := patchFixture(t)
			a, tt := openApp(t, dir)
			a.repo.Config.Gui.UseHunkModeInDiffView = true
			press(tt, "4")
			press(tt, "enter")
			press(tt, "0")
			press(tt, "space")
			tt.Key(ui.Ctrl, ui.KeyP)
			tt.Frame()
			key := "n"
			if before {
				key = "N"
			}
			press(tt, key)
			if _, ok := a.topOverlay().(*commitOverlay); !ok {
				t.Fatalf("commit editor missing: %T", a.topOverlay())
			}
			tt.Type("split change")
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if a.topOverlay() != nil {
				t.Fatalf("operation failed: %T, %s", a.topOverlay(), a.status)
			}
			if a.customPatch != nil || a.staging != nil || a.commitFiles != nil {
				t.Fatal("patch mode was not consumed")
			}
			selected := "split change"
			if before {
				selected = "changes"
			}
			if a.selectedCommit() == nil || a.selectedCommit().Name != selected {
				t.Fatalf("new commit not selected: %+v", a.selectedCommit())
			}
			log := git(t, dir, "log", "-2", "--format=%s")
			want := "split change\nchanges\n"
			if before {
				want = "changes\nsplit change\n"
			}
			if log != want {
				t.Fatalf("log %q want %q", log, want)
			}
			if got := git(t, dir, "show", "HEAD:file.txt"); got != "ONE\ntwo\nTHREE\nfour\nfive\n" {
				t.Fatalf("tip content changed: %q", got)
			}
		})
	}
}
func TestCustomPatchWholeFileAndSourceSwitch(t *testing.T) {
	dir, _ := patchFixture(t)
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "enter")
	press(tt, "space")
	if a.customPatch == nil || !strings.Contains(a.customPatch.plain, "+ONE") || !strings.Contains(a.customPatch.plain, "+THREE") {
		t.Fatal("whole-file patch missing")
	}
	press(tt, "space")
	if a.customPatch != nil {
		t.Fatal("whole-file toggle did not reset")
	}
	press(tt, "space")
	press(tt, "esc")
	press(tt, "j")
	press(tt, "enter")
	press(tt, "space")
	if _, ok := a.topOverlay().(*confirmOverlay); !ok {
		t.Fatalf("source switch did not ask to discard old patch: %T", a.topOverlay())
	}
	old := a.customPatch.to
	press(tt, "esc")
	if a.customPatch == nil || a.customPatch.to != old {
		t.Fatal("cancel lost old patch")
	}
}

func TestCustomPatchPartialDeletion(t *testing.T) {
	dir := cleanRepo(t)
	commitFile(t, dir, "deleted.txt", "first\nsecond\nthird\n", "base")
	git(t, dir, "rm", "deleted.txt")
	git(t, dir, "commit", "-qm", "delete file")
	a, tt := openApp(t, dir)
	a.repo.Config.Gui.UseHunkModeInDiffView = false
	press(tt, "4")
	press(tt, "enter")
	press(tt, "0")
	press(tt, "j")
	press(tt, "space")
	if a.customPatch == nil || a.topOverlay() != nil || !strings.Contains(a.customPatch.preview, "-second") || strings.Contains(a.customPatch.preview, "-first") {
		t.Fatalf("partial deletion preview: %+v, overlay %T", a.customPatch, a.topOverlay())
	}
	tt.Key(ui.Ctrl, ui.KeyP)
	tt.Frame()
	press(tt, "n")
	tt.Type("move deletion")
	press(tt, "enter")
	if a.topOverlay() != nil {
		t.Fatalf("split failed: %T %s", a.topOverlay(), a.status)
	}
	if got := git(t, dir, "show", "HEAD^:deleted.txt"); got != "second\n" {
		t.Fatalf("source file contents %q", got)
	}
	if got := git(t, dir, "ls-tree", "HEAD", "deleted.txt"); got != "" {
		t.Fatalf("new commit did not delete file: %q", got)
	}
}
func TestFocusedDiffCopyAndHorizontalScroll(t *testing.T) {
	dir := cleanRepo(t)
	commitFile(t, dir, "file.txt", "old\n", "base")
	testrepo.Write(t, dir, "file.txt", "new "+strings.Repeat("word ", 90)+"\n")
	a, tt := openApp(t, dir)
	a.repo.Config.Gui.WrapLinesInDiffView = false
	a.repo.Config.Gui.UseHunkModeInDiffView = false
	copied := ""
	a.copyText = func(text string) error { copied = text; return nil }
	press(tt, "0")
	tt.Key(ui.Ctrl, ui.KeyO)
	tt.Frame()
	if copied != "old\n" {
		t.Fatalf("copy deletion %q", copied)
	}
	press(tt, "L")
	if a.staging.horizontal.X <= 0 || a.staging.horizontal.MaxX <= 0 {
		t.Fatalf("long line didn't scroll: %+v", a.staging.horizontal)
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/focused-horizontal.png")
	}
	testrepo.Write(t, dir, "file.txt", "short\n")
	a.Refresh()
	tt.Frame()
	if a.staging.horizontal.X != 0 {
		t.Fatalf("narrower diff didn't scroll back: %+v", a.staging.horizontal)
	}
	a.repo.Config.Gui.WrapLinesInDiffView = true
	testrepo.Write(t, dir, "file.txt", "wrapped "+strings.Repeat("word ", 90)+"\n")
	a.Refresh()
	tt.Frame()
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/focused-wrapped.png")
	}
}

func TestCustomPatchIgnoresEmptyCommit(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "commit", "--allow-empty", "-qm", "empty")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "enter")
	if !a.emptyCommitFiles() {
		t.Fatal("empty commit did not open its file list")
	}
	press(tt, "space")
	if a.customPatch != nil || a.topOverlay() != nil {
		t.Fatal("empty commit started a patch")
	}
}

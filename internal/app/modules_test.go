package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/testrepo"
)

func TestWorktreeManagement(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	dir = a.snap.Root
	t.Cleanup(func() { a.repo.Close() })
	press(tt, "2")
	press(tt, "]")
	if a.scope() != scopeWorktrees {
		t.Fatal(a.scope())
	}
	press(tt, "d")
	if !strings.Contains(a.status, "Cannot remove") {
		t.Fatal(a.status)
	}
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "linked tree")
	press(tt, "n")
	tt.Type(path)
	press(tt, "enter")
	// The default base is HEAD.
	press(tt, "enter")
	tt.Type("worktree-feature")
	press(tt, "enter")
	if len(a.snap.Worktrees) != 2 {
		t.Fatal(a.status, tt.Texts())
	}
	press(tt, "j")
	if a.selectedWorktree().Path != path {
		t.Fatal(a.selectedWorktree())
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/worktrees.png")
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/worktrees-dark.png")
		tt.SetDark(false)
		tt.Frame()
	}
	testrepo.Write(t, path, "untracked.txt", "keep me")
	press(tt, "d")
	press(tt, "enter")
	if a.topOverlay() == nil {
		t.Fatal("dirty worktree was removed")
	}
	if _, err := os.Stat(filepath.Join(path, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	press(tt, "esc")
	if err := os.Remove(filepath.Join(path, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	press(tt, "enter")
	if a.snap.Root != path {
		t.Fatal(a.snap.Root, tt.Texts())
	}
	// Return through the main worktree entry, then remove the linked tree.
	press(tt, "2")
	press(tt, "]")
	for i, w := range a.snap.Worktrees {
		if w.IsMain {
			a.cursor[panelFiles] = i
		}
	}
	press(tt, "enter")
	if a.snap.Root != dir {
		t.Fatal(a.snap.Root)
	}
	press(tt, "2")
	press(tt, "]")
	press(tt, "j")
	press(tt, "d")
	press(tt, "enter")
	if len(a.snap.Worktrees) != 1 {
		t.Fatal(tt.Texts())
	}
}

func TestSubmoduleManagement(t *testing.T) {
	source := testrepo.New(t)
	git(t, source, "reset", "--hard", "HEAD")
	git(t, source, "clean", "-fd")
	dir := testrepo.New(t)
	// Keep Git's default file protocol policy unchanged outside this test.
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	a, tt := openApp(t, dir)
	dir = a.snap.Root
	t.Cleanup(func() { a.repo.Close() })
	press(tt, "]")
	press(tt, "]")
	if a.scope() != scopeSubmodules || !tt.HasText("No submodules") {
		t.Fatal(tt.Texts())
	}
	press(tt, "n")
	tt.Type(source)
	press(tt, "enter")
	tt.Key(ui.Ctrl, ui.KeyU)
	tt.Type("libs/example")
	press(tt, "enter")
	press(tt, "enter") // the path suggests the name
	if len(a.snap.Submodules) != 1 {
		t.Fatal(tt.Texts())
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/submodules.png")
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/submodules-dark.png")
		tt.SetDark(false)
		tt.Frame()
	}
	git(t, dir, "commit", "-m", "Add submodule")
	press(tt, "i")
	press(tt, "u")
	git(t, dir, "submodule", "deinit", "-f", "--", "libs/example")
	press(tt, "enter")
	if a.topOverlay() == nil || a.snap.Root != dir {
		t.Fatal("opened uninitialized submodule")
	}
	press(tt, "esc")
	press(tt, "b")
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/submodule-bulk.png")
	}
	press(tt, "j")
	press(tt, "j")
	press(tt, "enter")
	if a.topOverlay() != nil {
		t.Fatal(tt.Texts())
	}
	press(tt, "enter")
	if a.snap.Root != filepath.Join(dir, "libs/example") {
		t.Fatal(a.snap.Root, tt.Texts())
	}
	press(tt, "esc")
	if a.snap.Root != dir || len(a.parents) != 0 {
		t.Fatal(a.snap.Root, a.parents)
	}
	press(tt, "]")
	press(tt, "]")
	press(tt, "e")
	// Replace the prefilled URL using the editor's select-all chord.
	press(tt, "enter")
	if a.topOverlay() != nil {
		t.Fatal(tt.Texts())
	}
	press(tt, "d")
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/remove-submodule.png")
	}
	press(tt, "enter")
	if len(a.snap.Submodules) != 0 {
		t.Fatal(tt.Texts())
	}
	if !strings.Contains(git(t, dir, "diff", "--cached", "--name-status"), "libs/example") {
		t.Fatal("removal not staged")
	}
}

func TestShotsModules(t *testing.T) {
	if shotDir() == "" {
		t.Skip("SHOT_DIR not set")
	}
	t.Run("worktrees", TestWorktreeManagement)
	t.Run("submodules", TestSubmoduleManagement)
}

func TestRepositorySwitchDiscardsOldRefresh(t *testing.T) {
	other := testrepo.New(t)
	a, _ := openApp(t, testrepo.New(t))
	t.Cleanup(a.Close)
	var pending []func()
	a.post = func(fn func()) { pending = append(pending, fn) }
	a.Refresh()
	oldPosts := pending
	pending = nil
	a.openRepository(other, false)
	// Finish the switch and its new snapshot before the old refresh arrives.
	for len(pending) > 0 {
		batch := pending
		pending = nil
		for _, fn := range batch {
			fn()
		}
	}
	root := a.snap.Root
	for _, fn := range oldPosts {
		fn()
	}
	if a.snap.Root != root || a.repo.Git.RepoPaths.WorktreePath() != root {
		t.Fatal("old refresh replaced the new repository", a.snap.Root, root)
	}
}

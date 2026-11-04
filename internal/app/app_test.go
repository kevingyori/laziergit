package app

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"laziergit/internal/repo"
	"laziergit/internal/testrepo"
)

func newTestApp(t *testing.T) (*App, *ui.Tester, string) {
	t.Helper()
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	return a, tt, dir
}

func shotDir() string { return os.Getenv("SHOT_DIR") }

func openApp(t *testing.T, dir string) (*App, *ui.Tester) {
	t.Helper()
	r, err := repo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	sync := func(fn func()) { fn() }
	quit := false
	a := New(r, sync, sync, func() { quit = true })
	_ = quit
	a.Refresh()
	tt := ui.NewTester(a.View, 1280, 800)
	tt.Frame()
	return a, tt
}

func TestRendersSnapshot(t *testing.T) {
	_, tt, _ := newTestApp(t)
	for _, want := range []string{"FILES", "BRANCHES", "COMMITS", "a.go", "new.txt", "feature", "second"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	if dir := os.Getenv("SHOT_DIR"); dir != "" {
		shot(t, tt, dir+"/light-files.png")
		tt.TypeKey(0, ui.Key4, "4")
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, dir+"/dark-commits.png")
	}
}

func TestKeyboardFollowsLazygitKeymap(t *testing.T) {
	a, tt, _ := newTestApp(t)
	if a.focus != panelFiles {
		t.Fatalf("focus %v", a.focus)
	}
	// j moves down (arrives as text, the way a real keyboard sends it).
	tt.TypeKey(0, ui.KeyJ, "j")
	if a.cursor[panelFiles] != 1 {
		t.Fatalf("j: cursor %d", a.cursor[panelFiles])
	}
	tt.TypeKey(0, ui.KeyK, "k")
	if a.cursor[panelFiles] != 0 {
		t.Fatalf("k: cursor %d", a.cursor[panelFiles])
	}
	// 4 jumps to commits; main pane follows.
	tt.TypeKey(0, ui.Key4, "4")
	if a.focus != panelCommits || !tt.HasText("second") {
		t.Fatalf("4: focus %v", a.focus)
	}
	// Arrow keys (named) work too; l / → cycle panels.
	tt.Key(0, ui.KeyRight)
	if a.focus != panelStash {
		t.Fatalf("→: focus %v", a.focus)
	}
	tt.TypeKey(0, ui.Key2, "2")
	// space stages the selected file.
	f := a.selectedFile()
	if f.Path != "a.go" {
		t.Fatalf("selected %s", f.Path)
	}
	tt.TypeKey(0, ui.KeySpace, " ")
	if f2 := a.selectedFile(); !f2.HasStagedChanges || f2.HasUnstagedChanges {
		t.Fatalf("not staged: %+v", f2)
	}
	if !tt.HasText("· 1 staged") {
		t.Errorf("staged count not shown: %q", tt.Texts())
	}
}

func TestClickKeepsKeyboard(t *testing.T) {
	a, tt, _ := newTestApp(t)
	if err := tt.Click("feature"); err != nil {
		t.Fatal(err)
	}
	if a.focus != panelBranches {
		t.Fatalf("click: focus %v", a.focus)
	}
	// After a click, single-letter keys must still reach the dispatcher.
	tt.TypeKey(0, ui.KeyK, "k")
	tt.TypeKey(0, ui.KeyJ, "j")
	if a.focus != panelBranches || a.cursor[panelBranches] != 1 {
		t.Fatalf("keys after click: focus %v cursor %d", a.focus, a.cursor[panelBranches])
	}
}

func shot(t *testing.T, tt *ui.Tester, path string) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, tt.Image())
}

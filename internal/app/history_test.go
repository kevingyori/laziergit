package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/app/daemon"
	"github.com/jesseduffield/lazygit/pkg/logs"
	"laziergit/internal/testrepo"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Rebase sequence editors re-enter the current executable, including in tests.
func TestMain(m *testing.M) {
	if daemon.InDaemonMode() {
		daemon.Handle(logs.NewProductionLogger())
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestHistoryRewrite(t *testing.T) {
	for _, action := range []string{"fixup", "squash", "reword", "edit"} {
		t.Run(action, func(t *testing.T) {
			dir := testrepo.New(t)
			git(t, dir, "reset", "--hard", "HEAD")
			git(t, dir, "clean", "-fd")
			a, tt := openApp(t, dir)
			press(tt, "4")
			switch action {
			case "reword":
				press(tt, "r")
				o, ok := a.topOverlay().(*commitOverlay)
				if !ok {
					t.Fatalf("editor: %T", a.topOverlay())
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("Renamed")
				tt.Frame()
				_ = o
				press(tt, "enter")
				if msg := git(t, dir, "log", "-1", "--format=%s"); strings.TrimSpace(msg) != "Renamed" {
					t.Fatal(msg)
				}
			case "edit":
				press(tt, "e")
				if a.topOverlay() != nil {
					t.Fatalf("edit failed: %T", a.topOverlay())
				}
				press(tt, "m")
				press(tt, "c")
				if a.topOverlay() != nil {
					t.Fatalf("continue failed: %T", a.topOverlay())
				}
			default:
				key := "f"
				if action == "squash" {
					key = "s"
				}
				press(tt, key)
				press(tt, "enter")
				if n := strings.TrimSpace(git(t, dir, "rev-list", "--count", "HEAD")); n != "1" {
					t.Fatalf("count %s; overlay %T", n, a.topOverlay())
				}
				if tree := git(t, dir, "show", "HEAD:a.go"); !strings.Contains(tree, "func A()") {
					t.Fatal(tree)
				}
			}
		})
	}
}

func TestCherryPick(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "C")
	if len(a.copiedCommits) != 1 {
		t.Fatal("commit not copied")
	}
	git(t, dir, "checkout", "-b", "target", "HEAD^")
	a.Refresh()
	tt.Frame()
	press(tt, "V")
	press(tt, "enter")
	if msg := strings.TrimSpace(git(t, dir, "log", "-1", "--format=%s")); msg != "second" {
		t.Fatal(msg)
	}
	if len(a.copiedCommits) != 0 {
		t.Fatal("copy not cleared")
	}
}

func TestConflictResolution(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	git(t, dir, "checkout", "-b", "other", "HEAD^")
	testrepo.Write(t, dir, "a.go", "other\n")
	git(t, dir, "commit", "-am", "other")
	git(t, dir, "checkout", "main")
	// A conflict is expected here.
	cmd := exec.Command("git", "-C", dir, "merge", "other")
	if cmd.Run() == nil {
		t.Fatal("expected conflict")
	}
	a, tt := openApp(t, dir)
	press(tt, "enter")
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/conflict.png")
	}
	if _, ok := a.topOverlay().(*conflictOverlay); !ok {
		t.Fatalf("conflict view: %T", a.topOverlay())
	}
	press(tt, "space")
	if st := git(t, dir, "diff", "--name-only", "--diff-filter=U"); st != "" {
		t.Fatal(st)
	}
	if b := git(t, dir, "show", ":a.go"); !strings.Contains(b, "func A()") {
		t.Fatal(b)
	}
}

func TestReorderCommit(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	testrepo.Write(t, dir, "independent.txt", "third\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "third")
	a, tt := openApp(t, dir)
	press(tt, "4")
	tt.Key(ui.Ctrl, ui.KeyJ)
	tt.Frame()
	press(tt, "enter")
	if msg := strings.TrimSpace(git(t, dir, "log", "-1", "--format=%s")); msg != "second" {
		t.Fatalf("%s; overlay %T", msg, a.topOverlay())
	}
}

func TestRewordAncestor(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	_, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "j")
	press(tt, "r")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Root renamed")
	tt.Frame()
	press(tt, "enter")
	if msg := strings.TrimSpace(git(t, dir, "log", "--reverse", "--format=%s")); msg != "Root renamed\nsecond" {
		t.Fatal(msg)
	}
}

func TestVisibleRebaseWorkflow(t *testing.T) {
	dir := cleanRepo(t)
	commitFile(t, dir, "three", "3\n", "third")
	commitFile(t, dir, "four", "4\n", "fourth")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "j")
	press(tt, "j")
	stopped := a.selectedCommit().Hash()
	press(tt, "e")
	if a.snap.Operation != "Rebasing" || a.selectedCommit().Hash() != stopped || a.selectedCommit().IsTODO() {
		t.Fatalf("lost stopping point: op %q commit %+v", a.snap.Operation, a.selectedCommit())
	}
	press(tt, "k")
	picked := a.selectedCommit().Hash()
	press(tt, "e")
	if a.selectedCommit().Action.String() != "edit" {
		t.Fatal("todo not edited")
	}
	tt.Key(ui.Ctrl, ui.KeyK)
	tt.Frame()
	if a.selectedCommit().Hash() != picked || a.cursor[panelCommits] != 0 {
		t.Fatal("moving todo lost selection")
	}
	press(tt, "p")
	if a.selectedCommit().Action.String() != "pick" {
		t.Fatal("todo not picked")
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/visible-rebase.png")
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/visible-rebase-dark.png")
	}
	press(tt, "m")
	if _, ok := a.topOverlay().(*menuOverlay); !ok {
		t.Fatalf("operation menu: %T", a.topOverlay())
	}
	press(tt, "c")
	if a.snap.Operation != "" || a.topOverlay() != nil {
		t.Fatalf("continue: %q %T", a.snap.Operation, a.topOverlay())
	}
}

func TestFocusedConflictNavigationUndo(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	git(t, dir, "checkout", "-b", "other")
	testrepo.Write(t, dir, "conflict.txt", "bottom one\ncontext\nbottom two\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "other")
	git(t, dir, "checkout", "main")
	testrepo.Write(t, dir, "conflict.txt", "top one\ncontext\ntop two\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "main")
	exec.Command("git", "-C", dir, "merge", "other").Run()
	testrepo.Write(t, dir, "conflict.txt", "<<<<<<< HEAD\ntop one\n=======\nbottom one\n>>>>>>> other\n"+strings.Repeat("context\n", 40)+"<<<<<<< HEAD\ntop two\n=======\nbottom two\n>>>>>>> other\n")
	a, tt := openApp(t, dir)
	a.mergeStartedInLazygit = true // the continue prompt is for operations started here
	press(tt, "enter")
	v, ok := a.topOverlay().(*conflictOverlay)
	if !ok {
		t.Fatalf("%T", a.topOverlay())
	}
	original := v.state.GetContent()
	press(tt, "l")
	first, last := v.list.Visible()
	if n := v.state.GetSelectedLine() - 1; n < first || n > last {
		t.Fatalf("selected line %d outside viewport %d..%d", n, first, last)
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/conflict-navigation.png")
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/conflict-navigation-dark.png")
		tt.SetDark(false)
		tt.Frame()
	}
	press(tt, "j")
	press(tt, "space")
	if v.state.NoConflicts() {
		t.Fatal("first conflict should remain")
	}
	if !strings.Contains(v.state.GetContent(), "bottom two") || strings.Contains(v.state.GetContent(), "top two") {
		t.Fatal(v.state.GetContent())
	}
	press(tt, "z")
	if v.state.GetContent() != original {
		t.Fatal("undo did not restore original content")
	}
	if got := git(t, dir, "diff", "--name-only", "--diff-filter=U"); strings.TrimSpace(got) != "conflict.txt" {
		t.Fatalf("undo must retain the unmerged index: %q", got)
	}
	press(tt, "l")
	press(tt, "j")
	press(tt, "space")
	press(tt, "h")
	press(tt, "b")
	if !v.state.NoConflicts() {
		t.Fatal("expected resolved conflict")
	}
	if got := git(t, dir, "show", ":conflict.txt"); !strings.Contains(got, "bottom one") {
		t.Fatal(got)
	}
	if _, ok := a.topOverlay().(*confirmOverlay); !ok {
		t.Fatalf("continue prompt: %T", a.topOverlay())
	}
	press(tt, "esc")
	if a.topOverlay() != nil {
		t.Fatalf("cancel should return to files: %T", a.topOverlay())
	}
}

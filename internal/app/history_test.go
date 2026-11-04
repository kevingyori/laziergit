package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/app/daemon"
	"github.com/jesseduffield/lazygit/pkg/logs"
	"laziergit/internal/testrepo"
	"os"
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
				press(tt, "i")
				press(tt, "enter")
				press(tt, "enter")
				if a.topOverlay() != nil {
					t.Fatalf("edit failed: %T", a.topOverlay())
				}
				press(tt, "i")
				for j := 0; j < 5; j++ {
					press(tt, "j")
				}
				press(tt, "enter")
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

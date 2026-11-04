package app

import (
	"path/filepath"
	"testing"

	"laziergit/internal/testrepo"
)

// TestShots renders the main surfaces for visual review. Run with
// SHOT_DIR=/some/dir.
func TestShots(t *testing.T) {
	out := shotDir()
	if out == "" {
		t.Skip("SHOT_DIR not set")
	}
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "a.go", "package a\n\nimport \"fmt\"\n\n// A says hello.\nfunc A() {\n\tfmt.Println(\"hello\")\n}\n\nfunc B() int {\n\treturn 42\n}\n")
	git(t, dir, "remote", "add", "origin", filepath.Join(t.TempDir(), "r.git"))
	a, tt := openApp(t, dir)
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		suffix := "-light"
		if dark {
			suffix = "-dark"
		}
		// staging with a range
		press(tt, "2")
		press(tt, "enter")
		press(tt, "j")
		press(tt, "v")
		press(tt, "j")
		press(tt, "j")
		shot(t, tt, out+"/staging"+suffix+".png")
		press(tt, "esc")
		press(tt, "esc")
		// keybindings menu
		press(tt, "?")
		shot(t, tt, out+"/keybindings"+suffix+".png")
		press(tt, "esc")
		// commit editor
		press(tt, "space")
		press(tt, "c")
		tt.Type("Say hello from A and return the answer")
		press(tt, "tab")
		tt.Type("Explains the greeting.\n\nAlso returns 42.")
		tt.Frame()
		shot(t, tt, out+"/commit"+suffix+".png")
		press(tt, "esc")
		press(tt, "space")
		// push prompt
		press(tt, "P")
		shot(t, tt, out+"/push"+suffix+".png")
		press(tt, "esc")
		// error
		press(tt, "3")
		press(tt, "c")
		tt.Type("nope")
		press(tt, "enter")
		shot(t, tt, out+"/error"+suffix+".png")
		press(tt, "esc")
		// hover card over the second commit
		press(tt, "4")
		if r, ok := tt.Find("first"); ok {
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Advance(600e6)
			tt.Frame()
			tt.Advance(600e6)
			tt.Frame()
		}
		shot(t, tt, out+"/hover"+suffix+".png")
		tt.Move(900, 700)
		press(tt, "2")
	}
	_ = a
}

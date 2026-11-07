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
		selectFile(t, a, "a.go")
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
		// commit editor (the light pass's draft is kept on esc, like lazygit's)
		a.draft = commitDraft{}
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
		// interactive rebase controls
		press(tt, "4")
		press(tt, "i")
		shot(t, tt, out+"/rebase"+suffix+".png")
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
		// screen modes: half and full with commits focused, then full with
		// the main view focused
		press(tt, "+")
		shot(t, tt, out+"/half"+suffix+".png")
		press(tt, "+")
		shot(t, tt, out+"/full"+suffix+".png")
		press(tt, "0")
		shot(t, tt, out+"/full-main"+suffix+".png")
		press(tt, "esc")
		press(tt, "+")
		press(tt, "2")
	}
	_ = a
}

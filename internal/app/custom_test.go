package app

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomCommand(t *testing.T) {
	a, tt, dir := newTestApp(t)
	a.repo.Config.CustomCommands = []config.CustomCommand{{Key: config.Keybinding{"X"}, Context: "files", Command: "printf '%s' {{.Form.Value | quote}} > custom.txt", Prompts: []config.CustomCommandPrompt{{Type: "input", Key: "Value", Title: "Value"}}}}
	a.actions = a.buildActions()
	press(tt, "X")
	tt.Type("literal $(touch unsafe) ' value")
	press(tt, "enter")
	if text := git(t, dir, "status", "--porcelain"); !strings.Contains(text, "custom.txt") {
		t.Fatal(text)
	}
	b, err := os.ReadFile(filepath.Join(dir, "custom.txt"))
	if err != nil || string(b) != "literal $(touch unsafe) ' value" {
		t.Fatalf("%q %v", b, err)
	}
}

func TestCustomCommandMenuFromCommand(t *testing.T) {
	a, tt, dir := newTestApp(t)
	a.repo.Config.CustomCommands = []config.CustomCommand{{Key: config.Keybinding{"X"}, Context: "files", Command: "printf '%s' {{.Form.Value | quote}} > choice.txt", Prompts: []config.CustomCommandPrompt{{Type: "menuFromCommand", Key: "Value", Title: "Choose", Command: "printf 'one\\ntwo\\n'", Filter: "(?P<word>.*)", ValueFormat: "{{.word}}"}}}}
	a.actions = a.buildActions()
	press(tt, "X")
	press(tt, "j")
	press(tt, "enter")
	b, err := os.ReadFile(filepath.Join(dir, "choice.txt"))
	if err != nil || string(b) != "two" {
		t.Fatalf("%q %v", b, err)
	}
}

func TestShotsCustom(t *testing.T) {
	out := shotDir()
	if out == "" {
		t.Skip("SHOT_DIR not set")
	}
	a, tt, _ := newTestApp(t)
	a.repo.Config.CustomCommands = []config.CustomCommand{{Key: config.Keybinding{"X"}, Context: "files", Description: "Repository summary", Command: "git log -3 --oneline", Output: "popup", OutputTitle: "Recent commits"}}
	a.actions = a.buildActions()
	press(tt, "X")
	shot(t, tt, out+"/custom-output.png")
}

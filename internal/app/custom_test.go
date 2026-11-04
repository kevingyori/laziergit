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

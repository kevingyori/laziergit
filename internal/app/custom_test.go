package app

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	"os"
	"os/exec"
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

func TestCustomCommandTerminal(t *testing.T) {
	a, tt, dir := newTestApp(t)
	a.terminalCommand = func(command string) error {
		script := filepath.Join(dir, "terminal.command")
		result := filepath.Join(dir, "terminal result")
		if err := os.WriteFile(script, []byte(a.terminalScript(command, result)), 0700); err != nil {
			return err
		}
		if output, err := exec.Command("/bin/sh", script).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v", output, err)
		}
		code, err := os.ReadFile(result)
		if err != nil || string(code) != "0" {
			t.Fatalf("exit: %q %v", code, err)
		}
		return nil
	}
	a.repo.Config.CustomCommands = []config.CustomCommand{{Key: config.Keybinding{"X"}, Context: "files", Command: "printf '%s' " + a.repo.OS.Quote("literal '$HOME' \" value") + " > terminal.txt", Output: "terminal"}}
	a.actions = a.buildActions()
	press(tt, "X")
	data, err := os.ReadFile(filepath.Join(dir, "terminal.txt"))
	if err != nil || string(data) != "literal '$HOME' \" value" {
		t.Fatalf("%q %v", data, err)
	}
	if !strings.Contains(git(t, dir, "status", "--porcelain"), "terminal.txt") {
		t.Fatal("terminal command did not refresh")
	}
}

func TestCustomCommandSubmenuSpecialKeys(t *testing.T) {
	a, tt, _ := newTestApp(t)
	a.repo.Config.CustomCommands = []config.CustomCommand{{Key: config.Keybinding{"X"}, Description: "Commands", CommandMenu: []config.CustomCommand{
		{Key: config.Keybinding{"j"}, Context: "files", Command: "printf j", Output: "popup"},
		{Key: config.Keybinding{"K"}, Context: "commits", Command: "printf wrong", Output: "popup"},
	}}}
	a.actions = a.buildActions()
	press(tt, "X")
	menu, ok := a.topOverlay().(*menuOverlay)
	if !ok || len(menu.items) != 1 {
		t.Fatalf("context menu: %#v", a.topOverlay())
	}
	press(tt, "j")
	if _, ok := a.topOverlay().(*confirmOverlay); !ok {
		t.Fatalf("j did not execute command: %T", a.topOverlay())
	}
}

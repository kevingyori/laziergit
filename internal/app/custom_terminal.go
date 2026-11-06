package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
)

// A command file gives interactive programs a real terminal and keeps the GUI
// responsive. The exit marker also lets us refresh after the shell finishes.
func (a *App) runTerminal(command string) error {
	if a.terminalCommand != nil {
		return a.terminalCommand(command)
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("external terminal integration is unavailable on %s", runtime.GOOS)
	}
	dir, err := os.MkdirTemp("", "laziergit-terminal-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	result := filepath.Join(dir, "result")
	script := filepath.Join(dir, "command.command")
	content := a.terminalScript(command, result)
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		return err
	}
	if err := exec.Command("open", "-a", "Terminal", script).Run(); err != nil {
		return err
	}
	for {
		data, err := os.ReadFile(result)
		if err == nil {
			code, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				return err
			}
			if code != 0 {
				return fmt.Errorf("custom command exited with status %d", code)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (a *App) terminalScript(command, result string) string {
	quote := a.repo.OS.Quote
	trap := "code=$?; printf '%s' \"$code\" > " + quote(result+".tmp") + "; mv " + quote(result+".tmp") + " " + quote(result)
	content := "#!/bin/sh\ntrap " + quote(trap) + " EXIT\ntrap 'exit 129' HUP\ntrap 'exit 130' INT\ntrap 'exit 143' TERM\ncd " + quote(a.repo.Git.RepoPaths.WorktreePath()) + " || exit\n"
	shell := a.repo.OS.Cmd.NewShell(command, a.repo.Config.OS.ShellFunctionsFile)
	args := shell.Args()
	for i := range args {
		args[i] = quote(args[i])
	}
	return content + strings.Join(args, " ") + "\n"
}

// subprocessCommand is a git command object as one shell command for
// runTerminal, keeping the environment it adds (lazygit's daemon
// instructions, GIT_EDITOR).
func (a *App) subprocessCommand(cmd *oscommands.CmdObj) string {
	base := map[string]bool{}
	for _, kv := range os.Environ() {
		base[kv] = true
	}
	var parts []string
	for _, kv := range cmd.GetEnvVars() {
		if !base[kv] {
			parts = append(parts, a.repo.OS.Quote(kv))
		}
	}
	if len(parts) > 0 {
		parts = append([]string{"env"}, parts...)
	}
	for _, arg := range cmd.Args() {
		parts = append(parts, a.repo.OS.Quote(arg))
	}
	return strings.Join(parts, " ")
}

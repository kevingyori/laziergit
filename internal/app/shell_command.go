package app

import (
	"slices"
	"strings"

	"laziergit/internal/keymap"
)

// shellCommandAction is lazygit's ExecuteShellCommand (":"): prompt for a
// command, run it through the shell and refresh. lazygit hands the terminal
// to the command; here its output goes to the command log and a failure to an
// error popup, as for custom commands.
func (a *App) shellCommandAction() action {
	return action{scopeGlobal, keymap.Parse(a.repo.Config.Keybinding.Universal.ExecuteShellCommand),
		a.repo.Common.Tr.ExecuteShellCommand, a.shellCommand, false}
}

func (a *App) shellCommand() {
	o := &promptOverlay{title: a.repo.Common.Tr.ShellCommand, onSubmit: func(command string) {
		if strings.TrimSpace(command) == "" {
			return
		}
		if !strings.HasPrefix(command, " ") {
			history := a.repo.Common.AppState.ShellCommandsHistory
			history = slices.DeleteFunc(history, func(v string) bool { return v == command })
			history = append([]string{command}, history...)
			if len(history) > 1000 {
				history = history[:1000]
			}
			a.repo.Common.AppState.ShellCommandsHistory = history
			if err := a.repo.AppConfig.SaveAppState(); err != nil {
				a.showError("Command history", err)
			}
		}
		a.run(a.repo.Common.Tr.Actions.CustomCommand, func() error {
			_, err := a.repo.RunCustom(command)
			return err
		}, nil)
	}}
	o.suggestions = func(input string) []string {
		return a.suggestionFilter(a.repo.Common.AppState.ShellCommandsHistory)(input)
	}
	o.editSuggestion = true
	o.deleteSuggestion = func(value string) {
		a.repo.Common.AppState.ShellCommandsHistory = slices.DeleteFunc(a.repo.Common.AppState.ShellCommandsHistory, func(v string) bool { return v == value })
		if err := a.repo.AppConfig.SaveAppState(); err != nil {
			a.showError("Command history", err)
		}
	}
	a.pushOverlay(o)
}

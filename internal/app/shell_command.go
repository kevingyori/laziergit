package app

import (
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
	a.pushOverlay(&promptOverlay{title: a.repo.Common.Tr.ShellCommand, onSubmit: func(command string) {
		if strings.TrimSpace(command) == "" {
			return
		}
		a.run(a.repo.Common.Tr.Actions.CustomCommand, func() error {
			_, err := a.repo.RunCustom(command)
			return err
		}, nil)
	}})
}

package app

import (
	"fmt"

	"github.com/jesseduffield/lazygit/pkg/utils"
)

func (a *App) undoFlow() {
	a.bg(func() {
		plan, err := a.repo.PlanUndo()
		a.post(func() {
			if err != nil {
				a.showError("Undo", err)
				return
			}
			tr := a.repo.Common.Tr
			prompt := fmt.Sprintf(tr.SoftResetPrompt, utils.ShortHash(plan.Target))
			switch plan.Kind {
			case "rebase":
				prompt = fmt.Sprintf(tr.HardResetAutostashPrompt, utils.ShortHash(plan.Target))
			case "checkout":
				prompt = fmt.Sprintf(tr.CheckoutAutostashPrompt, plan.Target)
			}
			a.confirm(a.repo.Common.Tr.Actions.Undo, prompt, true, func() {
				a.run("Undoing…", func() error { return a.repo.Undo(plan) }, a.done("Undid last action"))
			})
		})
	})
}

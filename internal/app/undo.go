package app

import (
	"fmt"

	"laziergit/internal/repo"

	"github.com/jesseduffield/lazygit/pkg/utils"
)

// undoFlow and redoFlow are lazygit's UndoController.reflogUndo/reflogRedo.
func (a *App) undoFlow() { a.reflogUndoRedo(false) }
func (a *App) redoFlow() { a.reflogUndoRedo(true) }

func (a *App) reflogUndoRedo(redo bool) {
	a.bg(func() {
		var plan *repo.UndoPlan
		var err error
		if redo {
			plan, err = a.repo.PlanRedo()
		} else {
			plan, err = a.repo.PlanUndo()
		}
		a.post(func() {
			tr := a.repo.Common.Tr
			if err != nil {
				a.pushOverlay(&errorOverlay{title: tr.Error, body: err.Error()})
				return
			}
			if plan == nil {
				return
			}
			title, status := tr.Actions.Undo, tr.UndoingStatus
			if redo {
				title, status = tr.Actions.Redo, tr.RedoingStatus
			}
			switch {
			case plan.Kind == "checkout":
				ref := plan.From
				if redo {
					ref = plan.To
				}
				a.confirm(title, fmt.Sprintf(tr.CheckoutAutostashPrompt, ref), false, func() {
					a.checkoutRefWith(ref, status, func() error { return a.repo.UndoCheckout(ref, redo) })
				})
			case plan.Kind == "commit" && !redo:
				a.confirm(title, fmt.Sprintf(tr.SoftResetPrompt, utils.ShortHash(plan.From)), false, func() {
					a.run(status, func() error { return a.repo.UndoReset(plan.From, "soft", false) }, a.resetSelection)
				})
			default:
				hash := plan.From
				if redo {
					hash = plan.To
				}
				a.confirm(title, fmt.Sprintf(tr.HardResetAutostashPrompt, utils.ShortHash(hash)), false, func() {
					a.run(status, func() error { return a.repo.UndoReset(hash, "hard", redo) }, a.resetSelection)
				})
			}
		})
	})
}

// resetSelection is ResetToRef's SelectHeadCommit refresh.
func (a *App) resetSelection() {
	a.commitTabCursor = [2]int{}
	if a.commitsView() <= cvReflog {
		a.cursor[panelCommits] = 0
	}
}

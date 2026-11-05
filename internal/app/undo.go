package app

func (a *App) undoFlow() {
	a.bg(func() {
		plan, err := a.repo.PlanUndo()
		a.post(func() {
			if err != nil {
				a.showError("Undo", err)
				return
			}
			effect := "Move HEAD back; keep all file changes staged."
			if plan.Kind == "rebase" {
				effect = "Restore the pre-rebase history, preserving local edits when possible. Git refuses overlapping edits."
			}
			if plan.Kind == "checkout" {
				effect = "Return to the previous branch or commit, preserving local edits when possible."
			}
			a.confirm(a.repo.Common.Tr.Undo, plan.Description+"\n\nTarget: "+plan.Target+"\n"+effect, true, func() {
				a.run("Undoing…", func() error { return a.repo.Undo(plan) }, a.done("Undid last action"))
			})
		})
	})
}

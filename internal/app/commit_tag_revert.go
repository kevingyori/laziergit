package app

import "github.com/jesseduffield/lazygit/pkg/utils"

func (a *App) createCommitTag() {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	ref := c.Hash()
	tr := a.repo.Common.Tr
	a.pushOverlay(&commitOverlay{title: tr.TagNameTitle, descriptionTitle: tr.TagMessageTitle, draft: &commitDraft{}, onSubmit: func(name, description string) {
		a.bg(func() {
			exists := a.repo.Git.Tag.HasTag(name)
			a.post(func() {
				create := func() {
					a.run(tr.CreatingTag, func() error { return a.repo.CreateTag(name, ref, description, exists) }, nil)
				}
				if !exists {
					create()
					return
				}
				prompt := utils.ResolvePlaceholderString(tr.ForceTagPrompt, map[string]string{"tagName": name, "cancelKey": a.repo.Config.Keybinding.Universal.Return.String(), "confirmKey": a.repo.Config.Keybinding.Universal.Confirm.String()})
				a.confirm(tr.ForceTag, prompt, false, create)
			})
		})
	}})
}

func (a *App) revertCommit() {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	tr := a.repo.Common.Tr
	prompt := utils.ResolvePlaceholderString(tr.ConfirmRevertCommit, map[string]string{"selectedCommit": c.ShortHash()})
	dirty := false
	for _, f := range a.snap.Files {
		if !f.IsSubmodule(a.snap.Submodules) && (f.HasStagedChanges || (f.Tracked && f.HasUnstagedChanges)) {
			dirty = true
			break
		}
	}
	a.confirm(tr.Actions.RevertCommit, prompt, false, func() {
		a.run(tr.RevertingStatus, func() error { return a.repo.RevertCommit(c.Hash(), c.IsMerge(), dirty, tr.AutoStashForReverting) }, func() { a.cursor[panelCommits] = clamp(a.cursor[panelCommits]+1, 0, a.count(panelCommits)-1) })
	})
}

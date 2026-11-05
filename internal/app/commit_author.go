package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"laziergit/internal/keymap"
)

func (a *App) amendCommitAttribute() {
	if a.selectedCommit() == nil {
		return
	}
	commits := a.snap.Commits
	start, end := a.selectionRange(panelCommits)
	tr, keys := a.repo.Common.Tr, a.repo.Config.Keybinding.AmendAttribute
	run := func(kind, value string) {
		a.run(tr.AmendingStatus, func() error { return a.repo.AmendAttribute(commits, start, end, kind, value) }, nil)
	}
	prompt := func(title, kind string) {
		a.promptSuggestions(title, "", "", "authors", func(value string) { run(kind, value) })
	}
	a.pushOverlay(&menuOverlay{title: tr.AmendCommitAttribute, items: []menuItem{
		{label: tr.ResetAuthor, key: keymap.Parse(keys.ResetAuthor).Label(), chord: keymap.Parse(keys.ResetAuthor), run: func() { run("reset", "") }},
		{label: tr.SetAuthor, key: keymap.Parse(keys.SetAuthor).Label(), chord: keymap.Parse(keys.SetAuthor), run: func() { prompt(tr.SetAuthorPromptTitle, "set") }},
		{label: tr.AddCoAuthor, key: keymap.Parse(keys.AddCoAuthor).Label(), chord: keymap.Parse(keys.AddCoAuthor), run: func() { prompt(tr.AddCoAuthorPromptTitle, "coauthor") }},
	}})
}

func (a *App) amendSelectedCommit() {
	if a.selectedCommit() == nil {
		return
	}
	commits, i := a.snap.Commits, a.cursor[panelCommits]
	if a.snap.Operation == "Rebasing" && !models.IsHeadCommit(commits, i) {
		a.setStatus(a.repo.Common.Tr.DisabledMenuItemPrefix+a.repo.Common.Tr.AlreadyRebasing, true)
		return
	}
	run := func() {
		a.run(a.repo.Common.Tr.AmendingStatus, func() error { return a.repo.AmendTo(commits, i) }, nil)
	}
	if a.repo.Config.Gui.SkipAmendWarning {
		run()
		return
	}
	a.confirm(a.repo.Common.Tr.AmendCommitTitle, a.repo.Common.Tr.AmendCommitPrompt, false, run)
}

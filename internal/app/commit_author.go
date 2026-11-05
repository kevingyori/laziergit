package app

import "laziergit/internal/keymap"

func (a *App) amendCommitAttribute() {
	if a.selectedCommit() == nil {
		return
	}
	commits, i := a.snap.Commits, a.cursor[panelCommits]
	start, end := i, i
	if a.commitRangeActive {
		start, end = min(i, a.commitRangeAnchor), max(i, a.commitRangeAnchor)
	}
	tr, keys := a.repo.Common.Tr, a.repo.Config.Keybinding.AmendAttribute
	run := func(kind, value string) {
		a.run(tr.AmendingStatus, func() error { return a.repo.AmendAttribute(commits, start, end, kind, value) }, nil)
	}
	prompt := func(title, kind string) { a.prompt(title, "", "", func(value string) { run(kind, value) }) }
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
	run := func() {
		a.run(a.repo.Common.Tr.AmendingStatus, func() error { return a.repo.AmendTo(commits, i) }, nil)
	}
	if a.repo.Config.Gui.SkipAmendWarning {
		run()
		return
	}
	a.confirm(a.repo.Common.Tr.AmendCommitTitle, a.repo.Common.Tr.AmendCommitPrompt, false, run)
}

func (a *App) toggleCommitRange() {
	if a.selectedCommit() == nil {
		return
	}
	a.commitRangeActive = !a.commitRangeActive
	a.commitRangeSticky = true
	a.commitRangeAnchor = a.cursor[panelCommits]
}

// extendCommitRange is lazygit's shift+up/down: start a non-sticky range at
// the cursor if none is active, then move the cursor end.
func (a *App) extendCommitRange(d int) {
	if a.selectedCommit() == nil || a.commitFiles != nil {
		return
	}
	if !a.commitRangeActive {
		a.commitRangeActive, a.commitRangeSticky = true, false
		a.commitRangeAnchor = a.cursor[panelCommits]
	}
	n := a.count(panelCommits)
	a.cursor[panelCommits] = clamp(a.cursor[panelCommits]+d, 0, n-1)
	a.lists[panelCommits].ScrollIntoView(a.cursor[panelCommits])
	a.loadMain()
}

// selectedCommitRange returns the selected commits' index range.
func (a *App) selectedCommitRange() (start, end int) {
	i := a.cursor[panelCommits]
	if a.commitRangeActive {
		return min(i, a.commitRangeAnchor), max(i, a.commitRangeAnchor)
	}
	return i, i
}

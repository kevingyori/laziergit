package app

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/utils"
)

// deleteBrowsedRemoteBranch is lazygit's d in remoteBranches
// (BranchesHelper.ConfirmDeleteRemote for one branch).
func (a *App) deleteBrowsedRemoteBranch() {
	s := a.browse(panelBranches)
	if s == nil || s.ref != nil || len(s.branches) == 0 {
		return
	}
	b := s.branches[a.cursor[panelBranches]]
	tr := a.repo.Common.Tr
	title := utils.ResolvePlaceholderString(tr.DeleteBranchTitle, map[string]string{"selectedBranchName": b.Name})
	msg := utils.ResolvePlaceholderString(tr.DeleteRemoteBranchPrompt, map[string]string{"selectedBranchName": b.Name, "upstream": b.RemoteName})
	a.confirm(title, msg, true, func() {
		a.run(tr.DeletingStatus, func() error { return a.repo.DeleteRemoteBranch(b.RemoteName, b.Name) }, a.reloadBrowsedRemote)
	})
}

// reloadBrowsedRemote refreshes the remote's branch list after a change.
func (a *App) reloadBrowsedRemote() {
	s, r := a.browse(panelBranches), a.repo
	if s == nil || s.ref != nil {
		return
	}
	a.bg(func() {
		branches, err := r.RemoteBranches(s.remote)
		a.post(func() {
			if a.browse(panelBranches) != s || err != nil {
				return
			}
			s.branches, s.all = branches, branches
			a.filterSnap()
			a.cursor[panelBranches] = clamp(a.cursor[panelBranches], 0, max(len(branches)-1, 0))
			a.diffKey = ""
			a.loadMain()
		})
	})
}

// newBranchFromBrowsedRemote is lazygit's n in remoteBranches.
func (a *App) newBranchFromBrowsedRemote() {
	s := a.browse(panelBranches)
	if s == nil || s.ref != nil || len(s.branches) == 0 {
		return
	}
	b := s.branches[a.cursor[panelBranches]]
	from := b.RefName()
	suggested := strings.SplitAfterN(from, "/", 2)[1]
	tr := a.repo.Common.Tr
	title := utils.ResolvePlaceholderString(tr.NewBranchNameBranchOff, map[string]string{"branchName": from})
	a.prompt(title, suggested, "", func(response string) {
		name := strings.ReplaceAll(response, " ", "-") // lazygit's SanitizedBranchName
		a.run(tr.CreatingBranchStatus, func() error { return a.repo.NewBranchFromRemote(name, from, name == suggested) }, func() {
			a.refBrowse = nil
			a.branchTab = 0
			a.cursor[panelBranches] = 0
			a.lists[panelBranches].ScrollTo(0, 0)
			a.diffKey = ""
			a.loadMain()
		})
	})
}

// sortOrderMenu is lazygit's RefsHelper.CreateSortOrderMenu.
func (a *App) sortOrderMenu(options []string, prompt, current string, onSelected func(string)) {
	tr := a.repo.Common.Tr
	type opt struct {
		label, description string
		key                rune
	}
	all := map[string]opt{
		"recency":      {tr.SortByRecency, tr.SortBasedOnReflog, 'r'},
		"alphabetical": {tr.SortAlphabetical, "--sort=refname", 'a'},
		"date":         {tr.SortByDate, "--sort=-committerdate", 'd'},
	}
	var items []menuItem
	for _, order := range options {
		o := all[order]
		radio := "( ) "
		if order == current {
			radio = "(•) "
		}
		items = append(items, menuItem{label: radio + o.label, detail: o.description, key: string(o.key), chord: runeBind(o.key), run: func() { onSelected(order) }})
	}
	a.pushOverlay(&menuOverlay{title: tr.SortOrder, body: prompt, items: items})
}

// remoteBranchSortMenu is lazygit's RemoteBranchesController.createSortMenu.
func (a *App) remoteBranchSortMenu() {
	s := a.browse(panelBranches)
	if s == nil || s.ref != nil {
		return
	}
	git := &a.repo.Config.Git
	a.sortOrderMenu([]string{"alphabetical", "date"}, a.repo.Common.Tr.SortOrderPromptRemoteBranches, git.RemoteBranchSortOrder, func(order string) {
		if git.RemoteBranchSortOrder != order {
			git.RemoteBranchSortOrder = order
			a.cursor[panelBranches] = 0
			a.lists[panelBranches].ScrollTo(0, 0)
			a.reloadBrowsedRemote()
		}
	})
}

// localBranchSortMenu is lazygit's BranchesController.createSortMenu.
func (a *App) localBranchSortMenu() {
	git := &a.repo.Config.Git
	a.sortOrderMenu([]string{"recency", "alphabetical", "date"}, a.repo.Common.Tr.SortOrderPromptLocalBranches, git.LocalBranchSortOrder, func(order string) {
		if git.LocalBranchSortOrder != order {
			git.LocalBranchSortOrder = order
			a.cursor[panelBranches] = 0
			a.cancelRange(panelBranches)
			a.lists[panelBranches].ScrollTo(0, 0)
			a.Refresh()
		}
	})
}

package app

import "github.com/jesseduffield/lazygit/pkg/utils"

func (a *App) branchTabTitle() string {
	return []string{"Branches", "Remotes", "Tags"}[a.branchTab]
}

func (a *App) switchBranchTab(delta int) {
	if a.focus == panelFiles {
		a.staging = nil
		a.mainFocused = false
		a.tabCursor[0][a.fileTab] = a.cursor[panelFiles]
		a.fileTab = (a.fileTab + delta + 3) % 3
		a.cursor[panelFiles] = clamp(a.tabCursor[0][a.fileTab], 0, max(a.count(panelFiles)-1, 0))
		a.lists[panelFiles].ScrollIntoView(a.cursor[panelFiles])
		a.diffKey = ""
		a.loadMain()
		return
	}
	if a.focus != panelBranches {
		return
	}
	if a.refBrowse != nil {
		// A drill-down (commits of a ref, remote branches) is not a tab, so
		// lazygit's next and previous tab keys leave it for the window's first tab.
		a.refBrowse = nil
		a.branchTab = 0
	} else {
		a.tabCursor[1][a.branchTab] = a.cursor[panelBranches]
		a.branchTab = (a.branchTab + delta + 3) % 3
	}
	a.cursor[panelBranches] = clamp(a.tabCursor[1][a.branchTab], 0, max(a.count(panelBranches)-1, 0))
	a.lists[panelBranches].ScrollIntoView(a.cursor[panelBranches])
	a.diffKey = ""
	a.loadMain()
}

func (a *App) checkoutTag() {
	if a.snap == nil || a.count(panelBranches) == 0 {
		return
	}
	name := a.snap.Tags[a.cursor[panelBranches]].Name
	a.confirm("Checkout tag", "Checkout "+name+" in detached HEAD state?", false, func() {
		a.run("Checking out…", func() error { return a.repo.Checkout("refs/tags/" + name) }, nil)
	})
}

// pushTagFlow is lazygit's push tag: ask for a remote, then push the tag.
func (a *App) pushTagFlow() {
	if a.snap == nil || a.count(panelBranches) == 0 {
		return
	}
	name := a.snap.Tags[a.cursor[panelBranches]].Name
	title := utils.ResolvePlaceholderString(a.repo.Common.Tr.PushTagTitle, map[string]string{"tagName": name})
	a.promptSuggestions(title, "origin", "", "remotes", func(remote string) {
		a.run("Pushing…", func() error { return a.repo.PushTags(remote, []string{name}) }, a.done("Pushed "+name))
	})
}

func (a *App) fileTabTitle() string { return []string{"Files", "Worktrees", "Submodules"}[a.fileTab] }

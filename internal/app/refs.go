package app

func (a *App) branchTabTitle() string {
	return []string{"Branches", "Remotes", "Tags"}[a.branchTab]
}

// switchTab is lazygit's next / previous tab in the focused window.
func (a *App) switchTab(delta int) {
	if a.focus == panelCommits && !a.mainFocused && a.staging == nil {
		a.switchCommitTab(delta)
		return
	}
	a.switchBranchTab(delta)
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
	if a.browse(panelBranches) != nil {
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

func (a *App) fileTabTitle() string { return []string{"Files", "Worktrees", "Submodules"}[a.fileTab] }

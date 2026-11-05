package app

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
	if a.focus != panelBranches || a.refBrowse != nil {
		return
	}
	a.tabCursor[1][a.branchTab] = a.cursor[panelBranches]
	a.branchTab = (a.branchTab + delta + 3) % 3
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

func (a *App) fileTabTitle() string { return []string{"Files", "Worktrees", "Submodules"}[a.fileTab] }

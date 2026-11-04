package app

func (a *App) branchTabTitle() string {
	return []string{"Branches", "Remotes", "Tags", "Worktrees"}[a.branchTab]
}

func (a *App) switchBranchTab(delta int) {
	if a.focus == panelFiles {
		a.staging = nil
		a.mainFocused = false
		a.fileTab = (a.fileTab + delta + 2) % 2
		a.cursor[panelFiles] = 0
		a.lists[panelFiles].ScrollTo(0, 0)
		a.diffKey = ""
		a.loadMain()
		return
	}
	if a.focus != panelBranches {
		return
	}
	a.branchTab = (a.branchTab + delta + 4) % 4
	a.cursor[panelBranches] = 0
	a.lists[panelBranches].ScrollTo(0, 0)
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

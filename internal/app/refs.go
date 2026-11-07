package app

func (a *App) branchTabTitle() string {
	return []string{"Branches", "Remotes", "Tags"}[a.branchTab]
}

// switchTab is lazygit's next / previous tab in the focused window.
func (a *App) switchTab(delta int) {
	if a.focus == panelCommits && (a.mainFocused || a.staging != nil) {
		return
	}
	a.switchWindowTab(delta)
}

func (a *App) fileTabTitle() string { return []string{"Files", "Worktrees", "Submodules"}[a.fileTab] }

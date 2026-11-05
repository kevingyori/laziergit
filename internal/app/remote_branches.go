package app

import "github.com/jesseduffield/lazygit/pkg/utils"

// deleteBrowsedRemoteBranch is lazygit's d in remoteBranches
// (BranchesHelper.ConfirmDeleteRemote for one branch).
func (a *App) deleteBrowsedRemoteBranch() {
	s := a.refBrowse
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
	s, r := a.refBrowse, a.repo
	if s == nil || s.ref != nil {
		return
	}
	a.bg(func() {
		branches, err := r.RemoteBranches(s.remote)
		a.post(func() {
			if a.refBrowse != s || err != nil {
				return
			}
			s.branches = branches
			a.cursor[panelBranches] = clamp(a.cursor[panelBranches], 0, max(len(branches)-1, 0))
			a.diffKey = ""
			a.loadMain()
		})
	})
}

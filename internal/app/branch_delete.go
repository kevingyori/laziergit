package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"laziergit/internal/keymap"
	"strings"
)

func (a *App) deleteBranchFlow() {
	b := a.selectedBranch()
	if b == nil {
		return
	}
	tr := a.repo.Common.Tr
	title := utils.ResolvePlaceholderString(tr.DeleteBranchTitle, map[string]string{"selectedBranchName": b.Name})
	localDisabled, remoteDisabled, bothDisabled := "", "", ""
	if b.Head {
		localDisabled = tr.CantDeleteCheckOutBranch
		bothDisabled = localDisabled
	}
	if !b.IsTrackingRemote() || b.UpstreamGone {
		remoteDisabled = tr.UpstreamNotSetError
		if bothDisabled == "" {
			bothDisabled = remoteDisabled
		}
	}
	item := func(label string, key rune, disabled string, run func()) menuItem {
		return menuItem{label: label, key: string(key), chord: keymap.Binding{{Rune: key}}, disabled: disabled, run: run}
	}
	a.pushOverlay(&menuOverlay{title: title, items: []menuItem{
		item(tr.DeleteLocalBranch, 'c', localDisabled, func() { a.deleteLocalBranch(b) }),
		item(tr.DeleteRemoteBranch, 'r', remoteDisabled, func() { a.deleteRemoteBranch(b) }),
		item(tr.DeleteLocalAndRemoteBranch, 'b', bothDisabled, func() { a.deleteBothBranches(b) }),
	}})
}

func (a *App) withBranchMergeStatus(b *models.Branch, done func(bool)) {
	a.busy = a.repo.Common.Tr.DeletingStatus
	a.bg(func() {
		merged, err := a.repo.BranchMerged(b)
		a.post(func() {
			a.busy = ""
			if err != nil {
				a.showError(a.repo.Common.Tr.DeleteLocalBranch, err)
				return
			}
			done(merged)
		})
	})
}

func (a *App) deleteLocalBranch(b *models.Branch) {
	if a.deleteWorktreeBranchMenu(b, false) {
		return
	}
	a.withBranchMergeStatus(b, func(merged bool) {
		tr := a.repo.Common.Tr
		remove := func() { a.run(tr.DeletingStatus, func() error { return a.repo.DeleteBranch(b.Name, true) }, nil) }
		if merged {
			remove()
			return
		}
		msg := utils.ResolvePlaceholderString(tr.ForceDeleteBranchMessage, map[string]string{"selectedBranchName": b.Name})
		a.confirm(tr.ForceDeleteBranchTitle, msg, true, remove)
	})
}

func (a *App) deleteRemoteBranch(b *models.Branch) {
	tr := a.repo.Common.Tr
	title := utils.ResolvePlaceholderString(tr.DeleteBranchTitle, map[string]string{"selectedBranchName": b.UpstreamBranch})
	msg := utils.ResolvePlaceholderString(tr.DeleteRemoteBranchPrompt, map[string]string{"selectedBranchName": b.UpstreamBranch, "upstream": b.UpstreamRemote})
	a.confirm(title, msg, true, func() {
		a.run(tr.DeletingStatus, func() error { return a.repo.DeleteRemoteBranch(b.UpstreamRemote, b.UpstreamBranch) }, nil)
	})
}

func (a *App) deleteBothBranches(b *models.Branch) {
	if a.deleteWorktreeBranchMenu(b, true) {
		return
	}
	a.withBranchMergeStatus(b, func(merged bool) {
		tr := a.repo.Common.Tr
		msg := utils.ResolvePlaceholderString(tr.DeleteLocalAndRemoteBranchPrompt, map[string]string{"localBranchName": b.Name, "remoteBranchName": b.UpstreamBranch, "remoteName": b.UpstreamRemote})
		if !merged {
			msg += "\n\n" + utils.ResolvePlaceholderString(tr.ForceDeleteBranchMessage, map[string]string{"selectedBranchName": b.Name})
		}
		a.confirm(tr.DeleteLocalAndRemoteBranch, msg, true, func() {
			a.run(tr.DeletingStatus, func() error {
				if err := a.repo.DeleteRemoteBranch(b.UpstreamRemote, b.UpstreamBranch); err != nil {
					return err
				}
				return a.repo.DeleteBranch(b.Name, true)
			}, nil)
		})
	})
}

func (a *App) deleteWorktreeBranchMenu(b *models.Branch, both bool) bool {
	w, ok := git_commands.WorktreeForBranch(b, a.snap.Worktrees)
	if !ok || w.IsCurrent {
		return false
	}
	tr := a.repo.Common.Tr
	title := utils.ResolvePlaceholderString(tr.BranchCheckedOutByWorktree, map[string]string{"worktreeName": w.Name, "branchName": b.Name})
	removeLabel, detachLabel := tr.RemoveWorktreeAndDeleteBranch, tr.DetachWorktreeAndDeleteBranch
	if both {
		removeLabel, detachLabel = tr.RemoveWorktreeAndDeleteBothBranches, tr.DetachWorktreeAndDeleteBothBranches
	}
	action := func(detach bool) {
		a.withBranchMergeStatus(b, func(merged bool) {
			var run func(bool)
			run = func(force bool) {
				a.busy = tr.DeletingStatus
				a.bg(func() {
					err := a.repo.ReleaseWorktree(w, detach, force)
					if err == nil && both {
						err = a.repo.DeleteRemoteBranch(b.UpstreamRemote, b.UpstreamBranch)
					}
					if err == nil {
						err = a.repo.DeleteBranch(b.Name, true)
					}
					snap := a.repo.Load()
					a.post(func() {
						a.busy = ""
						a.apply(snap)
						if err == nil {
							return
						}
						if !detach && !force && (strings.Contains(err.Error(), "--force") || strings.Contains(err.Error(), "working trees containing submodules")) {
							msg := utils.ResolvePlaceholderString(tr.ForceRemoveWorktreePrompt, map[string]string{"worktreeName": w.Name})
							a.confirm(tr.RemoveWorktreeTitle, msg, true, func() { run(true) })
							return
						}
						a.showError(tr.DeletingStatus, err)
					})
				})
			}

			if merged {
				run(false)
				return
			}
			msg := utils.ResolvePlaceholderString(tr.ForceDeleteBranchMessage, map[string]string{"selectedBranchName": b.Name})
			a.confirm(tr.ForceDeleteBranchTitle, msg, true, func() { run(false) })
		})
	}
	a.pushOverlay(&menuOverlay{title: title, items: []menuItem{
		{label: removeLabel, key: "r", chord: keymap.Binding{{Rune: 'r'}}, run: func() { action(false) }},
		{label: detachLabel, key: "d", chord: keymap.Binding{{Rune: 'd'}}, run: func() { action(true) }},
	}})
	return true
}

package app

import (
	"laziergit/internal/keymap"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"
)

// deleteBranchFlow is lazygit's BranchesController.delete.
func (a *App) deleteBranchFlow() {
	if a.selectedBranch() == nil {
		return
	}
	start, end := a.selectionRange(panelBranches)
	branches := a.snap.Branches[start : end+1]
	tr := a.repo.Common.Tr
	many := len(branches) > 1
	checkedOut := lo.SomeBy(branches, func(b *models.Branch) bool { return b.Head })
	hasUpstream := lo.EveryBy(branches, func(b *models.Branch) bool { return b.IsTrackingRemote() && !b.UpstreamGone })
	localDisabled, remoteDisabled, bothDisabled := "", "", ""
	noUpstream := lo.Ternary(many, tr.UpstreamsNotSetError, tr.UpstreamNotSetError)
	if checkedOut {
		localDisabled, bothDisabled = tr.CantDeleteCheckOutBranch, tr.CantDeleteCheckOutBranch
	}
	if !hasUpstream {
		remoteDisabled = noUpstream
		if bothDisabled == "" {
			bothDisabled = noUpstream
		}
	}
	title := tr.DeleteBranchesTitle
	if !many {
		title = utils.ResolvePlaceholderString(tr.DeleteBranchTitle, map[string]string{"selectedBranchName": branches[0].Name})
	}
	item := func(label string, key rune, disabled string, run func()) menuItem {
		return menuItem{label: label, key: string(key), chord: keymap.Binding{{Rune: key}}, disabled: disabled, run: run}
	}
	a.pushOverlay(&menuOverlay{title: title, items: []menuItem{
		item(lo.Ternary(many, tr.DeleteLocalBranches, tr.DeleteLocalBranch), 'c', localDisabled, func() { a.deleteLocalBranches(branches) }),
		item(lo.Ternary(many, tr.DeleteRemoteBranches, tr.DeleteRemoteBranch), 'r', remoteDisabled, func() { a.deleteRemoteBranches(branches) }),
		item(lo.Ternary(many, tr.DeleteLocalAndRemoteBranches, tr.DeleteLocalAndRemoteBranch), 'b', bothDisabled, func() { a.deleteBothBranches(branches) }),
		{label: tr.Cancel, run: func() {}},
	}})
}

func (a *App) withBranchMergeStatus(b *models.Branch, done func(bool)) {
	a.withBranchesMerged([]*models.Branch{b}, done)
}

// withBranchesMerged is lazygit's allBranchesMerged, off the main thread.
func (a *App) withBranchesMerged(branches []*models.Branch, done func(bool)) {
	a.busy = a.repo.Common.Tr.DeletingStatus
	a.bg(func() {
		merged, err := true, error(nil)
		for _, b := range branches {
			var m bool
			if m, err = a.repo.BranchMerged(b); err != nil || !m {
				merged = false
				break
			}
		}
		a.post(func() {
			a.busy = ""
			if err != nil {
				a.errorPopup(err.Error())
				return
			}
			done(merged)
		})
	})
}

func (a *App) someCheckedOutByOtherWorktree(branches []*models.Branch) bool {
	return len(branches) > 1 && lo.SomeBy(branches, func(b *models.Branch) bool {
		return git_commands.CheckedOutByOtherWorktree(b, a.snap.Worktrees)
	})
}

func (a *App) afterBranchDelete() {
	a.collapseRangeToTop(panelBranches)
}

// deleteLocalBranches is lazygit's BranchesHelper.ConfirmLocalDelete.
func (a *App) deleteLocalBranches(branches []*models.Branch) {
	tr := a.repo.Common.Tr
	if a.someCheckedOutByOtherWorktree(branches) {
		a.errorPopup(tr.SomeBranchesCheckedOutByWorktreeError)
		return
	}
	if len(branches) == 1 && a.deleteWorktreeBranchMenu(branches[0], false) {
		return
	}
	names := lo.Map(branches, func(b *models.Branch, _ int) string { return b.Name })
	a.withBranchesMerged(branches, func(merged bool) {
		remove := func() {
			a.run(tr.DeletingStatus, func() error { return a.repo.DeleteBranches(names) }, a.afterBranchDelete)
		}
		if merged {
			remove()
			return
		}
		a.confirm(tr.ForceDeleteBranchTitle, a.forceDeleteMessage(branches), true, remove)
	})
}

func (a *App) forceDeleteMessage(branches []*models.Branch) string {
	tr := a.repo.Common.Tr
	if len(branches) == 1 {
		return utils.ResolvePlaceholderString(tr.ForceDeleteBranchMessage, map[string]string{"selectedBranchName": branches[0].Name})
	}
	return tr.ForceDeleteBranchesMessage
}

// deleteRemoteBranches is lazygit's BranchesHelper.ConfirmDeleteRemote for
// the upstreams of local branches.
func (a *App) deleteRemoteBranches(branches []*models.Branch) {
	tr := a.repo.Common.Tr
	title, msg := tr.DeleteBranchesTitle, tr.DeleteRemoteBranchesPrompt
	if len(branches) == 1 {
		b := branches[0]
		title = utils.ResolvePlaceholderString(tr.DeleteBranchTitle, map[string]string{"selectedBranchName": b.UpstreamBranch})
		msg = utils.ResolvePlaceholderString(tr.DeleteRemoteBranchPrompt, map[string]string{"selectedBranchName": b.UpstreamBranch, "upstream": b.UpstreamRemote})
	}
	a.confirm(title, msg, true, func() {
		a.run(tr.DeletingStatus, func() error { return a.deleteUpstreams(branches) }, nil)
	})
}

func (a *App) deleteUpstreams(branches []*models.Branch) error {
	byRemote := lo.GroupBy(branches, func(b *models.Branch) string { return b.UpstreamRemote })
	for _, remote := range lo.Uniq(lo.Map(branches, func(b *models.Branch, _ int) string { return b.UpstreamRemote })) {
		names := lo.Map(byRemote[remote], func(b *models.Branch, _ int) string { return b.UpstreamBranch })
		if err := a.repo.DeleteRemoteBranches(remote, names); err != nil {
			return err
		}
	}
	return nil
}

// deleteBothBranches is lazygit's BranchesHelper.ConfirmLocalAndRemoteDelete.
func (a *App) deleteBothBranches(branches []*models.Branch) {
	tr := a.repo.Common.Tr
	if a.someCheckedOutByOtherWorktree(branches) {
		a.errorPopup(tr.SomeBranchesCheckedOutByWorktreeError)
		return
	}
	if len(branches) == 1 && a.deleteWorktreeBranchMenu(branches[0], true) {
		return
	}
	names := lo.Map(branches, func(b *models.Branch, _ int) string { return b.Name })
	a.withBranchesMerged(branches, func(merged bool) {
		msg := tr.DeleteLocalAndRemoteBranchesPrompt
		if len(branches) == 1 {
			b := branches[0]
			msg = utils.ResolvePlaceholderString(tr.DeleteLocalAndRemoteBranchPrompt, map[string]string{"localBranchName": b.Name, "remoteBranchName": b.UpstreamBranch, "remoteName": b.UpstreamRemote})
		}
		if !merged {
			msg += "\n\n" + a.forceDeleteMessage(branches)
		}
		a.confirm(tr.DeleteLocalAndRemoteBranch, msg, true, func() {
			a.run(tr.DeletingStatus, func() error {
				if err := a.deleteUpstreams(branches); err != nil {
					return err
				}
				return a.repo.DeleteBranches(names)
			}, a.afterBranchDelete)
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
		a.confirmForceIfUnmerged(b, func() {
			a.releaseWorktree(w, detach, false, func() error {
				if both {
					if err := a.repo.DeleteRemoteBranch(b.UpstreamRemote, b.UpstreamBranch); err != nil {
						return err
					}
				}
				return a.repo.DeleteBranch(b.Name, true)
			})
		})
	}
	a.pushOverlay(&menuOverlay{title: title, items: []menuItem{
		{label: removeLabel, key: "r", chord: keymap.Binding{{Rune: 'r'}}, run: func() { action(false) }},
		{label: detachLabel, key: "d", chord: keymap.Binding{{Rune: 'd'}}, run: func() { action(true) }},
	}})
	return true
}

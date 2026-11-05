package app

import (
	"path/filepath"
	"strings"

	"laziergit/internal/keymap"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// Ported from lazygit's WorktreeHelper and BranchesHelper.RemoveWorktreeMenu.

func menuKeyItem(label string, key rune, disabled string, run func()) menuItem {
	return menuItem{label: label, key: string(key), chord: keymap.Binding{{Rune: key}}, disabled: disabled, run: run}
}

func sanitizedBranchName(s string) string { return strings.ReplaceAll(s, " ", "-") }

// switchToWorktree is lazygit's WorktreeHelper.Switch.
func (a *App) switchToWorktree(w *models.Worktree, focus string) {
	if w.IsCurrent {
		a.errorPopup(a.repo.Common.Tr.AlreadyInWorktree)
		return
	}
	a.switchFocus = focus
	a.openRepository(w.Path, false)
}

func (a *App) openWorktree() {
	if w := a.selectedWorktree(); w != nil {
		a.switchToWorktree(w, "worktrees")
	}
}

// promptToCheckoutWorktree is the branches controller's "Switch to worktree".
func (a *App) promptToCheckoutWorktree(w *models.Worktree) {
	tr := a.repo.Common.Tr
	sw := func() { a.switchToWorktree(w, "localBranches") }
	if a.repo.Config.Gui.SkipSwitchWorktreeOnCheckoutWarning {
		sw()
		return
	}
	a.confirm(tr.SwitchToWorktree, utils.ResolvePlaceholderString(tr.AlreadyCheckedOutByWorktree, map[string]string{"worktreeName": w.Name}), false, sw)
}

func (a *App) newWorktreeFlow() {
	tr := a.repo.Common.Tr
	a.promptSuggestions(tr.NewWorktreeForBranchTitle, "", "", "worktree", func(value string) {
		for _, b := range a.raw.Branches {
			if b.Name == value {
				if w, ok := git_commands.WorktreeForBranch(b, a.raw.Worktrees); ok {
					a.errorPopup(utils.ResolvePlaceholderString(tr.BranchCheckedOutByWorktree, map[string]string{"branchName": b.Name, "worktreeName": w.Name}))
					return
				}
				prompt := utils.ResolvePlaceholderString(tr.WorktreeLocationPromptCheckout, map[string]string{"branchName": b.Name})
				a.worktreeLocation(b.Name, prompt, func(path string) { a.createWorktree(path, b.RefName(), "", false, "worktrees") })
				return
			}
		}
		for _, r := range a.raw.Remotes {
			for _, b := range r.Branches {
				if r.Name+"/"+b.Name == value {
					prompt := utils.ResolvePlaceholderString(tr.WorktreeLocationPromptTrackingBranch, map[string]string{"name": b.Name, "ref": value})
					a.worktreeLocation(b.Name, prompt, func(path string) { a.createWorktree(path, value, b.Name, false, "worktrees") })
					return
				}
			}
		}
		name := sanitizedBranchName(value)
		base := "HEAD"
		if len(a.raw.Branches) > 0 && a.raw.Branches[0].Head {
			base = a.raw.Branches[0].RefName()
		}
		prompt := utils.ResolvePlaceholderString(tr.WorktreeLocationPromptNewBranch, map[string]string{"name": name, "base": base})
		a.worktreeLocation(name, prompt, func(path string) { a.createWorktree(path, base, name, false, "worktrees") })
	})
}

func (a *App) createWorktree(path, base, branch string, detach bool, focus string) {
	a.run(a.repo.Common.Tr.AddingWorktree, func() error { return a.repo.NewWorktree(path, base, branch, detach) }, func() {
		a.switchFocus = focus
		a.openRepository(path, false)
	})
}

// worktreeLocation is lazygit's promptForWorktreeLocation.
func (a *App) worktreeLocation(name, prompt string, create func(string)) {
	tr := a.repo.Common.Tr
	root := a.repo.Git.RepoPaths.RepoPath()
	var parents []string
	add := func(path string) {
		for _, p := range parents {
			if p == path {
				return
			}
		}
		parents = append(parents, path)
	}
	for _, w := range a.raw.Worktrees {
		if !w.IsMain {
			add(filepath.Dir(w.Path))
		}
	}
	if p := utils.ExpandTilde(a.repo.Config.Worktree.DefaultPath); p != "" {
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		add(filepath.Clean(p))
	}
	if len(parents) == 0 {
		add(filepath.Dir(root))
	}
	var items []menuItem
	for _, parent := range parents {
		path := filepath.Join(parent, name)
		items = append(items, menuItem{label: path, run: func() { create(path) }})
	}
	initial := filepath.Join(parents[0], name)
	items = append(items, menuItem{label: tr.WorktreeLocationOther, run: func() {
		a.prompt(tr.NewWorktreePath, initial, "", func(path string) { create(utils.ExpandTilde(path)) })
	}})
	a.pushOverlay(&menuOverlay{title: tr.WorktreeLocationTitle, body: prompt, items: items})
}

func (a *App) promptForWorktreeName(title, initial string, next func(string)) {
	a.prompt(title, initial, "", func(s string) { next(sanitizedBranchName(s)) })
}

func (a *App) newBranchAndWorktreeItem(ref, base string) menuItem {
	tr := a.repo.Common.Tr
	return menuKeyItem(utils.ResolvePlaceholderString(tr.NewBranchAndWorktreeFromRef, map[string]string{"ref": ref}), 'b', "", func() {
		a.promptForWorktreeName(tr.NewBranchAndWorktreeName, "", func(name string) {
			prompt := utils.ResolvePlaceholderString(tr.WorktreeLocationPromptNewBranch, map[string]string{"name": name, "base": ref})
			a.worktreeLocation(name, prompt, func(path string) { a.createWorktree(path, base, name, false, "localBranches") })
		})
	})
}

func (a *App) detachedWorktreeItem(ref, base, dirName string) menuItem {
	tr := a.repo.Common.Tr
	prompt := utils.ResolvePlaceholderString(tr.WorktreeLocationPromptDetached, map[string]string{"ref": ref})
	create := func(path string) { a.createWorktree(path, base, "", true, "localBranches") }
	return menuKeyItem(utils.ResolvePlaceholderString(tr.DetachedWorktreeAtRef, map[string]string{"ref": ref}), 'd', "", func() {
		if dirName != "" {
			a.worktreeLocation(dirName, prompt, create)
			return
		}
		a.promptForWorktreeName(tr.NewWorktreeName, "", func(name string) { a.worktreeLocation(name, prompt, create) })
	})
}

func (a *App) worktreeMenu(items ...menuItem) {
	a.pushOverlay(&menuOverlay{title: a.repo.Common.Tr.NewWorktree, items: items})
}

func (a *App) newWorktreeMenuForBranch() {
	b := a.selectedBranch()
	if b == nil {
		return
	}
	tr := a.repo.Common.Tr
	disabled := ""
	if w, ok := git_commands.WorktreeForBranch(b, a.snap.Worktrees); ok {
		disabled = utils.ResolvePlaceholderString(tr.BranchCheckedOutByWorktree, map[string]string{"branchName": b.Name, "worktreeName": w.Name})
	}
	forBranch := menuKeyItem(utils.ResolvePlaceholderString(tr.WorktreeForRef, map[string]string{"ref": b.Name}), 'w', disabled, func() {
		prompt := utils.ResolvePlaceholderString(tr.WorktreeLocationPromptCheckout, map[string]string{"branchName": b.Name})
		a.worktreeLocation(b.Name, prompt, func(path string) { a.createWorktree(path, b.RefName(), "", false, "localBranches") })
	})
	a.worktreeMenu(a.newBranchAndWorktreeItem(b.Name, b.RefName()), forBranch, a.detachedWorktreeItem(b.Name, b.RefName(), b.Name))
}

func (a *App) newWorktreeMenuForCommit() {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	a.worktreeMenu(a.newBranchAndWorktreeItem(c.ShortHash(), c.RefName()), a.detachedWorktreeItem(c.ShortHash(), c.RefName(), ""))
}

func (a *App) newWorktreeMenuForTag() {
	if a.snap == nil || a.count(panelBranches) == 0 {
		return
	}
	t := a.snap.Tags[a.cursor[panelBranches]]
	a.worktreeMenu(a.newBranchAndWorktreeItem(t.Name, t.RefName()), a.detachedWorktreeItem(t.Name, t.RefName(), ""))
}

func (a *App) newWorktreeMenuForStash() {
	s := a.selectedStash()
	if s == nil {
		return
	}
	a.worktreeMenu(a.newBranchAndWorktreeItem(s.RefName(), s.FullRefName()), a.detachedWorktreeItem(s.RefName(), s.FullRefName(), ""))
}

func (a *App) newWorktreeMenuForRemoteBranch() {
	s := a.browse(panelBranches)
	if s == nil || s.ref != nil || len(s.branches) == 0 {
		return
	}
	rb := s.branches[a.cursor[panelBranches]]
	tr := a.repo.Common.Tr
	stripped := strings.SplitAfterN(rb.RefName(), "/", 2)[1]
	full := rb.FullName()
	local := menuKeyItem(utils.ResolvePlaceholderString(tr.NewLocalBranchAndWorktreeFromRef, map[string]string{"ref": full}), 'b', "", func() {
		a.promptForWorktreeName(tr.NewBranchAndWorktreeName, stripped, func(name string) {
			prompt := utils.ResolvePlaceholderString(tr.WorktreeLocationPromptTrackingBranch, map[string]string{"name": name, "ref": full})
			a.worktreeLocation(name, prompt, func(path string) { a.createWorktree(path, full, name, false, "localBranches") })
		})
	})
	a.worktreeMenu(local, a.detachedWorktreeItem(full, full, stripped))
}

// releaseWorktree is lazygit's WorktreeHelper.Remove/Detach: removal that git
// refuses for a dirty worktree (or one with submodules) asks to force.
func (a *App) releaseWorktree(w *models.Worktree, detach, force bool, then func() error) {
	tr := a.repo.Common.Tr
	label := tr.RemovingWorktree
	if detach {
		label = tr.DetachingWorktree
	}
	a.busy = label
	a.bg(func() {
		err := a.repo.ReleaseWorktree(w, detach, force)
		if err == nil && then != nil {
			err = then()
		}
		snap := a.repo.Load()
		a.post(func() {
			a.busy = ""
			a.apply(snap)
			if err == nil {
				return
			}
			msg := err.Error()
			if !detach && !force && (strings.Contains(msg, "--force") || strings.Contains(msg, "fatal: working trees containing submodules cannot be moved or removed")) {
				prompt := utils.ResolvePlaceholderString(tr.ForceRemoveWorktreePrompt, map[string]string{"worktreeName": w.Name})
				a.confirm(tr.RemoveWorktreeTitle, prompt, true, func() { a.releaseWorktree(w, false, true, then) })
				return
			}
			a.showError(label, err)
		})
	})
}

// confirmForceIfUnmerged runs next directly when b is merged, else after the
// force-delete warning.
func (a *App) confirmForceIfUnmerged(b *models.Branch, next func()) {
	tr := a.repo.Common.Tr
	a.withBranchMergeStatus(b, func(merged bool) {
		if merged {
			next()
			return
		}
		msg := utils.ResolvePlaceholderString(tr.ForceDeleteBranchMessage, map[string]string{"selectedBranchName": b.Name})
		a.confirm(tr.ForceDeleteBranchTitle, msg, true, next)
	})
}

func (a *App) deleteWorktreeFlow() {
	if w := a.selectedWorktree(); w != nil {
		a.removeWorktreeMenu(w)
	}
}

// removeWorktreeMenu is lazygit's BranchesHelper.RemoveWorktreeMenu.
func (a *App) removeWorktreeMenu(w *models.Worktree) {
	tr := a.repo.Common.Tr
	if w.IsMain {
		a.errorPopup(tr.CantDeleteMainWorktree)
		return
	}
	if w.IsCurrent {
		a.errorPopup(tr.CantDeleteCurrentWorktree)
		return
	}
	var branch *models.Branch
	for _, b := range a.raw.Branches {
		if b.Name == w.Branch {
			branch = b
			break
		}
	}
	andBranch, both := "", ""
	if branch == nil {
		andBranch, both = tr.WorktreeNotCheckedOutOnBranch, tr.WorktreeNotCheckedOutOnBranch
	} else if !branch.IsTrackingRemote() || branch.UpstreamGone {
		both = tr.UpstreamNotSetError
	}
	deleteLocal := func() error { return a.repo.DeleteBranch(branch.Name, true) }
	title := utils.ResolvePlaceholderString(tr.RemoveWorktreeMenuTitle, map[string]string{"worktreeName": w.Name})
	a.pushOverlay(&menuOverlay{title: title, items: []menuItem{
		menuKeyItem(tr.RemoveWorktree, 'w', "", func() { a.releaseWorktree(w, false, false, nil) }),
		menuKeyItem(tr.RemoveWorktreeAndDeleteBranch, 'b', andBranch, func() {
			a.confirmForceIfUnmerged(branch, func() { a.releaseWorktree(w, false, false, deleteLocal) })
		}),
		menuKeyItem(tr.RemoveWorktreeAndDeleteBothBranches, 'r', both, func() {
			a.confirmForceIfUnmerged(branch, func() {
				a.releaseWorktree(w, false, false, func() error {
					if err := a.repo.DeleteRemoteBranch(branch.UpstreamRemote, branch.UpstreamBranch); err != nil {
						return err
					}
					return deleteLocal()
				})
			})
		}),
	}})
}

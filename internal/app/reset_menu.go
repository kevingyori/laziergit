package app

import (
	"fmt"

	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/keymap"
)

// resetMenu is lazygit's RefsHelper.CreateGitResetMenu: mixed, soft and hard
// reset of the current branch onto ref (named name in the menu).
func (a *App) resetMenu(name, ref string) {
	tr := a.repo.Common.Tr
	type strength struct {
		kind, label string
		key         rune
		tooltip     string
	}
	rows := []strength{
		{"mixed", "Mixed reset", 'm', tr.ResetMixedTooltip},
		{"soft", "Soft reset", 's', tr.ResetSoftTooltip},
		{"hard", "Hard reset", 'h', tr.ResetHardTooltip},
	}
	var items []menuItem
	for _, row := range rows {
		row := row
		items = append(items, menuItem{
			label: row.label, detail: fmt.Sprintf("reset --%s %s", row.kind, name),
			key: string(row.key), chord: keymap.Binding{{Rune: row.key}},
			run: func() {
				reset := func() {
					a.run(tr.ResettingStatus+"…", func() error {
						return a.repo.Locked(func() error { return a.repo.Git.Commit.ResetToCommit(ref, row.kind, nil) })
					}, func() { a.cursor[panelCommits] = 0 })
				}
				if row.kind == "hard" && a.workingTreeDirty() {
					a.confirm(tr.Actions.HardReset, tr.ResetHardConfirmation, false, reset)
					return
				}
				reset()
			},
		})
	}
	a.pushOverlay(&menuOverlay{title: fmt.Sprintf("%s %s", tr.ResetTo, name), items: items})
}

// workingTreeDirty is lazygit's IsWorkingTreeDirtyExceptSubmodules.
func (a *App) workingTreeDirty() bool {
	return a.anyFile(func(f *models.File) bool { return f.HasStagedChanges || f.Tracked })
}

// resetSelected is `g` in the branches, tags, remote branches and commits views.
func (a *App) resetSelected() {
	if a.snap == nil {
		return
	}
	switch a.scope() {
	case scopeBranches:
		if b := a.selectedBranch(); b != nil {
			a.resetMenu(b.Name, b.FullRefName())
		}
	case scopeTags:
		if a.count(panelBranches) > 0 {
			t := a.snap.Tags[a.cursor[panelBranches]]
			a.resetMenu(t.Name, t.FullRefName())
		}
	case scopeRemoteBranches:
		if s := a.refBrowse; s != nil && len(s.branches) > 0 {
			b := s.branches[a.cursor[panelBranches]]
			a.resetMenu(b.FullName(), b.FullRefName())
		}
	case scopeCommits:
		if c := a.selectedCommit(); c != nil {
			a.resetMenu(c.Hash(), c.Hash())
		}
	}
}

// resetToUpstreamMenu is `g` in files.
func (a *App) resetToUpstreamMenu() { a.resetMenu("@{upstream}", "@{upstream}") }

// workspaceResetMenu is lazygit's createResetMenu in the files view (`D`).
func (a *App) workspaceResetMenu() {
	tr := a.repo.Common.Tr
	wt := a.repo.Git.WorkingTree
	nuke := "git reset --hard HEAD && git clean -fd"
	if len(a.snap.Submodules) > 0 {
		nuke = fmt.Sprintf("%s (%s)", nuke, tr.AndResetSubmodules)
	}
	do := func(op func() error) func() {
		return func() { a.run("", func() error { return a.repo.Locked(op) }, nil) }
	}
	item := func(label, cmd string, r rune, run func()) menuItem {
		return menuItem{label: label, detail: cmd, key: string(r), chord: keymap.Binding{{Rune: r}}, run: run}
	}
	a.pushOverlay(&menuOverlay{items: []menuItem{
		item(tr.DiscardAllChangesToAllFiles, nuke, 'x', func() {
			a.confirm(tr.Actions.NukeWorkingTree, tr.NukeTreeConfirmation, false, do(wt.ResetAndClean))
		}),
		item(tr.DiscardAnyUnstagedChanges, "git checkout -- .", 'u', do(wt.DiscardAnyUnstagedFileChanges)),
		item(tr.DiscardUntrackedFiles, "git clean -fd", 'c', do(wt.RemoveUntrackedFiles)),
		item(tr.DiscardStagedChanges, "stash staged and drop stash", 'S', func() {
			if !a.workingTreeDirty() {
				a.errorPopup(tr.NoTrackedStagedFilesStash)
				return
			}
			do(func() error {
				if err := a.repo.Git.Stash.SaveStagedChanges("[lazygit] tmp stash"); err != nil {
					return err
				}
				return a.repo.Git.Stash.DropNewest()
			})()
		}),
		item(tr.SoftReset, "git reset --soft HEAD", 's', do(func() error { return wt.ResetSoft("HEAD") })),
		item("mixed reset", "git reset --mixed HEAD", 'm', do(func() error { return wt.ResetMixed("HEAD") })),
		item(tr.HardReset, "git reset --hard HEAD", 'h', func() {
			hard := do(func() error { return wt.ResetHard("HEAD") })
			if a.workingTreeDirty() {
				a.confirm(tr.Actions.HardReset, tr.ResetHardConfirmation, false, hard)
				return
			}
			hard()
		}),
	}})
}

package app

import (
	"fmt"
	"laziergit/internal/repo"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"

	"github.com/jesseduffield/lazygit/pkg/utils"
)

// The stash panel's actions: lazygit's StashController.

func (a *App) stashApplyFlow() {
	s := a.selectedStash()
	if s == nil {
		return
	}
	tr, gui := a.repo.Common.Tr, a.repo.Config.Gui
	apply := func() {
		a.run(tr.ApplyingStashStatus+"…", func() error { return a.repo.StashApply(s.Index) }, func() {
			if gui.SwitchToFilesAfterStashApply {
				a.focusPanel(panelFiles)
			}
		})
	}
	if gui.SkipStashWarning {
		apply()
		return
	}
	a.confirm(tr.StashApply, tr.SureApplyStashEntry, false, apply)
}

func (a *App) stashPopFlow() {
	s := a.selectedStash()
	if s == nil {
		return
	}
	tr, gui := a.repo.Common.Tr, a.repo.Config.Gui
	pop := func() {
		a.run(tr.PoppingStashStatus+"…", func() error { return a.repo.StashPop(s.Index) }, func() {
			if gui.SwitchToFilesAfterStashPop {
				a.focusPanel(panelFiles)
			}
		})
	}
	if gui.SkipStashWarning {
		pop()
		return
	}
	a.confirm(tr.StashPop, tr.SurePopStashEntry, false, pop)
}

// stashDropFlow drops every selected entry, highest index first so the
// lower indices stay valid, then collapses the range to its top.
func (a *App) stashDropFlow() {
	if a.selectedStash() == nil {
		return
	}
	start, end := a.selectionRange(panelStash)
	entries := a.snap.Stash[start : end+1]
	tr := a.repo.Common.Tr
	a.confirm(tr.StashDrop, tr.SureDropStashEntry, true, func() {
		a.run("Dropping stash…", func() error {
			for i := len(entries) - 1; i >= 0; i-- {
				if err := a.repo.StashDrop(entries[i].Index); err != nil {
					return err
				}
			}
			return nil
		}, func() { a.collapseRangeToTop(panelStash) })
	})
}

// stashNewBranchFlow is lazygit's Refs.NewBranch off the stash entry.
func (a *App) stashNewBranchFlow() {
	s := a.selectedStash()
	if s == nil {
		return
	}
	tr := a.repo.Common.Tr
	title := utils.ResolvePlaceholderString(tr.NewBranchNameBranchOff, map[string]string{"branchName": s.Description()})
	a.prompt(title, "", "", func(name string) {
		name = strings.ReplaceAll(strings.TrimSpace(name), " ", "-")
		if name == "" {
			return
		}
		a.autostashing("Creating branch…", fmt.Sprintf(tr.AutoStashForNewBranch, name), func() error { return a.repo.NewBranch(name, s.FullRefName()) }, func() {
			a.branchTab = 0
			a.cursor[panelBranches] = 0
			a.focusPanel(panelBranches)
		})
	})
}

// stashRenameFlow is lazygit's handleRenameStashEntry.
func (a *App) stashRenameFlow() {
	s := a.selectedStash()
	if s == nil {
		return
	}
	tr := a.repo.Common.Tr
	title := utils.ResolvePlaceholderString(tr.RenameStashPrompt, map[string]string{"stashName": s.RefName()})
	a.pushOverlay(&promptOverlay{title: title, value: s.Name, onSubmit: func(msg string) {
		a.run("Renaming stash…", func() error {
			return a.repo.Locked(func() error { return a.repo.Git.Stash.Rename(s.Index, msg) })
		}, func() {
			a.cursor[panelStash] = 0
			a.diffKey = ""
			a.loadMain()
		})
	}})
}

// enterStashFiles is lazygit's SwitchToDiffFilesController on the stash: the
// entry's files (stash^..stash) in the commit-files view, which can't rebase.
// Like files opened from sub-commits, the view lives in the commits window.
func (a *App) enterStashFiles() {
	s := a.selectedStash()
	if s == nil {
		return
	}
	r := a.repo
	c := r.StashCommit(s)
	idx := a.cursor[panelStash]
	from, reverse := a.diffingFrom(repo.CommitFrom(c))
	a.bg(func() {
		files, err := r.DiffFiles(from, c.Hash(), reverse)
		a.post(func() {
			if a.repo != r {
				return
			}
			if err != nil {
				a.showError("Load commit files", err)
				return
			}
			a.commitFiles = &commitFilesState{commit: c, commitIdx: a.cursor[panelCommits], fromStash: true, stashIdx: idx, tab: a.commitTab, files: files, tree: newCommitFileTree(a, c, files)}
			a.updateTreeRows()
			a.cursor[panelCommits] = 0
			a.lists[panelCommits].ScrollTo(0, 0)
			a.focusPanel(panelCommits)
		})
	})
}

// discardCommitFiles is lazygit's CommitFilesController.discard: remove the
// selected files' changes from the commit, only for a local commit.
func (a *App) discardCommitFiles() {
	cf := a.commitFiles
	tr := a.repo.Common.Tr
	if cf == nil || a.selectedCommitFileNode() == nil {
		return
	}
	if cf.fromRef || cf.fromSub || cf.fromStash || cf.tab != 0 {
		a.errorPopup(tr.CanOnlyDiscardFromLocalCommits)
		return
	}
	if a.snap.Operation != "" {
		a.errorPopup(tr.CantPatchWhileRebasingError)
		return
	}
	start, end := a.selectionRange(panelCommits)
	var nodes []*filetree.CommitFileNode
	for i := start; i <= end && i < cf.tree.Len(); i++ {
		nodes = append(nodes, cf.tree.Get(i))
	}
	var paths []string
	for _, n := range nodes {
		if descendsFromSelected(n, nodes) {
			continue
		}
		_ = n.ForEachFile(func(f *models.CommitFile) error {
			paths = append(paths, f.Names()...)
			return nil
		})
	}
	prompt := tr.DiscardFileChangesPrompt
	if a.customPatch != nil {
		prompt = tr.DiscardFileChangesPromptResetPatch
	}
	commits, idx := a.snap.Commits, cf.commitIdx
	r := a.repo
	a.confirm(tr.DiscardFileChangesTitle, prompt, true, func() {
		var commit *models.Commit
		var files []*models.CommitFile
		a.run(tr.RebasingStatus+"…", func() error {
			err := r.EditPatch(func(p *git_commands.PatchCommands) error {
				if p.PatchBuilder.Active() {
					p.PatchBuilder.Reset()
				}
				return r.Git.Rebase.DiscardOldFileChanges(commits, idx, paths)
			})
			if err != nil {
				return err
			}
			// The rewritten commit's files, for the view that stays open.
			if snap := r.Load(); idx < len(snap.Commits) {
				commit = snap.Commits[idx]
				files, err = r.CommitFiles(commit)
			}
			return err
		}, func() {
			a.customPatch = nil
			a.cancelRange(panelCommits)
			if commit != nil && a.commitFiles == cf {
				cf.commit, cf.files = commit, files
				cf.tree = newCommitFileTree(a, commit, files)
				a.updateTreeRows()
				a.cursor[panelCommits] = clamp(a.cursor[panelCommits], 0, max(cf.tree.Len()-1, 0))
				a.diffKey = ""
				a.loadMain()
			}
		})
	})
}

func descendsFromSelected(n *filetree.CommitFileNode, selected []*filetree.CommitFileNode) bool {
	for _, s := range selected {
		if !s.IsFile() && strings.HasPrefix(n.GetInternalPath(), s.GetInternalPath()+"/") {
			return true
		}
	}
	return false
}

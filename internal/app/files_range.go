package app

import (
	"errors"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"
)

// selectedFileNodes are the Files tab's selected rows, the whole range when
// one is selected (lazygit's FilesController getSelectedItems).
func (a *App) selectedFileNodes() []*filetree.FileNode {
	if a.fileTab != 0 || a.files.Len() == 0 {
		return nil
	}
	start, end := a.selectionRange(panelFiles)
	nodes := make([]*filetree.FileNode, 0, end-start+1)
	for i := start; i <= end; i++ {
		nodes = append(nodes, a.files.Get(i))
	}
	return nodes
}

// normalisedSelectedNodes drops nodes already covered by a selected
// directory, as lazygit's function of the same name.
func normalisedSelectedNodes(nodes []*filetree.FileNode) []*filetree.FileNode {
	return lo.Filter(nodes, func(n *filetree.FileNode, _ int) bool {
		path := n.GetInternalPath()
		for _, d := range nodes {
			if !d.IsFile() && strings.HasPrefix(path, d.GetInternalPath()+"/") {
				return false
			}
		}
		return true
	})
}

// toggleStagedRange is lazygit's press on several rows: stage everything in
// them if anything has unstaged changes, else unstage everything.
func (a *App) toggleStagedRange(nodes []*filetree.FileNode) {
	submodules := a.snap.Submodules
	tr := a.repo.Common.Tr
	for _, n := range nodes { // lazygit's canStageSelection
		if n.SomeFile(func(f *models.File) bool {
			return f.HasMergeConflicts && (f.IsSubmodule(submodules) || !f.HasInlineMergeConflicts)
		}) {
			a.setStatus(tr.DisabledMenuItemPrefix+utils.ResolvePlaceholderString(tr.StageConflictsRangeDisabled,
				map[string]string{"goIntoKey": a.repo.Config.Keybinding.Universal.GoInto.String()}), true)
			return
		}
	}
	for _, n := range nodes {
		if n.GetHasInlineMergeConflicts() {
			a.run("Staging…", func() error { return errors.New(a.repo.Common.Tr.ErrStageDirWithInlineMergeConflicts) }, nil)
			return
		}
	}
	nodes = normalisedSelectedNodes(nodes)
	if a.files.IsFiltering() {
		a.run("Staging…", func() error { return a.repo.ToggleStagedVisibleNodes(nodes...) }, nil)
		return
	}
	unstaged := lo.Filter(nodes, func(n *filetree.FileNode, _ int) bool {
		// lazygit's fileHasStageableUnstagedChanges
		return n.SomeFile(func(f *models.File) bool {
			return f.HasUnstagedChanges && (!f.IsSubmodule(submodules) || !f.HasStagedChanges)
		})
	})
	paths := func(nodes []*filetree.FileNode) []string {
		return lo.Map(nodes, func(n *filetree.FileNode, _ int) string { return n.GetPath() })
	}
	wt := a.repo.Git.WorkingTree
	if len(unstaged) > 0 {
		ps := paths(unstaged)
		a.run("Staging…", func() error { return a.repo.Locked(func() error { return wt.StageFiles(ps, nil) }) }, nil)
		return
	}
	tracked, untracked := lo.FilterReject(nodes, func(n *filetree.FileNode, _ int) bool { return !n.IsFile() || n.GetIsTracked() })
	a.run("Unstaging…", func() error {
		return a.repo.Locked(func() error {
			if len(untracked) > 0 {
				if err := wt.UnstageUntrackedFiles(paths(untracked)); err != nil {
					return err
				}
			}
			if len(tracked) > 0 {
				return wt.UnstageTrackedFiles(paths(tracked))
			}
			return nil
		})
	}, nil)
}

package repo

import (
	"errors"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
)

// While a text filter is on, staging acts only on the files that are
// visible, never on a whole directory (lazygit's FilesController with
// IsFiltering).

func filePaths(nodes ...*filetree.FileNode) []string {
	var paths []string
	for _, n := range nodes {
		_ = n.ForEachFile(func(f *models.File) error {
			paths = append(paths, f.Path)
			return nil
		})
	}
	return paths
}

// unstageVisible is FilesController.unstageFilteredFiles.
func (r *Repo) unstageVisible(nodes ...*filetree.FileNode) error {
	var tracked, untracked []string
	for _, n := range nodes {
		_ = n.ForEachFile(func(f *models.File) error {
			if f.Tracked || f.HasStagedChanges {
				tracked = append(tracked, f.Path)
			} else {
				untracked = append(untracked, f.Path)
			}
			return nil
		})
	}
	if len(untracked) > 0 {
		if err := r.Git.WorkingTree.UnstageUntrackedFiles(untracked); err != nil {
			return err
		}
	}
	if len(tracked) > 0 {
		return r.Git.WorkingTree.UnstageTrackedFiles(tracked)
	}
	return nil
}

// ToggleStagedVisible stages the visible files below n when any has unstaged
// changes, else unstages them. n is a directory or the tree's root.
func (r *Repo) ToggleStagedVisible(n *filetree.FileNode) error {
	return r.ToggleStagedVisibleNodes(n)
}

func (r *Repo) ToggleStagedVisibleNodes(nodes ...*filetree.FileNode) error {
	return r.locked(func() error {
		unstaged := false
		for _, n := range nodes {
			unstaged = unstaged || n.GetHasUnstagedChanges()
			if n.GetHasInlineMergeConflicts() {
				return errors.New(r.Common.Tr.ErrStageDirWithInlineMergeConflicts)
			}
		}
		if unstaged {
			return r.Git.WorkingTree.StageFiles(filePaths(nodes...), nil)
		}
		return r.unstageVisible(nodes...)
	})
}

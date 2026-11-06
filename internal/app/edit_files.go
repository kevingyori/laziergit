package app

import (
	"errors"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"path/filepath"

	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"
)

// absPath is filepath.Abs as lazygit sees it: relative to the worktree.
func (a *App) absPath(path string) string {
	return filepath.Join(a.repo.Git.RepoPaths.WorktreePath(), path)
}

// editFiles is lazygit's FilesHelper.EditFiles (paths already absolute).
func (a *App) editFiles(abs []string) {
	cmd, suspend := a.repo.Git.File.GetEditCmdStr(abs)
	a.callEditor(cmd, suspend)
}

// callEditor is lazygit's FilesHelper.callEditor.
func (a *App) callEditor(cmd string, suspend bool) {
	if suspend {
		a.runSubprocess(func() (string, error) { return cmd, nil })
		return
	}
	shell := a.repo.OS.Cmd.NewShell(cmd, a.repo.Config.OS.ShellFunctionsFile)
	a.bg(func() {
		err := shell.Run()
		a.post(func() {
			if err != nil {
				a.errorPopup(err.Error())
			}
		})
	})
}

// openFile is lazygit's FilesHelper.OpenFile.
func (a *App) openFile(abs string) {
	a.bg(func() {
		err := a.repo.OS.OpenFile(abs)
		a.post(func() {
			if err != nil {
				a.errorPopup(err.Error())
			}
		})
	})
}

// editSelectedFiles is FilesController.edit.
func (a *App) editSelectedFiles() {
	nodes := a.selectedFileNodes()
	paths := lo.FilterMap(nodes, func(n *filetree.FileNode, _ int) (string, bool) { return a.absPath(n.GetPath()), n.IsFile() })
	if len(paths) == 0 {
		if len(nodes) > 0 {
			a.errorPopup(a.repo.Common.Tr.ErrCannotEditDirectory)
		}
		return
	}
	a.editFiles(paths)
}

// openSelectedFile is FilesController.Open.
func (a *App) openSelectedFile() {
	if a.selectingRange(panelFiles) {
		return
	}
	if n := a.selectedFileNode(); n != nil {
		a.openFile(a.absPath(n.GetPath()))
	}
}

// existingPathsOfDiff is lazygit's DiffHelper.existingPathsInWorktreeOfDiff.
func (a *App) existingPathsOfDiff(paths []string) ([]string, error) {
	abs := lo.Map(paths, func(p string, _ int) string { return a.absPath(p) })
	existing := lo.Filter(abs, func(p string, _ int) bool {
		ok, err := a.repo.OS.FileExists(p)
		return ok || err != nil
	})
	if len(existing) > 0 {
		return existing, nil
	}
	tr := a.repo.Common.Tr
	if len(abs) == 1 {
		return nil, errors.New(utils.ResolvePlaceholderString(tr.FileOfDiffDoesNotExist, map[string]string{"path": abs[0]}))
	}
	return nil, errors.New(utils.ResolvePlaceholderString(tr.NoFileOfDiffExists, map[string]string{"worktreePath": a.repo.Git.RepoPaths.WorktreePath()}))
}

// editCommitFiles is CommitFilesController.edit.
func (a *App) editCommitFiles() {
	cf := a.commitFiles
	if cf == nil || cf.tree.Len() == 0 {
		return
	}
	start, end := a.selectionRange(panelCommits)
	var paths []string
	for i := start; i <= end && i < cf.tree.Len(); i++ {
		if n := cf.tree.Get(i); n.IsFile() {
			paths = append(paths, n.GetPath())
		}
	}
	if len(paths) == 0 {
		a.errorPopup(a.repo.Common.Tr.ErrCannotEditDirectory)
		return
	}
	abs, err := a.existingPathsOfDiff(paths)
	if err != nil {
		a.errorPopup(err.Error())
		return
	}
	a.editFiles(abs)
}

// openCommitFile is CommitFilesController.open.
func (a *App) openCommitFile() {
	n := a.selectedCommitFileNode()
	if n == nil || a.selectingRange(panelCommits) {
		return
	}
	abs, err := a.existingPathsOfDiff([]string{n.GetPath()})
	if err != nil {
		a.errorPopup(err.Error())
		return
	}
	a.openFile(abs[0])
}

// editDiffLine is lazygit's MainViewController.editDiffLine: open the file of
// the selected diff row in the editor at its line (a file header opens the
// file itself).
func (a *App) editDiffLine() {
	s := a.staging
	if s == nil || s.cursor < 0 || s.cursor >= len(s.rows) {
		return
	}
	r := s.rows[s.cursor]
	if !r.ok || r.path == "" {
		return
	}
	abs, err := a.existingPathsOfDiff([]string{r.path})
	if err != nil {
		a.errorPopup(err.Error())
		return
	}
	if r.newLine <= 0 || r.kind == patch.PATCH_HEADER {
		a.editFiles(abs)
		return
	}
	// lazygit's AdjustLineNumber: the line is carried forward to where it
	// is in the file now, through a diff of what is shown against it.
	var args []string
	switch {
	case s.staged:
		args = []string{"--", r.path}
	case !s.worktree:
		if ref := a.refForAdjustingLineNumber(); ref != "" {
			args = []string{ref, "--", r.path}
		}
	}
	line, r2 := r.newLine, a.repo
	a.bg(func() {
		if args != nil {
			if diff, err := r2.Git.Diff.GetDiff(false, append([]string{"--unified=0"}, args...)...); err == nil {
				line = patch.Parse(diff).AdjustLineNumber(line)
			}
		}
		a.post(func() {
			cmd, suspend := r2.Git.File.GetEditAtLineCmdStr(abs[0], line)
			a.callEditor(cmd, suspend)
		})
	})
}

// refForAdjustingLineNumber is lazygit's RefForAdjustingLineNumberInDiff of
// the side panel showing the diff: the commit whose diff it is.
func (a *App) refForAdjustingLineNumber() string {
	if cf := a.commitFiles; cf != nil && cf.commit != nil {
		return cf.commit.Hash()
	}
	if a.focus == panelCommits {
		if c := a.selectedCommit(); c != nil {
			return c.Hash()
		}
	}
	return ""
}

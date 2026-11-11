package app

import (
	"errors"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"path/filepath"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"

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
	abs := lo.Map(paths, func(p string, _ int) string { return a.pathInWorktreeOfDiff(p) })
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
	worktreePath, ok := a.otherWorktreeOfDiff()
	if !ok {
		worktreePath = a.repo.Git.RepoPaths.WorktreePath()
	}
	return nil, errors.New(utils.ResolvePlaceholderString(tr.NoFileOfDiffExists, map[string]string{"worktreePath": worktreePath}))
}

// otherWorktreeOfDiff is lazygit's DiffHelper.otherWorktreeOfDiff: the
// worktree that has the branch of the shown commits checked out, when that
// isn't the current one.
func (a *App) otherWorktreeOfDiff() (string, bool) {
	b := a.browse(a.focus)
	if cf := a.commitFiles; cf != nil {
		// CommitFilesContext.BranchOfDiff: its parent's branch.
		if b = nil; cf.fromSub {
			b = a.refBrowse
		}
	}
	if b == nil || a.snap == nil {
		return "", false
	}
	branch, ok := b.ref.(*models.Branch)
	if !ok || branch == nil {
		return "", false
	}
	w, ok := git_commands.WorktreeForBranch(branch, a.snap.Worktrees)
	if !ok || w.IsCurrent || w.IsPathMissing {
		return "", false
	}
	return w.Path, true
}

// pathInWorktreeOfDiff is lazygit's DiffHelper.pathInWorktreeOfDiff, made
// absolute (AbsolutePathOfFileOfDiff).
func (a *App) pathInWorktreeOfDiff(path string) string {
	worktreePath, ok := a.otherWorktreeOfDiff()
	if !ok {
		if filepath.IsAbs(path) {
			return path
		}
		return a.absPath(path)
	}
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(a.repo.Git.RepoPaths.WorktreePath(), path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return path
		}
		path = rel
	}
	return filepath.Join(worktreePath, path)
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
	a.editDiffRow(s.rows[s.cursor], s.staged, s.worktree)
}

// editDiffRow opens the file of diff row r in the editor at its line: r is
// a row of the staged or unstaged working-tree diff, or of a commit's.
func (a *App) editDiffRow(r diffRow, staged, worktree bool) {
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
	case staged:
		args = []string{"--", r.path}
	case !worktree:
		if ref := a.refForAdjustingLineNumber(); ref != "" {
			args = []string{ref, "--", r.path}
		}
	}
	line, r2 := r.newLine, a.repo
	other, inOther := a.otherWorktreeOfDiff()
	a.bg(func() {
		if args != nil {
			getDiff := func(args ...string) (string, error) { return r2.Git.Diff.GetDiff(false, args...) }
			if inOther {
				getDiff = func(args ...string) (string, error) { return r2.Git.Diff.GetDiffInOtherWorktree(other, args...) }
			}
			if diff, err := getDiff(append([]string{"--unified=0"}, args...)...); err == nil {
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
	if a.focus == panelCommits || a.subCommits() != nil {
		if c := a.selectedCommit(); c != nil {
			return c.Hash()
		}
	}
	return ""
}

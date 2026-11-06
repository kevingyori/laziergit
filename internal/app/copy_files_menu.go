package app

import (
	"path/filepath"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/i18n"
	"laziergit/internal/repo"
)

// copyMenuItem builds one row of lazygit's file copy menus: the text is read
// off the main thread, then copied and toasted.
func (a *App) copyMenuItem(label string, key rune, tooltip, disabled, toast string, text func() (string, error)) menuItem {
	r := a.repo
	it := menuItem{label: label, tooltip: tooltip, disabled: disabled, key: string(key), chord: runeBind(key)}
	it.run = func() {
		a.bg(func() {
			s, err := text()
			a.post(func() {
				if a.repo != r {
					return
				}
				if err == nil {
					err = a.copyText(s)
				}
				if err != nil {
					a.errorPopup(err.Error())
					return
				}
				if toast != "" {
					a.setStatus(toast, false)
				}
			})
		})
	}
	return it
}

func singleSelected(tr *i18n.TranslationSet, rangeSelected, none bool) string {
	switch {
	case rangeSelected:
		return tr.RangeSelectNotSupported
	case none:
		return tr.NoItemSelected
	}
	return ""
}

func orReason(reasons ...string) string {
	for _, r := range reasons {
		if r != "" {
			return r
		}
	}
	return ""
}

// openFilesCopyMenu is lazygit's FilesController.openCopyMenu.
func (a *App) openFilesCopyMenu() {
	tr := a.repo.Common.Tr
	r := a.repo
	node := a.selectedFileNode()
	start, end := a.selectionRange(panelFiles)
	single := singleSelected(tr, start != end, node == nil)
	root := r.Git.RepoPaths.WorktreePath()
	str := func(s func() string) func() (string, error) { return func() (string, error) { return s(), nil } }
	var name, path string
	diffDisabled := single
	hasStaged := false
	if node != nil {
		name, path = node.Name(), node.GetPath()
		if diffDisabled == "" && !node.GetHasStagedOrTrackedChanges() {
			diffDisabled = tr.NoContentToCopyError
		}
		hasStaged = node.SomeFile(func(f *models.File) bool { return f.HasStagedChanges })
	}
	anyStaged := a.anyFile(func(f *models.File) bool { return f.HasStagedChanges })
	allDisabled := ""
	if !anyStaged && !a.anyFile(func(f *models.File) bool { return f.Tracked }) {
		allDisabled = tr.NoContentToCopyError
	}
	a.pushOverlay(&menuOverlay{title: tr.CopyToClipboardMenu, items: []menuItem{
		a.copyMenuItem(tr.CopyFileName, 'n', "", single, tr.FileNameCopiedToast, str(func() string { return name })),
		a.copyMenuItem(tr.CopyRelativeFilePath, 'p', "", single, tr.FilePathCopiedToast, str(func() string { return path })),
		a.copyMenuItem(tr.CopyAbsoluteFilePath, 'P', "", single, tr.FilePathCopiedToast, str(func() string { return filepath.Join(root, path) })),
		a.copyMenuItem(tr.CopySelectedDiff, 's', tr.CopyFileDiffTooltip, diffDisabled, tr.FileDiffCopiedToast, func() (string, error) {
			return r.Git.Diff.GetDiff(hasStaged, "--", path)
		}),
		a.copyMenuItem(tr.CopyAllFilesDiff, 'a', tr.CopyFileDiffTooltip, allDisabled, tr.AllFilesDiffCopiedToast, func() (string, error) {
			return r.Git.Diff.GetDiff(anyStaged, "--")
		}),
	}})
}

// openCommitFilesCopyMenu is lazygit's CommitFilesController.openCopyMenu.
func (a *App) openCommitFilesCopyMenu() {
	cf := a.commitFiles
	if cf == nil {
		return
	}
	tr := a.repo.Common.Tr
	r := a.repo
	node := a.selectedCommitFileNode()
	start, end := a.selectionRange(panelCommits)
	single := singleSelected(tr, start != end, node == nil)
	any := ""
	if node == nil {
		any = tr.NoItemSelected
	}
	from, to, reverse := a.commitFileRefs()
	var name, path string
	var paths []string
	contentDisabled := single
	if node != nil {
		name, path = node.Name(), node.GetPath()
		paths = commitFileNodePaths(cf, node)
		if contentDisabled == "" && !node.IsFile() {
			contentDisabled = tr.ErrCannotCopyContentOfDirectory
		}
	}
	str := func(s string) func() (string, error) { return func() (string, error) { return s, nil } }
	a.pushOverlay(&menuOverlay{title: tr.CopyToClipboardMenu, items: []menuItem{
		a.copyMenuItem(tr.CopyFileName, 'n', "", single, tr.FileNameCopiedToast, str(name)),
		a.copyMenuItem(tr.CopyRelativeFilePath, 'p', "", single, tr.FilePathCopiedToast, str(path)),
		a.copyMenuItem(tr.CopyAbsoluteFilePath, 'P', "", single, tr.FilePathCopiedToast, str(a.pathInWorktreeOfDiff(path))),
		a.copyMenuItem(tr.CopySelectedDiff, 's', "", single, tr.FileDiffCopiedToast, func() (string, error) {
			return r.DiffPaths(from, to, reverse, paths)
		}),
		a.copyMenuItem(tr.CopyAllFilesDiff, 'a', "", any, tr.AllFilesDiffCopiedToast, func() (string, error) {
			return r.DiffPaths(from, to, reverse, []string{"."})
		}),
		a.copyMenuItem(tr.CopyFileContent, 'c', "", contentDisabled, tr.FileContentCopiedToast, func() (string, error) {
			return r.Git.Commit.ShowFileContentCmdObj(to, path).RunWithOutput()
		}),
	}})
}

func commitFileNodePaths(cf *commitFilesState, n *filetree.CommitFileNode) []string {
	return repo.DiffPathsForNode(n.Raw(), cf.tree.GetRoot().Raw(), cf.files, cf.tree.IsFiltering())
}

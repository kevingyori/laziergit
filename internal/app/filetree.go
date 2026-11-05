package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
)

// The Files and commit-files panels hold their rows in lazygit's own tree
// view models (pkg/gui/filetree): the same tree, compression, collapsing and
// selection keeping as lazygit, in tree or flat mode (gui.showFileTree).

func (a *App) newFileTrees() {
	r := a.repo
	a.files = filetree.NewFileTreeViewModel(func() []*models.File {
		if a.snap == nil {
			return nil
		}
		return a.snap.Files
	}, r.Common, r.Config.Gui.ShowFileTree)
}

func newCommitFileTree(a *App, commit *models.Commit, files []*models.CommitFile) *filetree.CommitFileTreeViewModel {
	vm := filetree.NewCommitFileTreeViewModel(func() []*models.CommitFile { return files }, a.repo.Common, a.repo.Config.Gui.ShowFileTree)
	vm.SetRef(commit)
	vm.SetTree()
	return vm
}

// updateTreeRows recomputes how the tree rows are drawn; call it after any
// change to a tree's contents, collapsing or mode.
func (a *App) updateTreeRows() {
	a.fileRows = treeRows(a.files.GetRoot().Raw(), a.files.CollapsedPaths(), filePreviousPath)
	if cf := a.commitFiles; cf != nil {
		cf.rows = treeRows(cf.tree.GetRoot().Raw(), cf.tree.CollapsedPaths(), commitFilePreviousPath)
	}
}

// syncFileTree rebuilds the files tree from the snapshot, letting lazygit
// keep the selection on the same node.
func (a *App) syncFileTree() {
	a.withTreeCursor(panelFiles, a.files.IListCursor, a.files.SetTree)
	a.updateTreeRows()
}

// selectedFileNode is the selected row of the Files tab: a file or a directory.
func (a *App) selectedFileNode() *filetree.FileNode {
	if a.fileTab != 0 || a.files.Len() == 0 {
		return nil
	}
	return a.files.Get(clamp(a.cursor[panelFiles], 0, a.files.Len()-1))
}

func (a *App) selectedCommitFileNode() *filetree.CommitFileNode {
	cf := a.commitFiles
	if cf == nil || cf.tree.Len() == 0 {
		return nil
	}
	return cf.tree.Get(clamp(a.cursor[panelCommits], 0, cf.tree.Len()-1))
}

// toggleCollapsed opens or closes the selected directory (lazygit's enter on
// a directory) in whichever tree has focus. It reports whether there was one.
func (a *App) toggleCollapsed() bool {
	switch {
	case a.focus == panelFiles:
		n := a.selectedFileNode()
		if n == nil || n.File != nil {
			return false
		}
		a.withTreeCursor(panelFiles, a.files.IListCursor, func() {
			a.files.ToggleCollapsed(n.GetInternalPath())
			a.files.SetTree()
		})
	case a.focus == panelCommits && a.commitFiles != nil:
		n := a.selectedCommitFileNode()
		if n == nil || n.File != nil {
			return false
		}
		vm := a.commitFiles.tree
		a.withTreeCursor(panelCommits, vm.IListCursor, func() {
			vm.ToggleCollapsed(n.GetInternalPath())
			vm.SetTree()
		})
	default:
		return false
	}
	a.updateTreeRows()
	a.loadMain()
	return true
}

// treeAction applies f to whichever file tree has focus and keeps the
// selected node selected.
func (a *App) treeAction(files func(*filetree.FileTreeViewModel), commitFiles func(*filetree.CommitFileTreeViewModel)) {
	switch {
	case a.focus == panelCommits && a.commitFiles != nil:
		vm := a.commitFiles.tree
		a.withTreeCursor(panelCommits, vm.IListCursor, func() { commitFiles(vm) })
	case a.focus == panelFiles:
		a.withTreeCursor(panelFiles, a.files.IListCursor, func() { files(a.files) })
	default:
		return
	}
	a.updateTreeRows()
	a.diffKey = ""
	a.loadMain()
}

func (a *App) collapseAll() {
	a.treeAction((*filetree.FileTreeViewModel).CollapseAll, (*filetree.CommitFileTreeViewModel).CollapseAll)
}

func (a *App) expandAll() {
	a.treeAction((*filetree.FileTreeViewModel).ExpandAll, (*filetree.CommitFileTreeViewModel).ExpandAll)
}

func (a *App) toggleTreeView() {
	a.treeAction((*filetree.FileTreeViewModel).ToggleShowTree, (*filetree.CommitFileTreeViewModel).ToggleShowTree)
}

// treeRow is one row of a file tree as lazygit draws it.
type treeRow struct {
	depth     int    // indentation
	name      string // path below the parent row, or "old → new" for a rename
	dir       bool
	collapsed bool
}

// treeRows walks a tree in lazygit's row order (presentation.renderAux), so
// row i is the view model's item i.
func treeRows[T any](root *filetree.Node[T], collapsed *filetree.CollapsedPaths, previousPath func(*T) string) []treeRow {
	var rows []treeRow
	var walk func(n *filetree.Node[T], treeDepth, depth int)
	walk = func(n *filetree.Node[T], treeDepth, depth int) {
		if n == nil {
			return
		}
		isRoot := treeDepth == -1
		if n.IsFile() {
			if !isRoot {
				rows = append(rows, treeRow{depth: depth, name: nodeName(n.GetInternalPath(), treeDepth, previousPath(n.File))})
			}
			return
		}
		isCollapsed := collapsed.IsCollapsed(n.GetInternalPath())
		if !isRoot {
			rows = append(rows, treeRow{depth: depth, name: nodeName(n.GetInternalPath(), treeDepth, ""), dir: true, collapsed: isCollapsed})
		}
		if isCollapsed {
			return
		}
		for _, child := range n.Children {
			walk(child, treeDepth+1+n.CompressionLevel, depth+1)
		}
	}
	walk(root, -1, -1)
	return rows
}

// nodeName is lazygit's fileNameAtDepth: the path segments below depth, "/"
// for the root item, and "previous → name" for a rename.
func nodeName(internalPath string, depth int, previous string) string {
	split := strings.Split(internalPath, "/")
	if depth == 0 && split[0] == "." {
		if len(split) == 1 {
			return "/"
		}
		depth = 1
	}
	depth = min(depth, len(split)-1)
	name := strings.Join(split[depth:], "/")
	if previous == "" {
		return name
	}
	prevSplit := filetree.SplitFileTreePath(previous, split[0] == ".")
	if len(prevSplit) == len(split) && strings.Join(split[:depth], "/") == strings.Join(prevSplit[:depth], "/") {
		previous = strings.Join(prevSplit[depth:], "/")
	}
	return previous + " → " + name
}

// dirState is a directory's staging state, for the main pane's cache key.
func dirState(n *filetree.FileNode) string {
	switch {
	case n.GetHasStagedChanges() && n.GetHasUnstagedChanges():
		return "partly staged"
	case n.GetHasStagedChanges():
		return "staged"
	}
	return "unstaged"
}

func filePreviousPath(f *models.File) string             { return f.PreviousPath }
func commitFilePreviousPath(f *models.CommitFile) string { return f.PreviousPath }

// dirRow draws a directory row: a disclosure chevron and the directory's
// path below its parent, coloured like lazygit (green when all staged,
// yellow when partly).
func dirRow(c *ui.Context, p *Palette, r treeRow, staged, unstaged bool) {
	chevron := "▾"
	if r.collapsed {
		chevron = "▸"
	}
	col := p.Text
	switch {
	case staged && !unstaged:
		col = p.Staged
	case staged:
		col = p.Unstaged
	}
	ui.Text(c, chevron).FontSize(12).TextColor(p.Faint).Width(18)
	ui.Text(c, r.name).TextColor(col).FontWeight(600).SingleLine().Shrink(1)
}

func indent(c *ui.Context, depth int) {
	if depth > 0 {
		ui.Box(c).Width(float32(14 * depth)).Shrink(0)
	}
}

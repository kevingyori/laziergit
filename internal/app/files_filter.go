package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/samber/lo"
)

// takeOverConflictFilter is the filter part of lazygit's refreshStateFiles:
// when conflicts appear the Files panel shows only conflicted files, unless
// the user chose a filter; once they're gone it shows everything again.
func (a *App) takeOverConflictFilter(files []*models.File, prevConflicts int) {
	conflicted := lo.FilterMap(files, func(f *models.File, _ int) (string, bool) { return f.Path, f.HasMergeConflicts })
	vm := a.files
	if len(conflicted) > 0 && prevConflicts == 0 {
		if vm.GetStatusFilter() == filetree.DisplayAll {
			vm.SetStatusFilter(filetree.DisplayConflicted)
		}
	} else if len(conflicted) == 0 && vm.GetStatusFilter() == filetree.DisplayConflicted {
		a.withTreeCursor(panelFiles, vm.IListCursor, func() { vm.SetStatusFilterPreservingSelection(filetree.DisplayAll) })
	}
	if vm.GetStatusFilter() == filetree.DisplayConflicted {
		vm.RememberConflictedPaths(conflicted)
	}
}

// filesFilterLabel is lazygit's FilesController.filteringLabel: the Files
// panel's subtitle.
func (a *App) filesFilterLabel() string {
	tr := a.repo.Common.Tr
	switch a.files.GetStatusFilter() {
	case filetree.DisplayStaged:
		return tr.FilterLabelStagedFiles
	case filetree.DisplayUnstaged:
		return tr.FilterLabelUnstagedFiles
	case filetree.DisplayTracked:
		return tr.FilterLabelTrackedFiles
	case filetree.DisplayUntracked:
		return tr.FilterLabelUntrackedFiles
	case filetree.DisplayConflicted:
		return tr.FilterLabelConflictingFiles
	}
	return ""
}

// statusFilterMenu is lazygit's FilesController.handleStatusFilterPressed.
func (a *App) statusFilterMenu() {
	tr := a.repo.Common.Tr
	current := a.files.GetStatusFilter()
	opts := []struct {
		label  string
		key    rune
		filter filetree.FileTreeDisplayFilter
	}{
		{tr.FilterStagedFiles, 's', filetree.DisplayStaged},
		{tr.FilterUnstagedFiles, 'u', filetree.DisplayUnstaged},
		{tr.FilterTrackedFiles, 't', filetree.DisplayTracked},
		{tr.FilterUntrackedFiles, 'T', filetree.DisplayUntracked},
		{tr.NoFilter, 'r', filetree.DisplayAll},
	}
	var items []menuItem
	for _, o := range opts {
		radio := "( ) "
		if o.filter == current {
			radio = "(•) "
		}
		items = append(items, menuItem{label: radio + o.label, key: string(o.key), chord: runeBind(o.key), run: func() { a.setStatusFiltering(o.filter) }})
	}
	a.pushOverlay(&menuOverlay{title: tr.FilteringMenuTitle, items: items})
}

func (a *App) setStatusFiltering(filter filetree.FileTreeDisplayFilter) {
	prev := a.files.GetStatusFilter()
	a.withTreeCursor(panelFiles, a.files.IListCursor, func() { a.files.SetStatusFilter(filter) })
	a.updateTreeRows()
	if prev != filter && (prev == filetree.DisplayUntracked || filter == filetree.DisplayUntracked) {
		a.repo.ForceShowUntracked = a.files.ForceShowUntracked()
		a.Refresh()
	} else {
		a.loadMain()
	}
}

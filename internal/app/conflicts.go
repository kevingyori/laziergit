package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/keymap"
)

func (a *App) resolveMenu(f *models.File) {
	if a.isSubmoduleCommitConflict(f) {
		a.submoduleConflictMenu(f)
		return
	}
	tr := a.repo.Common.Tr
	keep := menuItem{label: tr.MergeConflictKeepFile, key: "k", chord: keymap.Binding{{Rune: 'k'}}, run: func() {
		a.run("Resolving…", func() error { return a.repo.ResolveFile(f, "resolved") }, a.done("Resolved "+f.Path))
	}}
	del := menuItem{label: tr.MergeConflictDeleteFile, key: "d", chord: keymap.Binding{{Rune: 'd'}}, run: func() {
		a.run("Resolving…", func() error { return a.repo.ResolveFile(f, "delete") }, a.done("Resolved "+f.Path))
	}}
	switch f.ShortStatus { // lazygit's openFileConflictMenu for conflicts without markers
	case "DD":
		a.pushOverlay(&menuOverlay{title: tr.MergeConflictsTitle, items: []menuItem{del}})
		return
	case "DU", "UD":
		a.pushOverlay(&menuOverlay{title: tr.MergeConflictsTitle, items: []menuItem{del, keep}})
		return
	case "AU", "UA":
		a.pushOverlay(&menuOverlay{title: tr.MergeConflictsTitle, items: []menuItem{keep, del}})
		return
	}
	a.openConflict(f)
}

func (a *App) conflictFileMenu(f *models.File) {
	items := []menuItem{}
	for _, option := range []struct{ label, choice string }{

		{"Keep ours (entire file)", "ours"}, {"Keep theirs (entire file)", "theirs"},
		{"Mark manually edited file resolved", "resolved"}, {"Resolve by deleting file", "delete"},
	} {
		choice := option.choice
		items = append(items, menuItem{label: option.label, run: func() {
			a.confirm("Resolve "+f.Path, "Apply this resolution? The file is staged when no conflicts remain. During rebase, ours is the rebased branch; theirs is the replayed commit.", choice != "resolved", func() {
				a.run("Resolving…", func() error { return a.repo.ResolveFile(f, choice) }, a.done("Resolved "+f.Path))
			})
		}})
	}
	a.pushOverlay(&menuOverlay{title: "Resolve conflict · " + f.Path, items: items})
}

func (a *App) sequenceMenu() {
	var items []menuItem
	for _, kind := range []string{"rebase", "merge", "cherry-pick", "revert"} {
		for _, action := range []string{"continue", "abort"} {
			kind, action := kind, action
			items = append(items, menuItem{label: action + " " + kind, run: func() {
				a.confirm(action+" "+kind, "Run git "+kind+" --"+action+"?", action == "abort", func() {
					a.selectHeadAfterMerge(action)
					a.run("Updating…", func() error { return a.repo.SequenceControl(kind, action) }, nil)
				})
			}})
		}
	}
	a.pushOverlay(&menuOverlay{title: "Continue / abort operation", items: items})
}

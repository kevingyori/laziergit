package app

import (
	"errors"

	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/keymap"
)

func runeBind(r rune) keymap.Binding { return keymap.Binding{{Rune: r}} }

// stashSave is lazygit's handleStashSave: ask for a message (it may be
// empty), then stash with the chosen variant.
func (a *App) stashSave(stash func(message string) error) {
	tr := a.repo.Common.Tr
	a.pushOverlay(&promptOverlay{
		title: tr.StashChanges, hint: "Message for the stash entry (optional)",
		onSubmit: func(msg string) {
			a.run(tr.StashingStatus+"…", func() error {
				return a.repo.Locked(func() error { return stash(msg) })
			}, nil)
		},
	})
}

// stashOptionsMenu is lazygit's createStashMenu (`S` in files).
func (a *App) stashOptionsMenu() {
	tr := a.repo.Common.Tr
	st := a.repo.Git.Stash
	staged := func() bool { return a.anyFile(func(f *models.File) bool { return f.HasStagedChanges }) }
	dirty := func() bool { return a.anyFile(func(f *models.File) bool { return f.HasStagedChanges || f.Tracked }) }
	need := func(ok bool, msg string, then func()) func() {
		return func() {
			if !ok {
				a.errorPopup(errors.New(msg).Error())
				return
			}
			then()
		}
	}
	item := func(label string, r rune, run func()) menuItem {
		return menuItem{label: label, key: string(r), chord: runeBind(r), run: run}
	}
	a.pushOverlay(&menuOverlay{title: tr.StashOptions, items: []menuItem{
		item(tr.StashAllChanges, 'a', need(dirty(), tr.NoFilesToStash, func() { a.stashSave(st.Push) })),
		item(tr.StashAllChangesKeepIndex, 'i', need(dirty(), tr.NoFilesToStash, func() { a.stashSave(st.StashAndKeepIndex) })),
		item(tr.StashIncludeUntrackedChanges, 'U', func() { a.stashSave(st.StashIncludeUntrackedChanges) }),
		item(tr.StashStagedChanges, 's', need(staged(), tr.NoTrackedStagedFilesStash, func() { a.stashSave(st.SaveStagedChanges) })),
		item(tr.StashUnstagedChanges, 'u', need(dirty(), tr.NoFilesToStash, func() {
			if staged() {
				a.stashSave(st.StashUnstagedChanges)
			} else {
				a.stashSave(st.Push)
			}
		})),
	}})
}

// anyFile is lazygit's *ExceptSubmodules working tree checks.
func (a *App) anyFile(pred func(*models.File) bool) bool {
	for _, f := range a.snap.Files {
		if !f.IsSubmodule(a.snap.Submodules) && pred(f) {
			return true
		}
	}
	return false
}

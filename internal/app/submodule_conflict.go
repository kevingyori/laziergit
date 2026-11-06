package app

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"

	"laziergit/internal/keymap"
)

// Ported from lazygit's FilesController: a submodule whose commit conflicts
// (both sides moved the gitlink) has no conflict markers; it is resolved by
// picking one side's commit.

// isSubmoduleCommitConflict is FilesController.isSubmoduleCommitConflict.
func (a *App) isSubmoduleCommitConflict(f *models.File) bool {
	return f != nil && f.HasInlineMergeConflicts && f.IsSubmodule(a.snap.Submodules)
}

// submoduleConflictText is FilesController.renderSubmoduleConflict.
func (a *App) submoduleConflictText(path string) string {
	r := a.repo
	tr := r.Common.Tr
	sm := r.Git.Submodule
	_, ours, theirs, err := sm.GetConflictCommits(path)
	if err != nil {
		return ""
	}
	sideBlock := func(header, side, otherSide string) string {
		log, err := sm.ConflictSideLog(path, side, otherSide)
		if err != nil {
			return header
		}
		if log = strings.TrimRight(log, "\n"); log == "" {
			if log, err = sm.GetCommitSummary(path, side); err != nil {
				return header
			}
		}
		return header + "\n\n  " + strings.ReplaceAll(log, "\n", "\n  ")
	}
	hint := utils.ResolvePlaceholderString(tr.SubmoduleMergeConflictDescription, map[string]string{"path": path}) +
		"\n\n" + fmt.Sprintf(tr.MergeConflictPressEnterToResolve, r.Config.Keybinding.Universal.GoInto)
	lines, _, _ := utils.WrapViewLinesToWidth(true, false, hint, 70, 4) // lazygit's wrapForReadability
	return strings.Join([]string{
		strings.Join(lines, "\n"),
		sideBlock(tr.MergeConflictCurrentDiff, ours, theirs),
		sideBlock(tr.MergeConflictIncomingDiff, theirs, ours),
	}, "\n\n")
}

// submoduleConflictMenu is FilesController.openSubmoduleConflictMenu.
func (a *App) submoduleConflictMenu(f *models.File) {
	r := a.repo
	tr := r.Common.Tr
	path := f.GetPath()
	a.bg(func() {
		sm := r.Git.Submodule
		_, ours, theirs, err := sm.GetConflictCommits(path)
		label := func(text, sha string) string {
			if summary, err := sm.GetCommitSummary(path, sha); err == nil && summary != "" {
				return fmt.Sprintf("%s (%s)", text, summary)
			}
			return text
		}
		var current, incoming string
		if err == nil {
			current, incoming = label(tr.MergeConflictTakeCurrentCommit, ours), label(tr.MergeConflictTakeIncomingCommit, theirs)
		}
		a.post(func() {
			if a.repo != r {
				return
			}
			if err != nil {
				a.showError(tr.MergeConflictsTitle, err)
				return
			}
			resolve := func(sha string) func() {
				return func() {
					a.run("Resolving…", func() error {
						return r.Locked(func() error {
							if err := sm.CheckoutConflictCommit(path, sha); err != nil {
								return err
							}
							return r.Git.WorkingTree.StageFile(path)
						})
					}, nil)
				}
			}
			a.pushOverlay(&menuOverlay{title: tr.MergeConflictsTitle,
				body: utils.ResolvePlaceholderString(tr.SubmoduleMergeConflictDescription, map[string]string{"path": path}),
				items: []menuItem{
					{label: current, key: "c", chord: keymap.Binding{{Rune: 'c'}}, run: resolve(ours)},
					{label: incoming, key: "i", chord: keymap.Binding{{Rune: 'i'}}, run: resolve(theirs)},
				}})
		})
	})
}

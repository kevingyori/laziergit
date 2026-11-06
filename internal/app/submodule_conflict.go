package app

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
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

// conflictResolutionHint is FilesController.conflictResolutionHint, wrapped
// at lazygit's readable width.
func (a *App) conflictResolutionHint(description string) string {
	message := description + "\n\n" + fmt.Sprintf(a.repo.Common.Tr.MergeConflictPressEnterToResolve, a.repo.Config.Keybinding.Universal.GoInto)
	lines, _, _ := utils.WrapViewLinesToWidth(true, false, message, 70, 4) // wrapForReadability
	return strings.Join(lines, "\n")
}

// nonTextualConflictText is FilesController.renderNonTextualConflict: the
// resolution hint, plus the base diff for the modify/delete cases.
func (a *App) nonTextualConflictText(f *models.File) string {
	tr := a.repo.Common.Tr
	message := a.conflictResolutionHint(f.GetMergeStateDescription(tr))
	if f.ShortStatus != "DU" && f.ShortStatus != "UD" {
		return message
	}
	label := tr.MergeConflictCurrentDiff
	if f.ShortStatus == "DU" {
		label = tr.MergeConflictIncomingDiff
	}
	diff, _ := a.repo.Git.Diff.DiffCmdObj([]string{"--base", "--", f.GetPath()}, git_commands.DiffModePlain).RunWithOutput()
	return message + "\n\n" + label + "\n\n" + diff
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
		if log = strings.TrimRight(utils.Decolorise(log), "\n"); log == "" {
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

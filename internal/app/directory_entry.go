package app

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// directoryEntryText is FilesController.renderDirectoryEntry: for an entry
// that git status reports as a whole directory, an explanation of what it is,
// a titled rule and the status of the repository it is (or the files in it).
func (a *App) directoryEntryText(f *models.File, width int) string {
	r := a.repo
	tr := r.Common.Tr
	wt := r.Git.WorkingTree
	kb := r.Config.Keybinding
	ph := map[string]string{
		"path":       f.Path,
		"discardKey": kb.Universal.Remove.String(),
		"removeKey":  kb.Universal.Remove.String(),
		"ignoreKey":  kb.Files.IgnoreFile.String(),
	}
	var description, heading string
	var out string
	if f.DirectoryKind == models.PlainDirectory {
		description = utils.ResolvePlaceholderString(tr.PlainDirectoryDescription, ph)
		heading = utils.ResolvePlaceholderString(tr.PlainDirectoryFiles, ph)
		out, _ = wt.UntrackedFilesCmdObj(f.Path).RunWithOutput()
	} else {
		switch {
		case f.IsWorktree:
			description = utils.ResolvePlaceholderString(tr.WorktreeInsideRepoDescription, ph)
		case f.DirectoryKind == models.RemovedSubmodule:
			description = utils.ResolvePlaceholderString(tr.RemovedSubmoduleDescription, ph)
		default:
			hint := tr.NestedRepositoryDiscardHint
			if wt.NestedRepositoryHasOwnGitDir(f.Path) {
				hint = tr.NestedRepositoryDiscardWarning
			}
			description = utils.ResolvePlaceholderString(tr.NestedRepositoryDescription+"\n\n"+hint, ph)
		}
		heading = utils.ResolvePlaceholderString(tr.NestedRepositoryStatus, ph)
		out, _ = wt.NestedRepositoryStatusCmdObj(f.Path).RunWithOutput()
	}
	return wrapForReadability(description, width) + "\n\n" + titledRule(heading, width) + "\n\n" + out
}

// wrapForReadability is lazygit's: prose wraps at 70 columns in a wider view.
func wrapForReadability(message string, viewWidth int) string {
	const readableWidth = 70
	if viewWidth <= readableWidth {
		return message
	}
	lines, _, _ := utils.WrapViewLinesToWidth(true, false, message, readableWidth, 4)
	return strings.Join(lines, "\n")
}

// titledRule is a horizontal line across width with a title in it.
func titledRule(title string, width int) string {
	start := "── " + title + " "
	return start + strings.Repeat("─", max(width-utils.StringWidth(start), 0))
}

// rotateAllBranchesLog is StatusController.switchToOrRotateAllBranchesLogs:
// show the all-branches log, or the next (previous) of git.allBranchesLogCmds
// when it's already shown.
func (a *App) rotateAllBranchesLog(forward bool) {
	b := a.repo.Git.Branch
	if _, n := b.GetAllBranchesLogIdxAndCount(); n == 0 {
		return
	}
	if a.statusLogShown || a.repo.Config.Gui.StatusPanelView == "allBranchesLog" {
		if forward {
			b.RotateAllBranchesLogIdx()
		} else {
			b.RotateAllBranchesLogIdxBackward()
		}
	}
	a.statusLogShown = true
	a.loadMain()
}

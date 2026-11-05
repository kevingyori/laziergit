package app

import (
	"fmt"
	"strconv"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/modes/cherrypicking"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// Cherry-pick context keys: lazygit lets you copy from one list at a time.
const (
	cherryLocalCommits = "commits"
	cherrySubCommits   = "subCommits"
)

// copyCommit is lazygit's CherryPickHelper.CopyRange for the focused commit
// list (commits or subCommits).
func (a *App) copyCommit() {
	var list []*models.Commit
	var start, end int
	key := cherryLocalCommits
	if a.focus == panelBranches && a.refBrowse != nil && a.refBrowse.ref != nil {
		if len(a.refBrowse.commits) == 0 {
			return
		}
		list, key = a.refBrowse.commits, cherrySubCommits
		start, end = a.selectionRange(panelBranches)
	} else {
		if a.selectedCommit() == nil || a.commitFiles != nil {
			return
		}
		list = a.snap.Commits
		start, end = a.selectionRange(panelCommits)
	}
	cp := a.cherry
	if cp.ContextKey != key {
		cp.ContextKey = key
		cp.CherryPickedCommits = make([]*models.Commit, 0)
	}
	// After a paste the buffer is hidden but not cleared: start fresh.
	if cp.DidPaste {
		cp.CherryPickedCommits = nil
		cp.DidPaste = false
	}
	set := cp.SelectedHashSet()
	all := true
	for _, c := range list[start : end+1] {
		all = all && set.Includes(c.Hash())
	}
	for _, c := range list[start : end+1] {
		if all {
			cp.Remove(c, list)
		} else {
			cp.Add(c, list)
		}
	}
	a.syncCherryPick()
}

// syncCherryPick mirrors the mode into the list highlight and the status bar.
func (a *App) syncCherryPick() {
	a.copiedCommits = nil
	if a.cherry.Active() {
		a.copiedCommits = a.cherry.CherryPickedCommits
	}
}

func (a *App) resetCherryPick() {
	a.cherry = cherrypicking.New()
	a.syncCherryPick()
}

// cherryPickInfo is the text lazygit's mode bar shows while commits are
// copied; empty when the mode is off.
func (a *App) cherryPickInfo() string {
	if !a.cherry.Active() {
		return ""
	}
	tr := a.repo.Common.Tr
	text := tr.CommitsCopied
	if len(a.cherry.CherryPickedCommits) == 1 {
		text = tr.CommitCopied
	}
	return fmt.Sprintf("%d %s", len(a.cherry.CherryPickedCommits), text)
}

// pasteCommits is lazygit's CherryPickHelper.Paste.
func (a *App) pasteCommits() {
	if !a.cherry.CanPaste() {
		return
	}
	tr := a.repo.Common.Tr
	commits := append([]*models.Commit(nil), a.cherry.CherryPickedCommits...)
	prompt := utils.ResolvePlaceholderString(tr.SureCherryPick, map[string]string{"numCommits": strconv.Itoa(len(commits))})
	a.confirm(tr.CherryPick, prompt, false, func() {
		// lazygit keeps the selection on the same commit when the pasted ones
		// are added above it.
		keep := ""
		if c := a.selectedCommit(); c != nil && a.focus == panelCommits && a.commitFiles == nil {
			keep = c.Hash()
		}
		a.run(tr.CherryPickingStatus, func() error { return a.repo.CherryPick(commits) }, func() {
			if keep != "" && a.focus == panelCommits && a.commitFiles == nil {
				for i, c := range a.snap.Commits {
					if c.Hash() == keep {
						a.cursor[panelCommits] = i
						a.lists[panelCommits].ScrollIntoView(i)
						a.diffKey = ""
						a.loadMain()
						break
					}
				}
			}
			// Conflicts leave the copied commits alone, so the paste can be retried.
			if a.snap.Operation != "Cherry-picking" {
				a.cherry.DidPaste = true
				a.syncCherryPick()
			}
		})
	})
}

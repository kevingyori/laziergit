package app

import (
	"slices"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/context/traits"
)

// commitSelection is lazygit's refreshCommitsWithLimit KeepCommitSelectionByHash
// state: the selected range by hash and the previous conflicted commit.
// commitSelectionBehavior is lazygit's types.CommitSelectionBehavior.
type commitSelectionBehavior int

const (
	keepCommitSelectionByHash commitSelectionBehavior = iota
	selectHeadCommit
	keepCommitSelectionIndex
)

type commitSelection struct {
	behavior                commitSelectionBehavior
	active, ok              bool
	selected, start         string
	selectedTODO, startTODO bool
	mode                    traits.RangeSelectMode
	prevConflict            *models.Commit
}

func (a *App) captureCommitSelection() commitSelection {
	var c commitSelection
	if a.snap == nil || a.listScope(panelCommits) != scopeCommits {
		return c
	}
	c.active = true
	commits := a.snap.Commits
	for _, cm := range commits {
		if cm.Status == models.StatusConflicted {
			c.prevConflict = cm
			break
		}
	}
	sel := a.cursor[panelCommits]
	start, mode := sel, traits.RangeSelectModeNone
	if a.selectingRange(panelCommits) {
		start, mode = clamp(a.ranges[panelCommits].start, 0, len(commits)-1), a.ranges[panelCommits].mode
	}
	if !restorableHash(commits, sel) || !restorableHash(commits, start) {
		return c // only the conflict check applies
	}
	c.ok = true
	c.selected, c.selectedTODO = commits[sel].Hash(), commits[sel].IsTODO()
	c.start, c.startTODO = commits[start].Hash(), commits[start].IsTODO()
	c.mode = mode
	return c
}

func (a *App) restoreCommitSelection(c commitSelection) {
	if a.snap == nil || a.listScope(panelCommits) != scopeCommits || !c.active {
		return
	}
	commits := a.snap.Commits
	switch c.behavior {
	case selectHeadCommit:
		if i := models.HeadCommitIdx(commits); i >= 0 {
			a.cursor[panelCommits] = i
			a.cancelRange(panelCommits)
			a.lists[panelCommits].ScrollIntoView(i)
		}
		return
	case keepCommitSelectionIndex:
		return
	}
	for i, cm := range commits {
		if cm.Status == models.StatusConflicted {
			if c.prevConflict == nil || c.prevConflict.Hash() != cm.Hash() {
				a.cursor[panelCommits] = i
				a.cancelRange(panelCommits)
				a.lists[panelCommits].ScrollIntoView(i)
				return
			}
			break
		}
	}
	if !c.ok {
		return
	}
	sel, ok1 := findCommitByHash(commits, c.selected, c.selectedTODO)
	start, ok2 := findCommitByHash(commits, c.start, c.startTODO)
	if !ok1 || !ok2 {
		return
	}
	a.setRange(panelCommits, sel, start, c.mode)
}

func restorableHash(commits []*models.Commit, i int) bool {
	return i >= 0 && i < len(commits) && commits[i].Hash() != ""
}

// findCommitByHash is findCommitByHashPreferringTODOStatus.
func findCommitByHash(commits []*models.Commit, hash string, todo bool) (int, bool) {
	fallback := -1
	for i, c := range commits {
		if c.Hash() != hash {
			continue
		}
		if c.IsTODO() == todo {
			return i, true
		}
		if fallback == -1 {
			fallback = i
		}
	}
	return fallback, fallback != -1
}

// selectCommitsOfCurrentBranch is BasicCommitsController's `*`: select the
// commits above the first merge or merged commit.
func (a *App) selectCommitsOfCurrentBranch() {
	p, ok := a.focusedList()
	if !ok {
		return
	}
	var commits []*models.Commit
	switch a.listScope(p) {
	case scopeCommits:
		commits = a.snap.Commits
	case scopeRefCommits:
		commits = a.refBrowse.commits
		if b := a.browse(panelBranches); p == panelBranches && b != nil {
			commits = b.commits
		}
	}
	index := slices.IndexFunc(commits, func(c *models.Commit) bool { return c.IsMerge() || c.Status == models.StatusMerged })
	if index <= 0 {
		tr := a.repo.Common.Tr
		a.setStatus(tr.DisabledMenuItemPrefix+tr.NoCommitsThisBranch, true)
		return
	}
	mode := traits.RangeSelectModeNonSticky
	if a.selectingRange(p) && a.ranges[p].mode == traits.RangeSelectModeSticky {
		mode = traits.RangeSelectModeSticky
	}
	a.setRange(p, 0, index-1, mode)
	a.lists[p].ScrollIntoView(0)
	a.diffKey = ""
	a.loadMain()
}

// selectHeadAfterMerge is genericMergeCommand's selectHeadCommitOnSuccess:
// continuing a merge selects the merge commit it creates.
func (a *App) selectHeadAfterMerge(action string) {
	if action == "continue" && a.snap != nil && a.snap.Operation == "Merging" {
		a.nextCommitSel = selectHeadCommit
	}
}

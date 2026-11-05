package app

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

// withCommittableFiles is lazygit's WithEnsureCommittableFiles: offer to
// stage everything when nothing is staged.
func (a *App) withCommittableFiles(handler func()) {
	tr := a.repo.Common.Tr
	if len(a.snap.Files) == 0 {
		a.errorPopup(tr.NoFilesStagedTitle)
		return
	}
	if a.hasStaged() {
		handler()
		return
	}
	for _, f := range a.snap.Files {
		if f.IsNestedRepository() {
			a.errorPopup(a.repo.Common.Tr.StageNestedRepositoryDisabled)
			return
		}
	}
	stage := func() { a.run(tr.Actions.StageAllFiles+"…", a.repo.StageAll, handler) }
	if a.repo.Config.Gui.SkipNoStagedFilesWarning {
		stage()
		return
	}
	a.confirm(tr.NoFilesStagedTitle, tr.NoFilesStagedPrompt, false, stage)
}

// createFixupMenu is lazygit's createFixupCommit (`F` in commits).
func (a *App) createFixupMenu() {
	c := a.selectedCommit()
	if c == nil || a.commitFiles != nil {
		return
	}
	tr := a.repo.Common.Tr
	idx, commits, branches := a.cursor[panelCommits], a.snap.Commits, a.snap.Branches
	create := func() {
		a.run(tr.CreatingFixupCommitStatus+"…", func() error {
			return a.repo.CreateFixupCommit(c, idx, commits, branches)
		}, a.keepSelectionAfterFixup(idx))
	}
	a.pushOverlay(&menuOverlay{title: tr.CreateFixupCommit, items: []menuItem{
		{label: tr.FixupMenu_Fixup, key: "f", chord: runeBind('f'), run: func() { a.withCommittableFiles(create) }},
		{label: tr.FixupMenu_AmendWithChanges, key: "a", chord: runeBind('a'), run: func() {
			a.withCommittableFiles(func() { a.createAmendCommit(c, true) })
		}},
		{label: tr.FixupMenu_AmendWithoutChanges, key: "r", chord: runeBind('r'), run: func() { a.createAmendCommit(c, false) }},
	}})
}

func (a *App) createAmendCommit(c *models.Commit, includeFiles bool) {
	tr := a.repo.Common.Tr
	idx, commits, branches := a.cursor[panelCommits], a.snap.Commits, a.snap.Branches
	a.bg(func() {
		msg, err := a.repo.Git.Commit.GetCommitMessage(c.Hash())
		a.post(func() {
			if err != nil {
				a.errorPopup(err.Error())
				return
			}
			original, _, _ := strings.Cut(msg, "\n")
			summary, body, _ := strings.Cut(strings.TrimSpace(msg), "\n")
			draft := &commitDraft{summary: summary, description: strings.TrimSpace(body)}
			a.pushOverlay(&commitOverlay{title: tr.CreateAmendCommit, draft: draft, onSubmit: func(s, d string) {
				a.run(tr.CreatingFixupCommitStatus+"…", func() error {
					return a.repo.CreateAmendCommit(c, idx, commits, branches, original, s, d, includeFiles)
				}, a.keepSelectionAfterFixup(idx))
			}})
		})
	})
}

// fixupMenu is lazygit's fixup (`f` in commits) outside a rebase.
func (a *App) fixupMenu() {
	tr := a.repo.Common.Tr
	start, end := a.selectionRange(panelCommits)
	commits := a.snap.Commits
	do := func(keep bool) func() {
		return func() {
			a.run(tr.FixingStatus+"…", func() error { return a.repo.FixupCommitRange(commits, start, end, keep) }, nil)
		}
	}
	a.pushOverlay(&menuOverlay{title: tr.Fixup, items: []menuItem{
		{label: tr.Fixup, key: "f", chord: runeBind('f'), run: do(false)},
		{label: tr.FixupKeepMessage, key: "c", chord: runeBind('c'), run: do(true)},
	}})
}

// squashFixupsMenu is lazygit's squashFixupCommits (`S` in commits).
func (a *App) squashFixupsMenu() {
	tr := a.repo.Common.Tr
	if a.selectedCommit() == nil || a.commitFiles != nil {
		return
	}
	commits, sel := a.snap.Commits, a.cursor[panelCommits]
	cur, curIdx, curErr := currentBranchFixupBase(commits, tr.CannotSquashCommitsInCurrentBranch)
	disabled := ""
	if curErr != "" {
		disabled = curErr
	}
	squash := func(c *models.Commit, startIdx int) func() {
		return func() {
			a.run(tr.SquashingStatus+"…", func() error { return a.repo.SquashFixupsAbove(c) }, func() {
				a.cursor[panelCommits] = max(sel-countSquashableAbove(commits, sel, startIdx), 0)
			})
		}
	}
	a.pushOverlay(&menuOverlay{title: tr.SquashAboveCommits, items: []menuItem{
		{label: tr.SquashCommitsInCurrentBranch, key: "b", chord: runeBind('b'), disabled: disabled, run: squash(cur, curIdx)},
		{label: tr.SquashCommitsAboveSelectedCommit, key: "a", chord: runeBind('a'), run: squash(commits[sel], sel)},
	}})
}

// currentBranchFixupBase is findCommitForSquashFixupsInCurrentBranch: the
// last commit before the first merged or merge commit.
func currentBranchFixupBase(commits []*models.Commit, errText string) (*models.Commit, int, string) {
	for i, c := range commits {
		if c.IsMerge() || c.Status == models.StatusMerged {
			if i == 0 {
				return nil, -1, errText
			}
			return commits[i-1], i - 1, ""
		}
	}
	return nil, -1, errText
}

func isFixupSubject(subject string) (string, bool) {
	prefixes := []string{"fixup! ", "squash! ", "amend! "}
	trim := func(s string) (string, bool) {
		for _, p := range prefixes {
			if t, ok := strings.CutPrefix(s, p); ok {
				return t, true
			}
		}
		return s, false
	}
	s, ok := trim(subject)
	if !ok {
		return subject, false
	}
	for {
		var more bool
		if s, more = trim(s); !more {
			return s, true
		}
	}
}

// countSquashableAbove anticipates how many commits above selectedIdx the
// autosquash removes, to keep the selection on the same commit.
func countSquashableAbove(commits []*models.Commit, selectedIdx, rebaseStartIdx int) int {
	n := 0
	for i, c := range commits[:min(selectedIdx, rebaseStartIdx+1)] {
		if base, ok := isFixupSubject(c.Name); ok {
			for _, b := range commits[i+1 : rebaseStartIdx+1] {
				if strings.HasPrefix(b.Name, base) {
					n++
					break
				}
			}
		}
	}
	return n
}

// keepSelectionAfterFixup keeps the selection on the same commit: a fixup
// commit that stayed on top pushes everything down by one.
func (a *App) keepSelectionAfterFixup(idx int) func() {
	return func() {
		if len(a.snap.Commits) > 0 {
			if _, ok := isFixupSubject(a.snap.Commits[0].Name); ok {
				a.cursor[panelCommits] = clamp(idx+1, 0, a.count(panelCommits)-1)
			}
		}
	}
}

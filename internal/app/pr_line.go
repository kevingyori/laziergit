package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"github.com/samber/lo"
)

// pullRequestDiff is lazygit's PullRequestDiffContext.PullRequestDiff for the
// panel beneath the focused main view: the branch whose pull request would
// show the diff on screen ("" where none would) and the commits it is of.
func (a *App) pullRequestDiff() (string, []*models.Commit) {
	s := a.staging
	if s == nil || s.worktree || a.snap == nil {
		return "", nil
	}
	if a.focus == panelCommits && a.commitsView() == cvCommits && a.commitFiles == nil {
		i := a.cursor[panelCommits]
		if a.snap.Head == nil || i >= len(a.snap.Commits) {
			return "", nil
		}
		return a.snap.Head.Name, []*models.Commit{a.snap.Commits[i]}
	}
	if cf := a.commitFiles; cf != nil && cf.commit != nil && a.focus == panelCommits && a.snap.Head != nil {
		return a.snap.Head.Name, []*models.Commit{cf.commit}
	}
	if b := a.browse(panelBranches); a.focus == panelBranches && b != nil && b.ref != nil {
		if br, ok := b.ref.(*models.Branch); ok && a.cursor[panelBranches] < len(b.commits) {
			return br.Name, []*models.Commit{b.commits[a.cursor[panelBranches]]}
		}
	}
	return "", nil
}

// openPullRequestAtLine is MainViewController.openPullRequestAtSelectedLine
// with its disabled reasons.
func (a *App) openPullRequestAtLine() {
	tr := a.repo.Common.Tr
	s := a.staging
	disabled := func(reason string) { a.setStatus(tr.DisabledMenuItemPrefix+reason, true) }
	if s == nil || s.cursor < 0 || s.cursor >= len(s.rows) {
		disabled(tr.NothingToSelectInDiff)
		return
	}
	if a.diffing.ref != "" {
		disabled(tr.NotAvailableInDiffingMode)
		return
	}
	if s.staged && !s.worktree {
		disabled(tr.NotAvailableForCustomPatch)
		return
	}
	branch, commits := a.pullRequestDiff()
	pr := a.prs[branch]
	if branch == "" || pr == nil {
		a.errorPopup(tr.NoPullRequestForBranch)
		return
	}
	if !lo.EveryBy(commits, func(c *models.Commit) bool { return c.Status == models.StatusPushed }) {
		if len(commits) == 1 {
			disabled(tr.CommitNotInPullRequest)
		} else {
			disabled(tr.CommitsNotInPullRequest)
		}
		return
	}
	if len(commits) == 0 {
		return
	}
	r := s.rows[s.cursor]
	if !r.ok || r.path == "" {
		return
	}
	suffix := ""
	switch {
	case r.kind == patch.DELETION:
		suffix = fmt.Sprintf("L%d", r.oldLine)
	case r.kind == patch.ADDITION || r.kind == patch.CONTEXT || r.kind == patch.HUNK_HEADER:
		suffix = fmt.Sprintf("R%d", r.newLine)
	}
	sum := sha256.Sum256([]byte(r.path))
	url := fmt.Sprintf("%s/changes/%s#diff-%s%s", pr.Url, commits[0].Hash(), hex.EncodeToString(sum[:]), suffix)
	if err := a.openLink(url); err != nil {
		a.errorPopup(err.Error())
	}
}

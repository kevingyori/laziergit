package app

import (
	"strings"
)

func (a *App) rebaseAction(action string) {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	commits := a.snap.Commits
	start, end := a.selectionRange(panelCommits)
	i := a.cursor[panelCommits]
	if c.IsTODO() {
		// Editing the todo list of a rebase in progress rewrites nothing yet.
		a.run("Updating rebase todo…", func() error { return a.repo.RewriteRange(commits, start, end, action) }, func() {
			switch action {
			case "up":
				a.cursor[panelCommits] = clamp(i-1, 0, a.count(panelCommits)-1)
				a.ranges[panelCommits].start = max(a.ranges[panelCommits].start-1, 0)
			case "down":
				a.cursor[panelCommits] = clamp(i+1, 0, a.count(panelCommits)-1)
				a.ranges[panelCommits].start = min(a.ranges[panelCommits].start+1, a.count(panelCommits)-1)
			}
		})
		return
	}
	tr := a.repo.Common.Tr
	title, prompt := "Rewrite history", action+" “"+c.Name+"”? This rewrites this commit and its descendants."
	switch action {
	case "drop":
		title, prompt = tr.DropCommitTitle, tr.DropCommitPrompt
		if c.IsMerge() {
			prompt = tr.DropMergeCommitPrompt
		}
	case "squash":
		title, prompt = tr.Squash, tr.SureSquashThisCommit
	}
	a.confirm(title, prompt, true, func() {
		a.run("Rebasing…", func() error { return a.repo.RewriteRange(commits, start, end, action) }, a.done("Rebase applied"))
	})
}

func (a *App) rewordFlow() {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	commits, i := a.snap.Commits, a.cursor[panelCommits]
	a.bg(func() {
		msg, err := a.repo.Git.Commit.GetCommitMessage(c.Hash())
		a.post(func() {
			if err != nil {
				a.showError("Read commit message", err)
				return
			}
			summary, body, _ := strings.Cut(strings.TrimSpace(msg), "\n")
			draft := &commitDraft{summary: summary, description: strings.TrimSpace(body)}
			a.pushOverlay(&commitOverlay{title: a.repo.Common.Tr.Actions.RewordCommit, draft: draft, onSubmit: func(s, b string) {
				a.run("Rewording…", func() error { return a.repo.Reword(commits, i, s, b) }, a.done("Commit reworded"))
			}})
		})
	})
}

func (a *App) rebaseMenu() {
	a.pushOverlay(&menuOverlay{title: "Interactive rebase", items: []menuItem{
		{label: "Edit selected commit", run: func() { a.rebaseAction("edit") }},
		{label: "Squash into commit below", run: func() { a.rebaseAction("squash") }},
		{label: "Fixup into commit below", run: func() { a.rebaseAction("fixup") }},
		{label: "Reword selected commit", run: a.rewordFlow},
		{label: "Move selected commit up", run: func() { a.rebaseAction("up") }},
		{label: "Move selected commit down", run: func() { a.rebaseAction("down") }},
		{label: "Drop selected commit", danger: true, run: func() { a.rebaseAction("drop") }},
		{label: "Continue rebase", run: func() { a.run("Continuing…", func() error { return a.repo.RebaseControl("continue") }, nil) }},
		{label: "Skip current commit", danger: true, run: func() {
			a.confirm("Skip commit", "Discard the current rebase commit?", true, func() { a.run("Skipping…", func() error { return a.repo.RebaseControl("skip") }, nil) })
		}},
		{label: "Abort rebase", run: func() { a.run("Aborting…", func() error { return a.repo.RebaseControl("abort") }, nil) }},
	}})
}

// squashOrFixup is lazygit's s / f in commits, with its canSquashOrFixup
// disabled reasons shown as an error toast.
func (a *App) squashOrFixup(action string) {
	if a.selectedCommit() == nil {
		return
	}
	tr := a.repo.Common.Tr
	start, end := a.selectionRange(panelCommits)
	reason := ""
	if end >= len(a.snap.Commits)-1 {
		reason = tr.CannotSquashOrFixupFirstCommit
	} else {
		for _, c := range a.snap.Commits[start : end+1] {
			if c.IsMerge() {
				reason = tr.CannotSquashOrFixupMergeCommit
			}
		}
	}
	if reason != "" {
		a.setStatus(tr.DisabledMenuItemPrefix+reason, true)
		return
	}
	if action == "fixup" && !a.selectedCommit().IsTODO() && a.snap.Operation != "Rebasing" {
		a.fixupMenu()
		return
	}
	a.rebaseAction(action)
}

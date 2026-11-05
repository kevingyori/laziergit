package app

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

func (a *App) rebaseAction(action string) {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	commits, i := a.snap.Commits, a.cursor[panelCommits]
	if c.IsTODO() {
		// Editing the todo list of a rebase in progress rewrites nothing yet.
		a.run("Updating rebase todo…", func() error { return a.repo.Rewrite(commits, i, action) }, func() {
			switch action {
			case "up":
				a.cursor[panelCommits] = clamp(i-1, 0, a.count(panelCommits)-1)
			case "down":
				a.cursor[panelCommits] = clamp(i+1, 0, a.count(panelCommits)-1)
			}
		})
		return
	}
	a.confirm("Rewrite history", action+" “"+c.Name+"”? This rewrites this commit and its descendants.", true, func() {
		a.run("Rebasing…", func() error { return a.repo.Rewrite(commits, i, action) }, a.done("Rebase applied"))
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
			a.pushOverlay(&commitOverlay{title: "Reword commit · rewrites descendants", draft: draft, onSubmit: func(s, b string) {
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

func (a *App) copyCommit() {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	for i, existing := range a.copiedCommits {
		if existing.Hash() == c.Hash() {
			a.copiedCommits = append(a.copiedCommits[:i], a.copiedCommits[i+1:]...)
			a.setStatus("Removed copied commit", false)
			return
		}
	}
	a.copiedCommits = append(a.copiedCommits, c)
	a.setStatus(fmt.Sprintf("%d commit(s) copied · V to cherry-pick", len(a.copiedCommits)), false)
}
func (a *App) pasteCommits() {
	if len(a.copiedCommits) == 0 {
		a.setStatus("Copy commits with C first", true)
		return
	}
	commits := append([]*models.Commit(nil), a.copiedCommits...)
	// Preserve the order chosen by the user; lazygit's API takes newest first.
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	a.confirm("Cherry-pick", fmt.Sprintf("Apply %d copied commit(s) to the current branch, in copy order?", len(commits)), false, func() {
		a.run("Cherry-picking…", func() error { return a.repo.CherryPick(commits) }, func() { a.copiedCommits = nil; a.setStatus("Cherry-picked commits", false) })
	})
}

package app

import "strings"

func (a *App) rebaseAction(action string) {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	commits, i := a.snap.Commits, a.cursor[panelCommits]
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
		{label: "Drop selected commit", danger: true, run: func() { a.rebaseAction("drop") }},
		{label: "Continue rebase", run: func() { a.run("Continuing…", func() error { return a.repo.RebaseControl("continue") }, nil) }},
		{label: "Skip current commit", danger: true, run: func() {
			a.confirm("Skip commit", "Discard the current rebase commit?", true, func() { a.run("Skipping…", func() error { return a.repo.RebaseControl("skip") }, nil) })
		}},
		{label: "Abort rebase", run: func() { a.run("Aborting…", func() error { return a.repo.RebaseControl("abort") }, nil) }},
	}})
}

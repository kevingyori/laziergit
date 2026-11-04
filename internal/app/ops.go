package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/keymap"
	"laziergit/internal/repo"
)

// run executes a git operation off the main thread, then reloads the
// snapshot. Errors open an error overlay with git's own output; onOK runs on
// success.
func (a *App) run(label string, op func() error, onOK func()) {
	a.busy = label
	a.setStatus("", false)
	a.bg(func() {
		err := op()
		s := a.repo.Load()
		a.post(func() {
			a.busy = ""
			a.apply(s)
			if err != nil {
				a.showError(strings.TrimSuffix(label, "…"), err)
				return
			}
			if onOK != nil {
				onOK()
			}
		})
	})
}

func (a *App) done(msg string) func() { return func() { a.setStatus(msg, false) } }

// --- files ------------------------------------------------------------------

func (a *App) toggleStaged() {
	f := a.selectedFile()
	if f == nil {
		return
	}
	if f.HasMergeConflicts {
		a.resolveMenu(f)
		return
	}
	a.run("Staging…", func() error { return a.repo.ToggleStaged(f) }, nil)
}

func (a *App) toggleStagedAll() {
	if a.snap == nil || len(a.snap.Files) == 0 {
		return
	}
	for _, f := range a.snap.Files {
		if f.HasMergeConflicts {
			a.setStatus("Resolve conflicts individually first", true)
			return
		}
	}
	for _, f := range a.snap.Files {
		if f.HasUnstagedChanges {
			a.run("Staging all…", a.repo.StageAll, nil)
			return
		}
	}
	a.run("Unstaging all…", a.repo.UnstageAll, nil)
}

func (a *App) hasStaged() bool {
	for _, f := range a.snap.Files {
		if f.HasStagedChanges {
			return true
		}
	}
	return false
}

func (a *App) commitFlow(skipHooks bool) {
	if a.snap == nil {
		return
	}
	for _, f := range a.snap.Files {
		if f.HasMergeConflicts {
			a.setStatus("Resolve conflicts before committing", true)
			return
		}
	}
	if len(a.snap.Files) == 0 {
		a.setStatus("There are no files to commit", true)
		return
	}
	open := func() {
		title := "Commit"
		if skipHooks {
			title = "Commit without hooks"
		}
		a.pushOverlay(&commitOverlay{title: title, draft: &a.draft, onSubmit: func(summary, desc string) {
			a.run("Committing…", func() error { return a.repo.Commit(summary, desc, skipHooks) }, func() {
				a.draft = commitDraft{}
				a.setStatus("Committed: "+summary, false)
			})
		}})
	}
	if a.hasStaged() {
		open()
		return
	}
	a.confirm("No files staged", "You have not staged any files. Stage all files and commit?", false, func() {
		a.run("Staging all…", a.repo.StageAll, open)
	})
}

func (a *App) amendFlow() {
	if a.snap == nil || len(a.snap.Commits) == 0 {
		return
	}
	a.confirm("Amend last commit", "Add the staged changes to the last commit, keeping its message?", false, func() {
		a.run("Amending…", a.repo.AmendHead, a.done("Amended last commit"))
	})
}

func (a *App) discardMenu() {
	f := a.selectedFile()
	if f == nil {
		return
	}
	items := []menuItem{{
		label: "Discard all changes", key: "x", chord: keymap.Binding{{Rune: 'x'}}, danger: true,
		run: func() { a.run("Discarding…", func() error { return a.repo.DiscardAll(f) }, nil) },
	}}
	if f.HasStagedChanges && f.HasUnstagedChanges {
		items = append(items, menuItem{
			label: "Discard unstaged changes", key: "u", chord: keymap.Binding{{Rune: 'u'}}, danger: true,
			run: func() { a.run("Discarding…", func() error { return a.repo.DiscardUnstaged(f) }, nil) },
		})
	}
	a.pushOverlay(&menuOverlay{title: "Discard changes to " + f.Path, items: items})
}

func (a *App) stashFlow() {
	if a.snap == nil || len(a.snap.Files) == 0 {
		a.setStatus("No changes to stash", true)
		return
	}
	a.prompt("Stash changes", "", "Message for the stash entry (optional)", func(msg string) {
		a.run("Stashing…", func() error { return a.repo.StashPush(msg) }, a.done("Stashed changes"))
	})
}

func (a *App) fetchFlow() {
	a.run("Fetching…", a.repo.Fetch, a.done("Fetched"))
}

// --- sync -------------------------------------------------------------------

func (a *App) pushFlow() {
	h := a.snapHead()
	if h == nil {
		return
	}
	if h.UpstreamBranch == "" {
		a.prompt("Push "+h.Name, a.repo.UpstreamSuggestion(h.Name), "Enter upstream as '<remote> <branch>'", func(v string) {
			remote, branch, ok := repo.ParseUpstream(v)
			if !ok {
				a.setStatus("Invalid upstream: "+v, true)
				return
			}
			a.push(git_commands.PushOpts{
				SetUpstream: true, Remote: remote,
				Refspecs: []string{"refs/heads/" + h.Name + ":refs/heads/" + branch},
			}, fmt.Sprintf("Pushed %s → %s/%s", h.Name, remote, branch))
		})
		return
	}
	target := h.UpstreamRemote + "/" + h.UpstreamBranch
	if behind(h) {
		a.confirm("Force push", fmt.Sprintf("%s is behind %s. Force push (with lease)?", h.Name, target), true, func() {
			a.push(git_commands.PushOpts{ForceWithLease: true}, "Force pushed "+h.Name)
		})
		return
	}
	a.push(git_commands.PushOpts{}, "Pushed "+h.Name+" → "+target)
}

func (a *App) push(opts git_commands.PushOpts, okMsg string) {
	a.busy = "Pushing…"
	a.setStatus("", false)
	a.bg(func() {
		err := a.repo.Push(opts)
		s := a.repo.Load()
		a.post(func() {
			a.busy = ""
			a.apply(s)
			switch {
			case err != nil && !opts.ForceWithLease && strings.Contains(err.Error(), "Updates were rejected"):
				a.confirm("Push rejected", "The remote has commits you don't have. Force push (with lease)?", true, func() {
					o := opts
					o.ForceWithLease = true
					a.push(o, "Force pushed")
				})
			case err != nil:
				a.showError("Push", err)
			default:
				a.setStatus(okMsg, false)
			}
		})
	})
}

func (a *App) pullFlow() {
	h := a.snapHead()
	if h == nil {
		return
	}
	if h.UpstreamBranch == "" {
		a.prompt("Pull "+h.Name, a.repo.UpstreamSuggestion(h.Name), "Set upstream as '<remote> <branch>' and pull", func(v string) {
			remote, branch, ok := repo.ParseUpstream(v)
			if !ok {
				a.setStatus("Invalid upstream: "+v, true)
				return
			}
			a.run("Pulling…", func() error {
				if err := a.repo.SetUpstream(remote, branch); err != nil {
					return err
				}
				return a.repo.Pull()
			}, a.done("Pulled"))
		})
		return
	}
	a.run("Pulling…", a.repo.Pull, a.done("Pulled "+h.UpstreamRemote+"/"+h.UpstreamBranch))
}

func (a *App) snapHead() *models.Branch {
	if a.snap == nil {
		return nil
	}
	return a.snap.Head
}

func behind(b *models.Branch) bool {
	return b.BehindForPull != "" && b.BehindForPull != "0" && b.BehindForPull != "?"
}

// --- branches ---------------------------------------------------------------

func (a *App) checkoutSelected() {
	b := a.selectedBranch()
	if b == nil || b.Head {
		return
	}
	a.run("Checking out…", func() error { return a.repo.Checkout(b.Name) }, a.done("Checked out "+b.Name))
}

func (a *App) checkoutByNameFlow() {
	a.prompt("Checkout branch", "", "Branch name, or a ref such as a tag or commit", func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		a.run("Checking out…", func() error { return a.repo.Checkout(name) }, a.done("Checked out "+name))
	})
}

func (a *App) newBranchFlow() {
	base := ""
	if b := a.selectedBranch(); b != nil {
		base = b.Name
	}
	a.prompt("New branch", "", fmt.Sprintf("Branch off '%s'", base), func(name string) {
		name = strings.ReplaceAll(strings.TrimSpace(name), " ", "-")
		if name == "" {
			return
		}
		a.run("Creating branch…", func() error { return a.repo.NewBranch(name, base) }, func() {
			a.cursor[panelBranches] = 0
			a.setStatus("Created and checked out "+name, false)
		})
	})
}

func (a *App) deleteBranchFlow() {
	b := a.selectedBranch()
	if b == nil {
		return
	}
	if b.Head {
		a.setStatus("You cannot delete the checked out branch", true)
		return
	}
	a.confirm("Delete branch", fmt.Sprintf("Delete the local branch '%s'?", b.Name), true, func() {
		a.busy = "Deleting branch…"
		a.bg(func() {
			err := a.repo.DeleteBranch(b.Name, false)
			s := a.repo.Load()
			a.post(func() {
				a.busy = ""
				a.apply(s)
				switch {
				case err != nil && strings.Contains(err.Error(), "not fully merged"):
					a.confirm("Force delete branch", fmt.Sprintf("'%s' is not fully merged. Delete it anyway?", b.Name), true, func() {
						a.run("Deleting branch…", func() error { return a.repo.DeleteBranch(b.Name, true) }, a.done("Deleted "+b.Name))
					})
				case err != nil:
					a.showError("Delete branch", err)
				default:
					a.setStatus("Deleted "+b.Name, false)
				}
			})
		})
	})
}

// --- stash ------------------------------------------------------------------

func (a *App) stashApplyFlow() {
	if s := a.selectedStash(); s != nil {
		a.run("Applying stash…", func() error { return a.repo.StashApply(s.Index) }, a.done("Applied "+s.Name))
	}
}

func (a *App) stashPopFlow() {
	if s := a.selectedStash(); s != nil {
		a.run("Popping stash…", func() error { return a.repo.StashPop(s.Index) }, a.done("Popped "+s.Name))
	}
}

func (a *App) stashDropFlow() {
	s := a.selectedStash()
	if s == nil {
		return
	}
	a.confirm("Drop stash", fmt.Sprintf("Drop stash@{%d}: %s?", s.Index, s.Name), true, func() {
		a.run("Dropping stash…", func() error { return a.repo.StashDrop(s.Index) }, a.done("Dropped stash"))
	})
}

// --- commit files -----------------------------------------------------------

type commitFilesState struct {
	commit    *models.Commit
	commitIdx int
	files     []*models.CommitFile
}

func (a *App) enterCommitFiles() {
	c := a.selectedCommit()
	if c == nil || a.commitFiles != nil {
		return
	}
	idx := a.cursor[panelCommits]
	r := a.repo
	a.bg(func() {
		files, err := r.CommitFiles(c)
		a.post(func() {
			if a.repo != r {
				return
			}
			if err != nil {
				a.showError("Load commit files", err)
				return
			}
			if len(files) == 0 {
				a.setStatus("No files in this commit", false)
				return
			}
			a.commitFiles = &commitFilesState{commit: c, commitIdx: idx, files: files}
			a.cursor[panelCommits] = 0
			a.lists[panelCommits].ScrollTo(0, 0)
			a.diffKey = ""
			a.loadMain()
		})
	})
}

var errNoFile = errors.New("file is no longer changed")

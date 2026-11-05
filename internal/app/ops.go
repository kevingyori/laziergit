package app

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/utils"

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
			if err != nil && isMergeConflict(err) && s.Operation != "" {
				a.conflictsMenu(s.Operation)
				return
			}
			var ae *askError
			if errors.As(err, &ae) {
				ae.ask()
				return
			}
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

// conflictMarkers are the phrases git prints when a merge, rebase,
// cherry-pick or revert stops on conflicts (lazygit's isMergeConflictErr).
var conflictMarkers = []string{
	"Failed to merge in the changes", "When you have resolved this problem", "fix conflicts",
	"Resolve all conflicts manually", "Merge conflict in file", "hint: after resolving the conflicts",
	"CONFLICT (content):",
}

// askError is an error lazygit answers with a confirmation instead of an
// error popup.
type askError struct {
	err error
	ask func()
}

func (e *askError) Error() string { return e.err.Error() }

// autostashing wraps a branch switch: when git refuses because of local
// changes, offer lazygit's autostash (stash, retry, pop).
func (a *App) autostashing(label, stashMsg string, op func() error, onOK func()) {
	a.run(label, func() error {
		err := op()
		if err != nil && strings.Contains(err.Error(), "Please commit your changes or stash them before you switch branch") {
			return &askError{err: err, ask: func() {
				a.confirm(a.repo.Common.Tr.AutoStashTitle, a.repo.Common.Tr.AutoStashPrompt, false, func() {
					a.run(label, func() error {
						if err := a.repo.Git.Stash.Push(stashMsg); err != nil {
							return err
						}
						if err := op(); err != nil {
							return err
						}
						return a.repo.Git.Stash.Pop(0)
					}, onOK)
				})
			}}
		}
		return err
	}, onOK)
}

func isMergeConflict(err error) bool {
	for _, m := range conflictMarkers {
		if strings.Contains(err.Error(), m) {
			return true
		}
	}
	return false
}

// conflictsMenu is lazygit's "Conflicts!" prompt after an operation stops
// on conflicts: go resolve them, or abort.
func (a *App) conflictsMenu(operation string) {
	kind := map[string]string{"Rebasing": "rebase", "Merging": "merge", "Cherry-picking": "cherry-pick", "Reverting": "revert"}[operation]
	a.setStatus("Conflicts! Resolve them in the files panel", true)
	a.pushOverlay(&menuOverlay{title: "Conflicts!", items: []menuItem{
		{label: "View conflicts", run: func() { a.focusPanel(panelFiles); a.fileTab = 0 }},
		{label: "Abort the " + kind, danger: true, run: func() {
			a.run("Aborting…", func() error { return a.repo.SequenceControl(kind, "abort") }, a.done("Aborted the "+kind))
		}},
	}})
}

func (a *App) done(msg string) func() { return func() { a.setStatus(msg, false) } }

// --- files ------------------------------------------------------------------

func (a *App) toggleStaged() {
	n := a.selectedFileNode()
	if n == nil {
		return
	}
	if n.File == nil {
		if a.files.IsFiltering() { // only the visible files below it
			a.run("Staging…", func() error { return a.repo.ToggleStagedVisible(n) }, nil)
			return
		}
		// A directory: stage it if anything below is unstaged, else unstage it.
		a.run("Staging…", func() error { return a.repo.ToggleStagedDir(n) }, nil)
		return
	}
	f := n.File
	if f.HasMergeConflicts {
		a.resolveMenu(f)
		return
	}
	a.run("Staging…", func() error { return a.repo.ToggleStaged(f) }, nil)
}

func (a *App) toggleStagedAll() {
	if a.snap == nil {
		return
	}
	if a.files == nil || a.files.Len() == 0 { // lazygit's anyFilesDisplayed
		a.setStatus(a.repo.Common.Tr.DisabledMenuItemPrefix+a.repo.Common.Tr.NoChangedFiles, true)
		return
	}
	if a.files.IsFiltering() { // only the visible files
		a.run("Staging all…", func() error { return a.repo.ToggleStagedVisible(a.files.GetRoot()) }, nil)
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
	tr := a.repo.Common.Tr
	if len(a.snap.Files) == 0 {
		a.errorPopup(tr.NoFilesStagedTitle)
		return
	}
	open := func() {
		if a.draft.summary == "" && a.draft.description == "" {
			a.draft.summary = a.initialCommitMessage(skipHooks)
		}
		a.pushOverlay(&commitOverlay{title: tr.CommitSummaryTitle, draft: &a.draft, onSubmit: func(summary, desc string) {
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
	for _, f := range a.snap.Files {
		if f.IsNestedRepository() {
			text := tr.StageNestedRepositoryDisabled
			if f.IsWorktree {
				text = tr.StageWorktreeDisabled
			}
			keys := a.repo.Config.Keybinding
			a.errorPopup(utils.ResolvePlaceholderString(text, map[string]string{
				"path": f.Path, "discardKey": keys.Universal.Remove.String(), "removeKey": keys.Universal.Remove.String(),
				"ignoreKey": keys.Files.IgnoreFile.String(),
			}))
			return
		}
	}
	a.confirm(tr.NoFilesStagedTitle, tr.NoFilesStagedPrompt, false, func() {
		a.run("Staging all…", a.repo.StageAll, open)
	})
}

// initialCommitMessage is lazygit's prefilled commit message: the skip-hooks
// prefix for a WIP commit, otherwise the first matching commitPrefix pattern
// applied to the branch name.
func (a *App) initialCommitMessage(wip bool) string {
	cfg := a.repo.Config
	if wip {
		return cfg.Git.SkipHookPrefix
	}
	prefixes := cfg.Git.CommitPrefix
	if p, ok := cfg.Git.CommitPrefixes[a.repo.Git.RepoPaths.RepoName()]; ok {
		prefixes = append(append([]config.CommitPrefixConfig(nil), p...), prefixes...)
	}
	branch := ""
	if h := a.snapHead(); h != nil {
		branch = h.Name
	}
	for _, p := range prefixes {
		if p.Pattern == "" {
			continue
		}
		rgx, err := regexp.Compile(p.Pattern)
		if err != nil {
			a.errorPopup(a.repo.Common.Tr.CommitPrefixPatternError + ": " + err.Error())
			return ""
		}
		if rgx.MatchString(branch) {
			return rgx.ReplaceAllString(branch, p.Replace)
		}
	}
	return ""
}

// errorPopup is lazygit's error alert.
func (a *App) errorPopup(msg string) {
	a.pushOverlay(&errorOverlay{title: a.repo.Common.Tr.Error, body: msg})
}

func (a *App) amendFlow() {
	if a.snap == nil || len(a.snap.Commits) == 0 {
		return
	}
	a.confirm(a.repo.Common.Tr.AmendLastCommitTitle, a.repo.Common.Tr.SureToAmend, false, func() {
		a.run("Amending…", a.repo.AmendHead, a.done("Amended last commit"))
	})
}

func (a *App) discardMenu() {
	n := a.selectedFileNode()
	if n == nil {
		return
	}
	if n.File == nil {
		tr := a.repo.Common.Tr
		disabled := ""
		if !n.GetHasStagedChanges() || !n.GetHasUnstagedChanges() {
			disabled = tr.DiscardUnstagedDisabled
		}
		items := []menuItem{
			{
				label: tr.DiscardAllChanges, key: "x", chord: keymap.Binding{{Rune: 'x'}}, danger: true,
				run: func() {
					a.confirmDiscardingNested(n, func() { a.run("Discarding…", func() error { return a.repo.DiscardDir(n, false) }, nil) })
				},
			},
			{
				label: tr.DiscardUnstagedChanges, key: "u", chord: keymap.Binding{{Rune: 'u'}}, danger: true, disabled: disabled,
				run: func() {
					a.confirmDiscardingNested(n, func() { a.run("Discarding…", func() error { return a.repo.DiscardDir(n, true) }, nil) })
				},
			},
		}
		a.pushOverlay(&menuOverlay{title: tr.DiscardChangesTitle, items: items})
		return
	}
	f := n.File
	items := []menuItem{{
		label: "Discard all changes", key: "x", chord: keymap.Binding{{Rune: 'x'}}, danger: true,
		run: func() {
			a.confirmDiscardingNested(n, func() { a.run("Discarding…", func() error { return a.repo.DiscardAll(f) }, nil) })
		},
	}}
	if f.HasStagedChanges && f.HasUnstagedChanges {
		items = append(items, menuItem{
			label: "Discard unstaged changes", key: "u", chord: keymap.Binding{{Rune: 'u'}}, danger: true,
			run: func() {
				a.confirmDiscardingNested(n, func() { a.run("Discarding…", func() error { return a.repo.DiscardUnstaged(f) }, nil) })
			},
		})
	}
	a.pushOverlay(&menuOverlay{title: a.repo.Common.Tr.DiscardChangesTitle, items: items})
}

// confirmDiscardingNested is lazygit's confirmDiscardingNestedRepositories:
// ask first if discarding n would lose something of a nested repository that
// can't be restored.
func (a *App) confirmDiscardingNested(n *filetree.FileNode, discard func()) {
	var nested []*models.File
	_ = n.ForEachFile(func(f *models.File) error {
		if f.IsNestedRepository() {
			nested = append(nested, f)
		}
		return nil
	})
	if len(nested) == 0 {
		discard()
		return
	}
	tr := a.repo.Common.Tr
	wt := a.repo.Git.WorkingTree
	a.busy = tr.CheckingNestedRepositories
	a.bg(func() {
		var losses []string
		for _, f := range nested {
			ph := map[string]string{"path": f.Path}
			if wt.NestedRepositoryHasOwnGitDir(f.Path) {
				losses = append(losses, utils.ResolvePlaceholderString(tr.NestedRepositoryHistoryWouldBeLost, ph))
				continue
			}
			// If we can't tell, assume that there are changes, so that we ask
			if changes, err := wt.NestedRepositoryHasUncommittedChanges(f.Path); changes || err != nil {
				losses = append(losses, utils.ResolvePlaceholderString(tr.NestedRepositoryChangesWouldBeLost, ph))
			}
		}
		a.post(func() {
			a.busy = ""
			if len(losses) == 0 {
				discard()
				return
			}
			a.confirm(tr.DiscardChangesTitle, tr.DiscardNestedRepositoriesPrompt+"\n\n- "+strings.Join(losses, "\n- "), true, discard)
		})
	})
}

func (a *App) stashFlow() { a.stashSave(a.repo.Git.Stash.Push) }

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
		a.prompt(a.repo.Common.Tr.EnterUpstream, a.repo.UpstreamSuggestion(h.Name), "", func(v string) {
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
	if h.IsBehindForPush() {
		a.confirm(a.repo.Common.Tr.ForcePush, a.forcePushPrompt(), true, func() {
			a.push(git_commands.PushOpts{ForceWithLease: true}, "Force pushed "+h.Name)
		})
		return
	}
	a.push(git_commands.PushOpts{}, "Pushed "+h.Name+" → "+target)
}

func (a *App) forcePushPrompt() string {
	return utils.ResolvePlaceholderString(a.repo.Common.Tr.ForcePushPrompt, map[string]string{
		"cancelKey":  a.repo.Config.Keybinding.Universal.Return.String(),
		"confirmKey": a.repo.Config.Keybinding.Universal.Confirm.String(),
	})
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
				a.confirm(a.repo.Common.Tr.ForcePush, a.forcePushPrompt(), true, func() {
					o := opts
					o.Force = true
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
		a.prompt(a.repo.Common.Tr.EnterUpstream, a.repo.UpstreamSuggestion(h.Name), "", func(v string) {
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
	a.autostashing("Checking out…", fmt.Sprintf(a.repo.Common.Tr.AutoStashForCheckout, b.Name), func() error { return a.repo.Checkout(b.Name) }, a.done("Checked out "+b.Name))
}

func (a *App) checkoutByNameFlow() {
	a.prompt(a.repo.Common.Tr.BranchName+":", "", "Branch name, or a ref such as a tag or commit", func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		tr := a.repo.Common.Tr
		a.run("Checking out…", func() error {
			err := a.repo.Checkout(name)
			if err != nil && strings.Contains(err.Error(), "did not match any file(s) known to git") {
				return &askError{err: err, ask: func() {
					a.confirm(tr.BranchNotFoundTitle, fmt.Sprintf("%s %s%s", tr.BranchNotFoundPrompt, name, "?"), false, func() {
						base := ""
						if b := a.selectedBranch(); b != nil {
							base = b.FullRefName()
						}
						a.run("Creating branch…", func() error { return a.repo.NewBranch(name, base) }, func() {
							a.cursor[panelBranches] = 0
							a.setStatus("Created and checked out "+name, false)
						})
					})
				}}
			}
			return err
		}, a.done("Checked out "+name))
	})
}

func (a *App) newBranchFlow() {
	base := ""
	if b := a.selectedBranch(); b != nil {
		base = b.Name
	}
	a.prompt(utils.ResolvePlaceholderString(a.repo.Common.Tr.NewBranchNameBranchOff, map[string]string{"branchName": base}), "", "", func(name string) {
		name = strings.ReplaceAll(strings.TrimSpace(name), " ", "-")
		if name == "" {
			return
		}
		a.autostashing("Creating branch…", fmt.Sprintf(a.repo.Common.Tr.AutoStashForNewBranch, name), func() error { return a.repo.NewBranch(name, base) }, func() {
			a.cursor[panelBranches] = 0
			a.setStatus("Created and checked out "+name, false)
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
	a.confirm(a.repo.Common.Tr.StashDrop, a.repo.Common.Tr.SureDropStashEntry, true, func() {
		a.run("Dropping stash…", func() error { return a.repo.StashDrop(s.Index) }, a.done("Dropped stash"))
	})
}

// --- commit files -----------------------------------------------------------

type commitFilesState struct {
	commit    *models.Commit
	commitIdx int
	fromRef   bool                              // opened from the branches panel's subCommits view
	files     []*models.CommitFile              // all changed files, independent of visible tree rows
	tree      *filetree.CommitFileTreeViewModel // lazygit's tree of the commit's files
	rows      []treeRow
}

func (a *App) enterCommitFiles() {
	c := a.selectedCommit()
	if c == nil || a.commitFiles != nil {
		return
	}
	idx := a.cursor[panelCommits]
	fromRef := a.focus == panelBranches && a.refBrowse != nil && a.refBrowse.ref != nil
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
			a.commitFiles = &commitFilesState{commit: c, commitIdx: idx, fromRef: fromRef, files: files, tree: newCommitFileTree(a, c, files)}
			a.updateTreeRows()
			a.cursor[panelCommits] = 0
			a.lists[panelCommits].ScrollTo(0, 0)
			if fromRef {
				a.focus = panelCommits // lazygit shows commitFiles in the commits window
			}
			a.diffKey = ""
			a.loadMain()
		})
	})
}

var errNoFile = errors.New("file is no longer changed")

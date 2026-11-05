package app

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gui/mergeconflicts"

	"laziergit/internal/keymap"
	"laziergit/internal/repo"
)

// bindings are the user's lazygit keybindings, parsed once.
type bindings struct {
	quit, ret, prev, next, prevBlock, nextBlock          keymap.Binding
	top, bottom, prevPage, nextPage, selectKey, goInto   keymap.Binding
	refresh, scrollUpMain, scrollDownMain, optionMenu    keymap.Binding
	togglePanel, toggleRange, rangeUp, rangeDown, remove keymap.Binding
	newKey, push, pull, extras, focusMain, confirmMenu   keymap.Binding
	confirmInEditor, submitEditor                        keymap.Binding
	jump                                                 []keymap.Binding
	files                                                config.KeybindingFilesConfig
	branches                                             config.KeybindingBranchesConfig
	commits                                              config.KeybindingCommitsConfig
	stash                                                config.KeybindingStashConfig
	main                                                 config.KeybindingMainConfig
}

func loadBindings(r *repo.Repo) bindings {
	kb := r.Config.Keybinding
	u := kb.Universal
	p := keymap.Parse
	b := bindings{
		quit: p(u.Quit), ret: p(u.Return), prev: p(u.PrevItem), next: p(u.NextItem),
		prevBlock: p(u.PrevBlock), nextBlock: p(u.NextBlock),
		top: p(u.GotoTop), bottom: p(u.GotoBottom), prevPage: p(u.PrevPage), nextPage: p(u.NextPage),
		selectKey: p(u.Select), goInto: p(u.GoInto), refresh: p(u.Refresh),
		scrollUpMain: p(u.ScrollUpMain), scrollDownMain: p(u.ScrollDownMain), optionMenu: p(u.OptionMenu),
		togglePanel: p(u.TogglePanel), toggleRange: p(u.ToggleRangeSelect),
		rangeUp: p(u.RangeSelectUp), rangeDown: p(u.RangeSelectDown), remove: p(u.Remove),
		newKey: p(u.New), push: p(u.Push), pull: p(u.Pull), extras: p(u.ExtrasMenu),
		focusMain: p(u.FocusMainView), confirmMenu: p(u.ConfirmMenu),
		confirmInEditor: append(p(u.ConfirmInEditor), p(u.ConfirmInEditorAlt)...),
		submitEditor:    p(u.SubmitEditorText),
		files:           kb.Files, branches: kb.Branches, stash: kb.Stash, commits: kb.Commits, main: kb.Main,
	}
	for _, j := range u.JumpToBlock {
		b.jump = append(b.jump, p(j))
	}
	return b
}

// scope says where an action applies.
type scope int

const (
	scopeGlobal scope = iota
	scopeStatus
	scopeFiles
	scopeBranches
	scopeCommits
	scopeCommitFiles
	scopeStash
	scopeStaging
	scopeMain
	scopeRemotes
	scopeTags
	scopeRefCommits
	scopeRemoteBranches
	scopeWorktrees
	scopeSubmodules
)

var scopeNames = map[scope]string{
	scopeRefCommits: "Commits", scopeRemoteBranches: "Remote branches", scopeWorktrees: "Worktrees", scopeSubmodules: "Submodules", scopeRemotes: "Remotes", scopeTags: "Tags", scopeGlobal: "Global", scopeStatus: "Status", scopeFiles: "Files", scopeBranches: "Branches",
	scopeCommits: "Commits", scopeCommitFiles: "Commit files", scopeStash: "Stash",
	scopeStaging: "Staging", scopeMain: "Main view",
}

// action is one entry of the keymap: the dispatcher, the hint bar and the
// keybindings menu are all generated from the same table.
type action struct {
	scope scope
	keys  keymap.Binding
	label string
	run   func()
	hint  bool // show in the hint bar
}

func (a *App) scope() scope {
	switch {
	case a.staging != nil:
		return scopeStaging
	case a.focus == panelBranches && !a.mainFocused && a.refBrowse != nil && a.refBrowse.ref != nil:
		return scopeRefCommits
	case a.focus == panelBranches && !a.mainFocused && a.refBrowse != nil:
		return scopeRemoteBranches
	case a.focus == panelFiles && !a.mainFocused && a.fileTab == 2:
		return scopeSubmodules
	case a.focus == panelFiles && !a.mainFocused && a.fileTab == 1:
		return scopeWorktrees
	case a.focus == panelBranches && !a.mainFocused && a.branchTab == 1:
		return scopeRemotes
	case a.focus == panelBranches && !a.mainFocused && a.branchTab == 2:
		return scopeTags
	case a.mainFocused:
		return scopeMain
	case a.focus == panelCommits && a.commitFiles != nil:
		return scopeCommitFiles
	}
	return [panelCount]scope{scopeStatus, scopeFiles, scopeBranches, scopeCommits, scopeStash}[a.focus]
}

func (a *App) buildActions() []action {
	k := a.keys
	p := keymap.Parse
	tr := a.repo.Common.Tr
	acts := []action{
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.Undo), "Undo last action", a.undoFlow, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.CreateRebaseOptionsMenu), tr.RebaseOptionsTitle, a.operationMenu, true},
		// Navigation (global; move() routes to whatever has focus).
		{scopeGlobal, k.next, "Next item", func() { a.move(1) }, false},
		{scopeGlobal, k.prev, "Previous item", func() { a.move(-1) }, false},
		{scopeGlobal, k.nextPage, "Next page", func() { a.move(a.pageSize()) }, false},
		{scopeGlobal, k.prevPage, "Previous page", func() { a.move(-a.pageSize()) }, false},
		{scopeGlobal, k.toggleRange, tr.ToggleRangeSelect, a.toggleRange, false},
		{scopeGlobal, k.rangeDown, tr.RangeSelectDown, func() { a.expandRange(1) }, false},
		{scopeGlobal, k.rangeUp, tr.RangeSelectUp, func() { a.expandRange(-1) }, false},
		{scopeGlobal, k.top, "Scroll to top", func() { a.move(-1 << 30) }, false},
		{scopeGlobal, k.bottom, "Scroll to bottom", func() { a.move(1 << 30) }, false},
		{scopeGlobal, k.nextBlock, "Next panel", func() { a.focusPanel((a.focus + 1) % panelCount) }, false},
		{scopeGlobal, k.prevBlock, "Previous panel", func() { a.focusPanel((a.focus + panelCount - 1) % panelCount) }, false},
		{scopeGlobal, k.focusMain, tr.FocusMainView, a.focusMainView, false},
		{scopeGlobal, k.scrollDownMain, "Scroll down main view", func() { a.scrollMain(3) }, false},
		{scopeGlobal, k.scrollUpMain, "Scroll up main view", func() { a.scrollMain(-3) }, false},
		{scopeGlobal, k.ret, "Return", a.back, false},
		{scopeGlobal, k.refresh, "Refresh", func() { a.Refresh(); a.RefreshPullRequests() }, false},
		{scopeGlobal, k.push, "Push", a.pushFlow, true},
		{scopeGlobal, k.pull, "Pull", a.pullFlow, true},
		{scopeGlobal, k.optionMenu, "Keybindings", a.openKeybindings, true},
		{scopeGlobal, k.extras, "Command log options", a.openExtras, false},
		{scopeGlobal, k.quit, "Quit", a.quit, false},

		// Files
		{scopeFiles, k.selectKey, "Stage / unstage", a.toggleStaged, true},
		{scopeFiles, p(k.files.ToggleStagedAll), "Stage / unstage all", a.toggleStagedAll, false},
		{scopeFiles, p(k.files.CommitChanges), "Commit", func() { a.commitFlow(false) }, true},
		{scopeFiles, p(k.files.CommitChangesWithoutHook), "Commit without hooks", func() { a.commitFlow(true) }, false},
		{scopeFiles, p(k.files.AmendLastCommit), "Amend last commit", a.amendFlow, false},
		{scopeFiles, k.goInto, tr.FileEnter, a.enterStaging, true},
		{scopeFiles, p(k.files.CollapseAll), tr.CollapseAll, a.collapseAll, false},
		{scopeFiles, p(k.files.ExpandAll), tr.ExpandAll, a.expandAll, false},
		{scopeFiles, p(k.files.ToggleTreeView), tr.ToggleTreeView, a.toggleTreeView, false},
		{scopeFiles, k.remove, "Discard", a.discardMenu, true},
		{scopeFiles, p(k.files.StashAllChanges), "Stash all changes", a.stashFlow, false},
		{scopeFiles, p(k.commits.ViewResetOptions), tr.ViewResetToUpstreamOptions, a.resetToUpstreamMenu, false},
		{scopeFiles, p(k.files.ViewResetOptions), tr.Reset, a.workspaceResetMenu, false},
		{scopeBranches, p(k.branches.SetUpstream), tr.ViewBranchUpstreamOptions, a.upstreamOptionsMenu, false},
		{scopeBranches, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, false},
		{scopeTags, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, false},
		{scopeRemoteBranches, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, false},
		{scopeCommits, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, false},
		{scopeFiles, p(k.files.IgnoreFile), tr.Actions.IgnoreExcludeFile, a.ignoreOrExcludeMenu, false},
		{scopeFiles, p(k.files.ViewStashOptions), tr.ViewStashOptions, a.stashOptionsMenu, false},
		{scopeFiles, p(k.files.Fetch), "Fetch", a.fetchFlow, false},
		{scopeFiles, p(k.files.RefreshFiles), "Refresh files", a.Refresh, false},

		{scopeBranches, p(a.repo.Config.Keybinding.Universal.NewWorktree), "New worktree", a.newWorktreeFlow, false},
		{scopeWorktrees, k.newKey, "New worktree", a.newWorktreeFlow, true},
		{scopeWorktrees, k.goInto, "Open worktree", a.openWorktree, true},
		{scopeWorktrees, k.selectKey, "Open worktree", a.openWorktree, false},
		{scopeWorktrees, k.remove, "Remove worktree", a.deleteWorktreeFlow, true},
		{scopeSubmodules, p(a.repo.Config.Keybinding.Submodules.BulkMenu), "Bulk actions", a.submoduleBulkMenu, false},
		{scopeSubmodules, k.newKey, "Add submodule", a.addSubmoduleFlow, true},
		{scopeSubmodules, k.goInto, "Open submodule", a.openSubmodule, true},
		{scopeSubmodules, p(a.repo.Config.Keybinding.Submodules.Init), "Initialize", a.initSubmodule, true},
		{scopeSubmodules, p(a.repo.Config.Keybinding.Submodules.Update), "Update", a.updateSubmodule, true},
		{scopeSubmodules, p(a.repo.Config.Keybinding.Universal.Edit), "Edit URL", a.submoduleURLFlow, false},
		{scopeSubmodules, k.remove, "Remove submodule", a.deleteSubmoduleFlow, true},
		{scopeStatus, k.ret, "Return to parent repository", a.returnToParent, false},
		// Branches
		{scopeBranches, k.goInto, tr.ViewCommits, a.enterRef, true},
		{scopeTags, k.goInto, tr.ViewCommits, a.enterRef, true},
		{scopeRemotes, k.goInto, "View remote branches", a.enterRef, true},
		{scopeRemoteBranches, k.goInto, tr.ViewCommits, a.enterRef, true},
		{scopeRemoteBranches, k.remove, tr.Delete, a.deleteBrowsedRemoteBranch, true},
		{scopeRemoteBranches, k.newKey, tr.NewBranch, a.newBranchFromBrowsedRemote, true},
		{scopeGlobal, p(k.commits.ResetCherryPick), tr.ResetCherryPick, a.resetCherryPick, false},
		{scopeRefCommits, k.goInto, "View files", a.enterCommitFiles, true},
		{scopeRefCommits, p(k.commits.CherryPickCopy), "Copy commit for cherry-pick", a.copyCommit, false},
		{scopeBranches, p(a.repo.Config.Keybinding.Universal.CopyToClipboard), tr.CopyToClipboardMenu, a.copySelectedItem, false},
		{scopeRemotes, p(a.repo.Config.Keybinding.Universal.CopyToClipboard), tr.CopyToClipboardMenu, a.copySelectedItem, false},
		{scopeTags, p(a.repo.Config.Keybinding.Universal.CopyToClipboard), tr.CopyToClipboardMenu, a.copySelectedItem, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.NextTab), "Next tab", func() { a.switchBranchTab(1) }, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.PrevTab), "Previous tab", func() { a.switchBranchTab(-1) }, false},
		{scopeRemotes, p(k.files.Fetch), "Fetch", a.fetchFlow, true},
		{scopeTags, k.selectKey, "Checkout tag", a.checkoutTag, true},
		{scopeTags, p(k.branches.PushTag), a.repo.Common.Tr.PushTag, a.pushTagFlow, false},
		{scopeBranches, k.selectKey, "Checkout", a.checkoutSelected, true},
		{scopeBranches, k.newKey, "New branch", a.newBranchFlow, true},
		{scopeBranches, p(k.branches.CheckoutBranchByName), "Checkout by name", a.checkoutByNameFlow, false},
		{scopeBranches, k.remove, "Delete branch", a.deleteBranchFlow, true},
		{scopeBranches, p(k.branches.RebaseBranch), "Rebase", a.rebaseFlow, true},
		{scopeBranches, p(k.branches.MergeIntoCurrentBranch), "Merge", a.mergeFlow, true},
		{scopeBranches, p(k.branches.FastForward), "Fast-forward", a.fastForwardFlow, false},
		{scopeBranches, p(k.branches.RenameBranch), "Rename branch", a.renameBranchFlow, false},
		{scopeBranches, p(k.branches.CreatePullRequest), "Create pull request", a.createPullRequestFlow, false},
		{scopeBranches, p(k.branches.ViewPullRequestOptions), "Pull request options", a.pullRequestMenu, false},
		{scopeBranches, p(k.branches.OpenPullRequestInBrowser), "Open pull request in browser", a.openPullRequestFlow, false},
		{scopeBranches, p(k.branches.CopyPullRequestURL), "Copy pull request URL", a.copyPullRequestURLFlow, false},

		// Commits
		{scopeCommits, p(k.commits.SetFixupMessage), tr.SetFixupMessage, a.fixupMessageMenu, false},
		{scopeCommits, p(k.commits.CopyCommitAttributeToClipboard), tr.CopyCommitAttributeToClipboard, a.copyCommitAttributeMenu, false},
		{scopeCommits, p(k.commits.CreateFixupCommit), tr.CreateFixupCommit, a.createFixupMenu, false},
		{scopeCommits, p(k.commits.SquashAboveCommits), tr.SquashAboveCommits, a.squashFixupsMenu, false},
		{scopeCommits, p(k.commits.ResetCommitAuthor), tr.AmendCommitAttribute, a.amendCommitAttribute, false},
		{scopeCommits, p(k.commits.AmendToCommit), tr.Amend, a.amendSelectedCommit, false},
		{scopeCommits, p(k.commits.CreateTag), tr.Actions.CreateLightweightTag, a.createCommitTag, false},
		{scopeCommits, p(k.commits.RevertCommit), tr.Actions.RevertCommit, a.revertCommit, false},
		{scopeCommits, p(k.commits.MoveUpCommit), "Move commit up", func() { a.rebaseAction("up") }, false},
		{scopeCommits, p(k.commits.MoveDownCommit), "Move commit down", func() { a.rebaseAction("down") }, false},
		{scopeCommits, p(k.commits.CherryPickCopy), "Copy commit for cherry-pick", a.copyCommit, false},
		{scopeCommits, p(k.commits.PasteCommits), "Cherry-pick copied commits", a.pasteCommits, false},
		{scopeCommits, p(k.commits.SquashDown), "Squash into commit below", func() { a.squashOrFixup("squash") }, true},
		{scopeCommits, p(k.commits.MarkCommitAsFixup), "Fixup into commit below", func() { a.squashOrFixup("fixup") }, false},
		{scopeCommits, p(k.commits.PickCommit), "Pick (rebase todo)", func() { a.rebaseAction("pick") }, false},
		{scopeCommits, k.remove, "Drop commit", func() { a.rebaseAction("drop") }, false},
		{scopeCommits, p(k.commits.RenameCommit), "Reword commit", a.rewordFlow, true},
		{scopeCommits, p(a.repo.Config.Keybinding.Universal.Edit), tr.Actions.EditCommit, func() { a.rebaseAction("edit") }, false},
		{scopeCommits, p(k.commits.StartInteractiveRebase), tr.QuickStartInteractiveRebase, a.quickStartRebase, true},
		{scopeCommits, k.goInto, "View files", a.enterCommitFiles, true},
		{scopeCommitFiles, k.ret, "Back to commits", a.back, true},
		{scopeCommitFiles, k.goInto, tr.FileEnter, func() {
			if !a.toggleCollapsed() {
				a.focusMainView()
			}
		}, false},
		{scopeCommitFiles, p(k.files.CollapseAll), tr.CollapseAll, a.collapseAll, false},
		{scopeCommitFiles, p(k.files.ExpandAll), tr.ExpandAll, a.expandAll, false},
		{scopeCommitFiles, p(k.files.ToggleTreeView), tr.ToggleTreeView, a.toggleTreeView, false},

		// Stash
		{scopeStash, k.selectKey, "Apply", a.stashApplyFlow, true},
		{scopeStash, p(k.stash.PopStash), "Pop", a.stashPopFlow, true},
		{scopeStash, k.remove, "Drop", a.stashDropFlow, true},

		// Staging view
		{scopeStaging, p(k.files.CommitChanges), "Commit", func() {
			if a.staging.worktree {
				a.commitFlow(false)
			}
		}, false},
		{scopeStaging, p(a.repo.Config.Keybinding.Universal.ScrollLeft), "Scroll left", func() { a.scrollFocusedDiff(-1) }, false},
		{scopeStaging, p(a.repo.Config.Keybinding.Universal.ScrollRight), "Scroll right", func() { a.scrollFocusedDiff(1) }, false},
		{scopeStaging, p(a.repo.Config.Keybinding.Universal.CopyToClipboard), tr.CopySelectedDiffLinesToClipboard, a.copyDiffSelection, false},
		{scopeStaging, k.selectKey, "Stage / unstage selection", a.stageSelection, true},
		{scopeStaging, k.toggleRange, "Toggle range select", func() { a.staging.toggleRange() }, true},
		{scopeStaging, k.rangeDown, "Range select down", func() { a.staging.extend(1) }, false},
		{scopeStaging, k.rangeUp, "Range select up", func() { a.staging.extend(-1) }, false},
		{scopeStaging, p(k.main.PrevHunk), tr.PrevHunk, func() { a.staging.moveBlock(-1) }, false},
		{scopeStaging, p(k.main.NextHunk), tr.NextHunk, func() { a.staging.moveBlock(1) }, false},
		{scopeStaging, p(k.main.PrevFile), tr.PrevFileInDiff, func() { a.staging.navigateFile(false) }, false},
		{scopeStaging, p(k.main.NextFile), tr.NextFileInDiff, func() { a.staging.navigateFile(true) }, false},
		{scopeStaging, p(k.main.ToggleSelectHunk), "Toggle hunk select", func() { a.staging.toggleHunk() }, true},
		{scopeStaging, k.togglePanel, "Switch staged / unstaged", a.switchStagingSide, true},
		{scopeStaging, k.remove, "Discard selection", a.discardSelection, false},
		{scopeStaging, k.ret, "Back to files", a.back, true},

		// Main view
		{scopeMain, k.ret, "Back", a.back, true},
	}
	for i, j := range k.jump {
		if i < int(panelCount) {
			pn := panel(i)
			acts = append(acts, action{scopeGlobal, j, "Focus " + panelTitles[pn], func() { a.jumpPanel(pn) }, false})
		}
	}
	acts = append(acts, a.searchActions()...)
	acts = append(acts, a.shellCommandAction())
	acts = append(acts, a.diffContextActions()...)
	acts = append(acts, action{scopeGlobal, p(a.repo.Config.Keybinding.Universal.CreatePatchOptionsMenu), tr.PatchOptionsTitle, a.patchOptions, false},
		action{scopeCommitFiles, k.selectKey, tr.ToggleAddToPatch, func() { a.togglePatchFiles(false) }, true},
		action{scopeCommitFiles, p(k.files.ToggleStagedAll), tr.ToggleAllInPatch, func() { a.togglePatchFiles(true) }, false})
	return append(a.customActions(), a.guardSingleItemActions(acts)...)
}

// dispatch runs the action bound to ch: overlays first, then the current
// scope, then global actions — the precedence lazygit's contexts have.
func (a *App) dispatch(ch keymap.Chord) bool {
	if ov := a.topOverlay(); ov != nil {
		ov.key(a, ch)
		return true // modal: nothing behind it reacts
	}
	if a.switching || a.busy != "" {
		return true
	}
	sc := a.scope()
	if a.searchKey(ch) {
		return true
	}
	for _, pass := range []scope{sc, scopeGlobal} {
		for _, act := range a.actions {
			if act.scope == pass && act.keys.Matches(ch) {
				act.run()
				return true
			}
		}
	}
	return false
}

// visibleActions lists the actions reachable now, scope first.
func (a *App) visibleActions() []action {
	sc := a.scope()
	var out []action
	for _, pass := range []scope{sc, scopeGlobal} {
		for _, act := range a.actions {
			if act.scope == pass && len(act.keys) > 0 {
				act.label = a.actionLabel(act)
				out = append(out, act)
			}
		}
	}
	return out
}

// openKeybindings is lazygit's `?` menu: every action available here, with
// its key, runnable from the list.
func (a *App) openKeybindings() {
	var items []menuItem
	for _, act := range a.visibleActions() {
		act := act
		section := scopeNames[act.scope]
		items = append(items, menuItem{label: act.label, key: act.keys.LabelAll(), section: section, run: act.run})
	}
	a.pushOverlay(&menuOverlay{title: "Keybindings", items: items, filterable: true, noItemKeys: true})
}

func (a *App) openExtras() {
	label := "Show command log"
	if a.showLog {
		label = "Hide command log"
	}
	a.pushOverlay(&menuOverlay{title: "Command log", items: []menuItem{
		{label: "Continue / abort git operation", run: a.sequenceMenu},
		{label: label, run: func() { a.showLog = !a.showLog }},
	}})
}

// Press dispatches lazygit key labels (e.g. "4", "<enter>", "<c-r>") as if
// typed: a dev hook for scripted captures.
func (a *App) Press(labels ...string) {
	for _, l := range labels {
		if ch, ok := keymap.ParseLabel(l); ok {
			a.dispatch(ch)
		}
	}
}

func (a *App) actionLabel(act action) string {
	if act.scope == scopeStaging && a.staging != nil && !a.staging.worktree {
		tr := a.repo.Common.Tr
		switch act.label {
		case "Stage / unstage selection":
			return tr.ToggleSelectionForPatch
		case "Discard selection":
			return tr.RemoveSelectionFromPatch
		case "Switch staged / unstaged":
			return tr.CustomPatch
		case "Back to files":
			return "Back"
		}
	}
	return act.label
}

func (v *conflictOverlay) actions(a *App) []action {
	tr := a.repo.Common.Tr
	u := a.repo.Config.Keybinding.Universal
	return []action{
		{keys: keymap.Parse(a.keys.files.OpenMergeOptions), label: tr.ViewMergeConflictOptions, run: func() { a.conflictFileMenu(v.file) }},
		{keys: a.keys.prev, label: tr.SelectPrevHunk, run: v.state.SelectPrevConflictHunk},
		{keys: a.keys.next, label: tr.SelectNextHunk, run: v.state.SelectNextConflictHunk},
		{keys: keymap.Parse(a.keys.main.PrevHunk), label: tr.PrevConflict, run: v.state.SelectPrevConflict},
		{keys: keymap.Parse(a.keys.main.NextHunk), label: tr.NextConflict, run: v.state.SelectNextConflict},
		{keys: a.keys.selectKey, label: tr.PickHunk, run: func() { v.resolve(a, v.state.Selection()) }},
		{keys: keymap.Parse(a.keys.main.PickBothHunks), label: tr.PickBothHunks, run: func() { v.resolve(a, mergeconflicts.BOTH) }},
		{keys: keymap.Parse(u.Undo), label: tr.Undo, run: func() { v.undo(a) }},
	}
}

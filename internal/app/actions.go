package app

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gui/mergeconflicts"
	"slices"

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
	confirmInEditor, submitEditor, commitMenu            keymap.Binding
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
		submitEditor:    p(u.SubmitEditorText), commitMenu: p(kb.CommitMessage.CommitMenu),
		files: kb.Files, branches: kb.Branches, stash: kb.Stash, commits: kb.Commits, main: kb.Main,
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
	scopeGuide
	scopeGuideDiagram
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
	scopeReflog
)

var scopeNames = map[scope]string{
	scopeGuide:        "Review guide",
	scopeGuideDiagram: "Review diagram",
	scopeReflog:       "Reflog", scopeRefCommits: "Commits", scopeRemoteBranches: "Remote branches", scopeWorktrees: "Worktrees", scopeSubmodules: "Submodules", scopeRemotes: "Remotes", scopeTags: "Tags", scopeGlobal: "Global", scopeStatus: "Status", scopeFiles: "Files", scopeBranches: "Branches",
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
	if a.guide != nil && a.guide.active {
		if a.guide.showDiagram && !a.guide.diagramCodeFocus {
			return scopeGuideDiagram
		}
		return scopeGuide
	}
	switch {
	case a.staging != nil:
		return scopeStaging
	case a.focus == panelBranches && !a.mainFocused && a.browse(panelBranches) != nil && a.browse(panelBranches).ref != nil:
		return scopeRefCommits
	case a.focus == panelBranches && !a.mainFocused && a.browse(panelBranches) != nil:
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
	case a.focus == panelCommits && a.commitsView() == cvSubCommits:
		return scopeRefCommits
	case a.focus == panelCommits && a.commitsView() == cvReflog:
		return scopeReflog
	}
	return [panelCount]scope{scopeStatus, scopeFiles, scopeBranches, scopeCommits, scopeStash}[a.focus]
}

func (a *App) buildActions() []action {
	a.tips = nil // the repo, and so its language, may have changed
	k := a.keys
	p := keymap.Parse
	tr := a.repo.Common.Tr
	acts := []action{
		{scopeGlobal, p(config.Keybinding{"~"}), "Toggle review guide", a.toggleGuide, false},
		{scopeGuide, p(config.Keybinding{"~"}), "Git", a.toggleGuide, true},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.Edit), "Expand / collapse note", func() { a.guide.expandedNote = !a.guide.expandedNote }, false},
		{scopeGuide, p(config.Keybinding{"c"}), "Comment", a.guideComment, true},
		{scopeGuide, p(config.Keybinding{"f"}), "Feedback", a.guideFeedbackMenu, false},
		{scopeGuide, p(config.Keybinding{"S"}), "Send review", a.guideSendFeedback, false},
		{scopeGuide, p(config.Keybinding{"C"}), "Commit", a.guideCommit, false},
		{scopeGuide, p(config.Keybinding{"G"}), "Publish to GitHub PR", a.guidePublishPR, false},
		{scopeGuide, p(config.Keybinding{"D"}), "Diagram / code", a.guideToggleDiagram, false},
		{scopeGuideDiagram, k.next, "Next shape", func() { a.guideDiagramMove(1) }, false},
		{scopeGuideDiagram, k.prev, "Previous shape", func() { a.guideDiagramMove(-1) }, false},
		{scopeGuideDiagram, k.nextBlock, "Next shape", func() { a.guideDiagramMove(1) }, false},
		{scopeGuideDiagram, k.prevBlock, "Previous shape", func() { a.guideDiagramMove(-1) }, false},
		{scopeGuideDiagram, k.goInto, "Jump to code", a.guideDiagramJump, true},
		{scopeGuideDiagram, p(a.repo.Config.Keybinding.Universal.Edit), "Edit label", a.guideDiagramLabel, true},
		{scopeGuideDiagram, p(config.Keybinding{"v"}), "Baseline / proposal", a.guideDiagramBaseline, false},
		{scopeGuideDiagram, p(config.Keybinding{"s"}), "Select tool", func() { a.guideDiagramTool("select") }, false},
		{scopeGuideDiagram, p(config.Keybinding{"a"}), "Arrow tool", func() { a.guideDiagramTool("arrow") }, false},
		{scopeGuideDiagram, p(config.Keybinding{"n"}), "Note tool", func() { a.guideDiagramTool("note") }, false},
		{scopeGuideDiagram, p(config.Keybinding{"p"}), "Draw tool", func() { a.guideDiagramTool("draw") }, false},
		{scopeGuideDiagram, p(config.Keybinding{"b"}), "Box tool", func() { a.guideDiagramTool("geo") }, false},
		{scopeGuideDiagram, p(config.Keybinding{"H"}), "Move shape left", func() { a.guideDiagramNudge(-20, 0) }, false},
		{scopeGuideDiagram, p(config.Keybinding{"L"}), "Move shape right", func() { a.guideDiagramNudge(20, 0) }, false},
		{scopeGuideDiagram, p(config.Keybinding{"K"}), "Move shape up", func() { a.guideDiagramNudge(0, -20) }, false},
		{scopeGuideDiagram, p(config.Keybinding{"J"}), "Move shape down", func() { a.guideDiagramNudge(0, 20) }, false},
		{scopeGuideDiagram, p(config.Keybinding{">"}), "Enlarge shape", func() { a.guideDiagramScale(1.15) }, false},
		{scopeGuideDiagram, p(config.Keybinding{"<"}), "Shrink shape", func() { a.guideDiagramScale(1 / 1.15) }, false},
		{scopeGuideDiagram, k.remove, "Delete selected shape", a.guideDiagramDelete, false},
		{scopeGuideDiagram, p(config.Keybinding{"u"}), "Undo diagram edit", a.guideDiagramUndo, false},
		{scopeGuideDiagram, p(config.Keybinding{"U"}), "Redo diagram edit", a.guideDiagramRedo, false},
		{scopeGuideDiagram, p(config.Keybinding{"="}), "Fit diagram", a.guideDiagramFit, false},
		{scopeGuideDiagram, p(config.Keybinding{"+"}), "Zoom in", func() {
			if a.guide.drawing != nil {
				a.guide.drawing.Editor().ZoomIn()
			}
		}, false},
		{scopeGuideDiagram, p(config.Keybinding{"-"}), "Zoom out", func() {
			if a.guide.drawing != nil {
				a.guide.drawing.Editor().ZoomOut()
			}
		}, false},
		{scopeGuide, k.next, "Next hunk", func() { a.guideMove(1) }, false},
		{scopeGuide, k.prev, "Previous hunk", func() { a.guideMove(-1) }, false},
		// Before nextBlock, whose alt2 is also Tab: staging gives Tab to togglePanel too.
		{scopeGuide, k.togglePanel, "Next panel", a.guideCycle, false},
		{scopeGuide, k.nextBlock, "Next section", func() { a.guideSection(1) }, false},
		{scopeGuide, k.prevBlock, "Previous section", func() { a.guideSection(-1) }, false},
		{scopeGuide, k.selectKey, "Reviewed", a.guideReview, true},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.NextTab), "Next unread", a.guideNext, true},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.PrevTab), "Previous unread", func() { a.guideUnread(-1) }, false},
		{scopeGuide, p(config.Keybinding{"!"}), "Guide status details", a.guideStatus, false},
		{scopeGuide, p(config.Keybinding{"M"}), "Copy MCP config", a.copyGuideMCP, false},
		{scopeGuide, p(config.Keybinding{"A"}), "Copy agent prompt", a.copyGuidePrompt, true},
		{scopeGuide, p(config.Keybinding{"b"}), "Base", a.guideBase, true},
		{scopeGuide, k.toggleRange, "Select range", a.guideToggleRange, false},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.StartSearch), "Search code", a.guideSearch, false},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.NextMatch), "Next match", func() { a.guideSearchNext(1) }, false},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.PrevMatch), "Previous match", func() { a.guideSearchNext(-1) }, false},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.OpenFile), "Edit file at line", a.guideEditAtLine, false},
		{scopeGuide, p(k.files.CopyFileInfoToClipboard), "Copy", a.guideCopyMenu, false},
		{scopeGuide, p(config.Keybinding{"r"}), "Reload", a.refreshGuide, true},
		{scopeGuide, k.goInto, "Read code", a.guideEnter, true},
		{scopeGuide, k.focusMain, "Read code", a.guideEnter, false},
		{scopeGuide, k.top, "First", func() { a.guideMove(-1 << 30) }, false},
		{scopeGuide, k.bottom, "Last", func() { a.guideMove(1 << 30) }, false},
		{scopeGuide, k.ret, "Back", a.guideBack, false},
		{scopeGuide, k.quit, "Quit", a.quitFlow, false},
		{scopeGuide, k.optionMenu, "Keys", a.openKeybindings, true},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.ScrollLeft), "Scroll left", func() { a.guide.horizontal.X = max(0, a.guide.horizontal.X-a.guide.viewportWidth/3) }, false},
		{scopeGuide, p(a.repo.Config.Keybinding.Universal.ScrollRight), "Scroll right", func() { a.guide.horizontal.X += a.guide.viewportWidth / 3 }, false},
		{scopeGuide, k.scrollDownMain, "Scroll diff down", func() { a.guideScroll(max(1, a.repo.Config.Gui.ScrollHeight)) }, false},
		{scopeGuide, k.scrollUpMain, "Scroll diff up", func() { a.guideScroll(-max(1, a.repo.Config.Gui.ScrollHeight)) }, false},

		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.FilteringMenu), tr.OpenFilteringMenu, a.whenLoaded(globalParts, a.filteringMenu), false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.DiffingMenu), tr.ViewDiffingOptions, a.whenLoaded(globalParts, a.diffingMenu), false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.Undo), tr.UndoReflog, a.whenLoaded(globalParts, a.undoFlow), false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.Redo), tr.RedoReflog, a.whenLoaded(globalParts, a.redoFlow), false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.CreateRebaseOptionsMenu), tr.RebaseOptionsTitle, a.whenLoaded(globalParts, a.operationMenu), false},
		// Navigation (global; move() routes to whatever has focus).
		{scopeGlobal, k.next, "Next item", func() { a.move(1) }, false},
		{scopeGlobal, k.prev, "Previous item", func() { a.move(-1) }, false},
		{scopeGlobal, k.nextPage, "Next page", func() { a.page(1) }, false},
		{scopeGlobal, k.prevPage, "Previous page", func() { a.page(-1) }, false},
		{scopeGlobal, k.toggleRange, tr.ToggleRangeSelect, a.toggleRange, false},
		{scopeGlobal, k.rangeDown, tr.RangeSelectDown, func() { a.expandRange(1) }, false},
		{scopeGlobal, k.rangeUp, tr.RangeSelectUp, func() { a.expandRange(-1) }, false},
		{scopeGlobal, k.top, "Scroll to top", func() { a.move(-1 << 30) }, false},
		{scopeGlobal, k.bottom, "Scroll to bottom", func() { a.move(1 << 30) }, false},
		{scopeGlobal, k.nextBlock, "Next panel", func() { a.cycleWindow(1) }, false},
		{scopeGlobal, k.prevBlock, "Previous panel", func() { a.cycleWindow(-1) }, false},
		{scopeGlobal, k.focusMain, tr.FocusMainView, a.focusMainView, false},
		{scopeGlobal, k.scrollDownMain, "Scroll down main view", func() { a.scrollMain(1) }, false},
		{scopeGlobal, k.scrollUpMain, "Scroll up main view", func() { a.scrollMain(-1) }, false},
		{scopeGlobal, k.ret, "Return", a.back, true},
		{scopeGlobal, k.refresh, "Refresh", func() { a.Refresh(); a.RefreshPullRequests() }, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.NextScreenMode), tr.NextScreenMode, a.nextScreenMode, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.PrevScreenMode), tr.PrevScreenMode, a.prevScreenMode, false},
		{scopeGlobal, k.push, "Push", a.whenLoaded(globalParts, a.pushFlow), false},
		{scopeGlobal, k.pull, "Pull", a.whenLoaded(globalParts, a.pullFlow), false},
		{scopeGlobal, k.optionMenu, "Keybindings", a.openKeybindings, true},
		{scopeGlobal, k.extras, "Command log options", a.openExtras, false},
		{scopeGlobal, k.quit, "Quit", a.quitFlow, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.OpenRecentRepos), tr.SwitchRepo, a.openRecentRepos, false},

		// Files
		{scopeFiles, k.selectKey, tr.Stage, a.toggleStaged, true},
		{scopeFiles, p(k.files.CopyFileInfoToClipboard), tr.CopyToClipboardMenu, a.openFilesCopyMenu, false},
		{scopeFiles, p(k.files.ToggleStagedAll), "Stage / unstage all", a.toggleStagedAll, false},
		{scopeFiles, p(a.repo.Config.Keybinding.Universal.Edit), tr.Edit, a.editSelectedFiles, false},
		{scopeFiles, p(a.repo.Config.Keybinding.Universal.OpenFile), tr.OpenFile, a.openSelectedFile, false},
		{scopeFiles, p(k.files.OpenMergeOptions), tr.ViewMergeConflictOptions, a.openMergeConflictMenu, false},
		{scopeFiles, p(k.files.FindBaseCommitForFixup), tr.FindBaseCommitForFixup, a.findBaseCommitForFixup, false},
		{scopeFiles, p(k.files.CommitChanges), "Commit", func() { a.commitFlow(false) }, true},
		{scopeFiles, p(k.files.CommitChangesWithoutHook), "Commit without hooks", func() { a.commitFlow(true) }, false},
		{scopeFiles, p(k.files.AmendLastCommit), "Amend last commit", a.amendFlow, false},
		{scopeFiles, k.goInto, tr.FileEnter, a.enterStaging, false},
		{scopeFiles, p(k.files.CollapseAll), tr.CollapseAll, a.collapseAll, false},
		{scopeFiles, p(k.files.ExpandAll), tr.ExpandAll, a.expandAll, false},
		{scopeFiles, p(k.files.OpenStatusFilter), tr.FileFilter, a.statusFilterMenu, false},
		{scopeFiles, p(k.files.ToggleTreeView), tr.ToggleTreeView, a.toggleTreeView, false},
		{scopeFiles, p(k.files.StashAllChanges), tr.Stash, a.stashFlow, true},
		{scopeFiles, k.remove, tr.Discard, a.discardMenu, true},
		{scopeFiles, p(k.commits.ViewResetOptions), tr.ViewResetToUpstreamOptions, a.resetToUpstreamMenu, false},
		{scopeFiles, p(k.files.ViewResetOptions), tr.Reset, a.workspaceResetMenu, true},
		{scopeBranches, p(k.branches.SetUpstream), tr.ViewBranchUpstreamOptions, a.upstreamOptionsMenu, true},
		{scopeBranches, p(k.branches.CheckoutPreviousBranch), tr.CheckoutPreviousBranch, a.checkoutPreviousBranch, false},
		{scopeBranches, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, true},
		{scopeTags, p(k.commits.ViewResetOptions), tr.Reset, a.resetSelected, true},
		{scopeRemoteBranches, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, false},
		{scopeCommits, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, true},
		{scopeCommits, p(k.commits.ViewBisectOptions), tr.ViewBisectOptions, a.bisectMenu, false},
		{scopeCommits, p(k.commits.MarkCommitAsBaseForRebase), tr.MarkAsBaseCommit, a.markAsBaseCommit, false},
		{scopeFiles, p(k.files.IgnoreFile), tr.Actions.IgnoreExcludeFile, a.ignoreOrExcludeMenu, false},
		{scopeFiles, p(k.files.ViewStashOptions), tr.ViewStashOptions, a.stashOptionsMenu, false},
		{scopeFiles, p(k.files.Fetch), "Fetch", a.fetchFlow, false},
		{scopeFiles, p(k.files.RefreshFiles), "Refresh files", a.Refresh, false},

		{scopeBranches, p(a.repo.Config.Keybinding.Universal.NewWorktree), tr.NewWorktree, a.newWorktreeMenuForBranch, false},
		{scopeCommits, p(a.repo.Config.Keybinding.Universal.NewWorktree), tr.NewWorktree, a.newWorktreeMenuForCommit, false},
		{scopeTags, p(a.repo.Config.Keybinding.Universal.NewWorktree), tr.NewWorktree, a.newWorktreeMenuForTag, false},
		{scopeStash, p(a.repo.Config.Keybinding.Universal.NewWorktree), tr.NewWorktree, a.newWorktreeMenuForStash, false},
		{scopeRemoteBranches, p(a.repo.Config.Keybinding.Universal.NewWorktree), tr.NewWorktree, a.newWorktreeMenuForRemoteBranch, false},
		{scopeWorktrees, k.newKey, tr.NewWorktree, a.newWorktreeFlow, true},
		{scopeWorktrees, k.goInto, "Open worktree", a.openWorktree, false},
		{scopeWorktrees, k.selectKey, tr.Switch, a.openWorktree, true},
		{scopeWorktrees, k.remove, tr.Remove, a.deleteWorktreeFlow, true},
		{scopeSubmodules, p(a.repo.Config.Keybinding.Submodules.BulkMenu), "Bulk actions", a.submoduleBulkMenu, false},
		{scopeSubmodules, k.newKey, tr.NewSubmodule, a.addSubmoduleFlow, true},
		{scopeSubmodules, k.goInto, tr.Enter, a.openSubmodule, true},
		{scopeSubmodules, p(a.repo.Config.Keybinding.Submodules.Init), "Initialize", a.initSubmodule, false},
		{scopeSubmodules, p(a.repo.Config.Keybinding.Submodules.Update), "Update", a.updateSubmodule, true},
		{scopeSubmodules, p(a.repo.Config.Keybinding.Universal.Edit), "Edit URL", a.submoduleURLFlow, false},
		{scopeSubmodules, k.remove, tr.Remove, a.deleteSubmoduleFlow, true},
		{scopeStatus, k.ret, "Return to parent repository", a.returnToParent, false},
		{scopeStatus, p(a.repo.Config.Keybinding.Status.AllBranchesLogGraph), tr.AllBranchesLogGraph, func() { a.rotateAllBranchesLog(true) }, false},
		{scopeStatus, p(a.repo.Config.Keybinding.Status.AllBranchesLogGraphReverse), tr.AllBranchesLogGraphReverse, func() { a.rotateAllBranchesLog(false) }, false},
		// Branches
		{scopeBranches, k.goInto, tr.ViewCommits, a.enterRef, false},
		{scopeTags, k.goInto, tr.ViewCommits, a.enterRef, false},
		{scopeRemotes, k.goInto, tr.ViewBranches, a.enterRef, true},
		{scopeRemoteBranches, k.goInto, tr.ViewCommits, a.enterRef, false},
		{scopeRemoteBranches, k.remove, tr.Delete, a.deleteBrowsedRemoteBranch, true},
		{scopeRemoteBranches, k.newKey, tr.NewBranch, a.newBranchFromBrowsedRemote, false},
		{scopeRemoteBranches, p(k.branches.SortOrder), tr.SortOrder, a.remoteBranchSortMenu, false},
		{scopeGlobal, p(k.commits.ResetCherryPick), tr.ResetCherryPick, a.resetCherryPick, false},
		{scopeRefCommits, k.goInto, "View files", a.enterCommitFiles, false},
		{scopeRefCommits, k.newKey, tr.NewBranch, a.newBranchFromCommit, false},
		{scopeRefCommits, p(k.commits.SelectCommitsOfCurrentBranch), tr.SelectCommitsOfCurrentBranch, a.selectCommitsOfCurrentBranch, false},
		{scopeRefCommits, p(k.commits.CherryPickCopy), tr.CherryPickCopy, a.copyCommit, true},
		{scopeReflog, k.goInto, tr.ViewCommits, a.enterRef, false},
		{scopeReflog, k.newKey, tr.NewBranch, a.newBranchFromCommit, false},
		{scopeReflog, p(k.commits.CheckoutCommit), tr.Checkout, a.checkoutCommitMenu, true},
		{scopeReflog, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, true},
		{scopeReflog, p(k.commits.CherryPickCopy), tr.CherryPickCopy, a.copyCommit, true},
		{scopeRefCommits, p(k.commits.CheckoutCommit), tr.Checkout, a.checkoutCommitMenu, true},
		{scopeRefCommits, p(k.commits.ViewResetOptions), tr.ViewResetOptions, a.resetSelected, true},
		{scopeBranches, p(a.repo.Config.Keybinding.Universal.CopyToClipboard), tr.CopyToClipboardMenu, a.copySelectedItem, false},
		{scopeRemotes, p(a.repo.Config.Keybinding.Universal.CopyToClipboard), tr.CopyToClipboardMenu, a.copySelectedItem, false},
		{scopeTags, p(a.repo.Config.Keybinding.Universal.CopyToClipboard), tr.CopyToClipboardMenu, a.copySelectedItem, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.NextTab), "Next tab", func() { a.switchTab(1) }, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.PrevTab), "Previous tab", func() { a.switchTab(-1) }, false},
		{scopeRemotes, k.newKey, tr.NewRemote, a.addRemoteFlow, true},
		{scopeRemotes, k.remove, tr.Remove, a.removeRemoteFlow, true},
		{scopeRemotes, p(a.repo.Config.Keybinding.Universal.Edit), tr.Edit, a.editRemoteFlow, true},
		{scopeRemotes, p(k.branches.FetchRemote), tr.Fetch, a.fetchRemoteFlow, true},
		{scopeRemotes, p(k.branches.AddForkRemote), tr.AddForkRemote, a.addForkRemoteFlow, true},
		{scopeTags, k.selectKey, tr.Checkout, a.checkoutTag, true},
		{scopeTags, k.newKey, tr.NewTag, a.newTagFlow, true},
		{scopeTags, k.remove, tr.Delete, a.deleteTagsMenu, true},
		{scopeTags, p(k.branches.PushTag), a.repo.Common.Tr.PushTag, a.pushTagFlow, true},
		{scopeBranches, k.selectKey, "Checkout", a.checkoutSelected, true},
		{scopeBranches, k.newKey, "New branch", a.newBranchFlow, true},
		{scopeBranches, p(k.branches.CreateTag), tr.NewTag, func() {
			if b := a.selectedBranch(); b != nil && !a.selectingRange(panelBranches) {
				a.openCreateTagPrompt(b.FullRefName(), func() {})
			}
		}, false},
		{scopeBranches, p(k.branches.MoveCommitsToNewBranch), tr.MoveCommitsToNewBranch, a.moveCommitsToNewBranch, false},
		{scopeBranches, p(k.branches.CheckoutBranchByName), "Checkout by name", a.checkoutByNameFlow, false},
		{scopeBranches, k.remove, tr.Delete, a.deleteBranchFlow, true},
		{scopeBranches, p(k.branches.RebaseBranch), "Rebase", a.rebaseFlow, true},
		{scopeBranches, p(k.branches.MergeIntoCurrentBranch), "Merge", a.mergeFlow, true},
		{scopeBranches, p(k.branches.FastForward), "Fast-forward", a.fastForwardFlow, false},
		{scopeBranches, p(k.branches.SortOrder), tr.SortOrder, a.localBranchSortMenu, false},
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
		{scopeCommits, p(k.commits.AmendToCommit), tr.Amend, a.amendSelectedCommit, true},
		{scopeCommits, p(k.commits.CreateTag), tr.Actions.CreateLightweightTag, a.createCommitTag, false},
		{scopeCommits, p(k.commits.RevertCommit), tr.Actions.RevertCommit, a.revertCommit, false},
		{scopeCommits, p(k.commits.MoveUpCommit), "Move commit up", func() { a.rebaseAction("up") }, false},
		{scopeCommits, p(k.commits.MoveDownCommit), "Move commit down", func() { a.rebaseAction("down") }, false},
		{scopeCommits, p(k.commits.CherryPickCopy), tr.CherryPickCopy, a.copyCommit, true},
		{scopeCommits, p(k.commits.PasteCommits), "Cherry-pick copied commits", a.pasteCommits, false},
		{scopeCommits, p(k.commits.SquashDown), tr.Squash, func() { a.squashOrFixup("squash") }, true},
		{scopeCommits, p(k.commits.MarkCommitAsFixup), tr.Fixup, func() { a.squashOrFixup("fixup") }, true},
		{scopeCommits, p(k.commits.PickCommit), "Pick (rebase todo)", func() { a.rebaseAction("pick") }, false},
		{scopeCommits, k.remove, tr.DropCommit, func() { a.rebaseAction("drop") }, true},
		{scopeCommits, p(k.commits.CheckoutCommit), tr.Checkout, a.checkoutCommitMenu, false},
		{scopeCommits, p(k.commits.RenameCommit), tr.Reword, a.rewordFlow, true},
		{scopeCommits, p(k.commits.RenameCommitWithEditor), tr.RewordCommitEditor, a.rewordInEditorFlow, false},
		{scopeCommits, p(a.repo.Config.Keybinding.Universal.Edit), tr.EditCommit, func() { a.rebaseAction("edit") }, true},
		{scopeCommits, p(k.commits.StartInteractiveRebase), tr.QuickStartInteractiveRebase, a.quickStartRebase, false},
		{scopeCommits, p(a.repo.Config.Keybinding.Universal.JumpToFile), tr.JumpToFileInDiff, func() { a.jumpToFileMenu(false) }, false},
		{scopeCommits, k.goInto, "View files", a.enterCommitFiles, false},
		{scopeCommits, k.newKey, tr.NewBranch, a.newBranchFromCommit, false},
		{scopeCommits, p(k.commits.SelectCommitsOfCurrentBranch), tr.SelectCommitsOfCurrentBranch, a.selectCommitsOfCurrentBranch, false},
		{scopeCommitFiles, k.ret, "Back to commits", a.back, false},
		{scopeCommitFiles, k.goInto, tr.FileEnter, func() {
			if !a.toggleCollapsed() {
				a.focusMainView()
			}
		}, false},
		{scopeCommitFiles, p(a.repo.Config.Keybinding.CommitFiles.CheckoutCommitFile), tr.Checkout, a.checkoutCommitFile, true},
		{scopeCommitFiles, k.remove, tr.Discard, a.discardCommitFiles, true},
		{scopeCommitFiles, p(k.files.CopyFileInfoToClipboard), tr.CopyToClipboardMenu, a.openCommitFilesCopyMenu, false},
		{scopeCommitFiles, p(k.files.CollapseAll), tr.CollapseAll, a.collapseAll, false},
		{scopeCommitFiles, p(k.files.ExpandAll), tr.ExpandAll, a.expandAll, false},
		{scopeCommitFiles, p(k.files.ToggleTreeView), tr.ToggleTreeView, a.toggleTreeView, false},

		// Stash
		{scopeStash, k.selectKey, "Apply", a.stashApplyFlow, true},
		{scopeStash, p(a.repo.Config.Keybinding.Universal.JumpToFile), tr.JumpToFileInDiff, func() { a.jumpToFileMenu(false) }, false},
		{scopeStash, p(k.stash.PopStash), "Pop", a.stashPopFlow, true},
		{scopeStash, k.remove, "Drop", a.stashDropFlow, true},
		{scopeStash, k.goInto, tr.ViewItemFiles, a.enterStashFiles, false},
		{scopeStash, k.newKey, tr.NewBranch, a.stashNewBranchFlow, false},
		{scopeStash, p(k.stash.RenameStash), tr.RenameStash, a.stashRenameFlow, false},

		// Staging view
		{scopeStaging, p(k.files.CommitChanges), "Commit", func() {
			if a.staging.worktree {
				a.commitFlow(false)
			}
		}, false},
		{scopeStaging, p(k.files.CommitChangesWithoutHook), tr.CommitChangesWithoutHook, func() {
			if a.staging.worktree {
				a.commitFlow(true)
			}
		}, false},
		{scopeStaging, p(a.repo.Config.Keybinding.Universal.JumpToFile), tr.JumpToFile, func() { a.jumpToFileMenu(true) }, false},
		{scopeStaging, p(a.repo.Config.Keybinding.Universal.ScrollLeft), "Scroll left", func() { a.scrollFocusedDiff(-1) }, false},
		{scopeStaging, p(a.repo.Config.Keybinding.Universal.ScrollRight), "Scroll right", func() { a.scrollFocusedDiff(1) }, false},
		{scopeStaging, p(a.repo.Config.Keybinding.Universal.CopyToClipboard), tr.CopySelectedDiffLinesToClipboard, a.needSelection(a.copyDiffSelection), false},
		{scopeStaging, k.selectKey, "Stage / unstage selection", a.needSelection(a.stageSelection), true},
		{scopeStaging, p(a.repo.Config.Keybinding.Universal.Edit), tr.EditFile, a.needSelection(a.editDiffLine), false},
		{scopeStaging, k.toggleRange, "Toggle range select", a.needSelection(func() { a.staging.toggleRange() }), true},
		{scopeStaging, k.rangeDown, "Range select down", a.needSelection(func() { a.staging.extend(1) }), false},
		{scopeStaging, k.rangeUp, "Range select up", a.needSelection(func() { a.staging.extend(-1) }), false},
		{scopeStaging, p(k.main.PrevHunk), tr.PrevHunk, a.needSelection(func() { a.staging.moveBlock(-1) }), false},
		{scopeStaging, p(k.main.NextHunk), tr.NextHunk, a.needSelection(func() { a.staging.moveBlock(1) }), false},
		{scopeStaging, p(k.main.PrevFile), tr.PrevFileInDiff, a.needSelection(func() { a.staging.navigateFile(false) }), false},
		{scopeStaging, p(k.main.NextFile), tr.NextFileInDiff, a.needSelection(func() { a.staging.navigateFile(true) }), false},
		{scopeStaging, p(k.main.ToggleSelectHunk), "Toggle hunk select", a.needSelection(func() { a.staging.toggleHunk() }), true},
		{scopeStaging, k.togglePanel, "Switch staged / unstaged", a.switchStagingSide, true},
		{scopeStaging, k.remove, "Discard selection", a.needSelection(a.discardSelection), false},
		{scopeStaging, p(k.main.EditSelectHunk), tr.EditHunk, a.needSelection(a.editHunk), false},
		{scopeStaging, k.ret, "Back to files", a.back, true},
		{scopeStaging, p(k.commits.OpenPullRequestInBrowser), tr.OpenPullRequestAtSelectedLine, a.openPullRequestAtLine, false},

		// Main view
		{scopeMain, k.ret, "Back", a.back, true},
	}
	for i, j := range k.jump {
		if i < 3 {
			n := i
			acts = append(acts, action{scopeGuide, j, "Focus " + [3]string{"story", "code", "diagram"}[n], func() { a.guideFocus(n) }, false})
		}
	}
	// lazygit's JumpToSideWindowController: jump keys go to the configured
	// panels by position.
	for i, views := range sidePanelConfig(a.repo.Config) {
		if i < len(k.jump) {
			acts = append(acts, action{scopeGlobal, k.jump[i], "Focus " + views[0].title(), func() { a.jumpPanel(i) }, false})
		}
	}
	acts = append(acts, a.searchActions()...)
	acts = append(acts, a.shellCommandAction())
	acts = append(acts, a.diffContextActions()...)
	acts = append(acts, action{scopeGlobal, p(a.repo.Config.Keybinding.Universal.CreatePatchOptionsMenu), tr.PatchOptionsTitle, a.patchOptions, false},
		action{scopeCommitFiles, k.selectKey, tr.ToggleAddToPatch, func() { a.togglePatchFiles(false) }, true},
		action{scopeCommitFiles, p(a.repo.Config.Keybinding.Universal.Edit), tr.Edit, a.editCommitFiles, false},
		action{scopeCommitFiles, p(a.repo.Config.Keybinding.Universal.OpenFile), tr.OpenFile, a.openCommitFile, false},
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
	if sc == scopeCommits && a.keys.ret.Matches(ch) && a.cancelCommitDrag() {
		return true
	}
	if sc != scopeGuide && sc != scopeGuideDiagram && a.searchKey(ch) {
		return true
	}
	for _, pass := range a.actionScopes(sc) {
		for _, act := range a.actions {
			if act.scope == pass && act.keys.Matches(ch) {
				// The first load of a repository: keys whose data hasn't
				// arrived do nothing, rather than act on an empty list.
				a.gatedRun(act)()
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
	for _, pass := range a.actionScopes(sc) {
		for _, act := range a.actions {
			if act.scope == pass && len(act.keys) > 0 {
				act.label = a.actionLabel(act)
				out = append(out, act)
			}
		}
	}
	return out
}

// menuActions is visibleActions with the unbound ones too: lazygit's
// keybindings menu lists a binding whose key is configured away.
func (a *App) menuActions() []action {
	sc := a.scope()
	var out []action
	for _, pass := range a.actionScopes(sc) {
		for _, act := range a.actions {
			if act.scope == pass {
				act.label = a.actionLabel(act)
				if act.label == "" {
					continue
				}
				out = append(out, act)
			}
		}
	}
	return out
}

// openKeybindings is lazygit's `?` menu: every action available here, with
// its key, runnable from the list.
//
// Its sections are lazygit's OptionsMenuAction's: the focused view's own
// bindings, the global ones, then the list navigation ones.
func (a *App) openKeybindings() {
	tr := a.repo.Common.Tr
	k := a.keys
	navigation := []keymap.Binding{k.prev, k.next, k.prevPage, k.nextPage, k.top, k.bottom}
	isNavigation := func(act action) bool {
		return act.scope == scopeGlobal && slices.ContainsFunc(navigation, func(b keymap.Binding) bool { return slices.Equal(b, act.keys) })
	}
	var local, global, nav []menuItem
	acts := a.menuActions()
	if v, ok := a.topOverlay().(*conflictOverlay); ok {
		// The merge conflicts view's own bindings, then the global ones.
		local := append([]action{{scope: scopeMain, keys: k.ret, label: tr.ReturnToFilesPanel, run: func() { a.closeOverlay(v) }}}, v.actions(a)...)
		for i := range local {
			local[i].scope = scopeMain
		}
		acts = append(local, slices.DeleteFunc(acts, func(act action) bool { return act.scope != scopeGlobal })...)
	}
	if a.mainFocused && a.staging == nil {
		// lazygit's MainViewController: the commands that act on a diff selection
		// describe themselves as nothing where the main view shows no diff (and
		// so aren't listed); over a diff with nothing to select they're listed,
		// disabled. The side list's range select belongs to the list.
		acts = slices.DeleteFunc(acts, func(act action) bool {
			return act.scope == scopeGlobal && (slices.Equal(act.keys, k.toggleRange) || slices.Equal(act.keys, k.rangeUp) || slices.Equal(act.keys, k.rangeDown))
		})
	}
	for _, item := range a.mainSelectionItems() {
		item.section = tr.KeybindingsMenuSectionLocal
		local = append(local, item)
	}
	for _, act := range acts {
		item := menuItem{label: act.label, key: act.keys.LabelAll(), run: a.gatedRun(act), tooltip: a.actionTooltip(act)}
		for _, ch := range act.keys {
			item.keyLabels = append(item.keyLabels, ch.Label())
		}
		switch {
		case act.scope != scopeGlobal:
			item.section = tr.KeybindingsMenuSectionLocal
			local = append(local, item)
		case isNavigation(act):
			item.section = tr.KeybindingsMenuSectionNavigation
			nav = append(nav, item)
		default:
			item.section = tr.KeybindingsMenuSectionGlobal
			global = append(global, item)
		}
	}
	items := append(append(local, global...), nav...)
	a.pushOverlay(&menuOverlay{title: tr.Keybindings, items: items, filterable: true, filterAsYouType: true, noItemKeys: true, allowKeyFilter: true})
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

// actionTooltip is lazygit's tooltip for an action: the keybindings menu
// shows it for the selected item, and hovering the action's button shows it.
func (a *App) actionTooltip(act action) string {
	tr := a.repo.Common.Tr
	if act.label == tr.ToggleWhitespaceInDiffView {
		return tr.ToggleWhitespaceInDiffViewTooltip
	}
	if act.scope == scopeStaging {
		if a.staging == nil {
			return ""
		}
		switch act.label {
		case "Stage / unstage selection":
			return tr.StageSelectionTooltip
		case "Discard selection":
			return tr.DiscardSelectionTooltip
		case tr.RemoveSelectionFromPatch:
			return tr.RemoveSelectionFromPatchTooltip
		case "Toggle hunk select":
			return tr.ToggleSelectHunkTooltip
		case tr.EditHunk:
			return tr.EditHunkTooltip
		}
	}
	return a.contextTip(act.scope, act.label)
}

// isDiffView is MainViewController.isDiffView for a focused main view: the
// side panel beneath shows a diff there.
func (a *App) isDiffView() bool {
	switch a.focus {
	case panelFiles:
		return a.fileTab == 0
	case panelCommits, panelStash:
		return true
	case panelBranches:
		b := a.browse(panelBranches)
		return b != nil && b.ref != nil
	}
	return false
}

// needSelection is lazygit's diffSelectionDisabledReason for a command
// acting on the focused diff's selection: with nothing selected (a hint,
// a binary file) it shows the reason instead.
func (a *App) needSelection(run func()) func() {
	return func() {
		if s := a.staging; s != nil && (s.cursor < 0 || s.cursor >= len(s.rows)) {
			tr := a.repo.Common.Tr
			a.setStatus(tr.DisabledMenuItemPrefix+tr.NothingToSelectInDiff, true)
			return
		}
		run()
	}
}

// mainSelectionItems are the selection commands of a focused main view over
// a diff with nothing to select: listed, disabled with lazygit's reason.
func (a *App) mainSelectionItems() []menuItem {
	if !a.mainFocused || a.staging != nil || !a.isDiffView() {
		return nil
	}
	tr := a.repo.Common.Tr
	k := a.keys
	reason := tr.NothingToSelectInDiff
	item := func(keys keymap.Binding, label, tooltip string) menuItem {
		it := menuItem{label: label, key: keys.LabelAll(), tooltip: tooltip, disabled: reason}
		for _, ch := range keys {
			it.keyLabels = append(it.keyLabels, ch.Label())
		}
		return it
	}
	p := keymap.Parse
	return []menuItem{
		item(p(k.main.ToggleSelectHunk), tr.SelectHunk, tr.ToggleSelectHunkTooltip),
		item(k.toggleRange, tr.ToggleRangeSelect, ""),
		item(p(a.repo.Config.Keybinding.Universal.Edit), tr.EditFile, tr.EditFileTooltip),
		item(p(k.main.PrevHunk), tr.PrevHunk, ""),
		item(p(k.main.NextHunk), tr.NextHunk, ""),
		item(p(k.main.PrevFile), tr.PrevFileInDiff, ""),
		item(p(k.main.NextFile), tr.NextFileInDiff, ""),
	}
}

func (a *App) actionLabel(act action) string {
	if act.scope == scopeGuide && a.guide != nil {
		if act.label == "Back" {
			if a.guide.focusCode {
				return "Back to story"
			}
			return "Back to git"
		}
		if a.guide.focusCode {
			switch act.label {
			case "Next hunk":
				return "Next line"
			case "Previous hunk":
				return "Previous line"
			case "Next section":
				return "Next hunk"
			case "Previous section":
				return "Previous hunk"
			}
		}
	}

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
	if act.scope == scopeStaging && act.label == a.repo.Common.Tr.OpenPullRequestAtSelectedLine {
		// pullRequestDescription: not offered over a diff of no branch.
		if branch, _ := a.pullRequestDiff(); branch == "" {
			return ""
		}
	}
	return act.label
}

func (v *conflictOverlay) actions(a *App) []action {
	tr := a.repo.Common.Tr
	u := a.repo.Config.Keybinding.Universal
	return []action{
		{keys: keymap.Parse(a.keys.files.OpenMergeOptions), label: tr.ViewMergeConflictOptions, run: func() { a.mergeConflictMenu([]string{v.file.Path}) }},
		{keys: a.keys.prev, label: tr.SelectPrevHunk, run: v.state.SelectPrevConflictHunk},
		{keys: a.keys.next, label: tr.SelectNextHunk, run: v.state.SelectNextConflictHunk},
		{keys: keymap.Parse(a.keys.main.PrevHunk), label: tr.PrevConflict, run: v.state.SelectPrevConflict},
		{keys: keymap.Parse(a.keys.main.NextHunk), label: tr.NextConflict, run: v.state.SelectNextConflict},
		{keys: a.keys.selectKey, label: tr.PickHunk, run: func() { v.resolve(a, v.state.Selection()) }},
		{keys: keymap.Parse(a.keys.main.PickBothHunks), label: tr.PickBothHunks, run: func() { v.resolve(a, mergeconflicts.BOTH) }},
		{keys: keymap.Parse(u.Undo), label: tr.Undo, run: func() { v.undo(a) }},
	}
}

func (a *App) actionScopes(sc scope) []scope {
	if sc == scopeGuideDiagram {
		return []scope{scopeGuideDiagram, scopeGuide}
	}
	if sc == scopeGuide {
		return []scope{scopeGuide}
	}
	return []scope{sc, scopeGlobal}
}

package app

import (
	"github.com/jesseduffield/lazygit/pkg/config"

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
)

var scopeNames = map[scope]string{
	scopeRemotes: "Remotes", scopeTags: "Tags", scopeGlobal: "Global", scopeStatus: "Status", scopeFiles: "Files", scopeBranches: "Branches",
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
	case a.focus == panelBranches && !a.mainFocused && a.branchTab == 1:
		return scopeRemotes
	case a.focus == panelBranches && !a.mainFocused && a.branchTab == 2:
		return scopeTags
	case a.staging != nil:
		return scopeStaging
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
	acts := []action{
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.Undo), "Undo last action", a.undoFlow, false},
		// Navigation (global; move() routes to whatever has focus).
		{scopeGlobal, k.next, "Next item", func() { a.move(1) }, false},
		{scopeGlobal, k.prev, "Previous item", func() { a.move(-1) }, false},
		{scopeGlobal, k.nextPage, "Next page", func() { a.move(a.pageSize()) }, false},
		{scopeGlobal, k.prevPage, "Previous page", func() { a.move(-a.pageSize()) }, false},
		{scopeGlobal, k.top, "Scroll to top", func() { a.move(-1 << 30) }, false},
		{scopeGlobal, k.bottom, "Scroll to bottom", func() { a.move(1 << 30) }, false},
		{scopeGlobal, k.nextBlock, "Next panel", func() { a.focusPanel((a.focus + 1) % panelCount) }, false},
		{scopeGlobal, k.prevBlock, "Previous panel", func() { a.focusPanel((a.focus + panelCount - 1) % panelCount) }, false},
		{scopeGlobal, k.focusMain, "Focus main view", func() { a.mainFocused = true }, false},
		{scopeGlobal, k.scrollDownMain, "Scroll down main view", func() { a.scrollMain(3) }, false},
		{scopeGlobal, k.scrollUpMain, "Scroll up main view", func() { a.scrollMain(-3) }, false},
		{scopeGlobal, k.ret, "Return", a.back, false},
		{scopeGlobal, k.refresh, "Refresh", a.Refresh, false},
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
		{scopeFiles, k.goInto, "Stage lines", a.enterStaging, true},
		{scopeFiles, k.remove, "Discard", a.discardMenu, true},
		{scopeFiles, p(k.files.StashAllChanges), "Stash all changes", a.stashFlow, false},
		{scopeFiles, p(k.files.Fetch), "Fetch", a.fetchFlow, false},
		{scopeFiles, p(k.files.RefreshFiles), "Refresh files", a.Refresh, false},

		// Branches
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.NextTab), "Next tab", func() { a.switchBranchTab(1) }, false},
		{scopeGlobal, p(a.repo.Config.Keybinding.Universal.PrevTab), "Previous tab", func() { a.switchBranchTab(-1) }, false},
		{scopeRemotes, p(k.files.Fetch), "Fetch", a.fetchFlow, true},
		{scopeTags, k.selectKey, "Checkout tag", a.checkoutTag, true},
		{scopeBranches, k.selectKey, "Checkout", a.checkoutSelected, true},
		{scopeBranches, k.newKey, "New branch", a.newBranchFlow, true},
		{scopeBranches, p(k.branches.CheckoutBranchByName), "Checkout by name", a.checkoutByNameFlow, false},
		{scopeBranches, k.remove, "Delete branch", a.deleteBranchFlow, true},

		// Commits
		{scopeCommits, p(k.commits.MoveUpCommit), "Move commit up", func() { a.rebaseAction("up") }, false},
		{scopeCommits, p(k.commits.MoveDownCommit), "Move commit down", func() { a.rebaseAction("down") }, false},
		{scopeCommits, p(k.commits.CherryPickCopy), "Copy commit for cherry-pick", a.copyCommit, false},
		{scopeCommits, p(k.commits.PasteCommits), "Cherry-pick copied commits", a.pasteCommits, false},
		{scopeCommits, p(k.commits.SquashDown), "Squash into commit below", func() { a.rebaseAction("squash") }, true},
		{scopeCommits, p(k.commits.MarkCommitAsFixup), "Fixup into commit below", func() { a.rebaseAction("fixup") }, false},
		{scopeCommits, p(k.commits.RenameCommit), "Reword commit", a.rewordFlow, true},
		{scopeCommits, p(k.commits.StartInteractiveRebase), "Interactive rebase", a.rebaseMenu, true},
		{scopeCommits, k.goInto, "View files", a.enterCommitFiles, true},
		{scopeCommitFiles, k.ret, "Back to commits", a.back, true},

		// Stash
		{scopeStash, k.selectKey, "Apply", a.stashApplyFlow, true},
		{scopeStash, p(k.stash.PopStash), "Pop", a.stashPopFlow, true},
		{scopeStash, k.remove, "Drop", a.stashDropFlow, true},

		// Staging view
		{scopeStaging, k.selectKey, "Stage / unstage selection", a.stageSelection, true},
		{scopeStaging, k.toggleRange, "Toggle range select", func() { a.staging.toggleRange() }, true},
		{scopeStaging, k.rangeDown, "Range select down", func() { a.staging.extend(1) }, false},
		{scopeStaging, k.rangeUp, "Range select up", func() { a.staging.extend(-1) }, false},
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
			acts = append(acts, action{scopeGlobal, j, "Focus " + panelTitles[pn], func() { a.focusPanel(pn) }, false})
		}
	}
	return append(a.customActions(), acts...)
}

// dispatch runs the action bound to ch: overlays first, then the current
// scope, then global actions — the precedence lazygit's contexts have.
func (a *App) dispatch(ch keymap.Chord) bool {
	if ov := a.topOverlay(); ov != nil {
		ov.key(a, ch)
		return true // modal: nothing behind it reacts
	}
	sc := a.scope()
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

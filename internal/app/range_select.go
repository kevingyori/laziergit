package app

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gui/context/traits"
	"github.com/jesseduffield/lazygit/pkg/gui/types"
	"github.com/jesseduffield/lazygit/pkg/utils"

	"laziergit/internal/keymap"
)

// listRange is lazygit's ListCursor range state (traits.ListCursor) for one
// panel; the panel's cursor is the range's moving end. Each panel shows one
// list at a time (a tab or a drill-down), so the range belongs to the list it
// was started in and is dormant while another list fills the panel, as
// lazygit's per-context cursors are.
type listRange struct {
	list  scope
	mode  traits.RangeSelectMode
	start int
}

// listScope is the list panel p is showing, focused or not.
func (a *App) listScope(p panel) scope {
	switch p {
	case panelFiles:
		return [...]scope{scopeFiles, scopeWorktrees, scopeSubmodules}[a.fileTab]
	case panelBranches:
		if b := a.refBrowse; b != nil {
			if b.ref != nil {
				return scopeRefCommits
			}
			return scopeRemoteBranches
		}
		return [...]scope{scopeBranches, scopeRemotes, scopeTags}[a.branchTab]
	case panelCommits:
		if a.commitFiles != nil {
			return scopeCommitFiles
		}
		return scopeCommits
	case panelStash:
		return scopeStash
	}
	return scopeStatus
}

// focusedList is the list panel that has focus, or false when focus is on
// something without a list cursor (status, main, staging, an overlay).
func (a *App) focusedList() (panel, bool) {
	if a.staging != nil || a.mainFocused || a.focus == panelStatus {
		return 0, false
	}
	return a.focus, true
}

// selectingRange is ListCursor.IsSelectingRange for panel p.
func (a *App) selectingRange(p panel) bool {
	r := a.ranges[p]
	return r.mode != traits.RangeSelectModeNone && r.list == a.listScope(p)
}

// selectionRange is ListCursor.GetSelectionRange: the sorted indices of
// panel p's selected items.
func (a *App) selectionRange(p panel) (start, end int) {
	i := a.cursor[p]
	if a.selectingRange(p) {
		return utils.SortRange(i, clamp(a.ranges[p].start, 0, a.count(p)-1))
	}
	return i, i
}

// multipleSelected is ListCursor.AreMultipleItemsSelected.
func (a *App) multipleSelected(p panel) bool {
	start, end := a.selectionRange(p)
	return start != end
}

func (a *App) inSelection(p panel, i int) bool {
	start, end := a.selectionRange(p)
	return i >= start && i <= end
}

func (a *App) cancelRange(p panel) { a.ranges[p].mode = traits.RangeSelectModeNone }

// collapseRangeToTop is ListCursor.CollapseRangeSelectionToTop, used after
// removing the selected items.
func (a *App) collapseRangeToTop(p panel) {
	start, _ := a.selectionRange(p)
	a.cursor[p] = clamp(start, 0, a.count(p)-1)
	a.cancelRange(p)
}

// setRange is ListCursor.SetSelectionRangeAndMode.
func (a *App) setRange(p panel, selected, start int, mode traits.RangeSelectMode) {
	n := a.count(p)
	a.cursor[p] = clamp(selected, 0, n-1)
	if mode == traits.RangeSelectModeNonSticky && selected == start {
		mode = traits.RangeSelectModeNone
	}
	a.ranges[p] = listRange{list: a.listScope(p), mode: mode, start: clamp(start, 0, n-1)}
}

// toggleRange is lazygit's ToggleStickyRange (v).
func (a *App) toggleRange() {
	p, ok := a.focusedList()
	if !ok || a.count(p) == 0 {
		return
	}
	if a.selectingRange(p) {
		a.cancelRange(p)
		return
	}
	a.ranges[p] = listRange{list: a.listScope(p), mode: traits.RangeSelectModeSticky, start: a.cursor[p]}
	a.diffKey = ""
	a.loadMain()
}

// expandRange is lazygit's ExpandNonStickyRange (shift+up/down).
func (a *App) expandRange(d int) {
	p, ok := a.focusedList()
	if !ok || a.count(p) == 0 {
		return
	}
	if !a.selectingRange(p) {
		a.ranges[p].start = a.cursor[p]
	}
	a.ranges[p].list = a.listScope(p)
	a.ranges[p].mode = traits.RangeSelectModeNonSticky
	a.cursor[p] = clamp(a.cursor[p]+d, 0, a.count(p)-1)
	a.lists[p].ScrollIntoView(a.cursor[p])
	a.loadMain()
}

// dismissRange is the first thing <esc> does in a list: cancel its range.
func (a *App) dismissRange() bool {
	p, ok := a.focusedList()
	if !ok || !a.selectingRange(p) {
		return false
	}
	a.cancelRange(p)
	a.diffKey = ""
	a.loadMain()
	return true
}

// singleItem guards an action lazygit only allows on one item: on a range it
// shows lazygit's disabled reason instead, as its keybinding dispatcher does.
func (a *App) singleItem(run func()) func() {
	return func() {
		if p, ok := a.focusedList(); ok && a.multipleSelected(p) {
			tr := a.repo.Common.Tr
			a.setStatus(tr.DisabledMenuItemPrefix+tr.RangeSelectNotSupported, true)
			return
		}
		run()
	}
}

// withTreeCursor runs f with panel p's selection loaded into a lazygit tree
// view model's ListCursor, then takes the selection back, so the model's
// selection keeping (SetTree, collapsing) moves the range as lazygit's does.
func (a *App) withTreeCursor(p panel, lc types.IListCursor, f func()) {
	c, ok := lc.(*traits.ListCursor)
	if !ok {
		f()
		return
	}
	r := a.ranges[p]
	own := r.list == a.listScope(p)
	mode := r.mode
	if !own {
		mode = traits.RangeSelectModeNone
	}
	c.SetSelectionRangeAndMode(a.cursor[p], r.start, mode)
	f()
	selected, start, mode := c.GetSelectionRangeAndMode()
	a.cursor[p] = max(selected, 0)
	if own || mode != traits.RangeSelectModeNone {
		a.ranges[p] = listRange{list: a.listScope(p), mode: mode, start: max(start, 0)}
	}
}

// singleItemActions are the keys each lazygit list controller guards with
// singleItemSelected: on a range they show RangeSelectNotSupported.
func (a *App) singleItemActions() map[scope][]keymap.Binding {
	kb := a.repo.Config.Keybinding
	u, br, cm := kb.Universal, kb.Branches, kb.Commits
	p := func(keys ...config.Keybinding) []keymap.Binding {
		out := make([]keymap.Binding, len(keys))
		for i, k := range keys {
			out[i] = keymap.Parse(k)
		}
		return out
	}
	basicCommits := []config.Keybinding{cm.CheckoutCommit, cm.CopyCommitAttributeToClipboard, cm.OpenInBrowser, u.New, cm.ViewResetOptions, u.OpenDiffTool}
	return map[scope][]keymap.Binding{
		scopeFiles: p(u.OpenFile, kb.Files.IgnoreFile, u.GoInto, u.OpenDiffTool),
		scopeBranches: p(u.Select, u.New, br.CreatePullRequest, br.ViewPullRequestOptions, br.OpenPullRequestInBrowser,
			br.CopyPullRequestURL, br.ForceCheckoutBranch, br.RebaseBranch, br.MergeIntoCurrentBranch, br.CreateTag,
			cm.ViewResetOptions, br.RenameBranch, br.SetUpstream, u.Remove, br.FastForward, u.OpenDiffTool, u.GoInto),
		scopeTags:  p(u.Remove, u.New, u.Select, cm.ViewResetOptions, u.OpenDiffTool, u.GoInto),
		scopeStash: p(u.Remove, u.Select, kb.Stash.PopStash, u.New, kb.Stash.RenameStash),
		scopeCommits: p(append([]config.Keybinding{cm.SetFixupMessage, cm.RenameCommit, cm.RenameCommitWithEditor, cm.CreateFixupCommit,
			cm.MarkCommitAsBaseForRebase, cm.AmendToCommit, cm.CreateTag}, basicCommits...)...),
		scopeRefCommits:     p(append([]config.Keybinding{u.GoInto}, basicCommits...)...),
		scopeCommitFiles:    p(kb.CommitFiles.CheckoutCommitFile, u.OpenFile, u.OpenDiffTool, u.GoInto),
		scopeRemotes:        p(u.GoInto, u.Remove, u.Edit, br.FetchRemote),
		scopeRemoteBranches: p(u.Remove, u.Select, u.New, br.MergeIntoCurrentBranch, br.RebaseBranch, br.SetUpstream, cm.ViewResetOptions, u.OpenDiffTool, u.GoInto),
		scopeWorktrees:      p(u.Select, u.GoInto, u.OpenFile, u.Remove),
		scopeSubmodules:     p(u.GoInto, u.Select, u.Remove, kb.Submodules.Update, u.Edit, kb.Submodules.Init),
	}
}

// guardSingleItemActions wraps the table's single-item actions in singleItem.
func (a *App) guardSingleItemActions(acts []action) []action {
	guarded := a.singleItemActions()
	for i := range acts {
		for _, keys := range guarded[acts[i].scope] {
			if sameKeys(keys, acts[i].keys) {
				acts[i].run = a.singleItem(acts[i].run)
				break
			}
		}
	}
	return acts
}

func sameKeys(x, y keymap.Binding) bool {
	if len(x) == 0 || len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

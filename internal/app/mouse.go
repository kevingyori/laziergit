package app

import (
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation"
	"github.com/jesseduffield/lazygit/pkg/gui/types"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// Mouse in the main and secondary views, as lazygit's MainViewController and
// SwitchToFocusedMainViewController handle it. Each has a keyboard
// equivalent: focusing the main view (0/enter), moving the selection, and
// e to edit the selected line.

// unfocusedMainRow is row el of the main view while it isn't focused,
// showing line src of the loaded text: a click focuses the view with that
// line selected; Alt- or Shift-click opens the line in the editor and
// leaves the focus where it is.
func (a *App) unfocusedMainRow(el ui.Element, src int) {
	a.cells.row("main", src, el)
	if !el.Clicked() {
		return
	}
	if el.ClickModifiers()&(ui.Alt|ui.Shift) != 0 {
		if src >= 0 && src < len(a.diffRows) {
			a.editDiffRow(a.diffRows[src], false, a.focus == panelFiles && a.fileTab == 0)
		}
		return
	}
	a.clickMainPane("main", src)
}

// clickMainPane is a click on line of pane ("main" or "secondary") while
// a side panel has the focus: lazygit's focusMainViewPane with the clicked
// line, which EstablishSelection selects.
func (a *App) clickMainPane(pane string, line int) {
	if a.staging != nil || a.mainFocused {
		return
	}
	// The diff loads off the main thread; the state takes the line when
	// its content arrives (reloadStaging).
	a.clickedLine = line + 1
	defer func() {
		if a.staging == nil {
			a.clickedLine = 0
		}
	}()
	if pane == "secondary" && a.focus == panelFiles && a.fileTab == 0 {
		if n := a.selectedFileNode(); n != nil && n.GetHasStagedChanges() {
			a.staging = &stagingState{worktree: true, staged: true, cursor: -1, anchor: -1, reveal: -1,
				viewportHeight: a.secondaryViewportHeight, startOrigin: a.unfocusedOrigin("secondary")}
			a.reloadStaging()
			return
		}
	}
	a.focusMainView()
}

// clickDiffLine is lazygit's selectClickedDiffLine, a click on line of the
// focused diff: in hunk mode a click inside the selected block narrows it to
// that line and one on another change selects that change's block; any
// other click selects just the line.
func (s *stagingState) clickDiffLine(line int) {
	if s.cursor < 0 || len(s.rows) == 0 {
		return
	}
	line = clamp(line, 0, len(s.rows)-1)
	s.dragAnchor = line
	if s.mode == selectHunk {
		if lo, hi, ok := s.blockBounds(s.anchor); ok && line >= lo && line <= hi {
			s.resetMode()
			s.cursor = line
			return
		}
		if s.isChange(line) {
			s.selectBlockAt(line)
			return
		}
	}
	s.resetMode()
	s.cursor = line
}

// selectBlockAt selects the change block around line without scrolling
// (lazygit's SelectChangeBlock for a click): the cursor on its first line,
// the anchor on its last.
func (s *stagingState) selectBlockAt(line int) {
	lo, hi, ok := s.blockBounds(line)
	if !ok {
		s.mode, s.anchor, s.cursor = selectLine, -1, line
		return
	}
	s.mode = selectHunk
	s.cursor, s.anchor = lo, hi
}

// sideMouse is the pointer input of the list of side view view (window tab
// v), lazygit's ListController mouse bindings: a press selects the row
// under it and focuses the view, a double click on a row of the focused
// view does what enter does. The keyboard equivalents are the panel's jump
// key, the arrows and enter.
func (a *App) sideMouse(view string, v sideView) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputPointerDown || ev.Button != 0 {
			return false
		}
		pane, ok := a.cells.panes[view]
		if !ok {
			return false
		}
		_, item, col := a.cells.at(view, pane.X+ev.X, pane.Y+ev.Y)
		a.sideClick(view, v, item, col, ev.Clicks)
		return true
	}
}

// sideClick is lazygit's ListController.HandleClick on item of view (-1:
// no item, as below the last row), with col the column of the element
// clicked where it acts on its own (-1 elsewhere).
func (a *App) sideClick(view string, v sideView, item, col, clicks int) {
	g := v.group()
	alreadyFocused := a.FocusedSideView() == view && a.focus == g && a.staging == nil && !a.mainFocused
	switch {
	case a.curView(g) != v:
		a.focusView(v)
	case a.focus != g || a.staging != nil || a.mainFocused:
		a.focusPanel(g)
		a.syncWindows()
	}
	if g == panelStatus {
		a.statusClick(col)
		return
	}
	if item < 0 || item >= a.count(g) {
		return
	}
	a.cancelRange(g)
	a.cursor[g] = item
	a.loadMain()
	if clicks == 2 && alreadyFocused {
		a.sideDoubleClick(g)
		return
	}
	// lazygit's FilesController/CommitFilesController.GetOnClick: a click on
	// a directory's arrow (or the space after it) folds or unfolds it.
	if col >= 0 && (view == "files" || view == "commitFiles") {
		a.toggleCollapsed()
	}
}

// sideDoubleClick is lazygit's GetOnDoubleClick of the focused list: what
// enter does on the selected row.
func (a *App) sideDoubleClick(pn panel) {
	switch pn {
	case panelFiles:
		if a.fileTab == 1 {
			a.openWorktree()
		} else if a.fileTab == 2 {
			a.openSubmodule()
		} else {
			a.enterStaging()
		}
	case panelCommits:
		if a.commitFiles != nil {
			if !a.toggleCollapsed() {
				a.focusMainView()
			}
		} else if a.commitsView() == cvReflog {
			a.enterRef()
		} else {
			a.enterCommitFiles()
		}
	case panelBranches:
		a.enterRef()
	}
}

// statusClick is lazygit's StatusController.onClick past focusing the view:
// a click on the working tree state ("(rebasing)") opens the rebase
// options, one on the repo name the recent repositories (keys: m, and the
// status panel's recent repos binding).
func (a *App) statusClick(col int) {
	if a.snap == nil || a.snap.Head == nil {
		return
	}
	upstream, state, repoName := a.statusParts()
	in := func(prefix, sub string) bool { return col >= runeCount(prefix) && col < runeCount(prefix+sub) }
	switch {
	case state != "" && in(upstream+" ", state):
		a.operationMenu()
	case state != "" && in(upstream+" "+state+" ", repoName):
		a.openRecentRepos()
	case state == "" && in(upstream+" ", repoName):
		a.openRecentRepos()
	}
}

// statusParts are the parts of the status line lazygit's onClick measures:
// the checked-out branch's upstream status, the working tree state in
// parentheses ("" for none) and the repo's name.
func (a *App) statusParts() (upstream, state, repoName string) {
	if h := a.snap.Head; h != nil {
		upstream = utils.Decolorise(presentation.BranchStatus(h, types.ItemOperationNone, a.repo.Common.Tr, time.Now(), a.repo.Config))
	}
	if t := a.snap.StatusLowerTitle; t != "" {
		state = "(" + t + ")"
	}
	return upstream, state, a.repo.Git.RepoPaths.RepoName()
}

func runeCount(s string) int { return utf8.RuneCountInString(s) }

// arrowSpan records a directory row's arrow at lazygit's columns: the arrow
// at visual depth × 2 and the space after it (FilesController.GetOnClick).
func (a *App) arrowSpan(i, depth int, arrow ui.Element) {
	if a.hitLine != nil {
		a.cells.span(a.hitView, a.hitLine(i), depth*2, depth*2+2, arrow)
	}
}

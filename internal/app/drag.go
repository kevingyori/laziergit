package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/context/traits"
	"github.com/samber/lo"
	"github.com/stefanhaller/git-todo-parser/todo"
)

// Mouse drags, as lazygit's: a drag in a side list extends a range selection
// from the pressed row (ListController.HandleDrag), a drag that starts on
// the selected commits moves them (LocalCommitsController's commit drag,
// with a "drop here" line where they would land), and a drag in the focused
// diff selects a range of lines from the clicked one
// (MainViewController.onDragInFocusedView). Held at a view's edge, the view
// keeps scrolling (helpers.DragAutoscroller). The keyboard equivalents:
// shift+up/down (range select) and the move commit up/down keys.

// pointerDrag is the gesture of the left button held down on a side list or
// the focused diff, from its press to its release.
type pointerDrag struct {
	view string // the lazygit view the press landed in
	v    sideView
	p    panel
	diff bool // the focused main view (staging), not a side list
	// line and col are the cell of the last event: gocui reports a drag
	// only when the pointer reaches another cell.
	line, col int
	moved     bool
	canceled  bool // gesture given up (escape): the rest is ignored
	commit    *commitDrag
	auto      dragAuto
}

// commitDrag is lazygit's commitDragState.
type commitDrag struct {
	pressed, start, end int
	ids                 []commitDragID
	selOffset, rangeOff int
	mode                traits.RangeSelectMode
	minIns, maxIns      int
	ins                 int // insertion index, -1 when dropping wouldn't move anything
	moved               bool
}

type commitDragID struct {
	hash, name string
	action     todo.TodoCommand
	flag       string
}

func commitDragIDOf(c *models.Commit) commitDragID {
	return commitDragID{c.Hash(), c.Name, c.Action, c.ActionFlag}
}

// dragAuto is lazygit's DragAutoscroller.
type dragAuto struct {
	dir      int
	interval time.Duration
	due      time.Time
	ptrRow   int // the pointer's row relative to the viewport
}

const (
	dragAutoscrollInitialDelay     = 300 * time.Millisecond
	dragAutoscrollSlowInterval     = 250 * time.Millisecond
	dragAutoscrollFastInterval     = 100 * time.Millisecond
	dragAutoscrollVeryFastInterval = 50 * time.Millisecond
)

// dragAutoscrollZone is lazygit's: beyond the view scrolls very fast, the
// outermost row fast, the row inside it slowly.
func dragAutoscrollZone(height, row int) (int, time.Duration) {
	switch {
	case row < 0:
		return -1, dragAutoscrollVeryFastInterval
	case row == 0:
		return -1, dragAutoscrollFastInterval
	case row == 1:
		return -1, dragAutoscrollSlowInterval
	case row > height-1:
		return 1, dragAutoscrollVeryFastInterval
	case row == height-1:
		return 1, dragAutoscrollFastInterval
	case row == height-2:
		return 1, dragAutoscrollSlowInterval
	}
	return 0, 0
}

// update is DragAutoscroller.Update: entering a zone arms the timer after
// an initial delay; it keeps going until the pointer leaves the zone.
func (d *dragAuto) update(now time.Time, row, height int, can func(int) bool) {
	dir, iv := dragAutoscrollZone(height, row)
	if dir != 0 && can != nil && !can(dir) {
		dir, iv = 0, 0
	}
	d.ptrRow = row
	if dir == d.dir {
		d.interval = iv
		return
	}
	d.dir, d.interval = dir, iv
	if dir != 0 {
		d.due = now.Add(dragAutoscrollInitialDelay)
	}
}

func (d *dragAuto) cancel() { d.dir, d.interval = 0, 0 }

// tickDrag runs the drag autoscroller's due ticks, at the start of a frame
// so the scroll shows in it, and asks for a frame when the next is due.
func (a *App) tickDrag(c *ui.Context) {
	now := c.Now()
	a.now = now
	d := a.drag
	if d == nil || d.canceled || d.auto.dir == 0 {
		return
	}
	if !now.Before(d.auto.due) {
		if a.dragScrollStep() {
			d.auto.due = now.Add(d.auto.interval)
		} else {
			d.auto.cancel()
			return
		}
	}
	c.After(d.auto.due.Sub(now))
}

// dragScrollStep is DragAutoscroller.tick: scroll one line and act on the
// line that is then under the pointer; false stops the scrolling.
func (a *App) dragScrollStep() bool {
	d := a.drag
	if d.diff {
		s := a.staging
		if s == nil || !a.canDiffAutoscroll() {
			return false
		}
		old := s.origin()
		target := clamp(old+d.auto.dir, 0, max(0, s.viewLines()-s.innerHeight()))
		if target == old {
			return false
		}
		s.setOrigin(target)
		line := clamp(target+d.auto.ptrRow, 0, max(0, s.viewLines()-1))
		cy := clamp(line-target, 0, max(0, s.innerHeight()-1))
		s.cursor = clamp(target+cy, 0, len(s.rows)-1)
		return true
	}
	if d.commit != nil && !a.canCommitAutoscroll(d.auto.dir) {
		return false
	}
	if d.commit == nil && !a.selectingRange(d.p) {
		return false
	}
	list := &a.lists[d.p]
	first, last := list.Visible()
	if first < 0 || last < first {
		return false
	}
	n := a.count(d.p)
	target := clamp(first+d.auto.dir, 0, max(0, n-(last-first+1)))
	if target == first {
		return false
	}
	list.ScrollTo(target, ui.Start)
	line := a.modelToViewLine(d.p, target) + d.auto.ptrRow
	if d.commit != nil {
		a.updateCommitDragInsertion(line)
		return a.canCommitAutoscroll(d.auto.dir)
	}
	a.selectRangeThroughLine(d.p, line)
	return true
}

// listViewport is the side list's viewport in view lines: its first line
// and its height.
func (a *App) listViewport(p panel) (origin, height int) {
	first, last := a.lists[p].Visible()
	if first < 0 || last < first {
		return 0, 0
	}
	return a.modelToViewLine(p, first), last - first + 1
}

// nonModelItems are the lines of panel p's list that show no item (section
// headers, the drop line), as lazygit's NonModelItem indices, in order.
func (a *App) nonModelItems(p panel) []int {
	if p != panelCommits || a.commitsView() != cvCommits || a.snap == nil {
		return nil
	}
	var out []int
	rebasing := a.snap.Operation == "Rebasing"
	if rebasing {
		out = append(out, 0)
	}
	if d := a.drag; d != nil && d.commit != nil && d.commit.ins >= 0 {
		out = append(out, d.commit.ins)
	}
	if rebasing {
		out = append(out, models.HeadCommitIdx(a.snap.Commits))
	}
	return out
}

// commitDropLine is the view line of the "drop here" line, -1 for none.
func (a *App) commitDropLine() int {
	d := a.drag
	if d == nil || d.commit == nil || d.commit.ins < 0 {
		return -1
	}
	for k, idx := range a.nonModelItems(panelCommits) {
		if idx == d.commit.ins && (a.snap.Operation != "Rebasing" || k == 1) {
			return idx + k
		}
	}
	return -1
}

// modelToViewLine is lazygit's modelIndexToViewIndex.
func (a *App) modelToViewLine(p panel, i int) int {
	i = clamp(i, 0, a.count(p))
	line := i
	for _, idx := range a.nonModelItems(p) {
		if idx <= i {
			line++
		}
	}
	return line
}

// viewLineToModel is lazygit's viewIndexToModelIndex.
func (a *App) viewLineToModel(p panel, line int) int {
	nm := a.nonModelItems(p)
	line = clamp(line, 0, a.count(p)+len(nm))
	i := line
	for k, idx := range nm {
		if idx+k < line {
			i--
		}
	}
	return i
}

// sidePress starts a drag on side list view at line: the press on the
// focused commits view's selection starts a commit drag (and doesn't
// select), as LocalCommitsController.handleCommitDragPress; reports whether
// it did.
func (a *App) sidePress(view string, v sideView, line, col, clicks int) bool {
	p := v.group()
	a.drag = &pointerDrag{view: view, v: v, p: p, line: line, col: col}
	if p != panelCommits || a.commitsView() != cvCommits || clicks != 1 || a.snap == nil ||
		a.FocusedSideView() != view || a.focus != p || a.staging != nil || a.mainFocused {
		return false
	}
	pressed := a.viewLineToModel(p, line)
	start, end := a.selectionRange(p)
	commits := a.snap.Commits
	if pressed < start || pressed > end || end >= len(commits) || a.modelToViewLine(p, pressed) != line ||
		!a.canMoveCommits(commits[start:end+1]) {
		return false
	}
	minIns, maxIns := a.commitDragBounds(start, end)
	r := a.ranges[p]
	rangeStart, mode := a.cursor[p], traits.RangeSelectModeNone
	if a.selectingRange(p) {
		rangeStart, mode = r.start, r.mode
	}
	a.drag.commit = &commitDrag{
		pressed: pressed, start: start, end: end,
		ids:       lo2ids(commits[start : end+1]),
		selOffset: a.cursor[p] - start, rangeOff: rangeStart - start, mode: mode,
		minIns: minIns, maxIns: maxIns, ins: -1,
	}
	return true
}

func lo2ids(cs []*models.Commit) []commitDragID {
	return lo.Map(cs, func(c *models.Commit, _ int) commitDragID { return commitDragIDOf(c) })
}

// canMoveCommits is lazygit's midRebaseMoveCommandEnabled.
func (a *App) canMoveCommits(cs []*models.Commit) bool {
	if a.snap.OperationCommand == "cherry-pick" || a.snap.OperationCommand == "revert" {
		return false
	}
	if a.snap.Operation != "Rebasing" {
		return !lo.SomeBy(cs, (*models.Commit).IsMerge)
	}
	for _, c := range cs {
		if !c.IsTODO() {
			return false
		}
		switch c.Action {
		case todo.Pick, todo.Drop, todo.Edit, todo.Fixup, todo.Squash, todo.Reword, todo.UpdateRef:
		default:
			return false
		}
	}
	return true
}

// commitDragBounds is lazygit's commitDragInsertionBounds: mid-rebase, the
// contiguous todos around the selection.
func (a *App) commitDragBounds(start, end int) (int, int) {
	commits := a.snap.Commits
	if a.snap.Operation != "Rebasing" {
		return 0, len(commits)
	}
	first, past := start, end+1
	for first > 0 && commits[first-1].IsTODO() && commits[first-1].Status != models.StatusConflicted {
		first--
	}
	for past < len(commits) && commits[past].IsTODO() && commits[past].Status != models.StatusConflicted {
		past++
	}
	return first, past
}

// sideDragMove is a held pointer moving to line, col of the press's view
// (ListController.HandleDrag, or handleCommitDrag).
func (a *App) sideDragMove(line, col int) {
	d := a.drag
	if d == nil || d.canceled || d.diff || (line == d.line && col == d.col) {
		return
	}
	d.line, d.col, d.moved = line, col, true
	origin, height := a.listViewport(d.p)
	if c := d.commit; c != nil {
		c.moved = true
		a.updateCommitDragInsertion(line)
		d.auto.update(a.now, line-origin, height, a.canCommitAutoscroll)
		return
	}
	if d.p == panelStatus {
		return
	}
	a.selectRangeThroughLine(d.p, line)
	d.auto.update(a.now, line-origin, height, func(int) bool { return a.selectingRange(d.p) })
}

// selectRangeThroughLine is lazygit's selectRangeThroughViewIndex: the
// range extends to the item at view line, which may be off screen; the
// view keeps its scroll.
func (a *App) selectRangeThroughLine(p panel, line int) {
	n := a.count(p)
	if n == 0 {
		return
	}
	i := clamp(a.viewLineToModel(p, line), 0, n-1)
	if !a.selectingRange(p) {
		a.ranges[p].start = a.cursor[p]
	}
	if i == a.cursor[p] && !a.selectingRange(p) {
		return
	}
	a.ranges[p].list = a.listScope(p)
	a.ranges[p].mode = traits.RangeSelectModeNonSticky
	a.cursor[p] = i
	if a.ranges[p].start == i {
		a.cancelRange(p)
	}
	a.loadMain()
}

// updateCommitDragInsertion is lazygit's: rows above the dragged block
// insert before the pointed-at commit, rows below it after it.
func (a *App) updateCommitDragInsertion(line int) {
	c := a.drag.commit
	n := a.count(panelCommits)
	var ins int
	switch {
	case line < 0:
		ins = c.minIns
	case line >= n+len(a.nonModelItems(panelCommits)):
		ins = c.maxIns
	default:
		i := a.viewLineToModel(panelCommits, line)
		ins = i
		if i > c.end {
			ins++
		}
		ins = max(c.minIns, min(ins, c.maxIns))
	}
	if ins >= c.start && ins <= c.end+1 {
		ins = -1
	}
	c.ins = ins
}

// canCommitAutoscroll is lazygit's canCommitDragAutoscroll.
func (a *App) canCommitAutoscroll(dir int) bool {
	d := a.drag
	if d == nil || d.commit == nil {
		return false
	}
	if dir < 0 {
		return d.commit.ins != d.commit.minIns
	}
	return d.commit.ins != d.commit.maxIns
}

// sideRelease ends the drag (handleCommitDragRelease / handleDragRelease).
func (a *App) sideRelease() {
	d := a.drag
	a.drag = nil
	if d == nil || d.canceled || d.commit == nil {
		return
	}
	c := d.commit
	p := panelCommits
	if !c.moved {
		a.cancelRange(p)
		a.cursor[p] = clamp(c.pressed, 0, a.count(p)-1)
		a.loadMain()
		return
	}
	if c.ins < 0 {
		return
	}
	offset := c.ins - c.start
	if c.ins > c.end {
		offset = c.ins - c.end - 1
	}
	start, end, ok := findCommitDragBlock(a.snap.Commits, c.ids)
	if !ok {
		return
	}
	a.setRange(p, start+c.selOffset, start+c.rangeOff, c.mode)
	commits := a.snap.Commits
	tr := a.repo.Common.Tr
	label := tr.MovingStatus
	rebasing := a.snap.Operation == "Rebasing"
	if rebasing {
		label = "Updating rebase todo…"
	}
	a.nextCommitSel = keepCommitSelectionIndex
	a.run(label, func() error { return a.repo.MoveCommitsBy(commits, start, end, offset) }, func() {
		n := a.count(p)
		a.cursor[p] = clamp(a.cursor[p]+offset, 0, n-1)
		if a.ranges[p].mode != traits.RangeSelectModeNone {
			a.ranges[p].start = clamp(a.ranges[p].start+offset, 0, n-1)
		}
		a.lists[p].ScrollIntoView(a.cursor[p])
		a.loadMain()
	})
}

// findCommitDragBlock is lazygit's: the dragged commits, found again by
// identity in the (maybe refreshed) list; one contiguous match or none.
func findCommitDragBlock(commits []*models.Commit, ids []commitDragID) (int, int, bool) {
	match := -1
	for s := 0; s+len(ids) <= len(commits); s++ {
		ok := true
		for k, id := range ids {
			if commitDragIDOf(commits[s+k]) != id {
				ok = false
				break
			}
		}
		if ok {
			if match >= 0 {
				return -1, -1, false
			}
			match = s
		}
	}
	if match < 0 {
		return -1, -1, false
	}
	return match, match + len(ids) - 1, true
}

// cancelCommitDrag is lazygit's handleCommitDragCancel (escape while
// dragging commits): the rest of the gesture is ignored.
func (a *App) cancelCommitDrag() bool {
	d := a.drag
	if d == nil || d.commit == nil || d.canceled {
		return false
	}
	d.canceled = true
	d.commit.ins = -1
	d.auto.cancel()
	return true
}

// diffPress is a press on line of the focused diff: the click selects
// (selectClickedDiffLine) and a drag may follow.
func (a *App) diffPress(view string, line, col int) {
	a.drag = &pointerDrag{view: view, diff: true, line: line, col: col}
}

// diffDragMove is lazygit's onDragInFocusedView: a plain range from the
// clicked line to the line under the pointer, which stays inside the view.
func (a *App) diffDragMove(line, col int) {
	d := a.drag
	s := a.staging
	if d == nil || !d.diff || d.canceled || s == nil || s.cursor < 0 || (line == d.line && col == d.col) {
		return
	}
	d.line, d.col, d.moved = line, col, true
	origin, height := s.origin(), s.innerHeight()
	cy := clamp(line-origin, 0, max(0, height-1))
	s.cursor = clamp(origin+cy, 0, len(s.rows)-1)
	s.mode, s.sticky, s.userHunk = selectRange, false, false
	s.anchor = clamp(s.dragAnchor, 0, len(s.rows)-1)
	d.auto.update(a.now, line-origin, height, func(int) bool { return a.canDiffAutoscroll() })
}

// canDiffAutoscroll is lazygit's MainViewController.canDragAutoscroll.
func (a *App) canDiffAutoscroll() bool {
	d, s := a.drag, a.staging
	return d != nil && d.diff && s != nil && s.cursor >= 0 && s.mode == selectRange
}

// stagingMouse is the pointer input of the focused diff s in pane: a plain
// press selects the line (selectClickedDiffLine, also from the other pane of
// the pair) and starts a drag; a wheel notch scrolls (paneMouse).
// Modified clicks and diffstat links go on to the rows' own handling.
func (a *App) stagingMouse(s *stagingState, pane string) func(ui.InputEvent) bool {
	wheel := a.paneMouse(pane)
	return func(ev ui.InputEvent) bool {
		at := func() (int, int, bool) {
			r, ok := a.cells.panes[pane]
			if !ok {
				return 0, 0, false
			}
			line, _, col := a.cells.at(pane, r.X+ev.X, r.Y+ev.Y)
			return line, col, true
		}
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 || ev.Mods&(ui.Alt|ui.Shift|ui.Ctrl|ui.Cmd) != 0 {
				return false
			}
			line, col, ok := at()
			if s.cursor < 0 {
				// Nothing to select: gocui's cursor still goes where the click points.
				if ok && col < 0 {
					s.clickDiffLine(line)
				}
				return false
			}
			if !ok || col >= 0 {
				return false
			}
			if a.staging != s {
				a.staging = s
			}
			s.clickDiffLine(line)
			a.diffPress(pane, line, int(ev.X))
			return true
		case ui.InputPointerMove:
			if d := a.drag; d != nil && d.diff && ev.Button == 0 {
				if line, _, ok := at(); ok {
					a.diffDragMove(line, int(ev.X))
				}
				return true
			}
		case ui.InputPointerUp:
			if d := a.drag; d != nil && d.diff {
				a.drag = nil
				return true
			}
		}
		return wheel(ev)
	}
}

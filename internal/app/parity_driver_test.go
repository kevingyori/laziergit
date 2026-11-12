package app

import (
	"fmt"
	"github.com/gookit/color"
	"github.com/xo/terminfo"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/generics/set"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gocui"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/graph"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/icons"
	"github.com/jesseduffield/lazygit/pkg/gui/types"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"

	"laziergit/internal/keymap"
)

// parityDriver implements lazygit's integrationTypes.GuiDriver on top of the
// headless app, so lazygit's own integration tests can drive laziergit.
//
// Keys go through real input events. Views are projections: each lazygit
// view name maps to the app state that plays its role, rendered with
// lazygit's own presentation code where our rows correspond to theirs, so a
// passing assertion means the same state and selection, not the same pixels.
type parityDriver struct {
	a    *App
	jobs *parityJobs
	tt   *ui.Tester
	// menuItemLine and menuItemTop are, per visible menu item, its line in
	// the menu projection and the first line it brings (its section header).
	menuItemLine, menuItemTop []int
	// lastKey is the last key pressed and the view it was pressed in, so a
	// failure can say what the app didn't do.
	lastKey string
	// toasts are status-bar messages the app set, standing in for lazygit's
	// toasts; status is the last one seen.
	toasts    []string
	statusSeq int
	status    string
	// The run continues past failed checks so a test's progress can be
	// measured: keys counts key presses, first is the first failed check
	// and firstAt the key count when it failed.
	keys    int
	first   *parityFailure
	firstAt int
	// pad holds the line count a failed count check expected, per view.
	// lazygit's assertions index those lines right after the count check,
	// assuming a failure aborts; padding keeps the run alive until the next
	// key. It only matters after a failure has been recorded.
	pad map[string]int
	// held: the mouse button is down (ClickAndHold until MouseRelease).
	held bool
}

// parityJobs preserves the old synchronous job boundary, except that a git
// credential prompt yields to the tester. UI callbacks always run on the driver
// goroutine. After an answer, wait drains the job (or reaches its next prompt)
// before another assertion can inspect the app. No sleeps or timing heuristics.
type parityJobs struct {
	a       *App
	driver  uint64
	mu      sync.Mutex
	events  []func()
	ready   []func() // driver-only: nested waits must see the rest of a drained batch
	wake    chan struct{}
	pending int
}

// parityGoroutineID reads the runtime's goroutine header only in this test harness.
func parityGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	id, err := strconv.ParseUint(strings.Fields(string(buf[:n]))[1], 10, 64)
	if err != nil {
		panic(err)
	}
	return id
}

func (j *parityJobs) post(fn func()) {
	j.mu.Lock()
	j.events = append(j.events, fn)
	j.mu.Unlock()
	select {
	case j.wake <- struct{}{}:
	default:
	}
}

func (j *parityJobs) bg(fn func()) {
	if parityGoroutineID() != j.driver {
		fn()
		return
	}
	if j.credential() != nil {
		panic(parityFailure{msg: "harness: background job started while a git credential prompt is open"})
	}
	j.pending++
	go func() {
		defer func() {
			failure := recover()
			j.post(func() {
				j.pending--
				if failure != nil {
					panic(failure)
				}
			})
		}()
		fn()
	}()
	j.wait()
}

// credential stands in for lazygit's cmd_obj_runner.go task.Pause()/Continue()
// around <-responseChan: the app passes gocui.NewFakeTask(). If the repo layer
// ever passes a real task, switch this signal to Pause/Continue.
func (j *parityJobs) credential() *promptOverlay {
	if j.a != nil {
		for _, overlay := range j.a.overlays {
			if o, ok := overlay.(*promptOverlay); ok && o.credential {
				return o
			}
		}
	}
	return nil
}

func (j *parityJobs) wait() {
	for {
		j.mu.Lock()
		events := j.events
		j.events = nil
		j.mu.Unlock()
		j.ready = append(j.ready, events...)
		if len(j.ready) > 0 {
			fn := j.ready[0]
			j.ready[0] = nil
			j.ready = j.ready[1:]
			fn()
			continue
		}
		if j.credential() != nil {
			return
		}
		if j.pending == 0 {
			return
		}
		<-j.wake
	}
}

// A failed assertion must not leave git waiting while its sandbox is removed.
// A job blocked on anything other than a prompt becomes "harness: timed out"
// through the 30 s runner timeout, which only affects failing tests.
func (j *parityJobs) close() {
	for j.pending > 0 {
		if o := j.credential(); o != nil {
			o.cancel(j.a)
		}
		j.wait()
	}
}

var lineCount = regexp.MustCompile(`lines in view '(\w+)'\. Expected (?:equals|at least) (\d+)`)

// parityFailure is what Fail panics with; the runner recovers it.
type parityFailure struct{ msg, after, screen string }

// unsupported fails a test on something the harness can't express, so the
// report can tell those apart from real behaviour gaps.
func unsupported(what string) { panic(parityFailure{msg: "harness: " + what + " not supported"}) }

func (d *parityDriver) settle() {
	d.jobs.wait()
	d.tt.Frame()
	d.jobs.wait()
	d.tt.Frame()
	d.jobs.wait()
	if d.a.status != d.status || d.a.statusSeq != d.statusSeq {
		d.status, d.statusSeq = d.a.status, d.a.statusSeq
		if d.status != "" {
			d.toasts = append(d.toasts, d.status)
		}
	}
}

func (d *parityDriver) send(label string) {
	d.keys++
	d.pad = nil
	d.lastKey = label + " in " + d.focusedView()
	ch, ok := keymap.ParseLabel(label)
	if !ok {
		panic(parityFailure{msg: "harness: unknown key " + label})
	}
	switch {
	case ch.Rune != 0 && ch.Mods&(ui.Ctrl|ui.Alt) == 0:
		d.tt.TypeKey(ch.Mods, runeKey(ch.Rune), string(ch.Rune))
	case ch.Rune != 0:
		d.tt.Key(ch.Mods, runeKey(ch.Rune))
	case ch.Key == ui.KeySpace:
		d.tt.TypeKey(ch.Mods, ui.KeySpace, " ")
	default:
		d.tt.Key(ch.Mods, ch.Key)
	}
}

func runeKey(r rune) ui.Key {
	switch {
	case r >= 'a' && r <= 'z':
		return ui.KeyA + ui.Key(r-'a')
	case r >= 'A' && r <= 'Z':
		return ui.KeyA + ui.Key(r-'A')
	case r >= '0' && r <= '9':
		return ui.Key0 + ui.Key(r-'0')
	}
	switch r {
	case '-':
		return ui.KeyMinus
	case '=':
		return ui.KeyEqual
	case ',':
		return ui.KeyComma
	case '.':
		return ui.KeyPeriod
	case '/':
		return ui.KeySlash
	case ';':
		return ui.KeySemicolon
	case '\'':
		return ui.KeyQuote
	case '[':
		return ui.KeyBracketLeft
	case ']':
		return ui.KeyBracketRight
	case '\\':
		return ui.KeyBackslash
	case '`':
		return ui.KeyBackquote
	}
	return ui.KeyUnknown
}

func (d *parityDriver) PressKey(k string) { d.send(k); d.settle() }

func (d *parityDriver) PressKeysRapidly(ks ...string) {
	for _, k := range ks {
		d.send(k)
	}
	d.settle()
}

func (d *parityDriver) Click(x, y int) { d.click(x, y, 0, 1) }
func (d *parityDriver) ClickWithModifier(x, y int, m gocui.Modifier) {
	mods := map[gocui.Modifier]ui.Modifiers{gocui.ModAlt: ui.Alt, gocui.ModShift: ui.Shift}[m]
	if mods == 0 {
		unsupported(fmt.Sprintf("mouse click with modifier %v", m))
	}
	d.click(x, y, mods, 1)
}
func (d *parityDriver) DoubleClick(x, y int) { d.click(x, y, 0, 2) }
func (d *parityDriver) ClickAndHold(x, y int) {
	d.keys++
	d.pad = nil
	d.tt.Advance(time.Second)
	d.settle()
	px, py, label := d.point(x, y)
	d.lastKey = "press at " + label
	d.tt.Press(px, py)
	d.held = true
	d.settle()
}
func (d *parityDriver) MouseMove(x, y int) {
	d.keys++
	d.pad = nil
	px, py, label := d.point(x, y)
	d.lastKey = "move to " + label
	d.tt.Move(px, py)
	d.settle()
}
func (d *parityDriver) MouseRelease(x, y int) {
	d.keys++
	d.pad = nil
	px, py, label := d.point(x, y)
	d.lastKey = "release at " + label
	d.tt.Release(px, py)
	d.held = false
	d.settle()
}
func (d *parityDriver) ScrollWheelDown(x, y int) {
	d.keys++
	d.pad = nil
	d.tt.Advance(time.Second)
	d.settle()
	px, py, label := d.point(x, y)
	d.lastKey = "wheel down at " + label
	// One notch of a mouse wheel (not a touchpad's precise scroll), a row's
	// worth of DIPs down, as a platform reports it.
	d.tt.Wheel(px, py, 0, rowHeight)
	d.settle()
}
func (d *parityDriver) FocusInAndClick(x, y int) {
	d.a.FocusIn()
	d.click(x, y, 0, 1)
}
func (d *parityDriver) RefreshInBackground() { d.a.Refresh(); d.settle() }
func (d *parityDriver) OnUIThreadAndWait(f func()) {
	// While the button is held, time passes between lazygit's polls of an
	// eventual assertion (assertEventually sleeps 10ms between them), and
	// with it the drag autoscroller's timers.
	if d.held {
		d.tt.Advance(10 * time.Millisecond)
	}
	f()
}
func (d *parityDriver) FocusIn()                      { d.a.FocusIn(); d.settle() }
func (d *parityDriver) Keys() config.KeybindingConfig { return d.a.repo.Config.Keybinding }
func (d *parityDriver) CursorVisible() bool           { return d.a.wantsTextFocus() }
func (d *parityDriver) Fail(message string) {
	if m := lineCount.FindStringSubmatch(message); m != nil {
		if d.pad == nil {
			d.pad = map[string]int{}
		}
		d.pad[m[1]], _ = strconv.Atoi(m[2])
	}
	if d.first == nil {
		d.first = &parityFailure{message, d.lastKey, d.dump()}
		d.firstAt = d.keys
	}
}

// dump is the focused view as the test saw it, the selected line marked.
func (d *parityDriver) dump() string {
	name := d.focusedView()
	v := d.View(name)
	sel := -1
	if v.Highlight {
		sel = v.SelectedLineIdx()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s\n", name, v.Title)
	if d.a.status != "" {
		fmt.Fprintf(&b, "status: %s\n", d.a.status)
	}
	for i, l := range v.BufferLines() {
		if i == 40 {
			b.WriteString("  …\n")
			break
		}
		mark := "  "
		if i == sel {
			mark = "> "
		}
		b.WriteString(mark + l + "\n")
	}
	return b.String()
}

func (d *parityDriver) Log(string)              {}
func (d *parityDriver) LogUI(string)            {}
func (d *parityDriver) SetCaption(string)       {}
func (d *parityDriver) SetCaptionPrefix(string) {}
func (d *parityDriver) NextToast() *string {
	if len(d.toasts) == 0 {
		return nil
	}
	t := d.toasts[0]
	d.toasts = d.toasts[1:]
	return &t
}

func (d *parityDriver) CheckAllToastsAcknowledged()               {}
func (d *parityDriver) Headless() bool                            { return true }
func (d *parityDriver) PretendMergeOrRebaseStartedInLazygit()     { d.a.mergeStartedInLazygit = true }
func (d *parityDriver) MainView() *gocui.View                     { return d.View("main") }
func (d *parityDriver) SecondaryView() *gocui.View                { return d.View("secondary") }
func (d *parityDriver) ContextForView(name string) types.Context  { return d.context(name) }
func (d *parityDriver) CurrentContext() types.Context             { return d.context(d.focusedView()) }
func (d *parityDriver) TopViewInWindow(window string) *gocui.View { return d.View(d.topView(window)) }
func (d *parityDriver) CheckedOutRef() *models.Branch {
	if d.a.snap == nil {
		return nil
	}
	return d.a.snap.Head
}

// parityContext answers the few Context methods the test drivers call; any
// other method hits the nil embedded interface and fails the test.
type parityContext struct {
	types.Context
	view   *gocui.View
	window string
}

func (c *parityContext) GetView() *gocui.View     { return c.view }
func (c *parityContext) GetViewName() string      { return c.view.Name() }
func (c *parityContext) GetWindowName() string    { return c.window }
func (c *parityContext) GetKey() types.ContextKey { return types.ContextKey(c.view.Name()) }

func (d *parityDriver) context(name string) types.Context {
	return &parityContext{view: d.View(name), window: d.windowOf(name)}
}

// windowOf is lazygit's window for a view: the app's configured side panels
// (gui.sidePanels); transient views take the window they were opened from.
func (d *parityDriver) windowOf(view string) string {
	if view == "mergeConflicts" {
		return "main"
	}
	return d.a.SideWindowOf(view)
}

func (d *parityDriver) topView(window string) string {
	if v := d.a.SideTopView(window); v != "" {
		return v
	}
	return window
}

// sideViews are lazygit's side views, tabs and transient views.
// Keep it in step with sideViewLazygitNames plus the transient views.
var sideViews = map[string]bool{"status": true, "files": true, "worktrees": true, "submodules": true, "localBranches": true,
	"remotes": true, "tags": true, "remoteBranches": true, "subCommits": true, "commits": true, "reflogCommits": true,
	"commitFiles": true, "stash": true}

// focusedView names the lazygit view that has focus in the app's state.
func (d *parityDriver) focusedView() string {
	a := d.a
	switch o := a.topOverlay().(type) {
	case nil:
	case *menuOverlay:
		return "menu"
	case *confirmOverlay, *errorOverlay:
		return "confirmation"
	case *promptOverlay:
		if o.suggestionsFocused {
			return "suggestions"
		}
		return "prompt"
	case *searchOverlay:
		return "search"
	case *conflictOverlay:
		return "mergeConflicts"
	case *commitOverlay:
		if d.tt.Focused("Description") {
			return "commitDescription"
		}
		return "commitMessage"
	default:
		return fmt.Sprintf("%T", o)
	}
	// lazygit stages lines in the focused main (unstaged) or secondary
	// (staged) view; our staging pane plays that role.
	if s := a.staging; s != nil {
		if s.staged {
			return "secondary"
		}
		return "main"
	}
	if a.mainFocused {
		return "main"
	}
	return d.sideView()
}

// sideView is the side panel view the app's focus is in, also while the main
// view or a popup has the keys (lazygit's current side context).
func (d *parityDriver) sideView() string { return d.a.FocusedSideView() }

// projectInColour is project with gookit/color forced to true colour (and
// restored even on a panic), only around the projection.
func (d *parityDriver) projectInColour(v *gocui.View, name string) ([]string, int) {
	oldLevel := color.ForceSetColorLevel(terminfo.ColorLevelMillions)
	defer color.ForceSetColorLevel(oldLevel)
	return d.project(v, name)
}

// wrapSelection turns a wrapped pane's origin, cursor and range, set in
// buffer lines above, into view lines by gocui's own mapping: the origin and
// a range's top end at the first view line of their buffer line, a range's
// bottom end at the last.
func (d *parityDriver) wrapSelection(v *gocui.View) {
	_, oy := v.Origin()
	_, cy := v.Cursor()
	cur := cy + oy
	anchor := v.RangeSelectStartY()
	first := func(l int) int { y, _ := v.ViewLineForBufferLine(l); return y }
	last := func(l int) int { y, _ := v.LastViewLineForBufferLine(l); return y }
	viewOY := first(oy)
	v.SetOrigin(0, viewOY)
	if !v.Highlight && anchor < 0 {
		// A hidden cursor (App.PaneHiddenCursor) isn't remapped in a wrapped
		// pane; a test that needs it is a new oracle decision (O-2026-11-07-7).
		return
	}
	switch {
	case anchor < 0:
		v.SetCursor(0, first(cur)-viewOY)
	case cur >= anchor:
		v.SetRangeSelectStart(first(anchor))
		v.SetCursor(0, last(cur)-viewOY)
	default:
		v.SetRangeSelectStart(last(anchor))
		v.SetCursor(0, first(cur)-viewOY)
	}
}

// sideListPanel is the app panel whose list draws side view name.
var sideListPanel = map[string]panel{"status": panelStatus, "files": panelFiles, "worktrees": panelFiles, "submodules": panelFiles,
	"localBranches": panelBranches, "remotes": panelBranches, "tags": panelBranches, "remoteBranches": panelBranches, "subCommits": panelBranches,
	"commits": panelCommits, "reflogCommits": panelCommits, "commitFiles": panelCommits, "stash": panelStash}

// View builds a fresh gocui view holding the projection of view name.
func (d *parityDriver) View(name string) *gocui.View {
	// A side list shows the rows the app's list really shows: its height is
	// the list's visible row count and its origin the first of them.
	height, originY := 2000, -1
	if p, ok := sideListPanel[name]; ok && sideViews[name] && d.topView(d.windowOf(name)) == name {
		if first, last := d.a.lists[p].Visible(); last >= first {
			// gocui's inner height is y1-y0-1: exactly the visible row count.
			// Visible() is the frame drawn after the last key (the driver
			// renders one per key). The top view of a window is drawn by
			// its panel's list (a.lists), whichever tab or transient view.
			height, originY = last-first+2, first
		}
	}
	width := 400
	wraps := (name == "main" || name == "secondary") && d.a.PaneWraps(name)
	if wraps {
		// The pane wraps: gocui wraps the projected lines at the app's
		// column count (App.paneInnerWidth, the renderers' COLUMNS too).
		width = d.a.paneInnerWidth(name) + 1
	}
	// Each view sits in a band of rows of its own (mouseBand), so a cell
	// lazygit's tests compute from its geometry names the view back.
	if height >= mouseBand {
		unsupported(fmt.Sprintf("view %s taller than a mouse band", name))
	}
	y0 := mouseBand * (slices.Index(mouseViews, name) + 1)
	v := gocui.NewView(name, 0, y0, width, y0+height, gocui.OutputTrue)
	v.Wrap = wraps
	// lazygit's integration tests run in a true-colour terminal: project the
	// colours lazygit's presentation gives the app's state.
	lines, sel := d.projectInColour(v, name)
	for len(lines) < d.pad[name] {
		lines = append(lines, " ") // a trailing empty line would be dropped
	}
	v.SetContent(strings.Join(lines, "\n"))
	if originY >= 0 {
		v.SetOrigin(0, originY)
	}
	if name == "main" || name == "secondary" {
		x, y := d.a.paneOrigin(name)
		v.SetOrigin(x, y)
	}
	if m, ok := d.overlay(func(o overlay) bool { _, ok := o.(*menuOverlay); return ok }).(*menuOverlay); ok && name == "menu" {
		// The menu's origin: the first item the native list shows, with the
		// section header (and blank line) its row draws above it.
		if first, _ := m.list.Visible(); first >= 0 && first < len(d.menuItemTop) {
			v.SetOrigin(0, d.menuItemTop[first])
		}
	}
	if (name == "remoteBranches" || name == "subCommits" || name == "commitFiles") && d.topView(d.windowOf(name)) != name {
		// A transient view is only shown while its window shows it.
		v.Visible = false
	}
	if sel >= 0 {
		v.Highlight = true
		// gocui keeps the cursor relative to the origin.
		v.SetCursor(0, sel-v.OriginY())
	} else if name == "main" || name == "secondary" {
		// A diff with nothing to select draws no selection, but gocui's
		// cursor still sits where a click put it (App.PaneHiddenCursor).
		if y := d.a.PaneHiddenCursor(name); y >= 0 {
			v.SetCursor(0, y-v.OriginY())
		}
	}
	if sideViews[name] {
		// lazygit's UpdateSelectionHighlights: a side list draws a selection
		// while its context is on the stack (it is the current side context,
		// under a focused main view or popup) and it has rows; dimmed unless it
		// is the current context.
		v.Highlight = sel >= 0 && len(lines) > 0 && strings.TrimSpace(strings.Join(lines, "")) != "" && name == d.sideView()
		v.HighlightInactive = name != d.focusedView()
	}
	var st *stagingState
	if name == "main" {
		st = d.a.mainDiffState()
	} else if name == "secondary" && d.a.staging != nil && d.a.staging.staged {
		st = d.a.staging
	}
	if st != nil && st.cursor >= 0 {
		v.HighlightInactive = st != d.a.staging
		if anchor := st.rangeAnchor(); anchor >= 0 {
			v.SetRangeSelectStart(anchor)
		}
		marks := make([]bool, len(st.rows))
		for i, row := range st.rows {
			marks[i] = !st.worktree && !st.staged && d.a.patchMarked(row)
		}
		v.SetInclusionGutter(d.a.customPatch != nil && !st.worktree && !st.staged, marks)
	} else if name == "main" && d.a.mainDiffState() == nil {
		// The unfocused main view marks the custom patch's lines while it
		// shows the diff the patch is built from (App.mainPatchMarks).
		if marks := d.a.mainPatchMarks(); marks != nil {
			v.SetInclusionGutter(true, marks)
		}
	}
	if wraps {
		d.wrapSelection(v)
	}
	if name == "mergeConflicts" {
		// lazygit's MergeConflictsContext.SetSelectedLineRange: the selected
		// hunk is always a range, anchored at its start, cursor at its end.
		if o, ok := d.overlay(func(o overlay) bool { _, ok := o.(*conflictOverlay); return ok }).(*conflictOverlay); ok && !o.state.NoConflicts() {
			start, _ := o.state.GetSelectedRange()
			v.SetRangeSelectStart(start)
		}
	}
	if name == "subCommits" {
		// lazygit's SubCommitsContext title and scroll origin: the title is
		// the ref it was opened for, the origin the first row the list shows.
		p := panelBranches
		if d.a.refBrowse != nil && d.a.refBrowse.ref != nil {
			v.Title = d.a.subCommitsTitle()
			p = d.a.refBrowse.panel
		}
		first, _ := d.a.lists[p].Visible()
		v.SetOrigin(0, first)
		if b := d.a.refBrowse; b != nil && b.ref != nil && d.a.multipleSelected(p) {
			first, last := d.a.selectionRange(p)
			anchor := first
			if d.a.cursor[p] == first {
				anchor = last
			}
			_, line := d.subCommitLines(b, anchor)
			v.SetRangeSelectStart(line)
		}
	}
	// A range selection in a flat list (one row per item): anchored at the
	// end of the range away from the cursor.
	if p, ok := map[string]panel{"files": panelFiles, "localBranches": panelBranches, "remotes": panelBranches, "tags": panelBranches,
		"remoteBranches": panelBranches, "stash": panelStash}[name]; ok && d.topView(d.windowOf(name)) == name && d.a.multipleSelected(p) {
		first, last := d.a.selectionRange(p)
		anchor := first
		if d.a.cursor[p] == first {
			anchor = last
		}
		v.SetRangeSelectStart(anchor)
	}
	if name == "commits" && d.a.commitFiles == nil && d.a.multipleSelected(panelCommits) {
		first, last := d.a.selectionRange(panelCommits)
		anchor := first
		if d.a.cursor[panelCommits] == first {
			anchor = last
		}
		_, line := d.commitLines(anchor)
		v.SetRangeSelectStart(line)
	}
	if name == "commitFiles" && d.a.commitFiles != nil && d.a.multipleSelected(panelCommits) {
		first, last := d.a.selectionRange(panelCommits)
		anchor := first
		if d.a.cursor[panelCommits] == first {
			anchor = last
		}
		v.SetRangeSelectStart(anchor)
	}
	return v
}

// tabSel is the selection of tab tab in panel p: a tab that isn't shown
// keeps its place, as lazygit's contexts each keep their own selection.
func (d *parityDriver) tabSel(p panel, tab int, active bool) int {
	if !active {
		w := 0
		if p == panelBranches {
			w = 1
		}
		return d.a.tabCursor[w][tab]
	}
	return d.cursorIf(p, true)
}

func (d *parityDriver) cursorIf(p panel, active bool) int {
	if !active {
		return 0
	}
	// While a commits drill-down is open, the branches panel's cursor belongs
	// to it; a tab keeps the place it had before the drill-down.
	if b := d.a.browse(panelBranches); p == panelBranches && b != nil && b.ref != nil {
		return d.a.tabCursor[1][d.a.branchTab]
	}
	return d.a.cursor[p]
}

func render(rows [][]string) []string {
	lines, _ := utils.RenderDisplayStrings(rows, nil)
	return lines
}

func noOp(types.HasUrn) types.ItemOperation { return types.ItemOperationNone }

// project returns view name's lines and selected line (-1: no selection),
// setting titles on v where the app has them.
func (d *parityDriver) project(v *gocui.View, name string) ([]string, int) {
	a := d.a
	s := a.snap
	if s == nil {
		return nil, -1
	}
	cfg := a.repo.Config
	tr := a.repo.Common.Tr
	switch name {
	case "status":
		// lazygit's statusController: the repo's name (RepoPaths.RepoName) and,
		// in a linked worktree, its name (WorktreeHelper.GetLinkedWorktreeName),
		// as the native status row draws them.
		repoName := a.repo.Git.RepoPaths.RepoName()
		if s.Head == nil {
			return []string{repoName}, 0
		}
		linked := ""
		if len(s.Worktrees) > 0 && !s.Worktrees[0].IsMain {
			linked = s.Worktrees[0].Name
		}
		return []string{presentation.FormatStatus(repoName, s.Head, types.ItemOperationNone, linked,
			a.repo.Git.Status.WorkingTreeState(), tr, cfg)}, 0
	case "files":
		// The app keeps the Files rows in lazygit's tree view model; render
		// them with lazygit's code, as its working-tree context does.
		showIcons := icons.IsIconEnabled() && cfg.Gui.ShowFileIcons
		return presentation.RenderFileTree(a.files, s.Submodules, showIcons, cfg.Gui.ShowNumstatInFilesView,
			&cfg.Gui.CustomIcons, cfg.Gui.ShowRootItemInFileTree, tr), d.tabSel(panelFiles, 0, a.fileTab == 0)
	case "worktrees":
		return render(presentation.GetWorktreeDisplayStrings(tr, s.Worktrees)), d.tabSel(panelFiles, 1, a.fileTab == 1)
	case "submodules":
		return render(presentation.GetSubmoduleListDisplayStrings(s.Submodules)), d.tabSel(panelFiles, 2, a.fileTab == 2)
	case "localBranches":
		return render(presentation.GetBranchListDisplayStrings(s.Branches, noOp, a.prs, a.enlarged(), "", 400, tr, cfg, s.Worktrees)),
			d.tabSel(panelBranches, 0, a.branchTab == 0)
	case "remotes":
		return render(presentation.GetRemoteListDisplayStrings(s.Remotes, "", noOp, tr, cfg)), d.tabSel(panelBranches, 1, a.branchTab == 1)
	case "remoteBranches":
		if b := a.browse(panelBranches); b != nil && b.ref == nil {
			return render(presentation.GetRemoteBranchListDisplayStrings(b.branches, "")), d.cursorIf(panelBranches, true)
		}
		return nil, -1
	case "tags":
		return render(presentation.GetTagListDisplayStrings(s.Tags, noOp, "", tr, cfg)), d.tabSel(panelBranches, 2, a.branchTab == 2)
	case "subCommits":
		if b := a.refBrowse; b != nil && b.ref != nil {
			return d.subCommitLines(b, a.cursor[b.panel])
		}
		return nil, -1
	case "commits":
		return d.commitLines(d.cursorIf(panelCommits, a.commitsView() == cvCommits))
	case "reflogCommits":
		// lazygit's ReflogCommitsContext rendering of the app's reflog rows.
		sel := a.commitTabCursor[1]
		if a.commitsView() == cvReflog {
			sel = a.cursor[panelCommits]
		}
		return render(presentation.GetReflogCommitListDisplayStrings(s.Reflog, 0, len(s.Reflog), a.enlarged(), a.cherry.SelectedHashSet(),
			"", time.Now(), cfg.Gui.TimeFormat, cfg.Gui.ShortTimeFormat, cfg.Git.ParseEmoji)), sel
	case "commitFiles":
		cf := a.commitFiles
		if cf == nil {
			return nil, -1
		}
		if a.emptyCommitFiles() {
			return []string{emptyCommitFilesLabel}, -1
		}
		showIcons := icons.IsIconEnabled() && cfg.Gui.ShowFileIcons
		return presentation.RenderCommitFileTree(cf.tree, a.repo.Git.Patch.PatchBuilder, showIcons, &cfg.Gui.CustomIcons),
			a.cursor[panelCommits]
	case "stash":
		return render(presentation.GetStashEntryListDisplayStrings(s.Stash, "")), a.cursor[panelStash]
	case "main":
		v.Visible = a.mainVisible()
		if !v.Visible {
			return nil, -1
		}
		v.Title = a.mainTitle
		v.TitlePrefix = a.focusMainJumpLabel("main")
		if st := a.mainDiffState(); st != nil {
			return d.stagingLines(st), st.cursor
		}
		// The raw text the pane was loaded from: parsing for display moves
		// headers (commit message, file names) into the pane's chrome.
		if _, load := a.mainSource(); load != nil {
			return strings.Split(strings.TrimRight(load(), "\n"), "\n"), -1
		}
		return nil, -1
	case "information":
		if info := a.modeInfo(); info != "" {
			return []string{info + " " + tr.ResetInParentheses}, -1
		}
		return nil, -1
	case "secondary":
		v.Title = a.secondaryTitle
		v.TitlePrefix = a.focusMainJumpLabel("secondary")
		if st := a.staging; st != nil && st.staged {
			return d.stagingLines(st), st.cursor
		}
		v.Visible = a.secondaryVisible()
		if !v.Visible || a.secondaryText == "" {
			return nil, -1
		}
		return strings.Split(strings.TrimRight(a.secondaryText, "\n"), "\n"), -1
	case "mergeConflicts":
		// The conflicted file's current content as the app's conflict view
		// holds it (lazygit's ColoredConflictFile minus colours), with the
		// selected hunk's end as the cursor. Once no conflict remains the app
		// shows no file content.
		if o, ok := d.overlay(func(o overlay) bool { _, ok := o.(*conflictOverlay); return ok }).(*conflictOverlay); ok && !o.state.NoConflicts() {
			_, end := o.state.GetSelectedRange()
			return utils.SplitLines(o.state.GetContent()), end
		}
		return nil, -1
	case "options":
		// lazygit's options view: the app's hint bar text (optionsText).
		return []string{a.optionsText()}, -1
	case "appStatus":
		// lazygit's layout leaves the appStatus window out while there is no status.
		v.Visible = a.busy != ""
		return []string{a.busy}, -1
	case "menu":
		if m := d.overlay(func(o overlay) bool { _, ok := o.(*menuOverlay); return ok }); m != nil {
			m := m.(*menuOverlay)
			v.Title = m.title
			// lazygit's menu view: the prompt's lines and a blank line
			// (MenuViewModel.GetNonModelItems), then the items, each with its
			// label columns.
			var lines []string
			if m.body != "" {
				lines = append(strings.Split(m.body, "\n"), "")
			}
			offset := len(lines)
			items := m.visible()
			// lazygit's RenderDisplayStrings over [label, columns...]: every
			// cell but a row's last is padded to its column's widest cell.
			// Only menus with columns; every row's cells count.
			var widths []int
			if slices.ContainsFunc(items, func(it menuItem) bool { return len(it.columns) > 0 }) {
				for _, it := range items {
					for i, c := range append([]string{it.label}, it.columns...) {
						if i >= len(widths) {
							widths = append(widths, 0)
						}
						widths[i] = max(widths[i], utils.StringWidth(c))
					}
				}
			}
			// lazygit's section headers (GetNonModelItems): "─── <title>"
			// before each new section, a blank line before all but the first;
			// none while a fuzzy filter is on.
			// The key column (first in RenderDisplayStrings) is padded to its
			// widest key, unless no item has a key (an empty column is left out).
			keyWidth := 0
			for _, it := range items {
				keyWidth = max(keyWidth, utils.StringWidth(it.key))
			}
			// Then the checkmark column (MenuWidget), likewise.
			widgetWidth := 0
			for _, it := range items {
				widgetWidth = max(widgetWidth, utils.StringWidth(it.widget))
			}
			keyCell := func(k string) string {
				if keyWidth == 0 {
					return ""
				}
				return utils.WithPadding(k, keyWidth, utils.AlignLeft) + " "
			}
			widgetCell := func(w string) string {
				if widgetWidth == 0 {
					return ""
				}
				return utils.WithPadding(w, widgetWidth, utils.AlignLeft) + " "
			}
			headers := !(m.filter != "" && cfg.Gui.UseFuzzySearch())
			prevSection, emitted := "", false
			d.menuItemLine = d.menuItemLine[:0]
			d.menuItemTop = d.menuItemTop[:0]
			for _, it := range items {
				top := len(lines)
				if headers && it.section != "" && it.section != prevSection {
					if emitted {
						lines = append(lines, "")
					}
					lines = append(lines, "─── "+it.section)
					prevSection, emitted = it.section, true
				}
				d.menuItemTop = append(d.menuItemTop, top)
				d.menuItemLine = append(d.menuItemLine, len(lines))
				line := strings.TrimRight(keyCell(it.key)+widgetCell(it.widget)+it.label, " ")
				if len(it.columns) > 0 {
					cols := append([]string{it.label}, it.columns...)
					for i := range cols[:len(cols)-1] {
						cols[i] = utils.WithPadding(cols[i], widths[i], utils.AlignLeft)
					}
					line = strings.TrimRight(keyCell(it.key)+widgetCell(it.widget)+strings.Join(cols, " "), " ")
				}
				if it.detail != "" {
					line += " " + it.detail
				}
				lines = append(lines, line)
			}
			_ = offset
			if m.cursor >= 0 && m.cursor < len(d.menuItemLine) {
				return lines, d.menuItemLine[m.cursor]
			}
			return lines, m.cursor + offset
		}
	case "menuFilter":
		// lazygit's menu filter view: shown while the menu's filter is.
		if m, ok := d.overlay(func(o overlay) bool { _, ok := o.(*menuOverlay); return ok }).(*menuOverlay); ok && (m.filtering || m.filter != "") {
			return []string{m.filter}, -1
		}
		v.Visible = false
		return nil, -1
	case "tooltip":
		// lazygit's TooltipForMenuItem: the selected item's tooltip, then
		// its disabled reason after DisabledMenuItemPrefix.
		if m, ok := d.overlay(func(o overlay) bool { _, ok := o.(*menuOverlay); return ok }).(*menuOverlay); ok {
			items := m.visible()
			if m.cursor < 0 || m.cursor >= len(items) {
				v.Visible = false
				return []string{""}, -1
			}
			tooltip := items[m.cursor].tooltip
			if it := items[m.cursor]; it.disabled != "" {
				if tooltip != "" {
					tooltip += "\n\n"
				}
				tooltip += a.repo.Common.Tr.DisabledMenuItemPrefix + it.disabled
			}
			// lazygit's layout: visible only with a menu and some text.
			v.Visible = tooltip != ""
			return strings.Split(tooltip, "\n"), -1
		}
		v.Visible = false
	case "confirmation":
		switch o := d.overlay(func(o overlay) bool {
			switch o.(type) {
			case *confirmOverlay, *errorOverlay:
				return true
			}
			return false
		}).(type) {
		case *confirmOverlay:
			v.Title = o.title
			return strings.Split(o.body, "\n"), -1
		case *errorOverlay:
			v.Title = "Error"
			return strings.Split(strings.TrimSpace(o.body), "\n"), -1
		}
	case "search":
		text, vis := a.searchStatus()
		v.Visible = vis
		return []string{text}, -1
	case "suggestions":
		if o, ok := a.topOverlay().(*promptOverlay); ok && o.suggestions != nil {
			items := o.matches()
			v.Visible = true
			if o.suggestionsFocused {
				return items, o.suggestionCursor
			}
			return items, -1
		}
		v.Visible = false
		return nil, -1
	case "prompt":
		if o, ok := d.overlay(func(o overlay) bool { _, ok := o.(*promptOverlay); return ok }).(*promptOverlay); ok {
			v.Title = o.title
			return []string{o.value}, -1
		}
	case "commitMessage", "commitDescription":
		if o, ok := d.overlay(func(o overlay) bool { _, ok := o.(*commitOverlay); return ok }).(*commitOverlay); ok {
			v.Title = o.title
			if name == "commitMessage" {
				return []string{o.draft.summary}, -1
			}
			return strings.Split(o.wrappedDescription(d.a), "\n"), -1
		}
	}
	return nil, -1
}

func (d *parityDriver) overlay(match func(overlay) bool) overlay {
	for i := len(d.a.overlays) - 1; i >= 0; i-- {
		if match(d.a.overlays[i]) {
			return d.a.overlays[i]
		}
	}
	return nil
}

func (d *parityDriver) stagingLines(st *stagingState) []string {
	lines := make([]string, len(st.lines))
	for i, l := range st.lines {
		lines[i] = l.Content
	}
	return lines
}

// subCommitLines renders the commits of the ref a sub-commits view was opened
// for with lazygit's commit presentation, one line per commit, as the commits
// projection does. The native rows draw no graph, branch-head markers or
// divergence headers, so those are left out here too.
func (d *parityDriver) subCommitLines(b *refBrowseState, sel int) ([]string, int) {
	a := d.a
	cfg := a.repo.Config
	if _, _, ok := b.divergenceHeaders(); len(b.commits) == 0 && !ok {
		return nil, sel
	}
	// The hash pointer from the pool (the graph compares addresses), and only
	// while the view is focused, as SubCommitsContext.
	var selected *string
	if sel >= 0 && sel < len(b.commits) && d.focusedView() == "subCommits" {
		selected = b.commits[sel].HashPtr()
	}
	// lazygit's SubCommitsContext: branch heads unless opened from a reflog
	// entry, the graph as configured, and the Local/Remote headers while
	// showing a divergence — as the native rows draw them.
	var branches []*models.Branch
	if b.showBranchHeads {
		branches = a.snap.Branches
	}
	symbols := graph.BoxDrawingSymbols
	if cfg.Gui.CommitGraphStyle == "detailed" {
		symbols = graph.BranchDrawingSymbols
	}
	var rows []string
	if len(b.commits) > 0 {
		rows = render(presentation.GetCommitListDisplayStrings(a.repo.Common, b.commits, branches, b.ref.RefName(), a.snap.RebaseUpdateRefs, a.enlarged(),
			set.New[string](), "", "", cfg.Gui.TimeFormat, cfg.Gui.ShortTimeFormat, time.Now(), cfg.Git.ParseEmoji, selected,
			0, len(b.commits), cfg.Git.Log.ShowGraph == "always", symbols, git_commands.NewNullBisectInfo()))
	}
	remoteAt, localAt, ok := b.divergenceHeaders()
	if !ok {
		return rows, sel
	}
	tr := a.repo.Common.Tr
	var out []string
	viewSel := sel
	for i := 0; i <= len(rows); i++ {
		for _, h := range []struct {
			at   int
			text string
		}{{remoteAt, tr.DivergenceSectionHeaderRemote}, {localAt, tr.DivergenceSectionHeaderLocal}} {
			if h.at == i {
				out = append(out, "─── "+h.text)
				if i <= sel {
					viewSel++
				}
			}
		}
		if i < len(rows) {
			out = append(out, rows[i])
		}
	}
	return out, viewSel
}

// commitLines renders the commits list the way lazygit does, including the
// section headers it puts around rebase / cherry-pick todos, and maps the
// selected commit to its line.
func (d *parityDriver) commitLines(sel int) ([]string, int) {
	a := d.a
	s := a.snap
	cfg := a.repo.Config
	tr := a.repo.Common.Tr
	if len(s.Commits) == 0 {
		return nil, sel
	}
	type header = commitHeader
	var headers []header
	if st := a.repo.Git.Status.WorkingTreeState(); st.CanShowTodos() {
		if st.Rebasing {
			headers = append(headers, header{at: 0, text: tr.PendingRebaseTodosSectionHeader})
		}
		if st.CherryPicking || st.Reverting {
			at := lo.IndexOf(lo.Map(s.Commits, func(c *models.Commit, _ int) bool {
				return c.Status == models.StatusCherryPickingOrReverting || c.Status == models.StatusConflicted
			}), true)
			label := lo.Ternary(st.CherryPicking, tr.PendingCherryPicksSectionHeader, tr.PendingRevertsSectionHeader)
			headers = append(headers, header{at: max(at, 0), text: label})
		}
		// lazygit's drop indicator goes after the other headers at its index
		// and before the Commits header (getNonModelItems).
		headers = append(headers, d.dropHeader()...)
		at := lo.IndexOf(lo.Map(s.Commits, func(c *models.Commit, _ int) bool { return !c.IsTODO() }), true)
		headers = append(headers, header{at: max(at, 0), text: tr.CommitsSectionHeader})
	} else {
		headers = append(headers, d.dropHeader()...)
	}
	head := s.CheckedOutBranch // lazygit's Model.CheckedOutBranch
	picked := set.New[string]()
	for _, c := range a.copiedCommits {
		picked.Add(c.Hash())
	}
	// The hash pointer from the pool (the graph compares addresses), and only
	// while the commits view is focused, as LocalCommitsContext.
	var selected *string
	if c := a.selectedCommit(); c != nil && d.focusedView() == "commits" {
		selected = c.HashPtr()
	}
	symbols := graph.BoxDrawingSymbols
	if cfg.Gui.CommitGraphStyle == "detailed" {
		symbols = graph.BranchDrawingSymbols
	}
	rows := render(presentation.GetCommitListDisplayStrings(a.repo.Common, s.Commits, s.Branches, head, s.RebaseUpdateRefs, a.enlarged(),
		picked, "", a.markedBaseCommit, cfg.Gui.TimeFormat, cfg.Gui.ShortTimeFormat, time.Now(), cfg.Git.ParseEmoji, selected,
		0, len(s.Commits), cfg.Git.Log.ShowGraph == "always", symbols, a.bisectInfo()))
	if len(headers) == 0 {
		return rows, sel
	}
	slices.SortStableFunc(headers, func(x, y header) int { return x.at - y.at })
	out := make([]string, 0, len(rows)+len(headers))
	viewSel := sel
	line := func(h header) string {
		if h.drop {
			return "━━━━━━ " + h.text + " ━━━━━━"
		}
		return "─── " + h.text
	}
	for i, row := range rows {
		for _, h := range headers {
			if h.at == i {
				out = append(out, line(h))
				if i <= sel {
					viewSel++
				}
			}
		}
		out = append(out, row)
	}
	for _, h := range headers {
		if h.at >= len(rows) {
			out = append(out, line(h))
		}
	}
	return out, viewSel
}

// Mouse. lazygit's tests point at terminal cells computed from a view's
// geometry (offset + 1 + column/row inside it). The projected views are
// placed one per band of mouseBand rows, so a cell decodes to a view, a row
// of its viewport and a column; the row plus the view's origin is a line of
// the projection. App.MouseCell answers where the app's frame draws that
// line (the element's real bounds) and column, and the pointer events go
// to that point through the Tester like a user's mouse: the app's own hit
// testing decides what is under it.
const mouseBand = 10000

var mouseViews = []string{"status", "files", "worktrees", "submodules", "localBranches", "remotes", "tags", "remoteBranches",
	"subCommits", "commits", "reflogCommits", "commitFiles", "stash", "main", "secondary", "information", "options",
	"appStatus", "menu", "confirmation", "prompt", "suggestions", "search", "mergeConflicts", "commitMessage",
	"commitDescription", "menuFilter", "extras"}

// point is the window point of terminal cell (x, y), and a label for it.
func (d *parityDriver) point(x, y int) (float32, float32, string) {
	band := y / mouseBand
	if band < 1 || band > len(mouseViews) {
		unsupported(fmt.Sprintf("mouse at cell %d,%d outside every view", x, y))
	}
	name := mouseViews[band-1]
	v := d.View(name)
	ox, oy, _, _ := v.Dimensions()
	line, col := v.OriginY()+y-oy-1, v.OriginX()+x-ox-1
	sub := 0
	if v.Wrap {
		if bl, ok := v.BufferLineForViewLine(line); ok {
			first, _ := v.ViewLineForBufferLine(bl)
			line, sub = bl, line-first
		}
	}
	px, py, ok := d.a.MouseCell(name, line, sub, col)
	if !ok {
		unsupported(fmt.Sprintf("mouse in view %s, which the app doesn't draw", name))
	}
	return px, py, fmt.Sprintf("%s line %d col %d", name, line, col)
}

// click presses and releases at cell (x, y) count times in quick
// succession (2: a double click), holding mods. A click starts after the
// double-click time, so separate clicks never count as one.
func (d *parityDriver) click(x, y int, mods ui.Modifiers, count int) {
	d.keys++
	d.pad = nil
	d.tt.Advance(time.Second)
	d.settle()
	px, py, label := d.point(x, y)
	what := map[int]string{1: "click", 2: "double-click"}[count]
	d.lastKey = what + " at " + label
	for range count {
		d.tt.ClickAtWith(mods, px, py)
	}
	d.settle()
}

// commitHeader is a non-model line of the commits view at model index at:
// a section header, or lazygit's drop indicator (drop).
type commitHeader struct {
	at   int
	text string
	drop bool
}

// dropHeader is the drop indicator of the app's commit drag, at its
// insertion index (LocalCommitsContext.SetDropInsertionIndex), as
// addCommitDropIndicator in getNonModelItems builds it: after the other
// headers at its index, before the Commits header, and after the last row
// when the index is len(commits).
func (d *parityDriver) dropHeader() []commitHeader {
	if dr := d.a.drag; dr != nil && dr.commit != nil && dr.commit.ins >= 0 {
		return []commitHeader{{at: dr.commit.ins, text: d.a.repo.Common.Tr.MoveCommitsHere, drop: true}}
	}
	return nil
}

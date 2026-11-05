package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
	a  *App
	tt *ui.Tester
	// lastKey is the last key pressed and the view it was pressed in, so a
	// failure can say what the app didn't do.
	lastKey string
	// toasts are status-bar messages the app set, standing in for lazygit's
	// toasts; status is the last one seen.
	toasts []string
	status string
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
}

var lineCount = regexp.MustCompile(`lines in view '(\w+)'\. Expected (?:equals|at least) (\d+)`)

// parityFailure is what Fail panics with; the runner recovers it.
type parityFailure struct{ msg, after, screen string }

// unsupported fails a test on something the harness can't express, so the
// report can tell those apart from real behaviour gaps.
func unsupported(what string) { panic(parityFailure{msg: "harness: " + what + " not supported"}) }

func (d *parityDriver) settle() {
	d.tt.Frame()
	d.tt.Frame()
	if d.a.status != d.status {
		d.status = d.a.status
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

func (d *parityDriver) Click(int, int) { unsupported("terminal-cell mouse click") }
func (d *parityDriver) ClickWithModifier(int, int, gocui.Modifier) {
	unsupported("terminal-cell mouse click")
}
func (d *parityDriver) DoubleClick(int, int)          { unsupported("terminal-cell mouse click") }
func (d *parityDriver) ClickAndHold(int, int)         { unsupported("terminal-cell mouse drag") }
func (d *parityDriver) MouseMove(int, int)            { unsupported("terminal-cell mouse drag") }
func (d *parityDriver) MouseRelease(int, int)         { unsupported("terminal-cell mouse drag") }
func (d *parityDriver) ScrollWheelDown(int, int)      { unsupported("terminal-cell mouse wheel") }
func (d *parityDriver) FocusInAndClick(int, int)      { unsupported("terminal-cell mouse click") }
func (d *parityDriver) RefreshInBackground()          { d.a.Refresh(); d.settle() }
func (d *parityDriver) OnUIThreadAndWait(f func())    { f() }
func (d *parityDriver) FocusIn()                      { d.a.Refresh(); d.settle() }
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
func (d *parityDriver) PretendMergeOrRebaseStartedInLazygit()     {}
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
	return &parityContext{view: d.View(name), window: windowOf(name)}
}

// windowOf is lazygit's window for a view: tabs share their first view's.
func windowOf(view string) string {
	switch view {
	case "files", "worktrees", "submodules":
		return "files"
	case "localBranches", "remotes", "tags", "remoteBranches":
		return "branches"
	case "commits", "reflogCommits", "commitFiles", "subCommits":
		return "commits"
	case "mergeConflicts":
		return "main"
	}
	return view
}

func (d *parityDriver) topView(window string) string {
	a := d.a
	switch window {
	case "files":
		return [...]string{"files", "worktrees", "submodules"}[a.fileTab]
	case "branches":
		if b := a.refBrowse; b != nil && b.ref == nil {
			return "remoteBranches"
		}
		return [...]string{"localBranches", "remotes", "tags"}[a.branchTab]
	case "commits":
		if a.commitFiles != nil {
			return "commitFiles"
		}
		return "commits"
	}
	return window
}

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
		return "prompt"
	case *searchOverlay:
		return "search"
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
	switch a.focus {
	case panelStatus:
		return "status"
	case panelFiles:
		return d.topView("files")
	case panelBranches:
		return d.topView("branches")
	case panelCommits:
		return d.topView("commits")
	case panelStash:
		return "stash"
	}
	return "unknown"
}

// View builds a fresh gocui view holding the projection of view name.
func (d *parityDriver) View(name string) *gocui.View {
	v := gocui.NewView(name, 0, 0, 400, 2000, gocui.OutputTrue)
	lines, sel := d.project(v, name)
	for len(lines) < d.pad[name] {
		lines = append(lines, " ") // a trailing empty line would be dropped
	}
	v.SetContent(strings.Join(lines, "\n"))
	if sel >= 0 {
		v.Highlight = true
		v.SetCursor(0, sel)
	}
	return v
}

func (d *parityDriver) cursorIf(p panel, active bool) int {
	if !active {
		return 0
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
		if s.Head == nil {
			return []string{filepath.Base(s.Root)}, 0
		}
		return []string{presentation.FormatStatus(filepath.Base(s.Root), s.Head, types.ItemOperationNone, "",
			a.repo.Git.Status.WorkingTreeState(), tr, cfg)}, 0
	case "files":
		// The app keeps the Files rows in lazygit's tree view model; render
		// them with lazygit's code, as its working-tree context does.
		showIcons := icons.IsIconEnabled() && cfg.Gui.ShowFileIcons
		return presentation.RenderFileTree(a.files, s.Submodules, showIcons, cfg.Gui.ShowNumstatInFilesView,
			&cfg.Gui.CustomIcons, cfg.Gui.ShowRootItemInFileTree, tr), d.cursorIf(panelFiles, a.fileTab == 0)
	case "worktrees":
		return render(presentation.GetWorktreeDisplayStrings(tr, s.Worktrees)), d.cursorIf(panelFiles, a.fileTab == 1)
	case "submodules":
		return render(presentation.GetSubmoduleListDisplayStrings(s.Submodules)), d.cursorIf(panelFiles, a.fileTab == 2)
	case "localBranches":
		return render(presentation.GetBranchListDisplayStrings(s.Branches, noOp, a.prs, false, "", 400, tr, cfg, s.Worktrees)),
			d.cursorIf(panelBranches, a.branchTab == 0)
	case "remotes":
		return render(presentation.GetRemoteListDisplayStrings(s.Remotes, "", noOp, tr, cfg)), d.cursorIf(panelBranches, a.branchTab == 1)
	case "remoteBranches":
		if b := a.refBrowse; b != nil && b.ref == nil {
			return render(presentation.GetRemoteBranchListDisplayStrings(b.branches, "")), d.cursorIf(panelBranches, true)
		}
		return nil, -1
	case "tags":
		return render(presentation.GetTagListDisplayStrings(s.Tags, noOp, "", tr, cfg)), d.cursorIf(panelBranches, a.branchTab == 2)
	case "commits":
		return d.commitLines(d.cursorIf(panelCommits, a.commitFiles == nil))
	case "commitFiles":
		cf := a.commitFiles
		if cf == nil {
			return nil, -1
		}
		showIcons := icons.IsIconEnabled() && cfg.Gui.ShowFileIcons
		return presentation.RenderCommitFileTree(cf.tree, a.repo.Git.Patch.PatchBuilder, showIcons, &cfg.Gui.CustomIcons),
			a.cursor[panelCommits]
	case "stash":
		return render(presentation.GetStashEntryListDisplayStrings(s.Stash, "")), a.cursor[panelStash]
	case "main":
		if st := a.staging; st != nil && !st.staged {
			return d.stagingLines(), st.cursor
		}
		// The raw text the pane was loaded from: parsing for display moves
		// headers (commit message, file names) into the pane's chrome.
		if _, load := a.mainSource(); load != nil {
			return strings.Split(strings.TrimRight(load(), "\n"), "\n"), -1
		}
		return nil, -1
	case "secondary":
		if st := a.staging; st != nil && st.staged {
			return d.stagingLines(), st.cursor
		}
	case "appStatus":
		return []string{a.busy}, -1
	case "menu":
		if m := d.overlay(func(o overlay) bool { _, ok := o.(*menuOverlay); return ok }); m != nil {
			m := m.(*menuOverlay)
			v.Title = m.title
			items := m.visible()
			lines := make([]string, len(items))
			for i, it := range items {
				lines[i] = strings.TrimSpace(it.key + " " + it.label)
			}
			return lines, m.cursor
		}
	case "tooltip":
		// lazygit's TooltipForMenuItem: the selected item's tooltip, then
		// its disabled reason after DisabledMenuItemPrefix.
		if m, ok := d.overlay(func(o overlay) bool { _, ok := o.(*menuOverlay); return ok }).(*menuOverlay); ok {
			items := m.visible()
			if m.cursor < 0 || m.cursor >= len(items) || items[m.cursor].disabled == "" {
				return []string{""}, -1
			}
			return strings.Split(a.repo.Common.Tr.DisabledMenuItemPrefix+items[m.cursor].disabled, "\n"), -1
		}
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
			return strings.Split(o.draft.description, "\n"), -1
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

func (d *parityDriver) stagingLines() []string {
	st := d.a.staging
	lines := make([]string, len(st.lines))
	for i, l := range st.lines {
		lines[i] = l.Content
	}
	return lines
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
	type header struct {
		at   int
		text string
	}
	var headers []header
	if st := a.repo.Git.Status.WorkingTreeState(); st.CanShowTodos() {
		if st.Rebasing {
			headers = append(headers, header{0, tr.PendingRebaseTodosSectionHeader})
		}
		if st.CherryPicking || st.Reverting {
			at := lo.IndexOf(lo.Map(s.Commits, func(c *models.Commit, _ int) bool {
				return c.Status == models.StatusCherryPickingOrReverting || c.Status == models.StatusConflicted
			}), true)
			label := lo.Ternary(st.CherryPicking, tr.PendingCherryPicksSectionHeader, tr.PendingRevertsSectionHeader)
			headers = append(headers, header{max(at, 0), label})
		}
		at := lo.IndexOf(lo.Map(s.Commits, func(c *models.Commit, _ int) bool { return !c.IsTODO() }), true)
		headers = append(headers, header{max(at, 0), tr.CommitsSectionHeader})
	}
	head := ""
	if s.Head != nil {
		head = s.Head.Name
	}
	picked := set.New[string]()
	for _, c := range a.copiedCommits {
		picked.Add(c.Hash())
	}
	var selected *string
	if c := a.selectedCommit(); c != nil {
		h := c.Hash()
		selected = &h
	}
	symbols := graph.BoxDrawingSymbols
	if cfg.Gui.CommitGraphStyle == "detailed" {
		symbols = graph.BranchDrawingSymbols
	}
	rows := render(presentation.GetCommitListDisplayStrings(a.repo.Common, s.Commits, s.Branches, head, false, false,
		picked, "", "", cfg.Gui.TimeFormat, cfg.Gui.ShortTimeFormat, time.Now(), cfg.Git.ParseEmoji, selected,
		0, len(s.Commits), cfg.Git.Log.ShowGraph == "always", symbols, git_commands.NewNullBisectInfo()))
	if len(headers) == 0 {
		return rows, sel
	}
	out := make([]string, 0, len(rows)+len(headers))
	viewSel := sel
	for i, row := range rows {
		for _, h := range headers {
			if h.at == i {
				out = append(out, "─── "+h.text)
				if i <= sel {
					viewSel++
				}
			}
		}
		out = append(out, row)
	}
	return out, viewSel
}

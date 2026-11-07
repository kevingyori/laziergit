package app

import (
	"slices"

	"github.com/jesseduffield/lazygit/pkg/config"
)

// sideView is one name of lazygit's gui.sidePanels: a list that stands alone
// as a side panel or shares one with others as its tabs.
type sideView int

const (
	vStatus sideView = iota
	vFiles
	vWorktrees
	vSubmodules
	vBranches
	vRemotes
	vTags
	vCommits
	vReflog
	vStash
	sideViewCount
)

// maxWindows bounds the side windows: every name in its own panel.
const maxWindows = int(sideViewCount)

// sideViewNames are config.ValidSidePanelTabs, in sideView order.
var sideViewNames = [sideViewCount]string{"status", "files", "worktrees", "submodules", "branches", "remotes", "tags", "commits", "reflog", "stash"}

// sideViewGroups is where each name lives in the app's state: the panel
// whose tab variable selects it, and that tab.
var sideViewGroups = [sideViewCount]struct {
	panel panel
	tab   int
}{
	{panelStatus, 0}, {panelFiles, 0}, {panelFiles, 1}, {panelFiles, 2},
	{panelBranches, 0}, {panelBranches, 1}, {panelBranches, 2},
	{panelCommits, 0}, {panelCommits, 1}, {panelStash, 0},
}

func (v sideView) group() panel { return sideViewGroups[v].panel }
func (v sideView) tab() int     { return sideViewGroups[v].tab }

// title is lazygit's sidePanelTabTitles, in the app's words for its tabs.
func (v sideView) title() string {
	return [sideViewCount]string{"Status", "Files", "Worktrees", "Submodules", "Branches", "Remotes", "Tags", "Commits", "Reflog", "Stash"}[v]
}

func viewOf(p panel, tab int) sideView {
	for v := range sideViewCount {
		if sideViewGroups[v].panel == p && sideViewGroups[v].tab == tab {
			return v
		}
	}
	return vStatus
}

// sideWindow is one configured side panel: its tabs and the one it shows.
type sideWindow struct {
	views  []sideView
	active sideView
}

// name is lazygit's window name: the panel's first tab.
func (w sideWindow) name() string { return sideViewNames[w.views[0]] }

// sidePanelConfig is gui.sidePanels as views; lazygit's default when the
// config is missing or doesn't validate.
func sidePanelConfig(cfg *config.UserConfig) [][]sideView {
	var out [][]sideView
	seen := map[sideView]bool{}
	for _, panel := range cfg.Gui.SidePanels {
		var views []sideView
		for _, name := range panel {
			i := slices.Index(sideViewNames[:], name)
			if i < 0 || seen[sideView(i)] {
				return defaultSidePanels()
			}
			seen[sideView(i)] = true
			views = append(views, sideView(i))
		}
		if len(views) == 0 {
			return defaultSidePanels()
		}
		out = append(out, views)
	}
	if !seen[vFiles] || !seen[vBranches] || !seen[vCommits] {
		return defaultSidePanels()
	}
	return out
}

func defaultSidePanels() [][]sideView {
	return [][]sideView{{vStatus}, {vFiles, vWorktrees, vSubmodules}, {vBranches, vRemotes, vTags}, {vCommits, vReflog}, {vStash}}
}

// groupTab is the tab the panel's state shows now.
func (a *App) groupTab(p panel) int {
	switch p {
	case panelFiles:
		return a.fileTab
	case panelBranches:
		return a.branchTab
	case panelCommits:
		return a.commitTab
	}
	return 0
}

func (a *App) curView(p panel) sideView { return viewOf(p, a.groupTab(p)) }

// focusedSideView is lazygit's current side context, as a gui.sidePanels name.
func (a *App) focusedSideView() sideView { return a.curView(a.focus) }

// applySidePanels is lazygit's applySidePanelConfig and reloadSidePanels:
// the windows come from the config, each showing its first tab, except that
// the focused view stays focused and shown; a focused view the config hides
// gives way to the files panel.
func (a *App) applySidePanels() {
	panels := sidePanelConfig(a.repo.Config)
	a.windows = a.windows[:0]
	for _, views := range panels {
		a.windows = append(a.windows, sideWindow{views: views, active: views[0]})
	}
	focused := a.focusedSideView()
	if a.windowOf(focused) < 0 {
		a.commitFiles, a.refBrowse = nil, nil
		a.staging, a.mainFocused = nil, false
		focused = vFiles
	}
	// Each group shows the first tab of a window it is in, and the focused
	// group its focused view.
	for i := len(a.windows) - 1; i >= 0; i-- {
		v := a.windows[i].views[0]
		if v.group() != focused.group() {
			a.setGroupTab(v.group(), v.tab())
		}
	}
	a.setGroupTab(focused.group(), focused.tab())
	a.focus = focused.group()
	a.syncWindows()
	a.diffKey = ""
}

// windowOf is the index of the window v is a tab of, -1 when hidden.
func (a *App) windowOf(v sideView) int {
	for i, w := range a.windows {
		if slices.Contains(w.views, v) {
			return i
		}
	}
	return -1
}

// syncWindows makes the focused window show the focused view.
func (a *App) syncWindows() {
	if len(a.windows) == 0 {
		return
	}
	if i := a.windowOf(a.focusedSideView()); i >= 0 {
		a.windows[i].active = a.focusedSideView()
	}
}

// focusWindow is lazygit's CurrentSideWindow, -1 if the focus is hidden.
func (a *App) focusWindow() int { return a.windowOf(a.focusedSideView()) }

// activeView is the tab window i shows: its group's current tab when that is
// one of the window's, or the tab it showed last.
func (a *App) activeView(i int) sideView {
	a.syncWindows()
	w := a.windows[i]
	if cur := a.curView(w.active.group()); slices.Contains(w.views, cur) {
		return cur
	}
	return w.active
}

// setGroupTab moves panel p's state to tab, each tab keeping its selection,
// range and scroll position.
func (a *App) setGroupTab(p panel, tab int) {
	old := a.groupTab(p)
	if old == tab {
		return
	}
	a.viewLists[viewOf(p, old)] = a.lists[p]
	a.lists[p] = a.viewLists[viewOf(p, tab)]
	switch p {
	case panelFiles:
		a.tabCursor[0][old] = a.cursor[p]
		a.fileTab = tab
		a.cursor[p] = clamp(a.tabCursor[0][tab], 0, max(a.count(p)-1, 0))
	case panelBranches:
		if a.browse(p) != nil {
			a.refBrowse = nil // a drill-down is not a tab: nothing to keep
		} else {
			a.tabCursor[1][old] = a.cursor[p]
			a.tabRange[old] = a.ranges[p]
		}
		a.branchTab = tab
		a.ranges[p] = a.tabRange[tab]
		a.cursor[p] = clamp(a.tabCursor[1][tab], 0, max(a.count(p)-1, 0))
	case panelCommits:
		if a.commitFiles != nil || a.browse(panelCommits) != nil {
			a.commitFiles = nil
			if a.browse(panelCommits) != nil {
				a.refBrowse = nil
			}
		} else {
			a.commitTabCursor[old] = a.cursor[p]
		}
		a.commitTab = tab
		a.cursor[p] = clamp(a.commitTabCursor[tab], 0, max(a.count(p)-1, 0))
	}
	a.lists[p].ScrollIntoView(a.cursor[p])
}

// focusView focuses v in its window (lazygit's Push of its context).
func (a *App) focusView(v sideView) {
	a.setGroupTab(v.group(), v.tab())
	a.focusPanel(v.group())
	a.syncWindows()
}

// cycleWindow is lazygit's next / previous side window, skipping panels the
// config hides; it lands on what the window shows, drill-downs included.
func (a *App) cycleWindow(d int) {
	n := len(a.windows)
	i := a.focusWindow()
	if i < 0 {
		i = 0
	} else {
		i = (i + d + n) % n
	}
	a.focusView(a.activeView(i))
}

// jumpWindow is lazygit's goToSideWindow without the tab switch.
func (a *App) jumpWindow(i int) {
	v := a.activeView(i)
	if v.group() == panelCommits && a.commitFiles != nil {
		a.cursor[panelCommits] = a.commitFiles.commitIdx
		a.commitFiles = nil
	}
	a.focusView(v)
}

// switchWindowTab is lazygit's next / previous tab: the focused window's tabs
// in their configured order. A drill-down is not a tab, so the keys leave it
// for the window's first tab.
func (a *App) switchWindowTab(delta int) {
	i := a.focusWindow()
	if i < 0 {
		return
	}
	w := a.windows[i]
	cur := a.focusedSideView()
	next := w.views[0]
	if !a.inDrillDown() {
		at := slices.Index(w.views, cur)
		next = w.views[(at+delta+len(w.views))%len(w.views)]
		if next == cur {
			return
		}
	}
	a.staging = nil
	a.mainFocused = false
	if a.inDrillDown() && next == cur {
		// setGroupTab keeps a drill-down on the same tab; leave it.
		switch next.group() {
		case panelCommits:
			a.commitFiles = nil
			a.refBrowse = nil
			a.cursor[panelCommits] = clamp(a.commitTabCursor[a.commitTab], 0, max(a.count(panelCommits)-1, 0))
		case panelBranches:
			a.refBrowse = nil
			a.cursor[panelBranches] = clamp(a.tabCursor[1][a.branchTab], 0, max(a.count(panelBranches)-1, 0))
		}
		a.lists[cur.group()].ScrollIntoView(a.cursor[cur.group()])
	}
	a.focusView(next)
}

// inDrillDown says the focused window shows a transient view (sub-commits,
// remote branches, commit files) over its tab.
func (a *App) inDrillDown() bool {
	switch a.focus {
	case panelCommits:
		return a.commitFiles != nil || a.browse(panelCommits) != nil
	case panelBranches:
		return a.browse(panelBranches) != nil
	}
	return false
}

// withView runs draw with panel state showing v, for a window showing a tab
// of a group whose current tab is in another window, then puts the state
// back. Clicks inside are recorded (deferredClick) and applied after.
func (a *App) withView(v sideView, draw func()) {
	p := v.group()
	if a.curView(p) == v {
		draw()
		return
	}
	fileTab, branchTab, commitTab := a.fileTab, a.branchTab, a.commitTab
	cursor, rng, list := a.cursor[p], a.ranges[p], a.lists[p]
	refBrowse, commitFiles := a.refBrowse, a.commitFiles
	switch p {
	case panelFiles:
		a.fileTab = v.tab()
		a.cursor[p] = a.tabCursor[0][v.tab()]
	case panelBranches:
		a.branchTab = v.tab()
		a.cursor[p] = a.tabCursor[1][v.tab()]
		a.ranges[p] = a.tabRange[v.tab()]
		if a.browse(p) != nil {
			a.refBrowse = nil
		}
	case panelCommits:
		a.commitTab = v.tab()
		a.cursor[p] = a.commitTabCursor[v.tab()]
		a.commitFiles = nil
		if a.browse(p) != nil {
			a.refBrowse = nil
		}
	}
	if p != panelBranches {
		a.ranges[p] = listRange{}
	}
	a.cursor[p] = clamp(a.cursor[p], 0, max(a.count(p)-1, 0))
	a.lists[p] = a.viewLists[v]
	a.drawSwapped = true
	draw()
	a.drawSwapped = false
	a.viewLists[v] = a.lists[p]
	a.fileTab, a.branchTab, a.commitTab = fileTab, branchTab, commitTab
	a.cursor[p], a.ranges[p], a.lists[p] = cursor, rng, list
	a.refBrowse, a.commitFiles = refBrowse, commitFiles
	if c := a.deferredClick; c != nil {
		a.deferredClick = nil
		a.focusView(c.view)
		if c.row >= 0 {
			a.cursor[p] = clamp(c.row, 0, max(a.count(p)-1, 0))
			a.loadMain()
		}
	}
}

// sideClick is a click on a tab or row of a window drawn by withView.
type sideClick struct {
	view sideView
	row  int // -1 for the tab or title
}

// clickSide focuses v (and selects row), or defers that while withView has
// the panel state borrowed.
func (a *App) clickSide(v sideView, row int) {
	if a.drawSwapped {
		a.deferredClick = &sideClick{v, row}
		return
	}
	if a.curView(v.group()) != v || a.focus != v.group() || a.staging != nil || a.mainFocused {
		a.focusView(v)
	}
	if row >= 0 {
		a.cursor[v.group()] = row
		a.loadMain()
	}
}

// Lazygit view names, for the parity harness and lazygit's window names.
var sideViewLazygitNames = [sideViewCount]string{"status", "files", "worktrees", "submodules", "localBranches", "remotes", "tags", "commits", "reflogCommits", "stash"}

// SideWindowOf is lazygit's window name of a view (its context's
// GetWindowName); transient views take the window they are drilled into
// from. Hidden views have a window of their own name.
func (a *App) SideWindowOf(view string) string {
	switch view {
	case "remoteBranches", "subCommits":
		if b := a.refBrowse; view == "subCommits" && b != nil && b.panel == panelCommits {
			return a.SideWindowOf("commits")
		}
		return a.SideWindowOf("localBranches")
	case "commitFiles":
		return a.SideWindowOf("commits")
	}
	i := slices.Index(sideViewLazygitNames[:], view)
	if i < 0 {
		return view
	}
	if w := a.windowOf(sideView(i)); w >= 0 {
		return a.windows[w].name()
	}
	return sideViewNames[i]
}

// SideTopView is lazygit's TopViewInWindow for a side window: the view it
// shows, drill-downs included. "" for a window that isn't a side window.
func (a *App) SideTopView(window string) string {
	for i, w := range a.windows {
		if w.name() != window {
			continue
		}
		v := a.activeView(i)
		if a.curView(v.group()) == v {
			switch v.group() {
			case panelBranches:
				if b := a.browse(panelBranches); b != nil {
					if b.ref == nil {
						return "remoteBranches"
					}
					return "subCommits"
				}
			case panelCommits:
				return [...]string{"commits", "reflogCommits", "subCommits", "commitFiles"}[a.commitsView()]
			}
		}
		return sideViewLazygitNames[v]
	}
	return ""
}

// FocusedSideView is lazygit's current side context's view name.
func (a *App) FocusedSideView() string {
	if i := a.focusWindow(); i >= 0 {
		return a.SideTopView(a.windows[i].name())
	}
	return sideViewLazygitNames[a.focusedSideView()]
}

// FocusIn is lazygit's focus handler: a changed config file is reloaded and
// its side panels applied (reloadSidePanels, resetKeybindings), then the repo
// refreshes.
func (a *App) FocusIn() {
	changed, err := a.repo.ReloadChangedConfig()
	if err != nil {
		a.showError("Config", err)
	} else if changed {
		a.applySidePanels()
		a.keys = loadBindings(a.repo)
		a.actions = a.buildActions()
	}
	a.Refresh()
}

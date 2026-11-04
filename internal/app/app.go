// Package app is laziergit's UI: lazygit's panels and keymap, drawn with
// MyGo's native UI.
package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/keymap"
	"laziergit/internal/repo"
)

type panel int

const (
	panelStatus panel = iota
	panelFiles
	panelBranches
	panelCommits
	panelStash
	panelCount
)

var panelTitles = [panelCount]string{"Status", "Files", "Branches", "Commits", "Stash"}

type bindings struct {
	quit, prev, next, prevBlock, nextBlock     keymap.Binding
	top, bottom, prevPage, nextPage, selectKey keymap.Binding
	refresh, scrollUpMain, scrollDownMain      keymap.Binding
	jump                                       []keymap.Binding
}

func loadBindings(r *repo.Repo) bindings {
	u := r.Config.Keybinding.Universal
	b := bindings{
		quit: keymap.Parse(u.Quit), prev: keymap.Parse(u.PrevItem), next: keymap.Parse(u.NextItem),
		prevBlock: keymap.Parse(u.PrevBlock), nextBlock: keymap.Parse(u.NextBlock),
		top: keymap.Parse(u.GotoTop), bottom: keymap.Parse(u.GotoBottom),
		prevPage: keymap.Parse(u.PrevPage), nextPage: keymap.Parse(u.NextPage),
		selectKey: keymap.Parse(u.Select), refresh: keymap.Parse(u.Refresh),
		scrollUpMain: keymap.Parse(u.ScrollUpMain), scrollDownMain: keymap.Parse(u.ScrollDownMain),
	}
	for _, kb := range u.JumpToBlock {
		b.jump = append(b.jump, keymap.Parse(kb))
	}
	return b
}

// App is the whole UI state. It is only touched on the main thread; git
// work runs through bg and comes back through post.
type App struct {
	repo *repo.Repo
	keys bindings

	snap   *repo.Snapshot
	focus  panel
	cursor [panelCount]int
	lists  [panelCount]ui.ListState

	diff     []diffLine
	diffList ui.ListState
	diffKey  string
	diffGen  int

	status string // transient error / info line

	post func(func()) // run on the main thread and redraw
	bg   func(func()) // run off the main thread
	quit func()
}

// New creates the app. post must run fn on the UI thread and redraw; bg runs
// fn in the background; quit closes the app.
func New(r *repo.Repo, post, bg func(func()), quit func()) *App {
	a := &App{repo: r, keys: loadBindings(r), focus: panelFiles, post: post, bg: bg, quit: quit}
	for i := range a.lists {
		a.lists[i].Selected = nil // we own selection and keys
	}
	return a
}

// Refresh reloads the snapshot in the background.
func (a *App) Refresh() {
	a.bg(func() {
		s := a.repo.Load()
		a.post(func() { a.apply(s) })
	})
}

func (a *App) apply(s *repo.Snapshot) {
	a.snap = s
	for p := range panelCount {
		a.cursor[p] = clamp(a.cursor[p], 0, a.count(p)-1)
	}
	if len(s.Errors) > 0 {
		a.status = s.Errors[0].Error()
	}
	a.diffKey = "" // content may have changed under the same selection
	a.loadMain()
}

func (a *App) count(p panel) int {
	if a.snap == nil {
		return 0
	}
	switch p {
	case panelStatus:
		return 1
	case panelFiles:
		return len(a.snap.Files)
	case panelBranches:
		return len(a.snap.Branches)
	case panelCommits:
		return len(a.snap.Commits)
	case panelStash:
		return len(a.snap.Stash)
	}
	return 0
}

func (a *App) selectedFile() *models.File {
	if a.count(panelFiles) == 0 {
		return nil
	}
	return a.snap.Files[a.cursor[panelFiles]]
}

// loadMain loads the main pane for the focused panel's selection, dropping
// results that arrive after the selection moved on.
func (a *App) loadMain() {
	if a.snap == nil {
		return
	}
	key, load := a.mainSource()
	if key == a.diffKey {
		return
	}
	a.diffKey = key
	a.diffGen++
	gen := a.diffGen
	if load == nil {
		a.diff = nil
		return
	}
	a.bg(func() {
		lines := parseDiff(load())
		a.post(func() {
			if gen == a.diffGen {
				a.diff = lines
				a.diffList.ScrollTo(0, ui.Start)
			}
		})
	})
}

func (a *App) mainSource() (string, func() string) {
	i := a.cursor[a.focus]
	if a.count(a.focus) == 0 {
		return "empty", nil
	}
	r := a.repo
	switch a.focus {
	case panelFiles:
		f := a.snap.Files[i]
		return "file:" + f.Path + ":" + f.ShortStatus, func() string { return r.FileDiff(f) }
	case panelBranches:
		b := a.snap.Branches[i]
		return "branch:" + b.Name, func() string { return r.BranchLog(b.Name) }
	case panelCommits:
		c := a.snap.Commits[i]
		return "commit:" + c.Hash(), func() string { return r.CommitDiff(c.Hash()) }
	case panelStash:
		s := a.snap.Stash[i]
		return "stash:" + s.Hash, func() string { return r.StashDiff(s.Index) }
	}
	return "status", nil
}

// handleInput is the single dispatcher for every key in the window, so the
// keymap behaves like lazygit's regardless of which widget the pointer last
// touched.
func (a *App) handleInput(ev ui.InputEvent) bool {
	ch, ok := keymap.FromEvent(ev)
	if !ok {
		return false
	}
	return a.dispatch(ch)
}

func (a *App) dispatch(ch keymap.Chord) bool {
	k := a.keys
	switch {
	case k.quit.Matches(ch):
		a.quit()
	case k.next.Matches(ch):
		a.move(1)
	case k.prev.Matches(ch):
		a.move(-1)
	case k.nextPage.Matches(ch):
		a.move(a.pageSize())
	case k.prevPage.Matches(ch):
		a.move(-a.pageSize())
	case k.top.Matches(ch):
		a.move(-1 << 30)
	case k.bottom.Matches(ch):
		a.move(1 << 30)
	case k.nextBlock.Matches(ch):
		a.focusPanel((a.focus + 1) % panelCount)
	case k.prevBlock.Matches(ch):
		a.focusPanel((a.focus + panelCount - 1) % panelCount)
	case k.refresh.Matches(ch):
		a.Refresh()
	case k.scrollDownMain.Matches(ch):
		a.scrollMain(3)
	case k.scrollUpMain.Matches(ch):
		a.scrollMain(-3)
	case k.selectKey.Matches(ch) && a.focus == panelFiles:
		a.toggleStaged()
	default:
		for i, b := range k.jump {
			if b.Matches(ch) && i < int(panelCount) {
				a.focusPanel(panel(i))
				return true
			}
		}
		return false
	}
	return true
}

func (a *App) pageSize() int {
	first, last := a.lists[a.focus].Visible()
	return max(1, last-first)
}

func (a *App) move(d int) {
	n := a.count(a.focus)
	if n == 0 {
		return
	}
	a.cursor[a.focus] = clamp(a.cursor[a.focus]+d, 0, n-1)
	a.lists[a.focus].ScrollIntoView(a.cursor[a.focus])
	a.loadMain()
}

func (a *App) focusPanel(p panel) {
	a.focus = p
	a.loadMain()
}

func (a *App) scrollMain(d int) {
	first, _ := a.diffList.Visible()
	a.diffList.ScrollTo(clamp(first+d, 0, max(0, len(a.diff)-1)), ui.Start)
}

func (a *App) toggleStaged() {
	f := a.selectedFile()
	if f == nil {
		return
	}
	a.bg(func() {
		err := a.repo.ToggleStaged(f)
		s := a.repo.Load()
		a.post(func() {
			if err != nil {
				a.status = err.Error()
			} else {
				a.status = ""
			}
			a.apply(s)
		})
	})
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return min(max(v, lo), hi)
}

type diffKind uint8

const (
	lineCtx diffKind = iota
	lineAdd
	lineDel
	lineHunk
	lineMeta
	lineFile // a file heading, replacing diff --git/index/---/+++
)

type diffLine struct {
	kind diffKind
	text string
}

func parseDiff(s string) []diffLine {
	lines := repo.SplitLines(s)
	out := make([]diffLine, 0, len(lines))
	inHunk := false
	for _, l := range lines {
		l = strings.ReplaceAll(l, "\t", "    ")
		k := lineCtx
		switch {
		case strings.HasPrefix(l, "diff --git"):
			inHunk = false
			out = append(out, diffLine{kind: lineFile, text: diffPath(l)})
			continue
		case strings.HasPrefix(l, "@@"):
			inHunk, k = true, lineHunk
		case !inHunk:
			switch {
			case strings.HasPrefix(l, "index "), strings.HasPrefix(l, "---"), strings.HasPrefix(l, "+++"):
				continue // redundant with the file heading
			case strings.HasPrefix(l, "new file"), strings.HasPrefix(l, "deleted file"),
				strings.HasPrefix(l, "similarity"), strings.HasPrefix(l, "rename "),
				strings.HasPrefix(l, "old mode"), strings.HasPrefix(l, "new mode"), strings.HasPrefix(l, "Binary"):
				k = lineMeta
			}
		case strings.HasPrefix(l, "+"):
			k = lineAdd
		case strings.HasPrefix(l, "-"):
			k = lineDel
		}
		out = append(out, diffLine{kind: k, text: l})
	}
	return out
}

// diffPath extracts the new path from a "diff --git a/x b/y" line.
func diffPath(l string) string {
	if i := strings.LastIndex(l, " b/"); i >= 0 {
		return l[i+3:]
	}
	return strings.TrimPrefix(l, "diff --git ")
}

// Package app is laziergit's UI: lazygit's panels and keymap, drawn with
// MyGo's native UI.
package app

import (
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/icons"
	"strings"
	"time"

	"github.com/jesseduffield/lazygit/pkg/gui/modes/cherrypicking"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/gui/context/traits"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"

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

// App is the whole UI state. It is only touched on the main thread; git
// work runs through bg and comes back through post.
type App struct {
	repo    *repo.Repo
	keys    bindings
	actions []action

	switchFocus string // context focused after a worktree switch (lazygit contextKey)

	snap *repo.Snapshot // what the panels show: raw with the list filters applied
	raw  *repo.Snapshot // the full snapshot
	// authors are the commit authors of every commit list loaded so far, by
	// email, as lazygit's Model().Authors: the suggestions for setting one.
	authors map[string]string
	srch    map[string]*searchCtx
	fidx    map[string][]int // per filtered list: row -> index in the full list
	focus   panel
	cursor  [panelCount]int
	lists   [panelCount]ui.ListState

	fileTab            int
	files              *filetree.FileTreeViewModel // the Files tab's rows (lazygit's tree)
	fileRows           []treeRow                   // how each of those rows is drawn
	parents            []string
	refBrowse          *refBrowseState
	filterSelectedHash string // lazygit's Filtering.selectedCommitHash
	branchTab          int
	commitTab          int                   // 0 commits, 1 reflog (lazygit's commits window tabs)
	commitTabCursor    [2]int                // each commits-window tab keeps its selection
	tabCursor          [2][3]int             // each files/branches tab keeps its selection, as in lazygit
	ranges             [panelCount]listRange // each list's range selection, as lazygit's ListCursor
	cherry             *cherrypicking.CherryPicking
	copiedCommits      []*models.Commit
	prs                map[string]*models.GithubPullRequest // GitHub PRs by local branch
	prKey              string                               // what prs was fetched for
	commitFiles        *commitFilesState                    // drill-down replacing the commits list
	customPatch        *customPatchState
	mainTitle          string
	secondaryText      string
	secondaryTitle     string
	secondaryGen       int
	secondaryList      ui.ListState
	sourceStaging      *stagingState
	staging            *stagingState // line staging in the main pane
	mainFocused        bool          // main pane focused for scrolling (key 0)

	diff     []diffLine
	diffList ui.ListState
	diffKey  string
	shownKey string
	diffGen  int

	overlays []overlay
	draft    commitDraft // survives closing the commit editor, as lazygit's does

	log     []logEntry
	logList ui.ListState
	showLog bool

	layout layout // border sizes the user dragged
	grips  []grip // this frame's draggable borders

	switching bool
	busy      string // label of a running operation
	status    string // last error or result, in the hint bar
	statusErr bool

	rootH ui.Handle // the focusable root that owns the keymap

	terminalCommand func(string) error     // external terminal; replaced in tests
	openLink        func(url string) error // browser; replaced in tests
	copyText        func(s string) error   // clipboard; replaced in tests

	post func(func()) // run on the main thread and redraw
	bg   func(func()) // run off the main thread
	quit func()
}

type logEntry struct {
	text  string
	isCmd bool
}

// New creates the app. post must run fn on the UI thread and redraw; bg runs
// fn in the background; quit closes the app.
func New(r *repo.Repo, post, bg func(func()), quit func()) *App {
	a := &App{repo: r, keys: loadBindings(r), focus: panelFiles, cherry: cherrypicking.New(), post: post, bg: bg, quit: quit, showLog: true}
	a.quit = func() { a.repo.Close(); quit() }
	a.openLink = func(url string) error { return a.repo.OS.OpenLink(url) }
	a.copyText = func(s string) error { return a.repo.OS.CopyToClipboard(s) }
	a.actions = a.buildActions()
	a.logList.FollowEnd = true
	a.connectRepo()
	return a
}

func (a *App) connectRepo() {
	r := a.repo
	// lazygit's onUserConfigLoaded
	icons.SetNerdFontsVersion(r.Config.Gui.NerdFontsVersion)
	a.newFileTrees()
	post := func(fn func()) {
		a.post(func() {
			if a.repo == r {
				fn()
			}
		})
	}
	a.repo.SetHooks(repo.Hooks{
		LogCommand: func(cmd string, isCmd bool) {
			post(func() {
				a.log = append(a.log, logEntry{text: cmd, isCmd: isCmd})
				if len(a.log) > 500 {
					a.log = a.log[len(a.log)-500:]
				}
			})
		},
		PromptCredential: func(kind oscommands.CredentialType) <-chan string {
			ch := make(chan string, 1)
			post(func() { a.promptCredential(kind, ch) })
			return ch
		},
	})
}

// Refresh reloads the snapshot in the background.
func (a *App) Refresh() {
	r := a.repo
	a.bg(func() {
		s := r.Load()
		a.post(func() {
			if a.repo == r {
				a.apply(s)
			}
		})
	})
}

// BackgroundFetch runs lazygit's autoFetch: fetch without prompting, then
// refresh.
func (a *App) BackgroundFetch() {
	r := a.repo
	a.bg(func() {
		_ = r.FetchBackground()
		s := r.Load()
		a.post(func() {
			if a.repo == r {
				a.apply(s)
				a.RefreshPullRequests()
			}
		})
	})
}

func (a *App) apply(s *repo.Snapshot) {
	a.raw = s
	a.noteAuthors(s.Commits)
	if a.customPatch != nil && !a.repo.Git.Patch.PatchBuilder.Active() {
		a.customPatch = nil
	}
	a.filterSnap()
	a.syncFileTree()
	for p := range panelCount {
		a.cursor[p] = clamp(a.cursor[p], 0, a.count(p)-1)
	}
	if len(s.Errors) > 0 {
		a.setStatus(s.Errors[0].Error(), true)
	}
	a.diffKey = "" // content may have changed under the same selection
	if prKey(s) != a.prKey {
		a.RefreshPullRequests()
	}
	a.loadMain()
	if a.staging != nil {
		a.reloadStaging()
	}
}

func (a *App) setStatus(s string, isErr bool) {
	a.status, a.statusErr = s, isErr
}

func (a *App) count(p panel) int {
	if a.snap == nil {
		return 0
	}
	switch p {
	case panelStatus:
		return 1
	case panelFiles:
		if a.fileTab == 1 {
			return len(a.snap.Worktrees)
		}
		if a.fileTab == 2 {
			return len(a.snap.Submodules)
		}
		return a.files.Len()
	case panelBranches:
		if a.browse(panelBranches) != nil {
			if a.browse(panelBranches).ref != nil {
				return len(a.browse(panelBranches).commits)
			}
			return len(a.browse(panelBranches).branches)
		}
		if a.branchTab == 1 {
			return len(a.snap.Remotes)
		}
		if a.branchTab == 2 {
			return len(a.snap.Tags)
		}
		return len(a.snap.Branches)
	case panelCommits:
		switch a.commitsView() {
		case cvCommitFiles:
			return a.commitFiles.tree.Len()
		case cvSubCommits:
			return len(a.refBrowse.commits)
		case cvReflog:
			return len(a.snap.Reflog)
		}
		return len(a.snap.Commits)
	case panelStash:
		return len(a.snap.Stash)
	}
	return 0
}

// selectedFile is the selected file in the Files tab; nil on a directory.
func (a *App) selectedFile() *models.File {
	if n := a.selectedFileNode(); n != nil {
		return n.File
	}
	return nil
}

func (a *App) selectedBranch() *models.Branch {
	if a.snap == nil || len(a.snap.Branches) == 0 || a.branchTab != 0 || a.browse(panelBranches) != nil {
		return nil
	}
	return a.snap.Branches[a.cursor[panelBranches]]
}

func (a *App) selectedCommit() *models.Commit {
	if b := a.subCommits(); b != nil && a.commitFiles == nil {
		if a.count(a.focus) == 0 {
			return nil
		}
		return b.commits[a.cursor[a.focus]]
	}
	if a.commitFiles != nil {
		return a.commitFiles.commit
	}
	if a.snap == nil || a.count(panelCommits) == 0 {
		return nil
	}
	i := a.cursor[panelCommits]
	switch a.commitsView() {
	case cvSubCommits:
		return a.refBrowse.commits[i]
	case cvReflog:
		return a.snap.Reflog[i]
	}
	return a.snap.Commits[i]
}

func (a *App) selectedStash() *models.StashEntry {
	if a.count(panelStash) == 0 {
		return nil
	}
	return a.snap.Stash[a.cursor[panelStash]]
}

// loadMain loads the main pane for the focused panel's selection, dropping
// results that arrive after the selection moved on.
func (a *App) loadMain() {
	if a.snap == nil || a.staging != nil {
		return
	}
	key, load := a.mainSource()
	a.loadSecondary()
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
			if gen != a.diffGen {
				return
			}
			a.diff = lines
			if key != a.shownKey { // a new selection starts at the top; a refresh keeps the place
				a.shownKey = key
				a.diffList.ScrollTo(0, ui.Start)
			}
		})
	})
}

func (a *App) mainSource() (string, func() string) {
	if a.count(a.focus) == 0 {
		if a.focus == panelFiles && a.fileTab == 0 {
			text := a.repo.Common.Tr.NoChangedFiles
			return "empty", func() string { return text }
		}
		return "empty", nil
	}
	i := a.cursor[a.focus]
	r := a.repo
	switch a.focus {
	case panelFiles:
		if a.fileTab == 1 {
			w := a.snap.Worktrees[i]
			return "worktree:" + w.Path, func() string { return "Path: " + w.Path + "\nBranch: " + w.Branch + "\nHEAD: " + w.Head }
		}
		if a.fileTab == 2 {
			s := a.snap.Submodules[i]
			return "submodule:" + s.Path, func() string { return "Path: " + s.Path + "\nURL: " + s.Url }
		}
		tree := a.files
		n := tree.Get(i)
		paths := repo.DiffPathsForNode(n.Raw(), tree.GetRoot().Raw(), tree.GetAllFiles(), tree.IsFiltering())
		return "file:" + n.GetPath() + ":" + dirState(n), func() string { return r.WorktreeNodeDiff(n, paths, false) }
	case panelBranches:
		if s := a.browse(panelBranches); s != nil {
			if s.ref != nil {
				c := s.commits[i]
				return "commit:" + c.Hash(), func() string { return r.CommitDiff(c) }
			}
			b := s.branches[i]
			return "branch:" + b.FullName(), func() string { return r.BranchLog(b.FullRefName()) }
		}
		if a.branchTab == 1 {
			rm := a.snap.Remotes[i]
			return "remote:" + rm.Name, func() string { return strings.Join(rm.Urls, "\n") }
		}
		if a.branchTab == 2 {
			tag := a.snap.Tags[i]
			return "tag:" + tag.Name, func() string { return r.TagInfo(tag.Name, tag.FullRefName()) }
		}
		b := a.snap.Branches[i]
		return "branch:" + b.Name, func() string { return r.BranchLog(b.Name) }
	case panelCommits:
		if cf := a.commitFiles; cf != nil {
			n := cf.tree.Get(i)
			if n.File == nil {
				paths := repo.DiffPathsForNode(n.Raw(), cf.tree.GetRoot().Raw(), cf.files, cf.tree.IsFiltering())
				return "commitdir:" + cf.commit.Hash() + ":" + n.GetPath(), func() string { return r.CommitPathDiff(cf.commit, paths) }
			}
			f := n.File
			return "commitfile:" + cf.commit.Hash() + ":" + f.Path, func() string { return r.CommitFileDiff(cf.commit, f) }
		}
		if a.commitsView() != cvCommits {
			c := a.selectedCommit()
			return "commit:" + c.Hash(), func() string { return r.CommitDiff(c) }
		}
		c := a.snap.Commits[i]
		// LocalCommitsContext.GetSelectedRefRangeForDiffFiles
		if start, end := a.selectionRange(panelCommits); start != end {
			from, to := a.snap.Commits[end], a.snap.Commits[start]
			if !from.IsTODO() && !to.IsTODO() {
				return "range:" + from.Hash() + ":" + to.Hash(), func() string { return r.RangeDiff(from, to) }
			}
		}
		if c.Hash() == "" {
			return "todo:" + c.Action.String() + ":" + c.Name, func() string { return c.Action.String() + " " + c.Name }
		}
		return "commit:" + c.Hash(), func() string { return r.CommitDiff(c) }
	case panelStash:
		s := a.snap.Stash[i]
		return "stash:" + s.Hash, func() string { return r.StashDiff(s.Index) }
	}
	return "status", nil
}

// handleInput is the single dispatcher for every key that reaches the root,
// so the keymap behaves like lazygit's regardless of which widget the
// pointer last touched.
func (a *App) handleInput(ev ui.InputEvent) bool {
	ch, ok := keymap.FromEvent(ev)
	if !ok {
		return false
	}
	return a.dispatch(ch)
}

func (a *App) pageSize() int {
	if a.staging != nil {
		return a.staging.pageDelta()
	}
	first, last := a.lists[a.focus].Visible()
	return max(1, last-first)
}

// move moves the cursor of whatever has focus: the staging view, the main
// pane, or the focused panel.
func (a *App) move(d int) {
	switch {
	case a.staging != nil:
		a.staging.move(d)
		return
	case a.mainFocused:
		a.scrollMain(d)
		return
	}
	if a.ranges[a.focus].mode == traits.RangeSelectModeNonSticky {
		a.cancelRange(a.focus)
	}
	n := a.count(a.focus)
	if n == 0 {
		return
	}
	a.cursor[a.focus] = clamp(a.cursor[a.focus]+d, 0, n-1)
	a.lists[a.focus].ScrollIntoView(a.cursor[a.focus])
	a.loadMain()
}

func (a *App) focusPanel(p panel) {
	a.staging = nil
	a.mainFocused = false
	a.focus = p
	a.diffKey = ""
	a.loadMain()
}

func (a *App) scrollMain(d int) {
	first, _ := a.diffList.Visible()
	a.diffList.ScrollTo(clamp(first+d, 0, max(0, len(a.diff)-1)), ui.Start)
}

// back is lazygit's <esc>: leave the innermost mode.
func (a *App) back() {
	switch {
	case a.cancelSearching(false):
	case a.dismissRange():
	case a.cancelSearching(true):
	case a.staging != nil:
		a.escapeStaging()
	case a.mainFocused:
		a.mainFocused = false
	case a.browse(panelBranches) != nil && a.focus == panelBranches:
		a.backRef()
	case a.commitsView() == cvSubCommits && a.focus == panelCommits:
		a.backRef()
	case a.commitFiles != nil && a.focus == panelCommits:
		a.cursor[panelCommits] = a.commitFiles.commitIdx
		if a.commitFiles.fromStash {
			a.focus = panelStash
			a.cursor[panelStash] = clamp(a.commitFiles.stashIdx, 0, a.count(panelStash)-1)
		}
		if a.commitFiles.fromRef {
			a.focus = panelBranches
		}
		a.commitFiles = nil
		a.diffKey = ""
		a.loadMain()
	case a.customPatch != nil:
		a.resetCustomPatch()
	case a.filteringActive():
		a.clearFiltering()
	case a.cherry.Active() && (a.focus == panelCommits || a.focus == panelBranches):
		a.resetCherryPick()
	default:
		a.returnToParent()
	}
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
	lineMsg  // commit message body, shown in the prose font
)

type diffLine struct {
	kind diffKind
	text string
}

func parseDiff(s string) []diffLine {
	lines := repo.SplitLines(s)
	out := make([]diffLine, 0, len(lines))
	inHunk := false
	if len(lines) > 0 && strings.HasPrefix(lines[0], "commit ") {
		// git show preamble: hash, author and date are in the header, and
		// so is the subject; keep the body as prose.
		i, subject := 1, true
		isStat := func(l string) bool { return strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "    ") }
		for ; i < len(lines) && !strings.HasPrefix(lines[i], "diff --git") && !isStat(lines[i]); i++ {
			l := lines[i]
			if strings.HasPrefix(l, "    ") {
				if subject {
					subject = false
					continue
				}
				out = append(out, diffLine{kind: lineMsg, text: strings.TrimPrefix(l, "    ")})
			}
		}
		for len(out) > 0 && strings.TrimSpace(out[0].text) == "" {
			out = out[1:]
		}
		if len(out) > 0 {
			out = append(out, diffLine{kind: lineMsg}) // space before the stat
		}
		lines = lines[i:]
	}
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

var _ = time.Second

// A panel jump returns to the panel's primary list. Clicking a nested list
// uses focusPanel instead, so it can retain the drill-down being clicked.
func (a *App) jumpPanel(p panel) {
	// lazygit's goToSideWindow: the focused window's own jump key moves to
	// its next tab when gui.switchTabsWithPanelJumpKeys is on.
	if a.focus == p && !a.mainFocused && a.staging == nil && a.repo.Config.Gui.SwitchTabsWithPanelJumpKeys {
		a.switchTab(1)
		return
	}
	if p == panelCommits && a.commitFiles != nil {
		a.cursor[panelCommits] = a.commitFiles.commitIdx
		a.commitFiles = nil
	}
	a.focusPanel(p)
}

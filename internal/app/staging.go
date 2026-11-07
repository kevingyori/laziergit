package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/utils"

	"laziergit/internal/repo"
)

// stagingState is lazygit's focused main view (main_view_controller.go): a
// line cursor over the diff the side panel shows, with range and hunk
// selection. Over the working tree's diff it stages and discards lines, in
// the main (unstaged) or secondary (staged) pane; over a commit's or a stash
// entry's diff it reads only.
type stagingState struct {
	worktree bool   // the files panel's diff: lines can be staged
	staged   bool   // the secondary pane, showing the staged side
	key      string // what the diff is of; a new key re-establishes the selection
	title    string

	text     string
	rendered bool        // text is a diff renderer's output
	ansi     []ansiLine  // the rendered rows, when rendered
	alts     [][]diffRow // every diff line a rendered row shows
	rows     []diffRow
	sections []diffSection
	lines    []*patch.PatchLine // one per row; Content is the row's text
	loaded   bool

	cursor   int // -1: nothing to select
	anchor   int // range anchor, -1 when none (in hunk mode, the block's far end)
	mode     selectMode
	sticky   bool // a range started with v, which plain moves extend
	userHunk bool // hunk mode turned on with a, not the configured default

	reveal         int // change ordinal to select once the diff is re-read, -1 when none
	gen            int
	list           ui.ListState
	horizontal     ui.ScrollState
	contentWidth   float32
	viewportWidth  float32
	viewportHeight float32
	wantOrigin     int
	originSet      bool // wantOrigin asked for, not yet laid out
	startOrigin    int  // the line the pane showed at the top before it was focused, -1 when none
}

type selectMode int

const (
	selectLine selectMode = iota
	selectRange
	selectHunk
)

// focusMainView is lazygit's SwitchToFocusedMainViewController: the side
// panels that show a diff focus it with a selection; the others only scroll.
func (a *App) focusMainView() {
	if a.focus == panelFiles && a.fileTab == 0 {
		if n := a.selectedFileNode(); n != nil {
			a.enterWorktreeDiff(n)
			return
		}
	}
	if key, load := a.mainSource(); load != nil && isDiffSource(key) {
		a.staging = &stagingState{key: key, cursor: -1, anchor: -1, reveal: -1, viewportHeight: a.mainViewportHeight, startOrigin: a.unfocusedOrigin("main")}
		a.reloadStaging()
		return
	}
	a.mainFocused = true
}

func isDiffSource(key string) bool {
	for _, p := range []string{"commit:", "commitfile:", "commitdir:", "stash:"} {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// enterStaging is <enter> on the files panel: a directory opens or closes,
// a conflicted file offers its resolutions, and a file focuses its diff.
func (a *App) enterStaging() {
	if a.toggleCollapsed() {
		return
	}
	f := a.selectedFile()
	if f == nil || (!f.HasUnstagedChanges && !f.HasStagedChanges) {
		return
	}
	if f.HasMergeConflicts {
		a.resolveMenu(f)
		return
	}
	a.enterWorktreeDiff(a.selectedFileNode())
}

func (a *App) enterWorktreeDiff(n *filetree.FileNode) {
	// The content may be in the secondary pane alone: a file with nothing but
	// staged changes shows them there.
	staged := !n.GetHasUnstagedChanges() && n.GetHasStagedChanges()
	pane, height := "main", a.mainViewportHeight
	if staged {
		pane, height = "secondary", a.secondaryViewportHeight
	}
	a.staging = &stagingState{worktree: true, staged: staged, cursor: -1, anchor: -1, reveal: -1, viewportHeight: height, startOrigin: a.unfocusedOrigin(pane)}
	a.reloadStaging()
}

// stagingFile is the file of the working tree a section of the diff is of.
func (a *App) stagingFile(path string) *models.File {
	if a.snap == nil {
		return nil
	}
	for _, f := range a.snap.Files {
		if f.Path == path {
			return f
		}
	}
	return nil
}

// reloadStaging re-reads the diff, after a refresh or an action. Over the
// working tree it follows the files panel's selection, and carries on in the
// other pane when the one it was in ran out of changes.
func (a *App) reloadStaging() {
	s := a.staging
	a.loadSecondary()
	var load func() string
	previewRendered := false
	if !s.worktree && s.staged {
		if a.customPatch == nil || a.customPatch.preview == "" {
			s.staged = false
			s.loaded = false
		} else {
			preview := a.customPatch.preview
			switch {
			case a.customPatch.raw != "":
				preview, previewRendered = a.customPatch.raw, true
			case a.customPatch.rendered != "":
				preview, previewRendered = a.customPatch.rendered, true
			}
			load = func() string { return preview }
		}
	}
	if load != nil {
		// The custom patch supplies its cached preview.
	} else if s.worktree {
		n := a.selectedFileNode()
		if n == nil || (!n.GetHasUnstagedChanges() && !n.GetHasStagedChanges()) {
			a.leaveStaging()
			a.mainFocused = true
			return
		}
		switch {
		case s.staged && !n.GetHasStagedChanges():
			s.staged = false
		case !s.staged && !n.GetHasUnstagedChanges():
			s.staged = true
		}
		paths := repo.DiffPathsForNode(n.Raw(), a.files.GetRoot().Raw(), a.files.GetAllFiles(), a.files.IsFiltering())
		r, staged := a.repo, s.staged
		pane := "main"
		if staged {
			pane = "secondary"
		}
		load = a.diffLoad(pane, func() string { return r.WorktreeNodeViewDiff(n, paths, staged) }, "", r.WorktreeNodeDiffCmd(n, paths, staged)) // the rendered diff, honouring ignore-whitespace
		key := "worktree:" + n.GetPath()
		if staged {
			key += ":staged"
		}
		if key != s.key {
			s.key, s.loaded = key, false
		}
		s.title = n.GetPath()
	} else {
		key, l := a.mainSource()
		if l == nil || !isDiffSource(key) {
			a.leaveStaging()
			return
		}
		if key != s.key {
			s.key, s.loaded = key, false
		}
		load = l
		s.title = ""
	}
	s.gen++
	gen := s.gen
	sourceKey, sourceLoad := a.mainSource()
	staged := s.staged
	source := a.sourceStaging
	if staged && (source == nil || source.key != sourceKey) {
		source = &stagingState{key: sourceKey, worktree: s.worktree, cursor: -1, anchor: -1, reveal: -1, viewportHeight: a.mainViewportHeight, startOrigin: -1}
		a.sourceStaging = source
	}
	rendered := a.repo.RenderersConfigured() && (s.worktree || !staged || previewRendered)
	sourceRendered := a.repo.RenderersConfigured()
	a.bg(func() {
		text := load()
		var sourceText string
		if staged && sourceLoad != nil {
			sourceText = sourceLoad()
		}
		a.post(func() {
			if a.staging != s || s.gen != gen {
				return
			}
			s.rendered = rendered
			s.setContent(text, a.hunkModeByDefault())
			if staged && source != nil {
				source.rendered = sourceRendered
				source.setContent(sourceText, a.hunkModeByDefault())
				a.diff = diffLinesOf(sourceText, sourceRendered)
				a.diffRows, _ = diffRowsOf(sourceText, sourceRendered)
				a.diffText, a.diffRendered = sourceText, sourceRendered
			}
		})
	})
}

func (a *App) leaveStaging() {
	if s := a.staging; s != nil && !s.staged {
		// One view, focused or not: it stays where it was scrolled to.
		a.mainOriginAfterLoad = s.origin()
	}
	a.staging = nil
	a.diffKey = ""
	a.loadMain()
}

func (a *App) hunkModeByDefault() bool {
	return a.repo.Config.Gui.UseHunkModeInDiffView
}

// setContent shows a newly read diff. A new diff gets its selection
// established; the same one re-read keeps its place, or, after an action,
// moves to the change that took the acted-on lines' place.
func (s *stagingState) setContent(text string, hunkDefault bool) {
	fresh := !s.loaded
	oldRows := s.rows
	oldCursor, oldAnchor := s.cursor, s.anchor
	// lazygit's PreserveDiffPositionOnRerender: the end of the selection
	// that is on screen, else the middle visible line.
	oy, height := s.origin(), s.innerHeight()
	onScreen := func(i int) bool { return i >= oy && i < oy+height }
	visualAnchor := (oy + min(oy+height, s.viewLines())) / 2
	if oldCursor >= 0 && onScreen(oldCursor) {
		visualAnchor = oldCursor
	} else if oldCursor >= 0 && oldAnchor >= 0 && onScreen(oldAnchor) {
		visualAnchor = oldAnchor
	}
	keepPosition := !fresh && s.reveal < 0
	s.contentWidth = 0
	s.loaded = true
	s.text = text
	var lines []string
	var rows []diffRow
	var sections []diffSection
	s.ansi, s.alts = nil, nil
	if s.rendered {
		d := parseRenderedDiff(text)
		lines, rows, sections, s.ansi, s.alts = d.texts, d.rows, d.sections, d.lines, d.alts
	} else {
		lines, rows, sections = parseDiffRows(text)
	}
	s.rows, s.sections = rows, sections
	s.lines = make([]*patch.PatchLine, len(lines))
	for i, l := range lines {
		s.lines[i] = &patch.PatchLine{Kind: rows[i].kind, Content: l}
	}
	if !s.hasChanges() {
		s.cursor, s.anchor, s.mode = -1, -1, selectLine
		s.reveal = -1
		return
	}
	switch {
	case s.reveal >= 0:
		s.revealOrdinal(s.reveal)
		s.reveal = -1
	case fresh || s.cursor < 0:
		s.establish(hunkDefault)
	default:
		if n, ok := restoreLine(oldRows, s.rows, oldCursor); ok {
			s.cursor = n
		} else {
			s.cursor = clamp(oldCursor, 0, len(s.rows)-1)
		}
		switch s.mode {
		case selectHunk:
			s.selectBlock(s.cursor)
		case selectRange:
			if n, ok := findDiffRow(s.rows, oldRows, oldAnchor); ok {
				s.anchor = n
			} else {
				s.resetMode()
			}
		}
	}
	s.clampOriginXToContent()
	if fresh && s.startOrigin >= 0 {
		s.setOrigin(s.startOrigin)
		s.startOrigin = -1
	} else if keepPosition && len(oldRows) > 0 {
		if origin, ok := restoreOrigin(oldRows, s.rows, oy, height, visualAnchor); ok {
			s.setOrigin(origin)
		}
	} else {
		s.scrollIntoView(s.cursor)
	}
}

func (s *stagingState) hasChanges() bool {
	for _, r := range s.rows {
		if r.isChange() {
			return true
		}
	}
	return false
}

func (s *stagingState) isChange(i int) bool {
	return i >= 0 && i < len(s.rows) && s.rows[i].isChange()
}

func (s *stagingState) changeFlags() []bool {
	out := make([]bool, len(s.rows))
	for i, r := range s.rows {
		out[i] = r.isChange()
	}
	return out
}

// establish is lazygit's EstablishSelection for keyboard focus: the first
// change on screen, widened to its block when hunk mode is the default.
func (s *stagingState) establish(hunkDefault bool) {
	s.resetMode()
	lo, hi := 0, len(s.rows)-1
	if s.viewportHeight <= 0 {
		s.startOrigin = -1 // never shown: nothing on screen to go by
	}
	if s.startOrigin >= 0 {
		// Focusing never moves the view: the selection goes where the user
		// is looking.
		lo = min(s.startOrigin, hi)
		hi = min(hi, lo+s.innerHeight()-1)
	}
	target := -1
	for i := lo; i <= hi && target < 0; i++ {
		if s.isChange(i) && (!hunkDefault || i == 0 || !s.isChange(i-1)) {
			target = i
		}
	}
	for i := lo; i <= hi && target < 0; i++ { // a block reaching in from above
		if s.isChange(i) {
			target = i
		}
	}
	if target < 0 {
		if s.startOrigin < 0 {
			s.cursor = -1
			return
		}
		// No change on screen: the middle visible line, the likeliest
		// one to be read.
		s.cursor = min((lo+min(lo+s.innerHeight(), s.viewLines()))/2, len(s.rows)-1)
		return
	}
	s.cursor = target
	if hunkDefault && !s.singleHunkForWholeFile(target) {
		s.mode = selectHunk
		s.selectBlock(target)
	}
}

func (s *stagingState) resetMode() {
	s.mode, s.sticky, s.userHunk, s.anchor = selectLine, false, false, -1
}

// singleHunkForWholeFile is lazygit's IsSingleHunkForWholeFile: a file shown
// as one solid block of changes of one kind (an added or deleted file),
// where hunk mode would select all of it.
func (s *stagingState) singleHunkForWholeFile(i int) bool {
	if !s.isChange(i) {
		return false
	}
	path, kind := s.rows[i].path, s.rows[i].kind
	for _, r := range s.rows {
		if !r.ok || r.path != path {
			continue
		}
		if r.kind == patch.CONTEXT || (r.isChange() && r.kind != kind) {
			return false
		}
	}
	return true
}

// blockBounds is lazygit's ChangeBlockBounds: the block of adjacent changes
// at or after i, else the one before it.
func (s *stagingState) blockBounds(i int) (int, int, bool) {
	start := max(i, 0)
	for start < len(s.rows) && !s.isChange(start) {
		start++
	}
	if start >= len(s.rows) {
		for start = min(i, len(s.rows)-1); start >= 0 && !s.isChange(start); start-- {
		}
		if start < 0 {
			return 0, 0, false
		}
	}
	end := start
	for start > 0 && s.isChange(start-1) {
		start--
	}
	for end < len(s.rows)-1 && s.isChange(end+1) {
		end++
	}
	return start, end, true
}

// selectBlock is lazygit's SelectChangeBlock: cursor on the block's first
// line, anchor on its last.
func (s *stagingState) selectBlock(i int) {
	lo, hi, ok := s.blockBounds(i)
	if !ok {
		s.mode, s.anchor = selectLine, -1
		s.cursor = clamp(i, 0, len(s.rows)-1)
		return
	}
	s.cursor, s.anchor = lo, hi
	s.scrollIntoView(lo)
}

// revealOrdinal is lazygit's RevealSelectionAfterAction: the change line
// that many changes into the diff, or the last one left.
func (s *stagingState) revealOrdinal(ordinal int) {
	if s.mode == selectRange {
		s.mode, s.sticky = selectLine, false
	}
	target, count := -1, 0
	for i := range s.rows {
		if !s.isChange(i) {
			continue
		}
		target = i
		if count == ordinal {
			break
		}
		count++
	}
	if s.mode == selectHunk {
		s.selectBlock(target)
		return
	}
	s.anchor = -1
	s.cursor = target
}

func (s *stagingState) changeOrdinal(row int) int {
	n := 0
	for i := 0; i < row && i < len(s.rows); i++ {
		if s.isChange(i) {
			n++
		}
	}
	return n
}

// selectingRange is lazygit's: range mode, sticky or actually extended.
func (s *stagingState) selectingRange() bool {
	return s.mode == selectRange && (s.sticky || s.anchor != s.cursor)
}

// collapseForLineMove drops hunk mode and a non-sticky range before a plain
// move.
func (s *stagingState) collapseForLineMove() {
	switch {
	case s.mode == selectHunk:
		s.mode, s.anchor = selectLine, -1
	case s.mode == selectRange && !s.sticky:
		s.mode, s.anchor = selectLine, -1
	}
}

func (s *stagingState) collapseNonStickyRange() {
	if s.mode == selectRange && !s.sticky {
		s.mode, s.anchor = selectLine, -1
	}
}

func (s *stagingState) pageDelta() int {
	first, last := s.list.Visible()
	return max(1, last-first)
}

// move is lazygit's adjustSelection: in hunk mode a single step goes to the
// next block; otherwise the selection collapses to a line and moves. With
// nothing to select the view scrolls.
func (s *stagingState) move(d int) {
	if len(s.rows) == 0 {
		return
	}
	if s.cursor < 0 {
		first, _ := s.list.Visible()
		s.setOrigin(clamp(first+d, 0, len(s.rows)-1))
		return
	}
	if s.mode == selectHunk && (d == 1 || d == -1) {
		s.navigateBlock(d > 0)
		return
	}
	s.collapseForLineMove()
	s.cursor = clamp(s.cursor+d, 0, len(s.rows)-1)
	s.scrollIntoView(s.cursor)
}

// navigateTo is lazygit's PlaceNavigationTarget for a focused pane.
func (s *stagingState) navigateTo(target int) {
	s.collapseNonStickyRange()
	if s.mode == selectHunk {
		s.selectBlock(target)
		return
	}
	s.cursor = target
	s.scrollIntoView(target)
}

func (s *stagingState) navigateBlock(forward bool) {
	if s.cursor < 0 {
		return
	}
	if t, ok := changeBlockStart(s.changeFlags(), s.cursor, forward); ok {
		s.navigateTo(t)
	}
}

func (s *stagingState) navigateFile(forward bool) {
	if s.cursor < 0 {
		return
	}
	if t, ok := adjacentFileStart(s.rows, s.cursor, forward); ok {
		s.alignFileTop(t)
		s.navigateTo(t)
	}
}

// moveBlock keeps the old name for the change-block keys.
func (s *stagingState) moveBlock(d int) { s.navigateBlock(d > 0) }

func (s *stagingState) toggleRange() {
	if s.cursor < 0 {
		return
	}
	if s.selectingRange() {
		s.mode, s.sticky, s.anchor = selectLine, false, -1
		return
	}
	s.mode, s.sticky, s.anchor = selectRange, true, s.cursor
}

// extend is shift+up/down: a range that grows while shift is held.
func (s *stagingState) extend(d int) {
	if s.cursor < 0 {
		return
	}
	if !s.selectingRange() {
		s.mode, s.anchor = selectRange, s.cursor
	}
	s.sticky = false
	s.cursor = clamp(s.cursor+d, 0, len(s.rows)-1)
	s.scrollIntoView(s.cursor)
}

func (s *stagingState) toggleHunk() {
	if s.cursor < 0 {
		return
	}
	if s.mode == selectHunk {
		s.mode, s.anchor = selectLine, -1
		return
	}
	s.mode, s.userHunk = selectHunk, true
	s.selectBlock(s.cursor)
}

// selection returns the selected rows (inclusive).
func (s *stagingState) rangeAnchor() int {
	if s.cursor >= 0 && s.anchor >= 0 && s.mode != selectLine {
		return s.anchor
	}
	return -1
}

func (s *stagingState) selection() (int, int) {
	if s.rangeAnchor() >= 0 {
		return min(s.anchor, s.cursor), max(s.anchor, s.cursor)
	}
	return s.cursor, s.cursor
}

// gotoEdge is lazygit's handleGotoTop/Bottom.
func (s *stagingState) gotoEdge(bottom bool) {
	if len(s.rows) == 0 {
		return
	}
	target := 0
	if bottom {
		target = len(s.rows) - 1
	}
	if s.cursor < 0 {
		s.setOrigin(target)
		return
	}
	s.collapseForLineMove()
	s.cursor = target
	s.scrollIntoView(target)
}

// escape is lazygit's: a range or a hunk selection the user asked for goes
// first, then the view.
func (a *App) escapeStaging() {
	s := a.staging
	if s.cursor >= 0 && (s.selectingRange() || (s.mode == selectHunk && s.userHunk)) {
		s.resetMode()
		return
	}
	a.leaveStaging()
}

// fileLines is the selected change lines of one file of a working-tree diff.
type fileLines struct {
	file    *models.File
	patch   *patch.Patch
	indices []int
	// ids names the lines instead, when the rows were a renderer's: they
	// are looked up in the file's current plain diff.
	ids []patch.LineIdentity
}

// refresh finds the lines in the file's current diff (the first one only,
// with first).
func (f fileLines) refresh(r *repo.Repo, staged, first bool) (*patch.Patch, []int, bool) {
	if f.patch == nil {
		ids := f.ids
		if first {
			ids = ids[:1]
		}
		return r.LinesByIdentity(f.file, staged, ids)
	}
	indices := f.indices
	if first {
		indices = indices[:1]
	}
	return r.RefreshLines(f.file, f.patch, staged, indices)
}

func (a *App) selectedFileLines() ([]fileLines, bool) {
	s := a.staging
	if s == nil || !s.worktree || s.cursor < 0 {
		return nil, false
	}
	lo, hi := s.selection()
	var out []fileLines
	if s.rendered {
		// Rows a renderer placed: by file and line identity.
		byPath := map[string]int{}
		for i := lo; i <= hi; i++ {
			for _, r := range s.identitiesAt(i) {
				if !r.isChange() {
					continue
				}
				k, ok := byPath[r.path]
				if !ok {
					f := a.stagingFile(r.path)
					if f == nil {
						a.errorPopup(utils.ResolvePlaceholderString(a.repo.Common.Tr.DiffLinesFileNotFound,
							map[string]string{"path": r.path}))
						return nil, false
					}
					k = len(out)
					byPath[r.path] = k
					out = append(out, fileLines{file: f})
				}
				out[k].ids = append(out[k].ids, r.lineIdentity())
			}
		}
		return out, len(out) > 0
	}
	bySection := map[int]int{}
	for i := lo; i <= hi; i++ {
		if !s.isChange(i) {
			continue
		}
		r := s.rows[i]
		k, ok := bySection[r.section]
		if !ok {
			sec := s.sections[r.section]
			f := a.stagingFile(sec.path)
			if f == nil {
				a.errorPopup(utils.ResolvePlaceholderString(a.repo.Common.Tr.DiffLinesFileNotFound,
					map[string]string{"path": sec.path}))
				return nil, false
			}
			k = len(out)
			bySection[r.section] = k
			out = append(out, fileLines{file: f, patch: sec.patch})
		}
		out[k].indices = append(out[k].indices, i-s.sections[r.section].start)
	}
	return out, len(out) > 0
}

// stageSelection is lazygit's PrimaryAction over the working tree's diff:
// stage the selected lines, or unstage them from the staged side.
func (a *App) stageSelection() {
	s := a.staging
	if s == nil {
		return
	}
	if !s.worktree {
		a.togglePatchSelection(false)
		return
	}
	if a.repo.Config.Git.DiffContextSize == 0 {
		a.errorPopup(fmt.Sprintf(a.repo.Common.Tr.Actions.NotEnoughContextToStage,
			a.repo.Config.Keybinding.Universal.IncreaseContextInDiffView))
		return
	}
	files, ok := a.selectedFileLines()
	if !ok {
		return
	}
	lo, _ := s.selection()
	staged := s.staged
	s.reveal = s.changeOrdinal(lo)
	a.run("Staging…", func() error {
		for _, f := range files {
			p, indices, ok := f.refresh(a.repo, staged, false)
			if !ok {
				return a.diffLinesNotFound(f.file.Path)
			}
			if err := a.repo.ApplyLines(f.file, p, staged, indices); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}

// switchStagingSide is <tab>: the other pane, when it is showing anything.
func (a *App) switchStagingSide() {
	s := a.staging
	if s == nil {
		return
	}
	if !s.worktree {
		if a.customPatch != nil {
			if !s.staged {
				a.sourceStaging = s
			}
			a.staging = &stagingState{key: s.key, staged: !s.staged, cursor: -1, anchor: -1, reveal: -1, viewportHeight: a.mainViewportHeight, startOrigin: -1}
			a.reloadStaging()
		}
		return
	}
	n := a.selectedFileNode()
	if n == nil || (s.staged && !n.GetHasUnstagedChanges()) || (!s.staged && !n.GetHasStagedChanges()) {
		return
	}
	a.staging = &stagingState{worktree: true, staged: !s.staged, cursor: -1, anchor: -1, reveal: -1, viewportHeight: a.mainViewportHeight, startOrigin: -1}
	a.reloadStaging()
}

// discardSelection is lazygit's DiscardSelection over the working tree's
// diff: out of the working tree (after asking), or out of the index on the
// staged side.
func (a *App) discardSelection() {
	s := a.staging
	if s == nil {
		return
	}
	if !s.worktree {
		a.togglePatchSelection(true)
		return
	}
	if a.repo.Config.Git.DiffContextSize == 0 {
		a.errorPopup(fmt.Sprintf(a.repo.Common.Tr.Actions.NotEnoughContextToDiscard,
			a.repo.Config.Keybinding.Universal.IncreaseContextInDiffView))
		return
	}
	files, ok := a.selectedFileLines()
	if !ok {
		return
	}
	lo, _ := s.selection()
	ordinal := s.changeOrdinal(lo)
	apply := func() {
		s.reveal = ordinal
		a.run("Discarding…", func() error {
			for _, f := range files {
				var err error
				p, indices, ok := f.refresh(a.repo, s.staged, false)
				if !ok {
					return a.diffLinesNotFound(f.file.Path)
				}
				if s.staged {
					err = a.repo.ApplyLines(f.file, p, true, indices)
				} else {
					err = a.repo.DiscardLines(f.file, p, indices)
				}
				if err != nil {
					return err
				}
			}
			return nil
		}, nil)
	}
	if s.staged || a.repo.Config.Gui.SkipDiscardChangeWarning {
		apply()
		return
	}
	a.confirm(a.repo.Common.Tr.DiscardChangeTitle, a.repo.Common.Tr.DiscardChangePrompt, true, apply)
}

// Context changes move row indices. Keep both ends by file and line identity;
// if the cursor vanished, use its nearest surviving neighbour. A lost far end
// collapses the selection, just as lazygit's DiffLineHelper does.
func sameDiffRow(a, b diffRow) bool {
	return a.ok && b.ok && a.path == b.path && a.kind == b.kind && a.lineIdentity() == b.lineIdentity()
}
func findDiffRow(rows, old []diffRow, i int) (int, bool) {
	if i < 0 || i >= len(old) {
		return 0, false
	}
	for n, r := range rows {
		if sameDiffRow(r, old[i]) {
			return n, true
		}
	}
	return 0, false
}
func restoreDiffRow(old, rows []diffRow, i int) int {
	for d := 0; d < len(old); d++ {
		if n, ok := findDiffRow(rows, old, i-d); ok {
			return n
		}
		if d > 0 {
			if n, ok := findDiffRow(rows, old, i+d); ok {
				return n
			}
		}
	}
	return clamp(i, 0, len(rows)-1)
}

// scrollFocusedDiff is lazygit's ViewTrait.ScrollLeft/ScrollRight: a third
// of the pane's width.
func (a *App) scrollFocusedDiff(direction int) {
	s := a.staging
	if s == nil || a.repo.Config.Gui.WrapLinesInDiffView {
		return
	}
	s.setOriginX(s.originX() + direction*(s.innerWidth()/3))
}

// diffLinesNotFound is lazygit's diffLinesNotFoundError, shown as a plain
// error alert.
func (a *App) diffLinesNotFound(path string) error {
	return &askError{err: errors.New(path), ask: func() {
		a.errorPopup(utils.ResolvePlaceholderString(a.repo.Common.Tr.DiffLinesNotFound, map[string]string{"path": path}))
	}}
}

// diffLinesFileNotFound is lazygit's diffLinesFileNotFoundError.
func (a *App) diffLinesFileNotFound(path string) error {
	return &askError{err: errors.New(path), ask: func() {
		a.errorPopup(utils.ResolvePlaceholderString(a.repo.Common.Tr.DiffLinesFileNotFound, map[string]string{"path": path}))
	}}
}

// editHunk is lazygit's WorkingTreeDiffActions.EditHunk: the hunk of the
// selection's first change goes to the editor as a patch, and what comes back
// is applied to the index.
func (a *App) editHunk() {
	s := a.staging
	if s == nil || !s.worktree {
		return
	}
	files, ok := a.selectedFileLines()
	if !ok {
		return
	}
	f, staged, r := files[0], s.staged, a.repo
	a.run("", func() error {
		p, indices, ok := f.refresh(r, staged, true)
		if !ok || len(indices) == 0 {
			return a.diffLinesNotFound(f.file.Path)
		}
		hunkIdx := p.HunkContainingLine(indices[0])
		hunkStart := p.HunkStartIdx(hunkIdx)
		text := p.Transform(patch.TransformOpts{
			Reverse:             staged,
			IncludedLineIndices: patch.ExpandRange(hunkStart, p.HunkEndIdx(hunkIdx)),
			FileNameOverride:    f.file.GetPath(),
		}).FormatPlain()
		path, err := r.Git.Patch.SaveTemporaryPatch(text)
		if err != nil {
			return err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		// The patch has a two-line header before its hunk.
		const headerLineCount = 2
		if err := a.runTerminal(r.Git.File.GetEditAtLineAndWaitCmdStr(abs, indices[0]-hunkStart+headerLineCount+1)); err != nil {
			return err
		}
		edited, err := r.Git.File.Cat(path)
		if err != nil {
			return err
		}
		lineCount := strings.Count(edited, "\n") + 1
		newText := patch.Parse(edited).Transform(patch.TransformOpts{
			IncludedLineIndices: patch.ExpandRange(0, lineCount),
			FileNameOverride:    f.file.GetPath(),
		}).FormatPlain()
		return r.Git.Patch.ApplyPatch(newText, git_commands.ApplyPatchOpts{Reverse: staged, Cached: true})
	}, nil)
}

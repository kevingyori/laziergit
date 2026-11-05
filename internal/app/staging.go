package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
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
	rows     []diffRow
	sections []diffSection
	lines    []*patch.PatchLine // one per row; Content is the row's text
	loaded   bool

	cursor   int // -1: nothing to select
	anchor   int // range anchor, -1 when none (in hunk mode, the block's far end)
	mode     selectMode
	sticky   bool // a range started with v, which plain moves extend
	userHunk bool // hunk mode turned on with a, not the configured default

	reveal        int // change ordinal to select once the diff is re-read, -1 when none
	gen           int
	list          ui.ListState
	horizontal    ui.ScrollState
	contentWidth  float32
	viewportWidth float32
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
		a.staging = &stagingState{key: key, cursor: -1, anchor: -1, reveal: -1}
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
	a.staging = &stagingState{worktree: true, staged: staged, cursor: -1, anchor: -1, reveal: -1}
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
	if !s.worktree && s.staged {
		if a.customPatch == nil || a.customPatch.preview == "" {
			s.staged = false
			s.loaded = false
		} else {
			preview := a.customPatch.preview
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
		load = func() string { return r.WorktreeNodeDiff(n, paths, staged) }
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
		source = &stagingState{key: sourceKey, worktree: s.worktree, cursor: -1, anchor: -1, reveal: -1}
		a.sourceStaging = source
	}
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
			s.setContent(text, a.hunkModeByDefault())
			if staged && source != nil {
				source.setContent(sourceText, a.hunkModeByDefault())
				a.diff = parseDiff(sourceText)
			}
		})
	})
}

func (a *App) leaveStaging() {
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
	first, last := s.list.Visible()
	visualAnchor := oldCursor
	if visualAnchor < first || visualAnchor > last {
		if oldAnchor >= first && oldAnchor <= last {
			visualAnchor = oldAnchor
		} else {
			visualAnchor = (first + last) / 2
		}
	}
	keepPosition := !fresh && s.reveal < 0
	s.contentWidth = 0
	s.loaded = true
	s.text = text
	lines, rows, sections := parseDiffRows(text)
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
		s.cursor = restoreDiffRow(oldRows, s.rows, oldCursor)
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
	if keepPosition && len(oldRows) > 0 {
		target := restoreDiffRow(oldRows, s.rows, visualAnchor)
		s.list.ScrollTo(max(0, target-(visualAnchor-first)), ui.Start)
	} else {
		s.list.ScrollIntoView(s.cursor)
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
	target := -1
	for i := range s.rows {
		if s.isChange(i) {
			target = i
			break
		}
	}
	if target < 0 {
		s.cursor = -1
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
	s.list.ScrollIntoView(lo)
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
		s.list.ScrollTo(clamp(first+d, 0, len(s.rows)-1), ui.Start)
		return
	}
	if s.mode == selectHunk && (d == 1 || d == -1) {
		s.navigateBlock(d > 0)
		return
	}
	s.collapseForLineMove()
	s.cursor = clamp(s.cursor+d, 0, len(s.rows)-1)
	s.list.ScrollIntoView(s.cursor)
}

// navigateTo is lazygit's PlaceNavigationTarget for a focused pane.
func (s *stagingState) navigateTo(target int) {
	s.collapseNonStickyRange()
	if s.mode == selectHunk {
		s.selectBlock(target)
		return
	}
	s.cursor = target
	s.list.ScrollIntoView(target)
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
	s.list.ScrollIntoView(s.cursor)
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
		s.list.ScrollTo(target, ui.Start)
		return
	}
	s.collapseForLineMove()
	s.cursor = target
	s.list.ScrollIntoView(target)
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
}

func (a *App) selectedFileLines() ([]fileLines, bool) {
	s := a.staging
	if s == nil || !s.worktree || s.cursor < 0 {
		return nil, false
	}
	lo, hi := s.selection()
	var out []fileLines
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
			if err := a.repo.ApplyLines(f.file, f.patch, staged, f.indices); err != nil {
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
			a.staging = &stagingState{key: s.key, staged: !s.staged, cursor: -1, anchor: -1, reveal: -1}
			a.reloadStaging()
		}
		return
	}
	n := a.selectedFileNode()
	if n == nil || (s.staged && !n.GetHasUnstagedChanges()) || (!s.staged && !n.GetHasStagedChanges()) {
		return
	}
	a.staging = &stagingState{worktree: true, staged: !s.staged, cursor: -1, anchor: -1, reveal: -1}
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
				if s.staged {
					err = a.repo.ApplyLines(f.file, f.patch, true, f.indices)
				} else {
					err = a.repo.DiscardLines(f.file, f.patch, f.indices)
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

// copyDiffSelection follows lazygit's dropDiffPrefix: retain diff syntax
// for mixed additions/deletions or headers, otherwise copy code as text.
func (a *App) copyDiffSelection() {
	s := a.staging
	tr := a.repo.Common.Tr
	if s == nil || s.cursor < 0 {
		a.setStatus(tr.SelectionNotFoundInDiffToast, true)
		return
	}
	lo, hi := s.selection()
	lines := make([]string, 0, hi-lo+1)
	plus, minus, other := false, false, false
	for i := lo; i <= hi; i++ {
		text := s.lines[i].Content
		lines = append(lines, text)
		if !s.rows[i].ok || s.rows[i].kind == patch.PATCH_HEADER || s.rows[i].kind == patch.HUNK_HEADER {
			other = true
		}
		switch s.rows[i].kind {
		case patch.ADDITION:
			plus = true
		case patch.DELETION:
			minus = true
		}
	}
	if !other && !(plus && minus) {
		for i, text := range lines {
			if len(text) > 0 {
				lines[i] = text[1:]
			}
		}
	}
	text := strings.Join(lines, "\n") + "\n"
	if err := a.copyText(text); err != nil {
		a.showError(tr.CopySelectedDiffLinesToClipboard, err)
	} else {
		a.setStatus(tr.SelectedDiffLinesCopiedToast, false)
	}
}

func (a *App) scrollFocusedDiff(direction int) {
	s := a.staging
	if s == nil || a.repo.Config.Gui.WrapLinesInDiffView {
		return
	}
	s.horizontal.X = max(0, s.horizontal.X+float32(direction)*s.viewportWidth/2)
}

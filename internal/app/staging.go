package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
)

// stagingState is lazygit's staging panel: a line cursor over one side of a
// file's diff (unstaged or staged), with range and hunk selection. Line
// indices are the parsed patch's, so a selection maps straight onto
// patch.Transform.
type stagingState struct {
	path   string
	staged bool
	patch  *patch.Patch
	lines  []*patch.PatchLine
	cursor int
	anchor int // range-select anchor, -1 when off
	hunk   bool
	gen    int
	list   ui.ListState
}

func (a *App) enterStaging() {
	if a.toggleCollapsed() { // enter on a directory opens or closes it
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
	a.staging = &stagingState{path: f.Path, staged: !f.HasUnstagedChanges, anchor: -1, cursor: -1}
	a.reloadStaging()
}

func (a *App) stagingFile() *models.File {
	if a.staging == nil || a.snap == nil {
		return nil
	}
	for _, f := range a.snap.Files {
		if f.Path == a.staging.path {
			return f
		}
	}
	return nil
}

// reloadStaging re-reads the patch for the current side, switching sides
// when one runs out of changes, and leaving staging when both have.
func (a *App) reloadStaging() {
	s := a.staging
	f := a.stagingFile()
	if f == nil {
		a.staging = nil
		a.diffKey = ""
		a.loadMain()
		return
	}
	switch {
	case s.staged && !f.HasStagedChanges:
		s.staged = false
	case !s.staged && !f.HasUnstagedChanges:
		s.staged = true
	}
	s.gen++
	gen, staged := s.gen, s.staged
	a.bg(func() {
		p := a.repo.WorktreePatch(f, staged)
		a.post(func() {
			if a.staging != s || s.gen != gen {
				return
			}
			s.patch, s.lines = p, p.Lines()
			if s.cursor < 0 {
				s.cursor = s.nextChange(0, 1)
			} else {
				s.cursor = clamp(s.cursor, 0, len(s.lines)-1)
				if !s.isChange(s.cursor) {
					s.cursor = s.nextChange(s.cursor, 1)
				}
			}
			s.list.ScrollIntoView(s.cursor)
		})
	})
}

func (s *stagingState) isChange(i int) bool {
	return i >= 0 && i < len(s.lines) && s.lines[i].IsChange()
}

// nextChange finds the nearest change line from i in direction d, falling
// back to the other direction.
func (s *stagingState) nextChange(i, d int) int {
	for j := i; j >= 0 && j < len(s.lines); j += d {
		if s.isChange(j) {
			return j
		}
	}
	for j := i; j >= 0 && j < len(s.lines); j -= d {
		if s.isChange(j) {
			return j
		}
	}
	return clamp(i, 0, len(s.lines)-1)
}

func (s *stagingState) firstBodyLine() int {
	for i, l := range s.lines {
		if l.Kind != patch.PATCH_HEADER {
			return i
		}
	}
	return 0
}

// move steps over lines like lazygit's staging cursor; in hunk mode it jumps
// between hunks.
func (s *stagingState) move(d int) {
	if len(s.lines) == 0 {
		return
	}
	if s.hunk && s.patch != nil && (d == 1 || d == -1) {
		h := s.patch.HunkContainingLine(s.cursor) + d
		if h >= 0 && h < s.patch.HunkCount() {
			s.cursor = s.nextChange(s.patch.HunkStartIdx(h), 1)
		}
	} else {
		s.cursor = clamp(s.cursor+d, s.firstBodyLine(), len(s.lines)-1)
	}
	s.list.ScrollIntoView(s.cursor)
}

func (s *stagingState) toggleRange() {
	if s.anchor >= 0 {
		s.anchor = -1
	} else {
		s.anchor, s.hunk = s.cursor, false
	}
}

func (s *stagingState) extend(d int) {
	if s.anchor < 0 {
		s.anchor, s.hunk = s.cursor, false
	}
	s.move(d)
}

func (s *stagingState) toggleHunk() {
	s.hunk, s.anchor = !s.hunk, -1
}

// selection returns the selected line range (inclusive).
func (s *stagingState) selection() (int, int) {
	switch {
	case s.anchor >= 0:
		return min(s.anchor, s.cursor), max(s.anchor, s.cursor)
	case s.hunk && s.patch != nil && len(s.lines) > 0:
		h := s.patch.HunkContainingLine(s.cursor)
		return s.patch.HunkStartIdx(h), s.patch.HunkEndIdx(h)
	}
	return s.cursor, s.cursor
}

func (s *stagingState) selectedIndices() []int {
	lo, hi := s.selection()
	var out []int
	for i := lo; i <= hi; i++ {
		if s.isChange(i) {
			out = append(out, i)
		}
	}
	return out
}

func (a *App) stageSelection() {
	s := a.staging
	f := a.stagingFile()
	if s == nil || f == nil || s.patch == nil {
		return
	}
	sel := s.selectedIndices()
	if len(sel) == 0 {
		return
	}
	label := "Staging…"
	if s.staged {
		label = "Unstaging…"
	}
	p, staged := s.patch, s.staged
	s.anchor = -1
	a.run(label, func() error { return a.repo.ApplyLines(f, p, staged, sel) }, nil)
}

func (a *App) switchStagingSide() {
	s := a.staging
	f := a.stagingFile()
	if s == nil || f == nil {
		return
	}
	if (s.staged && !f.HasUnstagedChanges) || (!s.staged && !f.HasStagedChanges) {
		a.setStatus("Nothing on the other side", false)
		return
	}
	s.staged = !s.staged
	s.cursor, s.anchor = -1, -1
	s.patch, s.lines = nil, nil
	a.reloadStaging()
}

func (a *App) discardSelection() {
	s := a.staging
	f := a.stagingFile()
	if s == nil || f == nil || s.patch == nil {
		return
	}
	if s.staged {
		a.setStatus("Unstage lines before discarding them", true)
		return
	}
	sel := s.selectedIndices()
	if len(sel) == 0 {
		return
	}
	p := s.patch
	a.confirm("Discard selection", "Remove the selected changes from the working tree? This cannot be undone.", true, func() {
		s.anchor = -1
		a.run("Discarding…", func() error { return a.repo.DiscardLines(f, p, sel) }, nil)
	})
}

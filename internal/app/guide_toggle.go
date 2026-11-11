package app

import (
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
)

type guideAnchor struct {
	path, layer      string
	oldLine, newLine int
	code             bool
}

func (a *App) guideAnchor() *guideAnchor {
	if s := a.staging; s != nil && s.worktree && s.cursor >= 0 && s.cursor < len(s.rows) {
		r := s.rows[s.cursor]
		layer := "Unstaged"
		if s.staged {
			layer = "Staged"
		}
		return &guideAnchor{path: r.path, layer: layer, newLine: r.newLine, oldLine: r.oldLine, code: true}
	}
	if a.focus == panelFiles && a.fileTab == 0 {
		if f := a.selectedFile(); f != nil {
			layer := "Unstaged"
			if !f.HasUnstagedChanges && f.HasStagedChanges {
				layer = "Staged"
			}
			return &guideAnchor{path: f.Path, layer: layer}
		}
	}
	return nil
}
func (g *guideState) selectAnchor(target *guideAnchor) {
	if target != nil && g.base == "" {
		found := false
		for section, s := range g.sections {
			for hunk, idx := range s.Hunks {
				h := g.hunks[idx]
				if h.Path != target.path || h.Layer != target.layer {
					continue
				}
				// No line target: preserve the selected hunk if it already maps to this file.
				if target.newLine == 0 && target.oldLine == 0 && g.current() != nil && g.current().Path == target.path && g.current().Layer == target.layer {
					found = true
					break
				}
				matched := -1
				for i, r := range g.streams[section] {
					if r.hunk == idx && ((target.newLine > 0 && r.newLine == target.newLine) || (target.oldLine > 0 && r.oldLine == target.oldLine)) {
						matched = i
						break
					}
				}
				if target.newLine > 0 || target.oldLine > 0 {
					if matched < 0 {
						continue
					}
				}
				changed := g.section != section || g.hunk != hunk
				g.section, g.hunk = section, hunk
				if changed {
					g.revealSelected()
				}
				if matched >= 0 && g.lineCursor != matched {
					g.lineCursor = matched
					g.diffList.ScrollIntoView(matched)
				}
				g.focusCode = target.code
				found = true
				break
			}
			if found {
				break
			}
		}
	}
	if target != nil {
		g.focusCode = target.code
	}
	if h := g.current(); h != nil {
		g.entryFingerprint = h.Fingerprint
		g.entryLine = g.lineCursor
	}
}
func (a *App) hideGuide() {
	g := a.guide
	if g == nil {
		return
	}
	g.active = false
	h := g.current()
	if h == nil || g.base != "" {
		return
	}
	// An unmodified round trip leaves lazier's exact focus, selection and scroll intact.
	if h.Fingerprint == g.entryFingerprint && (!g.focusCode || g.lineCursor == g.entryLine) {
		return
	}
	a.mapGuideSelection(g, true)
}
func (a *App) mapGuideSelection(g *guideState, reload bool) {
	h := g.current()
	if h == nil || a.snap == nil || a.files == nil {
		return
	}
	internal := filetree.InternalTreePathForFilePath(h.Path, a.repo.Config.Gui.ShowRootItemInFileTree)
	i, ok := a.files.GetIndexForPath(internal)
	if !ok {
		a.files.ExpandAll()
		a.updateTreeRows()
		i, ok = a.files.GetIndexForPath(internal)
	}
	if !ok {
		if reload {
			r := a.repo
			a.bg(func() {
				snapshot := r.Load()
				a.post(func() {
					if a.repo == r && a.guide == g && !g.active {
						a.apply(snapshot)
						a.mapGuideSelection(g, false)
					}
				})
			})
		}
		return // File may no longer have working-tree changes.
	}
	a.fileTab = 0
	a.focus = panelFiles
	a.staging = nil
	a.mainFocused = false
	a.cursor[panelFiles] = i
	a.lists[panelFiles].ScrollIntoView(i)
	a.diffKey = ""
	a.loadMain()
	if !g.focusCode {
		return
	}
	target := &guideAnchor{path: h.Path, layer: h.Layer, code: true}
	rows := g.rows()
	if g.lineCursor >= 0 && g.lineCursor < len(rows) {
		target.newLine, target.oldLine = rows[g.lineCursor].newLine, rows[g.lineCursor].oldLine
	}
	a.guideReturn = target
	a.staging = &stagingState{worktree: true, staged: h.Layer == "Staged", cursor: -1, anchor: -1, reveal: -1, startOrigin: -1}
	a.reloadStaging()
}
func (a *App) restoreGuideLine(s *stagingState) {
	target := a.guideReturn
	if target == nil {
		return
	}
	a.guideReturn = nil
	for i, r := range s.rows {
		if r.path == target.path && ((target.newLine > 0 && r.newLine == target.newLine) || (target.oldLine > 0 && r.oldLine == target.oldLine)) {
			s.navigateTo(i)
			return
		}
	}
}

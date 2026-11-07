package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// A section is one scrolling stream, with a note followed by real diff rows.
// stream rows retain the original hunk and old/new coordinates for navigation.
type guideStreamRow struct {
	hunk             int // index in guideState.hunks; -1 for the section note
	line             diffLine
	oldLine, newLine int
	tokens           []guideToken
}

func (g *guideState) rebuildStreams() {
	g.streams = make([][]guideStreamRow, len(g.sections))
	g.widths = make([]int, len(g.sections))
	for section, s := range g.sections {
		rows := []guideStreamRow{{hunk: -1}}
		for _, idx := range s.Hunks {
			h := g.hunks[idx]
			_, _, parts := parseDiffRows(h.Text)
			var newNums, oldNums []int
			if len(parts) > 0 {
				newNums, oldNums = parts[0].patch.LineNumbersOfLines()
			}
			for _, line := range parseDiff(h.Text) {
				// Git's plumbing headers repeat for each hunk; retain meaningful metadata.
				if line.kind == lineMeta && (strings.HasPrefix(line.text, "index ") || strings.HasPrefix(line.text, "--- ") || strings.HasPrefix(line.text, "+++ ")) {
					continue
				}
				if line.text == "" {
					continue
				}
				row := guideStreamRow{hunk: idx, line: line}
				if line.src < len(newNums) && line.src < len(oldNums) {
					if line.kind == lineCtx || line.kind == lineDel {
						row.oldLine = oldNums[line.src]
					}
					if line.kind == lineCtx || line.kind == lineAdd {
						row.newLine = newNums[line.src]
					}
				}
				row.tokens = g.syntaxTokens(row)
				rows = append(rows, row)
				g.widths[section] = max(g.widths[section], len([]rune(strings.ReplaceAll(line.text, "\t", "    "))))
			}
		}
		g.streams[section] = rows
	}
	g.lineCursor = 0
	g.revealSelected()
}
func (g *guideState) rows() []guideStreamRow {
	if g.section < 0 || g.section >= len(g.streams) {
		return nil
	}
	return g.streams[g.section]
}
func (g *guideState) selectedIndex() int {
	if g.current() == nil {
		return -1
	}
	return g.sections[g.section].Hunks[g.hunk]
}
func (g *guideState) storyRow() int {
	n := 0
	for i, s := range g.sections {
		if i == g.section {
			return n + 1 + g.hunk
		}
		n += 1 + len(s.Hunks)
	}
	return 0
}
func (g *guideState) revealSelected() {
	idx := g.selectedIndex()
	for i, r := range g.rows() {
		if r.hunk == idx && r.line.kind != lineFile && r.line.kind != lineMeta {
			g.lineCursor = i
			g.diffList.ScrollTo(max(0, i-1), ui.Start)
			break
		}
	}
	g.list.ScrollIntoView(g.storyRow())
}
func (a *App) guideEnter() {
	g := a.guide
	if g.current() == nil {
		return
	}
	g.focusCode = true
	// Prefer actual code over a file/hunk heading.
	for i := g.lineCursor; i < len(g.rows()); i++ {
		if r := g.rows()[i]; r.oldLine > 0 || r.newLine > 0 {
			g.lineCursor = i
			g.diffList.ScrollIntoView(i)
			return
		}
	}
}
func (a *App) guideBack() {
	if a.guide.focusCode {
		a.guide.focusCode = false
		return
	}
	a.hideGuide()
}
func (a *App) guideLineMove(d int) {
	g := a.guide
	rows := g.rows()
	if len(rows) == 0 {
		return
	}
	g.lineCursor = clamp(g.lineCursor+d, 1, len(rows)-1)
	if idx := rows[g.lineCursor].hunk; idx >= 0 {
		for i, h := range g.sections[g.section].Hunks {
			if h == idx {
				g.hunk = i
				break
			}
		}
	}
	g.diffList.ScrollIntoView(g.lineCursor)
	g.list.ScrollIntoView(g.storyRow())
}

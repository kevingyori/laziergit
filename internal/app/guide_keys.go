package app

import (
	"fmt"
	"strings"
)

// guideCycle is togglePanel (Tab): story → code → diagram (when the section
// has one) → story.
func (a *App) guideCycle() {
	g := a.guide
	switch {
	case g.showDiagram && !g.diagramCodeFocus:
		a.guideFocus(0)
	case g.focusCode:
		if len(g.sections) > 0 && g.sections[g.section].Diagram != nil {
			a.guideFocus(2)
		} else {
			a.guideFocus(0)
		}
	default:
		a.guideFocus(1)
	}
}

// guideToggleRange is lazygit's range select (v) on the code: c then comments
// on the whole range.
func (a *App) guideToggleRange() {
	g := a.guide
	if !g.focusCode {
		a.guideEnter()
	}
	if g.rangeStart >= 0 {
		g.rangeStart = -1
		return
	}
	g.rangeStart = g.lineCursor
}

// guideRange is the selected code rows, inclusive; a single row without a range.
func (g *guideState) guideRange() (int, int) {
	if g.rangeStart < 0 || !g.focusCode {
		return g.lineCursor, g.lineCursor
	}
	return min(g.rangeStart, g.lineCursor), max(g.rangeStart, g.lineCursor)
}

// guideSearch searches the code of every section, starting after the cursor.
func (a *App) guideSearch() {
	g := a.guide
	a.prompt("Search", g.query, "Matches code lines in every section; n/N find the next and previous.", func(q string) {
		g.query = strings.TrimSpace(q)
		if g.query != "" {
			a.guideSearchNext(1)
		}
	})
}
func (a *App) guideSearchNext(dir int) {
	g := a.guide
	if g.query == "" || len(g.streams) == 0 {
		return
	}
	q := strings.ToLower(g.query)
	type pos struct{ section, row int }
	var all []pos
	for s, rows := range g.streams {
		for i, r := range rows {
			if r.hunk >= 0 && (r.oldLine > 0 || r.newLine > 0) && strings.Contains(strings.ToLower(r.line.text), q) {
				all = append(all, pos{s, i})
			}
		}
	}
	if len(all) == 0 {
		g.notice = fmt.Sprintf("No match for %q", g.query)
		return
	}
	// The first match past the cursor in that direction, wrapping around.
	next := 0
	if dir > 0 {
		for i, p := range all {
			if p.section > g.section || (p.section == g.section && p.row > g.lineCursor) {
				next = i
				break
			}
		}
	} else {
		next = -1
		for i := len(all) - 1; i >= 0; i-- {
			if p := all[i]; p.section < g.section || (p.section == g.section && p.row < g.lineCursor) {
				next = i
				break
			}
		}
		if next < 0 {
			next = len(all) - 1
		}
	}
	p := all[next]
	if p.section != g.section {
		g.section, g.hunk = p.section, 0
		g.showDiagram, g.diagramCodeFocus = false, false
	}
	g.focusCode = true
	a.guideSelectStream(p.row)
	g.diffList.ScrollIntoView(p.row)
	g.notice = fmt.Sprintf("Match %d of %d for %q", next+1, len(all), g.query)
}

// guideEditAtLine opens the selected line's file in your editor at that line
// (lazygit's editAtLine template).
func (a *App) guideEditAtLine() {
	g := a.guide
	rows := g.rows()
	if g.current() == nil {
		return
	}
	path, line := g.current().Path, 1
	if g.focusCode && g.lineCursor >= 0 && g.lineCursor < len(rows) && rows[g.lineCursor].newLine > 0 {
		line = rows[g.lineCursor].newLine
	}
	cmd, suspend := a.repo.Git.File.GetEditAtLineCmdStr(a.absPath(path), line)
	a.callEditor(cmd, suspend)
}

// guideCopyMenu copies the selected location, code or section title.
func (a *App) guideCopyMenu() {
	g := a.guide
	h := g.current()
	if h == nil {
		return
	}
	rows := g.rows()
	lo, hi := g.guideRange()
	location, code := h.Path, ""
	if g.focusCode && lo >= 0 && hi < len(rows) {
		var first, last int
		var lines []string
		for _, r := range rows[lo : hi+1] {
			if r.newLine > 0 {
				if first == 0 {
					first = r.newLine
				}
				last = r.newLine
			}
			if r.oldLine > 0 || r.newLine > 0 {
				lines = append(lines, r.line.text)
			}
		}
		switch {
		case first > 0 && first != last:
			location = fmt.Sprintf("%s:%d-%d", h.Path, first, last)
		case first > 0:
			location = fmt.Sprintf("%s:%d", h.Path, first)
		}
		code = strings.Join(lines, "\n")
	}
	item := func(label string, key rune, text, disabled string) menuItem {
		return menuItem{label: label, key: string(key), chord: runeBind(key), disabled: disabled, run: func() {
			if err := a.copyText(text); err != nil {
				g.err = err.Error()
				return
			}
			g.notice = label + " copied"
		}}
	}
	noCode := ""
	if code == "" {
		noCode = "Select code lines first (2 or Enter)"
	}
	a.pushOverlay(&menuOverlay{title: "Copy to clipboard", items: []menuItem{
		item("Location", 'p', location, ""),
		item("Selected lines", 'l', code, noCode),
		item("Hunk diff", 'h', h.Text, ""),
		item("Section title", 's', g.sections[g.section].Title, ""),
	}})
}

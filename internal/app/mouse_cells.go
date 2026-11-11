package app

import (
	"sort"

	"github.com/egoist/mygo/ui"
)

// cellMap records where each line of a lazygit view is drawn, so a pointer
// position can be named in lazygit's terms: view, line and column. Rows
// record their real bounds as they are built (Element.Bounds, the frame
// before); columns only where a line has elements that act on their own
// (an arrow, a name), as spans of lazygit's columns they cover.
type cellMap struct {
	rows  map[string]map[int]ui.Rect
	items map[string]map[int]int // a line's item (list row), for lines that show one
	spans map[string]map[int][]cellSpan
	panes map[string]ui.Rect
}

type cellSpan struct {
	from, to int // lazygit columns [from, to)
	rect     ui.Rect
}

func (m *cellMap) reset() { *m = cellMap{} }

// row records line of view drawn by el.
func (m *cellMap) row(view string, line int, el ui.Element) {
	r := el.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if m.rows == nil {
		m.rows = map[string]map[int]ui.Rect{}
	}
	if m.rows[view] == nil {
		m.rows[view] = map[int]ui.Rect{}
	}
	m.rows[view][line] = r
}

// span records that el draws lazygit columns [from, to) of line.
func (m *cellMap) span(view string, line, from, to int, el ui.Element) {
	r := el.Bounds()
	if r.W <= 0 || r.H <= 0 || to <= from {
		return
	}
	if m.spans == nil {
		m.spans = map[string]map[int][]cellSpan{}
	}
	if m.spans[view] == nil {
		m.spans[view] = map[int][]cellSpan{}
	}
	m.spans[view][line] = append(m.spans[view][line], cellSpan{from, to, r})
}

// pane records the content area of view, for a view with no lines.
func (m *cellMap) pane(view string, el ui.Element) {
	r := el.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if m.panes == nil {
		m.panes = map[string]ui.Rect{}
	}
	m.panes[view] = r
}

// MouseCell is the window point (DIPs) showing column col of line of
// lazygit view view (sub: the wrapped row of the line, 0 unless the view
// wraps). A column covered by a span is its element's centre; any other
// column the row's left edge, inside the row. A line the frame didn't draw
// lies its distance in rows from the nearest drawn line, which may be
// outside the view, where lazygit's cell would be outside it too.
func (a *App) MouseCell(view string, line, sub, col int) (x, y float32, ok bool) {
	pitch := float32(rowHeight)
	if view == "main" || view == "secondary" {
		pitch = diffRowHeight
	}
	for _, s := range a.cells.spans[view][line] {
		if col >= s.from && col < s.to && sub == 0 {
			return s.rect.X + s.rect.W/2, s.rect.Y + s.rect.H/2, true
		}
	}
	if r, ok := a.cells.rows[view][line]; ok {
		y := r.Y + r.H/2
		if sub > 0 || r.H > pitch*1.5 {
			y = r.Y + min(float32(sub)*pitch+pitch/2, r.H-1)
		}
		return r.X + edgeInset, y, true
	}
	rows := a.cells.rows[view]
	if len(rows) == 0 {
		if r, ok := a.cells.panes[view]; ok {
			return r.X + edgeInset, r.Y + (float32(line)+0.5)*pitch, true
		}
		return 0, 0, false
	}
	lines := make([]int, 0, len(rows))
	for l := range rows {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	near := lines[0]
	if line > lines[len(lines)-1] {
		near = lines[len(lines)-1]
	} else {
		for _, l := range lines {
			if l <= line {
				near = l
			}
		}
	}
	r := rows[near]
	if line > near {
		return r.X + edgeInset, r.Y + r.H + (float32(line-near-1)+0.5)*pitch, true
	}
	return r.X + edgeInset, r.Y - (float32(near-line)-0.5)*pitch, true
}

// edgeInset is how far inside a row's left edge a click on it lands: in the
// row's padding, clear of the grip on the pane's border.
const edgeInset = 6

// item records that line of view shows item.
func (m *cellMap) item(view string, line, item int) {
	if m.items == nil {
		m.items = map[string]map[int]int{}
	}
	if m.items[view] == nil {
		m.items[view] = map[int]int{}
	}
	m.items[view][line] = item
}

// at names the window point (x, y) in view: the line drawn there (or, off
// the drawn lines, the line that would be, counting rows from the nearest),
// its item (-1 for none) and the first column of the span under the point
// (-1 for none).
func (m *cellMap) at(view string, x, y float32) (line, item, col int) {
	rows := m.rows[view]
	if len(rows) == 0 {
		return -1, -1, -1
	}
	pitch := float32(rowHeight)
	if view == "main" || view == "secondary" {
		pitch = diffRowHeight
	}
	best, bestDist := 0, float32(-1)
	for l, r := range rows {
		if y >= r.Y && y < r.Y+r.H {
			best, bestDist = l, 0
			break
		}
		d := min(abs32(y-r.Y), abs32(y-(r.Y+r.H)))
		if bestDist < 0 || d < bestDist {
			best, bestDist = l, d
		}
	}
	line = best
	if r := rows[best]; bestDist > 0 {
		if y >= r.Y+r.H {
			line += 1 + int((y-(r.Y+r.H))/pitch)
		} else {
			line -= 1 + int((r.Y-y)/pitch)
		}
	}
	item = -1
	if i, ok := m.items[view][line]; ok {
		item = i
	}
	col = -1
	for _, s := range m.spans[view][line] {
		if x >= s.rect.X && x < s.rect.X+s.rect.W && y >= s.rect.Y && y < s.rect.Y+s.rect.H {
			col = s.from
		}
	}
	return line, item, col
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

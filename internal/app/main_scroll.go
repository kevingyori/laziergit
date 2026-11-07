package app

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// paneOrigin is lazygit's view origin of the main or secondary pane in
// cells: the line of the pane's loaded text at its top, and how many mono
// columns it is scrolled sideways.
func (a *App) paneOrigin(name string) (x, y int) {
	var st *stagingState
	if name == "main" {
		st = a.mainDiffState()
	} else if a.staging != nil && a.staging.staged {
		st = a.staging
	}
	if st != nil {
		first, _ := st.list.Visible()
		return st.originX(), max(0, first)
	}
	rows := a.diff
	first, _ := a.diffList.Visible()
	if name == "secondary" {
		rows = diffLinesOf(a.secondaryText, a.secondaryRendered)
		first, _ = a.secondaryList.Visible()
	}
	if first <= 0 || len(rows) == 0 {
		// At the top: lines the pane's header shows instead of rows (a
		// commit's preamble) are above the first row, on screen.
		return 0, 0
	}
	if first >= len(rows) { // scrolled past the bottom
		return 0, rows[len(rows)-1].src + first - (len(rows) - 1)
	}
	return 0, rows[first].src
}

// scrollMain is lazygit's scrollUpMain/scrollDownMain: the pane the focus
// is in when it is in one of them, and otherwise the pane the main section
// shows, scrolled by gui.scrollHeight lines.
func (a *App) scrollMain(dir int) {
	name := "main"
	if (a.staging != nil && a.staging.staged) || (!a.mainVisible() && a.secondaryVisible()) {
		name = "secondary"
	}
	a.scrollPane(name, dir*a.repo.Config.Gui.ScrollHeight)
}

// scrollPane is gocui's ScrollDown/ScrollUp on pane name's origin: never
// above the top, and down only while lines remain below it (all but the
// last two with gui.scrollPastBottom, else until the last line shows).
func (a *App) scrollPane(name string, d int) {
	list, rows, srcOf := a.paneRows(name)
	if list == nil || rows == 0 {
		return
	}
	_, oy := a.paneOrigin(name)
	st := a.paneState(name)
	if st != nil {
		oy = st.origin()
	}
	total := srcOf(rows-1) + 2 // the loaded text's lines and the empty one after its last newline
	if d < 0 {
		d = max(d, -oy)
	} else {
		y, margin := oy, 2
		if !a.repo.Config.Gui.ScrollPastBottom {
			y, margin = oy+a.paneInnerHeight(name), 0
		}
		scrollable := total - y
		if scrollable < 0 {
			return
		}
		d = min(d, scrollable-margin)
		if oy+d < 0 || d <= 0 {
			return
		}
	}
	if d == 0 {
		return
	}
	target := oy + d
	row := rows - 1
	for i := 0; i < rows; i++ {
		if srcOf(i) >= target {
			row = i
			break
		}
	}
	if target > srcOf(rows-1) {
		row = rows - 1 + (target - srcOf(rows-1)) // into the rows past the bottom
	}
	if st != nil {
		st.setOrigin(row)
		return
	}
	list.ScrollTo(row, ui.Start)
}

// paneState is pane name's diff with a selection, nil for a plain one.
func (a *App) paneState(name string) *stagingState {
	if name == "main" {
		return a.mainDiffState()
	}
	if a.staging != nil && a.staging.staged {
		return a.staging
	}
	return nil
}

// paneRows is pane name's list, its row count and each row's line of the
// loaded text.
func (a *App) paneRows(name string) (*ui.ListState, int, func(int) int) {
	var st *stagingState
	if name == "main" {
		st = a.mainDiffState()
	} else if a.staging != nil && a.staging.staged {
		st = a.staging
	}
	if st != nil {
		return &st.list, len(st.lines), func(i int) int { return i }
	}
	rows, list := a.diff, &a.diffList
	if name == "secondary" {
		rows, list = diffLinesOf(a.secondaryText, a.secondaryRendered), &a.secondaryList
	}
	return list, len(rows), func(i int) int { return rows[i].src }
}

// paneInnerHeight is how many diff lines pane name shows at once.
func (a *App) paneInnerHeight(name string) int {
	var st *stagingState
	if name == "main" {
		st = a.mainDiffState()
	} else if a.staging != nil && a.staging.staged {
		st = a.staging
	}
	if st != nil {
		return st.innerHeight()
	}
	return max(1, int(a.mainViewportHeight/diffRowHeight))
}

func (s *stagingState) innerHeight() int { return max(1, int(s.viewportHeight/diffRowHeight)) }

// pastBottomRows is how many empty rows follow the diff so that it can be
// scrolled past its end, as gui.scrollPastBottom lets lazygit's views.
func (s *stagingState) pastBottomRows(a *App) int {
	if !a.repo.Config.Gui.ScrollPastBottom || len(s.lines) == 0 {
		return 0
	}
	return s.innerHeight()
}

func (a *App) mainPastBottomRows() int {
	if !a.repo.Config.Gui.ScrollPastBottom || len(a.diff) == 0 {
		return 0
	}
	return a.paneInnerHeight("main")
}

// paneInnerWidth is how many mono columns of a diff line pane name shows.
func (a *App) paneInnerWidth(name string) int {
	var st *stagingState
	if name == "main" {
		st = a.mainDiffState()
	} else if a.staging != nil && a.staging.staged {
		st = a.staging
	}
	if st != nil {
		return st.innerWidth()
	}
	return max(1, int(a.mainViewportWidth/monoAdvance()))
}

// innerWidth is the text width of a row: the pane less the selection bar
// and the row's padding.
func (s *stagingState) innerWidth() int { return max(1, int((s.viewportWidth-40)/monoAdvance())) }

// MainPaneCells is the main pane's size in cells, lazygit's InnerWidth and
// InnerHeight of the main view.
func (a *App) MainPaneCells() (cols, rows int) {
	return a.paneInnerWidth("main"), a.paneInnerHeight("main")
}

// origin is the pane's top line: the one asked for this frame, else the
// one the last frame showed.
func (s *stagingState) origin() int {
	if s.originSet {
		return s.wantOrigin
	}
	first, _ := s.list.Visible()
	return max(0, first)
}

// setOrigin scrolls line y to the top of the pane.
func (s *stagingState) setOrigin(y int) {
	s.wantOrigin, s.originSet = max(0, y), true
	s.list.ScrollTo(s.wantOrigin, ui.Start)
}

// scrollIntoView is gocui's FocusPoint with scrollIntoView: a line off
// screen is brought to the middle of the pane, as far as the end allows.
func (s *stagingState) scrollIntoView(line int) {
	s.setOrigin(calculateNewOrigin(line, s.origin(), s.viewLines(), s.innerHeight()))
}

// calculateNewOrigin is gocui's.
func calculateNewOrigin(selectedLine, oldOrigin, lineCount, viewHeight int) int {
	if viewHeight >= lineCount {
		return 0
	} else if selectedLine < oldOrigin || selectedLine >= oldOrigin+viewHeight {
		return max(0, min(selectedLine-viewHeight/2, lineCount-viewHeight))
	}
	return oldOrigin
}

// viewLines is gocui's ViewLinesHeight of the diff: its lines and the empty
// one its last newline starts.
func (s *stagingState) viewLines() int { return len(s.lines) + 1 }

// unfocusedOrigin is the top line of pane name while it has no selection.
func (a *App) unfocusedOrigin(name string) int {
	_, y := a.paneOrigin(name)
	return y
}

// scrollDiffListTo puts line y of the main pane's text at its top.
func (a *App) scrollDiffListTo(y int) {
	row := len(a.diff) - 1
	for i, l := range a.diff {
		if l.src >= y {
			row = i
			break
		}
	}
	if n := len(a.diff); n > 0 && y > a.diff[n-1].src {
		row = n - 1 + y - a.diff[n-1].src
	}
	a.diffList.ScrollTo(max(0, row), ui.Start)
}

// originX is how many mono columns the pane is scrolled sideways.
func (s *stagingState) originX() int { return int(math.Round(float64(s.horizontal.X / monoAdvance()))) }

func (s *stagingState) setOriginX(x int) { s.horizontal.X = float32(max(0, x)) * monoAdvance() }

// clampOriginXToContent is gocui's ClampOriginXToContent, after a render.
func (s *stagingState) clampOriginXToContent() {
	if s.originX() == 0 {
		return
	}
	widest := 0
	for _, l := range s.lines {
		widest = max(widest, utils.StringWidth(strings.ReplaceAll(l.Content, "\t", "    ")))
	}
	s.setOriginX(min(s.originX(), max(0, widest-s.innerWidth())))
}

// scrollColumnsIntoView is gocui's: columns already on screen stay put; ones
// that fit from the left edge go back to it; otherwise xStart ends up a third
// of the way across.
func (s *stagingState) scrollColumnsIntoView(xStart, xEnd int) {
	width, ox := s.innerWidth(), s.originX()
	if xStart >= ox && xEnd <= ox+width {
		return
	}
	if xEnd <= width {
		s.setOriginX(0)
	} else {
		s.setOriginX(xStart - width/3)
	}
}

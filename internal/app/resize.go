package app

import "github.com/egoist/mygo/ui"

// Layout the user has dragged. Zero values mean lazygit's layout.
type layout struct {
	sideFrac  float32             // side column's share of the window width
	sideScale [maxWindows]float32 // multiplies a weighted side panel's weight
	logHeight float32             // command log drawer

	sideW float32                 // side column width, as last laid out
	winW  float32                 // window size, as last laid out
	winH  float32
	diffH float32 // the main section (the diff panes) height, as last laid out
	sideH [maxWindows]float32     // side panel heights, as last laid out
	drags [maxWindows + 2]resizer // one per draggable border
}

const (
	minSideWidth = 200
	minMainWidth = 320
	defaultLog   = 118
	minLog       = 24 + 17*2
	gripSize     = 7 // the pointer's target around a one-DIP hairline
)

// resizer follows one drag of a border: apply gets how far the pointer has
// moved since the drag began.
type resizer struct {
	active bool
	moved  float32
	apply  func(moved float32)
}

// grip is a border's pointer target, placed over the line once everything
// else is built so that no pane drawn after the line covers it.
type grip struct {
	line   ui.Element
	column bool
	r      *resizer
	key    int // which border, so a grip keeps its pointer state as others come and go
	begin  func() func(float32)
	reset  func()
}

// border draws a one-DIP hairline. With r set, a grip over it resizes what
// it divides: begin runs as a drag starts and returns how to apply it, and
// a double click calls reset. column says the line runs top to bottom.
func (a *App) border(c *ui.Context, p *Palette, column bool, r *resizer, begin func() func(float32), reset func()) {
	line := ui.Box(c).Shrink(0).Background(p.Hairline)
	if column {
		line.Width(1).FillHeight()
	} else {
		line.Height(1).FillWidth()
	}
	if r == nil {
		return
	}
	a.grips = append(a.grips, grip{line, column, r, a.gripKey(r), begin, reset})
}

// placeGrips puts each border's grip over where its line was last laid
// out. It runs last in the root, whose origin is the window's.
func (a *App) placeGrips(c *ui.Context, p *Palette) {
	for _, g := range a.grips {
		b := g.line.Bounds()
		if b.W == 0 && b.H == 0 {
			continue
		}
		h := ui.Box(c.Key(g.key)).Absolute()
		if g.column {
			h.Top(b.Y).Left(b.X - (gripSize-1)/2).Width(gripSize).Height(b.H).Cursor(ui.CursorResizeEW)
		} else {
			h.Top(b.Y - (gripSize-1)/2).Left(b.X).Width(b.W).Height(gripSize).Cursor(ui.CursorResizeNS)
		}
		r := g.r
		dx, dy, held := h.Dragged()
		switch {
		case h.DoubleClicked():
			r.active = false
			g.reset()
		case held:
			if !r.active {
				r.active, r.moved, r.apply = true, 0, g.begin()
			}
			d := dy
			if g.column {
				d = dx
			}
			if d != 0 {
				r.moved += d
				r.apply(r.moved)
			}
		default:
			r.active = false
		}
		// The grip lights the line it covers in the same frame the pointer
		// reaches or leaves it.
		if hovered := h.Hovered(); held || hovered {
			h.Children(func() {
				hl := ui.Box(c).Absolute().Background(p.Muted)
				if g.column {
					hl.Top(0).Bottom(0).Left((gripSize - 1) / 2).Width(1)
				} else {
					hl.Left(0).Right(0).Top((gripSize - 1) / 2).Height(1)
				}
			})
		}
	}
	a.grips = a.grips[:0]
}

// sideWidth is the side column's width in a window w wide: lazygit's
// sidePanelWidth until the user drags it.
func (a *App) sideWidth(w float32) float32 {
	frac := a.layout.sideFrac
	if frac == 0 {
		frac = float32(a.repo.Config.Gui.SidePanelWidth)
		if a.splitMainSideBySide() {
			// getMidSectionWeights: the side panel shrinks to make way for
			// two main panes (main section = 5 × side).
			frac = 1.0 / 6
		}
	}
	return clampf(w*frac, minSideWidth, w-minMainWidth)
}

// sideBorder resizes the side column against the main pane.
func (a *App) sideBorder(c *ui.Context, p *Palette) {
	w, _ := c.Size()
	a.border(c, p, true, &a.layout.drags[maxWindows], func() func(float32) {
		start := a.layout.sideW
		return func(d float32) {
			a.layout.sideFrac = clampf(start+d, minSideWidth, w-minMainWidth) / w
		}
	}, func() { a.layout.sideFrac = 0 })
}

// panelBorder divides side panel pn from the one above it. Only two panels
// that share space by weight trade it; fixed ones keep lazygit's size.
func (a *App) panelBorder(c *ui.Context, p *Palette, pn int, boxes []sideBox) {
	above := pn - 1
	if boxes[above].weight == 0 || boxes[pn].weight == 0 {
		a.border(c, p, false, nil, nil, nil)
		return
	}
	a.border(c, p, false, &a.layout.drags[pn], func() func(float32) {
		hA, hB := a.layout.sideH[above], a.layout.sideH[pn]
		wA, wB := boxes[above].weight, boxes[pn].weight
		sA, sB := a.scale(above), a.scale(pn)
		minH := rowsPx(3)
		return func(d float32) {
			sum := hA + hB
			if sum < 2*minH {
				return
			}
			v := clampf(hA+d, minH, sum-minH)
			// Keep the pair's total weight; split it as the pointer says.
			a.layout.sideScale[above] = sA * (wA + wB) * v / sum / wA
			a.layout.sideScale[pn] = sB * (wA + wB) * (sum - v) / sum / wB
		}
	}, func() { a.layout.sideScale = [maxWindows]float32{} })
}

// logBorder resizes the command log against the main pane above it.
func (a *App) logBorder(c *ui.Context, p *Palette) {
	_, h := c.Size()
	a.border(c, p, false, &a.layout.drags[maxWindows+1], func() func(float32) {
		start := a.logHeight()
		return func(d float32) {
			a.layout.logHeight = clampf(start-d, minLog, h-hintBarHeight-160)
		}
	}, func() { a.layout.logHeight = 0 })
}

func (a *App) logHeight() float32 {
	if a.layout.logHeight == 0 {
		return defaultLog
	}
	return a.layout.logHeight
}

func (a *App) gripKey(r *resizer) int {
	for i := range a.layout.drags {
		if &a.layout.drags[i] == r {
			return i
		}
	}
	return -1
}

func (a *App) scale(pn int) float32 {
	if s := a.layout.sideScale[pn]; s > 0 {
		return s
	}
	return 1
}

func clampf(v, lo, hi float32) float32 {
	return max(lo, min(v, hi))
}

// splitMainSideBySide is lazygit's splitMainPanelSideBySide: with both main
// panes shown, gui.mainPanelSplitMode "horizontal" puts them side by side,
// "vertical" stacks them, and "flexible" stacks them only in a window
// narrower than 200 columns and taller than 30 rows. A column is a mono
// column; the rows are those of a terminal whose main view had the whole
// section: its inner rows plus its frame and the options bar.
func (a *App) splitMainSideBySide() bool {
	if a.snap == nil || !a.mainVisible() || !a.secondaryVisible() {
		return false
	}
	switch a.repo.Config.Gui.MainPanelSplitMode {
	case "vertical":
		return false
	case "horizontal":
		return true
	}
	cols := int(a.layout.winW / monoAdvance())
	rows := int((a.layout.diffH-diffHeaderHeight-1)/diffRowHeight) + 3
	if a.showLog {
		rows += 3 // the extras window, a row and its frame below 40 rows
	}
	return !(cols < 200 && rows > 30)
}

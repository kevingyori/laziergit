package app

import "github.com/egoist/mygo/ui"

// Layout the user has dragged. Zero values mean lazygit's layout.
type layout struct {
	sideFrac  float32             // side column's share of the window width
	sideScale [panelCount]float32 // multiplies a weighted side panel's weight
	logHeight float32             // command log drawer

	sideW float32                 // side column width, as last laid out
	sideH [panelCount]float32     // side panel heights, as last laid out
	drags [panelCount + 2]resizer // one per draggable border
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
	lit    bool // hovered or held, as of the last frame
	moved  float32
	apply  func(moved float32)
}

// grip is a border's pointer target, placed over the line once everything
// else is built so that no pane drawn after the line covers it.
type grip struct {
	line   ui.Element
	column bool
	r      *resizer
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
	if r.lit {
		line.Background(p.Muted)
	}
	a.grips = append(a.grips, grip{line, column, r, begin, reset})
}

// placeGrips puts each border's grip over where its line was last laid
// out. It runs last in the root, whose origin is the window's.
func (a *App) placeGrips(c *ui.Context) {
	for i, g := range a.grips {
		b := g.line.Bounds()
		if b.W == 0 && b.H == 0 {
			continue
		}
		h := ui.Box(c.Key(i)).Absolute()
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
		r.lit = held || h.Hovered()
	}
	a.grips = a.grips[:0]
}

// sideWidth is the side column's width in a window w wide: lazygit's
// sidePanelWidth until the user drags it.
func (a *App) sideWidth(w float32) float32 {
	frac := a.layout.sideFrac
	if frac == 0 {
		frac = float32(a.repo.Config.Gui.SidePanelWidth)
	}
	return clampf(w*frac, minSideWidth, w-minMainWidth)
}

// sideBorder resizes the side column against the main pane.
func (a *App) sideBorder(c *ui.Context, p *Palette) {
	w, _ := c.Size()
	a.border(c, p, true, &a.layout.drags[panelCount], func() func(float32) {
		start := a.layout.sideW
		return func(d float32) {
			a.layout.sideFrac = clampf(start+d, minSideWidth, w-minMainWidth) / w
		}
	}, func() { a.layout.sideFrac = 0 })
}

// panelBorder divides side panel pn from the one above it. Only two panels
// that share space by weight trade it; fixed ones keep lazygit's size.
func (a *App) panelBorder(c *ui.Context, p *Palette, pn panel, boxes [panelCount]sideBox) {
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
	}, func() { a.layout.sideScale = [panelCount]float32{} })
}

// logBorder resizes the command log against the main pane above it.
func (a *App) logBorder(c *ui.Context, p *Palette) {
	_, h := c.Size()
	a.border(c, p, false, &a.layout.drags[panelCount+1], func() func(float32) {
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

func (a *App) scale(pn panel) float32 {
	if s := a.layout.sideScale[pn]; s > 0 {
		return s
	}
	return 1
}

func clampf(v, lo, hi float32) float32 {
	return max(lo, min(v, hi))
}

package app

import (
	"github.com/egoist/mygo/ui"
	"laziergit/internal/godraw/editor"
	"laziergit/internal/godraw/geo"
)

// Embedding chooses which of godraw's own panels a host shows over the canvas.
type Embedding struct {
	Tools      bool // the toolbar, the style panel for a selection, zoom
	Background *editor.Color
}

// EmbeddedView draws godraw inside a parent app without installing global
// shortcuts or taking keyboard focus. The host dispatches keys and label edits.
func (a *App) EmbeddedView(c *ui.Context) { a.EmbeddedViewWith(c, Embedding{}) }

// EmbeddedViewWith is EmbeddedView with godraw's panels and a host background.
func (a *App) EmbeddedViewWith(c *ui.Context, opt Embedding) {
	a.ed.BackgroundOverride = opt.Background
	a.embedded = true
	a.services = c.Services()
	a.hasSvc = true
	a.ed.DarkMode = c.Theme().Dark
	if b := a.canvasHandle.Bounds(c); b.W > 0 {
		a.ed.VW, a.ed.VH = float64(b.W), float64(b.H)
		a.ed.FitNewPage()
	}
	a.ed.ModsChanged(modsOf(c.Modifiers()))
	ui.Box(c).Fill().Clip().Children(func() {
		cv := ui.Box(c.Key("canvas")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Bind(&a.canvasHandle).Cursor(cursorOf(a.ed.CursorNow()))
		a.textEditor(c)
		cv.Label("Review diagram").Draw(a.paintCanvas).HandleInput(func(ev ui.InputEvent) bool { return a.input(c, ev) })
		if opt.Tools {
			a.embeddedTools(c)
		}
	})
	a.helpDialog(c)
}

// NudgeSelection moves selected shapes with the keyboard, sharing godraw's undo.
func (a *App) NudgeSelection(dx, dy float64) { a.ed.Nudge(geo.V(dx, dy)) }

// ScaleSelection resizes selected shapes with the keyboard, sharing godraw's undo.
func (a *App) ScaleSelection(f float64) { a.ed.ScaleSelection(f) }

// embeddedTools is godraw's toolbar, compact for a canvas that shares the
// screen: one column on the left, with styles behind a toggle so they never
// cover the drawing until asked for.
func (a *App) embeddedTools(c *ui.Context) {
	e := a.ed
	t := c.Theme()
	small := func(key, name, tip string, active bool) ui.Element {
		b := ui.ButtonBase(c.Key(key)).Size(28, 28).Radius(6).Center().Label(tip).Tooltip(tip)
		switch {
		case active:
			b.Background(t.Accent).TextColor(t.AccentText)
		case b.Hovered():
			b.Background(t.SurfaceHover)
		}
		b.Children(func() { ui.Icon(c, icon(name)).Size(15, 15) })
		return b
	}
	sep := func() { ui.Box(c).Size(18, 1).Margin(3, 0).Background(t.Border) }
	panelStyle(c, ui.Column(c.Key("embedded-tools"))).Absolute().Left(8).Top(8).Padding(3).Gap(1).AlignItems(ui.Center).Children(func() {
		if !e.ReadOnly {
			for _, tool := range []struct {
				tool      editor.Tool
				icon, tip string
			}{
				{editor.ToolSelect, "mouse-pointer-2", "Select — s"},
				{editor.ToolArrow, "arrow-up-right", "Arrow — a"},
				{editor.ToolGeo, "square", "Box — b"},
				{editor.ToolText, "type", "Text"},
				{editor.ToolNote, "sticky-note", "Note — n"},
				{editor.ToolDraw, "pencil", "Draw — p"},
			} {
				tool := tool
				small(string(tool.tool), tool.icon, tool.tip, e.Tool == tool.tool).OnClick(func() { e.SetTool(tool.tool) })
			}
			sep()
			if small("styles", "layers", "Style of the selection", a.stylesOpen).Clicked() {
				a.stylesOpen = !a.stylesOpen
			}
			small("undo", "undo-2", "Undo — u", false).OnClick(e.Undo)
			sep()
		}
		small("zoom-in", "plus", "Zoom in — +", false).OnClick(e.ZoomIn)
		small("zoom-out", "minus", "Zoom out — -", false).OnClick(e.ZoomOut)
		small("zoom-fit", "maximize", "Fit — =", false).OnClick(e.ZoomToFit)
	})
	if a.stylesOpen && !e.ReadOnly {
		a.stylePanel(c)
	}
}

package app

import (
	"github.com/egoist/mygo/ui"
	"laziergit/internal/godraw/geo"
)

// EmbeddedView draws godraw inside a parent app without installing global
// shortcuts or taking keyboard focus. The host dispatches keys and label edits.
func (a *App) EmbeddedView(c *ui.Context) {
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
	})
}

// NudgeSelection moves selected shapes with the keyboard, sharing godraw's undo.
func (a *App) NudgeSelection(dx, dy float64) { a.ed.Nudge(geo.V(dx, dy)) }

package app

import (
	"github.com/egoist/mygo/ui"
	"testing"
	"time"
)

func TestMenuFilterMouseAndKeyboard(t *testing.T) {
	a, tt, _ := newTestApp(t)
	press(tt, "?")
	m := a.topOverlay().(*menuOverlay)
	if m.subtitle(a) != "(Type to filter)" || a.textCursorVisible() {
		t.Fatal("initial filter hint/caret")
	}
	for _, r := range "co" {
		press(tt, string(r))
	}
	if !a.textCursorVisible() || m.subtitle(a) != "" {
		t.Fatal("filter not active")
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	selected := m.cursor
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	if m.cursor != selected {
		t.Fatal("text cursor moved menu selection")
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	// Line 1 follows the section heading, just as in the upstream test.
	tt.Frame()
	tt.Frame()
	x, y, ok := a.MouseCell("menu", 1, 0, 0)
	if !ok {
		t.Fatal("menu row has no recorded bounds")
	}
	tt.Advance(time.Second)
	tt.ClickAt(x, y)
	tt.Frame()
	tt.Frame()
	if a.topOverlay() != m || m.cursor != 0 || m.filter != "co" {
		t.Fatalf("single click executed or changed filter: cursor=%d filter=%q", m.cursor, m.filter)
	}
	press(tt, "m")
	if m.filter != "com" {
		t.Fatalf("filter after click: %q", m.filter)
	}
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	press(tt, "x")
	if m.filter != "coxm" {
		t.Fatalf("filter caret insertion: %q", m.filter)
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyU)
	tt.Frame()
	for _, r := range "whitespace" {
		press(tt, string(r))
	}
	if len(m.visible()) != 1 || m.tooltip(a) == "" {
		t.Fatal("whitespace filter/tooltip")
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/menu-filter-whitespace.png")
	}
	press(tt, "esc")
	if a.topOverlay() != m || m.visible()[m.cursor].label != a.repo.Common.Tr.ToggleWhitespaceInDiffView || m.filterShown() {
		t.Fatal("escape did not preserve selected item and hide filter")
	}
	press(tt, "esc")
	if a.topOverlay() != nil {
		t.Fatal("second escape did not close menu")
	}
	press(tt, "?")
	if a.topOverlay().(*menuOverlay).filterShown() {
		t.Fatal("new menu retained filter")
	}
}

func TestMenuDoubleClickExecutes(t *testing.T) {
	a, tt, _ := newTestApp(t)
	ran := false
	m := &menuOverlay{title: "Choose", items: []menuItem{{label: "Run", run: func() { ran = true }}}}
	a.pushOverlay(m)
	tt.Frame()
	tt.Frame()
	x, y, ok := a.MouseCell("menu", 0, 0, 0)
	if !ok {
		t.Fatal("missing row bounds")
	}
	tt.ClickAt(x, y)
	tt.Frame()
	if ran {
		t.Fatal("single click executed")
	}
	tt.ClickAt(x, y)
	tt.Frame()
	if !ran || a.topOverlay() != nil {
		t.Fatal("double click did not execute")
	}
}

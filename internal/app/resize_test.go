package app

import (
	"testing"

	"laziergit/internal/testrepo"
)

func drag(tt interface {
	Press(x, y float32)
	Move(x, y float32)
	Release(x, y float32)
	Frame()
}, x0, y0, x1, y1 float32) {
	tt.Move(x0, y0)
	tt.Frame()
	tt.Press(x0, y0)
	tt.Frame()
	for i := 1; i <= 4; i++ {
		f := float32(i) / 4
		tt.Move(x0+(x1-x0)*f, y0+(y1-y0)*f)
		tt.Frame()
	}
	tt.Release(x1, y1)
	tt.Frame()
}

func TestDragResizesBorders(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	tt.Frame()

	// Side column against the main pane.
	w0 := a.layout.sideW
	drag(tt, w0+3, 400, w0+103, 400)
	if got := a.layout.sideW; got < w0+95 || got > w0+105 {
		t.Fatalf("side width %v, want about %v", got, w0+100)
	}

	// Branches against commits (both share space by weight).
	tt.Frame()
	hB, hC := a.layout.sideH[panelBranches], a.layout.sideH[panelCommits]
	var y float32
	for pn := range panelBranches + 1 {
		y += a.layout.sideH[pn] + 1
	}
	drag(tt, 100, y+2.5, 100, y+63)
	tt.Frame()
	if got := a.layout.sideH[panelBranches]; got < hB+50 || got > hB+70 {
		t.Fatalf("branches height %v, want about %v", got, hB+60)
	}
	if got := a.layout.sideH[panelCommits]; got > hC-50 {
		t.Fatalf("commits height %v, want about %v", got, hC-60)
	}

	// Command log against the main pane.
	var h float32 = 800
	logTop := h - hintBarHeight - 1 - a.logHeight() - 1
	drag(tt, w0+400, logTop+0.5, w0+400, logTop-80)
	if got := a.logHeight(); got < defaultLog+70 || got > defaultLog+90 {
		t.Fatalf("log height %v, want about %v", got, defaultLog+80)
	}

	// A double click restores lazygit's layout.
	x := a.layout.sideW
	tt.ClickAt(x, 400)
	tt.ClickAt(x, 400)
	tt.Frame()
	if a.layout.sideFrac != 0 {
		t.Fatalf("double click kept side width %v", a.layout.sideW)
	}
}

// A border lights while the pointer is over it and goes dark as soon as the
// pointer leaves, also when focus changes which borders are draggable.
func TestBorderHoverFollowsPointer(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	tt.Frame()
	px := func(x, y float32) uint32 {
		r, _, _, _ := tt.Image().At(int(x), int(y)).RGBA()
		return r
	}
	sideX := a.layout.sideW + 0.5
	var panelY float32
	for pn := range panelBranches + 1 {
		panelY += a.layout.sideH[pn] + 1
	}
	panelY -= 1 // the line itself
	side, panelLine := px(sideX, 600), px(100, panelY)

	tt.Move(sideX, 300)
	if px(sideX, 600) == side {
		t.Fatal("side border not lit under the pointer")
	}
	tt.Move(sideX+200, 300)
	if px(sideX, 600) != side {
		t.Fatal("side border stayed lit after the pointer left")
	}

	tt.Move(100, panelY)
	if px(100, panelY) == panelLine {
		t.Fatal("panel border not lit under the pointer")
	}
	tt.Type("5") // focus stash: the set of draggable borders changes
	tt.Frame()
	tt.Move(100, panelY-100)
	tt.Type("4")
	if px(100, panelY) != panelLine || px(sideX, 600) != side {
		t.Fatal("a border stayed lit after the pointer left")
	}
}

package app

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/jesseduffield/lazygit/pkg/config"
)

// sideBox starts as a terminal row size or weight; sideBoxes converts it to
// a native height and row spacing before rendering. A weight is retained for
// sharing what the fixed panels leave.
type sideBox struct {
	size      float32
	weight    float32
	rowHeight float32
	foot      float32
}

const (
	panelHead = 28 // a side panel's title row
	panelFoot = 6  // padding under its list
)

// rowsPx is the pixel height of a lazygit box of n rows: its two frame rows
// become the title row and the foot.
func rowsPx(n int) float32 {
	if n <= 1 {
		return panelHead
	}
	return panelHead + float32(n-2)*rowHeight + panelFoot
}

// winBox is what sizing a side window needs: the tab it shows, whether it
// is lazygit's CurrentSideWindow, and its content rows.
type winBox struct {
	view    sideView
	focused bool
	count   int
}

// fixedThree says lazygit keeps window w at three rows: status always, stash
// unless focused (keyed off the tab the window shows, as lazygit does).
func (w winBox) fixedThree() bool {
	return w.view == vStatus || (w.view == vStash && !w.focused)
}

// sidePanelBoxes ports lazygit's sidePanelChildren
// (window_arrangement_helper.go): every panel weighs the same, status and an
// unfocused stash keep three rows, and a window too short for that squashes
// every panel but the focused one. height is the side column's height in
// terminal rows.
func sidePanelBoxes(cfg *config.UserConfig, wins []winBox, height int) []sideBox {
	boxes := make([]sideBox, len(wins))
	rows := height
	windows := len(wins)
	minNormal := min(28, 28*windows/5)
	minTallSquashed := min(21, 21*windows/5)

	if rows < minNormal {
		squashed := float32(1)
		if rows >= minTallSquashed {
			squashed = 3
		}
		for i, w := range wins {
			if w.focused {
				boxes[i] = sideBox{weight: 1}
			} else {
				boxes[i] = sideBox{size: squashed}
			}
		}
		return boxes
	}

	if cfg.Gui.ShrinkSidePanelsToContent {
		if b, ok := shrinkToContentBoxes(cfg, wins, height); ok {
			return b
		}
	}

	for i, w := range wins {
		switch {
		case w.view == vStatus:
			boxes[i] = sideBox{size: 3}
		case w.fixedThree():
			boxes[i] = sideBox{size: 3}
		case w.focused && cfg.Gui.ExpandFocusedSidePanel:
			boxes[i] = sideBox{weight: float32(cfg.Gui.ExpandedSidePanelWeight)}
		default:
			boxes[i] = sideBox{weight: 1}
		}
	}
	return boxes
}

// shrinkToContentBoxes ports lazygit's shrinkToContentSidePanelBoxes: panels
// whose content fits their weighted share get exactly their content, and the
// rest share what is left.
func shrinkToContentBoxes(cfg *config.UserConfig, wins []winBox, height int) ([]sideBox, bool) {
	boxes := make([]sideBox, len(wins))
	type flexible struct {
		pn      int
		desired float32
		weight  float32
		capped  bool
	}
	var flex []*flexible
	available := float32(height)
	for pn, w := range wins {
		if w.fixedThree() {
			boxes[pn] = sideBox{size: 3}
			available -= 3
			continue
		}
		weight := float32(1)
		if cfg.Gui.ExpandFocusedSidePanel && w.focused {
			weight = float32(cfg.Gui.ExpandedSidePanelWeight)
		}
		flex = append(flex, &flexible{pn: pn, desired: float32(max(w.count+1, 2) + 2), weight: weight})
	}
	if len(flex) == 0 || available <= 0 {
		return boxes, false
	}

	remaining := available
	for {
		var total float32
		for _, f := range flex {
			if !f.capped {
				total += f.weight
			}
		}
		if total == 0 {
			break
		}
		var capped []*flexible
		for _, f := range flex {
			if !f.capped && f.desired*total <= remaining*f.weight {
				capped = append(capped, f)
			}
		}
		if len(capped) == 0 {
			break
		}
		for _, f := range capped {
			f.capped = true
			remaining -= f.desired
		}
	}

	anyUncapped := false
	for _, f := range flex {
		anyUncapped = anyUncapped || !f.capped
	}
	for i, f := range flex {
		switch {
		case !anyUncapped:
			// Everything fits: share the leftover equally.
			size := int(f.desired) + int(remaining)/len(flex)
			if i < int(remaining)%len(flex) {
				size++
			}
			boxes[f.pn] = sideBox{weight: float32(size)}
		case f.capped:
			boxes[f.pn] = sideBox{size: f.desired}
		default:
			boxes[f.pn] = sideBox{weight: f.weight}
		}
	}
	return boxes, true
}

// allocateSideRows uses lazygit's boxlayout, including integer rounding and
// remainder distribution. Pixel flex allocation cannot preserve these viewports.
func allocateSideRows(boxes []sideBox, height int) []int {
	children := make([]*boxlayout.Box, len(boxes))
	for i, b := range boxes {
		children[i] = &boxlayout.Box{Window: strconv.Itoa(i), Size: int(b.size), Weight: int(b.weight)}
	}
	dims := boxlayout.ArrangeWindows(&boxlayout.Box{Direction: boxlayout.ROW, Children: children}, 0, 0, 1, height)
	rows := make([]int, len(boxes))
	for i := range rows {
		d := dims[strconv.Itoa(i)]
		rows[i] = max(0, d.Y1-d.Y0+1)
	}
	return rows
}

// sideBoxes allocates terminal rows first, then converts them to native heights.
func (a *App) sideBoxes(c *ui.Context) []sideBox {
	focus := a.focusWindow()
	wins := make([]winBox, len(a.windows))
	for i := range a.windows {
		v := a.activeView(i)
		wins[i] = winBox{view: v, focused: i == focus}
		a.withView(v, func() { wins[i].count = a.count(v.group()) })
	}
	_, h := c.Size()
	// Use the main pane's text grid as the common terminal row model. Its
	// native header replaces two terminal frame rows; the bottom bar replaces
	// one. The command log similarly replaces its terminal-sized extras box.
	midHeight := h - hintBarHeight - 1
	mainHeight := midHeight
	if a.showLog {
		mainHeight -= a.logHeight() + 1
	}
	rows := max(0, int((mainHeight-diffHeaderHeight-1)/diffRowHeight)) + 2
	if a.showLog {
		extras := 3
		if rows+3 >= 39 {
			extras = a.repo.Config.Gui.CommandLogSize + 2
		}
		rows += extras
	}
	// getMidSectionWeights: an enlarged side view above the main section
	// has weight one against two, with boxlayout's first remainder row.
	if a.portrait() {
		rows = (rows + 2) / 3
	}
	boxes := sidePanelBoxes(a.repo.Config, wins, rows)
	if a.enlarged() && focus >= 0 {
		for i := range boxes {
			boxes[i] = sideBox{}
		}
		boxes[focus] = sideBox{weight: 1}
	}

	allocated := allocateSideRows(boxes, rows)
	// Preserve the native chrome, then fit whole content rows into the column.
	// Smaller windows compress row spacing instead of revealing extra rows.
	available := midHeight
	if a.portrait() {
		available = (midHeight - 1) / 3
	}
	shown, content := 0, 0
	for i, n := range allocated {
		if a.sidePanelShown(i) {
			shown++
			content += max(0, n-2)
		}
	}
	available -= float32(max(0, shown-1))
	unit := float32(rowHeight)
	if content > 0 {
		unit = min(unit, max(1, (available-float32(shown)*(panelHead+panelFoot))/float32(content)))
	}
	spare := max(0, available-float32(shown)*(panelHead+panelFoot)-float32(content)*unit)
	for i, n := range allocated {
		if !a.sidePanelShown(i) {
			continue
		}
		body := max(0, n-2)
		foot := float32(panelFoot)
		// Spare pixels belong below the list, never to a partial extra row.
		if i == len(boxes)-1 || a.enlarged() {
			foot += spare
		}
		boxes[i].size = panelHead + float32(body)*unit + foot
		boxes[i].rowHeight, boxes[i].foot = unit, foot
	}

	// Drag overrides trade native pixels between flexible panels. Their
	// weights must start at the rounded native heights, so dragging doesn't
	// jump back to an unrounded proportional layout on its first pixel.
	available, total := float32(0), float32(0)
	custom := false
	for i := range boxes {
		if boxes[i].weight > 0 {
			boxes[i].weight = boxes[i].size
			available += boxes[i].size
			total += boxes[i].weight * a.scale(i)
			custom = custom || a.scale(i) != 1
		}
	}
	if custom && total > 0 {
		for i := range boxes {
			if boxes[i].weight > 0 {
				boxes[i].size = available * boxes[i].weight * a.scale(i) / total
			}
		}
	}

	return boxes
}

package app

import "github.com/jesseduffield/lazygit/pkg/config"

// sideBox is one side panel's height: a fixed size in pixels, or a weight
// sharing what the fixed panels leave.
type sideBox struct {
	size   float32
	weight float32
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
// pixels.
func sidePanelBoxes(cfg *config.UserConfig, wins []winBox, height float32) []sideBox {
	boxes := make([]sideBox, len(wins))
	rows := int(height / rowHeight)
	windows := len(wins)
	minNormal := min(28, 28*windows/5)
	minTallSquashed := min(21, 21*windows/5)

	if rows < minNormal {
		squashed := rowsPx(1)
		if rows >= minTallSquashed {
			squashed = rowsPx(3)
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
			boxes[i] = sideBox{size: rowsPx(3)}
		case w.fixedThree():
			boxes[i] = sideBox{size: rowsPx(3)}
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
func shrinkToContentBoxes(cfg *config.UserConfig, wins []winBox, height float32) ([]sideBox, bool) {
	boxes := make([]sideBox, len(wins))
	type flexible struct {
		pn      int
		desired float32
		weight  float32
		capped  bool
	}
	var flex []*flexible
	available := height
	for pn, w := range wins {
		if w.fixedThree() {
			boxes[pn] = sideBox{size: rowsPx(3)}
			available -= rowsPx(3)
			continue
		}
		weight := float32(1)
		if cfg.Gui.ExpandFocusedSidePanel && w.focused {
			weight = float32(cfg.Gui.ExpandedSidePanelWeight)
		}
		flex = append(flex, &flexible{pn: pn, desired: rowsPx(max(w.count+1, 2) + 2), weight: weight})
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
	for _, f := range flex {
		switch {
		case !anyUncapped:
			// Everything fits: share the leftover equally.
			boxes[f.pn] = sideBox{size: f.desired + remaining/float32(len(flex))}
		case f.capped:
			boxes[f.pn] = sideBox{size: f.desired}
		default:
			boxes[f.pn] = sideBox{weight: f.weight}
		}
	}
	return boxes, true
}

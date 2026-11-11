package app

import (
	"slices"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/config"
)

func TestSidePanelBoxes(t *testing.T) {
	cfg := config.GetDefaultConfig()
	counts := [panelCount]int{1, 0, 2, 500, 0}
	tall := float32(40 * rowHeight)

	// Default: equal weights, whatever the content; status and an unfocused
	// stash keep three rows.
	b := sidePanelBoxes(cfg, defaultWins(panelCommits, counts), tall)
	want := []sideBox{{size: rowsPx(3)}, {weight: 1}, {weight: 1}, {weight: 1}, {size: rowsPx(3)}}
	if !slices.Equal(b, want) {
		t.Fatalf("default: %+v", b)
	}
	if b := sidePanelBoxes(cfg, defaultWins(panelStash, counts), tall); b[panelStash] != (sideBox{weight: 1}) {
		t.Fatalf("focused stash: %+v", b[panelStash])
	}

	// Accordion.
	cfg.Gui.ExpandFocusedSidePanel = true
	if b := sidePanelBoxes(cfg, defaultWins(panelCommits, counts), tall); b[panelCommits] != (sideBox{weight: 2}) || b[panelFiles] != (sideBox{weight: 1}) {
		t.Fatalf("accordion: %+v", b)
	}
	cfg.Gui.ExpandFocusedSidePanel = false

	// Short window: everything but the focused panel squashes.
	b = sidePanelBoxes(cfg, defaultWins(panelBranches, counts), 24*rowHeight)
	if b[panelBranches] != (sideBox{weight: 1}) || b[panelCommits] != (sideBox{size: rowsPx(3)}) {
		t.Fatalf("squashed: %+v", b)
	}
	if b := sidePanelBoxes(cfg, defaultWins(panelBranches, counts), 10*rowHeight); b[panelFiles] != (sideBox{size: rowsPx(1)}) {
		t.Fatalf("very short: %+v", b)
	}

	// Shrink to content: short panels fit their content, the long one scrolls.
	cfg.Gui.ShrinkSidePanelsToContent = true
	b = sidePanelBoxes(cfg, defaultWins(panelCommits, counts), tall)
	if b[panelFiles] != (sideBox{size: rowsPx(4)}) || b[panelBranches] != (sideBox{size: rowsPx(5)}) || b[panelCommits] != (sideBox{weight: 1}) {
		t.Fatalf("shrink: %+v", b)
	}
}

// defaultWins is the default side panels, focus on panel focus.
func defaultWins(focus panel, counts [panelCount]int) []winBox {
	views := []sideView{vStatus, vFiles, vBranches, vCommits, vStash}
	wins := make([]winBox, len(views))
	for i, v := range views {
		wins[i] = winBox{view: v, focused: v.group() == focus, count: counts[v.group()]}
	}
	return wins
}

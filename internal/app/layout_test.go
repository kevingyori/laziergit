package app

import (
	"slices"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gui/controllers/helpers"
)

func TestSidePanelBoxes(t *testing.T) {
	cfg := config.GetDefaultConfig()
	counts := [panelCount]int{1, 0, 2, 500, 0}
	tall := 40

	// Default: equal weights, whatever the content; status and an unfocused
	// stash keep three rows.
	b := sidePanelBoxes(cfg, defaultWins(panelCommits, counts), tall)
	want := []sideBox{{size: 3}, {weight: 1}, {weight: 1}, {weight: 1}, {size: 3}}
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
	b = sidePanelBoxes(cfg, defaultWins(panelBranches, counts), 24)
	if b[panelBranches] != (sideBox{weight: 1}) || b[panelCommits] != (sideBox{size: 3}) {
		t.Fatalf("squashed: %+v", b)
	}
	if b := sidePanelBoxes(cfg, defaultWins(panelBranches, counts), 10); b[panelFiles] != (sideBox{size: 1}) {
		t.Fatalf("very short: %+v", b)
	}

	// Shrink to content: short panels fit their content, the long one scrolls.
	cfg.Gui.ShrinkSidePanelsToContent = true
	b = sidePanelBoxes(cfg, defaultWins(panelCommits, counts), tall)
	if b[panelFiles] != (sideBox{size: 4}) || b[panelBranches] != (sideBox{size: 5}) || b[panelCommits] != (sideBox{weight: 1}) {
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

// Compare the native allocator to lazygit across thresholds, panel counts,
// active tabs, focused stash/status, accordion weights and shrink-to-content.
func TestSideRowAllocationMatchesLazygit(t *testing.T) {
	for _, panels := range [][][]sideView{defaultSidePanels(), {{vFiles}, {vBranches}, {vCommits}}, {{vFiles, vStatus}, {vBranches}, {vStash}, {vCommits}, {vReflog}, {vTags}}} {
		for _, shrink := range []bool{false, true} {
			for _, weight := range []int{1, 2, 5} {
				cfg := config.GetDefaultConfig()
				cfg.Gui.PortraitMode = "never"
				cfg.Gui.ShowBottomLine = false
				cfg.Gui.ShrinkSidePanelsToContent = shrink
				cfg.Gui.ExpandFocusedSidePanel = weight != 1
				cfg.Gui.ExpandedSidePanelWeight = weight
				cfg.Gui.SidePanels = nil
				for _, tabs := range panels {
					names := []string{}
					for _, v := range tabs {
						names = append(names, sideViewNames[v])
					}
					cfg.Gui.SidePanels = append(cfg.Gui.SidePanels, names)
				}
				for focus := range panels {
					for height := 6; height <= 100; height++ {
						wins := make([]winBox, len(panels))
						counts := map[string]int{}
						for i, tabs := range panels {
							wins[i] = winBox{view: tabs[0], focused: i == focus, count: i * i}
							counts[sideViewNames[tabs[0]]] = i * i
						}
						dims := helpers.GetWindowDimensions(helpers.WindowArrangementArgs{
							Width: 120, Height: height, UserConfig: cfg,
							CurrentWindow: sideViewNames[panels[focus][0]], CurrentSideWindow: sideViewNames[panels[focus][0]],
							ActiveViewForWindow:    func(w string) string { return w },
							ContentHeightForWindow: func(w string) int { return counts[w] },
						})
						got := allocateSideRows(sidePanelBoxes(cfg, wins, height), height)
						for i, tabs := range panels {
							d := dims[sideViewNames[tabs[0]]]
							want := max(0, d.Y1-d.Y0+1)
							if got[i] != want {
								t.Fatalf("height=%d focus=%d shrink=%v weight=%d panel=%s got=%v want=%d", height, focus, shrink, weight, sideViewNames[tabs[0]], got, want)
							}
						}
					}
				}
			}
		}
	}
}

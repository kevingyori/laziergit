package app

import (
	"github.com/egoist/mygo/ui"
	"laziergit/internal/repo"
)

// loadSecondary snapshots the selected node before any git I/O. Both panes
// are available even while focus remains in the side panel.
func (a *App) loadSecondary() {
	a.secondaryGen++
	gen := a.secondaryGen
	a.secondaryText = ""
	a.secondaryTitle = ""
	a.mainTitle = a.repo.Common.Tr.DiffTitle
	switch a.focus {
	case panelCommits, panelStash:
		a.mainTitle = a.repo.Common.Tr.Patch
	case panelBranches:
		a.mainTitle = a.repo.Common.Tr.LogTitle
		if a.branchTab == 2 || (a.browse(panelBranches) != nil && a.browse(panelBranches).ref != nil) {
			a.mainTitle = a.repo.Common.Tr.Patch
		}
	}
	if a.focus == panelFiles && a.fileTab == 0 && a.snap != nil {
		n := a.selectedFileNode()
		if n == nil {
			return
		}
		always := a.repo.Config.Gui.SplitDiff == "always"
		if n.GetHasUnstagedChanges() || always || !n.GetHasStagedChanges() {
			a.mainTitle = a.repo.Common.Tr.UnstagedChanges
		} else {
			a.mainTitle = ""
		}
		if !n.GetHasStagedChanges() && !always {
			return
		}
		paths := repo.DiffPathsForNode(n.Raw(), a.files.GetRoot().Raw(), a.files.GetAllFiles(), a.files.IsFiltering())
		r := a.repo
		a.secondaryTitle = r.Common.Tr.StagedChanges
		a.bg(func() {
			text := r.WorktreeNodeViewDiff(n, paths, true)
			a.post(func() {
				if a.repo == r && a.secondaryGen == gen {
					a.secondaryText = text
				}
			})
		})
		return
	}
	if a.customPatch != nil {
		a.secondaryText = a.customPatch.preview
		a.secondaryTitle = a.repo.Common.Tr.CustomPatch
	}
}

// Paired panes use independent scroll positions. Tab and clicking the
// preview switch focus, with the selected side's existing keyboard grammar.
func (a *App) secondaryPane(c *ui.Context, p *Palette) {
	lines := parseDiff(a.secondaryText)
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Background(p.Panel).Children(func() {
		header := ui.Row(c).PaddingX(20).PaddingY(10).Gap(10).Cursor(ui.CursorPointer)
		if header.Clicked() {
			if a.staging == nil {
				a.focusMainView()
			}
			if a.staging != nil && !a.staging.staged {
				a.switchStagingSide()
			}
		}
		header.Children(func() {
			if l := a.focusMainJumpLabel("secondary"); l != "" {
				keycap(c, p, a.keys.focusMain.Label())
			}
			ui.Text(c, a.secondaryTitle).FontWeight(600)
			keycap(c, p, a.keys.togglePanel.Label())
		})
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		list := ui.List(c, &a.secondaryList, len(lines), func(i int) { diffLineRow(c, p, lines[i]) }).Grow(1).MinHeight(0)
		a.secondaryViewportHeight = list.Bounds().H
	})
}

// focusMainJumpLabel is lazygit's TitlePrefix of pane ("main" or
// "secondary"): the label of the key that focuses the diff, on the pane that
// key focuses — the secondary one while it is the only one shown.
func (a *App) focusMainJumpLabel(pane string) string {
	keys := a.repo.Config.Keybinding.Universal.FocusMainView
	if !a.repo.Config.Gui.ShowPanelJumps || len(keys) == 0 {
		return ""
	}
	on := "main"
	if !a.mainVisible() && a.secondaryVisible() {
		on = "secondary"
	}
	if pane != on {
		return ""
	}
	return "[" + keys[0] + "]"
}

func (a *App) mainVisible() bool {
	if a.focus == panelFiles && a.fileTab == 0 && a.snap != nil {
		n := a.selectedFileNode()
		return n == nil || n.GetHasUnstagedChanges() || !n.GetHasStagedChanges() || a.repo.Config.Gui.SplitDiff == "always"
	}
	return true
}
func (a *App) secondaryVisible() bool {
	if a.staging != nil && a.staging.staged {
		return true
	}
	if a.focus == panelFiles && a.fileTab == 0 && a.snap != nil {
		n := a.selectedFileNode()
		return n != nil && (n.GetHasStagedChanges() || a.repo.Config.Gui.SplitDiff == "always")
	}
	return a.customPatch != nil && a.secondaryText != ""
}
func (a *App) diffPanes(c *ui.Context, p *Palette) {
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
		if a.mainVisible() {
			if s := a.staging; s != nil && !s.staged {
				a.stagingPane(c, p)
			} else if source := a.mainDiffState(); source != nil {
				a.diffSelectionPane(c, p, source, false)
			} else {
				a.mainPane(c, p)
			}
		}
		if a.secondaryVisible() {
			if a.mainVisible() {
				ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
			}
			if s := a.staging; s != nil && s.staged {
				a.stagingPane(c, p)
			} else {
				a.secondaryPane(c, p)
			}
		}
	})
}
func (a *App) modeInfo() string {
	if info := a.diffingInfo(); info != "" {
		return info
	}
	if a.customPatch != nil {
		return a.repo.Common.Tr.BuildingPatch
	}
	if info := a.filteringInfo(); info != "" {
		return info
	}
	if info := a.cherryPickInfo(); info != "" {
		return info
	}
	if a.snap != nil && a.snap.Operation != "" {
		return a.snap.StatusTitle
	}
	if a.bisectInfo().Started() {
		return a.repo.Common.Tr.Bisect.Bisecting
	}
	return ""
}

func (a *App) mainDiffState() *stagingState {
	if a.staging == nil {
		return nil
	}
	if !a.staging.staged {
		return a.staging
	}
	return a.sourceStaging
}

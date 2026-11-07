package app

// screenMode is lazygit's ScreenMode: normal, half (the focused view takes
// half the window) and full (it takes all of it).
type screenMode int

const (
	screenNormal screenMode = iota
	screenHalf
	screenFull
)

// parseScreenMode is lazygit's parseScreenModeArg (gui.screenMode).
func parseScreenMode(s string) screenMode {
	switch s {
	case "half":
		return screenHalf
	case "full":
		return screenFull
	}
	return screenNormal
}

// nextScreenMode and prevScreenMode are lazygit's ScreenModeActions: they
// cycle normal → half → full. Rows that show more when enlarged (authors,
// upstreams) follow on the next frame.
func (a *App) nextScreenMode() { a.screenMode = (a.screenMode + 1) % 3 }
func (a *App) prevScreenMode() { a.screenMode = (a.screenMode + 2) % 3 }

// enlarged is lazygit's ScreenMode != SCREEN_NORMAL, which lists render
// their full descriptions for.
func (a *App) enlarged() bool { return a.screenMode != screenNormal }

// mainSideFocused says the main or secondary view has the keys (lazygit's
// CurrentWindow is main or secondary).
func (a *App) mainSideFocused() bool { return a.mainFocused || a.staging != nil }

// portrait is lazygit's shouldUsePortraitMode as far as the app goes: half
// screen with gui.enlargedSideViewLocation "top" stacks the side panel over
// the main view.
func (a *App) portrait() bool {
	return a.screenMode == screenHalf && a.repo.Config.Gui.EnlargedSideViewLocation == "top" && !a.mainSideFocused()
}

// sideShown and mainSectionShown are lazygit's getMidSectionWeights: in half
// and full screen the focused main view hides the side panels, and in full
// screen a focused side panel hides the main views.
func (a *App) sideShown() bool { return a.screenMode == screenNormal || !a.mainSideFocused() }
func (a *App) mainSectionShown() bool {
	return a.screenMode != screenFull || a.mainSideFocused()
}

// sidePanelShown is lazygit's sidePanelChildren in half and full screen:
// only the focused side panel keeps its window.
func (a *App) sidePanelShown(pn panel) bool { return a.screenMode == screenNormal || pn == a.focus }

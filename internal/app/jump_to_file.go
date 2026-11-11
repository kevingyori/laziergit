package app

// jumpToFileMenu is lazygit's jump-to-file menu over the files of the diff
// in the main view: focused, the selection goes to the chosen file; from the
// panel the diff belongs to, only the diff scrolls, the panel keeps the focus.
func (a *App) jumpToFileMenu(focused bool) {
	tr := a.repo.Common.Tr
	var sections []diffSection
	if focused {
		if a.staging == nil {
			return
		}
		sections = a.staging.sections
	} else {
		_, sections = diffRowsOf(a.diffText, a.diffRendered, a.repo.Git.RepoPaths.WorktreePath())
	}
	if len(sections) == 0 {
		return
	}
	if len(sections) == 1 {
		a.setStatus(tr.OnlyOneFileInDiff, false)
		return
	}
	title := tr.JumpToFileInDiff
	if focused {
		title = tr.JumpToFile
	}
	items := make([]menuItem, len(sections))
	for i, sec := range sections {
		start := sec.start
		items[i] = menuItem{label: sec.path, run: func() { a.jumpToSection(focused, start) }}
	}
	items = append(items, menuItem{label: tr.Cancel, run: func() {}})
	a.pushOverlay(&menuOverlay{title: title, items: items, filterable: true, filterAsYouType: true, noItemKeys: true})
}

// jumpToSection shows the file of the diff starting at line start: focused,
// the selection goes there; otherwise only the diff scrolls.
func (a *App) jumpToSection(focused bool, start int) {
	if focused {
		if s := a.staging; s != nil {
			s.alignFileTop(start)
			s.navigateTo(start)
		}
		return
	}
	// The section start is a line of the loaded text, as is the origin;
	// the last screenful of lines is as far as the view scrolls.
	height := a.paneInnerHeight("main")
	lines := len(a.diff)
	if n := len(a.diff); n > 0 {
		lines = a.diff[n-1].src + 1
	}
	limit := lines + 1 - height
	if !a.repo.Config.Gui.ScrollPastBottom {
		limit = lines - height
	}
	a.scrollDiffListTo(min(start, max(0, limit)))
}

// alignFileTop is lazygit's PlaceNavigationTarget with alignTop: a jump to a
// file reads it from its start, at the left edge.
func (s *stagingState) alignFileTop(t int) {
	s.setOriginX(0)
	if oy, h := s.origin(), s.innerHeight(); t < oy || t >= oy+h {
		s.setOrigin(min(t, max(0, s.viewLines()-h)))
	}
}

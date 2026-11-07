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
		items[i] = menuItem{label: sec.path, run: func() {
			if focused {
				if s := a.staging; s != nil {
					s.alignFileTop(start)
					s.navigateTo(start)
				}
				return
			}
			height := a.paneInnerHeight("main")
			a.scrollDiffListTo(min(start, max(0, len(a.diff)+1-height)))
		}}
	}
	items = append(items, menuItem{label: tr.Cancel, run: func() {}})
	a.pushOverlay(&menuOverlay{title: title, items: items, filterable: true, filterAsYouType: true, noItemKeys: true})
}

// alignFileTop is lazygit's PlaceNavigationTarget with alignTop: a jump to a
// file reads it from its start, at the left edge.
func (s *stagingState) alignFileTop(t int) {
	s.setOriginX(0)
	if oy, h := s.origin(), s.innerHeight(); t < oy || t >= oy+h {
		s.setOrigin(min(t, max(0, s.viewLines()-h)))
	}
}

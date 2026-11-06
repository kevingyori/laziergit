package app

// paneOrigin is lazygit's view origin of the main or secondary pane in
// cells: the line of the pane's loaded text at its top, and how many mono
// columns it is scrolled sideways.
func (a *App) paneOrigin(name string) (x, y int) {
	var st *stagingState
	if name == "main" {
		st = a.mainDiffState()
	} else if a.staging != nil && a.staging.staged {
		st = a.staging
	}
	if st != nil {
		first, _ := st.list.Visible()
		return int(st.horizontal.X / monoAdvance()), max(0, first)
	}
	rows := a.diff
	first, _ := a.diffList.Visible()
	if name == "secondary" {
		rows = parseDiff(a.secondaryText)
		first, _ = a.secondaryList.Visible()
	}
	if first < 0 || first >= len(rows) {
		return 0, 0
	}
	return 0, rows[first].src
}

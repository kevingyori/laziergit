package app

import (
	"github.com/egoist/mygo/ui"

	"laziergit/internal/repo"
)

// What each area of the window is drawn from, as repo parts. An area whose
// parts have never arrived for this repository (the first load after
// opening or switching to it) draws a skeleton; a later refresh keeps
// showing the data it already has.
//
// rows are the parts a side view's rows and count are drawn from; main the
// parts its main view's content is read from, which reload it when they
// arrive again.
var viewParts = [sideViewCount]struct{ rows, main repo.PartKind }{
	vStatus:     {repo.StatePart | repo.HeadPart, repo.StatePart | repo.BranchesPart | repo.CommitsPart | repo.RemotesPart | repo.TagsPart},
	vFiles:      {repo.FilesPart, repo.StatePart | repo.FilesPart},
	vWorktrees:  {repo.WorktreesPart, repo.WorktreesPart},
	vSubmodules: {repo.SubmodulesPart, repo.SubmodulesPart | repo.FilesPart},
	vBranches:   {repo.BranchesPart, repo.BranchesPart},
	vRemotes:    {repo.RemotesPart, repo.RemotesPart},
	vTags:       {repo.TagsPart, repo.TagsPart},
	vCommits:    {repo.CommitsPart, repo.CommitsPart},
	vReflog:     {repo.ReflogPart, repo.ReflogPart},
	vStash:      {repo.StashPart, repo.StashPart},
}

// scopeParts are the parts a scope's actions read: until they have arrived,
// its keys do nothing. Scopes missing here (navigation, the main view,
// drill-downs that only open once data is there) need nothing.
var scopeParts = map[scope]repo.PartKind{
	scopeStatus:     repo.StatePart | repo.BranchesPart,
	scopeFiles:      repo.StatePart | repo.FilesPart,
	scopeBranches:   repo.StatePart | repo.BranchesPart | repo.RemotesPart,
	scopeCommits:    repo.StatePart | repo.BranchesPart | repo.CommitsPart,
	scopeStash:      repo.StatePart | repo.StashPart,
	scopeRemotes:    repo.StatePart | repo.RemotesPart,
	scopeTags:       repo.StatePart | repo.TagsPart,
	scopeWorktrees:  repo.StatePart | repo.WorktreesPart,
	scopeSubmodules: repo.StatePart | repo.SubmodulesPart | repo.FilesPart,
	scopeReflog:     repo.StatePart | repo.ReflogPart,
}

// globalParts is what the global actions that act on repository state read
// (push and pull read the checked-out branch's upstream, undo the reflog).
const globalParts = repo.AllParts

// loaded reports whether every part in kinds has arrived for this repository.
func (a *App) loaded(kinds repo.PartKind) bool {
	return kinds == 0 || a.raw.Has(kinds)
}

func (a *App) viewLoaded(v sideView) bool { return a.loaded(viewParts[v].rows) }

// whenLoaded wraps a global action that reads repository state: before its
// parts arrive the key does nothing.
func (a *App) whenLoaded(kinds repo.PartKind, run func()) func() {
	return func() {
		if a.loaded(kinds) {
			run()
		}
	}
}

// gatedRun is how an action runs from a key or a click on its hint: not
// at all while its scope's parts haven't arrived.
func (a *App) gatedRun(act action) func() {
	return func() {
		if a.loaded(scopeParts[act.scope]) {
			act.run()
		}
	}
}

// mainReads reports whether a part's arrival can change what the main view
// shows for the focused side view.
func (a *App) mainReads(kind repo.PartKind) bool {
	if a.commitFiles != nil || a.refBrowse != nil || a.diffing.ref != "" {
		return true
	}
	return viewParts[a.focusedSideView()].main&kind != 0
}

// mainPending: nothing has been shown in the main view of this repository
// yet, and something will be.
func (a *App) mainPending() bool {
	return a.shownKey == "" && a.staging == nil && (!a.loaded(viewParts[a.focusedSideView()].rows) || a.diffKey != "" && a.diffKey != "empty")
}

// Skeleton geometry: bars sit where text would, in the rows' own rhythm.
var skeletonWidths = []float32{0.62, 0.48, 0.71, 0.4, 0.55, 0.66, 0.36, 0.58}

// skeletonBar is one neutral rounded bar standing in for a line of text.
func skeletonBar(c *ui.Context, p *Palette, w, h float32) ui.Element {
	return ui.Box(c).Width(w).Height(h).Radius(h / 2).Background(p.Skeleton).Shrink(0)
}

// skeletonRows fills a side list's box with placeholder rows of the list's
// own height and padding.
func skeletonRows(c *ui.Context, p *Palette, v sideView, height, rowH float32) {
	n := int(height / rowH)
	if v == vStatus {
		n = min(n, 1)
	}
	ui.Column(c).Height(max(0, height)).Shrink(0).ClipY().Children(func() {
		for i := range n {
			ui.Row(c).Height(rowH).PaddingX(12).Gap(8).AlignItems(ui.Center).MarginX(6).FillWidth().Children(func() {
				switch v {
				case vCommits, vReflog:
					skeletonBar(c, p, 50, 8) // the hash column
				case vStash:
					skeletonBar(c, p, 12, 8)
				case vFiles:
					skeletonBar(c, p, 14, 8) // the status letters
				}
				ui.Box(c).Grow(1).Height(8).Children(func() {
					ui.Box(c).WidthPercent(skeletonWidths[(i+int(v))%len(skeletonWidths)] * 100).Height(8).Radius(4).Background(p.Skeleton)
				})
				if v == vCommits || v == vBranches || v == vReflog || v == vStash {
					skeletonBar(c, p, 22, 8) // the date or recency
				}
			})
		}
	})
}

// skeletonDiff stands in for the main view's lines.
func skeletonDiff(c *ui.Context, p *Palette) {
	ui.Column(c).PaddingX(20).PaddingY(10).Gap(diffRowHeight - 8).FillWidth().ClipY().Children(func() {
		for i := range 12 {
			ui.Box(c).WidthPercent(skeletonWidths[(i*3)%len(skeletonWidths)] * 90).Height(8).Radius(4).Background(p.Skeleton)
		}
	})
}

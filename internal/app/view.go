package app

import (
	"fmt"
	"laziergit/internal/keymap"
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"github.com/stefanhaller/git-todo-parser/todo"

	"laziergit/internal/graph"
)

// View builds the window. One focusable root owns the keyboard; nothing
// inside it is focusable, so lazygit's keymap works wherever the pointer
// has been.
func (a *App) View(c *ui.Context) {
	p := palette(c.Theme().Dark)
	// The root takes the keyboard back whenever no text field needs it:
	// overlays that close and open in one frame don't hand focus back.
	if !a.wantsTextFocus() && !a.rootH.Focused(c) {
		a.rootH.Focus()
	}
	ui.Column(c).Fill().Background(p.Bg).FontSize(sizeBody).TextColor(p.Text).
		Focusable().AutoFocus().FocusRing(false).Bind(&a.rootH).
		InputMethod(false).TextCaret(ui.Rect{}).
		HandleInput(a.handleInput).
		Children(func() {
			ui.Row(c).Grow(1).MinHeight(0).Children(func() {
				w, _ := c.Size()
				a.layout.sideW = a.sideWidth(w)
				ui.Column(c).Width(a.layout.sideW).Shrink(0).FillHeight().Background(p.Panel).Children(func() {
					boxes := a.sideBoxes(c)
					for pn := range panelCount {
						if pn > 0 {
							a.panelBorder(c, p, pn, boxes)
						}
						a.section(c.Key(pn), p, pn, boxes[pn])
					}
				})
				a.sideBorder(c, p)
				ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
					a.diffPanes(c, p)
					if a.showLog {
						a.logDrawer(c, p)
					}
				})
			})
			a.searchBar(c, p)
			a.hintBar(c, p)
			a.placeGrips(c, p)
			a.overlayLayer(c, p)
		})
}

// sideBoxes sizes the side panels the way lazygit does (see layout.go).
func (a *App) sideBoxes(c *ui.Context) [panelCount]sideBox {
	var counts [panelCount]int
	for pn := range panelCount {
		counts[pn] = a.count(pn)
	}
	_, h := c.Size()
	// The hint bar and its hairline sit under the side column, and a
	// hairline divides each pair of panels.
	h -= hintBarHeight + 1 + float32(panelCount-1)
	boxes := sidePanelBoxes(a.repo.Config, a.focus, counts, h)
	for pn := range panelCount {
		boxes[pn].weight *= a.scale(pn)
	}
	return boxes
}

func (a *App) section(c *ui.Context, p *Palette, pn panel, box sideBox) {
	focused := a.focus == pn
	col := ui.Column(c).MinHeight(0).Clip()
	if box.weight > 0 {
		col.Grow(box.weight)
	} else {
		col.Height(box.size).Shrink(0)
	}
	a.layout.sideH[pn] = col.Bounds().H
	col.Children(func() {
		head := ui.Row(c).Height(28).PaddingX(12).Gap(6).AlignItems(ui.Center)
		if head.Clicked() {
			a.focusPanel(pn)
		}
		head.Children(func() {
			titleColor := p.Muted
			if focused {
				titleColor = p.Accent
			}
			title := panelTitles[pn]
			if pn == panelFiles {
				title = a.fileTabTitle()
			}
			if pn == panelBranches {
				title = a.branchTabTitle()
			}
			if pn == panelCommits && a.commitFiles != nil {
				title = "Commit files"
			}
			if b := a.browse(pn); b != nil {
				title := fmt.Sprintf(a.repo.Common.Tr.RemoteBranchesDynamicTitle, b.remote)
				if b.ref != nil {
					title = a.subCommitsTitle()
				}
				ui.Text(c, strings.ToUpper(title)).FontSize(sizeCaption).FontWeight(650).TextColor(titleColor)
			} else if pn == panelBranches || pn == panelFiles || (pn == panelCommits && a.commitFiles == nil) {
				labels := []string{"Branches", "Remotes", "Tags"}
				selected := a.branchTab
				if pn == panelFiles {
					labels = []string{"Files", "Worktrees", "Submodules"}
					selected = a.fileTab
				}
				if pn == panelCommits {
					labels = []string{"Commits", "Reflog"}
					selected = a.commitTab
				}
				for i, label := range labels {
					tab := ui.Row(c).PaddingX(3)
					if tab.Clicked() {
						a.focusPanel(pn)
						a.switchTab(i - selected)
					}
					tab.Children(func() {
						col := p.Faint
						if selected == i {
							col = titleColor
						}
						ui.Text(c, strings.ToUpper(label)).FontSize(sizeCaption).FontWeight(650).TextColor(col)
					})
				}
			} else {
				ui.Text(c, strings.ToUpper(title)).FontSize(sizeCaption).FontWeight(650).LetterSpacing(0.7).TextColor(titleColor)
			}
			if pn == panelCommits && a.commitFiles != nil {
				ui.Text(c, a.commitFiles.commit.ShortHash()).Font(monoFont).FontSize(sizeCaption).TextColor(p.Accent)
			}
			if pn != panelStatus {
				ui.Textf(c, "%d", a.count(pn)).FontSize(sizeCaption).TextColor(p.Faint).FontFeatures("tnum")
			}
			if pn == panelFiles && a.fileTab == 0 && a.snap != nil {
				staged := 0
				for _, f := range a.snap.Files {
					if f.HasStagedChanges {
						staged++
					}
				}
				if staged > 0 {
					ui.Textf(c, "· %d staged", staged).FontSize(sizeCaption).TextColor(p.Staged)
				}
			}
			ui.Box(c).Grow(1)
			if pn == panelCommits && a.snap != nil && a.snap.Operation != "" {
				if ui.Text(c, a.snap.OperationTitle+" · "+keymap.Parse(a.repo.Config.Keybinding.Universal.CreateRebaseOptionsMenu).Label()).FontSize(sizeCaption).TextColor(p.Muted).Cursor(ui.CursorPointer).Clicked() {
					a.operationMenu()
				}
			}
			keycap(c, p, fmt.Sprint(int(pn)+1))
		})
		if pn == panelCommits && a.emptyCommitFiles() {
			ui.Text(c, emptyCommitFilesLabel).PaddingX(18).PaddingY(6).TextColor(p.Muted)
		}
		ui.List(c, &a.lists[pn], a.count(pn), func(i int) {
			if pn == panelCommits && a.commitFiles == nil && a.snap.Operation == "Rebasing" {
				ui.Column(c).FillWidth().Children(func() {
					if i == 0 {
						ui.Text(c, a.repo.Common.Tr.PendingRebaseTodosSectionHeader).FontSize(sizeCaption).TextColor(p.Muted).PaddingX(18).PaddingY(6)
					}
					if i == models.HeadCommitIdx(a.snap.Commits) {
						ui.Text(c, a.repo.Common.Tr.CommitsSectionHeader).FontSize(sizeCaption).TextColor(p.Muted).PaddingX(18).PaddingY(6)
					}
					a.row(c, p, pn, i)
				})
			} else {
				a.row(c, p, pn, i)
			}
		}).Grow(1).MinHeight(0).Padding(0, 0, 6, 0)
	})
}

func (a *App) row(c *ui.Context, p *Palette, pn panel, i int) {
	sel := a.inSelection(pn, i)
	row := ui.Row(c).Height(rowHeight).PaddingX(12).Gap(8).AlignItems(ui.Center).Radius(5).MarginX(6)
	active := a.focus == pn && a.staging == nil && !a.mainFocused
	switch {
	case sel && active:
		row.Background(p.AccentSoft)
	case sel:
		row.Background(p.Idle)
	case row.Hovered():
		row.Background(p.Hover)
	}
	if row.Clicked() {
		if a.focus != pn || a.staging != nil || a.mainFocused {
			a.focusPanel(pn)
		}
		a.cursor[pn] = i
		a.loadMain()
	}
	if row.DoubleClicked() {
		switch pn {
		case panelFiles:
			if a.fileTab == 1 {
				a.openWorktree()
			} else if a.fileTab == 2 {
				a.openSubmodule()
			} else {
				a.enterStaging()
			}
		case panelCommits:
			if a.commitFiles != nil {
				if !a.toggleCollapsed() {
					a.focusMainView()
				}
			} else if a.commitsView() == cvReflog {
				a.enterRef()
			} else {
				a.enterCommitFiles()
			}
		case panelBranches:
			a.enterRef()
		}
	}
	if pn == panelCommits && a.commitsView() == cvCommits && !sel {
		a.hoverCard(c, p, row, a.snap.Commits[i])
	}
	row.Children(func() {
		switch pn {
		case panelStatus:
			a.statusRow(c, p)
		case panelFiles:
			if a.fileTab == 1 {
				w := a.snap.Worktrees[i]
				if w.IsCurrent {
					ui.Text(c, "●").TextColor(p.Accent)
				}
				ui.Text(c, w.Name).SingleLine().Grow(1).Shrink(1)
				ref := w.Branch
				if ref == "" {
					ref = "detached"
				}
				ui.Text(c, ref).SingleLine().Shrink(1).TextColor(p.Muted)
				return
			}
			if a.fileTab == 2 {
				ui.Text(c, a.snap.Submodules[i].Path).SingleLine().Grow(1)
				return
			}
			r := a.fileRows[i]
			indent(c, r.depth)
			if n := a.files.Get(i); n.File == nil {
				dirRow(c, p, r, n.GetHasStagedChanges(), n.GetHasUnstagedChanges())
			} else {
				fileRow(c, p, n.File, r.name, a.files.InTreeMode())
			}
		case panelBranches:
			if s := a.browse(panelBranches); s != nil {
				if s.ref != nil {
					commitRow(c, p, s.commits[i])
				} else {
					ui.Text(c, s.branches[i].Name).SingleLine().Grow(1)
				}
				return
			}
			if a.branchTab == 1 {
				ui.Text(c, a.snap.Remotes[i].Name).SingleLine().Grow(1)
				return
			}
			if a.branchTab == 2 {
				ui.Text(c, a.snap.Tags[i].Name).SingleLine().Grow(1)
				return
			}
			b := a.snap.Branches[i]
			branchRow(c, p, b, a.shownPR(b))
		case panelCommits:
			if a.commitFiles != nil {
				r := a.commitFiles.rows[i]
				if cp := a.customPatch; cp != nil && cp.to == a.commitFiles.commit.Hash() {
					node := a.commitFiles.tree.Get(i)
					whole := node.EveryFile(func(f *models.CommitFile) bool { return cp.statuses[f.Path] == patch.WHOLE })
					any := node.SomeFile(func(f *models.CommitFile) bool { return cp.statuses[f.Path] != patch.UNSELECTED })
					mark := " "
					if whole {
						mark = "●"
					} else if any {
						mark = "◐"
					}
					ui.Text(c, mark).FontSize(12).TextColor(p.Staged).Width(10).Shrink(0)
				}
				indent(c, r.depth)
				if n := a.commitFiles.tree.Get(i); n.File == nil {
					dirRow(c, p, r, false, false)
				} else {
					commitFileRow(c, p, n.File, r.name, a.commitFiles.tree.InTreeMode())
				}
				return
			}
			if v := a.commitsView(); v != cvCommits {
				if v == cvSubCommits {
					commitRow(c, p, a.refBrowse.commits[i])
				} else {
					commitRow(c, p, a.snap.Reflog[i])
				}
				return
			}
			if i < len(a.snap.Graph) && !a.snap.Commits[i].IsTODO() {
				graphCell(c, p, a.snap.Graph[i])
			}
			if a.snap.Operation == "Rebasing" && models.IsHeadCommit(a.snap.Commits, i) {
				ui.Text(c, "← HEAD").Font(monoFont).FontSize(sizeCaption).TextColor(p.Accent).Shrink(0)
			}
			commitRow(c, p, a.snap.Commits[i])
		case panelStash:
			s := a.snap.Stash[i]
			ui.Textf(c, "%d", s.Index).Font(monoFont).FontSize(12).TextColor(p.Faint).Width(16)
			ui.Text(c, s.Name).SingleLine().Grow(1).Shrink(1)
			ui.Text(c, s.Recency).FontSize(12).TextColor(p.Faint)
		}
	})
}

func (a *App) statusRow(c *ui.Context, p *Palette) {
	ui.Text(c, path.Base(a.snap.Root)).FontWeight(600)
	if a.snap.Operation != "" {
		ui.Text(c, a.snap.Operation).FontSize(sizeCaption).TextColor(p.Conflict)
	}
	if h := a.snap.Head; h != nil {
		ui.Text(c, "→").TextColor(p.Faint)
		ui.Text(c, h.Name).TextColor(p.Accent).SingleLine().Shrink(1)
		aheadBehind(c, p, h)
	}
}

// fileRow draws a file: its two status letters and its name. In tree mode
// the name is the path below its directory row; flat, the directory follows
// the name, faint.
func fileRow(c *ui.Context, p *Palette, f *models.File, name string, inTree bool) {
	st := f.ShortStatus + "  "
	xColor, yColor := p.Staged, p.Unstaged
	switch {
	case f.HasMergeConflicts:
		xColor, yColor = p.Conflict, p.Conflict
	case !f.Tracked:
		xColor, yColor = p.Untracked, p.Untracked
	}
	ui.RichText(c,
		ui.Span{Text: st[:1], Font: monoFont, Size: 12, Color: xColor, Weight: 600},
		ui.Span{Text: st[1:2], Font: monoFont, Size: 12, Color: yColor, Weight: 600},
	).Width(18)
	nameColor := p.Text
	if f.HasStagedChanges && !f.HasUnstagedChanges {
		nameColor = p.Staged
	}
	pathRow(c, p, name, inTree, nameColor)
}

// pathRow draws a file's name: as given in a tree, or as name then faint
// directory in a flat list.
func pathRow(c *ui.Context, p *Palette, name string, inTree bool, col ui.Color) {
	if inTree || strings.Contains(name, " → ") {
		ui.Text(c, name).TextColor(col).SingleLine().Shrink(1)
		return
	}
	dir, base := path.Split(name)
	ui.Text(c, base).TextColor(col).SingleLine().Shrink(0)
	if dir != "" {
		ui.Text(c, strings.TrimSuffix(dir, "/")).TextColor(p.Faint).SingleLine().Shrink(1)
	}
}

func branchRow(c *ui.Context, p *Palette, b *models.Branch, pr *models.GithubPullRequest) {
	mark, markColor := "", p.Faint
	if b.Head {
		mark, markColor = "●", p.Accent
	}
	ui.Text(c, mark).TextColor(markColor).FontSize(9).Width(10)
	name := ui.Text(c, b.Name).SingleLine().Shrink(1)
	if b.Head {
		name.FontWeight(600)
	}
	aheadBehind(c, p, b)
	if pr != nil {
		ui.Textf(c, "#%d", pr.Number).FontSize(12).TextColor(prColor(p, pr.State)).FontFeatures("tnum")
	}
	ui.Box(c).Grow(1)
	if b.Head {
		return // lazygit shows "*" as the checked-out branch's recency
	}
	ui.Text(c, b.Recency).FontSize(12).TextColor(p.Faint).FontFeatures("tnum")
}

// prColor is lazygit's pull-request state colour.
func prColor(p *Palette, state string) ui.Color {
	switch state {
	case "OPEN":
		return p.Staged
	case "MERGED":
		return p.MergedPR
	case "CLOSED":
		return p.Conflict
	}
	return p.Faint // draft
}

func prStateLabel(state string) string {
	switch state {
	case "OPEN":
		return "Open"
	case "MERGED":
		return "Merged"
	case "CLOSED":
		return "Closed"
	case "DRAFT":
		return "Draft"
	}
	return strings.ToLower(state)
}

func aheadBehind(c *ui.Context, p *Palette, b *models.Branch) {
	if b.UpstreamGone {
		ui.Text(c, "gone").FontSize(11).TextColor(p.Conflict)
		return
	}
	if b.AheadForPull != "" && b.AheadForPull != "0" && b.AheadForPull != "?" {
		ui.Text(c, "↑"+b.AheadForPull).FontSize(12).TextColor(p.Unpushed).FontFeatures("tnum")
	}
	if b.BehindForPull != "" && b.BehindForPull != "0" && b.BehindForPull != "?" {
		ui.Text(c, "↓"+b.BehindForPull).FontSize(12).TextColor(p.Accent).FontFeatures("tnum")
	}
}

const (
	laneWidth = 11
	maxLanes  = 10
)

// graphCell draws one row of the commit graph as vector lanes: straight
// where a lane holds, an S-curve where it shifts, a dot at the commit.
func graphCell(c *ui.Context, p *Palette, g graph.Row) {
	lanes := min(g.Width, maxLanes)
	ui.Box(c).Width(float32(lanes) * laneWidth).Height(rowHeight).Shrink(0).Clip().
		Draw(func(pt *ui.Painter, r ui.Rect) {
			x := func(l int16) float32 { return r.X + float32(l)*laneWidth + laneWidth/2 }
			top, mid, bot := r.Y, r.Y+r.H/2, r.Y+r.H
			seg := func(x0, y0, x1, y1 float32, col ui.Color) {
				path := new(ui.Path).MoveTo(x0, y0)
				if x0 == x1 {
					path.LineTo(x1, y1)
				} else {
					my := (y0 + y1) / 2
					path.CubeTo(x0, my, x1, my, x1, y1)
				}
				pt.StrokePath(path, 1.6, col)
			}
			var dot ui.Color
			for _, pp := range g.Pipes {
				col := p.Lanes[pp.Color%uint32(len(p.Lanes))]
				switch pp.Kind {
				case graph.Continues:
					seg(x(pp.From), top, x(pp.To), bot, col)
				case graph.Terminates:
					seg(x(pp.From), top, x(pp.To), mid, col)
				case graph.Starts:
					if pp.From == g.Commit && dot == (ui.Color{}) {
						dot = col
					}
					seg(x(pp.From), mid, x(pp.To), bot, col)
				}
			}
			cx := x(g.Commit)
			if g.Merge {
				pt.FillPath(new(ui.Path).Circle(cx, mid, 4), p.Panel)
				pt.StrokePath(new(ui.Path).Circle(cx, mid, 3.2), 1.6, dot)
			} else {
				pt.FillPath(new(ui.Path).Circle(cx, mid, 3.6), dot)
			}
		})
}

func commitRow(c *ui.Context, p *Palette, cm *models.Commit) {
	hashColor := p.Pushed
	switch cm.Status {
	case models.StatusUnpushed:
		hashColor = p.Unpushed
	case models.StatusMerged:
		hashColor = p.Merged
	}
	ui.Text(c, cm.ShortHash()).Font(monoFont).FontSize(12).TextColor(hashColor).Width(58).Shrink(0)
	switch {
	case cm.Status == models.StatusConflicted:
		ui.Text(c, cm.Action.String()+"  ← CONFLICT").Font(monoFont).FontSize(12).TextColor(p.Conflict).Shrink(0)
	case cm.IsTODO():
		color := p.Accent
		if cm.Action == todo.Drop {
			color = p.Conflict
		}
		ui.Text(c, todoActionLabel(cm)).Font(monoFont).FontSize(12).TextColor(color).MinWidth(48).Shrink(0)
	}
	for _, tag := range cm.Tags {
		ui.Text(c, tag).FontSize(11).TextColor(p.Accent).Background(p.AccentSoft).Radius(4).PaddingX(5).Shrink(0)
	}
	ui.Text(c, cm.Name).SingleLine().Grow(1).Shrink(1)
	if cm.UnixTimestamp != 0 {
		ui.Text(c, ago(cm.UnixTimestamp)).FontSize(12).TextColor(p.Faint).FontFeatures("tnum").Shrink(0)
	}
}

func (a *App) mainPane(c *ui.Context, p *Palette) {
	pane := ui.Column(c).Grow(1).MinHeight(0).FillWidth().Background(p.Panel)
	if a.mainFocused {
		pane.InsetShadow(0, 0, 0, 1.5, p.Accent)
	}
	pane.Children(func() {
		ui.Column(c).PaddingX(20).PaddingY(14).Gap(4).Children(func() { a.mainHeader(c, p) })
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		ui.List(c, &a.diffList, len(a.diff), func(i int) { diffLineRow(c, p, a.diff[i]) }).
			Grow(1).MinHeight(0).PaddingY(8)
	})
}

func (a *App) mainHeader(c *ui.Context, p *Palette) {
	if a.mainTitle != "" {
		ui.Text(c, a.mainTitle).FontSize(sizeCaption).TextColor(p.Muted)
	}
	title := func(s string) { ui.Text(c, s).FontSize(sizeTitle).FontWeight(600).SingleLine() }
	meta := func(build func()) {
		ui.Row(c).Gap(10).AlignItems(ui.Center).FontSize(12).TextColor(p.Muted).Children(build)
	}
	if a.snap == nil || a.count(a.focus) == 0 {
		empty := map[panel]string{panelFiles: "Working tree clean", panelStash: "No stash entries",
			panelBranches: "No " + strings.ToLower(a.branchTabTitle()), panelCommits: "No commits yet"}[a.focus]
		if a.focus == panelFiles && a.fileTab != 0 {
			title(a.fileTabTitle())
			empty = "No " + strings.ToLower(a.fileTabTitle())
		} else if a.focus == panelBranches {
			title(a.branchTabTitle())
		} else if a.focus == panelCommits && a.commitsView() == cvReflog {
			title(a.commitTabTitle())
			empty = "No reflog history"
		} else {
			title(panelTitles[a.focus])
		}
		meta(func() { ui.Text(c, empty) })
		return
	}
	i := a.cursor[a.focus]
	switch a.focus {
	case panelStatus:
		title(path.Base(a.snap.Root))
		meta(func() { ui.Text(c, a.snap.Root).SingleLine() })
	case panelFiles:
		if a.fileTab == 1 {
			w := a.snap.Worktrees[i]
			title(w.Name)
			meta(func() {
				ui.Text(c, w.Path).SingleLine()
				if w.IsMain {
					ui.Text(c, "main")
				}
				if w.IsCurrent {
					ui.Text(c, "current")
				}
				if w.IsPathMissing {
					ui.Text(c, "missing").TextColor(p.Conflict)
				}
			})
			return
		}
		if a.fileTab == 2 {
			s := a.snap.Submodules[i]
			title(s.Name)
			meta(func() { ui.Text(c, s.Path).SingleLine() })
			return
		}
		n := a.files.Get(i)
		if n.File == nil {
			title(n.GetPath() + "/")
			meta(func() { ui.Text(c, dirState(n)).TextColor(p.Unstaged) })
			return
		}
		f := n.File
		dir, name := path.Split(f.Path)
		title(name)
		meta(func() {
			if dir != "" {
				ui.Text(c, dir).SingleLine()
			}
			ui.Text(c, fileState(f)).TextColor(p.Unstaged)
		})
	case panelBranches:
		if s := a.browse(panelBranches); s != nil {
			if s.ref != nil {
				cm := s.commits[i]
				title(cm.Name)
				meta(func() { ui.Text(c, cm.ShortHash()).Font(monoFont).TextColor(p.Accent); ui.Text(c, cm.AuthorName) })
			} else {
				title(s.branches[i].FullName())
			}
			return
		}
		if a.branchTab == 1 {
			rm := a.snap.Remotes[i]
			title(rm.Name)
			meta(func() { ui.Text(c, strings.Join(rm.Urls, " · ")) })
			return
		}
		if a.branchTab == 2 {
			tag := a.snap.Tags[i]
			title(tag.Name)
			meta(func() { ui.Text(c, tag.Message) })
			return
		}
		b := a.snap.Branches[i]
		title(b.Name)
		meta(func() {
			if b.UpstreamBranch != "" {
				ui.Text(c, "tracks "+b.UpstreamRemote+"/"+b.UpstreamBranch)
			} else {
				ui.Text(c, "no upstream")
			}
			if !b.Head {
				ui.Text(c, b.Recency)
			}
		})
		if pr := a.shownPR(b); pr != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).FontSize(12).Children(func() {
				ui.Text(c, prStateLabel(pr.State)).FontWeight(600).TextColor(prColor(p, pr.State))
				if pr.ChecksState != "" && pr.State == "OPEN" {
					ui.Text(c, "checks "+strings.ToLower(pr.ChecksState)).TextColor(p.Muted)
				}
				ui.Text(c, pr.Title).SingleLine().Shrink(1)
				ui.Textf(c, "#%d", pr.Number).TextColor(p.Muted).FontFeatures("tnum")
			})
		}
	case panelCommits:
		if cf := a.commitFiles; cf != nil {
			n := cf.tree.Get(i)
			if n.File == nil {
				title(n.GetPath() + "/")
				meta(func() {
					ui.Text(c, "in "+cf.commit.ShortHash()).Font(monoFont).TextColor(p.Accent)
					ui.Text(c, cf.commit.Name).SingleLine().Shrink(1)
				})
				return
			}
			f := n.File
			dir, name := path.Split(f.Path)
			title(name)
			meta(func() {
				if dir != "" {
					ui.Text(c, dir).SingleLine()
				}
				ui.Text(c, "in "+cf.commit.ShortHash()).Font(monoFont).TextColor(p.Accent)
				ui.Text(c, cf.commit.Name).SingleLine().Shrink(1)
			})
			return
		}
		cm := a.selectedCommit()
		title(cm.Name)
		meta(func() {
			ui.Text(c, cm.ShortHash()).Font(monoFont).TextColor(p.Accent)
			ui.Text(c, cm.AuthorName)
			ui.Text(c, time.Unix(cm.UnixTimestamp, 0).Format("Jan 2, 2006 15:04"))
		})
	case panelStash:
		s := a.snap.Stash[i]
		title(s.Name)
		meta(func() { ui.Textf(c, "stash@{%d}", s.Index).Font(monoFont) })
	}
}

func diffLineRow(c *ui.Context, p *Palette, l diffLine) {
	if l.kind == lineFile {
		ui.Row(c).Height(34).PaddingX(20).PaddingY(0).Gap(8).AlignItems(ui.End).Padding(0, 20, 6, 20).Children(func() {
			dir, name := path.Split(l.text)
			ui.Text(c, name).FontWeight(600).SingleLine()
			ui.Text(c, dir).FontSize(12).TextColor(p.Faint).SingleLine().Shrink(1)
		})
		return
	}
	if l.kind == lineMsg {
		ui.Text(c, l.text).PaddingX(20).PaddingY(2).TextColor(p.Text).LineHeight(1.45)
		return
	}
	row := ui.Row(c).Height(diffRowHeight).PaddingX(20).AlignItems(ui.Center)
	color := p.Text
	switch l.kind {
	case lineAdd:
		row.Background(p.AddBg)
		color = p.AddText
	case lineDel:
		row.Background(p.DelBg)
		color = p.DelText
	case lineHunk:
		row.Background(p.HunkBg)
		color = p.HunkText
	case lineMeta:
		color = p.Meta
	}
	row.Children(func() {
		ui.Text(c, l.text).Font(monoFont).FontSize(12).TextColor(color).SingleLine().NoWrap()
	})
}

func (a *App) hintBar(c *ui.Context, p *Palette) {
	ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
	ui.Row(c).Height(hintBarHeight).PaddingX(14).Gap(16).AlignItems(ui.Center).Background(p.Panel).ClipX().Children(func() {
		hint := func(key, label string, run func()) {
			h := ui.Row(c).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			if h.Clicked() && run != nil {
				run()
			}
			h.Children(func() {
				keycap(c, p, key)
				col := p.Muted
				if h.Hovered() {
					col = p.Text
				}
				ui.Text(c, label).FontSize(12).TextColor(col)
			})
		}
		// Hints are the scope's own actions marked for the bar, then the
		// global ones; ? lists everything.
		sc := a.scope()
		for _, pass := range []scope{sc, scopeGlobal} {
			for _, act := range a.actions {
				if act.scope == pass && act.hint && len(act.keys) > 0 {
					hint(act.keys.Label(), a.actionLabel(act), act.run)
				}
			}
		}
		ui.Box(c).Grow(1)
		if info := a.modeInfo(); info != "" {
			if ui.Text(c, info+" "+a.repo.Common.Tr.ResetInParentheses).FontSize(12).TextColor(p.Accent).SingleLine().Cursor(ui.CursorPointer).Clicked() {
				if a.customPatch != nil {
					a.resetCustomPatch()
				} else {
					a.resetCherryPick()
				}
			}
		}
		switch {
		case a.busy != "":
			ui.Spinner(c).Size(12, 12)
			ui.Text(c, a.busy).FontSize(12).TextColor(p.Muted)
		case a.status != "":
			col := p.Staged
			if a.statusErr {
				col = p.Conflict
			}
			ui.Text(c, a.status).FontSize(12).TextColor(col).SingleLine().Shrink(1)
		}
	})
}

func keycap(c *ui.Context, p *Palette, s string) {
	ui.Text(c, s).Font(monoFont).FontSize(sizeCaption).TextColor(p.Muted).
		Background(p.Idle).Radius(4).PaddingX(5).PaddingY(1)
}

func fileState(f *models.File) string {
	switch {
	case f.HasMergeConflicts:
		return "Conflicted"
	case !f.Tracked:
		return "Untracked"
	case f.HasStagedChanges && f.HasUnstagedChanges:
		return "Partially staged"
	case f.HasStagedChanges:
		return "Staged"
	case f.Deleted:
		return "Deleted"
	}
	return "Modified"
}

func ago(unix int64) string {
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy", int(d.Hours()/24/365))
}

func commitFileRow(c *ui.Context, p *Palette, f *models.CommitFile, name string, inTree bool) {
	col := p.Unstaged
	switch f.ChangeStatus {
	case "A":
		col = p.Staged
	case "D":
		col = p.Conflict
	case "R":
		col = p.Accent
	}
	ui.Text(c, f.ChangeStatus).Font(monoFont).FontSize(12).FontWeight(600).TextColor(col).Width(18)
	pathRow(c, p, name, inTree, p.Text)
}

// hoverCard shows a commit's details beside its row after the pointer rests
// on it. The selected row needs none: the main pane shows it.
func (a *App) hoverCard(c *ui.Context, p *Palette, row ui.Element, cm *models.Commit) {
	hovered := row.Hovered()
	target := float32(0)
	if hovered {
		target = 1
	}
	if row.Animate("hover", target, 450*time.Millisecond) < 0.999 || !hovered {
		return
	}
	ui.Overlay(c, func() {
		ui.Column(c).AttachTo(row, ui.AnchorTopRight, ui.AnchorTopLeft).Left(10).Width(380).
			Background(p.Panel).Radius(10).Border(1, p.Hairline).Shadow(0, 10, 30, 0, ui.Hex("#00000040")).
			Padding(12, 14, 12, 14).Gap(6).IgnoreInput(true).Children(func() {
			ui.Text(c, cm.Name).FontWeight(600).LineHeight(1.35)
			ui.Row(c).Gap(8).AlignItems(ui.Center).FontSize(12).TextColor(p.Muted).Children(func() {
				ui.Text(c, cm.ShortHash()).Font(monoFont).TextColor(p.Accent)
				ui.Text(c, cm.AuthorName)
				ui.Text(c, time.Unix(cm.UnixTimestamp, 0).Format("Jan 2, 2006 15:04"))
			})
			if cm.ExtraInfo != "" {
				ui.Text(c, strings.Trim(cm.ExtraInfo, " ()")).FontSize(12).TextColor(p.Faint).MaxLines(2)
			}
			status := map[models.CommitStatus]string{
				models.StatusUnpushed: "Not pushed", models.StatusPushed: "Pushed", models.StatusMerged: "Merged into main",
			}[cm.Status]
			if status != "" {
				ui.Text(c, status).FontSize(11).TextColor(p.Faint)
			}
		})
	})
}

func (a *App) logDrawer(c *ui.Context, p *Palette) {
	a.logBorder(c, p)
	ui.Column(c).Height(a.logHeight()).Shrink(0).FillWidth().Background(p.Bg).Children(func() {
		ui.Row(c).Height(24).PaddingX(14).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "COMMAND LOG").FontSize(sizeCaption).FontWeight(650).LetterSpacing(0.7).TextColor(p.Faint)
			ui.Box(c).Grow(1)
			keycap(c, p, a.keys.extras.Label())
		})
		if len(a.log) == 0 {
			ui.Text(c, "Git commands that change the repository appear here.").FontSize(12).
				TextColor(p.Faint).PaddingX(14).PaddingY(2)
		}
		ui.List(c, &a.logList, len(a.log), func(i int) {
			e := a.log[i]
			col := p.Muted
			prefix := "  "
			if e.isCmd {
				col, prefix = p.Text, "$ "
			}
			ui.Text(c, prefix+e.text).Font(monoFont).FontSize(11.5).TextColor(col).SingleLine().PaddingX(14).Height(17)
		}).Grow(1).MinHeight(0)
	})
}

func (a *App) stagingPane(c *ui.Context, p *Palette) { a.diffSelectionPane(c, p, a.staging, true) }

func (a *App) diffSelectionPane(c *ui.Context, p *Palette, s *stagingState, active bool) {
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Background(p.Panel).Children(func() {
		ui.Column(c).PaddingX(20).PaddingY(14).Gap(8).Children(func() {
			if s.title != "" {
				dir, name := path.Split(s.title)
				ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
					ui.Text(c, name).FontSize(sizeTitle).FontWeight(600)
					ui.Text(c, dir).FontSize(12).TextColor(p.Faint).SingleLine().Shrink(1)
				})
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if s.worktree || a.customPatch != nil {
					seg := ui.Row(c).Background(p.Idle).Radius(7).Padding(2)
					seg.Children(func() {
						for _, side := range []bool{false, true} {
							label := a.repo.Common.Tr.UnstagedChanges
							if !s.worktree {
								label = a.mainTitle
							}
							if side {
								label = a.repo.Common.Tr.StagedChanges
								if !s.worktree {
									label = a.repo.Common.Tr.CustomPatch
								}
							}
							b := ui.Text(c, label).FontSize(12).PaddingX(10).PaddingY(3).Radius(5).Cursor(ui.CursorPointer)
							if side == s.staged {
								b.Background(p.Panel).FontWeight(600)
							} else {
								b.TextColor(p.Muted)
							}
							if b.Clicked() && side != s.staged {
								a.switchStagingSide()
							}
						}
					})
					keycap(c, p, a.keys.togglePanel.Label())
					ui.Box(c).Width(8)
				}
				mode := "Line"
				switch {
				case s.cursor < 0:
					mode = "No"
				case s.selectingRange():
					mode = "Range"
				case s.mode == selectHunk:
					mode = "Hunk"
				}
				ui.Text(c, mode+" selection").FontSize(12).TextColor(p.Muted)
				ui.Box(c).Grow(1)
				if s.worktree && active {
					verb := "stage"
					if s.staged {
						verb = "unstage"
					}
					ui.Text(c, a.keys.selectKey.Label()+" to "+verb).FontSize(12).TextColor(p.Faint)
				}
			})
		})
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		wrap := a.repo.Config.Gui.WrapLinesInDiffView
		if !wrap && s.contentWidth == 0 {
			for _, line := range s.lines {
				width, _ := c.MeasureText(0, ui.Span{Text: strings.ReplaceAll(line.Content, "\t", "    "), Font: monoFont, Size: 12})
				s.contentWidth = max(s.contentWidth, width+48)
			}
		}
		draw := func() {
			lo, hi := s.selection()
			list := ui.List(c, &s.list, len(s.lines), func(i int) {
				l := s.lines[i]
				row := ui.Row(c).MinHeight(diffRowHeight).AlignItems(ui.Center)
				if !wrap {
					row.Height(diffRowHeight)
				}
				color := p.Text
				switch {
				case !s.rows[i].ok:
					color = p.Muted
				case l.Kind == patch.PATCH_HEADER:
					color = p.Meta
				case l.Kind == patch.ADDITION:
					row.Background(p.AddBg)
					color = p.AddText
				case l.Kind == patch.DELETION:
					row.Background(p.DelBg)
					color = p.DelText
				case l.Kind == patch.HUNK_HEADER:
					row.Background(p.HunkBg)
					color = p.HunkText
				case l.Kind == patch.NEWLINE_MESSAGE:
					color = p.Meta
				}
				inSel := s.cursor >= 0 && i >= lo && i <= hi
				if row.Clicked() && s.cursor >= 0 {
					if !active {
						a.staging = s
					}
					if row.ClickModifiers()&ui.Shift != 0 {
						if !s.selectingRange() {
							s.mode, s.anchor = selectRange, s.cursor
						}
						s.cursor = i
					} else {
						s.resetMode()
						s.cursor = i
					}
				}
				row.Children(func() {
					bar := ui.Box(c).Width(3).FillHeight()
					if i == s.cursor {
						if active {
							bar.Background(p.Accent)
						} else {
							bar.Background(p.Accent.Alpha(0.45))
						}
					} else if inSel {
						bar.Background(p.Accent.Alpha(0.45))
					}
					line := ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Center).PaddingX(17)
					if inSel {
						if active {
							line.Background(p.AccentSoft)
						} else {
							line.Background(p.Idle)
						}
					}
					line.Children(func() {
						if !s.worktree && !s.staged && a.customPatch != nil {
							mark := " "
							if a.patchMarked(s.rows[i]) {
								mark = "▎"
							}
							ui.Text(c, mark).Font(monoFont).FontSize(12).TextColor(p.Staged).Width(8).Shrink(0)
						}
						text := ui.Text(c, strings.ReplaceAll(l.Content, "\t", "    ")).Font(monoFont).FontSize(12).TextColor(color)
						if wrap {
							text.Grow(1).MinWidth(0)
						} else {
							text.SingleLine().NoWrap()
						}
					})
				})
			}).Grow(1).MinHeight(0).FillHeight().PaddingY(8)
			if !wrap {
				list.MinWidth(max(s.contentWidth, s.viewportWidth))
			}
		}
		if wrap {
			draw()
		} else {
			container := ui.ScrollHorizontal(c).Grow(1).MinHeight(0).FillWidth().TrackScroll(&s.horizontal)
			s.viewportWidth = container.Bounds().W
			container.Children(draw)
		}
	})
}

func todoActionLabel(cm *models.Commit) string {
	label := cm.Action.String()
	if cm.Action == todo.Fixup && cm.ActionFlag != "" {
		label += " " + cm.ActionFlag
	}
	return label
}

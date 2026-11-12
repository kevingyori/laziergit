package app

import (
	"cmp"
	"fmt"
	"laziergit/internal/keymap"
	"path"
	"slices"
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
	a.tickDrag(c)
	a.cells.reset()
	a.light = !c.Theme().Dark
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
			if a.guide != nil && a.guide.active {
				a.guideView(c, p)
				return
			}
			// lazygit's mid section: side panels beside (or, in portrait, over)
			// the main views, weighted by the screen mode.
			mid := ui.Row(c)
			if a.portrait() {
				mid = ui.Column(c)
			}
			mid.Grow(1).MinHeight(0).Children(func() {
				w, h := c.Size()
				a.layout.winW, a.layout.winH = w, h
				a.layout.sideW = a.sideWidth(w)
				if a.sideShown() {
					side := ui.Column(c).Background(p.Panel)
					switch {
					case a.portrait():
						// enlargedSideViewLocation top: main = 2 × side.
						side.Grow(1).Basis(0).MinHeight(0).FillWidth()
					case !a.mainSectionShown():
						side.Grow(1).MinWidth(0).FillHeight()
					case a.screenMode == screenHalf:
						side.Grow(1).Basis(0).MinWidth(0).FillHeight()
					default:
						side.Width(a.layout.sideW).Shrink(0).FillHeight()
					}
					side.Children(func() {
						boxes := a.sideBoxes(c)
						first := true
						for win := range a.windows {
							if !a.sidePanelShown(win) {
								continue
							}
							if !first {
								a.panelBorder(c, p, win, boxes)
							}
							first = false
							v := a.activeView(win)
							a.withView(v, func() { a.section(c.Key(win), p, win, v, boxes[win]) })
						}
					})
				}
				if !a.sideShown() || !a.mainSectionShown() {
				} else if a.portrait() {
					ui.Box(c).Height(1).FillWidth().Shrink(0).Background(p.Hairline)
				} else if a.screenMode == screenNormal {
					a.sideBorder(c, p)
				} else {
					a.border(c, p, true, nil, nil, nil)
				}
				if a.mainSectionShown() {
					main := ui.Column(c).Grow(1).MinWidth(0)
					if a.portrait() {
						main.Grow(2).Basis(0).MinHeight(0).FillWidth()
					} else if a.screenMode == screenHalf && a.sideShown() {
						main.Basis(0).FillHeight()
					} else {
						main.FillHeight()
					}
					main.Children(func() {
						a.diffPanes(c, p)
						if a.showLog {
							a.logDrawer(c, p)
						}
					})
				}
			})
			a.searchBar(c, p)
			a.hintBar(c, p)
			a.placeGrips(c, p)
			a.overlayLayer(c, p)
		})
}

func (a *App) section(c *ui.Context, p *Palette, win int, v sideView, box sideBox) {
	pn := v.group()
	views := a.windows[win].views
	focused := a.focus == pn && !a.drawSwapped
	col := ui.Column(c).MinHeight(0).Clip()
	col.Height(box.size).Shrink(0)
	a.layout.sideH[win] = col.Bounds().H
	col.Children(func() {
		head := ui.Row(c).Height(28).PaddingX(12).Gap(6).AlignItems(ui.Center)
		if head.Clicked() {
			a.clickSide(v, -1)
		}
		head.Children(func() {
			titleColor := p.Muted
			if focused {
				titleColor = p.Accent
			}
			title := v.title()
			if pn == panelCommits && a.commitFiles != nil {
				title = "Commit files"
			}
			if b := a.browse(pn); b != nil && !(pn == panelCommits && a.commitFiles != nil) {
				title := fmt.Sprintf(a.repo.Common.Tr.RemoteBranchesDynamicTitle, b.remote)
				if b.ref != nil {
					title = a.subCommitsTitle()
				}
				ui.Text(c, strings.ToUpper(title)).FontSize(sizeCaption).FontWeight(650).TextColor(titleColor)
			} else if len(views) > 1 && !(pn == panelCommits && a.commitFiles != nil) {
				// lazygit's tabs: the window's configured tabs, in order.
				for _, tv := range views {
					tab := ui.Row(c).PaddingX(3)
					if tab.Clicked() {
						a.clickSide(tv, -1)
					}
					tab.Children(func() {
						col := p.Faint
						if tv == v {
							col = titleColor
						}
						ui.Text(c, strings.ToUpper(tv.title())).FontSize(sizeCaption).FontWeight(650).TextColor(col)
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
				if label := a.filesFilterLabel(); label != "" { // lazygit's Files subtitle
					ui.Text(c, label).FontSize(sizeCaption).TextColor(p.Muted)
				}
			}
			ui.Box(c).Grow(1)
			if pn == panelCommits && a.snap != nil && a.snap.Operation != "" {
				if ui.Text(c, a.snap.OperationTitle+" · "+keymap.Parse(a.repo.Config.Keybinding.Universal.CreateRebaseOptionsMenu).Label()).FontSize(sizeCaption).TextColor(p.Muted).Cursor(ui.CursorPointer).Clicked() {
					a.operationMenu()
				}
			}
			if win < len(a.keys.jump) {
				keycap(c, p, a.keys.jump[win].Label())
			}
		})
		if pn == panelCommits && a.emptyCommitFiles() {
			ui.Text(c, emptyCommitFilesLabel).PaddingX(18).PaddingY(6).TextColor(p.Muted)
		}
		b := a.browse(pn)
		if pn == panelCommits && a.commitFiles != nil {
			b = nil
		}
		remoteAt, localAt, divergence := b.divergenceHeaders()
		// The lazygit view this window shows, and the line of its item i:
		// section headers are lines of their own.
		view := a.SideTopView(a.windows[win].name())
		rebasing := pn == panelCommits && a.commitFiles == nil && a.snap != nil && a.snap.Operation == "Rebasing"
		a.hitView = view
		// Non-model lines of the commits list: section headers mid-rebase and
		// the line a dragged block of commits would land on.
		nonModel := []int(nil)
		if pn == panelCommits && a.commitFiles == nil {
			nonModel = a.nonModelItems(pn)
		}
		dropLine := a.commitDropLine()
		a.hitLine = func(i int) int {
			switch {
			case divergence:
				return i + boolInt(remoteAt >= 0 && i >= remoteAt) + boolInt(localAt >= 0 && i >= localAt)
			case len(nonModel) > 0:
				return a.modelToViewLine(pn, i)
			}
			return i
		}
		header := func(text string, line int) {
			h := ui.Text(c, text).FontSize(sizeCaption).TextColor(p.Muted).PaddingX(18).PaddingY(6)
			a.cells.row(view, line, h)
		}
		if divergence && len(b.commits) == 0 {
			header(a.repo.Common.Tr.DivergenceSectionHeaderRemote, 0)
			header(a.repo.Common.Tr.DivergenceSectionHeaderLocal, 1)
		}
		list := ui.List(c, a.listFor(pn), a.count(pn), func(i int) {
			if divergence {
				ui.Column(c).FillWidth().Children(func() {
					if i == remoteAt {
						header(a.repo.Common.Tr.DivergenceSectionHeaderRemote, a.hitLine(i)-1)
					}
					if i == localAt {
						header(a.repo.Common.Tr.DivergenceSectionHeaderLocal, a.hitLine(i)-1)
					}
					a.row(c, p, pn, i, box.rowHeight)
					if localAt == len(b.commits) && i == localAt-1 {
						header(a.repo.Common.Tr.DivergenceSectionHeaderLocal, a.hitLine(i)+1)
					}
				})
				return
			}
			if len(nonModel) > 0 {
				// The non-model lines at item i, in their order, then the row.
				nonModelLine := func(k, idx int) {
					line := idx + k
					switch {
					case line == dropLine:
						a.dropRow(c, p, view, line)
					case rebasing && k == 0:
						header(a.repo.Common.Tr.PendingRebaseTodosSectionHeader, line)
					case rebasing:
						header(a.repo.Common.Tr.CommitsSectionHeader, line)
					}
				}
				ui.Column(c).FillWidth().Children(func() {
					for k, idx := range nonModel {
						if idx == i {
							nonModelLine(k, idx)
						}
					}
					a.row(c, p, pn, i, box.rowHeight)
					if i == a.count(pn)-1 {
						for k, idx := range nonModel {
							if idx > i {
								nonModelLine(k, idx)
							}
						}
					}
				})
			} else {
				a.row(c, p, pn, i, box.rowHeight)
			}
		}).Height(max(0, box.size-panelHead-box.foot)).Shrink(0)
		ui.Box(c).Height(box.foot).Shrink(0)
		a.cells.pane(view, list)
		list.HandleInput(a.sideMouse(view, v))
	})
}

// dropRow is lazygit's commit drop indicator ("drop here"): where the
// dragged commits land on release. Keys: the move commit up/down bindings.
func (a *App) dropRow(c *ui.Context, p *Palette, view string, line int) {
	r := ui.Row(c).Height(rowHeight).PaddingX(18).Gap(8).AlignItems(ui.Center).FillWidth()
	a.cells.row(view, line, r)
	r.Children(func() {
		ui.Box(c).Height(1).Grow(1).Background(p.Accent)
		ui.Text(c, a.repo.Common.Tr.MoveCommitsHere).FontSize(sizeCaption).TextColor(p.Accent).FontWeight(600).SingleLine()
		ui.Box(c).Height(1).Grow(1).Background(p.Accent)
	})
}

func (a *App) row(c *ui.Context, p *Palette, pn panel, i int, height float32) {
	sel := a.inSelection(pn, i)
	row := ui.Row(c).Height(height).PaddingX(12).Gap(8).AlignItems(ui.Center).Radius(5).MarginX(6)
	active := a.focus == pn && a.staging == nil && !a.mainFocused && !a.drawSwapped
	switch {
	case sel && active:
		row.Background(p.AccentSoft)
	case sel:
		row.Background(p.Idle)
	case row.Hovered():
		row.Background(p.Hover)
	}
	if a.hitLine != nil {
		a.cells.row(a.hitView, a.hitLine(i), row)
		a.cells.item(a.hitView, a.hitLine(i), i)
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
				a.arrowSpan(i, r.depth, dirRow(c, p, r, n.GetHasStagedChanges(), n.GetHasUnstagedChanges()))
			} else {
				fileRow(c, p, n.File, r.name, a.files.InTreeMode())
			}
		case panelBranches:
			if s := a.browse(panelBranches); s != nil {
				if s.ref != nil {
					a.subCommitRow(c, p, s, i, a.subCommitHeads(s))
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
			branchRow(c, p, b, a.shownPR(b), a.enlarged(), a.repo.Config.Gui.ShowDivergenceFromBaseBranch)
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
					a.arrowSpan(i, r.depth, dirRow(c, p, r, false, false))
				} else {
					commitFileRow(c, p, n.File, r.name, a.commitFiles.tree.InTreeMode())
				}
				return
			}
			if v := a.commitsView(); v != cvCommits {
				if v == cvSubCommits {
					a.subCommitRow(c, p, a.refBrowse, i, a.subCommitHeads(a.refBrowse))
				} else {
					commitRow(c, p, a.snap.Reflog[i], false, a.enlarged())
				}
				return
			}
			if i < len(a.snap.Graph) && !a.snap.Commits[i].IsTODO() {
				var selected *string
				if a.focus == panelCommits && a.staging == nil && !a.mainFocused && a.topOverlay() == nil {
					if cm := a.selectedCommit(); cm != nil {
						selected = cm.HashPtr()
					}
				}
				graphCell(c, p, a.snap.Graph[i], selected)
			}
			if a.snap.Operation == "Rebasing" && models.IsHeadCommit(a.snap.Commits, i) {
				ui.Text(c, "← HEAD").Font(monoFont).FontSize(sizeCaption).TextColor(p.Accent).Shrink(0)
			}
			if m := a.bisectMarker(i); m != "" {
				ui.Text(c, m).Font(monoFont).FontSize(sizeCaption).TextColor(p.Accent).Shrink(0)
			}
			if marks := a.rebaseMarks(a.snap.Commits); marks != nil && marks[i] != "" {
				// lazygit's marks for a marked base commit (yellow there).
				ui.Text(c, marks[i]).Font(monoFont).FontSize(sizeCaption).TextColor(p.Accent).Shrink(0)
			}
			commitRow(c, p, a.snap.Commits[i], a.branchHeads(a.snap.Commits, a.snap.CheckedOutBranch)[a.snap.Commits[i].Hash()], a.enlarged())
		case panelStash:
			s := a.snap.Stash[i]
			ui.Textf(c, "%d", s.Index).Font(monoFont).FontSize(12).TextColor(p.Faint).Width(16)
			ui.Text(c, s.Name).SingleLine().Grow(1).Shrink(1)
			ui.Text(c, s.Recency).FontSize(12).TextColor(p.Faint)
		}
	})
}

func (a *App) statusRow(c *ui.Context, p *Palette) {
	upstream, state, repoName := a.statusParts()
	name := ui.Text(c, repoName).FontWeight(600).Cursor(ui.CursorPointer)
	// lazygit's FormatStatus names the linked worktree we're in.
	if w := a.snap.Worktrees; len(w) > 0 && !w[0].IsMain {
		ui.Text(c, "("+w[0].Name+")").TextColor(p.Faint).SingleLine().Shrink(1)
	}
	// The columns lazygit's StatusController.onClick takes for the working
	// tree state and the repo name (cursorInSubstring).
	prefix := upstream + " "
	if a.snap.Operation != "" {
		op := ui.Text(c, a.snap.Operation).FontSize(sizeCaption).TextColor(p.Conflict).Cursor(ui.CursorPointer)
		if state != "" {
			a.cells.span("status", 0, runeCount(prefix), runeCount(prefix+state), op)
			prefix += state + " "
		}
	}
	a.cells.span("status", 0, runeCount(prefix), runeCount(prefix+repoName), name)
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

// full is lazygit's fullDescription (half and full screen): the upstream
// follows the name.
func branchRow(c *ui.Context, p *Palette, b *models.Branch, pr *models.GithubPullRequest, full bool, divergence string) {
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
	// gui.showDivergenceFromBaseBranch: commits the base branch has that b lacks.
	if behind := b.BehindBaseBranch.Load(); behind != 0 && divergence != "none" {
		mark := "↓"
		if divergence == "arrowAndNumber" {
			mark = fmt.Sprintf("↓%d", behind)
		}
		ui.Text(c, mark).FontSize(12).TextColor(p.Accent).FontFeatures("tnum")
	}
	if pr != nil {
		ui.Textf(c, "#%d", pr.Number).FontSize(12).TextColor(prColor(p, pr.State)).FontFeatures("tnum")
	}
	if full && b.IsTrackingRemote() {
		ui.Text(c, b.UpstreamRemote+" "+b.UpstreamBranch).FontSize(12).TextColor(p.Muted).SingleLine().Shrink(1)
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
// graphCell draws a row's lanes; the lines leaving the selected commit (a
// pool pointer, nil when the list isn't focused) are drawn heavier on top,
// lazygit's highlighted lines of the selected commit.
func graphCell(c *ui.Context, p *Palette, g graph.Row, selected *string) {
	lanes := min(g.Width, maxLanes)
	ui.Box(c).Width(float32(lanes) * laneWidth).Height(rowHeight).Shrink(0).Clip().
		Draw(func(pt *ui.Painter, r ui.Rect) {
			x := func(l int16) float32 { return r.X + float32(l)*laneWidth + laneWidth/2 }
			top, mid, bot := r.Y, r.Y+r.H/2, r.Y+r.H
			width := float32(1.6)
			seg := func(x0, y0, x1, y1 float32, col ui.Color) {
				path := new(ui.Path).MoveTo(x0, y0)
				if x0 == x1 {
					path.LineTo(x1, y1)
				} else {
					my := (y0 + y1) / 2
					path.CubeTo(x0, my, x1, my, x1, y1)
				}
				pt.StrokePath(path, width, col)
			}
			var dot ui.Color
			pipes := g.Pipes
			if selected != nil {
				pipes = slices.Clone(pipes)
				slices.SortStableFunc(pipes, func(x, y graph.Pipe) int {
					return cmp.Compare(boolInt(x.FromHash() == selected), boolInt(y.FromHash() == selected))
				})
			}
			for _, pp := range pipes {
				col := p.Lanes[pp.Color%uint32(len(p.Lanes))]
				width = 1.6
				if selected != nil && pp.FromHash() == selected {
					width = 2.6
				}
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

// subCommitRow is a sub-commits row: graph lanes and branch-head marks as
// lazygit's SubCommitsContext draws them.
func (a *App) subCommitRow(c *ui.Context, p *Palette, b *refBrowseState, i int, heads map[string]bool) {
	if a.repo.Config.Git.Log.ShowGraph == "always" && i < len(b.graph) {
		var selected *string
		if a.focus == b.panel && a.commitFiles == nil && a.staging == nil && !a.mainFocused && a.topOverlay() == nil {
			if cur := a.cursor[b.panel]; cur >= 0 && cur < len(b.commits) {
				selected = b.commits[cur].HashPtr()
			}
		}
		graphCell(c, p, b.graph[i], selected)
	}
	commitRow(c, p, b.commits[i], heads[b.commits[i].Hash()], a.enlarged())
}

// divergenceHeader draws one of the sub-commits view's Local/Remote headers.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// full is lazygit's fullDescription (half and full screen): the author's
// name precedes the date.
func commitRow(c *ui.Context, p *Palette, cm *models.Commit, branchHead bool, full bool) {
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
	if branchHead {
		// lazygit's marker for a commit another branch points at.
		ui.Text(c, "*").Font(monoFont).FontSize(12).TextColor(p.Accent).Shrink(0)
	}
	for _, tag := range cm.Tags {
		ui.Text(c, tag).FontSize(11).TextColor(p.Accent).Background(p.AccentSoft).Radius(4).PaddingX(5).Shrink(0)
	}
	ui.Text(c, cm.Name).SingleLine().Grow(1).Shrink(1)
	if full && cm.AuthorName != "" {
		ui.Text(c, cm.AuthorName).FontSize(12).TextColor(p.Muted).SingleLine().Shrink(0)
	}
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
		ui.Column(c).PaddingX(20).PaddingY(14).Gap(4).MinHeight(diffHeaderHeight).Children(func() { a.mainHeader(c, p) })
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		marks := a.mainPatchMarks()
		wrapWidth := 0
		if a.PaneWraps("main") {
			wrapWidth = a.paneInnerWidth("main")
		}
		var ansi []ansiLine
		if a.diffRendered {
			ansi = a.mainAnsi.get(a.diffText)
		}
		links, raw := a.mainStatLinks()
		list := ui.List(c, &a.diffList, len(a.diff)+a.mainPastBottomRows(), func(i int) {
			if i >= len(a.diff) {
				ui.Box(c).Height(diffRowHeight) // scrolled past the bottom
				return
			}
			l := a.diff[i]
			if marks != nil && l.kind != lineFile && l.kind != lineMsg {
				// The custom patch's lines, marked as the focused view marks them.
				row := ui.Row(c).Children(func() {
					mark := " "
					if l.src >= 0 && l.src < len(marks) && marks[l.src] {
						mark = "▎"
					}
					ui.Text(c, mark).Font(monoFont).FontSize(12).TextColor(p.Staged).Width(8).Shrink(0)
					ui.Column(c).Grow(1).Children(func() { diffLineRow(c, p, l, ansiSpansAt(ansi, l.src), wrapWidth) })
				})
				a.unfocusedMainRow(row, l.src)
				return
			}
			var link *[3]string
			from, to := 0, 0
			if l.src >= 0 && l.src < len(raw) {
				if f, t, parts, ok := statLinkCols(links, l.src, raw[l.src], l.text); ok {
					link, from, to = &parts, f, t
				}
			}
			row, linkEl, linked := diffLineRowLinked(c, p, l, ansiSpansAt(ansi, l.src), wrapWidth, link)
			if linked {
				a.cells.span("main", l.src, from, to, linkEl)
				if linkEl.Clicked() {
					a.jumpToDiffStatEntry(false, link[1])
					return
				}
			}
			a.unfocusedMainRow(row, l.src)
		}).Grow(1).MinHeight(0)
		a.cells.pane("main", list)
		list.HandleInput(a.paneMouse("main"))
		a.mainViewportHeight = list.Bounds().H
		a.mainViewportWidth = list.Bounds().W - 40 // the rows' text, inside their padding
	})
}

func (a *App) mainHeader(c *ui.Context, p *Palette) {
	if a.mainTitle != "" {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if a.focusMainJumpLabel("main") != "" {
				keycap(c, p, a.keys.focusMain.Label())
			}
			ui.Text(c, a.mainTitle).FontSize(sizeCaption).TextColor(p.Muted)
		})
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

// ansiSpansAt is row i's spans of a rendering, nil for a native diff.
func ansiSpansAt(ansi []ansiLine, i int) []ansiSpan {
	if i < 0 || i >= len(ansi) {
		return nil
	}
	return ansi[i].spans
}

// wrapWidth > 0 wraps the row's text at that many columns, as gocui does.
func diffLineRow(c *ui.Context, p *Palette, l diffLine, spans []ansiSpan, wrapWidth int) ui.Element {
	row, _, _ := diffLineRowLinked(c, p, l, spans, wrapWidth, nil)
	return row
}

// diffLineRowLinked is diffLineRow with a link over part of the line (a
// diffstat entry's file name): link holds the text before, in and after it,
// and the link's element is returned too, with whether it is drawn.
func diffLineRowLinked(c *ui.Context, p *Palette, l diffLine, spans []ansiSpan, wrapWidth int, link *[3]string) (ui.Element, ui.Element, bool) {
	if l.kind == lineAnsi {
		// A renderer's row: its own colours, mono, no tinting of ours.
		var linkEl ui.Element
		row := ui.Row(c).Height(diffRowHeight).PaddingX(20).AlignItems(ui.Center).Children(func() {
			if link == nil {
				ui.RichText(c, ansiUISpans(spans, p, p.Text)...).SingleLine().NoWrap()
				return
			}
			parts := splitSpans(spans, len(link[0]), len(link[0])+len(link[1]))
			for i, part := range parts {
				el := ui.RichText(c, ansiUISpans(part, p, p.Text)...).SingleLine().NoWrap().Shrink(0)
				if i == 1 {
					linkEl = el.Cursor(ui.CursorPointer)
					if el.Hovered() {
						el.Underline()
					}
				}
			}
		})
		return row, linkEl, link != nil
	}
	if l.kind == lineFile {
		return ui.Row(c).Height(34).PaddingX(20).PaddingY(0).Gap(8).AlignItems(ui.End).Padding(0, 20, 6, 20).Children(func() {
			dir, name := path.Split(l.text)
			ui.Text(c, name).FontWeight(600).SingleLine()
			ui.Text(c, dir).FontSize(12).TextColor(p.Faint).SingleLine().Shrink(1)
		}), ui.Element{}, false
	}
	if l.kind == lineMsg {
		return ui.Text(c, l.text).PaddingX(20).PaddingY(2).TextColor(p.Text).LineHeight(1.45), ui.Element{}, false
	}
	segs := []string{l.text}
	if wrapWidth > 0 {
		segs = wrapViewLine(l.text, wrapWidth)
	}
	row := ui.Row(c).Height(diffRowHeight * float32(len(segs))).PaddingX(20).AlignItems(ui.Center)
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
	var linkEl ui.Element
	linked := false
	row.Children(func() {
		if link != nil && len(segs) == 1 {
			ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Children(func() {
				mono := func(s string) ui.Element {
					return ui.Text(c, s).Font(monoFont).FontSize(12).TextColor(color).SingleLine().NoWrap().Height(diffRowHeight).Shrink(0)
				}
				mono(link[0])
				linkEl, linked = mono(link[1]).Cursor(ui.CursorPointer), true
				if linkEl.Hovered() {
					linkEl.Underline()
				}
				mono(link[2])
			})
			return
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			for _, seg := range segs {
				ui.Text(c, seg).Font(monoFont).FontSize(12).TextColor(color).SingleLine().NoWrap().Height(diffRowHeight)
			}
		})
	})
	return row, linkEl, linked
}

func (a *App) hintBar(c *ui.Context, p *Palette) {
	ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
	ui.Row(c).Height(hintBarHeight).PaddingX(14).Gap(16).AlignItems(ui.Center).Background(p.Panel).ClipX().Children(func() {
		// lazygit's options bar (optionsMap); ? lists everything.
		for _, o := range a.optionsMap() {
			h := ui.Row(c).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			if h.Clicked() && o.run != nil {
				o.run()
			}
			h.Children(func() {
				keycap(c, p, o.key)
				col := p.Muted
				if o.mode {
					col = p.Accent
				} else if h.Hovered() {
					col = p.Text
				}
				ui.Text(c, o.label).FontSize(12).TextColor(col).SingleLine().NoWrap()
			})
		}
		ui.Box(c).Grow(1)
		if info := a.modeInfo(); info != "" {
			if ui.Text(c, info+" "+a.repo.Common.Tr.ResetInParentheses).FontSize(12).TextColor(p.Accent).SingleLine().Cursor(ui.CursorPointer).Clicked() {
				if a.customPatch != nil {
					a.resetCustomPatch()
				} else if a.filteringActive() {
					a.clearFiltering()
				} else if a.markedBaseCommit != "" {
					a.markedBaseCommit = ""
				} else if a.cherryPickInfo() == "" && a.snap.Operation == "" && a.bisectInfo().Started() {
					a.resetBisect()
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
		ui.Column(c).PaddingX(20).PaddingY(14).Gap(8).MinHeight(diffHeaderHeight).Children(func() {
			if s.title != "" {
				dir, name := path.Split(s.title)
				ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
					ui.Text(c, name).FontSize(sizeTitle).FontWeight(600).SingleLine().Shrink(1)
					ui.Text(c, dir).FontSize(12).TextColor(p.Faint).SingleLine().Shrink(1)
				})
			}
			// One line, like lazygit's titles: labels truncate in narrow panes.
			ui.Row(c).Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
				if s.worktree || a.customPatch != nil {
					seg := ui.Row(c).Background(p.Idle).Radius(7).Padding(2).MinWidth(0).Shrink(1)
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
							b := ui.Text(c, label).FontSize(12).PaddingX(10).PaddingY(3).Radius(5).Cursor(ui.CursorPointer).SingleLine().MinWidth(0).Shrink(1)
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
				ui.Text(c, mode+" selection").FontSize(12).TextColor(p.Muted).SingleLine().MinWidth(0).Shrink(1)
				ui.Box(c).Grow(1)
				if s.worktree && active {
					verb := "stage"
					if s.staged {
						verb = "unstage"
					}
					ui.Text(c, a.keys.selectKey.Label()+" to "+verb).FontSize(12).TextColor(p.Faint).SingleLine().MinWidth(0).Shrink(1)
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
			pane := "main"
			if s.staged {
				pane = "secondary"
			}
			links, linkTexts := s.statLinkMap()
			linkText := func(i int) string {
				if i < len(linkTexts) {
					return linkTexts[i]
				}
				return ""
			}
			list := ui.List(c, &s.list, len(s.lines)+s.pastBottomRows(a), func(i int) {
				if i >= len(s.lines) {
					ui.Box(c).Height(diffRowHeight) // scrolled past the bottom
					return
				}
				l := s.lines[i]
				row := ui.Row(c).MinHeight(diffRowHeight).AlignItems(ui.Center)
				if !wrap {
					row.Height(diffRowHeight)
				}
				color := p.Text
				switch {
				case s.ansi != nil:
					// A renderer's row keeps its own colours.
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
				a.cells.row(pane, i, row)
				linkClicked := ""
				defer func() {
					if linkClicked != "" {
						a.jumpToDiffStatEntry(true, linkClicked)
					}
				}()
				if row.Clicked() && row.ClickModifiers()&(ui.Alt|ui.Shift) != 0 {
					// lazygit's editClickedLine.
					if i < len(s.rows) {
						a.editDiffRow(s.rows[i], s.staged, s.worktree)
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
					line := ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Center).Padding(0, 20, 0, 17)
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
						if from, to, parts, ok := statLinkCols(links, i, linkText(i), linkText(i)); ok &&
							(!wrap || len(wrapViewLine(l.Content, s.innerWidth())) == 1) {
							// A diffstat entry: its file name links to the file's diff.
							var spans [3][]ansiSpan
							if s.ansi != nil {
								spans = splitSpans(ansiSpansAt(s.ansi, i), len(parts[0]), len(parts[0])+len(parts[1]))
							}
							for k, part := range parts {
								var el ui.Element
								if s.ansi != nil {
									el = ui.RichText(c, ansiUISpans(spans[k], p, p.Text)...)
								} else {
									el = ui.Text(c, strings.ReplaceAll(part, "\t", "    ")).Font(monoFont).FontSize(12).TextColor(color)
								}
								el.SingleLine().NoWrap().Shrink(0)
								if k == 1 {
									el.Cursor(ui.CursorPointer)
									if el.Hovered() {
										el.Underline()
									}
									a.cells.span(pane, i, from, to, el)
									if el.Clicked() {
										linkClicked = parts[1]
									}
								}
							}
							return
						}
						if wrap && s.ansi == nil {
							// gocui's wrapping at the pane's column count.
							ui.Column(c).Grow(1).MinWidth(0).Children(func() {
								for _, seg := range wrapViewLine(l.Content, s.innerWidth()) {
									ui.Text(c, seg).Font(monoFont).FontSize(12).TextColor(color).SingleLine().NoWrap().Height(diffRowHeight)
								}
							})
							return
						}
						var text ui.Element
						if s.ansi != nil {
							text = ui.RichText(c, ansiUISpans(ansiSpansAt(s.ansi, i), p, p.Text)...)
						} else {
							text = ui.Text(c, strings.ReplaceAll(l.Content, "\t", "    ")).Font(monoFont).FontSize(12).TextColor(color)
						}
						if wrap {
							text.Grow(1).MinWidth(0)
						} else {
							text.SingleLine().NoWrap()
						}
					})
				})
			}).Grow(1).MinHeight(0).FillHeight()
			a.cells.pane(pane, list)
			list.HandleInput(a.stagingMouse(s, pane))
			s.viewportHeight = list.Bounds().H
			if wrap {
				s.viewportWidth = list.Bounds().W // the wrap width of the next frame
			}
			s.originSet = false
			if !wrap {
				// Room for a match at the end of a line to come a third of the way
				// across, as gocui lets the origin pass the content until it is
				// rendered again.
				list.MinWidth(max(s.contentWidth, s.viewportWidth) + s.viewportWidth)
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

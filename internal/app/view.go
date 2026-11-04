package app

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/graph"
)

// View builds the window. One focusable root owns the keyboard; nothing
// inside it is focusable, so lazygit's keymap works wherever the pointer
// has been.
func (a *App) View(c *ui.Context) {
	p := palette(c.Theme().Dark)
	ui.Column(c).Fill().Background(p.Bg).FontSize(sizeBody).TextColor(p.Text).
		Focusable().AutoFocus().FocusRing(false).
		InputMethod(false).TextCaret(ui.Rect{}).
		HandleInput(a.handleInput).
		Children(func() {
			ui.Row(c).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Width(360).FillHeight().Background(p.Panel).
					Dividers(1, p.Hairline).Children(func() {
					for pn := range panelCount {
						a.section(c.Key(pn), p, pn)
					}
				})
				ui.Box(c).Width(1).FillHeight().Background(p.Hairline)
				a.mainPane(c, p)
			})
			a.hintBar(c, p)
		})
}

func (a *App) section(c *ui.Context, p *Palette, pn panel) {
	focused := a.focus == pn
	// Space goes where the content is: every panel is capped at its content
	// height, the focused one weighs more, and none collapses below two rows.
	col := ui.Column(c).MinHeight(0)
	content := float32(28 + max(a.count(pn), 1)*rowHeight + 6)
	switch {
	case pn == panelStatus:
		col.Height(content)
	case focused:
		col.Grow(3).MinHeight(min(content, 28+5*rowHeight)).MaxHeight(content)
	default:
		col.Grow(1).MinHeight(min(content, 28+2*rowHeight+6)).MaxHeight(content)
	}
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
			ui.Text(c, strings.ToUpper(panelTitles[pn])).FontSize(sizeCaption).FontWeight(650).
				LetterSpacing(0.7).TextColor(titleColor)
			if pn != panelStatus {
				ui.Textf(c, "%d", a.count(pn)).FontSize(sizeCaption).TextColor(p.Faint).FontFeatures("tnum")
			}
			if pn == panelFiles && a.snap != nil {
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
			keycap(c, p, fmt.Sprint(int(pn)+1))
		})
		ui.List(c, &a.lists[pn], a.count(pn), func(i int) { a.row(c, p, pn, i) }).Grow(1).MinHeight(0).Padding(0, 0, 6, 0)
	})
}

func (a *App) row(c *ui.Context, p *Palette, pn panel, i int) {
	sel := i == a.cursor[pn]
	row := ui.Row(c).Height(rowHeight).PaddingX(12).Gap(8).AlignItems(ui.Center).Radius(5).MarginX(6)
	switch {
	case sel && a.focus == pn:
		row.Background(p.AccentSoft)
	case sel:
		row.Background(p.Idle)
	case row.Hovered():
		row.Background(p.Hover)
	}
	if row.Clicked() {
		a.focus, a.cursor[pn] = pn, i
		a.loadMain()
	}
	row.Children(func() {
		switch pn {
		case panelStatus:
			a.statusRow(c, p)
		case panelFiles:
			fileRow(c, p, a.snap.Files[i])
		case panelBranches:
			branchRow(c, p, a.snap.Branches[i])
		case panelCommits:
			if i < len(a.snap.Graph) {
				graphCell(c, p, a.snap.Graph[i])
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
	if h := a.snap.Head; h != nil {
		ui.Text(c, "→").TextColor(p.Faint)
		ui.Text(c, h.Name).TextColor(p.Accent).SingleLine().Shrink(1)
		aheadBehind(c, p, h)
	}
}

func fileRow(c *ui.Context, p *Palette, f *models.File) {
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
	dir, name := path.Split(f.Path)
	nameColor := p.Text
	if f.HasStagedChanges && !f.HasUnstagedChanges {
		nameColor = p.Staged
	}
	ui.Text(c, name).TextColor(nameColor).SingleLine().Shrink(0)
	if dir != "" {
		ui.Text(c, strings.TrimSuffix(dir, "/")).TextColor(p.Faint).SingleLine().Shrink(1)
	}
}

func branchRow(c *ui.Context, p *Palette, b *models.Branch) {
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
	ui.Box(c).Grow(1)
	if b.Head {
		return // lazygit shows "*" as the checked-out branch's recency
	}
	ui.Text(c, b.Recency).FontSize(12).TextColor(p.Faint).FontFeatures("tnum")
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
	ui.Box(c).Width(float32(lanes)*laneWidth).Height(rowHeight).Shrink(0).Clip().
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
	for _, tag := range cm.Tags {
		ui.Text(c, tag).FontSize(11).TextColor(p.Accent).Background(p.AccentSoft).Radius(4).PaddingX(5).Shrink(0)
	}
	ui.Text(c, cm.Name).SingleLine().Grow(1).Shrink(1).Tooltip(cm.Name)
	ui.Text(c, ago(cm.UnixTimestamp)).FontSize(12).TextColor(p.Faint).FontFeatures("tnum").Shrink(0)
}

func (a *App) mainPane(c *ui.Context, p *Palette) {
	ui.Column(c).Grow(1).FillHeight().MinWidth(0).Background(p.Panel).Children(func() {
		ui.Column(c).PaddingX(20).PaddingY(14).Gap(4).Children(func() { a.mainHeader(c, p) })
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		ui.List(c, &a.diffList, len(a.diff), func(i int) { diffLineRow(c, p, a.diff[i]) }).
			Grow(1).MinHeight(0).PaddingY(8)
	})
}

func (a *App) mainHeader(c *ui.Context, p *Palette) {
	title := func(s string) { ui.Text(c, s).FontSize(sizeTitle).FontWeight(600).SingleLine() }
	meta := func(build func()) {
		ui.Row(c).Gap(10).AlignItems(ui.Center).FontSize(12).TextColor(p.Muted).Children(build)
	}
	if a.snap == nil || a.count(a.focus) == 0 {
		empty := map[panel]string{panelFiles: "Working tree clean", panelStash: "No stash entries",
			panelBranches: "No branches", panelCommits: "No commits yet"}[a.focus]
		title(panelTitles[a.focus])
		meta(func() { ui.Text(c, empty) })
		return
	}
	i := a.cursor[a.focus]
	switch a.focus {
	case panelStatus:
		title(path.Base(a.snap.Root))
		meta(func() { ui.Text(c, a.snap.Root).SingleLine() })
	case panelFiles:
		f := a.snap.Files[i]
		dir, name := path.Split(f.Path)
		title(name)
		meta(func() {
			if dir != "" {
				ui.Text(c, dir).SingleLine()
			}
			ui.Text(c, fileState(f)).TextColor(p.Unstaged)
		})
	case panelBranches:
		b := a.snap.Branches[i]
		title(b.Name)
		meta(func() {
			if b.UpstreamBranch != "" {
				ui.Text(c, "tracks "+b.UpstreamRemote+"/"+b.UpstreamBranch)
			} else {
				ui.Text(c, "no upstream")
			}
			ui.Text(c, b.Recency)
		})
	case panelCommits:
		cm := a.snap.Commits[i]
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
	ui.Row(c).Height(30).PaddingX(14).Gap(16).AlignItems(ui.Center).Background(p.Panel).Children(func() {
		hint := func(key, label string) {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				keycap(c, p, key)
				ui.Text(c, label).FontSize(12).TextColor(p.Muted)
			})
		}
		k := a.keys
		hint(k.prev.Label()+k.next.Label(), "Navigate")
		hint(k.prevBlock.Label()+k.nextBlock.Label(), "Panels")
		if a.focus == panelFiles {
			hint(k.selectKey.Label(), "Stage")
		}
		hint(k.refresh.Label(), "Refresh")
		hint(k.quit.Label(), "Quit")
		ui.Box(c).Grow(1)
		if a.status != "" {
			ui.Text(c, a.status).FontSize(12).TextColor(p.Conflict).SingleLine().Shrink(1)
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

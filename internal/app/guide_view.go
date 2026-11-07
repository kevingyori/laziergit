package app

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

func guideButton(c *ui.Context, p *Palette, label string, run func()) {
	b := ui.Row(c).PaddingX(10).PaddingY(5).Radius(4).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	if b.Clicked() {
		run()
	}
	b.Children(func() { ui.Text(c, label).TextColor(p.Muted).SingleLine() })
}
func (a *App) guideView(c *ui.Context, p *Palette) {
	g := a.guide
	reviewed := 0
	for _, h := range g.hunks {
		if g.isReviewed(h) {
			reviewed++
		}
	}
	ui.Row(c).Height(54).PaddingX(18).Gap(12).AlignItems(ui.Center).Background(p.Panel).Children(func() {
		ui.Text(c, "laziestgit").FontSize(sizeTitle).FontWeight(650)
		ui.Text(c, "/  review guide").TextColor(p.Muted)
		ui.Box(c).Grow(1)
		scope := "Working tree · staged + unstaged"
		if g.base != "" {
			scope = g.base + " → HEAD · merge base"
		}
		guideButton(c, p, scope+"  [b]", a.guideBase)
		guideButton(c, p, "Claude Code guide  [A]", a.generateGuide)
		guideButton(c, p, "Back to git  [esc]", a.closeGuide)
	})
	ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
	ui.Row(c).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Width(330).FillHeight().Background(p.Panel).Children(func() {
			ui.Row(c).Height(44).PaddingX(18).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "CHANGE STORY").FontSize(sizeCaption).TextColor(p.Muted).FontWeight(650)
				ui.Box(c).Grow(1)
				ui.Textf(c, "%d / %d reviewed", reviewed, len(g.hunks)).FontSize(sizeCaption).TextColor(p.Staged)
			})
			for i, s := range g.sections {
				n := 0
				for _, idx := range s.Hunks {
					if g.isReviewed(g.hunks[idx]) {
						n++
					}
				}
				row := ui.Row(c).PaddingX(12).PaddingY(10).MarginX(6).Radius(4).Gap(8).Cursor(ui.CursorPointer)
				if i == g.section {
					row.Background(p.AccentSoft)
				} else if row.Hovered() {
					row.Background(p.Hover)
				}
				if row.Clicked() {
					a.guideSection(i - g.section)
				}
				row.Children(func() {
					ui.Textf(c, "%02d", i+1).Font(monoFont).FontSize(sizeCaption).TextColor(p.Faint)
					ui.Text(c, s.Title).Grow(1).Shrink(1).SingleLine()
					ui.Textf(c, "%d/%d", n, len(s.Hunks)).FontSize(sizeCaption).TextColor(p.Muted)
				})
			}
			ui.Box(c).Height(1).FillWidth().Background(p.Hairline).MarginY(12)
			ui.Text(c, "IN THIS SECTION").PaddingX(18).FontSize(sizeCaption).TextColor(p.Faint)
			count := 0
			if len(g.sections) > 0 {
				count = len(g.sections[g.section].Hunks)
			}
			ui.List(c, &g.list, count, func(i int) {
				h := g.hunks[g.sections[g.section].Hunks[i]]
				row := ui.Column(c).PaddingX(12).PaddingY(8).MarginX(6).Radius(4).Gap(4).Cursor(ui.CursorPointer)
				if i == g.hunk {
					row.Background(p.Idle)
				} else if row.Hovered() {
					row.Background(p.Hover)
				}
				if row.Clicked() {
					a.guideMove(i - g.hunk)
				}
				row.Children(func() {
					mark, col := "○", p.Muted
					if g.isReviewed(h) {
						mark, col = "✓", p.Staged
					} else if g.reviewed[g.slot(h)] != "" {
						mark, col = "↻", p.Unstaged
					}
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, mark).TextColor(col)
						ui.Text(c, h.Path).Font(monoFont).FontSize(sizeCaption).SingleLine().Shrink(1)
					})
					label := h.Layer + " · " + h.ID
					if mark == "↻" {
						label += " · changed since reviewed"
					}
					ui.Text(c, label).FontSize(sizeCaption).TextColor(col).PaddingX(20)
				})
			}).Grow(1).MinHeight(0).MarginY(10)
			ui.Text(c, "Snapshot review · r reloads changes\nM copies MCP config for Codex / Claude.").FontSize(sizeCaption).TextColor(p.Faint).Padding(12)
		})
		ui.Box(c).Width(1).FillHeight().Background(p.Hairline)
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			ui.Column(c).FillWidth().Padding(22).Gap(8).Background(p.Panel).Children(func() {
				label := g.source
				if g.loading {
					label = "Loading changes…"
				}
				if g.generating {
					label = "Writing AI guide… · you can keep reviewing"
				}
				ui.Text(c, label).FontSize(sizeCaption).TextColor(p.Muted)
				if len(g.sections) > 0 {
					s := g.sections[g.section]
					ui.Text(c, s.Title).FontSize(sizeTitle).FontWeight(600)
					ui.Text(c, s.Summary).LineHeight(1.5)
					if h := g.current(); h != nil {
						label := "Mark reviewed  [space]"
						if g.isReviewed(*h) {
							label = "✓ Reviewed · undo  [space]"
						}
						guideButton(c, p, label, a.guideReview)
					}
				} else if !g.loading {
					ui.Text(c, "No changes to review").FontSize(sizeTitle)
					ui.Text(c, "Choose a branch base with b, or make a working-tree change.").TextColor(p.Muted)
				}
				if g.err != "" {
					ui.Text(c, g.err).TextColor(p.Conflict).FontSize(sizeCaption)
				}
			})
			ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
			var lines []diffLine
			if h := g.current(); h != nil {
				lines = parseDiff(h.Text)
			}
			ui.List(c, &g.diffList, len(lines), func(i int) { diffLineRow(c, p, lines[i]) }).Grow(1).MinHeight(0)
		})
	})
	ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
	ui.Row(c).Height(34).PaddingX(14).Gap(16).AlignItems(ui.Center).Background(p.Panel).Children(func() {
		for _, act := range a.visibleActions() {
			if !act.hint {
				continue
			}
			act := act
			guideButton(c, p, fmt.Sprintf("%s  %s", act.keys.Label(), act.label), act.run)
		}
	})
	a.overlayLayer(c, p)
}

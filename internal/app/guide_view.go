package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/reviewbridge"
)

func guideButton(c *ui.Context, p *Palette, label string, run func()) {
	b := ui.Row(c).PaddingX(5).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	if b.Clicked() {
		run()
	}
	b.Children(func() { ui.Text(c, label).FontSize(sizeCaption).TextColor(p.Muted).SingleLine() })
}
func (g *guideState) mark(h guideHunk, p *Palette) (string, ui.Color) {
	if g.isReviewed(h) {
		return "✓", p.Staged
	}
	if g.reviewed[g.slot(h)] != "" {
		return "↻", p.Unstaged
	}
	return "○", p.Muted
}
func (a *App) guideView(c *ui.Context, p *Palette) {
	g := a.guide
	reviewed := 0
	for _, h := range g.hunks {
		if g.isReviewed(h) {
			reviewed++
		}
	}
	ui.Row(c).Grow(1).MinHeight(0).Children(func() {
		width, _ := c.Size()
		ui.Column(c).Width(a.sideWidth(width)).Shrink(0).FillHeight().Background(p.Panel).Children(func() {
			ui.Row(c).Height(panelHead).PaddingX(12).Gap(8).AlignItems(ui.Center).Children(func() {
				color := p.Muted
				if !g.focusCode && (!g.showDiagram || g.diagramCodeFocus) {
					color = p.Accent
				}
				ui.Text(c, "STORY").FontSize(sizeCaption).FontWeight(650).TextColor(color)
				ui.Textf(c, "%d/%d reviewed", reviewed, len(g.hunks)).FontSize(sizeCaption).TextColor(p.Staged)
				ui.Box(c).Grow(1)
			})
			if g.overview != nil {
				ui.Column(c).PaddingX(12).PaddingY(6).Gap(3).Children(func() {
					ui.Text(c, guideProvenance(g.provenance)).FontSize(sizeCaption).TextColor(p.Faint)
					if !g.expandedNote {
						a.richText(c, p, g.overview.Purpose, p.Text, 3)
						return
					}
					a.richText(c, p, g.overview.Purpose, p.Text, 0)
					if g.overview.NotDone != "" {
						a.richText(c, p, "<i>Not done</i> · "+g.overview.NotDone, p.Muted, 0)
					}
					width, _ := c.Size()
					a.guideMedia(c, p, g.overview.Media, width)
				})
				ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
			}
			count := len(g.sections)
			for _, s := range g.sections {
				count += len(s.Hunks)
			}
			ui.List(c, &g.list, count, func(row int) {
				section, hunk := 0, -1
				n := row
				for i, s := range g.sections {
					if n == 0 {
						section = i
						break
					}
					if n <= len(s.Hunks) {
						section, hunk = i, n-1
						break
					}
					n -= 1 + len(s.Hunks)
				}
				s := g.sections[section]
				if hunk < 0 {
					b := ui.Row(c).Height(rowHeight).PaddingX(12).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					if b.Hovered() {
						b.Background(p.Hover)
					}
					if b.Clicked() {
						a.guideFocus(0)
						g.section, g.hunk = section, 0
						g.revealSelected()
					}
					b.Children(func() {
						ui.Textf(c, "%02d", section+1).Font(monoFont).FontSize(sizeCaption).TextColor(p.Faint)
						ui.Text(c, s.Title).FontWeight(600).SingleLine().Grow(1).Shrink(1)
						done := 0
						for _, idx := range s.Hunks {
							if g.isReviewed(g.hunks[idx]) {
								done++
							}
						}
						ui.Textf(c, "%d/%d", done, len(s.Hunks)).FontSize(sizeCaption).TextColor(p.Faint)
					})
					return
				}
				h := g.hunks[s.Hunks[hunk]]
				selected := section == g.section && hunk == g.hunk
				b := ui.Row(c).Height(rowHeight).PaddingX(12).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				if selected {
					if g.focusCode {
						b.Background(p.Idle)
					} else {
						b.Background(p.AccentSoft)
					}
				} else if b.Hovered() {
					b.Background(p.Hover)
				}
				if b.Clicked() {
					a.guideFocus(0)
					g.section, g.hunk = section, hunk
					g.revealSelected()
				}
				b.Children(func() {
					mark, col := g.mark(h, p)
					ui.Text(c, mark).TextColor(col).Width(14)
					layer := "U"
					if h.Layer == "Staged" {
						layer = "S"
					}
					if h.Layer == "Branch" {
						layer = "·"
					}
					ui.Text(c, layer).Font(monoFont).FontSize(sizeCaption).TextColor(p.Faint)
					ui.Text(c, h.Path).Font(monoFont).FontSize(sizeCaption).SingleLine().Grow(1).Shrink(1)
					// Line ranges distinguish several hunks in the same file without exposing MCP IDs.
					for _, r := range g.streams[section] {
						if r.hunk == s.Hunks[hunk] && (r.newLine > 0 || r.oldLine > 0) {
							ui.Textf(c, ":%d", max(r.newLine, r.oldLine)).Font(monoFont).FontSize(sizeCaption).TextColor(p.Faint)
							break
						}
					}
				})
			}).Grow(1).MinHeight(0)
		})
		ui.Box(c).Width(1).FillHeight().Background(p.Hairline)
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Background(p.Panel).Children(func() {
			ui.Row(c).Height(panelHead).PaddingX(12).Gap(10).AlignItems(ui.Center).Children(func() {
				color := p.Muted
				if g.focusCode {
					color = p.Accent
				}
				title := "Review guide"
				if len(g.sections) > 0 {
					title = g.sections[g.section].Title
				}
				ui.Text(c, title).FontWeight(600).TextColor(color).SingleLine().Grow(1).Shrink(1)
				scope := "Working tree"
				if g.base != "" {
					scope = g.base + " → HEAD"
				}
				guideButton(c, p, scope+" · b", a.guideBase)
				guideButton(c, p, a.keysLabelNote(), func() { g.expandedNote = !g.expandedNote })
				if len(g.sections) > 0 && g.sections[g.section].Diagram != nil {
					guideButton(c, p, "D diagram", a.guideToggleDiagram)
				}
				if len(g.feedback.Items) > 0 {
					guideButton(c, p, fmt.Sprintf("f feedback (%d)", len(g.feedback.Items)), a.guideFeedbackMenu)
				}
				drafts := 0
				for _, item := range g.feedback.Items {
					if item.Status == "draft" {
						drafts++
					}
				}
				if drafts > 0 {
					guideButton(c, p, fmt.Sprintf("S send (%d)", drafts), a.guideSendFeedback)
				}
			})
			ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
			if g.showDiagram {
				a.guideDiagramView(c, p)
			} else {
				rows := g.rows()
				if len(rows) == 0 {
					msg := "No changes to review · b chooses a branch base"
					if g.loading {
						msg = "Loading changes…"
					}
					ui.Text(c, msg).Padding(12).TextColor(p.Muted)
				}
				// The section's explanation stays put while its files scroll.
				if len(g.sections) > 0 {
					_, height := c.Size()
					ui.Scroll(c).MaxHeight(max(80, height*0.3)).FillWidth().Children(func() { a.guideSectionNote(c, p) })
					ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
				}
				draw := func() {
					list := ui.List(c, &g.diffList, len(rows), func(i int) {
						r := rows[i]
						if r.hunk < 0 {
							ui.Box(c).Height(0) // the note is pinned above
							return
						}
						a.guideCodeRow(c, p, i, r)
					}).Grow(1).MinHeight(0)
					if !a.repo.Config.Gui.WrapLinesInDiffView && g.section < len(g.widths) {
						list.MinWidth(max(g.viewportWidth, float32(g.widths[g.section])*monoAdvance()+100))
					}
					g.viewport = list.Bounds().H
				}
				ui.Box(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
					if a.repo.Config.Gui.WrapLinesInDiffView {
						g.viewportWidth, _ = c.Size()
						ui.Column(c).Fill().Children(draw)
					} else {
						container := ui.ScrollHorizontal(c).Fill().TrackScroll(&g.horizontal)
						g.viewportWidth = container.Bounds().W
						container.Children(draw)
					}
					a.guideStickyFile(c, p, rows)
				})
			}
		})
	})
	ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
	ui.Row(c).Height(hintBarHeight).PaddingX(7).Gap(9).AlignItems(ui.Center).Background(p.Panel).ClipX().Children(func() {
		for _, act := range a.visibleActions() {
			if act.hint {
				act := act
				guideButton(c, p, act.keys.Label()+" "+act.label, act.run)
			}
		}
		ui.Box(c).Grow(1)
		// Quiet unless there is something to say: progress, a notice or an error.
		status := ""
		if g.notice != "" {
			status = g.notice
		}
		if g.loading {
			status = "Loading…"
		}
		if g.current() != nil {
			mark, _ := g.mark(*g.current(), p)
			if mark == "↻" {
				status = "Changed since reviewed"
			}
		}
		color := p.Faint
		if g.err != "" {
			status = "Guide error · ! details"
			color = p.Conflict
		}
		label := ui.Text(c, status).FontSize(sizeCaption).TextColor(color).SingleLine().Shrink(1)
		if g.err != "" || g.notice != "" {
			if label.Cursor(ui.CursorPointer).Clicked() {
				a.guideStatus()
			}
		}
	})
	a.overlayLayer(c, p)
}

// guideSectionNote shows why the section exists and, expanded, how it works.
// The provenance says whether the author stated it or a cold reader inferred it.
func (a *App) guideSectionNote(c *ui.Context, p *Palette) {
	g := a.guide
	s := g.sections[g.section]
	box := ui.Column(c).MaxWidth(max(200, g.viewportWidth)).PaddingX(12).PaddingY(5).Gap(3).Cursor(ui.CursorPointer)
	if box.Clicked() {
		g.expandedNote = !g.expandedNote
	}
	box.Children(func() {
		if s.Why == "" {
			note := ui.Text(c, s.Summary).LineHeight(1.3).TextColor(p.Muted)
			if !g.expandedNote {
				note.MaxLines(2)
			}
			return
		}
		ui.Row(c).Gap(6).Children(func() {
			ui.Text(c, guideProvenance(g.provenance)).FontSize(sizeCaption).TextColor(p.Faint)
			if s.Kind != "" {
				ui.Text(c, "· "+s.Kind).FontSize(sizeCaption).TextColor(p.Faint)
			}
		})
		if !g.expandedNote {
			a.richText(c, p, s.Why, p.Text, 2)
			return
		}
		a.richText(c, p, s.Why, p.Text, 0)
		if s.How != "" {
			a.richText(c, p, s.How, p.Muted, 0)
		}
		a.guideMedia(c, p, s.Media, g.viewportWidth)
	})
}
func guideProvenance(p string) string {
	if p == "author" {
		return "Stated by the author"
	}
	return "Inferred by a reader of the code"
}
func (a *App) keysLabelNote() string {
	return a.repo.Config.Keybinding.Universal.Edit.String() + " note"
}

// guideCodeRow is one diff line, followed by the guide's notes anchored to it.
// A note without a line sits under its hunk header.
func (a *App) guideCodeRow(c *ui.Context, p *Palette, i int, r guideStreamRow) {
	g := a.guide
	var notes []reviewbridge.Note
	if g.section < len(g.sections) {
		for _, n := range g.sections[g.section].Notes {
			if n.HunkID != g.hunks[r.hunk].ID {
				continue
			}
			if (n.Line > 0 && n.Line == r.newLine && r.line.kind != lineDel) || (n.Line == 0 && r.line.kind == lineHunk) {
				notes = append(notes, n)
			}
		}
	}
	threads := g.threadsAt(r)
	composing := g.composerAt(r)
	if len(notes)+len(threads) == 0 && !composing {
		a.guideCodeLine(c, p, i, r)
		return
	}
	ui.Column(c).Children(func() {
		a.guideCodeLine(c, p, i, r)
		for _, n := range notes {
			guideNoteView(c, p, n, g.viewportWidth)
		}
		for _, t := range threads {
			a.guideThreadInline(c, p, t, g.viewportWidth)
		}
		if composing {
			a.guideComposerView(c, p, g.viewportWidth)
		}
	})
}
func guideNoteView(c *ui.Context, p *Palette, n reviewbridge.Note, width float32) {
	kind := p.Muted
	if n.Kind == "question" {
		kind = p.Accent
	}
	ui.Row(c).PaddingX(12).PaddingY(4).Gap(8).MaxWidth(max(240, width)).Background(p.Header).Children(func() {
		ui.Box(c).Width(2).FillHeight().Background(kind)
		ui.Column(c).Gap(2).Grow(1).Shrink(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, n.Kind).FontSize(sizeCaption).FontWeight(600).TextColor(kind)
				if n.EndLine > n.Line {
					ui.Text(c, "lines"+strings.TrimPrefix(lineRange(n.Line, n.EndLine), ":")).FontSize(sizeCaption).TextColor(p.Faint)
				}
			})
			ui.Text(c, n.Text).LineHeight(1.35)
			if n.Evidence != "" {
				ui.Text(c, "Evidence · "+n.Evidence).Font(monoFont).FontSize(sizeCaption).TextColor(p.Faint)
			}
		})
	})
}
func (a *App) guideCodeLine(c *ui.Context, p *Palette, i int, r guideStreamRow) {
	g := a.guide
	h := g.hunks[r.hunk]
	selected := r.hunk == g.selectedIndex()
	if r.line.kind == lineFile {
		b := ui.Row(c).Height(rowHeight).PaddingX(10).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		if selected {
			b.Background(p.Idle)
		} else {
			b.Background(p.Header)
		}
		if b.Clicked() {
			a.guideSelectStream(i)
		}
		b.Children(func() {
			mark, col := g.mark(h, p)
			if ui.Text(c, mark).TextColor(col).Width(15).Cursor(ui.CursorPointer).Clicked() {
				a.guideSelectStream(i)
				a.guideReview()
			}
			ui.Text(c, h.Path).Font(monoFont).FontSize(sizeCaption).SingleLine().Grow(1).Shrink(1)
			ui.Text(c, h.Layer).FontSize(sizeCaption).TextColor(p.Muted)
		})
		return
	}
	b := ui.Row(c).MinHeight(diffRowHeight).AlignItems(ui.Center)
	if !a.repo.Config.Gui.WrapLinesInDiffView {
		b.Height(diffRowHeight)
	}
	color := p.Text
	switch r.line.kind {
	case lineAdd:
		b.Background(p.AddBg)
		color = p.AddText
	case lineDel:
		b.Background(p.DelBg)
		color = p.DelText
	case lineHunk:
		b.Background(p.HunkBg)
		color = p.HunkText
	case lineMeta:
		color = p.Meta
	}
	if b.Clicked() {
		a.guideSelectStream(i)
		g.focusCode = true
		g.diagramCodeFocus = g.showDiagram // the diagram stays; the keys read code
	}
	b.Children(func() {
		cursor := ui.Box(c).Width(3).FillHeight().Shrink(0)
		linked := g.showDiagram && r.hunk == g.linkHunk && g.linkFrom > 0 && r.newLine >= g.linkFrom && r.newLine <= g.linkTo && r.line.kind != lineDel
		lo, hi := g.guideRange()
		if (g.focusCode || g.showDiagram) && i >= lo && i <= hi {
			cursor.Background(p.Accent)
		} else if linked {
			cursor.Background(p.Accent.Alpha(0.6)) // the lines the selected shape links to
		} else if selected {
			cursor.Background(p.Accent.Alpha(0.3))
		}
		num := func(n int) {
			text := ""
			if n > 0 {
				text = fmt.Sprint(n)
			}
			ui.Text(c, text).Font(monoFont).FontSize(sizeCaption).TextColor(p.Faint).Width(34).Shrink(0).TextAlign(ui.End).PaddingX(4)
		}
		num(r.oldLine)
		num(r.newLine)
		tokens := r.tokens
		if len(tokens) == 0 {
			text := ui.Text(c, strings.ReplaceAll(r.line.text, "\t", "    ")).Font(monoFont).FontSize(12).TextColor(color).PaddingX(8)
			if a.repo.Config.Gui.WrapLinesInDiffView {
				text.Grow(1).MinWidth(0)
			} else {
				text.SingleLine().NoWrap()
			}
		} else {
			spans := []ui.Span{{Text: r.line.text[:1], Font: monoFont, Size: 12, Color: color}}
			for _, t := range tokens {
				spans = append(spans, ui.Span{Text: strings.ReplaceAll(t.text, "\t", "    "), Font: monoFont, Size: 12, Color: guideTokenColor(t.kind, p)})
			}
			text := ui.RichText(c, spans...).PaddingX(8)
			if a.repo.Config.Gui.WrapLinesInDiffView {
				text.Grow(1).MinWidth(0)
			} else {
				text.SingleLine().NoWrap()
			}
		}
	})
}
func (a *App) guideSelectStream(i int) {
	g := a.guide
	rows := g.rows()
	if i < 0 || i >= len(rows) || rows[i].hunk < 0 {
		return
	}
	for h, idx := range g.sections[g.section].Hunks {
		if idx == rows[i].hunk {
			g.hunk = h
			break
		}
	}
	g.lineCursor = i
	g.list.ScrollIntoView(g.storyRow())
}

// guideStickyFile names the file at the top of the code once its own header
// has scrolled away: path, layer and review mark stay in view. It floats over
// the code, so nothing shifts when it appears.
func (a *App) guideStickyFile(c *ui.Context, p *Palette, rows []guideStreamRow) {
	g := a.guide
	first, _ := g.diffList.Visible()
	if first <= 0 || first >= len(rows) || rows[first].hunk < 0 || rows[first].line.kind == lineFile {
		return
	}
	h := g.hunks[rows[first].hunk]
	b := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(rowHeight).PaddingX(10).Gap(8).AlignItems(ui.Center).Background(p.Header).Cursor(ui.CursorPointer)
	if b.Clicked() {
		for i := first; i >= 0; i-- {
			if rows[i].hunk == rows[first].hunk && rows[i].line.kind == lineFile {
				g.diffList.ScrollTo(i, ui.Start)
				break
			}
		}
	}
	b.Children(func() {
		mark, col := g.mark(h, p)
		ui.Text(c, mark).TextColor(col).Width(15)
		ui.Text(c, h.Path).Font(monoFont).FontSize(sizeCaption).SingleLine().Grow(1).Shrink(1)
		ui.Text(c, h.Layer).FontSize(sizeCaption).TextColor(p.Muted)
	})
}

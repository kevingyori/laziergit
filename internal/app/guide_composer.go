package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/reviewbridge"
)

// guideComposer writes a comment, a draft edit or a reply in place, under
// the line it is about: the code stays visible and scrolls as usual.
type guideComposer struct {
	mode   string // new, edit, reply
	anchor *reviewbridge.Anchor
	item   reviewbridge.Feedback // edit, reply
	text   string
	gen    int
}

var composerGen int

// composerAt reports whether the composer belongs under this code row.
func (g *guideState) composerAt(row guideStreamRow) bool {
	if g.composer == nil || g.composer.anchor == nil || row.hunk < 0 {
		return false
	}
	an, h := g.composer.anchor, g.hunks[row.hunk]
	if an.Path != h.Path || an.Layer != h.Layer {
		return false
	}
	if an.Side == "old" {
		return row.line.kind == lineDel && row.oldLine == an.Line
	}
	return row.line.kind != lineDel && row.newLine == an.Line
}

// openComposer starts writing under anchor, or reports false when the anchor
// isn't in the code on screen (general notes keep the prompt).
func (a *App) openComposer(mode string, anchor *reviewbridge.Anchor, item reviewbridge.Feedback, text string) bool {
	g := a.guide
	if anchor == nil {
		return false
	}
	composerGen++
	g.composer = &guideComposer{mode: mode, anchor: anchor, item: item, text: text, gen: composerGen}
	for i, row := range g.rows() {
		if g.composerAt(row) {
			g.focusCode = true
			g.lineCursor = i
			g.diffList.ScrollIntoView(min(i+2, len(g.rows())-1))
			return true
		}
	}
	g.composer = nil
	return false
}
func (a *App) submitComposer(text string) {
	g := a.guide
	cm := g.composer
	g.composer = nil
	text = strings.TrimSpace(text)
	if cm == nil || text == "" {
		return
	}
	var err error
	switch cm.mode {
	case "new":
		err = reviewbridge.SaveDraft(g.session, reviewbridge.Feedback{Kind: "line", Revision: g.revision, Anchor: cm.anchor, Body: text})
		g.notice = "Comment saved as draft · S sends review"
	case "edit":
		next := cm.item
		next.Body, next.Anchor, next.Revision = text, cm.anchor, g.revision
		err = reviewbridge.SaveDraft(g.session, next)
		g.notice = "Draft updated"
	case "reply":
		err = reviewbridge.ReplyAsReviewer(g.session, cm.item.ID, text)
		g.notice = "Reply sent to agents"
	}
	if err != nil {
		g.err, g.notice = err.Error(), ""
	} else {
		g.err = ""
	}
	a.loadGuideFeedback()
}
func (a *App) guideComposerView(c *ui.Context, p *Palette, width float32) {
	g := a.guide
	cm := g.composer
	title, hint := "New comment", "Private draft until S sends review"
	color := p.MergedPR
	switch cm.mode {
	case "edit":
		title = "Edit draft"
	case "reply":
		title, hint, color = "Reply to agents", "Sent through MCP when you press Enter", p.Accent
	}
	ui.Row(c).PaddingX(12).PaddingY(6).Gap(8).MaxWidth(max(320, width)).Background(p.Header).Children(func() {
		ui.Box(c).Width(2).FillHeight().Background(color)
		ui.Column(c).Gap(4).Grow(1).Shrink(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, title).FontSize(sizeCaption).FontWeight(600)
				ui.Text(c, fmt.Sprintf("%s · enter saves · shift+enter new line · esc cancels", hint)).FontSize(sizeCaption).TextColor(p.Muted).SingleLine().Shrink(1)
			})
			ui.TextArea(c.Key(fmt.Sprintf("composer%d", cm.gen)), &cm.text).Lines(2, 10).FillWidth().
				Placeholder("Write a comment…").Label(title).AutoFocus().
				OnTextKey(func(k ui.TextKey) bool {
					switch {
					case k.Key == ui.KeyEscape:
						g.composer = nil
						return true
					case k.Key == ui.KeyEnter && k.Mods&ui.Shift == 0:
						a.submitComposer(k.Text())
						return true
					}
					return false
				})
		})
	})
}

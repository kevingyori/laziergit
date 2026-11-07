package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/config"
	"laziergit/internal/keymap"
	"laziergit/internal/reviewbridge"
)

func (a *App) loadGuideFeedback() {
	g := a.guide
	if g == nil || g.session == "" {
		return
	}
	state, err := reviewbridge.LoadFeedback(g.session)
	if err != nil {
		g.err = err.Error()
		return
	}
	g.feedback = state
}
// guideCursorAnchor anchors feedback on the selected code line. Deleted lines
// anchor on the old side; every other line on the new side.
func (a *App) guideCursorAnchor() (*reviewbridge.Anchor, bool) {
	g := a.guide
	if !g.focusCode {
		a.guideEnter()
	}
	rows := g.rows()
	if g.lineCursor < 0 || g.lineCursor >= len(rows) {
		return nil, false
	}
	row := rows[g.lineCursor]
	if row.hunk < 0 || (row.oldLine == 0 && row.newLine == 0) {
		g.notice = "Select a code line first"
		return nil, false
	}
	h := g.hunks[row.hunk]
	side, line := "new", row.newLine
	if row.line.kind == lineDel {
		side, line = "old", row.oldLine
	}
	text := row.line.text
	if len(text) > 0 {
		text = text[1:]
	}
	return &reviewbridge.Anchor{Path: h.Path, Layer: h.Layer, Side: side, Line: line, Hunk: h.Fingerprint, Text: text}, true
}
func (a *App) guideComment() {
	g := a.guide
	if g == nil || g.current() == nil {
		return
	}
	if g.showDiagram {
		a.guideDiagramComment()
		return
	}
	anchor, ok := a.guideCursorAnchor()
	if !ok {
		return
	}
	item := reviewbridge.Feedback{Kind: "line", Revision: g.revision, Anchor: anchor}
	a.prompt(fmt.Sprintf("Comment · %s:%d (%s)", anchor.Path, anchor.Line, anchor.Side), "", "Saved privately as a draft. S sends your review to connected agents.", func(body string) {
		item.Body = strings.TrimSpace(body)
		if err := reviewbridge.SaveDraft(g.session, item); err != nil {
			g.err = err.Error()
			return
		}
		a.loadGuideFeedback()
		g.notice = "Comment saved as draft · S sends review"
		g.err = ""
	})
}
func (a *App) guideSendFeedback() {
	g := a.guide
	a.loadGuideFeedback()
	count := 0
	for _, item := range g.feedback.Items {
		if item.Status == "draft" {
			count++
		}
	}
	if count == 0 {
		g.notice = "No draft comments or diagram proposals to send"
		return
	}
	a.confirm("Send review to agents?", fmt.Sprintf("Send %d draft feedback items through MCP. Nothing is posted to a PR.", count), false, func() {
		if err := reviewbridge.SendFeedback(g.session); err != nil {
			g.err = err.Error()
			return
		}
		a.loadGuideFeedback()
		g.notice = "Review sent · agents can read it through MCP"
		g.err = ""
	})
}
func (a *App) guideFeedbackMenu() {
	a.loadGuideFeedback()
	g := a.guide
	items := []menuItem{}
	for _, item := range g.feedback.Items {
		item := item
		title := item.Body
		if item.Diagram != nil {
			title = item.Diagram.Section + ": " + strings.Join(item.Diagram.Changes, "; ")
		}
		status := feedbackStatus(item)
		anchor := item.CurrentAnchor
		if anchor == nil {
			anchor = item.Anchor
		}
		if anchor != nil {
			title = fmt.Sprintf("%s:%d · %s", anchor.Path, anchor.Line, title)
		}
		if len(item.Replies) > 0 {
			title += fmt.Sprintf(" · %d replies", len(item.Replies))
		}
		label := fmt.Sprintf("[%s] %s", status, title)
		items = append(items, menuItem{label: label, run: func() { a.guideFeedbackItem(item) }})
	}
	if len(items) == 0 {
		items = append(items, menuItem{label: "No feedback yet · c comments on a line", disabled: "No feedback"})
	}
	a.pushOverlay(&menuOverlay{title: "Review feedback · private to this worktree", items: items})
}
func (a *App) guideFeedbackItem(item reviewbridge.Feedback) {
	g := a.guide
	save := func(next reviewbridge.Feedback, notice string) {
		if err := reviewbridge.SaveDraft(g.session, next); err != nil {
			g.err = err.Error()
		} else {
			g.err, g.notice = "", notice
		}
		a.loadGuideFeedback()
	}
	items := []menuItem{{label: "Open thread", run: func() { a.pushOverlay(&threadOverlay{id: item.ID}) }}}
	if item.Status == "draft" {
		if item.Diagram == nil {
			items = append(items, menuItem{label: "Edit draft", run: func() {
				a.prompt("Edit draft", item.Body, "Private until S sends review", func(body string) {
					next := item
					next.Body = body
					if next.Anchor != nil && next.Placement != "outdated" {
						// A safely moved draft adopts its current anchor.
						next.Anchor = next.CurrentAnchor
					}
					if next.Placement == "outdated" && next.Revision != g.revision {
						g.notice = "Outdated draft · re-anchor or convert it before editing"
						return
					}
					next.Revision = g.revision
					save(next, "Draft updated")
				})
			}})
		}
		if item.Anchor != nil && item.Diagram == nil {
			reason := ""
			if !g.focusCode && !g.showDiagram {
				reason = "Enter the code and select a line first"
			}
			items = append(items, menuItem{label: "Re-anchor to selected line", disabled: reason, run: func() {
				anchor, ok := a.guideCursorAnchor()
				if !ok {
					return
				}
				next := item
				next.Anchor, next.Revision, next.Kind = anchor, g.revision, "line"
				save(next, fmt.Sprintf("Draft re-anchored to %s:%d", anchor.Path, anchor.Line))
			}})
			items = append(items, menuItem{label: "Convert to general review note", run: func() {
				next := item
				if next.Anchor != nil {
					next.Body = fmt.Sprintf("(Originally on %s:%d: %s)\n%s", next.Anchor.Path, next.Anchor.Line, strings.TrimSpace(next.Anchor.Text), next.Body)
				}
				next.Anchor, next.Revision, next.Kind = nil, g.revision, "general"
				save(next, "Draft converted to a general review note")
			}})
		}
		items = append(items, menuItem{label: "Delete draft", run: func() {
			if err := reviewbridge.DeleteDraft(g.session, item.ID); err != nil {
				g.err = err.Error()
			}
			if item.Diagram != nil {
				a.discardDiagramProposal(item)
			}
			a.loadGuideFeedback()
		}})
	} else {
		items = append(items, menuItem{label: "Reply to agents", run: func() { a.guideReply(item.ID) }})
		label := "Resolve thread"
		if item.Resolved {
			label = "Reopen thread"
		}
		items = append(items, menuItem{label: label, run: func() {
			if err := reviewbridge.ResolveFeedback(g.session, item.ID, !item.Resolved); err != nil {
				g.err = err.Error()
			}
			a.loadGuideFeedback()
		}})
	}
	a.pushOverlay(&menuOverlay{title: "Feedback actions", items: items})
}
func (a *App) guideReply(id string) {
	g := a.guide
	a.prompt("Reply to agents", "", "Sent through MCP as soon as you submit. Nothing is posted to a PR.", func(body string) {
		if err := reviewbridge.ReplyAsReviewer(g.session, id, body); err != nil {
			g.err = err.Error()
		} else {
			g.err, g.notice = "", "Reply sent to agents"
		}
		a.loadGuideFeedback()
	})
}
func feedbackStatus(item reviewbridge.Feedback) string {
	status := item.Status
	switch {
	case item.Resolved:
		status = "Resolved"
	case item.Status == "draft":
		status = "Draft · private"
	case item.Status == "sent":
		status = "Sent"
	case item.Status == "addressed":
		status = "Agent says addressed"
	case item.Status == "acknowledged":
		status = "Agent acknowledged"
	case item.Status == "question":
		status = "Agent asks"
	case item.Status == "declined":
		status = "Agent declined"
	}
	if item.Placement == "outdated" {
		status += " · outdated"
	} else if item.Placement == "moved" {
		status += " · moved"
	}
	return status
}

// threadOverlay reads one feedback thread: anchor, reviewer note, proposed
// diagram operations and agent replies. It reloads by ID, so replies that
// arrive while it is open appear on the next refresh.
type threadOverlay struct {
	id     string
	scroll ui.ScrollState
}

func (o *threadOverlay) item(a *App) (reviewbridge.Feedback, bool) {
	if a.guide == nil {
		return reviewbridge.Feedback{}, false
	}
	for _, item := range a.guide.feedback.Items {
		if item.ID == o.id {
			return item, true
		}
	}
	return reviewbridge.Feedback{}, false
}
func (o *threadOverlay) key(a *App, ch keymap.Chord) {
	item, ok := o.item(a)
	switch {
	case !ok || a.keys.ret.Matches(ch) || a.keys.confirmMenu.Matches(ch):
		a.closeOverlay(o)
	case keymap.Parse(config.Keybinding{"r"}).Matches(ch) && item.Status != "draft":
		a.guideReply(o.id)
	case a.keys.next.Matches(ch):
		o.scroll.Y += 40
	case a.keys.prev.Matches(ch):
		o.scroll.Y = max(0, o.scroll.Y-40)
	}
}
func (o *threadOverlay) view(a *App, c *ui.Context, p *Palette) {
	item, ok := o.item(a)
	if !ok {
		return
	}
	statusColor := p.Accent
	switch {
	case item.Resolved:
		statusColor = p.Faint
	case item.Status == "draft":
		statusColor = p.MergedPR
	case item.Placement == "outdated":
		statusColor = p.Conflict
	}
	card(c, p, 620, func() {
		ui.Row(c).Padding(16, 18, 6, 18).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(statusColor)
			title := "Review comment"
			if item.Diagram != nil {
				title = "Diagram proposal · " + item.Diagram.Section
			} else if item.Kind == "diagram_note" {
				title = "Diagram note"
			} else if item.Anchor == nil {
				title = "General review note"
			}
			ui.Text(c, title).FontWeight(600).FontSize(15).Grow(1).Shrink(1).SingleLine()
			ui.Text(c, feedbackStatus(item)).FontSize(sizeCaption).TextColor(statusColor)
		})
		ui.Scroll(c).TrackScroll(&o.scroll).MaxHeight(380).Padding(0, 18, 8, 18).Children(func() {
			ui.Column(c).Gap(10).Children(func() {
				anchor := item.CurrentAnchor
				if anchor == nil {
					anchor = item.Anchor
				}
				if anchor != nil {
					ui.Column(c).Gap(2).Padding(6, 8, 6, 8).Background(p.HunkBg).Radius(4).Children(func() {
						ui.Text(c, fmt.Sprintf("%s:%d · %s side · %s", anchor.Path, anchor.Line, anchor.Side, anchor.Layer)).Font(monoFont).FontSize(sizeCaption).TextColor(p.Muted)
						ui.Text(c, anchor.Text).Font(monoFont).FontSize(12).SingleLine()
					})
				}
				if item.Placement == "outdated" {
					ui.Text(c, "The reviewed code changed and this line can't be found unambiguously. Not resolved: re-anchor, convert or delete it from the feedback menu.").FontSize(sizeCaption).TextColor(p.Conflict).LineHeight(1.4)
				}
				if item.Body != "" && item.Diagram == nil {
					ui.Text(c, item.Body).LineHeight(1.45).Selectable()
				}
				if item.Diagram != nil {
					ui.Text(c, "Your proposed changes to the agent's diagram").FontSize(sizeCaption).TextColor(p.Muted)
					for _, change := range item.Diagram.Changes {
						ui.Text(c, "• "+change).TextColor(p.MergedPR).LineHeight(1.4)
					}
				}
				for _, reply := range item.Replies {
					ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
					who := reply.Agent + " · " + reply.Kind
					if reply.Kind == reviewbridge.ReviewerReply {
						who = "You"
					}
					ui.Text(c, who).FontSize(sizeCaption).FontWeight(600).TextColor(p.Muted)
					ui.Text(c, reply.Body).LineHeight(1.45).Selectable()
				}
			})
		})
		footer(c, p, func() {
			if item.Status != "draft" && button(c, p, "Reply", "r", false, false) {
				a.guideReply(o.id)
			}
			if button(c, p, "Close", a.keys.ret.Label(), true, false) {
				a.closeOverlay(o)
			}
		})
	})
}
func (g *guideState) commentsOn(row guideStreamRow) int {
	if row.hunk < 0 {
		return 0
	}
	h := g.hunks[row.hunk]
	count := 0
	for _, item := range g.feedback.Items {
		an := item.CurrentAnchor
		if an == nil {
			an = item.Anchor
		}
		if an == nil || item.Resolved || item.Placement == "outdated" || an.Path != h.Path || an.Layer != h.Layer {
			continue
		}
		if (an.Side == "new" && an.Line == row.newLine) || (an.Side == "old" && an.Line == row.oldLine) {
			count++
		}
	}
	return count
}

package app

import (
	"fmt"
	"strings"

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
func (a *App) guideComment() {
	g := a.guide
	if g == nil || g.current() == nil {
		return
	}
	if g.showDiagram {
		a.guideDiagramComment()
		return
	}
	if !g.focusCode {
		a.guideEnter()
	}
	rows := g.rows()
	if g.lineCursor < 0 || g.lineCursor >= len(rows) {
		return
	}
	row := rows[g.lineCursor]
	if row.oldLine == 0 && row.newLine == 0 {
		g.notice = "Select a code line before commenting"
		return
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
	anchor := &reviewbridge.Anchor{Path: h.Path, Layer: h.Layer, Side: side, Line: line, Hunk: h.Fingerprint, Text: text}
	item := reviewbridge.Feedback{Kind: "line", Revision: g.revision, Anchor: anchor}
	a.prompt(fmt.Sprintf("Comment · %s:%d (%s)", h.Path, line, side), "", "Saved privately as a draft. S sends your review to connected agents.", func(body string) {
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
		status := item.Status
		if item.Status == "addressed" {
			status = "Agent says addressed"
		}
		if item.Resolved {
			status = "Resolved"
		}
		if item.Placement == "outdated" {
			status += " · outdated"
		} else if item.Placement == "moved" {
			status += " · moved"
		}
		anchor := item.CurrentAnchor
		if anchor == nil {
			anchor = item.Anchor
		}
		if anchor != nil {
			title = fmt.Sprintf("%s:%d · %s", anchor.Path, anchor.Line, title)
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
	items := []menuItem{{label: "Read comment and agent replies", run: func() {
		text := item.Body
		if item.Diagram != nil {
			text += "\n\n" + strings.Join(item.Diagram.Changes, "\n")
		}
		if item.Anchor != nil {
			text = fmt.Sprintf("%s:%d (%s, %s)\n%s\n\n%s", item.Anchor.Path, item.Anchor.Line, item.Anchor.Side, item.Anchor.Layer, item.Anchor.Text, text)
		}
		if item.Placement == "outdated" {
			text = "Outdated anchor · the reviewed snapshot has changed. This is not resolved.\n\n" + text
		}
		for _, reply := range item.Replies {
			text += "\n\n" + reply.Agent + " · " + reply.Kind + ":\n" + reply.Body
		}
		a.showError("Review thread", fmt.Errorf("%s", text))
	}}}
	if item.Status == "draft" {
		items = append(items, menuItem{label: "Delete draft", run: func() {
			if err := reviewbridge.DeleteDraft(g.session, item.ID); err != nil {
				g.err = err.Error()
			}
			a.loadGuideFeedback()
		}})
		if item.Diagram == nil {
			items = append(items, menuItem{label: "Edit draft", run: func() {
				a.prompt("Edit draft", item.Body, "Private until S sends review", func(body string) {
					item.Body = body
					if item.CurrentAnchor != nil && item.Placement != "outdated" {
						item.Anchor = item.CurrentAnchor
						item.Revision = g.revision
					}
					if err := reviewbridge.SaveDraft(g.session, item); err != nil {
						g.err = err.Error()
					}
					a.loadGuideFeedback()
				})
			}})
		}
	} else {
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

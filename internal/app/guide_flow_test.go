package app

import (
	"strings"
	"testing"

	"laziergit/internal/reviewbridge"
	"laziergit/internal/testrepo"
)

// The foot of the story always names the next step: connect an agent, read
// its doubts, send it back or commit. C hands the commit to lazier.
func TestGuideNextStepLeadsToSendBackOrCommit(t *testing.T) {
	a, tt, dir := newTestApp(t)
	var copied []string
	a.copyText = func(s string) error { copied = append(copied, s); return nil }
	press(tt, "~")
	defer a.closeGuide()
	g := a.guide
	if !tt.HasText("Grouped by file type · no agent guide yet") || !tt.HasText("M connect") || !tt.HasText("A ask") {
		t.Fatalf("no way in to the agent flow: %v", tt.Texts())
	}
	press(tt, "A")
	if len(copied) != 1 || copied[0] != reviewbridge.Prompt || g.notice != guideWaiting {
		t.Fatalf("A: %q %q", copied, g.notice)
	}
	ids := []string{}
	for _, h := range g.hunks {
		ids = append(ids, h.ID)
	}
	guide := reviewbridge.Guide{Revision: g.revision, Provenance: "inferred", Overview: &reviewbridge.Overview{Purpose: "A returns early."}, Sections: []reviewbridge.Section{{Title: "Stop printing from A", Kind: "core", Why: "Callers must not see output.", IDs: ids,
		Notes: []reviewbridge.Note{{HunkID: ids[0], Kind: "question", Text: "A has callers outside this repository?", Evidence: "search A\\(: none here"}}}}}
	reviewCall(t, g.session, "publish_review_guide", guide)
	a.Refresh()
	tt.Frame()
	if g.notice != "The agent's guide arrived" || !tt.HasText("1 question") || !tt.HasText("?") || !tt.HasText("Guide by MCP client · a reader of the code") {
		t.Fatalf("guide arrival: %q %v", g.notice, tt.Texts())
	}

	// Send it back: S copies what to tell the agent.
	press(tt, "c")
	tt.Type("Not what I asked for.")
	press(tt, "enter")
	tt.Frame()
	if !tt.HasText("1 draft comment") || !tt.HasText("S send back") {
		t.Fatalf("draft not surfaced: %v", tt.Texts())
	}
	press(tt, "S")
	press(tt, "enter")
	tt.Frame()
	if copied[len(copied)-1] != reviewbridge.FeedbackPrompt || !strings.Contains(g.notice, "prompt copied") || !tt.HasText("1 comment with the agent") {
		t.Fatalf("send back: %q %v", g.notice, tt.Texts())
	}
	id := g.feedback.Items[0].ID
	reviewCall(t, g.session, "reply_to_feedback", map[string]any{"item_id": id, "kind": "addressed", "body": "Fixed."})
	a.Refresh()
	tt.Frame()
	if g.notice != "The agent answered 1 comment · f opens the feedback" {
		t.Fatalf("reply not announced: %q", g.notice)
	}

	// The agent's fix moves the code past the snapshot; the reviewer is told.
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() { return } // fixed\n")
	a.Refresh()
	tt.Frame()
	if !g.moved || !tt.HasText("The code changed since this snapshot") || !tt.HasText("r reload") {
		t.Fatalf("moved code not noticed: %v", tt.Texts())
	}
	press(tt, "r")
	tt.Frame()
	if g.moved || !tt.HasText("The agent's guide is for older code") {
		t.Fatalf("stale guide not named: %v", tt.Texts())
	}

	// Commit: every hunk reviewed, then C opens lazier's own commit flow.
	for range g.hunks {
		press(tt, "space")
		press(tt, "]")
	}
	tt.Frame()
	if !tt.HasText("All reviewed") || !tt.HasText("C commit") {
		t.Fatalf("no commit step: %v", tt.Texts())
	}
	press(tt, "C")
	if g.active || a.focus != panelFiles || a.topOverlay() == nil {
		t.Fatalf("C must hand over to lazier's commit flow: active=%v overlay=%T", g.active, a.topOverlay())
	}
}

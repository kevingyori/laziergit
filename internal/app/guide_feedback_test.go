package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/godraw/doc"
	"laziergit/internal/reviewbridge"
	"laziergit/internal/testrepo"
)

func reviewCall(t *testing.T, dir, name string, args any) string {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	var out bytes.Buffer
	if err := reviewbridge.Serve(dir, strings.NewReader(string(data)+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), `"isError":true`) {
		t.Fatal(out.String())
	}
	return out.String()
}
func TestGuideCommentsPersistAndReachAgents(t *testing.T) {
	a, tt, dir := newTestApp(t)
	press(tt, "~")
	defer a.closeGuide()
	g := a.guide
	press(tt, "enter")
	for i, r := range g.rows() {
		if r.line.kind == lineDel {
			a.guideSelectStream(i)
			break
		}
	}
	press(tt, "c")
	tt.Type("Please preserve the existing failure behavior.")
	press(tt, "enter")
	if len(g.feedback.Items) != 1 {
		t.Fatal("comment not saved")
	}
	item := g.feedback.Items[0]
	if item.Anchor == nil || item.Anchor.Side != "old" || item.Anchor.Line == 0 || item.Status != "draft" {
		t.Fatalf("bad deletion anchor: %+v", item)
	}
	before, err := reviewbridge.AgentFeedback(g.session, 0)
	if err != nil || len(before.Items) != 0 {
		t.Fatal("agent can see a private draft")
	}
	press(tt, "S")
	press(tt, "enter")
	after, err := reviewbridge.AgentFeedback(g.session, before.Cursor)
	if err != nil || len(after.Items) != 1 {
		t.Fatalf("sent comment not delivered: %+v %v", after, err)
	}
	reviewCall(t, g.session, "reply_to_feedback", map[string]any{"item_id": item.ID, "kind": "addressed", "body": "Kept the existing error behavior and added coverage."})
	a.Refresh()
	tt.Frame()
	if g.feedback.Items[0].Status != "addressed" || g.feedback.Items[0].Resolved {
		t.Fatal("agent must not resolve human feedback")
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/review-comment.png")
		press(tt, "f")
		shot(t, tt, out+"/review-feedback.png")
		press(tt, "esc")
	}
	store := g.session
	a.closeGuide()
	press(tt, "~")
	if a.guide.session != store || len(a.guide.feedback.Items) != 1 || len(a.guide.feedback.Items[0].Replies) != 1 {
		t.Fatal("restart lost feedback")
	}
	resolved, err := reviewbridge.ResolveReview(dir)
	if err != nil || resolved != store {
		t.Fatalf("stable MCP discovery: %s %v", resolved, err)
	}
	testrepo.Write(t, dir, "a.go", "package a\nfunc Changed() {}\n")
	press(tt, "r")
	if a.guide.feedback.Items[0].Revision == a.guide.revision || a.guide.feedback.Items[0].Resolved {
		t.Fatal("changed code must not resolve old feedback")
	}
	// Persistent review state never appears among working-tree files.
	if files := git(t, dir, "status", "--porcelain"); strings.Contains(files, "laziergit/review") {
		t.Fatal("review artifacts leaked into git status")
	}
}
func TestGuideEmbeddedGodrawProposalRoundTrip(t *testing.T) {
	a, tt, _ := newTestApp(t)
	press(tt, "~")
	defer a.closeGuide()
	g := a.guide
	ids := []string{}
	for _, h := range g.hunks {
		ids = append(ids, h.ID)
	}
	diagram := &reviewbridge.Diagram{Title: "Keep the review tied to the code", Nodes: []reviewbridge.DiagramNode{{ID: "snapshot", Label: "Reviewed snapshot", HunkID: ids[0]}, {ID: "render", Label: "Render the same code", HunkID: ids[len(ids)-1]}}, Edges: []reviewbridge.DiagramEdge{{From: "snapshot", To: "render", Label: "immutable hunks"}}}
	guide := reviewbridge.Guide{Revision: g.revision, Sections: []reviewbridge.Section{{Title: "Keep the reviewed code stable", Summary: "The reviewer must see the exact changes behind the story, even while an agent edits other files.", IDs: ids, Diagram: diagram}}}
	reviewCall(t, g.session, "publish_review_guide", guide)
	a.Refresh()
	tt.Frame()
	press(tt, "D")
	if !g.showDiagram || g.drawing == nil || len(g.drawing.Editor().Doc.Shapes) != 3 {
		t.Fatal("godraw editor did not embed")
	}
	press(tt, "j")
	press(tt, "e")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Validate before rendering")
	press(tt, "enter")
	if len(g.feedback.Items) != 1 || g.feedback.Items[0].Kind != "diagram_proposal" {
		t.Fatalf("diagram change not saved as feedback: %+v", g.feedback)
	}
	proposal := g.feedback.Items[0].Diagram
	if len(proposal.Changes) == 0 || proposal.Baseline.Nodes[0].Label != "Reviewed snapshot" {
		t.Fatal("baseline mutated")
	}
	if _, err := doc.Read(bytes.NewReader(proposal.ProposedDocument)); err != nil {
		t.Fatal("proposal is not a native .tldr document", err)
	}
	if out := shotDir(); out != "" {
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/review-diagram-proposal.png")
		press(tt, "v")
		shot(t, tt, out+"/review-diagram-baseline.png")
		press(tt, "v")
		tt.SetDark(false)
		tt.Frame()
		shot(t, tt, out+"/review-diagram-light.png")
	}
	press(tt, "v")
	if !g.baselineView || !g.drawing.Editor().ReadOnly {
		t.Fatal("baseline should be immutable")
	}
	press(tt, "v")
	press(tt, "S")
	press(tt, "enter")
	sent, err := reviewbridge.AgentFeedback(g.session, 0)
	if err != nil || len(sent.Items) != 1 || sent.Items[0].Diagram == nil {
		t.Fatal("agent did not receive diagram delta")
	}
	// Layout-only adjustments after sending are persisted without creating semantic feedback.
	a.guideDiagramMove(0)
	g.drawing.NudgeSelection(20, 0)
	drafts := 0
	for _, item := range g.feedback.Items {
		if item.Status == "draft" {
			drafts++
		}
	}
	if drafts != 0 {
		t.Fatal("layout-only move created duplicate feedback")
	}
	path := filepath.Join(g.session, "proposal-"+g.drawingKey+".tldr")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("proposal not persisted")
	}
	g.drawing = nil
	a.ensureDiagram()
	if g.drawing == nil {
		t.Fatal("proposal not restored")
	}
	press(tt, "s")
	press(tt, "j")
	press(tt, "enter")
	if g.showDiagram || !g.focusCode {
		t.Fatal("diagram anchor did not jump to code")
	}
	// Guide publication can replace a diagram without changing the code snapshot.
	guide.Sections[0].Diagram.Title = "Updated explanation"
	reviewCall(t, g.session, "publish_review_guide", guide)
	a.Refresh()
	tt.Frame()
	if g.sections[0].Diagram.Title != "Updated explanation" {
		t.Fatal("same-revision guide update was ignored")
	}
}

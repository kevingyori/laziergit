package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/reviewbridge"
	"laziergit/internal/testrepo"
)

func TestGuideReviewAndMCPRoundTrip(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "a_test.go", "package a\nfunc ExampleA() { A() }\n")
	testrepo.Write(t, dir, "go.sum", "example v1 hash\n")
	tt.TypeKey(ui.Ctrl, ui.KeyG, "g")
	tt.Frame()
	defer a.closeGuide()
	g := a.guide
	if g == nil || len(g.hunks) < 3 {
		t.Fatalf("missing review snapshot: %+v", g)
	}
	if !tt.HasText("Implementation") || !tt.HasText("Tests") {
		t.Fatalf("missing sections: %v", tt.Texts())
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/guide-local-light.png")
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/guide-local-dark.png")
		tt.SetDark(false)
		tt.Frame()
	}
	first := *g.current()
	press(tt, "space")
	if !g.isReviewed(first) {
		t.Fatal("space must review")
	}
	if out := shotDir(); out != "" {
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/guide-reviewed-dark.png")
		tt.SetDark(false)
		tt.Frame()
	}
	// Review mode's c must not open a commit editor or mutate the index.
	press(tt, "c")
	if a.topOverlay() != nil {
		t.Fatal("git action leaked into review mode")
	}
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() { panic(\"changed\") }\n")
	press(tt, "r")
	if g.isReviewed(*g.current()) {
		t.Fatal("edited hunk retained review mark")
	}
	if !tt.HasText("Unstaged · h1 · changed since reviewed") {
		t.Fatalf("missing stale marker: %v", tt.Texts())
	}
	if out := shotDir(); out != "" {
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/guide-changed-dark.png")
		tt.SetDark(false)
		tt.Frame()
	}
	// Exercise the actual stdio MCP tools, then import the published guide.
	var snapshot reviewbridge.Snapshot
	if err := reviewbridge.Read(g.session, "snapshot.json", &snapshot); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, h := range snapshot.Hunks {
		ids = append(ids, h.ID)
	}
	guide := reviewbridge.Guide{Revision: snapshot.Revision, Sections: []reviewbridge.Section{{Title: "Protect the A call path", Summary: "A now raises an error; check how callers and tests handle the failure.", IDs: ids}}}
	args, _ := json.Marshal(guide)
	input := fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"get_review_diff\",\"arguments\":{}}}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"publish_review_guide\",\"arguments\":%s}}\n", args)
	var output strings.Builder
	if err := reviewbridge.Serve(g.session, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `"isError":true`) {
		t.Fatal(output.String())
	}
	a.Refresh()
	tt.Frame()
	if !strings.HasPrefix(g.source, "AI guide") || !tt.HasText("Protect the A call path") {
		t.Fatalf("guide not imported: %v", tt.Texts())
	}
	press(tt, "]")
	if g.isReviewed(*g.current()) {
		t.Fatal("next unread must skip reviewed hunks")
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/guide-ai-light.png")
		tt.SetDark(true)
		tt.Frame()
		shot(t, tt, out+"/guide-ai-dark.png")
	}
	press(tt, "esc")
	if a.guide != nil || !tt.HasText("FILES") {
		t.Fatal("escape should restore git view")
	}
}
func TestGuideBranchAndPartialStaging(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() { println(\"index\") }\n")
	git(t, dir, "add", "a.go")
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() { println(\"worktree\") }\n")
	tt.TypeKey(ui.Ctrl, ui.KeyG, "g")
	tt.Frame()
	defer a.closeGuide()
	var staged, unstaged bool
	for _, h := range a.guide.hunks {
		staged = staged || h.Layer == "Staged"
		unstaged = unstaged || h.Layer == "Unstaged"
	}
	if !staged || !unstaged {
		t.Fatal("partially staged changes must include both layers")
	}
	git(t, dir, "checkout", "-qb", "review-test")
	git(t, dir, "commit", "-qm", "change A")
	a.guide.base = "main"
	a.refreshGuide()
	if len(a.guide.hunks) != 1 || !strings.Contains(a.guide.hunks[0].Text, "index") || strings.Contains(a.guide.hunks[0].Text, "worktree") {
		t.Fatalf("wrong branch diff: %+v", a.guide.hunks)
	}
	a.guide.base = "does-not-exist"
	a.refreshGuide()
	if a.guide.err == "" {
		t.Fatal("invalid base should be visible")
	}
}
func TestGuideRejectsInvalidAgentResults(t *testing.T) {
	s := reviewbridge.Snapshot{Revision: "current", Hunks: []reviewbridge.Hunk{{ID: "h1"}, {ID: "h2"}}}
	for _, g := range []reviewbridge.Guide{
		{Revision: "old"},
		{Revision: "current", Sections: []reviewbridge.Section{{Title: "x", Summary: "y", IDs: []string{"h1"}}}},
		{Revision: "current", Sections: []reviewbridge.Section{{Title: "x", Summary: "y", IDs: []string{"h1", "invented"}}}},
		{Revision: "current", Sections: []reviewbridge.Section{{Title: "x", Summary: "y", IDs: []string{"h1", "h1", "h2"}}}},
	} {
		if reviewbridge.Validate(s, g) == nil {
			t.Fatalf("accepted invalid guide: %+v", g)
		}
	}
}
func TestGuideCLIError(t *testing.T) {
	// Failure leaves the complete local guide usable.
	a, tt, _ := newTestApp(t)
	tt.TypeKey(ui.Ctrl, ui.KeyG, "g")
	tt.Frame()
	defer a.closeGuide()
	t.Setenv("PATH", t.TempDir())
	press(tt, "A")
	if a.guide.generating || a.guide.err == "" || len(a.guide.sections) == 0 {
		t.Fatal("CLI error lost guide or left spinner running")
	}
}

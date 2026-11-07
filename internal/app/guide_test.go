package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/config"
	"laziergit/internal/reviewbridge"
	"laziergit/internal/testrepo"
)

func TestGuideReviewAndMCPRoundTrip(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "a_test.go", "package a\nfunc ExampleA() { A() }\n")
	testrepo.Write(t, dir, "go.sum", "example v1 hash\n")
	press(tt, "~")
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
	if !tt.HasText("Changed since reviewed") {
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
	if a.guide == nil || a.guide.active || !tt.HasText("FILES") {
		t.Fatal("escape should restore git view")
	}
}
func TestGuideBranchAndPartialStaging(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() { println(\"index\") }\n")
	git(t, dir, "add", "a.go")
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() { println(\"worktree\") }\n")
	press(tt, "~")
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
	press(tt, "~")
	tt.Frame()
	defer a.closeGuide()
	t.Setenv("PATH", t.TempDir())
	press(tt, "A")
	if a.guide.generating || a.guide.err == "" || len(a.guide.sections) == 0 {
		t.Fatal("CLI error lost guide or left spinner running")
	}
}

func TestGuideNavigationStreamsAndConfiguredKeys(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "a_test.go", "package a\nfunc ExampleA() { A() }\n")
	press(tt, "~")
	defer a.closeGuide()
	g := a.guide
	if len(g.sections) < 2 {
		t.Fatal("need multiple sections")
	}
	// The whole implementation is available together, not replaced per hunk.
	paths := map[string]bool{}
	numbered := false
	for _, r := range g.rows() {
		if r.hunk >= 0 {
			paths[g.hunks[r.hunk].Path] = true
		}
		if r.oldLine > 0 || r.newLine > 0 {
			numbered = true
		}
	}
	if !paths["a.go"] || !paths["new.txt"] || !numbered {
		t.Fatalf("incomplete stream: %+v", g.rows())
	}
	press(tt, "j")
	if g.current().Path != "new.txt" {
		t.Fatal("j must move to next hunk")
	}
	press(tt, "j")
	if g.section != 1 || g.current().Path != "a_test.go" {
		t.Fatal("j must cross sections")
	}
	press(tt, "k")
	if g.section != 0 || g.current().Path != "new.txt" {
		t.Fatal("k must cross sections")
	}
	press(tt, "l")
	if g.section != 1 || g.hunk != 0 {
		t.Fatal("l must jump sections")
	}
	press(tt, "h")
	if g.section != 0 || g.hunk != 0 {
		t.Fatal("h must land on first hunk")
	}
	press(tt, "space")
	press(tt, "]")
	if g.current().Path != "new.txt" {
		t.Fatal("next unread")
	}
	press(tt, "[")
	if g.section != 1 {
		t.Fatal("previous unread must wrap past reviewed hunks")
	}
	press(tt, "enter")
	if !g.focusCode {
		t.Fatal("enter should focus code")
	}
	before := g.lineCursor
	press(tt, "j")
	if g.lineCursor != before+1 {
		t.Fatalf("j must move by line: %d -> %d", before, g.lineCursor)
	}
	press(tt, "esc")
	if !g.active || g.focusCode {
		t.Fatal("escape should return to story")
	}
	// Respect user keybindings for the two most important motions.
	a.repo.Config.Keybinding.Universal.NextItem = config.Keybinding{"n"}
	a.repo.Config.Keybinding.Universal.PrevTab = config.Keybinding{"{"}
	a.keys = loadBindings(a.repo)
	a.actions = a.buildActions()
	press(tt, "h")
	path := g.current().Path
	press(tt, "j")
	if g.current().Path != path {
		t.Fatal("old next-item key remains active")
	}
	press(tt, "n")
	if g.current().Path == path {
		t.Fatal("remapped next-item key failed")
	}
	press(tt, "{")
	if g.section != 1 {
		t.Fatal("remapped previous-unread key failed")
	}
}
func TestGuideTogglePreservesStagingAndReview(t *testing.T) {
	a, tt, _ := newTestApp(t)
	selectFile(t, a, "a.go")
	tt.Frame()
	press(tt, "enter")
	original := a.staging
	cursor := original.cursor
	press(tt, "~")
	defer a.closeGuide()
	g := a.guide
	if !g.focusCode || g.current().Path != "a.go" {
		t.Fatal("staging selection not mapped")
	}
	press(tt, "space")
	reviewed := *g.current()
	for range 10 {
		press(tt, "~")
		if g.active || a.staging != original || a.staging.cursor != cursor {
			t.Fatal("round trip lost staging selection")
		}
		press(tt, "~")
		if a.guide != g || !g.isReviewed(reviewed) {
			t.Fatal("round trip lost guide state")
		}
	}
}
func TestGuideToggleMapsMovedSelection(t *testing.T) {
	a, tt, _ := newTestApp(t)
	selectFile(t, a, "a.go")
	tt.Frame()
	press(tt, "~")
	defer a.closeGuide()
	press(tt, "j")
	if a.guide.current().Path != "new.txt" {
		t.Fatal("test requires next file")
	}
	press(tt, "enter")
	press(tt, "~")
	if selectedPath(a) != "new.txt" || a.staging == nil || !a.staging.worktree {
		t.Fatal("guide selection not mapped to staging")
	}
	if row := a.staging.rows[a.staging.cursor]; row.path != "new.txt" || row.newLine != 1 {
		t.Fatalf("wrong line: %+v", row)
	}
	press(tt, "~")
	if !a.guide.focusCode || a.guide.current().Path != "new.txt" {
		t.Fatal("staging selection not mapped back")
	}
}
func TestGuideCtrlGStillBelongsToLazygit(t *testing.T) {
	a, tt, _ := newTestApp(t)
	a.Press("<c-g>")
	tt.Frame()
	if a.guide != nil {
		t.Fatal("ctrl-g must not open guide")
	}
}

func TestGuideToggleFromFilesReturnsStoryFocus(t *testing.T) {
	a, tt, _ := newTestApp(t)
	selectFile(t, a, "a.go")
	tt.Frame()
	press(tt, "enter")
	press(tt, "~")
	defer a.closeGuide()
	press(tt, "~")
	press(tt, "esc") // Leave staging in lazier.
	press(tt, "~")
	if a.guide.focusCode {
		t.Fatal("entering from Files should focus story")
	}
}
func TestGuideToggleFindsNewlyCreatedFile(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "fresh.go", "package fresh\nvar Value = 42\n")
	press(tt, "~")
	defer a.closeGuide()
	for a.guide.current().Path != "fresh.go" {
		press(tt, "j")
		if a.guide.current().Path == "new.txt" {
			t.Fatal("missing fresh file")
		}
	}
	press(tt, "enter")
	press(tt, "~")
	if selectedPath(a) != "fresh.go" || a.staging == nil {
		t.Fatal("new file not mapped after refreshing Files")
	}
}
func TestGuideLongLinesScrollAndWrap(t *testing.T) {
	a, tt, dir := newTestApp(t)
	a.repo.Config.Gui.WrapLinesInDiffView = false
	testrepo.Write(t, dir, "a.go", "package a\nvar Message = \""+strings.Repeat("long ", 100)+"\"\n")
	press(tt, "~")
	defer a.closeGuide()
	if a.guide.horizontal.MaxX <= 0 {
		t.Fatal("long lines need a horizontal viewport")
	}
	press(tt, "L")
	if a.guide.horizontal.X <= 0 {
		t.Fatal("configured scroll-right key failed")
	}
	press(tt, "H")
	if a.guide.horizontal.X != 0 {
		t.Fatal("configured scroll-left key failed")
	}
	a.repo.Config.Gui.WrapLinesInDiffView = true
	tt.Frame()
	if !tt.HasText("STORY") {
		t.Fatal("wrapped guide did not render")
	}
}

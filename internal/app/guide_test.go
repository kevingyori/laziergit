package app

import (
	"bytes"
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
	// Review mode must not open the files panel's ignore action.
	press(tt, "i")
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
		{Revision: "current", Sections: []reviewbridge.Section{{Title: "Stop printing from A", Summary: "y", IDs: []string{"h1"}}}},
		{Revision: "current", Sections: []reviewbridge.Section{{Title: "Stop printing from A", Summary: "y", IDs: []string{"h1", "invented"}}}},
		{Revision: "current", Sections: []reviewbridge.Section{{Title: "Stop printing from A", Summary: "y", IDs: []string{"h1", "h1", "h2"}}}},
	} {
		if reviewbridge.Validate(s, g) == nil {
			t.Fatalf("accepted invalid guide: %+v", g)
		}
	}
	ok := reviewbridge.Guide{Revision: "current", Sections: []reviewbridge.Section{{Title: "Stop printing from A", Summary: "y", IDs: []string{"h1", "h2"}}}}
	if err := reviewbridge.Validate(s, ok); err != nil {
		t.Fatal("rejected a valid v1 guide:", err)
	}
}
func TestGuideCopiesAgentPrompt(t *testing.T) {
	// A copies a request for an MCP-connected agent; the app runs nothing.
	a, tt, _ := newTestApp(t)
	var copied []string
	a.copyText = func(s string) error { copied = append(copied, s); return nil }
	press(tt, "~")
	tt.Frame()
	defer a.closeGuide()
	press(tt, "A")
	if len(copied) != 1 || !strings.Contains(copied[0], "get_review_diff") || !strings.Contains(copied[0], "publish_review_guide") {
		t.Fatalf("copied %q", copied)
	}
	if len(a.overlays) != 0 || a.guide.err != "" || !strings.Contains(a.guide.notice, "Prompt copied") || len(a.guide.sections) == 0 {
		t.Fatalf("overlays %v, err %q, notice %q", a.overlays, a.guide.err, a.guide.notice)
	}
	var snapshot reviewbridge.Snapshot
	if err := reviewbridge.Read(a.guide.session, "snapshot.json", &snapshot); err != nil || snapshot.Context.Branch == "" || len(snapshot.Context.Commits) == 0 {
		t.Fatalf("snapshot lacks author context: %+v %v", snapshot.Context, err)
	}
}

func TestGuidePromptExplainsEmptyReview(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Git(t, dir, "add", "-A")
	testrepo.Git(t, dir, "commit", "-qm", "clean")
	copied := false
	a.copyText = func(string) error { copied = true; return nil }
	press(tt, "~")
	tt.Frame()
	defer a.closeGuide()
	press(tt, "A")
	if copied || !strings.Contains(a.guide.notice, "Nothing to review") {
		t.Fatalf("copied %v, notice %q", copied, a.guide.notice)
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

func reviewCallErr(t *testing.T, dir, name string, args any) string {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	var out bytes.Buffer
	if err := reviewbridge.Serve(dir, strings.NewReader(string(data)+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// A cold reader reads pinned sources, not the live tree, and the contract
// rejects homework and unsupported claims.
func TestGuideSourceToolsAndNoteContract(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "b.go", "package a\n\nfunc B() { A() }\n")
	testrepo.Git(t, dir, "add", "b.go")
	testrepo.Git(t, dir, "commit", "-qm", "add caller B", "-m", "B needs A to return early.")
	press(tt, "~")
	defer a.closeGuide()
	g := a.guide
	for _, c := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"path": "a.go", "side": "old"}, "func A() {}"},
		{map[string]any{"path": "a.go"}, "func A() { return }"},
		{map[string]any{"path": "new.txt"}, "1\\thello"},
		{map[string]any{"path": "b.go", "start": 3, "end": 3}, "3\\tfunc B() { A() }"},
	} {
		if out := reviewCall(t, g.session, "read_file", c.args); !strings.Contains(out, c.want) {
			t.Fatalf("read_file %v: %s", c.args, out)
		}
	}
	if out := reviewCallErr(t, g.session, "read_file", map[string]any{"path": "../secret"}); !strings.Contains(out, `"isError":true`) {
		t.Fatal("path escape accepted")
	}
	if out := reviewCall(t, g.session, "list_files", map[string]any{}); !strings.Contains(out, "b.go") || !strings.Contains(out, "new.txt") {
		t.Fatalf("list_files: %s", out)
	}
	if out := reviewCall(t, g.session, "list_files", map[string]any{"path_glob": "*.txt"}); strings.Contains(out, "a.go") || !strings.Contains(out, "new.txt") {
		t.Fatalf("list_files glob: %s", out)
	}
	if out := reviewCall(t, g.session, "search", map[string]any{"pattern": "A\\(\\)"}); !strings.Contains(out, "b.go:3:") {
		t.Fatalf("search: %s", out)
	}
	if out := reviewCall(t, g.session, "get_review_diff", map[string]any{}); !strings.Contains(out, "B needs A to return early.") || strings.Contains(out, dir) {
		t.Fatalf("context missing or worktree path leaked: %s", out)
	}
	testrepo.Write(t, dir, "new.txt", "changed\n")
	if out := reviewCallErr(t, g.session, "read_file", map[string]any{"path": "new.txt"}); !strings.Contains(out, "changed after the snapshot") {
		t.Fatalf("live file served as snapshot: %s", out)
	}
	ids := []string{}
	for _, h := range g.hunks {
		ids = append(ids, h.ID)
	}
	section := func(n reviewbridge.Note) reviewbridge.Guide {
		return reviewbridge.Guide{Revision: g.revision, Provenance: "inferred", Overview: &reviewbridge.Overview{Purpose: "A returns early so B never prints."}, Sections: []reviewbridge.Section{{Title: "Stop printing from A", Kind: "core", Why: "Callers like B must not see output.", How: "A returns instead of calling println.", IDs: ids, Notes: []reviewbridge.Note{n}}}}
	}
	for _, bad := range []reviewbridge.Note{
		{HunkID: ids[0], Kind: "explain", Text: "Check that B still compiles."},
		{HunkID: ids[0], Kind: "assumption", Text: "B is the only caller."},
		{HunkID: ids[0], Kind: "explain", Line: 999, Text: "Out of range."},
	} {
		if reviewbridge.Validate(snapshotOf(t, g), section(bad)) == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	good := section(reviewbridge.Note{HunkID: ids[0], Kind: "assumption", Text: "B is the only caller of A.", Evidence: "search A\\(\\): b.go:3"})
	good.Sections[0].How = "A <b>returns</b> instead of calling <code>println</code>:<ul><li>B stays quiet</li><li>tests see no output</li></ul>"
	good.Sections[0].Media = []reviewbridge.Media{{SVG: `<svg xmlns="http://www.w3.org/2000/svg" width="220" height="60"><rect x="1" y="1" width="90" height="40" rx="6" fill="none" stroke="#4465e9"/><text x="14" y="26" font-size="13">B()</text><path d="M92 21 H150" stroke="#888"/><rect x="150" y="1" width="60" height="40" rx="6" fill="none" stroke="#4465e9"/><text x="168" y="26" font-size="13">A()</text></svg>`, Caption: "B calls A, which now returns early"}}
	bad := good
	bad.Sections = []reviewbridge.Section{good.Sections[0]}
	bad.Sections[0].Media = []reviewbridge.Media{{Path: "a.go"}}
	if reviewbridge.Validate(snapshotOf(t, g), bad) == nil {
		t.Fatal("non-image media accepted")
	}
	reviewCall(t, g.session, "publish_review_guide", good)
	a.Refresh()
	tt.Frame()
	if g.provenance != "inferred" || g.overview == nil || g.sections[0].Why == "" || len(g.sections[0].Notes) != 1 {
		t.Fatalf("v2 guide not imported: %+v", g.sections)
	}
	tt.Frame()
	tt.Frame()
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/review-notes.png")
	}
}
func snapshotOf(t *testing.T, g *guideState) reviewbridge.Snapshot {
	var s reviewbridge.Snapshot
	if err := reviewbridge.Read(g.session, "snapshot.json", &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRichExplanationsParseTheHTMLSubset(t *testing.T) {
	p := palette(false)
	blocks, links := parseRich(`Keep <b>marks</b> stable.<p>See <a href="https://example.com/x">docs</a>:</p><ol><li>one</li><li>two <code>f()</code></li></ol><script>alert(1)</script>`, p, 13, p.Text)
	if len(blocks) != 5 || blocks[0].spans[1].Weight != 650 || blocks[2].bullet != "1." || blocks[3].bullet != "2." || blocks[3].spans[1].Font != monoFont || len(links) != 1 {
		t.Fatalf("blocks: %+v links %v", blocks, links)
	}
	if plain, _ := parseRich("just text", p, 13, p.Text); len(plain) != 1 || plain[0].spans[0].Text != "just text" {
		t.Fatalf("plain text: %+v", plain)
	}
}

func TestGuideTabRangeSearchCopy(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() int {\n\tone := 1\n\ttwo := 2\n\treturn one + two\n}\n")
	var copied []string
	a.copyText = func(s string) error { copied = append(copied, s); return nil }
	press(tt, "~")
	tt.Frame()
	defer a.closeGuide()
	g := a.guide
	// Tab: story → code → story (no diagram).
	press(tt, "tab")
	if !g.focusCode {
		t.Fatalf("tab did not focus code: loading %v hunks %d scope %v", g.loading, len(g.hunks), a.scope())
	}
	press(tt, "tab")
	if g.focusCode {
		t.Fatal("tab did not return to the story")
	}
	// Search lands on the line; n wraps to it again.
	press(tt, "/")
	tt.Type("two := 2")
	press(tt, "enter")
	rows := g.rows()
	if !g.focusCode || !strings.Contains(rows[g.lineCursor].line.text, "two := 2") {
		t.Fatalf("search landed on %q", rows[g.lineCursor].line.text)
	}
	at := g.lineCursor
	press(tt, "n")
	if g.lineCursor != at || !strings.Contains(g.notice, "1 of 1") {
		t.Fatalf("n: cursor %d→%d, notice %q", at, g.lineCursor, g.notice)
	}
	// v selects two lines; y copies their location; c comments on the range.
	press(tt, "k")
	press(tt, "v")
	press(tt, "j")
	press(tt, "y")
	chooseMenu(t, a, tt, "Location")
	if len(copied) != 1 || !strings.HasSuffix(copied[0], "a.go:4-5") {
		t.Fatalf("copied %q", copied)
	}
	press(tt, "c")
	tt.Type("these two")
	press(tt, "enter")
	a.loadGuideFeedback()
	var an *reviewbridge.Anchor
	for _, item := range g.feedback.Items {
		if item.Body == "these two" {
			an = item.Anchor
		}
	}
	if an == nil || an.StartLine != 4 || an.Line != 5 || g.rangeStart != -1 {
		t.Fatalf("range comment anchor %+v, range %d", an, g.rangeStart)
	}
}

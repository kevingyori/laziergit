package reviewbridge

import (
	"encoding/json"
	"strings"
	"testing"
)

func contractSnapshot() Snapshot {
	return Snapshot{Revision: "current-revision", Hunks: []Hunk{
		{ID: "h1", Path: "a.go", Diff: "@@ -1,2 +1,3 @@\n package a\n-func A() {}\n+func A() {\n+}\n"},
		{ID: "h2", Path: "b.go", Diff: "@@ -10,1 +10,1 @@\n-old\n+new\n"},
	}}
}

// An agent acts on the error text alone, so the text is the contract: each
// problem names where it is and what to do.
func TestRejectionsSayWhereAndHowToFix(t *testing.T) {
	s := contractSnapshot()
	for _, c := range []struct {
		name string
		args string
		want []string
	}{
		{"fields nested in overview", `{"overview":{"purpose":"p","revision":"current-revision","sections":[]}}`,
			[]string{`overview.revision is in the wrong place: "revision" belongs at the top level`, `overview.sections is in the wrong place`}},
		{"missing revision", `{"sections":[{"title":"Do a thing","why":"w","hunk_ids":["h1","h2"]}]}`,
			[]string{`revision is missing`, `copied from get_review_diff`}},
		{"stale revision", `{"revision":"older","sections":[{"title":"Do a thing","why":"w","hunk_ids":["h1","h2"]}]}`,
			[]string{`revision older is not the current snapshot (current-revi…)`, `Call get_review_diff again`}},
		{"everything at once", `{"revision":"current-revision","provenance":"me","sections":[{"title":"Limiter","kind":"big","why":"w","hunk_ids":["h1","h9"],"notes":[{"hunk_id":"h1","line":40,"kind":"question","text":"Verify it works"},{"hunk_id":"h2","kind":"explain","text":"x"}]}]}`,
			[]string{`Guide not published: 9 problems`, `provenance "me": use "author"`, `.title is an identifier`, `.kind "big": use "core", "supporting" or "mechanical"`,
				`"h9" is not a hunk of this review. The ids are: h1, h2.`, `hands work to the reviewer`, `is a question without evidence`,
				`links line 40, which is not a new-side line of h1: it shows new-side lines 1-3`, `.hunk_id "h2" is not in this section's hunk_ids (h1, h9)`,
				`These hunks are in no section: h2 (b.go)`}},
		{"title longer than promised", `{"revision":"current-revision","sections":[{"title":"This title is deliberately far too long to pass the stated word limit","why":"w","hunk_ids":["h1","h2"]}]}`,
			[]string{`.title is 13 words: name the outcome in at most 10 words`}},
		{"hunk in two sections", `{"revision":"current-revision","sections":[{"title":"Do one thing","why":"w","hunk_ids":["h1","h2"]},{"title":"Do another thing","why":"w","hunk_ids":["h2"]}]}`,
			[]string{`"h2" is in sections[1] and sections[2]`}},
		{"wrong type", `{"revision":"current-revision","sections":[{"title":"Do one thing","why":"w","hunk_ids":"h1"}]}`,
			[]string{`hunk_ids must be an array, not string`}},
		{"unknown field", `{"revision":"current-revision","verdict":"ship it","sections":[]}`,
			[]string{`verdict is not a field here. This level takes: revision, provenance, overview, sections.`}},
	} {
		g, problems := ParseGuide(json.RawMessage(c.args))
		if len(problems) == 0 {
			problems = Problems(s, g)
		}
		text := rejection(problems)
		for _, want := range c.want {
			if !strings.Contains(text, want) {
				t.Errorf("%s: missing %q in:\n%s", c.name, want, text)
			}
		}
	}
	ok := `{"revision":"current-revision","provenance":"inferred","overview":{"purpose":"p"},"sections":[{"title":"Do one thing","kind":"core","why":"w","hunk_ids":["h1","h2"],"notes":[{"hunk_id":"h1","line":2,"end_line":3,"kind":"assumption","text":"A has one caller.","evidence":"b.go:3"}]}]}`
	g, problems := ParseGuide(json.RawMessage(ok))
	if problems = append(problems, Problems(s, g)...); len(problems) > 0 {
		t.Fatalf("rejected a valid guide: %v", problems)
	}
	if msg := published(s, g); !strings.Contains(msg, "1 sections covering all 2 hunks, 1 notes (1 on a line, 1 assumptions or questions)") || !strings.Contains(msg, "get_review_feedback") {
		t.Fatal(msg)
	}
}

// Notes take new-side line numbers; agents miscount them from @@ headers, so
// read_hunks prints them.
func TestReadHunksNumbersNewSideLines(t *testing.T) {
	text, err := ReadHunks(contractSnapshot(), []string{"h1", "h2"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"      @@ -1,2 +1,3 @@\n    1  package a\n      -func A() {}\n    2 +func A() {\n    3 +}\n", "   10 +new\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if _, err := ReadHunks(contractSnapshot(), []string{"h7"}); err == nil || !strings.Contains(err.Error(), "The ids are: h1, h2.") {
		t.Fatalf("unknown hunk: %v", err)
	}
}

// An uncommitted change has no commit message; earlier commits must not read
// as its description (an agent in the eval quoted one as the change's title).
func TestReviewIndexSeparatesHistoryFromTheChange(t *testing.T) {
	s := contractSnapshot()
	s.Context = Context{Scope: "Working tree", Commits: []string{"Earlier work"}}
	s.WorkingTree = true
	b, _ := json.Marshal(ReviewIndex(s))
	if !strings.Contains(string(b), `"earlier_commits_not_part_of_this_change":["Earlier work"]`) || strings.Contains(string(b), "commits_in_this_change") {
		t.Fatal(string(b))
	}
	s.WorkingTree = false
	b, _ = json.Marshal(ReviewIndex(s))
	if !strings.Contains(string(b), `"commits_in_this_change":["Earlier work"]`) || !strings.Contains(string(b), `"guide_template":{"revision":"current-revision"`) {
		t.Fatal(string(b))
	}
}

// A server started before the guide was opened answers with what to do, and
// works once the review exists: no reconnect.
func TestServerWithoutAReviewSaysHowToOpenOne(t *testing.T) {
	dir := ""
	resolve := func() (string, error) {
		if dir == "" {
			return "", noReview("/repo")
		}
		return dir, nil
	}
	ask := func() string {
		var out strings.Builder
		in := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"eval"}}}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" + `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_review_diff"}}` + "\n"
		if err := ServeReview(resolve, strings.NewReader(in), &out); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	out := ask()
	if !strings.Contains(out, `"isError":true`) || !strings.Contains(out, "press ~") || !strings.Contains(out, `"instructions":"laziestgit shows`) {
		t.Fatal(out)
	}
	// Property order is the order agents write in: revision before the nested objects.
	if i, j := strings.Index(out, `"revision":{"type":"string","description":"The revision string from get_review_diff, unchanged"}`), strings.Index(out, `"overview":{"type":"object"`); i < 0 || j < i {
		t.Fatal("publish schema must list revision before overview")
	}
	dir = t.TempDir()
	if err := Write(dir, "snapshot.json", contractSnapshot()); err != nil {
		t.Fatal(err)
	}
	if out := ask(); strings.Contains(out, `"isError":true`) || !strings.Contains(out, "current-revision") {
		t.Fatal(out)
	}
}

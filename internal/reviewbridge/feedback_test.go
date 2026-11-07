package reviewbridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testRound(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	s := Snapshot{Revision: strings.Repeat("a", 64)}
	if err := Write(dir, "snapshot.json", s); err != nil {
		t.Fatal(err)
	}
	if err := SaveRound(dir, s); err != nil {
		t.Fatal(err)
	}
	return dir
}
func TestFeedbackJournalConcurrentWritersAndRecovery(t *testing.T) {
	dir := testRound(t)
	var wg sync.WaitGroup
	fail := make(chan error, 24)
	for i := range 24 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			fail <- SaveDraft(dir, Feedback{Revision: strings.Repeat("a", 64), Kind: "line", Body: fmt.Sprint("comment ", i)})
		}(i)
	}
	wg.Wait()
	close(fail)
	for err := range fail {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err := LoadFeedback(dir)
	if err != nil || len(state.Items) != 24 || state.Cursor != 25 {
		t.Fatalf("lost writes: %+v %v", state, err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"torn":`)
	f.Close()
	if err := SendFeedback(dir); err != nil {
		t.Fatal(err)
	}
	state, err = AgentFeedback(dir, 0)
	if err != nil || len(state.Items) != 24 || state.Cursor != 26 {
		t.Fatalf("torn tail not repaired: %+v %v", state, err)
	}
	if err := ReplyFeedback(dir, state.Items[0].ID, Reply{Kind: "resolved", Body: "not allowed"}); err == nil {
		t.Fatal("agent resolved feedback")
	}
}
func TestDiagramValidationRejectsInventedAnchors(t *testing.T) {
	d := Diagram{Title: "Flow", Nodes: []DiagramNode{{ID: "a", Label: "Code", HunkID: "invented"}}}
	if ValidateDiagram(d, map[string]bool{"real": true}) == nil {
		t.Fatal("invented anchor accepted")
	}
	d.Nodes[0].Context = true
	if err := ValidateDiagram(d, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	d.Edges = []DiagramEdge{{From: "a", To: "missing"}}
	if ValidateDiagram(d, map[string]bool{}) == nil {
		t.Fatal("dangling edge accepted")
	}
}

func TestFeedbackAnchorsMoveOnlyWhenUnambiguous(t *testing.T) {
	item := Feedback{Revision: "old", Anchor: &Anchor{Path: "a.go", Layer: "Unstaged", Side: "old", Line: 3, Hunk: "stable", Text: "old()"}}
	h := Hunk{Path: "a.go", Layer: "Unstaged", Fingerprint: "stable", Diff: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -11 +11 @@\n-old()\n+new()\n"}
	snapshot := Snapshot{Revision: "new", Hunks: []Hunk{h}}
	placement, anchor := PlaceFeedback(item, snapshot)
	if placement != "moved" || anchor.Line != 11 || item.Anchor.Line != 3 {
		t.Fatal("move lost original anchor")
	}
	snapshot.Hunks = append(snapshot.Hunks, h)
	if placement, _ := PlaceFeedback(item, snapshot); placement != "outdated" {
		t.Fatal("ambiguous quote must not be guessed")
	}
	snapshot.Hunks = nil
	if placement, _ := PlaceFeedback(item, snapshot); placement != "outdated" {
		t.Fatal("deleted quote must stay outdated")
	}
	if item.Resolved {
		t.Fatal("outdated cannot imply resolved")
	}
}

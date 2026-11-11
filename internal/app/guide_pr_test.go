package app

import (
	"encoding/json"
	"strings"
	"testing"

	"laziergit/internal/reviewbridge"
	"laziergit/internal/testrepo"
)

func TestGuidePublishesPendingPRReview(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Git(t, dir, "checkout", "-q", "-b", "topic")
	testrepo.Git(t, dir, "commit", "-qam", "return early")
	a, tt := openApp(t, dir)
	press(tt, "~")
	defer a.closeGuide()
	g := a.guide
	g.base = "main"
	a.refreshGuide()
	if len(g.hunks) == 0 || g.hunks[0].Layer != "Branch" {
		t.Fatalf("branch review not loaded: %+v", g.hunks)
	}
	press(tt, "enter")
	for i, r := range g.rows() {
		if r.line.kind == lineAdd {
			a.guideSelectStream(i)
			break
		}
	}
	press(tt, "c")
	tt.Type("Return an error instead?")
	press(tt, "enter")
	if err := reviewbridge.SaveDraft(g.session, reviewbridge.Feedback{Kind: "general", Revision: g.revision, Body: "Overall this reads well."}); err != nil {
		t.Fatal(err)
	}
	if err := reviewbridge.SaveDraft(g.session, reviewbridge.Feedback{Kind: "diagram_proposal", Revision: g.revision, Body: "x", Diagram: &reviewbridge.DiagramProposal{Section: "s"}}); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))
	var calls [][]string
	var posted []byte
	prHead := "0000000000000000000000000000000000000000"
	a.runGH = func(_ string, stdin []byte, args ...string) ([]byte, error) {
		calls = append(calls, args)
		if args[0] == "pr" {
			return json.Marshal(map[string]any{"number": 7, "url": "https://github.com/o/r/pull/7", "headRefOid": prHead, "baseRefName": "main"})
		}
		posted = stdin
		return []byte(`{"id":1,"html_url":"https://github.com/o/r/pull/7#pullrequestreview-1"}`), nil
	}
	var opened string
	a.openLink = func(u string) error { opened = u; return nil }
	press(tt, "esc")
	press(tt, "G")
	if !strings.Contains(g.err, "Push first") || posted != nil {
		t.Fatalf("unpushed HEAD must not publish: %q", g.err)
	}
	prHead, g.err = head, ""
	press(tt, "G")
	if a.topOverlay() == nil {
		t.Fatal("publishing must be confirmed")
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/review-pr-confirm.png")
	}
	press(tt, "enter")
	var review struct {
		CommitID string            `json:"commit_id"`
		Body     string            `json:"body"`
		Event    string            `json:"event"`
		Comments []ghReviewComment `json:"comments"`
	}
	if err := json.Unmarshal(posted, &review); err != nil {
		t.Fatal(err)
	}
	if review.Event != "" || review.CommitID != head || review.Body != "Overall this reads well." || len(review.Comments) != 1 || review.Comments[0].Side != "RIGHT" || review.Comments[0].Path != "a.go" || review.Comments[0].Line == 0 {
		t.Fatalf("review payload: %s", posted)
	}
	if last := calls[len(calls)-1]; last[0] != "api" || !strings.Contains(strings.Join(last, " "), "pulls/7/reviews") {
		t.Fatalf("gh call: %v", last)
	}
	press(tt, "enter")
	if opened == "" {
		t.Fatal("did not offer to open the pending review")
	}
	published := 0
	for _, item := range g.feedback.Items {
		if item.Published != "" {
			published++
		} else if item.Diagram == nil {
			t.Fatalf("not marked published: %+v", item)
		}
	}
	if published != 2 {
		t.Fatalf("published %d items", published)
	}
	posted = nil
	press(tt, "G")
	if posted != nil || !strings.Contains(g.notice, "Nothing to publish") {
		t.Fatal("published comments went out twice")
	}
}

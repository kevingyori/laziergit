// sendfeedback plays the reviewer for guide-eval: it sends one line comment
// and one general note through the same journal the app's c and S keys use.
// Usage: go run ./scripts/guide-eval/sendfeedback <worktree>
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/patch"

	"laziergit/internal/reviewbridge"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: sendfeedback <worktree>")
	}
	dir, err := reviewbridge.ResolveReview(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	var s reviewbridge.Snapshot
	if err := reviewbridge.Read(dir, "snapshot.json", &s); err != nil {
		log.Fatal(err)
	}
	const needle = `r.Header.Get("X-Forwarded-For")`
	var anchor *reviewbridge.Anchor
	for _, h := range s.Hunks {
		p := patch.Parse(h.Diff)
		nums, _ := p.LineNumbersOfLines()
		for i, line := range p.Lines() {
			if line.Kind == patch.ADDITION && strings.Contains(line.Content, needle) {
				anchor = &reviewbridge.Anchor{Path: h.Path, Layer: h.Layer, Side: "new", Line: nums[i], Hunk: h.Fingerprint, Text: line.Content[1:]}
			}
		}
	}
	if anchor == nil {
		log.Fatalf("fixture line %q not found in the snapshot", needle)
	}
	drafts := []reviewbridge.Feedback{
		{Kind: "line", Revision: s.Revision, Anchor: anchor, Body: "Anyone can send this header. Doesn't that let a client dodge the limit by rotating it? We don't run behind a proxy in dev."},
		{Kind: "general", Revision: s.Revision, Body: "Does the per-client map ever shrink? I asked for per-client limits, not a memory leak."},
	}
	for _, d := range drafts {
		if err := reviewbridge.SaveDraft(dir, d); err != nil {
			log.Fatal(err)
		}
	}
	if err := reviewbridge.SendFeedback(dir); err != nil {
		log.Fatal(err)
	}
	fmt.Println("sent 2 feedback items")
}

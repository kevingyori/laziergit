package app

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/reviewbridge"
)

func (g *guideState) exportSnapshot() error {
	dir := reviewbridge.ScopeDir(g.reviewRoot, g.base)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	g.session = dir
	if err := os.MkdirAll(g.reviewRoot, 0700); err != nil {
		return err
	}
	if err := reviewbridge.Write(g.reviewRoot, "active.json", map[string]string{"scope": g.base}); err != nil {
		return err
	}
	snapshot := reviewbridge.Snapshot{Worktree: g.worktree, OldTree: g.context.OldTree, NewTree: g.context.NewTree, WorkingTree: g.base == ""}
	snapshot.Context = reviewbridge.Context{Scope: g.context.Scope, Branch: g.context.Branch, Commits: g.context.Commits}
	snapshot.Hunks = []reviewbridge.Hunk{}
	for _, h := range g.hunks {
		snapshot.Hunks = append(snapshot.Hunks, reviewbridge.Hunk{ID: h.ID, Fingerprint: h.Fingerprint, Path: h.Path, Layer: h.Layer, Diff: h.Text, OldBlob: h.OldHash, NewBlob: h.NewHash})
	}
	snapshot.Revision = guideRevision(g.hunks)
	if err := reviewbridge.Write(g.session, "snapshot.json", snapshot); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cfg := map[string]any{"mcpServers": map[string]any{"laziestgit": map[string]any{"command": executable, "args": []string{"--guide-mcp", g.session}}}}
	if err := reviewbridge.Write(g.session, "mcp.json", cfg); err != nil {
		return err
	}
	if err := reviewbridge.SaveRound(g.session, snapshot); err != nil {
		return err
	}
	g.revision = snapshot.Revision
	return nil
}

// guideRevision identifies a set of hunks; a guide is only shown over the
// exact hunks it was written for.
func guideRevision(hunks []guideHunk) string {
	out := []reviewbridge.Hunk{}
	for _, h := range hunks {
		out = append(out, reviewbridge.Hunk{ID: h.ID, Fingerprint: h.Fingerprint, Path: h.Path, Layer: h.Layer, Diff: h.Text, OldBlob: h.OldHash, NewBlob: h.NewHash})
	}
	b, _ := json.Marshal(out)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func readAgentGuide(dir string) (reviewbridge.Guide, error) {
	var snapshot reviewbridge.Snapshot
	var guide reviewbridge.Guide
	if err := reviewbridge.Read(dir, "snapshot.json", &snapshot); err != nil {
		return guide, err
	}
	if err := reviewbridge.Read(dir, "guide.json", &guide); err != nil {
		return guide, err
	}
	return guide, reviewbridge.Validate(snapshot, guide)
}
func (g *guideState) importGuide(guide reviewbridge.Guide) {
	if guide.Revision != g.revision {
		return
	}
	byID := map[string]int{}
	for i, h := range g.hunks {
		byID[h.ID] = i
	}
	var sections []guideSection
	for _, s := range guide.Sections {
		section := guideSection{Title: s.Title, Summary: s.Summary, Kind: s.Kind, Why: s.Why, How: s.How, Notes: s.Notes, Media: s.Media, Diagram: s.Diagram}
		for _, id := range s.IDs {
			section.Hunks = append(section.Hunks, byID[id])
		}
		sections = append(sections, section)
	}
	g.sections = sections
	g.section, g.hunk = 0, 0
	g.provenance, g.overview = guide.Provenance, guide.Overview
	g.source = "AI guide · verify explanations against the diff"
	g.agent, g.staleGuide = guide.Agent, false
	g.list, g.diffList = ui.ListState{}, ui.ListState{}
	g.imported = guide.Revision
	g.rebuildStreams()
	g.drawing = nil
	g.drawingKey = ""
}
func (a *App) pollGuide() {
	g := a.guide
	a.loadGuideFeedback()
	if g == nil || g.session == "" || g.loading {
		return
	}
	a.noteAgentReplies()
	a.checkGuideMoved()
	stat, err := os.Stat(g.session + "/guide.json")
	if err != nil || (g.imported == g.revision && stat.ModTime().UnixNano() == g.guideStamp) {
		return
	}
	arrived := g.guideStamp != 0 || g.imported == ""
	if guide, err := readAgentGuide(g.session); err == nil {
		first := g.imported != guide.Revision
		g.importGuide(guide)
		g.guideStamp = stat.ModTime().UnixNano()
		if arrived && first && g.active && g.notice == guideWaiting {
			g.notice = "The agent's guide arrived"
		}
		return
	}
	// A guide written for other code is not shown; say so instead of
	// silently falling back to the grouping by file type.
	var guide reviewbridge.Guide
	if reviewbridge.Read(g.session, "guide.json", &guide) == nil && guide.Revision != g.revision {
		g.staleGuide = true
	}
}

// guideWaiting is the notice after A; the guide's arrival replaces it.
const guideWaiting = "Prompt copied · paste it into your agent; the guide appears here when it publishes"

// noteAgentReplies announces answers that arrived since the last look.
func (a *App) noteAgentReplies() {
	g := a.guide
	replies := 0
	for _, item := range g.feedback.Items {
		for _, r := range item.Replies {
			if r.Kind != reviewbridge.ReviewerReply {
				replies++
			}
		}
	}
	if replies > g.seenReplies && g.seenReplies >= 0 && g.feedbackLoaded {
		n := replies - g.seenReplies
		g.notice = fmt.Sprintf("The agent answered %s · f opens the feedback", plural(n, "comment"))
	}
	g.seenReplies, g.feedbackLoaded = replies, true
}

// checkGuideMoved notices, off the main thread, that the code no longer
// matches the snapshot under review: the agent (or anyone) edited it since.
// The snapshot stays fixed until r; the reviewer is only told.
func (a *App) checkGuideMoved() {
	g := a.guide
	if !g.active || g.checking || g.moved {
		return
	}
	g.checking = true
	generation, r, base := g.generation, a.repo, g.base
	a.bg(func() {
		text, err := r.GuideDiff(base)
		revision := guideRevision(guideHunks(text))
		a.post(func() {
			g.checking = false
			if a.guide == g && a.repo == r && g.generation == generation && !g.loading && err == nil {
				g.moved = revision != g.revision
			}
		})
	})
}
func (a *App) copyGuideMCP() {
	g := a.guide
	if g.session == "" {
		return
	}
	executable, err := os.Executable()
	cfg := map[string]any{"mcpServers": map[string]any{"laziestgit": map[string]any{"command": executable, "args": []string{"--review-mcp", g.worktree}}}}
	b, marshalErr := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		err = marshalErr
	}
	if err == nil {
		err = a.copyText(string(b))
	}
	if err != nil {
		g.err = err.Error()
	} else {
		g.err = ""
		g.notice = "MCP config copied · add it to your coding agent once, then A copies the prompt"
	}
}

// copyGuidePrompt copies a request for a coding agent connected with M's
// config. The agent that wrote the change publishes as author; the guide
// arrives on auto-refresh like any other MCP answer.
func (a *App) copyGuidePrompt() {
	g := a.guide
	switch {
	case g.loading || g.session == "":
		g.notice = "The diff is still loading; press A again once it shows."
		return
	case len(g.hunks) == 0:
		g.notice = "Nothing to review, so no prompt was copied. b chooses a branch base."
		return
	}
	if err := a.copyText(reviewbridge.Prompt); err != nil {
		g.err = err.Error()
		return
	}
	g.err = ""
	g.notice = guideWaiting
}

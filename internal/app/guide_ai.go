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
	for _, h := range g.hunks {
		snapshot.Hunks = append(snapshot.Hunks, reviewbridge.Hunk{ID: h.ID, Fingerprint: h.Fingerprint, Path: h.Path, Layer: h.Layer, Diff: h.Text, OldBlob: h.OldHash, NewBlob: h.NewHash})
	}
	b, _ := json.Marshal(snapshot.Hunks)
	snapshot.Revision = fmt.Sprintf("%x", sha256.Sum256(b))
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
	stat, err := os.Stat(g.session + "/guide.json")
	if err != nil || (g.imported == g.revision && stat.ModTime().UnixNano() == g.guideStamp) {
		return
	}
	if guide, err := readAgentGuide(g.session); err == nil {
		g.importGuide(guide)
		g.guideStamp = stat.ModTime().UnixNano()
	}
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
		g.notice = "MCP config copied. Connect a coding agent; ask it to read the diff and publish a review guide. Press r to import."
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
	g.notice = "Prompt copied. Paste it into a coding agent connected with M's config; the guide appears when it publishes."
}

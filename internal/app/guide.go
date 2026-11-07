package app

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/egoist/mygo/ui"
)

type guideHunk struct {
	ID, Path, Layer, Text, Fingerprint string
}
type guideSection struct {
	Title, Summary string
	Hunks          []int
}
type guideState struct {
	session, revision, imported string
	base                        string
	hunks                       []guideHunk
	sections                    []guideSection
	section, hunk               int
	list, diffList              ui.ListState
	reviewed                    map[string]string
	loading, generating         bool
	generation                  int
	err, source                 string
}

// OpenGuide opens the experimental laziestgit review surface.
func (a *App) OpenGuide() {
	if a.guide != nil {
		return
	}
	a.guide = &guideState{reviewed: map[string]string{}, source: "Local guide"}
	a.refreshGuide()
}
func (a *App) closeGuide() {
	if a.guide != nil {
		os.RemoveAll(a.guide.session)
	}
	a.guide = nil
}
func (a *App) refreshGuide() {
	g := a.guide
	if g == nil || g.loading {
		return
	}
	g.loading = true
	g.generation++
	generation, r, base := g.generation, a.repo, g.base
	a.bg(func() {
		text, err := r.GuideDiff(base)
		hunks := guideHunks(text)
		a.post(func() {
			if a.guide != g || a.repo != r || g.generation != generation {
				return
			}
			g.loading = false
			if err != nil {
				g.err = err.Error()
				return
			}
			g.err = ""
			g.hunks = hunks
			g.sections = classifyGuide(hunks)
			g.section, g.hunk = 0, 0
			g.source = "Local guide"
			g.imported = ""
			if err := g.exportSnapshot(); err != nil {
				g.err = err.Error()
			}
			a.pollGuide()
			g.list, g.diffList = ui.ListState{}, ui.ListState{}
		})
	})
}
func guideHunks(text string) []guideHunk {
	var out []guideHunk
	layer := "Branch"
	// Split by file before splitting hunks, preserving rename and binary metadata.
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); {
		if strings.HasPrefix(lines[i], "# guide:") {
			layer = map[string]string{"# guide:index": "Staged", "# guide:worktree": "Unstaged"}[lines[i]]
			i++
			continue
		}
		if !strings.HasPrefix(lines[i], diffFilePrefix) {
			i++
			continue
		}
		end := i + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], diffFilePrefix) && !strings.HasPrefix(lines[end], "# guide:") {
			end++
		}
		name := pathFromDiffHeader(lines[i:end])
		first := i + 1
		for first < end && !strings.HasPrefix(lines[first], "@@ ") {
			first++
		}
		header := strings.Join(lines[i:first], "\n")
		if first == end {
			first = i
		}
		for first < end {
			next := first + 1
			for next < end && !strings.HasPrefix(lines[next], "@@ ") {
				next++
			}
			body := strings.TrimRight(strings.Join(lines[first:next], "\n"), "\n")
			shown := body
			if first != i {
				shown = header + "\n" + body
			}
			// Exclude line offsets from review identity; context and code still count.
			identity := body
			if strings.HasPrefix(body, "@@ ") {
				_, identity, _ = strings.Cut(body, "\n")
			}
			sum := sha256.Sum256([]byte(name + "\n" + layer + "\n" + identity))
			out = append(out, guideHunk{ID: fmt.Sprintf("h%d", len(out)+1), Path: name, Layer: layer, Text: shown, Fingerprint: fmt.Sprintf("%x", sum)})
			first = next
		}
		i = end
	}
	return out
}
func classifyGuide(hunks []guideHunk) []guideSection {
	sections := []guideSection{
		{Title: "Implementation", Summary: "Start with the implementation. Check the behavior and how the changed pieces fit together."},
		{Title: "Tests", Summary: "Check that the tests exercise the changed behavior, including failure paths."},
		{Title: "Documentation & configuration", Summary: "Review the public explanation and configuration alongside the implementation."},
		{Title: "Generated & dependencies", Summary: "Dependency and generated-file changes are grouped here. Verify their source and impact before marking reviewed."},
	}
	for i, h := range hunks {
		lower := strings.ToLower(h.Path)
		name := path.Base(lower)
		bucket := 0
		switch {
		case strings.HasSuffix(name, ".lock") || name == "go.sum" || name == "package-lock.json" || strings.HasSuffix(name, ".generated.go") || strings.HasSuffix(name, ".pb.go"):
			bucket = 3
		case strings.Contains(lower, "test") || strings.Contains(lower, "spec."):
			bucket = 1
		case strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".toml") || name == "go.mod":
			bucket = 2
		}
		sections[bucket].Hunks = append(sections[bucket].Hunks, i)
	}
	var out []guideSection
	for _, s := range sections {
		if len(s.Hunks) > 0 {
			out = append(out, s)
		}
	}
	return out
}
func (g *guideState) current() *guideHunk {
	if len(g.sections) == 0 {
		return nil
	}
	s := g.sections[g.section]
	if len(s.Hunks) == 0 {
		return nil
	}
	return &g.hunks[s.Hunks[g.hunk]]
}
func (g *guideState) slot(h guideHunk) string {
	// Stable within the current file/layer; content fingerprints invalidate edits.
	ordinal := 0
	for _, other := range g.hunks {
		if other.ID == h.ID {
			break
		}
		if other.Path == h.Path && other.Layer == h.Layer {
			ordinal++
		}
	}
	return fmt.Sprintf("%s:%s:%s:%d", g.base, h.Layer, h.Path, ordinal)
}
func (g *guideState) isReviewed(h guideHunk) bool { return g.reviewed[g.slot(h)] == h.Fingerprint }
func (a *App) guideReview() {
	g := a.guide
	if h := g.current(); h != nil {
		key := g.slot(*h)
		if g.isReviewed(*h) {
			delete(g.reviewed, key)
		} else {
			g.reviewed[key] = h.Fingerprint
		}
	}
}
func (a *App) guideMove(d int) {
	g := a.guide
	if len(g.sections) == 0 {
		return
	}
	g.hunk = clamp(g.hunk+d, 0, len(g.sections[g.section].Hunks)-1)
	g.list.ScrollTo(g.hunk, ui.Center)
	g.diffList = ui.ListState{}
}
func (a *App) guideSection(d int) {
	g := a.guide
	if len(g.sections) == 0 {
		return
	}
	g.section = clamp(g.section+d, 0, len(g.sections)-1)
	g.hunk = 0
	g.list, g.diffList = ui.ListState{}, ui.ListState{}
}
func (a *App) guideNext() {
	g := a.guide
	if len(g.hunks) == 0 {
		return
	}
	s, h := g.section, g.hunk
	for range len(g.hunks) {
		g.hunk++
		if g.hunk >= len(g.sections[g.section].Hunks) {
			g.hunk = 0
			g.section = (g.section + 1) % len(g.sections)
		}
		if !g.isReviewed(*g.current()) {
			g.diffList = ui.ListState{}
			g.list.ScrollTo(g.hunk, ui.Center)
			return
		}
	}
	g.section, g.hunk = s, h
}
func (a *App) guideBase() {
	a.prompt("Review branch against base", "main", "Committed changes from the merge base to HEAD. Leave empty for working tree.", func(base string) {
		a.guide.base = strings.TrimSpace(base)
		a.refreshGuide()
	})
}

func (a *App) guideScroll(d int) {
	g := a.guide
	if h := g.current(); h != nil {
		first, _ := g.diffList.Visible()
		target := clamp(max(0, first)+d, 0, len(parseDiff(h.Text))-1)
		g.diffList.ScrollTo(target, ui.Start)
	}
}

// GuideGenerating reports whether the experimental guide has an AI request in flight.
func (a *App) GuideGenerating() bool { return a.guide != nil && a.guide.generating }

package app

import (
	"crypto/sha256"
	"fmt"
	godraw "laziergit/internal/godraw/app"
	"laziergit/internal/godraw/doc"
	"laziergit/internal/repo"
	"laziergit/internal/reviewbridge"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

type guideHunk struct {
	ID, Path, Layer, Text, Fingerprint string
	OldHash, NewHash                   string
}
type guideSection struct {
	Title, Summary string
	Kind, Why, How string
	Notes          []reviewbridge.Note
	Media          []reviewbridge.Media
	Hunks          []int
	Diagram        *reviewbridge.Diagram
}
type guideState struct {
	active, focusCode           bool
	expandedNote                bool
	viewport                    float32
	horizontal                  ui.ScrollState
	viewportWidth               float32
	widths                      []int
	streams                     [][]guideStreamRow
	syntax                      guideSyntax
	lineCursor                  int
	entry                       *guideAnchor
	entryFingerprint            string
	entryLine                   int
	session, revision, imported string
	guideStamp                  int64
	reviewRoot, worktree        string
	feedback                    reviewbridge.FeedbackState
	showDiagram, baselineView   bool
	rangeStart                  int    // first row of a v range in the code; -1 for none
	query                       string // the last / search
	drawing                     *godraw.App
	drawingKey, diagramFit      string
	diagramSelected             string
	composer                    *guideComposer
	media                       map[string]ui.ImageSource
	linkHunk, linkFrom, linkTo  int // the selected shape's linked lines
	diagramCodeFocus            bool
	followingCode               bool
	baselineDrawing             *doc.Drawing
	diagramCursor               int
	diagramBaseline             *reviewbridge.Diagram
	baselineDocument            []byte
	base                        string
	context                     repo.GuideContext
	provenance                  string
	overview                    *reviewbridge.Overview
	hunks                       []guideHunk
	sections                    []guideSection
	section, hunk               int
	list, diffList              ui.ListState
	reviewed                    map[string]string
	loading                     bool
	generation                  int
	err, source                 string
	notice                      string
}

// OpenGuide opens the experimental laziestgit review surface.
func (a *App) OpenGuide() {
	target := a.guideAnchor()
	if a.guide != nil {
		a.guide.active = true
		a.guide.selectAnchor(target)
		a.guide.entry = target
		return
	}
	a.guide = &guideState{active: true, expandedNote: true, linkHunk: -1, rangeStart: -1, reviewed: map[string]string{}, source: "Local guide", entry: target, reviewRoot: filepath.Join(a.repo.Git.RepoPaths.RepoGitDirPath(), "laziergit", "review"), worktree: a.repo.Git.RepoPaths.WorktreePath()}
	// Marks are keyed by scope, slot and fingerprint, so a stale mark shows ↻.
	_ = reviewbridge.Read(a.guide.reviewRoot, "reviewed.json", &a.guide.reviewed)
	if a.guide.reviewed == nil {
		a.guide.reviewed = map[string]string{}
	}
	// Reopen the scope chosen last time in this worktree (b changes it).
	var active struct {
		Scope string `json:"scope"`
	}
	if reviewbridge.Read(a.guide.reviewRoot, "active.json", &active) == nil {
		a.guide.base = active.Scope
	}
	a.refreshGuide()
}
func (a *App) toggleGuide() {
	if a.guide != nil && a.guide.active {
		a.hideGuide()
	} else {
		a.OpenGuide()
	}
}
func (a *App) closeGuide() {
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
		context := r.GuideContext(base)
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
			g.notice = ""
			g.hunks = hunks
			g.context = context
			g.provenance, g.overview = "", nil
			g.syntax = nil
			g.sections = classifyGuide(hunks)
			g.section, g.hunk = 0, 0
			g.source = "Local guide"
			g.imported = ""
			if err := g.exportSnapshot(); err != nil {
				g.err = err.Error()
			}
			a.pollGuide()
			g.list, g.diffList = ui.ListState{}, ui.ListState{}
			g.rebuildStreams()
			g.selectAnchor(g.entry)
			a.loadGuideFeedback()
			g.drawing = nil
			g.drawingKey = ""
			// The guide shows now; highlighting follows when it is ready.
			a.bg(func() {
				syntax := loadGuideSyntax(r, hunks)
				a.post(func() {
					if a.guide == g && g.generation == generation {
						g.applySyntax(syntax)
					}
				})
			})
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
			oldHash, newHash := guideBlobHashes(shown)
			out = append(out, guideHunk{OldHash: oldHash, NewHash: newHash, ID: fmt.Sprintf("h%d", len(out)+1), Path: name, Layer: layer, Text: shown, Fingerprint: fmt.Sprintf("%x", sum)})
			first = next
		}
		i = end
	}
	return out
}
func classifyGuide(hunks []guideHunk) []guideSection {
	sections := []guideSection{
		{Title: "Implementation", Summary: "Implementation changes, ordered before supporting files."},
		{Title: "Tests", Summary: "Tests and examples that accompany the implementation."},
		{Title: "Documentation & configuration", Summary: "Documentation and configuration changes."},
		{Title: "Generated & dependencies", Summary: "Dependency and generated-file changes. These stay unreviewed until you mark them."},
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
		err := os.MkdirAll(g.reviewRoot, 0700)
		if err == nil {
			err = reviewbridge.Write(g.reviewRoot, "reviewed.json", g.reviewed)
		}
		if err != nil {
			g.err = err.Error()
		}
	}
}

// guideMove is the dominant motion: hunks in the story, lines in code.
func (a *App) guideMove(d int) {
	g := a.guide
	if g.focusCode {
		a.guideLineMove(d)
		return
	}
	a.guideHunkMove(d)
}
func (a *App) guideHunkMove(d int) {
	g := a.guide
	if g.current() == nil {
		return
	}
	flat := g.hunk
	total := 0
	for i, s := range g.sections {
		if i < g.section {
			flat += len(s.Hunks)
		}
		total += len(s.Hunks)
	}
	flat = clamp(flat+d, 0, total-1)
	for i, s := range g.sections {
		if flat < len(s.Hunks) {
			g.section, g.hunk = i, flat
			break
		}
		flat -= len(s.Hunks)
	}
	g.revealSelected()
}
func (a *App) guideSection(d int) {
	g := a.guide
	if g.current() == nil {
		return
	}
	if g.focusCode {
		a.guideHunkMove(d)
		return
	}
	g.section = clamp(g.section+d, 0, len(g.sections)-1)
	g.hunk = 0
	g.revealSelected()
}
func (a *App) guideUnread(d int) {
	g := a.guide
	if g.current() == nil {
		return
	}
	s, h := g.section, g.hunk
	for range len(g.hunks) {
		g.hunk += d
		if g.hunk >= len(g.sections[g.section].Hunks) {
			g.section = (g.section + 1) % len(g.sections)
			g.hunk = 0
		}
		if g.hunk < 0 {
			g.section = (g.section + len(g.sections) - 1) % len(g.sections)
			g.hunk = len(g.sections[g.section].Hunks) - 1
		}
		if !g.isReviewed(*g.current()) {
			g.revealSelected()
			return
		}
	}
	g.section, g.hunk = s, h
}
func (a *App) guideNext() { a.guideUnread(1) }
func (a *App) guideBase() {
	a.prompt("Review branch against base", "main", "Committed changes from the merge base to HEAD. Leave empty for working tree.", func(base string) {
		a.guide.base = strings.TrimSpace(base)
		a.refreshGuide()
	})
}

func (a *App) guideScroll(d int) {
	g := a.guide
	if len(g.rows()) == 0 {
		return
	}
	first, _ := g.diffList.Visible()
	g.diffList.ScrollTo(clamp(max(0, first)+d, 0, len(g.rows())-1), ui.Start)
}

// GuideLoading reports whether the guide is still reading its diff (scripted captures wait for it).
func (a *App) GuideLoading() bool { return a.guide != nil && a.guide.loading }

func (a *App) guideStatus() {
	text := a.guide.err
	if text == "" {
		text = a.guide.notice
	}
	if text == "" {
		text = a.guide.source
	}
	a.showError("Review guide", fmt.Errorf("%s", text))
}

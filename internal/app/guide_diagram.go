package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	godraw "laziergit/internal/godraw/app"
	"laziergit/internal/godraw/doc"
	"laziergit/internal/godraw/editor"
	"laziergit/internal/reviewbridge"
)

type diagramHost struct {
	app   *App
	state *guideState
}

func (h diagramHost) Window() *mygo.Window { return nil }
func (h diagramHost) Update(fn func()) {
	h.app.post(func() {
		if h.app.guide == h.state {
			fn()
		}
	})
}
func (h diagramHost) Changed(d *doc.Drawing) {
	if h.app.guide == h.state {
		h.app.saveDiagramProposal(d)
	}
}

func drawingBytes(d *doc.Drawing) []byte {
	var b bytes.Buffer
	if doc.Write(&b, d) != nil {
		return nil
	}
	return b.Bytes()
}
func (a *App) ensureDiagram() {
	g := a.guide
	if len(g.sections) == 0 {
		return
	}
	graph := g.sections[g.section].Diagram
	if graph == nil {
		g.drawing = nil
		return
	}
	key := diagramKey(g.revision, *graph)
	if g.drawing != nil && g.drawingKey == key {
		return
	}
	g.drawingKey = key
	g.baselineView = false
	g.diagramBaseline = graph
	baseline := diagramDrawing(*graph)
	g.baselineDrawing = diagramDrawing(*graph)
	g.baselineDocument = drawingBytes(baseline)
	drawing := baseline
	if b, err := os.ReadFile(filepath.Join(g.session, "proposal-"+key+".tldr")); err == nil {
		if parsed, err := doc.Read(bytes.NewReader(b)); err == nil {
			drawing = parsed
		}
	}
	g.drawing = godraw.New(drawing, diagramHost{a, g})
	g.diagramFit = ""
	g.drawing.Editor().Next = editor.DefaultStyle()
	g.drawing.Editor().Next.Color = "violet"
	g.drawing.Editor().Next.Dash = "dashed"
	g.drawing.Editor().Next.Font = "sans"
	g.diagramCursor = 0
	a.decorateProposal()
}
func (a *App) guideToggleDiagram() {
	g := a.guide
	g.diagramCodeFocus = false
	if g.showDiagram {
		g.showDiagram = false
		return
	}
	if len(g.sections) == 0 {
		return
	}
	if g.sections[g.section].Diagram == nil {
		g.notice = "This section has no agent diagram yet"
		return
	}
	g.showDiagram = !g.showDiagram
	if g.showDiagram {
		a.ensureDiagram()
	}
}
func (a *App) saveDiagramProposal(d *doc.Drawing) {
	g := a.guide
	if g == nil || g.diagramBaseline == nil || g.baselineView {
		return
	}
	data := drawingBytes(d)
	if len(data) > 2<<20 {
		g.err = "Diagram exceeds the 2 MB proposal limit"
		return
	}
	if err := reviewbridge.WriteBytes(g.session, "proposal-"+g.drawingKey+".tldr", data); err != nil {
		g.err = err.Error()
		return
	}
	proposed := drawingDiagram(d, g.diagramBaseline.Kind)
	changes := reviewbridge.DiagramChanges(*g.diagramBaseline, proposed)
	var existing *reviewbridge.Feedback
	a.loadGuideFeedback()
	for _, item := range g.feedback.Items {
		if item.Kind == "diagram_proposal" && item.Status == "draft" && item.Revision == g.revision && item.Diagram != nil && item.Diagram.Section == g.sections[g.section].Title {
			copy := item
			existing = &copy
			break
		}
	}
	alreadySent := false
	for _, sent := range g.feedback.Items {
		if sent.Status != "draft" && sent.Diagram != nil && sent.Revision == g.revision && sent.Diagram.Section == g.sections[g.section].Title && len(reviewbridge.DiagramChanges(sent.Diagram.Proposed, proposed)) == 0 {
			alreadySent = true
		}
	}
	if len(changes) == 0 || alreadySent {
		if existing != nil {
			_ = reviewbridge.DeleteDraft(g.session, existing.ID)
		}
		a.loadGuideFeedback()
		a.decorateProposal()
		return
	}
	item := reviewbridge.Feedback{Kind: "diagram_proposal", Revision: g.revision, Body: "Proposed diagram changes", Diagram: &reviewbridge.DiagramProposal{Section: g.sections[g.section].Title, Baseline: *g.diagramBaseline, Proposed: proposed, Changes: changes, BaselineDocument: g.baselineDocument, ProposedDocument: data}}
	if existing != nil {
		item.ID = existing.ID
	}
	if err := reviewbridge.SaveDraft(g.session, item); err != nil {
		g.err = err.Error()
		return
	}
	a.loadGuideFeedback()
	g.notice = "Diagram proposal saved privately · S sends review"
	a.decorateProposal()
}

// diagramKey names a section diagram's proposal file: one per reviewed
// revision and agent baseline.
func diagramKey(revision string, graph reviewbridge.Diagram) string {
	b, _ := json.Marshal(graph)
	return fmt.Sprintf("%x", sha256.Sum256(append([]byte(revision), b...)))
}

// discardDiagramProposal removes a deleted draft's working document, so the
// editor shows the agent baseline again instead of an untracked proposal.
func (a *App) discardDiagramProposal(item reviewbridge.Feedback) {
	g := a.guide
	key := diagramKey(item.Revision, item.Diagram.Baseline)
	if err := os.Remove(filepath.Join(g.session, "proposal-"+key+".tldr")); err != nil && !os.IsNotExist(err) {
		g.err = err.Error()
	}
	if g.drawingKey == key {
		g.drawing, g.drawingKey = nil, ""
		if g.showDiagram {
			a.ensureDiagram()
		}
	}
}

// arrowEnds names the logical nodes an arrow's bindings connect.
func arrowEnds(d *doc.Drawing, arrow doc.ID) (from, to string) {
	for _, binding := range d.Bindings {
		if binding.From != arrow {
			continue
		}
		switch binding.Terminal {
		case "start":
			from = diagramLogicalID(d.Shapes[binding.To])
		case "end":
			to = diagramLogicalID(d.Shapes[binding.To])
		}
	}
	return from, to
}

// decorateProposal paints the reviewer's semantic changes purple: dashed while
// private, solid once the same proposal was sent. Moving or resizing shapes is
// layout, not a proposal, and keeps the agent's colours.
func (a *App) decorateProposal() {
	g := a.guide
	if g.drawing == nil || g.diagramBaseline == nil || g.baselineDrawing == nil {
		return
	}
	kind := g.diagramBaseline.Kind
	baseline := g.baselineDrawing
	current := g.drawing.Editor().Doc
	dash := "dashed"
	proposed := drawingDiagram(current, kind)
	for _, item := range g.feedback.Items {
		if item.Status != "draft" && item.Diagram != nil && item.Revision == g.revision && item.Diagram.Section == g.sections[g.section].Title && len(reviewbridge.DiagramChanges(item.Diagram.Proposed, proposed)) == 0 {
			dash = "solid"
		}
	}
	for id, shape := range current.Shapes {
		old := baseline.Shapes[id]
		sem := shapeSemantics(current, shape, kind)
		changed := sem != "" && (old == nil || sem != shapeSemantics(baseline, old, kind))
		color, style := shape.Color, shape.Dash
		if changed && !g.baselineView {
			color, style = "violet", dash
		} else if old != nil {
			color, style = old.Color, old.Dash
		}
		if color != shape.Color || style != shape.Dash {
			copy := shape.Clone()
			copy.Color, copy.Dash = color, style
			current.Shapes[id] = copy
		}
	}
}
func (a *App) guideDiagramNudge(dx, dy float64) {
	g := a.guide
	if g.drawing != nil && !g.baselineView {
		g.drawing.NudgeSelection(dx, dy)
	}
}
func (a *App) guideDiagramScale(f float64) {
	g := a.guide
	if g.drawing != nil && !g.baselineView {
		g.drawing.ScaleSelection(f)
	}
}

// diagramIDs are the shapes j/k walk through, in reading order: flow nodes
// by column, sequence participants left to right and then messages top down.
func (a *App) diagramIDs() []doc.ID {
	g := a.guide
	if g.drawing == nil {
		return nil
	}
	d := g.drawing.Editor().Doc
	seq := g.diagramBaseline != nil && g.diagramBaseline.IsSequence()
	ids := []doc.ID{}
	for id, s := range d.Shapes {
		m := metaOf(s)
		if m.decoration() || (s.Type == doc.TypeArrow && !seq) {
			continue
		}
		ids = append(ids, id)
	}
	rank := func(s *doc.Shape) [3]float64 {
		if seq {
			if s.Type == doc.TypeArrow {
				start, _ := arrowPoints(s)
				return [3]float64{1, start.Y, start.X}
			}
			return [3]float64{0, s.X, s.Y}
		}
		return [3]float64{0, math.Round(s.X / 40), s.Y}
	}
	sort.Slice(ids, func(i, j int) bool {
		x, y := rank(d.Shapes[ids[i]]), rank(d.Shapes[ids[j]])
		if x != y {
			for k := range x {
				if x[k] != y[k] {
					return x[k] < y[k]
				}
			}
		}
		return ids[i] < ids[j]
	})
	return ids
}
func (a *App) guideDiagramMove(d int) {
	ids := a.diagramIDs()
	g := a.guide
	if len(ids) == 0 {
		return
	}
	if sel := g.drawing.Editor().Selection(); len(sel) > 0 {
		for i, id := range ids {
			if id == sel[0] {
				g.diagramCursor = i
			}
		}
	}
	g.diagramCursor = clamp(g.diagramCursor+d, 0, len(ids)-1)
	g.drawing.Editor().Select(ids[g.diagramCursor])
	a.guideDiagramFollowSelection()
}
func (a *App) guideDiagramLabel() {
	g := a.guide
	if g.drawing == nil || g.baselineView {
		return
	}
	selection := g.drawing.Editor().Selection()
	if len(selection) == 0 {
		a.guideDiagramMove(0)
		selection = g.drawing.Editor().Selection()
	}
	if len(selection) == 0 {
		return
	}
	id := selection[0]
	shape := g.drawing.Editor().Doc.Shapes[id]
	if shape == nil {
		return
	}
	a.prompt("Edit proposed label", shape.Text, "This edits your proposal, not the agent baseline or code.", func(text string) {
		e := g.drawing.Editor()
		e.StartEditing(id)
		e.EditText = text
		e.EditChanged()
		e.StopEditing()
	})
}

// guideDiagramJump moves the keys to the code under the diagram, at the
// selected shape's link. The diagram stays visible; Escape returns to it.
func (a *App) guideDiagramJump() {
	g := a.guide
	if g.drawing == nil {
		return
	}
	if len(g.drawing.Editor().Selection()) == 0 {
		a.guideDiagramMove(0)
	}
	g.diagramSelected = ""
	a.guideDiagramFollowSelection()
	g.focusCode, g.diagramCodeFocus = true, true
}
func (a *App) guideDiagramComment() {
	g := a.guide
	a.prompt("Comment on diagram", "", "Private draft until S sends review", func(body string) {
		if err := reviewbridge.SaveDraft(g.session, reviewbridge.Feedback{Kind: "diagram_note", Revision: g.revision, Body: g.sections[g.section].Title + ": " + body}); err != nil {
			g.err = err.Error()
		}
		a.loadGuideFeedback()
	})
}
func (a *App) guideDiagramBaseline() {
	g := a.guide
	if g.drawing == nil {
		return
	}
	g.baselineView = !g.baselineView
	if g.baselineView {
		d, _ := doc.Read(bytes.NewReader(g.baselineDocument))
		g.drawing.Editor().Replace(d)
		g.drawing.Editor().ReadOnly = true
	} else {
		g.drawing = nil
		a.ensureDiagram()
	}
}

// selectedLink is the code the selected shape links to, if any.
func (a *App) selectedLink() (shapeMeta, bool) {
	g := a.guide
	if g.drawing == nil {
		return shapeMeta{}, false
	}
	sel := g.drawing.Editor().Selection()
	if len(sel) == 0 {
		return shapeMeta{}, false
	}
	m := metaOf(g.drawing.Editor().Doc.Shapes[sel[0]])
	return m, m.HunkID != ""
}
func (a *App) guideDiagramView(c *ui.Context, p *Palette) {
	g := a.guide
	a.ensureDiagram()
	if g.drawing == nil {
		// Moving to a section without a diagram goes back to its code.
		g.showDiagram, g.diagramCodeFocus = false, false
		return
	}
	_, height := c.Size()
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Row(c).Height(30).PaddingX(12).Gap(8).AlignItems(ui.Center).Children(func() {
			title := g.diagramBaseline.Title
			ui.Text(c, title).FontWeight(600).SingleLine().Shrink(1)
			label := "Your proposal · purple marks your changes · dashed until sent"
			color := p.MergedPR
			if g.baselineView {
				label, color = "Agent's diagram · read-only", p.Muted
			}
			ui.Text(c, label).FontSize(sizeCaption).TextColor(color).Grow(1).Shrink(1).SingleLine()
			guideButton(c, p, "v agent / yours", a.guideDiagramBaseline)
		})
		a.decorateProposal()
		bg := editorColor(p.Bg)
		canvas := ui.Box(c).Height(max(240, height*0.5)).FillWidth().Clip()
		if g.diagramCodeFocus {
			canvas.Opacity(0.85)
		}
		canvas.Children(func() { g.drawing.EmbeddedViewWith(c, godraw.Embedding{Tools: true, Background: &bg}) })
		// Fit each diagram once its canvas has a real size, and again when the
		// window resizes, so bent arrows and labels are never clipped.
		if e := g.drawing.Editor(); e.VW > 0 && e.VH > 0 {
			if size := fmt.Sprintf("%s:%.0fx%.0f", g.drawingKey, e.VW, e.VH); g.diagramFit != size {
				g.diagramFit = size
				e.ZoomToFit()
			}
		}
		a.guideDiagramFollowSelection()
		a.guideDiagramFollowCode()
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		ui.Row(c).Height(28).PaddingX(12).Gap(10).AlignItems(ui.Center).Children(func() {
			link, ok := a.selectedLink()
			switch {
			case ok:
				where := ""
				for _, h := range g.hunks {
					if h.ID == link.HunkID {
						where = h.Path
					}
				}
				where += lineRange(link.Line, link.EndLine)
				ui.Text(c, "↳ "+where).Font(monoFont).FontSize(sizeCaption).TextColor(p.Accent).SingleLine().Shrink(1)
				ui.Text(c, "shown below · enter reads it").FontSize(sizeCaption).TextColor(p.Muted).SingleLine()
			case len(g.drawing.Editor().Selection()) > 0:
				ui.Text(c, "Not linked to changed code").FontSize(sizeCaption).TextColor(p.Muted)
			default:
				ui.Text(c, "Blue shapes link to code · click one or use j/k").FontSize(sizeCaption).TextColor(p.Muted)
			}
			ui.Box(c).Grow(1)
			if changes := reviewbridge.DiagramChanges(*g.diagramBaseline, drawingDiagram(g.drawing.Editor().Doc, g.diagramBaseline.Kind)); len(changes) > 0 && !g.baselineView {
				ui.Text(c, fmt.Sprintf("%d proposed · ", len(changes))+strings.Join(changes, " · ")).FontSize(sizeCaption).TextColor(p.MergedPR).SingleLine().Shrink(1)
			}
		})
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		rows := g.rows()
		ui.List(c, &g.diffList, len(rows), func(i int) {
			if rows[i].hunk >= 0 {
				a.guideCodeRow(c, p, i, rows[i])
			}
		}).Grow(1).MinHeight(0)
	})
}
func editorColor(c ui.Color) editor.Color {
	return editor.Color{R: c.R, G: c.G, B: c.B, A: float64(c.A) / 255}
}

func (a *App) guideDiagramTool(tool string) {
	g := a.guide
	if g.drawing != nil && !g.baselineView {
		g.drawing.Editor().SetTool(editor.Tool(tool))
	}
}
func (a *App) guideDiagramDelete() {
	g := a.guide
	if g.drawing != nil && !g.baselineView {
		g.drawing.Editor().Delete()
	}
}
func (a *App) guideDiagramUndo() {
	g := a.guide
	if g.drawing != nil && !g.baselineView {
		g.drawing.Editor().Undo()
	}
}
func (a *App) guideDiagramRedo() {
	g := a.guide
	if g.drawing != nil && !g.baselineView {
		g.drawing.Editor().Redo()
	}
}
func (a *App) guideDiagramFit() {
	g := a.guide
	if g.drawing != nil {
		g.drawing.Editor().ZoomToFit()
	}
}

// guideDiagramFollowSelection shows the selected shape's code below the
// diagram whenever the selection changes, by click or by key.
func (a *App) guideDiagramFollowSelection() {
	g := a.guide
	if g.drawing == nil {
		return
	}
	sel := g.drawing.Editor().Selection()
	id := ""
	if len(sel) > 0 {
		id = string(sel[0])
	}
	if id == g.diagramSelected {
		return
	}
	g.linkHunk = -1
	g.diagramSelected = id
	if g.followingCode {
		return // the code cursor chose this shape; leave the code where it is
	}
	if id != "" && g.diagramCodeFocus {
		g.diagramCodeFocus, g.focusCode = false, false // a click in the diagram takes the keys back
	}
	m, ok := a.selectedLink()
	if !ok {
		return
	}
	for i, idx := range g.sections[g.section].Hunks {
		if g.hunks[idx].ID != m.HunkID {
			continue
		}
		g.hunk = i
		g.revealSelected()
		g.linkHunk, g.linkFrom, g.linkTo = idx, m.Line, max(m.Line, m.EndLine)
		if m.Line > 0 {
			for r, row := range g.rows() {
				if row.hunk == idx && row.newLine == m.Line && row.line.kind != lineDel {
					g.lineCursor = r
					g.diffList.ScrollTo(max(0, r-3), ui.Start)
					break
				}
			}
		}
		return
	}
}

// guideDiagramFollowCode is the reverse link: reading code under the diagram
// selects the shape that links to it, the nearest line at or above the cursor.
func (a *App) guideDiagramFollowCode() {
	g := a.guide
	if g.drawing == nil || !g.diagramCodeFocus {
		return
	}
	rows := g.rows()
	if g.lineCursor < 0 || g.lineCursor >= len(rows) || rows[g.lineCursor].hunk < 0 {
		return
	}
	row := rows[g.lineCursor]
	hunk := g.hunks[row.hunk].ID
	best, bestLine := doc.ID(""), -1
	for id, s := range g.drawing.Editor().Doc.Shapes {
		m := metaOf(s)
		if m.HunkID != hunk {
			continue
		}
		line := m.Line
		if line > 0 && (row.newLine == 0 || line > row.newLine) {
			continue
		}
		if m.EndLine > 0 && row.newLine > m.EndLine {
			line = 0 // the cursor is past this range: only a weak match
		}
		if line > bestLine || (line == bestLine && id < best) {
			best, bestLine = id, line
		}
	}
	if best != "" && string(best) != g.diagramSelected {
		g.followingCode = true
		g.drawing.Editor().Select(best)
		a.guideDiagramFollowSelection()
		g.followingCode = false
	}
}

// lineRange formats a link's lines as :12 or :12–18.
func lineRange(from, to int) string {
	switch {
	case from <= 0:
		return ""
	case to > from:
		return fmt.Sprintf(":%d–%d", from, to)
	}
	return fmt.Sprintf(":%d", from)
}

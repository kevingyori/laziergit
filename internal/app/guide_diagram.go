package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	godraw "laziergit/internal/godraw/app"
	"laziergit/internal/godraw/doc"
	"laziergit/internal/godraw/editor"
	"laziergit/internal/godraw/geo"
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

type nodeAnchor struct {
	HunkID  string `json:"hunk_id,omitempty"`
	NodeID  string `json:"node_id,omitempty"`
	Context bool   `json:"context,omitempty"`
}

func diagramDrawing(graph reviewbridge.Diagram) *doc.Drawing {
	d := doc.New()
	d.Name = graph.Title
	last := ""
	nodes := map[string]*doc.Shape{}
	add := func(s *doc.Shape) {
		s.Parent = d.Pages[0].ID
		s.Index = doc.IndexBetween(last, "")
		last = s.Index
		d.Shapes[s.ID] = s
	}
	// Stable three-column placement; godraw handles bound arrows and subsequent edits.
	for i, node := range graph.Nodes {
		s := doc.NewShape(doc.ID("shape:node-"+node.ID), doc.TypeGeo)
		s.X, s.Y = float64(i%3)*260, float64(i/3)*180
		s.W, s.H = 210, 90
		s.Dash = "solid"
		s.Size = "s"
		s.Font = "sans"
		s.Text = node.Label
		s.Meta, _ = json.Marshal(nodeAnchor{NodeID: node.ID, HunkID: node.HunkID, Context: node.Context})
		add(s)
		nodes[node.ID] = s
	}
	for i, edge := range graph.Edges {
		from, to := nodes[edge.From], nodes[edge.To]
		if from == nil || to == nil {
			continue
		}
		s := doc.NewShape(doc.ID(fmt.Sprintf("shape:graph-edge-%d", i)), doc.TypeArrow)
		s.Start = geo.V(from.X+from.W/2, from.Y+from.H/2)
		s.End = geo.V(to.X+to.W/2, to.Y+to.H/2)
		s.Text = edge.Label
		if from.Y == to.Y {
			s.Bend = -70
		}
		s.Dash = "solid"
		s.Size = "s"
		s.Font = "sans"
		add(s)
		for term, target := range map[string]*doc.Shape{"start": from, "end": to} {
			id := doc.ID(fmt.Sprintf("binding:edge-%d-%s", i, term))
			d.Bindings[id] = &doc.Binding{ID: id, Type: "arrow", From: s.ID, To: target.ID, Terminal: term, Anchor: geo.V(.5, .5), Snap: "none"}
		}
	}
	return d
}
func drawingDiagram(d *doc.Drawing) reviewbridge.Diagram {
	graph := reviewbridge.Diagram{Title: d.Name}
	ids := make([]string, 0, len(d.Shapes))
	for id := range d.Shapes {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		s := d.Shapes[doc.ID(id)]
		if s.Type == doc.TypeArrow {
			continue
		}
		var anchor nodeAnchor
		_ = json.Unmarshal(s.Meta, &anchor)
		label := s.Text
		if label == "" {
			label = s.Type
		}
		graph.Nodes = append(graph.Nodes, reviewbridge.DiagramNode{ID: diagramLogicalID(s), Label: label, HunkID: anchor.HunkID, Context: anchor.Context || anchor.HunkID == ""})
	}
	for _, id := range ids {
		s := d.Shapes[doc.ID(id)]
		if s.Type != doc.TypeArrow {
			continue
		}
		from, to := "", ""
		for _, binding := range d.Bindings {
			if binding.From == s.ID {
				if binding.Terminal == "start" {
					from = diagramLogicalID(d.Shapes[binding.To])
				}
				if binding.Terminal == "end" {
					to = diagramLogicalID(d.Shapes[binding.To])
				}
			}
		}
		if from != "" && to != "" {
			graph.Edges = append(graph.Edges, reviewbridge.DiagramEdge{From: from, To: to, Label: s.Text})
		} else {
			graph.Nodes = append(graph.Nodes, reviewbridge.DiagramNode{ID: strings.TrimPrefix(id, "shape:"), Label: "Unbound arrow: " + s.Text, Context: true})
		}
	}
	return graph
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
	proposed := drawingDiagram(d)
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
	if g.drawing == nil || g.diagramBaseline == nil {
		return
	}
	baseline := diagramDrawing(*g.diagramBaseline)
	current := g.drawing.Editor().Doc
	dash := "dashed"
	proposed := drawingDiagram(current)
	for _, item := range g.feedback.Items {
		if item.Status != "draft" && item.Diagram != nil && item.Revision == g.revision && item.Diagram.Section == g.sections[g.section].Title && len(reviewbridge.DiagramChanges(item.Diagram.Proposed, proposed)) == 0 {
			dash = "solid"
		}
	}
	for id, shape := range current.Shapes {
		old := baseline.Shapes[id]
		changed := old == nil || shape.Text != old.Text || shape.Type != old.Type
		if shape.Type == doc.TypeArrow && old != nil {
			from, to := arrowEnds(current, id)
			oldFrom, oldTo := arrowEnds(baseline, id)
			changed = changed || from != oldFrom || to != oldTo
		}
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
func (a *App) diagramIDs() []doc.ID {
	g := a.guide
	if g.drawing == nil {
		return nil
	}
	ids := []doc.ID{}
	for id, s := range g.drawing.Editor().Doc.Shapes {
		if s.Type != doc.TypeArrow {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
func (a *App) guideDiagramMove(d int) {
	ids := a.diagramIDs()
	g := a.guide
	if len(ids) == 0 {
		return
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
	a.prompt("Edit proposed node label", shape.Text, "This edits your proposal, not the agent baseline or code.", func(text string) {
		e := g.drawing.Editor()
		e.StartEditing(id)
		e.EditText = text
		e.EditChanged()
		e.StopEditing()
	})
}
func (a *App) guideDiagramJump() {
	g := a.guide
	if g.drawing == nil {
		return
	}
	selection := g.drawing.Editor().Selection()
	if len(selection) == 0 {
		return
	}
	s := g.drawing.Editor().Doc.Shapes[selection[0]]
	if s == nil {
		return
	}
	var anchor nodeAnchor
	_ = json.Unmarshal(s.Meta, &anchor)
	for i, idx := range g.sections[g.section].Hunks {
		if g.hunks[idx].ID == anchor.HunkID {
			g.hunk = i
			g.showDiagram = false
			g.focusCode = true
			g.revealSelected()
			a.guideEnter()
			return
		}
	}
	g.notice = "This context shape has no changed-code anchor"
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
func (a *App) guideDiagramView(c *ui.Context, p *Palette) {
	g := a.guide
	a.ensureDiagram()
	if g.drawing == nil {
		ui.Text(c, "No diagram in this section · D returns to code").Padding(12)
		return
	}
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Row(c).Height(30).PaddingX(10).Gap(8).AlignItems(ui.Center).Children(func() {
			label := "Your proposal · purple: proposed code · dashed drafts / solid sent"
			if g.baselineView {
				label = "Agent baseline · read-only explanation of the reviewed code"
			}
			// Purple only ever means the reviewer's proposal.
			color := p.MergedPR
			if g.baselineView {
				color = p.Muted
			}
			ui.Text(c, label).FontSize(sizeCaption).TextColor(color).Grow(1).Shrink(1).SingleLine()
			guideButton(c, p, "v baseline / proposal", a.guideDiagramBaseline)
			guideButton(c, p, "e label", a.guideDiagramLabel)
		})
		a.decorateProposal()
		ui.Box(c).Height(300).FillWidth().Clip().Children(func() { g.drawing.EmbeddedView(c) })
		// Fit each diagram once its canvas has a real size, and again when the
		// window resizes, so bent arrows and labels are never clipped.
		if e := g.drawing.Editor(); e.VW > 0 && e.VH > 0 {
			if size := fmt.Sprintf("%s:%.0fx%.0f", g.drawingKey, e.VW, e.VH); g.diagramFit != size {
				g.diagramFit = size
				e.ZoomToFit()
			}
		}
		a.guideDiagramFollowSelection()
		changes := reviewbridge.DiagramChanges(*g.diagramBaseline, drawingDiagram(g.drawing.Editor().Doc))
		if len(changes) > 0 && !g.baselineView {
			ui.Text(c, strings.Join(changes, " · ")).FontSize(sizeCaption).TextColor(p.MergedPR).SingleLine().PaddingX(10)
		}
		ui.Row(c).Height(28).PaddingX(8).Gap(10).Children(func() {
			for _, tool := range []struct {
				key, label string
				tool       editor.Tool
			}{{"s", "Select", editor.ToolSelect}, {"a", "Arrow", editor.ToolArrow}, {"n", "Note", editor.ToolNote}, {"p", "Draw", editor.ToolDraw}, {"b", "Box", editor.ToolGeo}} {
				tool := tool
				guideButton(c, p, tool.key+" "+tool.label, func() {
					if !g.baselineView {
						g.drawing.Editor().SetTool(tool.tool)
					}
				})
			}
			guideButton(c, p, "u undo", func() { g.drawing.Editor().Undo() })
			guideButton(c, p, "= fit", func() { g.drawing.Editor().ZoomToFit() })
		})
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		ui.Text(c, "Code from the reviewed snapshot · Enter on a shape follows its anchor").FontSize(sizeCaption).TextColor(p.Muted).PaddingX(10).PaddingY(4)
		rows := g.rows()
		ui.List(c, &g.diffList, len(rows), func(i int) {
			if rows[i].hunk >= 0 {
				a.guideCodeRow(c, p, i, rows[i])
			}
		}).Grow(1).MinHeight(0)

	})
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

func diagramLogicalID(s *doc.Shape) string {
	if s == nil {
		return ""
	}
	var anchor nodeAnchor
	_ = json.Unmarshal(s.Meta, &anchor)
	if anchor.NodeID != "" {
		return anchor.NodeID
	}
	return strings.TrimPrefix(string(s.ID), "shape:")
}

func (a *App) guideDiagramFollowSelection() {
	g := a.guide
	if g.drawing == nil {
		return
	}
	selection := g.drawing.Editor().Selection()
	if len(selection) == 0 {
		return
	}
	shape := g.drawing.Editor().Doc.Shapes[selection[0]]
	if shape == nil {
		return
	}
	var anchor nodeAnchor
	_ = json.Unmarshal(shape.Meta, &anchor)
	for i, idx := range g.sections[g.section].Hunks {
		if g.hunks[idx].ID == anchor.HunkID && i != g.hunk {
			g.hunk = i
			g.revealSelected()
			break
		}
	}
}

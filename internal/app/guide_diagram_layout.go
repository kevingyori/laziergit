package app

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"laziergit/internal/godraw/doc"
	"laziergit/internal/godraw/geo"
	"laziergit/internal/reviewbridge"
)

// shapeMeta is what a diagram shape means: which logical node, participant
// or message it draws, and the code it links to. Lanes and lifelines are
// decoration: locked, and never part of a proposal.
type shapeMeta struct {
	Role    string `json:"role,omitempty"` // node, participant, message, lane, lifeline
	NodeID  string `json:"node_id,omitempty"`
	HunkID  string `json:"hunk_id,omitempty"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"end_line,omitempty"`
	Context bool   `json:"context,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Group   string `json:"group,omitempty"`
	Reply   bool   `json:"reply,omitempty"`
	Style   string `json:"style,omitempty"`
}

func metaOf(s *doc.Shape) shapeMeta {
	var m shapeMeta
	if s != nil {
		_ = json.Unmarshal(s.Meta, &m)
	}
	return m
}
func (m shapeMeta) decoration() bool { return m.Role == "lane" || m.Role == "lifeline" }

// Layout constants, in page units. Labels use godraw's small sans text, the
// app's prose face; links to code are blue, everything else stays neutral.
const (
	colW, rowH         = 320.0, 84.0 // room between boxes for edge labels
	nodeW, nodeH       = 190.0, 56.0
	laneHead, laneGap  = 34.0, 12.0
	partW, partH, msgH = 172.0, 52.0, 54.0
)

type drawingBuilder struct {
	d    *doc.Drawing
	last string
}

func (b *drawingBuilder) add(s *doc.Shape, m shapeMeta) *doc.Shape {
	s.Parent = b.d.Pages[0].ID
	s.Index = doc.IndexBetween(b.last, "")
	b.last = s.Index
	s.Size, s.Font, s.Dash = "s", "sans", "solid"
	s.Meta, _ = json.Marshal(m)
	b.d.Shapes[s.ID] = s
	return s
}
func linkColor(hunk string) string {
	if hunk != "" {
		return "blue"
	}
	return "grey"
}

func diagramDrawing(graph reviewbridge.Diagram) *doc.Drawing {
	b := &drawingBuilder{d: doc.New()}
	b.d.Name = graph.Title
	if graph.IsSequence() {
		b.sequence(graph.MessageIDs())
	} else {
		b.flow(graph)
	}
	return b.d
}

// flow ranks nodes left to right by the longest path through the edges
// (cycles are broken where a depth-first walk meets them) and stacks each
// group as a horizontal lane.
func (b *drawingBuilder) flow(graph reviewbridge.Diagram) {
	index := map[string]int{}
	for i, n := range graph.Nodes {
		index[n.ID] = i
	}
	out := make([][]int, len(graph.Nodes))
	for _, e := range graph.Edges {
		if from, ok := index[e.From]; ok {
			if to, ok := index[e.To]; ok {
				out[from] = append(out[from], to)
			}
		}
	}
	state := make([]int, len(graph.Nodes)) // 0 new, 1 on stack, 2 done
	var order []int
	back := map[[2]int]bool{}
	var visit func(int)
	visit = func(u int) {
		state[u] = 1
		for _, v := range out[u] {
			if state[v] == 1 {
				back[[2]int{u, v}] = true
			} else if state[v] == 0 {
				visit(v)
			}
		}
		state[u] = 2
		order = append(order, u)
	}
	for i := range graph.Nodes {
		if state[i] == 0 {
			visit(i)
		}
	}
	rank := make([]int, len(graph.Nodes))
	for i := len(order) - 1; i >= 0; i-- {
		u := order[i]
		for _, v := range out[u] {
			if !back[[2]int{u, v}] {
				rank[v] = max(rank[v], rank[u]+1)
			}
		}
	}
	maxRank := 0
	for _, r := range rank {
		maxRank = max(maxRank, r)
	}
	lanes := []reviewbridge.DiagramGroup{}
	ungrouped := false
	for _, n := range graph.Nodes {
		ungrouped = ungrouped || n.Group == ""
	}
	if ungrouped {
		lanes = append(lanes, reviewbridge.DiagramGroup{})
	}
	lanes = append(lanes, graph.Groups...)
	shapes := map[string]*doc.Shape{}
	width := float64(maxRank)*colW + nodeW + 40
	y := 0.0
	for li, lane := range lanes {
		cells := map[int][]int{}
		rows := 1
		for i, n := range graph.Nodes {
			if n.Group == lane.ID {
				cells[rank[i]] = append(cells[rank[i]], i)
				rows = max(rows, len(cells[rank[i]]))
			}
		}
		if len(cells) == 0 {
			continue
		}
		top := y
		if lane.ID != "" {
			bg := doc.NewShape(doc.ID(fmt.Sprintf("shape:lane-%d", li)), doc.TypeGeo)
			bg.X, bg.Y, bg.W, bg.H = -20, y, width, laneHead+float64(rows)*rowH
			bg.Color, bg.Fill, bg.Text, bg.Align, bg.VAlign, bg.Locked = "grey", "semi", lane.Label, "start", "start", true
			b.add(bg, shapeMeta{Role: "lane", Group: lane.ID})
			top += laneHead
		}
		for r, members := range cells {
			for k, i := range members {
				n := graph.Nodes[i]
				s := doc.NewShape(doc.ID("shape:node-"+n.ID), doc.TypeGeo)
				s.X, s.Y, s.W, s.H = float64(r)*colW, top+float64(k)*rowH+(rowH-nodeH)/2, nodeW, nodeH
				s.Color, s.Fill, s.Text = linkColor(n.HunkID), "semi", n.Label
				shapes[n.ID] = b.add(s, shapeMeta{Role: "node", NodeID: n.ID, HunkID: n.HunkID, Line: n.Line, EndLine: n.EndLine, Context: n.Context, Kind: n.Kind, Group: n.Group})
				switch n.Kind {
				case "decision":
					s.Geo = "diamond"
				case "external", "actor":
					s.Geo = "ellipse"
				case "data":
					s.Geo = "hexagon"
				case "note":
					s.Fill, s.Dash = "none", "dotted"
				}
			}
		}
		y += laneHead*boolf(lane.ID != "") + float64(rows)*rowH + laneGap
	}
	for i, e := range graph.Edges {
		from, to := shapes[e.From], shapes[e.To]
		if from == nil || to == nil {
			continue
		}
		s := doc.NewShape(doc.ID(fmt.Sprintf("shape:graph-edge-%d", i)), doc.TypeArrow)
		s.Start = geo.V(from.X+from.W/2, from.Y+from.H/2)
		s.End = geo.V(to.X+to.W/2, to.Y+to.H/2)
		s.Text, s.Kind = e.Label, "elbow"
		if from.X >= to.X {
			s.Kind, s.Bend = "arc", -60 // back or same-column edges curve away
		}
		arrow := b.add(s, shapeMeta{Style: e.Style})
		arrow.Color = "grey"
		switch e.Style {
		case "data":
			arrow.Dash = "dashed"
		case "async":
			arrow.Dash = "dotted"
		}
		for term, target := range map[string]*doc.Shape{"start": from, "end": to} {
			id := doc.ID(fmt.Sprintf("binding:edge-%d-%s", i, term))
			b.d.Bindings[id] = &doc.Binding{ID: id, Type: "arrow", From: s.ID, To: target.ID, Terminal: term, Anchor: geo.V(.5, .5), Snap: "none"}
		}
	}
}
func boolf(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// sequence draws participants along the top with dashed lifelines, and each
// message as a horizontal arrow between them, in order, top to bottom.
func (b *drawingBuilder) sequence(graph reviewbridge.Diagram) {
	center := map[string]float64{}
	bottom := partH + 30 + float64(len(graph.Messages))*msgH
	for i, p := range graph.Participants {
		s := doc.NewShape(doc.ID("shape:node-"+p.ID), doc.TypeGeo)
		s.X, s.Y, s.W, s.H = float64(i)*(partW+48), 0, partW, partH
		s.Color, s.Fill, s.Text = linkColor(p.HunkID), "semi", p.Label
		b.add(s, shapeMeta{Role: "participant", NodeID: p.ID, HunkID: p.HunkID, Line: p.Line, EndLine: p.EndLine, Context: p.HunkID == ""})
		center[p.ID] = s.X + partW/2
		line := doc.NewShape(doc.ID("shape:lifeline-"+p.ID), doc.TypeLine)
		line.X, line.Y, line.Locked = center[p.ID], partH, true
		line.Points = []doc.LinePoint{{ID: "a1", Index: "a1"}, {ID: "a2", Index: "a2", Y: bottom - partH}}
		b.add(line, shapeMeta{Role: "lifeline", NodeID: p.ID})
		line.Color, line.Dash = "grey", "dashed"
	}
	for i, m := range graph.Messages {
		s := doc.NewShape(doc.ID("shape:message-"+m.ID), doc.TypeArrow)
		y := partH + 40 + float64(i)*msgH
		s.Start, s.End = geo.V(center[m.From], y), geo.V(center[m.To], y)
		if m.From == m.To {
			s.End, s.Bend = geo.V(center[m.From], y+msgH*0.6), -50
		}
		s.Text = m.Label
		arrow := b.add(s, shapeMeta{Role: "message", NodeID: m.ID, HunkID: m.HunkID, Line: m.Line, EndLine: m.EndLine, Reply: m.Reply})
		arrow.Color = "black"
		if m.HunkID != "" {
			arrow.Color = "blue"
		}
		if m.Reply {
			arrow.Dash = "dashed"
		}
	}
}

// arrowPoints returns an arrow's ends in page space.
func arrowPoints(s *doc.Shape) (geo.Vec, geo.Vec) {
	return geo.V(s.X+s.Start.X, s.Y+s.Start.Y), geo.V(s.X+s.End.X, s.Y+s.End.Y)
}

// lifelineOwner is the participant whose centre is nearest to x: dragging a
// message's end onto another lifeline reroutes it.
func lifelineOwner(d *doc.Drawing, x float64) string {
	best, dist := "", math.Inf(1)
	for _, s := range d.Shapes {
		if m := metaOf(s); s.Type != doc.TypeArrow && !m.decoration() && (m.Role == "participant" || m.Role == "") {
			if dd := math.Abs(s.X + s.W/2 - x); dd < dist {
				best, dist = diagramLogicalID(s), dd
			}
		}
	}
	return best
}

// drawingDiagram reads the meaning back out of a (possibly edited) drawing.
func drawingDiagram(d *doc.Drawing, kind string) reviewbridge.Diagram {
	graph := reviewbridge.Diagram{Title: d.Name, Kind: kind}
	ids := make([]string, 0, len(d.Shapes))
	for id := range d.Shapes {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	type msg struct {
		y float64
		m reviewbridge.DiagramMessage
	}
	var messages []msg
	for _, id := range ids {
		s := d.Shapes[doc.ID(id)]
		m := metaOf(s)
		if m.decoration() || s.Type == doc.TypeArrow {
			continue
		}
		label := s.Text
		if label == "" {
			label = s.Type
		}
		node := reviewbridge.DiagramNode{ID: diagramLogicalID(s), Label: label, Kind: m.Kind, Group: m.Group, HunkID: m.HunkID, Line: m.Line, EndLine: m.EndLine, Context: m.Context || m.HunkID == ""}
		if kind == "sequence" {
			graph.Participants = append(graph.Participants, node)
		} else {
			graph.Nodes = append(graph.Nodes, node)
		}
	}
	for _, id := range ids {
		s := d.Shapes[doc.ID(id)]
		if s.Type != doc.TypeArrow {
			continue
		}
		m := metaOf(s)
		if kind == "sequence" {
			start, end := arrowPoints(s)
			messages = append(messages, msg{start.Y, reviewbridge.DiagramMessage{ID: diagramLogicalID(s), From: lifelineOwner(d, start.X), To: lifelineOwner(d, end.X), Label: s.Text, Reply: m.Reply, HunkID: m.HunkID, Line: m.Line, EndLine: m.EndLine}})
			continue
		}
		from, to := arrowEnds(d, s.ID)
		if from != "" && to != "" {
			graph.Edges = append(graph.Edges, reviewbridge.DiagramEdge{From: from, To: to, Label: s.Text, Style: m.Style})
		} else {
			graph.Nodes = append(graph.Nodes, reviewbridge.DiagramNode{ID: strings.TrimPrefix(id, "shape:"), Label: "Unbound arrow: " + s.Text, Context: true})
		}
	}
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].y < messages[j].y })
	for _, m := range messages {
		graph.Messages = append(graph.Messages, m.m)
	}
	return graph
}

// shapeSemantics is what a shape says, for marking proposals: its label and,
// for connectors, what they connect. Decoration says nothing.
func shapeSemantics(d *doc.Drawing, s *doc.Shape, kind string) string {
	m := metaOf(s)
	if m.decoration() {
		return ""
	}
	if s.Type != doc.TypeArrow {
		return s.Type + "\x00" + s.Text
	}
	if kind == "sequence" {
		start, end := arrowPoints(s)
		return s.Text + "\x00" + lifelineOwner(d, start.X) + "\x00" + lifelineOwner(d, end.X)
	}
	from, to := arrowEnds(d, s.ID)
	return s.Text + "\x00" + from + "\x00" + to
}

func diagramLogicalID(s *doc.Shape) string {
	if s == nil {
		return ""
	}
	if m := metaOf(s); m.NodeID != "" {
		return m.NodeID
	}
	return strings.TrimPrefix(string(s.ID), "shape:")
}

package reviewbridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Diagram is an agent's explanatory picture of a section: a flow graph of
// components, functions and data in optional lanes, or a sequence of messages
// between participants. Anything drawn can link to a hunk, and a line in it.
type Diagram struct {
	Title        string           `json:"title"`
	Kind         string           `json:"kind,omitempty"` // flow (default) or sequence
	Groups       []DiagramGroup   `json:"groups,omitempty"`
	Nodes        []DiagramNode    `json:"nodes,omitempty"`
	Edges        []DiagramEdge    `json:"edges,omitempty"`
	Participants []DiagramNode    `json:"participants,omitempty"`
	Messages     []DiagramMessage `json:"messages,omitempty"`
}
type DiagramGroup struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type DiagramNode struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Kind    string `json:"kind,omitempty"`
	Group   string `json:"group,omitempty"`
	HunkID  string `json:"hunk_id,omitempty"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"end_line,omitempty"`
	Context bool   `json:"context,omitempty"`
}
type DiagramEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
	Style string `json:"style,omitempty"` // call, data, async
}
type DiagramMessage struct {
	ID      string `json:"id,omitempty"`
	From    string `json:"from"`
	To      string `json:"to"`
	Label   string `json:"label"`
	Reply   bool   `json:"reply,omitempty"`
	HunkID  string `json:"hunk_id,omitempty"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"end_line,omitempty"`
}

var nodeKinds = map[string]bool{"": true, "component": true, "function": true, "data": true, "external": true, "decision": true, "actor": true, "note": true}
var edgeStyles = map[string]bool{"": true, "call": true, "data": true, "async": true}

// IsSequence reports whether d draws messages between participants.
func (d Diagram) IsSequence() bool { return d.Kind == "sequence" }

// MessageIDs names messages by position where the agent left them unnamed,
// so proposals can tell an edited message from a new one.
func (d Diagram) MessageIDs() Diagram {
	d.Messages = append([]DiagramMessage(nil), d.Messages...)
	for i := range d.Messages {
		if d.Messages[i].ID == "" {
			d.Messages[i].ID = fmt.Sprintf("m%d", i+1)
		}
	}
	return d
}

// ValidateDiagram checks a diagram against its section: anchors must be
// section hunks and, with a line, a new-side line that hunk shows.
func ValidateDiagram(d Diagram, allowed map[string]bool, lineOK func(hunk string, line int) bool) error {
	if strings.TrimSpace(d.Title) == "" || len(d.Title) > 120 {
		return fmt.Errorf("diagram needs a title of at most 120 characters")
	}
	anchor := func(what, id, hunk string, line, end int, context bool) error {
		if hunk == "" {
			if !context && what != "message" && what != "participant" {
				return fmt.Errorf("%s %s needs a section hunk_id or context:true", what, id)
			}
			return nil
		}
		if !allowed[hunk] {
			return fmt.Errorf("%s %s links hunk %q, which is not in this section", what, id, hunk)
		}
		return CheckLines(what+" "+id, hunk, line, end, lineOK)
	}
	label := func(s string, n int) bool { return strings.TrimSpace(s) != "" && len(s) <= n }
	known := map[string]bool{}
	switch d.Kind {
	case "", "flow":
		if len(d.Nodes) == 0 || len(d.Nodes) > 24 || len(d.Edges) > 40 || len(d.Groups) > 6 || len(d.Participants)+len(d.Messages) > 0 {
			return fmt.Errorf("flow diagrams have 1-24 nodes, at most 40 edges and 6 groups (participants/messages belong to kind sequence)")
		}
		groups := map[string]bool{"": true}
		for _, g := range d.Groups {
			if g.ID == "" || groups[g.ID] || !label(g.Label, 60) {
				return fmt.Errorf("groups need unique ids and short labels")
			}
			groups[g.ID] = true
		}
		for _, n := range d.Nodes {
			if n.ID == "" || len(n.ID) > 80 || known[n.ID] || !label(n.Label, 120) || !nodeKinds[n.Kind] || !groups[n.Group] {
				return fmt.Errorf("node %q: unique id, label up to 120 characters, kind component/function/data/external/decision/actor/note, and a declared group", n.ID)
			}
			if err := anchor("node", n.ID, n.HunkID, n.Line, n.EndLine, n.Context); err != nil {
				return err
			}
			known[n.ID] = true
		}
		for _, e := range d.Edges {
			if !known[e.From] || !known[e.To] || e.From == e.To || len(e.Label) > 80 || !edgeStyles[e.Style] {
				return fmt.Errorf("edge %s → %s: both nodes must exist, differ, label up to 80 characters, style call/data/async", e.From, e.To)
			}
		}
	case "sequence":
		if len(d.Participants) < 2 || len(d.Participants) > 10 || len(d.Messages) == 0 || len(d.Messages) > 30 || len(d.Nodes)+len(d.Edges) > 0 {
			return fmt.Errorf("sequence diagrams have 2-10 participants and 1-30 messages (nodes/edges belong to kind flow)")
		}
		for _, p := range d.Participants {
			if p.ID == "" || known[p.ID] || !label(p.Label, 80) {
				return fmt.Errorf("participant %q needs a unique id and a short label", p.ID)
			}
			if err := anchor("participant", p.ID, p.HunkID, p.Line, p.EndLine, true); err != nil {
				return err
			}
			known[p.ID] = true
		}
		for i, m := range d.Messages {
			if !known[m.From] || !known[m.To] || !label(m.Label, 80) {
				return fmt.Errorf("message %d: from/to must be participants, label up to 80 characters", i+1)
			}
			if err := anchor("message", fmt.Sprint(i+1), m.HunkID, m.Line, m.EndLine, true); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("diagram kind must be flow or sequence")
	}
	return nil
}

// CheckLines validates a link to lines line..end (end optional) of a hunk:
// both ends must be new-side lines that hunk shows.
func CheckLines(what, hunk string, line, end int, lineOK func(string, int) bool) error {
	if line == 0 && end == 0 {
		return nil
	}
	if line <= 0 || (end != 0 && end < line) {
		return fmt.Errorf("%s: line must be positive and end_line at least line", what)
	}
	if lineOK == nil {
		return nil
	}
	for _, n := range []int{line, end} {
		if n != 0 && !lineOK(hunk, n) {
			return fmt.Errorf("%s links line %d, which is not a new-side line of %s; use lines shown in that hunk", what, n, hunk)
		}
	}
	return nil
}
func CloneDiagram(d Diagram) Diagram {
	d.Groups = append([]DiagramGroup(nil), d.Groups...)
	d.Nodes = append([]DiagramNode(nil), d.Nodes...)
	d.Edges = append([]DiagramEdge(nil), d.Edges...)
	d.Participants = append([]DiagramNode(nil), d.Participants...)
	d.Messages = append([]DiagramMessage(nil), d.Messages...)
	return d
}

// DiagramChanges lists what a reviewer changed in meaning: labels, nodes,
// participants, connections and messages. Layout is not a change.
func DiagramChanges(before, after Diagram) []string {
	var out []string
	nodes := func(kind string, before, after []DiagramNode) {
		old := map[string]DiagramNode{}
		next := map[string]DiagramNode{}
		for _, n := range before {
			old[n.ID] = n
		}
		for _, n := range after {
			next[n.ID] = n
		}
		for _, n := range before {
			if m, ok := next[n.ID]; !ok {
				out = append(out, "Remove "+kind+": "+n.Label)
			} else if m.Label != n.Label {
				out = append(out, "Rename "+kind+": "+n.Label+" → "+m.Label)
			}
		}
		for _, n := range after {
			if _, ok := old[n.ID]; !ok {
				out = append(out, "Add "+kind+": "+n.Label)
			}
		}
	}
	nodes("node", before.Nodes, after.Nodes)
	nodes("participant", before.Participants, after.Participants)
	key := func(e DiagramEdge) string { return e.From + "\x00" + e.To + "\x00" + e.Label }
	edges, next := map[string]bool{}, map[string]bool{}
	for _, e := range before.Edges {
		edges[key(e)] = true
	}
	for _, e := range after.Edges {
		next[key(e)] = true
	}
	for _, e := range before.Edges {
		if !next[key(e)] {
			out = append(out, "Remove connection: "+e.From+" → "+e.To)
		}
	}
	for _, e := range after.Edges {
		if !edges[key(e)] {
			out = append(out, "Add connection: "+e.From+" → "+e.To)
		}
	}
	b, a := before.MessageIDs(), after.MessageIDs()
	oldMsg := map[string]DiagramMessage{}
	newMsg := map[string]bool{}
	for _, m := range b.Messages {
		oldMsg[m.ID] = m
	}
	for _, m := range a.Messages {
		newMsg[m.ID] = true
		o, ok := oldMsg[m.ID]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("Add message: %s → %s: %s", m.From, m.To, m.Label))
		case o.From != m.From || o.To != m.To:
			out = append(out, fmt.Sprintf("Reroute message %q: %s → %s", m.Label, m.From, m.To))
		case o.Label != m.Label:
			out = append(out, "Rename message: "+o.Label+" → "+m.Label)
		}
	}
	for _, m := range b.Messages {
		if !newMsg[m.ID] {
			out = append(out, "Remove message: "+m.Label)
		}
	}
	if before.Title != after.Title {
		out = append(out, "Rename diagram: "+after.Title)
	}
	return out
}

func diagramSchema() map[string]any {
	str := map[string]any{"type": "string"}
	link := map[string]any{
		"hunk_id":  map[string]any{"type": "string", "description": "A hunk of this section; clicking the shape shows that code"},
		"line":     map[string]any{"type": "integer", "description": "Optional first new-side line inside that hunk"},
		"end_line": map[string]any{"type": "integer", "description": "Optional last new-side line of the range (inclusive)"},
	}
	node := map[string]any{"type": "object", "properties": merge(link, map[string]any{
		"id": str, "label": str,
		"kind":    map[string]any{"type": "string", "enum": []string{"component", "function", "data", "external", "decision", "actor", "note"}},
		"group":   map[string]any{"type": "string", "description": "Id of a group (lane) this node sits in"},
		"context": map[string]any{"type": "boolean", "description": "true for unchanged code or concepts without a hunk"},
	}), "required": []string{"id", "label"}}
	return map[string]any{"type": "object", "description": "Optional explanatory diagram that links to code. Use kind flow for structure and data flow (nodes, edges, optional groups as lanes) and kind sequence for calls over time between components (participants, messages).", "properties": map[string]any{
		"title":  str,
		"kind":   map[string]any{"type": "string", "enum": []string{"flow", "sequence"}},
		"groups": map[string]any{"type": "array", "maxItems": 6, "items": map[string]any{"type": "object", "properties": map[string]any{"id": str, "label": str}, "required": []string{"id", "label"}}},
		"nodes":  map[string]any{"type": "array", "maxItems": 24, "items": node},
		"edges": map[string]any{"type": "array", "maxItems": 40, "items": map[string]any{"type": "object", "properties": map[string]any{
			"from": str, "to": str, "label": str, "style": map[string]any{"type": "string", "enum": []string{"call", "data", "async"}},
		}, "required": []string{"from", "to"}}},
		"participants": map[string]any{"type": "array", "maxItems": 10, "items": node},
		"messages": map[string]any{"type": "array", "maxItems": 30, "items": map[string]any{"type": "object", "properties": merge(link, map[string]any{
			"from": str, "to": str, "label": str, "reply": map[string]any{"type": "boolean"},
		}), "required": []string{"from", "to", "label"}}},
	}, "required": []string{"title"}}
}
func merge(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func WriteBytes(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, ".diagram-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}

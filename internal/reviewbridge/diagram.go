package reviewbridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Diagram struct {
	Title string        `json:"title"`
	Nodes []DiagramNode `json:"nodes"`
	Edges []DiagramEdge `json:"edges"`
}
type DiagramNode struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	HunkID  string `json:"hunk_id,omitempty"`
	Context bool   `json:"context,omitempty"`
}
type DiagramEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

func ValidateDiagram(d Diagram, allowed map[string]bool) error {
	if strings.TrimSpace(d.Title) == "" || len(d.Title) > 120 || len(d.Nodes) == 0 || len(d.Nodes) > 12 || len(d.Edges) > 24 {
		return fmt.Errorf("diagram needs a title, 1–12 nodes and at most 24 edges")
	}
	known := map[string]bool{}
	for _, n := range d.Nodes {
		if n.ID == "" || len(n.ID) > 80 || known[n.ID] || strings.TrimSpace(n.Label) == "" || len(n.Label) > 120 {
			return fmt.Errorf("diagram node IDs must be unique with bounded labels")
		}
		if !n.Context && !allowed[n.HunkID] {
			return fmt.Errorf("node %s needs a section hunk anchor or context:true", n.ID)
		}
		known[n.ID] = true
	}
	for _, e := range d.Edges {
		if !known[e.From] || !known[e.To] || e.From == e.To || len(e.Label) > 80 {
			return fmt.Errorf("invalid diagram connection")
		}
	}
	return nil
}
func CloneDiagram(d Diagram) Diagram {
	d.Nodes = append([]DiagramNode(nil), d.Nodes...)
	d.Edges = append([]DiagramEdge(nil), d.Edges...)
	return d
}
func DiagramChanges(before, after Diagram) []string {
	var out []string
	old := map[string]DiagramNode{}
	new := map[string]DiagramNode{}
	for _, n := range before.Nodes {
		old[n.ID] = n
	}
	for _, n := range after.Nodes {
		new[n.ID] = n
	}
	for _, n := range before.Nodes {
		next, ok := new[n.ID]
		if !ok {
			out = append(out, "Remove node: "+n.Label)
		} else if n != next {
			out = append(out, "Change node: "+n.Label+" → "+next.Label)
		}
	}
	for _, n := range after.Nodes {
		if _, ok := old[n.ID]; !ok {
			out = append(out, "Add node: "+n.Label)
		}
	}
	key := func(e DiagramEdge) string { return e.From + "\x00" + e.To + "\x00" + e.Label }
	edges := map[string]bool{}
	next := map[string]bool{}
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
	if before.Title != after.Title {
		out = append(out, "Rename diagram: "+after.Title)
	}
	return out
}

func diagramSchema() map[string]any {
	return map[string]any{"type": "object", "description": "Optional explanatory graph, only when it clarifies the change. Anchors refer to this section's hunk IDs.", "properties": map[string]any{
		"title": map[string]any{"type": "string"},
		"nodes": map[string]any{"type": "array", "minItems": 1, "maxItems": 12, "items": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}, "hunk_id": map[string]any{"type": "string"}, "context": map[string]any{"type": "boolean"}}, "required": []string{"id", "label"}}},
		"edges": map[string]any{"type": "array", "maxItems": 24, "items": map[string]any{"type": "object", "properties": map[string]any{"from": map[string]any{"type": "string"}, "to": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}}, "required": []string{"from", "to"}}},
	}, "required": []string{"title", "nodes", "edges"}}
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

package reviewbridge

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/patch"
)

// PlaceFeedback leaves the original anchor untouched. A unique matching hunk or
// line quote can move; ambiguous or missing text is outdated, never resolved.
func PlaceFeedback(item Feedback, snapshot Snapshot) (string, *Anchor) {
	if item.Revision == snapshot.Revision {
		return "current", item.Anchor
	}
	if item.Anchor == nil {
		return "outdated", nil
	}
	var exact, quotes []Anchor
	target := item.Anchor
	for _, h := range snapshot.Hunks {
		if h.Path != target.Path || h.Layer != target.Layer {
			continue
		}
		p := patch.Parse(h.Diff)
		lines := p.Lines()
		newNums, oldNums := p.LineNumbersOfLines()
		for i, line := range lines {
			if len(line.Content) == 0 {
				continue
			}
			n := 0
			if target.Side == "old" && (line.Kind == patch.DELETION || line.Kind == patch.CONTEXT) {
				n = oldNums[i]
			}
			if target.Side == "new" && (line.Kind == patch.ADDITION || line.Kind == patch.CONTEXT) {
				n = newNums[i]
			}
			if n < 1 || strings.ReplaceAll(line.Content[1:], "\t", "    ") != target.Text {
				continue
			}
			found := *target
			found.Line = n
			if target.StartLine > 0 {
				found.StartLine = target.StartLine + n - target.Line
			}
			found.Hunk = h.Fingerprint
			quotes = append(quotes, found)
			if h.Fingerprint != "" && h.Fingerprint == target.Hunk {
				exact = append(exact, found)
			}
		}
	}
	matches := quotes
	if len(exact) > 0 {
		matches = exact
	}
	if len(matches) != 1 {
		return "outdated", nil
	}
	current := matches[0]
	if current.Line == target.Line {
		return "current", &current
	}
	return "moved", &current
}

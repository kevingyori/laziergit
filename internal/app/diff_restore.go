package app

import "github.com/jesseduffield/lazygit/pkg/commands/patch"

// patchLineKey is lazygit's patchLine: what stays the same about a diff
// line when the same diff is rendered again differently. Every content kind
// is one kind, an addition and the context line it becomes when whitespace
// stops counting being the same line of the same file.
type patchLineKey struct {
	path       string
	header     patch.PatchLineKind // PATCH_HEADER or HUNK_HEADER; 0 for content
	line       int
	isDeletion bool
}

func (r diffRow) key() (patchLineKey, bool) {
	if !r.ok {
		return patchLineKey{}, false
	}
	switch r.kind {
	case patch.PATCH_HEADER, patch.HUNK_HEADER:
		return patchLineKey{path: r.path, header: r.kind, line: r.newLine}, true
	case patch.DELETION:
		return patchLineKey{path: r.path, line: r.oldLine, isDeletion: true}, true
	default:
		return patchLineKey{path: r.path, line: r.newLine}, true
	}
}

func indexRows(rows []diffRow) map[patchLineKey]int {
	m := map[patchLineKey]int{}
	for i, r := range rows {
		if k, ok := r.key(); ok {
			if _, seen := m[k]; !seen {
				m[k] = i
			}
		}
	}
	return m
}

// nearbyRows is lazygit's nearbyDiffLines order: the anchor, then outward,
// at-or-below first on ties.
func nearbyRows(n, anchor int, visit func(i int) bool) {
	if anchor < 0 || anchor >= n {
		return
	}
	if visit(anchor) {
		return
	}
	for below, above := anchor+1, anchor-1; below < n || above >= 0; below, above = below+1, above-1 {
		if below < n && visit(below) {
			return
		}
		if above >= 0 && visit(above) {
			return
		}
	}
}

// restoreOrigin is lazygit's PreserveDiffPositionOnRerender for the view's
// position: the nearest line to anchor that the new rendering still has goes
// back on the screen row it was on (clamped into the view). ok is false when
// none survives, and the view keeps its offset.
func restoreOrigin(old, rows []diffRow, oy, height, anchor int) (int, bool) {
	index := indexRows(rows)
	origin, ok := 0, false
	nearbyRows(len(old), anchor, func(i int) bool {
		k, has := old[i].key()
		if !has {
			return false
		}
		n, found := index[k]
		if !found {
			return false
		}
		row := i - oy // lines above the view go to its top, those below to its bottom
		row = min(max(row, 0), max(0, height-1))
		origin, ok = max(0, n-row), true
		return true
	})
	return origin, ok
}

// restoreLine is the line of the new rendering the old line i is on, or the
// nearest surviving one, as lazygit's selectionLine finds a cursor again.
func restoreLine(old, rows []diffRow, i int) (int, bool) {
	index := indexRows(rows)
	line, ok := 0, false
	nearbyRows(len(old), i, func(j int) bool {
		k, has := old[j].key()
		if !has {
			return false
		}
		if n, found := index[k]; found {
			line, ok = n, true
			return true
		}
		return false
	})
	return line, ok
}

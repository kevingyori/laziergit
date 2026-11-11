package app

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"

	"laziergit/internal/repo"
)

// The identity of each row of a renderer's output, as lazygit's
// diff_line_parser.go recovers it: a renderer that states OSC 1717 records
// for any row is read by its records alone; any other rendering is parsed as
// a unified diff (parseDiffRows), which works while it keeps a diff's
// structure.

// renderedDiff is a diff as a renderer produced it.
type renderedDiff struct {
	lines    []ansiLine
	texts    []string
	rows     []diffRow
	alts     [][]diffRow // every diff line a row shows, when it shows more than one
	sections []diffSection
	byRecord bool // the rows were placed by the renderer's records
}

// root is the worktree, against which a record's absolute path is made
// repo-relative (lazygit's diffLineInfoIn and repoRelativePath).
func parseRenderedDiff(text, root string) renderedDiff {
	lines := parseANSI(text)
	d := renderedDiff{lines: lines, texts: make([]string, len(lines))}
	for i, l := range lines {
		d.texts[i] = l.text
	}
	for _, l := range lines {
		for _, rec := range l.records {
			if _, ok := parseDiffLineRecord(rec); ok {
				d.byRecord = true
			}
		}
	}
	if !d.byRecord {
		_, d.rows, d.sections = parseDiffRows(strings.Join(d.texts, "\n"))
		if len(d.rows) < len(lines) { // a trailing empty row
			d.rows = append(d.rows, make([]diffRow, len(lines)-len(d.rows))...)
		}
		return d
	}
	d.rows = make([]diffRow, len(lines))
	d.alts = make([][]diffRow, len(lines))
	for i, l := range lines {
		d.rows[i] = diffRow{section: -1, kind: patch.PATCH_HEADER}
		for _, rec := range l.records {
			if r, ok := parseDiffLineRecord(rec); ok {
				if filepath.IsAbs(r.path) {
					rel, err := filepath.Rel(root, r.path)
					if err != nil || strings.HasPrefix(rel, "..") {
						continue // no file of this repo
					}
					r.path = filepath.ToSlash(rel)
				}
				d.alts[i] = append(d.alts[i], r)
			}
		}
		if len(d.alts[i]) > 0 {
			d.rows[i] = d.alts[i][0]
		}
	}
	return d
}

// parseDiffLineRecord is lazygit's parseDiffLineMetadata:
// version;type;new-line;old-line;file.
func parseDiffLineRecord(payload string) (diffRow, bool) {
	f := strings.SplitN(payload, ";", 5)
	if len(f) < 5 || f[0] != "1" {
		return diffRow{}, false
	}
	var kind patch.PatchLineKind
	switch f[1] {
	case "c":
		kind = patch.CONTEXT
	case "a":
		kind = patch.ADDITION
	case "d":
		kind = patch.DELETION
	case "f":
		kind = patch.PATCH_HEADER
	case "h":
		kind = patch.HUNK_HEADER
	default:
		return diffRow{}, false
	}
	newLine := 0
	if f[2] != "" {
		n, err := strconv.Atoi(f[2])
		if err != nil {
			return diffRow{}, false
		}
		newLine = n
	} else if kind != patch.PATCH_HEADER {
		return diffRow{}, false
	}
	oldLine := 0
	if f[3] != "" {
		n, err := strconv.Atoi(f[3])
		if err != nil {
			return diffRow{}, false
		}
		oldLine = n
	}
	return diffRow{ok: true, path: f[4], kind: kind, newLine: newLine, oldLine: oldLine, section: -1}, true
}

// diffLinesOf is the unfocused pane's rows of a loaded text: git's plain diff
// drawn natively, or a renderer's output drawn as it rendered it.
func diffLinesOf(text string, rendered bool) []diffLine {
	if !rendered {
		return parseDiff(text)
	}
	lines := parseANSI(text)
	out := make([]diffLine, len(lines))
	for i, l := range lines {
		out[i] = diffLine{kind: lineAnsi, text: strings.ReplaceAll(l.text, "\t", "    "), src: i}
	}
	return out
}

// diffRowsOf is the identity of each row of a loaded text.
func diffRowsOf(text string, rendered bool, root string) ([]diffRow, []diffSection) {
	if !rendered {
		_, rows, sections := parseDiffRows(text)
		return rows, sections
	}
	d := parseRenderedDiff(text, root)
	return d.rows, d.sections
}

// diffLoad is the load of a diff for pane: git's plain diff (plain) while no
// renderer is configured; otherwise build's command run through the current
// renderer (lazygit's newRenderTask), or, in a focused pane whose renderer's
// rows can't be placed, git's own coloured diff (MainViewDiffMode).
func (a *App) diffLoad(pane string, plain func() string, prefix string, build repo.DiffCmd) func() string {
	r := a.repo
	if !r.RenderersConfigured() {
		return plain
	}
	view := repo.RenderView{Width: a.paneInnerWidth(pane), Light: a.light}
	focused := a.staging != nil || a.mainFocused
	return func() string {
		mode := git_commands.DiffModeRendered
		if focused {
			mode = r.FocusedDiffMode()
		}
		return prefix + r.Render(build, mode, view)
	}
}

// ansiCache keeps the parsed rows of the last text drawn in a pane.
type ansiCache struct {
	text  string
	lines []ansiLine
}

func (c *ansiCache) get(text string) []ansiLine {
	if c.lines == nil || c.text != text {
		c.text, c.lines = text, parseANSI(text)
	}
	return c.lines
}

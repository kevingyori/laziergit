package app

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/patch"
)

// The rows of a diff shown in the focused main view, and what each one is a
// line of: lazygit's diff_line_parser.go, for git's own unified diff. A
// (possibly multi-file) diff is cut into file sections at their "diff --git"
// headers; each section is parsed with lazygit's patch parser, whose line
// indices line up with the section's rows. Rows outside every section (a
// commit's message, its diffstat) have no identity.

// diffRow is one row of the focused diff.
type diffRow struct {
	ok      bool // the row's identity is known
	path    string
	kind    patch.PatchLineKind
	newLine int
	oldLine int
	section int // index into the sections, -1 outside every section
}

func (r diffRow) isChange() bool {
	return r.ok && (r.kind == patch.ADDITION || r.kind == patch.DELETION)
}

// lineIdentity is lazygit's DiffLineInfo.PatchLineIdentity.
func (r diffRow) lineIdentity() patch.LineIdentity {
	if r.kind == patch.DELETION {
		return patch.LineIdentity{LineNumber: r.oldLine, IsDeletion: true}
	}
	return patch.LineIdentity{LineNumber: r.newLine}
}

// diffSection is one file's part of a diff: rows [start, end).
type diffSection struct {
	path       string
	start, end int
	patch      *patch.Patch
}

const diffFilePrefix = "diff --git "

var submoduleSectionPattern = regexp.MustCompile(
	`^Submodule (.+) (?:contains (?:untracked|modified) content|[0-9a-f]+\.{2,3}[0-9a-f]+(?: \(.*\))?:?)$`)

// parseDiffRows splits text into rows and works out each row's identity.
func parseDiffRows(text string) ([]string, []diffRow, []diffSection) {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil, nil, nil
	}
	lines := strings.Split(text, "\n")
	rows := make([]diffRow, len(lines))
	for i := range rows {
		rows[i] = diffRow{section: -1, kind: patch.PATCH_HEADER}
	}
	var sections []diffSection
	for i := 0; i < len(lines); {
		if !startsFileSection(lines[i]) {
			i++
			continue
		}
		end := fileSectionEnd(lines, i)
		if sec, ok := parseFileSection(lines[i:end], end == len(lines)); ok {
			sec.start, sec.end = i, end
			n := len(sections)
			sections = append(sections, sec)
			plines := sec.patch.Lines()
			newNums, oldNums := sec.patch.LineNumbersOfLines()
			for j := i; j < end && j-i < len(plines); j++ {
				k := j - i
				r := diffRow{ok: true, path: sec.path, kind: plines[k].Kind, newLine: newNums[k], section: n}
				if r.kind == patch.DELETION {
					r.oldLine = oldNums[k]
				}
				rows[j] = r
			}
		}
		i = end
	}
	return lines, rows, sections
}

func startsFileSection(line string) bool {
	return strings.HasPrefix(line, diffFilePrefix) || submodulePath(line) != ""
}

func fileSectionEnd(lines []string, start int) int {
	if submodulePath(lines[start]) != "" {
		end := start + 1
		for end < len(lines) && (strings.HasPrefix(lines[end], "  > ") || strings.HasPrefix(lines[end], "  < ")) {
			end++
		}
		return end
	}
	for i := start + 1; i < len(lines); i++ {
		if startsFileSection(lines[i]) {
			return i
		}
	}
	return len(lines)
}

func submodulePath(line string) string {
	if !strings.HasPrefix(line, "Submodule ") {
		return ""
	}
	if m := submoduleSectionPattern.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

func parseFileSection(lines []string, endsTheBuffer bool) (diffSection, bool) {
	path := pathFromDiffHeader(lines)
	if path == "" {
		return diffSection{}, false
	}
	p := patch.Parse(strings.Join(lines, "\n"))
	wellFormed := p.IsWellFormed
	if endsTheBuffer {
		wellFormed = p.IsWellFormedSoFar
	}
	if !wellFormed() {
		return diffSection{}, false
	}
	return diffSection{path: path, patch: p}, true
}

func pathFromDiffHeader(lines []string) string {
	if p := submodulePath(lines[0]); p != "" {
		return p
	}
	var oldPath, newPath, renamed string
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			break
		}
		switch {
		case strings.HasPrefix(line, "+++ "):
			newPath = pathFromDiffHeaderField(strings.TrimPrefix(line, "+++ "))
		case strings.HasPrefix(line, "--- "):
			oldPath = pathFromDiffHeaderField(strings.TrimPrefix(line, "--- "))
		case strings.HasPrefix(line, "rename to "):
			renamed, _ = unquoteDiffPath(strings.TrimPrefix(line, "rename to "))
		case strings.HasPrefix(line, "copy to "):
			renamed, _ = unquoteDiffPath(strings.TrimPrefix(line, "copy to "))
		}
	}
	switch {
	case newPath != "" && newPath != "/dev/null":
		return newPath
	case oldPath != "" && oldPath != "/dev/null":
		return oldPath
	case renamed != "":
		return renamed
	}
	return pathFromDiffGitLine(lines[0])
}

func pathFromDiffHeaderField(field string) string {
	path, ok := unquoteDiffPath(field)
	if !ok {
		return ""
	}
	if path == "/dev/null" {
		return path
	}
	_, rest, found := strings.Cut(path, "/")
	if !found {
		return ""
	}
	return rest
}

func unquoteDiffPath(field string) (string, bool) {
	field = strings.TrimSuffix(field, "\t")
	if strings.HasPrefix(field, `"`) {
		s, err := strconv.Unquote(field)
		return s, err == nil
	}
	return field, true
}

func pathFromDiffGitLine(line string) string {
	rest := strings.TrimPrefix(line, diffFilePrefix)
	for i, c := range rest {
		if c != ' ' {
			continue
		}
		p := pathFromDiffHeaderField(rest[i+1:])
		if p != "" && p == pathFromDiffHeaderField(rest[:i]) {
			return p
		}
	}
	return ""
}

// changeBlockStart is lazygit's: the first line of the change block next to
// from in the given direction.
func changeBlockStart(isChange []bool, from int, forward bool) (int, bool) {
	if from < 0 || from >= len(isChange) {
		return 0, false
	}
	i := from
	if forward {
		for i < len(isChange) && isChange[i] {
			i++
		}
		for i < len(isChange) && !isChange[i] {
			i++
		}
		return i, i < len(isChange)
	}
	for i >= 0 && isChange[i] {
		i--
	}
	for i >= 0 && !isChange[i] {
		i--
	}
	if i < 0 {
		return 0, false
	}
	for i > 0 && isChange[i-1] {
		i--
	}
	return i, true
}

type diffFileStart struct {
	path string
	row  int
}

// fileStarts is where each file of a diff begins, in order.
func fileStarts(rows []diffRow) []diffFileStart {
	var starts []diffFileStart
	prev := ""
	for i, r := range rows {
		if !r.ok || r.path == prev {
			continue
		}
		prev = r.path
		starts = append(starts, diffFileStart{r.path, i})
	}
	return starts
}

// adjacentFileStart is lazygit's fileStart.
func adjacentFileStart(rows []diffRow, from int, forward bool) (int, bool) {
	if from < 0 {
		return 0, false
	}
	anchor := ""
	for i := from; i < len(rows) && anchor == ""; i++ {
		if rows[i].ok {
			anchor = rows[i].path
		}
	}
	for i := min(from, len(rows)) - 1; i >= 0 && anchor == ""; i-- {
		if rows[i].ok {
			anchor = rows[i].path
		}
	}
	if anchor == "" {
		return 0, false
	}
	starts := fileStarts(rows)
	for k, s := range starts {
		if s.path != anchor {
			continue
		}
		t := k + 1
		if !forward {
			t = k - 1
		}
		if t < 0 || t >= len(starts) {
			return 0, false
		}
		return starts[t].row, true
	}
	return 0, false
}

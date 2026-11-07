package app

import (
	"regexp"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/utils"
)

// Diff-stat links, lazygit's DiffStatLinkWriter: the names of the files in
// the diffstat a diff opens with link to where their diff begins. Clicking
// one jumps there (JumpToFileNamedInDiffStat); the keyboard equivalent is
// the jump-to-file menu.

var diffStatEntryPattern = regexp.MustCompile(`^ (.*[^ ]) +\| +(?:Bin|\d+)`)

// diffStatLinks maps each diffstat line of text to its file name's byte
// range: the entries from the first, up to the first line that is none or
// where the diff itself begins.
func diffStatLinks(text string) map[int][2]int {
	var out map[int][2]int
	in := false
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "diff --") {
			return out
		}
		m := []int(nil)
		if strings.HasPrefix(line, " ") && strings.Contains(line, "|") {
			m = diffStatEntryPattern.FindStringSubmatchIndex(line)
		}
		if m == nil {
			if in {
				return out
			}
			continue
		}
		in = true
		if out == nil {
			out = map[int][2]int{}
		}
		out[i] = [2]int{m[2], m[3]}
	}
	return out
}

// statLinkCache keeps the links of the last text asked about, with the
// text of each row they were found in.
type statLinkCache struct {
	text  string
	links map[int][2]int
	lines []string
}

// mainStatLinks are the links of the unfocused main view's diffstat, with
// the text of each row: only over a diff of the panel's whose rows can be
// placed (lazygit's DiffRowsCanBePlaced: git's own diff, or a renderer
// that states its rows).
func (a *App) mainStatLinks() (map[int][2]int, []string) {
	if key, _ := a.mainSource(); !a.diffPlaceable || !(isDiffSource(key) || strings.HasPrefix(key, "range:")) {
		return nil, nil
	}
	c := &a.statLinks
	if c.lines != nil && c.text == a.diffText {
		return c.links, c.lines
	}
	texts := strings.Split(a.diffText, "\n")
	if a.diffRendered {
		ansi := a.mainAnsi.get(a.diffText)
		texts = make([]string, len(ansi))
		for i, l := range ansi {
			texts[i] = l.text
		}
	}
	c.text, c.links, c.lines = a.diffText, diffStatLinks(strings.Join(texts, "\n")), texts
	return c.links, c.lines
}

// statLinkCols is the link of line src in lazygit's columns (display
// width), and the parts of text before, in and after it; ok is false when
// text doesn't show the linked name where the loaded line has it.
func statLinkCols(links map[int][2]int, src int, raw, text string) (from, to int, parts [3]string, ok bool) {
	r, found := links[src]
	if !found || r[1] > len(raw) || strings.ReplaceAll(raw, "\t", "    ") != text {
		return 0, 0, parts, false
	}
	parts = [3]string{raw[:r[0]], raw[r[0]:r[1]], raw[r[1]:]}
	from = utils.StringWidth(parts[0])
	return from, from + utils.StringWidth(parts[1]), parts, true
}

// jumpToDiffStatEntry is lazygit's JumpToFileNamedInDiffStat: the diff
// goes to the file the entry names, or a toast says there's none.
func (a *App) jumpToDiffStatEntry(focused bool, entry string) {
	var sections []diffSection
	if focused {
		if a.staging == nil {
			return
		}
		sections = a.staging.sections
	} else {
		_, sections = diffRowsOf(a.diffText, a.diffRendered, a.repo.Git.RepoPaths.WorktreePath())
	}
	names := make([]string, len(sections))
	for i, s := range sections {
		names[i] = s.path
	}
	i, ok := fileNamedByDiffStatEntry(entry, names)
	if !ok {
		a.setStatus(utils.ResolvePlaceholderString(a.repo.Common.Tr.NoFileInDiffNamed, map[string]string{"path": entry}), true)
		return
	}
	a.jumpToSection(focused, sections[i].start)
}

func fileNamedByDiffStatEntry(entry string, paths []string) (int, bool) {
	name := renamedTo(strings.TrimSpace(entry))
	if i, ok := theOneMatching(paths, func(p string) bool { return p == name }); ok {
		return i, true
	}
	tail := strings.TrimPrefix(name, "...")
	return theOneMatching(paths, func(p string) bool { return strings.HasSuffix(p, tail) })
}

func renamedTo(entry string) string {
	const arrow = " => "
	at := strings.Index(entry, arrow)
	if at == -1 {
		return entry
	}
	shared := ""
	if brace := strings.Index(entry[:at], "{"); brace != -1 {
		shared = entry[:brace]
	}
	renamed := entry[at+len(arrow):]
	if closing := strings.Index(renamed, "}"); closing != -1 {
		return shared + renamed[:closing] + renamed[closing+1:]
	}
	return shared + renamed
}

func theOneMatching(paths []string, matches func(string) bool) (int, bool) {
	found := -1
	for i, p := range paths {
		if !matches(p) {
			continue
		}
		if found != -1 {
			return 0, false
		}
		found = i
	}
	return found, found != -1
}

// splitSpans cuts styled spans at byte offsets from and to of their text,
// for a link drawn over part of a rendered row.
func splitSpans(spans []ansiSpan, from, to int) [3][]ansiSpan {
	var out [3][]ansiSpan
	at := 0
	for _, sp := range spans {
		for len(sp.text) > 0 {
			part, end := 0, from
			switch {
			case at >= to:
				part, end = 2, at+len(sp.text)
			case at >= from:
				part, end = 1, to
			}
			n := min(len(sp.text), end-at)
			if n <= 0 {
				n = len(sp.text)
			}
			out[part] = append(out[part], ansiSpan{text: sp.text[:n], style: sp.style})
			sp.text = sp.text[n:]
			at += n
		}
	}
	return out
}

// statLinkMap is the focused diff's diffstat links over the text of its
// rows (a focused diff's rows can always be placed: lazygit renders it raw
// otherwise).
func (s *stagingState) statLinkMap() (map[int][2]int, []string) {
	c := &s.statLinks
	if c.lines != nil && c.text == s.text {
		return c.links, c.lines
	}
	texts := make([]string, len(s.lines))
	for i, l := range s.lines {
		texts[i] = l.Content
	}
	for i, l := range s.ansi {
		if i < len(texts) {
			texts[i] = l.text
		}
	}
	c.text, c.links, c.lines = s.text, diffStatLinks(strings.Join(texts, "\n")), texts
	return c.links, c.lines
}

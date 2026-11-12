package reviewbridge

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var identifierTitle = regexp.MustCompile(`^[A-Za-z0-9_.()]+$`)
var homework = regexp.MustCompile(`(?i)^\s*(check|ensure|verify|make sure|consider|confirm)\b`)

// maxProblems bounds one rejection: enough to fix a guide in one round trip
// without burying a weak agent.
const maxProblems = 10

// titleWords is the limit the instructions and schema state. An agent in the
// eval found the stated limit wasn't enforced; what is promised is checked.
const titleWords = 10

// Validate reports whether a guide fits its snapshot. The error lists every
// problem, each with where it is and how to fix it, so an agent can correct
// them all before publishing again.
func Validate(s Snapshot, g Guide) error {
	problems := Problems(s, g)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", rejection(problems))
}

func rejection(problems []string) string {
	var b strings.Builder
	n := len(problems)
	if n == 1 {
		b.WriteString("Guide not published: 1 problem. Fix it and call publish_review_guide again.\n")
	} else {
		fmt.Fprintf(&b, "Guide not published: %d problems. Fix all of them, then call publish_review_guide again.\n", n)
	}
	for i, p := range problems {
		if i == maxProblems {
			fmt.Fprintf(&b, "…and %d more of the same kinds.\n", n-maxProblems)
			break
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, p)
	}
	return strings.TrimRight(b.String(), "\n")
}

func short(rev string) string {
	if len(rev) > 12 {
		return rev[:12] + "…"
	}
	return rev
}

func quoteList(ids []string) string {
	if len(ids) > 24 {
		return strings.Join(ids[:24], ", ") + fmt.Sprintf(", … (%d in all)", len(ids))
	}
	return strings.Join(ids, ", ")
}

// lineRange names the new-side lines a hunk shows, for line errors.
func lineRange(diff string) string {
	lines := newSideLines(diff)
	if len(lines) == 0 {
		return "it has no new-side lines (a pure deletion): leave line out"
	}
	lo, hi := 0, 0
	for n := range lines {
		if lo == 0 || n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	return fmt.Sprintf("it shows new-side lines %d-%d, numbered in read_hunks' left column", lo, hi)
}

// Problems lists everything that keeps a guide from being shown. A stale or
// missing revision is reported alone: nothing else can be judged against a
// snapshot the agent hasn't read.
func Problems(s Snapshot, g Guide) []string {
	switch {
	case strings.TrimSpace(g.Revision) == "":
		return []string{`revision is missing. Add "revision" at the top level of the arguments (next to "sections", not inside "overview"), copied from get_review_diff.`}
	case g.Revision != s.Revision:
		return []string{fmt.Sprintf("revision %s is not the current snapshot (%s): the reviewer reloaded and hunks may have changed. Call get_review_diff again, rebuild hunk_ids from its index, and publish with the new revision.", short(g.Revision), short(s.Revision))}
	}
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }
	known := map[string]bool{}
	diffs := map[string]string{}
	paths := map[string]string{}
	var all []string
	for _, h := range s.Hunks {
		known[h.ID] = true
		diffs[h.ID] = h.Diff
		paths[h.ID] = h.Path
		all = append(all, h.ID)
	}
	lineOK := func(h string, line int) bool { return newSideLines(diffs[h])[line] }
	if g.Provenance != "" && g.Provenance != "author" && g.Provenance != "inferred" {
		add(`provenance %q: use "author" if you made this change, "inferred" if you are reading it cold.`, g.Provenance)
	}
	if g.Overview != nil {
		switch {
		case strings.TrimSpace(g.Overview.Purpose) == "":
			add("overview.purpose is empty: say in at most two sentences what this change does for a user or caller, and why.")
		case len(g.Overview.Purpose) > 2000:
			add("overview.purpose is %d characters: keep it to two sentences (2000 characters at most).", len(g.Overview.Purpose))
		}
		if len(g.Overview.NotDone) > 1000 {
			add("overview.not_done is %d characters: keep it under 1000.", len(g.Overview.NotDone))
		}
		if err := ValidateMedia(s, g.Overview.Media); err != nil {
			add("overview.%v", err)
		}
	}
	if len(g.Sections) == 0 {
		add(`sections is empty or missing. Add "sections" at the top level (not inside "overview"): 1-6 sections that together hold every hunk id exactly once.`)
	}
	seen := map[string]int{}
	questions := 0
	for i, section := range g.Sections {
		at := fmt.Sprintf("sections[%d]", i+1)
		if t := strings.TrimSpace(section.Title); t != "" {
			at += fmt.Sprintf(" %q", t)
		}
		switch {
		case strings.TrimSpace(section.Title) == "":
			add("%s.title is missing: name the outcome in at most %d words.", at, titleWords)
		case len(section.Title) > 90 || len(strings.Fields(section.Title)) > titleWords:
			add("%s.title is %d words: name the outcome in at most %d words (90 characters).", at, len(strings.Fields(section.Title)), titleWords)
		case identifierTitle.MatchString(section.Title):
			add("%s.title is an identifier: name the outcome a user or caller notices, in a few words.", at)
		}
		if strings.TrimSpace(section.Summary) == "" && strings.TrimSpace(section.Why) == "" {
			add("%s.why is missing: one sentence on the purpose for a user or caller.", at)
		}
		if len(section.Why) > 1000 {
			add("%s.why is %d characters: one sentence, 1000 characters at most. Move mechanism into how.", at, len(section.Why))
		}
		if len(section.How) > 3000 {
			add("%s.how is %d characters: a short paragraph or list, 3000 characters at most.", at, len(section.How))
		}
		if len(section.Summary) > 1000 {
			add("%s.summary is %d characters: 1000 at most (prefer why and how).", at, len(section.Summary))
		}
		if err := ValidateMedia(s, section.Media); err != nil {
			add("%s.%v", at, err)
		}
		if section.Kind != "" && section.Kind != "core" && section.Kind != "supporting" && section.Kind != "mechanical" {
			add(`%s.kind %q: use "core", "supporting" or "mechanical".`, at, section.Kind)
		}
		allowed := map[string]bool{}
		if len(section.IDs) == 0 {
			add("%s.hunk_ids is empty: list the hunk ids this section explains, or drop the section.", at)
		}
		for _, id := range section.IDs {
			allowed[id] = true
			switch {
			case !known[id]:
				add("%s.hunk_ids: %q is not a hunk of this review. The ids are: %s.", at, id, quoteList(all))
			case seen[id] == i+1:
				add("%s.hunk_ids lists %q twice: list it once.", at, id)
			case seen[id] != 0:
				add("%q is in sections[%d] and sections[%d]: a hunk belongs to exactly one section, so remove it from one.", id, seen[id], i+1)
			}
			if known[id] && seen[id] == 0 {
				seen[id] = i + 1
			}
		}
		if len(section.Notes) > 3 {
			add("%s has %d notes: keep the 3 that matter most for the commit-or-send-back decision.", at, len(section.Notes))
		}
		sectionQuestions := 0
		for j, n := range section.Notes {
			nat := fmt.Sprintf("%s.notes[%d]", at, j+1)
			inSection := allowed[n.HunkID]
			switch {
			case n.HunkID == "":
				add("%s.hunk_id is missing: anchor the note to one of this section's hunks (%s).", nat, quoteList(section.IDs))
			case !inSection && known[n.HunkID]:
				add("%s.hunk_id %q is not in this section's hunk_ids (%s): move the note to the section that holds %s, or anchor it to one of these.", nat, n.HunkID, quoteList(section.IDs), n.HunkID)
			case !inSection:
				add("%s.hunk_id %q is not a hunk of this review: use one of this section's hunk_ids (%s).", nat, n.HunkID, quoteList(section.IDs))
			}
			if n.Kind != "explain" && n.Kind != "assumption" && n.Kind != "question" {
				add(`%s.kind %q: use "explain", "assumption" or "question".`, nat, n.Kind)
			}
			switch {
			case strings.TrimSpace(n.Text) == "":
				add("%s.text is missing.", nat)
			case len(n.Text) > 320:
				add("%s.text is %d characters: at most 320 (about 40 words). Keep the finding, cut the restating.", nat, len(n.Text))
			case homework.MatchString(n.Text):
				add("%s.text %q hands work to the reviewer. Look it up with read_file or search and state what you found; if the code cannot tell you, make it a question that says what is unknown.", nat, n.Text)
			}
			if (n.Kind == "assumption" || n.Kind == "question") && strings.TrimSpace(n.Evidence) == "" {
				add(`%s is a %s without evidence: add "evidence" with the path:line or tool result that backs it.`, nat, n.Kind)
			}
			if inSection {
				if err := CheckLines(nat, n.HunkID, n.Line, n.EndLine, lineOK); err != nil {
					add("%v", lineProblem(err, diffs[n.HunkID]))
				}
			}
			if n.Kind == "question" {
				sectionQuestions++
				questions++
			}
		}
		if sectionQuestions > 1 {
			add(`%s has %d questions: at most one per section. Keep the one that matters most; answer the others from the code and make them "explain" or "assumption".`, at, sectionQuestions)
		}
		if section.Diagram != nil {
			if err := ValidateDiagram(*section.Diagram, allowed, lineOK); err != nil {
				add("%s.diagram: %v", at, err)
			}
		}
	}
	if questions > 3 {
		add(`The guide has %d questions: at most three. Keep the ones that decide commit-or-send-back; answer the rest from the code.`, questions)
	}
	var missing []string
	for _, id := range all {
		if seen[id] == 0 {
			missing = append(missing, fmt.Sprintf("%s (%s)", id, paths[id]))
		}
	}
	if len(missing) > 0 && len(g.Sections) > 0 {
		sort.Strings(missing)
		add(`These hunks are in no section: %s. Add each to the hunk_ids of the section it supports; renames, formatting and generated files can share one "mechanical" section.`, quoteList(missing))
	}
	return out
}

// lineProblem adds the hunk's real line range to a CheckLines error.
func lineProblem(err error, diff string) string {
	msg := err.Error()
	if strings.Contains(msg, "not a new-side line") {
		if i := strings.Index(msg, "; use lines"); i >= 0 {
			msg = msg[:i]
		}
		return msg + ": " + lineRange(diff) + "."
	}
	return msg
}

// guideFields names what each level of the publish arguments accepts, so a
// misplaced field is reported with where it belongs instead of being dropped.
var guideFields = map[string][]string{
	"":         {"revision", "provenance", "overview", "sections"},
	"overview": {"purpose", "not_done", "media"},
	"section":  {"title", "kind", "why", "how", "summary", "hunk_ids", "notes", "media", "diagram"},
	"note":     {"hunk_id", "line", "end_line", "kind", "text", "evidence"},
}

// Misplaced reports fields the guide's structure doesn't have. Weak agents
// nest top-level fields inside overview or sections; a silent drop would turn
// that into a misleading "revision is missing".
func Misplaced(raw json.RawMessage) []string {
	var out []string
	var walk func(raw json.RawMessage, level, at string)
	walk = func(raw json.RawMessage, level, at string) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil {
			return
		}
		names := make([]string, 0, len(fields))
		for name := range fields {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			path := name
			if at != "" {
				path = at + "." + name
			}
			if !contains(guideFields[level], name) {
				where := ""
				for _, l := range []string{"", "section", "note", "overview"} {
					if l != level && contains(guideFields[l], name) {
						where = map[string]string{"": "the top level of the arguments", "section": "each item of sections", "note": "each item of a section's notes", "overview": "overview"}[l]
						break
					}
				}
				if where != "" {
					out = append(out, fmt.Sprintf("%s is in the wrong place: %q belongs at %s. Move it there.", path, name, where))
				} else {
					out = append(out, fmt.Sprintf("%s is not a field here. This level takes: %s.", path, strings.Join(guideFields[level], ", ")))
				}
				continue
			}
			child := map[string]string{"overview": "overview", "sections": "section", "notes": "note"}[name]
			if child == "" || (name == "overview" && level != "") || (name == "sections" && level != "") || (name == "notes" && level != "section") {
				continue
			}
			var items []json.RawMessage
			if json.Unmarshal(fields[name], &items) == nil {
				for i, item := range items {
					walk(item, child, fmt.Sprintf("%s[%d]", path, i+1))
				}
			} else {
				walk(fields[name], child, path)
			}
		}
	}
	walk(raw, "", "")
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ParseGuide decodes publish arguments, reporting misplaced fields and wrong
// types as problems an agent can fix.
func ParseGuide(raw json.RawMessage) (Guide, []string) {
	var g Guide
	if problems := Misplaced(raw); len(problems) > 0 {
		return g, problems
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		msg := err.Error()
		if te, ok := err.(*json.UnmarshalTypeError); ok {
			msg = fmt.Sprintf("%s must be %s, not %s", te.Field, jsonKind(te.Type.Kind().String()), te.Value)
		}
		return g, []string{"The arguments don't match the schema: " + msg + "."}
	}
	return g, nil
}

func jsonKind(kind string) string {
	switch kind {
	case "slice":
		return "an array"
	case "struct", "ptr", "map":
		return "an object"
	case "int":
		return "an integer"
	case "bool":
		return "true or false"
	}
	return "a " + kind
}

package app

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"github.com/samber/lo"
)

// Copying the focused diff's selection, as lazygit's MainViewController.copySelection
// and DiffLineHelper.PlainDiffOfSelection do it: what a pane shows may be a renderer's
// picture of a diff, so the selected lines are looked up by identity in git's plain
// diff, and the rows above the diff are copied as they read on screen.

// identitiesAt is every diff line row i shows.
func (s *stagingState) identitiesAt(i int) []diffRow {
	if i < len(s.alts) && len(s.alts[i]) > 0 {
		return s.alts[i]
	}
	if i < len(s.rows) && s.rows[i].ok {
		return []diffRow{s.rows[i]}
	}
	return nil
}

// plainDiffFor is the diff behind what pane s shows, for the given files
// (lazygit's FocusedMainViewDiffSource.PlainDiff). Run it off the main thread.
func (a *App) plainDiffFor(s *stagingState) func(paths []string) string {
	r := a.repo
	switch {
	case s.worktree:
		n := a.selectedFileNode()
		if n == nil {
			return nil
		}
		staged := s.staged
		return func(paths []string) string {
			out, _ := r.Git.WorkingTree.WorktreeFileDiffCmdObj(n, git_commands.DiffModePlain, staged, paths).DontLog().RunWithOutput()
			return out
		}
	case s.staged:
		return func([]string) string {
			var out string
			_ = r.EditPatch(func(p *git_commands.PatchCommands) error {
				if dir := p.PatchBuilder.TempDir(); dir != "" {
					out, _ = r.Git.Diff.CustomPatchDiffCmdObj(dir, git_commands.DiffModePlain).DontLog().RunWithOutput()
				}
				return nil
			})
			return out
		}
	}
	var from, to string
	reverse := false
	if a.commitFiles != nil && a.focus == panelCommits {
		from, to, reverse = a.commitFileRefs()
	} else {
		from, to, _ = a.patchTarget()
		from, reverse = a.diffingFrom(from)
	}
	if to == "" {
		return nil
	}
	return func(paths []string) string {
		out, _ := r.Git.WorkingTree.ShowFileDiffCmdObj(from, to, reverse, paths, git_commands.DiffModePlain).DontLog().RunWithOutput()
		return out
	}
}

// copyDiffSelection is lazygit's MainViewController.copySelection.
func (a *App) copyDiffSelection() {
	s := a.staging
	tr := a.repo.Common.Tr
	if s == nil || s.cursor < 0 {
		a.setStatus(tr.SelectionNotFoundInDiffToast, true)
		return
	}
	first, last := s.selection()
	// textAboveDiff: the selected rows above the first file, as they read.
	aboveDiff := ""
	if start := lo.IndexOf(lo.Map(s.rows, func(r diffRow, _ int) bool { return r.ok }), true); start >= 0 {
		if end := min(last, start-1); end >= first {
			texts := make([]string, 0, end-first+1)
			for i := first; i <= end; i++ {
				texts = append(texts, s.lines[i].Content)
			}
			aboveDiff = strings.Join(texts, "\n") + "\n"
		}
	}
	type selected struct {
		lines  map[patchLineKey]bool
		header bool
	}
	var paths []string
	selection := map[string]*selected{}
	for i := first; i <= last; i++ {
		for _, r := range s.identitiesAt(i) {
			switch r.kind {
			case patch.PATCH_HEADER, patch.HUNK_HEADER, patch.CONTEXT, patch.ADDITION, patch.DELETION:
			default:
				continue
			}
			sel, ok := selection[r.path]
			if !ok {
				paths = append(paths, r.path)
				sel = &selected{lines: map[patchLineKey]bool{}}
				selection[r.path] = sel
			}
			if r.kind == patch.PATCH_HEADER {
				sel.header = true
			} else {
				k, _ := r.key()
				sel.lines[k] = true
			}
		}
	}
	plain := a.plainDiffFor(s)
	finish := func(fromDiff string) {
		if aboveDiff == "" {
			fromDiff = dropDiffPrefix(fromDiff)
		}
		text := aboveDiff + fromDiff
		if text == "" {
			a.setStatus(tr.SelectionNotFoundInDiffToast, true)
			return
		}
		if err := a.copyText(text); err != nil {
			a.showError(tr.CopySelectedDiffLinesToClipboard, err)
		} else {
			a.setStatus(tr.SelectedDiffLinesCopiedToast, false)
		}
	}
	if len(paths) == 0 || plain == nil {
		finish("")
		return
	}
	a.bg(func() {
		diff := strings.Split(strings.TrimSuffix(plain(paths), "\n"), "\n")
		_, rows, _ := parseDiffRows(strings.Join(diff, "\n"))
		runs := map[string][2]int{}
		for i, r := range rows {
			if !r.ok {
				continue
			}
			sel := selection[r.path]
			if sel == nil {
				continue
			}
			if r.kind == patch.PATCH_HEADER {
				if !sel.header {
					continue
				}
			} else if k, _ := r.key(); !sel.lines[k] {
				continue
			}
			if run, ok := runs[r.path]; ok {
				runs[r.path] = [2]int{run[0], i}
			} else {
				runs[r.path] = [2]int{i, i}
			}
		}
		var b strings.Builder
		for _, path := range paths {
			if run, ok := runs[path]; ok {
				for _, line := range diff[run[0] : run[1]+1] {
					b.WriteString(line)
					b.WriteString("\n")
				}
			}
		}
		fromDiff := b.String()
		a.post(func() { finish(fromDiff) })
	})
}

// dropDiffPrefix is lazygit's: a selection of one kind of change loses its
// +/- column; anything else stays a diff.
func dropDiffPrefix(diff string) string {
	if diff == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	plus, minus, other := false, false, false
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "+"):
			plus = true
		case strings.HasPrefix(l, "-"):
			minus = true
		case strings.HasPrefix(l, " "):
		default:
			other = true
		}
	}
	if other || (plus && minus) {
		return diff
	}
	return strings.Join(lo.Map(lines, func(l string, _ int) string { return l[1:] + "\n" }), "")
}

package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/mergeconflicts"
	"laziergit/internal/keymap"
)

// conflictOverlay retains lazygit's selection and undo stack for the session.
type conflictOverlay struct {
	file   *models.File
	state  *mergeconflicts.State
	busy   bool
	list   ui.ListState
	reveal bool
}

func (a *App) openConflict(f *models.File) {
	var content string
	a.run("Loading conflict…", func() error {
		b, err := os.ReadFile(f.Path)
		content = string(b)
		return err
	}, func() {
		// One state for every file, like lazygit's MergeConflictsContext: the
		// selected hunk side carries over to the next file.
		if a.conflictState == nil {
			a.conflictState = mergeconflicts.NewState()
		}
		s := a.conflictState
		s.Reset()
		s.SetContent(content, f.Path, f.ConflictMarkerSize)
		a.pushOverlay(&conflictOverlay{file: f, state: s, reveal: true})
	})
}

func (v *conflictOverlay) key(a *App, ch keymap.Chord) {
	if v.busy {
		return
	}
	if a.keys.ret.Matches(ch) || a.keys.quit.Matches(ch) {
		a.closeOverlay(v)
		return
	}
	if v.state.NoConflicts() && a.keys.goInto.Matches(ch) {
		v.continueOperation(a)
		return
	}
	if a.keys.prevPage.Matches(ch) || a.keys.nextPage.Matches(ch) {
		first, _ := v.list.Visible()
		delta := -12
		if a.keys.nextPage.Matches(ch) {
			delta = 12
		}
		v.list.ScrollTo(max(0, first+delta), ui.Start)
		return
	}
	for _, act := range v.actions(a) {
		if act.keys.Matches(ch) {
			act.run()
			v.reveal = true
			return
		}
	}
}

func (v *conflictOverlay) resolve(a *App, selection mergeconflicts.Selection) {
	if v.state.NoConflicts() {
		return
	}
	v.busy = true
	var content string
	a.run("Resolving…", func() error {
		ok, next, err := v.state.ContentAfterConflictResolve(selection)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("no inline conflict remains")
		}
		content = next
		return a.repo.WriteConflict(v.file, v.state.GetContent(), content, false)
	}, func() {
		v.state.PushContent(content)
		if v.state.NoConflicts() {
			a.closeOverlay(v)
			a.focus = panelFiles
			a.mainFocused = false
		}
	})
	// a.run completes asynchronously; clear busy in its completion, including errors.
}

func (v *conflictOverlay) undo(a *App) {
	old := v.state.GetContent()
	// Build the prospective undo without losing the history on a failed write.
	if !v.state.Undo() {
		return
	}
	next := v.state.GetContent()
	v.state.PushContent(old)
	v.busy = true
	a.run("Undo…", func() error { return a.repo.WriteConflict(v.file, old, next, true) }, func() { v.state.Undo() })
}

func (v *conflictOverlay) view(a *App, c *ui.Context, p *Palette) {
	// run serializes mutations; errors also release the view for retry.
	if a.busy == "" {
		v.busy = false
	}
	card(c, p, 900, func() {
		ui.Text(c, "Resolve conflict · "+v.file.Path).FontWeight(600).Padding(12)
		if v.state.NoConflicts() {
			ui.Text(c, "All conflicts resolved. File staged.").Padding(12)
			r := ui.Row(c).Padding(12)
			if r.Clicked() {
				v.continueOperation(a)
			}
			r.Children(func() { ui.Text(c, "Continue operation"); keycap(c, p, "enter") })
		} else {
			start, end := v.state.GetSelectedRange()
			if v.reveal {
				v.list.ScrollTo(max(0, start-3), ui.Start)
				v.reveal = false
			}
			lines := strings.Split(v.state.GetContent(), "\n")
			lo, hi := 0, len(lines)
			ui.List(c, &v.list, hi-lo, func(i int) {
				n := lo + i
				row := ui.Row(c).PaddingX(12).Height(22)
				if n >= start && n <= end {
					row.Background(p.AccentSoft)
				}
				row.Children(func() { ui.Text(c, fmt.Sprintf("%4d  %s", n+1, lines[n])).Font(monoFont).FontSize(12) })
			}).Height(min(float32(hi-lo)*22, 440))
		}
		for _, act := range v.actions(a) {
			act := act
			row := ui.Row(c).PaddingX(12).PaddingY(3).Gap(10)
			if row.Clicked() && !v.busy {
				act.run()
				v.reveal = true
			}
			row.Children(func() { ui.Text(c, act.label); ui.Box(c).Grow(1); keycap(c, p, act.keys.LabelAll()) })
		}
	})
}

func (v *conflictOverlay) continueOperation(a *App) {
	for _, f := range a.snap.Files {
		if f.HasMergeConflicts {
			return
		}
	}
	kind := map[string]string{"Rebasing": "rebase", "Merging": "merge", "Cherry-picking": "cherry-pick", "Reverting": "revert"}[a.snap.Operation]
	if kind == "" {
		a.closeOverlay(v)
		return
	}
	a.confirm(a.repo.Common.Tr.Continue, fmt.Sprintf(a.repo.Common.Tr.ConflictsResolved, kind), false, func() {
		a.closeOverlay(v)
		a.continueResolvedOperation(kind)
	})
}

// Refresh after confirmation: the user may have fixed build errors while it was open.
func (a *App) continueResolvedOperation(kind string) {
	a.run("Checking files…", func() error { return nil }, func() {
		resume := func() {
			a.selectHeadAfterMerge("continue")
			a.run("Continuing…", func() error { return a.repo.SequenceControl(kind, "continue") }, nil)
		}
		var paths []string
		for _, f := range a.snap.Files {
			if f.HasUnstagedChanges && f.Tracked && !f.IsSubmodule(a.snap.Submodules) { // GetUnstagedFilesExceptSubmodules
				paths = append(paths, f.Path)
			}
		}
		if len(paths) > 0 {
			a.confirm(a.repo.Common.Tr.Continue, a.repo.Common.Tr.UnstagedFilesAfterConflictsResolved, false, func() { a.run("Staging…", func() error { return a.repo.StageConflictEdits(paths) }, resume) })
			return
		}

		resume()
	})
}

// promptToContinue is lazygit's MergeAndRebaseHelper.PromptToContinueRebase,
// opened by a refresh that finds the last conflict of an operation started
// here resolved.
func (a *App) promptToContinue() {
	if a.snap == nil || a.snap.OperationCommand == "" {
		return
	}
	kind := a.snap.OperationCommand
	a.confirm(a.repo.Common.Tr.Continue, fmt.Sprintf(a.repo.Common.Tr.ConflictsResolved, kind), false, func() {
		a.continueResolvedOperation(kind)
	})
}

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
	file  *models.File
	state *mergeconflicts.State
	busy  bool
	list  ui.ListState
}

func (a *App) openConflict(f *models.File) {
	var content string
	a.run("Loading conflict…", func() error {
		b, err := os.ReadFile(f.Path)
		content = string(b)
		return err
	}, func() {
		s := mergeconflicts.NewState()
		s.SetContent(content, f.Path, f.ConflictMarkerSize)
		a.pushOverlay(&conflictOverlay{file: f, state: s})
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
	for _, act := range v.actions(a) {
		if act.keys.Matches(ch) {
			act.run()
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
	}, func() { v.state.PushContent(content) })
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
			lines := strings.Split(v.state.GetContent(), "\n")
			lo, hi := max(0, start-4), min(len(lines), end+5)
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
			}
			row.Children(func() { ui.Text(c, act.label); ui.Box(c).Grow(1); keycap(c, p, act.keys.LabelAll()) })
		}
	})
}

func (v *conflictOverlay) continueOperation(a *App) {
	for _, f := range a.snap.Files {
		if f.HasMergeConflicts {
			a.setStatus("Resolve the remaining files before continuing", false)
			return
		}
	}
	kind := map[string]string{"Rebasing": "rebase", "Merging": "merge", "Cherry-picking": "cherry-pick", "Reverting": "revert"}[a.snap.Operation]
	if kind == "" {
		a.closeOverlay(v)
		return
	}
	a.confirm("continue "+kind, "Run git "+kind+" --continue?", false, func() {
		a.closeOverlay(v)
		a.run("Continuing…", func() error { return a.repo.SequenceControl(kind, "continue") }, nil)
	})
}

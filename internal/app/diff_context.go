package app

import (
	"fmt"
	"math"

	"laziergit/internal/keymap"
)

// Lazygit's ContextLinesController: } and { change how many context lines
// diffs show, for the rest of the session.

func (a *App) diffContextActions() []action {
	tr := a.repo.Common.Tr
	u := a.repo.Config.Keybinding.Universal
	return []action{
		{scopeGlobal, keymap.Parse(u.IncreaseContextInDiffView), tr.IncreaseContextInDiffView, func() { a.changeDiffContext(1) }, false},
		{scopeGlobal, keymap.Parse(u.DecreaseContextInDiffView), tr.DecreaseContextInDiffView, func() { a.changeDiffContext(-1) }, false},
	}
}

func (a *App) changeDiffContext(d int) {
	g := &a.repo.Config.Git
	switch {
	case d > 0 && g.DiffContextSize < math.MaxUint64:
		g.DiffContextSize++
	case d < 0 && g.DiffContextSize > 0:
		g.DiffContextSize--
	}
	a.setStatus(fmt.Sprintf(a.repo.Common.Tr.DiffContextSizeChanged, g.DiffContextSize), false)
	a.diffKey = ""
	if a.staging != nil {
		a.reloadStaging()
		return
	}
	a.loadMain()
}

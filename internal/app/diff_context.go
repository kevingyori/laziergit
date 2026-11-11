package app

import (
	"fmt"
	"math"
	"strconv"

	"github.com/jesseduffield/lazygit/pkg/utils"

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
		{scopeGlobal, keymap.Parse(u.ToggleWhitespaceInDiffView), tr.ToggleWhitespaceInDiffView, a.toggleWhitespace, false},
		{scopeGlobal, keymap.Parse(u.CycleDiffRenderers), tr.CycleDiffRenderers, func() { a.cycleDiffRenderers(true) }, false},
		{scopeGlobal, keymap.Parse(u.CycleDiffRenderersReverse), tr.CycleDiffRenderersReverse, func() { a.cycleDiffRenderers(false) }, false},
		{scopeGlobal, keymap.Parse(u.IncreaseRenameSimilarityThreshold), tr.IncreaseRenameSimilarityThreshold, func() { a.changeRenameSimilarity(5) }, false},
		{scopeGlobal, keymap.Parse(u.DecreaseRenameSimilarityThreshold), tr.DecreaseRenameSimilarityThreshold, func() { a.changeRenameSimilarity(-5) }, false},
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

// changeRenameSimilarity is lazygit's RenameSimilarityThresholdController.
func (a *App) changeRenameSimilarity(d int) {
	tr := a.repo.Common.Tr
	if a.customPatch != nil {
		a.errorPopup(tr.CantChangeRenameThresholdError)
		return
	}
	g := &a.repo.Config.Git
	switch {
	case d > 0 && g.RenameSimilarityThreshold < 100:
		g.RenameSimilarityThreshold = min(100, g.RenameSimilarityThreshold+d)
	case d < 0 && g.RenameSimilarityThreshold > 5:
		g.RenameSimilarityThreshold = max(5, g.RenameSimilarityThreshold+d)
	}
	a.setStatus(fmt.Sprintf(tr.RenameSimilarityThresholdChanged, g.RenameSimilarityThreshold), false)
	a.diffKey = ""
	switch { // by the side panel (lazygit's CurrentSide), even with the main view focused
	case a.focus == panelFiles:
		a.Refresh()
	case a.staging != nil:
		a.reloadStaging()
	case a.focus == panelCommits && a.commitFiles != nil:
		a.reloadCommitFiles()
	default:
		a.loadMain()
	}
}

// cycleDiffRenderers is lazygit's GlobalController.cycleDiffRenderers(Backward).
func (a *App) cycleDiffRenderers(forward bool) {
	tr := a.repo.Common.Tr
	m := a.repo.DiffRenderers
	if _, total := m.CurrentDiffRendererIndex(); total <= 1 {
		a.setStatus(tr.DisabledMenuItemPrefix+tr.CycleDiffRenderersDisabledReason, true)
		return
	}
	if forward {
		m.CycleDiffRenderers()
	} else {
		m.CycleDiffRenderersBackward()
	}
	a.rerenderPatchPreview()
	a.diffKey = ""
	if a.staging != nil {
		a.reloadStaging()
	} else {
		a.loadMain()
	}
	current, total := m.CurrentDiffRendererIndex()
	a.setStatus(utils.ResolvePlaceholderString(tr.SelectedDiffRenderers, map[string]string{
		"name":    m.CurrentDiffRendererName(tr),
		"current": strconv.Itoa(current + 1),
		"total":   strconv.Itoa(total),
	}), false)
}

// toggleWhitespace is lazygit's ToggleWhitespaceAction.
func (a *App) toggleWhitespace() {
	g := &a.repo.Config.Git
	g.IgnoreWhitespaceInDiffView = !g.IgnoreWhitespaceInDiffView
	a.diffKey = ""
	if a.staging != nil {
		a.reloadStaging()
		return
	}
	a.loadMain()
}

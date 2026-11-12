package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"laziergit/internal/keymap"
)

// optionHint is one keybinding suggestion of the options bar (lazygit's
// bindingInfo). mode marks the mode-specific suggestions lazygit colours.
type optionHint struct {
	key, label, tip string
	run             func()
	mode            bool
}

// optionsMap is lazygit's OptionsMapMgr.renderContextOptionsMap: the
// focused context's bindings shown on screen, then the global ones whose
// first key the context doesn't use, minus disabled bindings; mode-specific
// suggestions go first. The hint bar draws it and the parity harness reads
// it as the options view.
func (a *App) optionsMap() []optionHint {
	sc := a.scope()
	used := map[keymap.Chord]bool{}
	for _, act := range a.actions {
		if act.scope == sc {
			for _, c := range act.keys {
				used[c] = true
			}
		}
	}
	var hints []optionHint
	for _, pass := range []scope{sc, scopeGlobal} {
		for _, act := range a.actions {
			if act.scope != pass || !act.hint || len(act.keys) == 0 {
				continue
			}
			if pass == scopeGlobal && sc != scopeGlobal && used[act.keys[0]] {
				continue
			}
			label, ok := a.optionLabel(act)
			if !ok || label == "" {
				continue
			}
			hints = append(hints, optionHint{key: lazygitKeyLabel(act.keys), label: label, tip: a.actionTooltip(act), run: act.run})
		}
	}
	tr := a.repo.Common.Tr
	kb := a.repo.Config.Keybinding
	prepend := func(key, label string, run func()) {
		hints = append([]optionHint{{key: key, label: label, tip: a.contextTip(sc, label), run: run, mode: true}}, hints...)
	}
	if sc == scopeCommits {
		if a.cherry.Active() {
			prepend(kb.Commits.PasteCommits.String(), tr.PasteCommits, a.pasteCommits)
		}
		if a.bisectInfo().Started() {
			prepend(kb.Commits.ViewBisectOptions.String(), tr.ViewBisectOptions, a.bisectMenu)
		}
	}
	if a.snap != nil && a.snap.OptionsMapTitle != "" {
		prepend(kb.Universal.CreateRebaseOptionsMenu.String(), a.snap.OptionsMapTitle, a.operationMenu)
	}
	if a.customPatch != nil {
		prepend(kb.Universal.CreatePatchOptionsMenu.String(), tr.ViewPatchOptions, a.patchOptions)
	}
	return hints
}

// optionsText is the options bar as lazygit's formatBindingInfos writes it.
func (a *App) optionsText() string {
	parts := make([]string, 0, 8)
	for _, h := range a.optionsMap() {
		parts = append(parts, fmt.Sprintf("%s: %s", h.label, h.key))
	}
	return strings.Join(parts, " | ")
}

// optionLabel is a binding's GetShortDescription, and false when the
// binding is disabled where we are (lazygit's GetDisabledReason).
func (a *App) optionLabel(act action) (string, bool) {
	tr := a.repo.Common.Tr
	if act.scope == scopeGlobal && act.label == "Return" {
		return a.escapeDescription()
	}
	label := a.actionLabel(act)
	switch label {
	case tr.EditCommit:
		label = tr.Edit
	case tr.ViewBranchUpstreamOptions:
		label = tr.Upstream
	}
	if act.scope == scopeGlobal || act.scope == scopeStaging || act.scope == scopeMain {
		return label, true
	}
	// Bindings that don't act on the selected item stay enabled in an
	// empty list.
	switch label {
	case tr.Commit, tr.Stash, tr.Reset, tr.NewRemote, tr.NewTag, tr.NewSubmodule, tr.NewWorktree:
		if act.scope != scopeTags || label != tr.Reset {
			return label, true
		}
	}
	if act.scope == scopeCommitFiles {
		return label, !a.emptyCommitFiles()
	}
	return label, a.count(a.focus) > 0
}

// escapeDescription is lazygit's QuitActions.EscapeDescription, false when
// EscapeEnabled says <esc> has nothing to do.
func (a *App) escapeDescription() (string, bool) {
	tr := a.repo.Common.Tr
	if p, ok := a.focusedList(); ok && a.selectingRange(p) {
		return tr.DismissRangeSelect, true
	}
	if t, ok := a.searchTarget(); ok && t.filterable && a.isFiltering(t.key) {
		return tr.ExitFilterMode, true
	}
	if !a.mainFocused && a.staging == nil && (a.focus == panelCommits && (a.commitFiles != nil || a.commitsView() == cvSubCommits) ||
		a.focus == panelBranches && a.browse(panelBranches) != nil) {
		return tr.ExitSubview, true
	}
	switch {
	case a.diffingInfo() != "":
		return tr.CancelDiffingMode, true
	case a.customPatch != nil:
		return tr.ResetCustomPatch, true
	case a.filteringActive():
		return tr.ExitFilterMode, true
	case a.cherry.Active():
		return tr.ResetCherryPickShort, true
	case a.snap != nil && a.snap.OperationCommand != "":
		return fmt.Sprintf(tr.AbortTitle, a.snap.OperationCommand), true
	case a.bisectInfo().Started():
		return tr.Actions.ResetBisect, true
	case len(a.parents) > 0:
		return tr.BackToParentRepo, true
	case a.repo.Config.QuitOnTopLevelReturn:
		return tr.Quit, true
	}
	return tr.Cancel, false
}

// lazygitKeyLabel is config.LabelForKey of the binding's first key.
func lazygitKeyLabel(b keymap.Binding) string {
	l := b.Label()
	for app, lg := range map[string]string{"↑": "up", "↓": "down", "←": "left", "→": "right", "pgdn": "pgdown", "del": "delete", "bksp": "backspace", "ins": "insert"} {
		l = strings.Replace(l, app, lg, 1)
	}
	if utf8.RuneCountInString(l) > 1 {
		l = "<" + l + ">"
	}
	return l
}

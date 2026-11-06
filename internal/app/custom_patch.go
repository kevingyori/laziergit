package app

import (
	"fmt"
	"slices"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"laziergit/internal/keymap"
)

// Immutable UI snapshot; the builder and all git reads stay on the worker.
type customPatchState struct {
	from, to, preview, plain string
	canRebase                bool
	included                 map[string][]patch.LineIdentity
	statuses                 map[string]patch.PatchStatus
}

func (a *App) patchTarget() (from, to string, canRebase bool) {
	if a.focus == panelStash {
		if s := a.selectedStash(); s != nil {
			return s.RefName() + "^", s.RefName(), false
		}
		return
	}
	c := a.selectedCommit()
	if c == nil {
		return
	}
	to = c.Hash()
	from = to + "^"
	if c.IsFirstCommit() {
		from = models.EmptyTreeCommitHash
	}
	canRebase = a.focus == panelCommits && (a.commitFiles == nil || !a.commitFiles.fromRef && !a.commitFiles.fromStash)
	return
}

func readCustomPatch(p *git_commands.PatchCommands) (*customPatchState, error) {
	b := p.PatchBuilder
	if !b.Active() || b.IsEmpty() {
		b.Reset()
		return nil, nil
	}
	// The no-index diff exits 1 when the trees differ.
	// Caller fills preview using the repository's diff command.
	s := &customPatchState{from: b.From, to: b.To, canRebase: b.CanRebase, plain: b.RenderAggregatedPatch(true), included: map[string][]patch.LineIdentity{}, statuses: map[string]patch.PatchStatus{}}
	for _, path := range b.AllFilesInPatch() {
		status := b.GetFileStatus(path, b.To)
		if status == patch.UNSELECTED {
			continue
		}
		s.statuses[path] = status
		s.included[path] = b.IncludedLineIdentities(path)
	}
	return s, nil
}

// editCustomPatch blocks only patch input while the builder is changing. It
// publishes a complete preview before accepting the next toggle.
func (a *App) editCustomPatch(edit func(*git_commands.PatchCommands) error, done func()) {
	if a.busy != "" {
		return
	}
	r := a.repo
	cf := a.commitFiles
	keepPath := ""
	if n := a.selectedCommitFileNode(); n != nil {
		keepPath = n.GetPath()
	}
	a.busy = r.Common.Tr.BuildingPatch
	a.bg(func() {
		var state *customPatchState
		err := r.EditPatch(func(p *git_commands.PatchCommands) error {
			opErr := edit(p)
			var e error
			state, e = readCustomPatch(p)
			if state != nil {
				state.preview, e = r.CustomPatchPreview(p)
			}
			if opErr != nil {
				return opErr
			}
			return e
		})
		snap := r.Load()
		var replacement *models.Commit
		var replacementFiles []*models.CommitFile
		if cf != nil && !cf.fromRef && !cf.fromStash && cf.commitIdx < len(snap.Commits) {
			found := false
			for _, c := range snap.Commits {
				if c.Hash() == cf.commit.Hash() {
					found = true
					break
				}
			}
			if !found {
				replacement = snap.Commits[cf.commitIdx]
				var loadErr error
				replacementFiles, loadErr = r.CommitFiles(replacement)
				if loadErr != nil && err == nil {
					err = loadErr
				}
			}
		}
		a.post(func() {
			if a.repo != r {
				return
			}
			a.busy = ""
			a.customPatch = state
			if replacement != nil && a.commitFiles == cf {
				cf.commit, cf.files = replacement, replacementFiles
				cf.tree = newCommitFileTree(a, replacement, replacementFiles)
				if i, ok := cf.tree.GetIndexForPath(keepPath); ok {
					a.cursor[panelCommits] = i
				}
			}
			a.apply(snap)
			if err != nil {
				if isMergeConflict(err) && snap.Operation != "" {
					a.conflictsMenu(snap.Operation)
				} else {
					a.showError(r.Common.Tr.BuildingPatch, err)
				}
				return
			}
			if done != nil {
				done()
			}
		})
	})
}

func (a *App) withPatchTarget(from, to string, canRebase bool, edit func(*git_commands.PatchCommands) error, done func()) {
	run := func() {
		a.editCustomPatch(func(p *git_commands.PatchCommands) error {
			b := p.PatchBuilder
			if b.NewPatchRequired(from, to, false) {
				b.Reset()
				b.Start(from, to, false, canRebase)
			}
			return edit(p)
		}, done)
	}
	if a.customPatch != nil && (a.customPatch.from != from || a.customPatch.to != to) {
		tr := a.repo.Common.Tr
		a.confirm(tr.DiscardPatch, tr.DiscardPatchConfirm, true, run)
	} else {
		run()
	}
}

func (a *App) togglePatchSelection(discard bool) {
	s := a.staging
	if s == nil || s.worktree || s.cursor < 0 || a.busy != "" {
		return
	}
	r := a.repo
	tr := r.Common.Tr
	if s.staged {
		if discard {
			a.errorPopup(tr.CannotDiscardFromCustomPatchView)
			return
		}
		a.removePreviewSelection()
		return
	}
	from, to, canRebase := a.patchTarget()
	if to == "" {
		return
	}
	if discard && (!canRebase || a.snap.Operation != "") {
		a.errorPopup(tr.CanOnlyDiscardFromLocalCommits)
		return
	}
	if a.repo.Config.Git.DiffContextSize == 0 {
		msg := tr.Actions.NotEnoughContextForCustomPatch
		if discard {
			msg = tr.Actions.NotEnoughContextToRemoveLines
		}
		a.errorPopup(fmt.Sprintf(msg, a.repo.Config.Keybinding.Universal.IncreaseContextInDiffView))
		return
	}
	lo, hi := s.selection()
	byPath := map[string][]patch.LineIdentity{}
	paths := []string{}
	count := 0
	for i := lo; i <= hi; i++ {
		r := s.rows[i]
		if !r.isChange() {
			continue
		}
		if _, ok := byPath[r.path]; !ok {
			paths = append(paths, r.path)
		}
		byPath[r.path] = append(byPath[r.path], r.lineIdentity())
		count++
	}
	if count == 0 {
		return
	}
	commits := a.raw.Commits
	source := -1
	for i, c := range commits {
		if c.Hash() == to {
			source = i
			break
		}
	}
	edit := func(p *git_commands.PatchCommands) error {
		b := p.PatchBuilder
		files, err := r.Git.Loaders.CommitFileLoader.GetFilesInDiff(from, to, false)
		if err != nil {
			return err
		}
		byFile := map[string]*models.CommitFile{}
		for _, f := range files {
			byFile[f.Path] = f
		}
		removing := false
		for n, path := range paths {
			f := byFile[path]
			if f == nil {
				return fmt.Errorf("file %s is no longer in the diff", path)
			}
			indices, whole, err := b.PatchLineIndicesForLines(path, f.PreviousPath, byPath[path])
			if err != nil {
				return err
			}
			if n == 0 {
				inc, e := b.GetFileIncLineIndices(path, f.PreviousPath)
				if e != nil {
					return e
				}
				removing = len(indices) > 0 && slices.Contains(inc, indices[0])
			}
			switch {
			case whole && (f.ChangeStatus == "A" || f.ChangeStatus == "D") && removing:
				err = b.RemoveFile(path, f.PreviousPath)
			case whole && (f.ChangeStatus == "A" || f.ChangeStatus == "D"):
				err = b.AddFileWhole(path, f.PreviousPath)
			case removing:
				err = b.RemoveFileLineRange(path, f.PreviousPath, indices)
			default:
				err = b.AddFileLineRange(path, f.PreviousPath, indices)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	done := func() {
		if a.staging == s {
			s.revealOrdinal(s.changeOrdinal(lo) + count)
		}
	}
	if discard {
		if source < 0 {
			a.errorPopup(utils.ResolvePlaceholderString(tr.CommitNotInCommitsPanel, map[string]string{"hash": utils.ShortHash(to)}))
			return
		}
		prompt := tr.DiscardLinesFromCommitPrompt
		if a.customPatch != nil {
			prompt = tr.DiscardLinesFromCommitPromptWithReset
		}
		a.confirm(tr.DiscardLinesFromCommitTitle, prompt, true, func() {
			a.editCustomPatch(func(p *git_commands.PatchCommands) error {
				p.PatchBuilder.Reset()
				p.PatchBuilder.Start(from, to, false, canRebase)
				if err := edit(p); err != nil {
					p.PatchBuilder.Reset()
					return err
				}
				return p.DeletePatchesFromCommit(commits, source)
			}, nil)
		})
		return
	}
	a.withPatchTarget(from, to, canRebase, edit, done)
}

func (a *App) togglePatchFiles(all bool) {
	cf := a.commitFiles
	if cf == nil || cf.tree.Len() == 0 || a.busy != "" {
		return
	}
	from, to, canRebase := a.patchTarget()
	start, end := a.selectionRange(panelCommits)
	var files []*models.CommitFile
	if all {
		files = append(files, cf.files...)
	} else {
		seen := map[string]bool{}
		for i := start; i <= end; i++ {
			_ = cf.tree.Get(i).ForEachFile(func(f *models.CommitFile) error {
				if !seen[f.Path] {
					files = append(files, f)
					seen[f.Path] = true
				}
				return nil
			})
		}
	}
	if len(files) == 0 {
		return
	}
	a.withPatchTarget(from, to, canRebase, func(p *git_commands.PatchCommands) error {
		b := p.PatchBuilder
		adding := false
		for _, f := range files {
			if b.GetFileStatus(f.Path, to) != patch.WHOLE {
				adding = true
			}
		}
		for _, f := range files {
			var err error
			if adding {
				err = b.AddFileWhole(f.Path, f.PreviousPath)
			} else {
				err = b.RemoveFile(f.Path, f.PreviousPath)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}, nil)
}

func (a *App) removePreviewSelection() {
	r := a.repo
	s := a.staging
	lo, hi := s.selection()
	s.reveal = s.changeOrdinal(lo)
	// Map each preview change's ordinal to the source indices: preview line
	// numbers shift when earlier changes were omitted from the patch.
	indices := map[string][]int{}
	ordinal := map[string]int{}
	for i, r := range s.rows {
		if !r.isChange() {
			continue
		}
		if i >= lo && i <= hi {
			indices[r.path] = append(indices[r.path], ordinal[r.path])
		}
		ordinal[r.path]++
	}
	a.editCustomPatch(func(p *git_commands.PatchCommands) error {
		b := p.PatchBuilder
		files, err := r.Git.Loaders.CommitFileLoader.GetFilesInDiff(b.From, b.To, false)
		if err != nil {
			return err
		}
		for _, f := range files {
			ords := indices[f.Path]
			if len(ords) == 0 {
				continue
			}
			inc := b.IncludedChangeLineIndices(f.Path)
			var remove []int
			for _, i := range ords {
				if i < len(inc) {
					remove = append(remove, inc[i])
				}
			}
			if err := b.RemoveFileLineRange(f.Path, f.PreviousPath, remove); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}

func (a *App) patchMarked(r diffRow) bool {
	state := a.customPatch
	if state == nil || !r.isChange() {
		return false
	}
	_, to, _ := a.patchTarget()
	if to != state.to {
		return false
	}
	return slices.Contains(state.included[r.path], r.lineIdentity())
}

func (a *App) resetCustomPatch() {
	a.editCustomPatch(func(p *git_commands.PatchCommands) error { p.PatchBuilder.Reset(); return nil }, nil)
}

func (a *App) patchOptions() {
	tr := a.repo.Common.Tr
	state := a.customPatch
	if state == nil {
		a.errorPopup(tr.NoPatchError)
		return
	}
	item := func(key, label string, run func()) menuItem {
		return menuItem{key: key, chord: keymap.Parse(config.Keybinding{key}), label: label, run: run}
	}
	items := []menuItem{item("c", tr.ResetPatch, a.resetCustomPatch), item("a", tr.ApplyPatch, func() { a.applyCustomPatch(false) }), item("r", tr.ApplyPatchInReverse, func() { a.applyCustomPatch(true) })}
	commits := a.raw.Commits
	source := -1
	for i, c := range commits {
		if c.Hash() == state.to {
			source = i
			break
		}
	}
	if state.canRebase && source >= 0 && a.snap.Operation == "" {
		operate := func(op func(*git_commands.PatchCommands) error) { a.editCustomPatch(op, nil) }
		if a.scope() == scopeCommits && a.selectedCommit() != nil && a.selectedCommit().Hash() != state.to {
			target := -1
			for i, c := range commits {
				if c.Hash() == a.selectedCommit().Hash() {
					target = i
					break
				}
			}
			m := item("m", fmt.Sprintf(tr.MovePatchToSelectedCommit, a.selectedCommit().Hash()), func() {
				operate(func(p *git_commands.PatchCommands) error { return p.MovePatchToSelectedCommit(commits, source, target) })
			})
			if a.multipleSelected(panelCommits) {
				m.disabled = tr.RangeSelectNotSupported
			}
			items = append(items[:1], append([]menuItem{m}, items[1:]...)...)
		}
		items = append(items, item("d", fmt.Sprintf(tr.RemovePatchFromOriginalCommit, utils.ShortHash(state.to)), func() {
			operate(func(p *git_commands.PatchCommands) error { return p.DeletePatchesFromCommit(commits, source) })
		}), item("i", tr.MovePatchOutIntoIndex, func() {
			dirty := false
			for _, f := range a.raw.Files {
				if !f.IsSubmodule(a.raw.Submodules) {
					dirty = true
					break
				}
			}
			run := func() {
				operate(func(p *git_commands.PatchCommands) error { return p.MovePatchIntoIndex(commits, source, dirty) })
			}
			if dirty {
				a.confirm(tr.MustStashTitle, tr.MustStashWarning, false, run)
			} else {
				run()
			}
		}), item("n", tr.MovePatchIntoNewCommit, func() { a.patchNewCommit(commits, source, false) }), item("N", tr.MovePatchIntoNewCommitBefore, func() { a.patchNewCommit(commits, source, true) }))
	}
	items = append(items, item("y", tr.CopyPatchToClipboard, func() {
		if err := a.copyText(state.plain); err != nil {
			a.showError(tr.CopyPatchToClipboard, err)
		} else {
			a.setStatus(tr.PatchCopiedToClipboard, false)
		}
	}))
	a.pushOverlay(&menuOverlay{title: tr.PatchOptionsTitle, items: items})
}
func (a *App) patchNewCommit(commits []*models.Commit, source int, before bool) {
	a.pushOverlay(&commitOverlay{title: a.repo.Common.Tr.CommitSummaryTitle, draft: &commitDraft{}, onSubmit: func(summary, description string) {
		a.editCustomPatch(func(p *git_commands.PatchCommands) error {
			if before {
				return p.PullPatchIntoNewCommitBefore(commits, source, summary, description)
			}
			return p.PullPatchIntoNewCommit(commits, source, summary, description)
		}, func() {
			a.commitFiles = nil
			a.staging = nil
			a.cursor[panelCommits] = source
			a.focusPanel(panelCommits)
		})
	}})
}
func (a *App) applyCustomPatch(reverse bool) {
	r := a.repo
	state := a.customPatch
	if state == nil {
		return
	}
	var affected []string
	for _, f := range a.raw.Files {
		if f.HasUnstagedChanges {
			if _, ok := state.included[f.Path]; ok {
				affected = append(affected, f.Path)
			}
		}
	}
	run := func() {
		a.editCustomPatch(func(p *git_commands.PatchCommands) error {
			if len(affected) > 0 {
				if err := r.Git.WorkingTree.StageFiles(affected, nil); err != nil {
					return err
				}
			}
			return p.ApplyCustomPatch(reverse, true)
		}, nil)
	}
	tr := a.repo.Common.Tr
	if len(affected) > 0 {
		a.confirm(tr.MustStageFilesAffectedByPatchTitle, tr.MustStageFilesAffectedByPatchWarning, false, run)
	} else {
		run()
	}
}

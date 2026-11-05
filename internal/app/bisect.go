package app

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"

	"laziergit/internal/keymap"
)

// bisectInfo is lazygit's Model().BisectInfo: what the last snapshot read.
func (a *App) bisectInfo() *git_commands.BisectInfo {
	if a.snap == nil || a.snap.BisectInfo == nil {
		return git_commands.NewNullBisectInfo()
	}
	return a.snap.BisectInfo
}

// bisectMenu is lazygit's BisectController.openMenu (`b` in commits).
func (a *App) bisectMenu() {
	commit := a.selectedCommit()
	if commit == nil {
		return
	}
	info := a.repo.Git.Bisect.GetInfo()
	if info.Started() {
		a.midBisectMenu(info, commit.Hash(), commit.ShortHash())
		return
	}
	a.startBisectMenu(info, commit.Hash(), commit.ShortHash())
}

func bisectItem(label string, key rune, disabled string, run func()) menuItem {
	return menuItem{label: label, key: string(key), chord: keymap.Binding{{Rune: key}}, disabled: disabled, run: run}
}

func (a *App) singleCommitDisabled() string {
	if a.multipleSelected(panelCommits) {
		return a.repo.Common.Tr.RangeSelectNotSupported
	}
	return ""
}

func (a *App) startBisectMenu(info *git_commands.BisectInfo, hash, short string) {
	tr := a.repo.Common.Tr
	git := a.repo.Git.Bisect
	start := func(term string) func() {
		return func() {
			a.run("", func() error {
				return a.repo.Locked(func() error {
					if err := git.Start(); err != nil {
						return err
					}
					return git.Mark(hash, term)
				})
			}, nil)
		}
	}
	disabled := a.singleCommitDisabled()
	a.pushOverlay(&menuOverlay{title: tr.Bisect.BisectMenuTitle, items: []menuItem{
		bisectItem(fmt.Sprintf(tr.Bisect.MarkStart, short, info.NewTerm()), 'b', disabled, start(info.NewTerm())),
		bisectItem(fmt.Sprintf(tr.Bisect.MarkStart, short, info.OldTerm()), 'g', disabled, start(info.OldTerm())),
		bisectItem(tr.Bisect.ChooseTerms, 't', "", func() {
			a.prompt(tr.Bisect.OldTermPrompt, "", "", func(oldTerm string) {
				a.prompt(tr.Bisect.NewTermPrompt, "", "", func(newTerm string) {
					a.run("", func() error {
						return a.repo.Locked(func() error { return git.StartWithTerms(oldTerm, newTerm) })
					}, nil)
				})
			})
		}),
		{label: tr.Cancel, run: func() {}},
	}})
}

func (a *App) midBisectMenu(info *git_commands.BisectInfo, hash, short string) {
	tr := a.repo.Common.Tr
	git := a.repo.Git.Bisect
	current := info.GetCurrentHash()
	selectCurrentAfter := current == "" || current == hash
	bisecting := current != ""
	toMark := lo.Ternary(bisecting, current, hash)
	shortToMark := utils.ShortHash(toMark)
	disabled := ""
	if !bisecting {
		disabled = a.singleCommitDisabled()
	}
	act := func(op func() error) func() {
		return func() { a.bisectMark(op, selectCurrentAfter) }
	}
	items := []menuItem{
		bisectItem(fmt.Sprintf(tr.Bisect.Mark, shortToMark, info.NewTerm()), 'b', disabled,
			act(func() error { return git.Mark(toMark, info.NewTerm()) })),
		bisectItem(fmt.Sprintf(tr.Bisect.Mark, shortToMark, info.OldTerm()), 'g', disabled,
			act(func() error { return git.Mark(toMark, info.OldTerm()) })),
		bisectItem(fmt.Sprintf(tr.Bisect.SkipCurrent, shortToMark), 's', disabled,
			act(func() error { return git.Skip(toMark) })),
	}
	if current != "" && current != hash {
		items = append(items, bisectItem(fmt.Sprintf(tr.Bisect.SkipSelected, short), 'S', a.singleCommitDisabled(),
			act(func() error { return git.Skip(hash) })))
	}
	items = append(items, bisectItem(tr.Bisect.ResetOption, 'r', "", a.resetBisect))
	// lazygit's menus end with Cancel (CreateMenuOptions without HideCancel).
	items = append(items, menuItem{label: tr.Cancel, run: func() {}})
	a.pushOverlay(&menuOverlay{title: tr.Bisect.BisectMenuTitle, items: items})
}

// bisectMark is lazygit's afterMark: mark, reload, select the new current
// commit if asked, and offer to reset once git has found the culprit.
func (a *App) bisectMark(op func() error, selectCurrent bool) {
	var done bool
	var candidates []string
	var oneline string
	a.run("", func() error {
		return a.repo.Locked(func() error {
			if err := op(); err != nil {
				return err
			}
			var err error
			done, candidates, err = a.repo.Git.Bisect.IsDone()
			if err != nil || !done {
				return err
			}
			oneline, err = a.repo.Git.Commit.GetCommitsOneline(candidates)
			return err
		})
	}, func() {
		if selectCurrent {
			if h := a.bisectInfo().GetCurrentHash(); h != "" {
				a.selectCommitHash(h)
			}
		}
		if done {
			a.bisectComplete(candidates, oneline)
		}
	})
}

// bisectComplete is lazygit's showBisectCompleteMessage.
func (a *App) bisectComplete(candidates []string, oneline string) {
	tr := a.repo.Common.Tr
	prompt := tr.Bisect.CompletePrompt
	if len(candidates) > 1 {
		prompt = tr.Bisect.CompletePromptIndeterminate
	}
	a.confirm(tr.Bisect.CompleteTitle, fmt.Sprintf(prompt, strings.TrimSpace(oneline)), false, a.doResetBisect)
}

// resetBisect is lazygit's BisectHelper.Reset.
func (a *App) resetBisect() {
	tr := a.repo.Common.Tr
	a.confirm(tr.Bisect.ResetTitle, tr.Bisect.ResetPrompt, false, a.doResetBisect)
}

func (a *App) doResetBisect() {
	a.run("", func() error { return a.repo.Locked(a.repo.Git.Bisect.Reset) }, nil)
}

// bisectMarker is the text lazygit's commit list puts after the hash
// (presentation.getBisectStatusText); index is the commit's place in the list.
func (a *App) bisectMarker(index int) string {
	info := a.bisectInfo()
	if !info.Started() || a.snap == nil {
		return ""
	}
	hash := a.snap.Commits[index].Hash()
	if info.GetCurrentHash() == hash {
		return "<-- current"
	}
	if status, ok := info.Status(hash); ok {
		switch status {
		case git_commands.BisectStatusNew:
			return "<-- " + info.NewTerm()
		case git_commands.BisectStatusOld:
			return "<-- " + info.OldTerm()
		case git_commands.BisectStatusSkipped:
			return "<-- skipped"
		}
		return ""
	}
	if info.Bisecting() {
		newIdx, oldIdx := -1, -1
		for i, c := range a.snap.Commits {
			if c.Hash() == info.GetNewHash() {
				newIdx = i
			}
			if s, ok := info.Status(c.Hash()); ok && s == git_commands.BisectStatusOld {
				oldIdx = i
				break
			}
		}
		if newIdx >= 0 && oldIdx >= 0 && index >= newIdx && index <= oldIdx {
			return "?"
		}
	}
	return ""
}

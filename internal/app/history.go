package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/context/traits"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/stefanhaller/git-todo-parser/todo"
	"laziergit/internal/keymap"
	"slices"
	"strings"
)

func (a *App) rebaseAction(action string) {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	if a.snap.OperationCommand == "cherry-pick" || a.snap.OperationCommand == "revert" {
		a.setStatus(a.repo.Common.Tr.DisabledMenuItemPrefix+a.repo.Common.Tr.NotAllowedMidCherryPickOrRevert, true)
		return
	}
	commits := a.snap.Commits
	start, end := a.selectionRange(panelCommits)
	i := a.cursor[panelCommits]
	if a.snap.Operation == "Rebasing" {
		tr := a.repo.Common.Tr
		reason := ""
		for _, cm := range commits[start : end+1] {
			if !cm.IsTODO() || cm.Status == models.StatusConflicted {
				reason = tr.MustSelectTodoCommits
				break
			}
			if cm.Action == todo.UpdateRef && (action == "drop" || action == "up" || action == "down") {
				continue
			}
			if !slices.Contains([]todo.TodoCommand{todo.Pick, todo.Drop, todo.Edit, todo.Fixup, todo.Squash, todo.Reword}, cm.Action) {
				reason = tr.ChangingThisActionIsNotAllowed
				break
			}
		}
		if reason == "" && (action == "up" || action == "down") {
			edge := start - 1
			if action == "down" {
				edge = end + 1
			}
			if edge < 0 || edge >= len(commits) || !commits[edge].IsTODO() || commits[edge].Status == models.StatusConflicted {
				reason = tr.CannotMoveAnyFurther
			}
		}
		if reason != "" {
			a.setStatus(tr.DisabledMenuItemPrefix+reason, true)
			return
		}
		// Editing the todo list of a rebase in progress rewrites nothing yet.
		removed := 0
		if action == "drop" {
			for _, cm := range commits[start : end+1] {
				if cm.Action == todo.UpdateRef {
					removed++
				}
			}
		}
		update := func() {
			a.nextCommitSel = keepCommitSelectionIndex // REBASE_COMMITS refresh
			a.run("Updating rebase todo…", func() error { return a.repo.RewriteRange(commits, start, end, action) }, func() {
				if removed > 0 {
					if i > a.ranges[panelCommits].start {
						a.cursor[panelCommits] = max(i-removed, start)
					} else {
						a.ranges[panelCommits].start = max(a.ranges[panelCommits].start-removed, i)
					}
				}
				switch action {
				case "up":
					a.cursor[panelCommits] = clamp(i-1, 0, a.count(panelCommits)-1)
					a.ranges[panelCommits].start = max(a.ranges[panelCommits].start-1, 0)
				case "down":
					a.cursor[panelCommits] = clamp(i+1, 0, a.count(panelCommits)-1)
					a.ranges[panelCommits].start = min(a.ranges[panelCommits].start+1, a.count(panelCommits)-1)
				}
				a.lists[panelCommits].ScrollIntoView(a.cursor[panelCommits])
				a.loadMain()
			})
		}
		if removed > 0 {
			a.confirm(tr.DropCommitTitle, tr.DropUpdateRefPrompt, false, update)
		} else {
			update()
		}
		return
	}
	if action == "edit" {
		hash := c.Hash()
		a.run("Rebasing…", func() error { return a.repo.RewriteRange(commits, start, end, action) }, func() { a.selectCommitHash(hash) })
		return
	}
	tr := a.repo.Common.Tr
	if action == "up" || action == "down" {
		// lazygit's move: no confirmation; canMoveUp/canMoveDown guard the ends.
		offset := 1
		if action == "up" {
			offset = -1
		}
		if (offset < 0 && start == 0) || (offset > 0 && end >= len(commits)-1) {
			a.setStatus(tr.DisabledMenuItemPrefix+tr.CannotMoveAnyFurther, true)
			return
		}
		a.nextCommitSel = keepCommitSelectionIndex
		a.run(tr.MovingStatus, func() error { return a.repo.RewriteRange(commits, start, end, action) }, func() {
			a.cursor[panelCommits] = clamp(i+offset, 0, a.count(panelCommits)-1)
			if a.ranges[panelCommits].mode != traits.RangeSelectModeNone {
				a.ranges[panelCommits].start = clamp(a.ranges[panelCommits].start+offset, 0, a.count(panelCommits)-1)
			}
			a.lists[panelCommits].ScrollIntoView(a.cursor[panelCommits])
			a.loadMain()
		})
		return
	}
	title, prompt := "Rewrite history", action+" “"+c.Name+"”? This rewrites this commit and its descendants."
	switch action {
	case "drop":
		title, prompt = tr.DropCommitTitle, tr.DropCommitPrompt
		if c.IsMerge() {
			prompt = tr.DropMergeCommitPrompt
		}
	case "squash":
		title, prompt = tr.Squash, tr.SureSquashThisCommit
	}
	a.confirm(title, prompt, true, func() {
		if !(action == "drop" && c.IsMerge()) {
			// selectRebaseResultCommit: the commit left at startIdx
			a.cursor[panelCommits] = start
			a.cancelRange(panelCommits)
		}
		a.run("Rebasing…", func() error { return a.repo.RewriteRange(commits, start, end, action) }, nil)
	})
}

func (a *App) rewordFlow() {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	commits, i := a.snap.Commits, a.cursor[panelCommits]
	if c.IsTODO() {
		a.setStatus(a.repo.Common.Tr.DisabledMenuItemPrefix+a.repo.Common.Tr.RewordNotSupported, true)
		return
	}
	if a.snap.Operation == "Rebasing" && !models.IsHeadCommit(commits, i) {
		a.setStatus(a.repo.Common.Tr.DisabledMenuItemPrefix+a.repo.Common.Tr.AlreadyRebasing, true)
		return
	}
	a.bg(func() {
		msg, err := a.repo.Git.Commit.GetCommitMessage(c.Hash())
		a.post(func() {
			if err != nil {
				a.showError("Read commit message", err)
				return
			}
			summary, body, _ := strings.Cut(strings.TrimSpace(msg), "\n")
			draft := &commitDraft{summary: summary, description: strings.TrimSpace(body)}
			a.pushOverlay(&commitOverlay{title: a.repo.Common.Tr.Actions.RewordCommit, draft: draft, onSubmit: func(s, b string) {
				a.run("Rewording…", func() error { return a.repo.Reword(commits, i, s, b) }, a.done("Commit reworded"))
			}})
		})
	})
}

// squashOrFixup is lazygit's s / f in commits, with its canSquashOrFixup
// disabled reasons shown as an error toast.
func (a *App) squashOrFixup(action string) {
	if a.selectedCommit() == nil {
		return
	}
	tr := a.repo.Common.Tr
	start, end := a.selectionRange(panelCommits)
	reason := ""
	if end >= len(a.snap.Commits)-1 {
		reason = tr.CannotSquashOrFixupFirstCommit
	} else {
		for _, c := range a.snap.Commits[start : end+1] {
			if c.IsMerge() {
				reason = tr.CannotSquashOrFixupMergeCommit
			}
		}
	}
	if reason != "" {
		a.setStatus(tr.DisabledMenuItemPrefix+reason, true)
		return
	}
	if action == "fixup" && !a.selectedCommit().IsTODO() && a.snap.Operation != "Rebasing" {
		a.fixupMenu()
		return
	}
	a.rebaseAction(action)
}

func (a *App) selectCommitHash(hash string) {
	for i, c := range a.snap.Commits {
		if c.Hash() == hash {
			a.cursor[panelCommits] = i
			a.lists[panelCommits].ScrollIntoView(i)
			a.loadMain()
			return
		}
	}
}

func (a *App) quickStartRebase() {
	tr := a.repo.Common.Tr
	if a.snap.Operation != "" {
		a.setStatus(tr.DisabledMenuItemPrefix+tr.AlreadyRebasing, true)
		return
	}
	for i, c := range a.snap.Commits {
		if c.IsMerge() || c.Status == models.StatusMerged {
			if i == 0 {
				break
			}
			hash := a.selectedCommit().Hash()
			a.run(tr.RebasingStatus, func() error { return a.repo.StartRebase(c.Hash()) }, func() { a.selectCommitHash(hash) })
			return
		}
	}
	a.setStatus(tr.DisabledMenuItemPrefix+utils.ResolvePlaceholderString(tr.CannotQuickStartInteractiveRebase, map[string]string{"editKey": a.repo.Config.Keybinding.Universal.Edit.String()}), true)
}

func (a *App) operationMenu() {
	if a.snap == nil || a.snap.Operation == "" {
		a.setStatus(a.repo.Common.Tr.NotMergingOrRebasing, true)
		return
	}
	kind := a.snap.OperationCommand
	items := []menuItem{}
	options := []string{"continue", "abort"}
	if a.snap.CanSkip {
		options = append(options, "skip")
	}
	for _, action := range options {
		items = append(items, menuItem{label: action, key: action[:1], chord: keymap.Binding{{Rune: rune(action[0])}}, run: func() {
			a.selectHeadAfterMerge(action)
			a.run("Updating…", func() error { return a.repo.SequenceControl(kind, action) }, nil)
		}})
	}
	a.pushOverlay(&menuOverlay{title: a.snap.OperationTitle, items: items})
}

func (a *App) fixupMessageMenu() {
	c := a.selectedCommit()
	if c == nil {
		return
	}
	tr := a.repo.Common.Tr
	reason := ""
	if a.snap.Operation != "Rebasing" {
		reason = tr.NotMidRebase
	} else if c.Action != todo.Fixup {
		reason = tr.MustSelectFixupCommit
	}
	if reason != "" {
		a.setStatus(tr.DisabledMenuItemPrefix+reason, true)
		return
	}
	a.pushOverlay(&menuOverlay{title: tr.SetFixupMessage, items: []menuItem{
		{label: tr.FixupDiscardMessage, key: "f", chord: keymap.Binding{{Rune: 'f'}}, run: func() { a.run(tr.FixingStatus, func() error { return a.repo.FixupMessage(c, "") }, nil) }},
		{label: tr.FixupKeepMessage, key: "c", chord: keymap.Binding{{Rune: 'c'}}, run: func() { a.run(tr.FixingStatus, func() error { return a.repo.FixupMessage(c, "-C") }, nil) }},
	}})
}

// rewordInEditorFlow is lazygit's LocalCommitsController.rewordEditor.
func (a *App) rewordInEditorFlow() {
	c := a.selectedCommit()
	if c == nil || a.selectingRange(panelCommits) {
		return
	}
	tr := a.repo.Common.Tr
	commits, i := a.snap.Commits, a.cursor[panelCommits]
	if c.IsTODO() {
		a.setStatus(tr.DisabledMenuItemPrefix+tr.RewordNotSupported, true)
		return
	}
	if a.snap.Operation == "Rebasing" && !models.IsHeadCommit(commits, i) {
		a.setStatus(tr.DisabledMenuItemPrefix+tr.AlreadyRebasing, true)
		return
	}
	do := func() {
		a.runSubprocess(func() (string, error) {
			if models.IsHeadCommit(commits, i) {
				return a.subprocessCommand(a.repo.Git.Commit.RewordLastCommitInEditorCmdObj()), nil
			}
			var cmd string
			err := a.repo.Locked(func() error {
				sub, err := a.repo.Git.Rebase.RewordCommitInEditor(commits, i)
				if sub != nil {
					cmd = a.subprocessCommand(sub)
				}
				return err
			})
			return cmd, err
		})
	}
	if a.repo.Config.Gui.SkipRewordInEditorWarning {
		do()
		return
	}
	a.confirm(tr.RewordInEditorTitle, tr.RewordInEditorPrompt, false, do)
}

// runSubprocess is lazygit's RunSubprocessAndRefresh: the command runs in a
// terminal; a failure opens an error alert; the repo refreshes either way.
func (a *App) runSubprocess(prepare func() (string, error)) {
	a.bg(func() {
		cmd, err := prepare()
		if err == nil && cmd != "" {
			err = a.runTerminal(cmd)
		}
		s := a.repo.Load()
		a.post(func() {
			a.apply(s)
			if err != nil {
				a.errorPopup(err.Error())
			}
		})
	})
}

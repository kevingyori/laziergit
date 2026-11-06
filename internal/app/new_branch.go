package app

import (
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"
)

// suggestedBranchName is lazygit's RefsHelper.getSuggestedBranchName.
func (a *App) suggestedBranchName() (string, error) {
	s, err := utils.ResolveTemplate(a.repo.Config.Git.BranchPrefix, nil, template.FuncMap{
		"runCommand": a.repo.Git.Custom.TemplateFunctionRunCommand,
	})
	return strings.ReplaceAll(s, "\t", " "), err
}

// newBranchFrom is lazygit's RefsHelper.NewBranch.
func (a *App) newBranchFrom(from, fromName, suggested string) {
	tr := a.repo.Common.Tr
	if suggested == "" {
		var err error
		if suggested, err = a.suggestedBranchName(); err != nil {
			a.errorPopup(err.Error())
			return
		}
	}
	title := utils.ResolvePlaceholderString(tr.NewBranchNameBranchOff, map[string]string{"branchName": fromName})
	a.prompt(title, suggested, "", func(response string) {
		name := strings.ReplaceAll(response, " ", "-") // SanitizedBranchName
		track := name == suggested
		create := func() error { return a.repo.NewBranchFromRemote(name, from, track) }
		a.nextCommitSel = selectHeadCommit
		a.autostashing(tr.CreatingBranchStatus, fmt.Sprintf(tr.AutoStashForNewBranch, name), create, func() {
			a.refBrowse = nil
			a.branchTab = 0
			a.cursor[panelBranches] = 0
			a.lists[panelBranches].ScrollTo(0, 0)
			if a.focus != panelBranches || a.commitFiles != nil {
				a.commitFiles = nil
				a.focusPanel(panelBranches)
			}
			a.diffKey = ""
			a.loadMain()
		})
	})
}

func (a *App) newBranchFromCommit() {
	if c := a.selectedCommit(); c != nil {
		a.newBranchFrom(c.RefName(), c.Description(), "")
	}
}

// moveCommitsToNewBranch is lazygit's RefsHelper.MoveCommitsToNewBranch.
func (a *App) moveCommitsToNewBranch() {
	tr := a.repo.Common.Tr
	if a.snap == nil || len(a.snap.Branches) == 0 {
		a.setStatus(tr.DisabledMenuItemPrefix+tr.NoBranchesThisRepo, true)
		return
	}
	current := a.snap.Branches[0]
	// CanMoveCommitsToNewBranch
	switch {
	case current.DetachedHead:
		a.errorPopup(tr.CannotMoveCommitsFromDetachedHead)
		return
	case !current.RemoteBranchStoredLocally():
		a.errorPopup(tr.CannotMoveCommitsNoUpstream)
		return
	case current.IsBehindForPull():
		a.errorPopup(tr.CannotMoveCommitsBehindUpstream)
		return
	case !current.IsAheadForPull():
		a.errorPopup(tr.CannotMoveCommitsNoUnpushedCommits)
		return
	}
	baseRef, err := a.repo.BaseBranch(current)
	if err != nil {
		a.errorPopup(err.Error())
		return
	}
	mustStash := a.workingTreeDirty()
	git := a.repo.Git
	withPrompt := func(baseName string, f func(name string) error) {
		suggested, err := a.suggestedBranchName()
		if err != nil {
			a.errorPopup(err.Error())
			return
		}
		title := utils.ResolvePlaceholderString(tr.NewBranchNameBranchOff, map[string]string{"branchName": baseName})
		a.prompt(title, suggested, "", func(response string) {
			name := strings.ReplaceAll(response, " ", "-")
			a.nextCommitSel = selectHeadCommit
			a.run(tr.MovingCommitsToNewBranchStatus, func() error { return a.repo.Locked(func() error { return f(name) }) }, func() {
				a.cursor[panelBranches] = 0
				a.lists[panelBranches].ScrollTo(0, 0)
				a.diffKey = ""
				a.loadMain()
			})
		})
	}
	stash := func(name string) error {
		if mustStash {
			return git.Stash.Push(fmt.Sprintf(tr.AutoStashForNewBranch, name))
		}
		return nil
	}
	pop := func() error {
		if mustStash {
			return git.Stash.Pop(0)
		}
		return nil
	}
	stacked := func(name string) error {
		if err := git.Branch.NewWithoutCheckout(name, "HEAD"); err != nil {
			return err
		}
		if err := stash(name); err != nil {
			return err
		}
		if err := git.Commit.ResetToCommit("@{u}", "hard", []string{}); err != nil {
			return err
		}
		if err := git.Branch.Checkout(name, git_commands.CheckoutOptions{}); err != nil {
			return err
		}
		return pop()
	}
	if slices.Contains(a.repo.Config.Git.MainBranches, current.Name) {
		prompt := utils.ResolvePlaceholderString(tr.MoveCommitsToNewBranchFromMainPrompt, map[string]string{"baseBranchName": current.Name})
		a.confirm(tr.MoveCommitsToNewBranch, prompt, false, func() { withPrompt(current.Name, stacked) })
		return
	}
	short := strings.TrimPrefix(strings.TrimPrefix(baseRef, "refs/heads/"), "refs/remotes/") // ShortBranchName
	toPick := lo.Filter(a.snap.Commits, func(c *models.Commit, _ int) bool { return c.Status == models.StatusUnpushed })
	offMain := func(name string) error {
		if err := stash(name); err != nil {
			return err
		}
		if err := git.Commit.ResetToCommit("@{u}", "hard", []string{}); err != nil {
			return err
		}
		if err := git.Branch.NewWithoutTracking(name, baseRef); err != nil {
			return err
		}
		if err := git.Rebase.CherryPickCommits(toPick); err != nil {
			return err
		}
		return pop()
	}
	a.pushOverlay(&menuOverlay{title: tr.MoveCommitsToNewBranch,
		body: utils.ResolvePlaceholderString(tr.MoveCommitsToNewBranchMenuPrompt, map[string]string{"baseBranchName": short}),
		items: []menuItem{
			{label: fmt.Sprintf(tr.MoveCommitsToNewBranchFromBaseItem, short), run: func() { withPrompt(short, offMain) }},
			{label: fmt.Sprintf(tr.MoveCommitsToNewBranchStackedItem, current.Name), run: func() { withPrompt(current.Name, stacked) }},
		}})
}

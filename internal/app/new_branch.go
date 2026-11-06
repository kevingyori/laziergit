package app

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/jesseduffield/lazygit/pkg/utils"
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

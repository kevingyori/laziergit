package app

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

func (a *App) selectedRemote() *models.Remote {
	if a.snap == nil || a.focus != panelBranches || a.branchTab != 1 || a.browse(panelBranches) != nil || a.count(panelBranches) == 0 {
		return nil
	}
	return a.snap.Remotes[a.cursor[panelBranches]]
}

// addRemoteFlow is lazygit's RemotesController.add.
func (a *App) addRemoteFlow() {
	tr := a.repo.Common.Tr
	a.prompt(tr.NewRemoteName, "", "", func(name string) {
		a.prompt(tr.NewRemoteUrl, "", "", func(url string) {
			a.addAndCheckoutRemote(name, url, "")
		})
	})
}

// addAndCheckoutRemote adds a remote, selects it, then fetches it and checks
// out branch when given.
func (a *App) addAndCheckoutRemote(name, url, branch string) {
	a.run(a.repo.Common.Tr.FetchingStatus, func() error { return a.repo.AddRemote(name, url) }, func() {
		a.selectRemote(name)
		a.fetchAndCheckout(name, branch)
	})
}

func (a *App) selectRemote(name string) {
	if a.snap == nil || a.branchTab != 1 {
		return
	}
	for i, r := range a.snap.Remotes {
		if r.Name == name {
			a.cursor[panelBranches] = i
			a.lists[panelBranches].ScrollIntoView(i)
			a.diffKey = ""
			a.loadMain()
		}
	}
}

// fetchAndCheckout is lazygit's RemotesController.fetchAndCheckout.
func (a *App) fetchAndCheckout(remote, branch string) {
	checkedOut := false
	a.run(a.repo.Common.Tr.FetchingStatus, func() error {
		var err error
		checkedOut, err = a.repo.FetchRemote(remote, branch)
		return err
	}, func() {
		if checkedOut {
			a.tabCursor[1][0] = 0
			if a.branchTab != 0 && a.browse(panelBranches) == nil && a.focus == panelBranches {
				a.switchBranchTab(-a.branchTab)
			}
			a.cursor[panelBranches] = 0
			a.lists[panelBranches].ScrollTo(0, 0)
			a.commitTabCursor = [2]int{}
			a.cursor[panelCommits] = 0
			a.focusPanel(panelBranches)
		}
	})
}

func (a *App) fetchRemoteFlow() {
	if r := a.selectedRemote(); r != nil {
		a.fetchAndCheckout(r.Name, "")
	}
}

// removeRemoteFlow is lazygit's RemotesController.remove.
func (a *App) removeRemoteFlow() {
	r := a.selectedRemote()
	if r == nil {
		return
	}
	tr := a.repo.Common.Tr
	a.confirm(tr.RemoveRemote, tr.RemoveRemotePrompt+" '"+r.Name+"'?", true, func() {
		a.run(tr.DeletingStatus, func() error { return a.repo.RemoveRemote(r.Name) }, nil)
	})
}

// editRemoteFlow is lazygit's RemotesController.edit.
func (a *App) editRemoteFlow() {
	r := a.selectedRemote()
	if r == nil {
		return
	}
	tr := a.repo.Common.Tr
	title := utils.ResolvePlaceholderString(tr.EditRemoteName, map[string]string{"remoteName": r.Name})
	a.prompt(title, r.Name, "", func(name string) {
		edit := func() {
			url := ""
			if len(r.Urls) > 0 {
				url = r.Urls[0]
			}
			title := utils.ResolvePlaceholderString(tr.EditRemoteUrl, map[string]string{"remoteName": name})
			a.prompt(title, url, "", func(newURL string) {
				a.run("Updating…", func() error { return a.repo.UpdateRemoteURL(name, newURL) }, nil)
			})
		}
		if name == r.Name {
			edit()
			return
		}
		a.run("Updating…", func() error { return a.repo.RenameRemote(r.Name, name) }, edit)
	})
}

var (
	forkURLRegex            = regexp.MustCompile(`^(git@[^:]+:|ssh://[^/]+/|https?://[^/]+/)([^/]+(?:/[^/]+)*)/([^/]+?)(\.git)?$`)
	integrationTestURLRegex = regexp.MustCompile(`^\.\./.+$`)
)

// replaceForkUsername is lazygit's: integration tests use sibling paths.
func replaceForkUsername(originURL, user string, integrationTest bool) (string, error) {
	if forkURLRegex.MatchString(originURL) {
		return forkURLRegex.ReplaceAllString(originURL, "${1}"+user+"/$3$4"), nil
	} else if integrationTest && integrationTestURLRegex.MatchString(originURL) {
		return "../" + user, nil
	}
	return "", fmt.Errorf("unsupported or invalid remote URL: %s", originURL)
}

// runningIntegrationTest is lazygit's RunningIntegrationTest: true while
// lazygit's integration tests (run from Go tests) drive the app.
func runningIntegrationTest() bool { return testing.Testing() }

// addForkRemoteFlow is lazygit's RemotesController.addFork.
func (a *App) addForkRemoteFlow() {
	tr := a.repo.Common.Tr
	var origin *models.Remote
	if a.snap != nil {
		for _, r := range a.snap.Remotes {
			if r.Name == "origin" {
				origin = r
			}
		}
	}
	if origin == nil {
		a.setStatus(tr.DisabledMenuItemPrefix+tr.NoOriginRemote, true)
		return
	}
	a.prompt(tr.AddForkRemoteUsername, "", "", func(user string) {
		branch := ""
		if parts := strings.SplitN(user, ":", 2); len(parts) == 2 {
			user, branch = parts[0], parts[1]
		}
		url, err := replaceForkUsername(origin.Urls[0], user, runningIntegrationTest())
		if err != nil {
			a.showError(tr.AddForkRemote, err)
			return
		}
		for _, r := range a.snap.Remotes {
			if r.Name == user {
				if !slices.Contains(r.Urls, url) {
					a.showError(tr.AddForkRemote, errors.New(utils.ResolvePlaceholderString(tr.IncompatibleForkAlreadyExistsError, map[string]string{"remoteName": user})))
					return
				}
				a.selectRemote(user)
				a.fetchAndCheckout(user, branch)
				return
			}
		}
		a.addAndCheckoutRemote(user, url, branch)
	})
}

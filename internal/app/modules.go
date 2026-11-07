package app

import (
	"errors"

	"fmt"
	"github.com/jesseduffield/lazygit/pkg/commands/direnv"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

func (a *App) selectedWorktree() *models.Worktree {
	if a.snap == nil || a.fileTab != 1 || a.count(panelFiles) == 0 {
		return nil
	}
	return a.snap.Worktrees[a.cursor[panelFiles]]
}
func (a *App) selectedSubmodule() *models.SubmoduleConfig {
	if a.snap == nil || a.fileTab != 2 || a.count(panelFiles) == 0 {
		return nil
	}
	return a.snap.Submodules[a.cursor[panelFiles]]
}

func (a *App) openSubmodule() {
	if s := a.selectedSubmodule(); s != nil {
		path := filepath.Join(a.snap.Root, s.FullPath())
		r := a.repo
		// An uninitialized submodule directory otherwise resolves to its parent repo.
		a.bg(func() {
			_, err := os.Stat(filepath.Join(path, ".git"))
			a.post(func() {
				if a.repo != r {
					return
				}
				if err != nil {
					a.showError("Initialize the submodule first", err)
					return
				}
				a.openRepository(path, true)
			})
		})
	}
}

// openRepository is lazygit's ReposHelper.switchToLocation and onNewRepo:
// the repo opens with the same config and app state, reloading the per-repo
// config, and each repo keeps the focus and selections it was left with.
func (a *App) openRepository(path string, child bool) {
	if a.busy != "" {
		return
	}
	old := a.repo
	root := a.snap.Root
	a.switching = true
	a.busy = "Opening repository…"
	a.logLine(utils.ResolvePlaceholderString(old.Common.Tr.ChangingDirectoryTo, map[string]string{"path": path}))
	a.bg(func() {
		r, err := old.OpenOther(path)
		var env direnv.LoadResult
		if err != nil {
			_ = os.Chdir(root)
			if os.IsNotExist(err) {
				err = errors.New(old.Common.Tr.ErrRepositoryMovedOrDeleted)
			}
		} else {
			if abs, aerr := filepath.Abs(path); aerr == nil {
				_ = os.Setenv("PWD", abs) // as a shell's cd: Getwd keeps the path we were given
			}
			env = a.loadDirenv(r)
			updateRecentRepoList(r)
		}
		a.post(func() {
			a.busy = ""
			a.switching = false
			if err != nil {
				a.switchFocus = ""
				a.showError("Open repository", err)
				return
			}
			if child {
				a.parents = append(a.parents, root)
			} else if len(a.parents) > 0 && a.parents[len(a.parents)-1] == path {
				a.parents = a.parents[:len(a.parents)-1]
			}
			a.saveRepoUIState(root)
			a.closeGuide()
			a.guideReturn = nil
			a.repo = r
			a.connectRepo()
			a.keys = loadBindings(r)
			a.actions = a.buildActions()
			a.snap, a.raw, a.srch, a.fidx = nil, nil, nil, nil
			a.cursor = [panelCount]int{}
			a.ranges = [panelCount]listRange{}
			a.lists = [panelCount]ui.ListState{}
			a.fileTab, a.branchTab, a.commitTab = 0, 0, 0
			a.commitTabCursor, a.tabCursor = [2]int{}, [2][3]int{}
			a.commitFiles, a.staging = nil, nil
			a.refBrowse = nil
			a.prs, a.prKey = nil, ""
			a.draft = commitDraft{}
			a.diffGen++
			a.diffKey, a.shownKey = "", ""
			a.diff = nil
			a.mainFocused = false
			a.focus = panelFiles
			a.restoreRepoUIState(r.Git.RepoPaths.WorktreePath())
			switch a.switchFocus {
			case "worktrees":
				a.focus, a.fileTab = panelFiles, 1
			case "localBranches":
				a.focus, a.branchTab = panelBranches, 0
				a.cursor[panelBranches] = 0 // the checked-out branch
				a.pendingBranch = ""
			}
			a.switchFocus = ""
			a.Refresh()
			if env.Blocked {
				a.promptDirenvApproval(env.EnvrcPath)
			} else if env.Err != nil {
				a.showError("direnv", env.Err)
			}
		})
	})
}
func (a *App) returnToParent() {
	if len(a.parents) == 0 || a.busy != "" {
		return
	}
	path := a.parents[len(a.parents)-1]
	a.openRepository(path, false)
}

func (a *App) addSubmoduleFlow() {
	tr := a.repo.Common.Tr
	a.prompt(tr.NewSubmoduleUrl, "", "", func(url string) {
		suggestion := filepath.Base(strings.TrimSuffix(url, filepath.Ext(url)))
		a.prompt(tr.NewSubmoduleName, suggestion, "", func(name string) {
			a.prompt(tr.NewSubmodulePath, name, "", func(path string) {
				a.run(tr.AddingSubmoduleStatus, func() error { return a.repo.AddSubmodule(url, name, path) }, nil)
			})
		})
	})
}
func (a *App) initSubmodule() {
	if s := a.selectedSubmodule(); s != nil {
		a.run("Initializing submodule…", func() error { return a.repo.InitSubmodule(s.Path) }, nil)
	}
}
func (a *App) updateSubmodule() {
	if s := a.selectedSubmodule(); s != nil {
		a.run("Updating submodule…", func() error { return a.repo.UpdateSubmodule(s.Path) }, nil)
	}
}
func (a *App) submoduleURLFlow() {
	if s := a.selectedSubmodule(); s != nil {
		a.prompt(fmt.Sprintf(a.repo.Common.Tr.UpdateSubmoduleUrl, s.FullName()), s.Url, "", func(url string) {
			if strings.TrimSpace(url) != "" {
				a.run("Updating submodule URL…", func() error { return a.repo.UpdateSubmoduleURL(s, url) }, nil)
			}
		})
	}
}
func (a *App) deleteSubmoduleFlow() {
	if s := a.selectedSubmodule(); s != nil {
		a.confirm(a.repo.Common.Tr.RemoveSubmodule, fmt.Sprintf(a.repo.Common.Tr.RemoveSubmodulePrompt, s.FullName()), true, func() {
			a.run("Removing submodule…", func() error { return a.repo.DeleteSubmodule(s) }, nil)
		})
	}
}

// Close releases resources for the repository currently shown.
func (a *App) Close() { a.closeGuide(); a.repo.Close() }

func (a *App) submoduleBulkMenu() {
	a.pushOverlay(&menuOverlay{title: "Submodules", items: []menuItem{
		{label: "Initialize all", run: func() {
			a.run("Initializing submodules…", func() error { return a.repo.BulkSubmodules("init") }, nil)
		}},
		{label: "Update all", run: func() { a.run("Updating submodules…", func() error { return a.repo.BulkSubmodules("update") }, nil) }},
		{label: "Initialize and update recursively", run: func() {
			a.run("Updating submodules recursively…", func() error { return a.repo.BulkSubmodules("recursive") }, nil)
		}},
	}})
}

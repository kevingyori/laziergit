package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
)

// ignoreOrExcludeMenu is lazygit's menu for `i` in files.
func (a *App) ignoreOrExcludeMenu() {
	n := a.selectedFileNode()
	if n == nil {
		return
	}
	tr := a.repo.Common.Tr
	wt := a.repo.Git.WorkingTree
	a.pushOverlay(&menuOverlay{title: tr.Actions.IgnoreExcludeFile, items: []menuItem{
		{label: tr.IgnoreFile, key: "i", chord: runeBind('i'), run: func() {
			if n.GetPath() == ".gitignore" {
				a.errorPopup(tr.Actions.IgnoreFileErr)
				return
			}
			a.ignoreOrExclude(n, tr.IgnoreTracked, tr.IgnoreTrackedPrompt, wt.Ignore)
		}},
		{label: tr.ExcludeFile, key: "e", chord: runeBind('e'), run: func() {
			if n.GetPath() == ".gitignore" {
				a.errorPopup(tr.Actions.ExcludeGitIgnoreErr)
				return
			}
			a.ignoreOrExclude(n, tr.ExcludeTracked, tr.ExcludeTrackedPrompt, wt.Exclude)
		}},
	}})
}

func (a *App) ignoreOrExclude(n *filetree.FileNode, title, prompt string, add func(string) error) {
	wt := a.repo.Git.WorkingTree
	do := func(tracked bool) func() {
		return func() {
			a.run("", func() error {
				return a.repo.Locked(func() error {
					if tracked {
						// not 100% sure this is necessary, but lazygit does it
						err := n.ForEachFile(func(f *models.File) error {
							if f.HasStagedChanges {
								return wt.UnStageFile(f.Names(), f.Tracked)
							}
							return nil
						})
						if err != nil {
							return err
						}
						if err := wt.RemoveTrackedFiles(n.GetPath()); err != nil {
							return err
						}
					}
					return add(n.GetPath())
				})
			}, nil)
		}
	}
	if n.GetIsTracked() {
		a.confirm(title, prompt, false, do(true))
		return
	}
	do(false)()
}

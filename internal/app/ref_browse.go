package app

import "github.com/jesseduffield/lazygit/pkg/commands/models"

type refBrowseState struct {
	remote       string
	branches     []*models.RemoteBranch
	ref          models.Ref
	commits      []*models.Commit
	parentCursor int
	remoteCursor int
}

func (a *App) enterRef() {
	if a.snap == nil || a.count(panelBranches) == 0 {
		return
	}
	if a.refBrowse != nil && a.refBrowse.ref == nil {
		a.loadRefCommits(a.refBrowse.branches[a.cursor[panelBranches]])
		return
	}
	if a.refBrowse != nil {
		return
	}
	switch a.branchTab {
	case 0:
		a.loadRefCommits(a.snap.Branches[a.cursor[panelBranches]])
	case 2:
		a.loadRefCommits(a.snap.Tags[a.cursor[panelBranches]])
	case 1:
		name, cursor, r := a.snap.Remotes[a.cursor[panelBranches]].Name, a.cursor[panelBranches], a.repo
		a.bg(func() {
			branches, err := r.RemoteBranches(name)
			a.post(func() {
				if a.repo != r || a.focus != panelBranches || a.refBrowse != nil || a.branchTab != 1 {
					return
				}
				if err != nil {
					a.showError("Remote branches", err)
					return
				}
				a.refBrowse = &refBrowseState{remote: name, branches: branches, parentCursor: cursor}
				a.cursor[panelBranches] = 0
				a.lists[panelBranches].ScrollTo(0, 0)
				a.diffKey = ""
				a.loadMain()
			})
		})
	}
}

func (a *App) loadRefCommits(ref models.Ref) {
	r, cursor, old := a.repo, a.cursor[panelBranches], a.refBrowse
	a.bg(func() {
		commits, err := r.RefCommits(ref)
		a.post(func() {
			if a.repo != r || a.focus != panelBranches || a.refBrowse != old {
				return
			}
			if err != nil {
				a.showError("Commits", err)
				return
			}
			if old == nil {
				a.refBrowse = &refBrowseState{parentCursor: cursor}
			} else {
				old.remoteCursor = cursor
			}
			a.refBrowse.ref = ref
			a.refBrowse.commits = commits
			a.cursor[panelBranches] = 0
			a.lists[panelBranches].ScrollTo(0, 0)
			a.diffKey = ""
			a.loadMain()
		})
	})
}

func (a *App) backRef() {
	s := a.refBrowse
	if s.ref != nil && s.remote != "" {
		s.ref = nil
		s.commits = nil
		a.cursor[panelBranches] = s.remoteCursor
	} else {
		a.refBrowse = nil
		a.cursor[panelBranches] = s.parentCursor
	}
	a.lists[panelBranches].ScrollIntoView(a.cursor[panelBranches])
	a.diffKey = ""
	a.loadMain()
}

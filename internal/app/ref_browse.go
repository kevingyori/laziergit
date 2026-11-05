package app

import (
	"fmt"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

type refBrowseState struct {
	remote         string
	title          string
	divergenceFrom string
	branches       []*models.RemoteBranch // what is shown: all, filtered
	all            []*models.RemoteBranch
	ref            models.Ref
	commits        []*models.Commit
	parentCursor   int
	remoteCursor   int
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
				a.refBrowse = &refBrowseState{remote: name, branches: branches, all: branches, parentCursor: cursor}
				a.tabCursor[1][a.branchTab] = cursor
				a.cursor[panelBranches] = 0
				a.lists[panelBranches].ScrollTo(0, 0)
				a.diffKey = ""
				a.loadMain()
			})
		})
	}
}

func (a *App) loadRefCommits(ref models.Ref) {
	a.loadRefCommitsDiverging(ref, "", "")
}

func (a *App) loadRefCommitsDiverging(ref models.Ref, from, title string) {
	r, cursor, old := a.repo, a.cursor[panelBranches], a.refBrowse
	a.bg(func() {
		commits, err := r.RefCommitsDiverging(ref, from)
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
				a.tabCursor[1][a.branchTab] = cursor // the tab keeps its place while a drill-down is open
			} else {
				old.remoteCursor = cursor
			}
			a.refBrowse.title, a.refBrowse.divergenceFrom = title, from
			a.refBrowse.ref = ref
			a.refBrowse.commits = commits
			a.noteAuthors(commits)
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

// subCommitsTitle is lazygit's sub-commits title: the name of the ref the
// commits were opened for, or the comparison they show, truncated as
// lazygit's TitleRef is.
func (a *App) subCommitsTitle() string {
	s := a.refBrowse
	ref := s.title
	if ref == "" {
		ref = s.ref.RefName()
	}
	return fmt.Sprintf(a.repo.Common.Tr.SubCommitsDynamicTitle, utils.TruncateWithEllipsis(ref, 50))
}

// noteAuthors records the authors of commits as lazygit's RefreshAuthors does:
// the first name seen for each email is kept, and authors are never forgotten.
func (a *App) noteAuthors(commits []*models.Commit) {
	if a.authors == nil {
		a.authors = map[string]string{}
	}
	for _, c := range commits {
		if _, ok := a.authors[c.AuthorEmail]; !ok {
			a.authors[c.AuthorEmail] = c.AuthorName + " <" + c.AuthorEmail + ">"
		}
	}
}

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
	// panel is the window the drill-down replaces: lazygit moves its one
	// subCommits view into the window of the context it was opened from.
	panel panel
	// showBranchHeads is SubCommitsContext's ShowBranchHeadsInSubCommits:
	// false for a reflog entry.
	showBranchHeads bool
}

// browse is the drill-down when it is open in panel p.
func (a *App) browse(p panel) *refBrowseState {
	if a.refBrowse != nil && a.refBrowse.panel == p {
		return a.refBrowse
	}
	return nil
}

// subCommits is the subCommits drill-down of the focused panel, if any.
func (a *App) subCommits() *refBrowseState {
	if b := a.browse(a.focus); b != nil && b.ref != nil {
		return b
	}
	return nil
}

func (a *App) enterRef() {
	if a.snap == nil || a.count(panelBranches) == 0 {
		return
	}
	if a.focus == panelCommits {
		if a.commitTab == 1 && a.browse(panelCommits) == nil && len(a.snap.Reflog) > 0 {
			a.loadRefCommitsIn(panelCommits, a.snap.Reflog[a.cursor[panelCommits]], "", "")
		}
		return
	}
	if b := a.browse(panelBranches); b != nil && b.ref == nil {
		a.loadRefCommits(b.branches[a.cursor[panelBranches]])
		return
	}
	if a.browse(panelBranches) != nil {
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
				if a.repo != r || a.focus != panelBranches || a.browse(panelBranches) != nil || a.branchTab != 1 {
					return
				}
				if err != nil {
					a.showError("Remote branches", err)
					return
				}
				a.refBrowse = &refBrowseState{remote: name, branches: branches, all: branches, parentCursor: cursor, panel: panelBranches}
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
	a.loadRefCommitsIn(panelBranches, ref, from, title)
}

// loadRefCommitsIn opens the subCommits drill-down for ref in panel p,
// moving it there from wherever it was open (lazygit's ViewSubCommits).
func (a *App) loadRefCommitsIn(p panel, ref models.Ref, from, title string) {
	r, cursor, old := a.repo, a.cursor[p], a.browse(p)
	a.bg(func() {
		commits, err := r.RefCommitsDiverging(ref, from)
		a.post(func() {
			if a.repo != r || a.focus != p || a.browse(p) != old {
				return
			}
			if err != nil {
				a.showError("Commits", err)
				return
			}
			if old == nil {
				if moved := a.refBrowse; moved != nil {
					a.closeBrowse(moved)
				}
				a.refBrowse = &refBrowseState{parentCursor: cursor, panel: p, showBranchHeads: p == panelBranches}
				if p == panelBranches {
					a.tabCursor[1][a.branchTab] = cursor // the tab keeps its place while a drill-down is open
				} else {
					a.commitTabCursor[a.commitTab] = cursor
				}
				delete(a.srch, "subCommits") // the moved view's search is cancelled
			} else {
				old.remoteCursor = cursor
			}
			a.refBrowse.title, a.refBrowse.divergenceFrom = title, from
			a.refBrowse.ref = ref
			a.refBrowse.commits = commits
			a.noteAuthors(commits)
			a.cursor[p] = 0
			a.lists[p].ScrollTo(0, 0)
			a.diffKey = ""
			a.loadMain()
		})
	})
}

// closeBrowse closes a drill-down that is being moved to another window:
// the window it leaves shows its tab again, at the place the tab had.
func (a *App) closeBrowse(s *refBrowseState) {
	a.refBrowse = nil
	if cf := a.commitFiles; cf != nil && cf.fromSub && s.panel == panelCommits {
		a.commitFiles = nil
	}
	if s.panel == panelBranches {
		a.cursor[panelBranches] = clamp(a.tabCursor[1][a.branchTab], 0, max(a.count(panelBranches)-1, 0))
	} else {
		a.cursor[panelCommits] = clamp(a.commitTabCursor[a.commitTab], 0, max(a.count(panelCommits)-1, 0))
	}
	a.lists[s.panel].ScrollIntoView(a.cursor[s.panel])
}

func (a *App) backRef() {
	s := a.refBrowse
	if s.ref != nil && s.remote != "" {
		s.ref = nil
		s.commits = nil
		a.cursor[s.panel] = s.remoteCursor
	} else {
		a.refBrowse = nil
		a.cursor[s.panel] = s.parentCursor
	}
	a.lists[s.panel].ScrollIntoView(a.cursor[s.panel])
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

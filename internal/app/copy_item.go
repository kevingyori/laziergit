package app

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/utils"
)

// copySelectedItem is lazygit's universal copyToClipboard on a side list:
// copy the selected item's id and toast it.
func (a *App) copySelectedItem() {
	id := a.selectedItemID()
	if id == "" {
		return
	}
	tr := a.repo.Common.Tr
	if err := a.copyText(id); err != nil {
		a.showError(tr.Actions.CopyToClipboard, err)
		return
	}
	a.setStatus(fmt.Sprintf("'%s' %s", utils.TruncateWithEllipsis(strings.ReplaceAll(id, "\n", " "), 50), tr.CopiedToClipboard), false)
}

func (a *App) selectedItemID() string {
	if a.snap == nil || a.mainFocused || a.staging != nil || a.count(a.focus) == 0 {
		return ""
	}
	i := a.cursor[a.focus]
	switch a.focus {
	case panelBranches:
		if a.refBrowse != nil {
			return ""
		}
		switch a.branchTab {
		case 0:
			return a.snap.Branches[i].ID()
		case 1:
			return a.snap.Remotes[i].ID()
		case 2:
			return a.snap.Tags[i].ID()
		}
	case panelCommits:
		if c := a.selectedCommit(); c != nil && a.commitFiles == nil {
			return c.ID()
		}
	}
	return ""
}

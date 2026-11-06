package app

import (
	"fmt"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"

	"laziergit/internal/keymap"
)

// commitsView is what the commits window shows: lazygit's commits and
// reflogCommits tabs, and the transient subCommits and commitFiles views.
type commitsView int

const (
	cvCommits commitsView = iota
	cvReflog
	cvSubCommits
	cvCommitFiles
)

func (a *App) commitsView() commitsView {
	switch {
	case a.commitFiles != nil:
		return cvCommitFiles
	case a.browse(panelCommits) != nil:
		return cvSubCommits
	case a.commitTab == 1:
		return cvReflog
	}
	return cvCommits
}

func (a *App) commitTabTitle() string {
	return []string{"Commits", "Reflog"}[a.commitTab]
}

// switchCommitTab is ] / [ in the commits window. A transient view (commit
// files, sub-commits) is not a tab, so the keys leave it for the first tab.
func (a *App) switchCommitTab(delta int) {
	a.mainFocused = false
	if a.commitFiles != nil || a.browse(panelCommits) != nil {
		a.commitFiles = nil
		if a.browse(panelCommits) != nil {
			a.refBrowse = nil
		}
		a.commitTab = 0
	} else {
		a.commitTabCursor[a.commitTab] = a.cursor[panelCommits]
		a.commitTab = (a.commitTab + delta + 2) % 2
	}
	a.cursor[panelCommits] = clamp(a.commitTabCursor[a.commitTab], 0, max(a.count(panelCommits)-1, 0))
	a.lists[panelCommits].ScrollIntoView(a.cursor[panelCommits])
	a.diffKey = ""
	a.loadMain()
}

// localCommitIndex is the selected row of the commits tab, whatever the
// window shows now.
func (a *App) localCommitIndex() int {
	switch {
	case a.commitsView() == cvCommits:
		return a.cursor[panelCommits]
	case a.commitFiles != nil && !a.commitFiles.fromSub && a.commitFiles.tab == 0:
		return a.commitFiles.commitIdx
	}
	return a.commitTabCursor[0]
}

// reflogIndex is the selected row of the reflog tab.
func (a *App) reflogIndex() int {
	if a.commitsView() == cvReflog {
		return a.cursor[panelCommits]
	}
	return a.commitTabCursor[1]
}

func (a *App) selectedReflogCommit() *models.Commit {
	if a.snap == nil {
		return nil
	}
	if i := a.reflogIndex(); i >= 0 && i < len(a.snap.Reflog) {
		return a.snap.Reflog[i]
	}
	return nil
}

// checkoutCommitMenu is BasicCommitsController.checkout (RefsHelper's
// CreateCheckoutMenu) for the selected commit of a commit list.
func (a *App) checkoutCommitMenu() {
	c := a.selectedCommit()
	if c == nil || a.snap == nil || c.Hash() == "" {
		return
	}
	tr := a.repo.Common.Tr
	head := ""
	if a.snap.Head != nil {
		head = a.snap.Head.Name
	}
	hash := c.Hash()
	items := []menuItem{{
		label: fmt.Sprintf(tr.Actions.CheckoutCommitAsDetachedHead, utils.ShortHash(hash)),
		key:   "d", chord: keymap.Binding{{Rune: 'd'}},
		run: func() { a.checkoutRef(hash) },
	}}
	branches := lo.Filter(a.raw.Branches, func(b *models.Branch, _ int) bool {
		return b.CommitHash == hash && b.Name != head
	})
	for i, b := range branches {
		item := menuItem{label: fmt.Sprintf(tr.Actions.CheckoutBranchAtCommit, b.Name), run: func() { a.checkoutRef(b.RefName()) }}
		if i < 9 {
			r := rune('1' + i)
			item.key, item.chord = string(r), keymap.Binding{{Rune: r}}
		}
		items = append(items, item)
	}
	if len(branches) == 0 {
		items = append(items, menuItem{label: tr.Actions.CheckoutBranch, key: "1", chord: keymap.Binding{{Rune: '1'}},
			disabled: tr.NoBranchesFoundAtCommitTooltip, run: func() {}})
	}
	items = append(items, menuItem{label: tr.Cancel, run: func() {}})
	a.pushOverlay(&menuOverlay{title: tr.Actions.CheckoutBranchOrCommit, items: items})
}

// checkoutRef is RefsHelper.CheckoutRef: it checks out ref (autostashing
// when needed) and selects the top of the branches, commits and reflog lists.
func (a *App) checkoutRef(ref string) {
	a.checkoutRefWith(ref, "Checking out…", func() error { return a.repo.Checkout(ref) })
}

// checkoutRefWith runs checkout op with CheckoutRef's focus, autostash and
// selection; lazygit pushes the branches context before checking out.
func (a *App) checkoutRefWith(ref, label string, op func() error) {
	if a.focus != panelBranches || a.mainFocused {
		a.focusPanel(panelBranches)
	}
	if a.branchTab != 0 && a.browse(panelBranches) == nil {
		a.switchBranchTab(-a.branchTab)
	}
	a.autostashing(label, fmt.Sprintf(a.repo.Common.Tr.AutoStashForCheckout, ref),
		op, func() {
			if a.browse(panelBranches) == nil {
				a.cursor[panelBranches] = 0
			}
			a.tabCursor[1][0] = 0
			a.commitTabCursor = [2]int{}
			if a.commitsView() <= cvReflog {
				a.cursor[panelCommits] = 0
			}
		})
}

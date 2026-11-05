package app

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"

	"laziergit/internal/repo"
)

// upstreamOptionsMenu is lazygit's viewUpstreamOptions (`u` in branches).
func (a *App) upstreamOptionsMenu() {
	b, h := a.selectedBranch(), a.snapHead()
	if b == nil || h == nil {
		return
	}
	tr := a.repo.Common.Tr
	upstream := tr.UpstreamGenericName
	if b.RemoteBranchStoredLocally() {
		upstream = b.ShortUpstreamRefName()
	}
	a.bg(func() {
		base, err := a.repo.BaseBranch(b)
		a.post(func() {
			if err != nil {
				a.errorPopup(err.Error())
				return
			}
			a.pushUpstreamMenu(b, h, upstream, base)
		})
	})
}

func (a *App) pushUpstreamMenu(b, h *models.Branch, upstream, base string) {
	tr := a.repo.Common.Tr
	ph := map[string]string{"upstream": upstream}
	notSet := ""
	if !b.RemoteBranchStoredLocally() {
		notSet = tr.UpstreamNotSetError
	}
	baseDisabled := ""
	if base == "" {
		base, baseDisabled = tr.CouldNotDetermineBaseBranch, tr.CouldNotDetermineBaseBranch
	}
	unsetDisabled := ""
	if !b.IsTrackingRemote() {
		unsetDisabled = tr.UpstreamNotSetError
	}

	a.pushOverlay(&menuOverlay{title: tr.BranchUpstreamOptionsTitle, items: []menuItem{
		{label: tr.ViewDivergenceFromUpstream, disabled: notSet, run: func() {
			a.loadRefCommitsDiverging(b, b.FullUpstreamRefName(), fmt.Sprintf("%s <-> %s", b.RefName(), upstream))
		}},
		{
			label: utils.ResolvePlaceholderString(tr.ViewDivergenceFromBaseBranch, map[string]string{"baseBranch": shortBranchName(base)}),
			key:   "b", chord: runeBind('b'), disabled: baseDisabled, run: func() {
				a.loadRefCommitsDiverging(b, base, fmt.Sprintf("%s <-> %s", b.RefName(), shortBranchName(base)))
			},
		},
		{label: tr.UnsetUpstream, key: "u", chord: runeBind('u'), disabled: unsetDisabled, run: func() {
			a.run("", func() error {
				return a.repo.Locked(func() error { return a.repo.Git.Branch.UnsetUpstream(b.Name) })
			}, nil)
		}},
		{label: tr.SetUpstream, key: "s", chord: runeBind('s'), run: func() {
			a.prompt(tr.EnterUpstream, "", "", func(v string) {
				remote, branch, ok := repo.ParseUpstream(v)
				if !ok {
					a.errorPopup(tr.InvalidUpstream)
					return
				}
				a.run("", func() error {
					return a.repo.Locked(func() error { return a.repo.Git.Branch.SetUpstream(remote, branch, b.Name) })
				}, nil)
			})
		}},
		{
			label: utils.ResolvePlaceholderString(tr.ViewUpstreamResetOptions, ph) + "...", key: "g", chord: runeBind('g'),
			disabled: notSet, run: func() { a.resetMenu(upstream, b.FullUpstreamRefName()) },
		},
		{
			label: utils.ResolvePlaceholderString(tr.ViewUpstreamRebaseOptions, ph) + "...", key: "r", chord: runeBind('r'),
			disabled: notSet, run: func() { a.branchRebaseMenu(upstream, h.Name, base) },
		},
	}})
}

func shortBranchName(full string) string {
	return strings.TrimPrefix(strings.TrimPrefix(full, "refs/heads/"), "refs/remotes/")
}

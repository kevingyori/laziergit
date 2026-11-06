package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"

	"laziergit/internal/keymap"
	"laziergit/internal/repo"
)

// Branch actions beyond checkout/create/delete: rename, merge, rebase,
// fast-forward and pull requests, following lazygit's branches controller.

func (a *App) renameBranchFlow() {
	b := a.selectedBranch()
	if b == nil {
		return
	}
	if b.DetachedHead {
		a.setStatus("The selected item is not a branch", true)
		return
	}
	ask := func() {
		a.prompt(a.repo.Common.Tr.NewBranchNamePrompt+" "+b.Name+":", b.Name, "Spaces become dashes", func(v string) {
			name := repo.SanitizedBranchName(v)
			if name == "" || name == b.Name {
				return
			}
			a.run("Renaming branch…", func() error { return a.repo.RenameBranch(b.Name, name) }, func() {
				a.selectBranch(name)
				a.setStatus("Renamed "+b.Name+" → "+name, false)
			})
		})
	}
	if b.IsTrackingRemote() {
		a.confirm(a.repo.Common.Tr.RenameBranch, a.repo.Common.Tr.RenameBranchWarning, false, ask)
		return
	}
	ask()
}

func (a *App) selectBranch(name string) {
	for i, br := range a.snap.Branches {
		if br.Name == name {
			a.cursor[panelBranches] = i
			a.lists[panelBranches].ScrollIntoView(i)
			a.diffKey = ""
			a.loadMain()
			return
		}
	}
}

// mergeFlow is lazygit's merge menu: regular (fast-forward when possible),
// the other fast-forward mode, and squash merges.
func (a *App) mergeFlow() {
	b, h := a.selectedBranch(), a.snapHead()
	if b == nil {
		return
	}
	if h == nil || h.DetachedHead {
		a.errorPopup("Cannot merge branch in detached head state. You might have checked out a commit directly or a remote branch, in which case you should checkout the local branch you want to be on")
		return
	}
	if b.Name == h.Name {
		a.errorPopup(a.repo.Common.Tr.CantMergeBranchIntoItself)
		return
	}
	ref, cur := b.Name, h.Name
	a.bg(func() {
		o := a.repo.MergeOptions(ref)
		a.post(func() { a.mergeMenu(ref, cur, o) })
	})
}

// mergeMenu is the menu of lazygit's MergeRefIntoCheckedOutBranch.
func (a *App) mergeMenu(ref, cur string, o repo.MergeOptions) {
	tr := a.repo.Common.Tr
	vars := map[string]string{"checkedOutBranch": cur, "selectedBranch": ref}
	merge := func(variant git_commands.MergeVariant) func() {
		return func() {
			a.nextCommitSel = selectHeadCommit
			a.run(tr.MergingStatus, func() error { return a.repo.Merge(ref, variant) }, nil)
		}
	}
	key := func(r rune) (string, keymap.Binding) { return string(r), keymap.Binding{{Rune: r}} }
	ffItem := func(variant git_commands.MergeVariant) menuItem {
		k, ch := key('m')
		if variant == git_commands.MERGE_VARIANT_FAST_FORWARD {
			k, ch = key('f')
		}
		it := menuItem{label: tr.RegularMergeFastForward, key: k, chord: ch, run: merge(variant),
			tooltip: utils.ResolvePlaceholderString(tr.RegularMergeFastForwardTooltip, vars)}
		if !o.CanFF {
			it.disabled = utils.ResolvePlaceholderString(tr.CannotFastForwardMerge, vars)
		}
		return it
	}
	noFF := func(variant git_commands.MergeVariant) menuItem {
		k, ch := key('m')
		if variant == git_commands.MERGE_VARIANT_NON_FAST_FORWARD {
			k, ch = key('n')
		}
		return menuItem{label: tr.RegularMergeNonFastForward, key: k, chord: ch, run: merge(variant),
			tooltip: utils.ResolvePlaceholderString(tr.RegularMergeNonFastForwardTooltip, vars)}
	}
	var items []menuItem
	if !o.WantNoFF && (o.WantFF || o.CanFF) {
		items = []menuItem{ffItem(git_commands.MERGE_VARIANT_REGULAR), noFF(git_commands.MERGE_VARIANT_NON_FAST_FORWARD)}
	} else {
		items = []menuItem{noFF(git_commands.MERGE_VARIANT_REGULAR), ffItem(git_commands.MERGE_VARIANT_FAST_FORWARD)}
	}
	sk, sch := key('s')
	Sk, Sch := key('S')
	items = append(items,
		menuItem{label: tr.SquashMergeUncommitted, key: sk, chord: sch,
			tooltip: utils.ResolvePlaceholderString(tr.SquashMergeUncommittedTooltip, vars), run: func() {
				a.run(tr.MergingStatus, func() error { return a.repo.Merge(ref, git_commands.MERGE_VARIANT_SQUASH) }, nil)
			}},
		menuItem{label: tr.SquashMergeCommitted, key: Sk, chord: Sch,
			tooltip: utils.ResolvePlaceholderString(tr.SquashMergeCommittedTooltip, vars), run: func() {
				a.nextCommitSel = selectHeadCommit
				a.run(tr.MergingStatus, func() error { return a.repo.SquashMergeCommitted(ref, cur) }, nil)
			}},
		menuItem{label: tr.Cancel, run: func() {}},
	)
	a.pushOverlay(&menuOverlay{title: tr.Merge, items: items})
}

// rebaseFlow is lazygit's rebase menu: simple, interactive, or onto the
// checked-out branch's base branch.
func (a *App) rebaseFlow() {
	b, h := a.selectedBranch(), a.snapHead()
	if b == nil || h == nil {
		return
	}
	ref, cur := b.Name, h.Name
	a.bg(func() {
		base, err := a.repo.BaseBranch(h)
		a.post(func() {
			if err != nil {
				a.showError("Find base branch", err)
				return
			}
			a.branchRebaseMenu(ref, cur, base)
		})
	})
}

func (a *App) branchRebaseMenu(ref, cur, base string) {
	self := ""
	if ref == cur {
		self = "You cannot rebase a branch onto itself"
	}
	rebase := func(onto string, interactive bool) func() {
		return func() {
			a.run("Rebasing…", func() error { return a.repo.RebaseOnto(onto, interactive) }, func() {
				if interactive {
					a.focusPanel(panelCommits)
					a.setStatus("Rebase started · edit the todo commits, then continue", false)
					return
				}
				a.setStatus("Rebased "+cur+" onto "+repo.ShortRefName(onto), false)
			})
		}
	}
	baseLabel, baseDisabled := repo.ShortRefName(base), ""
	if base == "" {
		baseLabel, baseDisabled = "base branch", "Could not determine the base branch"
	}
	a.pushOverlay(&menuOverlay{title: utils.ResolvePlaceholderString(a.repo.Common.Tr.RebasingTitle, map[string]string{"checkedOutBranch": cur}), items: []menuItem{
		{label: "Simple rebase onto " + ref, key: "s", chord: keymap.Binding{{Rune: 's'}}, disabled: self, run: rebase(ref, false)},
		{label: "Interactive rebase onto " + ref, key: "i", chord: keymap.Binding{{Rune: 'i'}}, disabled: self, run: rebase(ref, true)},
		{label: "Rebase onto " + baseLabel, key: "b", chord: keymap.Binding{{Rune: 'b'}}, disabled: baseDisabled, run: rebase(base, false)},
	}})
}

// fastForwardFlow is lazygit's BranchesController.fastForward: bring the
// selected branches up to their upstreams without checking them out.
func (a *App) fastForwardFlow() {
	if a.selectedBranch() == nil {
		return
	}
	start, end := a.selectionRange(panelBranches)
	branches := a.snap.Branches[start : end+1]
	tr := a.repo.Common.Tr
	if lo.SomeBy(branches, func(b *models.Branch) bool { return b.DetachedHead }) {
		a.setStatus(tr.DisabledMenuItemPrefix+tr.SelectedItemIsNotABranch, true)
		return
	}
	switch {
	case !lo.EveryBy(branches, func(b *models.Branch) bool { return b.IsTrackingRemote() }):
		a.showError("Fast-forward", errors.New(tr.FwdNoUpstream))
		return
	case !lo.EveryBy(branches, func(b *models.Branch) bool { return b.RemoteBranchStoredLocally() }):
		a.showError("Fast-forward", errors.New(tr.FwdNoLocalUpstream))
		return
	case lo.SomeBy(branches, func(b *models.Branch) bool { return b.IsAheadForPull() && !b.IsBehindForPull() }):
		a.showError("Fast-forward", errors.New(tr.FwdCommitsToPush))
		return
	}
	worktrees := a.snap.Worktrees
	a.run("Fast-forwarding…", func() error { return a.repo.FastForwardBranches(branches, worktrees) }, func() {})
}

// --- pull requests ------------------------------------------------------------

func (a *App) openURL(label, url string) {
	if err := a.openLink(url); err != nil {
		a.showError(label, err)
		return
	}
	a.setStatus("Opened "+url, false)
}

// createPullRequest opens the hosting service's new-PR page from the remote
// branch from into to ("" for the default branch).
func (a *App) createPullRequest(from, to string) {
	a.bg(func() {
		url, err := a.repo.PullRequestURL(from, to)
		a.post(func() {
			if err != nil {
				a.showError("Create pull request", err)
				return
			}
			a.openURL("Create pull request", url)
		})
	})
}

func (a *App) createPullRequestFlow() {
	if b := a.selectedBranch(); b != nil {
		a.createPullRequestFor(b)
	}
}

func (a *App) createPullRequestFor(b *models.Branch) {
	if !b.IsTrackingRemote() {
		a.errorPopup(a.repo.Common.Tr.PullRequestNoUpstream)
		return
	}
	a.createPullRequest(b.UpstreamBranch, "")
}

// pullRequestMenu is lazygit's "pull request options": from the checked-out
// branch into the selected one, and from either into the default or a
// chosen branch.
func (a *App) pullRequestMenu() {
	b, h := a.selectedBranch(), a.snapHead()
	if b == nil {
		return
	}
	forBranch := func(br *models.Branch) []menuItem {
		return []menuItem{
			{label: br.Name + " → " + a.repo.Common.Tr.DefaultBranch, run: func() { a.createPullRequestFor(br) }},
			{label: br.Name + " → " + a.repo.Common.Tr.SelectBranch, run: func() { a.pullRequestTargetFlow(br) }},
		}
	}
	var items []menuItem
	if h != nil && h.Name != b.Name {
		items = append(items, menuItem{label: h.Name + " → " + b.Name, run: func() {
			if !h.IsTrackingRemote() || !b.IsTrackingRemote() {
				a.errorPopup(a.repo.Common.Tr.PullRequestNoUpstream)
				return
			}
			a.createPullRequest(h.UpstreamBranch, b.UpstreamBranch)
		}})
		items = append(items, forBranch(h)...)
	}
	items = append(items, forBranch(b)...)
	a.pushOverlay(&menuOverlay{title: a.repo.Common.Tr.CreatePullRequestOptions, items: items})
}

func (a *App) pullRequestTargetFlow(b *models.Branch) {
	if !b.IsTrackingRemote() {
		a.errorPopup(a.repo.Common.Tr.PullRequestNoUpstream)
		return
	}
	pickBranch := func(remote string) {
		if !slices.ContainsFunc(a.snap.Remotes, func(r *models.Remote) bool { return r.Name == remote }) {
			a.errorPopup(fmt.Sprintf(a.repo.Common.Tr.NoValidRemoteName, remote))
			return
		}
		a.promptSuggestions(b.UpstreamBranch+" → "+remote+"/", "", "", "remote:"+remote, func(to string) {
			a.createPullRequest(b.UpstreamBranch, to)
		})
	}
	if len(a.snap.Remotes) == 1 {
		pickBranch(a.snap.Remotes[0].Name)
		return
	}
	a.promptSuggestions(a.repo.Common.Tr.SelectTargetRemote, "", "", "remotes", pickBranch)
}

func (a *App) openPullRequestFlow() {
	b := a.selectedBranch()
	if b == nil {
		return
	}
	pr, ok := a.prs[b.Name]
	if !ok {
		a.setStatus("No pull request for "+b.Name, true)
		return
	}
	a.openURL("Open pull request", pr.Url)
}

func (a *App) copyPullRequestURLFlow() {
	b := a.selectedBranch()
	if b == nil {
		return
	}
	copyURL := func(url string) {
		if err := a.copyText(url); err != nil {
			a.showError("Copy pull request URL", err)
			return
		}
		a.setStatus("Pull request URL copied to clipboard", false)
	}
	if pr, ok := a.prs[b.Name]; ok {
		copyURL(pr.Url)
		return
	}
	name := b.Name
	a.bg(func() {
		var url string
		err := errors.New("branch " + name + " does not exist on the remote")
		if a.repo.RemoteBranchExists(name) {
			url, err = a.repo.PullRequestURL(name, "")
		}
		a.post(func() {
			if err != nil {
				a.showError("Copy pull request URL", err)
				return
			}
			copyURL(url)
		})
	})
}

// prKey identifies which pull requests to ask GitHub for, so the network
// is only hit when the set of tracking branches changes (or on request).
func prKey(s *repo.Snapshot) string {
	var parts []string
	for _, b := range s.Branches {
		if b.IsTrackingRemote() {
			parts = append(parts, b.Name+"="+b.UpstreamRemote+"/"+b.UpstreamBranch)
		}
	}
	for _, rm := range s.Remotes {
		parts = append(parts, rm.Name+"@"+strings.Join(rm.Urls, ","))
	}
	return strings.Join(parts, "\n")
}

// RefreshPullRequests reloads GitHub pull requests in the background.
// Without a GitHub remote and login it quietly shows none, as lazygit does.
func (a *App) RefreshPullRequests() {
	if a.snap == nil {
		return
	}
	r, s := a.repo, a.snap
	a.prKey = prKey(s)
	a.bg(func() {
		prs, err := r.PullRequests(s.Branches, s.Remotes)
		a.post(func() {
			if a.repo != r {
				return
			}
			switch {
			case err == nil:
				a.prs = prs
			case errors.Is(err, repo.ErrNoGitHub):
				a.prs = nil
			} // a network error keeps what we had
		})
	})
}

// shownPR is the pull request to show for b. Like lazygit, a main branch
// only shows one that is still open.
func (a *App) shownPR(b *models.Branch) *models.GithubPullRequest {
	pr, ok := a.prs[b.Name]
	if !ok {
		return nil
	}
	if slices.Contains(a.repo.Config.Git.MainBranches, b.Name) && (pr.State == "CLOSED" || pr.State == "MERGED") {
		return nil
	}
	return pr
}

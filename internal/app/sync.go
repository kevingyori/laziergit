package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"

	"laziergit/internal/repo"
)

// Push and pull, ported from lazygit's SyncController: a branch with
// branches stacked below it that also need pushing (or have changed on the
// remote) offers to push (pull) the whole stack.

type pushOpts struct {
	force, forceWithLease, setUpstream bool
	upstreamRemote, upstreamBranch     string
	// see lazygit's pushOpts.remoteBranchStoredLocally
	remoteBranchStoredLocally bool
	branchesBelow             []*models.Branch
	forceWithLeaseBelow       bool
}

// branchesBelowInStack is lazygit's helpers.BranchesBelowInStack (copied:
// importing the helpers package drags in the whole GUI): the branches whose
// tips are commits of b's own, nearest first, main branches excluded.
func (a *App) branchesBelowInStack(b *models.Branch) []*models.Branch {
	mainBranches := a.repo.Config.Git.MainBranches
	byTip := map[string][]*models.Branch{}
	for _, other := range a.snap.Branches {
		if other.Name == b.Name || lo.Contains(mainBranches, other.Name) {
			continue
		}
		byTip[other.CommitHash] = append(byTip[other.CommitHash], other)
	}
	var result []*models.Branch
	for _, c := range a.snap.Commits {
		if c.IsTODO() || c.Status == models.StatusMerged {
			continue
		}
		result = append(result, byTip[c.Hash()]...)
	}
	return result
}

func (a *App) pushFlow() {
	h := a.snapHead()
	if h == nil {
		return
	}
	tr := a.repo.Common.Tr
	below := lo.Filter(a.branchesBelowInStack(h), func(b *models.Branch, _ int) bool {
		return b.PushRemote != "" && !b.UpstreamGone && b.IsAheadForPush()
	})
	if len(below) == 0 {
		a.pushCurrentBranch(h)
		return
	}
	lines := lo.Map(below, func(b *models.Branch, _ int) string {
		d := "↑" + b.AheadForPush
		if b.IsBehindForPush() {
			d = "↓" + b.BehindForPush + d
		}
		return b.Name + " " + d
	})
	name := map[string]string{"branchName": h.Name}
	a.pushOverlay(&menuOverlay{title: tr.Push,
		body: utils.ResolvePlaceholderString(tr.BranchesBelowHaveCommitsToPush, name) + "\n\n  " + strings.Join(lines, "\n  "),
		items: []menuItem{
			{label: tr.PushBranchAndBranchesBelow, run: func() { a.pushWithBranchesBelow(h, below) }},
			{label: utils.ResolvePlaceholderString(tr.PushOnlyCurrentBranch, name), run: func() { a.pushCurrentBranch(h) }},
			{label: tr.Cancel, run: func() {}},
		}})
}

func (a *App) pushCurrentBranch(h *models.Branch) {
	a.resolvePushOfCurrentBranch(h, func(opts pushOpts) {
		if h.IsBehindForPush() {
			a.confirm(a.repo.Common.Tr.ForcePush, a.forcePushPrompt(), true, func() {
				opts.forceWithLease = true
				a.pushAux(h, opts)
			})
			return
		}
		a.pushAux(h, opts)
	})
}

func (a *App) pushWithBranchesBelow(h *models.Branch, below []*models.Branch) {
	if h.RemoteBranchStoredLocally() && h.PushRemote != "" {
		branches := append([]*models.Branch{h}, below...)
		a.confirmForcePushIfNeeded(branches, func(forceWithLease bool) {
			a.runPush(func() error { return a.pushBranches(branches, forceWithLease) })
		})
		return
	}
	a.resolvePushOfCurrentBranch(h, func(opts pushOpts) {
		a.confirmForcePushIfNeeded(below, func(forceWithLease bool) {
			opts.branchesBelow = below
			opts.forceWithLeaseBelow = forceWithLease
			a.pushAux(h, opts)
		})
	})
}

func (a *App) confirmForcePushIfNeeded(branches []*models.Branch, push func(forceWithLease bool)) {
	diverged := lo.Filter(branches, func(b *models.Branch, _ int) bool { return b.IsBehindForPush() })
	if len(diverged) == 0 {
		push(false)
		return
	}
	tr := a.repo.Common.Tr
	if a.repo.Config.Git.DisableForcePushing {
		a.showError("Push", errors.New(tr.ForcePushBranchesDisabled))
		return
	}
	a.confirm(tr.ForcePush, utils.ResolvePlaceholderString(tr.ForcePushBranchesPrompt, map[string]string{
		"branches":   "  " + strings.Join(lo.Map(diverged, func(b *models.Branch, _ int) string { return b.Name }), "\n  "),
		"cancelKey":  a.repo.Config.Keybinding.Universal.Return.String(),
		"confirmKey": a.repo.Config.Keybinding.Universal.Confirm.String(),
	}), true, func() { push(true) })
}

// resolvePushOfCurrentBranch works out where the current branch goes: its
// upstream, a same-named branch when push.default is "current", or an
// upstream the user enters.
func (a *App) resolvePushOfCurrentBranch(h *models.Branch, onResolved func(pushOpts)) {
	if h.IsTrackingRemote() {
		onResolved(pushOpts{remoteBranchStoredLocally: h.RemoteBranchStoredLocally()})
		return
	}
	if a.repo.Git.Config.GetPushToCurrent() {
		onResolved(pushOpts{setUpstream: true})
		return
	}
	a.promptSuggestions(a.repo.Common.Tr.EnterUpstream, a.repo.UpstreamSuggestion(h.Name), "", "upstream", func(v string) {
		remote, branch, ok := repo.ParseUpstream(v)
		if !ok {
			a.showError("Push", errors.New(a.repo.Common.Tr.InvalidUpstream))
			return
		}
		onResolved(pushOpts{setUpstream: true, upstreamRemote: remote, upstreamBranch: branch})
	})
}

// runPush runs a push off the main thread and refreshes; errors (and the
// force-push offer, via askError) come back through a.run.
func (a *App) runPush(op func() error) {
	a.run("Pushing…", op, a.done("Pushed"))
}

func (a *App) pushAux(h *models.Branch, opts pushOpts) {
	tr := a.repo.Common.Tr
	a.runPush(func() error {
		var refspecs []string
		if opts.upstreamBranch != "" {
			refspecs = append(refspecs, fmt.Sprintf("refs/heads/%s:%s", h.Name, opts.upstreamBranch))
		}
		err := a.repo.Push(git_commands.PushOpts{
			Force: opts.force, ForceWithLease: opts.forceWithLease, SetUpstream: opts.setUpstream,
			Remote: opts.upstreamRemote, Refspecs: refspecs,
		})
		if err != nil {
			if !opts.force && !opts.forceWithLease && strings.Contains(err.Error(), "Updates were rejected") {
				if opts.remoteBranchStoredLocally {
					return errors.New(tr.UpdatesRejected)
				}
				if a.repo.Config.Git.DisableForcePushing {
					return errors.New(tr.UpdatesRejectedAndForcePushDisabled)
				}
				return &askError{err: err, ask: func() {
					a.confirm(tr.ForcePush, a.forcePushPrompt(), true, func() {
						o := opts
						o.force = true
						a.pushAux(h, o)
					})
				}}
			}
			return err
		}
		return a.pushBranches(opts.branchesBelow, opts.forceWithLeaseBelow)
	})
}

// pushBranches pushes branches to their push destinations, one command per
// remote.
func (a *App) pushBranches(branches []*models.Branch, forceWithLease bool) error {
	remotes := lo.Uniq(lo.Map(branches, func(b *models.Branch, _ int) string { return b.PushRemote }))
	for _, remote := range remotes {
		refspecs := lo.FilterMap(branches, func(b *models.Branch, _ int) (string, bool) {
			return fmt.Sprintf("refs/heads/%s:refs/heads/%s", b.Name, b.PushBranch), b.PushRemote == remote
		})
		if err := a.repo.Push(git_commands.PushOpts{ForceWithLease: forceWithLease, Remote: remote, Refspecs: refspecs}); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) pullFlow() {
	h := a.snapHead()
	if h == nil {
		return
	}
	worktrees := a.snap.Worktrees
	candidates := lo.Filter(a.branchesBelowInStack(h), func(b *models.Branch, _ int) bool {
		return !git_commands.CheckedOutByOtherWorktree(b, worktrees) && b.IsBehindForPull()
	})
	if len(candidates) == 0 {
		a.pullCurrentBranch(h, nil)
		return
	}
	// lazygit's BranchLoader works out in the background whether a diverged
	// branch's upstream was rewritten (it has no commits of its own); ask now.
	a.busy = "Pulling…"
	r := a.repo
	a.bg(func() {
		below := lo.Filter(candidates, func(b *models.Branch, _ int) bool {
			if !b.IsAheadForPull() {
				return true
			}
			local, err := r.Git.Branch.HasLocalOnlyCommits(b)
			return err == nil && !local
		})
		a.post(func() {
			a.busy = ""
			if a.repo != r {
				return
			}
			a.pullMenu(h, below)
		})
	})
}

func (a *App) pullMenu(h *models.Branch, below []*models.Branch) {
	if len(below) == 0 {
		a.pullCurrentBranch(h, nil)
		return
	}
	tr := a.repo.Common.Tr
	lines := lo.Map(below, func(b *models.Branch, _ int) string {
		d := "↓" + b.BehindForPull
		if b.IsAheadForPull() {
			d += "↑" + b.AheadForPull
		}
		return b.Name + " " + d
	})
	name := map[string]string{"branchName": h.Name}
	a.pushOverlay(&menuOverlay{title: tr.Pull,
		body: utils.ResolvePlaceholderString(tr.BranchesBelowHaveChangedOnRemote, name) + "\n\n  " + strings.Join(lines, "\n  "),
		items: []menuItem{
			{label: tr.PullBranchAndBranchesBelow, run: func() { a.pullCurrentBranch(h, below) }},
			{label: utils.ResolvePlaceholderString(tr.PullOnlyCurrentBranch, name), run: func() { a.pullCurrentBranch(h, nil) }},
			{label: tr.Cancel, run: func() {}},
		}})
}

// pullCurrentBranch pulls the current branch, after updating the given
// branches below it to their upstreams (if that fails, nothing is pulled).
func (a *App) pullCurrentBranch(h *models.Branch, below []*models.Branch) {
	worktrees := a.snap.Worktrees
	pull := func(setUpstream func() error) {
		a.nextCommitSel = selectHeadCommit // CheckMergeOrRebaseAndSelectHeadCommit
		a.run("Pulling…", func() error {
			if setUpstream != nil {
				if err := setUpstream(); err != nil {
					return err
				}
			}
			if len(below) > 0 {
				if err := a.repo.FastForwardBranches(below, worktrees); err != nil {
					return err
				}
			}
			return a.repo.Pull()
		}, a.done("Pulled"))
	}
	if h.IsTrackingRemote() {
		pull(nil)
		return
	}
	a.promptSuggestions(a.repo.Common.Tr.EnterUpstream, a.repo.UpstreamSuggestion(h.Name), "", "upstream", func(v string) {
		remote, branch, ok := repo.ParseUpstream(v)
		if !ok {
			a.showError("Pull", errors.New(a.repo.Common.Tr.InvalidUpstream))
			return
		}
		pull(func() error {
			err := a.repo.SetUpstream(remote, branch)
			if err != nil && strings.Contains(err.Error(), "does not exist") {
				return fmt.Errorf("upstream branch %s/%s not found.\nIf you expect it to exist, you should fetch (with 'f').\nOtherwise, you should push (with 'shift+P')", remote, branch)
			}
			return err
		})
	})
}

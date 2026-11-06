package repo

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gocui"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// Branch actions: rename, merge, rebase onto, fast-forward and pull-request
// URLs. Each mirrors the lazygit branches controller it is named after.

// SanitizedBranchName is lazygit's branch-name cleanup: spaces become dashes.
func SanitizedBranchName(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), " ", "-")
}

// ShortRefName drops refs/heads/ or refs/remotes/ for display.
func ShortRefName(ref string) string {
	return strings.TrimPrefix(strings.TrimPrefix(ref, "refs/heads/"), "refs/remotes/")
}

func (r *Repo) RenameBranch(oldName, newName string) error {
	return r.locked(func() error { return r.Git.Branch.Rename(oldName, newName) })
}

// MergeOptions is what lazygit's merge menu needs to know: the user's
// fast-forward preference (lazygit's git.merging.args, then git's merge.ff)
// and whether ref can be fast-forwarded into HEAD at all.
type MergeOptions struct {
	WantFF, WantNoFF, CanFF bool
}

func (r *Repo) MergeOptions(ref string) MergeOptions {
	var o MergeOptions
	args := r.Config.Git.Merging.Args
	switch ff := r.Git.Config.GetMergeFF(); {
	case strings.Contains(args, "--ff"): // also covers --ff-only
		o.WantFF = true
	case strings.Contains(args, "--no-ff"):
		o.WantNoFF = true
	case ff == "true" || ff == "only":
		o.WantFF = true
	case ff == "false":
		o.WantNoFF = true
	}
	o.CanFF = r.Git.Branch.IsAncestor("HEAD", ref)
	return o
}

func (r *Repo) Merge(ref string, variant git_commands.MergeVariant) error {
	return r.locked(func() error { return r.Git.Branch.Merge(ref, variant) })
}

// SquashMergeCommitted squash-merges ref and commits it with the user's
// git.merging.squashMergeMessage.
func (r *Repo) SquashMergeCommitted(ref, current string) error {
	return r.locked(func() error {
		if err := r.Git.Branch.Merge(ref, git_commands.MERGE_VARIANT_SQUASH); err != nil {
			return err
		}
		msg := utils.ResolvePlaceholderString(r.Config.Git.Merging.SquashMergeMessage, map[string]string{
			"selectedRef": ref, "currentBranch": current,
		})
		return r.Git.Commit.CommitCmdObj(msg, "", false).PromptOnCredentialRequest(gocui.NewFakeTask()).Run()
	})
}

// BaseBranch is the main branch b forked from (lazygit's "rebase onto base
// branch"), or "" when it can't be determined.
func (r *Repo) BaseBranch(b *models.Branch) (string, error) {
	return r.Git.Loaders.BranchLoader.GetBaseBranch(b, r.mainBr)
}

// RebaseOnto rebases the checked-out branch onto ref. Interactive stops
// before the first commit so the todo list can be edited in the commits
// panel.
func (r *Repo) RebaseOnto(ref string, interactive bool) error {
	return r.locked(func() error {
		if interactive {
			return r.Git.Rebase.EditRebase(ref)
		}
		return r.Git.Rebase.RebaseBranch(ref)
	})
}

// branchToForward is one of the branches a fast-forward brings to its
// upstream (lazygit's BranchesHelper.branchToForward).
type branchToForward struct {
	branch   *models.Branch
	worktree *models.Worktree
	reset    bool
}

func worktreeArgs(w *models.Worktree) (string, string) {
	if w.IsCurrent {
		return "", ""
	}
	return w.GitDir, w.Path
}

// FastForward brings b up to its upstream (one branch of FastForwardBranches).
func (r *Repo) FastForward(b *models.Branch, worktrees []*models.Worktree) error {
	return r.FastForwardBranches([]*models.Branch{b}, worktrees)
}

// FastForwardBranches is lazygit's BranchesHelper.FastForwardBranches: fetch
// the upstream branches, then move each branch to its upstream (or, where it
// is checked out, merge --ff-only / reset --keep there). A branch that has
// diverged is reset only if none of its own commits would be lost; if any
// branch can't be updated, none of them is.
func (r *Repo) FastForwardBranches(branches []*models.Branch, worktrees []*models.Worktree) error {
	toForward := make([]*branchToForward, len(branches))
	for i, b := range branches {
		w, _ := git_commands.WorktreeForBranch(b, worktrees)
		toForward[i] = &branchToForward{branch: b, worktree: w}
	}
	for _, f := range toForward {
		if f.worktree != nil && f.worktree.IsRebasingOrBisecting {
			return errors.New(utils.ResolvePlaceholderString(r.Common.Tr.FwdBranchRebasingOrBisecting, map[string]string{"branchName": f.branch.Name, "worktreeName": f.worktree.Name}))
		}
	}
	var remotes []string
	for _, b := range branches {
		if !slices.Contains(remotes, b.UpstreamRemote) {
			remotes = append(remotes, b.UpstreamRemote)
		}
	}
	for _, remote := range remotes {
		var names []string
		for _, b := range branches {
			if b.UpstreamRemote == remote {
				names = append(names, b.UpstreamBranch)
			}
		}
		if err := r.Git.Sync.FetchRemoteBranches(gocui.NewFakeTask(), remote, names); err != nil {
			return err
		}
	}
	return r.locked(func() error {
		for _, f := range toForward {
			if err := r.planForwardingBranch(f); err != nil {
				return err
			}
		}
		return r.forwardBranches(toForward)
	})
}

func (r *Repo) planForwardingBranch(f *branchToForward) error {
	f.reset = !r.Git.Branch.IsAncestor(f.branch.FullRefName(), f.branch.FullUpstreamRefName())
	if !f.reset {
		return nil
	}
	local, err := r.Git.Branch.HasLocalOnlyCommits(f.branch)
	if err != nil {
		return err
	}
	if local {
		return errors.New(utils.ResolvePlaceholderString(r.Common.Tr.FwdLocalOnlyCommits, map[string]string{"branchName": f.branch.Name}))
	}
	if f.worktree != nil {
		gitDir, path := worktreeArgs(f.worktree)
		dirty, err := r.Git.WorkingTree.HasChangesToTrackedFiles(gitDir, path)
		if err != nil {
			return err
		}
		if dirty {
			return errors.New(utils.ResolvePlaceholderString(r.Common.Tr.FwdUncommittedChanges, map[string]string{"branchName": f.branch.Name}))
		}
	}
	return nil
}

// forwardBranches keeps going when a branch fails, so it doesn't hold up the
// others, and returns all the errors.
func (r *Repo) forwardBranches(toForward []*branchToForward) error {
	var errs []error
	updates := ""
	for _, f := range toForward {
		if f.worktree == nil {
			// The old value makes the update fail if the branch moved since
			// the user looked at it.
			updates += fmt.Sprintf("update %s %s %s\n", f.branch.FullRefName(), f.branch.FullUpstreamRefName(), f.branch.CommitHash)
		}
	}
	if updates != "" {
		if err := r.Git.Branch.UpdateBranchRefs(updates, "lazygit: update to upstream branch"); err != nil {
			errs = append(errs, err)
		}
	}
	for _, f := range toForward {
		if f.worktree == nil {
			continue
		}
		gitDir, path := worktreeArgs(f.worktree)
		var err error
		if f.reset {
			err = r.Git.WorkingTree.ResetKeep(f.branch.FullUpstreamRefName(), gitDir, path)
		} else {
			err = r.Git.Branch.FastForwardMerge(f.branch.FullUpstreamRefName(), gitDir, path)
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// AutoForwardBranches is lazygit's BranchesHelper.AutoForwardBranches, run
// after a fetch: branches strictly behind their upstream (all of them, or the
// main branches, per git.autoForwardBranches) are moved to it, except the
// checked-out one and those whose worktree is current, busy, missing or dirty.
func (r *Repo) AutoForwardBranches(branches []*models.Branch, worktrees []*models.Worktree) error {
	mode := r.Config.Git.AutoForwardBranches
	if mode == "none" || len(branches) == 0 {
		return nil
	}
	var toForward []*branchToForward
	for _, b := range branches[1:] {
		if !b.RemoteBranchStoredLocally() || !(mode == "allBranches" || slices.Contains(r.Config.Git.MainBranches, b.Name)) {
			continue
		}
		if !(b.IsBehindForPull() && !b.IsAheadForPull()) {
			continue
		}
		w, _ := git_commands.WorktreeForBranch(b, worktrees)
		if w != nil && (w.IsCurrent || w.IsRebasingOrBisecting || w.IsPathMissing) {
			continue
		}
		toForward = append(toForward, &branchToForward{branch: b, worktree: w})
	}
	if len(toForward) == 0 {
		return nil
	}
	return r.locked(func() error {
		var keep []*branchToForward
		for _, f := range toForward {
			if f.worktree != nil {
				gitDir, path := worktreeArgs(f.worktree)
				if dirty, err := r.Git.WorkingTree.HasChangesToTrackedFiles(gitDir, path); err != nil || dirty {
					continue
				}
			}
			keep = append(keep, f)
		}
		if len(keep) == 0 {
			return nil
		}
		return r.forwardBranches(keep)
	})
}

// PullRequestURL is the hosting service's "new pull request" page from one
// remote branch to another ("" targets the default branch).
func (r *Repo) PullRequestURL(from, to string) (string, error) {
	return r.Git.HostingService.GetPullRequestURL(from, to)
}

// RemoteBranchExists reports whether origin has a branch called name.
func (r *Repo) RemoteBranchExists(name string) bool {
	return r.Git.Remote.CheckRemoteBranchExists(name)
}

// ErrNoGitHub means there is no GitHub remote with a login to ask.
var ErrNoGitHub = errors.New("no authenticated GitHub remote")

// PullRequests fetches the GitHub pull requests of the tracking branches,
// keyed by local branch name — lazygit's PR refresh. It needs a GitHub
// remote and a gh login (or GH_TOKEN); otherwise it returns ErrNoGitHub.
func (r *Repo) PullRequests(branches []*models.Branch, remotes []*models.Remote) (map[string]*models.GithubPullRequest, error) {
	var names []string
	for _, b := range branches {
		if b.IsTrackingRemote() {
			names = append(names, b.UpstreamBranch)
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	configured := r.Git.GitHub.ConfiguredBaseRemoteName()
	tokens := map[string]string{}
	var candidates []*models.Remote
	byName := map[string]*models.Remote{}
	for _, rm := range remotes {
		if len(rm.Urls) == 0 {
			continue
		}
		info, err := r.Git.HostingService.GetServiceInfo(rm.Urls[0])
		if err != nil || info.Provider != "github" {
			continue
		}
		tok, ok := tokens[info.WebDomain]
		if !ok {
			tok = r.Git.GitHub.GetAuthToken(info.WebDomain)
			tokens[info.WebDomain] = tok
		}
		if tok == "" {
			continue
		}
		candidates = append(candidates, rm)
		byName[rm.Name] = rm
	}
	var base *models.Remote
	switch {
	case configured != "":
		base = byName[configured]
	case len(candidates) == 1:
		base = candidates[0]
	default:
		base = byName["upstream"]
		if base == nil {
			base = byName["origin"]
		}
	}
	if base == nil {
		return nil, ErrNoGitHub
	}
	info, err := r.Git.HostingService.GetServiceInfo(base.Urls[0])
	if err != nil {
		return nil, err
	}
	prs, err := r.Git.GitHub.FetchRecentPRs(names, &info, tokens[info.WebDomain])
	if err != nil {
		return nil, err
	}
	return git_commands.GenerateGithubPullRequestMap(prs, branches, remotes), nil
}

package repo

import (
	"errors"
	"fmt"
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

// FastForward brings b up to its upstream, as lazygit's fast-forward does:
// fetch the upstream branch, then move the ref (or, where b is checked out
// in wt, merge --ff-only there). A branch that has diverged is reset to its
// upstream only if none of its own commits would be lost.
func (r *Repo) FastForward(b *models.Branch, wt *models.Worktree) error {
	if wt != nil && wt.IsRebasingOrBisecting {
		return errors.New(utils.ResolvePlaceholderString(r.Common.Tr.FwdBranchRebasingOrBisecting, map[string]string{"branchName": b.Name, "worktreeName": wt.Name}))
	}
	if err := r.Git.Sync.FetchRemoteBranches(gocui.NewFakeTask(), b.UpstreamRemote, []string{b.UpstreamBranch}); err != nil {
		return err
	}
	return r.locked(func() error {
		upstream := b.FullUpstreamRefName()
		reset := !r.Git.Branch.IsAncestor(b.FullRefName(), upstream)
		var gitDir, path string
		if wt != nil && !wt.IsCurrent {
			gitDir, path = wt.GitDir, wt.Path
		}
		if reset {
			local, err := r.Git.Branch.HasLocalOnlyCommits(b)
			if err != nil {
				return err
			}
			if local {
				return errors.New(utils.ResolvePlaceholderString(r.Common.Tr.FwdLocalOnlyCommits, map[string]string{"branchName": b.Name}))
			}
			if wt != nil {
				dirty, err := r.Git.WorkingTree.HasChangesToTrackedFiles(gitDir, path)
				if err != nil {
					return err
				}
				if dirty {
					return errors.New(utils.ResolvePlaceholderString(r.Common.Tr.FwdUncommittedChanges, map[string]string{"branchName": b.Name}))
				}
			}
		}
		switch {
		case wt == nil:
			// The old value makes the update fail if the branch moved since
			// the user looked at it.
			return r.Git.Branch.UpdateBranchRefs(
				fmt.Sprintf("update %s %s %s\n", b.FullRefName(), upstream, b.CommitHash),
				"lazygit: update to upstream branch")
		case reset:
			return r.Git.WorkingTree.ResetKeep(upstream, gitDir, path)
		default:
			return r.Git.Branch.FastForwardMerge(upstream, gitDir, path)
		}
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

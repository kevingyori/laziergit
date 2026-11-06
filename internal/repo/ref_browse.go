package repo

import (
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

func (r *Repo) RefCommits(ref models.Ref) ([]*models.Commit, error) {
	return r.RefCommitsDiverging(ref, "")
}

func (r *Repo) RefCommitsDiverging(ref models.Ref, from string) ([]*models.Commit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Git.Loaders.CommitLoader.GetCommits(git_commands.GetCommitsOptions{Limit: true, RefName: ref.FullRefName(), RefForPushedStatus: ref, RefToShowDivergenceFrom: from, MainBranches: r.mainBr, HashPool: r.hashPool})
}

func (r *Repo) RemoteBranches(name string) ([]*models.RemoteBranch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	branches, err := r.Git.Loaders.RemoteLoader.GetRemoteBranchesByRemoteName()
	return branches[name], err
}

// NewBranchFromRemote is lazygit's RefsHelper.NewBranch confirm: track the
// remote branch only when the name is the suggested one.
func (r *Repo) NewBranchFromRemote(name, from string, track bool) error {
	return r.locked(func() error {
		if track {
			return r.Git.Branch.New(name, from)
		}
		return r.Git.Branch.NewWithoutTracking(name, from)
	})
}

// StashCommit is a stash entry as a commit, for its files (stash^..stash).
func (r *Repo) StashCommit(s *models.StashEntry) *models.Commit {
	return models.NewCommit(r.hashPool, models.NewCommitOpts{Hash: s.Hash, Name: s.Name, Parents: []string{s.Hash + "^"}})
}

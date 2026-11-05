package repo

import (
	"errors"
	"fmt"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/stefanhaller/git-todo-parser/todo"
)

// FixupCommits squashes commits[i] into the commit below it, as lazygit's
// fixup menu does; keepMessage uses the fixed-up commit's message (-C).
func (r *Repo) FixupCommits(commits []*models.Commit, i int, keepMessage bool) error {
	return r.FixupCommitRange(commits, i, i, keepMessage)
}

func (r *Repo) FixupCommitRange(commits []*models.Commit, start, end int, keepMessage bool) error {
	return r.locked(func() error {
		if err := r.checkRewrite(commits); err != nil {
			return err
		}
		if start < 0 || end < start || end+1 >= len(commits) {
			return fmt.Errorf("select a non-merge commit with a parent below it")
		}
		flag := ""
		if keepMessage {
			flag = "-C"
		}
		for _, c := range commits[start : end+1] {
			if c.IsMerge() {
				return fmt.Errorf("select non-merge commits")
			}
		}
		return r.Git.Rebase.InteractiveRebase(commits, start, end, todo.Fixup, flag)
	})
}

// CreateFixupCommit commits the staged changes as "fixup! <commit>", then
// moves it under the stacked branch that owns the commit (lazygit's
// moveFixupCommitToOwnerStackedBranch).
func (r *Repo) CreateFixupCommit(target *models.Commit, selectedIdx int, commits []*models.Commit, branches []*models.Branch) error {
	return r.locked(func() error {
		if err := r.Git.Commit.CreateFixupCommit(target.Hash()); err != nil {
			return err
		}
		return r.moveFixupToOwnerBranch(target, selectedIdx, commits, branches)
	})
}

// CreateAmendCommit creates an "amend!" commit for target.
func (r *Repo) CreateAmendCommit(target *models.Commit, selectedIdx int, commits []*models.Commit, branches []*models.Branch, originalSubject, summary, description string, includeFileChanges bool) error {
	return r.locked(func() error {
		if err := r.Git.Commit.CreateAmendCommit(originalSubject, summary, description, includeFileChanges); err != nil {
			return err
		}
		return r.moveFixupToOwnerBranch(target, selectedIdx, commits, branches)
	})
}

func (r *Repo) moveFixupToOwnerBranch(target *models.Commit, selectedIdx int, commits []*models.Commit, branches []*models.Branch) error {
	if r.Git.Version.IsOlderThan(2, 38, 0) || r.Git.Status.WorkingTreeState().Any() ||
		target.Status == models.StatusMerged || !r.Git.Config.GetRebaseUpdateRefs() {
		return nil
	}
	head := -1
	for i := selectedIdx; i > 0; i-- {
		for _, b := range branches {
			if b.CommitHash == commits[i].Hash() {
				head = i
				break
			}
		}
		if head != -1 {
			break
		}
	}
	if head == -1 {
		return nil
	}
	return r.Git.Rebase.MoveFixupCommitDown(commits, head)
}

// SquashFixupsAbove applies the fixup commits above commit
// (git rebase --autosquash).
func (r *Repo) SquashFixupsAbove(commit *models.Commit) error {
	return r.locked(func() error {
		if commit == nil {
			return errors.New("no commit")
		}
		return r.Git.Rebase.SquashAllAboveFixupCommits(commit)
	})
}

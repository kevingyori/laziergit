package repo

import (
	"fmt"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/stefanhaller/git-todo-parser/todo"
	"strings"
)

func (r *Repo) Rewrite(commits []*models.Commit, i int, action string) error {
	return r.locked(func() error {
		if err := r.checkRewrite(commits); err != nil {
			return err
		}
		if i < 0 || i >= len(commits) {
			return fmt.Errorf("commit no longer available")
		}
		if (action == "squash" || action == "fixup") && (i+1 >= len(commits) || commits[i].IsMerge()) {
			return fmt.Errorf("select a non-merge commit with a parent below it")
		}
		base := i + 1
		if action == "squash" || action == "fixup" || action == "down" {
			base++
		}
		if base >= len(commits) && !commits[len(commits)-1].IsFirstCommit() {
			return fmt.Errorf("load a shorter rewrite range: its base is outside the loaded history")
		}
		if action == "up" && i == 0 || action == "down" && i+1 >= len(commits) {
			return fmt.Errorf("cannot move past the end of the commit list")
		}
		switch action {
		case "up":
			return r.Git.Rebase.MoveCommitsUp(commits, i, i)
		case "down":
			return r.Git.Rebase.MoveCommitsDown(commits, i, i)
		case "squash":
			return r.Git.Rebase.InteractiveRebase(commits, i, i, todo.Squash, "")
		case "fixup":
			return r.Git.Rebase.InteractiveRebase(commits, i, i, todo.Fixup, "")
		case "edit":
			return r.Git.Rebase.BeginInteractiveRebaseForCommit(commits, i, false)
		case "drop":
			return r.Git.Rebase.InteractiveRebase(commits, i, i, todo.Drop, "")
		}
		return fmt.Errorf("unknown rebase action: %s", action)
	})
}

func (r *Repo) Reword(commits []*models.Commit, i int, summary, body string) error {
	return r.locked(func() error {
		if err := r.checkRewrite(commits); err != nil {
			return err
		}
		if i < 0 || i >= len(commits) {
			return fmt.Errorf("commit no longer available")
		}
		if i+1 >= len(commits) && !commits[i].IsFirstCommit() {
			return fmt.Errorf("commit base is outside the loaded history")
		}
		return r.Git.Rebase.RewordCommit(commits, i, summary, body)
	})
}

func (r *Repo) RebaseControl(action string) error {
	return r.locked(func() error { return r.Git.Rebase.GenericMergeOrRebaseAction("rebase", action) })
}

func (r *Repo) CherryPick(commits []*models.Commit) error {
	return r.locked(func() error { return r.Git.Rebase.CherryPickCommits(commits) })
}

func (r *Repo) checkRewrite(commits []*models.Commit) error {
	if r.Git.Status.WorkingTreeState().Any() {
		return fmt.Errorf("finish or abort the current git operation first")
	}
	if len(commits) == 0 {
		return fmt.Errorf("no commits to rewrite")
	}
	head, err := r.OS.Cmd.New(git_commands.NewGitCmd("rev-parse").Arg("HEAD").ToArgv()).DontLog().RunWithOutput()
	if err != nil {
		return err
	}
	if strings.TrimSpace(head) != commits[0].Hash() {
		return fmt.Errorf("history changed; refresh before rewriting")
	}
	return nil
}

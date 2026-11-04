package repo

import (
	"fmt"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/stefanhaller/git-todo-parser/todo"
)

func (r *Repo) Rewrite(commits []*models.Commit, i int, action string) error {
	return r.locked(func() error {
		if i < 0 || i >= len(commits) {
			return fmt.Errorf("commit no longer available")
		}
		if (action == "squash" || action == "fixup") && (i+1 >= len(commits) || commits[i].IsMerge()) {
			return fmt.Errorf("select a non-merge commit with a parent below it")
		}
		switch action {
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
	return r.locked(func() error { return r.Git.Rebase.RewordCommit(commits, i, summary, body) })
}

func (r *Repo) RebaseControl(action string) error {
	return r.locked(func() error { return r.Git.Rebase.GenericMergeOrRebaseAction("rebase", action) })
}

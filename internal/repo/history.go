package repo

import (
	"fmt"
	"slices"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/stefanhaller/git-todo-parser/todo"
	"strings"
)

func (r *Repo) Rewrite(commits []*models.Commit, i int, action string) error {
	return r.RewriteRange(commits, i, i, action)
}

func (r *Repo) RewriteRange(commits []*models.Commit, start, end int, action string) error {
	return r.locked(func() error {
		if start < 0 || end < start || end >= len(commits) {
			return fmt.Errorf("commit no longer available")
		}
		if r.Git.Status.WorkingTreeState().Rebasing {
			return r.editTodoRange(commits, start, end, action)
		}
		if action == "pick" {
			return fmt.Errorf("pick only applies to the todo commits of a rebase in progress")
		}
		if err := r.checkRewrite(commits); err != nil {
			return err
		}

		if (action == "squash" || action == "fixup") && (end+1 >= len(commits) || slices.ContainsFunc(commits[start:end+1], (*models.Commit).IsMerge)) {
			return fmt.Errorf("select a non-merge commit with a parent below it")
		}
		base := end + 1
		if action == "squash" || action == "fixup" || action == "down" {
			base++
		}
		if base >= len(commits) && !commits[len(commits)-1].IsFirstCommit() {
			return fmt.Errorf("load a shorter rewrite range: its base is outside the loaded history")
		}
		if action == "up" && start == 0 || action == "down" && end+1 >= len(commits) {
			return fmt.Errorf("cannot move past the end of the commit list")
		}
		switch action {
		case "up":
			return r.Git.Rebase.MoveCommitsUp(commits, start, end)
		case "down":
			return r.Git.Rebase.MoveCommitsDown(commits, start, end)
		case "squash":
			return r.Git.Rebase.InteractiveRebase(commits, start, end, todo.Squash, "")
		case "fixup":
			return r.Git.Rebase.InteractiveRebase(commits, start, end, todo.Fixup, "")
		case "edit":
			if commits[end].IsMerge() {
				if err := r.Git.Rebase.EditRebase(commits[end].Hash()); err != nil {
					return err
				}
				todos := []*models.Commit{}
				for _, c := range commits[start:end] {
					if !c.IsMerge() {
						copy := *c
						copy.Action = todo.Pick
						todos = append(todos, &copy)
					}
				}
				if len(todos) > 0 {
					return r.Git.Rebase.EditRebaseTodo(todos, todo.Edit, "")
				}
				return nil
			}
			return r.Git.Rebase.InteractiveRebase(commits, start, end, todo.Edit, "")
		case "drop":
			if start == end && commits[start].IsMerge() {
				return r.Git.Rebase.DropMergeCommit(commits, start)
			}
			return r.Git.Rebase.InteractiveRebase(commits, start, end, todo.Drop, "")
		}
		return fmt.Errorf("unknown rebase action: %s", action)
	})
}

func (r *Repo) Reword(commits []*models.Commit, i int, summary, body string) error {
	return r.locked(func() error {
		if r.Git.Status.WorkingTreeState().Rebasing {
			if !models.IsHeadCommit(commits, i) {
				return fmt.Errorf("%s", r.Common.Tr.AlreadyRebasing)
			}
			return r.Git.Commit.RewordLastCommit(summary, body).Run()
		}
		if err := r.checkRewrite(commits); err != nil {
			return err
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

// editTodo changes the todo list of a rebase in progress, as lazygit does
// when you act on a todo commit mid-rebase.
func (r *Repo) editTodo(commits []*models.Commit, i int, action string) error {
	return r.editTodoRange(commits, i, i, action)
}

func (r *Repo) editTodoRange(commits []*models.Commit, start, end int, action string) error {
	if start < 0 || end < start || end >= len(commits) {
		return fmt.Errorf("commit no longer available")
	}
	for _, c := range commits[start : end+1] {
		if !c.IsTODO() || c.Status == models.StatusConflicted {
			return fmt.Errorf("%s", r.Common.Tr.MustSelectTodoCommits)
		}
		if c.Action == todo.UpdateRef && (action == "up" || action == "down" || action == "drop") {
			continue
		}
		if !slices.Contains([]todo.TodoCommand{todo.Pick, todo.Drop, todo.Edit, todo.Fixup, todo.Squash, todo.Reword}, c.Action) {
			return fmt.Errorf("%s", r.Common.Tr.ChangingThisActionIsNotAllowed)
		}
	}
	actions := map[string]todo.TodoCommand{"pick": todo.Pick, "squash": todo.Squash, "fixup": todo.Fixup, "drop": todo.Drop, "edit": todo.Edit}
	switch action {
	case "up", "down":
		offset, edge := -1, start-1
		if action == "down" {
			offset, edge = 1, end+1
		}
		if edge < 0 || edge >= len(commits) || !commits[edge].IsTODO() || commits[edge].Status == models.StatusConflicted {
			return fmt.Errorf("%s", r.Common.Tr.CannotMoveAnyFurther)
		}
		return r.Git.Rebase.MoveTodos(commits[start:end+1], offset)
	}
	if action == "drop" {
		refs, rest := []*models.Commit{}, []*models.Commit{}
		for _, c := range commits[start : end+1] {
			if c.Action == todo.UpdateRef {
				refs = append(refs, c)
			} else {
				rest = append(rest, c)
			}
		}
		if len(refs) > 0 {
			if err := r.Git.Rebase.DeleteUpdateRefTodos(refs); err != nil {
				return err
			}
		}
		if len(rest) == 0 {
			return nil
		}
		return r.Git.Rebase.EditRebaseTodo(rest, todo.Drop, "")
	}
	if a, ok := actions[action]; ok {
		return r.Git.Rebase.EditRebaseTodo(commits[start:end+1], a, "")
	}
	return fmt.Errorf("%s is not supported on a rebase todo", action)
}

func (r *Repo) AmendAttribute(commits []*models.Commit, start, end int, kind, value string) error {
	return r.locked(func() error {
		switch kind {
		case "reset":
			return r.Git.Rebase.ResetCommitAuthor(commits, start, end)
		case "set":
			return r.Git.Rebase.SetCommitAuthor(commits, start, end, value)
		case "coauthor":
			return r.Git.Rebase.AddCommitCoAuthor(commits, start, end, value)
		}
		return fmt.Errorf("unknown amend attribute: %s", kind)
	})
}

func (r *Repo) AmendTo(commits []*models.Commit, i int) error {
	if models.IsHeadCommit(commits, i) {
		return r.AmendHead()
	}
	return r.locked(func() error { return r.Git.Rebase.AmendTo(commits, i) })
}

func (r *Repo) StartRebase(hash string) error {
	return r.locked(func() error { return r.Git.Rebase.EditRebase(hash) })
}

func (r *Repo) FixupMessage(c *models.Commit, flag string) error {
	return r.locked(func() error { return r.Git.Rebase.EditRebaseTodo([]*models.Commit{c}, todo.Fixup, flag) })
}

func (r *Repo) SkipEmptyCommit() error {
	return r.locked(func() error {
		state := r.Git.Status.WorkingTreeState()
		return r.Git.Rebase.GenericMergeOrRebaseAction(state.CommandName(), "skip")
	})
}

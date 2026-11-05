package repo

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

func (r *Repo) NewWorktree(path, base, branch string, detach bool) error {
	return r.locked(func() error {
		if !filepath.IsAbs(path) {
			path = filepath.Join(r.Git.RepoPaths.WorktreePath(), path)
		}
		return r.Git.Worktree.New(git_commands.NewWorktreeOpts{Path: path, Base: base, Branch: branch, Detach: detach})
	})
}

func (r *Repo) DeleteWorktree(w *models.Worktree) error {
	return r.locked(func() error {
		if w.IsMain || w.IsCurrent {
			return fmt.Errorf("cannot remove the main or current worktree")
		}
		return r.Git.Worktree.Delete(w.Path, false)
	})
}

func (r *Repo) AddSubmodule(url, name, path string) error {
	return r.locked(func() error {
		path = filepath.Clean(path)
		if path == "." || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			return fmt.Errorf("submodule path must be inside the repository")
		}
		return r.Git.Submodule.Add(name, path, url)
	})
}

func (r *Repo) UpdateSubmodule(path string) error {
	return r.locked(func() error { return r.Git.Submodule.Update(path) })
}

func (r *Repo) InitSubmodule(path string) error {
	return r.locked(func() error { return r.Git.Submodule.Init(path) })
}

func (r *Repo) UpdateSubmoduleURL(s *models.SubmoduleConfig, url string) error {
	return r.locked(func() error { return r.Git.Submodule.UpdateUrl(s, url) })
}

func (r *Repo) DeleteSubmodule(s *models.SubmoduleConfig) error {
	return r.locked(func() error { return r.Git.Submodule.Delete(s) })
}

// BulkSubmodules updates to the recorded commits without discarding local changes.
func (r *Repo) BulkSubmodules(operation string) error {
	return r.locked(func() error {
		switch operation {
		case "init":
			return r.Git.Submodule.BulkInitCmdObj().Run()
		case "update":
			return r.Git.Submodule.BulkUpdateCmdObj().Run()
		case "recursive":
			return r.Git.Submodule.BulkUpdateRecursivelyCmdObj().Run()
		default:
			return fmt.Errorf("unknown submodule operation: %s", operation)
		}
	})
}

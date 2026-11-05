package repo

import (
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

// WorktreeNodeDiff is the diff of a files-panel item (a file or a
// directory) on one side of the index, as the focused main view shows it.
func (r *Repo) WorktreeNodeDiff(n models.IFile, paths []string, staged bool) string {
	out, _ := r.Git.WorkingTree.WorktreeFileDiffCmdObj(n, git_commands.DiffModePlain, staged, paths).RunWithOutput()
	return out
}

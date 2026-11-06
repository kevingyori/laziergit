package repo

import (
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// WorktreeNodeDiff is the diff of a files-panel item (a file or a
// directory) on one side of the index, as the focused main view shows it.
func (r *Repo) WorktreeNodeDiff(n models.IFile, paths []string, staged bool) string {
	out, _ := r.Git.WorkingTree.WorktreeFileDiffCmdObj(n, git_commands.DiffModePlain, staged, paths).RunWithOutput()
	return out
}

// WorktreeNodeViewDiff is WorktreeNodeDiff for reading in the unfocused main
// view: with git.ignoreWhitespaceInDiffView it leaves whitespace changes out,
// as lazygit's rendered diff does (a plain diff never does, since patches
// are built from it).
func (r *Repo) WorktreeNodeViewDiff(n models.IFile, paths []string, staged bool) string {
	if !r.Config.Git.IgnoreWhitespaceInDiffView {
		return r.WorktreeNodeDiff(n, paths, staged)
	}
	out, _ := r.Git.WorkingTree.WorktreeFileDiffCmdObj(n, git_commands.DiffModeRaw, staged, paths).RunWithOutput()
	return utils.Decolorise(out)
}

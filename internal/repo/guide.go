package repo

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
)

// GuideDiff is a read-only, unfiltered review snapshot. A base reviews committed
// branch changes from the merge base; empty base reviews the index and worktree
// separately, so partially staged files never hide either side.
func (r *Repo) GuideDiff(base string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := func(args ...string) (string, error) {
		return r.OS.Cmd.New(git_commands.NewGitCmd(args[0]).Arg(args[1:]...).ToArgv()).DontLog().RunWithOutput()
	}
	if base != "" {
		ancestor, err := run("merge-base", "--", base, "HEAD")
		if err != nil {
			return "", err
		}
		return run("diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", strings.TrimSpace(ancestor), "HEAD", "--")
	}
	staged, err := run("diff", "--cached", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--")
	if err != nil {
		return "", err
	}
	unstaged, err := run("diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--")
	if err != nil {
		return "", err
	}
	// lazygit already knows how to render untracked files (including binaries).
	files := r.Git.Loaders.FileLoader.GetStatusFiles(git_commands.GetStatusFileOptions{ForceShowUntracked: true})
	for _, f := range files {
		if !f.Tracked {
			unstaged += r.FileDiff(f)
		}
	}
	// Prefixes label the two snapshots in the guide's hunk model.
	return "# guide:index\n" + staged + "# guide:worktree\n" + unstaged, nil
}

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

// GuideContext is what a guide writer may know beyond the diff: the branch,
// the commits in range and the trees that pin unchanged files.
type GuideContext struct {
	Scope, Branch, OldTree, NewTree string
	Commits                         []string
}

func (r *Repo) GuideContext(base string) GuideContext {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := func(args ...string) string {
		out, err := r.OS.Cmd.New(git_commands.NewGitCmd(args[0]).Arg(args[1:]...).ToArgv()).DontLog().RunWithOutput()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}
	ctx := GuideContext{Branch: run("rev-parse", "--abbrev-ref", "HEAD"), NewTree: run("rev-parse", "--verify", "--quiet", "HEAD")}
	ctx.OldTree = ctx.NewTree
	rng := []string{"--max-count", "5", "HEAD"}
	ctx.Scope = "Working tree: staged and unstaged changes and untracked files against HEAD. Commits listed are recent history for context, not part of the change."
	if base != "" {
		ctx.OldTree = run("merge-base", "--", base, "HEAD")
		rng = []string{"--max-count", "30", ctx.OldTree + "..HEAD"}
		ctx.Scope = "Branch: the commits from the merge base with " + base + " through HEAD, excluding uncommitted edits."
	}
	if ctx.NewTree == "" {
		return ctx
	}
	// Records are NUL-separated so commit bodies keep their own blank lines.
	for _, c := range strings.Split(run(append([]string{"log", "--format=%s%n%n%b%x00"}, rng...)...), "\x00") {
		c = strings.TrimSpace(c)
		if len(c) > 1200 {
			c = c[:1200] + "…"
		}
		if c != "" {
			ctx.Commits = append(ctx.Commits, c)
		}
	}
	return ctx
}

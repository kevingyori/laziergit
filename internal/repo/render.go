package repo

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/samber/lo"
)

// Rendering a diff through git.diffRenderers, as lazygit's newRenderTask
// (gui/main_view_render.go) does on its piped path: git runs with the
// renderer's arguments, an external diff is named in GIT_EXTERNAL_DIFF, and a
// stdin filter is a command of our own that git's output is piped into.

// DiffCmd builds a diff command in the given mode.
type DiffCmd func(mode git_commands.DiffMode) *oscommands.CmdObj

// RenderView is what a render needs to know about the view it goes into.
type RenderView struct {
	Width int  // the view's inner width in columns
	Light bool // the app shows a light background
}

// RenderersConfigured reports whether git.diffRenderers names any renderer:
// only then does the app show rendered diffs; otherwise it draws git's plain
// diff natively.
func (r *Repo) RenderersConfigured() bool {
	return len(r.Config.Git.DiffRenderers) > 0
}

// Render runs build's command in mode and returns its output as the view
// shows it: through the configured renderer for DiffModeRendered, git's own
// coloured diff for DiffModeRaw.
func (r *Repo) Render(build DiffCmd, mode git_commands.DiffMode, view RenderView) string {
	cmdObj := build(mode).DontLog()
	cmd := cmdObj.GetCmd()
	// lazygit asks every renderer for OSC 1717 records (see newRenderTask).
	cmd.Env = append(cmd.Env, "OSC1717=V1")
	if mode != git_commands.DiffModeRendered || r.DiffRenderers.GetDiffRendererType() == config.DiffRendererType_RawGit {
		cmd.Env = append(cmd.Env, fmt.Sprintf("COLUMNS=%d", view.Width))
		out, err := cmdObj.RunWithOutput()
		if err != nil && out == "" {
			return err.Error()
		}
		return out
	}
	values := config.DiffRendererValues{
		Width:           view.Width,
		DiffContext:     r.Config.Git.DiffContextSize,
		LightBackground: view.Light,
	}
	stdinFilter, stdinErr := r.DiffRenderers.GetStdinFilterCommand(values)
	externalDiff, extErr := r.DiffRenderers.GetExternalDiffCommand(values)
	if err := errors.Join(stdinErr, extErr); err != nil {
		return err.Error()
	}
	env := cmd.Env[:0:0]
	for _, v := range cmd.Env {
		if !isTermEnvVar(v) {
			env = append(env, v)
		}
	}
	cmd.Env = append(env, "TERM=dumb")
	if externalDiff != "" {
		cmd.Env = append(cmd.Env, "GIT_EXTERNAL_DIFF="+externalDiff)
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("COLUMNS=%d", view.Width))
	if stdinFilter == "" {
		out, err := cmdObj.RunWithOutput()
		if err != nil && out == "" {
			return err.Error()
		}
		return out
	}
	// The filter runs in a plain shell with git's environment (pipedRender).
	pipeline, reader, err := r.OS.StartPipeline(
		r.OS.Cmd.NewFromCmd(cmd).DontLog(),
		r.OS.Cmd.NewShell(stdinFilter, "").SetEnviron(cmd.Env).DontLog(),
	)
	if err != nil {
		return err.Error()
	}
	b, _ := io.ReadAll(reader)
	reader.Close()
	_ = pipeline.Wait()
	return string(b)
}

func isTermEnvVar(v string) bool {
	for _, p := range []string{"TERM=", "TERM_PROGRAM=", "TERM_PROGRAM_VERSION=", "TERMINAL_EMULATOR=", "TERMINAL_NAME=", "TERMINAL_VERSION_"} {
		if strings.HasPrefix(v, p) {
			return true
		}
	}
	return false
}

// rendererProbe is lazygit's DiffLineHelper verdict cache: whether the
// current renderer states OSC 1717 records, asked once per renderer.
type rendererProbe struct {
	mu        sync.Mutex
	signature string
	asked     bool
	verdict   bool
}

var probes sync.Map // *Repo → *rendererProbe

// DiffNeedsMetadata is lazygit's diffNeedsMetadata: a renderer other than
// git's own plain diff can be acted on only through its records.
func (r *Repo) DiffNeedsMetadata() bool {
	m := r.DiffRenderers
	if m.GetDiffRendererType() != config.DiffRendererType_RawGit {
		return true
	}
	return len(m.GetRawGitArgs()) > 0
}

// RendererEmitsMetadata is lazygit's diffRendererEmitsMetadata. It runs the
// renderer, so call it off the main thread.
func (r *Repo) RendererEmitsMetadata() bool {
	m := r.DiffRenderers
	index, _ := m.CurrentDiffRendererIndex()
	values := config.DiffRendererValues{}
	ext, _ := m.GetExternalDiffCommand(values)
	filter, _ := m.GetStdinFilterCommand(values)
	sig := fmt.Sprintf("%d\x00%s\x00%s\x00%s", index, ext, filter, strings.Join(m.GetRawGitArgs(), "\x00"))
	v, _ := probes.LoadOrStore(r, &rendererProbe{})
	p := v.(*rendererProbe)
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.asked || p.signature != sig {
		p.verdict = r.Git.Diff.ProbeDiffRendererEmitsMetadata()
		p.signature, p.asked = sig, true
	}
	return p.verdict
}

// FocusedDiffMode is lazygit's MainViewDiffMode for a focused main view:
// git's own diff when the renderer's rows couldn't be placed.
func (r *Repo) FocusedDiffMode() git_commands.DiffMode {
	if r.DiffNeedsMetadata() && !r.RendererEmitsMetadata() {
		return git_commands.DiffModeRaw
	}
	return git_commands.DiffModeRendered
}

// CommitDiffCmd is CommitDiff's command.
func (r *Repo) CommitDiffCmd(c *models.Commit) DiffCmd {
	paths := r.FilterPathsForCommit(c)
	return func(m git_commands.DiffMode) *oscommands.CmdObj { return r.Git.Commit.ShowCmdObj(c.Hash(), paths, m) }
}

// RangeDiffCmd is RangeDiff's command, without its prefix (RangeDiffPrefix).
func (r *Repo) RangeDiffCmd(from, to *models.Commit) DiffCmd {
	args := []string{from.ParentRefName(), to.RefName(), "--stat", "-p", "--"}
	if path, _ := r.Filter(); path != "" {
		args = append(args, lo.Uniq(append(r.FilterPathsForCommit(from), r.FilterPathsForCommit(to)...))...)
	}
	return r.DiffArgsCmd(args)
}

// RangeDiffPrefix is the line lazygit shows above a range's diff.
func (r *Repo) RangeDiffPrefix(from, to *models.Commit) string {
	return fmt.Sprintf("%s %s-%s\n\n", r.Common.Tr.ShowingDiffForRange, from.ShortRefName(), to.ShortRefName())
}

// DiffArgsCmd is `git diff` with args.
func (r *Repo) DiffArgsCmd(args []string) DiffCmd {
	return func(m git_commands.DiffMode) *oscommands.CmdObj { return r.Git.Diff.DiffCmdObj(args, m) }
}

// StashDiffCmd is StashDiff's command.
func (r *Repo) StashDiffCmd(i int) DiffCmd {
	return func(m git_commands.DiffMode) *oscommands.CmdObj { return r.Git.Stash.ShowStashEntryCmdObj(i, m) }
}

// WorktreeNodeDiffCmd is WorktreeNodeDiff's command.
func (r *Repo) WorktreeNodeDiffCmd(n models.IFile, paths []string, staged bool) DiffCmd {
	return func(m git_commands.DiffMode) *oscommands.CmdObj {
		return r.Git.WorkingTree.WorktreeFileDiffCmdObj(n, m, staged, paths)
	}
}

// DiffPathsCmd is DiffPaths's command.
func (r *Repo) DiffPathsCmd(from, to string, reverse bool, paths []string) DiffCmd {
	return func(m git_commands.DiffMode) *oscommands.CmdObj {
		return r.Git.WorkingTree.ShowFileDiffCmdObj(from, to, reverse, paths, m)
	}
}

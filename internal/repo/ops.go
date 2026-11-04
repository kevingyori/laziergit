package repo

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"github.com/jesseduffield/lazygit/pkg/gocui"
)

// Write operations. Each takes the repo lock so a snapshot never observes a
// half-applied change; network operations (push/pull/fetch) don't, so the
// UI keeps refreshing while they run.

func (r *Repo) locked(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return fn()
}

func (r *Repo) StageAll() error {
	return r.locked(func() error { return r.Git.WorkingTree.StageAll(false) })
}

func (r *Repo) UnstageAll() error {
	return r.locked(r.Git.WorkingTree.UnstageAll)
}

// DiscardAll discards every change to f, staged and unstaged (lazygit's
// "Discard all changes" in the files panel).
func (r *Repo) DiscardAll(f *models.File) error {
	return r.locked(func() error { return r.Git.WorkingTree.DiscardAllFileChanges(f) })
}

// DiscardUnstaged discards only f's unstaged changes.
func (r *Repo) DiscardUnstaged(f *models.File) error {
	return r.locked(func() error { return r.Git.WorkingTree.DiscardUnstagedFileChanges(f) })
}

func (r *Repo) Commit(summary, description string, skipHooks bool) error {
	return r.locked(func() error {
		return r.Git.Commit.CommitCmdObj(summary, description, skipHooks).PromptOnCredentialRequest(gocui.NewFakeTask()).Run()
	})
}

func (r *Repo) AmendHead() error {
	return r.locked(func() error {
		return r.Git.Commit.AmendHeadCmdObj().PromptOnCredentialRequest(gocui.NewFakeTask()).Run()
	})
}

// HeadMessage returns the last commit's message (for amend prefill).
func (r *Repo) HeadMessage() string {
	out, err := r.Git.Commit.GetCommitMessage("HEAD")
	if err != nil {
		return ""
	}
	return out
}

func (r *Repo) Push(opts git_commands.PushOpts) error {
	return r.Git.Sync.Push(gocui.NewFakeTask(), opts)
}

func (r *Repo) Pull() error {
	return r.Git.Sync.Pull(gocui.NewFakeTask(), git_commands.PullOptions{})
}

func (r *Repo) Fetch() error {
	return r.Git.Sync.Fetch(gocui.NewFakeTask())
}

// FetchBackground fetches without prompting for credentials (lazygit's
// autoFetch).
func (r *Repo) FetchBackground() error {
	return r.Git.Sync.FetchBackground()
}

func (r *Repo) Checkout(name string) error {
	return r.locked(func() error { return r.Git.Branch.Checkout(name, git_commands.CheckoutOptions{}) })
}

func (r *Repo) NewBranch(name, base string) error {
	if base == "" {
		base = "HEAD"
	}
	return r.locked(func() error { return r.Git.Branch.New(name, base) })
}

func (r *Repo) DeleteBranch(name string, force bool) error {
	return r.locked(func() error { return r.Git.Branch.LocalDelete([]string{name}, force) })
}

func (r *Repo) StashPush(message string) error {
	return r.locked(func() error { return r.Git.Stash.Push(message) })
}

func (r *Repo) StashApply(i int) error {
	return r.locked(func() error { return r.Git.Stash.Apply(i) })
}

func (r *Repo) StashPop(i int) error {
	return r.locked(func() error { return r.Git.Stash.Pop(i) })
}

func (r *Repo) StashDrop(i int) error {
	return r.locked(func() error { return r.Git.Stash.Drop(i) })
}

// WorktreePatch parses f's diff on one side (staged or unstaged) for the
// staging view. Line indices of the result are what ApplyLines takes.
func (r *Repo) WorktreePatch(f *models.File, staged bool) *patch.Patch {
	return patch.Parse(r.Git.WorkingTree.WorktreeFileDiff(f, git_commands.DiffModePlain, staged))
}

// ApplyLines stages (or, on the staged side, unstages) the selected line
// indices of p — lazygit's line staging. Selecting every change acts on the
// whole file, as lazygit does, so added and deleted files stay intact.
func (r *Repo) ApplyLines(f *models.File, p *patch.Patch, staged bool, lines []int) error {
	return r.locked(func() error {
		changes, selected := 0, 0
		sel := make(map[int]bool, len(lines))
		for _, i := range lines {
			sel[i] = true
		}
		for i, l := range p.Lines() {
			if l.IsChange() {
				changes++
				if sel[i] {
					selected++
				}
			}
		}
		if selected == 0 {
			return nil
		}
		if selected == changes {
			if staged {
				return r.Git.WorkingTree.UnStageFile(f.Names(), f.Tracked)
			}
			return r.Git.WorkingTree.StageFile(f.Path)
		}
		if !f.Tracked && !staged {
			// A partial patch of a new file needs the file in the index.
			if err := r.OS.Cmd.New(git_commands.NewGitCmd("add").Arg("--intent-to-add", "--", f.Path).ToArgv()).Run(); err != nil {
				return err
			}
		}
		text := p.Transform(patch.TransformOpts{
			Reverse:             staged,
			IncludedLineIndices: lines,
			FileNameOverride:    f.Path,
		}).FormatPlain()
		return r.Git.Patch.ApplyPatch(text, git_commands.ApplyPatchOpts{Cached: true, Reverse: staged})
	})
}

// DiscardLines removes the selected unstaged lines from the working tree.
func (r *Repo) DiscardLines(f *models.File, p *patch.Patch, lines []int) error {
	return r.locked(func() error {
		text := p.Transform(patch.TransformOpts{
			Reverse:             true,
			IncludedLineIndices: lines,
			FileNameOverride:    f.Path,
		}).FormatPlain()
		return r.Git.Patch.ApplyPatch(text, git_commands.ApplyPatchOpts{Reverse: true})
	})
}

// CommitFiles lists the files changed by a commit.
func (r *Repo) CommitFiles(c *models.Commit) ([]*models.CommitFile, error) {
	from := c.Hash() + "^"
	if c.IsFirstCommit() {
		from = models.EmptyTreeCommitHash
	}
	return r.Git.Loaders.CommitFileLoader.GetFilesInDiff(from, c.Hash(), false)
}

// CommitFileDiff is one file's diff within a commit.
func (r *Repo) CommitFileDiff(c *models.Commit, f *models.CommitFile) string {
	from := c.Hash() + "^"
	if c.IsFirstCommit() {
		from = models.EmptyTreeCommitHash
	}
	out, err := r.Git.WorkingTree.ShowFileDiff(from, c.Hash(), false, f.Path, f.PreviousPath, git_commands.DiffModePlain)
	if err != nil {
		return err.Error()
	}
	return out
}

// UpstreamSuggestion is what lazygit pre-fills when pushing a branch without
// an upstream: "<remote> <branch>".
func (r *Repo) UpstreamSuggestion(branch string) string {
	remotes := r.Git.Loaders.RemoteLoader.GetRemotes()
	remote := "origin"
	if len(remotes) > 0 {
		remote = remotes[0].Name
		for _, rm := range remotes {
			if rm.Name == "origin" {
				remote = "origin"
			}
		}
	}
	return fmt.Sprintf("%s %s", remote, branch)
}

// ParseUpstream splits "remote branch" or "remote/branch".
func ParseUpstream(s string) (remote, branch string, ok bool) {
	s = strings.TrimSpace(s)
	if r, b, found := strings.Cut(s, " "); found {
		return r, strings.TrimSpace(b), r != "" && strings.TrimSpace(b) != ""
	}
	if r, b, found := strings.Cut(s, "/"); found {
		return r, b, r != "" && b != ""
	}
	return "", "", false
}

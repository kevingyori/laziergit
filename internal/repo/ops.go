package repo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"github.com/jesseduffield/lazygit/pkg/gocui"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
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
		return r.gitCommandFailed(r.Git.Commit.CommitCmdObj(summary, description, skipHooks).PromptOnCredentialRequest(gocui.NewFakeTask()).Run())
	})
}

// gitCommandFailed is lazygit's GpgHelper.runAndStream: the output of a
// failed commit already streamed to the user, so the error only points to the
// command log.
func (r *Repo) gitCommandFailed(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(r.Common.Tr.GitCommandFailed, r.Config.Keybinding.Universal.ExtrasMenu)
}

func (r *Repo) AmendHead() error {
	return r.locked(func() error {
		return r.gitCommandFailed(r.Git.Commit.AmendHeadCmdObj().PromptOnCredentialRequest(gocui.NewFakeTask()).Run())
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
	return r.DiffFiles(CommitFrom(c), c.Hash(), false)
}

// CommitFrom is the ref a commit's own diff starts from.
func CommitFrom(c *models.Commit) string {
	if c.IsFirstCommit() {
		return models.EmptyTreeCommitHash
	}
	return c.Hash() + "^"
}

// DiffFiles are the files changed between two refs (lazygit's commit files
// of a ref range or of diffing mode).
func (r *Repo) DiffFiles(from, to string, reverse bool) ([]*models.CommitFile, error) {
	return r.Git.Loaders.CommitFileLoader.GetFilesInDiff(from, to, reverse)
}

// DiffPaths is the diff between two refs limited to paths.
func (r *Repo) DiffPaths(from, to string, reverse bool, paths []string) (string, error) {
	return r.Git.WorkingTree.ShowFileDiffCmdObj(from, to, reverse, paths, git_commands.DiffModePlain).DontLog().RunWithOutput()
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

func (r *Repo) SetUpstream(remote, branch string) error {
	return r.locked(func() error { return r.Git.Branch.SetCurrentBranchUpstream(remote, branch) })
}

// ToggleStagedDir stages a directory when anything below it is unstaged,
// else unstages it, as lazygit's space on a directory does (directories
// count as tracked when unstaging).
func (r *Repo) ToggleStagedDir(n *filetree.FileNode) error {
	return r.locked(func() error {
		if n.GetHasInlineMergeConflicts() {
			return errors.New(r.Common.Tr.ErrStageDirWithInlineMergeConflicts)
		}
		if n.GetHasUnstagedChanges() {
			return r.Git.WorkingTree.StageFile(n.GetPath())
		}
		return r.Git.WorkingTree.UnstageTrackedFiles([]string{n.GetPath()})
	})
}

// DiscardNodes discards every change in the given rows, or only the
// unstaged ones (lazygit's DiscardAllDirChanges / DiscardUnstagedDirChanges).
func (r *Repo) DiscardNodes(nodes []*filetree.FileNode, unstagedOnly bool) error {
	return r.locked(func() error {
		ns := make([]git_commands.IFileNode, len(nodes))
		for i, n := range nodes {
			ns[i] = n
		}
		if unstagedOnly {
			return r.Git.WorkingTree.DiscardUnstagedDirChanges(ns)
		}
		return r.Git.WorkingTree.DiscardAllDirChanges(ns)
	})
}

// DiscardDir discards every change below a directory, or only the unstaged ones.
func (r *Repo) DiscardDir(n *filetree.FileNode, unstagedOnly bool) error {
	return r.locked(func() error {
		nodes := []git_commands.IFileNode{n}
		if unstagedOnly {
			return r.Git.WorkingTree.DiscardUnstagedDirChanges(nodes)
		}
		return r.Git.WorkingTree.DiscardAllDirChanges(nodes)
	})
}

// DirDiff is the working-tree diff of everything below a directory: staged
// when nothing below it is unstaged, as for a file.
func (r *Repo) DirDiff(n *filetree.FileNode, paths []string) string {
	cached := !n.GetHasUnstagedChanges() && n.GetHasStagedChanges()
	out, _ := r.Git.WorkingTree.WorktreeFileDiffCmdObj(n, git_commands.DiffModePlain, cached, paths).DontLog().RunWithOutput()
	return out
}

// CommitPathDiff is a commit's diff limited to a path (a directory in the
// commit-files tree).
func (r *Repo) CommitPathDiff(c *models.Commit, paths []string) string {
	from := c.Hash() + "^"
	if c.IsFirstCommit() {
		from = models.EmptyTreeCommitHash
	}
	out, err := r.Git.WorkingTree.ShowFileDiffCmdObj(from, c.Hash(), false, paths, git_commands.DiffModePlain).DontLog().RunWithOutput()
	if err != nil {
		return err.Error()
	}
	return out
}

// DeleteRemoteBranch deletes the actual upstream name and allows credential prompts.
// DeleteBranches force-deletes local branches (lazygit's doDeleteLocalBranches).
func (r *Repo) DeleteBranches(names []string) error {
	return r.locked(func() error { return r.Git.Branch.LocalDelete(names, true) })
}

func (r *Repo) DeleteRemoteBranches(remote string, branches []string) error {
	return r.Git.Remote.DeleteRemoteBranch(gocui.NewFakeTask(), remote, branches)
}

func (r *Repo) DeleteRemoteBranch(remote, branch string) error {
	return r.Git.Remote.DeleteRemoteBranch(gocui.NewFakeTask(), remote, []string{branch})
}

func (r *Repo) BranchMerged(branch *models.Branch) (bool, error) {
	return r.Git.Branch.IsBranchMerged(branch, r.mainBr)
}

func (r *Repo) ReleaseWorktree(w *models.Worktree, detach, force bool) error {
	return r.locked(func() error {
		if detach {
			return r.Git.Worktree.Detach(w.Path)
		}
		return r.Git.Worktree.Delete(w.Path, force)
	})
}

func (r *Repo) AddRemote(name, url string) error {
	return r.locked(func() error { return r.Git.Remote.AddRemote(name, url) })
}

func (r *Repo) RemoveRemote(name string) error {
	return r.locked(func() error { return r.Git.Remote.RemoveRemote(name) })
}

func (r *Repo) RenameRemote(from, to string) error {
	return r.locked(func() error { return r.Git.Remote.RenameRemote(from, to) })
}

func (r *Repo) UpdateRemoteURL(name, url string) error {
	return r.locked(func() error { return r.Git.Remote.UpdateRemoteUrl(name, url) })
}

// FetchRemote fetches one remote, then (lazygit's fetchAndCheckout) creates
// and checks out branch from it when branch is given.
func (r *Repo) FetchRemote(name, branch string) (checkedOut bool, err error) {
	if err := r.Git.Sync.FetchRemote(gocui.NewFakeTask(), name); err != nil {
		return false, err
	}
	if branch == "" {
		return false, nil
	}
	err = r.locked(func() error { return r.Git.Branch.New(branch, name+"/"+branch) })
	return err == nil, err
}

// RefreshLines finds the selected lines of a diff that may be stale in the
// file's current diff (lazygit re-reads the diff before staging): it returns
// the fresh patch and the selected lines' indices in it, or ok=false when a
// selected line is no longer there.
func (r *Repo) RefreshLines(f *models.File, p *patch.Patch, staged bool, lines []int) (*patch.Patch, []int, bool) {
	fresh := patch.Parse(r.Git.WorkingTree.WorktreeFileDiff(f, git_commands.DiffModePlain, staged))
	type key struct {
		kind    patch.PatchLineKind
		content string
		line    int
	}
	index := map[key]int{}
	for i, l := range fresh.Lines() {
		if l.IsChange() {
			index[key{l.Kind, l.Content, fresh.LineNumberOfLine(i)}] = i
		}
	}
	old := p.Lines()
	out := make([]int, 0, len(lines))
	for _, i := range lines {
		if i < 0 || i >= len(old) {
			return nil, nil, false
		}
		l := old[i]
		j, ok := index[key{l.Kind, l.Content, p.LineNumberOfLine(i)}]
		if !ok {
			return nil, nil, false
		}
		out = append(out, j)
	}
	return fresh, out, true
}

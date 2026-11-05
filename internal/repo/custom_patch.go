package repo

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
)

// EditPatch serializes patch changes and history rewrites with snapshot loads.
// Call only from a git worker, never while drawing a view.
func (r *Repo) EditPatch(edit func(*git_commands.PatchCommands) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return edit(r.Git.Patch)
}

// CustomPatchPreview materializes the builder using lazygit's diff trees.
// A partial deletion must leave a file in the preview's after tree. Its
// apply-in-reverse representation still needs the deletion header when
// removing it from the source commit, so normalize that header only here.
// All patches used to rewrite history stay under lazygit's control.
// Caller holds EditPatch's lock.
func (r *Repo) CustomPatchPreview(p *git_commands.PatchCommands) (string, error) {
	b := p.PatchBuilder
	partialDeletion := false
	var patches []string
	for _, name := range b.AllFilesInPatch() {
		status := b.GetFileStatus(name, b.To)
		if status == patch.UNSELECTED {
			continue
		}
		text := b.RenderPatchForFile(patch.RenderPatchForFileOpts{Filename: name, Plain: true})
		if status == patch.PART && strings.Contains(text, "\ndeleted file mode ") {
			partialDeletion = true
			parsed := patch.Parse(text)
			all := make([]int, len(parsed.Lines()))
			for i := range all {
				all[i] = i
			}
			text = parsed.Transform(patch.TransformOpts{FileNameOverride: name, IncludedLineIndices: all}).FormatPlain()
		}
		patches = append(patches, text)
	}
	if !partialDeletion {
		if err := p.EnsureCustomPatchDiffTrees(); err != nil {
			return "", err
		}
	} else {
		dir := b.TempDir()
		before, after := filepath.Join(dir, "a"), filepath.Join(dir, "b")
		for _, tree := range []string{before, after} {
			if err := os.RemoveAll(tree); err != nil {
				return "", err
			}
			if err := os.MkdirAll(tree, 0o700); err != nil {
				return "", err
			}
		}
		for _, file := range b.FilesInPatch() {
			content, err := r.Git.Commit.ShowFileContentCmdObj(b.From, file.ContentPath).RunWithOutput()
			added := err != nil
			if added {
				content = ""
			}
			if err := r.OS.CreateFileWithContent(filepath.Join(before, file.Path), content); err != nil {
				return "", err
			}
			if !added {
				if err := r.OS.CreateFileWithContent(filepath.Join(after, file.Path), content); err != nil {
					return "", err
				}
			}
		}
		path, err := p.SaveTemporaryPatch(strings.Join(patches, ""))
		if err != nil {
			return "", err
		}
		if err := r.OS.Cmd.New(git_commands.NewGitCmd("apply").NoLineEndingConversion().Arg(path).Dir(after).ToArgv()).Run(); err != nil {
			return "", err
		}
	}
	// A no-index diff exits 1 when the trees differ; the output is the preview.
	out, err := r.Git.Diff.CustomPatchDiffCmdObj(b.TempDir(), git_commands.DiffModePlain).RunWithOutput()
	if out != "" {
		return out, nil
	}
	return out, err
}

package repo

import (
	"fmt"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/mergeconflicts"
	"os"
	"strings"
)

func (r *Repo) ResolveFile(f *models.File, choice string) error {
	return r.locked(func() error {
		switch choice {
		case "top", "bottom", "both":
			b, err := os.ReadFile(f.Path)
			if err != nil {
				return err
			}
			state := mergeconflicts.NewState()
			state.SetContent(string(b), f.Path, f.ConflictMarkerSize)
			selection := mergeconflicts.TOP
			if choice == "bottom" {
				selection = mergeconflicts.BOTTOM
			}
			if choice == "both" {
				selection = mergeconflicts.BOTH
			}
			ok, content, err := state.ContentAfterConflictResolve(selection)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("no inline conflict remains")
			}
			info, err := os.Stat(f.Path)
			if err != nil {
				return err
			}
			if err := os.WriteFile(f.Path, []byte(content), info.Mode()); err != nil {
				return err
			}
			state.SetContent(content, f.Path, f.ConflictMarkerSize)
			if !state.NoConflicts() {
				return nil
			}

		case "ours", "theirs":
			if err := r.OS.Cmd.New(git_commands.NewGitCmd("checkout").Arg("--"+choice, "--", f.Path).ToArgv()).Run(); err != nil {
				return err
			}
		case "delete":
			return r.Git.WorkingTree.RemoveConflictedFile(f.Path)
		case "resolved":
			b, err := os.ReadFile(f.Path)
			if err != nil {
				return err
			}
			size := f.ConflictMarkerSize
			if size < 7 {
				size = 7
			}
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(line, strings.Repeat("<", size)) || strings.HasPrefix(line, strings.Repeat("=", size)) || strings.HasPrefix(line, strings.Repeat(">", size)) {
					return fmt.Errorf("conflict markers remain in %s", f.Path)
				}
			}
		default:
			return fmt.Errorf("unknown resolution")
		}
		return r.Git.WorkingTree.StageFile(f.Path)
	})
}

func (r *Repo) SequenceControl(kind, action string) error {
	return r.locked(func() error { return r.Git.Rebase.GenericMergeOrRebaseAction(kind, action) })
}

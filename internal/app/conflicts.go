package app

import (
	"os"
	"path/filepath"

	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/keymap"
)

func (a *App) resolveMenu(f *models.File) {
	if a.isSubmoduleCommitConflict(f) {
		a.submoduleConflictMenu(f)
		return
	}
	tr := a.repo.Common.Tr
	keep := menuItem{label: tr.MergeConflictKeepFile, key: "k", chord: keymap.Binding{{Rune: 'k'}}, run: func() {
		a.run("Resolving…", func() error { return a.repo.ResolveFile(f, "resolved") }, a.done("Resolved "+f.Path))
	}}
	del := menuItem{label: tr.MergeConflictDeleteFile, key: "d", chord: keymap.Binding{{Rune: 'd'}}, run: func() {
		a.run("Resolving…", func() error { return a.repo.ResolveFile(f, "delete") }, a.done("Resolved "+f.Path))
	}}
	switch f.ShortStatus { // lazygit's openFileConflictMenu for conflicts without markers
	case "DD":
		a.pushOverlay(&menuOverlay{title: tr.MergeConflictsTitle, items: []menuItem{del}})
		return
	case "DU", "UD":
		a.pushOverlay(&menuOverlay{title: tr.MergeConflictsTitle, items: []menuItem{del, keep}})
		return
	case "AU", "UA":
		a.pushOverlay(&menuOverlay{title: tr.MergeConflictsTitle, items: []menuItem{keep, del}})
		return
	}
	a.openConflict(f)
}

// openMergeConflictMenu is lazygit's FilesController.openMergeConflictMenu.
func (a *App) openMergeConflictMenu() {
	var paths []string
	seen := map[string]bool{}
	for _, n := range a.selectedFileNodes() {
		for _, leaf := range n.GetLeaves() {
			if leaf.File != nil && leaf.File.HasInlineMergeConflicts && !seen[leaf.GetPath()] {
				seen[leaf.GetPath()] = true
				paths = append(paths, leaf.GetPath())
			}
		}
	}
	if len(paths) == 0 {
		tr := a.repo.Common.Tr
		a.setStatus(tr.DisabledMenuItemPrefix+tr.NoFilesWithMergeConflicts, true)
		return
	}
	a.mergeConflictMenu(paths)
}

// mergeConflictMenu is lazygit's WorkingTreeHelper.CreateMergeConflictMenu.
func (a *App) mergeConflictMenu(paths []string) {
	tr := a.repo.Common.Tr
	git := a.repo.Git
	strategy := func(s string) func() {
		return func() {
			a.run(tr.MergeConflictOptionsTitle, func() error {
				return a.repo.Locked(func() error {
					for _, path := range paths {
						out, err := a.mergeFile(path, s)
						if err != nil {
							return err
						}
						if err := os.WriteFile(filepath.Join(git.RepoPaths.WorktreePath(), path), []byte(out), 0o644); err != nil {
							return err
						}
					}
					return git.WorkingTree.StageFiles(paths, nil)
				})
			}, nil)
		}
	}
	a.pushOverlay(&menuOverlay{title: tr.MergeConflictOptionsTitle, items: []menuItem{
		{label: tr.UseCurrentChanges, detail: "git merge-file --ours", key: "c", chord: runeBind('c'), run: strategy("--ours")},
		{label: tr.UseIncomingChanges, detail: "git merge-file --theirs", key: "i", chord: runeBind('i'), run: strategy("--theirs")},
		{label: tr.UseBothChanges, detail: "git merge-file --union", key: "b", chord: runeBind('b'), run: strategy("--union")},
		{label: tr.OpenMergeTool, detail: "git mergetool", key: "m", chord: runeBind('m'), run: func() {
			cmd := a.subprocessCommand(git.WorkingTree.OpenMergeToolCmdObj())
			a.run(tr.Actions.OpenMergeTool, func() error { return a.runTerminal(cmd) }, nil)
		}},
	}})
}

// mergeFile is lazygit's WorkingTreeHelper.mergeFile.
func (a *App) mergeFile(path, strategy string) (string, error) {
	wt := a.repo.Git.WorkingTree
	if a.repo.Git.Version.IsOlderThan(2, 43, 0) {
		var files [3]string
		for i, label := range []string{"base", "ours", "theirs"} {
			out, err := wt.ShowFileAtStage(path, i+1)
			if err != nil {
				return "", err
			}
			f, err := os.CreateTemp(a.repo.OS.GetTempDir(), "mergefile-"+label+"-*")
			if err != nil {
				return "", err
			}
			_, err = f.Write([]byte(out))
			f.Close()
			defer os.Remove(f.Name())
			if err != nil {
				return "", err
			}
			files[i] = f.Name()
		}
		return wt.MergeFileForFiles(strategy, files[1], files[0], files[2])
	}
	var ids [3]string
	for i := range ids {
		id, err := wt.ObjectIDAtStage(path, i+1)
		if err != nil {
			return "", err
		}
		ids[i] = id
	}
	return wt.MergeFileForObjectIDs(strategy, ids[1], ids[0], ids[2])
}

func (a *App) sequenceMenu() {
	var items []menuItem
	for _, kind := range []string{"rebase", "merge", "cherry-pick", "revert"} {
		for _, action := range []string{"continue", "abort"} {
			kind, action := kind, action
			items = append(items, menuItem{label: action + " " + kind, run: func() {
				a.confirm(action+" "+kind, "Run git "+kind+" --"+action+"?", action == "abort", func() {
					a.selectHeadAfterMerge(action)
					a.sequenceControl("Updating…", kind, action)
				})
			}})
		}
	}
	a.pushOverlay(&menuOverlay{title: "Continue / abort operation", items: items})
}

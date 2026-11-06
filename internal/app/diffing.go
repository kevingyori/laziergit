package app

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
)

// diffingState is lazygit's modes/diffing: a blank ref means not diffing.
type diffingState struct {
	ref     string
	reverse bool
}

// diffingFrom is lazygit's Diffing.GetFromAndReverseArgsForDiff: in diffing
// mode every diff starts from the diffed ref, optionally reversed.
func (a *App) diffingFrom(from string) (string, bool) {
	if a.diffing.ref != "" {
		return a.diffing.ref, a.diffing.reverse
	}
	return from, false
}

// diffTerminals is lazygit's DiffHelper.CurrentDiffTerminals: the refs the
// selected item of the side panel can be diffed as.
func (a *App) diffTerminals() []string {
	if a.snap == nil || a.count(a.focus) == 0 {
		return nil
	}
	i := a.cursor[a.focus]
	switch a.focus {
	case panelBranches:
		if s := a.browse(panelBranches); s != nil {
			if s.ref != nil {
				return []string{s.commits[i].RefName()}
			}
			return []string{s.branches[i].RefName()}
		}
		switch a.branchTab {
		case 1:
			return []string{a.snap.Remotes[i].RefName()}
		case 2:
			return []string{a.snap.Tags[i].RefName()}
		}
		b := a.snap.Branches[i]
		names := []string{b.ID()}
		if b.IsTrackingRemote() {
			names = append(names, b.ID()+"@{u}")
		}
		return names
	case panelCommits:
		if cf := a.commitFiles; cf != nil {
			return []string{cf.commit.RefName()}
		}
		if c := a.selectedCommit(); c != nil {
			return []string{c.RefName()}
		}
	case panelStash:
		if s := a.selectedStash(); s != nil {
			return []string{s.RefName()}
		}
	}
	return nil
}

// diffArgs is lazygit's DiffHelper.DiffArgs.
func (a *App) diffArgs() []string {
	args := []string{"--stat", "-p", a.diffing.ref}
	if t := a.diffTerminals(); len(t) > 0 {
		args = append(args, t[0])
	}
	if a.diffing.reverse {
		args = append(args, "-R")
	}
	args = append(args, "--")
	file := ""
	if !a.mainFocused && a.staging == nil {
		switch {
		case a.focus == panelFiles && a.fileTab == 0:
			if n := a.selectedFileNode(); n != nil {
				file = n.GetPath()
			}
		case a.focus == panelCommits && a.commitFiles != nil:
			if n := a.selectedCommitFileNode(); n != nil {
				file = n.GetPath()
			}
		}
	}
	if file != "" {
		args = append(args, file)
	} else if path, _ := a.repo.Filter(); path != "" {
		args = append(args, path)
	}
	return args
}

func (a *App) diffingInfo() string {
	if a.diffing.ref == "" {
		return ""
	}
	return fmt.Sprintf("%s %s", a.repo.Common.Tr.ShowingGitDiff, "git diff "+strings.Join(a.diffArgs(), " "))
}

// diffingSource is lazygit's WithDiffModeCheck → RenderDiff for the side
// panels that render through it; ok is false when the panel renders as usual.
func (a *App) diffingSource() (string, func() string, bool) {
	if a.diffing.ref == "" || a.snap == nil || a.count(a.focus) == 0 {
		return "", nil, false
	}
	switch {
	case a.focus == panelFiles && a.fileTab != 1:
	case a.focus == panelBranches, a.focus == panelStash:
	case a.focus == panelCommits && a.commitFiles == nil:
	default:
		return "", nil, false
	}
	args := a.diffArgs()
	r := a.repo
	prefix := fmt.Sprintf("%s %s\n\n", r.Common.Tr.ShowingGitDiff, "git diff "+strings.Join(args, " "))
	return "diffing:" + strings.Join(args, "\x00"), func() string {
		out, err := r.Git.Diff.DiffCmdObj(args, git_commands.DiffModePlain).DontLog().RunWithOutput()
		if err != nil {
			return prefix + err.Error()
		}
		return prefix + out
	}, true
}

// diffingMenu is lazygit's DiffingMenuAction.
func (a *App) diffingMenu() {
	tr := a.repo.Common.Tr
	var items []menuItem
	for _, name := range a.diffTerminals() {
		items = append(items, menuItem{label: fmt.Sprintf("%s %s", tr.Diff, name), run: func() { a.setDiffing(name, a.diffing.reverse) }})
	}
	items = append(items, menuItem{label: tr.EnterRefToDiff, run: func() {
		a.promptSuggestions(tr.EnterRefName, "", "", "refs", func(ref string) { a.setDiffing(ref, a.diffing.reverse) })
	}})
	if a.diffing.ref != "" {
		items = append(items,
			menuItem{label: tr.SwapDiff, run: func() { a.setDiffing(a.diffing.ref, !a.diffing.reverse) }},
			menuItem{label: tr.ExitDiffMode, run: a.exitDiffing},
		)
	}
	a.pushOverlay(&menuOverlay{title: tr.DiffingMenuTitle, items: items})
}

func (a *App) exitDiffing() { a.setDiffing("", false) }

// setDiffing changes the mode and refreshes what depends on it: the main
// view and an open commit-files list.
func (a *App) setDiffing(ref string, reverse bool) {
	if ref == "" {
		reverse = false
	}
	a.diffing = diffingState{ref: ref, reverse: reverse}
	a.diffKey = ""
	a.loadMain()
	cf := a.commitFiles
	if cf == nil {
		return
	}
	from, to, rev := a.commitFileRefs()
	r := a.repo
	a.bg(func() {
		files, err := r.DiffFiles(from, to, rev)
		a.post(func() {
			if a.repo != r || a.commitFiles != cf || err != nil {
				return
			}
			cf.files, cf.tree = files, newCommitFileTree(a, cf.commit, files)
			a.updateTreeRows()
			a.diffKey = ""
			a.loadMain()
		})
	})
}

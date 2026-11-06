package app

import (
	"fmt"
	"slices"
)

// filteringActive is lazygit's Modes().Filtering.Active().
func (a *App) filteringActive() bool {
	path, author := a.repo.Filter()
	return path != "" || author != ""
}

// filteringInfo is the filtering mode's label in the information view.
func (a *App) filteringInfo() string {
	path, author := a.repo.Filter()
	if path == "" && author == "" {
		return ""
	}
	if path == "" {
		path = author
	}
	return fmt.Sprintf("%s '%s'", a.repo.Common.Tr.FilteringBy, path)
}

// filteringMenu is lazygit's FilteringMenuAction (<c-s>).
func (a *App) filteringMenu() {
	tr := a.repo.Common.Tr
	fileName, author := "", ""
	switch {
	case a.focus == panelFiles && a.fileTab == 0:
		if n := a.selectedFileNode(); n != nil {
			fileName = n.GetPath()
		}
	case a.commitsView() == cvCommitFiles && a.focus == panelCommits:
		if n := a.selectedCommitFileNode(); n != nil {
			fileName = n.GetPath()
		}
	case a.commitsView() == cvCommits && a.focus == panelCommits:
		if c := a.selectedCommit(); c != nil && c.Hash() != "" {
			author = fmt.Sprintf("%s <%s>", c.AuthorName, c.AuthorEmail)
		}
	}
	var items []menuItem
	if fileName != "" {
		items = append(items, menuItem{label: fmt.Sprintf("%s '%s'", tr.FilterBy, fileName), run: func() { a.setFiltering(fileName, "") }})
	}
	if author != "" {
		items = append(items, menuItem{label: fmt.Sprintf("%s '%s'", tr.FilterBy, author), run: func() { a.setFiltering("", author) }})
	}
	items = append(items,
		menuItem{label: tr.FilterPathOption, run: a.filterPathPrompt},
		menuItem{label: tr.FilterAuthorOption, run: func() {
			a.promptSuggestions(tr.EnterAuthor, "", "", "authors", func(s string) { a.setFiltering("", s) })
		}},
	)
	if a.filteringActive() {
		items = append(items, menuItem{label: tr.ExitFilterMode, run: a.clearFiltering})
	}
	items = append(items, menuItem{label: tr.Cancel, run: func() {}})
	a.pushOverlay(&menuOverlay{title: tr.FilteringMenuTitle, items: items})
}

// filterPathPrompt suggests every file in the repo and its directories
// (lazygit's GetFilePathSuggestionsFunc), loaded off the main thread.
func (a *App) filterPathPrompt() {
	tr := a.repo.Common.Tr
	a.prompt(tr.EnterFileName, "", "", func(s string) { a.setFiltering(s, "") })
	o, _ := a.topOverlay().(*promptOverlay)
	if o == nil {
		return
	}
	o.suggestions = a.suggestionFilter(nil)
	r := a.repo
	a.bg(func() {
		files, err := r.Git.WorkingTree.AllRepoFiles()
		if err != nil {
			return
		}
		seen := map[string]bool{}
		var all []string
		for _, f := range files {
			for i := range len(f) {
				if f[i] == '/' && !seen[f[:i]] {
					seen[f[:i]] = true
					all = append(all, f[:i])
				}
			}
			if !seen[f] {
				seen[f] = true
				all = append(all, f)
			}
		}
		slices.Sort(all)
		a.post(func() { o.suggestions = a.suggestionFilter(all) })
	})
}

// setFiltering is lazygit's ModeHelper.setFiltering: replace the filter,
// remember the selected commit, reload and focus the first commit.
func (a *App) setFiltering(path, author string) {
	selected := a.localCommitHash()
	a.filterSelectedHash = selected
	a.changeFiltering(path, author, func() {
		a.commitFiles, a.refBrowse = nil, nil
		a.commitTab = 0
		a.focus = panelCommits
		a.mainFocused = false
		a.cursor[panelCommits] = 0
		a.commitTabCursor[0] = 0
		a.cancelRange(panelCommits)
		a.lists[panelCommits].ScrollTo(0, 0)
	})
}

// clearFiltering is lazygit's ModeHelper.ClearFiltering: keep the commit
// selected in filtering mode, else go back to the one selected before.
func (a *App) clearFiltering() {
	selected := a.localCommitHash()
	before := a.filterSelectedHash
	a.changeFiltering("", "", func() {
		if !a.selectFilterCommit(selected) {
			a.selectFilterCommit(before)
		}
	})
}

func (a *App) changeFiltering(path, author string, then func()) {
	tr := a.repo.Common.Tr
	label := tr.ApplyingFilterStatus
	if path == "" && author == "" {
		label = tr.RemovingFilterStatus
	}
	a.repo.SetFilter(path, author)
	a.run(label, func() error { return nil }, func() {
		then()
		a.diffKey = ""
		a.loadMain()
	})
}

// localCommitHash is LocalCommits.GetSelectedCommitHash.
func (a *App) localCommitHash() string {
	if a.snap == nil {
		return ""
	}
	i := a.cursor[panelCommits]
	if a.commitsView() != cvCommits {
		i = a.commitTabCursor[0]
	}
	if i < 0 || i >= len(a.snap.Commits) {
		return ""
	}
	return a.snap.Commits[i].Hash()
}

func (a *App) selectFilterCommit(hash string) bool {
	if hash == "" || a.snap == nil {
		return false
	}
	for i, c := range a.snap.Commits {
		if c.Hash() == hash {
			if a.commitsView() == cvCommits {
				a.cursor[panelCommits] = i
				a.cancelRange(panelCommits)
				a.lists[panelCommits].ScrollIntoView(i)
			}
			a.commitTabCursor[0] = i
			return true
		}
	}
	return false
}

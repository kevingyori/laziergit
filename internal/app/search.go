package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/sahilm/fuzzy"

	"laziergit/internal/keymap"
)

// Search and filter, ported from lazygit's SearchHelper, SearchTrait,
// FilteredList and the search / filter controllers. lazygit calls both
// "searching": a filterable context (files, branches, remotes, tags, ...)
// narrows its list as the user types, a searchable one (commits, sub-commits,
// the main view) keeps its list and jumps between matches with n / N.

// searchTarget names the lazygit context that has focus and what / does there.
type searchTarget struct {
	key        string
	filterable bool
}

// searchCtx is the per-context state lazygit keeps in SearchTrait and
// FilteredList.
type searchCtx struct {
	filter  string
	search  string
	history *utils.HistoryBuffer[string]
	// cur is the current match; syncedY the cursor line it was last aligned
	// with, so a move by hand re-aligns it (lazygit's SetNearestSearchPosition).
	cur, syncedY int
}

func (a *App) sctx(key string) *searchCtx {
	if a.srch == nil {
		a.srch = map[string]*searchCtx{}
	}
	c := a.srch[key]
	if c == nil {
		c = &searchCtx{history: utils.NewHistoryBuffer[string](1000), syncedY: -1}
		a.srch[key] = c
	}
	return c
}

// searchTarget is the context / would search or filter now.
func (a *App) searchTarget() (searchTarget, bool) {
	switch {
	case a.staging != nil:
		return searchTarget{key: a.stagingKey()}, true
	case a.mainFocused:
		return searchTarget{key: "main"}, true
	}
	switch a.focus {
	case panelFiles:
		return searchTarget{key: [...]string{"files", "worktrees", "submodules"}[a.fileTab], filterable: true}, true
	case panelBranches:
		if b := a.refBrowse; b != nil {
			if b.ref != nil {
				return searchTarget{key: "subCommits"}, true
			}
			return searchTarget{key: "remoteBranches", filterable: true}, true
		}
		return searchTarget{key: [...]string{"localBranches", "remotes", "tags"}[a.branchTab], filterable: true}, true
	case panelCommits:
		if a.commitFiles != nil {
			return searchTarget{key: "commitFiles", filterable: true}, true
		}
		return searchTarget{key: "commits"}, true
	case panelStash:
		return searchTarget{key: "stash", filterable: true}, true
	}
	return searchTarget{}, false
}

// searchScopes are the scopes / is bound in; filterable ones are labelled
// "Filter the current view by text", as in lazygit.
func (a *App) searchActions() []action {
	tr := a.repo.Common.Tr
	keys := keymap.Parse(a.repo.Config.Keybinding.Universal.StartSearch)
	var out []action
	for _, sc := range []scope{scopeFiles, scopeCommitFiles, scopeBranches, scopeRemotes, scopeTags, scopeWorktrees,
		scopeSubmodules, scopeRemoteBranches, scopeStash} {
		out = append(out, action{sc, keys, tr.StartFilter, a.openSearchPrompt, false})
	}
	for _, sc := range []scope{scopeCommits, scopeRefCommits, scopeMain, scopeStaging} {
		out = append(out, action{sc, keys, tr.StartSearch, a.openSearchPrompt, false})
	}
	return out
}

// --- the prompt ----------------------------------------------------------------

// searchOverlay is lazygit's search prompt: a one-line input that filters
// as you type (filterable contexts) or collects the text to search for.
type searchOverlay struct {
	target  searchTarget
	value   string
	histIdx int // lazygit's PrevSearchIndex
}

func (a *App) openSearchPrompt() {
	t, ok := a.searchTarget()
	if !ok {
		return
	}
	o := &searchOverlay{target: t, histIdx: -1}
	if t.filterable {
		a.applyFilter(t.key, "") // OnPromptContentChanged(""): the prompt starts empty
	}
	a.pushOverlay(o)
}

func (o *searchOverlay) prefix(a *App) string {
	if o.target.filterable {
		return a.repo.Common.Tr.FilterPrefix
	}
	return a.repo.Common.Tr.SearchPrefix
}

func (o *searchOverlay) key(a *App, ch keymap.Chord) {
	if a.keys.ret.Matches(ch) {
		o.cancel(a)
	}
}

// change is the prompt's content changing: filterable contexts follow it.
func (o *searchOverlay) change(a *App, v string) {
	o.value = v
	if o.target.filterable {
		a.applyFilter(o.target.key, v)
	}
}

func (o *searchOverlay) confirm(a *App, v string) {
	o.value = v
	if v == "" {
		o.cancel(a)
		return
	}
	a.closeOverlay(o)
	c := a.sctx(o.target.key)
	if o.target.filterable {
		a.applyFilter(o.target.key, v)
		c.history.Push(v)
		return
	}
	c.search = v
	c.history.Push(v)
	c.cur = 0
	matches := a.searchMatches(o.target.key)
	y := a.searchCursorY(o.target.key)
	for i, m := range matches {
		if m.y > y || (m.y == y && m.x > 0) {
			c.cur = i
			break
		}
	}
	if len(matches) > 0 {
		a.selectMatch(o.target.key, matches, c.cur)
	}
}

func (o *searchOverlay) cancel(a *App) {
	a.closeOverlay(o)
	a.clearSearching(o.target.key)
}

// scrollHistory is lazygit's ScrollHistory: +1 goes to an older entry.
func (o *searchOverlay) scrollHistory(a *App, inc int) {
	h := a.sctx(o.target.key).history
	v, err := h.PeekAt(o.histIdx + inc)
	if err != nil {
		return
	}
	o.histIdx += inc
	o.change(a, v)
}

func (o *searchOverlay) view(a *App, c *ui.Context, p *Palette) {
	card(c, p, 520, func() {
		ui.Column(c).Padding(16, 18, 16, 18).Gap(10).Children(func() {
			ui.Text(c, strings.TrimSpace(o.prefix(a))).FontWeight(600)
			ui.TextInput(c.Key("search"), &o.value).AutoFocus().Label(strings.TrimSpace(o.prefix(a))).
				OnChange(func() { o.change(a, o.value) }).
				OnTextKey(func(k ui.TextKey) bool {
					switch {
					case k.Key == ui.KeyEnter && k.Mods == 0:
						o.confirm(a, k.Text())
						return true
					case k.Key == ui.KeyEscape:
						o.cancel(a)
						return true
					case k.Mods == 0 && a.keys.prev.Matches(keymap.Chord{Key: k.Key}):
						o.scrollHistory(a, 1)
						return true
					case k.Mods == 0 && a.keys.next.Matches(keymap.Chord{Key: k.Key}):
						o.scrollHistory(a, -1)
						return true
					case isClearKey(k):
						o.change(a, "")
						return true
					}
					return false
				})
		})
	})
}

// --- filtering -------------------------------------------------------------------

func (a *App) filterOf(key string) string {
	switch key {
	case "files":
		return a.files.GetFilter()
	case "commitFiles":
		if a.commitFiles != nil {
			return a.commitFiles.tree.GetFilter()
		}
		return ""
	}
	return a.sctx(key).filter
}

func (a *App) isFiltering(key string) bool { return a.filterOf(key) != "" }

// applyFilter is SearchHelper.ApplyFilter: select the first item, filter,
// redraw.
func (a *App) applyFilter(key, v string) {
	a.cancelRange(a.panelOf(key))
	fuzzy := a.repo.Config.Gui.UseFuzzySearch()
	switch key {
	case "files":
		a.files.SetFilter(v, fuzzy)
		a.cursor[panelFiles] = 0
		a.files.SetSelectedLineIdx(0)
	case "commitFiles":
		vm := a.commitFiles.tree
		vm.SetFilter(v, fuzzy)
		a.cursor[panelCommits] = 0
		vm.SetSelectedLineIdx(0)
	default:
		a.sctx(key).filter = v
		a.filterSnap()
		a.cursor[a.panelOf(key)] = 0
		a.lists[a.panelOf(key)].ScrollTo(0, ui.Start)
	}
	a.updateTreeRows()
	a.diffKey = ""
	a.loadMain()
}

func (a *App) panelOf(key string) panel {
	switch key {
	case "files", "worktrees", "submodules":
		return panelFiles
	case "localBranches", "remotes", "tags", "remoteBranches", "subCommits":
		return panelBranches
	case "commits", "commitFiles":
		return panelCommits
	case "stash":
		return panelStash
	}
	return a.focus
}

// clearSearching is SearchHelper.Cancel: forget the filter or search. The
// selected item stays selected (lazygit maps the filtered index back).
func (a *App) clearSearching(key string) {
	switch key {
	case "files":
		if a.files.IsFiltering() {
			a.files.SetSelectedLineIdx(a.cursor[panelFiles])
			a.files.ClearFilter()
			a.cursor[panelFiles] = a.files.GetSelectedLineIdx()
		}
	case "commitFiles":
		if vm := a.commitFiles.tree; vm.IsFiltering() {
			vm.SetSelectedLineIdx(a.cursor[panelCommits])
			vm.ClearFilter()
			a.cursor[panelCommits] = vm.GetSelectedLineIdx()
		}
	default:
		c := a.sctx(key)
		c.search = ""
		if c.filter != "" {
			p := a.panelOf(key)
			idx := a.unfilteredIndex(key, a.cursor[p])
			c.filter = ""
			a.filterSnap()
			a.cursor[p] = clamp(idx, 0, max(a.count(p)-1, 0))
			a.lists[p].ScrollIntoView(a.cursor[p])
		}
	}
	a.updateTreeRows()
	a.diffKey = ""
	a.loadMain()
}

// cancelSearching handles <esc> in a searched or filtered context before it
// leaves anything: searching is cancelled by gocui first, filtering by
// QuitActions.Escape after the range selection.
func (a *App) cancelSearching(filters bool) bool {
	t, ok := a.searchTarget()
	if !ok {
		return false
	}
	if t.filterable != filters {
		return false
	}
	if filters && !a.isFiltering(t.key) || !filters && a.sctx(t.key).search == "" {
		return false
	}
	a.clearSearching(t.key)
	return true
}

// unfilteredIndex maps a row of a filtered list back to the full list.
func (a *App) unfilteredIndex(key string, i int) int {
	idx := a.fidx[key]
	if idx == nil || i < 0 || i >= len(idx) {
		return i
	}
	return idx[i]
}

type fuzzySource[T any] struct {
	list   []T
	fields func(T) []string
}

func (s fuzzySource[T]) String(i int) string { return strings.Join(s.fields(s.list[i]), " ") }
func (s fuzzySource[T]) Len() int            { return len(s.list) }

// filterList is FilteredList.applyFilter. idx is nil when not filtering.
func filterList[T any](list []T, filter string, useFuzzy bool, fields func(T) []string) ([]T, []int) {
	if filter == "" {
		return list, nil
	}
	matches := utils.FindFrom(filter, fuzzySource[T]{list, fields}, useFuzzy)
	out := make([]T, len(matches))
	idx := make([]int, len(matches))
	for i, m := range matches {
		out[i], idx[i] = list[m.Index], m.Index
	}
	return out, idx
}

var _ fuzzy.Source = fuzzySource[string]{}

// filterSnap rebuilds what the panels show from the full snapshot and the
// filters, as ReApplyFilter does on every refresh.
func (a *App) filterSnap() {
	s := a.raw
	a.fidx = map[string][]int{}
	if s == nil {
		a.snap = nil
		return
	}
	fz := a.repo.Config.Gui.UseFuzzySearch()
	f := func(key string) string { return a.sctx(key).filter }
	cp := *s
	cp.Branches, a.fidx["localBranches"] = filterList(s.Branches, f("localBranches"), fz, func(b *models.Branch) []string { return []string{b.Name} })
	cp.Remotes, a.fidx["remotes"] = filterList(s.Remotes, f("remotes"), fz, func(r *models.Remote) []string { return []string{r.Name} })
	cp.Tags, a.fidx["tags"] = filterList(s.Tags, f("tags"), fz, func(t *models.Tag) []string { return []string{t.Name, t.Message} })
	cp.Worktrees, a.fidx["worktrees"] = filterList(s.Worktrees, f("worktrees"), fz, func(w *models.Worktree) []string { return []string{w.Name, w.Branch} })
	cp.Submodules, a.fidx["submodules"] = filterList(s.Submodules, f("submodules"), fz, func(m *models.SubmoduleConfig) []string { return []string{m.FullName()} })
	cp.Stash, a.fidx["stash"] = filterList(s.Stash, f("stash"), fz, func(e *models.StashEntry) []string { return []string{e.Name} })
	a.snap = &cp
	if b := a.refBrowse; b != nil && b.ref == nil {
		b.branches, a.fidx["remoteBranches"] = filterList(b.all, f("remoteBranches"), fz, func(r *models.RemoteBranch) []string { return []string{r.Name} })
	}
	for p := range panelCount {
		a.cursor[p] = clamp(a.cursor[p], 0, a.count(p)-1)
	}
}

// --- searching -------------------------------------------------------------------

type searchMatch struct{ y, x int }

// searchNormalize is modelSearchResults: a search with any uppercase letter
// is case sensitive.
func searchNormalize(s string) (string, func(string) string) {
	if utils.ContainsUppercase(s) {
		return s, func(s string) string { return s }
	}
	return strings.ToLower(s), strings.ToLower
}

func commitMatches(commits []*models.Commit, s string) []searchMatch {
	needle, norm := searchNormalize(s)
	var out []searchMatch
	for i, c := range commits {
		if h := strings.Index(norm(c.Hash()), needle); h >= 0 {
			out = append(out, searchMatch{i, h})
		} else if strings.Contains(norm(c.Name), needle) || strings.Contains(norm(c.ExtraInfo), needle) {
			out = append(out, searchMatch{i, 1})
		}
	}
	return out
}

// searchMatches are the current matches of context key, worked out from
// the model each time so a refresh can't leave them stale.
func (a *App) searchMatches(key string) []searchMatch {
	s := a.sctx(key).search
	if s == "" || a.snap == nil {
		return nil
	}
	switch key {
	case "commits":
		return commitMatches(a.snap.Commits, s)
	case "subCommits":
		if a.refBrowse == nil {
			return nil
		}
		return commitMatches(a.refBrowse.commits, s)
	case "staging", "stagingSecondary", "main":
		st := a.staging
		if st == nil {
			needle, norm := searchNormalize(s)
			var out []searchMatch
			for i, l := range a.diff {
				if x := strings.Index(norm(l.text), needle); x >= 0 {
					out = append(out, searchMatch{i, x})
				}
			}
			return out
		}
		needle, norm := searchNormalize(s)
		var out []searchMatch
		for i, l := range st.lines {
			if x := strings.Index(norm(l.Content), needle); x >= 0 {
				out = append(out, searchMatch{i, x})
			}
		}
		return out
	}
	return nil
}

func (a *App) searchCursorY(key string) int {
	switch key {
	case "commits":
		return a.cursor[panelCommits]
	case "subCommits":
		return a.cursor[panelBranches]
	case "staging", "stagingSecondary":
		if a.staging != nil {
			return a.staging.cursor
		}
	case "main":
		return a.mainCursorLine()
	}
	return 0
}

// stagingKey names the search context of the staging pane's side: lazygit's
// staging and stagingSecondary contexts.
func (a *App) stagingKey() string {
	if a.staging != nil && !a.staging.worktree {
		return "main"
	}
	if a.staging != nil && a.staging.staged {
		return "stagingSecondary"
	}
	return "staging"
}

// mainCursorLine follows the focused diff cursor, otherwise its viewport.
func (a *App) mainCursorLine() int {
	if a.staging != nil {
		return a.staging.cursor
	}
	first, _ := a.diffList.Visible()
	return first
}

func (a *App) selectMatch(key string, matches []searchMatch, i int) {
	c := a.sctx(key)
	c.cur = i
	y := matches[i].y
	c.syncedY = y
	switch key {
	case "commits", "subCommits":
		p := a.panelOf(key)
		a.cursor[p] = y
		a.lists[p].ScrollIntoView(y)
		a.loadMain()
	case "staging", "stagingSecondary", "main":
		if a.staging != nil {
			a.staging.resetMode()
			a.staging.cursor = y
			a.staging.list.ScrollIntoView(y)
		} else {
			a.diffList.ScrollTo(y, ui.Center)
		}
	}
}

// syncSearch is SetNearestSearchPosition: after the cursor moved by hand, the
// current match is the one nearest it.
func (a *App) syncSearch(key string, matches []searchMatch) *searchCtx {
	c := a.sctx(key)
	if len(matches) == 0 {
		return c
	}
	y := a.searchCursorY(key)
	if y != c.syncedY {
		last := 0
		for i, m := range matches {
			if m.y == y {
				last = i
				break
			}
			if m.y > y {
				break
			}
			last = i
		}
		c.cur, c.syncedY = last, y
	}
	c.cur = clamp(c.cur, 0, len(matches)-1)
	return c
}

// searchKey handles n / N while a searchable context is searching.
func (a *App) searchKey(ch keymap.Chord) bool {
	t, ok := a.searchTarget()
	if !ok || t.filterable || a.sctx(t.key).search == "" {
		return false
	}
	u := a.repo.Config.Keybinding.Universal
	next := keymap.Parse(u.NextMatch).Matches(ch)
	if !next && !keymap.Parse(u.PrevMatch).Matches(ch) {
		return false
	}
	matches := a.searchMatches(t.key)
	if len(matches) == 0 {
		return true
	}
	c := a.syncSearch(t.key, matches)
	y := a.searchCursorY(t.key)
	if next {
		if y < matches[c.cur].y {
			a.selectMatch(t.key, matches, c.cur)
			return true
		}
		a.selectMatch(t.key, matches, (c.cur+1)%len(matches))
		return true
	}
	if y > matches[c.cur].y {
		a.selectMatch(t.key, matches, c.cur)
		return true
	}
	a.selectMatch(t.key, matches, (c.cur+len(matches)-1)%len(matches))
	return true
}

// --- the search view -------------------------------------------------------------

// searchStatus is what lazygit's search view shows: the prompt text while
// it is open, else "matches for ..." for the focused context when it is
// searching or filtering. visible is false when the view is hidden.
func (a *App) searchStatus() (text string, visible bool) {
	if o, ok := a.topOverlay().(*searchOverlay); ok {
		return o.value, true
	}
	t, ok := a.searchTarget()
	if !ok {
		return "", false
	}
	tr := a.repo.Common.Tr
	ret := a.repo.Config.Keybinding.Universal.Return
	u := a.repo.Config.Keybinding.Universal
	if !t.filterable {
		s := a.sctx(t.key).search
		if s == "" {
			return "", false
		}
		matches := a.searchMatches(t.key)
		if len(matches) == 0 {
			return fmt.Sprintf(tr.NoMatchesFor, s, fmt.Sprintf(tr.ExitSearchMode, ret)), true
		}
		c := a.syncSearch(t.key, matches)
		return fmt.Sprintf(tr.MatchesFor, s, c.cur+1, len(matches),
			fmt.Sprintf(tr.SearchKeybindings, u.NextMatch, u.PrevMatch, ret)), true
	}
	if f := a.filterOf(t.key); f != "" {
		return fmt.Sprintf("matches for '%s' ", f) + fmt.Sprintf(tr.ExitTextFilterMode, ret), true
	}
	return "", false
}

// searchBar draws the search view above the hint bar.
func (a *App) searchBar(c *ui.Context, p *Palette) {
	text, ok := a.searchStatus()
	if !ok || a.topOverlay() != nil {
		return
	}
	prefix := a.repo.Common.Tr.SearchPrefix
	if t, ok := a.searchTarget(); ok && t.filterable {
		prefix = a.repo.Common.Tr.FilterPrefix
	}
	ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
	ui.Row(c).Height(hintBarHeight).PaddingX(14).Gap(6).AlignItems(ui.Center).Background(p.Panel).ClipX().Children(func() {
		ui.Text(c, prefix).FontSize(12).TextColor(p.Muted)
		ui.Text(c, text).FontSize(12).TextColor(p.Accent).SingleLine().Shrink(1)
	})
}

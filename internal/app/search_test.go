package app

import (
	"strings"
	"testing"

	"laziergit/internal/testrepo"
)

// typeSearch opens the prompt, types text and confirms.
func typeSearch(t *testing.T, a *App, text string) {
	t.Helper()
	a.Press("/")
	if _, ok := a.topOverlay().(*searchOverlay); !ok {
		t.Fatalf("no search prompt, top overlay %T", a.topOverlay())
	}
	o := a.topOverlay().(*searchOverlay)
	o.change(a, text)
	o.confirm(a, text)
}

func TestFileFilterAndEscapeKeepsSelection(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "apple-one", "1")
	testrepo.Write(t, dir, "apple-two", "2")
	a.Refresh()
	tt.Frame()
	typeSearch(t, a, "apple")
	if !a.files.IsFiltering() {
		t.Fatal("not filtering")
	}
	if text, vis := a.searchStatus(); !vis || !strings.Contains(text, "matches for 'apple'") {
		t.Fatalf("status %q %v", text, vis)
	}
	selectFile(t, a, "apple-two")
	a.Press("<esc>")
	if a.files.IsFiltering() || selectedPath(a) != "apple-two" {
		t.Fatalf("after esc: filtering %v selected %q", a.files.IsFiltering(), selectedPath(a))
	}
	if _, vis := a.searchStatus(); vis {
		t.Fatal("status still visible")
	}
}

func TestFilterStageAllOnlyTouchesVisibleFiles(t *testing.T) {
	a, tt, dir := newTestApp(t)
	testrepo.Write(t, dir, "apple", "a")
	testrepo.Write(t, dir, "orange", "o")
	a.Refresh()
	tt.Frame()
	typeSearch(t, a, "apple")
	a.Press("a")
	staged := git(t, dir, "diff", "--cached", "--name-only")
	if !strings.Contains(staged, "apple") || strings.Contains(staged, "orange") {
		t.Fatalf("staged %q", staged)
	}
}

func TestBranchFilterThenEscapeSelectsSameBranch(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "branch", "alpha-x")
	git(t, dir, "branch", "beta-y")
	a, tt := openApp(t, dir)
	press(tt, "3")
	typeSearch(t, a, "beta")
	if n := a.count(panelBranches); n != 1 || a.selectedBranch().Name != "beta-y" {
		t.Fatalf("filtered rows %d, selected %v", n, a.selectedBranch())
	}
	if full := len(a.raw.Branches); full <= 1 {
		t.Fatalf("full list shrank to %d", full)
	}
	a.Press("<esc>")
	if b := a.selectedBranch(); b == nil || b.Name != "beta-y" {
		t.Fatalf("selection after clearing filter: %v", b)
	}
	if a.count(panelBranches) != len(a.raw.Branches) {
		t.Fatal("filter not cleared")
	}
}

func TestFilterIsReappliedWhenModelChanges(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "branch", "keep-a")
	a, tt := openApp(t, dir)
	press(tt, "3")
	typeSearch(t, a, "keep")
	git(t, dir, "branch", "keep-b")
	git(t, dir, "branch", "other")
	a.Refresh()
	tt.Frame()
	if n := a.count(panelBranches); n != 2 {
		t.Fatalf("rows %d, want the two matching branches", n)
	}
}

func TestCommitSearchNextPrevious(t *testing.T) {
	dir := cleanRepo(t)
	for _, m := range []string{"needle a", "plain", "needle b", "needle c", "plain too"} {
		git(t, dir, "commit", "--allow-empty", "-qm", m)
	}
	a, tt := openApp(t, dir)
	press(tt, "4")
	status := func() string { s, _ := a.searchStatus(); return s }
	at := func() string { return a.selectedCommit().Name }
	typeSearch(t, a, "needle")
	// Newest first: plain too, needle c, needle b, plain, needle a.
	if at() != "needle c" || !strings.Contains(status(), "(1 of 3)") {
		t.Fatalf("first match: %q %q", at(), status())
	}
	a.Press("n")
	if at() != "needle b" || !strings.Contains(status(), "(2 of 3)") {
		t.Fatalf("n: %q %q", at(), status())
	}
	a.Press("n")
	a.Press("n")
	if at() != "needle c" || !strings.Contains(status(), "(1 of 3)") {
		t.Fatalf("n wraps: %q %q", at(), status())
	}
	a.Press("N")
	if at() != "needle a" || !strings.Contains(status(), "(3 of 3)") {
		t.Fatalf("N wraps: %q %q", at(), status())
	}
	// Moving by hand re-aligns the status with the nearest match, and n
	// then goes to the match after the cursor.
	a.cursor[panelCommits] = 3 // plain
	if !strings.Contains(status(), "(2 of 3)") {
		t.Fatalf("status after moving: %q", status())
	}
	a.Press("n")
	if at() != "needle a" {
		t.Fatalf("n from between matches: %q", at())
	}
	// <esc> ends the search.
	a.Press("<esc>")
	if _, vis := a.searchStatus(); vis {
		t.Fatal("esc left the search status up")
	}
	// A new search starts from the cursor, not from the last match; a match
	// on the cursor's own line counts.
	a.cursor[panelCommits] = 3
	typeSearch(t, a, "needle")
	if at() != "needle a" {
		t.Fatalf("new search from the cursor: %q", at())
	}
}

func TestSearchWithoutMatchesSaysSo(t *testing.T) {
	dir := cleanRepo(t)
	a, tt := openApp(t, dir)
	press(tt, "4")
	typeSearch(t, a, "zzzz-no-such")
	if s, vis := a.searchStatus(); !vis || !strings.Contains(s, "No matches for 'zzzz-no-such'") {
		t.Fatalf("status %q %v", s, vis)
	}
}

func TestSearchHistoryIsPerPanelAndScrollsWithArrows(t *testing.T) {
	a, tt, _ := newTestApp(t)
	for _, s := range []string{"1", "2", "3"} {
		typeSearch(t, a, s)
	}
	a.Press("/")
	o := a.topOverlay().(*searchOverlay)
	o.scrollHistory(a, 1)
	o.scrollHistory(a, 1)
	if o.value != "2" {
		t.Fatalf("history gave %q", o.value)
	}
	o.scrollHistory(a, -1)
	o.scrollHistory(a, -1)
	if o.value != "" {
		t.Fatalf("scrolling back past the newest: %q", o.value)
	}
	a.Press("<esc>")
	press(tt, "4")
	a.Press("/")
	o = a.topOverlay().(*searchOverlay)
	o.scrollHistory(a, 1)
	if o.value != "" {
		t.Fatalf("commits inherited files' history: %q", o.value)
	}
}

func TestStagingSearchSelectsMatchingLine(t *testing.T) {
	dir := cleanRepo(t)
	commitFile(t, dir, "f.txt", "", "add f")
	testrepo.Write(t, dir, "f.txt", "NEEDLE a\ntwo\nNEEDLE b\nfour\nNEEDLE c\n")
	a, tt := openApp(t, dir)
	press(tt, "0")
	if a.staging == nil {
		t.Fatal("not in staging")
	}
	typeSearch(t, a, "NEEDLE")
	line := func() string { return a.staging.lines[a.staging.cursor].Content }
	if !strings.Contains(line(), "NEEDLE a") {
		t.Fatalf("landed on %q", line())
	}
	a.Press("n")
	a.Press("n")
	if !strings.Contains(line(), "NEEDLE c") {
		t.Fatalf("n n landed on %q", line())
	}
	a.Press("n")
	if !strings.Contains(line(), "NEEDLE a") {
		t.Fatalf("n wraps to %q", line())
	}
}

func TestShellCommandRunsAndRefreshes(t *testing.T) {
	dir := cleanRepo(t)
	a, tt := openApp(t, dir)
	press(tt, ":")
	o, ok := a.topOverlay().(*promptOverlay)
	if !ok || o.title != a.repo.Common.Tr.ShellCommand {
		t.Fatalf("overlay %T", a.topOverlay())
	}
	o.submit(a, "git branch from-shell")
	tt.Frame()
	found := false
	for _, b := range a.snap.Branches {
		found = found || b.Name == "from-shell"
	}
	if !found {
		t.Fatal("branch not created / not refreshed")
	}
}

func TestDiffContextSizeKeys(t *testing.T) {
	a, tt, _ := newTestApp(t)
	before := a.repo.Config.Git.DiffContextSize
	press(tt, "}")
	if a.repo.Config.Git.DiffContextSize != before+1 || !strings.Contains(a.status, "Changed diff context size to") {
		t.Fatalf("size %d status %q", a.repo.Config.Git.DiffContextSize, a.status)
	}
	press(tt, "{")
	if a.repo.Config.Git.DiffContextSize != before {
		t.Fatalf("size %d", a.repo.Config.Git.DiffContextSize)
	}
}

func TestSearchBarShot(t *testing.T) {
	out := shotDir()
	if out == "" {
		t.Skip("SHOT_DIR not set")
	}
	dir := cleanRepo(t)
	git(t, dir, "branch", "feature-apple")
	_, tt := openApp(t, dir)
	press(tt, "3")
	press(tt, "/")
	tt.Type("feat")
	tt.Frame()
	shot(t, tt, out+"/search-prompt.png")
	press(tt, "enter")
	shot(t, tt, out+"/search-filtered.png")
}

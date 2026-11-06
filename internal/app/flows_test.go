package app

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"laziergit/internal/testrepo"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// pressUntil presses key until cond holds, failing after a bound so a
// broken keymap can't hang the suite.
func pressUntil(t *testing.T, tt *ui.Tester, key string, cond func() bool) {
	t.Helper()
	for i := 0; !cond(); i++ {
		if i > 50 {
			t.Fatalf("condition not reached after 50 × %q", key)
		}
		press(tt, key)
	}
}

// press sends a key the way a keyboard does: printable keys as text.
func press(tt *ui.Tester, s string) {
	switch s {
	case "enter":
		tt.Key(0, ui.KeyEnter)
	case "esc":
		tt.Key(0, ui.KeyEscape)
	case "tab":
		tt.Key(0, ui.KeyTab)
	case "space":
		tt.TypeKey(0, ui.KeySpace, " ")
	default:
		tt.TypeKey(0, ui.KeyUnknown, s)
	}
	tt.Frame()
}

func TestCommitFlow(t *testing.T) {
	a, tt, dir := newTestApp(t)
	press(tt, "space") // stage a.go
	press(tt, "c")
	if _, ok := a.topOverlay().(*commitOverlay); !ok {
		t.Fatalf("commit editor not open: %T", a.topOverlay())
	}
	tt.Type("Add return")
	press(tt, "tab")
	tt.Type("Body text")
	tt.Frame()
	if a.draft.summary != "Add return" || a.draft.description != "Body text" {
		t.Fatalf("draft %+v", a.draft)
	}
	press(tt, "tab")
	press(tt, "enter")
	if msg := git(t, dir, "log", "-1", "--format=%B"); !strings.HasPrefix(msg, "Add return\n\nBody text") {
		t.Fatalf("log: %q", msg)
	}
	if a.topOverlay() != nil || a.draft.summary != "" {
		t.Fatalf("editor not closed/cleared: %T %+v", a.topOverlay(), a.draft)
	}
}

func TestCommitWithNothingStagedOffersStageAll(t *testing.T) {
	a, tt, dir := newTestApp(t)
	press(tt, "c")
	if _, ok := a.topOverlay().(*confirmOverlay); !ok {
		t.Fatalf("expected confirm, got %T", a.topOverlay())
	}
	press(tt, "enter")
	if _, ok := a.topOverlay().(*commitOverlay); !ok {
		t.Fatalf("expected commit editor, got %T", a.topOverlay())
	}
	tt.Type("Everything")
	press(tt, "enter")
	if st := git(t, dir, "status", "--porcelain"); st != "" {
		t.Fatalf("tree not clean: %q", st)
	}
}

func TestStagingSingleLine(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() {}\n\nfunc B() {}\n\nfunc C() {}\n")
	a, tt := openApp(t, dir)
	selectFile(t, a, "a.go")
	press(tt, "enter")
	s := a.staging
	if s == nil || s.staged {
		t.Fatalf("staging %+v", s)
	}
	// Hunk mode is lazygit's default; a goes line by line. The cursor starts on
	// the first change: +(blank) then +func B; walk to func C.
	press(tt, "a")
	pressUntil(t, tt, "j", func() bool { return strings.HasPrefix(s.lines[s.cursor].Content, "+func C") })
	press(tt, "space")
	cached := git(t, dir, "diff", "--cached")
	if !strings.Contains(cached, "+func C() {}") || strings.Contains(cached, "+func B() {}") {
		t.Fatalf("cached:\n%s", cached)
	}
	if a.staging == nil || a.staging.staged {
		t.Fatalf("should stay on unstaged side with changes left: %+v", a.staging)
	}
	// Switch to staged side and unstage the whole hunk.
	press(tt, "tab")
	if !a.staging.staged {
		t.Fatal("tab did not switch sides")
	}
	press(tt, "space") // the staged side starts in hunk mode

	if cached := git(t, dir, "diff", "--cached"); cached != "" {
		t.Fatalf("still staged:\n%s", cached)
	}
	press(tt, "esc")
	if a.staging != nil {
		t.Fatal("esc did not leave staging")
	}
}

func TestStagingRangeSelect(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\nfunc D() {}\n")
	a, tt := openApp(t, dir)
	selectFile(t, a, "a.go")
	press(tt, "enter")
	s := a.staging
	pressUntil(t, tt, "j", func() bool { return strings.HasPrefix(s.lines[s.cursor].Content, "+func B") })
	press(tt, "v")
	press(tt, "j")
	press(tt, "space")
	cached := git(t, dir, "diff", "--cached")
	if !strings.Contains(cached, "+func B") || !strings.Contains(cached, "+func C") || strings.Contains(cached, "+func D") {
		t.Fatalf("cached:\n%s", cached)
	}
}

func TestKeybindingsMenuFilterAndRun(t *testing.T) {
	a, tt, _ := newTestApp(t)
	press(tt, "?")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || !tt.HasText("Keybindings") || !tt.HasText("FILES") {
		t.Fatalf("menu not open: %T %q", a.topOverlay(), tt.Texts())
	}
	// The menu filters as you type, like lazygit's.
	for _, r := range "stash" {
		press(tt, string(r))
	}
	got := m.visible()
	idx := slices.IndexFunc(got, func(it menuItem) bool { return it.label == "Stash" })
	if idx < 0 || len(got) >= len(a.actions) {
		t.Fatalf("filtered: %+v", got)
	}
	for range idx {
		press(tt, "down")
	}
	press(tt, "enter")
	if _, ok := a.topOverlay().(*promptOverlay); !ok {
		t.Fatalf("stash prompt not open: %T", a.topOverlay())
	}
	press(tt, "esc")
	if a.topOverlay() != nil {
		t.Fatal("esc did not close prompt")
	}
}

func TestPushSetsUpstreamThenPull(t *testing.T) {
	dir := testrepo.New(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, dir, "init", "-q", "--bare", remote)
	git(t, dir, "remote", "add", "origin", remote)
	a, tt := openApp(t, dir)
	press(tt, "P")
	po, ok := a.topOverlay().(*promptOverlay)
	if !ok || po.value != "origin main" {
		t.Fatalf("upstream prompt: %T %+v", a.topOverlay(), po)
	}
	press(tt, "enter")
	if !tt.HasText("Pushed") {
		t.Fatalf("no push confirmation: %q", tt.Texts())
	}
	if a.snap.Head.UpstreamBranch != "main" {
		t.Fatalf("upstream %+v", a.snap.Head)
	}
	press(tt, "p")
	if !tt.HasText("Pulled") {
		t.Fatalf("no pull confirmation: %q (log %v)", tt.Texts(), a.log)
	}
	if len(a.log) == 0 {
		t.Fatal("command log empty")
	}
}

func TestBranchesNewCheckoutDelete(t *testing.T) {
	a, tt, dir := newTestApp(t)
	press(tt, "3")
	press(tt, "n")
	tt.Type("topic one")
	press(tt, "enter")
	if a.snap.Head.Name != "topic-one" {
		t.Fatalf("head %s", a.snap.Head.Name)
	}
	// Select main and check it out.
	pressUntil(t, tt, "j", func() bool { return a.selectedBranch().Name == "main" })
	press(tt, "space")
	if a.snap.Head.Name != "main" {
		t.Fatalf("head %s", a.snap.Head.Name)
	}
	pressUntil(t, tt, "j", func() bool { return a.selectedBranch().Name == "topic-one" })
	press(tt, "d")
	press(tt, "enter")
	if strings.Contains(git(t, dir, "branch"), "topic-one") {
		t.Fatal("branch not deleted")
	}
}

func TestCommitFilesDrillDown(t *testing.T) {
	a, tt, _ := newTestApp(t)
	press(tt, "4")
	press(tt, "enter")
	if a.commitFiles == nil || a.count(panelCommits) != 1 || !tt.HasText("COMMIT FILES") {
		t.Fatalf("commit files: %+v %q", a.commitFiles, tt.Texts())
	}
	press(tt, "esc")
	if a.commitFiles != nil || a.count(panelCommits) != 2 {
		t.Fatal("esc did not return to commits")
	}
}

func TestDiscardAndStash(t *testing.T) {
	a, tt, dir := newTestApp(t)
	pressUntil(t, tt, "j", func() bool { return selectedPath(a) == "new.txt" })
	press(tt, "d")
	press(tt, "x")
	if strings.Contains(git(t, dir, "status", "--porcelain"), "new.txt") {
		t.Fatal("new.txt not discarded")
	}
	press(tt, "s")
	tt.Type("wip")
	press(tt, "enter")
	if len(a.snap.Stash) != 2 || len(a.snap.Files) != 0 {
		t.Fatalf("stash=%d files=%d", len(a.snap.Stash), len(a.snap.Files))
	}
	press(tt, "5")
	press(tt, "g")
	press(tt, "enter") // lazygit's "Stash pop" confirmation
	if len(a.snap.Stash) != 1 || len(a.snap.Files) != 1 {
		t.Fatalf("after pop stash=%d files=%d", len(a.snap.Stash), len(a.snap.Files))
	}
}

func TestErrorOverlayShowsGitOutput(t *testing.T) {
	a, tt, dir := newTestApp(t)
	git(t, dir, "remote", "add", "broken", "/does/not/exist")
	git(t, dir, "config", "branch.main.remote", "broken")
	git(t, dir, "config", "branch.main.merge", "refs/heads/main")
	a.Refresh()
	press(tt, "p")
	eo, ok := a.topOverlay().(*errorOverlay)
	if !ok || !strings.Contains(eo.body, "/does/not/exist") {
		t.Fatalf("error overlay: %T %+v", a.topOverlay(), eo)
	}
	press(tt, "esc")
	if a.topOverlay() != nil {
		t.Fatal("error not dismissed")
	}
}

func TestFocusedWorkingTreeDiffStagesRange(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\n")
	a, tt := openApp(t, dir)
	selectFile(t, a, "a.go")
	press(tt, "0")
	s := a.staging
	if s == nil || s.staged || !s.isChange(s.cursor) {
		t.Fatalf("focused diff: %+v", s)
	}
	press(tt, "a") // leave default hunk mode for line selection
	pressUntil(t, tt, "j", func() bool { return strings.HasPrefix(s.lines[s.cursor].Content, "+func B") })
	press(tt, "v")
	press(tt, "j")
	press(tt, "space")
	cached := git(t, dir, "diff", "--cached")
	if !strings.Contains(cached, "+func B") || !strings.Contains(cached, "+func C") || strings.Contains(cached, "+func A") {
		t.Fatalf("cached diff:\n%s", cached)
	}
}

func TestFocusedDiffNavigatesChangeBlocks(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "navigation.txt", "one\ntwo\nseparator\nthree\nfour\n")
	git(t, dir, "add", "navigation.txt")
	git(t, dir, "commit", "-m", "navigation fixture")
	testrepo.Write(t, dir, "navigation.txt", "ONE\nTWO\nseparator\nTHREE\nFOUR\n")
	a, tt := openApp(t, dir)
	selectFile(t, a, "navigation.txt")
	press(tt, "0")
	selected := func() string { return a.staging.lines[a.staging.cursor].Content }
	press(tt, "a") // line by line
	press(tt, "l")
	if got := selected(); got != "-three" {
		t.Fatalf("next block: %q", got)
	}
	press(tt, "h")
	if got := selected(); got != "-one" {
		t.Fatalf("previous block should start at its first line: %q", got)
	}
	pressUntil(t, tt, "j", func() bool { return selected() == " separator" })
	press(tt, "l")
	if got := selected(); got != "-three" {
		t.Fatalf("next block from context: %q", got)
	}
}

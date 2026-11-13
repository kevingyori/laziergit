package app

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"laziergit/internal/repo"
	"laziergit/internal/testrepo"
)

// loadParts is one complete load of dir, part by part, as emitted.
func loadParts(t *testing.T, r *repo.Repo) []repo.Part {
	t.Helper()
	var parts []repo.Part
	r.LoadParts(func(p repo.Part) { parts = append(parts, p) })
	return parts
}

// unloadedApp is an app whose first load hasn't delivered anything yet.
func unloadedApp(t *testing.T, dir string) (*App, *ui.Tester, *repo.Repo) {
	t.Helper()
	r, err := repo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	sync := func(fn func()) { fn() }
	a := New(r, sync, sync, func() {})
	tt := ui.NewTester(a.View, 1280, 800)
	tt.Frame()
	return a, tt, r
}

func partOf(parts []repo.Part, kind repo.PartKind) repo.Part {
	i := slices.IndexFunc(parts, func(p repo.Part) bool { return p.Kind == kind })
	return parts[i]
}

// emptyStates are what an empty panel says; a skeleton never says them.
var emptyStates = []string{"Working tree clean", "No commits yet", "No stash entries", "No branches", "No changed files"}

func TestSkeletonBeforePartsArrive(t *testing.T) {
	a, tt, r := unloadedApp(t, testrepo.New(t))
	for v := range sideViewCount {
		if a.viewLoaded(v) {
			t.Errorf("%s loaded before any part", sideViewNames[v])
		}
	}
	if !a.mainPending() {
		t.Error("main view not pending before any part")
	}
	for _, s := range append(emptyStates, "a.go", "second") {
		if tt.HasText(s) {
			t.Errorf("%q drawn before its data arrived", s)
		}
	}
	if !tt.HasText("FILES") || !tt.HasText("COMMITS") {
		t.Fatalf("panel titles missing during load: %q", tt.Texts())
	}

	// Files land alone: they draw at once, the rest stays skeleton.
	parts := loadParts(t, r)
	a.applyPart(partOf(parts, repo.StatePart))
	a.applyPart(partOf(parts, repo.FilesPart))
	tt.Frame()
	if !tt.HasText("new.txt") {
		t.Fatalf("files not drawn on arrival: %q", tt.Texts())
	}
	if tt.HasText("second") || tt.HasText("No commits yet") || a.viewLoaded(vCommits) {
		t.Fatal("commits drawn before their part")
	}
	// The status line names the branch before the branch list loads.
	a.applyPart(partOf(parts, repo.HeadPart))
	tt.Frame()
	if !a.viewLoaded(vStatus) || !tt.HasText("main") {
		t.Fatalf("status line waits for the branch list: %q", tt.Texts())
	}
	for _, p := range parts {
		a.applyPart(p)
	}
	tt.Frame()
	for v := range sideViewCount {
		if !a.viewLoaded(v) {
			t.Errorf("%s still a skeleton after the load", sideViewNames[v])
		}
	}
	if a.mainPending() {
		t.Error("main view still pending after the load")
	}
	if !tt.HasText("second") || !tt.HasText("feature") {
		t.Fatalf("loaded data missing: %q", tt.Texts())
	}
}

func TestNoSkeletonOnLaterRefresh(t *testing.T) {
	a, tt, _ := newTestApp(t)
	var jobs []func()
	a.bg = func(f func()) { jobs = append(jobs, f) }
	parts := make(chan repo.Part, 1)
	a.RefreshFrom(func() <-chan repo.Part { return parts }, nil)
	go jobs[len(jobs)-1]()
	tt.Frame()
	for v := range sideViewCount {
		if !a.viewLoaded(v) {
			t.Errorf("%s turned into a skeleton on refresh", sideViewNames[v])
		}
	}
	if a.mainPending() || !tt.HasText("new.txt") || !tt.HasText("second") {
		t.Fatalf("refresh hid loaded data: %q", tt.Texts())
	}
	close(parts)
}

func TestSkeletonOnRepositorySwitch(t *testing.T) {
	other := testrepo.New(t)
	if err := os.WriteFile(filepath.Join(other, "other.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, tt := openApp(t, testrepo.New(t))
	t.Cleanup(a.Close)
	var pending []func()
	a.post = func(fn func()) { pending = append(pending, fn) }
	var loads []func()
	a.openRepository(other, false)
	// The switch itself lands; the new repository's load is held back.
	a.bg = func(fn func()) { loads = append(loads, fn) }
	for _, fn := range pending {
		fn()
	}
	pending = nil
	tt.Frame()
	if a.viewLoaded(vFiles) || a.viewLoaded(vCommits) || tt.HasText("new.txt") || tt.HasText("second") {
		t.Fatalf("switching repositories kept the old data instead of skeletons: %q", tt.Texts())
	}
	a.bg = func(fn func()) { fn() }
	for _, fn := range loads {
		fn()
	}
	for len(pending) > 0 {
		batch := pending
		pending = nil
		for _, fn := range batch {
			fn()
		}
	}
	tt.Frame()
	if !a.viewLoaded(vFiles) || !tt.HasText("other.txt") {
		t.Fatalf("new repository did not load: %q", tt.Texts())
	}
}

// Parts land in whatever order git finishes them; the window ends up the
// same as after a load in emitted order.
func TestPartsInAnyOrderDrawTheSame(t *testing.T) {
	dir := testrepo.New(t)
	want, wantT, r := unloadedApp(t, dir)
	parts := loadParts(t, r)
	for _, p := range parts {
		want.applyPart(p)
	}
	wantT.Frame()
	for seed := range 5 {
		a, tt, _ := unloadedApp(t, dir)
		order := slices.Clone(parts)
		rand.New(rand.NewPCG(uint64(seed), 7)).Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		for _, p := range order {
			a.applyPart(p)
		}
		tt.Frame()
		if !slices.Equal(tt.Texts(), wantT.Texts()) {
			t.Fatalf("seed %d drew\n%q\nwant\n%q", seed, tt.Texts(), wantT.Texts())
		}
		if a.cursor != want.cursor || a.files.Len() != want.files.Len() {
			t.Fatalf("seed %d: cursors %v, want %v", seed, a.cursor, want.cursor)
		}
	}
}

func TestKeysDuringSkeleton(t *testing.T) {
	a, tt, r := unloadedApp(t, testrepo.New(t))
	before := r.Load()
	// Every key of every panel, as a user mashing keys while it loads
	// (not R: refreshing loads the data, here at once).
	for _, keys := range [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"0"}} {
		a.Press(keys...)
		for _, k := range []string{"<space>", "<enter>", "j", "k", "d", "D", "c", "a", "s", "S", "P", "p", "f", "n", "r", "g", "M", "z", "Z", "e", "o", "i", "w", "]", "[", "<tab>", "<esc>", "/", "<esc>"} {
			a.Press(k)
			tt.Frame()
			if len(a.overlays) > 0 {
				t.Fatalf("%s opened %T during the load", k, a.overlays[0])
			}
		}
	}
	if a.busy != "" {
		t.Fatalf("an operation started during the load: %q", a.busy)
	}
	after := r.Load()
	if len(after.Files) != len(before.Files) || len(after.Stash) != len(before.Stash) || len(after.Commits) != len(before.Commits) ||
		after.Head.Name != before.Head.Name {
		t.Fatal("keys during the load changed the repository")
	}
	// Once the data is there the same keys act again.
	for _, p := range loadParts(t, r) {
		a.applyPart(p)
	}
	a.Press("2")
	a.fileTab = 0 // the tab keys above moved through the files window's tabs
	selectFile(t, a, "new.txt")
	a.Press("<space>")
	if f := a.selectedFile(); f == nil || !f.HasStagedChanges {
		t.Fatal("staging after the load did nothing")
	}
}

// Before a repository is open the window draws from the config alone, and
// what it draws is what the real window draws until its first part lands:
// swapping one for the other moves nothing.
func TestPlaceholderMatchesUnloadedWindow(t *testing.T) {
	for _, args := range [][]string{nil, {"log"}, {"--screen-mode", "half"}} {
		o, err := ParseCLI(args)
		if err != nil {
			t.Fatal(err)
		}
		dir := testrepo.New(t)
		ph, err := repo.Placeholder()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(ph.Close)
		p := NewPlaceholder(ph, o, func() {})
		pt := ui.NewTester(p.View, 1280, 800)
		pt.Frame()
		r, err := o.Open(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(r.Close)
		sync := func(fn func()) { fn() }
		a := NewWithCLI(r, o, sync, sync, func() {})
		at := ui.NewTester(a.View, 1280, 800)
		at.Frame()
		if !slices.Equal(pt.Texts(), at.Texts()) {
			t.Fatalf("%v: placeholder drew\n%q\nwindow drew\n%q", args, pt.Texts(), at.Texts())
		}
		if p.focus != a.focus || p.screenMode != a.screenMode {
			t.Fatalf("%v: placeholder focus %v mode %v, window %v %v", args, p.focus, p.screenMode, a.focus, a.screenMode)
		}
		// Keys before the repository opens do nothing, and don't crash.
		p.Press("2", "<space>", "c", "P", "z", "4", "<enter>", "]", "?", "<esc>", "q")
		pt.Frame()
	}
}

package app

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"laziergit/internal/repo"
)

// TestPerfLargeRepo measures snapshot load and frame cost against a real
// repository. Run with PERF_REPO=/path/to/repo.
func TestPerfLargeRepo(t *testing.T) {
	dir := os.Getenv("PERF_REPO")
	if dir == "" {
		t.Skip("PERF_REPO not set")
	}
	t.Setenv("CONFIG_DIR", t.TempDir())
	start := time.Now()
	r, err := repo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	t.Logf("open: %v", time.Since(start))

	for i := range 3 {
		start = time.Now()
		s := r.Load()
		t.Logf("load #%d: %v (files=%d branches=%d commits=%d stash=%d errs=%v)", i, time.Since(start),
			len(s.Files), len(s.Branches), len(s.Commits), len(s.Stash), s.Errors)
	}

	sync := func(fn func()) { fn() }
	a := New(r, sync, sync, func() {})
	a.Refresh()
	tt := ui.NewTester(a.View, 1280, 800)
	tt.Frame()

	// A huge synthetic diff: 50k lines.
	var b strings.Builder
	b.WriteString("diff --git a/big.go b/big.go\n@@ -1,50000 +1,50000 @@\n")
	for i := range 50000 {
		switch i % 3 {
		case 0:
			fmt.Fprintf(&b, "+\tadded line %d with some code := call(arg, %d)\n", i, i)
		case 1:
			fmt.Fprintf(&b, "-\tremoved line %d\n", i)
		default:
			fmt.Fprintf(&b, " \tcontext line %d\n", i)
		}
	}
	start = time.Now()
	a.diff = parseDiff(b.String())
	t.Logf("parse 50k-line diff: %v", time.Since(start))

	frames := func(label string, n int, step func(i int)) {
		start := time.Now()
		for i := range n {
			step(i)
			tt.Frame()
		}
		t.Logf("%s: %v/frame (headless, incl. software raster)", label, time.Since(start)/time.Duration(n))
	}
	frames("idle redraw", 30, func(int) {})
	frames("scroll 50k diff", 60, func(i int) { a.diffList.ScrollTo(i*800, ui.Start) })
	tt.TypeKey(0, ui.Key4, "4")
	a.diffKey = "pinned"
	frames("move commit cursor (j)", 60, func(int) { a.cursor[panelCommits]++; a.lists[panelCommits].ScrollIntoView(a.cursor[panelCommits]) })
}

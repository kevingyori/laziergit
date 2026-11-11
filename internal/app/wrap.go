package app

import (
	"strings"

	"sync"

	"github.com/jesseduffield/lazygit/pkg/gocui"
)

// PaneWraps reports whether pane name ("main" or "secondary") wraps its
// lines, as lazygit's main views do: content that is no diff (a branch's
// log, the status, an explanation) always, a diff only with
// gui.wrapLinesInDiffView. A renderer's output is never wrapped here.
func (a *App) PaneWraps(name string) bool {
	wrapDiff := a.repo.Config.Gui.WrapLinesInDiffView
	if st := a.paneState(name); st != nil {
		return wrapDiff && st.ansi == nil
	}
	if name == "secondary" {
		return wrapDiff && !a.secondaryRendered
	}
	if a.diffRendered {
		return false
	}
	key := a.diffKey
	if isDiffSource(key) || strings.HasPrefix(key, "file:") || strings.HasPrefix(key, "diffing:") {
		return wrapDiff
	}
	return true
}

// wrapViewLine splits a line into the view lines gocui wraps it into at
// width columns: gocui's own wrapping (word boundaries, long words split,
// tabs expanded, display width), cached as rows are drawn every frame.
func wrapViewLine(line string, width int) []string {
	if width <= 0 {
		return []string{line}
	}
	k := wrapKey{line, width}
	wrapMu.Lock()
	defer wrapMu.Unlock()
	if w, ok := wrapCache[k]; ok {
		return w
	}
	if len(wrapCache) > 20000 {
		clear(wrapCache)
	}
	v := gocui.NewView("wrap", 0, 0, width+1, 2, gocui.OutputTrue)
	v.Wrap = true
	v.SetContent(line)
	w := v.ViewBufferLines()
	if len(w) == 0 {
		w = []string{""}
	}
	wrapCache[k] = w
	return w
}

type wrapKey struct {
	line  string
	width int
}

var (
	wrapMu    sync.Mutex
	wrapCache = map[wrapKey][]string{}
)

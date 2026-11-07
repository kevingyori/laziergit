package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/gocui"
)

// The native panes must wrap exactly as gocui does (the parity harness
// projects them through gocui).
func TestWrapViewLineMatchesGocui(t *testing.T) {
	inputs := []string{
		"",
		"short",
		strings.Repeat("word ", 40),
		"+CHANGED " + strings.Repeat("word ", 40),
		strings.Repeat("x", 130),
		"a\tb\tc\t" + strings.Repeat("tab\t", 20),
		"日本語のテキスト " + strings.Repeat("幅広い文字 ", 15),
		"commit 0123456789abcdef  " + strings.Repeat("long-hyphenated-words ", 8),
	}
	for _, width := range []int{1, 7, 20, 47, 80} {
		for _, in := range inputs {
			v := gocui.NewView("v", 0, 0, width+1, 500, gocui.OutputTrue)
			v.Wrap = true
			v.SetContent(in)
			want := v.ViewBufferLines()
			got := wrapViewLine(in, width)
			if !slices.Equal(got, want) {
				t.Errorf("width %d, %q:\n got %q\nwant %q", width, in, got, want)
			}
		}
	}
}

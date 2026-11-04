package app

import "testing"

func TestParseCommitShow(t *testing.T) {
	in := "commit abc (HEAD -> main)\nAuthor: A <a@a>\nDate:   now\n\n    Subject line\n\n    Body one\n    Body two\n\n a.go | 2 ++\n 1 file changed\n\ndiff --git a/a.go b/a.go\nindex 1..2 100644\n--- a/a.go\n+++ b/a.go\n@@ -1 +1,2 @@\n x\n+y\n"
	got := parseDiff(in)
	want := []diffLine{
		{lineMsg, "Body one"}, {lineMsg, "Body two"}, {lineMsg, ""},
		{lineCtx, " a.go | 2 ++"}, {lineCtx, " 1 file changed"}, {lineCtx, ""},
		{lineFile, "a.go"}, {lineHunk, "@@ -1 +1,2 @@"}, {lineCtx, " x"}, {lineAdd, "+y"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines: %+v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/testrepo"
)

func TestCommitTagAndForce(t *testing.T) {
	dir := testrepo.New(t)
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "T")
	if o, ok := a.topOverlay().(*commitOverlay); !ok || o.title != a.repo.Common.Tr.TagNameTitle {
		t.Fatalf("tag editor: %#v", a.topOverlay())
	}
	tt.Type("release")
	press(tt, "enter")
	if got := strings.TrimSpace(git(t, dir, "rev-parse", "release^{}")); got != strings.TrimSpace(git(t, dir, "rev-parse", "HEAD")) {
		t.Fatal(got)
	}
	press(tt, "j")
	press(tt, "T")
	tt.Type("release")
	tt.Key(0, ui.KeyTab)
	tt.Type("annotation")
	tt.Key(0, ui.KeyTab)
	press(tt, "enter")
	if _, ok := a.topOverlay().(*confirmOverlay); !ok {
		t.Fatalf("force confirmation: %T", a.topOverlay())
	}
	press(tt, "enter")
	if got := strings.TrimSpace(git(t, dir, "rev-parse", "release^{}")); got != strings.TrimSpace(git(t, dir, "rev-parse", "HEAD^")) {
		t.Fatal(got)
	}
	if got := strings.TrimSpace(git(t, dir, "cat-file", "-t", "release")); got != "tag" {
		t.Fatal(got)
	}
}

func TestRevertCommitKeepsDirtyWork(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	testrepo.Write(t, dir, "local.txt", "base\n")
	git(t, dir, "add", "local.txt")
	git(t, dir, "commit", "-m", "base file")
	testrepo.Write(t, dir, "target.txt", "target\n")
	git(t, dir, "add", "target.txt")
	git(t, dir, "commit", "-m", "target")
	testrepo.Write(t, dir, "local.txt", "local change\n")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "t")
	if _, ok := a.topOverlay().(*confirmOverlay); !ok {
		t.Fatalf("revert confirmation: %T", a.topOverlay())
	}
	press(tt, "enter")
	if got := git(t, dir, "log", "-1", "--format=%s"); !strings.Contains(got, "Revert") {
		t.Fatal(got)
	}
	if got := git(t, dir, "diff", "--", "local.txt"); !strings.Contains(got, "local change") {
		t.Fatal(got)
	}
	if a.cursor[panelCommits] != 1 {
		t.Fatalf("cursor = %d", a.cursor[panelCommits])
	}
}

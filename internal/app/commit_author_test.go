package app

import (
	"strings"
	"testing"
)

func TestCommitAuthorRangeAndAmendConfirmation(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "config", "user.name", "Original Author")
	git(t, dir, "config", "user.email", "original@example.com")
	commitFile(t, dir, "one", "one", "one")
	commitFile(t, dir, "two", "two", "two")
	t.Setenv("GIT_AUTHOR_NAME", "New Author")
	t.Setenv("GIT_AUTHOR_EMAIL", "new@example.com")

	t.Setenv("GIT_COMMITTER_NAME", "New Author")
	t.Setenv("GIT_COMMITTER_EMAIL", "new@example.com")
	git(t, dir, "config", "user.name", "New Author")
	git(t, dir, "config", "user.email", "new@example.com")
	a, tt := openApp(t, dir)
	press(tt, "4")
	press(tt, "v")
	press(tt, "j")
	press(tt, "a")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || len(m.items) != 3 {
		t.Fatalf("author menu: %#v", a.topOverlay())
	}
	press(tt, "enter")
	for _, ref := range []string{"HEAD", "HEAD~1"} {
		if author := git(t, dir, "show", "-s", "--format=%an", ref); strings.TrimSpace(author) != "New Author" {
			t.Fatalf("%s author: %s", ref, author)
		}
	}
	press(tt, "esc")
	press(tt, "A")
	if _, ok := a.topOverlay().(*confirmOverlay); !ok {
		t.Fatalf("amend confirmation: %T", a.topOverlay())
	}
}

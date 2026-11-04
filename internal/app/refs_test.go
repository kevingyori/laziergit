package app

import (
	"laziergit/internal/testrepo"
	"strings"
	"testing"
)

func TestRefTabs(t *testing.T) {
	dir := testrepo.New(t)
	git(t, dir, "remote", "add", "origin", "https://example.com/test.git")
	git(t, dir, "tag", "v1")
	a, tt := openApp(t, dir)
	press(tt, "3")
	press(tt, "]")
	if a.scope() != scopeRemotes || !tt.HasText("origin") {
		t.Fatal(tt.Texts())
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/remotes.png")
	}
	press(tt, "]")
	if a.scope() != scopeTags || !tt.HasText("v1") {
		t.Fatal(tt.Texts())
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/tags.png")
	}
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	press(tt, "space")
	press(tt, "enter")
	if head := strings.TrimSpace(git(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); head != "HEAD" {
		t.Fatal(head)
	}
	press(tt, "]")
	if a.branchTab != 0 {
		t.Fatal(a.branchTab)
	}
}

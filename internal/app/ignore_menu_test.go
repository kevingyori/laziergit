package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"laziergit/internal/testrepo"
)

func TestIgnoreOrExcludeMenu(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "secret.txt", "x\n")
	a, tt := openApp(t, dir)
	press(tt, "2")
	selectFile(t, a, "secret.txt")
	press(tt, "i")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || m.title != a.repo.Common.Tr.Actions.IgnoreExcludeFile {
		t.Fatalf("menu: %#v", a.topOverlay())
	}
	press(tt, "e")
	b, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil || !strings.Contains(string(b), "secret.txt") {
		t.Fatalf("exclude: %q %v", b, err)
	}
}

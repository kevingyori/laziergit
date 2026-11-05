package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"laziergit/internal/testrepo"
)

func TestFileTreeDirectoryActions(t *testing.T) {
	dir := testrepo.New(t)
	if err := os.MkdirAll(filepath.Join(dir, "nested/deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	testrepo.Write(t, dir, "nested/deep/one.txt", "one\n")
	testrepo.Write(t, dir, "nested/deep/two.txt", "two\n")
	a, tt := openApp(t, dir)
	a.repo.Config.Gui.ShowRootItemInFileTree = true
	a.repo.Config.Gui.ShowFileTree = true
	a.newFileTrees()
	a.Refresh()
	tt.Frame()
	selectFile(t, a, "nested/deep")
	if out := shotDir(); out != "" {
		tt.Frame()
		shot(t, tt, out+"/file-tree.png")
	}
	press(tt, "enter")
	if _, found := a.files.GetIndexForPath("./nested/deep/one.txt"); found || a.staging != nil {
		t.Fatal("enter on a directory should collapse it")
	}
	press(tt, "space")
	if staged := git(t, dir, "diff", "--cached", "--name-only"); staged != "nested/deep/one.txt\nnested/deep/two.txt\n" {
		t.Fatalf("staged outside selected directory: %q", staged)
	}
	if a.selectedFileNode().GetPath() != "nested/deep" {
		t.Fatal("staging moved selection away from the directory")
	}
	press(tt, "space")
	if staged := git(t, dir, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("directory was not unstaged: %q", staged)
	}
	press(tt, "=")
	selectFile(t, a, "nested/deep/two.txt")
	press(tt, "`")
	if a.files.InTreeMode() || selectedPath(a) != "nested/deep/two.txt" {
		t.Fatal("flat view lost the selected file")
	}
	press(tt, "`")
	if !a.files.InTreeMode() || selectedPath(a) != "nested/deep/two.txt" {
		t.Fatal("tree view lost the selected file")
	}
	press(tt, "-")
	if a.count(panelFiles) != 1 || a.selectedFileNode().GetPath() != "." {
		t.Fatal("collapse-all should select the root")
	}
	press(tt, "=")
	selectFile(t, a, "nested/deep")
	press(tt, "d")
	menu, ok := a.topOverlay().(*menuOverlay)
	if !ok || len(menu.items) != 2 || menu.items[1].disabled == "" {
		t.Fatal("discard menu should retain the disabled unstaged-only action")
	}
	press(tt, "x")
	if _, err := os.Stat(filepath.Join(dir, "nested/deep/one.txt")); !os.IsNotExist(err) {
		t.Fatalf("directory not discarded: %v", err)
	}
	if st := git(t, dir, "status", "--porcelain"); !strings.Contains(st, "a.go") || !strings.Contains(st, "new.txt") {
		t.Fatalf("discard touched files outside the directory: %q", st)
	}
}

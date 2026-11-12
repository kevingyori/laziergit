package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/testrepo"
)

func cliApp(t *testing.T, dir string, args ...string) (*App, *ui.Tester) {
	t.Helper()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE"} {
		t.Setenv(key, "")
		_ = os.Unsetenv(key)
	}
	options, err := ParseCLI(args)
	if err != nil {
		t.Fatal(err)
	}
	r, err := options.Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	sync := func(fn func()) { fn() }
	a := NewWithCLI(r, options, sync, sync, func() {})
	a.Refresh()
	tt := ui.NewTester(a.View, 1280, 800)
	tt.Frame()
	return a, tt
}

func TestCLIStartup(t *testing.T) {
	for _, tc := range []struct {
		arg   string
		focus panel
	}{
		{"status", panelFiles}, {"branch", panelBranches}, {"log", panelCommits}, {"stash", panelStash},
	} {
		t.Run(tc.arg, func(t *testing.T) {
			a, _ := cliApp(t, testrepo.New(t), tc.arg)
			if a.focus != tc.focus || a.screenMode != screenHalf {
				t.Fatalf("focus=%v mode=%v", a.focus, a.screenMode)
			}
		})
	}
	t.Run("filter overrides panel", func(t *testing.T) {
		a, tt := cliApp(t, testrepo.New(t), "branch", "-f=a.go")
		if a.focus != panelCommits || a.screenMode != screenHalf || !a.filteringActive() || len(a.snap.Commits) != 2 {
			t.Fatalf("focus=%v mode=%v commits=%d", a.focus, a.screenMode, len(a.snap.Commits))
		}
		if !strings.Contains(strings.Join(tt.Texts(), " "), "a.go") {
			t.Fatal(tt.Texts())
		}
		press(tt, "esc")
		if a.filteringActive() || a.screenMode != screenNormal {
			t.Fatal("escape did not leave filtering mode")
		}
	})
	t.Run("screen override", func(t *testing.T) {
		a, _ := cliApp(t, testrepo.New(t), "--filter=a.go", "--screen-mode=full")
		if a.screenMode != screenFull {
			t.Fatal(a.screenMode)
		}
	})
}

func TestCLIDotfileRepoAndReturn(t *testing.T) {
	dir := testrepo.New(t)
	dir, _ = filepath.EvalSymlinks(dir)
	source := testrepo.New(t)
	testrepo.Git(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", source, "child")
	testrepo.Git(t, dir, "commit", "-am", "add submodule")
	gitDir := filepath.Join(t.TempDir(), "bare")
	if err := os.Rename(filepath.Join(dir, ".git"), gitDir); err != nil {
		t.Fatal(err)
	}
	// The submodule gitfile moved along with the parent's git directory.
	if err := os.WriteFile(filepath.Join(dir, "child", ".git"), []byte("gitdir: "+filepath.Join(gitDir, "modules", "child")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testrepo.Git(t, dir, "--git-dir="+filepath.Join(gitDir, "modules", "child"), "--work-tree="+filepath.Join(dir, "child"), "config", "core.worktree", filepath.Join(dir, "child"))
	a, tt := cliApp(t, dir, "--git-dir="+gitDir, "--work-tree="+dir)
	a.fileTab, a.focus = 2, panelFiles
	tt.Frame()
	press(tt, "enter")
	if a.snap.Root != filepath.Join(dir, "child") {
		t.Fatalf("still in %s", a.snap.Root)
	}
	press(tt, "esc")
	if a.snap.Root != dir || a.fileTab != 2 || filepath.Clean(os.Getenv("GIT_DIR")) != canonicalPath(t, gitDir) {
		t.Fatalf("return root=%s tab=%d gitdir=%s", a.snap.Root, a.fileTab, os.Getenv("GIT_DIR"))
	}
}

func TestCLIParsing(t *testing.T) {
	t.Setenv("GIT_DIR", "/git")
	t.Setenv("GIT_WORK_TREE", "/tree")
	o, err := ParseCLI([]string{"log", "-sm", "full", "-ucd=/config", "-ucf=a,b", "-d"})
	if err != nil {
		t.Fatal(err)
	}
	if o.GitArg != "log" || o.ScreenMode != "full" || o.UseConfigDir != "/config" || o.CustomConfigFile != "a,b" || !o.Debug || o.GitDir != "/git" || o.WorkTree != "/tree" {
		t.Fatalf("%+v", o)
	}
	for _, args := range [][]string{{"--unknown"}, {"invalid-panel"}, {"--path=repo"}, {"--filter"}, {"log", "branch"}} {
		if _, err := ParseCLI(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	o, err = ParseCLI([]string{"--help"})
	if err != nil || !strings.Contains(o.HelpText, "git-dir") {
		t.Fatalf("help: %v %+v", err, o)
	}
}

func TestCLIGitDirWithConfiguredWorktree(t *testing.T) {
	dir := testrepo.New(t)
	dir, _ = filepath.EvalSymlinks(dir)
	gitDir := filepath.Join(t.TempDir(), "bare")
	if err := os.Rename(filepath.Join(dir, ".git"), gitDir); err != nil {
		t.Fatal(err)
	}
	testrepo.Git(t, dir, "--git-dir="+gitDir, "config", "core.worktree", dir)
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	a, tt := cliApp(t, nested, "-g", gitDir)
	if a.snap.Root != dir || len(a.snap.Commits) != 2 {
		t.Fatalf("root=%s commits=%d", a.snap.Root, len(a.snap.Commits))
	}
	selectFile(t, a, "a.go")
	tt.Frame()
	press(tt, "space")
	if !a.selectedFile().HasStagedChanges {
		t.Fatal("file was not staged")
	}
}

func TestCLIPath(t *testing.T) {
	for _, flag := range []string{"-p", "--path", ""} {
		t.Run(flag, func(t *testing.T) {
			dir := testrepo.New(t)
			args := []string{dir}
			if flag != "" {
				args = append([]string{flag}, args...)
			}
			a, _ := cliApp(t, t.TempDir(), args...)
			if len(a.snap.Commits) != 2 || a.focus != panelFiles || a.screenMode != screenNormal {
				t.Fatalf("unexpected startup state")
			}
		})
	}
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

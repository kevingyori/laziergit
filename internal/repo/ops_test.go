package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/testrepo"
)

func open(t *testing.T, dir string) *Repo {
	t.Helper()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}

func file(t *testing.T, s *Snapshot, path string) *models.File {
	t.Helper()
	for _, f := range s.Files {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("no file %s in %v", path, s.Files)
	return nil
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestApplyLinesStagesOneLine(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "a.go", "package a\n\nfunc A() { return }\n\nfunc B() {}\n\nfunc C() {}\n")
	r := open(t, dir)
	f := file(t, r.Load(), "a.go")
	p := r.WorktreePatch(f, false)
	// Stage only the line adding func C.
	var idx = -1
	for i, l := range p.Lines() {
		if l.Content == "+func C() {}" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("no +func C line in\n%s", p.FormatPlain())
	}
	if err := r.ApplyLines(f, p, false, []int{idx}); err != nil {
		t.Fatal(err)
	}
	cached := gitOut(t, dir, "diff", "--cached")
	if !strings.Contains(cached, "+func C() {}") || strings.Contains(cached, "+func B() {}") {
		t.Fatalf("cached diff:\n%s", cached)
	}
	// Unstage it again from the staged side.
	f = file(t, r.Load(), "a.go")
	sp := r.WorktreePatch(f, true)
	for i, l := range sp.Lines() {
		if l.Content == "+func C() {}" {
			idx = i
		}
	}
	if err := r.ApplyLines(f, sp, true, []int{idx}); err != nil {
		t.Fatal(err)
	}
	if cached := gitOut(t, dir, "diff", "--cached"); strings.Contains(cached, "func C") {
		t.Fatalf("still staged:\n%s", cached)
	}
}

func TestApplyLinesUntrackedPartial(t *testing.T) {
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "new.txt", "one\ntwo\n")
	r := open(t, dir)
	f := file(t, r.Load(), "new.txt")
	p := r.WorktreePatch(f, false)
	idx := -1
	for i, l := range p.Lines() {
		if l.Content == "+one" {
			idx = i
		}
	}
	if err := r.ApplyLines(f, p, false, []int{idx}); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, dir, "show", ":new.txt"); got != "one\n" {
		t.Fatalf("index has %q", got)
	}
}

func TestCommitBranchStash(t *testing.T) {
	dir := testrepo.New(t)
	r := open(t, dir)
	if err := r.StageAll(); err != nil {
		t.Fatal(err)
	}
	if err := r.Commit("third", "with body", false); err != nil {
		t.Fatal(err)
	}
	if msg := gitOut(t, dir, "log", "-1", "--format=%B"); !strings.Contains(msg, "third\n\nwith body") {
		t.Fatalf("message %q", msg)
	}
	if err := r.NewBranch("topic", ""); err != nil {
		t.Fatal(err)
	}
	if s := r.Load(); s.Head.Name != "topic" {
		t.Fatalf("head %s", s.Head.Name)
	}
	if err := r.Checkout("main"); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteBranch("topic", false); err != nil {
		t.Fatal(err)
	}
	testrepo.Write(t, dir, "a.go", "changed\n")
	if err := r.StashPush("wip"); err != nil {
		t.Fatal(err)
	}
	s := r.Load()
	if len(s.Stash) != 2 || len(s.Files) != 0 {
		t.Fatalf("stash=%d files=%d", len(s.Stash), len(s.Files))
	}
	if err := r.StashPop(0); err != nil {
		t.Fatal(err)
	}
	if err := r.StashDrop(0); err != nil {
		t.Fatal(err)
	}
	if s := r.Load(); len(s.Stash) != 0 || len(s.Files) != 1 {
		t.Fatalf("after pop/drop stash=%d files=%d", len(s.Stash), len(s.Files))
	}
}

func TestPushPullLocalRemote(t *testing.T) {
	dir := testrepo.New(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitOut(t, dir, "init", "-q", "--bare", remote)
	gitOut(t, dir, "remote", "add", "origin", remote)
	r := open(t, dir)
	var logged []string
	r.SetHooks(Hooks{LogCommand: func(c string, _ bool) { logged = append(logged, c) }})
	if got := r.UpstreamSuggestion("main"); got != "origin main" {
		t.Fatalf("suggestion %q", got)
	}
	if err := r.Push(git_commands.PushOpts{SetUpstream: true, Remote: "origin", Refspecs: []string{"refs/heads/main:refs/heads/main"}}); err != nil {
		t.Fatal(err)
	}
	if s := r.Load(); s.Head.UpstreamBranch != "main" {
		t.Fatalf("upstream not set: %+v", s.Head)
	}
	if err := r.Fetch(); err != nil {
		t.Fatal(err)
	}
	if err := r.Pull(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(logged, "\n"), "git push") {
		t.Fatalf("push not logged: %v", logged)
	}
	_ = os.Remove
}

package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"laziergit/internal/testrepo"
)

func TestStreamFinishesBeforeConsumerAttaches(t *testing.T) {
	r, err := Open(testrepo.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	parts := r.Stream()
	deadline := time.Now().Add(10 * time.Second)
	for len(parts) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("startup load did not begin")
		}
		time.Sleep(time.Millisecond)
	}
	// A subsequent read takes the same lock, and therefore finishes only after
	// the unconsumed startup stream has released it.
	finished := make(chan struct{})
	go func() { r.Load(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("startup producer blocked without a consumer")
	}
	s := &Snapshot{}
	var seen PartKind
	for p := range parts {
		seen |= p.Kind
		p.Merge(s)
	}
	want := StatePart | FilesPart | StashPart | ReflogPart | BranchesPart | CommitsPart | RemotesPart | WorktreesPart | SubmodulesPart | DonePart
	if seen != want {
		t.Fatalf("scopes: %b, want %b", seen, want)
	}
	if len(s.Files) != 2 || len(s.Commits) != 2 || len(s.Branches) != 2 || len(s.Stash) != 1 {
		t.Fatalf("lost startup data: %+v", s)
	}
}

func TestWorktreeMarkingDoesNotMutatePublishedFiles(t *testing.T) {
	f := &models.File{Path: "linked"}
	s := &Snapshot{Root: "/repo", Files: []*models.File{f}, Worktrees: []*models.Worktree{{Path: "/repo/linked"}}}
	s.MarkWorktreeFiles()
	if !s.Files[0].IsWorktree || f.IsWorktree {
		t.Fatal("worktree marking must copy loader files")
	}
	s.Worktrees = nil
	s.MarkWorktreeFiles()
	if s.Files[0].IsWorktree {
		t.Fatal("removed worktree retained its flag")
	}
}

func TestCommitsDoNotWaitForReflog(t *testing.T) {
	dir := testrepo.New(t)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	gate := filepath.Join(bin, "release-reflog")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := "#!/bin/sh\nfor arg in \"$@\"; do\nif [ \"$arg\" = '-g' ]; then\nwhile [ ! -f " + quote(gate) + " ]; do sleep 0.01; done\nfi\ndone\nexec " + quote(gitPath) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	parts := r.Stream()
	defer func() {
		_ = os.WriteFile(gate, nil, 0600)
		for range parts {
		}
	}()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case p := <-parts:
			if p.Kind == ReflogPart || p.Kind == BranchesPart {
				t.Fatal("reflog unexpectedly passed gate")
			}
			if p.Kind == CommitsPart {
				if len(p.Snapshot.Commits) != 2 {
					t.Fatalf("commits: %v", p.Snapshot.Commits)
				}
				return
			}
		case <-deadline:
			t.Fatal("commits waited for reflog")
		}
	}
}

func TestRepeatedFilterDoesNotWaitForLoad(t *testing.T) {
	r := &Repo{filterPath: "file.txt"}
	r.mu.Lock()
	defer r.mu.Unlock()
	done := make(chan struct{})
	go func() { r.SetFilter("file.txt", ""); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("unchanged startup filter waited for the load")
	}
}

func TestCommitRefWithTagNamedLikeBranch(t *testing.T) {
	dir := testrepo.New(t)
	gitOut(t, dir, "tag", "main")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if s := r.Load(); s.CheckedOutBranch != "main" {
		t.Fatalf("checked out branch: %q", s.CheckedOutBranch)
	}
}

package repo

import (
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

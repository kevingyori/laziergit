package app

import (
	"testing"
	"time"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"laziergit/internal/repo"
)

func TestProgressiveRefreshRejectsStaleParts(t *testing.T) {
	a, _, _ := newTestApp(t)
	var jobs []func()
	posts := make(chan func(), 32)
	a.bg = func(f func()) { jobs = append(jobs, f) }
	a.post = func(f func()) { posts <- f }
	attach := func() chan repo.Part {
		parts := make(chan repo.Part, 16)
		a.RefreshFrom(func() <-chan repo.Part { return parts }, nil)
		job := jobs[len(jobs)-1]
		go job()
		return parts
	}
	apply := func() {
		t.Helper()
		select {
		case f := <-posts:
			f()
		case <-time.After(5 * time.Second):
			t.Fatal("part did not arrive")
		}
	}
	old := attach()
	old <- repo.Part{Kind: repo.StashPart, Snapshot: &repo.Snapshot{Stash: []*models.StashEntry{{Name: "early"}}}}
	apply()
	if a.snap.Stash[0].Name != "early" {
		t.Fatal("stash waits for other scopes")
	}
	newer := attach()
	newer <- repo.Part{Kind: repo.StashPart, Snapshot: &repo.Snapshot{Stash: []*models.StashEntry{{Name: "new"}}}}
	apply()
	old <- repo.Part{Kind: repo.StashPart, Snapshot: &repo.Snapshot{Stash: []*models.StashEntry{{Name: "stale"}}}}
	apply()
	if a.snap.Stash[0].Name != "new" {
		t.Fatal("older generation replaced newer data")
	}
	close(old)
	close(newer)
}

func TestProgressivePartsPreserveCommitSelectionIntent(t *testing.T) {
	a, _, _ := newTestApp(t)
	a.commitSel = selectHeadCommit
	a.cursor[panelCommits] = 1
	a.applyPart(repo.Part{Kind: repo.StashPart, Snapshot: &repo.Snapshot{}})
	if a.commitSel != selectHeadCommit || a.cursor[panelCommits] != 1 {
		t.Fatal("unrelated scope consumed commit selection")
	}
	a.applyPart(repo.Part{Kind: repo.CommitsPart, Snapshot: &repo.Snapshot{Commits: a.raw.Commits, Graph: a.raw.Graph}})
	if a.cursor[panelCommits] != 0 || a.commitSel != keepCommitSelectionByHash {
		t.Fatal("commit scope did not apply selection")
	}
}

func TestProgressivePendingBranchSurvivesStatePart(t *testing.T) {
	a, _, _ := newTestApp(t)
	a.snap, a.raw = nil, nil
	a.pendingBranch = "feature"
	a.applyPart(repo.Part{Kind: repo.StatePart, Snapshot: &repo.Snapshot{}})
	a.applyPart(repo.Part{Kind: repo.BranchesPart, Snapshot: &repo.Snapshot{Branches: []*models.Branch{{Name: "main"}, {Name: "feature"}}}})
	if a.cursor[panelBranches] != 1 {
		t.Fatal("state part lost saved branch selection")
	}
}

func TestProgressiveSupersedingRefreshRetainsOperationCompletion(t *testing.T) {
	a, _, _ := newTestApp(t)
	var jobs []func()
	posts := make(chan func(), 32)
	a.bg = func(f func()) { jobs = append(jobs, f) }
	a.post = func(f func()) { posts <- f }
	old, newer := make(chan repo.Part, 1), make(chan repo.Part, 1)
	completed := 0
	a.refreshFrom(func() <-chan repo.Part { return old }, nil, func() { completed++ })
	go jobs[len(jobs)-1]()
	a.RefreshFrom(func() <-chan repo.Part { return newer }, nil)
	go jobs[len(jobs)-1]()
	old <- repo.Part{Kind: repo.DonePart}
	select {
	case f := <-posts:
		f()
	case <-time.After(5 * time.Second):
		t.Fatal("old completion did not arrive")
	}
	if completed != 0 {
		t.Fatal("stale load completed operation")
	}
	newer <- repo.Part{Kind: repo.DonePart}
	select {
	case f := <-posts:
		f()
	case <-time.After(5 * time.Second):
		t.Fatal("new completion did not arrive")
	}
	if completed != 1 {
		t.Fatal("superseding refresh lost operation completion")
	}
	close(old)
	close(newer)
}

func TestProgressiveWorktreesBeforeFilesKeepRestoredTree(t *testing.T) {
	a, _, _ := newTestApp(t)
	files := a.raw.Files
	selectFile(t, a, "new.txt")
	a.snap, a.raw = nil, nil
	a.refreshParts = 0
	a.applyPart(repo.Part{Kind: repo.StatePart, Snapshot: &repo.Snapshot{}})
	a.applyPart(repo.Part{Kind: repo.WorktreesPart, Snapshot: &repo.Snapshot{}})
	if a.files.Len() == 0 {
		t.Fatal("worktrees cleared the saved files tree")
	}
	a.applyPart(repo.Part{Kind: repo.FilesPart, Snapshot: &repo.Snapshot{Files: files}})
	if selectedPath(a) != "new.txt" {
		t.Fatal("saved file selection was lost")
	}
}

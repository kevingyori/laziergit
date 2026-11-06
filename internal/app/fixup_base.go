package app

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/jesseduffield/generics/set"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

// fixupHunk is lazygit's FixupHelper hunk.
type fixupHunk struct {
	filename     string
	startLineIdx int
	numLines     int
}

// findBaseCommitForFixup is lazygit's FixupHelper.HandleFindBaseCommitForFixupPress.
func (a *App) findBaseCommitForFixup() {
	if a.snap == nil {
		return
	}
	tr := a.repo.Common.Tr
	commits := slices.Clone(a.snap.Commits)
	git := a.repo.Git
	a.bg(func() {
		index, warn, hasStaged, err := func() (int, bool, bool, error) {
			args := []string{"-U0", "--ignore-submodules=all", "HEAD", "--"}
			hasStaged := true
			diff, err := git.Diff.DiffIndexCmdObj(append([]string{"--cached"}, args...)...).DontLog().RunWithOutput()
			if err == nil && diff == "" {
				hasStaged = false
				diff, err = git.Diff.DiffIndexCmdObj(args...).DontLog().RunWithOutput()
			}
			if err != nil {
				return 0, false, false, err
			}
			deleted, added := parseFixupDiff(diff)
			var hashes []string
			warn := false
			if len(deleted) > 0 {
				hashes, err = a.blameDeletedLines(deleted)
				warn = len(added) > 0
			} else if len(added) > 0 {
				hashes, err = a.blameAddedLines(commits, added)
			} else {
				return 0, false, false, errors.New(tr.NoChangedFiles)
			}
			if err != nil {
				return 0, false, false, err
			}
			if len(hashes) == 0 {
				return 0, false, false, errors.New(tr.NoBaseCommitsFound)
			}
			notFoundMeansMerged := len(commits) > 0 && commits[len(commits)-1].Status == models.StatusMerged
			const (
				merged = iota
				notMerged
				cannotTell
			)
			groups := lo.GroupBy(hashes, func(hash string) int {
				c, _, ok := fixupFindCommit(commits, hash)
				if ok {
					return lo.Ternary(c.Status == models.StatusMerged, merged, notMerged)
				}
				return lo.Ternary(notFoundMeansMerged, merged, cannotTell)
			})
			if len(groups[cannotTell]) > 0 {
				return 0, false, false, errors.New(tr.BaseCommitIsNotInCurrentView)
			}
			if len(groups[notMerged]) == 0 {
				return 0, false, false, errors.New(tr.BaseCommitIsAlreadyOnMainBranch)
			}
			found := removeFixupCommits(commitsForHashes(commits, groups[notMerged]))
			if len(found) > 1 {
				subjects := strings.Join(lo.Map(found, func(c *models.Commit, _ int) string {
					return fmt.Sprintf("%s %s", c.ShortRefName(), c.Name)
				}), "\n")
				msg := lo.Ternary(hasStaged, tr.MultipleBaseCommitsFoundStaged, tr.MultipleBaseCommitsFoundUnstaged)
				return 0, false, false, fmt.Errorf("%s\n\n%s", msg, subjects)
			}
			_, index, _ := fixupFindCommit(commits, found[0].Hash())
			return index, warn, hasStaged, nil
		}()
		a.post(func() {
			if err != nil {
				a.errorPopup(err.Error())
				return
			}
			do := func() {
				show := func() {
					a.cancelRange(panelCommits)
					a.cursor[panelCommits] = index
					a.lists[panelCommits].ScrollIntoView(index)
					a.focusPanel(panelCommits)
				}
				if hasStaged {
					show()
					return
				}
				a.run("Staging…", func() error { return git.WorkingTree.StageAll(true) }, show)
			}
			if warn {
				a.confirm(tr.FindBaseCommitForFixup, tr.HunksWithOnlyAddedLinesWarning, false, do)
				return
			}
			do()
		})
	})
}

func fixupFindCommit(commits []*models.Commit, hash string) (*models.Commit, int, bool) {
	return lo.FindIndexOf(commits, func(c *models.Commit) bool { return c.Hash() == hash })
}

func commitsForHashes(commits []*models.Commit, hashes []string) []*models.Commit {
	want := set.NewFromSlice(hashes)
	var result []*models.Commit
	for _, c := range commits {
		if want.Includes(c.Hash()) {
			result = append(result, c)
			want.Remove(c.Hash())
			if want.Len() == 0 {
				break
			}
		}
	}
	return result
}

func removeFixupCommits(commits []*models.Commit) []*models.Commit {
	if len(commits) <= 1 {
		return commits
	}
	base, lastIsFixup := isFixupCommit(commits[len(commits)-1].Name)
	if lastIsFixup {
		return commits
	}
	for i := len(commits) - 2; i >= 0; i-- {
		if subject, ok := isFixupCommit(commits[i].Name); ok && subject == base {
			commits = utils.Remove(commits, i)
		}
	}
	return commits
}

// isFixupCommit is lazygit's helpers.IsFixupCommit.
func isFixupCommit(subject string) (string, bool) {
	trimmed := false
	for {
		cut := false
		for _, prefix := range []string{"fixup! ", "squash! ", "amend! "} {
			if s, ok := strings.CutPrefix(subject, prefix); ok {
				subject, cut, trimmed = s, true, true
				break
			}
		}
		if !cut {
			return subject, trimmed
		}
	}
}

var fixupHunkHeader = regexp.MustCompile(`@@ -(\d+)(?:,\d+)? \+\d+(?:,\d+)? @@`)

// parseFixupDiff is lazygit's FixupHelper parseDiff.
func parseFixupDiff(diff string) (deleted, added []*fixupHunk) {
	var filename string
	var cur *fixupHunk
	nDel, nAdd := 0, 0
	finish := func() {
		if cur != nil {
			if nDel > 0 {
				cur.numLines = nDel
				deleted = append(deleted, cur)
			} else if nAdd > 0 {
				cur.numLines = nAdd
				added = append(added, cur)
			}
		}
		nDel, nAdd = 0, 0
	}
	for _, line := range strings.Split(strings.TrimSuffix(diff, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			finish()
			cur = nil
		case cur == nil && strings.HasPrefix(line, "--- "):
			filename = strings.TrimRight(line[6:], "\t")
		case strings.HasPrefix(line, "@@ "):
			finish()
			m := fixupHunkHeader.FindStringSubmatch(line)
			cur = &fixupHunk{filename, utils.MustConvertToInt(m[1]), 0}
		case cur != nil && line != "" && line[0] == '-':
			nDel++
		case cur != nil && line != "" && line[0] == '+':
			nAdd++
		}
	}
	finish()
	return deleted, added
}

func (a *App) blameDeletedLines(hunks []*fixupHunk) ([]string, error) {
	var errg errgroup.Group
	ch := make(chan string)
	for _, h := range hunks {
		errg.Go(func() error {
			out, err := a.repo.Git.Blame.BlameLineRange(h.filename, "HEAD", h.startLineIdx, h.numLines)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
				ch <- strings.Split(line, " ")[0]
			}
			return nil
		})
	}
	go func() { _ = errg.Wait(); close(ch) }()
	result := set.New[string]()
	for h := range ch {
		result.Add(h)
	}
	return result.ToSlice(), errg.Wait()
}

func (a *App) blameAddedLines(commits []*models.Commit, hunks []*fixupHunk) ([]string, error) {
	tr := a.repo.Common.Tr
	var errg errgroup.Group
	ch := make(chan []string)
	for _, h := range hunks {
		errg.Go(func() error {
			result := make([]string, 0, 2)
			appendBlamed := func(out string) {
				lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
				if len(lines) == 1 {
					result = append(result, strings.Split(lines[0], " ")[0])
				}
			}
			if h.startLineIdx > 0 {
				out, err := a.repo.Git.Blame.BlameLineRange(h.filename, "HEAD", h.startLineIdx, 1)
				if err != nil {
					return err
				}
				appendBlamed(out)
			}
			out, err := a.repo.Git.Blame.BlameLineRange(h.filename, "HEAD", h.startLineIdx+1, 1)
			if err != nil {
				if h.startLineIdx == 0 {
					return errors.New("Entire file")
				}
			} else {
				appendBlamed(out)
			}
			ch <- result
			return nil
		})
	}
	go func() { _ = errg.Wait(); close(ch) }()
	result := set.New[string]()
	var failed error
	for hashes := range ch {
		if len(hashes) == 1 {
			result.Add(hashes[0])
		} else if len(hashes) > 1 {
			if hashes[0] == hashes[1] {
				result.Add(hashes[0])
				continue
			}
			_, i1, ok1 := fixupFindCommit(commits, hashes[0])
			_, i2, ok2 := fixupFindCommit(commits, hashes[1])
			switch {
			case ok1 && ok2:
				result.Add(lo.Ternary(i1 < i2, hashes[0], hashes[1]))
			case ok1:
				result.Add(hashes[0])
			case ok2:
				result.Add(hashes[1])
			default:
				failed = errors.New(tr.NoBaseCommitsFound)
			}
		}
	}
	if err := errg.Wait(); err != nil {
		return nil, err
	}
	return result.ToSlice(), failed
}

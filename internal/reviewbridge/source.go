package reviewbridge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Context is what a guide writer knows beyond the diff: where the change
// lives, the commits in range and, when someone supplied it, the task.
type Context struct {
	Scope   string   `json:"scope"`
	Branch  string   `json:"branch,omitempty"`
	Commits []string `json:"commits,omitempty"`
	Task    string   `json:"task,omitempty"`
}

// Source limits keep a cold reader's context bounded and cheap.
const (
	sourceLines   = 400
	sourceCalls   = 40
	searchMatches = 200
	sourceBytes   = 1 << 20
)

func gitOutput(dir string, stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && stderr.Len() > 0 {
		err = fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return out, err
}

func cleanPath(p string) (string, error) {
	c := path.Clean(strings.TrimPrefix(p, "./"))
	if p == "" || path.IsAbs(c) || c == ".." || strings.HasPrefix(c, "../") || strings.ContainsAny(c, "\x00:") {
		return "", fmt.Errorf("path must be relative to the repository")
	}
	return c, nil
}
func isHash(h string) bool {
	if len(h) < 7 || strings.Trim(h, "0") == "" {
		return false
	}
	for _, c := range h {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// SourceText returns one side of a file pinned to the snapshot: changed files
// by the blob hashes in their diff, unchanged files from the snapshot's trees.
// A worktree file is only accepted when its blob hash still matches the diff.
func SourceText(s Snapshot, file, side string) (string, error) {
	file, err := cleanPath(file)
	if err != nil {
		return "", err
	}
	if side != "old" && side != "new" {
		return "", fmt.Errorf("side must be old or new")
	}
	if s.Worktree == "" {
		return "", fmt.Errorf("this snapshot has no source access")
	}
	blob, inDiff, worktree := "", false, false
	for _, h := range s.Hunks {
		if h.Path != file {
			continue
		}
		inDiff = true
		if side == "old" && blob == "" {
			blob = h.OldBlob
		}
		if side == "new" {
			// Unstaged hunks follow staged ones: the last is the newest side.
			blob, worktree = h.NewBlob, h.Layer == "Unstaged"
		}
	}
	if inDiff {
		if !isHash(blob) {
			return "", fmt.Errorf("%s has no %s side in this change", file, side)
		}
		if size, err := gitOutput(s.Worktree, nil, "cat-file", "-s", blob); err == nil {
			var n int
			if _, err := fmt.Sscan(string(size), &n); err != nil || n > sourceBytes {
				return "", fmt.Errorf("%s is too large to read", file)
			}
			b, err := gitOutput(s.Worktree, nil, "cat-file", "blob", blob)
			return string(b), err
		}
		if !worktree {
			return "", fmt.Errorf("%s side of %s is unavailable", side, file)
		}
		b, err := os.ReadFile(filepath.Join(s.Worktree, filepath.FromSlash(file)))
		if err != nil || len(b) > sourceBytes {
			return "", fmt.Errorf("%s is unavailable or too large", file)
		}
		got, err := gitOutput(s.Worktree, b, "hash-object", "--stdin", "--path", file)
		if err != nil || !strings.HasPrefix(strings.TrimSpace(string(got)), blob) {
			return "", fmt.Errorf("%s changed after the snapshot; ask the reviewer to reload", file)
		}
		return string(b), nil
	}
	tree := s.NewTree
	if side == "old" {
		tree = s.OldTree
	}
	if !isHash(tree) {
		return "", fmt.Errorf("no %s tree for unchanged files", side)
	}
	b, err := gitOutput(s.Worktree, nil, "cat-file", "-s", tree+":"+file)
	if err != nil {
		return "", fmt.Errorf("%s does not exist on the %s side", file, side)
	}
	var n int
	if _, err := fmt.Sscan(string(b), &n); err != nil || n > sourceBytes {
		return "", fmt.Errorf("%s is too large to read", file)
	}
	out, err := gitOutput(s.Worktree, nil, "cat-file", "blob", tree+":"+file)
	return string(out), err
}

// ReadSource returns numbered lines start..end (1-based, inclusive).
func ReadSource(s Snapshot, file, side string, start, end int) (string, error) {
	text, err := SourceText(s, file, side)
	if err != nil {
		return "", err
	}
	if bytes.IndexByte([]byte(text), 0) >= 0 {
		return "", fmt.Errorf("%s is binary", file)
	}
	lines := strings.Split(text, "\n")
	start = max(1, start)
	if end <= 0 || end < start {
		end = start + sourceLines - 1
	}
	end = min(end, start+sourceLines-1, len(lines))
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s side, lines %d-%d of %d)\n", file, side, start, end, len(lines))
	for i := start; i <= end; i++ {
		fmt.Fprintf(&b, "%d\t%s\n", i, lines[i-1])
	}
	return b.String(), nil
}

// SearchSource greps the new side: the snapshot's tree for unchanged files and,
// in working-tree scope, tracked and untracked worktree files.
func SearchSource(s Snapshot, pattern, glob string) (string, error) {
	if strings.TrimSpace(pattern) == "" || len(pattern) > 200 {
		return "", fmt.Errorf("pattern must be 1-200 characters")
	}
	if s.Worktree == "" || !isHash(s.NewTree) {
		return "", fmt.Errorf("this snapshot has no source access")
	}
	args := []string{"grep", "-n", "-I", "-E", "--max-count", "50", "-e", pattern}
	if s.WorkingTree {
		args = append(args, "--untracked")
	} else {
		args = append(args, s.NewTree)
	}
	args = append(args, "--")
	if glob != "" {
		g, err := cleanPath(glob)
		if err != nil {
			return "", err
		}
		args = append(args, ":(glob)"+g)
	}
	out, err := gitOutput(s.Worktree, nil, args...)
	if err != nil && len(out) == 0 {
		if strings.TrimSpace(err.Error()) == "exit status 1" {
			return "No matches.", nil
		}
		return "", err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	prefix := s.NewTree + ":"
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, prefix)
		if len(lines[i]) > 300 {
			lines[i] = lines[i][:300] + "…"
		}
	}
	if len(lines) > searchMatches {
		lines = append(lines[:searchMatches], fmt.Sprintf("… more matches omitted; narrow the pattern or path_glob"))
	}
	return strings.Join(lines, "\n"), nil
}

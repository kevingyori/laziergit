package reviewbridge

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ScopeDir(root, scope string) string {
	if scope == "" {
		scope = "worktree"
	}
	return filepath.Join(root, fmt.Sprintf("%x", sha256.Sum256([]byte(scope))))
}

// ResolveReview locates the app-selected review, never another worktree's store.
func ResolveReview(repo string) (string, error) {
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--absolute-git-dir")
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	root := filepath.Join(strings.TrimSpace(string(b)), "laziergit", "review")
	var active struct {
		Scope string `json:"scope"`
	}
	if err := Read(root, "active.json", &active); err != nil {
		return "", fmt.Errorf("open a guide in this worktree first: %w", err)
	}
	dir := ScopeDir(root, active.Scope)
	if _, err := os.Stat(filepath.Join(dir, "snapshot.json")); err != nil {
		return "", err
	}
	return dir, nil
}

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
		return "", fmt.Errorf("%s is not a git repository, so there is no review to read. The MCP config's --review-mcp argument must be the repository's working directory: ask the user to copy it again with M in laziestgit.", repo)
	}
	root := filepath.Join(strings.TrimSpace(string(b)), "laziergit", "review")
	var active struct {
		Scope string `json:"scope"`
	}
	if err := Read(root, "active.json", &active); err != nil {
		return "", noReview(repo)
	}
	dir := ScopeDir(root, active.Scope)
	if _, err := os.Stat(filepath.Join(dir, "snapshot.json")); err != nil {
		return "", noReview(repo)
	}
	return dir, nil
}

// noReview is what an agent sees when it connects before the guide was
// opened: it can't open the guide itself, so it says who can and how.
func noReview(repo string) error {
	return fmt.Errorf("No review is open for %s yet. Ask the user to open that repository in laziergit and press ~ (the review guide), then call this tool again. You don't need to reconnect.", repo)
}

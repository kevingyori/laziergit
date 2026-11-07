package repo

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
)

const GuideBlobLimit = 1 << 20

// GuideBlob reads the immutable blob named by a diff's index header. An
// unstaged/untracked new side may not exist in the object database: read that
// file once and accept it only when its Git blob hash matches the diff.
// Files which are large, absent or have changed since the diff are plain text.
func (r *Repo) GuideBlob(hash, path string) (string, bool) {
	if len(hash) < 7 || strings.Trim(hash, "0") == "" {
		return "", false
	}
	for _, c := range hash {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return "", false
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	run := func(args ...string) (string, error) {
		return r.OS.Cmd.New(git_commands.NewGitCmd("cat-file").Arg(args...).ToArgv()).DontLog().RunWithOutput()
	}
	size, err := run("-s", hash)
	if err == nil {
		var n int
		if _, err := fmt.Sscan(size, &n); err != nil || n > GuideBlobLimit {
			return "", false
		}
		data, err := run("blob", hash)
		return data, err == nil
	}
	if path == "" {
		return "", false
	}
	root := r.Git.RepoPaths.WorktreePath()
	abs := filepath.Join(root, filepath.FromSlash(path))
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	stat, err := os.Lstat(abs)
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > GuideBlobLimit {
		return "", false
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, GuideBlobLimit+1))
	if err != nil || len(data) > GuideBlobLimit {
		return "", false
	}
	object := append([]byte(fmt.Sprintf("blob %d\x00", len(data))), data...)
	shaOne := fmt.Sprintf("%x", sha1.Sum(object))
	shaTwo := fmt.Sprintf("%x", sha256.Sum256(object))
	if !strings.HasPrefix(shaOne, hash) && !strings.HasPrefix(shaTwo, hash) {
		return "", false
	}
	return string(data), true
}

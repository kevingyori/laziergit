package repo

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GuideBlobRequest names a blob from a diff's index header. Path is set for an
// unstaged/untracked new side, which may not exist in the object database.
type GuideBlobRequest struct{ Hash, Path string }

func guideHash(hash string) bool {
	if len(hash) < 7 || strings.Trim(hash, "0") == "" {
		return false
	}
	for _, c := range hash {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// GuideBlob reads one blob; see GuideBlobs.
func (r *Repo) GuideBlob(hash, path string) (string, bool) {
	text, ok := r.GuideBlobs([]GuideBlobRequest{{hash, path}})[hash]
	return text, ok
}

// GuideBlobs reads the immutable blobs a diff names through one
// `git cat-file --batch`. A blob missing from the object database is read
// from the worktree file once and accepted only when its Git blob hash still
// matches the diff. There is no size limit: callers work off the UI thread.
func (r *Repo) GuideBlobs(reqs []GuideBlobRequest) map[string]string {
	out := map[string]string{}
	var input bytes.Buffer
	var asked []GuideBlobRequest
	for _, q := range reqs {
		if guideHash(q.Hash) {
			asked = append(asked, q)
			fmt.Fprintln(&input, q.Hash)
		}
	}
	if len(asked) == 0 {
		return out
	}
	root := r.Git.RepoPaths.WorktreePath()
	cmd := exec.Command("git", "-C", root, "cat-file", "--batch")
	cmd.Stdin = &input
	stdout, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	missing := map[string]bool{}
	if err == nil {
		br := bufio.NewReaderSize(stdout, 1<<16)
		for _, q := range asked {
			header, err := br.ReadString('\n')
			if err != nil {
				break
			}
			var oid, kind string
			var size int
			if n, _ := fmt.Sscan(header, &oid, &kind, &size); n != 3 || kind != "blob" {
				missing[q.Hash] = true
				continue
			}
			data := make([]byte, size+1) // the content and its newline
			if _, err := io.ReadFull(br, data); err != nil {
				break
			}
			out[q.Hash] = string(data[:size])
		}
		_ = cmd.Wait()
	}
	for _, q := range asked {
		if _, ok := out[q.Hash]; !ok && q.Path != "" {
			if text, ok := worktreeBlob(root, q.Hash, q.Path); ok {
				out[q.Hash] = text
			}
		}
	}
	return out
}

func worktreeBlob(root, hash, path string) (string, bool) {
	abs := filepath.Join(root, filepath.FromSlash(path))
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	stat, err := os.Lstat(abs)
	if err != nil || !stat.Mode().IsRegular() {
		return "", false
	}
	data, err := os.ReadFile(abs)
	if err != nil {
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

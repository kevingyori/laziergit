// Package parity manages forks of lazygit's integration tests: copies of an
// upstream test edited where laziergit deliberately differs from lazygit.
//
// Forks live in integration/tests/<area>/<name>.go below this package (lazygit
// names a test after the path below "integration/tests/"), each with a header
// recording the upstream file it came from:
//
//	// Forked from lazygit pkg/integration/tests/conflicts/filter.go.
//	// lazygit: v0.66.2
//	// upstream-sha256: 3f2a…
//	// why: conflicts are resolved from a menu, not lazygit's merge view
//
// The drift test fails when the upstream file changes, so every fork is
// re-read against lazygit's new version. cmd/parityfork creates, accepts and
// registers forks.
package parity

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ForksDir is where forks live, relative to this package.
const ForksDir = "integration/tests"

// Lazygit locates the lazygit module this build uses.
func Lazygit() (dir, version string, err error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}} {{.Version}}", "github.com/jesseduffield/lazygit").Output()
	if err != nil {
		return "", "", fmt.Errorf("go list lazygit: %w", err)
	}
	dir, version, _ = strings.Cut(strings.TrimSpace(string(out)), " ")
	return dir, version, nil
}

// UpstreamPath is the upstream test file for a test name like "conflicts/filter".
func UpstreamPath(lazygitDir, name string) string {
	return filepath.Join(lazygitDir, "pkg", "integration", "tests", filepath.FromSlash(name)+".go")
}

// Hash is the hex sha256 of a file's bytes.
func Hash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// Fork is one forked test.
type Fork struct {
	Name    string // e.g. "conflicts/filter"
	Path    string // the fork's file
	Version string // lazygit version it was forked from (or last accepted at)
	Hash    string // sha256 of the upstream file at that version
	Why     string
	Var     string // the exported test variable
}

// Forks lists the forks below root (this package's directory).
func Forks(root string) ([]Fork, error) {
	var forks []Fork
	base := filepath.Join(root, ForksDir)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || filepath.Dir(path) == base {
			return nil
		}
		f, err := ReadFork(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(base, path)
		f.Name = strings.TrimSuffix(filepath.ToSlash(rel), ".go")
		forks = append(forks, f)
		return nil
	})
	sort.Slice(forks, func(i, j int) bool { return forks[i].Name < forks[j].Name })
	return forks, err
}

// ReadFork parses a fork's header and finds its test variable.
func ReadFork(path string) (Fork, error) {
	f := Fork{Path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "//") {
			break
		}
		k, v, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, "//")), ": ")
		if !ok {
			continue
		}
		switch k {
		case "lazygit":
			f.Version = v
		case "upstream-sha256":
			f.Hash = v
		case "why":
			f.Why = v
		}
	}
	if f.Version == "" || f.Hash == "" {
		return f, fmt.Errorf("%s: missing fork header (lazygit, upstream-sha256)", path)
	}
	if strings.TrimSpace(f.Why) == "" {
		return f, fmt.Errorf("%s: a fork must say why it differs from lazygit (// why: …)", path)
	}
	f.Var, err = testVar(path, b)
	return f, err
}

// testVar finds the exported variable a test file declares.
func testVar(path string, src []byte) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		return "", err
	}
	for _, obj := range file.Scope.Objects {
		if obj.Kind == ast.Var && token.IsExported(obj.Name) {
			return obj.Name, nil
		}
	}
	return "", fmt.Errorf("%s: no exported test variable", path)
}

// Header is the comment block a fork starts with.
func Header(name, version, hash, why string) string {
	return fmt.Sprintf("// Forked from lazygit pkg/integration/tests/%s.go.\n// lazygit: %s\n// upstream-sha256: %s\n// why: %s\n\n",
		name, version, hash, why)
}

// SetHeader replaces a fork's header, keeping the rest of the file.
func SetHeader(src []byte, header string) []byte {
	lines := strings.SplitAfter(string(src), "\n")
	i := 0
	for i < len(lines) && (strings.HasPrefix(lines[i], "//") || strings.TrimSpace(lines[i]) == "") {
		i++
	}
	return []byte(header + strings.Join(lines[i:], ""))
}

// Registry is the generated forks.go that lists every fork for the runner.
func Registry(forks []Fork) ([]byte, error) {
	var b strings.Builder
	b.WriteString("// Code generated by parityfork; DO NOT EDIT.\n\n")
	b.WriteString("// Package tests lists laziergit's forks of lazygit's integration tests.\npackage tests\n\n")
	b.WriteString("import (\n\t\"github.com/jesseduffield/lazygit/pkg/integration/components\"\n")
	areas := map[string]bool{}
	for _, f := range forks {
		area, _, _ := strings.Cut(f.Name, "/")
		if !areas[area] {
			areas[area] = true
			fmt.Fprintf(&b, "\t%s \"laziergit/internal/parity/%s/%s\"\n", alias(area), ForksDir, area)
		}
	}
	b.WriteString(")\n\n// Forks replace the upstream tests of the same name.\nvar Forks = []*components.IntegrationTest{\n")
	for _, f := range forks {
		area, _, _ := strings.Cut(f.Name, "/")
		fmt.Fprintf(&b, "\t%s.%s,\n", alias(area), f.Var)
	}
	b.WriteString("}\n")
	return format.Source([]byte(b.String()))
}

func alias(area string) string { return "fork_" + area }

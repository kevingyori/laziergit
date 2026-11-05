// Command parityfork forks lazygit integration tests into laziergit, for
// behaviour laziergit deliberately does differently. Run from the repo root:
//
//	go run ./internal/parity/cmd/parityfork -why "reason" conflicts/filter …
//	go run ./internal/parity/cmd/parityfork -accept conflicts/filter   # after reviewing upstream drift
//	go run ./internal/parity/cmd/parityfork -gen                       # rewrite forks.go
//
// A fork replaces the upstream test of the same name in TestLazygitParity and
// is reported separately from the parity score.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"laziergit/internal/parity"
)

func main() {
	why := flag.String("why", "", "why laziergit differs from lazygit here (required to fork)")
	accept := flag.Bool("accept", false, "record the current upstream version for the named forks, after reviewing lazygit's change")
	gen := flag.Bool("gen", false, "only regenerate forks.go")
	flag.Parse()
	root := filepath.Join("internal", "parity")
	if _, err := os.Stat(root); err != nil {
		fail("run from the laziergit repo root")
	}
	lgDir, version, err := parity.Lazygit()
	check(err)

	switch {
	case *gen:
	case *accept:
		for _, name := range flag.Args() {
			path := forkPath(root, name)
			f, err := parity.ReadFork(path)
			check(err)
			h, err := parity.Hash(parity.UpstreamPath(lgDir, name))
			check(err)
			src, err := os.ReadFile(path)
			check(err)
			check(os.WriteFile(path, parity.SetHeader(src, parity.Header(name, version, h, f.Why)), 0o644))
			fmt.Printf("accepted %s at lazygit %s\n", name, version)
		}
	default:
		if *why == "" || flag.NArg() == 0 {
			fail("usage: parityfork -why \"reason\" <area/test>…")
		}
		for _, name := range flag.Args() {
			up := parity.UpstreamPath(lgDir, name)
			src, err := os.ReadFile(up)
			if err != nil {
				fail(fmt.Sprintf("no lazygit test %s (%v)", name, err))
			}
			path := forkPath(root, name)
			if _, err := os.Stat(path); err == nil {
				fail(path + " already exists")
			}
			h, err := parity.Hash(up)
			check(err)
			check(os.MkdirAll(filepath.Dir(path), 0o755))
			check(os.WriteFile(path, parity.SetHeader(src, parity.Header(name, version, h, *why)), 0o644))
			fmt.Printf("forked %s → %s\n", name, path)
		}
	}

	forks, err := parity.Forks(root)
	check(err)
	reg, err := parity.Registry(forks)
	check(err)
	check(os.WriteFile(filepath.Join(root, parity.ForksDir, "forks.go"), reg, 0o644))
	// A fork that leans on helpers private to lazygit's test package won't
	// compile on its own; say so now rather than at test time.
	if out, err := exec.Command("go", "vet", "./"+filepath.ToSlash(root)+"/...").CombinedOutput(); err != nil {
		fail(fmt.Sprintf("forks don't build (copy any package-private helpers they use):\n%s", out))
	}
}

func forkPath(root, name string) string {
	return filepath.Join(root, parity.ForksDir, filepath.FromSlash(name)+".go")
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "parityfork:", msg)
	os.Exit(1)
}

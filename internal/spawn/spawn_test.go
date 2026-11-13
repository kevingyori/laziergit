package spawn

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// result is what a caller of exec.Cmd can observe.
type result struct {
	Stdout, Stderr string
	Err            string
	ErrType        string
	ExitCode       int
}

func observe(stdout, stderr *bytes.Buffer, cmd *exec.Cmd, err error) result {
	r := result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: -2}
	if err != nil {
		r.Err, r.ErrType = err.Error(), fmt.Sprintf("%T", err)
	}
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
	return r
}

// both runs build() through os/exec and through Run and returns the two
// observations.
func both(t *testing.T, build func() *exec.Cmd) (forked, spawned result) {
	t.Helper()
	run := func(useSpawn bool) result {
		cmd := build()
		var stdout, stderr bytes.Buffer
		if cmd.Stdout == nil {
			cmd.Stdout = &stdout
		}
		if cmd.Stderr == nil {
			cmd.Stderr = &stderr
		}
		if !useSpawn {
			return observe(&stdout, &stderr, cmd, cmd.Run())
		}
		if !Supported(cmd) {
			t.Fatalf("not supported: %v", cmd.Args)
		}
		return observe(&stdout, &stderr, cmd, Run(cmd))
	}
	return run(false), run(true)
}

func sh(script string) *exec.Cmd { return exec.Command("/bin/sh", "-c", script) }

func TestRunMatchesExec(t *testing.T) {
	tmp, err := os.MkdirTemp("", "spawn-dir-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	cases := map[string]func() *exec.Cmd{
		"stdout and stderr": func() *exec.Cmd { return sh("echo out; echo err >&2; echo out2") },
		"exit code":         func() *exec.Cmd { return sh("echo failing >&2; exit 3") },
		"killed":            func() *exec.Cmd { return sh("kill -9 $$") },
		"terminated":        func() *exec.Cmd { return sh("kill -TERM $$") },
		"env": func() *exec.Cmd {
			c := sh(`printf '%s|%s|%s' "$FOO" "$BAR" "$PWD"`)
			c.Env = []string{"FOO=1", "BAR=2", "FOO=3"} // os/exec keeps the last FOO
			return c
		},
		"inherited env and PWD from Dir": func() *exec.Cmd {
			c := sh(`printf '%s|%s|' "$HOME" "$PWD"; pwd -P`)
			c.Dir = tmp
			return c
		},
		"git in a dir": func() *exec.Cmd {
			c := exec.Command("git", "rev-parse", "--show-toplevel")
			c.Dir = tmp
			return c
		},
		"stdin": func() *exec.Cmd {
			c := exec.Command("/bin/cat")
			c.Stdin = strings.NewReader("hello\nworld\n")
			return c
		},
		"stdin not read": func() *exec.Cmd {
			c := sh("exit 0")
			c.Stdin = strings.NewReader(strings.Repeat("x", 1<<20))
			return c
		},
		"no stdin is /dev/null": func() *exec.Cmd { return sh("cat; echo done") },
		"missing dir": func() *exec.Cmd {
			c := sh("true")
			c.Dir = "/nonexistent/dir"
			return c
		},
		"not executable": func() *exec.Cmd { return exec.Command("/etc/hosts") },
		"missing binary": func() *exec.Cmd { return exec.Command("/nonexistent/binary") },
		"argv0": func() *exec.Cmd {
			c := sh(`echo "$0" "$@"`)
			c.Args = append(c.Args, "custom", "a b", "")
			return c
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			forked, spawned := both(t, build)
			if forked != spawned {
				t.Fatalf("os/exec:\n%#v\nspawn:\n%#v", forked, spawned)
			}
		})
	}
}

func TestCombinedOutputSharesOnePipe(t *testing.T) {
	build := func() *exec.Cmd { return sh("for i in 1 2 3 4 5; do echo out$i; echo err$i >&2; done") }
	want, err := build().CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	cmd := build()
	var b bytes.Buffer
	cmd.Stdout, cmd.Stderr = &b, &b
	if err := Run(cmd); err != nil {
		t.Fatal(err)
	}
	if b.String() != string(want) {
		t.Fatalf("got %q want %q", b.String(), want)
	}
}

func TestExitErrorIsReal(t *testing.T) {
	cmd := sh("exit 1")
	err := Run(cmd)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("got %T %v", err, err)
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); !ok || ws.ExitStatus() != 1 {
		t.Fatalf("wait status %#v", exitErr.Sys())
	}
	if cmd.ProcessState == nil || cmd.Process == nil || cmd.Process.Pid != cmd.ProcessState.Pid() {
		t.Fatal("Process and ProcessState not set")
	}
}

// childFds lists the fds a child sees (including the one ls opens to list them).
func childFds(t *testing.T, run func(*exec.Cmd) error) []string {
	t.Helper()
	cmd := sh("ls /dev/fd")
	if !Supported(cmd) {
		t.Fatal("descriptor probe is not supported")
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := run(cmd); err != nil {
		t.Fatal(err)
	}
	return strings.Fields(out.String())
}

func TestOnlyStdioReachesTheChild(t *testing.T) {
	// The shell and ls open descriptors of their own. Capture those before
	// adding our sentinel; os/exec may also inherit unrelated descriptors
	// from the test runner, so its descriptor list is not a clean baseline.
	want := childFds(t, Run)
	// An fd without close-on-exec: os/exec's fork leaks it, posix_spawn
	// with POSIX_SPAWN_CLOEXEC_DEFAULT must not.
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	// High, so it can't collide with the fds ls opens.
	fd, err := unix.FcntlInt(null.Fd(), unix.F_DUPFD, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	leaked := fmt.Sprint(fd)
	forked := childFds(t, (*exec.Cmd).Run)
	if !slices.Contains(forked, leaked) {
		t.Fatalf("os/exec no longer leaks fd %s; the test can't tell", leaked)
	}
	if slices.Contains(want, leaked) {
		t.Fatalf("sentinel fd %s already present in baseline %v", leaked, want)
	}
	if got := childFds(t, Run); !reflect.DeepEqual(got, want) {
		t.Fatalf("child fds %v, want %v", got, want)
	}
}

func TestChildSignals(t *testing.T) {
	// Blocked signals show in ps's sigmask; the child's must be empty, and
	// SIGPIPE must kill it (see TestEarlyStop).
	cmd := sh(`ps -o sigmask= -p $$`)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := Run(cmd); err != nil {
		t.Fatal(err)
	}
	if mask := strings.TrimSpace(out.String()); mask != "0" && mask != "00000000" {
		t.Fatalf("child sigmask %q", mask)
	}
}

func TestEarlyStop(t *testing.T) {
	for _, useSpawn := range []bool{false, true} {
		cmd := exec.Command("/usr/bin/yes")
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdout = pw
		var wait func() error
		if useSpawn {
			p, err := Start(cmd)
			if err != nil {
				t.Fatal(err)
			}
			wait = p.Wait
		} else {
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			wait = cmd.Wait
		}
		pw.Close()
		sc := bufio.NewScanner(pr)
		for i := 0; i < 3 && sc.Scan(); i++ {
		}
		pr.Close()
		if err := wait(); err == nil || err.Error() != "signal: broken pipe" {
			t.Fatalf("spawn=%v: got %v, want death by SIGPIPE", useSpawn, err)
		}
	}
}

func TestConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := range 64 {
		wg.Go(func() {
			cmd := sh(fmt.Sprintf("echo %d; echo e%d >&2", i, i))
			var out, errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errOut
			if err := Run(cmd); err != nil {
				errs <- err
				return
			}
			if out.String() != fmt.Sprintf("%d\n", i) || errOut.String() != fmt.Sprintf("e%d\n", i) {
				errs <- fmt.Errorf("%d: got %q %q", i, out.String(), errOut.String())
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestSupported(t *testing.T) {
	if !Supported(sh("true")) {
		t.Fatal("plain command not supported")
	}
	notSupported := map[string]func() *exec.Cmd{
		"lookup failed": func() *exec.Cmd { return exec.Command("no-such-binary-anywhere") },
		"relative":      func() *exec.Cmd { c := sh("true"); c.Path = "sh"; return c },
		"sysprocattr":   func() *exec.Cmd { c := sh("true"); c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}; return c },
		"extra files":   func() *exec.Cmd { c := sh("true"); c.ExtraFiles = []*os.File{os.Stdin}; return c },
		"context":       func() *exec.Cmd { return exec.CommandContext(t.Context(), "/bin/sh", "-c", "true") },
		"NUL in arg":    func() *exec.Cmd { return sh("true\x00") },
		"NUL in env":    func() *exec.Cmd { c := sh("true"); c.Env = []string{"A=\x00"}; return c },
	}
	for name, build := range notSupported {
		if Supported(build()) {
			t.Errorf("%s: supported", name)
		}
	}
}

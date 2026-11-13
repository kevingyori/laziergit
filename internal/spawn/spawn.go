// Package spawn starts commands with libc posix_spawn instead of Go's
// fork+exec. On macOS, fork() in a process that is showing a window stalls
// AppKit's main thread (fork takes libc's and the frameworks' locks and
// copies the address space); posix_spawn creates the child in the kernel
// without forking, so the UI keeps painting while git runs.
//
// Start and Run take an *exec.Cmd and behave like its Start and Run for the
// commands Supported accepts: same stdio wiring, environment, working
// directory, *exec.ExitError and error text. Anything else should go through
// exec.Cmd itself.
package spawn

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"
)

// started counts the processes Start has spawned (for tests).
var started atomic.Int64

// Supported reports whether Start can run cmd exactly as cmd.Start would.
func Supported(cmd *exec.Cmd) bool {
	if !loadLibc() {
		return false
	}
	if cmd.Process != nil || cmd.Err != nil || cmd.SysProcAttr != nil ||
		len(cmd.ExtraFiles) > 0 || cmd.Cancel != nil || cmd.WaitDelay != 0 {
		return false
	}
	if !filepath.IsAbs(cmd.Path) || hasNUL(cmd.Path) || hasNUL(cmd.Dir) {
		return false
	}
	for _, list := range [][]string{cmd.Args, cmd.Env} {
		for _, s := range list {
			if hasNUL(s) {
				return false
			}
		}
	}
	return true
}

func hasNUL(s string) bool { return strings.IndexByte(s, 0) >= 0 }

// spawnRequest is one posix_spawn call: what os.StartProcess would get.
type spawnRequest struct {
	path string
	argv []string
	envv []string
	dir  string // "" keeps the parent's
	// stdio[i] is the parent's fd that becomes the child's fd i, or -1 for
	// /dev/null.
	stdio [3]int
}

// Proc is a command started by Start.
type Proc struct {
	cmd        *exec.Cmd
	copies     chan error // one result per stdio copying goroutine
	nCopies    int
	closeAfter []io.Closer // parent ends of pipes, closed by Wait
}

// Run is cmd.Run for a Supported cmd.
func Run(cmd *exec.Cmd) error {
	p, err := Start(cmd)
	if err != nil {
		return err
	}
	return p.Wait()
}

// Start is cmd.Start for a Supported cmd. It sets cmd.Process; Proc.Wait
// sets cmd.ProcessState. Stdin, Stdout and Stderr are wired as os/exec does:
// nil is /dev/null, an *os.File is handed to the child, any other reader or
// writer is copied through a pipe, and Stderr equal to Stdout shares its pipe.
func Start(cmd *exec.Cmd) (*Proc, error) {
	p := &Proc{cmd: cmd, copies: make(chan error, 3)}
	var closeAfterStart []io.Closer
	var copiers []func() error
	fail := func(err error) (*Proc, error) {
		closeAll(closeAfterStart)
		closeAll(p.closeAfter)
		return nil, err
	}

	var req spawnRequest
	switch in := cmd.Stdin.(type) {
	case nil:
		req.stdio[0] = -1
	case *os.File:
		req.stdio[0] = int(in.Fd())
	default:
		pr, pw, err := os.Pipe()
		if err != nil {
			return fail(err)
		}
		req.stdio[0] = int(pr.Fd())
		closeAfterStart = append(closeAfterStart, pr)
		p.closeAfter = append(p.closeAfter, pw)
		copiers = append(copiers, func() error {
			_, err := io.Copy(pw, in)
			if skipStdinCopyError(err) {
				err = nil
			}
			if err1 := pw.Close(); err == nil {
				err = err1
			}
			return err
		})
	}
	writer := func(w io.Writer) (int, error) {
		switch w := w.(type) {
		case nil:
			return -1, nil
		case *os.File:
			return int(w.Fd()), nil
		}
		pr, pw, err := os.Pipe()
		if err != nil {
			return 0, err
		}
		closeAfterStart = append(closeAfterStart, pw)
		p.closeAfter = append(p.closeAfter, pr)
		copiers = append(copiers, func() error {
			_, err := io.Copy(w, pr)
			pr.Close() // in case io.Copy stopped due to a write error
			return err
		})
		return int(pw.Fd()), nil
	}
	var err error
	if req.stdio[1], err = writer(cmd.Stdout); err != nil {
		return fail(err)
	}
	if cmd.Stderr != nil && interfaceEqual(cmd.Stderr, cmd.Stdout) {
		req.stdio[2] = req.stdio[1]
	} else if req.stdio[2], err = writer(cmd.Stderr); err != nil {
		return fail(err)
	}
	// A parent fd below 3 that lands on another slot would be overwritten by
	// an earlier dup2 in the child; move it out of the way first.
	for i, fd := range req.stdio {
		if fd >= 0 && fd < 3 && fd != i {
			moved, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 3)
			if err != nil {
				return fail(err)
			}
			req.stdio[i] = moved
			closeAfterStart = append(closeAfterStart, os.NewFile(uintptr(moved), ""))
		}
	}

	req.path = cmd.Path
	req.argv = cmd.Args
	if len(req.argv) == 0 {
		req.argv = []string{cmd.Path}
	}
	req.envv = cmd.Environ()
	req.dir = cmd.Dir
	// os.StartProcess checks the directory first for a clearer error.
	if req.dir != "" {
		if _, err := os.Stat(req.dir); err != nil {
			pe := err.(*fs.PathError)
			pe.Op = "chdir"
			return fail(pe)
		}
	}
	pid, err := posixSpawn(req)
	if err != nil {
		return fail(&fs.PathError{Op: "fork/exec", Path: cmd.Path, Err: err})
	}
	closeAll(closeAfterStart)
	started.Add(1)
	cmd.Process, err = os.FindProcess(pid)
	if err != nil { // never on unix
		return nil, err
	}
	p.nCopies = len(copiers)
	for _, c := range copiers {
		go func() { p.copies <- c() }()
	}
	return p, nil
}

// Wait is cmd.Wait: it reaps the child, sets cmd.ProcessState, waits for the
// stdio copies and returns an *exec.ExitError for a non-zero exit.
func (p *Proc) Wait() error {
	state, err := p.cmd.Process.Wait()
	if err == nil && !state.Success() {
		err = &exec.ExitError{ProcessState: state}
	}
	p.cmd.ProcessState = state
	var copyErr error
	for range p.nCopies {
		if e := <-p.copies; e != nil && copyErr == nil {
			copyErr = e
		}
	}
	closeAll(p.closeAfter)
	if err != nil {
		return err
	}
	return copyErr
}

func closeAll(cs []io.Closer) {
	for _, c := range cs {
		c.Close()
	}
}

// interfaceEqual is os/exec's: whether a == b, without panicking on
// incomparable types.
func interfaceEqual(a, b any) bool {
	defer func() { recover() }()
	return a == b
}

// skipStdinCopyError is os/exec's: the child exiting without reading all of
// its stdin is not an error.
func skipStdinCopyError(err error) bool {
	pe, ok := err.(*fs.PathError)
	return ok && pe.Op == "write" && pe.Path == "|1" && pe.Err == syscall.EPIPE
}

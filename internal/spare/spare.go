// Package spare implements the one-shot local hand-off to a pre-launched GUI.
package spare

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Identity identifies the executable, including replacements at the same path.
type Identity struct {
	Path           string
	Device, Inode  uint64
	Size, Modified int64
}

// Request is the complete terminal invocation; Args includes argv[0].
type Request struct {
	Identity  Identity
	Dir       string
	Args, Env []string
}

// State is the one-shot spare lifecycle. Only Claimed requests reach the GUI.
type State string

const (
	Listening State = "listening"
	Claimed   State = "claimed"
	Stale     State = "stale"
	Refused   State = "refused"
)

func Executable() (Identity, error) {
	p, err := os.Executable()
	if err != nil {
		return Identity{}, err
	}
	return Identify(p)
}
func Identify(p string) (Identity, error) {
	p, err := filepath.EvalSymlinks(p)
	if err != nil {
		return Identity{}, err
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return Identity{}, err
	}
	s, err := os.Stat(p)
	if err != nil {
		return Identity{}, err
	}
	stat := s.Sys().(*syscall.Stat_t)
	return Identity{p, uint64(stat.Dev), stat.Ino, s.Size(), s.ModTime().UnixNano()}, nil
}

// Socket is stable across bundle replacements, so an old spare can be retired.
func Socket(id Identity) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(id.Path))
	dir := filepath.Join(base, "laziergit", fmt.Sprintf("spare-%x", h[:8]))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	s, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !s.IsDir() || s.Mode().Perm() != 0700 || s.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
		return "", errors.New("unsafe spare directory")
	}
	return filepath.Join(dir, "s"), nil
}

// Listen serializes stale-file recovery as well as bind: a racing launcher must
// never unlink the socket that another launcher has just bound.
func Listen(path string) (*net.UnixListener, error) {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return nil, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	if c, err := net.DialTimeout("unix", path, 100*time.Millisecond); err == nil {
		c.Close()
		return nil, errors.New("spare already listening")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

func Send(path string, req Request) (State, error) {
	c, err := net.DialTimeout("unix", path, 150*time.Millisecond)
	if err != nil {
		return Refused, err
	}
	defer c.Close()
	if err := CheckPeer(c.(*net.UnixConn), uint32(os.Getuid())); err != nil {
		return Refused, err
	}
	c.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return Refused, err
	}
	var state State
	err = json.NewDecoder(c).Decode(&state)
	return state, err
}

// Receive ignores probes and malformed/unauthorized clients. A stale request
// retires the process; a valid request is acknowledged before repo loading.
func Receive(l *net.UnixListener, id Identity) (Request, State, error) {
	for {
		c, err := l.AcceptUnix()
		if err != nil {
			return Request{}, Refused, err
		}
		c.SetDeadline(time.Now().Add(2 * time.Second))
		if err := CheckPeer(c, uint32(os.Getuid())); err != nil {
			c.Close()
			continue
		}
		var req Request
		err = json.NewDecoder(io.LimitReader(c, 16<<20)).Decode(&req)
		if err != nil {
			c.Close()
			continue
		}
		state := Claimed
		if req.Identity != id {
			state = Stale
		} else if req.Dir == "" || len(req.Args) == 0 {
			state = Refused
		}
		err = json.NewEncoder(c).Encode(state)
		c.Close()
		if state == Refused {
			continue
		}
		l.Close()
		return req, state, err
	}
}

// Apply must precede CLI parsing and any repo/config access. The process only
// accepts one request, so no other repository can observe this environment.
func (r Request) Apply() error {
	if err := os.Chdir(r.Dir); err != nil {
		return err
	}
	os.Clearenv()
	for _, entry := range r.Env {
		k, v, ok := strings.Cut(entry, "=")
		if !ok {
			return errors.New("invalid environment entry")
		}
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	os.Args = append([]string(nil), r.Args...)
	return nil
}

func Spawn(id Identity, args, env []string, dir string) error {
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	cmd := exec.Command(id.Path, args...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.Stdin = null
	cmd.Stdout = null
	cmd.Stderr = null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Window processes outlive racing spares that lose the bind. Reap those
	// children without tying the caller or terminal to their lifetime.
	go func() { _ = cmd.Wait() }()
	return nil
}

// Available probes without consuming the one-shot listener.
func Available(path string) bool {
	c, err := net.DialTimeout("unix", path, 100*time.Millisecond)
	if err != nil {
		return false
	}
	defer c.Close()
	return CheckPeer(c.(*net.UnixConn), uint32(os.Getuid())) == nil
}

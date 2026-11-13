package spare

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func socketPath(t *testing.T) string { t.Helper(); return filepath.Join(t.TempDir(), "s") }
func TestRoundTrip(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "claimed", true: "stale"}[stale], func(t *testing.T) {
			p := socketPath(t)
			l, err := Listen(p)
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			id := Identity{Path: "/test", Inode: 1}
			req := Request{Identity: id, Dir: "/repo", Args: []string{"laziergit", "-p", "/repo", "--guide"}, Env: []string{"PATH=/bin", "EDITOR=ed"}}
			want := Claimed
			if stale {
				req.Identity.Inode++
				want = Stale
			}
			done := make(chan Request, 1)
			go func() {
				r, s, e := Receive(l, id)
				if e != nil || s != want {
					t.Errorf("receive: %s %v", s, e)
				}
				done <- r
			}()
			state, err := Send(p, req)
			if err != nil || state != want {
				t.Fatalf("send: %s %v", state, err)
			}
			if got := <-done; !reflect.DeepEqual(got, req) {
				t.Fatalf("request: %#v", got)
			}
		})
	}
}
func TestApply(t *testing.T) {
	cwd, _ := os.Getwd()
	env := os.Environ()
	args := os.Args
	defer func() { _ = (Request{Dir: cwd, Args: args, Env: env}).Apply() }()
	dir := t.TempDir()
	r := Request{Dir: dir, Args: []string{"app", "-w", "tree"}, Env: []string{"ONLY=present", "VALUE=a=b"}}
	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.Getwd()
	realDir, _ := filepath.EvalSymlinks(dir)
	if got != realDir || os.Getenv("ONLY") != "present" || os.Getenv("VALUE") != "a=b" || os.Getenv("HOME") != "" || !reflect.DeepEqual(os.Args, r.Args) {
		t.Fatal("invocation not applied exactly")
	}
}
func TestStaleSocketAndRace(t *testing.T) {
	p := socketPath(t)
	l, e := net.ListenUnix("unix", &net.UnixAddr{Name: p, Net: "unix"})
	if e != nil {
		t.Fatal(e)
	}
	l.SetUnlinkOnClose(false)
	l.Close()
	var wg sync.WaitGroup
	winners := make(chan *net.UnixListener, 16)
	for range 16 {
		wg.Go(func() {
			if l, e := Listen(p); e == nil {
				winners <- l
			}
		})
	}
	wg.Wait()
	close(winners)
	count := 0
	for l := range winners {
		count++
		defer l.Close()
	}
	if count != 1 {
		t.Fatalf("%d spares won", count)
	}
}
func TestPeerUID(t *testing.T) {
	p := socketPath(t)
	l, e := Listen(p)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	done := make(chan error, 1)
	go func() {
		c, e := l.AcceptUnix()
		if e == nil {
			defer c.Close()
			e = CheckPeer(c, uint32(os.Getuid()))
			if e == nil {
				if CheckPeer(c, uint32(os.Getuid()+1)) == nil {
					t.Error("accepted wrong uid")
				}
			}
		}
		done <- e
	}()
	c, e := net.Dial("unix", p)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}
func TestIdentityReplacement(t *testing.T) {
	p := filepath.Join(t.TempDir(), "app")
	os.WriteFile(p, []byte("old"), 0700)
	a, e := Identify(p)
	if e != nil {
		t.Fatal(e)
	}
	os.Rename(p, p+".old")
	os.WriteFile(p, []byte("new"), 0700)
	b, e := Identify(p)
	if e != nil || a == b {
		t.Fatal("replacement not identified")
	}
}

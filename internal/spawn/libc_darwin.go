package spawn

import (
	"os"
	"runtime"
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
)

// From <spawn.h> and <sys/signal.h> on darwin.
const (
	flagSetSigDef      = 0x0004 // POSIX_SPAWN_SETSIGDEF
	flagSetSigMask     = 0x0008 // POSIX_SPAWN_SETSIGMASK
	flagCloexecDefault = 0x4000 // POSIX_SPAWN_CLOEXEC_DEFAULT
	libSystem          = "/usr/lib/libSystem.B.dylib"
	oRdonly, oWronly   = 0x0, 0x1
)

// libc is libSystem's posix_spawn family, bound without cgo. The opaque
// posix_spawnattr_t and posix_spawn_file_actions_t are pointers on darwin;
// sigset_t is a uint32.
var libc struct {
	once sync.Once
	ok   bool

	spawn      func(pid *int32, path *byte, fa, attr *uintptr, argv, envp **byte) int32
	attrInit   func(attr *uintptr) int32
	attrFree   func(attr *uintptr) int32
	attrFlags  func(attr *uintptr, flags int16) int32
	attrMask   func(attr *uintptr, set *uint32) int32
	attrSigDef func(attr *uintptr, set *uint32) int32
	faInit     func(fa *uintptr) int32
	faFree     func(fa *uintptr) int32
	faDup2     func(fa *uintptr, fd, newfd int32) int32
	faOpen     func(fa *uintptr, fd int32, path *byte, oflag int32, mode uint16) int32
	faChdir    func(fa *uintptr, path *byte) int32
}

func loadLibc() bool {
	libc.once.Do(func() {
		h, err := purego.Dlopen(libSystem, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		syms := []struct {
			fn   any
			name string
		}{
			{&libc.spawn, "posix_spawn"},
			{&libc.attrInit, "posix_spawnattr_init"},
			{&libc.attrFree, "posix_spawnattr_destroy"},
			{&libc.attrFlags, "posix_spawnattr_setflags"},
			{&libc.attrMask, "posix_spawnattr_setsigmask"},
			{&libc.attrSigDef, "posix_spawnattr_setsigdefault"},
			{&libc.faInit, "posix_spawn_file_actions_init"},
			{&libc.faFree, "posix_spawn_file_actions_destroy"},
			{&libc.faDup2, "posix_spawn_file_actions_adddup2"},
			{&libc.faOpen, "posix_spawn_file_actions_addopen"},
			{&libc.faChdir, "posix_spawn_file_actions_addchdir_np"},
		}
		for _, s := range syms {
			if _, err := purego.Dlsym(h, s.name); err != nil {
				return
			}
			purego.RegisterLibFunc(s.fn, h, s.name)
		}
		libc.ok = true
	})
	return libc.ok
}

// posixSpawn starts req and returns the child's pid. Only the three stdio
// fds reach the child (POSIX_SPAWN_CLOEXEC_DEFAULT), its signal mask is
// empty and SIGPIPE has its default action, as after Go's fork and exec.
func posixSpawn(req spawnRequest) (int, error) {
	var pin runtime.Pinner
	defer pin.Unpin()
	cstr := func(s string) *byte {
		b, _ := syscall.BytePtrFromString(s) // callers reject NUL bytes
		pin.Pin(b)
		return b
	}
	cvec := func(ss []string) **byte {
		v := make([]*byte, len(ss)+1)
		for i, s := range ss {
			v[i] = cstr(s)
		}
		pin.Pin(&v[0])
		return &v[0]
	}

	var attr, fa uintptr
	if e := libc.attrInit(&attr); e != 0 {
		return 0, syscall.Errno(e)
	}
	defer libc.attrFree(&attr)
	if e := libc.faInit(&fa); e != 0 {
		return 0, syscall.Errno(e)
	}
	defer libc.faFree(&fa)

	empty, sigpipe := uint32(0), uint32(1)<<(uint(syscall.SIGPIPE)-1)
	check := func(e int32) error {
		if e != 0 {
			return syscall.Errno(e)
		}
		return nil
	}
	if err := check(libc.attrFlags(&attr, flagCloexecDefault|flagSetSigMask|flagSetSigDef)); err != nil {
		return 0, err
	}
	if err := check(libc.attrMask(&attr, &empty)); err != nil {
		return 0, err
	}
	if err := check(libc.attrSigDef(&attr, &sigpipe)); err != nil {
		return 0, err
	}
	null := cstr(os.DevNull)
	for child, parent := range req.stdio {
		var e int32
		switch {
		case parent < 0 && child == 0:
			e = libc.faOpen(&fa, int32(child), null, oRdonly, 0)
		case parent < 0:
			e = libc.faOpen(&fa, int32(child), null, oWronly, 0)
		default:
			// dup2 onto itself clears FD_CLOEXEC, so CLOEXEC_DEFAULT keeps it.
			e = libc.faDup2(&fa, int32(parent), int32(child))
		}
		if err := check(e); err != nil {
			return 0, err
		}
	}
	if req.dir != "" {
		if err := check(libc.faChdir(&fa, cstr(req.dir))); err != nil {
			return 0, err
		}
	}

	var pid int32
	e := libc.spawn(&pid, cstr(req.path), &fa, &attr, cvec(req.argv), cvec(req.envv))
	if e != 0 {
		return 0, syscall.Errno(e)
	}
	return int(pid), nil
}

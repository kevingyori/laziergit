package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"

	"laziergit/internal/repo"
	"laziergit/internal/testrepo"
)

// TestCredentialPromptOverHTTPS runs a real git fetch against a server that
// demands Basic auth. git asks for a username and password on lazygit's pty;
// the answers must come from our prompt overlays.
func TestCredentialPromptOverHTTPS(t *testing.T) {
	var mu sync.Mutex
	var gotUser, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		gotUser, gotPass = u, p
		mu.Unlock()
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dir := testrepo.New(t)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GIT_ASKPASS", "")
	t.Setenv("SSH_ASKPASS", "")
	git(t, dir, "remote", "add", "origin", srv.URL+"/repo.git")

	r, err := repo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// Real goroutines for git; main-thread work is queued and pumped here.
	var qmu sync.Mutex
	var queue []func()
	post := func(fn func()) { qmu.Lock(); queue = append(queue, fn); qmu.Unlock() }
	a := New(r, post, func(fn func()) { go fn() }, func() {})
	tt := ui.NewTester(a.View, 1280, 800)
	pump := func() {
		qmu.Lock()
		q := queue
		queue = nil
		qmu.Unlock()
		for _, fn := range q {
			fn()
		}
		tt.Frame()
	}
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s (overlay %T, log %v)", what, a.topOverlay(), a.log)
			}
			pump()
			time.Sleep(10 * time.Millisecond)
		}
	}
	a.Refresh()
	waitFor("snapshot", func() bool { return a.raw.Has(repo.AllParts) })

	press(tt, "f") // fetch, in the files panel
	waitFor("username prompt", func() bool {
		po, ok := a.topOverlay().(*promptOverlay)
		return ok && po.title == "Username"
	})
	if dir := shotDir(); dir != "" {
		shot(t, tt, filepath.Join(dir, "credential-username.png"))
	}
	tt.Type("alice")
	press(tt, "enter")
	waitFor("password prompt", func() bool {
		po, ok := a.topOverlay().(*promptOverlay)
		return ok && po.title == "Password" && po.mask
	})
	if dir := shotDir(); dir != "" {
		shot(t, tt, filepath.Join(dir, "credential-password.png"))
	}
	tt.Type("s3cret")
	press(tt, "enter")
	waitFor("fetch to finish", func() bool { return a.busy == "" && a.topOverlay() != nil })
	mu.Lock()
	defer mu.Unlock()
	if gotUser != "alice" || gotPass != "s3cret" {
		t.Fatalf("server saw %q:%q", gotUser, gotPass)
	}
	if _, ok := a.topOverlay().(*errorOverlay); !ok {
		t.Fatalf("expected the 404 to surface as an error, got %T", a.topOverlay())
	}
}

// Credential requests must remain distinguishable from ordinary text prompts
// while the git operation is still running, including unmasked usernames.
func TestCredentialPromptContract(t *testing.T) {
	dir := testrepo.New(t)
	r, err := repo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	sync := func(fn func()) { fn() }
	a := New(r, sync, sync, func() {})
	tr := r.Common.Tr
	for _, tc := range []struct {
		kind  oscommands.CredentialType
		title string
		mask  bool
	}{
		{oscommands.Username, tr.CredentialsUsername, false},
		{oscommands.Password, tr.CredentialsPassword, true},
		{oscommands.Passphrase, tr.CredentialsPassphrase, true},
		{oscommands.PIN, tr.CredentialsPIN, true},
		{oscommands.Token, tr.CredentialsToken, true},
	} {
		t.Run(tc.title, func(t *testing.T) {
			for _, cancel := range []bool{false, true} {
				ch := make(chan string, 1)
				a.promptCredential(tc.kind, ch)
				o := a.topOverlay().(*promptOverlay)
				if o.title != tc.title || o.mask != tc.mask || !o.credential {
					t.Fatalf("credential prompt: %+v", o)
				}
				want := "answer\n"
				if cancel {
					o.cancel(a)
					want = "\n"
				} else {
					o.onSubmit("answer")
					a.closeOverlay(o)
				}
				select {
				case got := <-ch:
					if got != want {
						t.Fatalf("answer %q, want %q", got, want)
					}
				default:
					t.Fatal("credential answer missing")
				}
			}
		})
	}
	a.showError("Pushing", errors.New("incorrect username/password"))
	o := a.topOverlay().(*errorOverlay)
	if o.title != tr.Error || o.body != "incorrect username/password" {
		t.Fatalf("error alert: %+v", o)
	}
}

#!/bin/sh
# Builds the guide-eval fixture: a small Go service with one committed state
# and an agent-style uncommitted change (staged, unstaged and untracked parts).
# Usage: fixture.sh <dir>
set -eu
dir=$1
rm -rf "$dir"
mkdir -p "$dir/limiter" "$dir/cmd/server" "$dir/config"
cd "$dir"
git init -q -b main
git config user.name eval
git config user.email eval@example.com

cat > go.mod <<'X'
module example.com/gate

go 1.22
X
cat > README.md <<'X'
# gate

A tiny HTTP gateway that rate-limits requests.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `GATE_LIMIT` | 100 | Requests allowed per window |
| `GATE_WINDOW` | 1m | Window length |

## Running

    go run ./cmd/server
X
cat > config/config.go <<'X'
// Package config reads the gateway's settings from the environment.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Limit  int
	Window time.Duration
	Addr   string
}

func Load() Config {
	c := Config{Limit: 100, Window: time.Minute, Addr: ":8080"}
	if v, err := strconv.Atoi(os.Getenv("GATE_LIMIT")); err == nil && v > 0 {
		c.Limit = v
	}
	if v, err := time.ParseDuration(os.Getenv("GATE_WINDOW")); err == nil && v > 0 {
		c.Window = v
	}
	if v := os.Getenv("GATE_ADDR"); v != "" {
		c.Addr = v
	}
	return c
}
X
cat > limiter/limiter.go <<'X'
// Package limiter is a fixed-window rate limiter.
package limiter

import (
	"sync"
	"time"
)

// Limiter allows up to limit calls per window, shared by every caller.
type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	start  time.Time
	count  int
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, start: time.Now()}
}

// Allow reports whether one more call fits in the current window.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.start) >= l.window {
		l.start = now
		l.count = 0
	}
	if l.count >= l.limit {
		return false
	}
	l.count++
	return true
}

// Remaining reports how many calls are left in the current window.
func (l *Limiter) Remaining() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit - l.count
}
X
cat > limiter/limiter_test.go <<'X'
package limiter

import (
	"testing"
	"time"
)

func TestAllowUpToLimit(t *testing.T) {
	l := New(2, time.Hour)
	if !l.Allow() || !l.Allow() {
		t.Fatal("first two calls must pass")
	}
	if l.Allow() {
		t.Fatal("third call must be rejected")
	}
}

func TestRemaining(t *testing.T) {
	l := New(3, time.Hour)
	l.Allow()
	if got := l.Remaining(); got != 2 {
		t.Fatalf("remaining = %d, want 2", got)
	}
}
X
cat > cmd/server/main.go <<'X'
package main

import (
	"fmt"
	"log"
	"net/http"

	"example.com/gate/config"
	"example.com/gate/limiter"
)

func main() {
	cfg := config.Load()
	l := limiter.New(cfg.Limit, cfg.Window)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "healthy")
	})

	log.Printf("listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, nil))
}
X
git add -A
git commit -q -m "Add gateway with a global fixed-window rate limiter"

# --- the agent's change: limit per client, tell clients when to retry ---
cat > limiter/clock.go <<'X'
package limiter

import "time"

// Clock lets tests control time.
type Clock func() time.Time
X
cat > limiter/limiter.go <<'X'
// Package limiter is a fixed-window rate limiter.
package limiter

import (
	"sync"
	"time"
)

type window struct {
	start time.Time
	count int
}

// Limiter allows up to limit calls per window for each key.
type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     Clock
	windows map[string]*window
}

func New(limit int, window time.Duration) *Limiter {
	return NewWithClock(limit, window, time.Now)
}

func NewWithClock(limit int, every time.Duration, now Clock) *Limiter {
	return &Limiter{limit: limit, window: every, now: now, windows: map[string]*window{}}
}

// Allow reports whether one more call from key fits in its current window,
// and if not, how long until the window resets.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w := l.windows[key]
	if w == nil || now.Sub(w.start) >= l.window {
		w = &window{start: now}
		l.windows[key] = w
	}
	if w.count >= l.limit {
		return false, l.window - now.Sub(w.start)
	}
	w.count++
	return true, 0
}

// Remaining reports how many calls key has left in its current window.
func (l *Limiter) Remaining(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.windows[key]
	if w == nil || l.now().Sub(w.start) >= l.window {
		return l.limit
	}
	return l.limit - w.count
}
X
cat > limiter/limiter_test.go <<'X'
package limiter

import (
	"testing"
	"time"
)

func TestAllowUpToLimit(t *testing.T) {
	l := New(2, time.Hour)
	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatal("first two calls must pass")
		}
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("third call must be rejected")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(1, time.Hour)
	l.Allow("a")
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("b must not be limited by a")
	}
}

func TestRetryAfterCountsDown(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewWithClock(1, time.Minute, func() time.Time { return now })
	l.Allow("a")
	now = now.Add(20 * time.Second)
	ok, wait := l.Allow("a")
	if ok || wait != 40*time.Second {
		t.Fatalf("got %v, %v; want false, 40s", ok, wait)
	}
}

func TestRemaining(t *testing.T) {
	l := New(3, time.Hour)
	l.Allow("a")
	if got := l.Remaining("a"); got != 2 {
		t.Fatalf("remaining = %d, want 2", got)
	}
}
X
cat > config/config.go <<'X'
// Package config reads the gateway's settings from the environment.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Limit      int
	Window     time.Duration
	Addr       string
	TrustProxy bool
}

func Load() Config {
	c := Config{Limit: 100, Window: time.Minute, Addr: ":8080", TrustProxy: true}
	if v, err := strconv.Atoi(os.Getenv("GATE_LIMIT")); err == nil && v > 0 {
		c.Limit = v
	}
	if v, err := time.ParseDuration(os.Getenv("GATE_WINDOW")); err == nil && v > 0 {
		c.Window = v
	}
	if v := os.Getenv("GATE_ADDR"); v != "" {
		c.Addr = v
	}
	if v, err := strconv.ParseBool(os.Getenv("GATE_TRUST_PROXY")); err == nil {
		c.TrustProxy = v
	}
	return c
}
X
git add limiter config
git reset -q limiter/clock.go
cat > cmd/server/main.go <<'X'
package main

import (
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"example.com/gate/config"
	"example.com/gate/limiter"
)

// clientKey names the caller a request is limited as.
func clientKey(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func main() {
	cfg := config.Load()
	limit := limiter.New(cfg.Limit, cfg.Window)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r, cfg.TrustProxy)
		ok, wait := limit.Allow(key)
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(limit.Remaining(key)))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "healthy")
	})

	log.Printf("listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, nil))
}
X
cat > README.md <<'X'
# gate

A tiny HTTP gateway that rate-limits requests per client.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `GATE_LIMIT` | 100 | Requests allowed per client per window |
| `GATE_WINDOW` | 1m | Window length |
| `GATE_TRUST_PROXY` | true | Take the client address from `X-Forwarded-For` |

Rejected requests get `429` with a `Retry-After` header in seconds.

## Running

    go run ./cmd/server
X

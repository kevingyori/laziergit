// laziergit is lazygit as a desktop GUI.
package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/app/daemon"
	"github.com/jesseduffield/lazygit/pkg/logs"

	"golang.org/x/term"
	"laziergit/internal/app"
	"laziergit/internal/repo"
	"laziergit/internal/reviewbridge"
	"laziergit/internal/spare"
)

// openSettings opens the settings window once a repository is open.
var openSettings func()

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--spare" {
		runSpare()
		return
	}
	detached := len(os.Args) > 1 && os.Args[1] == "--window"
	if detached {
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	argv := append([]string(nil), os.Args...)

	if len(os.Args) >= 2 && os.Args[1] == "--review-mcp" {
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		// Resolved per call: the server may start before the guide is opened.
		err := reviewbridge.ServeReview(func() (string, error) { return reviewbridge.ResolveReview(dir) }, os.Stdin, os.Stdout)
		if err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) == 3 && os.Args[1] == "--guide-mcp" {
		if err := reviewbridge.Serve(os.Args[2], os.Stdin, os.Stdout); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}

	if handleCLIInstall() {
		return
	}
	if daemon.InDaemonMode() {
		daemon.Handle(logs.NewProductionLogger())
		return
	}
	guide := false
	for i := 1; i < len(os.Args); {
		if os.Args[i] == "--guide" {
			guide = true
			os.Args = append(os.Args[:i], os.Args[i+1:]...)
		} else {
			i++
		}
	}
	options, err := app.ParseCLI(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if handleCLIOutput(options) {
		return
	}
	dir, _ := os.Getwd()
	dev := os.Getenv("LAZIERGIT_CAPTURE") != "" || os.Getenv("LAZIERGIT_KEYS") != ""
	if !detached && !dev {
		id, err := spare.Executable()
		if err == nil {
			if socket, e := spare.Socket(id); e == nil {
				state, e := spare.Send(socket, spare.Request{Identity: id, Dir: dir, Args: argv, Env: os.Environ()})
				if e == nil && state == spare.Claimed {
					return
				}
			}
			// Finder/LaunchServices already supplies a detached application lifecycle.
			if term.IsTerminal(int(os.Stdin.Fd())) || term.IsTerminal(int(os.Stdout.Fd())) || term.IsTerminal(int(os.Stderr.Fd())) {
				if err := spare.Spawn(id, append([]string{"--window"}, argv[1:]...), os.Environ(), dir); err == nil {
					return
				} else {
					log.Print(err)
				}
			}
		}
	}
	runWindow(dir, options, guide, nil)
}

func runWindow(dir string, options *app.CLIOptions, guide bool, warmed *prewarmed) {
	var r *repo.Repo
	var current *app.App
	mygo.App.OnQuit(func() {
		if current != nil {
			current.Close()
		} else if r != nil {
			r.Close()
		}
	})

	// Git starts while AppKit launches: opening the repository and loading
	// its first snapshot overlap the app's startup instead of following it.
	early := make(chan earlyRepo, 1)
	go func() { early <- openEarly(dir, options) }()

	if warmed == nil {
		mygo.Use(native.Plugin)
	}
	ready := func() {
		if warmed != nil {
			if err := warmed.activate(); err != nil {
				log.Print(err)
				mygo.App.Quit()
				return
			}
		}
		setupApplicationMenu()
		go func() {
			pre := <-early
			opened, showRecent := startupRepository(pre.r, pre.err, options)
			if opened == nil {
				mygo.App.Quit()
				return
			}
			mygo.RunOnMain(func() {
				r = opened
				var win *mygo.Window
				a := app.NewWithCLI(r, options,
					func(fn func()) { win.Update(fn) },
					func(fn func()) { go fn() },
					mygo.App.Quit,
				)
				current = a
				win = mygo.NewWindow(mygo.WindowOptions{
					Title:     "laziergit",
					Width:     1280,
					Height:    800,
					MinWidth:  720,
					MinHeight: 420,
					StateKey:  "main",
					Content:   ui.View(a.View),
				})
				if warmed != nil {
					// Destroyed only once the real window exists: closing the
					// last window quits the app.
					warmed.fonts.Destroy()
					mygo.App.Focus()
				}
				openSettings = func() { win.Update(a.OpenSettings) }
				// Activation refreshes and the next spare wait for the first load:
				// activating this window would otherwise start a second load, and
				// a spare launching now would compete with this window for git.
				loaded := func() {
					startRefresher(r, win, a)
					go refillSpare()
				}
				if pre.r == opened {
					a.RefreshFrom(func() <-chan repo.Part { return pre.parts }, loaded)
				} else {
					a.RefreshFrom(opened.Stream, loaded)
				}
				if guide {
					a.OpenGuide()
				}
				if showRecent {
					a.OpenRecentRepos()
				}
				if out := os.Getenv("LAZIERGIT_CAPTURE"); out != "" {
					go captureAndQuit(win, a, out)
				}
			})
		}()
	}
	if warmed != nil {
		mygo.RunOnMain(ready)
		return
	}
	mygo.App.WhenReady(ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// captureAndQuit writes a PNG of the real, GPU-rendered window and quits:
// a dev hook for verifying the UI from scripts.
func captureAndQuit(win *mygo.Window, a *app.App, out string) {
	time.Sleep(time.Second)
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		loading := make(chan bool, 1)
		win.Update(func() { loading <- a.GuideLoading() })
		if !<-loading {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if keys := os.Getenv("LAZIERGIT_KEYS"); keys != "" {
		// Space-separated lazygit key labels, replayed before the capture.
		for _, k := range strings.Fields(keys) {
			win.Update(func() { a.Press(k) })
			time.Sleep(150 * time.Millisecond)
		}
	}
	if delay, err := time.ParseDuration(os.Getenv("LAZIERGIT_CAPTURE_DELAY")); err == nil && delay > 0 {
		time.Sleep(delay)
	}
	time.Sleep(time.Second)
	png, err := win.CapturePage()
	if err != nil {
		log.Print(err)
	} else if err := os.WriteFile(out, png, 0o644); err != nil {
		log.Print(err)
	}
	mygo.App.Quit()
}

// startRefresher keeps the snapshot fresh the way lazygit does: on
// git.autoRefresh every refresher.refreshInterval, on git.autoFetch every
// refresher.fetchInterval, and whenever the app becomes active again.
func startRefresher(r *repo.Repo, win *mygo.Window, a *app.App) {
	mygo.App.OnDidBecomeActive(func() { win.Update(a.FocusIn) })
	cfg := r.Config
	if cfg.Git.AutoRefresh && cfg.Refresher.RefreshInterval > 0 {
		go func() {
			for range time.Tick(cfg.Refresher.RefreshIntervalDuration()) {
				win.Update(a.Refresh)
			}
		}()
	}
	if cfg.Git.AutoFetch && cfg.Refresher.FetchInterval > 0 {
		go func() {
			for range time.Tick(cfg.Refresher.FetchIntervalDuration()) {
				win.Update(a.BackgroundFetch)
			}
		}()
	}
}

// Finder launches have no repository working directory. Let the user choose
// one instead of exiting before the application menu becomes available.
// Called off the UI thread, since opening a repository runs git commands.
func startupRepository(r *repo.Repo, err error, options *app.CLIOptions) (*repo.Repo, bool) {
	if err == nil {
		return r, false
	}
	if options.RepoPath != "" || options.WorkTree != "" || options.GitDir != "" {
		mygo.Dialog.Error("Could not open repository", err.Error())
		return nil, false
	}
	// lazygit opened outside a repo: the most recent repo that still exists,
	// with the recent repositories menu showing.
	for _, p := range repo.RecentRepos() {
		if r, err := repo.Open(p); err == nil {
			return r, true
		}
	}
	for {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title: "Open Git Repository", Directory: true, ButtonLabel: "Open",
		})
		if err != nil || len(paths) == 0 {
			return nil, false
		}
		r, err = repo.Open(paths[0])
		if err == nil {
			return r, false
		}
		mygo.Dialog.Error("Could not open repository", err.Error())
	}
}

// earlyRepo buffers the startup repository's first refresh before the window.
type earlyRepo struct {
	r     *repo.Repo
	err   error
	parts <-chan repo.Part
}

func openEarly(dir string, options *app.CLIOptions) earlyRepo {
	r, err := options.Open(dir, nil)
	if err != nil {
		return earlyRepo{err: err}
	}
	r.SetFilter(options.FilterPath, "")
	return earlyRepo{r: r, parts: r.Stream()}
}

// laziergit is lazygit as a desktop GUI.
package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/app/daemon"
	"github.com/jesseduffield/lazygit/pkg/logs"

	"laziergit/internal/app"
	"laziergit/internal/repo"
	"laziergit/internal/reviewbridge"
)

func main() {
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
	dir, _ := os.Getwd()
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	var r *repo.Repo
	var current *app.App
	defer func() {
		if current != nil {
			current.Close()
		} else if r != nil {
			r.Close()
		}
	}()

	mygo.App.WhenReady(func() {
		setupApplicationMenu()
		go func() {
			opened, showRecent := startupRepository(dir, len(os.Args) > 1)
			if opened == nil {
				mygo.App.Quit()
				return
			}
			mygo.RunOnMain(func() {
				r = opened
				var win *mygo.Window
				a := app.New(r,
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
				a.Refresh()
				if guide {
					a.OpenGuide()
				}
				if showRecent {
					a.OpenRecentRepos()
				}
				startRefresher(r, win, a)
				if out := os.Getenv("LAZIERGIT_CAPTURE"); out != "" {
					go captureAndQuit(win, a, out)
				}
			})
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// captureAndQuit writes a PNG of the real, GPU-rendered window and quits:
// a dev hook for verifying the UI from scripts.
func captureAndQuit(win *mygo.Window, a *app.App, out string) {
	time.Sleep(time.Second)
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
	// Let AI-driven captures show the completed guide rather than the spinner.
	for deadline := time.Now().Add(125 * time.Second); time.Now().Before(deadline); {
		pending := make(chan bool, 1)
		win.Update(func() { pending <- a.GuideGenerating() })
		if !<-pending {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
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
	mygo.App.OnDidBecomeActive(func() { win.Update(a.Refresh) })
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
func startupRepository(dir string, explicit bool) (*repo.Repo, bool) {
	r, err := repo.Open(dir)
	if err == nil {
		return r, false
	}
	if explicit {
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

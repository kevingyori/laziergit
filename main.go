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
)

func main() {
	if handleCLIInstall() {
		return
	}
	if daemon.InDaemonMode() {
		daemon.Handle(logs.NewProductionLogger())
		return
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
			opened := startupRepository(dir, len(os.Args) > 1)
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
func startupRepository(dir string, explicit bool) *repo.Repo {
	r, err := repo.Open(dir)
	if err == nil {
		return r
	}
	if explicit {
		mygo.Dialog.Error("Could not open repository", err.Error())
		return nil
	}
	for {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title: "Open Git Repository", Directory: true, ButtonLabel: "Open",
		})
		if err != nil || len(paths) == 0 {
			return nil
		}
		r, err = repo.Open(paths[0])
		if err == nil {
			return r
		}
		mygo.Dialog.Error("Could not open repository", err.Error())
	}
}

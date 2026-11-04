// laziergit is lazygit as a desktop GUI.
package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"laziergit/internal/app"
	"laziergit/internal/repo"
)

func main() {
	dir, _ := os.Getwd()
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	r, err := repo.Open(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	mygo.App.WhenReady(func() {
		var win *mygo.Window
		a := app.New(r,
			func(fn func()) { win.Update(fn) },
			func(fn func()) { go fn() },
			mygo.App.Quit,
		)
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

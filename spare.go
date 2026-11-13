package main

import (
	"log"
	"os"

	"github.com/adrg/xdg"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	"laziergit/internal/app"
	"laziergit/internal/spare"
)

// prewarmed is what a spare process readies before a launch claims it.
type prewarmed struct {
	// fonts draws each panel font once while hidden. It is not the window
	// the launch shows: a hidden window draws only its first frame, so
	// showing it would flash that frame until the real one draws.
	fonts    *mygo.Window
	activate func() error
}

func runSpare() {
	id, err := spare.Executable()
	if err != nil {
		return
	}
	socket, err := spare.Socket(id)
	if err != nil {
		return
	}
	listener, err := spare.Listen(socket)
	if err != nil {
		return
	}
	defer listener.Close()
	// Register services before Run, but delay updater timers and UI until use.
	plugin := native.Plugin
	setup := plugin.Setup
	plugin.Setup = nil
	mygo.Use(plugin)
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	warmed := prewarmed{activate: func() error {
		if err := setup(); err != nil {
			return err
		}
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
		return nil
	}}
	mygo.App.WhenReady(func() {
		warmed.fonts = mygo.NewWindow(mygo.WindowOptions{Width: 400, Height: 300, Hidden: true, Content: ui.View(app.Warm)})
		go func() {
			req, state, err := spare.Receive(listener, id)
			if err != nil || state != spare.Claimed {
				mygo.App.Quit()
				return
			}
			if err := req.Apply(); err != nil {
				log.Print(err)
				mygo.App.Quit()
				return
			}
			// lazygit uses XDG globals initialized before the terminal hand-off.
			xdg.Reload()
			args := make([]string, 0, len(req.Args))
			guide := false
			for _, arg := range req.Args[1:] {
				if arg == "--guide" {
					guide = true
				} else {
					args = append(args, arg)
				}
			}
			options, err := app.ParseCLI(args)
			if err != nil {
				log.Print(err)
				mygo.App.Quit()
				return
			}
			if handleCLIOutput(options) {
				mygo.App.Quit()
				return
			}
			runWindow(req.Dir, options, guide, &warmed)
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Print(err)
	}
}

func refillSpare() {
	if os.Getenv("LAZIERGIT_CAPTURE") != "" || os.Getenv("LAZIERGIT_KEYS") != "" {
		return
	}
	id, err := spare.Executable()
	if err != nil {
		return
	}
	socket, err := spare.Socket(id)
	if err != nil || spare.Available(socket) {
		return
	}
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	env := os.Environ()
	if err := spare.Spawn(id, []string{"--spare"}, env, dir); err != nil {
		log.Print(err)
	}
}

// laziergit is lazygit as a desktop GUI.
package main

import (
	"log"
	"os"

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
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"laziergit/internal/cliinstall"
)

// Handle the elevated helper before opening a repository or starting the UI.
func handleCLIInstall() bool {
	if len(os.Args) != 2 || os.Args[1] != "--install-cli" {
		return false
	}
	executable, err := os.Executable()
	if err == nil {
		executable, err = cliinstall.BundleExecutable(executable)
	}
	if err == nil {
		err = cliinstall.Install(executable, cliinstall.Destination)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return true
}

func setupApplicationMenu() {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, updater.MenuItem(), mygo.Separator(),
			{Label: "Settings…", Accelerator: "Cmd+,", Click: func(*mygo.MenuItem, *mygo.Window) {
				if openSettings != nil {
					openSettings()
				}
			}},
			{Label: "Install Command Line Tool…", Accelerator: "Cmd+Shift+L", Click: func(_ *mygo.MenuItem, win *mygo.Window) {
				go installCommandLineTool(win)
			}},
			mygo.Separator(), {Role: mygo.RoleServices}, mygo.Separator(),
			{Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide},
			mygo.Separator(), {Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleFileMenu}, {Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu}, {Role: mygo.RoleWindowMenu},
	}))
}

func installCommandLineTool(win *mygo.Window) {
	executable, err := os.Executable()
	if err == nil {
		executable, err = cliinstall.BundleExecutable(executable)
	}
	if err == nil {
		var result mygo.MessageResult
		result, err = mygo.Dialog.Message(mygo.MessageOptions{
			Parent: win, Type: mygo.MessageQuestion, Message: "Install the laziergit command?",
			Detail:  "Creates " + cliinstall.Destination + ". macOS may ask for an administrator password.",
			Buttons: []string{"Install", "Cancel"}, CancelButton: 1,
		})
		if err == nil && result.Button != 0 {
			return
		}
	}
	if err == nil {
		err = cliinstall.Install(executable, cliinstall.Destination)
		if errors.Is(err, os.ErrPermission) {
			// Quote independently for the shell and for AppleScript. The helper
			// accepts no destination argument and never overwrites another command.
			command := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "' --install-cli"
			script := "do shell script " + appleScriptString(command) + " with administrator privileges"
			output, runErr := exec.Command("/usr/bin/osascript", "-e", script).CombinedOutput()
			if runErr != nil {
				err = fmt.Errorf("installation did not complete: %s", strings.TrimSpace(string(output)))
			} else {
				err = nil
			}
		}
	}
	if err != nil {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Type: mygo.MessageError,
			Message: "Could not install the command", Detail: err.Error()})
		return
	}
	_, _ = mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Type: mygo.MessageInfo,
		Message: "Command line tool installed",
		Detail:  "Run laziergit in a Git repository, or laziergit /path/to/repo.\nIf your shell cannot find it, add /usr/local/bin to your PATH. Keep the app in its current location."})
}

func appleScriptString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`).Replace(s) + `"`
}

package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/jesseduffield/lazygit/pkg/constants"
)

// dashboard is lazygit's StatusController.showDashboard: the status panel's
// main view while gui.statusPanelView is "dashboard" (the default). The app
// has no lazygit version of its own, so the docs links name "master", as
// lazygit's do when its version can't be parsed.
func dashboard() string {
	versionStr := "master"
	return strings.Join([]string{
		lazygitTitle,
		fmt.Sprintf("Copyright %d Jesse Duffield", time.Now().Year()),
		fmt.Sprintf("Keybindings: %s", fmt.Sprintf(constants.Links.Docs.Keybindings, versionStr)),
		fmt.Sprintf("Config Options: %s", fmt.Sprintf(constants.Links.Docs.Config, versionStr)),
		fmt.Sprintf("Tutorial: %s", constants.Links.Docs.Tutorial),
		fmt.Sprintf("Raise an Issue: %s", constants.Links.Issues),
		fmt.Sprintf("Release Notes: %s", constants.Links.Releases),
		fmt.Sprintf("Become a sponsor: %s", constants.Links.Donate),
	}, "\n\n") + "\n"
}

const lazygitTitle = `
   _                       _ _
  | |                     (_) |
  | | __ _ _____   _  __ _ _| |_
  | |/ _` + "`" + ` |_  / | | |/ _` + "`" + ` | | __|
  | | (_| |/ /| |_| | (_| | | |_
  |_|\__,_/___|\__, |\__, |_|\__|
                __/ | __/ |
               |___/ |___/       `

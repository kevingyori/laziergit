package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/logs/tail"
	"gopkg.in/yaml.v3"
	"laziergit/internal/app"
)

// Non-window CLI operations use the same lazygit config and log locations.
func handleCLIOutput(o *app.CLIOptions) bool {
	if o.UseConfigDir != "" {
		os.Setenv("CONFIG_DIR", o.UseConfigDir)
	}
	if o.CustomConfigFile != "" {
		os.Setenv("LG_CONFIG_FILE", o.CustomConfigFile)
	}
	switch {
	case o.HelpText != "":
		fmt.Print(o.HelpText)
	case o.PrintVersionInfo:
		fmt.Printf("version=laziergit, os=%s, arch=%s\n", runtime.GOOS, runtime.GOARCH)
	case o.PrintDefaultConfig:
		if err := yaml.NewEncoder(os.Stdout).Encode(config.GetDefaultConfigForPlatform(config.KeybindingPlatform())); err != nil {
			panic(err)
		}
	case o.PrintConfigDir:
		fmt.Println(config.ConfigDir())
	case o.TailLogs:
		path, err := config.LogPath()
		if err != nil {
			panic(err)
		}
		tail.TailLogs(path)
	default:
		if o.Profile {
			go func() { _ = http.ListenAndServe("localhost:6060", nil) }()
		}
		return false
	}
	return true
}

package app

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/env"
	"laziergit/internal/repo"
)

// CLIOptions mirrors lazygit's entry point. Its parser is private upstream.
type CLIOptions struct {
	HelpText           string
	RepoPath           string
	FilterPath         string
	GitArg             string
	UseConfigDir       string
	WorkTree           string
	GitDir             string
	CustomConfigFile   string
	ScreenMode         string
	PrintVersionInfo   bool
	Debug              bool
	TailLogs           bool
	Profile            bool
	PrintDefaultConfig bool
	PrintConfigDir     bool
}

// ParseCLI parses startup arguments without opening a window or changing process state.
func ParseCLI(args []string) (*CLIOptions, error) {
	flags := flag.NewFlagSet("laziergit", flag.ContinueOnError)
	var output bytes.Buffer
	flags.SetOutput(&output)
	repoPath := ""
	flags.StringVar(&repoPath, "p", repoPath, "Path of git repo. (equivalent to --work-tree=<path> --git-dir=<path>/.git/)")
	flags.StringVar(&repoPath, "path", repoPath, "Path of git repo. (equivalent to --work-tree=<path> --git-dir=<path>/.git/)")

	filterPath := ""
	flags.StringVar(&filterPath, "f", filterPath, "Path to filter on in `git log -- <path>`. When in filter mode, the commits, reflog, and stash are filtered based on the given path, and some operations are restricted")
	flags.StringVar(&filterPath, "filter", filterPath, "Path to filter on in `git log -- <path>`. When in filter mode, the commits, reflog, and stash are filtered based on the given path, and some operations are restricted")

	gitArg := ""

	printVersionInfo := false
	flags.BoolVar(&printVersionInfo, "v", printVersionInfo, "Print the current version")
	flags.BoolVar(&printVersionInfo, "version", printVersionInfo, "Print the current version")

	debug := false
	flags.BoolVar(&debug, "d", debug, "Run in debug mode with logging (see --logs flag below). Use the LOG_LEVEL env var to set the log level (debug/info/warn/error)")
	flags.BoolVar(&debug, "debug", debug, "Run in debug mode with logging (see --logs flag below). Use the LOG_LEVEL env var to set the log level (debug/info/warn/error)")

	tailLogs := false
	flags.BoolVar(&tailLogs, "l", tailLogs, "Tail lazygit logs (intended to be used when `lazygit --debug` is called in a separate terminal tab)")
	flags.BoolVar(&tailLogs, "logs", tailLogs, "Tail lazygit logs (intended to be used when `lazygit --debug` is called in a separate terminal tab)")

	profile := false
	flags.BoolVar(&profile, "profile", profile, "Start the profiler and serve it on http port 6060. See CONTRIBUTING.md for more info.")

	printDefaultConfig := false
	flags.BoolVar(&printDefaultConfig, "c", printDefaultConfig, "Print the default config")
	flags.BoolVar(&printDefaultConfig, "config", printDefaultConfig, "Print the default config")

	printConfigDir := false
	flags.BoolVar(&printConfigDir, "cd", printConfigDir, "Print the config directory")
	flags.BoolVar(&printConfigDir, "print-config-dir", printConfigDir, "Print the config directory")

	useConfigDir := ""
	flags.StringVar(&useConfigDir, "ucd", useConfigDir, "override default config directory with provided directory")
	flags.StringVar(&useConfigDir, "use-config-dir", useConfigDir, "override default config directory with provided directory")

	workTree := os.Getenv("GIT_WORK_TREE")
	flags.StringVar(&workTree, "w", workTree, "equivalent of the --work-tree git argument")
	flags.StringVar(&workTree, "work-tree", workTree, "equivalent of the --work-tree git argument")

	gitDir := os.Getenv("GIT_DIR")
	flags.StringVar(&gitDir, "g", gitDir, "equivalent of the --git-dir git argument")
	flags.StringVar(&gitDir, "git-dir", gitDir, "equivalent of the --git-dir git argument")

	customConfigFile := ""
	flags.StringVar(&customConfigFile, "ucf", customConfigFile, "Comma separated list to custom config file(s)")
	flags.StringVar(&customConfigFile, "use-config-file", customConfigFile, "Comma separated list to custom config file(s)")

	screenMode := ""
	flags.StringVar(&screenMode, "sm", screenMode, "The initial screen-mode, which determines the size of the focused panel. Valid options: 'normal' (default), 'half', 'full'")
	flags.StringVar(&screenMode, "screen-mode", screenMode, "The initial screen-mode, which determines the size of the focused panel. Valid options: 'normal' (default), 'half', 'full'")

	// Parse flags on either side of the optional panel argument.
	for len(args) > 0 {
		if err := flags.Parse(args); err != nil {
			if err == flag.ErrHelp {
				return &CLIOptions{HelpText: "laziergit [flags] [status|branch|log|stash]\n" + output.String()}, nil
			}
			return nil, err
		}
		args = flags.Args()
		if len(args) == 0 {
			break
		}
		if gitArg != "" {
			return nil, fmt.Errorf("unexpected argument: %s", args[0])
		}
		gitArg, args = args[0], args[1:]
	}
	switch gitArg {
	case "", "status", "branch", "log", "stash":
	default:
		// Retain laziergit's existing positional repository path.
		if info, err := os.Stat(gitArg); err == nil && info.IsDir() && repoPath == "" {
			repoPath, gitArg = gitArg, ""
		} else {
			return nil, fmt.Errorf("Invalid git arg value: '%s'. Must be one of the following values: status, branch, log, stash. e.g. 'lazygit status'. See 'lazygit --help'.", gitArg)
		}
	}
	if repoPath != "" && (workTree != "" || gitDir != "") {
		return nil, fmt.Errorf("--path option is incompatible with the --work-tree and --git-dir options")
	}

	if os.Getenv("DEBUG") == "TRUE" {
		debug = true
	}

	return &CLIOptions{
		RepoPath:           repoPath,
		FilterPath:         filterPath,
		GitArg:             gitArg,
		PrintVersionInfo:   printVersionInfo,
		Debug:              debug,
		TailLogs:           tailLogs,
		Profile:            profile,
		PrintDefaultConfig: printDefaultConfig,
		PrintConfigDir:     printConfigDir,
		UseConfigDir:       useConfigDir,
		WorkTree:           workTree,
		GitDir:             gitDir,
		CustomConfigFile:   customConfigFile,
		ScreenMode:         screenMode,
	}, nil
}

// Open opens the startup repository off the UI thread. Like lazygit, startup
// sets process-wide cwd and git location variables; repo switches preserve them.
func (o *CLIOptions) Open(dir string, setup func(*config.AppConfig)) (*repo.Repo, error) {
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}
	if o.RepoPath != "" {
		absolute, err := filepath.Abs(o.RepoPath)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(absolute, ".git")); err != nil {
			return nil, fmt.Errorf("%s is not a valid git repository.", absolute)
		}
		dir = absolute
		env.SetGitDirEnv(filepath.Join(absolute, ".git"))
	} else {
		if o.WorkTree != "" {
			env.SetWorkTreeEnv(o.WorkTree)
			dir = o.WorkTree
		}
		if o.GitDir != "" {
			env.SetGitDirEnv(o.GitDir)
		}
	}
	if o.CustomConfigFile != "" {
		os.Setenv("LG_CONFIG_FILE", o.CustomConfigFile)
	}
	if o.UseConfigDir != "" {
		os.Setenv("CONFIG_DIR", o.UseConfigDir)
	}
	return repo.OpenWithConfigDebug(dir, setup, o.Debug)
}

// NewWithCLI applies startup state before the first snapshot is loaded.
func NewWithCLI(r *repo.Repo, o *CLIOptions, post, bg func(func()), quit func()) *App {
	r.SetFilter(o.FilterPath, "")
	a := New(r, post, bg, quit)
	if o.ScreenMode != "" {
		a.screenMode = parseScreenMode(o.ScreenMode)
	} else if o.FilterPath != "" || o.GitArg != "" {
		a.screenMode = screenHalf
	}
	if o.FilterPath != "" {
		a.focus = panelCommits
	} else {
		switch o.GitArg {
		case "status":
			a.focus = panelFiles
		case "branch":
			a.focus = panelBranches
		case "log":
			a.focus = panelCommits
		case "stash":
			a.focus = panelStash
		}
	}
	return a
}

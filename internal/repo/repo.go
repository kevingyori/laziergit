// Package repo boots lazygit's git layer (pkg/commands) without its TUI and
// loads immutable snapshots of repository state for the UI to render.
package repo

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/jesseduffield/lazygit/pkg/commands"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/git_config"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/common"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/i18n"
	"github.com/jesseduffield/lazygit/pkg/logs"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/spf13/afero"

	"laziergit/internal/graph"
)

// Repo is an open repository: lazygit's command layer plus the user's
// lazygit config.
type Repo struct {
	Git     *commands.GitCommand
	Common  *common.Common
	OS      *oscommands.OSCommand
	Config  *config.UserConfig
	tempDir string

	mu       sync.Mutex // serialises git writes against snapshot loads
	hashPool *utils.StringPool
	mainBr   *git_commands.MainBranches

	hooksMu sync.Mutex
	hooks   Hooks
}

// Hooks connect lazygit's command runner to the UI. They are called from
// background goroutines.
type Hooks struct {
	// LogCommand receives every command lazygit runs (for the command log).
	LogCommand func(cmd string, isCommandLine bool)
	// PromptCredential asks the user for a username/password/passphrase that
	// a running git command requested; the answer is sent on the channel.
	PromptCredential func(kind oscommands.CredentialType) <-chan string
}

// SetHooks installs UI hooks. Safe to call at any time.
func (r *Repo) SetHooks(h Hooks) {
	r.hooksMu.Lock()
	r.hooks = h
	r.hooksMu.Unlock()
}

func (r *Repo) getHooks() Hooks {
	r.hooksMu.Lock()
	defer r.hooksMu.Unlock()
	return r.hooks
}

// Open boots lazygit's git layer for the repository containing dir. It reads
// the user's lazygit config (LG_CONFIG_FILE or the standard config dir).
func Open(dir string) (*Repo, error) {
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "laziergit-*")
	if err != nil {
		return nil, err
	}
	appConfig, err := config.NewAppConfig("lazygit", "laziergit", "", "", "laziergit", false, tempDir)
	if err != nil {
		return nil, fmt.Errorf("loading lazygit config: %w", err)
	}
	userConfig := appConfig.GetUserConfig()
	cmn := &common.Common{
		Log:      logs.NewProductionLogger(),
		Tr:       i18n.EnglishTranslationSet(),
		AppState: appConfig.GetAppState(),
		Fs:       afero.NewOsFs(),
	}
	cmn.SetUserConfig(userConfig)

	r := &Repo{Common: cmn, Config: userConfig, tempDir: tempDir, hashPool: &utils.StringPool{}}
	guiIO := oscommands.NewGuiIO(cmn.Log,
		func(cmd string, isCommandLine bool) {
			if h := r.getHooks(); h.LogCommand != nil {
				h.LogCommand(cmd, isCommandLine)
			}
		},
		func() io.Writer { return io.Discard },
		func(kind oscommands.CredentialType) <-chan string {
			if h := r.getHooks(); h.PromptCredential != nil {
				return h.PromptCredential(kind)
			}
			return nil // kills the process, as lazygit does without a prompt
		})
	osCmd := oscommands.NewOSCommand(cmn, appConfig, oscommands.GetPlatform(), guiIO)
	version, err := git_commands.GetGitVersion(osCmd)
	if err != nil {
		return nil, fmt.Errorf("git version: %w", err)
	}
	gitCmd, err := commands.NewGitCommand(cmn, version, osCmd,
		git_config.NewStdCachedGitConfig(cmn.Log),
		config.NewDiffRendererConfigManager(cmn.UserConfig))
	if err != nil {
		return nil, err
	}
	r.Git, r.OS = gitCmd, osCmd
	r.mainBr = git_commands.NewMainBranches(cmn, osCmd.Cmd)
	return r, nil
}

func (r *Repo) Close() { os.RemoveAll(r.tempDir) }

// Snapshot is one consistent read of the repository. Panels render from a
// single snapshot so they never disagree with each other.
type Snapshot struct {
	Root     string
	Head     *models.Branch
	Files    []*models.File
	Remotes  []*models.Remote
	Tags     []*models.Tag
	Branches []*models.Branch
	Commits  []*models.Commit
	Stash    []*models.StashEntry
	Graph    []graph.Row // one per commit
	Errors   []error
}

// Load reads the full snapshot. Independent loads run in parallel.
func (r *Repo) Load() *Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &Snapshot{Root: r.Git.RepoPaths.WorktreePath()}
	var wg sync.WaitGroup
	var emu sync.Mutex
	fail := func(err error) {
		if err != nil {
			emu.Lock()
			s.Errors = append(s.Errors, err)
			emu.Unlock()
		}
	}
	wg.Go(func() {
		s.Files = r.Git.Loaders.FileLoader.GetStatusFiles(git_commands.GetStatusFileOptions{})
	})
	wg.Go(func() {
		s.Stash = r.Git.Loaders.StashLoader.GetStashEntries("")
	})
	wg.Go(func() {
		reflog, _, err := r.Git.Loaders.ReflogCommitLoader.GetReflogCommits(r.hashPool, nil, "", "")
		fail(err)
		branches, err := r.Git.Loaders.BranchLoader.Load(reflog, r.mainBr, nil, false,
			func(f func() error) { fail(f()) }, func() {})
		fail(err)
		s.Branches = branches
		for _, b := range branches {
			if b.Head {
				s.Head = b
			}
		}
		var ref models.Ref
		if s.Head != nil {
			ref = s.Head
		}
		commits, err := r.Git.Loaders.CommitLoader.GetCommits(git_commands.GetCommitsOptions{
			Limit:                true,
			IncludeRebaseCommits: true,
			RefName:              "HEAD",
			RefForPushedStatus:   ref,
			MainBranches:         r.mainBr,
			HashPool:             r.hashPool,
		})
		fail(err)
		s.Commits = commits
		s.Graph = graph.Rows(commits)
	})
	wg.Go(func() {
		s.Remotes = r.Git.Loaders.RemoteLoader.GetRemotes()
		var err error
		s.Tags, err = r.Git.Loaders.TagLoader.GetTags()
		fail(err)
	})
	wg.Wait()
	return s
}

// FileDiff returns the plain unified diff for a working-tree file: unstaged
// changes if any, otherwise staged ones.
func (r *Repo) FileDiff(f *models.File) string {
	cached := !f.HasUnstagedChanges && f.HasStagedChanges
	return r.Git.WorkingTree.WorktreeFileDiff(f, git_commands.DiffModePlain, cached)
}

// CommitDiff returns `git show` output for a commit.
func (r *Repo) CommitDiff(hash string) string {
	out, err := r.Git.Commit.ShowCmdObj(hash, nil, git_commands.DiffModePlain).DontLog().RunWithOutput()
	if err != nil {
		return err.Error()
	}
	return out
}

// ToggleStaged stages a file with unstaged changes, else unstages it — the
// behaviour of lazygit's <space> in the files panel for a single file.
func (r *Repo) ToggleStaged(f *models.File) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f.HasUnstagedChanges {
		return r.Git.WorkingTree.StageFile(f.Path)
	}
	paths := []string{f.Path}
	if f.PreviousPath != "" {
		paths = append(paths, f.PreviousPath)
	}
	return r.Git.WorkingTree.UnStageFile(paths, f.Tracked)
}

// SplitLines splits diff output into lines without the trailing empty one.
func SplitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// BranchLog returns the user's configured branch log (lazygit's
// git.branchLogCmd) with terminal colours stripped.
func (r *Repo) BranchLog(name string) string {
	out, err := r.Git.Branch.GetGraph(name)
	if err != nil {
		return err.Error()
	}
	return ansi.ReplaceAllString(out, "")
}

// StashDiff returns the patch of stash entry i.
func (r *Repo) StashDiff(i int) string {
	out, err := r.Git.Stash.ShowStashEntryCmdObj(i, git_commands.DiffModePlain).DontLog().RunWithOutput()
	if err != nil {
		return err.Error()
	}
	return out
}

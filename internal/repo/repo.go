// Package repo boots lazygit's git layer (pkg/commands) without its TUI and
// loads immutable snapshots of repository state for the UI to render.
package repo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	"github.com/samber/lo"

	"laziergit/internal/graph"
)

// Repo is an open repository: lazygit's command layer plus the user's
// lazygit config.
type Repo struct {
	Git       *commands.GitCommand
	Common    *common.Common
	OS        *oscommands.OSCommand
	AppConfig *config.AppConfig
	Config    *config.UserConfig
	// DiffRenderers is lazygit's DiffRendererConfigManager (git.pagers).
	DiffRenderers *config.DiffRendererConfigManager
	tempDir       string

	mu       sync.Mutex // serialises git writes against snapshot loads
	hashPool *utils.StringPool
	mainBr   *git_commands.MainBranches
	// prevFiles are the files of the last Load (lazygit's prevFiles in
	// refreshStateFiles).
	prevFiles []*models.File

	// lazygit's filtering mode (modes/filtering): the path or author that
	// commit, reflog and stash loads are restricted to.
	filterPath, filterAuthor string
	// ForceShowUntracked is set while the Files filter shows untracked
	// files (lazygit's GetStatusFileOptions.ForceShowUntracked).
	ForceShowUntracked bool

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
func Open(dir string) (*Repo, error) { return OpenWithConfig(dir, nil) }

// OpenWithConfig is Open with a hook that may adjust the loaded config before
// anything reads it, the way lazygit's integration tests do (SetupConfig).
func OpenWithConfig(dir string, setup func(*config.AppConfig)) (*Repo, error) {
	return OpenWithConfigDebug(dir, setup, false)
}

// OpenWithConfigDebug also honors the startup --debug flag.
func OpenWithConfigDebug(dir string, setup func(*config.AppConfig), debug bool) (*Repo, error) {
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "laziergit-*")
	if err != nil {
		return nil, err
	}
	appConfig, err := config.NewAppConfig("lazygit", "laziergit", "", "", "laziergit", debug, tempDir)
	if err != nil {
		return nil, fmt.Errorf("loading lazygit config: %w", err)
	}
	if setup != nil {
		setup(appConfig)
		// lazygit's entry point keeps the test's config changes across the
		// config reloads of repo switches by saving them.
		appConfig.SaveGlobalUserConfig()
	}
	return openRepo(appConfig, tempDir)
}

// RecentRepos is lazygit's AppState.RecentRepos, most recent first.
func RecentRepos() []string {
	tempDir, err := os.MkdirTemp("", "laziergit-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tempDir)
	appConfig, err := config.NewAppConfig("lazygit", "laziergit", "", "", "laziergit", false, tempDir)
	if err != nil {
		return nil
	}
	return appConfig.GetAppState().RecentRepos
}

// OpenOther opens the repository at dir with this repo's lazygit config and
// app state, reloading the per-repo config files the way lazygit's onNewRepo
// does. The two repos share a temp dir: close only the one kept.
func (r *Repo) OpenOther(dir string) (*Repo, error) {
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}
	return openRepo(r.AppConfig, r.tempDir)
}

func openRepo(appConfig *config.AppConfig, tempDir string) (*Repo, error) {
	userConfig := appConfig.GetUserConfig()
	cmn := &common.Common{
		Log:      logs.NewProductionLogger(),
		Tr:       i18n.EnglishTranslationSet(),
		AppState: appConfig.GetAppState(),
		Fs:       afero.NewOsFs(),
	}
	cmn.SetUserConfig(userConfig)

	r := &Repo{AppConfig: appConfig, Common: cmn, Config: userConfig, tempDir: tempDir, hashPool: &utils.StringPool{}}
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
	r.DiffRenderers = config.NewDiffRendererConfigManager(cmn.UserConfig)
	gitCmd, err := commands.NewGitCommand(cmn, version, osCmd,
		git_config.NewStdCachedGitConfig(cmn.Log), r.DiffRenderers)
	if err != nil {
		return nil, err
	}
	r.Git, r.OS = gitCmd, osCmd
	// lazygit's onNewRepo: the global config plus the repo's own files.
	if err := appConfig.ReloadUserConfigForRepo(perRepoConfigFiles(gitCmd.RepoPaths)); err != nil {
		return nil, err
	}
	r.Config = appConfig.GetUserConfig()
	cmn.SetUserConfig(r.Config)
	r.mainBr = git_commands.NewMainBranches(cmn, osCmd.Cmd)
	return r, nil
}

// perRepoConfigFiles is lazygit's getPerRepoConfigFiles.
func perRepoConfigFiles(paths *git_commands.RepoPaths) []*config.ConfigFile {
	files := []*config.ConfigFile{{
		Path:   filepath.Join(paths.RepoGitDirPath(), "lazygit.yml"),
		Policy: config.ConfigFilePolicySkipIfMissing,
	}}
	prevDir := paths.RepoPath()
	dir := filepath.Dir(prevDir)
	for dir != prevDir {
		files = utils.Prepend(files, &config.ConfigFile{
			Path:   filepath.Join(dir, ".lazygit.yml"),
			Policy: config.ConfigFilePolicySkipIfMissing,
		})
		prevDir = dir
		dir = filepath.Dir(dir)
	}
	return files
}

func (r *Repo) Close() { os.RemoveAll(r.tempDir) }

// Snapshot is one consistent read of the repository. Panels render from a
// single snapshot so they never disagree with each other.
type Snapshot struct {
	RebaseUpdateRefs bool
	OperationCommand string
	OperationTitle   string
	StatusTitle      string // lazygit's WorkingTreeState.Title (the effective state)
	StatusLowerTitle string // WorkingTreeState.LowerCaseTitle, "" while no operation is in progress
	OptionsMapTitle  string // lazygit's WorkingTreeState.OptionsMapTitle
	CanSkip          bool
	Operation        string
	Root             string
	Head             *models.Branch
	CheckedOutBranch string // lazygit's Model.CheckedOutBranch (determineCheckedOutRef)
	Files            []*models.File
	Remotes          []*models.Remote
	Worktrees        []*models.Worktree
	Submodules       []*models.SubmoduleConfig
	Tags             []*models.Tag
	Branches         []*models.Branch
	Commits          []*models.Commit
	Stash            []*models.StashEntry
	Reflog           []*models.Commit // the Reflog tab of the commits window
	Graph            []graph.Row      // one per commit
	BisectInfo       *git_commands.BisectInfo
	Errors           []error
}

// Load reads the full snapshot. Independent loads run in parallel.
func (r *Repo) Load() *Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &Snapshot{Root: r.Git.RepoPaths.WorktreePath()}
	state := r.Git.Status.WorkingTreeState()
	s.OperationTitle = state.OptionsMenuTitle(r.Common.Tr)
	s.StatusTitle = state.Title(r.Common.Tr)
	if state.Any() {
		s.StatusLowerTitle = state.LowerCaseTitle(r.Common.Tr)
	}
	s.OptionsMapTitle = state.OptionsMapTitle(r.Common.Tr)
	s.OperationCommand = state.CommandName()
	s.CanSkip = state.CanSkip()
	s.RebaseUpdateRefs = r.Git.Config.GetRebaseUpdateRefs()
	switch {
	case state.Rebasing:
		s.Operation = "Rebasing"
	case state.Merging:
		s.Operation = "Merging"
	case state.CherryPicking:
		s.Operation = "Cherry-picking"
	case state.Reverting:
		s.Operation = "Reverting"
	}
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
		r.stageResolvedConflicts()
		s.Files = r.Git.Loaders.FileLoader.GetStatusFiles(git_commands.GetStatusFileOptions{ForceShowUntracked: r.ForceShowUntracked})
		r.prevFiles = s.Files
	})
	wg.Go(func() {
		s.Stash = r.Git.Loaders.StashLoader.GetStashEntries(r.filterPath)
	})
	wg.Go(func() {
		reflog, _, err := r.Git.Loaders.ReflogCommitLoader.GetReflogCommits(r.hashPool, nil, "", "")
		fail(err)
		s.Reflog = reflog
		if r.filterPath != "" || r.filterAuthor != "" {
			// Branch recency uses the whole reflog; the Reflog tab the filtered one.
			filtered, _, err := r.Git.Loaders.ReflogCommitLoader.GetReflogCommits(r.hashPool, nil, r.filterPath, r.filterAuthor)
			fail(err)
			s.Reflog = filtered
		}
		branches, err := r.Git.Loaders.BranchLoader.Load(reflog, r.mainBr, nil, false,
			func(f func() error) { fail(f()) }, func() {})
		fail(err)
		if r.Config.Gui.ShowDivergenceFromBaseBranch != "none" {
			fail(r.Git.Loaders.BranchLoader.GetBehindBaseBranchValuesForAllBranches(branches, r.mainBr, func() {}))
		}
		s.Branches = branches
		for _, b := range branches {
			if b.Head {
				s.Head = b
			}
		}
		// lazygit's determineCheckedOutRef: during a rebase, the branch
		// being rebased.
		var ref models.Ref
		if rebased := r.Git.Status.BranchBeingRebased(); rebased != "" {
			ref = &models.Branch{Name: strings.TrimPrefix(rebased, "refs/heads/")}
		} else if s.Head != nil {
			ref = s.Head
		}
		// lazygit's refForLog and determineCheckedOutRef: while bisecting
		// the log starts from the ref the bisect was started on.
		info := r.Git.Bisect.GetInfo()
		s.BisectInfo = info
		refName := "HEAD"
		if info.Started() {
			refName = info.GetStartHash()
			if info.Bisecting() && !r.Git.Bisect.ReachableFromStart(info) {
				refName = info.GetNewHash()
			}
			if info.Bisecting() && info.GetStartHash() != "" {
				ref = &models.Branch{Name: info.GetStartHash()}
			}
		}
		if ref != nil && !(s.Head != nil && ref == models.Ref(s.Head) && s.Head.DetachedHead) {
			s.CheckedOutBranch = ref.RefName() // "" on a detached head (CurrentBranchName)
		}
		commits, err := r.Git.Loaders.CommitLoader.GetCommits(git_commands.GetCommitsOptions{
			Limit:                true,
			IncludeRebaseCommits: true,
			FilterPath:           r.filterPath,
			FilterAuthor:         r.filterAuthor,
			RefName:              refName,
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
		branches, branchErr := r.Git.Loaders.RemoteLoader.GetRemoteBranchesByRemoteName()
		fail(branchErr)
		for _, remote := range s.Remotes {
			remote.Branches = branches[remote.Name]
		}
		var err error
		s.Tags, err = r.Git.Loaders.TagLoader.GetTags()
		fail(err)
	})
	wg.Go(func() {
		var err error
		s.Worktrees, err = r.Git.Loaders.Worktrees.GetWorktrees()
		fail(err)
	})
	wg.Go(func() {
		var err error
		s.Submodules, err = r.Git.Submodule.GetConfigs(nil)
		fail(err)
	})
	wg.Wait()
	// lazygit's markWorktreeFiles: files that are linked worktrees of this repo.
	root := r.Git.RepoPaths.WorktreePath()
	for _, f := range s.Files {
		abs := filepath.Join(root, f.Path)
		for _, w := range s.Worktrees {
			if w.Path == abs {
				f.IsWorktree = true
			}
		}
	}
	return s
}

// FileDiff returns the plain unified diff for a working-tree file: unstaged
// changes if any, otherwise staged ones.
func (r *Repo) FileDiff(f *models.File) string {
	cached := !f.HasUnstagedChanges && f.HasStagedChanges
	return r.Git.WorkingTree.WorktreeFileDiff(f, git_commands.DiffModePlain, cached)
}

// SetFilter sets lazygit's filtering mode; empty strings leave it.
func (r *Repo) SetFilter(path, author string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filterPath, r.filterAuthor = path, author
}

// Filter returns the filtering mode's path and author.
func (r *Repo) Filter() (path, author string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.filterPath, r.filterAuthor
}

// FilterPathsForCommit is lazygit's DiffHelper.FilterPathsForCommit.
func (r *Repo) FilterPathsForCommit(c *models.Commit) []string {
	path, _ := r.Filter()
	if path == "" {
		return nil
	}
	if c != nil && len(c.FilterPaths) > 0 {
		return c.FilterPaths
	}
	return []string{path}
}

// CommitDiff returns `git show` output for a commit, restricted to the
// filter path in filtering mode.
func (r *Repo) CommitDiff(c *models.Commit, width int) string {
	out, err := withColumns(r.Git.Commit.ShowCmdObj(c.Hash(), r.FilterPathsForCommit(c), git_commands.DiffModePlain), width).DontLog().RunWithOutput()
	if err != nil {
		return err.Error()
	}
	return out
}

// RangeDiff is the range branch of lazygit's
// GetUpdateTaskForRenderingCommitsDiff: the diff from `from`'s parent to
// `to`, behind lazygit's "Showing diff for range" prefix.
func (r *Repo) RangeDiff(from, to *models.Commit, width int) string {
	args := []string{from.ParentRefName(), to.RefName(), "--stat", "-p", "--"}
	if path, _ := r.Filter(); path != "" {
		args = append(args, lo.Uniq(append(r.FilterPathsForCommit(from), r.FilterPathsForCommit(to)...))...)
	}
	out, err := withColumns(r.Git.Diff.DiffCmdObj(args, git_commands.DiffModePlain), width).DontLog().RunWithOutput()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%s %s-%s\n\n", r.Common.Tr.ShowingDiffForRange, from.ShortRefName(), to.ShortRefName()) + out
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
func (r *Repo) StashDiff(i int, width int) string {
	out, err := withColumns(r.Git.Stash.ShowStashEntryCmdObj(i, git_commands.DiffModePlain), width).DontLog().RunWithOutput()
	if err != nil {
		return err.Error()
	}
	return out
}

// TagInfo is lazygit's TagsController main view: the tag's info, then the
// ref's graph.
func (r *Repo) TagInfo(name, fullRef string) string {
	tr := r.Common.Tr
	info := tr.LightweightTag + ": " + name
	if annotated, _ := r.Git.Tag.IsTagAnnotated(name); annotated {
		info = tr.AnnotatedTag + ": " + name
		if out, err := r.Git.Tag.ShowAnnotationInfo(name); err == nil {
			info += "\n\n" + strings.TrimRight(filterOutPgpSignature(out), "\n")
		}
	}
	return info + "\n\n---\n\n" + r.BranchLog(fullRef)
}

func filterOutPgpSignature(output string) string {
	var kept []string
	in := false
	for _, line := range strings.Split(output, "\n") {
		if line == "-----END PGP SIGNATURE-----" {
			in = false
			continue
		}
		if line == "-----BEGIN PGP SIGNATURE-----" {
			in = true
		}
		if !in {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// ReloadChangedConfig is lazygit's ReloadChangedUserConfigFiles on focus: when
// a config file changed, the user config is reloaded. It reports whether it was.
func (r *Repo) ReloadChangedConfig() (bool, error) {
	err, changed := r.AppConfig.ReloadChangedUserConfigFiles()
	if err != nil || !changed {
		return false, err
	}
	r.Config = r.AppConfig.GetUserConfig()
	r.Common.SetUserConfig(r.Config)
	return true, nil
}

// withColumns runs cmd as lazygit's main view runs it, in a terminal as
// wide as the pane (width columns; none when 0): git sizes a diffstat to it.
func withColumns(cmd *oscommands.CmdObj, width int) *oscommands.CmdObj {
	if width > 0 {
		cmd.AddEnvVars(fmt.Sprintf("COLUMNS=%d", width))
	}
	return cmd
}

package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jesseduffield/lazygit/pkg/commands/direnv"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/icons"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"

	"laziergit/internal/repo"
)

// Ported from lazygit's ReposHelper (recent repos menu, switching repos),
// recent_repos_panel.go and the startup popups of gui/layout.go.

// startupPopupVersion is lazygit's StartupPopupVersion.
const startupPopupVersion = 5

// lazygit's menuMaxWidth and the recent repos column limits.
const (
	menuMaxWidth              = 90
	recentReposNameMaxWidth   = 30
	recentReposBranchMaxWidth = 30
)

// repoUIState is what lazygit keeps per repo in RepoStateMap that the app
// restores when switching back: the focused context and each list's place.
type repoUIState struct {
	focus           panel
	cursor          [panelCount]int
	fileTab         int
	branchTab       int
	commitTab       int
	commitTabCursor [2]int
	tabCursor       [2][3]int
	files           *filetree.FileTreeViewModel // lazygit keeps each repo's contexts, so the selected file is kept by path
}

func (a *App) saveRepoUIState(root string) {
	if a.repoStates == nil {
		a.repoStates = map[string]*repoUIState{}
	}
	a.repoStates[root] = &repoUIState{focus: a.focus, cursor: a.cursor, fileTab: a.fileTab, branchTab: a.branchTab,
		commitTab: a.commitTab, commitTabCursor: a.commitTabCursor, tabCursor: a.tabCursor, files: a.files}
}

func (a *App) restoreRepoUIState(root string) bool {
	s := a.repoStates[root]
	if s == nil {
		a.newFileTrees()
		return false
	}
	a.files = s.files
	a.focus, a.cursor, a.fileTab, a.branchTab = s.focus, s.cursor, s.fileTab, s.branchTab
	a.commitTab, a.commitTabCursor, a.tabCursor = s.commitTab, s.commitTabCursor, s.tabCursor
	return true
}

// startup is lazygit's onInitialViewsCreation: the intro popup on first use
// and the recent repos menu when asked for (or when opened outside a repo).
func (a *App) startup(showRecentRepos bool) {
	r := a.repo
	if !r.Config.DisableStartupPopups && r.Common.AppState.StartupPopupVersion < startupPopupVersion {
		a.showIntroPopup()
	}
	a.bg(func() {
		updateRecentRepoList(r)
		if showRecentRepos {
			items := a.recentRepoItems(r)
			a.post(func() {
				if a.repo == r {
					a.showRecentReposMenu(items)
				}
			})
		}
	})
}

func (a *App) showIntroPopup() {
	r := a.repo
	done := func() {
		r.Common.AppState.StartupPopupVersion = startupPopupVersion
		a.bg(func() { _ = r.AppConfig.SaveAppState() })
	}
	msg := utils.ResolvePlaceholderString(r.Common.Tr.IntroPopupMessage, map[string]string{
		"confirmationKey": r.Config.Keybinding.Universal.Confirm.String(),
	})
	a.pushOverlay(&confirmOverlay{body: msg, onYes: done, onClose: done})
}

// quitFlow is lazygit's QuitActions.quitAux.
func (a *App) quitFlow() {
	if a.repo.Config.ConfirmOnQuit {
		a.confirm("", a.repo.Common.Tr.ConfirmQuit, false, a.quit)
		return
	}
	a.quit()
}

// updateRecentRepoList is lazygit's: the repo we're in goes first. Runs off
// the main thread (it stats every recent repo and writes the app state).
func updateRecentRepoList(r *repo.Repo) {
	if r.Git.Status.IsBareRepo() {
		return
	}
	current, err := os.Getwd()
	if err != nil {
		return
	}
	repos := []string{current}
	for _, p := range r.Common.AppState.RecentRepos {
		if p == current {
			continue
		}
		if _, err := os.Stat(filepath.Join(p, ".git")); err != nil {
			continue
		}
		repos = append(repos, p)
	}
	r.Common.AppState.RecentRepos = repos
	_ = r.AppConfig.SaveAppState()
}

func (a *App) openRecentRepos() {
	r := a.repo
	a.bg(func() {
		items := a.recentRepoItems(r)
		a.post(func() {
			if a.repo == r {
				a.showRecentReposMenu(items)
			}
		})
	})
}

func (a *App) showRecentReposMenu(items []menuItem) {
	tr := a.repo.Common.Tr
	items = append(items, menuItem{label: tr.Cancel, run: func() {}})
	a.pushOverlay(&menuOverlay{title: tr.RecentRepos, items: items, filterable: true, filterAsYouType: true, noItemKeys: true})
}

type recentRepoEntry struct {
	path, branchName                    string
	nameColumn, branchColumn, dirColumn string
}

// recentRepoItems is CreateRecentReposMenu's items. Off the main thread: it
// reads every repo's HEAD.
func (a *App) recentRepoItems(r *repo.Repo) []menuItem {
	tr := r.Common.Tr
	var paths []string
	if len(r.Common.AppState.RecentRepos) > 0 {
		paths = r.Common.AppState.RecentRepos[1:]
	}
	branches := make([]string, len(paths))
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Go(func() { branches[i] = currentBranch(r, p) })
	}
	wg.Wait()
	entries := lo.Map(paths, func(p string, i int) recentRepoEntry {
		branchColumn := branches[i]
		if icons.IsIconEnabled() {
			branchColumn = icons.BRANCH_ICON + " " + branches[i]
		}
		return recentRepoEntry{path: p, branchName: branches[i], nameColumn: filepath.Base(p),
			branchColumn: branchColumn, dirColumn: utils.ContractTilde(filepath.Dir(p))}
	})
	nameW, branchW := utils.StringWidth(tr.Cancel), 0
	for _, e := range entries {
		nameW = max(nameW, utils.StringWidth(e.nameColumn))
		branchW = max(branchW, utils.StringWidth(e.branchColumn))
	}
	nameW, branchW = min(nameW, recentReposNameMaxWidth), min(branchW, recentReposBranchMaxWidth)
	dirW := menuMaxWidth - 2 - 2 - nameW - branchW

	items := make([]menuItem, 0, len(entries))
	for _, e := range entries {
		name := utils.TruncateWithEllipsis(e.nameColumn, nameW)
		branch := utils.TruncateWithEllipsis(e.branchColumn, branchW)
		dir := utils.TruncateWithEllipsisInMiddle(e.dirColumn, dirW)
		type field struct{ label, value string }
		var fields []field
		if name != e.nameColumn {
			fields = append(fields, field{tr.RecentReposRepoLabel, e.nameColumn})
		}
		if branch != e.branchColumn {
			fields = append(fields, field{tr.RecentReposBranchLabel, e.branchName})
		}
		if dir != e.dirColumn {
			fields = append(fields, field{tr.RecentReposPathLabel, e.dirColumn})
		}
		labelW := utils.MaxFn(fields, func(f field) int { return utils.StringWidth(f.label) })
		lines := lo.Map(fields, func(f field, _ int) string {
			return utils.WithPadding(f.label, labelW, utils.AlignLeft) + " " + f.value
		})
		path := e.path
		items = append(items, menuItem{
			label:      name,
			columns:    []string{branch, dir},
			filterText: strings.Join([]string{e.nameColumn, e.branchName, e.path}, " "),
			tooltip:    strings.Join(lines, "\n"),
			run:        func() { a.switchToRecentRepo(path) },
		})
	}
	return items
}

// currentBranch is ReposHelper.getCurrentBranch.
func currentBranch(r *repo.Repo, path string) string {
	branch, hash, ok := readHeadInfo(path)
	if !ok {
		branch, hash, ok = askGitForHeadInfo(r, path)
	}
	switch {
	case !ok:
		return r.Common.Tr.BranchUnknown
	case branch != "":
		return branch
	}
	return utils.ResolvePlaceholderString(r.Common.Tr.HeadDetachedAt, map[string]string{"hash": utils.ShortHash(hash)})
}

func readHeadInfo(path string) (branch, hash string, ok bool) {
	gitDir, ok := git_commands.GitDirOfRepo(path)
	if !ok {
		return "", "", false
	}
	content, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", "", false
	}
	head := strings.TrimSpace(string(content))
	if b, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		if b == ".invalid" { // a reftable repo's placeholder
			return "", "", false
		}
		return b, "", true
	}
	return "", head, true
}

func askGitForHeadInfo(r *repo.Repo, path string) (branch, hash string, ok bool) {
	ask := func(sub string, args ...string) (string, bool) {
		cmd := r.OS.Cmd.New(git_commands.NewGitCmd(sub).Dir(path).Arg(args...).ToArgv()).DontLog()
		out, _, err := git_commands.ForOtherRepo(cmd).RunWithOutputs()
		out = strings.TrimSpace(out)
		return out, err == nil && out != ""
	}
	if b, ok := ask("symbolic-ref", "--short", "--quiet", "HEAD"); ok {
		return b, "", true
	}
	if h, ok := ask("rev-parse", "HEAD"); ok {
		return "", h, true
	}
	return "", "", false
}

func (a *App) switchToRecentRepo(path string) {
	if a.busy != "" {
		a.setStatus(a.repo.Common.Tr.CantSwitchWhileOperationInProgress, true)
		return
	}
	// Leaving a submodule for another repo forgets the way back.
	a.parents = nil
	a.openRepository(path, false)
}

// loadDirenv is the direnv part of ReposHelper.switchToLocation, off the
// main thread: it applies the new directory's env and logs what direnv said.
func (a *App) loadDirenv(r *repo.Repo) direnv.LoadResult {
	res := direnv.Load(r.OS.Cmd)
	if res.Message != "" {
		a.logLine(res.Message)
	}
	return res
}

func (a *App) logLine(msg string) {
	a.post(func() { a.log = append(a.log, logEntry{text: msg}) })
}

// promptDirenvApproval is ReposHelper.promptDirenvApproval.
func (a *App) promptDirenvApproval(envrcPath string) {
	r := a.repo
	content, err := os.ReadFile(envrcPath)
	if err != nil {
		return
	}
	indented := "  " + strings.ReplaceAll(strings.TrimRight(string(content), "\n"), "\n", "\n  ")
	prompt := utils.ResolvePlaceholderString(r.Common.Tr.DirenvApprovalPrompt, map[string]string{
		"confirmKey": r.Config.Keybinding.Universal.Confirm.String(),
		"cancelKey":  r.Config.Keybinding.Universal.Return.String(),
		"content":    indented,
	})
	a.confirm(r.Common.Tr.DirenvApprovalTitle, prompt, false, func() {
		a.bg(func() {
			err := direnv.Allow(r.OS.Cmd, envrcPath)
			if err == nil {
				err = a.loadDirenv(r).Err
			}
			if err != nil {
				a.post(func() { a.showError("direnv", err) })
			}
		})
	})
}

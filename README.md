<img src="resources/icon.png" width="64" height="64" alt="">

# laziergit

[lazygit](https://github.com/jesseduffield/lazygit) as a desktop GUI, keeping what makes lazygit
fast (one-key actions, everything visible, your keybindings) while adding what a terminal can't do:
real type hierarchy, hairline borders, a full colour palette, a drawn commit graph, hover and mouse.

Built with [MyGo](https://mygo.egoist.dev)'s native UI (Go, GPU-drawn, no webview) on top of
lazygit's own git layer (`pkg/commands`), so git behaviour matches lazygit's.

```sh
go run . /path/to/repo
```

Uses your lazygit config and keybindings (`LG_CONFIG_FILE` or the standard config location).
Git credential prompts open in-app. Cmd shortcuts stay with macOS.

On macOS, drag the app from the DMG into Applications, open it, then choose
**laziergit → Install Command Line Tool…** (Cmd+Shift+L). This installs
`/usr/local/bin/laziergit`; macOS asks for administrator permission if needed.
Run `laziergit` inside a Git repository, or `laziergit /path/to/repo`.
Keep the app in place after installation, and ensure `/usr/local/bin` is on your PATH.
Opening the app from Finder prompts you to choose a repository.

## Lazygit compatibility

✅ supported · ❌ missing · notes indicate partial support

| Feature | laziergit |
| --- | --- |
| Panel navigation and keybindings menu | ✅ |
| Lazygit config | Keybindings and selected git/refresh settings |
| Auto-refresh and auto-fetch | ✅ |
| File diffs and commit graph | ✅ |
| Stage / unstage files, lines, hunks and ranges | ✅ |
| Discard changes | Files and unstaged selections |
| Commit and amend | ✅ |
| Push, pull and fetch | ✅ |
| Git credential prompts | ✅ |
| Local branches | Create, checkout and delete |
| Stash | Save, apply, pop and drop |
| Commit inspection | Diff and changed files |
| Interactive rebase | Edit, squash, fixup, reword, reorder, drop, continue, skip and abort |
| Cherry-pick | Copy commits and apply in copy order |
| Merge conflicts | Top/bottom/both, whole-file, manual and delete resolutions; limited navigation |
| Remotes | Inspect and fetch; no management UI |
| Tags | Inspect and checkout; no management UI |
| Undo | Reflog-based commit, checkout and rebase undo |
| Redo | ❌ |
| Custom commands | Templates, menus, prompts and popup/log output; no terminal integration, suggestions or full session properties |
| Command log | ✅ |
| Merge / revert / reset actions | Continue/abort existing operations; no dedicated start/reset UI |
| Reflog browser | ❌ |
| Worktrees | Create, open and remove |
| Submodules | Add, initialize, update, edit URL, open and remove; bulk and recursive updates |
| Bisect | ❌ |

## Worktrees and submodules

The Files pane has **Files**, **Worktrees**, and **Submodules** tabs, matching
lazygit's layout. Use `2` to focus it and `[` / `]` to change tabs, or click a tab.
Your configured lazygit bindings apply; `?` lists and runs the available actions.

In Worktrees, `n` creates a worktree from a ref, optionally on a new branch;
Enter or Space opens it, and `d` removes it after confirmation. Git refuses to
remove dirty or locked worktrees. The main and current worktrees cannot be removed.
From Branches, `w` creates a worktree with the selected branch as its base.

In Submodules, `n` adds one, `i` initializes it, `u` updates to the recorded commit,
`e` edits its URL, and Enter opens it. Escape returns to the parent repository.
The `b` menu initializes or updates all submodules, including recursive updates.
Removing with `d` requires confirmation: it discards local changes and local Git
data, including unpushed commits, and stages the removal. Each repository shows
its direct submodules; open a submodule to manage its nested modules.

## Development

```sh
go test ./...                                   # headless UI tests drive real keypresses and git
SHOT_DIR=/tmp/shots go test ./internal/app -run TestShots   # screenshots for visual review
PERF_REPO=/path/to/big/repo go test ./internal/app -run TestPerf -v
LAZIERGIT_KEYS="4 <enter>" LAZIERGIT_CAPTURE=out.png go run . /path/to/repo   # capture the real window
```

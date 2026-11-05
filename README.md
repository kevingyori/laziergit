<img src="resources/icon.png" width="64" height="64" alt="">

# laziergit

[lazygit](https://github.com/jesseduffield/lazygit) as a desktop GUI, keeping what makes lazygit
fast (one-key actions, everything visible, your keybindings) while adding what a terminal can't do:
real type hierarchy, hairline borders, a full colour palette, a drawn commit graph, hover and mouse.

Built with [MyGo](https://mygo.egoist.dev)'s native UI (Go, GPU-drawn, no webview) on top of
lazygit's own git layer (`pkg/commands`), so git behaviour matches lazygit's.

![laziergit showing the lazygit repository's commit graph and a code diff](resources/screenshot.png)

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

## Feature parity with lazygit

Our goal is to get as close to 100% feature parity with lazygit as possible.
The table below tracks current support and the remaining gaps.

✅ supported · ❌ missing · other entries describe partial support and known gaps

| Feature | Supported in laziergit | Known gaps |
| --- | --- | --- |
| Panel navigation and keybindings menu | ✅ | — |
| Lazygit config | Keybindings and selected git/refresh settings | GUI theme/layout settings and bindings for unimplemented actions |
| Auto-refresh and auto-fetch | ✅ | — |
| File diffs and commit graph | ✅ | — |
| Stage / unstage files, lines, hunks and ranges | ✅ | — |
| Discard changes | Files and unstaged selections | Direct discard of staged lines/hunks; unstage them first |
| Commit and amend | ✅ | — |
| Push, pull and fetch | ✅ | — |
| Git credential prompts | ✅ | — |
| Local branches | Create, checkout, delete, rename, merge, rebase, fast-forward and pull requests | Range selection, force checkout, previous-branch checkout, upstream options, sort order, tags and resets from a branch, and marking a base commit for rebase |
| Stash | Save, apply, pop and drop | Rename and selective stashing of files or staged changes |
| Commit inspection | Diff and changed files | Checkout a historical file, log filtering and clipboard/browser actions |
| Interactive rebase | Edit, squash, fixup, reword, reorder, drop, continue, skip and abort; pick, drop, squash, fixup, edit and reorder todos of a rebase in progress | Operations on selected commit ranges, amend-to-commit and author editing |
| Cherry-pick | Copy commits and apply in copy order | Range selection and a dedicated reset-copied-commits action |
| Merge conflicts | Top/bottom/both, whole-file, manual and delete resolutions | Next/previous conflict navigation and selecting an arbitrary conflict; top/bottom/both targets the first conflict |
| Remotes | Inspect and fetch | Add, edit and remove remotes; remote-branch management |
| Tags | Inspect and checkout | Create, delete and push tags |
| Undo | Reflog-based commit, checkout and rebase undo | Automatic stashing of local edits before undoing a rebase |
| Redo | ❌ | Entire feature |
| Custom commands | Templates, menus, prompts and popup/log output | Terminal integration, suggestions and full session properties |
| Command log | ✅ | — |
| Merge / revert / reset actions | Merge a branch (regular, fast-forward, no-ff, squash); continue/abort existing operations | Start a revert; reset to a selected commit |
| Reflog browser | ❌ | Entire feature |
| Worktrees | Create, open and remove | Open in an external editor |
| Submodules | Add, initialize, update, edit URL, open and remove; bulk and recursive updates | Submodule diff preview in the Submodules tab and Select-key entry |
| Bisect | ❌ | Entire feature |

## Development

```sh
go test ./...                                   # headless UI tests drive real keypresses and git
SHOT_DIR=/tmp/shots go test ./internal/app -run TestShots   # screenshots for visual review
PERF_REPO=/path/to/big/repo go test ./internal/app -run TestPerf -v
LAZIERGIT_KEYS="4 <enter>" LAZIERGIT_CAPTURE=out.png go run . /path/to/repo   # capture the real window
```

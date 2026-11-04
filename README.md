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
| Worktree and submodule management | ❌ |
| Bisect | ❌ |

## Development

```sh
go test ./...                                   # headless UI tests drive real keypresses and git
SHOT_DIR=/tmp/shots go test ./internal/app -run TestShots   # screenshots for visual review
PERF_REPO=/path/to/big/repo go test ./internal/app -run TestPerf -v
LAZIERGIT_KEYS="4 <enter>" LAZIERGIT_CAPTURE=out.png go run . /path/to/repo   # capture the real window
```

## App icon

`assets/AppIcon.icon` is the layered Icon Composer source. Open it in Apple's
Icon Composer to edit the glass mascot and its separate expression layer.
`mygo.json` uses it for the macOS asset catalog; `go tool mygo build -skip-dmg`
compiles it with Xcode 26 or later and packages the catalog and legacy ICNS.

After editing, run `scripts/export-icon.sh` on macOS with full Xcode selected to
regenerate `resources/icon.png`, the README image and fallback for builds without
Icon Composer support. Keep the layer artwork free of baked-in lighting and masks;
Icon Composer supplies the material, shadow, and platform shape.

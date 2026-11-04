<img src="assets/icon.svg" width="64" height="64" alt="">

# laziergit

[lazygit](https://github.com/jesseduffield/lazygit) as a desktop GUI, keeping what makes lazygit
fast (one-key actions, everything visible, your keybindings) while adding what a terminal can't do:
real type hierarchy, hairline borders, a full colour palette, a drawn commit graph, hover and mouse.

Built with [MyGo](https://mygo.egoist.dev)'s native UI (Go, GPU-drawn, no webview) on top of
lazygit's own git layer (`pkg/commands`), so git behaviour matches lazygit's.

```sh
go run . /path/to/repo
```

## Compatibility

- Reads your lazygit config (`~/Library/Application Support/lazygit/config.yml`, or `LG_CONFIG_FILE`):
  keybindings, `git.branchLogCmd`, `git.autoFetch`, `refresher.*`, `git.skipHookPrefix`, ...
- Keys are matched the way lazygit means them: `?`, `<`, `@` are characters, whatever your layout;
  `<c-r>`/`<a-…>` are Ctrl/Option chords. Cmd is left to macOS.
- Credential prompts from git (HTTPS username/password, SSH passphrases) open in-app.

## What works

| Area | Keys (defaults) |
| --- | --- |
| Panels, navigation | `1`–`5`, `h`/`l`, `j`/`k`, `<`/`>`, `,`/`.`, `0` focus main, `esc` back |
| Files | `space` stage, `a` stage all, `enter` stage lines, `d` discard, `c` commit, `w` commit without hooks, `A` amend, `s` stash, `f` fetch |
| Staging | `space` stage/unstage, `v` range, `a` hunk, `tab` staged/unstaged, `d` discard |
| Branches | `space` checkout, `n` new, `c` checkout by name, `d` delete |
| Commits | `enter` view files, `s` squash down, `f` fixup down, `r` reword, `i` rebase controls, Ctrl+`j`/`k` reorder, `C` copy, `V` cherry-pick |
| Conflicts | `space` or `enter` opens top/bottom/both, whole-file, manual and delete resolutions; `@` → continue/abort |
| Branch tabs | `[` / `]` or click Branches / Remotes / Tags; `space` checks out a tag (detached HEAD), `f` fetches in Remotes |
| Undo | `z` previews undo of the latest supported reflog action; commit changes remain staged |
| Stash | `space` apply, `g` pop, `d` drop |
| Sync | `P` push (prompts for upstream; force-with-lease when diverged), `p` pull |
| Everything else | `?` keybindings menu (`/` to filter), `@` command log |

## Phase 3 details

History editing uses lazygit's rebase engine and sequence editor. Squash keeps both
messages; fixup keeps the parent's message. Rewrites ask for confirmation, reject
stale history, and expose continue, skip and abort in `i`. The status panel shows
an active rebase, merge, cherry-pick or revert. Resolve conflicts before continuing.
For repeated conflicts in a file, reopen `space` to resolve the next conflict;
the file stages when the final conflict is resolved. During rebase, “ours” is the
rebased branch and “theirs” is the commit being replayed.

`C` toggles copied commits; `V` applies them to the current branch in copy order.
Copy before switching branches. Undo reads git's reflog, groups a rebase into one
action, and skips actions already undone. It preserves local edits or lets git
refuse overlapping edits. Undo is unavailable during an active git operation;
it does not restore discarded working-tree edits, deleted branches or dropped stashes.

Configured custom commands participate in the action table, including overrides,
context-specific keys and nested menus. Supported prompts: input, confirm, menu,
and menuFromCommand (regex groups and value/label templates). `quote`, `runCommand`,
`.Form`, `.PromptResponses`, and selected file/branch/commit/stash template values
are available. Popup output is selectable; log output appears after completion.
Terminal-owning commands, input suggestions, ANSI colour templates, and the full
set of lazygit session template properties are not yet supported. Remotes/tags
currently support inspection, fetch and tag checkout; creation/editing/deletion
can use custom commands. Redo remains future work.

## Development

```sh
go test ./...                                   # headless UI tests drive real keypresses and git
SHOT_DIR=/tmp/shots go test ./internal/app -run TestShots   # screenshots for visual review
PERF_REPO=/path/to/big/repo go test ./internal/app -run TestPerf -v
LAZIERGIT_KEYS="4 <enter>" LAZIERGIT_CAPTURE=out.png go run . /path/to/repo   # capture the real window
```

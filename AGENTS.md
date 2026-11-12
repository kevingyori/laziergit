# laziergit

lazygit as a desktop GUI: MyGo native UI (Go, GPU, no webview) on lazygit's `pkg/commands`.

## Principles
- Get as close to 100% feature parity with lazygit as possible.
- Keep lazygit's speed and keyboard grammar: same panels and layout, your lazygit config and keybindings, one-key actions.
- Add what terminals can't do: type hierarchy (prose font for words, mono for hashes and diffs, at most 3 sizes), hairline borders, hover, mouse.
- Colour only for meaning (state, diff, focus, graph lanes); the chrome stays neutral. No hidden details, no Bloomberg density.
- Every mouse or hover affordance has a keyboard equivalent.
- Reuse lazygit's code (git layer, patch, config); rewrite only the UI.
- Never add feature documentation to README.md.

## Implemented
Read-only shell, graph, keybindings menu, auto-refresh, staging, commits, sync, branches, stash, credentials, command log, history editing, cherry-pick, conflict resolution, custom commands, remotes/tags tabs, reflog undo, branch rename/merge/rebase/fast-forward/pull requests (GitHub PR status via `gh` login), and a Settings window (Cmd+,) that edits the config file's scalar and list options.

## Next
lazygit behaviour its integration tests don't cover: open pull request at a diff line for commit ranges and stacks (`pr_line.go` handles one commit), re-running diff renderers on a pane resize, `gui.colorScheme`, `git.log.showGraph: when-maximised`, lazygit's Cancel row in every menu (only bisect menus have it), and the options bar's `…` truncation and controller order. Later, in both lazier and laziest: hover tooltips on every button explaining the action (from the action table), and Settings controls for the map and struct-list options (custom commands, side panels, icons, services) plus each option's description.

## Non-obvious
- Only the root element is focusable and owns all keys (`App.dispatch`). Text fields use `OnTextKey`. The root takes focus back each frame when no text overlay is open, because MyGo doesn't hand focus back when an overlay is replaced within one frame.
- Unmodified keys match on `InputText` (characters, layout-independent); named keys and Ctrl/Alt chords match on KeyDown. Cmd belongs to macOS.
- Add an action to the table in `internal/app/actions.go`: that one entry drives the keys, the hint bar and the `?` menu.
- Git calls run off the main thread (`a.run`/`a.bg`) and come back through `a.post`. Never call git in a view.
- Credential answers must end in `\n` (they're typed into git's pty).
- Lane layout is copied into `internal/graph` because lazygit's Pipe fields are private.
- Verify every change: `go test ./...` (headless tests drive real keys and git), `SHOT_DIR=… go test ./internal/app -run TestShots`, and `LAZIERGIT_KEYS="4 <enter>" LAZIERGIT_CAPTURE=out.png go run . <repo>` for the real window.
- Parity: `parity/run.sh all` runs lazygit's own integration tests against the app and regenerates `parity/TASKS.md`; `parity/passing.txt` is a ratchet that `go test` enforces. Feature work toward parity follows `parity/AGENT.md`. Everything should behave and read exactly like lazygit (wording included): every failing parity test is a bug. The harness (`internal/app/parity*_test.go`) only projects what the app draws; never change it to make a test pass.
- Menu items with `disabled` stay listed with their reason (lazygit's DisabledReason); choosing one only sets the status.
- `a.run` turns git conflict output into the "Conflicts!" menu when an operation is left in progress.
- Tests never touch the network: `testrepo.New` points `GH_PATH` at nothing, and `a.openLink`/`a.copyText` are swappable.
- Commit small and often.

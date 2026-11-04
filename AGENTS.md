# laziergit

lazygit as a desktop GUI: MyGo native UI (Go, GPU, no webview) on lazygit's `pkg/commands`.

## Principles
- Get as close to 100% feature parity with lazygit as possible.
- Keep lazygit's speed and keyboard grammar: same panels and layout, your lazygit config and keybindings, one-key actions.
- Add what terminals can't do: type hierarchy (prose font for words, mono for hashes and diffs, at most 3 sizes), hairline borders, hover, mouse.
- Colour only for meaning (state, diff, focus, graph lanes); the chrome stays neutral. No hidden details, no Bloomberg density.
- Every mouse or hover affordance has a keyboard equivalent.
- Reuse lazygit's code (git layer, patch, config); rewrite only the UI.

## Implemented
Read-only shell, graph, keybindings menu, auto-refresh, staging, commits, sync, branches, stash, credentials, command log, history editing, cherry-pick, conflict resolution, custom commands, remotes/tags tabs and reflog undo. See README for workflows and feature parity gaps with lazygit.

## Next
Custom-command feature parity with lazygit (terminal integration, suggestions, complete session properties), remote/tag management, redo, richer conflict navigation.

## Non-obvious
- Only the root element is focusable and owns all keys (`App.dispatch`). Text fields use `OnTextKey`. The root takes focus back each frame when no text overlay is open, because MyGo doesn't hand focus back when an overlay is replaced within one frame.
- Unmodified keys match on `InputText` (characters, layout-independent); named keys and Ctrl/Alt chords match on KeyDown. Cmd belongs to macOS.
- Add an action to the table in `internal/app/actions.go`: that one entry drives the keys, the hint bar and the `?` menu.
- Git calls run off the main thread (`a.run`/`a.bg`) and come back through `a.post`. Never call git in a view.
- Credential answers must end in `\n` (they're typed into git's pty).
- Lane layout is copied into `internal/graph` because lazygit's Pipe fields are private.
- Verify every change: `go test ./...` (headless tests drive real keys and git), `SHOT_DIR=… go test ./internal/app -run TestShots`, and `LAZIERGIT_KEYS="4 <enter>" LAZIERGIT_CAPTURE=out.png go run . <repo>` for the real window.
- Commit small and often.

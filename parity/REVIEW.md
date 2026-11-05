# Review

Features the parity agent finished, newest last, with screenshots of the
lazygit tests they made pass (each test's last frame, rendered offscreen).
Look for anything that doesn't fit the app's design (AGENTS.md Principles)
or doesn't match lazygit.


## file-tree: Native file and commit-file trees
Commit cd5a304 (UI), ee2dce1 (projection) · +8 tests · 2026-11-05
What to look at: compressed directories, retained selection when switching tree/flat views, directory staging and discard, and renames across directories. Remaining failures move on to range selection, filtering, split diff panes, and other file actions.

![Collapse and expand directories](review/file-tree/file__collapse_expand.png)
![Commit directory renames](review/file-tree/commit__directory_diff_with_renamed_files.png)
![Discard only unstaged directory changes](review/file-tree/file__discard_unstaged_dir_changes.png)
![Renames without a root row](review/file-tree/file__renamed_files_no_root_item.png)

## batch-3: Search, popup menus, ref navigation, list ranges and focused diffs
Commits a8cf243–d018dab · +53 tests · 2026-11-05

What to look at: filtered staging acts only on visible files; commit amend/fixup menus use lazygit's wording and keys; diff ranges span file boundaries. Ref commits open their files and return to the parent selection. Search follows the focused line and collapses a previous range or hunk selection.

Full run: 168 pass, 468 fail, 35 need harness support, 17 skipped; no regressions. `go vet ./...`, `go test ./...`, and offscreen screenshots pass. The harness change is exactly the search projection approved in O-2026-11-05-5. Remaining failures and their first assertions are regenerated in TASKS.md; this integration does not claim complete feature parity.

![Staging only filtered files](review/batch-3/filter_and_search__filter_files_stage_all.png)
![Creating an amend commit](review/batch-3/commit__create_amend_commit.png)
![Staging a range across files](review/batch-3/main_view__stage_range_spanning_files.png)
![Squashing current-branch fixups](review/batch-3/interactive_rebase__squash_fixups_in_current_branch.png)

Native-test screenshots below provide UI evidence for ref navigation and the dark focused diff. Ref navigation remains blocked in lazygit's tests by the missing `subCommits` projection; it is covered by native tests instead.

![Native ref-commit navigation](review/batch-3/native-ref-commits.png)
![Native focused diff in dark mode](review/batch-3/native-staging-dark.png)

## focused-patches: Paired diffs and custom patch editing
Commits 54fd574 (UI/git integration), 12e0803 (approved projection) · +71 tests · 2026-11-05

What to look at: commit diffs now build patches with Space, by line, hunk,
range, directory or whole file. Ctrl+P applies or reverses a patch, removes it
from its source commit, moves it into the index or another commit, splits it
into a new commit, copies it, or resets it. The preview has its own selectable
lines; the source keeps visible inclusion marks while the preview is focused.

Working-tree diffs show staged and unstaged panes consistently, including a
staged-only pane taking the space. Line identities preserve selections when
context changes; rewrites refresh the source commit and its file tree.
Focused diffs support copying selected code, wrapped lines, horizontal
scrolling, and returning to the left edge as the content becomes narrower.
Partial deletions use a normalized display patch when materializing the
preview; patches used to rewrite history remain under lazygit's commands.

Validation: `go vet ./...`, `go test ./...`, offscreen `TestShots`, native
custom-patch/focused-diff tests and the full `parity/run.sh all` pass.
Full run: 239 pass, 397 fail, 35 need harness support, 17 skipped; no
regressions or panics. The 71 newly passing tests include 60 main-view cases,
5 patch-building cases and related file, commit, submodule and filter/search cases.
Harness changes were approved in O-2026-11-05-6 (range/marks only), -7 and -8.

Remaining: external diff renderers and renderer switching, unfocused-source
marks outside the focused diff pair, viewport/wrapped-row projections in the
harness, conflict-view navigation, commit-file copy menus, rename-threshold
controls and selection-command tooltips still have failing tests. Native
wrapping and horizontal scrolling are tested directly; terminal-cell origin
assertions are not yet represented by the parity projection. This does not
claim complete main-view or patch-building parity.

![Custom patch beside its source, light](review/focused-patches/custom-patch-light.png)
![Custom patch beside its source, dark](review/focused-patches/custom-patch-dark.png)
![A partial patch moved into the index](review/focused-patches/move-partial-patch-to-index.png)
![A deleted file split into two commits](review/focused-patches/split-deleted-file.png)
![Horizontal scrolling in a focused diff](review/focused-patches/focused-horizontal.png)
![Wrapped lines in a focused diff](review/focused-patches/focused-wrapped.png)

## rebase-todos: Visible interactive rebase
Commits 89faa7a/d4c563f (workflow), 5be438c (approved projection) · +29 tests · 2026-11-05

What to look at: pending todos and applied commits are separate sections; HEAD marks the stopping point. `e` starts editing, `i` quick-starts at the branch boundary, and `m` opens continue/abort/skip with mouse access from the panel header. Editing, picking, dropping, fixup message flags and reordering retain the selected range and refresh its diff. Update-ref deletion confirms first; hashless instructions show their text and cannot open commit files. Reword/amend work at the current stopping point. Conflicts select the failed todo, and commits that become empty are skipped as lazygit does.

Native light/dark captures inspected. Harness changes are limited to O-2026-11-05-9: capture updateRefs and project the actual commit range through section headers. Remaining failures belong to conflict navigation, external-editor integration, repeat-toast projection and nested revert options; no claim of complete lazygit feature parity.

![Native rebase workflow](review/rebase-todos/visible-rebase.png)
![Native rebase workflow in dark mode](review/rebase-todos/visible-rebase-dark.png)
![Todo range operations](review/rebase-todos/interactive_rebase__mid_rebase_range_select.png)
![Quick start preserves selection](review/rebase-todos/interactive_rebase__quick_start_keep_selection.png)
![Fixup message action flag](review/rebase-todos/interactive_rebase__fixup_keep_message_rebase.png)
![Moving update-ref todos](review/rebase-todos/interactive_rebase__move_update_ref_todo.png)

Full run: 268 pass (+29), 368 fail, 35 need harness support, 17 skipped; no regressions or panics. `go vet ./...`, `go test ./...`, offscreen TestShots and the real-key workflow test pass. Generated task boundaries are in TASKS.md.

## merge-conflicts-view: focused conflict navigation and continuation
Commit 10810bd (workflow), fe45782 (view), 82c8231 (approved projection) · +16 targeted tests · 2026-11-05

What to look at: configured previous/next conflict and hunk keys, selected side or both (including diff3), undo that preserves the unmerged index, and mouse actions. The entire file scrolls, and navigating reveals the selected hunk. Resolving the last conflict returns to files and offers lazygit's Continue prompt when every file is resolved. Continuing refreshes files and asks before staging subsequent build fixes. Conflict-time amend offers cancel, continue, or amend; the operation banner shows the ongoing rebase.

Native light/dark captures inspected. Targeted group: 18/19 pass; the remaining patch-move test fails on commit selection after resolution, outside this workflow. No additional harness changes.

![Whole-file navigation](review/merge-conflicts-view/conflict-navigation.png)
![Whole-file navigation in dark mode](review/merge-conflicts-view/conflict-navigation-dark.png)
![Diff3 both-hunk resolution](review/merge-conflicts-view/conflicts__pick_both_hunks_diff3.png)
![Undo resolution](review/merge-conflicts-view/conflicts__undo_choose_hunk.png)
![Continue after build fixes](review/merge-conflicts-view/branch__rebase_conflicts_fix_build_errors.png)

Full run: 288 pass (+18 from the post-oracle 270), 348 fail, 35 need harness support, 17 skipped; no regressions or panics. `go vet ./...`, `go test ./...`, offscreen shots and focused native tests pass. Generated task reports move the remaining patch-relocation selection failure to range-select; conflict-navigation is complete within this task's scope.

## shared-suggestions: suggestions and command history
Commit 78c67d0 · +6 tests · 2026-11-05
What to look at: branch suggestions and shared prompt focus; shell history filtering, editing, deletion and leading-space opt-out. Oracle-approved driver projection now verifies these six flows against lazygit. Upstream/worktree suggestions and selection after custom commands still have application gaps.

![Branch suggestions](review/shared-suggestions/branch__suggestions.png)
![Suggestion focus](review/shared-suggestions/ui__suggestions_selection_follows_the_focus.png)
![Shell history](review/shared-suggestions/shell_commands__history.png)
![Edit history](review/shared-suggestions/shell_commands__edit_history.png)
![Delete history](review/shared-suggestions/shell_commands__delete_from_history.png)
![History opt-out](review/shared-suggestions/shell_commands__omit_from_history.png)

## shared-suggestions-completion: remote suggestions and worktree creation
Commit e774e7c · +4 tests · 2026-11-05
What to look at: custom command suggestions preserve the selected branch after checkout; upstream suggestions include loaded remote branches; remote worktree suggestions exclude branches already present locally and create a tracking branch before switching into the new worktree.

![Command suggestions](review/shared-suggestions-completion/custom_commands__suggestions_command.png)
![Preset suggestions](review/shared-suggestions-completion/custom_commands__suggestions_preset.png)
![Pull with upstream](review/shared-suggestions-completion/sync__pull_and_set_upstream.png)
![Remote worktree](review/shared-suggestions-completion/worktree__new_worktree_picker_remote.png)

## custom-commands: Custom command compatibility
Commit 891670d · +8 tests · 2026-11-05
What to look at: established lazygit commands can use session metadata and paths, conditional forms, generated colored menu labels, context-filtered submenus and macOS interactive terminals. Terminal launch itself needs a manual check; automated tests exercise the shell wrapper without opening a window. Remaining parity failures concern reflog navigation and the protected menu whitespace projection.
![Selected path](review/custom-commands/custom_commands__selected_path.png)
![Commit range](review/custom-commands/custom_commands__selected_commit_range.png)
![Submodule](review/custom-commands/custom_commands__selected_submodule.png)
![Conditional prompts](review/custom-commands/custom_commands__conditional_prompts.png)
![Command submenu](review/custom-commands/custom_commands__custom_commands_submenu.png)
![Generated menu](review/custom-commands/custom_commands__menu_from_command.png)

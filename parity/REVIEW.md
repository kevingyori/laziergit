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

## branch-commits: <enter> on a branch, tag or reflog entry shows its commits (subCommits)
Commit 5d2df5f · +19 tests · 2026-11-05
What to look at: <enter> on a local branch, remote branch or tag opens its commits in the branches panel, titled "Commits (<ref>)" and starting at the top each time. Next and previous tab keys leave that view for the branches tabs, and Set author / Add co-author suggest every author seen in loaded commit lists. Native rows draw no graph, branch-head markers or Local/Remote headers yet; lazygit's tests for those still fail.
![Scroll position reset](review/branch-commits/ui__sub_commits_scroll_position_is_reset.png)
![Cherry-pick range](review/branch-commits/cherry_pick__cherry_pick_range.png)
![Cherry-pick](review/branch-commits/cherry_pick__cherry_pick.png)
![Set author](review/branch-commits/commit__set_author.png)

## reflog-tab: Reflog tab in the commits panel (] / [ switch tabs)
Commit a873a89 · +8 tests · 2026-11-05
What to look at: the commits panel has COMMITS / REFLOG tabs that keep their own selection. In the reflog, <space> opens "Checkout branch or commit", g opens the reset menu and C copies the entry for cherry-pick. <enter> shows the entry's commits in the commits panel, moving that view out of branches if it was open there. Commit files opened from it can build a patch.
![Checkout](review/reflog-tab/reflog__checkout.png)
![Reset](review/reflog-tab/reflog__reset.png)
![Cherry-pick](review/reflog-tab/reflog__cherry_pick.png)
![Patch](review/reflog-tab/reflog__patch.png)
![Nested filter](review/reflog-tab/filter_and_search__nested_filter_transient.png)
![Selected commit](review/reflog-tab/custom_commands__selected_commit.png)

## undo-redo: z undoes and Z redoes the last reflog action
Commit b33546f · +3 tests · 2026-11-05
What to look at: z asks "Undo" (soft reset after a commit, hard reset with auto-stash after a rebase or drop, checkout after a branch switch); Z asks "Redo" the same way and walks forward over earlier undos. Both refuse with lazygit's error during a rebase/merge. A checkout (also <space> on a branch) focuses branches and selects the checked-out branch.
![Checkout and drop](review/undo-redo/undo__undo_checkout_and_drop.png)
![Undo commit](review/undo-redo/undo__undo_commit.png)
![Undo drop](review/undo-redo/undo__undo_drop.png)

## worktree-management: worktree menus, removal and switching
Commit 0674fa6 · +24 tests · 2026-11-05
What to look at: w on a branch, commit, tag, stash or remote branch opens lazygit's "New worktree" menu (new branch + worktree, worktree for the branch, detached worktree) and then switches into it with branches focused; d on a worktree (or a linked worktree in Files) offers remove / remove and delete branch / and both branches, with force prompts; <space> on a branch checked out elsewhere asks "Switch to worktree"; the status row reads repo(linked-worktree) → branch; copied commits survive a worktree switch; move commit up/down no longer asks for confirmation.
![Add from branch](review/worktree-management/worktree__add_from_branch.png)
![Remove worktree and branch](review/worktree-management/worktree__remove_worktree_and_branch.png)
![Switch to worktree](review/worktree-management/worktree__associate_branch_rebase.png)
![CRUD](review/worktree-management/worktree__crud.png)
![Worktree inside repo](review/worktree-management/worktree__worktree_inside_repo.png)
![Cherry-pick across worktrees](review/worktree-management/cherry_pick__cherry_pick_across_worktrees.png)

## tags-remotes: tag and remote management, panel jump keys switch tabs
Commit 05a8fdc · +18 tests · 2026-11-05
What to look at: n in tags opens the tag name/description panel; d on tags offers local / remote / both (range selection works); tag main view shows Annotated/Lightweight tag info; remotes n/d/e/f/F (add, remove, edit, fetch, add fork); s in remote branches opens the sort-order menu with radio marks; with gui.switchTabsWithPanelJumpKeys the focused panel's number key cycles its tabs.
![tag/crud_annotated](review/tags-remotes/tag__crud_annotated.png)
![tag/delete_multiple](review/tags-remotes/tag__delete_multiple.png)
![remote/add_fork_remote](review/tags-remotes/remote__add_fork_remote.png)
![branch/sort_remote_branches](review/tags-remotes/branch__sort_remote_branches.png)

## bisect: git bisect
Commit d009a95 · +6 tests · 2026-11-05
What to look at: `b` in commits opens lazygit's Bisect menu (mark bad/good, custom terms, skip current/selected, reset); commits show `<-- bad/good/current/skipped` and `?` markers; the bar shows "Bisecting (Reset)"; finishing offers "Bisect complete" with the culprit.
![bisect/skip](review/bisect/bisect__skip.png)
![bisect/basic](review/bisect/bisect__basic.png)
![bisect/choose_terms](review/bisect/bisect__choose_terms.png)
![bisect/from_other_branch](review/bisect/bisect__from_other_branch.png)
![worktree/associate_branch_bisect](review/bisect/worktree__associate_branch_bisect.png)

## stash-panel: stash panel actions and files
Commit 9ada003, eb53745 · +14 tests · 2026-11-06
What to look at: Stash apply/pop/drop confirmations (drop works on a range), n new branch off a stash entry, r rename prompt, <enter> shows a stash entry's files (commit files view, in the commits window), d in commit files discards old file changes (refused outside local commits).
![apply](review/stash-panel/stash__apply.png)
![drop_multiple](review/stash-panel/stash__drop_multiple.png)
![create_branch](review/stash-panel/stash__create_branch.png)
![rename](review/stash-panel/stash__rename.png)
![stash_and_keep_index](review/stash-panel/stash__stash_and_keep_index.png)
![prevent_discarding_file_changes](review/stash-panel/stash__prevent_discarding_file_changes.png)

## filtering-mode: Filtering by path and author
Commit c2c1f54 · +12 tests · 2026-11-06
What to look at: <ctrl+s> Filtering menu (filter by selected file/author, enter path/author with suggestions, stop filtering), the "Filtering by '…' (Reset)" label, filtered commits/reflog/stash, the filtered file selected on entering a commit, and "Showing diff for range" in the commits main view.
![type_author](review/filtering-mode/filter_by_author__type_author.png)
![keep_same_commit_selected_on_exit](review/filtering-mode/filter_by_path__keep_same_commit_selected_on_exit.png)
![select_filtered_file_when_entering_commit](review/filtering-mode/filter_by_path__select_filtered_file_when_entering_commit.png)
![show_diffs_for_renamed_file](review/filtering-mode/filter_by_path__show_diffs_for_renamed_file.png)
![stash filter_by_path](review/filtering-mode/stash__filter_by_path.png)
![drop_commit_in_filtering_mode](review/filtering-mode/filter_by_path__drop_commit_in_filtering_mode.png)

## sync-remaining: stacked push/pull, auto-forward, range fast-forward, branch sort
Commit 897915f · +19 tests · 2026-11-06
What to look at: P/p on a branch with stacked branches below opens lazygit's Push/Pull menu (prompt listing the branches with divergence, push/pull all vs. only current, Cancel) and the force-push confirmation listing diverged branches; f (fetch) auto-forwards branches per git.autoForwardBranches; f in branches fast-forwards a range (errors as alerts, failing worktree doesn't hold up others); s in branches opens the sort-order menu; push.default=current pushes without asking for an upstream; the branch selection follows its branch across refreshes.
![push_stacked_branches](review/sync-remaining/sync__push_stacked_branches.png)
![pull_stacked_branches_update_fails](review/sync-remaining/sync__pull_stacked_branches_update_fails.png)
![fetch_and_auto_forward_branches_all_branches](review/sync-remaining/sync__fetch_and_auto_forward_branches_all_branches.png)
![fast_forward_rewritten_stack_of_branches](review/sync-remaining/sync__fast_forward_rewritten_stack_of_branches.png)
![fetch_when_sorted_by_date](review/sync-remaining/sync__fetch_when_sorted_by_date.png)
![push_and_auto_set_upstream](review/sync-remaining/sync__push_and_auto_set_upstream.png)

## diffing-copy: Diffing mode, copy menus, diff view toggles
Commit 219a0d1 · +12 tests · 2026-11-06
What to look at: W opens the Diffing menu and the "Showing output for: git diff …" banner and diffs follow the selection; y in files/commit files opens the Copy to clipboard menu; ( ) | and <c-w> change rename threshold, diff renderer and whitespace with lazygit's toasts.
![diff](review/diffing-copy/diff__diff.png)
![diff_commits](review/diffing-copy/diff__diff_commits.png)
![copy_menu](review/diffing-copy/file__copy_menu.png)
![copy_to_clipboard](review/diffing-copy/diff__copy_to_clipboard.png)
![ignore_whitespace](review/diffing-copy/diff__ignore_whitespace.png)
![cycle_diff_renderers](review/diffing-copy/diff__cycle_diff_renderers.png)

## recent-repos: Recent repositories and switching repos
Commit 1a3adc9 · +12 tests · 2026-11-06
What to look at: <c-r> recent repositories menu (name, branch, directory columns, truncation tooltips, type-to-filter), switching repos keeps each repo's focus and selections and loads its own lazygit.yml, direnv approval prompt, intro popup on first open, confirm-on-quit.
![recent_repos_with_long_names](review/recent-repos/misc__recent_repos_with_long_names.png)
![initial_open](review/recent-repos/misc__initial_open.png)
![confirm_on_quit](review/recent-repos/misc__confirm_on_quit.png)
![direnv_approves_envrc](review/recent-repos/misc__direnv_approves_envrc.png)
![custom_commands_in_per_repo_config](review/recent-repos/config__custom_commands_in_per_repo_config.png)

## submodules: Submodule views, staging, conflicts and reset
Commit c6a0604 · +10 tests · 2026-11-06
What to look at: the submodule main view (Name/Path/Url and the new submodule commits), staging a submodule only stages its commit ("Nothing to stage" alert for dirty-only), a conflicted submodule explains both sides and <enter>/<space> pick a commit, discarding a submodule offers "Stash uncommitted submodule changes and update" (and refuses mixed selections), returning from a submodule keeps the selected file and branch.
![enter](review/submodules/submodule__enter.png)
![stage](review/submodules/submodule__stage.png)
![stage_dirty_only](review/submodules/submodule__stage_dirty_only.png)
![resolve_conflict](review/submodules/submodule__resolve_conflict.png)
![reset_folder](review/submodules/submodule__reset_folder.png)
![add](review/submodules/submodule__add.png)

## range-select: Commit selection after operations
Commit 438c6fe · +8 tests · 2026-11-06
What to look at: the commits selection follows the selected commit (and range) by hash across refreshes (external commits, reverts, rebase todos); a new conflict selects the conflicted commit, moves keep the moved commit selected, drop/squash/fixup collapse to the commit left in place, pull and continuing a merge select HEAD; `*` selects the current branch's commits (also in sub-commits). The app-only "Rebase applied" and "Conflicts!" toasts are gone.
![select_commits_of_current_branch](review/range-select/branch__select_commits_of_current_branch.png)
![keep_selected_commit_after_external_commit](review/range-select/commit__keep_selected_commit_after_external_commit.png)
![swap_with_conflict](review/range-select/interactive_rebase__swap_with_conflict.png)
![outside_rebase_range_select](review/range-select/interactive_rebase__outside_rebase_range_select.png)
![quick_start_keep_selection_range](review/range-select/interactive_rebase__quick_start_keep_selection_range.png)
![cherry_pick_conflicts](review/range-select/cherry_pick__cherry_pick_conflicts.png)

## file-tree: File tree in the Files and commit-files panels
Commit 2cf77d3 · +10 tests · 2026-11-06
What to look at: while conflicts exist the Files panel shows only conflicted files (and the ones resolved since), with lazygit's filter label in the header; `<c-b>` opens the status filter menu. Space stages a resolved-but-unmerged file (enter opens the conflict view), resolving the last conflict of an operation started here offers to continue it, the hunk side picked carries to the next file, `c` in commit files checks the file out, and the rename threshold keys refresh the files from the focused diff.
![filter](review/file-tree/conflicts__filter.png)
![resolve_multiple_files](review/file-tree/conflicts__resolve_multiple_files.png)
![discard_various_changes_range_select](review/file-tree/file__discard_various_changes_range_select.png)
![checkout_file_from_commit](review/file-tree/commit__checkout_file_from_commit.png)
![rename_similarity_threshold_change](review/file-tree/file__rename_similarity_threshold_change.png)

## main-view-cursor: Selection highlight of side lists
Commit e9a87e9 · +4 tests · 2026-11-06
What to look at: the parity checks now see side lists the way lazygit draws them: active selection in the focused list, dimmed while the diff or a popup has focus, none for an empty list or a list of another panel. No app change; the renderer-driven main view is still to do.
![switch_repo_moves_the_selection](review/main-view-cursor/ui__switch_repo_moves_the_selection.png)
![toggle_whitespace_keeps_unfocused_selection_dimmed](review/main-view-cursor/ui__toggle_whitespace_keeps_unfocused_selection_dimmed.png)
![unfocused_list_hides_selection_when_emptied](review/main-view-cursor/ui__unfocused_list_hides_selection_when_emptied.png)

## options-bar: lazygit's options bar in the hint bar
Commit a58f3f8 · +5 tests · 2026-11-06
What to look at: the bottom bar lists lazygit's on-screen bindings per panel (e.g. Commit/Stash/Reset/Keybindings in Files), hides ones disabled in empty lists, and puts mode suggestions (View rebase/merge/revert options: m, Paste, bisect, custom patch) first in the accent colour.
![keybinding_suggestions_when_switching_repos](review/options-bar/ui__keybinding_suggestions_when_switching_repos.png)
![mode_specific_keybinding_suggestions](review/options-bar/ui__mode_specific_keybinding_suggestions.png)
![revert_with_conflict_single_commit](review/options-bar/commit__revert_with_conflict_single_commit.png)

# Parity log

One line per event, newest last: `START <id>`, `DONE <id>: +<n> tests`,
`SKIP <id>: <why>`, `FLAKY <test>`. See parity/AGENT.md.


START file-tree: resume interrupted implementation of lazygit file tree view models
DONE file-tree: +8 tests; native trees, directory actions and rename diffs. Full run: 44 pass, 592 fail, 35 need harness support, 17 skipped; no regressions or panics. Remaining cases need conflict-only filtering, range selection and commit-file checkout.

START lazygit-strings: branch checkout prompt wording
PROGRESS lazygit-strings: checkout-by-name title uses lazygit translation; +0 tests. checkout_by_name now reaches missing-branch confirmation; suggestions now reaches missing suggestions. Headless checks pass; 44-test ratchet unchanged.
PROGRESS lazygit-strings: branch rename prompt uses lazygit translation; +1 test (branch/rename). Vet, Go tests and offscreen shots pass; 45-test ratchet.
PROGRESS lazygit-strings: merge menu title uses lazygit translation; +1 test (branch/merge_non_fast_forward). Fast-forward case reaches disabled-option message mismatch. Vet, Go tests and offscreen shots pass; 46-test ratchet.

PROGRESS main-view-cursor: integrated focused working-tree entry and change-block navigation from parallel diff agent; +1 test (commit/stage_range_of_lines), 47-test ratchet. Parent review corrected previous-block and context navigation; simultaneous staged/unstaged panes and range projection remain separate work.
PARALLEL STOP: user requested wrap-up and instructions, not further autonomous implementation. Diff foundation integrated on main (89c239f, f94348b), 47-test ratchet verified. Search partial preserved unmerged at 376297d on t3/side-panel-search; ref navigation partial preserved unmerged at 3e15d42 on t3/ref-drilldown-navigation. Both need further native coverage and oracle-approved harness projections before parity gains can be assessed. No harness changes made in this batch.

PARALLEL DONE batch 2: integrated key-a-commits (d8cd0c4), key-t-commits (e5ef22b), key-d-localbranches (e0acce6); oracle O-2026-11-05-2 approved tooltip projection, O-2026-11-05-3 rejected (author menu keys fixed in app). Full run: 72 pass, 564 fail, 35 harness; no regressions.
DONE session: range revert, remoteBranches view (O-2026-11-05-4) with d/n, per-tab selection, squash/fixup first-commit toast, stage-all toast, ctrl+o copy. Full run: 80 pass, 556 fail, 35 harness; no regressions.

PARALLEL START batch 3 (2026-11-05), base 337d582+d83f413, 115 pass. Five worktree agents, one branch each, not merged:
main-view-cursor (Opus), range-select + the *_multiple tests (Opus), branch-commits + cherry_pick (Sonnet),
search reusing t3/side-panel-search 376297d (Sonnet), one-key popup menus key-* (Sonnet).
Integration still to do: review each branch's diff (reject changes to resources/, README.md, harness), merge one at a time,
then parity/run.sh all and commit passing.txt/TASKS.md/README. Pending oracle decisions: harness-only failures
(main/secondary titles "Unstaged changes"/"Staged changes": staged_changes_in_lower_pane, change_screen_mode_in_focused_diff,
focus_follows_when_pane_goes, focus_follows_a_pane_emptied_from_outside; clipboard file: misc/copy_confirmation_message_to_clipboard)
plus any projections the agents report. rebase-todos waits until range-select merges.
BATCH 3 search DONE on branch t3/10f85922 (a95a386), not merged. +20 tests only with a harness `search` view projection
(oracle needed; patch in parity/proposed-harness-search.patch). Remaining: nested_filter (needs subCommits),
filter_commit_files_toggle_directory (custom patch), search_scrolls_to_a_match_off_the_edge (OriginX). Conflicts expected in
app.go, actions.go, ref_browse.go/remote_branches.go (refBrowseState.all) with branch-commits; mainCursorLine() placeholder
should switch to main-view-cursor's cursor. Note: a.snap is now the filtered view of a.raw.
BATCH 3 STOPPED (shared token budget). Search done (above). one-key menus: 5 commits on t3/parity-one-key-menus, unfinished.
main-view-cursor (t3/main-view-cursor), range-select (t3/range-selection-parity), branch-commits (t3/browse-branch-commits):
no commits; any uncommitted work stays in their worktrees (`git worktree list`). Resume from there.
search harness projection approved as O-2026-11-05-5: merge t3/10f85922, apply parity/proposed-harness-search.patch,
commit with "Oracle-approved: O-2026-11-05-5", then parity/run.sh all.

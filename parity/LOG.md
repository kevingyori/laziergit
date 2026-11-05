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
START batch-3-integration: resume all five saved efforts serially on main, preserve their original branches/worktrees, apply only the user-approved O-2026-11-05-5 search projection. Search ce6cde4/a8cf243; menus 0634346; ref browsing/cherry-pick f43af52; list ranges 0604c5f. Integration fixes cover background Git reads, upstream divergence, bounded filtered range staging, commit-range rewrites and fixup selection below a branch boundary. Full native checks, offscreen visual review and complete parity regeneration in progress.
DONE batch-3-integration: all five saved efforts integrated (focused diffs d018dab); +53 parity tests from the 115-test starting ratchet. Full run: 168 pass, 468 fail, 35 need harness support, 17 skipped; no regressions or panics. go vet ./..., go test ./... and offscreen TestShots/search/ref-navigation shots pass; screenshots inspected in light/dark and saved in review/batch-3. README/TASKS/task-tests regenerated. Original saved branches and their dirty worktrees remain untouched as recovery copies; main contains the integrated versions, so do not merge them again wholesale. No push.
REMAINING batch-3: subCommits focus/content/mode-bar projection still missing (no additional harness approval used); focused main/secondary pane titles and selection ranges are not fully projected; horizontal diff scrolling, custom patch building and unimplemented multi-item operations remain in TASKS.md. Only O-2026-11-05-5 was applied. These are parity gaps, not uncommitted integration work. Fixed run.sh's report-only sed locale so non-UTF-8 diff output cannot interrupt printing the result table; scoring and test execution are unchanged.

START main-view-cursor/custom-patches: finish focused diff selection preservation and paired panes; add commit patch building and patch operations using lazygit commands. Diagnose harness projections separately and seek oracle approval before changing them.
DONE main-view-cursor/custom-patches: paired staged/unstaged and source/patch panes; line/hunk/range and whole-file patch building; preview line removal; apply/reverse/copy/reset; remove/move/split patches through lazygit commands; selection restoration and rewritten commit refresh; native wrapping/horizontal scroll/copy. Source 54fd574, approved harness/ratchet 12e0803 (O-2026-11-05-6 range/marks only, -7, -8). Full run: 239 pass (+71), 397 fail, 35 harness, 17 skipped; no regressions/panics. go vet ./..., go test ./..., offscreen TestShots and native custom/focused tests pass; light/dark and wrapped/horizontal screenshots inspected and saved under review/focused-patches. Generated parity reports refreshed; no feature documentation added to README. No push.
REMAINING focused-patches: external renderers/switching, unfocused-source marks outside the focused pair, viewport/wrapped-row harness projection, selection-command tooltips/availability, conflict-view navigation, commit-file copy menus and rename-threshold controls. Rebase-todos is now the largest generated task; it is outside this request and was not started.

START rebase-todos: complete visible interactive rebase, native sections and stopping point, action labels, todo editing/reordering and selection preservation. Baseline targeted run: 0/29 pass.
DONE rebase-todos: visible pending/applied sections, HEAD stopping point, e/i/m workflow, selection-preserving todo/range edits and moves, fixup flags, confirmed update-ref deletion, hashless instruction preview/guards, current-HEAD reword/amend, conflict stop selection and empty replay auto-skip. Feature 89faa7a/d4c563f; projection 5be438c approved by O-2026-11-05-9. Full run: 268 pass (+29), 368 fail, 35 harness, 17 skipped; no regressions/panics. go vet ./..., go test ./..., TestShots and native workflow captures pass. Light/dark captures inspected and saved under review/rebase-todos; generated reports refreshed. No feature documentation added to README. No push.
REMAINING rebase-todos: conflict-view navigation, external-editor rewording, repeat-toast projection, nested revert options projection and accordion viewport behavior remain separate work; task tests now fail at those boundaries.

START merge-conflicts-view: focused native conflict session using lazygit's mergeconflicts.State; configured hunk/conflict navigation, pick/both, undo, retained whole-file options and guarded operation continuation. No harness changes.
DONE merge-conflicts-view UI: focused conflict session, configured previous/next hunk and conflict keys, pick selected side/both (including diff3 via lazygit), undo with index reset, external-edit guard, whole-file menu, and matching operation continuation guarded by remaining unmerged files. go vet ./..., go test ./... (268-test ratchet), focused two-conflict keyboard tests and offscreen TestShots pass. Inspected /tmp/laziergit-conflict-shots/conflict.png. Targeted parity task remains 0/19 because the protected driver reports *app.conflictOverlay instead of mergeConflicts; no harness changes or new passing claims. Remaining: oracle-approved harness projection and end-to-end parity follow-through. No README changes or push.

HARNESS merge-conflicts-view: mergeConflicts projection (focus, file content, hunk range) approved as O-2026-11-05-10 and committed separately. Targeted run: 2/19 pass (conflicts/conflict_marker_size_resolve, conflicts/undo_choose_hunk), genuine; ratchet 270. go vet ./... and go test ./... pass.
REMAINING merge-conflicts-view (application bugs, not harness): after the last hunk is resolved lazygit leaves the conflict view and asks to continue (15 tests expect the confirmation popup; the app keeps its "All conflicts resolved" card); resolve_without_trailing_lf expects lazygit's alert popup; branch/rebase and rebase_conflicts_fix_build_errors expect the information view to read "Rebasing" during a rebase (modeInfo is empty); the native view draws only lines start-4..end+5 of the file instead of the whole scrollable file.

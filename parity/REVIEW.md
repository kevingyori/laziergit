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

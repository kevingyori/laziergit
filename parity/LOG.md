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

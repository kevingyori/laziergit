# Parity log

One line per event, newest last: `START <id>`, `DONE <id>: +<n> tests`,
`SKIP <id>: <why>`, `FLAKY <test>`. See parity/AGENT.md.


START file-tree: resume interrupted implementation of lazygit file tree view models
DONE file-tree: +8 tests; native trees, directory actions and rename diffs. Full run: 44 pass, 592 fail, 35 need harness support, 17 skipped; no regressions or panics. Remaining cases need conflict-only filtering, range selection and commit-file checkout.

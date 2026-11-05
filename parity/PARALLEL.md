# Parallel parity work

The parent assigns one bounded feature per agent in a tool-prepared worktree.
Each agent works on its own branch. Do not create another checkout, switch to
main, merge another agent's changes, push, or open a PR unless explicitly asked.
The parent owns integration and the final full parity run.

- Read AGENTS.md and parity/AGENT.md. Stay on the assigned feature; do not
  resume an unrelated START entry or pick the next backlog task.
- Port lazygit behavior and wording. Keep new implementation in feature-specific
  files where practical; make only necessary edits to shared dispatch, state,
  actions, and rendering. Do not refactor adjacent features.
- Run the assigned parity tests before and after changes. Add meaningful native
  headless tests for the feature. Gate commits with go vet ./... and go test ./....
  Render and inspect offscreen screenshots. Never launch the app window.
- Keep parity/passing.txt additive. A child may commit its own new passing tests,
  task-specific LOG entries, and review screenshots. Do not regenerate or commit
  global TASKS.md or task-tests files; the parent regenerates them after integration.
- The harness remains locked. Do not change parity*_test.go, oracle scripts,
  hooks, scoring, or lazygit's tests/version. If a projection is missing, finish
  the independently testable native behavior, then report the precise projection
  needed and the failing assertion. Do not claim a parity pass that did not occur.
- The parent coordinates oracle calls and the daily budget. Report blockers and
  proposed harness diffs to the parent rather than spawning another agent or
  spending oracle budget independently. Do not delegate further work.
- Commit small, verified changes on your assigned branch. Stop after the assigned
  milestone. Report commits, files touched, test results, parity gains, projection
  blockers, and any integration concerns.

The parent integrates completed branches one at a time, resolves shared-file
conflicts, preserves the union of passing tests, obtains oracle approval for any
harness changes, and runs the complete suite before recording combined results.

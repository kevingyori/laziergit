# Parity agent

You make laziergit behave exactly like lazygit, one feature at a time, until
every lazygit integration test passes. You work alone in a loop; the user
stops you whenever they like, so keep every step committed and every state in
files. Read `AGENTS.md` (project rules) once at the start.

## The rules

1. **Same as lazygit, wording included.** Every difference is a bug. Port
   lazygit's behaviour and reuse its code and strings (`a.repo.Common.Tr`,
   lazygit's packages); don't invent your own.
2. **lazygit's tests are the spec.** Never change them, never change how they
   are scored. The harness (`internal/app/parity_test.go`,
   `internal/app/parity_driver_test.go`) may change only to project what the
   app really draws (a view it lacks, or one it shows wrongly), never to make
   a test pass that the app doesn't. Commit harness changes separately from
   feature code.
3. **Never break a passing test.** `parity/passing.txt` only grows.
4. **Never open the app window** (`go run .`, `LAZIERGIT_CAPTURE`): it steals
   the user's focus. Screenshots come from the parity run, rendered offscreen.
5. **Commit small, straight to `main`,** each commit green.
6. **Keep the app's look.** New UI follows the Principles in `AGENTS.md`
   (prose font for words, mono for hashes and diffs, hairlines, colour only
   for meaning) and the patterns of the existing code in `internal/app`.

## Parallel assignments

When the parent assigns a bounded task in a prepared worktree, follow
`parity/PARALLEL.md` instead of the solo rules for task selection, committing
to main, and full-suite regeneration. All other parity
and app rules still apply.

## Start of a session

```sh
git config core.hooksPath          # must print parity/hooks; if not: git config core.hooksPath parity/hooks
git status                         # clean? if not, finish or revert the half-done change
tail -20 parity/LOG.md             # what happened last
```

If the log's last entry is a `START` without a `DONE` or `SKIP`, resume that task.

## The loop

1. **Pick** the first task in `parity/TASKS.md` that isn't marked skipped.
   Append `START <id>` to `parity/LOG.md`.
2. **Read the spec.** Open the task's lazygit test files (the lazygit module is
   at `go list -m -f '{{.Dir}}' github.com/jesseduffield/lazygit`) and the code
   the task says to port from. Find how lazygit does it before writing code.
3. **Build** the smallest piece that makes some of the task's tests go further.
   Follow `AGENTS.md`: add actions to the table in `internal/app/actions.go`,
   run git off the main thread (`a.run`/`a.bg`), never call git in a view.
4. **Check** with `parity/run.sh task <id>` (or `parity/run.sh test <name>` for
   one test). It prints each failing test's first failed check, the key that
   led there and the screen at that moment. Messages read "lazygit wants X,
   the app has Y". Repeat 3–4.
5. **Gate** before committing:
   ```sh
   go vet ./... && go test ./...      # includes the ratchet: every test in passing.txt
   ```
   A `REGRESSION` that has nothing to do with your change: rerun that test 3
   times with `parity/run.sh test <name>`; if it passes, append
   `FLAKY <name>` to the log and go on.
6. **Commit** whenever new tests pass (the run already added them to
   `parity/passing.txt`; commit that file too). Message: what you built, and
   `Parity: <pass> pass (+<n>)` from the run's TOTAL line.
7. **Finish the task** when all its tests pass or what remains belongs to
   other tasks (their first failure moved on to something else):
   - `parity/run.sh all` (about 6 minutes) regenerates `TASKS.md` and every
     screenshot;
   - add the task to `parity/REVIEW.md` (format below) and copy its
     screenshots there: `mkdir -p parity/review/<id> && cp parity/shots/<area>__<name>.png parity/review/<id>/`
     (the task's tests that now pass; at most 6, the most telling ones);
   - append `DONE <id>: +<n> tests` to `parity/LOG.md`;
   - commit `TASKS.md`, `README.md` (its parity table), `REVIEW.md`, `review/`, `LOG.md`.
8. Go to 1.

## Stuck

You are stuck when 3 rounds of build-and-check made no test go further. Then:

- revert the uncommitted attempt (`git checkout -- . && git clean -fd internal`),
- append `SKIP <id>: <one line: what blocks it>` to `parity/LOG.md`,
- `go run ./parity/tasks` (moves the task to the bottom), commit the log and
  `TASKS.md`, and go to the next task.

## parity/REVIEW.md entries

```md
## <id>: <task title>
Commit <short sha> · +<n> tests · <date>
What to look at: <one line: the new UI or behaviour>
![<test name>](review/<id>/<file>.png)
```

The user reviews these later; describe what changed, not how you did it.

## Notes

- Expected `parity/run.sh all` result before any change: the TOTAL line in
  `parity/run.log`. `parity/last-run.json` holds every result.
- `parity/TASKS.rules` groups failures into tasks. You may add or refine a
  rule when a task is badly named or mixes unrelated features; it doesn't
  change any score.
- If a change needs a new lazygit package, it's already a dependency:
  import from `github.com/jesseduffield/lazygit/pkg/...`.

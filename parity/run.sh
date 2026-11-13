#!/bin/sh
# The parity agent's one entry point for running lazygit's tests against the app.
#
#   parity/run.sh all          # every test: report, ratchet update, TASKS.md, STATUS.md, screenshots (~5 min)
#   parity/run.sh task <id>    # the tests of one task from parity/TASKS.md
#   parity/run.sh test <name>  # one or more tests by name, e.g. stash/rename
#
# Every run adds newly passing tests to parity/passing.txt and saves each
# test's last frame, rendered offscreen, to parity/shots/<area>__<test>.png.
# Never launch the app window (go run ., LAZIERGIT_CAPTURE): it steals focus.
set -eu
cd "$(dirname "$0")/.."

run() { # $1: regexp of test names ('' for all), $2: results file
	LAZYGIT_PARITY=1 LAZYGIT_PARITY_UPDATE=1 LAZYGIT_PARITY_SHOTS=parity/shots \
		LAZYGIT_PARITY_RUN="$1" LAZYGIT_PARITY_OUT="$2" \
		go test ./internal/app -run 'TestLazygitParity$' -count=1 -v -timeout 60m >parity/run.log 2>&1 || status=$?
	LC_ALL=C sed -n '/area /,/^ *n\/a:/p' parity/run.log | LC_ALL=C sed 's/^        //'
	grep -E 'REGRESSION|passing.txt:' parity/run.log | sed 's/^ *//' || true
	if [ "${status:-0}" != 0 ] && ! grep -q 'REGRESSION' parity/run.log; then
		echo "the parity run itself failed; see parity/run.log" >&2
		tail -20 parity/run.log >&2
		exit 1
	fi
	if grep -q 'REGRESSION' parity/run.log; then exit 1; fi
}

names() { # test names → anchored regexp
	printf '^(%s)$' "$(printf '%s\n' "$@" | sed 's/[.]/\\./g' | paste -sd '|' -)"
}

case "${1:-}" in
all)
	run '' parity/last-run.json
	go run ./parity/tasks
	;;
task)
	f="parity/task-tests/${2:?task id}.txt"
	[ -f "$f" ] || { echo "no task $2 (see parity/TASKS.md)" >&2; exit 2; }
	# shellcheck disable=SC2046
	run "$(names $(cat "$f"))" parity/task-run.json
	go run ./parity/tasks -show parity/task-run.json
	;;
test)
	shift
	[ $# -gt 0 ] || { echo "usage: parity/run.sh test <area/name>…" >&2; exit 2; }
	run "$(names "$@")" parity/task-run.json
	go run ./parity/tasks -show parity/task-run.json
	;;
*)
	sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac

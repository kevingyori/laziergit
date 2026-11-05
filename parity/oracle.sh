#!/bin/sh
# Oracle budget for the parity agent. Each consult of the oracle (a stronger
# model) takes one unit from today's budget and gets an entry in
# parity/ORACLE.md, which the agent fills in with the question and answer.
#
#   parity/oracle.sh take "<one-line question>"   # prints the consult id, or fails when out of budget
#   parity/oracle.sh left                         # units left today
set -eu
cd "$(dirname "$0")"
budget=$(sed -n 's/^Daily budget: *\([0-9][0-9]*\).*/\1/p' ORACLE.md)
today=$(date +%Y-%m-%d)
used=$(grep -c "^## O-$today-" ORACLE.md || true)
case "${1:-}" in
left)
	echo $((budget - used))
	;;
take)
	[ -n "${2:-}" ] || { echo "usage: oracle.sh take \"<question>\"" >&2; exit 2; }
	if [ "$used" -ge "$budget" ]; then
		echo "oracle budget for $today is used up ($budget). Skip this task and log it." >&2
		exit 1
	fi
	id="O-$today-$((used + 1))"
	printf '\n## %s: %s\n\nAnswer: (fill in)\n' "$id" "$2" >> ORACLE.md
	echo "$id"
	;;
*)
	echo "usage: oracle.sh take \"<question>\" | left" >&2
	exit 2
	;;
esac

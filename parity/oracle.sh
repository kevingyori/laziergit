#!/bin/sh
# Oracle consults for the parity agent. Each consult of the oracle (a stronger
# model) gets an entry in parity/ORACLE.md, which the agent fills in with the
# question and answer. There is no daily budget.
#
#   parity/oracle.sh take "<one-line question>"   # prints the consult id
#   parity/oracle.sh left                         # always "unlimited"
set -eu
cd "$(dirname "$0")"
today=$(date +%Y-%m-%d)
used=$(grep -c "^## O-$today-" ORACLE.md || true)
case "${1:-}" in
left)
	echo unlimited
	;;
take)
	[ -n "${2:-}" ] || { echo "usage: oracle.sh take \"<question>\"" >&2; exit 2; }
	id="O-$today-$((used + 1))"
	printf '\n## %s: %s\n\nAnswer: (fill in)\n' "$id" "$2" >> ORACLE.md
	echo "$id"
	;;
*)
	echo "usage: oracle.sh take \"<question>\" | left" >&2
	exit 2
	;;
esac

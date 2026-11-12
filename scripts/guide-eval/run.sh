#!/bin/sh
# Runs one cold agent against the review MCP server, the way a user would:
# the agent gets the config M copies and the prompt A copies, nothing else.
# Usage: run.sh <laziergit-binary> <out-dir> <name> <prompt-file> <feedback-prompt-file> [extra-prompt-file]
# The prompt files hold what A copies and what the user says after S, for the
# build under test. Needs the `claude` CLI. MODEL overrides the model (default haiku).
set -eu
bin=$1 out=$2 name=$3 extra=${6:-}
prompt=$(cat "$4")
followup=$(cat "$5")
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
mkdir -p "$out"
out=$(cd "$out" && pwd)
repo=$out/$name-repo
"$here/fixture.sh" "$repo"

# Opening the guide writes the snapshot the MCP server serves.
LAZIERGIT_KEYS="~" LAZIERGIT_CAPTURE="$out/$name-before.png" "$bin" "$repo"
ls "$repo"/.git/laziergit/review/*/snapshot.json >/dev/null 2>&1 || LAZIERGIT_KEYS="~" LAZIERGIT_CAPTURE="$out/$name-before.png" LAZIERGIT_CAPTURE_DELAY=2s "$bin" "$repo"

# What M copies.
cat > "$out/$name-mcp.json" <<X
{"mcpServers":{"laziestgit":{"command":"$bin","args":["--review-mcp","$repo"]}}}
X
[ -n "$extra" ] && prompt="$prompt

$(cat "$extra")"

empty=$out/$name-cwd
mkdir -p "$empty"
agent() {
	(cd "$empty" && printf '%s' "$1" | claude -p --model "${MODEL:-haiku}" \
		--mcp-config "$out/$name-mcp.json" --strict-mcp-config \
		--setting-sources "" --tools "" --allowedTools "mcp__laziestgit" \
		--output-format stream-json --verbose $2) > "$3"
}
agent "$prompt" "" "$out/$name-publish.jsonl"
session=$(sed -n 's/.*"session_id":"\([^"]*\)".*/\1/p' "$out/$name-publish.jsonl" | head -1)
cp "$(git -C "$repo" rev-parse --absolute-git-dir)"/laziergit/review/*/guide.json "$out/$name-guide.json" 2>/dev/null || echo "no guide published" > "$out/$name-guide.json"

# The reviewer sends feedback (c, S); the user tells the agent in one line.
(cd "$root" && go run ./scripts/guide-eval/sendfeedback "$repo")
agent "$followup" "--resume $session" "$out/$name-feedback.jsonl"
cp "$(git -C "$repo" rev-parse --absolute-git-dir)"/laziergit/review/*/events.jsonl "$out/$name-events.jsonl"
LAZIERGIT_KEYS="~" LAZIERGIT_CAPTURE="$out/$name-after.png" "$bin" "$repo"

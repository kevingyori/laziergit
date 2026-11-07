# laziestgit PoC

Run `go run . --guide /path/to/repo`, or press **Ctrl+G** inside laziergit.
This experimental review screen shares the native UI, diff renderer, lazygit
configuration and git layer. Escape returns to the regular git screen.

- **A** launches your installed, signed-in Claude Code CLI as an MCP client.
  Claude reads the snapshot using `get_review_diff`, then publishes narrative
  sections with `publish_review_guide`. It uses the remote model through your
  normal Claude Code account; no local model or additional API key is required.
- **M** copies the MCP server configuration for an external client, including
  Codex or Claude Code. Connect that config and ask: “Read the review diff and
  publish a narrative guide.” The app imports the result on auto-refresh or **r**.
  For a stable executable path, launch a built binary (`go build -o /tmp/laziestgit .`).
- **j/k** select hunks; **h/l** select sections (your configured previous/next
  item and panel keys apply). Mouse selection works too.
- **Space** toggles reviewed. **]** jumps to the next unreviewed hunk.
- **b** chooses a branch base. Empty means working tree; otherwise the scope is
  the merge base of that ref and HEAD through HEAD, excluding uncommitted edits.
- **r** reloads the snapshot. Changed reviewed hunks show a refresh marker.
- **?** lists all available keys, including diff scrolling and MCP config copy.

The initial guide groups implementation, tests, documentation/configuration,
and dependency/generated changes using filename heuristics. AI guides replace
those groups with behavior-oriented sections and short explanations. The server
rejects missing, duplicated or invented hunk IDs and stale snapshot revisions.
It exposes no git-write tools. Explanations still need human review.

The working-tree snapshot shows staged and unstaged patches separately, including
both sides of partially staged files and untracked files. Review identity includes
path, layer and patch content. Review marks are in-memory for this screen's session;
closing the guide resets them. Hunk reordering can conservatively invalidate marks.
The PoC has a 180 KB AI snapshot limit and a two-minute Claude Code timeout.
Snapshot/config/result files live in a private temporary directory until the guide
or app closes. The guide snapshot stays fixed until **r**; automatic repository
refresh can import an agent's answer without changing the snapshot beneath it.

`--guide-mcp <session-directory>` runs the stdio MCP server without opening a
window. It implements initialize, ping, tools/list and tools/call. The two tools
read `snapshot.json` and atomically publish `guide.json` in that directory.

No commit planning, PR comment publishing, semantic move detection, persistent
review history, or AI streaming is included yet.

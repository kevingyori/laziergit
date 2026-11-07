# laziestgit PoC

Run `go run . --guide /path/to/repo`, or press **~** inside laziergit.
This experimental review screen shares the native UI, diff renderer, lazygit
configuration and git layer. **~** switches in either direction, keeping review
marks and each screen's position. An unchanged round trip restores the exact
staging selection. A new selection maps back to its file and line in staging.
Ctrl+G keeps lazygit's jump-to-file behavior.

- **A** launches your installed, signed-in Claude Code CLI as an MCP client.
  Claude reads the snapshot using `get_review_diff`, then publishes narrative
  sections with `publish_review_guide`. It uses the remote model through your
  normal Claude Code account; no local model or additional API key is required.
- **M** copies the MCP server configuration for an external client, including
  Codex or Claude Code. Connect that config and ask: “Read the review diff and
  publish a narrative guide.” The app imports the result on auto-refresh or **r**.
  For a stable executable path, launch a built binary (`go build -o /tmp/laziestgit .`).
- In the story list, **j/k** select hunks across section boundaries; **h/l**
  jump between sections. Your configured previous/next item and panel keys apply.
- **Enter** or **0** focuses the code: **j/k** moves by line and **h/l** by hunk.
  **Escape** returns to the story; Escape there returns to git.
- **Space** toggles reviewed without staging or advancing. **[/]** selects the
  previous/next unreviewed hunk across sections, with wrap (configured tab keys).
- **J/K** scrolls code without changing selection; **H/L** scrolls horizontally.
  The existing diff-wrap setting is honored. **e** expands/collapses the section
  note (your configured edit key). Mouse selection works too.
- **b** chooses a branch base. Empty means working tree; otherwise the scope is
  the merge base of that ref and HEAD through HEAD, excluding uncommitted edits.
- **r** reloads the snapshot. Changed reviewed hunks show a refresh marker.
- **?** lists all available keys, including diff scrolling and MCP config copy.
  **!** opens status/error details; long errors stay out of the hint bar.

The initial guide groups implementation, tests, documentation/configuration,
and dependency/generated changes using filename heuristics. AI guides replace
those groups with behavior-oriented sections and short explanations. The server
rejects missing, duplicated or invented hunk IDs and stale snapshot revisions.
It exposes no git-write tools. Explanations still need human review.

The working-tree snapshot shows staged and unstaged patches separately, including
both sides of partially staged files and untracked files. Review identity includes
path, layer and patch content. Review marks are in-memory for this screen's session;
switching to git preserves them; exiting the app resets them. Hunk reordering can conservatively invalidate marks.
The PoC has a 180 KB AI snapshot limit and a two-minute Claude Code timeout.
Snapshot/config/result files live in a private temporary directory until the guide
app closes or repository changes. The guide snapshot stays fixed until **r**; automatic repository
refresh can import an agent's answer without changing the snapshot beneath it.

`--guide-mcp <session-directory>` runs the stdio MCP server without opening a
window. It implements initialize, ping, tools/list and tools/call. The two tools
read `snapshot.json` and atomically publish `guide.json` in that directory.

Sections render as continuous diff streams with old/new line numbers. Chroma
highlights whole old/new file versions off the UI thread, shared across hunks.
The sources are pinned to the diff's blob hashes, including verified worktree
content for new/unstaged files. Syntax colors overlay the existing diff backgrounds.
Unsupported, binary, changed-since-snapshot and >1 MB sources fall back to plain
text. The per-snapshot highlight budget is 256 file versions / 32 MB. Regular
laziergit diff rendering is unchanged.

No commit planning, PR comment publishing, semantic move detection, persistent
review history, or AI streaming is included yet.

## Deferred: generation progress and provider choice

Requested for later; no implementation in this note.

- **Current progress:** only the footer text “Writing AI guide…”, not a progress
  bar, elapsed timer or streamed stage updates. A changed-since-reviewed message
  can currently override that text. Keep generation visible independently of the
  selected hunk's review state.
- **Progress UX:** an indeterminate activity indicator, elapsed time and truthful
  phases (starting client, reading context, drafting, validating, ready). Only
  show a percentage when the backend exposes measurable progress. Keep reviewing
  available, with cancellation, retry and accessible error details.
- **Current providers:** A launches the hard-coded Claude Code CLI. There is no
  in-app provider/model selector. External MCP clients, including Codex, can
  connect manually; the copied JSON config may need conversion to their format.
- **Provider choice:** introduce a generation adapter for Claude Code, Codex and
  other compatible clients, with explicit provider/model/effort settings and
  remembered defaults. Reuse each client's existing authentication and the same
  validated narrative-guide MCP contract. Show which provider is running; do not
  silently switch clients or fall back to heuristic buckets as if they were an
  AI-authored story.

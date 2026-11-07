# laziestgit PoC

Run `go run . --guide /path/to/repo`, or press **~** inside laziergit.
This experimental review screen shares the native UI, diff renderer, lazygit
configuration and git layer. **~** switches in either direction, keeping review
marks and each screen's position. An unchanged round trip restores the exact
staging selection. A new selection maps back to its file and line in staging.
Ctrl+G keeps lazygit's jump-to-file behavior.

- **A** first asks one optional line: what the change was for. It is remembered
  for this review scope. It then launches your installed, signed-in Claude Code
  CLI as an MCP client (cold-reader mode, labelled *inferred*). Claude reads the
  snapshot with `get_review_diff`, resolves doubts with `read_file`/`search`, and
  publishes with `publish_review_guide`. It uses your normal Claude Code account.
- **M** copies the MCP server configuration (`--review-mcp <worktree>`) for an
  external client such as Codex or the coding agent that wrote the change
  (author mode: publish with `"provenance": "author"`). The app imports the
  result on auto-refresh or **r**. For a stable executable path, launch a built
  binary (`go build -o /tmp/laziestgit .`).
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
and dependency/generated changes using filename heuristics. That is only the
offline fallback: AI guides replace it with behaviour-oriented sections that
mix code, tests and supporting edits across files. Explanations still need
human review.

### Guide contract

- `get_review_diff` returns hunks, the revision, instructions, a worked
  weak/better example, and context: scope, branch, commits in range (recent
  history in working-tree scope) and the optional task.
- `read_file(path, side, start, end)` reads either side as of the snapshot:
  changed files by their diff blob hashes, unchanged files from the snapshot
  trees, unstaged files only while their blob hash still matches. At most 400
  lines per call. `search(pattern, path_glob)` is `git grep` on the new side,
  at most 200 matches; in working-tree scope it reads the live tree. Both share
  a 40-call budget per server process. No shell, git writes or network.
- `publish_review_guide` takes `provenance` (author or inferred), `overview`
  (purpose, not_done) and sections with title, kind (core/supporting/mechanical),
  why, how, hunk_ids, up to three anchored `notes` and an optional diagram.
  Summary-only guides still validate. Rejected: missing, duplicated or invented
  hunk IDs, stale revisions, identifier-only titles, notes opening with
  Check/Ensure/Verify/Make sure/Consider/Confirm, assumptions or questions
  without evidence, note lines outside the hunk's new side, more than one
  question per section or three per guide.
- The UI shows provenance and why above each section (**e** adds how and the
  overview's not-done line). Notes appear under the line or hunk they anchor.

### Comments, threads and diagram proposals

Feedback lives in `$GIT_DIR/laziergit/review/<scope>/` (append-only journal,
cross-process lock), never in the working tree, and survives restarts. Nothing
is posted to a PR.

- **c** comments on the selected code line (deleted lines anchor the old side).
  Comments are private drafts until **S** sends every draft over MCP.
- **f** lists feedback with status (draft, sent, agent acknowledged/addressed/
  asks/declined, resolved, moved, outdated). Its actions: open the thread
  inspector, edit or delete a draft, re-anchor a draft to the selected line,
  convert it to a general note, reply to agents, resolve or reopen.
  In the inspector **r** replies; replies are sent immediately and reopen the
  thread for agents. Only the reviewer resolves.
- Outdated drafts (their line can't be found unambiguously after the code
  changed) block **S** until re-anchored, converted or deleted.
- Agents call `get_review_feedback(since)` with a cursor and
  `reply_to_feedback(item_id, kind, body)`. They never see drafts.
- **D** shows the section's agent diagram in an embedded godraw editor above
  the code. **v** toggles the read-only agent baseline and your proposal.
  Your semantic changes (labels, added/removed nodes, rewired arrows) are
  purple: dashed while a draft, solid once sent. Moving and resizing is layout
  and isn't sent. Keys: **j/k/h/l** select shapes, **Enter** jumps to the
  shape's hunk, **e** edits its label, **H/J/K/L** move it, **<**/**>** resize,
  **s/a/n/p/b** tools, **d** deletes, **u/U** undo/redo, **=/+/-** fit/zoom.
  Deleting a diagram draft also discards its working document.

Review marks persist per worktree (`reviewed.json`); the guide reopens on
the last base chosen with **b**.

The working-tree snapshot shows staged and unstaged patches separately, including
both sides of partially staged files and untracked files. Review identity includes
path, layer and patch content. Review marks are kept per worktree;
they persist across restarts. Hunk reordering can conservatively invalidate marks.
The PoC has a 180 KB AI snapshot limit and a five-minute Claude Code timeout. The guide snapshot stays fixed until **r**; automatic repository
refresh can import an agent's answer without changing the snapshot beneath it.

`--review-mcp [worktree]` runs the stdio MCP server for the review the app last
opened in that worktree (bound at server start); `--guide-mcp <review-directory>`
serves one review directory. Both implement initialize, ping, tools/list and
tools/call, read `snapshot.json`, and atomically publish `guide.json`.

Sections render as continuous diff streams with old/new line numbers. Chroma
highlights whole old/new file versions off the UI thread, shared across hunks.
The sources are pinned to the diff's blob hashes, including verified worktree
content for new/unstaged files. Syntax colors overlay the existing diff backgrounds.
Unsupported, binary, changed-since-snapshot and >1 MB sources fall back to plain
text. The per-snapshot highlight budget is 256 file versions / 32 MB. Regular
laziergit diff rendering is unchanged.

No commit planning, PR comment publishing, semantic move detection, agent
push notifications/long-poll, or AI streaming is included yet.

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

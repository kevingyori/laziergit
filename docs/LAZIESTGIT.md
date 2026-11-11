# laziestgit PoC

Run `go run . --guide /path/to/repo`, or press **~** inside laziergit.
This experimental review screen shares the native UI, diff renderer, lazygit
configuration and git layer. **~** switches in either direction, keeping review
marks and each screen's position. An unchanged round trip restores the exact
staging selection. A new selection maps back to its file and line in staging.
Ctrl+G keeps lazygit's jump-to-file behavior.

- **M** copies the MCP server configuration (`--review-mcp <worktree>`) once
  per coding agent — Claude Code, Codex, or the agent that wrote the change.
  For a stable executable path, use the installed `laziergit` command or a
  built binary (`go build -o /tmp/laziestgit .`), not `go run`.
- **A** copies a prompt to paste into that agent: read the diff with
  `get_review_diff`, explore with `list_files`/`search`/`read_file`, and
  publish with `publish_review_guide` — `"provenance": "author"` if it wrote
  the change, otherwise *inferred*. The app runs no model itself; it imports
  the guide on auto-refresh or **r**.
- In the story list, **j/k** select hunks across section boundaries; **h/l**
  jump between sections. Your configured previous/next item and panel keys apply.
- **1/2/3** (your `jumpToBlock` keys) focus the story, the code and the
  section's diagram; an open diagram stays open while the story or code has
  the keys, and Escape from the story returns to git.
- **Enter** or **0** focuses the code: **j/k** moves by line and **h/l** by hunk.
  **Escape** returns to the story; Escape there returns to git.
- **Space** toggles reviewed without staging or advancing. **[/]** selects the
  previous/next unreviewed hunk across sections, with wrap (configured tab keys).
- **J/K** scrolls code without changing selection; **H/L** scrolls horizontally.
  The existing diff-wrap setting is honored. The section's explanation is
  pinned above its code (expanded by default; **e**, your configured edit key,
  collapses it), and a bar names the current file once its header scrolls away.
  Mouse selection works too.
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

- `get_review_diff` returns the revision, instructions, a worked weak/better
  example, context (scope, branch, and commits in range — recent history in
  working-tree scope) and one index line per hunk
  (path, header, +/− counts, first changed line), without diff text.
  `read_hunks(ids)` returns diffs, about 24 KB per call, naming any it left
  out. Agents read what they need; no diff size limit applies. (A single
  response with the whole diff exceeded Claude Code's MCP output limit on
  larger reviews, and the guide was never published.)
- `list_files(path_glob)` names the repository's files as of the snapshot
  (including files new in the change), at most 1000.
- `read_file(path, side, start, end)` reads either side as of the snapshot:
  changed files by their diff blob hashes, unchanged files from the snapshot
  trees, unstaged files only while their blob hash still matches. At most 400
  lines per call. `search(pattern, path_glob)` is `git grep` on the new side,
  at most 200 matches; in working-tree scope it reads the live tree. Both share
  a 40-call budget per server process. No shell, git writes or network.
- `publish_review_guide` takes `provenance` (author or inferred), `overview`
  (purpose, not_done) and sections with title, kind (core/supporting/mechanical),
  why, how, hunk_ids, up to three anchored `notes`, up to three `media`
  pictures and an optional diagram (below). Notes, diagram nodes and messages
  link to a hunk and a validated new-side line range (`line`, `end_line`).
  why, how and the overview may use a small HTML subset (b, i, code, a, p,
  ul/ol/li, pre) rendered as native rich text; nothing is executed or fetched.
  Media are repository images as of the snapshot, base64 data URLs or SVG
  (SVG `<text>` isn't drawn), decoded off the UI thread.
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
goes to GitHub unless you press **G**.

- **c** opens a comment composer under the selected code line (deleted lines
  anchor the old side) — no dialog; the code stays visible and scrollable.
  **Enter** saves, **Shift+Enter** adds a line, **Escape** cancels. Draft edits
  and replies use the same composer under their thread's line.
  Comments are private drafts until **S** sends every draft over MCP. Threads
  show under their line with their state and the latest reply; click one, or
  press **f** on its line, to open it.
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
- **G** publishes unresolved line comments and general notes to the branch's
  GitHub pull request as a **pending review** (through `gh`), which you then
  submit on GitHub. It needs a branch review (**b**) whose HEAD is the PR's
  pushed head, lists what it leaves out (diagram proposals always stay local;
  outdated, working-tree, resolved and already published items) and asks
  before posting. Published items are marked "on GitHub".

### Diagrams

Agents may attach one diagram per section, which **D** opens in the embedded
godraw editor above the section's code (half the height; the code stays
visible below). Two kinds:

- **flow**: up to 24 nodes (kinds component, function, data, external,
  decision, actor, note), 40 edges (styles call, data, async) and 6 groups,
  laid out left to right by rank with each group as a lane.
- **sequence**: 2–10 participants with dashed lifelines and up to 30 ordered
  messages (`reply` draws a dashed return).

Nodes, participants and messages link to a section hunk and optionally a
new-side line range (validated). Linked shapes are blue. Selecting one — click,
or **j/k/h/l** — highlights its lines in the code below and names them under
the canvas;
**Enter** moves the keys to that code with the diagram still visible, and
reading code selects the shape linked to the nearest line. **Escape** returns
to the diagram, and again to the code view.

godraw's own tools sit in a compact column on the canvas: select, arrow, box,
text, note and draw tools, a style panel for the selection (toggle), undo and
zoom. Mouse dragging, double-click label editing and bound arrows are godraw's.
Keys: **e** edits a label, **H/J/K/L** move, **<**/**>** resize, **s/a/n/p/b**
tools, **d** deletes, **u/U** undo/redo, **=/+/-** fit/zoom.

**v** toggles the read-only agent diagram and your proposal. Your semantic
changes — labels, added or removed nodes and participants, rewired arrows,
messages dragged onto another lifeline, and moved nodes, participants and
messages (where a box sits is part of what it says) — are purple: dashed while
a draft, solid once sent. Resizing alone isn't a change. Deleting a
diagram draft also discards its working document.

Review marks persist per worktree (`reviewed.json`); the guide reopens on
the last base chosen with **b**.

The working-tree snapshot shows staged and unstaged patches separately, including
both sides of partially staged files and untracked files. Review identity includes
path, layer and patch content. Review marks are kept per worktree;
they persist across restarts. Hunk reordering can conservatively invalidate marks.
The guide snapshot stays fixed until **r**; automatic repository
refresh can import an agent's answer without changing the snapshot beneath it.

`--review-mcp [worktree]` runs the stdio MCP server for the review the app last
opened in that worktree (bound at server start); `--guide-mcp <review-directory>`
serves one review directory. Both implement initialize, ping, tools/list and
tools/call, read `snapshot.json`, and atomically publish `guide.json`.

Sections render as continuous diff streams with old/new line numbers. The guide
appears as soon as the diff is parsed; Chroma then highlights whole old/new
file versions off the UI thread (one `git cat-file --batch`, files lexed in
parallel) and colour fills in without moving the selection.
The sources are pinned to the diff's blob hashes, including verified worktree
content for new/unstaged files. Syntax colors overlay the existing diff backgrounds.
Unsupported, binary and changed-since-snapshot sources fall back to plain text;
there is no size cap. Regular laziergit diff rendering is unchanged.

No commit planning, syncing GitHub replies back, semantic move detection, agent
push notifications/long-poll, or AI streaming is included yet.

## Future: tooltips and a settings window (lazier and laziest)

Requested for later, for both screens:

- **Hover tooltips on every button** that say what the action does (and its
  key), generated from the same action table that drives keys, the hint bar
  and `?`.
- **An interactive settings window** for editing the lazygit config file:
  browse and change options with descriptions and validation, keybindings
  included, writing the user's config.yml.

## Generation happens in the agent

The app no longer launches a model: **A** copies a prompt for whichever
MCP-connected agent you use, so provider, model and effort are that agent's
settings and its progress shows there. The app only notices the published
guide. A later addition could show that an agent is reading (tool calls arrive
at the MCP server) without claiming progress it can't measure.

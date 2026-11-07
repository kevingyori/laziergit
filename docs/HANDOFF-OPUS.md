# Handoff: laziestgit PoC

Take over this exact checkout and branch. The user asked to transfer **all the work
from this conversation** to a new top-level T3 thread on **Claude Opus 5.5, medium**.
Continue implementation autonomously; this is not another review-only assignment.
The predecessor will stop editing after launching you.

## Workspace and rules

- Worktree: `/Users/kevingyori/.t3/worktrees/laziergit/t3-f6f328c9`
- Branch: `t3/ideate-linear-diff-guides`
- Read `AGENTS.md`. Keep lazygit parity, root-owned keys, background git calls,
  native MyGo/GPU/no webview, semantic colour, prose/mono hierarchy, and keyboard
  equivalents for pointer actions. Never add feature docs to README.
- Keep commits small and working. User explicitly said “btw you can commit to keep
  this poc tidy”. No PR/push/merge requested. Do not edit parity harness/oracle files.
- Latest explicit authorization: “you can copy godraw, it's fine”. Do NOT reopen a
  licensing-permission loop or substitute an external viewer for the embedded editor.
  Upstream provenance and asset notices are retained in `internal/godraw/UPSTREAM.md`.
- Parent T3 thread, if history is needed: `8697b89e-18b8-4023-8d5f-07fa8e106d1b`.
  Read it with t3_thread_read before discussing it. This document is enough to start.

## Product intent and user preferences

laziergit is the git client, laziestgit the **story-based understanding/review view**.
The user likes Linear-inspired semantic walkthroughs and wants to understand what
an agent did, why, and whether it matches the task without losing lazygit muscle
memory. They specifically rejected implementation/tests/dependencies as the final
information architecture. Those buckets are ONLY an offline fallback; an AI guide
must group arbitrary cross-file hunks by behavior/intent, including relevant tests.

Priorities agreed after a Claude Opus 5.5 medium review:

1. `j/k` dominant, `h/l` next, `[/]` for review progress. In story: hunks across
   sections, section jumps, previous/next unreviewed. Enter focuses code: lines and
   hunk jumps. Use lazygit config bindings.
2. `~` toggles lazier/laziest, retaining selection, scroll and review marks. Ctrl+G
   belongs to lazygit jump-to-file. Compact chrome and continuous section diffs,
   line numbers and syntax highlighting.
3. Notes should explain purpose and mechanism, resolve doubts using evidence,
   avoid paraphrasing visible code and generic “Check/Ensure/Verify” homework.
   The author agent knows the task; the cold-reader A invocation currently doesn't.
4. Agent-generated diagrams should explain the code **inside the app using godraw**.
   Reviewers must be able to edit them as feedback. The agent baseline and human
   proposal must be unmistakably different. Purple proposal changes, dashed drafts
   and solid sent changes; green/red stay for actual code diffs.
5. Code comments must get back to agents. Optional PR publishing is a possible
   later export, never automatic. Local drafts until explicit send, agents can
   acknowledge/report addressed but only humans resolve. Persist across restarts
   and keep outdated separate from resolved.
6. User asked about progress bars and choosing Codex/other providers, then explicitly
   said **do not implement now, just note for later**. These are noted at the end
   of docs/LAZIESTGIT.md. Do not expand this slice into provider selection/progress UX.

## Read these design/research documents

- `docs/LAZIESTGIT-DIRECTION.md`: lead Claude review, goals, navigation and source
  research. Treat opinions as recommendations; user accepted direction with the
  overriding requirement of embedded editable godraw.
- `docs/LAZIESTGIT-REVIEW-TRANSPORT.md`: lead Claude's detailed transport and diagram
  proposal, competitors, data model, flows and acceptance tests. Many parts remain
  a proposal, not implemented. No need to contact maintainers (user approved copying).
- `docs/research/laziestgit-x-notes.md`: supporting Grok research. Rate-limited and
  sparse; do not overstate it as validated broad sentiment.
- `docs/LAZIESTGIT.md`: usage notes, currently needs updating for this new slice.

## Committed history

- `0d3b9fa`: original MCP-powered guide PoC.
- `d31a055`: direction and X research docs.
- `20d0e23`: continuous diff, navigation, stateful toggle, full-blob Chroma highlighting.
- `80ac3db`: transport/editable-diagram research proposal.
- `9f0ea22`: deferred provider/progress notes.
- `fc4f9f9`: copied actual godraw source at upstream `0371738...`, relocated under
  `internal/godraw`, preserving source/tests/assets notices. A thin EmbeddedView
  suppresses global shortcuts/canvas focus and delegates keys to our root. Copied
  upstream tests pass. This is already committed separately.
- A WIP integration checkpoint will follow this document; inspect HEAD/log for it.

## Current implementation (integration is WIP, not fully finished)

Earlier working features live in internal/app/guide*.go:
- Immutable diff snapshot, exact hunk coverage and revision validation.
- A spawns authenticated Claude Code as MCP client; M copies client config.
- Continuous section stream, compact story tree, old/new line gutters.
- Whole-file syntax tokenization by blob hash with verified worktree fallback,
  not fragment lexing; tests cover multiline strings spanning omitted context.
- `~` maps file/layer/line and keeps state. Offscreen/native captures verified.

New uncommitted-at-handoff integration (checkpoint it preserves):

### Persistent feedback store: internal/reviewbridge

- `feedback.go`: append-only events.jsonl, monotonic cursor, file lock, fsync,
  torn-last-line recovery. Draft comments and diagram proposals, send batches,
  agent replies, human resolve/reopen. No git writes, no network.
- `lock.go`, `lock_unix.go`, `lock_windows.go`: cross-process locks; Windows
  reviewbridge cross-compile passed.
- `anchors.go`: preserve original anchor and derive current/moved/outdated by
  exact hunk fingerprint/unique side-specific quote; ambiguity stays outdated.
- `diagram.go`: bounded graph schema (1–12 anchored/context nodes, <=24 edges),
  semantic diagram delta and atomic native document write.
- `discovery.go`: per-worktree store in `$GIT_DIR/laziergit/review/<scope-hash>/`;
  active.json selection, stable repo-bound discovery.
- `mcp.go`: Section optionally has Diagram; publication validates section anchors.
  Added get_review_feedback(since) and reply_to_feedback(item_id,kind,body).
  Drafts never appear to agents; no agent resolve tool. Client name comes from
  initialize clientInfo. Existing get_review_diff/publish_review_guide preserved.
- main.go supports `--review-mcp [repo-path]` in addition to --guide-mcp <dir>.
  It binds to the active review **at server startup**.

### UI integration

- `guide_feedback.go`: `c` comments on selected line (deletions anchor old side),
  private drafts saved immediately; `S` confirms/sends a batch; `f` feedback menu,
  thread details, edit/delete drafts, human resolve/reopen, gutter markers.
- `guide_diagram.go`: actual godraw doc/editor/native canvas; deterministic layout
  from MCP graph, bound arrows, contextual/code anchors. Separate baseline .tldr
  bytes and editable working proposal .tldr. Semantic changes include added,
  removed, renamed and rewired graph elements; layout-only changes are saved but
  aren't sent as semantic feedback. Both native documents go with sent proposals.
- `D` toggles diagram/code; diagram and code context are visible together now.
  `v` baseline/proposal (baseline read-only), `e` edit selected label, `j/k/h/l`
  select shapes, Enter jumps to selected shape's hunk; `s/a/n/p/b` tools,
  `d` delete, `u/U` undo/redo, `=/+/-` fit/zoom. Root action table owns these.
  Pointer/double-click text editing uses godraw's text field with focus exception
  and Escape/Cmd+Enter handled in vendored embedded adapter.
- guide.go/guide_ai.go now store sessions durably instead of removing on close;
  round snapshots persisted; guide publication mtime permits same-revision updates.
  M copies stable --review-mcp repo config. A's own config stays scope-specific.
  Prompt advertises optional diagrams. Provider selection is still hard-coded A.
- overlays.go lets embedded text editing keep focus. Regular lazier isn't affected.

## Verification already performed

- `go test ./internal/godraw/...` passed (copied upstream doc/editor/app/geo tests).
- `SHOT_DIR=/tmp/laziergit-review-shots go test ./internal/app ./internal/reviewbridge
  -run 'TestGuide|TestFeedback|TestDiagram' -count=1` most recently passed.
- `go vet ./...` most recently passed.
- `GOOS=windows GOARCH=amd64 go test ./internal/reviewbridge -c
  -o /tmp/laziergit-reviewbridge.test.exe` passed.
- New tests: comment draft privacy -> send -> agent addressed -> human resolve
  boundary -> restart/discovery; old-side deletion anchor; godraw label editing
  -> proposal -> native .tldr -> sent delta -> no duplicate layout-only feedback;
  same-revision diagram republication; 24 concurrent journal writers; torn-tail
  recovery; invalid diagram anchors; unambiguous moved vs outdated anchors.
- Full `go test ./...` was started, including six-minute parity ratchet, BEFORE
  the most recent layout/anchor refinements. Its final result/stopping status is
  recorded in the handoff checkpoint notes below. Re-run the full gate for final code.
- New slice has NOT yet had a real GPU-window capture. It has offscreen actual
  godraw/UI screenshots (listed below), not HTML facsimiles.

## Immediate next work / known gaps

1. Review integration, fix bugs and run full checks. Update usage docs (README
   remains untouched), capture a real native window with an actual thematic guide
   and diagram, and show updated screenshots. Keep tests offline.
2. Finish intuitive proposal state. Added/renamed nodes are purple; sent semantic
   match switches to solid. Deleted nodes are listed as operations and visible in
   baseline mode. Rewired arrow highlighting currently compares raw endpoints,
   not all binding changes; semantic payload uses bindings. Check/fix that mismatch.
3. Deleting a diagram draft in the feedback menu currently removes the journal
   item but NOT its proposal .tldr working file. Reset/delete the matching file or
   otherwise avoid showing an untracked proposal after its draft is deleted.
4. Outdated draft comments block S (by design), but the menu lacks an explicit
   re-anchor-to-selected-line / convert-to-general-note flow. Safely moved drafts
   can be edited/adopted; add a clear action for genuinely outdated drafts.
5. Keyboard parity for godraw mouse actions: selection, label edits, tools and
   delete/undo are wired, but keyboard nudge/resize isn't yet in the root table.
   NudgeSelection exists on the embedded adapter and is tested directly. Ensure
   every supported mouse affordance has an appropriate keyboard equivalent.
6. Persistence is real for feedback/diagrams/rounds; reviewed marks still in memory.
   Several proposal-doc operations could be tidier (bounds, journal growth,
   concurrent app instances). Agent long-poll/listener presence/delivery tracking
   are NOT implemented; current transport is cursor-based MCP pull + reply.
7. Feedback thread text currently reuses showError/errorOverlay as an information
   popup. Consider a proper thread inspector and replies UX instead of error styling.
8. No PR publishing yet. Default should be explicit user action creating a pending
   PR review only for eligible committed/pushed/current anchors; do not post without
   user authorization. Diagram feedback remains local/MCP unless explicitly exported.
9. AI notes still need author/cold-reader contract/context improvements from direction
   doc. Current A has no source-read tool, only diff and guide publish. Do not mistake
   a hand-authored example for the cold-reader prompt being fixed. On a larger diff
   Claude sometimes returns without publishing; stdout diagnostics now preserved
   and ! opens details. Improve deliberately without implementing deferred provider UI.
10. Docs/readme safety: no app feature docs in README; no parity harness changes.

## Artifacts

All under ignored build/ or /tmp; the worktree itself stays tidy when committed.
- `/tmp/laziergit-review-shots/review-diagram-proposal.png`
- `/tmp/laziergit-review-shots/review-diagram-baseline.png`
- `/tmp/laziergit-review-shots/review-diagram-light.png`
- `/tmp/laziergit-review-shots/review-comment.png`
- `/tmp/laziergit-review-shots/review-feedback.png`
- `build/laziestgit-review-v2/index.html`: earlier compact/syntax screenshots.
- `build/proper-review/index.html`, guide.json, snapshot.json, provenance.json,
  story-1.png … story-4.png: a **real story-based review** of commit20d0e23, authored
  by Codex with implementation context and published through the real MCP handler
  into an isolated copy of the real repo. 44/44 coverage; tests and deps are mixed
  into thematic sections. Its separate diagram was explicitly a concept preview
  BEFORE current godraw integration, not an app feature at the time.
- `build/godraw-source/`: upstream shallow clone used for import. Not a worktree
  needing handoff; application code is copied and committed in internal/godraw.

## User communication and delivery

User prefers action and concise updates, not repeated confirmation. They want to
see a proper review, not generic fallback screenshots. Show actual native screenshots
when possible, clearly label fixtures/author-mode versus automatic A generation.
For requested visuals use T3 html_preview then html_render (no server); artifact
paths and PNGs can also be linked. No need for a new research/delegation round
unless the user asks. You are the new main thread and own completion from here.

## Final predecessor checkpoint

The in-flight full `go test ./...` finished successfully: internal/app took
344.806 s, including the parity ratchet; all packages and imported godraw tests
passed. It compiled before the latest anchor/layout refinements, which separately
passed the targeted guide/feedback/diagram suite and vet described above. There are
no remaining test processes, native app windows or servers owned by this thread.
The integration and this handoff are committed as a checkpoint; the checkout is
clean at transfer. The source thread stops editing once the new thread is launched.

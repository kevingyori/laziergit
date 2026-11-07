# laziestgit — review transport and editable diagrams

Status: proposal (research pass, 2026-11-07). It covers how review feedback (line
comments and user-edited diagrams) travels between the reviewer, coding agents and,
optionally, a GitHub PR. It builds on `docs/LAZIESTGIT-DIRECTION.md` (the guide,
navigation and density decisions) and does not reopen them. The decisions come first,
then the mechanism, the journeys, the next slice and the open questions. Evidence and
sources are at the end. Every competitor claim is marked **firsthand** (read in their
source, docs or issues today) or **inference** (my reading).

---

## 0. The decision on one page

1. **The source of truth is a local review store in the git dir, not GitHub, not the
   agent's chat, and not a temp directory.** It lives at
   `$(git rev-parse --git-dir)/laziergit/review/`, one per worktree. It is never in the
   work tree and never committed. Agents never touch git through it. All facts go in an
   **append-only event log** (`events.jsonl`) that the app and any number of MCP server
   processes append to under a file lock. Everything else is derived from it, or is a
   file that only the app writes (drafts, marks).
2. **The transport to agents is MCP, served by laziergit itself and bound to the repo,
   not to a session.** `laziergit --review-mcp` finds the repo from its working
   directory, so one user-scope config (`claude mcp add laziestgit -- laziergit
   --review-mcp`) works for the author agent, a fresh agent next week and `A`. Agents
   **pull** feedback with a cursor (`get_review_feedback`) and **block** on new feedback
   with a bounded long poll (`wait_for_feedback`). Push notifications (Claude Code
   channels) are an optional accelerator, not the mechanism.
3. **Human feedback is pending until you send it.** Comments and diagram edits are
   local drafts that agents cannot see. One key sends every draft as a batch, like
   GitHub's pending review or Crit's "Finish Review". After that, each item is still
   tracked on its own: delivered, acknowledged, claimed addressed (the agent's word),
   then resolved (only you can resolve).
4. **Review rounds are first-class.** Each snapshot you review becomes a numbered,
   persisted round. Anchors carry the round, path, layer, side (old/new), lines,
   hunk fingerprint and the anchored text. A new round re-anchors each item as
   **current**, **moved** or **outdated**. "Outdated" is about where the item sits;
   "resolved" is your decision. They are separate fields and never inferred from each
   other.
5. **Diagrams have two layers that never merge.** The **baseline** is the agent's
   explanation of the code at round N. It is immutable and every node is anchored to code.
   The **proposal** is your edited copy, i.e. what you want the code to become. On send,
   the proposal becomes one feedback item: a machine-readable list of semantic operations
   (add, remove, relabel, rewire, move into a group) plus notes, with the `.tldr` and a
   PNG attached. Proposals use a reserved "proposal" hue and dashed or solid strokes.
   **Diff green and red stay reserved for real code changes**, so a wished-for box can
   never look like added code.
6. **Embedded native editing is the requirement, and godraw is today technically close
   but legally and structurally blocked.** Its editor is already split into a headless
   `editor` package with public pointer, pan, zoom, undo and read-only APIs, plus a
   `View(c)` component on the same MyGo v0.7.4. But it has **no license**, all its code
   is under `internal/`, and its module path is the bare `godraw`. We can't import,
   vendor or fork it. The plan: build the diagram **model, proposal diff, states and
   transport now**, renderer-independent and stored as `.tldr` records with our metadata
   in `meta`. Then plug in godraw's editor when it is licensed and exported (path A). If
   that doesn't happen, write our own clean-room editor for the same file subset
   (path B). That choice is the user's (§10, Q1).
7. **GitHub is an optional, explicit export.** Only you can publish, from a menu. The
   default creates a **pending** review on the PR under your name. A comment is postable
   only if its round's HEAD equals the PR head and its line falls on that side of the PR
   diff. Uncommitted, stale, outdated and diagram items stay local, and the app lists
   them with the reason. Agent replies never go to GitHub unless you pick them.

---

## 1. What the PoC does today and what has to change

Facts from the code at `d31a055`:

- `exportSnapshot` makes a private temp dir per guide session
  (`internal/app/guide_ai.go:21`) and writes `snapshot.json` and an `mcp.json` that
  points at `laziergit --guide-mcp <that dir>` (`guide_ai.go:44`). Closing the guide
  deletes the dir (`internal/app/guide.go:62`). **Consequence:** an external agent's
  config goes stale every session, and nothing survives a restart.
- `reviewbridge.Serve` re-reads `snapshot.json` on every `tools/call`
  (`internal/reviewbridge/mcp.go:127`) and atomically writes `guide.json`
  (`mcp.go:139`). It offers tools only, with protocol `2024-11-05` (`mcp.go:114`). The
  app imports the guide by polling on auto-refresh (`pollGuide`). **This
  file-plus-poll shape is right, and the event log keeps it.**
- Hunk identity is `sha256(path, layer, body without @@ offsets)`
  (`guide.go:146`), and marks are keyed by "nth hunk of this file/layer" plus that
  fingerprint (`guide.go:192`). Marks live in memory only (`guide.go:35`). **The
  fingerprint idea carries over to anchors. The ordinal slot doesn't, because it can't
  survive reordering across rounds.**
- `A` runs `claude -p` with `--tools ""`, allowing only the two MCP tools
  (`guide_ai.go:123`). **That stays the read-only cold reader. It is not a feedback
  consumer by default (§10, Q3).**

---

## 2. How the others move feedback (mechanism, not features)

### Crit — tomasz-tomczyk/crit (MIT, ≈1.3k★). The most complete local loop.

Firsthand:
- **Transport is a blocking CLI call, not MCP.** The agent runs `crit` in the
  background with a 24 h timeout. The call returns when the human clicks **Finish
  Review**, and stdout holds the unresolved comments as JSON. Re-running `crit` signals
  "round complete" and blocks again. Approval means finishing with zero comments.
  (`integrations/claude-code/skills/crit/SKILL.md`, steps 2–5.)
- **Store:** central `~/.crit/reviews/<key>.json`, keyed by cwd + branch, written on
  every comment change with a 200 ms debounce. Per-round file snapshots go in a
  separate `snapshots.json` "so agent tooling that reads review.json [stays] insulated"
  (`AGENTS.md` items 7, 8, 14; `internal/session/round_snapshot.go`).
- **Anchors:** `start_line`/`end_line` plus `anchor` (the full text of the commented
  lines) and an optional `quote` (selected words). Across rounds an LCS line map is
  checked against the anchor text, then a whole-file search, else `drifted: true` with
  `DriftedOnRound` (`internal/session/watch.go`, `verifyAndCorrectPosition`,
  `remapCommentPosition`).
- **Resolution belongs to the reviewer.** The agent replies with `crit comment
  --reply-to <id>` and "**Do not pass `--resolve`**. Resolving is the reviewer's call"
  (crit SKILL.md step 4; crit-cli SKILL.md hard rules).
- **Concurrent writers:** a review write lock plus a three-way merge "comments and
  replies by ID, … fields by JSON key" (`internal/session/review_merge.go`,
  `SaveCritJSONMerged`). Headless `crit comment` writes the file directly, and SSE
  tells a running server (`AGENTS.md` item 12).
- **GitHub push sorts comments into buckets.** Comments are postable, full-stack,
  unmapped (no anchor, or a stale head SHA, "includes the SHA the comment was authored
  against") or review-level ("not pushable to PR review API"). Comments already on
  GitHub (`GitHubID != 0`) and live DOM pins are skipped
  (`internal/github/push_buckets.go`). `crit pull` imports RIGHT-side comments only,
  deduplicated by author + lines + body (`AGENTS.md` line 146). Push maps `LEFT`/`OLD`
  to `LEFT` and everything else to `RIGHT` (`internal/github/github.go:1230`).
- **Known pain:** comments vanish when HEAD moves in range mode (#1148), and there is
  no round-to-round diff (#1147) (cited in DIRECTION §8).

Inference: blocking until "Finish" is the most reliable wake-up because it works with
every agent that has a shell. But it ties the agent's turn to the human's, and
Claude Code's 2 h background cap shows up as a documented failure mode in their own
skill.

### Hunk — modem-dev/hunk (MIT, ≈9.8k★). The live loop people post about.

Firsthand:
- **Transport:** a loopback daemon that the TUI registers with, driven by `hunk session
  …` CLI commands. There are owner-private credentials and signed responses, and "no
  credential flags" (`docs/agent-workflows.md`, "How live session control works";
  `docs/session-broker-sdk.md`). Heartbeat is 10 s, stale TTL 45 s (`session-broker-sdk.md`
  around line 480).
- **Anchors** are `--old-line` or `--new-line` (exactly one), `--hunk`, or `--reply-to`,
  which inherits the parent's anchor (`packages/hunk/skills/hunk-review/SKILL.md`).
- **Provenance:** notes are typed `user`, `ai` (sidecar), `agent` or `live`. "Agents can
  remove or bulk-clear human notes … but cannot create or edit them"
  (`agent-workflows.md`).
- **No persistence:** "If you quit Hunk, you lose your live agent comments"
  ([#113](https://github.com/modem-dev/hunk/issues/113), open), and human notes "die
  with the session" ([#473](https://github.com/modem-dev/hunk/issues/473), open PR).
  Comments are "one-way annotations", with no thread or reply transport back to the
  agent ([#115](https://github.com/modem-dev/hunk/issues/115), open).
- **Concurrency bugs:** a live agent reply arriving while the composer is open froze
  the draft ([#1196](https://github.com/modem-dev/hunk/issues/1196), closed). A
  commented file leaving a `--watch` diff dropped the whole session from the daemon
  ([#1138](https://github.com/modem-dev/hunk/issues/1138), closed).
- **GitHub:** requested, not built: approve or reject agent notes, then publish the
  accepted ones as a PR review ([#460](https://github.com/modem-dev/hunk/issues/460)),
  and a native PR review export ([#705](https://github.com/modem-dev/hunk/issues/705)).

Inference: Hunk proves the value of the loop (the X posts in
`docs/research/laziestgit-x-notes.md`). Its open issues are exactly persistence,
threads and replies, and survival across diff changes. An in-memory daemon is the
wrong source of truth.

### DiffPrism — CodeJonesW/diffprism (Apache-2.0, 3★). The most careful about waiting.

Firsthand (from its decision journal, `docs/journal/entries/`):
- **MCP tools include `wait_for_comments`, `get_review_comments`, `reply`,
  `get_review_state` and `get_user_focus`** (`packages/core/src/mcp-tools.ts`).
- **The wait must tell the agent the next step it can take right now.** When a wait
  ran out, agents stopped and asked the user, and later feedback "reached no one". Fix:
  the timeout says "run it again straight away". Lesson: "Advice that depends on
  something the reader can't see … isn't advice an agent can follow"
  (`2026-09-28-keep-listening…`). And "the review outlives the wait that opened it": a
  decision records the diff it answered and is kept on retry only if the diff is
  identical (`2026-09-17-a-review-outlives…`).
- **"No agent is listening."** The server records when an agent last read a session's
  threads. If nobody reads within 5 s, the thread says so and offers a copyable prompt
  (`2026-09-17-say-when-no-agent-is-listening`).
- **"Fixed" is the agent's word, and the UI says so.** It is a field on the agent's
  reply that only an agent can set, shown as "Fixed by the agent". "It's a report, not
  a check" (`2026-09-27-findings-show-as-fixed`). One action sends each finding to its
  own thread, because a batch "request changes" couldn't be followed per item.
- **Rounds compare hunks by content, not position.** The baseline is "what was on your
  screen", and "Mark as seen" advances it (`2026-09-27-what-changed-since-you-looked`).
- **GitHub:** "nothing goes public unless you tick it". The decision is recorded only
  after GitHub accepts it. Threads record their side, because a deleted line posted
  without a side lands on a different line. The "Reviewed with DiffPrism" footer was
  removed (`2026-09-17-decide-a-pr-review-and-post-it-to-github`).
- **One review per repo and worktree, PR sessions keyed `owner/repo#n`.** There is no
  fallback to "the most recent session" ("a tool guessing which review you meant is
  worse than an error") (`2026-09-17-one-review-contract…`).

### Gnosis — oddur/gnosis (MIT, 54★). GitHub-first.

Firsthand: local notes can be copied "as Markdown for any agent", and inline comments
are submitted to GitHub as approve, request changes or comment (README). A local draft
records `headSha`, `baseSha`, `baseBranch`, `line > 0` and `side ∈ {LEFT, RIGHT}`, and
is stored per account (or `local`) and URL (`lib/changes-review-drafts.ts`). "GitHub
coordinates are valid only for the displayed revision and side" (`lib/feedback.ts`).
Stale-review detection shows a banner when a PR gets new commits (README).
Inference: the clipboard is its only transport to an agent.

### Guided Review — nshntarora/guidedreview (Apache-2.0, 78★)

Firsthand: on local changes, "Line notes stay in the running session; there is no
GitHub submit. **Generate Prompt** builds a coding-agent prompt from those notes and
copies it", without sending it anywhere (README). On a PR it posts one review with
`event` and `commit_id: head.sha`, sends `start_line`/`start_side` only for multi-line
comments, and requires a body for COMMENT and REQUEST_CHANGES
(`apps/extension/src/lib/github/submitReview.ts`).

### Review Assist — uditk2/review-assist (Apache-2.0 in `LICENSE`, the API says NOASSERTION; 1★)

Firsthand: the author agent writes an **Intent Document committed to the branch**
(`.intent/<pr-or-branch>.json`), pinned to `commit_range {base_sha, head_sha}`. A
validator fails CI if the document is stale (`head_sha` ≠ PR head) or leaves a hunk
unexplained. Anchors are `{path, hunk: {old_start, old_lines, new_start, new_lines}}`
pinned to `head_sha`. The reviewer marks each item accepted, question or blocking, and
"the verdict and your notes post to the pull request as you". Diagrams are Mermaid
inside the document (`SPEC.md` §§2–6; README "How it works").
Inference: git itself is the transport, which is great for teams and impossible for
uncommitted work.

### What this adds up to (inference)

| Pattern | Who | Take it? |
|---|---|---|
| Durable local store, outside the work tree | Crit (`~/.crit`), Gnosis | **Yes**, in the git dir, so it is per worktree and travels with the repo |
| Batch "send" with per-item tracking | Crit (Finish), DiffPrism (threads) | **Yes**: drafts until sent, then states per item |
| Agent blocks until feedback, with self-renewing advice | Crit (CLI), DiffPrism (MCP) | **Yes**, as a bounded MCP long poll |
| Listener presence ("no agent is listening") | DiffPrism | **Yes** |
| Anchor = lines + text, then drift detection | Crit, Hunk (old/new) | **Yes**, plus our hunk fingerprint and layer |
| Agent can't resolve, can't edit human notes | Crit, Hunk, DiffPrism | **Yes**, enforced by the tool surface |
| Postability buckets for GitHub | Crit | **Yes** |
| In-memory daemon as the truth | Hunk | **No** (#113, #473, #1138) |
| Committed review artefacts | Review Assist | **No** for local review. Maybe later for team sharing (Q5) |

None of the six tools lets a reviewer **edit a diagram as feedback**. Gnosis (D2) and
Review Assist (Mermaid) render agent diagrams read-only (firsthand, README and SPEC).
That part is new ground.

---

## 3. The local review store (source of truth)

### 3.1 Layout

```
$(git rev-parse --git-dir)/laziergit/review/          # per worktree; never in the work tree
  index.json                    # active review per scope; app-owned
  <review-id>/
    events.jsonl                # append-only log; the truth; app + MCP servers append
    review.lock                 # flock target for appends
    rounds/0001.json …          # immutable snapshot per round (hunks, diff, oids)
    diagrams/<diagram-id>/
      baseline-r0001.tldr       # immutable, agent-authored (laid out by the app)
      proposal.tldr             # app-owned working copy of the reviewer's edits
      sent-<batch>.tldr / .png  # frozen copy attached to a sent item
    drafts.json                 # unsent comments; app-owned; agents never read it
    marks.json                  # reviewed marks (DIRECTION iteration 1.4); app-owned
    presence/<instance>.json    # MCP server heartbeat while waiting; deleted on exit
```

A **review** is one (worktree, scope) pair. The scope is `worktree`, `base:<ref>` or
`pr:<owner>/<repo>#<n>`. One is active per scope, as in DiffPrism. An ambiguous lookup
returns an error that lists the candidates and never guesses.

Why the git dir and not `~/.something`: it is per worktree for free (linked worktrees
have their own git dir), it is removed with the repo, it is private, and it is outside
the work tree, so it can't leak into a diff or a commit. Writing a file there is not a
git write in the S7 sense (no refs, no index, no objects).

### 3.2 Concurrency and recovery

- **Writers:** the app, plus one stdio MCP server process per connected agent. Each
  append takes `flock(review.lock)`, reads the last `seq`, and writes one JSON line with
  `O_APPEND`. Lines are small, so a crash leaves at most one torn last line, which the
  reader ignores. That gives total order, no lost updates and no merge code. (Crit
  needed a three-way merge because its truth is one mutable document. We avoid that by
  only appending.)
- **App-owned files** (`drafts.json`, `marks.json`, `proposal.tldr`, `index.json`) are
  written only by the app, with the existing atomic rename (`reviewbridge.Write`). They
  are never shared, so no merge is needed. If two app windows open the same repo, the
  second one sees a changed mtime before it writes and offers a reload (the edge case).
- **Readers** fold the log into state. The app already polls on auto-refresh, so it
  re-folds when the log's size or mtime changes. The MCP server's `wait_for_feedback`
  watches the same file.
- **Recovery:** the state is always "fold of the log + app files". Restarting the app,
  the agent or the machine loses nothing that was sent. Unsent drafts survive because
  `drafts.json` is saved on every edit (debounced, as in Crit's 200 ms).
- **Size:** `rounds/` holds the diff text (bounded like today's 180 KB AI limit, plus
  compression). Compaction and archiving of closed reviews come later. Nothing is
  deleted automatically (Crit's `cleanup_on_approve` would also delete the history).

### 3.3 Data model

Every event has this envelope:

```jsonc
{ "seq": 41, "id": "ev_2kq9…", "ts": "2026-11-07T13:02:11Z",
  "actor": { "kind": "human", "name": "Kevin Gyori" }        // git user.name
        // | { "kind": "agent", "client": "claude-code", "client_version": "…",
        //     "instance": "mcp_7f3a", "label": "author" }    // from MCP initialize clientInfo
  "type": "…", "data": { … } }
```

Event types (`data` shapes abbreviated):

| type | who | data |
|---|---|---|
| `round_created` | app | `{round, revision, scope, base_oid, head_oid, worktree_dirty, hunk_count}` |
| `guide_published` | agent | the guide v2 from DIRECTION §4.3, pinned to `revision` |
| `diagram_baseline_published` | agent | `{diagram_id, round, title, nodes[], edges[], groups[]}` (structure; the app lays it out into `baseline-rNNNN.tldr`) |
| `feedback_sent` | app (human) | `{batch_id, round, items: [FeedbackItem]}` |
| `feedback_amended` / `feedback_withdrawn` | app (human) | `{item_id, body?}`; history is kept |
| `feedback_delivered` | MCP server | `{item_ids, instance}`, written when an agent read returns new items |
| `agent_reply` | agent | `{item_id, kind: ack\|addressed\|declined\|question\|note, body, op_status?}` |
| `human_reply` | app | `{item_id, body}` |
| `item_resolved` / `item_reopened` | app (human only) | `{item_id, reason?}` |
| `rereview_requested` | agent | `{summary, addressed: [item_id]}` |
| `anchors_updated` | app | `{round, items: [{item_id, status: current\|moved\|outdated, start_line, end_line}]}` |
| `published_to_github` | app | `{item_ids, review_url, pending: bool, github_ids}` |

`FeedbackItem`:

```jsonc
{
  "id": "fb_9c2e",                       // random, unique per review
  "kind": "line" | "file" | "review" | "diagram_proposal",
  "round": 2, "revision": "<snapshot sha256>",
  "anchor": {                            // absent for kind "review"
    "path": "internal/app/guide.go", "old_path": null,     // renames keep both
    "layer": "unstaged" | "staged" | "branch",
    "start_side": "new", "side": "new",  // "old" = deletion or old-file context
    "start_line": 171, "end_line": 174,  // numbers on that side's file, 1-based
    "hunk_fingerprint": "<layer-free content hash>",
    "text": "the anchored lines verbatim",
    "quote": "optional selected words"
  },
  "body": "Plain text or Markdown, written by the human",
  "diagram": null                        // for diagram_proposal, see §6.4
}
```

Item state, derived from the fold and never stored as a mutable field:

```
draft ──send──▶ sent ──agent read──▶ delivered ──agent reply──▶ acknowledged
                                                   ├─ addressed (agent's claim)
                                                   ├─ declined  (agent's reason)
                                                   └─ question  (needs your reply)
any of these ──you──▶ resolved ──you──▶ reopened (back to sent)
```

Placement status (`current`, `moved`, `outdated`) is a **separate axis**, recomputed per
round. "Outdated + agent says addressed + unresolved" is a normal, honest combination:
the code under the comment is gone, the agent claims it fixed it, and you haven't
checked yet.

---

## 4. Agent transport (MCP)

### 4.1 Server and discovery

- **One server binary, bound to the repo:** `laziergit --review-mcp [--repo <path>]`.
  Without `--repo` it resolves `git rev-parse --git-dir` from its working directory,
  which is the agent's project directory. The `M` action then copies a **stable**
  config line instead of a temp-dir path, plus the instruction paragraph from DIRECTION
  §4.3, item 7.
- **`--guide-mcp <dir>` stays** for `A`'s throwaway cold-reader run, or `A` moves to the
  store. Either works. Moving is cleaner, because then the cold reader's guide is also
  persisted.
- **Review selection:** tools take an optional `review_id`. Without it, the server uses
  the active review of the worktree's current scope. If it finds none, or several, it
  returns an error that lists `review_id`, scope and last activity (DiffPrism's rule).
- **Identity:** the server records `clientInfo` from `initialize` (for example
  `claude-code`) and makes a per-process `instance` id. The optional `label` argument on
  write tools (`"author"`, `"fixer"`) shows in the UI. The provenance labels from
  DIRECTION §4.3 (author vs inferred) reuse this.
- **Protocol:** stay on stdio and tools. Move the advertised `protocolVersion` up to a
  current revision when the server is rewritten, and keep negotiating down for old
  clients.

### 4.2 Tools

Read tools (none of them change git; all are bounded):

```jsonc
get_review_state({ review_id? }) → {
  review_id, scope, current_round, rounds: [{n, revision, created_at, head_oid, dirty}],
  counts: { sent, delivered, acknowledged, addressed, declined, question, resolved, outdated },
  diagrams: [{ diagram_id, title, baseline_round, proposal: "none"|"sent"|"resolved" }],
  cursor,                         // current end of log
  instructions                    // short, includes the rules in §4.4
}

get_review_diff({ round? })       // existing tool, gains `round`; default = current
get_review_feedback({ since?: cursor, include_resolved?: false }) → {
  cursor,                         // pass back next time
  items: [ FeedbackItem + {
    state, placement: { status, round, start_line, end_line },   // where it is NOW
    code_at_comment: "…",          // the anchored text from the item's round
    code_now: "…" | null,          // the same region in the current round, if placed
    thread: [{ actor, kind, body, ts, round }]
  } ]
}
// since omitted → every unresolved sent item (a cold agent's first call)

wait_for_feedback({ since: cursor, timeout_s?: 540 }) → same as get_review_feedback
   | { status: "pending", cursor,
       next: "Nothing new yet. Call wait_for_feedback again with the same cursor now. The reviewer is still reading. Do not ask the user." }

get_diagram({ diagram_id, round?, layer: "baseline" | "sent_proposal" }) →
  { nodes, edges, groups, tldr_path, png_path }
```

Write tools (agent-authored facts only):

```jsonc
publish_review_guide({...v2})                     // existing; may include diagrams
publish_diagram({ revision, diagram_id?, title, nodes, edges, groups,
                  supersedes_round? })           // baseline for this round
reply_to_feedback({ replies: [{ item_id, kind: "ack"|"addressed"|"declined"|"question"|"note",
                    body, op_status?: { "<op_id>": "done"|"declined"|"partial" } }],
                    label? })
request_rereview({ summary, addressed: [item_id], label? })
```

`timeout_s` defaults to 540 s, just under DiffPrism's 10-minute wait, which it states
is well within Claude Code's tool-call limits. Crit instead blocks a background shell
for up to 24 h. The `pending` text tells the agent exactly what to do now, which is
DiffPrism's lesson. While a wait is open, the server keeps `presence/<instance>.json`
updated every 10 s.

### 4.3 Who gets woken up, and how

- **The author session** (the agent that made the change, connected through `M`): after
  it finishes the change, it calls `publish_review_guide`, then `wait_for_feedback`.
  When you send, the wait returns within one poll tick. It replies with `ack`, edits
  the code with its own tools, replies `addressed` per item, calls `request_rereview`,
  then waits again.
- **A new session** ("address my laziestgit feedback", tomorrow, possibly another
  client): `get_review_state`, then `get_review_feedback()` with no cursor, which
  returns every unresolved sent item. Each one carries the code at comment time, the
  code now and the full thread, so no prior conversation is needed.
- **Nobody listening:** if no `presence/*` heartbeat is fresh within 5 s of a send, the
  status line says "No agent is listening · copy prompt", and the copy key puts
  `Address my laziestgit review feedback in <repo> (laziestgit MCP:
  get_review_feedback)` on the clipboard. Once a read is logged, it shows "Seen by
  claude-code (author)". This is DiffPrism's pattern.
- **Optional push:** Claude Code **channels** let an MCP server push events into a
  running session (`notifications/claude/channel`, capability
  `experimental["claude/channel"]`). But they are a research preview, need claude.ai or
  Console auth, and custom servers need `--dangerously-load-development-channels`
  (firsthand: code.claude.com/docs/en/channels, channels-reference). So this is **an
  opt-in accelerator for later**, emitted alongside the log, and never required. MCP
  resource subscriptions (`notifications/resources/updated`) exist in the spec, but
  whether clients show them to the model is client-specific, so we don't depend on
  them.

### 4.4 Rules the tool surface enforces

- Agents can't see drafts, can't resolve or reopen, and can't edit or withdraw human
  items. The tools don't exist (stricter than Hunk, which lets agents clear human notes).
- `addressed` is shown as "**Agent says addressed**" with the agent's label, never as a
  check mark (DiffPrism).
- Any write that names a stale `revision` is rejected with "read get_review_state
  again" (the current guide rule, extended).
- Diff text, comment bodies and diagram labels are untrusted data in tool output
  (existing rule). The instructions say so.
- No git-write tool, no network tool, no GitHub tool (S7).

---

## 5. Rounds, anchors and staleness

### 5.1 What a round is

A round is an immutable snapshot that you actually looked at. The app creates one when
you first enter laziest for a scope, when you press `r`, and, after
`rereview_requested`, when you accept the banner (`r`). The agent never creates a
round, because snapshots come from the app's lazygit git layer. If the app is closed,
the request waits in the log. `rounds/NNNN.json` stores the hunks (id, path, layer,
fingerprints, header, text), the diff text and `base_oid`/`head_oid`/dirty state.

"What changed since round N" means the hunks of the new round whose **layer-free
content fingerprint** wasn't in round N. That is DiffPrism's "compare by what they
change, not where", and it fits the DIRECTION iteration 3.10 scope.

### 5.2 Anchor rules

- **Sides:** an added line, and a context line by default, anchor `side: new`, numbered
  in the new file. A deleted line anchors `side: old`, numbered in the old file. For the
  working tree, "old" means the index (unstaged layer) or HEAD (staged layer). For a
  branch scope it means the merge base. A range can cross sides inside one hunk, so we
  store `start_side`, as GitHub does.
- **Layer** is part of the anchor, because partially staged files show two hunks for
  one region. A re-anchor may move an item across layers ("moved · now staged") when the
  same content shows up in the other layer.
- **The fingerprint is layer-free for re-anchoring.** The current
  `sha256(path, layer, body)` stays for review marks. Anchors add
  `sha256(path, body-without-offsets)` so staging a hunk doesn't make its comments
  outdated.
- **Renames:** `path` is the new path and `old_path` is kept. Matching tries both.

### 5.3 Re-anchoring an item into round N+1

1. A hunk with the same layer-free fingerprint exists → **current**. The lines are
   shifted by the hunk header's offset.
2. Otherwise, the anchor `text` is searched on the same side of the same file, within
   round N+1's hunks, nearest to the predicted position. Found → **moved** (Crit's
   approach, firsthand).
3. Not found → **outdated**. The item keeps showing its original code (`code_at_comment`)
   in an "Outdated" group, the way GitHub shows outdated threads, and is never attached
   to different code.

Shortcut for the PoC: search only inside the new round's hunks, not whole files. Code
that left the diff entirely becomes outdated, which is the conservative choice. Whole-
file search needs blob content per round and can come later.

### 5.4 The four things you must never confuse

| Shown as | Meaning | Who changes it |
|---|---|---|
| `↻ changed since r2` chip | The work tree moved after the snapshot you're reading | nobody, it's a fact |
| **outdated** (grey, hatched gutter mark) | The commented code isn't in this round's diff | re-anchoring |
| **Agent says addressed** | The agent's claim | agent |
| **Resolved** ✓ (collapsed, faint) | Your decision | you only |

Drafts written on round 2 stay anchored to round 2 while you read. When round 3
arrives, they re-anchor like sent items. An outdated **draft** blocks sending until you
re-place it (`enter` on the new line) or delete it. Feedback is never sent to a
location that doesn't exist.

---

## 6. Diagrams: agent baseline vs your proposal

### 6.1 Semantics (the part that must not be confused)

- **Baseline** = "the agent's explanation of the code as it is at round N". The agent
  publishes structure (DIRECTION §6: nodes, edges and groups, each node anchored to a
  hunk or path, or marked `context`). The app lays it out once and freezes it as
  `baseline-rNNNN.tldr`. It is **never edited**. It is labelled "Agent's diagram · r2 ·
  explains code, not verified". Nodes are evidence links: `enter` jumps to the anchored
  hunk.
- **Proposal** = "what you want the code to become". It is a working copy that starts
  from the baseline. Editing never changes the baseline or the code. The proposal is
  **feedback**: a request, not a description.
- **Agent revision** = a new baseline for round N+1, published after the agent changes
  the code. It explains the **new reality** and is compared with both the previous
  baseline (what changed in the code) and your sent proposal (did it do what you
  asked).

### 6.2 Storage and `.tldr` interoperability

Baselines and proposals are `.tldr` files, using a subset: geo rectangles, text, notes,
frames, and arrows with bindings. Each shape carries our data in record `meta`, which
tldraw's record format has and which godraw **keeps and writes back** (firsthand,
godraw README, "What the app doesn't model is kept and written back: … record `meta`"):

```jsonc
"meta": { "laziest": { "node": "reviewstore", "origin": "baseline" | "proposal",
                       "round": 2, "anchor": { "path": "internal/reviewstore/store.go",
                                               "hunk_fingerprint": "…" } } }
```

So any `.tldr` we write opens in godraw or tldraw.com for sharing. We generate the JSON
ourselves from our own model. No tldraw or godraw code is involved.

### 6.3 Proposal diff: from edits to semantic operations

The proposal is compared with its baseline **by record id**. godraw's records are
immutable and replaced on edit, which makes this cheap; for path B we keep the same
invariant. Only **semantic** changes count:

| Edit in the canvas | Operation sent |
|---|---|
| new box or text | `add_node {op_id, label, in_group?, connects?}` |
| delete a baseline box | `remove_node {node}` |
| change a label | `relabel {node, from, to}` |
| new arrow, or re-pointing an arrow's binding | `add_edge` / `rewire {edge, from_to, to_to}` |
| delete an arrow | `remove_edge {from, to}` |
| drag a box into another frame | `move_into {node, group}` |
| sticky note on or near a shape | `note {target, text}` |
| moving or resizing only | **nothing**, layout isn't feedback |

Each operation names baseline nodes, and baseline nodes carry code anchors, so "move
`reviewstore` into `internal/app`" reaches the agent with the file paths attached. A
new node can optionally be anchored with `enter` on a hunk.

### 6.4 What a sent diagram looks like to the agent

```jsonc
{ "kind": "diagram_proposal", "round": 2,
  "diagram": {
    "diagram_id": "d_flow", "baseline_round": 2,
    "agent_baseline": { "nodes": [...], "edges": [...] },      // what YOU said the code is
    "reviewer_proposed_changes": [                             // what the reviewer WANTS
      { "op_id": "o1", "op": "move_into", "node": "reviewstore", "group": "internal/app" },
      { "op_id": "o2", "op": "remove_edge", "from": "app", "to": "mcp" }
    ],
    "notes": [{ "target": "mcp", "text": "The server must not depend on app state." }],
    "attachments": { "tldr": ".../sent-b3.tldr", "png": ".../sent-b3.png" }
  },
  "body": "optional free text" }
```

The tool instructions repeat the rule: "`reviewer_proposed_changes` are requests to
change the code. They are not facts about the code. Reply per `op_id`, and after
changing the code publish a new baseline for the new round."

### 6.5 Visual language (colour only for meaning, never colour alone)

| State | Stroke and fill | Pill | Colour |
|---|---|---|---|
| Agent baseline (actual-state claim) | solid hairline, no fill | `agent · r2` | neutral ink |
| Your draft proposal: added | **dashed** outline | `draft` on the canvas frame | proposal hue |
| Your draft proposal: removed | ghost at 35% opacity, strikethrough label | — | proposal hue |
| Your draft proposal: relabelled | new label, old label struck through underneath | — | proposal hue |
| Sent | solid outline replaces dashed | `sent · r2 · waiting` / `seen by claude-code` | proposal hue |
| Agent says done or declined (per op) | small badge on the op's shape | `agent: done` / `agent: declined` | neutral, badge only |
| Resolved | collapses to faint, check mark | `resolved` | neutral |
| Outdated (baseline superseded) | hatched frame | `based on r1 · baseline r2 exists` | grey |
| Agent revision r1 → r2 (code reality changed) | shown only in "What changed" mode | `r1 → r2` | **diff green and red** |

The **proposal hue** is one reserved accent that is not green, red or the focus colour.
I'd pick lazygit's magenta family to fit lazygit's palette (final choice when themed).
Draft vs sent is shown by **stroke style plus pill**, so it still reads in monochrome.
Green and red appear only for agent-published reality changes. A box you added can
never be mistaken for added code.

Three view modes: **Baseline**, **Proposal** (the default when a draft exists; the
proposal is drawn over a dimmed baseline) and **What changed** (baseline rN against
rN+1, plus each sent op marked "now reflected" or "still differs"). The mode toggle is
one key.

### 6.6 Send, update, acknowledge

- The send key sends the comment drafts **and** any proposal drafts as one batch
  (§0.3). The proposal is frozen as `sent-<batch>.tldr/.png`, and the working
  `proposal.tldr` stays editable. A later send creates a **new** item that `supersedes`
  the old one, so the agent always gets the full current request, never a patch on a
  patch.
- The agent acknowledges with `reply_to_feedback` (`ack`, then per-op `op_status`), and
  normally publishes a new baseline after changing the code.
- When baseline rN+1 arrives, the app matches each sent op against it (by node id
  and label, then by anchor): **reflected** if the new baseline contains the requested
  structure, otherwise **still differs**. That's an aid, not a verdict. You resolve.
- If you keep editing a proposal based on r1 after r2's baseline exists, it shows
  "based on r1". "Carry to r2" replays the operations onto the new baseline, and any
  operation whose target is gone is listed as a conflict to redo by hand.

### 6.7 Native embedded godraw: feasibility, blockers, paths

**Firsthand from https://github.com/egoist/godraw (main `0371738`, pushed
2026-11-02, 242★):**

- It runs on the **same MyGo v0.7.4** and `go 1.27.3` as laziergit (`go.mod`). There is
  no version skew today.
- Its structure is embedding-friendly:
  - `internal/editor` is "the canvas without a window … no UI dependency". The
    `Editor` struct has public `PointerDown/Move/Up`, `Pan`, `ZoomAt`, `Undo/Redo`,
    `ReadOnly`, `Hint map[doc.ID]bool` (shapes outlined), `OnChange(*doc.Drawing)`
    (fires per commit, not per drag move), and `ToScreen`/`ToPage`.
  - `internal/doc` holds immutable records and `.tldr` `Read`/`Write`, preserving
    `Meta`.
  - `internal/app.App` is already a component: `New(drawing, Host)` plus
    `View(c *ui.Context)`, with a three-method `Host` interface (`Changed`, `Update`,
    `Window`).
- **Blocker 1, licence:** the GitHub API reports `license: null`, and
  [issue #5 "license?"](https://github.com/egoist/godraw/issues/5) (2026-11-03) is open
  with no replies. Without a licence we may not copy, vendor, fork, ship or adapt its
  code, including a "temporary" fork that moves packages out of `internal/`.
- **Blocker 2, import boundary:** everything is under `internal/`, and the module path
  is the bare `godraw` (not `github.com/egoist/godraw`). Go can't import it from our
  module even with a licence, so upstream must rename the module and export `doc`,
  `geo`, `editor` and ideally the painter.
- **Blocker 3, input and focus model:** godraw's canvas is `Focusable()` and grabs
  focus on pointer down (`app.go`, `input`). It registers window-level single-key
  shortcuts (`keys.go`, `c.Shortcut`). laziergit's contract is that only the root is
  focusable and `App.dispatch` owns every key (AGENTS.md). Embedding `App.View` as-is
  would steal keys from lazygit's keymap.
- **Blocker 4, painter:** the display-list painter is in `internal/app/canvas.go`. The
  `editor` package alone doesn't draw.

**Path A (target, needs upstream):** a licensed `github.com/egoist/godraw` that exports
`doc`, `geo` and `editor` (plus the painter, or a `canvas` widget with an
"embedded / no shortcuts / external focus" option). Our `internal/diagram` wraps
`editor.Editor`. The root forwards pointer and key events to it while the diagram pane
is focused, translated through our keymap. Overlays (proposal hue, pills, states) are
drawn by us on top, using `ToScreen`, or through `Hint` for outlines. Estimate: about
1–2 weeks after the upstream API exists. Fidelity: full godraw editing.

**Path B (fallback, no upstream dependency):** a clean-room editor in
`internal/diagram` for the subset in §6.2: select, move, add box, connect, relabel,
delete, frame, note, undo. It reads and writes our `.tldr` subset from the public file
format and our own model, without reading godraw's implementation while writing it.
Estimate: 2–4 weeks for a decent feel. Lower fidelity: no freehand, no rich styling.
**This is not a downgrade to read-only, but it is "godraw-compatible", not "godraw
built in"**, so it needs the user's agreement (Q1).

**Rejected:** launching godraw as a separate app (the requirement is embedded),
webview tldraw (no webview, and tldraw's SDK licence), and read-only diagrams.

**Unaffected by A or B:** the model, record `meta`, the semantic diff, operations,
states, storage and the MCP payloads. All of that can be built and tested now with a
static renderer of baselines.

### 6.8 Keyboard equivalents (proposal; every mouse action has one)

Diagram pane, entered with `enter` on a diagram row (keys from the lazygit config where
a match exists):

| Key | Action |
|---|---|
| `j`/`k` | next or previous shape in reading order (`nextItem`/`prevItem`) |
| `h`/`l` | follow the selected arrow back or forward |
| `enter` | jump to the node's anchored hunk (`goInto`) |
| `n` | add a node connected from the selected one (prompt for its label) |
| `R` | relabel (`renameBranch`-style rename) |
| `d` | remove: marks the shape removed in the proposal (`remove`) |
| `w` | wire: pick a target with `j`/`k`, then `enter` |
| `m` | move into a group: pick the group |
| `c` | note on the selected shape |
| `z` / `Z` | undo / redo (lazygit's `undo`/`redo` defaults) |
| `v` | cycle Baseline / Proposal / What changed |
| `esc` | back to the story |

---

## 7. Optional GitHub PR transport

**Default:** off. A **Publish to PR…** menu item appears only when the scope is a branch
or PR with a known GitHub PR (we already resolve PR status through `gh`). Nothing is
ever posted automatically. **This research posted nothing.**

**Mechanics** (firsthand, GitHub REST "Create a review for a pull request"):
- Omitting `event` creates a **PENDING** review that you finish later. `APPROVE`,
  `REQUEST_CHANGES` or `COMMENT` submits it. `REQUEST_CHANGES` and `COMMENT` require a
  `body`.
- Comments take `path`, `line`, `side ∈ {LEFT, RIGHT}` and optional
  `start_line`/`start_side`. LEFT is for deletions. `commit_id` should be the latest
  head, or comments may show as outdated. `subject_type: file` allows file-level
  comments. Replies can only target a top-level review comment ("Replies to replies are
  not supported") (REST "Review comments").

**The publish dialog** (lazygit-style menu, lists everything it will not post):
1. Choose: **Create pending review** (default: visible only to you on GitHub until you
   submit it there or here) / **Comment** / **Request changes** / **Approve**.
2. Items are listed **unticked** (DiffPrism). Only your words post. Agent replies stay
   local unless ticked one by one. No footer.
3. Each item lands in one bucket (Crit's model):
   - **postable**: round head = PR head SHA, branch layer, anchor inside the PR diff on
     that side. Maps to `line`/`side`/`start_*`, with `commit_id` = PR head.
   - **not pushed**: local HEAD has commits the PR head lacks. The action is "push first
     (lazygit `P`), then publish". No auto-push.
   - **uncommitted**: staged or unstaged layer. Never posted. GitHub has no
     representation for it, and posting the snippet would publish unpushed code.
   - **outdated**: not posted.
   - **review-level**: goes into the review `body` (we choose to post these; Crit
     doesn't).
   - **diagram proposal**: GitHub has no native representation. Opt-in only, as a
     Markdown op list in the body ("move `reviewstore` into `internal/app`"). The PNG
     can't be attached via the REST review API (the image upload endpoint isn't public:
     **inference, to verify**).
4. Success records `published_to_github` with the GitHub ids, so re-publishing never
   duplicates. Failure shows GitHub's reason and keeps your selection (DiffPrism).

**Known constraint (inference, to verify before building):** GitHub allows one pending
review per user per PR. If one already exists, creating another fails, and appending
needs the GraphQL review-thread mutations. The first version should detect an existing
pending review and say so rather than work around it.

**Not now:** pulling PR comments into the local store (Crit pulls RIGHT-side only, with
dedup by author + lines + body; Gnosis syncs threads). The schema is ready for it
(`origin: github`, GitHub ids).

---

## 8. User journeys

### 8.1 Working tree, no PR, author agent connected

1. Claude Code finishes a change in the terminal. Its MCP config (set once with `M`) is
   `laziergit --review-mcp`. It calls `publish_review_guide` (author provenance), then
   `wait_for_feedback`.
2. You press `~` in lazier. laziest opens on the matching hunk, round 1 is created, and
   the status line says "Agent listening · claude-code (author)".
3. You read with `j`, comment with `c` on two lines (dashed gutter dots, "2 drafts"),
   and open the guide's diagram. You drag `reviewstore` into the `internal/app` frame
   and delete the `app → mcp` arrow. Both show as dashed magenta with a `draft` pill.
4. You press send. The drafts become one batch, and the pills turn solid "sent · r1".
   The agent's wait returns the two line items and one diagram item, and it replies
   `ack`. The pills read "seen by claude-code".
5. The agent edits the code, replies `addressed` on two items and `declined` on op `o2`
   with a reason, publishes baseline r2 and calls `request_rereview`.
6. The banner says "Round 2 ready · agent says 2 addressed, 1 declined". You press `r`.
   Round 2 is created, both comments re-anchor (one current, one outdated), and the
   "new since r1" filter shows three hunks. The diagram shows op `o1` "reflected" and
   `o2` "still differs · agent declined: …". You resolve one comment, reply to the
   decline and send again. The loop continues.

### 8.2 Nobody is connected

Same up to step 4. After the send, nothing reads within 5 s, so the status line shows
"No agent is listening · copy prompt". You copy it and paste it into any agent with the
config. It calls `get_review_feedback()` and gets everything, with code context.

### 8.3 PR-backed branch

Scope `pr:owner/repo#42`. The round head is checked against the PR head on entry. After
step 6 of 8.1 you choose **Publish to PR…**. Three line comments are postable, one is
"not pushed" (the agent's fix commit is local only), and the diagram item is opt-in
text. You push with lazier's `P`, publish again, and choose "Create pending review".
The dialog links to the review on GitHub. The local store stays the truth. GitHub got a
copy of your words.

### 8.4 The agent changes code mid-review (no rereview signal)

Auto-refresh notices the work tree moved, and the chip says "↻ changed since r1". Your
screen, drafts and marks stay on r1 (snapshot rule S6). You can finish reading, or press
`r`. On r2, drafts re-anchor. One draft is outdated and blocks send until you re-place
or delete it. Sent items show "outdated" where their code is gone, never "resolved".

### 8.5 Coming back next week

You start laziergit and press `~`. The active review for this worktree and scope loads:
rounds, marks, drafts, threads. If the work tree differs from the last round, the chip
shows it. A new agent session asked to "continue the review" calls `get_review_state`
and sees open items, and which ones the previous session claimed addressed, with no
memory of the old conversation.

---

## 9. Next slice, and what can start now

### 9.1 Minimum coherent slice: "line comments round-trip, persisted"

Without diagrams. It proves the store, the transport, rounds and states end to end:

1. `internal/reviewstore`: layout (§3.1), locked append and fold, `drafts.json`,
   `rounds/`, ids, presence. Pure Go, no UI.
2. `laziergit --review-mcp` with repo discovery and the tools `get_review_state`,
   `get_review_diff(round)`, `get_review_feedback`, `wait_for_feedback`,
   `reply_to_feedback` and `request_rereview`, plus the existing `publish_review_guide`
   writing into the store.
3. Anchoring library: the layer-free fingerprint, side and line mapping from parsed
   hunks, and re-anchoring (§5.3) within hunks.
4. App: `c` on the line cursor (a draft), a drafts and threads list in the story panel,
   one send key, gutter markers per state (Crit #801: never between code lines), the
   listener status line, the round banner, the "new since rN" filter, and resolve and
   reopen.
5. `M` copies the stable config.

**Acceptance (headless tests, repo-local, no network):**
- Two drafts on new-side lines and one on a deleted line (`side: old`) survive an app
  restart, and an MCP client sees **none** of them before send.
- After send, `get_review_feedback()` from a separate process returns all three with
  path, side, lines, layer, fingerprint and code. A second call with the returned
  cursor returns nothing. `wait_for_feedback` blocks, and returns within 1 s of a later
  send.
- Two MCP processes and the app append concurrently (100 events each). The fold has 300
  events in total order, and none is lost.
- The agent replies `addressed`. The UI shows "Agent says addressed", not resolved. Only
  the app can write `item_resolved`, because no tool exists for it.
- Round 2, after editing a file above a commented hunk: the comment is `current` with
  shifted lines. After deleting the commented code, it is `outdated` and still shows
  the original text. After staging the commented hunk, it is `moved · now staged`.
- No presence heartbeat within 5 s of a send shows "No agent is listening" and copies
  the prompt (swappable `a.copyText`).
- Parity ratchet is unchanged. No files are written outside `--git-dir`.

### 9.2 Slice 2: diagram feedback (model first, editor second)

- 2a (independent of godraw): structure → layout → `baseline-rN.tldr` writer with
  `meta.laziest`, a static renderer in the main view, the proposal model, the semantic
  diff (§6.3), op ids, the `diagram_proposal` item and payload (§6.4), states and
  view-mode rendering (§6.5) using a minimal in-house interaction set (select, delete,
  relabel, move into a group, wire). That is enough to prove semantics.
  **Accept when:** a test edits a proposal (relabel, remove an edge, move into a group,
  plus a pure move), sends it, and the MCP payload has exactly 3 ops and no layout op.
  A new baseline marks one op reflected. The `.tldr` round-trips through our reader
  with `meta` intact.
- 2b: swap in the full canvas, path A or B per Q1.

### 9.3 Can start now, without touching the main agent's UI work

- `internal/reviewstore` and its tests.
- The new MCP server and tool tests (stdin/stdout harness like the existing one).
- The anchor and re-anchor library over `guideHunks` output. It reads the existing
  parser and adds the layer-free fingerprint beside it.
- The diagram model, semantic diff and `.tldr` writer/reader for our subset (no UI).

**Depends on the main agent's in-flight work:** the main-view line cursor and line-number
gutter (needed for `c` and the gutter markers), the continuous section stream (where
threads render) and the `~` toggle (where the "changed since" chip lives). The
transport only needs one small seam from that work: a function that returns
`(path, layer, side, line)` under the cursor, and a hook to draw gutter marks per line.

---

## 10. Questions that need your choice

1. **godraw: path A or B?** Embedding godraw's real editor needs the author to add a
   licence and export the packages. We won't contact them. Will you ask, or should we
   plan path B (a clean-room, `.tldr`-compatible native editor, lower fidelity) if
   there's no licence by the time slice 2b starts? Slices 1 and 2a don't depend on the
   answer.
2. **Delivery granularity:** send in batches (recommended: Crit's Finish and GitHub's
   pending review; nothing half-written reaches the agent), or stream each comment as
   soon as it's saved (Hunk/bentlegen style: the agent works while you read)? A
   per-comment "send now" can be added to batches later.
3. **Should the app launch an agent to act on feedback?** Today `A` is a read-only cold
   reader. A "fix with Claude" action would spawn an agent with edit tools. That's a
   product and safety decision (it writes the work tree). Recommendation: not in the
   next slices. Feedback goes to your own agent.
4. **GitHub default:** pending review (recommended), or submit as Comment directly? And
   should review-level and diagram items go into the review body by default
   (recommended: review-level yes, diagram opt-in)?
5. **Team sharing later:** keep reviews private in the git dir (recommended), or
   eventually allow an opt-in committed export (`.laziest/` like Review Assist's
   `.intent/`) for teammates?

Key names (send, mode toggle, copy prompt) follow the DIRECTION §9 key decisions and
are not repeated here.

---

## 11. Feasibility constraints that are still open

- **godraw licence and exports** (§6.7). This blocks path A and nothing else.
- **Tool-call ceilings:** `wait_for_feedback` assumes clients allow tool calls of about
  9 minutes. DiffPrism states that Claude Code's limits are "well above our 10-minute
  wait" (journal). Codex and other clients aren't verified. The `pending` and re-call
  contract covers shorter limits.
- **Channels** are a research preview behind a development flag for custom servers.
  They're optional and must stay optional.
- **GitHub:** the one-pending-review constraint and the lack of a public image upload
  API are from experience, not re-verified today. Verify both before building §7.
- **Agent sandboxes** may block reading the git dir from a different worktree. The
  server resolves from its own cwd, which is normally the agent's checkout. Hunk hit
  the loopback equivalent of this (agent-workflows.md, "agent sandbox may be blocking
  loopback").
- **Large reviews:** the round diff is bounded by the existing 180 KB rule. Bigger
  scopes need per-file blobs and compaction (§3.2).

---

## Sources

Local code: `internal/reviewbridge/mcp.go`, `internal/app/guide.go`,
`internal/app/guide_ai.go`, `internal/repo/guide.go`, `main.go`,
`internal/app/actions.go`, at commit `d31a055`.

godraw: https://github.com/egoist/godraw (`README.md`, `go.mod`, `internal/editor/editor.go`,
`internal/app/app.go`, `internal/app/keys.go`, `internal/doc/doc.go`, `internal/doc/tldr.go`;
`gh api repos/egoist/godraw` → `license: null`), https://github.com/egoist/godraw/issues/5.
MyGo v0.7.4 licence: MIT (module cache `LICENSE`).

Crit (https://github.com/tomasz-tomczyk/crit): `AGENTS.md`;
`integrations/claude-code/skills/crit/SKILL.md`; `integrations/claude-code/skills/crit-cli/SKILL.md`;
`docs/agent-hooks.md`; `internal/github/push_buckets.go`; `internal/github/github.go`;
`internal/session/{review_paths,round_snapshot,review_merge,watch}.go`.

Hunk (https://github.com/modem-dev/hunk): `docs/agent-workflows.md`, `docs/session-broker-sdk.md`,
`packages/hunk/skills/hunk-review/SKILL.md`; issues #113, #115, #460, #473, #705, #1138, #1196.

DiffPrism (https://github.com/CodeJonesW/diffprism): `packages/core/src/mcp-tools.ts`;
`docs/journal/entries/2026-09-17-{a-review-outlives-the-wait-that-opened-it,say-when-no-agent-is-listening,decide-a-pr-review-and-post-it-to-github,one-review-contract-for-every-entry-point}.md`,
`2026-09-27-{findings-show-as-fixed,what-changed-since-you-looked}.md`,
`2026-09-28-keep-listening-and-forms-over-a-review.md`.

Gnosis (https://github.com/oddur/gnosis): `README.md`, `lib/changes-review-drafts.ts`,
`lib/feedback.ts`, `docs/github-sync.md`.

Guided Review (https://github.com/nshntarora/guidedreview): `README.md`,
`apps/extension/src/lib/github/submitReview.ts`, `apps/cli/src/server/createServer.ts`.

Review Assist (https://github.com/uditk2/review-assist): `README.md`, `SPEC.md`, `LICENSE`.

GitHub REST: https://docs.github.com/en/rest/pulls/reviews (pending via omitted `event`,
`commit_id`, comment fields), https://docs.github.com/en/rest/pulls/comments (`side`,
`start_side`, `subject_type`, reply rules).

MCP and Claude Code: https://code.claude.com/docs/en/channels,
https://code.claude.com/docs/en/channels-reference,
https://modelcontextprotocol.io/specification/2025-06-18/server/resources (subscriptions).

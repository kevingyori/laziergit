# laziestgit — direction

Review of PoC `0d3b9fa` (branch `t3/ideate-linear-diff-guides`): goals, decisions
and the next iteration. Decisions come first. Evidence and sources are at the end.
Status: proposal. Open decisions are listed in §9.

---

## 0. Verdict in one paragraph

The core bet holds up. A lazygit-native screen where an agent's ordering and notes
sit on top of the **real** diff, checked by hunk coverage and a snapshot revision,
is the right foundation. Most competitors lack the coverage guarantee, and Crit's
best-known Story-mode bug is exactly the failure it prevents (§8). The PoC still
falls short in four ways:

1. It **reads like a web app, not lazygit**: a branded header bar, a padded summary
   card and two-line list rows put about 220 pt of chrome above the first diff
   line (§3).
2. It **shows one hunk at a time** with no line numbers or surrounding code, so
   reviewing it is harder than reviewing in lazier. This hurts review more than
   the missing syntax colours do.
3. **The notes are weak mainly because of what the agent is given and what the
   schema asks for, not the wording of the prompt.** The spawned agent runs with
   `--tools ""` and sees only the hunks. It knows nothing about the task, and the
   instruction literally says "explain what changed **and what to check**", which
   is where the "Check whether…" boilerplate comes from (§4).
4. **There is no fluid path between lazier and laziest.** Ctrl+G is one-way.
   Closing the guide throws away review marks. Ctrl+G is also lazygit's
   `jumpToFile`, so it means different things in different panels (§2.4).

---

## 1. The job, goals and non-goals

**Primary job.** *"An agent just changed my repo. Let me understand what it did
and whether it's what I asked for, quickly and with evidence, then commit it or
send it back, without leaving lazygit's keys."*

laziestgit is an **explainer and navigator for local, agent-authored changes**.
It is not an AI reviewer, not a git client (lazier is the git client) and not a
PR platform.

**Success criteria** (testable):

| # | Criterion |
|---|---|
| S1 | Every hunk can be reached and marked with `j/k/h/l/[/]/space` alone. Mouse is optional. |
| S2 | One keypress switches lazier ⇄ laziest in either direction. The selection maps to the same file and hunk. Each side keeps its own scroll. Review marks survive the switch. |
| S3 | At 1512×916 pt, with the default layout, at least 75% of the main-view height is code (the PoC is about 70% of a narrower column and shows one hunk). |
| S4 | Every agent claim is anchored to a hunk, and optionally a line. A guide with an unanchored question, or a question starting with a generic verb ("Check/Ensure/Verify…") and no stated observation, is rejected by the validator (§4). |
| S5 | With no tools, a cold reader can answer "what does this change do for the user, and why" from the overview alone, in two sentences. |
| S6 | The code on screen always comes from the snapshot, never from the model. 100% hunk coverage is kept. |
| S7 | Agents never write to git through laziestgit. |

**Non-goals for now:** AI verdicts, approval or risk scores (Gnosis has a
low/medium/high risk rating; we shouldn't, because it invites trust without
evidence). Also out: posting PR comments, chat with slides, side-by-side diffs,
mandatory diagrams, and GitHub PR ingestion.

**Next goal after reading (P1, iteration 2):** the feedback loop back to the
agent, meaning line comments the agent reads over MCP. This is the main loop in
Crit and DiffPrism. It is also the part of Hunk workflows that X posters
describe as changing their work (§8). It is the second half of the job ("send it
back"), but it needs the reading experience to be right first.

---

## 2. Navigation — exact proposal

The principle: **laziest uses lazygit's own two-level structure**, a side list plus
a main view entered with Enter or `0`, exactly like Files → Staging. Every key
comes from the user's lazygit keybinding config, never a hard-coded literal (the
PoC hard-codes `]`).

### 2.1 Layout model

- **Story panel (left, focused by default).** One continuous list, like lazygit's
  file tree. Section headers are rows, and hunk rows sit beneath them. Sections
  don't hide each other. This replaces the PoC's two lists ("CHANGE STORY" plus
  "IN THIS SECTION").
- **Main view (right).** It shows the **whole current section as one scrolling
  stream**: section note, then each hunk with its file header, line numbers and
  inline notes. The selected hunk is outlined and scrolled into view. This
  replaces the PoC's one-hunk-at-a-time view and is the biggest review fix.

### 2.2 Story panel keys (default focus)

| Key (lazygit config name) | Action |
|---|---|
| `j` / `k` (`nextItem`/`prevItem`) | Next or previous **hunk in story order**, crossing section boundaries. Section header rows are skipped. **This is the main motion: reading the story is pressing `j` repeatedly.** |
| `h` / `l` (`prevBlock`/`nextBlock`) | Previous or next **section**, landing on its first hunk. On the first hunk of a section, `h` goes to the previous section's start (no-op at the top). |
| `[` / `]` (`prevTab`/`nextTab`) | Previous or next **unreviewed** hunk, wrapping, across sections. This is review progress, kept separate from selection. |
| `space` (`select`) | Toggle reviewed on the selected hunk, then **stay on it** (no auto-advance, so behaviour is predictable). `]` is the "next" after marking. |
| `J`/`K`, `ctrl+d`/`ctrl+u`, `pgdn`/`pgup` (`scrollDownMain`/`scrollUpMain`) | Scroll the main view without changing the selection (lazygit's behaviour). |
| `<`/`>` (`gotoTop`/`gotoBottom`) | First or last hunk. |
| `enter` / `0` (`goInto`/`focusMainView`) | Focus the main view at the selected hunk. |
| `/` | Filter the story by path (lazygit's search). |
| `+` / `_` (screen mode) | Normal → half → full main view, using lazygit's existing screen modes. |
| `{` / `}` | Diff context, shown from the snapshot's blobs (view-only, so hunk IDs and the revision don't change). |
| **toggle key** | Switch to lazier (§2.4). |

### 2.3 Main view keys (after Enter), mirroring lazygit's staging view

| Key | Action |
|---|---|
| `j` / `k` | Line cursor. |
| `h` / `l` | Previous or next hunk (lazygit's staging uses `prevBlock`/`nextBlock` for hunks). This also moves the story selection. |
| `[` / `]` | Previous or next unreviewed, as in the panel. |
| `space` | Toggle reviewed on the hunk under the cursor. |
| `esc` | Back to the story panel with the selection kept. |

So `j/k > h/l > [/]` holds in both contexts, with the granularity one step finer
inside the main view. That is exactly how lazygit's files panel relates to its
staging view, so muscle memory carries over.

### 2.4 The lazier ⇄ laziest toggle

- **One unmodified key in both screens.** It must be free in lazygit's default
  keymap. **Recommended: `~`.** It is unbound in lazygit 0.66 defaults, matched
  via `InputText` so it doesn't depend on keyboard layout, and it sits next to
  `` ` `` (tree/flat view toggle), which is the same idea: a different view of the
  same data. Make it configurable under laziergit's own config key, not
  `keybinding.universal`.
- **Remove the global `ctrl+g`.** It is lazygit's `universal.jumpToFile`.
  `actionScopes` tries the panel scope before global, so in Commits, Stash and
  Staging it jumps to a file, while in Files and Branches it opens the guide. It
  also adds a non-lazygit row to the `?` menu, which is a parity bug.
- **Mapping lazier → laziest:** the selected file in Files, or the hunk under the
  cursor in Staging (path + layer + new-side line), becomes the matching hunk in
  the story. The first entry builds the snapshot. Later entries reuse it and show
  a "↻ changed since snapshot · r" chip when the working tree has moved.
- **Mapping laziest → lazier:** focus the Files panel on the hunk's file. If the
  main view was focused, open Staging on the same hunk, with the matching
  Unstaged/Staged tab. In branch-base scope, use the commit-files view if it
  exists, otherwise Files.
- **The guide state outlives the toggle.** Snapshot, guide, marks and per-screen
  scroll stay alive until `r` or a scope change. `esc` at the top of laziest also
  goes back to lazier, harmlessly. `q` means quit, as in lazygit.

---

## 3. Density — match the regular screen

Measured from `build/laziestgit-review/guide-native-ai.png` (1512×916 pt) against
`/tmp/laziestgit-shots/staging-dark.png`:

| Element | PoC | Regular laziergit | Change to |
|---|---|---|---|
| App header bar ("laziestgit / review guide", 3 buttons) | 54 pt + hairline | none | **Delete.** Scope, the AI action and back move to the footer hint bar and the panel title. |
| Section summary card (caption, title, 2-line prose, button, 22 pt padding) | ≈165 pt, fixed | Staging header ≈85 px including tabs | **Delete as a card.** The section note becomes the first block of the main-view stream, like a commit message above its patch in lazygit's commit view, and scrolls away with the code. |
| Panel title | "CHANGE STORY" at 44 pt | 12 pt caps title with a number badge | `STORY  12/23` in the same style as `FILES 2`. |
| Hunk rows | 2 lines (path, then "Unstaged · h22") | 1 line, 22 pt | 1 line: `○ U internal/repo/guide.go  +42`. Layer is an S/U glyph coloured like lazygit's file status. **Hide hunk IDs** (they are internal). |
| Footer | 34 pt hint bar plus a 2-line hint above it in the panel | ≈30 pt hint bar | Hint bar only. Delete the "Snapshot review · r reloads…" text and put it in `?`. |
| "Mark reviewed [space]" button | in the card | none | Gone. The ○/✓ in the gutter of the selected hunk header is clickable. |
| Story panel width | fixed 330 pt | lazygit's side-panel ratio | Use the same side-panel width logic and `gui.sidePanels` as lazier. |

**Prose on demand.** The section note shows **why** plus **how** (§4) and is at
most 3 lines. Per-hunk notes are one line above the hunk header, in prose font
and muted. Longer `detail` text expands in place with `e` (lazygit `edit` has no
meaning in laziest; open decision) and collapses again. Following Crit #801, notes
never interrupt code lines. Line-anchored notes show as a gutter dot, and the text
appears on hover or when the line cursor is on that line.

---

## 4. Agent authoring — fix the contract, not the adjectives

### 4.1 Why the current notes read as plumbing

- **Information diet.** `generateGuide` runs `claude -p … --tools ""` and allows
  only `get_review_diff` and `publish_review_guide`. The model cannot open the
  file around a hunk, see callers, see the commit message, or know the user's
  task. With only the hunk text, it can only paraphrase the code back.
- **The instruction asks for checklists.** "Explain what changed **and what to
  check**", plus "review questions" in the same 60-word field, leads to
  "Check whether X, how Y, and whether Z".
- **The schema has one prose field per section.** Purpose, mechanism and doubt
  compete for 60 words, and nothing is anchored.
- **No provenance.** A cold reader's guess and the author's statement look the
  same.

### 4.2 Weak vs improved — section 01 of the real PoC guide

**Weak (actual output, guide-native-ai.png):**

> **Review snapshot and hunk identity**
> A read-only git diff, either the working tree with staged and unstaged parts plus
> untracked files, or merge base to HEAD, is split into hunks with fingerprints,
> heuristic sections and review marks. Check whether the merge-base argument order
> with "--" works, how headerless or binary diffs are handled, and whether
> ordinal-based slots mark the wrong hunk after reordering.

Problems: sentence 1 lists mechanisms with no purpose. Sentence 2 asks three
questions the agent could have answered itself. I ran `git merge-base -- main HEAD`
here and it exits 0, so the first question **costs the reviewer a lookup and is a
false alarm**.

**Improved (same hunks, same length budget):**

> **Review the exact diff you'd commit**
> *Why* — The guide must show the same changes lazier would stage or commit,
> including both halves of a partially staged file, so that "reviewed" means
> something. *(author-stated)*
> *How* — `GuideDiff` takes the index and worktree diffs separately and appends
> untracked files using lazygit's own renderer. Each hunk's identity is
> path + layer + body with line offsets removed, so a mark survives edits
> elsewhere in the file.
> ▸ `guide.go:171` *explain* — A mark's slot is "nth hunk of this file". If an
> earlier hunk in the file is removed, the stored fingerprint no longer matches,
> so the mark shows ↻ instead of moving to the wrong hunk. That's conservative,
> not wrong. *(evidence: `slot` plus the fingerprint comparison in `isReviewed`)*

What changed: a behaviour title (the outcome, not the mechanism), a split between
**why** and **how**, a provenance tag, and one anchored note that **resolves** a
doubt with evidence instead of handing it to the reviewer.

### 4.3 The new contract

**Two authoring modes, labelled in the UI:**

1. **Author mode (preferred).** The coding agent that made the change connects
   with `M` and publishes. It knows the task, the rejected alternatives and what
   it didn't do. Its notes are labelled *author*. This is Review Assist's key idea
   (§8).
2. **Cold-reader mode (`A`).** A spawned agent with read-only context tools. Its
   claims are labelled *inferred* unless backed by a tool result.

**MCP tools** (all read-only, all bounded, all pinned to the snapshot revision):

| Tool | Returns | Bound |
|---|---|---|
| `get_review_diff` | hunks, revision, scope, branch name, commit subjects in range, optional `task` (the user's request text, if the author or user supplied one) | 180 KB (existing) |
| `read_file(path, side=old\|new, start, end)` | lines from the snapshot's blobs (not the live tree) | ≤400 lines per call, ≤40 calls |
| `search(pattern, path_glob)` | `path:line: text` at the snapshot's new side (`git grep` on the tree, plus worktree files for working-tree scope) | ≤200 matches |
| `publish_review_guide` | validation errors, or ok with coverage | — |

No shell, no git writes, no network. Diff and file text stays untrusted data
(existing rule).

**`publish_review_guide` schema v2:**

```jsonc
{
  "revision": "…",
  "provenance": "author" | "inferred",
  "overview": {               // ≤ 2 sentences each; shown once, top of story
    "purpose": "What this does for a user/caller, and why now.",
    "not_done": "Optional: what the change deliberately leaves out."
  },
  "sections": [{
    "title": "≤ 8 words, names an outcome",       // "Review the exact diff you'd commit"
    "kind": "core" | "supporting" | "mechanical",  // mechanical collapses by default, but its
                                                    // hunks stay unreviewed and reachable by ]
    "why": "≤ 1 sentence",
    "how": "≤ 2 sentences, may name functions",
    "hunk_ids": ["h22", "h18"],
    "notes": [{                                     // 0–3 per section
      "hunk_id": "h18", "line": 182,                // new-side line, optional
      "kind": "explain" | "assumption" | "question",
      "text": "≤ 30 words",
      "evidence": "path:line or tool result that backs it"  // required for assumption/question
    }],
    "diagram": null                                 // §6, optional
  }]
}
```

**Validator rules** (reject with a message the agent can act on, same as the
current coverage checks):

- Every `hunk_id` appears exactly once (existing). Note anchors must be in their
  section, and lines must be inside the hunk's new-side range.
- `assumption` and `question` require `evidence`. A `question` must say what the
  agent **couldn't** determine. If the tools could answer it, the agent must
  answer it.
- At most 1 `question` per section and 3 per guide.
- Lint: reject `text` starting with `Check|Ensure|Verify|Make sure|Consider`.
- `why` must not repeat the title. Reject titles that are only identifiers
  (`^[A-Za-z_.]+$`).

**Prompt (replaces `guideInstructions`):** a short role statement, then the
contract above, then three rules:

1. "Write for a reviewer who knows this codebase but not this task. Lead with
   purpose and behaviour; mention functions only in `how`."
2. "Use `read_file`/`search` to resolve your own doubts before writing. A question
   is only for what the code cannot tell you."
3. "Put mechanical changes in `kind: mechanical` with a one-line `why`, not a
   narrative."

Ship one worked example (the §4.2 pair) inside `get_review_diff`. A good/bad pair
teaches more than adjectives do.

**Evaluation.** Keep three fixture diffs: this PoC, a small bug fix and a
rename-heavy refactor. Score guides on S4 and S5 manually, before and after the
change. Prompt tweaks don't merge without that comparison.

---

## 5. Syntax highlighting and code view

**Facts checked:**

- MyGo `v0.7.4` has `ui.RichText(c, ui.Span{…})` with per-span colour, font and
  weight. laziergit already uses it, and MyGo benchmarks it for mono code lines.
  It has **no syntax highlighter**: `ui/markdown`'s `CodeBlock` is a hook.
- lazygit has **no built-in highlighting**. It relies on pagers (delta, etc.),
  which laziergit already supports when configured (cycle with `|`).
- Neither go.mod/go.sum has a lexer today.

**Decision.** Use **Chroma v2** (`github.com/alecthomas/chroma/v2`, MIT, pure Go,
about 250 lexers) in **laziest only**.

- **Lexing.** Lex the **whole old and new blob** of each file once per snapshot,
  off the main thread (`a.bg`), and cache by blob hash. Map tokens to line numbers
  and colour diff lines by looking them up. Lexing hunk fragments breaks on
  multi-line strings and comments.
- **Colour.** The foreground comes from about 6 token classes (keyword, string,
  comment, number, type/function, punctuation/other), taken from the app palette
  in light and dark. The add/delete **backgrounds** stay as they are, and the
  `+/-` sign column keeps its diff colour. That matches "colour for meaning":
  diff state is the background, syntax is the foreground.
- **Intra-line emphasis.** Add a stronger add/delete background on the changed
  word range of paired `-`/`+` lines. This helps review more than token colours
  do.
- **Line numbers.** Show an old/new number gutter in mono and faint colour. This
  is P0. Notes anchor to new-side numbers.
- **Limits.** No highlighting for files over 1 MB or with no lexer (plain mono).
  Fall back gracefully.
- **Lazier is unchanged.** Parity says lazier renders like lazygit. Users who want
  colours there configure delta, which already works.

**Not now:** side-by-side diffs (a 2-column layout fights the story panel, and
Gnosis offers it as an option rather than the default). LSP-style
go-to-definition (Crit #1149 asks for it) comes later, through the same
`search`/`read_file` backing as the agent.

---

## 6. Diagrams and godraw

**What godraw actually is** (checked from the repo, `pushed 2026-11-02`, 242★): a
**native MyGo app** (same MyGo `v0.7.4` as us, no webview) that reimplements
tldraw's whiteboard and **reads and writes `.tldr` files** at
`@tldraw/tlschema` 5.6.0. It contains no tldraw code and isn't affiliated with
tldraw. It is **not** tldraw and not embeddable as-is:

1. **No license.** The GitHub API reports none, and issue #5 ("license?") is
   still open. We cannot copy or vendor its code.
2. All its code is under `internal/` (`geo`, `doc`, `editor`, `app`), so **Go
   won't let another module import it** even with a license.
3. It's an app with document windows, not a widget.

**Decision for diagrams:**

- **The agent emits structure, not pixels.** Add optional `diagram` on a section
  (at most 1 per guide at first):
  `{nodes:[{id,label,anchor:{hunk_id|path}}], edges:[{from,to,label}], kind:"flow"|"structure"}`
  with at most 12 nodes. The validator requires every node to be anchored to a
  hunk or a changed path, or flagged `"context": true`. Prompt rule: "only when 3
  or more components interact and the order or ownership isn't obvious from the
  code." Crit's guide says the same ("leave `diagram` empty unless [it] clarifies
  a real structure or flow").
- **laziestgit draws it natively and read-only** in the main view: a simple
  layered box-and-arrow layout with hairline borders and prose labels. Keyboard:
  `j/k` cycles nodes, `enter` jumps to the node's hunk. That satisfies the "mouse
  has a keyboard equivalent" rule.
- **Editing and sharing go through `.tldr`.** "Open in godraw" writes the
  diagram as a `.tldr` file (documented JSON, which we generate ourselves) into
  the session dir and launches `godraw file.tldr` if it is installed. The same
  file opens in tldraw.com. No webview, no tldraw code, no dependency.
- **Ask egoist** (who also maintains MyGo) for a license and an exported read-only
  canvas package. If both arrive, swap our renderer for theirs. That is an
  upgrade, not a blocker.
- **Priority: P2.** Gnosis's README shows diagrams can be done (D2 diagrams,
  animated walkthroughs). But nothing I found shows that users of these tools
  lack diagrams, while there is concrete evidence of them struggling with basic
  diff reading (§8).

---

## 7. Next iterations (ordered by value)

### Iteration 1 — "Feels like lazygit" (P0)

1. Remove the header bar and summary card. Use a single continuous story list
   with section rows and 1-line hunk rows. The main view streams the whole
   section with an old/new line-number gutter.
   **Accept when:** a TestShots surface at 1512×916 has its first code line at or
   below 90 pt from the window top. At least 75% of the main-view height is code.
   The footer matches the regular hint bar height.
2. Navigation per §2.2–2.3, with all keys from the lazygit config.
   **Accept when:** headless tests drive `j` across a section boundary, `l`/`h`
   between sections, `]`/`[` skipping reviewed hunks with wrap, `enter` and line
   `j`, `h/l` hunk in the main view, `esc` back, and `J/K` scrolling without
   changing the selection. Remapping `nextItem` to another key works.
3. The `~` toggle with the mapping per §2.4. Remove the global `ctrl+g`.
   **Accept when:** from Staging on a hunk, `~` lands on that hunk in laziest.
   `~` back restores Staging on it. Marks and scroll survive 10 round trips.
   `ctrl+g` in every panel does lazygit's `jumpToFile` or nothing. The parity
   ratchet passes.
4. Persist marks in `$GIT_DIR/laziergit/review.json`, keyed by fingerprint.
   **Accept when:** marks survive a restart. Editing a reviewed hunk shows ↻.
   Nothing is written to the work tree.

### Iteration 2 — "Notes worth reading, code worth reading" (P1)

5. MCP v2: `read_file`, `search`, schema v2, the validator rules and the
   provenance label. `A` allows the new tools. The prompt is replaced and includes
   the worked example.
   **Accept when:** on the three fixtures, guides have 0 lint-rejected notes after
   at most 1 retry, every question has evidence, and two people agree S5 holds
   for each overview. The current guide fails these checks (keep it as the
   "before" fixture).
6. Chroma highlighting, intra-line emphasis and context `{`/`}` from blobs.
   **Accept when:** Go, TS and Markdown fixtures render with token colours in
   light and dark (shots). A multi-line raw string spanning a hunk boundary is
   coloured correctly. Scrolling a 5k-line section keeps 60 fps (bench). Lazier
   is unchanged (parity green).
7. Author mode in `M`: the copied config comes with a one-paragraph instruction
   the user can paste into their coding agent ("you wrote this change; publish
   it with provenance: author").

8. Line comments → agent: `c` on a line in the main view adds a comment, and the
   agent reads them with `get_review_feedback` (read-only for the agent, so no
   git writes). The status shows "n comments waiting".
   **Accept when:** a headless test adds 2 comments, an MCP client reads both with
   path, new-side line and hunk fingerprint, and the comments survive `~` and a
   restart.

### Iteration 3 — later (P2)

9. Diagrams per §6 (native read-only plus `.tldr` export).
10. A "changed since my last review" scope (Crit #1147, DiffPrism #265/#283; the
    X question "agents rewriting the same hunk mid-review?").
11. Go-to-definition and peek using the same backing as `search`/`read_file`.

---

## 8. Competitor lessons (verified; observation vs inference)

Identities were checked with the GitHub API on 2026-11-07. The user's pasted
descriptions were treated as leads.

| Project | Verified | Lesson for us |
|---|---|---|
| **Crit** — [tomasz-tomczyk/crit](https://github.com/tomasz-tomczyk/crit), MIT, ≈1.3k★, active | Story mode is real. [docs/story-mode.md](https://github.com/tomasz-tomczyk/crit/blob/main/docs/story-mode.md) calls it "an explainer, not a reviewer". Prologue, chapters and `support[]` for mechanical hunks. Hunk refs are `(file_path, old_start)`. Optional Mermaid. The agent "may open related source for context". Ingest repairs omitted hunks into `support`. | Copy the "explainer, not reviewer" stance, a separate mechanical bucket and agent context access. Keep our strict coverage instead of auto-repair. |
| | **[#869](https://github.com/tomasz-tomczyk/crit/issues/869)** (user Yofou): "Story mode often hides a file's diff ('These hunks are no longer in the diff.')", and they have to "swap back over to review tab just to read the diff". Fixed as a lazy-loading bug. | *Observation:* when the code disappears from the story, users leave the story. *For us:* the code always comes from the snapshot (S6), and the toggle returns to the same spot. |
| | **[#1035](https://github.com/tomasz-tomczyk/crit/issues/1035)**: asks for hunk-level reviewed marks, "finish a chapter without affecting another chapter". | Confirms our hunk-level marks. Keep them. |
| | **[#801](https://github.com/tomasz-tomczyk/crit/issues/801)**: comment rows "visually sever" the code. Asks for gutter markers. | Notes go in the gutter or above hunks, never between code lines (§3). |
| | **[#1147](https://github.com/tomasz-tomczyk/crit/issues/1147), [#1148](https://github.com/tomasz-tomczyk/crit/issues/1148)**: no round-to-round diff in range mode, and comments vanish when HEAD moves. | Review state must be keyed by content, not commit (our fingerprint approach), and persisted. Iteration 1.4 and 3.9. |
| | **[#1149](https://github.com/tomasz-tomczyk/crit/issues/1149)**: wants go-to-definition inside review. | Validates context tools (P1) and navigation (P2). |
| **Guided Review** — [nshntarora/guidedreview](https://github.com/nshntarora/guidedreview), Apache-2.0, 78★ | CLI and Chrome extension. LLM groups real hunks. Only 2 issues, neither about UX (#106 custom API base URL). | Little complaint data. Its "code always from the real diff" principle matches ours. |
| **Gnosis** — [oddur/gnosis](https://github.com/oddur/gnosis), MIT, 54★ | README confirms an ordered slideshow with a *why* per slide, optional D2 diagrams with animated walkthroughs, risk rating, "signal boost" filtering of trivial changes, split diff, Claude or Codex CLI. Issue tracker is essentially empty (#229 by the author). | *Inference:* the most feature-rich, but no user evidence for what's valued. Adopt "why per section" and trivial-change collapsing. Skip risk rating and chat. |
| **Review Assist** — [uditk2/review-assist](https://github.com/uditk2/review-assist), 1★ | The repo exists (description: intent documents, validator, MCP server, GitHub app). Details only from [mcpservers.org](https://mcpservers.org/servers/uditk2/review-assist): problem, assumptions, rejected alternatives, anchored walkthrough, coverage "5 of 5 changes explained". | The author agent writes the intent while it still has context. This is our **author mode** and provenance label. Not adopted: the GitHub App. |
| **DiffPrism** — [CodeJonesW/diffprism](https://github.com/CodeJonesW/diffprism), Apache-2.0, 3★ | MCP-integrated. Almost all issues were filed by the maintainer, e.g. [#255](https://github.com/CodeJonesW/diffprism/issues/255)/[#299](https://github.com/CodeJonesW/diffprism/issues/299) (diff background not reaching full width on long lines) and [#265](https://github.com/CodeJonesW/diffprism/issues/265)/[#283](https://github.com/CodeJonesW/diffprism/issues/283) (review rounds). | They show the maintainer's priorities, not user complaints. Basic diff rendering and rounds keep coming up. |
| **DiffLoupe** — [markerikson/diffloupe](https://github.com/markerikson/diffloupe), MIT, 12★ | Exists ("Understand your diffs before you merge them"). No issues. Not inspected further. | — |

**X/Twitter feedback** (from `docs/research/laziestgit-x-notes.md`, a Grok pass
that was partly rate-limited: reply threads unread, semantic search only). I
haven't checked the posts myself. What I take from it, and what I reject:

- **Adopted. Restating the code is the main failure.** Kenton Varda's widely
  shared moratorium post (1.8k likes) says AI descriptions "outline details of the
  code that could easily be seen by looking at the code". This is exactly the PoC
  failure, and it supports the why/how split and the lint in §4.
- **Adopted. The return path matters.** Several Hunk posts (d0xxed, Jnatanh,
  ryanlanciaux, bentlegen) describe the agent → tour → human notes → agent loop
  as what changes the work. I moved comments → agent from P2 to iteration 2
  (item 8).
- **Adopted. Hidden is not reviewed.** d0xxed hides tests and generated files to
  turn 2,000 lines into about 500. Mechanical hunks collapse, but the model never
  marks coverage. `]` still reaches them.
- **Noted, ranking unchanged.** Syntax colour wasn't a complaint. Density was
  (unchanged blocks, generated files, scrolling), plus one wish for "10× density…
  don't make me scroll". That supports doing iteration 1 (density and the
  continuous stream) before Chroma. I still keep highlighting in iteration 2,
  because the user asked for it and it's cheap with Chroma.
- **Rejected.** The note's suggestion that section text should say "what the
  reviewer should check". That is the checklist boilerplate we're removing. Doubt
  belongs in anchored `question` notes with evidence.
- **Keymap.** No real debate on X. The precedents (lumen `hjkl`/`}`/space;
  guidedreview.dev left/right units; DiffLoupe `j/k` files, `]/[` stops, from its
  README) are the same family as §2. No change.
- **Gap.** Reply threads on the Linear launch and the Kenton post weren't read.
  They are the likeliest source of complaints about guide quality.

---

## 9. Open decisions for the user

1. **Toggle key.** `~` (recommended), `;`, or keep a chord? This also confirms
   that removing global `ctrl+g` is acceptable.
2. **Is the git index the review state?** For working-tree scope, `space` could
   **stage** the hunk (lazygit's own `space`) instead of keeping separate marks.
   That gives persistence for free and makes "review then commit" a single motion.
   It conflicts with reviewing already-staged changes. Recommendation: keep
   separate marks for iteration 1 and try it as an option later.
3. **Expand-note key** (`e` proposed). laziest has no edit action, but `e` means
   "edit file" in lazygit. An alternative is `o` (openFile) to open at the line,
   with notes always expanded.
4. **Task text for cold-reader mode.** Should `A` prompt for "what did you ask the
   agent?" (one optional line)? It's cheap and improves S5 a lot.
   Recommendation: yes, optional, remembered per branch.
5. **Should we contact egoist** about a godraw license and an exported canvas
   package (§6)?

---

## Sources

- PoC code: `internal/app/guide*.go`, `internal/reviewbridge/mcp.go`,
  `internal/repo/guide.go`, `internal/app/actions.go` (`actionScopes`, line 519).
  Screenshots in `build/laziestgit-review/`, `/tmp/laziestgit-shots/staging-dark.png`.
- lazygit 0.66.2 default keybindings: `docs/Config.md` in the module cache
  (`jumpToFile: <ctrl+g>`, `prevTab/nextTab`, `scrollUpMain/DownMain`, screen modes).
- MyGo v0.7.4: `ui.RichText`/`ui.Span`, `ui/markdown` `CodeBlock` hook (no highlighter).
- godraw: https://github.com/egoist/godraw (README, tree, go.mod) and
  https://github.com/egoist/godraw/issues/5.
- Crit: https://github.com/tomasz-tomczyk/crit, docs/story-mode.md,
  internal/story/prompt.go, issues #801 #869 #1035 #1147 #1148 #1149.
- Guided Review: https://github.com/nshntarora/guidedreview (and #106).
- Gnosis: https://github.com/oddur/gnosis (README).
- Review Assist: https://github.com/uditk2/review-assist,
  https://mcpservers.org/servers/uditk2/review-assist.
- DiffPrism: https://github.com/CodeJonesW/diffprism (issues #255 #265 #283 #299).
- DiffLoupe: https://github.com/markerikson/diffloupe.
- X evidence: `docs/research/laziestgit-x-notes.md` (Grok pass, 2026-11-07; post URLs are in that file).

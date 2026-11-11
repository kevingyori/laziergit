# X notes: AI diff walkthroughs and local review

Research date: 2026-11-07. Audience: the product/UX review of laziestgit guide mode. This note does not judge the design. It records what showed up in search, then a short inference section.

## How this was searched

Native X search was partly available.

- `x_semantic_search` returned posts with ids, authors, dates, and text.
- `x_keyword_search` and `x_thread_fetch` failed on every attempt with HTTP 429 (Cloudflare 1015). Reply threads were not read unless a status page itself included them.
- Public `x.com` status and profile pages were fetched to recover named-product posts that semantic search missed or drowned in unrelated hits. Those pages are still X posts. They are not a complete search.

Semantic search is a poor recall tool for rare names. "Story mode", "Gnosis", and "DiffLoupe" queries mostly returned unrelated posts. Empty results below mean "not found in this pass", not "nobody has posted".

No posts were invented. Sentiment is not inferred for a product with no firsthand posts. Product READMEs and blogs are labeled as such and are not treated as X discussion.

## Do not conflate these names

| Name in the brief | What it actually is | What X uses the same words for |
| --- | --- | --- |
| Linear Diffs / Guides | Linear's in-app PR review. Guided Reviews group a diff and explain sections. Launched 2026-05-28. | "Guided review" on X usually means this, or a copy of the idea. |
| Crit Story Mode | [crit.md](https://crit.md/) / [tomasz-tomczyk/crit](https://github.com/tomasz-tomczyk/crit). Local browser review of agent plans, diffs, and apps. Story mode is a later, opt-in, LLM-authored chapter overview. The project says it is an explainer, not a reviewer. | "Crit" also hits Critty (`crittylabs/critty`), game "crit", and generic "critic" agents. No X post found that discusses Crit story mode. |
| Guided Review | [guidedreview.dev](https://guidedreview.dev/): Chrome extension plus local CLI. AI returns review units over a real parsed diff. Keyboard docs: left/right move units, up/down scroll code. | No X post found for guidedreview.dev. On X, "Guided Reviews" is Linear, Capy, Zuse, and guides.show. |
| Gnosis | [oddur/gnosis](https://github.com/oddur/gnosis): local Electron app. A PR becomes a themed slideshow (foundations, then features, then tests/config), with explanations and optional D2 diagrams. Uses the Claude Code or Codex CLI. | Not [skorokithakis/gnosis](https://github.com/skorokithakis/gnosis) (agent decision log), not Project Gnosis (repo maps), not the LLM "self-awareness" paper. |
| DiffLoupe | [markerikson/diffloupe](https://github.com/markerikson/diffloupe): local web UI. Groups files by code links, multi-agent findings, then a walkthrough. README keyboard: `j`/`k` files, `n`/`p` findings, `]`/`[` stops. | Not PHP Loupe, not the screenshot app named Loupe. |
| Review Assist | [uditk2/review-assist](https://github.com/uditk2/review-assist) / reviewassist.dev: session becomes an Intent Document, validated against the diff, rendered on the GitHub PR. | Not Graphite Reviewer, not GitHub Copilot code review. No X post found. |
| DiffPrism | [CodeJonesW/diffprism](https://github.com/CodeJonesW/diffprism): local browser PR/diff UI. Claude Code or Cursor comment through MCP. | Author posts only. No third-party X review found. |

Adjacent tools that did get practitioner posts: [modem-dev/hunk](https://github.com/modem-dev/hunk), Sahaj's lumen diff, Capy's Guided Reviews, [guides.show](https://guides.show) (plannotator), Claude Code's in-app diff, and Sourcegraph's unchanged-block hiding.

## What people actually posted

### Linear: vendor posts dominate

Firsthand, mostly from Linear:

- 2026-05-28, [@linear](https://x.com/linear/status/2060048202796040655) (1,573 likes, 58 replies, 65 quotes). Launch: realtime updates, "Guided reviews with AI (beta)", notifications, iterate with agents, threaded comments. Replies were not retrieved.
- 2026-06-11, [@hugorcd](https://x.com/hugorcd/status/2065157869012647977) (319 likes). Independent user: "The UI is so fucking good… the AI makes sense." This is praise of flow and polish. It does not say the guide caught or missed bugs.
- 2026-06-16, [@penzington](https://x.com/penzington/status/2066958532096782798) (897 likes). Linear engineer, about pre-release Diffs: the feature was "pretty bad" slow because DOM reads blocked the main thread for tens or hundreds of milliseconds. He describes fixing that. This is a builder post about a problem they claim to have fixed, not a current user complaint.
- 2026-10-07, [@karrisaarinen](https://x.com/karrisaarinen/status/2107676903553585155) (269 likes, 44 replies). Asks what people want for understanding agent changes. Lists: sections with explained code, speed on large diffs and thousands of files, live agent edits, themes and formatting, Cmd-F and Cmd-click symbols, rendered markdown and image diffs, phone review in beta. One reply retrieved: [@sekeidesign](https://x.com/sekeidesign/status/2107808920269901998) wants Diffs to be a command center for agent work, and says the review tab is not all of their open PRs.
- 2026-10-09, [@karrisaarinen](https://x.com/karrisaarinen/status/2108633559938609292). "Guided Diffs (ai makes you a review guide of the PR changes)" on paid plans. [@penzington](https://x.com/penzington/status/2108657460550471916) says guides are "the main reason people stick with Linear Diffs". That is the builder's claim, not a user quote.
- 2026-11-04, [@karrisaarinen](https://x.com/karrisaarinen/status/2117802472274264493), replying to Darren Shepherd's "Does a good diff viewer exist to review code from agents?": Linear's AI Guide "groups changes, explains them and skips over some." A new version is in progress.
- 2026-11-05, [@linear](https://x.com/linear/status/2118402509291065804). Mobile Diffs: guided reviews, synced viewed state, agent replies, merge from the phone.
- 2026-11-06, [@kenneth_skovhus](https://x.com/kenneth_skovhus/status/2118614580549079080). Linear employee: switched from GitHub, "can't live without Guided Reviews with the amount of code changes we are dealing with."

Linear's own blog, not an X post: [Reviewing code in the agent era](https://linear.app/now/reviewing-code-in-the-agent-era) (Maciek Pekala, 2026-06-10) says he scrolls once, opens a guided review when the change is large, and does not read line by line because agents now cover bug-finding. [Code review should be fast](https://linear.app/now/code-review-should-be-fast) (Tuomas Artman, 2026-05-28) describes chapters in reasoning order, core change first, glue collapsed, structural highlighting. Treat both as vendor description.

### The same shape, copied in public

- 2026-10-06, [@0xluffy](https://x.com/0xluffy/status/2107593041519501477) for Capy: file order wastes the first half hour; chapters in plain words; core change first; database, tests, and generated files last. [@DhravyaShah](https://x.com/elmd_/status/2107735673612783839) (parent on that page, 2026-10-06, 321 likes) calls it the best review experience he has had. [@elmd_](https://x.com/elmd_/status/2107735673612783839) asks how it compares to Linear. No answer was on the fetched page. [@coreyward](https://x.com/coreyward/status/2107664827774799952) (14 likes): "Why does this look so similar to Linear's implementation that's been around for months?"
- 2026-10-12, [@backnotprop](https://x.com/backnotprop/status/2109708148030246929). guides.show: open-source HTML chapter guides, "inspired by Linear". The quoted plannotator post says a smaller model is enough because the diff can be parsed by most levels of intelligence.
- 2026-10-23, [@swarajb](https://x.com/swarajb/status/2113735305463492751) (4 likes). Zuse copies the same guide shape and says so.
- 2026-11-06, [@ahmadyan](https://x.com/ahmadyan/status/2118527006921121832). Claims "guided diffs" started at a YC startup called Staged Review, later Vespin, and that his product and Linear copied it. That is his claim. It was not checked against a Staged Review source.

### Hunk: the densest firsthand local-review talk

- 2026-07-06, [@Meijin_garden](https://x.com/Meijin_garden/status/2074064016541127163) (136 likes, 206 bookmarks). Uses Crit, compares it with difit and Hunk, and is impressed that Hunk does the review from the CLI. A reply points at Codiff. This is preference, not a bug report.
- 2026-07-07, [@i_mika_el](https://x.com/i_mika_el/status/2074450412728402369), under a Hunk feature list: "watch mode feels like the hard part. does it handle agents rewriting the same hunk mid-review?" No answer on the fetched page.
- 2026-07-16 workflow, quoted from [@ryanlanciaux](https://x.com/ryanlanciaux/status/2080273288446533913): Hunk in one pane, agent in the other; notes on the diff go back to the agent before commit. He says he does not use it for every change.
- 2026-08-14, [@Jnatanh](https://x.com/Jnatanh/status/2088267718394794307) (281 likes). Hunk is his agent-first terminal diff viewer. He built a plugin so one key sends inline comments back to the cloud agent that wrote the change.
- 2026-10-29, [@bentlegen](https://x.com/bentlegen/status/2115807138279027147) quotes [@markobilal](https://x.com/bentlegen/status/2115807138279027147): reading all merged code is impossible, skipping the important parts is also wrong. His loop is Hunk plus a skill that polls user comments and replies inline while he reads.
- 2026-11-02, [@tomasz_wostal](https://x.com/tomasz_wostal/status/2117210201635848638). Hunk is the best local review tool he has tried, and "useless for teams" because notes are not a shared PR review.
- 2026-11-03, [@d0xxed](https://x.com/d0xxed/status/2117672152724083113) (162 likes, 231 bookmarks) and the [follow-up with the prompt](https://x.com/d0xxed/status/2117672153764315626). He has the agent open Hunk, annotate a guided tour (`3/22: …`), and hide tests, helpers, constructors, and generated files. A 2,000-line PR became about 500 lines. He trusts his test setup. He does not trust the model for architecture or idiom. Hunk's next-comment key follows file order, so the numbers are the reading path. He says vibe-merging loses the repo.

### Narrative versus paraphrase

The clearest complaint is not about a walkthrough product.

- 2026-07-08, [@KentonVarda](https://x.com/KentonVarda/status/2074924213983740233) (1,811 likes, 147 replies, 74 quotes). Moratorium on AI-written PR descriptions, commit messages, and tickets. They "outline details of the code that could easily be seen by looking at the code" and omit the higher-level framing. Structured output feels professional and is not useful. He would rather read the prompt. A good description is short if the author understands the change. Replies were not retrieved.
- 2026-01-14, [@addyosmani](https://x.com/addyosmani/status/2011338548763664475) (328 likes). Review the diff for correctness and the trajectory for intent. Same day he [agrees](https://x.com/addyosmani/status/2011341767673626715) with a reply that attached PRDs to PRs. This is a proposal, not a tool review.
- 2026-03-10, [@ThePrimeagen](https://x.com/ThePrimeagen/status/2031367166759506323) (505 likes). Wants review tools inside the editor. His "99" tool lists what is correct and incorrect about a change set within about three minutes and caught bugs he would have debugged. He calls guiding work under-explored next to code generation. Not a named product in the brief.

Crit's own docs (not X) say story mode is an explainer, costs on the order of $1–$1.40 for a complex PR on Claude Opus 5, and is not a verdict. No X post was found that confirms or disputes that.

### Keyboard, density, highlighting

Almost nobody on X argued about `j`/`k` versus `h`/`l` versus `[`/`]`.

- 2025-12-16, [@iamsahaj_xyz](https://x.com/iamsahaj_xyz/status/2001020330475970862) (585 likes). lumen diff: fast, tree-sitter highlighting, vim-like `hjkl`, `}` next changed hunk, space marks a file viewed.
- 2025-12-29, [@iamsahaj_xyz](https://x.com/iamsahaj_xyz/status/2005712582825619540) (759 likes). Shipped as `lumen diff`. The feature list below the post was not in the fetched text.
- 2026-06-25, [@beyang](https://x.com/beyang/status/2070253296917651674) (983 likes). Unchanged-code-block detection "reduces cognitive load" on long diffs, with GitHub as the worse screenshot. [@samdenty](https://x.com/samdenty/status/2070359493536301395) points at semanticdiff.com for AST diffs that ignore formatting.
- 2026-08-25, [@asselinpaul](https://x.com/0xluffy/status/2107666475528126942) (shown in that page's thread context, 3 likes; standalone status URL not retrieved). Wishlist: review from the prompt when the point is design, block diagrams, blast radius and compatibility, a fast question box with PR context loaded, the reviewer's rubric rather than the author's, semantic grouping, agents handling the first bot rounds, and "increase the information density 10x… don't make me scroll."

DiffLoupe's `j`/`k`, `]`/`[` map is README documentation, not an X discussion. [@acemarke](https://x.com/acemarke/status/2112401067128607009) (2026-10-20, 8 likes) only links the repo under someone else's "I built a review tool" post.

### Named tools with little or no X audience

- **Gnosis.** 2026-02-26 launch and [2026-02-28](https://x.com/oddur/status/2027747354774773863): Oddur says going from a coding agent back to manual diffs feels like "regressing from science fiction to medieval tools." The launch post shows 5 likes. No user report of the slideshow, diagrams, or trust was found. README, not X: optional D2 diagrams, animated SVG, syntax theme setting, signal boost that drops whitespace and import noise.
- **DiffPrism.** February 2026 posts on [@CodeWJones](https://x.com/CodeWJones) announce diffprism.com for reviewing agent work across worktrees, before a GitHub PR. Shown engagement is 0 likes. No user reply was found.
- **DiffLoupe.** Only the author link above. No praise or complaint about intent alignment, findings, or the walkthrough.
- **Review Assist.** No X post found for the Intent Document product.
- **guidedreview.dev.** No X post found. Its keyboard and "the code on screen is the real diff" behavior are docs, not discussion.
- **Crit story mode.** No X post found. Earlier Crit posts on [@tomasztomczyk](https://x.com/tomasztomczyk) (2026-03) describe browser inline comments, round diffs, vim keys, and plan review. They predate the story-mode changelog (project site: 2026-09-29).
- **lazygit.** The lazygit posts found were ordinary TUI staging workflows, for example [@kamon](https://x.com/kamon/status/2087961546856997055) (2026-08-13). Nothing tied lazygit to an AI guide.
- **godraw.** No X mention found.

Claude's own in-app diff, 2026-01-15, [@claudeai](https://x.com/claudeai/status/2011925349341483413) (3,595 likes) and the [follow-up](https://x.com/claudeai/status/2011925351656763736): full diffs and inline comments without switching to GitHub or an IDE. Vendor announcement. Replies not read.

## Conflicts in the evidence

These are disagreements between posts, not a resolved finding.

- **Hide noise, or show the frame.** Linear, Capy, Zuse, d0, and Beyang all want less file-order scrolling and less boilerplate on screen. Kenton says AI text that restates visible code is worse than useless, and that the missing piece is high-level framing. Hiding tests (d0) conflicts with wanting blast radius and guarantees (the August 2026 wishlist).
- **Who still reads lines.** Linear's blog says bug-finding has mostly moved to agents, so the human reads fit and intent. d0 says the opposite about judgment: he skips tests because he trusts them, and he still reads architecture because he does not trust the model. Primeagen wants a list of concrete correct/incorrect claims, not a story.
- **Author versus reviewer control.** Capy and Linear guides follow the change's structure. The August wishlist asks for the reviewer's rubric, not the author's tour.
- **Local notes versus team review.** Hunk posts praise the local agent loop. Tomasz Wostal says that loop is useless for a team. Linear's pitch is the shared PR, inside the issue tracker.
- **Speed claims.** Terminal authors call their viewers fast. Linear's engineer said the web diff was badly slow before launch, then that large diffs are fast. No post compares either to lazygit. No numbers were posted by users.
- **"Best review experience" is contested and mostly vendor-adjacent.** Hugo (independent) praises Linear's UI. Kenneth and Maciek work at Linear. Dhravya praises Capy. Corey thinks Capy looks like Linear. Nobody in the fetched posts compared these with Crit, Gnosis, DiffLoupe, or a TUI.

## Inference for laziestgit

This section is the researcher's reading. It is not something a poster said.

The current guide (`docs/LAZIESTGIT.md`) already matches the parts X is loudest about: one key into the review (`Ctrl+G`), the real diff stays on screen, `j`/`k` move hunks, `h`/`l` move sections, `]` jumps to the next unreviewed hunk, and the agent publishes sections over MCP without a git-write tool. The gap X points at is what the words are for, and what "skipped" means.

- Write the section line as framing Kenton asked for: why this group exists, what depends on it, what the reviewer should check. A sentence that restates the hunk is the failure mode with the most engagement in this search.
- If the agent hides tests, lockfiles, or helpers, keep them reachable and unmarked. d0 hides them on purpose. A hidden hunk that looks reviewed would cross the other complaint, that important code gets skipped. The existing unreviewed jump is the right status model. Do not let the model mark coverage.
- Keep guide text beside the native diff. Every walkthrough that got posts still shows the code. Crit's docs, off X, call the story an explainer. That split is the one to keep: the guide orders and frames; it does not approve.
- One key from the lazygit screen is the context-switch complaint (GitHub, a browser, a second app) answered without leaving the TUI. X did not ask for lazygit. It did praise Hunk and lumen for being local, fast, and next to the agent. A browser guide would give up the thing those posts are about.
- Do not retune `j`/`k`, `h`/`l`, and `]` from this search. The only firsthand keymap is lumen's `hjkl`, `}`, and space. DiffLoupe's similar keys are a README. laziestgit's order is already in that family.
- Syntax highlighting and color were not a complaint. Density was: unchanged blocks, generated files, and scrolling. Structural folding beats another highlighter if speed has to be spent somewhere.
- Diagrams showed up once as a wish (block diagrams of how the change fits) and in Gnosis's README as D2 and animated SVG. No post mentions godraw. A diagram is worth a section only when it shows structure the diff does not. An agent-drawn picture of the code already on screen repeats Kenton's failure mode in another medium.
- The MCP shape people praise on Hunk is two-way: the agent writes the tour, the human writes line notes, the agent reads them. Publishing a guide is the first half. The posts treat the return path as the part that changes the work.

## What this pass could not settle

Reply threads on the Linear launch (58 replies), Kenton's moratorium (147 replies), and Karri's October question (44 replies) were not read, because thread fetch was rate-limited. Those threads are the most likely place for complaints about guide quality, trust, and missing keyboard behavior. A later keyword search, after the 429 clears, should start with `from:karrisaarinen Diffs`, `conversation_id` of `2060048202796040655` and `2074924213983740233`, and the exact strings `diffloupe`, `reviewassist.dev`, `guidedreview.dev`, and `crit story`.

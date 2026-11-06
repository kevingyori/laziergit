# Oracle consults

No budget: the user removed the daily limit on 2026-11-05. Consult whenever
the rules below call for it; every consult still gets an entry here.

The parity agent consults a stronger model when it is stuck, needs a design
decision, or wants to change the harness. `parity/oracle.sh take "<question>"`
adds an entry below and prints its id; fill in the answer. A harness change
must cite an entry whose answer approves it, in the commit message:
`Oracle-approved: O-2026-11-05-1`.


## O-2026-11-05-1: file-tree: render the files and commitFiles views from the app's lazygit tree view models with lazygit's RenderFileTree/RenderCommitFileTree

Answer: Approved. Answered by the session that set up this environment
(Claude Opus 5.5) as the reference example. The files and commitFiles
projections now call presentation.RenderFileTree / RenderCommitFileTree on
the app's own lazygit view models (a.files, commitFiles.tree) with the same
arguments lazygit's working-tree and commit-files contexts pass, so the
projection is lazygit's rendering of the app's actual rows and selection.
Future changes to those trees need no harness change.

## O-2026-11-05-2: key-d-localbranches: project lazygit's tooltip view for the menu

Answer: Approved. Answered by the parent session acting as oracle (user
granted oracle authority). The harness had no `tooltip` view, so
`Tooltip(Contains(...))` always saw empty text. The projection mirrors
lazygit's ConfirmationHelper.TooltipForMenuItem for the selected visible menu
item: Tr.DisabledMenuItemPrefix + the item's disabled reason (the app's
menuItem has no separate tooltip text yet; when one is added, prepend it with
"\n\n" here). Verified: branch/delete now passes the tooltip assertion and
stops later at remoteBranches.

## O-2026-11-05-3: key-a-commits: menu projection should match " Set author" (leading space)

Answer: Not approved. lazygit renders menu rows as "<key> <label>" and gives
the author menu keys (amendAttribute.resetAuthor/setAuthor/addCoAuthor:
a/A/c); the test's leading space matches the space after the key. The
projection already renders key + label. The fix belongs in the app: give the
author menu items lazygit's keys.

## O-2026-11-05-4: remote-branches: project lazygit's remoteBranches view

Answer: Approved. Answered by the parent session acting as oracle. The
harness had no remoteBranches view, so every test that presses <enter> in
remotes failed at the focus check even though the app drills in. When the
app's branches panel shows a remote's branches (refBrowse with no ref), the
branches window's top view is remoteBranches and its rows are lazygit's
GetRemoteBranchListDisplayStrings of the app's own list, with the app's
cursor. Verified: sort_remote_branches_in_stack_order passes; the others now
reach remoteBranches actions (d, n, g, s, /).

## O-2026-11-05-5: search: project lazygit's search view

Answer: Approved by the user. The harness had no `search` view, so every
search test failed at "Expected search prompt to be focused". focusedView
returns "search" for the app's *searchOverlay, and the "search" view's
single line is a.searchStatus() (the prompt text while open, else lazygit's
"matches for…" status), visible when the app shows it. Patch:
parity/proposed-harness-search.patch, to apply when merging t3/10f85922
(it needs a.searchStatus from that branch). Measured: 20 of 23 search tests pass.

## O-2026-11-05-6: focused diffs: approve range, patch inclusion, paired secondary content and Building patch information projections from real app state

Answer: Partially approved by Claude Opus 5.5. Range projection and inclusion
gutter are approved after sharing rangeAnchor() between production and harness,
requiring a valid cursor, and drawing patch marks independently of the cursor.
Information, main title and secondary content/visibility were not approved:
production must first expose modeInfo(), mainTitle and secondaryVisible(), use
Tr.CustomPatch/Tr.DiffTitle, and render a real persistent patch mode label.
Those three hunks will be resubmitted separately after production corrections.

## O-2026-11-05-7: focused diffs: resubmit actual modeInfo, mainTitle/mainVisible and secondaryVisible cached content projections after production corrections

Answer: Approved by Claude Opus 5.5 after inspecting the exact proposed diff
and production fields/methods. The information text matches the visible mode
bar; mainVisible/mainTitle and secondaryVisible/secondaryText/secondaryTitle
match the real pane layout, including staged-only changes. Apply and gofmt,
then commit separately with this consult ID. Production follow-ups noted:
use Patch/LogTitle outside Files, keep the unfocused source pane refreshed,
and leave other modes for their corresponding feature tasks.

## O-2026-11-05-8: custom patches: project real inactive source diff state, commit-file ranges and visible empty commit placeholder

Answer: Approved by Claude Opus 5.5 as written. mainDiffState is exactly the
source state drawn by diffPanes, including its dim selection and independent
patch marks while the preview is focused. Commit-file ranges use the same
selectionRange as native row highlighting. The empty-commit placeholder uses
the same production helper and label as the native section. Apply as a
separate harness commit with this consult ID. An unselected muted empty row
is a remaining presentation difference from lazygit, not fabricated parity.

## O-2026-11-05-9: Approve rebase commits projection: use captured updateRefs config for lazygit renderer and map real commit ranges through todo section headers.

Answer: Approved. The native app keeps a real panelCommits range but View only projects commitFiles ranges. Map the actual range anchor through commitLines so headers receive the same offsets as the cursor. Require commitFiles == nil. Pass Snapshot.RebaseUpdateRefs (captured in Load from Git.Config.GetRebaseUpdateRefs) instead of the hardcoded false, matching lazygit local_commits_context.go and presentation/commits.go. No scoring changes. Conditions: commit the production snapshot field before the separate harness commit; gofmt the added block. The extra renderer call is acceptable for tests. Native rendering must also hide branch heads on todo rows with updateRefs (native rows currently draw no branch heads). Oracle: Claude Opus 5.5, approved 2026-11-05.

## O-2026-11-05-10: Review and update merge-conflicts-view harness projection for the native conflictOverlay without changing scoring or lazygit tests

Answer: Approved by Claude Opus 5.5 (user asked Opus to update the harness),
2026-11-05, for exactly the three hunks below and nothing else. The harness
had no `mergeConflicts` view: focusedView reported `*app.conflictOverlay`, so
all 19 merge-conflicts-view tests failed at the focus check after <enter> in
files although the app opened its conflict view. The projection is real app
state: focusedView names the conflictOverlay `mergeConflicts` (lazygit's
MERGE_CONFLICTS_CONTEXT_KEY, window "main", already in windowOf); its lines
are utils.SplitLines of the overlay's own mergeconflicts.State content (the
text lazygit's ColoredConflictFile colours); its selection mirrors lazygit's
MergeConflictsContext.SetSelectedLineRange (range start = hunk start, cursor
= hunk end) from the same GetSelectedRange the native view uses to highlight
rows. Once no conflict remains the native view draws no file content, so the
projection is empty; lazygit's flow (leave the view, ask to continue) is then
the app's job, not the harness's. No scoring, matcher or lazygit test change.
Not approved, and not part of this change: a TopViewInWindow("main") mapping
or a title for the view (the native card title is the app's own wording);
add them only when a test needs them and the app shows lazygit's view.
Known application differences this does not hide: the native view renders
only lines start-4..end+5 of the file (lazygit shows the whole file,
scrollable); the "All conflicts resolved" card replaces lazygit's
auto-return to files and continue prompt.

```diff
--- a/internal/app/parity_driver_test.go
+++ b/internal/app/parity_driver_test.go
@@ focusedView
 	case *searchOverlay:
 		return "search"
+	case *conflictOverlay:
+		return "mergeConflicts"
@@ View
+	if name == "mergeConflicts" {
+		// lazygit's MergeConflictsContext.SetSelectedLineRange: the selected
+		// hunk is always a range, anchored at its start, cursor at its end.
+		if o, ok := d.overlay(func(o overlay) bool { _, ok := o.(*conflictOverlay); return ok }).(*conflictOverlay); ok && !o.state.NoConflicts() {
+			start, _ := o.state.GetSelectedRange()
+			v.SetRangeSelectStart(start)
+		}
+	}
@@ project
+	case "mergeConflicts":
+		if o, ok := d.overlay(func(o overlay) bool { _, ok := o.(*conflictOverlay); return ok }).(*conflictOverlay); ok && !o.state.NoConflicts() {
+			_, end := o.state.GetSelectedRange()
+			return utils.SplitLines(o.state.GetContent()), end
+		}
+		return nil, -1
```

## O-2026-11-05-11: shared-suggestions: approve faithful projection of actual suggestion values, focus and selection in parity driver

Answer: Approved. The diff is faithful, and I found nothing that adds a result the app doesn't produce.

**Scope:** only the two hunks in `internal/app/parity_driver_test.go` as written: the `focusedView` branch for `suggestions` and the `suggestions` case in `project`. No changes to tests, matchers, scoring, the ratchet or key driving.

**Why it's faithful**
- **Focus:** `focusedView` returns `suggestions` only when the app's own `suggestionsFocused` is set. That flag is also what `wantsTextFocus` and `CursorVisible` already read, so focus and cursor reporting agree. Lazygit's `PromptController` switches to the suggestions context only when there are suggestions. The app's prompt only sets the flag when `len(o.matches()) > 0`, so the projection can't claim focus the app doesn't have.
- **Lines:** `o.matches()` is the same call `promptOverlay.view` and `promptOverlay.key` make on every frame and key. The clamp it applies to `suggestionCursor` is a write the UI already makes on every frame, so it can't mask an out-of-range cursor. The rows are exactly what the app lists.
- **Selection:** the real cursor is reported while suggestions are focused, and nothing is reported while the prompt is focused, which matches the app only highlighting a row when focused. The `-1` leaves the gocui cursor at 0, which is what lazygit's `SetSuggestions` gives after typing. `ConfirmFirstSuggestion`'s `IsFocused().SelectedLineIdx(0)` therefore checks the app's real state after Tab.
- **Visibility:** lazygit sets `Suggestions.Visible = true` only when a `FindSuggestionsFunc` exists, and hides it when the popup closes. "No provider means invisible" mirrors that. A provider with zero matches still shows a visible view with 0 lines, which is also correct.

**Non-blocking notes (not conditions of approval)**
1. The `suggestions` case looks at `a.topOverlay()`, while `prompt` searches the whole stack with `d.overlay(...)`. If something is ever stacked over a prompt, `prompt` would still project but `suggestions` would read invisible. Lazygit doesn't stack popups like that, so this is acceptable; just keep it in mind if such a test shows up.
2. `v.Title` and `v.Subtitle` aren't projected. That's correct to leave out for now. If a later test asserts them, project the strings the app actually renders, not lazygit's.

**App gaps that should stay as test failures, not be fixed in the adapter**
- **Title key label:** lazygit's title is `fmt.Sprintf(SuggestionsTitle, Universal.TogglePanel)`, using the raw `<tab>` string. The app formats it with `keymap.Parse(...).Label()`, so the wording may differ.
- **Missing subtitle:** when delete is supported, lazygit's `switchToSuggestions` sets `SuggestionsSubtitle` (the remove and edit keys). The app doesn't render this.
- **Label vs value:** lazygit shows each suggestion's `Label`, while the app shows the value string. Any preset where the two differ should fail rather than be patched over in the driver.

## O-2026-11-05-12: branch-commits: project the subCommits view (lazygit's SubCommitsContext) in the parity driver: topView shows subCommits while the branches panel has a ref drill-down, window and title, scroll origin and range start come from the app's own state, and its commit rows are rendered with lazygit's commit presentation plus the Local/Remote divergence headers

Answer: Not approved. The projected Local/Remote divergence headers are rows the native branches panel never draws (no code outside tests mentions them), so the two show_divergence tests would pass on output the app does not show. Required: drop the headers from the projection, or first draw them in the app and resubmit.

## O-2026-11-05-13: branch-commits (resubmit of O-2026-11-05-12): project the subCommits view in the parity driver without divergence headers, which the app does not draw yet

Answer: Not approved. Divergence headers are gone, but the rows still show lazygit's graph (the native sub-commits rows draw no graph, so cherry_pick_merge would pass on lines the app does not draw), branch-head markers are projected although the native rows draw none, and the hidden localBranches/tags projections read the sub-commit cursor while a drill-down is open. Required: showGraph false, no branches, and the saved tab cursor for hidden tab views.

## O-2026-11-05-14: branch-commits (resubmit of O-2026-11-05-13): project the subCommits rows exactly as the native view draws them (no graph, no branch-head markers, no divergence headers); cursorIf uses the saved tab cursor while a commits drill-down is open

Answer: Approved. Every projected value matches the app's real state: rows (one per commit, model index = line), cursor, range start, title (subCommitsTitle), origin (lists[panelBranches].Visible()), focus (refBrowse ref set), and cursorIf uses the saved tab cursor for both drill-down entry points. No path lets a test pass that lazygit would not allow. Non-blocking: the hidden remotes view still reads the remote-branch cursor while the remote list is open (pre-existing; a later fix should use the tab cursor there too). Approved and committed as 1bfb01d.

## O-2026-11-05-15: reflog-tab: project lazygit's reflogCommits view and move subCommits into the commits window when opened from a reflog entry

Answer: Approved (Claude Opus 5.5 oracle). The reflogCommits projection
renders the app's own a.snap.Reflog with lazygit's
GetReflogCommitListDisplayStrings using ReflogCommitsContext's arguments; its
selection is the live reflog cursor when shown, else the tab's saved cursor.
topView("commits") reads a.commitsView(), so a wrong view in the app still
fails. subCommits' window, cursor, origin and range follow refBrowse.panel,
matching lazygit's SetWindowName(opts.Context.GetWindowName()); the branches
window counts the drill-down only when it really lives there. The commits
cursor is active only while the commits tab shows. Nothing forces a result
or skips an assertion.

## O-2026-11-05-16: user: remove the daily oracle budget (oracle.sh no longer refuses consults)

Answer: Approved by the user, who owns the oracle rules. oracle.sh `take`
never refuses and `left` prints "unlimited"; nothing about scoring or the
parity tests changes.

## O-2026-11-05-17: worktree-management: project lazygit's status line with RepoPaths.RepoName() and the linked worktree name (GetLinkedWorktreeName) instead of basename(root) and ""

Answer: Approved (Claude Opus 5.5 oracle). The status projection reads the
app's own RepoPaths.RepoName() and s.Worktrees snapshot, matching both the
native status row and lazygit's statusController (RepoName,
GetLinkedWorktreeName); the old basename(root) projected the linked worktree's
folder name, which neither shows. No effect outside linked worktrees.
Non-blocking: with Head == nil the projection omits the linked name while the
native row still draws it.

## O-2026-11-05-18: Approve harness: inactive-tab selection, flat-list range projection, test-configured clipboard command

Answer: Approved. tabSel projects the app's own per-tab saved cursor (lazygit contexts keep their selection); flat-list range projection uses the app's multipleSelected/selectionRange only for the window's top view; the clipboard runs lazygit's OS.CopyToClipboard only when the test configures its own CopyToClipboardCmd. Note: hidden-tab cursor is unclamped; consider clamping later.

## O-2026-11-05-19: bisect: project the app's BisectInfo in the commits projection instead of NewNullBisectInfo?

Answer: Approved. lazygit's LocalCommitsContext passes Model().BisectInfo (from git.Bisect.GetInfo() at commits refresh) to GetCommitListDisplayStrings; subCommits pass a null info, so subCommitLines stays null. Conditions: a.bisectInfo() returns exactly the Snapshot.BisectInfo lazygit's GetInfo() produced, falling back to NewNullBisectInfo(); the snapshot reloads after every bisect command.

## O-2026-11-06-1: sync-remaining: project menu prompt lines (+blank) before items and items' detail column in the harness menu view

Answer: Approved (Claude Opus 5.5 oracle). lazygit's menu view shows the
prompt lines plus one blank line (GetNonModelItems, layoutMenuPrompt) before
the items, and item rows carry all LabelColumns; the selection index skips the
non-model lines, so the cursor offset is right. Conditions: prompt lines are
not trimmed; the blank line only with a prompt; no wrapping (a future
wrapping failure is a new consult).

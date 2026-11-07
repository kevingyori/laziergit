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

## O-2026-11-06-2: diffing-copy: harness toast sequence counter + menu item tooltip projection

Answer: Approved. lazygit raises a toast on every call even when the text repeats, so recording each non-empty setStatus via statusSeq matches it; the tooltip projection now follows TooltipForMenuItem (tooltip, then "\n\n" + DisabledMenuItemPrefix + reason). Watch for app code that re-sets the same status as a side effect: extra toasts are app bugs, not a reason to loosen the harness.

## O-2026-11-06-3: recent-repos: project menu items' label columns padded like lazygit's RenderDisplayStrings

Answer: Approved. Menu projection renders lazygit's RenderDisplayStrings over [label, columns...] when some visible item has columns: widths from every row's cells (Cancel's label counts toward the first column), every cell but a row's last padded, the key kept first. Other menus project as before.

## O-2026-11-06-4: recent-repos: tooltip projection invisible when the selected item's tooltip is empty

Answer: Approved. Tooltip projection is visible only while a menu is open and the final tooltip text (after the disabled-reason suffix) is non-empty, as lazygit's layout (Tooltip.Visible = Menu.Visible && Buffer != ""). Only recent_repos_column_widths (IsInvisible) and filter_menu_as_you_type (IsVisible with text) use it.

## O-2026-11-06-5: submodules: project the files view's range selection (add files to the flat-list range map in parityDriver.View)?

Answer: Approved. lazygit's WorkingTreeContext range-selects over the visible tree rows (root row included), which the app's files rows match 1:1; the projection is the range the app draws and acts on; the topView guard keeps it to the Files tab. Commit separately with Oracle-approved.

## O-2026-11-06-6: file-tree: may PretendMergeOrRebaseStartedInLazygit set the app's merge-started-in-lazygit flag?

Answer: Approved. `PretendMergeOrRebaseStartedInLazygit` sets `d.a.mergeStartedInLazygit = true`, mirroring lazygit's gui_driver. App-side conditions: clear the flag only on a refresh that finds no operation in progress; prompt only on a conflict count going from >0 to 0 (previous file state), with lazygit's wording (Continue / ConflictsResolved with the command name).

## O-2026-11-06-7: main-view-cursor: project side-list Highlight/HighlightInactive like lazygit's UpdateSelectionHighlights?

Answer: Approved. Side list views (files/branches/commits windows, stash, status) project Highlight = on the stack (the current side context, possibly under a focused main view or popup) && has rows, HighlightInactive = not the current context, as lazygit's UpdateSelectionHighlights. If a list ever draws a placeholder row with no items, compare against the item count instead of row text.

## O-2026-11-06-8: options-bar: approve options-view projection reading App.optionsText()

Answer: Approved. The projection reads optionsText(), which joins the same optionsMap() list the hint bar draws, so it reports what the app shows; it holds only while the bar renders straight from optionsMap(). The missing "…" truncation at the view width is a known gap; the two interactive_rebase revert tests are tracked under the information-view failure.

## O-2026-11-06-9: Harness: stub a.terminalCommand so subprocesses (commit in editor) run headless instead of opening Terminal.app

Answer: Approved. Harness-only change to how subprocesses launch; run a.terminalScript with /bin/sh (same path as runTerminal), stdin not a TTY, inherit the environment.

## O-2026-11-06-10: Harness: commitDescription view projects the description with lazygit's auto-wrap soft line breaks (commitOverlay.wrappedDescription)

Answer: Approved. The projection shows the description as lazygit's text area content (soft breaks as lines), the same function the commit path uses; submit commits wrappedDescription exactly, reword unwraps only when autoWrapCommitMessage is on.

## O-2026-11-06-11: main-view origin: projection of the app's scroll origin, and terminal geometry (Width/Height) for main view scroll tests

Answer: (A) Approved: View() sets main/secondary origin from a.paneOrigin(name), the app's real first visible row (projected-line index) and horizontal offset in mono columns; no stored pretend origin; projected view stays 400x2000. (B) Approved: the app scrolls with a cells model of its real panes (rows = px/rowHeight, cols = px/mono advance; ScrollRight innerCols/3, keep-position by middle visible row, scrollPastBottom clamps); the harness sizes the tester from it.HeadlessDimensions() only when not lazygit's 150x100 default (w = tw*monoAdvance(), h = th*rowHeight, monoAdvance the app's own function); layout mismatches are app bugs. (C) Realistic: scroll_diff_horizontally, jump_to_another_file_scrolls_to_the_left_edge, scroll_back_when_the_diff_gets_narrower, search_scrolls_to_a_match_off_the_edge, file_navigation_scrolls_to_the_top, select_visible_change/hunk_on_focusing_main_view, keep_position_* (context size, ignoring whitespace, off screen, visible end, both panes), keep_selected_range_when_changing_context_size. Deferred (renderer redesign): keep_position*_switching_diff_renderers, raw_fallback_under_an_external_diff, keep_scroll_when_the_diff_cant_be_read, keep_a_wrapped_line_covered_across_a_rerender. Ratchet holds.

Amendment (approved): View() sets the main/secondary origin before the cursor and sets the cursor relative to it (`v.SetCursor(0, sel-v.OriginY())`), as gocui stores cy; SetRangeSelectStart stays absolute. A selection above the origin is an app bug, not clamped.

Amendment 2 (approved): harness sizes the window so the app's main viewport equals lazygit's main inner cells: render at (tw*monoAdvance, th*rowHeight), then grow/shrink by the difference between App.MainPaneCells() and lazygit's main inner size from helpers.GetWindowDimensions (files focused, MainPaneOnly, normal screen; inner = Y1-Y0+1-2), failing the test as a harness error if they still differ. No per-test constants.

Amendment 3 (approved): the sizing uses lazygit's split at startup (BothMainPanes for a selected file with staged and unstaged changes or splitDiff always, else MainPaneOnly) and fails as an app bug when the app's split differs; still main compared to main, no SecondaryPaneOnly case.

## O-2026-11-06-12: keybindings menu: menu projection with lazygit's section header lines, menuFilter view, menu origin

Answer: Approved (1-3). 1: menu projection inserts "─── <title>" before each new section (tracking the last emitted section, items without a section don't reset it), a blank line before every header but the first, none while a non-empty filter uses fuzzy search; selected = the cursor item's line. 2: "menuFilter" view shows [m.filter] while the filter is open (m.filtering || m.filter != ""), else invisible. 3: menu origin = the projection line of the first item the native list shows, stepped back over the header (and blank) its row draws; cursor relative to the origin.

## O-2026-11-06-13: leftovers: four harness projections (unfocused main patch marks, main/secondary TitlePrefix, menu key column padding, run a test's own os.openLink)

Answer: Approved (1-4). 1: unfocused main gutter reads only App.mainPatchMarks(), off when nil or a staging state exists, marks from the same diffRows the projection prints. 2: TitlePrefix only from App.focusMainJumpLabel, on exactly one pane. 3: key column padded over visible items, dropped when no item has a key (RenderDisplayStrings); ratchet stays green. 4: only a test config that sets os.openLink runs r.OS.OpenLink; the default browser path stays a no-op.

## O-2026-11-06-14: leftovers: menu checkmark column, commits marked base and CheckedOutBranch in the commits projection

Answer: Approved (1-3). 1: widget holds only lazygit's MenuWidget strings ("( )", "(•)", "[ ]", "[✓]"), never repeated in the label; Cancel indented by the column width. 2: the projection reads only App.markedBaseCommit; the harness never sets or clears it. 3: Snapshot.CheckedOutBranch computed in internal/repo as determineCheckedOutRef (rebased branch, bisect start, else HEAD's branch, "" on a detached head), never derived in the harness.

## O-2026-11-06-15: sub-commits: project graph, branch heads and divergence headers (now drawn natively); selected commit as a pool pointer while focused

Answer: Approved (1-2). 1: subCommits rows with branches (showBranchHeads), the graph (git.log.showGraph "always", the native rows' condition; both change together if "when-maximised" is ever supported) in gui.commitGraphStyle's symbol set as commitLines, and "─── Remote"/"─── Local" headers from refBrowseState.divergenceHeaders(), shown with zero commits, selection shifted past headers above it. 2: selected commit as the pool pointer (HashPtr) only while commits/subCommits is the focused view; the native heavier lines use the same focus rule (none under an overlay, a focused main view or staging).

## O-2026-11-06-16: main split: harness window sizing steps until the main pane agrees (12 steps instead of 3)

Answer: Approved. The loop stays capped at 12 steps with the same step formula and the same size target; a size still off after 12 steps fails as before, never skipped.

## O-2026-11-06-17: window sizing: lazygit's main view size with the extras window (ShowExtrasWindow) as lazygit starts

Answer: Approved. ShowExtrasWindow comes from the app's live a.showLog at startup, never hard-coded; a.showLog is lazygit's gui.go expression (gui.showCommandLog && !AppState.HideCommandLog), with the harness app state leaving HideCommandLog false (lazygit's default).

## O-2026-11-07-1: screen-modes: project appStatus visibility and fullDescription (branches, commits, reflog, subCommits) from the app's screen mode

Answer: Approved. appStatus Visible = a.busy != "" (lazygit adds the window only with a status); fullDescription = a.enlarged() (ScreenMode != SCREEN_NORMAL) in localBranches, commits, subCommits and reflog, nothing else; showGraph stays "always" only (O-15). Native rows show upstream/author when enlarged.

## O-2026-11-07-2: side-panels: harness reads app's configured gui.sidePanels layout for windowOf/topView/sideView, FocusIn calls App.FocusIn, transient views invisible unless top of window

Answer: Approved. Conditions: run the full ratchet first and commit the harness on its own. commitFiles should take the window of its parent view the way lazygit's does (the app draws it in the commits panel, so this is left as a comment in SideWindowOf). Comment sideViews so it stays in step with sideViewLazygitNames.

## O-2026-11-07-3: remaining-20: harness projections: true-colour level, side-list viewport (height/origin from the app's list), wrapped main rows

Answer: 1 Approved (force true colour only around projection, restored with defer; full ratchet first; harness committed alone). 2 Approved (height/origin only from the app's ListState.Visible() of the frame drawn after the last key; inner height = visible rows exactly; non-top views stay 400x2000; never pad or clamp). 3 No harness route: ui/accordion and ui/page_up_and_down are app layout blockers ("side-panel row allocation (lazygit layout weights/accordion)") until the app lays side panels out in lazygit's row model. 4 Shape approved: App.PaneWraps(name) drives both native pane and projection (non-diff always, diff iff gui.wrapLinesInDiffView), width = App.paneInnerWidth(name), the app must wrap exactly as gocui (word breaks, long words, tabs, display width) with a unit test against gocui; ranges from gocui's own mapping; submit the diff for a final check before committing.
Item 4 final check: Approved (harness wrap projection reviewed; wrapViewLine uses gocui's own View wrapping; wrapCache is cleared at ~20k entries, and any caching change must keep gocui as the source of wrapped lines).

## O-2026-11-07-4: mouse step 1: harness click/double-click/Alt-Shift-click/FocusInAndClick via view bands + App.MouseCell, real Tester pointer events

Answer: Approved. Conditions: MouseCell reads only bounds recorded from elements built in the last frame (plus row-distance extrapolation), never app state or handlers; each span names the lazygit function it copies (GetOnClick arrow, StatusController.onClick cursorInSubstring, diffstat hyperlink columns), any span-rule change needs a new consult; unreachable cells stay `unsupported` (no default point); views taller than a band fail as unsupported; edit_clicked_diff_line and open_link_failure stay failing until the app matches; harness committed alone; wheel and drag are separate consults.

## O-2026-11-07-5: mouse step 2: harness ScrollWheelDown as one wheel notch via Tester.Wheel; startup sizing for SecondaryPaneOnly

Answer: Approved. Replaces O-2026-11-06-11 amendment 3 ("no SecondaryPaneOnly case") for the files panel: lazygit's FilesController.GetOnRenderToMain (files_controller.go:436-465) leaves Main nil for a file with staged and no unstaged changes and splitDiff not always, so mainPanesFor gives SecondaryPaneOnly; the harness then sizes the secondary pane against dims["secondary"]. One ScrollWheelDown call is exactly one non-precise Tester.Wheel notch; the harness never reaches a list or pane directly. Full run before commit.

## O-2026-11-07-6: mouse step 3: harness ClickAndHold/MouseMove/MouseRelease as real Tester Press/Move/Release via App.MouseCell; 10ms Tester time per OnUIThreadAndWait while held; drop-indicator line in the commits projection

Answer: Approved. Conditions: ClickAndHold/MouseMove/MouseRelease go only through d.point (App.MouseCell, last frame bounds) and real Tester Press/Move/Release, unreachable cells stay unsupported, the harness never sets drag state, insertion or scroll; Tester time advances 10ms per OnUIThreadAndWait only while held (lazygit assertEventually's poll), any other amount or advancing while not held needs a new consult, held cleared on release; dropHeader reads only a.drag.commit.ins, text Tr.MoveCommitsHere framed as lazygit's ━━━━━━ … ━━━━━━, the app draws the same line, ordering per getNonModelItems (commented), stable sort keeps existing header order; full run with no regressions before commit, the 9 drag tests in passing.txt with the harness commit, harness committed alone.

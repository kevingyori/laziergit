# Oracle consults

Daily budget: 10

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

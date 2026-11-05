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

# Review

Features the parity agent finished, newest last, with screenshots of the
lazygit tests they made pass (each test's last frame, rendered offscreen).
Look for anything that doesn't fit the app's design (AGENTS.md Principles)
or doesn't match lazygit.


## file-tree: Native file and commit-file trees
Commit cd5a304 (UI), ee2dce1 (projection) · +8 tests · 2026-11-05
What to look at: compressed directories, retained selection when switching tree/flat views, directory staging and discard, and renames across directories. Remaining failures move on to range selection, filtering, split diff panes, and other file actions.

![Collapse and expand directories](review/file-tree/file__collapse_expand.png)
![Commit directory renames](review/file-tree/commit__directory_diff_with_renamed_files.png)
![Discard only unstaged directory changes](review/file-tree/file__discard_unstaged_dir_changes.png)
![Renames without a root row](review/file-tree/file__renamed_files_no_root_item.png)

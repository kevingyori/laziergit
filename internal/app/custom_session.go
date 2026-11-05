package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/samber/lo"
)

func commitShimFromModelCommit(commit *models.Commit) *customCommit {
	if commit == nil {
		return nil
	}

	return &customCommit{
		Hash:          commit.Hash(),
		Sha:           commit.Hash(),
		Name:          commit.Name,
		Status:        commit.Status,
		Action:        commit.Action,
		Tags:          commit.Tags,
		ExtraInfo:     commit.ExtraInfo,
		AuthorName:    commit.AuthorName,
		AuthorEmail:   commit.AuthorEmail,
		UnixTimestamp: commit.UnixTimestamp,
		Divergence:    commit.Divergence,
		Parents:       commit.Parents(),
	}
}

func fileShimFromModelFile(file *models.File) *customFile {
	if file == nil {
		return nil
	}

	return &customFile{
		Name:                    file.Path,
		PreviousName:            file.PreviousPath,
		HasStagedChanges:        file.HasStagedChanges,
		HasUnstagedChanges:      file.HasUnstagedChanges,
		Tracked:                 file.Tracked,
		Added:                   file.Added,
		Deleted:                 file.Deleted,
		HasMergeConflicts:       file.HasMergeConflicts,
		HasInlineMergeConflicts: file.HasInlineMergeConflicts,
		DisplayString:           file.DisplayString,
		ShortStatus:             file.ShortStatus,
		IsWorktree:              file.IsWorktree,
	}
}

func submoduleShimFromModelSubmodule(submodule *models.SubmoduleConfig) *customSubmodule {
	if submodule == nil {
		return nil
	}

	return &customSubmodule{
		Name: submodule.Name,
		Path: submodule.Path,
		Url:  submodule.Url,
	}
}

func branchShimFromModelBranch(branch *models.Branch) *customBranch {
	if branch == nil {
		return nil
	}

	return &customBranch{
		Name:           branch.Name,
		DisplayName:    branch.DisplayName,
		Recency:        branch.Recency,
		Pushables:      branch.AheadForPull,
		Pullables:      branch.BehindForPull,
		AheadForPull:   branch.AheadForPull,
		BehindForPull:  branch.BehindForPull,
		AheadForPush:   branch.AheadForPush,
		BehindForPush:  branch.BehindForPush,
		UpstreamGone:   branch.UpstreamGone,
		Head:           branch.Head,
		DetachedHead:   branch.DetachedHead,
		UpstreamRemote: branch.UpstreamRemote,
		UpstreamBranch: branch.UpstreamBranch,
		Subject:        branch.Subject,
		CommitHash:     branch.CommitHash,
	}
}

func remoteBranchShimFromModelRemoteBranch(remoteBranch *models.RemoteBranch) *customRemoteBranch {
	if remoteBranch == nil {
		return nil
	}

	return &customRemoteBranch{
		Name:       remoteBranch.Name,
		RemoteName: remoteBranch.RemoteName,
	}
}

func remoteShimFromModelRemote(remote *models.Remote) *customRemote {
	if remote == nil {
		return nil
	}

	return &customRemote{
		Name:     remote.Name,
		Urls:     remote.Urls,
		PushUrls: remote.PushUrls,
		Branches: lo.Map(remote.Branches, func(branch *models.RemoteBranch, _ int) *customRemoteBranch {
			return remoteBranchShimFromModelRemoteBranch(branch)
		}),
	}
}

func tagShimFromModelRemote(tag *models.Tag) *customTag {
	if tag == nil {
		return nil
	}

	return &customTag{
		Name:    tag.Name,
		Message: tag.Message,
	}
}

func stashEntryShimFromModelRemote(stashEntry *models.StashEntry) *customStashEntry {
	if stashEntry == nil {
		return nil
	}

	return &customStashEntry{
		Index:   stashEntry.Index,
		Recency: stashEntry.Recency,
		Name:    stashEntry.Name,
	}
}

func commitFileShimFromModelRemote(commitFile *models.CommitFile) *customCommitFile {
	if commitFile == nil {
		return nil
	}

	return &customCommitFile{
		Name:         commitFile.Path,
		ChangeStatus: commitFile.ChangeStatus,
	}
}

func worktreeShimFromModelRemote(worktree *models.Worktree) *customWorktree {
	if worktree == nil {
		return nil
	}

	return &customWorktree{
		IsMain:        worktree.IsMain,
		IsCurrent:     worktree.IsCurrent,
		Path:          worktree.Path,
		IsPathMissing: worktree.IsPathMissing,
		GitDir:        worktree.GitDir,
		Branch:        worktree.Branch,
		Name:          worktree.Name,
	}
}

func (a *App) customSession() *customSessionState {
	s := &customSessionState{
		SelectedLocalBranch: branchShimFromModelBranch(a.selectedBranch()),
		SelectedStashEntry:  stashEntryShimFromModelRemote(a.selectedStash()),
		SelectedFile:        fileShimFromModelFile(a.selectedFile()),
		SelectedSubmodule:   submoduleShimFromModelSubmodule(a.selectedSubmodule()),
		SelectedWorktree:    worktreeShimFromModelRemote(a.selectedWorktree()),
		SelectedCommit:      commitShimFromModelCommit(a.selectedCommit()),
	}
	if a.snap == nil {
		return s
	}
	s.CheckedOutBranch = branchShimFromModelBranch(a.snap.Head)
	fileIndex := func(tab int) int {
		if a.fileTab == tab {
			return a.cursor[panelFiles]
		}
		return a.tabCursor[0][tab]
	}
	branchIndex := func(tab int) int {
		if a.branchTab == tab && a.browse(panelBranches) == nil {
			return a.cursor[panelBranches]
		}
		return a.tabCursor[1][tab]
	}
	if i := fileIndex(0); i >= 0 && i < a.files.Len() {
		n := a.files.Get(i)
		s.SelectedFile = fileShimFromModelFile(n.File)
		s.SelectedPath = n.GetPath()
	}
	if i := fileIndex(1); i >= 0 && i < len(a.snap.Worktrees) {
		s.SelectedWorktree = worktreeShimFromModelRemote(a.snap.Worktrees[i])
	}
	if i := fileIndex(2); i >= 0 && i < len(a.snap.Submodules) {
		s.SelectedSubmodule = submoduleShimFromModelSubmodule(a.snap.Submodules[i])
	}
	if i := branchIndex(0); i >= 0 && i < len(a.snap.Branches) {
		s.SelectedLocalBranch = branchShimFromModelBranch(a.snap.Branches[i])
	}
	if i := branchIndex(1); i >= 0 && i < len(a.snap.Remotes) {
		s.SelectedRemote = remoteShimFromModelRemote(a.snap.Remotes[i])
	}
	if i := branchIndex(2); i >= 0 && i < len(a.snap.Tags) {
		s.SelectedTag = tagShimFromModelRemote(a.snap.Tags[i])
	}
	if n := a.selectedFileNode(); n != nil {
		s.SelectedPath = n.GetPath()
	}
	if n := a.selectedCommitFileNode(); n != nil {
		s.SelectedCommitFilePath = n.GetPath()
		s.SelectedCommitFile = commitFileShimFromModelRemote(n.File)
		s.SelectedPath = s.SelectedCommitFilePath
	}
	// SessionStateLoader: each commit list keeps its selection, and
	// SelectedCommit is the one of the current context or its parent.
	single := func(c *models.Commit) *customCommitRange {
		if c == nil {
			return nil
		}
		return &customCommitRange{From: c.Hash(), To: c.Hash()}
	}
	listRange := func(list []*models.Commit, p panel, shown bool, c *models.Commit) *customCommitRange {
		if !shown {
			return single(c)
		}
		start, end := a.selectionRange(p)
		if start < 0 || end >= len(list) {
			return single(c)
		}
		return &customCommitRange{From: list[end].Hash(), To: list[start].Hash()}
	}
	var local, reflog, sub *models.Commit
	if i := a.localCommitIndex(); i >= 0 && i < len(a.snap.Commits) {
		local = a.snap.Commits[i]
	}
	reflog = a.selectedReflogCommit()
	var subList []*models.Commit
	subShown := false
	if b := a.refBrowse; b != nil && b.ref != nil {
		subList = b.commits
		i := a.cursor[b.panel]
		subShown = b.panel == panelBranches || a.commitFiles == nil
		if cf := a.commitFiles; cf != nil && cf.fromSub && b.panel == panelCommits {
			i = cf.commitIdx
		}
		if i >= 0 && i < len(b.commits) {
			sub = b.commits[i]
		}
	}
	s.SelectedLocalCommit = commitShimFromModelCommit(local)
	s.SelectedReflogCommit = commitShimFromModelCommit(reflog)
	s.SelectedSubCommit = commitShimFromModelCommit(sub)
	s.SelectedCommit = s.SelectedLocalCommit
	s.SelectedCommitRange = listRange(a.snap.Commits, panelCommits, a.commitsView() == cvCommits, local)
	switch {
	case a.focus == panelCommits && a.commitFiles != nil && a.commitFiles.fromSub,
		a.focus == panelBranches && a.subCommits() != nil:
		s.SelectedCommit = s.SelectedSubCommit
		s.SelectedCommitRange = listRange(subList, a.refBrowse.panel, subShown, sub)
	case a.focus == panelCommits && (a.commitsView() == cvReflog || a.commitsView() == cvSubCommits):
		s.SelectedCommit = s.SelectedReflogCommit
		s.SelectedCommitRange = listRange(a.snap.Reflog, panelCommits, a.commitsView() == cvReflog, reflog)
	}
	idx := a.cursor[panelBranches]
	if a.browse(panelBranches) != nil && a.browse(panelBranches).ref == nil && idx < len(a.browse(panelBranches).branches) {
		s.SelectedRemoteBranch = remoteBranchShimFromModelRemoteBranch(a.browse(panelBranches).branches[idx])
	} else if a.browse(panelBranches) == nil {
		if a.branchTab == 1 && idx < len(a.snap.Remotes) {
			s.SelectedRemote = remoteShimFromModelRemote(a.snap.Remotes[idx])
		}
		if a.branchTab == 2 && idx < len(a.snap.Tags) {
			s.SelectedTag = tagShimFromModelRemote(a.snap.Tags[idx])
		}
	}
	return s
}

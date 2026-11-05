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
		if a.branchTab == tab && a.refBrowse == nil {
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
	if i := a.cursor[panelCommits]; a.commitFiles == nil && i >= 0 && i < len(a.snap.Commits) {
		s.SelectedLocalCommit = commitShimFromModelCommit(a.snap.Commits[i])
	}

	if n := a.selectedFileNode(); n != nil {
		s.SelectedPath = n.GetPath()
	}
	if n := a.selectedCommitFileNode(); n != nil {
		s.SelectedCommitFilePath = n.GetPath()
		s.SelectedCommitFile = commitFileShimFromModelRemote(n.File)
		s.SelectedPath = s.SelectedCommitFilePath
	}
	commits, pn := a.snap.Commits, panelCommits
	if a.refBrowse != nil && a.refBrowse.ref != nil && a.focus == panelBranches {
		s.SelectedSubCommit = s.SelectedCommit
		commits, pn = a.refBrowse.commits, panelBranches
	} else {
		s.SelectedLocalCommit = s.SelectedCommit
	}
	if a.commitFiles == nil && len(commits) > 0 {
		start, end := a.selectionRange(pn)
		if start >= 0 && end < len(commits) {
			s.SelectedCommitRange = &customCommitRange{From: commits[end].Hash(), To: commits[start].Hash()}
		}
	} else if s.SelectedCommit != nil {
		s.SelectedCommitRange = &customCommitRange{From: s.SelectedCommit.Hash, To: s.SelectedCommit.Hash}
	}
	idx := a.cursor[panelBranches]
	if a.refBrowse != nil && a.refBrowse.ref == nil && idx < len(a.refBrowse.branches) {
		s.SelectedRemoteBranch = remoteBranchShimFromModelRemoteBranch(a.refBrowse.branches[idx])
	} else if a.refBrowse == nil {
		if a.branchTab == 1 && idx < len(a.snap.Remotes) {
			s.SelectedRemote = remoteShimFromModelRemote(a.snap.Remotes[idx])
		}
		if a.branchTab == 2 && idx < len(a.snap.Tags) {
			s.SelectedTag = tagShimFromModelRemote(a.snap.Tags[idx])
		}
	}
	return s
}

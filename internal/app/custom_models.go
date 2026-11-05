package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/stefanhaller/git-todo-parser/todo"
)

// Copied from lazygit v0.66.2's custom_commands/models.go and session_state_loader.go.
// Keep these template shims aligned with upstream without importing its GUI
// controllers and their terminal/window dependencies.
type customCommit struct {
	Hash          string
	Sha           string // deprecated: use Hash
	Name          string
	Status        models.CommitStatus
	Action        todo.TodoCommand
	Tags          []string
	ExtraInfo     string
	AuthorName    string
	AuthorEmail   string
	UnixTimestamp int64
	Divergence    models.Divergence
	Parents       []string
}

type customFile struct {
	Name                    string
	PreviousName            string
	HasStagedChanges        bool
	HasUnstagedChanges      bool
	Tracked                 bool
	Added                   bool
	Deleted                 bool
	HasMergeConflicts       bool
	HasInlineMergeConflicts bool
	DisplayString           string
	ShortStatus             string
	IsWorktree              bool
}

type customSubmodule struct {
	Name string
	Path string
	Url  string
}

type customBranch struct {
	Name           string
	DisplayName    string
	Recency        string
	Pushables      string // deprecated: use AheadForPull
	Pullables      string // deprecated: use BehindForPull
	AheadForPull   string
	BehindForPull  string
	AheadForPush   string
	BehindForPush  string
	UpstreamGone   bool
	Head           bool
	DetachedHead   bool
	UpstreamRemote string
	UpstreamBranch string
	Subject        string
	CommitHash     string
}

type customRemoteBranch struct {
	Name       string
	RemoteName string
}

type customRemote struct {
	Name     string
	Urls     []string
	PushUrls []string
	Branches []*customRemoteBranch
}

type customTag struct {
	Name    string
	Message string
}

type customStashEntry struct {
	Index   int
	Recency string
	Name    string
}

type customCommitFile struct {
	Name         string
	ChangeStatus string
}

type customWorktree struct {
	IsMain        bool
	IsCurrent     bool
	Path          string
	IsPathMissing bool
	GitDir        string
	Branch        string
	Name          string
}

type customCommitRange struct {
	From string
	To   string
}

type customSessionState struct {
	SelectedLocalCommit    *customCommit // deprecated, use SelectedCommit
	SelectedReflogCommit   *customCommit // deprecated, use SelectedCommit
	SelectedSubCommit      *customCommit // deprecated, use SelectedCommit
	SelectedCommit         *customCommit
	SelectedCommitRange    *customCommitRange
	SelectedFile           *customFile
	SelectedSubmodule      *customSubmodule
	SelectedPath           string
	SelectedLocalBranch    *customBranch
	SelectedRemoteBranch   *customRemoteBranch
	SelectedRemote         *customRemote
	SelectedTag            *customTag
	SelectedStashEntry     *customStashEntry
	SelectedCommitFile     *customCommitFile
	SelectedCommitFilePath string
	SelectedWorktree       *customWorktree
	CheckedOutBranch       *customBranch
}

package repo

import (
	"path/filepath"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

// PartKind identifies one independently loaded piece of a snapshot. Every
// load emits each kind exactly once, then DonePart.
type PartKind uint32

const (
	StatePart      PartKind = 1 << iota // repo root and the operation in progress
	HeadPart                            // the checked-out branch's name, long before the branch list
	FilesPart                           // working-tree files
	StashPart                           // stash entries
	ReflogPart                          // reflog commits
	BranchesPart                        // local branches, in reflog recency order
	DivergencePart                      // the branches' behind-base counts (gui.showDivergenceFromBaseBranch)
	CommitsPart                         // the commit list, its graph and pushed status
	RemotesPart                         // remotes and their branches
	TagsPart                            // tags
	WorktreesPart                       // linked worktrees
	SubmodulesPart                      // submodule configs
	DonePart                            // the load finished; fills nothing

	// AllParts is every kind a load fills.
	AllParts = DonePart - 1
)

// partSpec is what one kind fills: the snapshot fields it owns, copied by
// merge from the part's own snapshot.
type partSpec struct {
	kind  PartKind
	name  string
	merge func(dst, src *Snapshot)
}

// partSpecs is the registry of parts, in no particular order: a part's
// arrival replaces exactly its own fields.
var partSpecs = []partSpec{
	{StatePart, "state", func(s, n *Snapshot) {
		s.Root = n.Root
		s.RebaseUpdateRefs, s.OperationCommand, s.OperationTitle = n.RebaseUpdateRefs, n.OperationCommand, n.OperationTitle
		s.StatusTitle, s.StatusLowerTitle, s.OptionsMapTitle = n.StatusTitle, n.StatusLowerTitle, n.OptionsMapTitle
		s.CanSkip, s.Operation = n.CanSkip, n.Operation
	}},
	{HeadPart, "head", func(s, n *Snapshot) { s.CurrentBranch = n.CurrentBranch }},
	{FilesPart, "files", func(s, n *Snapshot) { s.Files = n.Files }},
	{StashPart, "stash", func(s, n *Snapshot) { s.Stash = n.Stash }},
	{ReflogPart, "reflog", func(s, n *Snapshot) { s.Reflog = n.Reflog }},
	{BranchesPart, "branches", func(s, n *Snapshot) { s.Branches = n.Branches }},
	// The counts live in the branches' atomic fields, set after the
	// BranchesPart of the same load was published.
	{DivergencePart, "divergence", func(s, n *Snapshot) {}},
	{CommitsPart, "commits", func(s, n *Snapshot) {
		s.Commits, s.Graph = n.Commits, n.Graph
		s.CheckedOutBranch, s.BisectInfo = n.CheckedOutBranch, n.BisectInfo
	}},
	{RemotesPart, "remotes", func(s, n *Snapshot) { s.Remotes = n.Remotes }},
	{TagsPart, "tags", func(s, n *Snapshot) { s.Tags = n.Tags }},
	{WorktreesPart, "worktrees", func(s, n *Snapshot) { s.Worktrees = n.Worktrees }},
	{SubmodulesPart, "submodules", func(s, n *Snapshot) { s.Submodules = n.Submodules }},
	{DonePart, "done", func(s, n *Snapshot) {}},
}

// derivation is a field computed from several parts. It reruns whenever one
// of its inputs arrives, with whatever the other inputs hold so far.
type derivation struct {
	inputs PartKind
	derive func(*Snapshot)
}

var derivations = []derivation{
	// Files show which of them are linked worktrees (paths below the root).
	{StatePart | FilesPart | WorktreesPart, (*Snapshot).markWorktreeFiles},
	// The status line's branch: the branch list's head once it is loaded,
	// the bare name before.
	{HeadPart | BranchesPart, (*Snapshot).deriveHead},
}

func (k PartKind) String() string {
	for _, p := range partSpecs {
		if p.kind == k {
			return p.name
		}
	}
	return "parts"
}

// Part transfers a completed scope. Its slice contents are immutable after
// publication (branch divergence uses lazygit's atomic fields).
type Part struct {
	Kind     PartKind
	Snapshot *Snapshot
}

// Merge replaces only this part's fields, retaining parts that have not
// refreshed yet, records it as loaded, and rederives what depends on it.
func (p Part) Merge(s *Snapshot) {
	n := p.Snapshot
	if n == nil {
		return
	}
	for _, spec := range partSpecs {
		if spec.kind == p.Kind {
			spec.merge(s, n)
		}
	}
	s.Loaded |= p.Kind
	for _, d := range derivations {
		if d.inputs&p.Kind != 0 {
			d.derive(s)
		}
	}
	s.Errors = append(s.Errors, n.Errors...)
}

// Has reports whether every part in kinds has arrived at least once.
func (s *Snapshot) Has(kinds PartKind) bool { return s != nil && s.Loaded&kinds == kinds }

// markWorktreeFiles derives file flags without mutating published loader data.
func (s *Snapshot) markWorktreeFiles() {
	files := make([]*models.File, len(s.Files))
	for i, f := range s.Files {
		copy := *f
		copy.IsWorktree = false
		for _, w := range s.Worktrees {
			if w.Path == filepath.Join(s.Root, f.Path) {
				copy.IsWorktree = true
			}
		}
		files[i] = &copy
	}
	s.Files = files
}

func (s *Snapshot) deriveHead() {
	s.Head = s.CurrentBranch
	if s.Loaded&BranchesPart != 0 {
		s.Head = nil
		for _, b := range s.Branches {
			if b.Head {
				s.Head = b
			}
		}
	}
}

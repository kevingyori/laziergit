// Package graph computes commit-graph lanes. The layout algorithm is
// lazygit's (pkg/gui/presentation/graph, MIT, © Jesse Duffield), vendored
// because its Pipe fields are unexported; rendering to characters is
// replaced by exported geometry the UI draws as vector paths.
package graph

import (
	"cmp"
	"hash/fnv"
	"slices"

	"github.com/jesseduffield/generics/set"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

type Kind uint8

const (
	Terminates Kind = iota // comes from above, ends at this row's commit
	Starts                 // leaves this row's commit, goes down
	Continues              // passes through the row
)

// Pipe is one line segment in a row: From is its lane at the row's top (or
// the commit, for Starts), To its lane at the row's bottom (or the commit,
// for Terminates).
type Pipe struct {
	From, To int16
	Kind     Kind
	fromHash *string
	toHash   *string
	Color    uint32 // stable per pipe origin; index into a palette
}

// Row is the graph for one commit row.
type Row struct {
	Pipes  []Pipe
	Commit int16 // lane of the commit dot
	Width  int16 // lanes used
	Merge  bool
}

var (
	emptyTree = models.EmptyTreeCommitHash
	start     = "START"
)

// Rows computes the graph rows for commits (newest first, with hashes from
// one StringPool, as lazygit's loaders produce).
func Rows(commits []*models.Commit) []Row {
	if len(commits) == 0 {
		return nil
	}
	pipes := []Pipe{{From: 0, To: 0, fromHash: &start, toHash: commits[0].HashPtr(), Kind: Starts}}
	rows := make([]Row, len(commits))
	for i, c := range commits {
		pipes = nextPipes(pipes, c)
		r := Row{Pipes: pipes}
		starts := 0
		for _, p := range pipes {
			switch p.Kind {
			case Starts:
				starts++
				r.Commit = p.From
			case Terminates:
				r.Commit = p.To
			}
			r.Width = max(r.Width, p.From+1, p.To+1)
		}
		r.Merge = starts > 1
		rows[i] = r
	}
	return rows
}

func colorOf(h *string) uint32 {
	if h == nil {
		return 0
	}
	f := fnv.New32a()
	f.Write([]byte(*h))
	return f.Sum32()
}

func eq(a, b *string) bool { return a != nil && b != nil && a == b }

func nextPipes(prev []Pipe, commit *models.Commit) []Pipe {
	current := make([]Pipe, 0, len(prev))
	for _, p := range prev {
		if p.Kind != Terminates && !eq(p.toHash, &emptyTree) && *p.toHash != emptyTree {
			current = append(current, p)
		}
	}
	maxPos := int16(-1)
	for _, p := range current {
		maxPos = max(maxPos, p.To)
	}
	out := make([]Pipe, 0, len(current)+len(commit.ParentPtrs()))
	pos := maxPos + 1
	for _, p := range current {
		if eq(p.toHash, commit.HashPtr()) {
			pos = p.To
			break
		}
	}
	taken, traversed := set.New[int](), set.New[int]()

	toHash := &emptyTree
	if !commit.IsFirstCommit() {
		toHash = commit.ParentPtrs()[0]
	}
	// The first-parent line keeps the colour of the line that reached this
	// commit, so a branch reads as one colour top to bottom.
	color := colorOf(commit.HashPtr())
	for _, p := range current {
		if eq(p.toHash, commit.HashPtr()) {
			color = p.Color
			break
		}
	}
	out = append(out, Pipe{From: pos, To: pos, fromHash: commit.HashPtr(), toHash: toHash, Kind: Starts, Color: color})

	traversedContinuing := set.New[int]()
	for _, p := range current {
		if !eq(p.toHash, commit.HashPtr()) {
			traversedContinuing.Add(int(p.To))
		}
	}
	nextContinuing := func() int16 {
		for i := int16(0); ; i++ {
			if !traversed.Includes(int(i)) {
				return i
			}
		}
	}
	nextNew := func() int16 {
		for i := int16(0); ; i++ {
			if !taken.Includes(int(i)) && !traversedContinuing.Includes(int(i)) {
				return i
			}
		}
	}
	traverse := func(from, to int16) {
		l, r := min(from, to), max(from, to)
		for i := l; i <= r; i++ {
			traversed.Add(int(i))
		}
		taken.Add(int(to))
	}

	for _, p := range current {
		if eq(p.toHash, commit.HashPtr()) {
			out = append(out, Pipe{From: p.To, To: pos, fromHash: p.fromHash, toHash: p.toHash, Kind: Terminates, Color: p.Color})
			traverse(p.To, pos)
		} else if p.To < pos {
			at := nextContinuing()
			out = append(out, Pipe{From: p.To, To: at, fromHash: p.fromHash, toHash: p.toHash, Kind: Continues, Color: p.Color})
			traverse(p.To, at)
		}
	}
	if commit.IsMerge() {
		for _, parent := range commit.ParentPtrs()[1:] {
			at := nextNew()
			out = append(out, Pipe{From: pos, To: at, fromHash: commit.HashPtr(), toHash: parent, Kind: Starts, Color: colorOf(parent)})
			taken.Add(int(at))
		}
	}
	for _, p := range current {
		if !eq(p.toHash, commit.HashPtr()) && p.To > pos {
			last := p.To
			for i := p.To; i > pos; i-- {
				if taken.Includes(int(i)) || traversed.Includes(int(i)) {
					break
				}
				last = i
			}
			out = append(out, Pipe{From: p.To, To: last, fromHash: p.fromHash, toHash: p.toHash, Kind: Continues, Color: p.Color})
			traverse(p.To, last)
		}
	}
	slices.SortFunc(out, func(a, b Pipe) int {
		if a.To == b.To {
			return cmp.Compare(a.Kind, b.Kind)
		}
		return cmp.Compare(a.To, b.To)
	})
	return out
}

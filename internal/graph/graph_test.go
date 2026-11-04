package graph

import (
	"testing"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

func commits(spec ...[]string) []*models.Commit {
	pool := &utils.StringPool{}
	var out []*models.Commit
	for _, s := range spec {
		out = append(out, models.NewCommit(pool, models.NewCommitOpts{Hash: s[0], Parents: s[1:]}))
	}
	return out
}

func TestLinear(t *testing.T) {
	rows := Rows(commits([]string{"c", "b"}, []string{"b", "a"}, []string{"a"}))
	for i, r := range rows {
		if r.Commit != 0 || r.Width != 1 || r.Merge {
			t.Errorf("row %d: %+v", i, r)
		}
	}
}

func TestMerge(t *testing.T) {
	// m merges b2 into a2; both branch from a1.
	rows := Rows(commits(
		[]string{"m", "a2", "b2"},
		[]string{"b2", "a1"},
		[]string{"a2", "a1"},
		[]string{"a1"},
	))
	if !rows[0].Merge || rows[0].Width != 2 {
		t.Errorf("merge row: %+v", rows[0])
	}
	if rows[1].Commit != 1 {
		t.Errorf("b2 should sit in lane 1: %+v", rows[1])
	}
	if rows[2].Commit != 0 || rows[3].Commit != 0 {
		t.Errorf("first-parent line should stay in lane 0: %+v %+v", rows[2], rows[3])
	}
}

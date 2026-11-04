package repo

import (
	"fmt"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"regexp"
	"strings"
)

type UndoPlan struct{ Head, Target, Kind, Description string }

// Follow lazygit's reflog grammar, grouping rebase steps into one operation
// and skipping actions already undone. Keep git's reflog as the durable history.
func (r *Repo) PlanUndo() (*UndoPlan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.planUndo()
}
func (r *Repo) planUndo() (*UndoPlan, error) {
	if r.Git.Status.WorkingTreeState().Any() {
		return nil, fmt.Errorf("finish or abort the current git operation before undoing")
	}
	out, err := r.OS.Cmd.New(git_commands.NewGitCmd("reflog").Arg("--format=%H%x00%gs").ToArgv()).DontLog().RunWithOutput()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	type entry struct{ hash, msg string }
	var entries []entry
	for _, line := range lines {
		h, msg, ok := strings.Cut(line, "\x00")
		if ok {
			entries = append(entries, entry{h, msg})
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no undo history")
	}
	checkout := regexp.MustCompile(`^checkout: moving from (\S+) to (\S+)`)
	rebaseEnd := regexp.MustCompile(`^rebase (-i )?\((finish|abort)\)`)
	rebaseStart := regexp.MustCompile(`^rebase (-i )?\(start\)`)
	skip := 0
	end := ""
	for i, e := range entries {
		if i+1 >= len(entries) {
			break
		}
		prev := entries[i+1].hash
		p := &UndoPlan{Head: entries[0].hash, Target: prev, Description: e.msg}
		if end != "" {
			if !rebaseStart.MatchString(e.msg) {
				continue
			}
			end = ""
			p.Kind = "rebase"
		} else {
			switch {
			case strings.HasPrefix(e.msg, "[laziergit undo]"), strings.HasPrefix(e.msg, "[lazygit undo]"):
				skip++
				continue
			case strings.HasPrefix(e.msg, "[lazygit redo]"):
				skip--
				continue
			case rebaseEnd.MatchString(e.msg):
				end = e.hash
				continue
			case checkout.MatchString(e.msg):
				p.Kind = "checkout"
				p.Target = checkout.FindStringSubmatch(e.msg)[1]
			case strings.HasPrefix(e.msg, "commit"), strings.HasPrefix(e.msg, "reset: moving to"), strings.HasPrefix(e.msg, "pull"), strings.HasPrefix(e.msg, "cherry-pick"):
				p.Kind = "commit"
			default:
				continue
			}
		}
		if p.Target == e.hash && p.Kind != "checkout" {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		return p, nil
	}
	return nil, fmt.Errorf("no earlier supported action to undo")
}
func (r *Repo) Undo(p *UndoPlan) error {
	return r.locked(func() error {
		current, err := r.planUndo()
		if err != nil {
			return err
		}
		if *current != *p {
			return fmt.Errorf("history changed; open undo again")
		}
		cmd := git_commands.NewGitCmd("reset").Arg("--soft", p.Target)
		if p.Kind == "rebase" {
			cmd = git_commands.NewGitCmd("reset").Arg("--keep", p.Target)
		}
		if p.Kind == "checkout" {
			cmd = git_commands.NewGitCmd("checkout").Arg(p.Target)
		}
		return r.OS.Cmd.New(cmd.ToArgv()).AddEnvVars("GIT_REFLOG_ACTION=[laziergit undo]").Run()
	})
}

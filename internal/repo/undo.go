package repo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// UndoPlan is one of lazygit's reflogActions chosen for undo or redo.
// Kind is "commit", "rebase" or "checkout"; From/To are hashes or refs.
type UndoPlan struct{ Kind, From, To string }

type reflogEntry struct{ hash, msg string }

var (
	reCheckout    = regexp.MustCompile(`^checkout: moving from ([\S]+) to ([\S]+)`)
	reCommit      = regexp.MustCompile(`^commit|^reset: moving to|^pull`)
	reRebaseEnd   = regexp.MustCompile(`^rebase (-i )?\(abort\)|^rebase (-i )?\(finish\)`)
	reRebaseStart = regexp.MustCompile(`^rebase (-i )?\(start\)`)
)

func (r *Repo) reflogEntries() ([]reflogEntry, error) {
	out, err := r.OS.Cmd.New(git_commands.NewGitCmd("reflog").Arg("--format=%H%x00%gs").ToArgv()).DontLog().RunWithOutput()
	if err != nil {
		return nil, err
	}
	var entries []reflogEntry
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if h, msg, ok := strings.Cut(line, "\x00"); ok {
			entries = append(entries, reflogEntry{h, msg})
		}
	}
	return entries, nil
}

// parseReflogForActions is lazygit's UndoController.parseReflogForActions.
func parseReflogForActions(entries []reflogEntry, onUserAction func(counter int, p UndoPlan) bool) {
	counter := 0
	rebaseFinish := ""
	for i, e := range entries {
		var p *UndoPlan
		prev := ""
		if i+1 < len(entries) {
			prev = entries[i+1].hash
		}
		if rebaseFinish == "" {
			switch {
			case strings.HasPrefix(e.msg, "[lazygit undo]"):
				counter++
			case strings.HasPrefix(e.msg, "[lazygit redo]"):
				counter--
			case reRebaseEnd.MatchString(e.msg):
				rebaseFinish = e.hash
			case reCheckout.MatchString(e.msg):
				m := reCheckout.FindStringSubmatch(e.msg)
				p = &UndoPlan{Kind: "checkout", From: m[1], To: m[2]}
			case reCommit.MatchString(e.msg):
				p = &UndoPlan{Kind: "commit", From: prev, To: e.hash}
			case reRebaseStart.MatchString(e.msg):
				p = &UndoPlan{Kind: "current-rebase", From: prev}
			}
		} else if reRebaseStart.MatchString(e.msg) {
			p = &UndoPlan{Kind: "rebase", From: prev, To: rebaseFinish}
			rebaseFinish = ""
		}
		if p == nil {
			continue
		}
		if p.Kind != "current-rebase" && p.From == p.To {
			continue
		}
		if onUserAction(counter, *p) {
			return
		}
		counter--
	}
}

// PlanUndo returns the action lazygit's undo would reverse, or nil.
func (r *Repo) PlanUndo() (*UndoPlan, error) { return r.plan(false) }

// PlanRedo returns the action lazygit's redo would reapply, or nil.
func (r *Repo) PlanRedo() (*UndoPlan, error) { return r.plan(true) }

func (r *Repo) plan(redo bool) (*UndoPlan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tr := r.Common.Tr
	if r.Git.Status.WorkingTreeState().Any() {
		if redo {
			return nil, errors.New(tr.CantRedoWhileRebasing)
		}
		return nil, errors.New(tr.CantUndoWhileRebasing)
	}
	entries, err := r.reflogEntries()
	if err != nil {
		return nil, err
	}
	var found *UndoPlan
	parseReflogForActions(entries, func(counter int, p UndoPlan) bool {
		if redo {
			if counter == 0 {
				return true
			} else if counter > 1 {
				return false
			}
		} else if counter != 0 {
			return false
		}
		if p.Kind != "current-rebase" {
			found = &p
		}
		return true
	})
	return found, nil
}

// UndoReset is RefsHelper.ResetToRef with the undo/redo reflog action; a
// hard reset autostashes tracked changes like hardResetWithAutoStash.
func (r *Repo) UndoReset(hash, mode string, redo bool) error {
	return r.locked(func() error {
		env := undoEnv(redo)
		reset := func() error {
			return r.OS.Cmd.New(git_commands.NewGitCmd("reset").Arg("--"+mode, hash).ToArgv()).AddEnvVars(env).Run()
		}
		if mode != "hard" {
			return reset()
		}
		out, err := r.OS.Cmd.New(git_commands.NewGitCmd("status").Arg("--porcelain", "--untracked-files=no", "--ignore-submodules").ToArgv()).DontLog().RunWithOutput()
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) == "" {
			return reset()
		}
		if err := r.Git.Stash.Push(fmt.Sprintf(r.Common.Tr.AutoStashForUndo, utils.ShortHash(hash))); err != nil {
			return err
		}
		if err := reset(); err != nil {
			return err
		}
		return r.Git.Stash.Pop(0)
	})
}

// UndoCheckout checks out ref with the undo/redo reflog action.
func (r *Repo) UndoCheckout(ref string, redo bool) error {
	return r.locked(func() error {
		return r.Git.Branch.Checkout(ref, git_commands.CheckoutOptions{EnvVars: []string{undoEnv(redo)}})
	})
}

func undoEnv(redo bool) string {
	if redo {
		return "GIT_REFLOG_ACTION=[lazygit redo]"
	}
	return "GIT_REFLOG_ACTION=[lazygit undo]"
}

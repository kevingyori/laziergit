package spawn

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/common"
	"github.com/jesseduffield/lazygit/pkg/config"
)

// builders returns lazygit's own command builder and the same builder with
// a Runner, each with its own record of the command log.
func builders(t *testing.T) (lazygit, spawned *oscommands.CmdObjBuilder, logs func() (string, string)) {
	t.Helper()
	var mu sync.Mutex
	var logged [2]strings.Builder
	osCmd := func(i int) *oscommands.OSCommand {
		cmn := common.NewDummyCommon()
		guiIO := oscommands.NewGuiIO(cmn.Log, func(cmd string, commandLine bool) {
			mu.Lock()
			defer mu.Unlock()
			fmt.Fprintf(&logged[i], "%s %v\n", cmd, commandLine)
		}, func() io.Writer { return io.Discard }, nil)
		return oscommands.NewOSCommand(cmn, config.NewDummyAppConfig(), oscommands.GetPlatform(), guiIO)
	}
	lazygit = osCmd(0).Cmd
	o := osCmd(1)
	spawned = o.Cmd.CloneWithNewRunner(func(inner oscommands.ICmdObjRunner) oscommands.ICmdObjRunner {
		return NewRunner(inner, o.Log, func(cmd string, commandLine bool) { o.LogCommand(cmd, commandLine) })
	})
	return lazygit, spawned, func() (string, string) {
		mu.Lock()
		defer mu.Unlock()
		return logged[0].String(), logged[1].String()
	}
}

func errText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T %s", err, err)
}

func TestRunnerMatchesLazygit(t *testing.T) {
	type build func(b *oscommands.CmdObjBuilder) *oscommands.CmdObj
	cmds := map[string]build{
		"ok": func(b *oscommands.CmdObjBuilder) *oscommands.CmdObj {
			return b.New([]string{"/bin/sh", "-c", "echo out; echo err >&2"})
		},
		"fails with output": func(b *oscommands.CmdObjBuilder) *oscommands.CmdObj {
			return b.New([]string{"/bin/sh", "-c", "echo out; echo bad >&2; exit 2"})
		},
		"fails silently": func(b *oscommands.CmdObjBuilder) *oscommands.CmdObj {
			return b.New([]string{"/bin/sh", "-c", "exit 1"}).DontLog()
		},
		"git error": func(b *oscommands.CmdObjBuilder) *oscommands.CmdObj {
			return b.New([]string{"git", "rev-parse", "--verify", "no-such-ref"}).SetWd("/")
		},
		"shell with stdin": func(b *oscommands.CmdObjBuilder) *oscommands.CmdObj {
			return b.NewShell("tr a-z A-Z", "").SetStdin("hello\n")
		},
		"missing binary": func(b *oscommands.CmdObjBuilder) *oscommands.CmdObj {
			return b.New([]string{"/nonexistent"})
		},
		"not found on PATH": func(b *oscommands.CmdObjBuilder) *oscommands.CmdObj {
			return b.New([]string{"no-such-binary-anywhere"})
		},
	}
	methods := map[string]func(*oscommands.CmdObj) string{
		"Run": func(c *oscommands.CmdObj) string { return errText(c.Run()) },
		"RunWithOutput": func(c *oscommands.CmdObj) string {
			out, err := c.RunWithOutput()
			return out + "|" + errText(err)
		},
		"RunWithOutputs": func(c *oscommands.CmdObj) string {
			out, errOut, err := c.RunWithOutputs()
			return out + "|" + errOut + "|" + errText(err)
		},
		"RunAndProcessLines": func(c *oscommands.CmdObj) string {
			var lines []string
			err := c.RunAndProcessLines(func(line string) (bool, error) {
				lines = append(lines, line)
				return false, nil
			})
			return strings.Join(lines, ",") + "|" + errText(err)
		},
	}
	for cname, build := range cmds {
		for mname, method := range methods {
			t.Run(cname+"/"+mname, func(t *testing.T) {
				lazygit, spawned, logs := builders(t)
				before := started.Load()
				want, got := method(build(lazygit)), method(build(spawned))
				if got != want {
					t.Errorf("lazygit: %q\nspawn:   %q", want, got)
				}
				if l, s := logs(); l != s {
					t.Errorf("command log: lazygit %q, spawn %q", l, s)
				}
				// Commands that can't start never reach posix_spawn's success.
				wantRan := int64(1)
				if cname == "not found on PATH" || cname == "missing binary" {
					wantRan = 0
				}
				if ran := started.Load() - before; ran != wantRan {
					t.Errorf("spawned %d processes, want %d", ran, wantRan)
				}
			})
		}
	}
}

func TestRunnerStopsEarly(t *testing.T) {
	_, spawned, _ := builders(t)
	var n int
	err := spawned.New([]string{"/usr/bin/yes"}).RunAndProcessLines(func(string) (bool, error) {
		n++
		return n == 3, nil
	})
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	stop := fmt.Errorf("stop")
	err = spawned.New([]string{"/usr/bin/yes"}).RunAndProcessLines(func(string) (bool, error) { return false, stop })
	if err != stop {
		t.Fatalf("err=%v", err)
	}
}

func TestRunnerDelegates(t *testing.T) {
	_, spawned, _ := builders(t)
	cases := map[string]*oscommands.CmdObj{
		"streamed":       spawned.New([]string{"/bin/echo", "hi"}).StreamOutput(),
		"fail on prompt": spawned.New([]string{"/bin/echo", "hi"}).FailOnCredentialRequest(),
	}
	for name, c := range cases {
		before := started.Load()
		if err := c.Run(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if started.Load() != before {
			t.Errorf("%s went through posix_spawn", name)
		}
	}
}

func TestRunGitConfigCmd(t *testing.T) {
	cmd := func(args ...string) *exec.Cmd {
		c := exec.Command("git", args...)
		c.Env = []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=laziergit.test", "GIT_CONFIG_VALUE_0=yes"}
		return c
	}
	if out, err := RunGitConfigCmd(cmd("config", "--get", "--null", "laziergit.test")); out != "yes" || err != nil {
		t.Fatalf("got %q %v", out, err)
	}
	_, err := RunGitConfigCmd(cmd("config", "--get", "--null", "laziergit.missing"))
	if err == nil || !strings.HasPrefix(err.Error(), "the key is not found for [git config") {
		t.Fatalf("got %v", err)
	}
}

package spawn

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/sirupsen/logrus"
)

// Runner is lazygit's command runner (oscommands' cmdObjRunner) with
// posix_spawn underneath. Commands lazygit's runner would simply capture
// (no credential prompt, no output streamed to the command log, stdio not
// preset) are run here the way it runs them, logging included; everything
// else goes to lazygit's runner unchanged.
type Runner struct {
	inner oscommands.ICmdObjRunner
	log   *logrus.Entry
	// logCommand is the GuiIO's logCommandFn: the command log panel.
	logCommand func(cmd string, isCommandLine bool)
}

var _ oscommands.ICmdObjRunner = (*Runner)(nil)

// NewRunner wraps lazygit's runner; log and logCommand must be the ones that
// runner was built with.
func NewRunner(inner oscommands.ICmdObjRunner, log *logrus.Entry, logCommand func(string, bool)) *Runner {
	return &Runner{inner: inner, log: log, logCommand: logCommand}
}

func (r *Runner) spawnable(c *oscommands.CmdObj) bool {
	if c.GetCredentialStrategy() != oscommands.NONE || c.ShouldStreamOutput() {
		return false
	}
	cmd := c.GetCmd()
	return cmd.Stdout == nil && cmd.Stderr == nil && Supported(cmd)
}

func lock(c *oscommands.CmdObj) func() {
	m := c.Mutex()
	if m == nil {
		return func() {}
	}
	m.Lock()
	return m.Unlock
}

func (r *Runner) Run(c *oscommands.CmdObj) error {
	if !r.spawnable(c) {
		return r.inner.Run(c)
	}
	defer lock(c)()
	_, err := r.runWithOutput(c)
	return err
}

func (r *Runner) RunWithOutput(c *oscommands.CmdObj) (string, error) {
	if !r.spawnable(c) {
		return r.inner.RunWithOutput(c)
	}
	defer lock(c)()
	return r.runWithOutput(c)
}

func (r *Runner) RunWithOutputs(c *oscommands.CmdObj) (string, string, error) {
	if !r.spawnable(c) {
		return r.inner.RunWithOutputs(c)
	}
	defer lock(c)()
	return r.runWithOutputs(c)
}

// runWithOutput is cmdObjRunner.RunWithOutputAux.
func (r *Runner) runWithOutput(c *oscommands.CmdObj) (string, error) {
	r.log.WithField("command", c.ToString()).Debug("RunCommand")
	if c.ShouldLog() {
		r.logCommand(c.ToString(), true)
	}

	t := time.Now()
	cmd := c.GetCmd()
	// cmd.CombinedOutput
	var b bytes.Buffer
	cmd.Stdout, cmd.Stderr = &b, &b
	err := Run(cmd)
	output, err := sanitisedCommandOutput(b.Bytes(), err)
	if err != nil {
		r.log.WithField("command", c.ToString()).Error(output)
	}

	wall := time.Since(t)
	if ps := cmd.ProcessState; ps != nil {
		r.log.Infof("%s (wall %s, cpu %s)", c.ToString(), wall, ps.UserTime()+ps.SystemTime())
	} else {
		r.log.Infof("%s (wall %s)", c.ToString(), wall)
	}
	return output, err
}

// runWithOutputs is cmdObjRunner.RunWithOutputsAux.
func (r *Runner) runWithOutputs(c *oscommands.CmdObj) (string, string, error) {
	r.log.WithField("command", c.ToString()).Debug("RunCommand")
	if c.ShouldLog() {
		r.logCommand(c.ToString(), true)
	}

	t := time.Now()
	var outBuffer, errBuffer bytes.Buffer
	cmd := c.GetCmd()
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer
	err := Run(cmd)

	r.log.Infof("%s (%s)", c.ToString(), time.Since(t))

	stdout := outBuffer.String()
	stderr, err := sanitisedCommandOutput(errBuffer.Bytes(), err)
	if err != nil {
		r.log.WithField("command", c.ToString()).Error(stderr)
	}
	return stdout, stderr, err
}

// RunAndProcessLines is cmdObjRunner.RunAndProcessLines: it feeds each line
// of stdout to onLine and, when onLine asks to stop, closes the pipe so the
// command dies of SIGPIPE on its next write.
func (r *Runner) RunAndProcessLines(c *oscommands.CmdObj, onLine func(line string) (bool, error)) error {
	if !r.spawnable(c) {
		return r.inner.RunAndProcessLines(c, onLine)
	}
	defer lock(c)()

	if c.ShouldLog() {
		r.logCommand(c.ToString(), true)
	}
	t := time.Now()

	cmd := c.GetCmd()
	// cmd.StdoutPipe
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdout = pw
	p, err := Start(cmd)
	pw.Close()
	if err != nil {
		pr.Close()
		return err
	}
	// lazygit returns without waiting on these paths; reap the child anyway.
	abandon := func(err error) error {
		pr.Close()
		go p.Wait()
		return err
	}

	scanner := bufio.NewScanner(pr)
	scanner.Split(utils.ScanLinesAndTruncateWhenLongerThanBuffer(bufio.MaxScanTokenSize))
	for scanner.Scan() {
		stop, err := onLine(scanner.Text())
		if err != nil {
			return abandon(err)
		}
		if stop {
			break
		}
	}
	if scanner.Err() != nil {
		return abandon(scanner.Err())
	}
	pr.Close()
	_ = p.Wait()

	r.log.Infof("%s (%s)", c.ToString(), time.Since(t))
	return nil
}

// sanitisedCommandOutput is oscommands' (cmd_obj_runner.go).
func sanitisedCommandOutput(output []byte, err error) (string, error) {
	outputString := string(output)
	if err != nil {
		// errors like 'exit status 1' are not very useful so we'll create an error
		// from the combined output
		if outputString == "" {
			return "", utils.WrapError(err)
		}
		return outputString, goerrors.New(outputString)
	}
	return outputString, nil
}

// RunGitConfigCmd is lazygit's runGitConfigCmd (git_config/get_key.go), the
// runner of its cached git config, on posix_spawn.
func RunGitConfigCmd(cmd *exec.Cmd) (string, error) {
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard

	var err error
	if Supported(cmd) {
		err = Run(cmd)
	} else {
		err = cmd.Run()
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok {
			if waitStatus.ExitStatus() == 1 {
				return "", fmt.Errorf("the key is not found for %s", cmd.Args)
			}
		}
		return "", err
	}

	return strings.TrimRight(stdout.String(), "\000"), nil
}

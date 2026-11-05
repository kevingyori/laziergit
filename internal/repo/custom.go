package repo

func (r *Repo) RunCustom(command string) (string, error) {
	return r.RunCustomOutput(command, "")
}

func (r *Repo) RunCustomOutput(command, output string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cmd := r.OS.Cmd.NewShell(command, r.Config.OS.ShellFunctionsFile)
	if output == "log" || output == "logWithPty" {
		cmd.StreamOutput()
	}
	if output == "logWithPty" {
		cmd.UsePty()
	}
	return cmd.RunWithOutput()
}

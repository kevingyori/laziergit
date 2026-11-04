package repo

func (r *Repo) RunCustom(command string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.OS.Cmd.NewShell(command, r.Config.OS.ShellFunctionsFile).RunWithOutput()
}

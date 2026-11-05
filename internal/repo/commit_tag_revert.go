package repo

func (r *Repo) CreateTag(name, ref, description string, force bool) error {
	return r.locked(func() error {
		if description != "" || r.Git.Config.GetGpgTagSign() {
			return r.Git.Tag.CreateAnnotatedObj(name, ref, description, force).Run()
		}
		return r.Git.Tag.CreateLightweightObj(name, ref, force).Run()
	})
}

func (r *Repo) RevertCommits(hashes []string, merge, dirty bool, stashMessage string) error {
	return r.locked(func() error {
		if dirty {
			if err := r.Git.Stash.Push(stashMessage); err != nil {
				return err
			}
		}
		if err := r.Git.Commit.Revert(hashes, merge); err != nil {
			return err
		}
		if dirty {
			return r.Git.Stash.Pop(0)
		}
		return nil
	})
}

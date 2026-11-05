package repo

// Locked runs fn while holding the repo's operation lock, like the mutating
// operations in ops.go.
func (r *Repo) Locked(fn func() error) error { return r.locked(fn) }

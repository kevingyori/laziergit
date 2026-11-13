//go:build !darwin

package spawn

import "errors"

func loadLibc() bool { return false }

func posixSpawn(spawnRequest) (int, error) { return 0, errors.ErrUnsupported }

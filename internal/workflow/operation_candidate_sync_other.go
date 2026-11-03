//go:build !windows

package workflow

import (
	"errors"
	"os"
)

func operationCandidateSyncDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

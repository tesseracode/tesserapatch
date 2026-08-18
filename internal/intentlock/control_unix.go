//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"os"
	"syscall"
)

func lockHeldDirectory(file *os.File) error {
	return flockThroughControl(file, syscall.LOCK_EX|syscall.LOCK_NB)
}

func unlockHeldDirectory(file *os.File) error {
	return flockThroughControl(file, syscall.LOCK_UN)
}

func flockThroughControl(file *os.File, operation int) error {
	raw, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var flockErr error
	if err := raw.Control(func(descriptor uintptr) {
		flockErr = syscall.Flock(int(descriptor), operation)
	}); err != nil {
		return err
	}
	return flockErr
}

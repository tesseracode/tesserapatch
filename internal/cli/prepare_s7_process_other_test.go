//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package cli

import (
	"os"
	"os/exec"
	"time"
)

func s7ConfigureObservedProcess(cmd *exec.Cmd) {
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return cmd.Process.Kill()
	}
}

func s7ObservedProcessAlive(int) bool {
	return false
}

func s7KillObservedPID(int) {}

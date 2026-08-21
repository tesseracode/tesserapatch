//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package cli

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func s7ConfigureObservedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}

func s7ObservedProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func s7KillObservedPID(pid int) {
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

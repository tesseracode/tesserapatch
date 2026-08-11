//go:build !linux && !darwin

// Fail-closed process-observer stub for every non-linux/darwin build
// target. The lock layer already refuses before any Dolt invocation is
// reachable on such a host (§7.2); this file exists so the symbol set
// is identical across build targets and the package compiles anywhere.

package rescap

import (
	"errors"
	"syscall"
)

// ObserverSupported reports whether this build target has a real
// waitid observer.
const ObserverSupported = false

var errObserverUnsupported = errors.New("waitid observer requires a linux or darwin host")

func observeLeaderEvent(pid int) error { return errObserverUnsupported }

func setProcessGroup(attr *syscall.SysProcAttr) {}

func signalProcessGroup(pgid int, sig syscall.Signal) error { return errObserverUnsupported }

func isNoChildError(err error) bool { return false }

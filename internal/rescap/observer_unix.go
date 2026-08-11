//go:build linux || darwin

// Non-reaping leader-event observer for
// PRD-feature-resource-claims-and-capture-adapters §6.4 / ADR-033 D5.
//
// The observer wraps the raw waitid(P_PID, pid, ..., WEXITED|WNOWAIT)
// syscall directly rather than calling cmd.Wait(). WNOWAIT leaves the
// leader in a waitable (zombie) state, so its PID — and, since
// Setpgid:true makes the PGID equal the leader's PID, the process
// group's own numeric identity — is not released back to the kernel's
// reuse pool merely because this observer fired. That is what keeps the
// SIGTERM -> grace -> SIGKILL window safe from PGID reuse.
//
// What a successful return *means* differs by platform and this
// implementation deliberately does not over-claim. On Linux only
// WEXITED is requested and waitid(2) returns only for requested event
// classes, so a return is exit-only. On Darwin the same call may return
// for a merely *stopped* child despite WEXITED being the only flag
// requested (issue #19314 — Go's own stdlib avoids waitid on Darwin
// entirely for this reason, see $(go env GOROOT)/src/os/wait_waitid.go,
// which is //go:build linux only). This project's stdlib-only rule
// forecloses that workaround, so the concept is relabelled instead: a
// successful return is a *leader event* / *cleanup trigger*, never
// "the leader has exited". Treating a Darwin stop as a fail-closed
// cleanup trigger is safe because the unconditional
// SIGTERM -> grace -> SIGKILL sequence is correct and effective whether
// the leader has exited or is merely stopped — SIGKILL terminates a
// stopped process too and cannot be caught, blocked, or ignored.
//
// No golang.org/x/sys dependency: syscall.Syscall6 only.

package rescap

import (
	"syscall"
	"unsafe"
)

// ObserverSupported reports whether this build target has a real
// waitid observer.
const ObserverSupported = true

// pIDType is waitid's P_PID selector.
const pIDType = 1

// siginfoSize is a siginfo_t-sized scratch buffer. The contents are
// deliberately discarded and never parsed: the observer only needs to
// know that the call returned, not what it returned.
const siginfoSize = 128

// observeLeaderEvent blocks until waitid reports an event for pid, or
// returns a terminal errno. EINTR is retried in a loop, mirroring Go's
// own linux-only ignoringEINTR wrapper around the same call.
//
// It never reaps: WNOWAIT is always set.
func observeLeaderEvent(pid int) error {
	var buf [siginfoSize]byte
	for {
		_, _, errno := syscall.Syscall6(
			syscall.SYS_WAITID,
			uintptr(pIDType),
			uintptr(pid),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(syscall.WEXITED|syscall.WNOWAIT),
			0,
			0,
		)
		if errno == 0 {
			return nil
		}
		if errno == syscall.EINTR {
			continue
		}
		return errno
	}
}

// setProcessGroup makes the spawned child the leader of a new process
// group whose PGID equals its own PID, distinct from tpatch's own
// group, so a -pgid signal can never reach tpatch itself.
func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.Setpgid = true
}

// signalProcessGroup sends sig to the whole process group led by pgid.
// nil, ESRCH and EPERM are tolerated: ESRCH means nothing in the group
// remains signalable, and Darwin can return EPERM when the group's sole
// remaining member is an unreaped zombie. Any other errno is reported.
func signalProcessGroup(pgid int, sig syscall.Signal) error {
	err := syscall.Kill(-pgid, sig)
	switch err {
	case nil, syscall.ESRCH, syscall.EPERM:
		return nil
	default:
		return err
	}
}

// isNoChildError reports whether a terminal observer errno is ECHILD —
// the case where the kernel no longer considers the PID a child of this
// process, so its numeric PID/PGID could already have been recycled and
// must never be signalled.
func isNoChildError(err error) bool { return err == syscall.ECHILD }

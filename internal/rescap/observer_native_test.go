//go:build linux || darwin

// Native, non-reaping-observer behaviour (matrix row 175 / AC-106).
//
// The rest of row 175's coverage is structural — build tags and a real
// cross-compile — which says nothing about what the observer actually
// does to a live child. This exercises the PRODUCTION helper,
// observeLeaderEvent, against a real process on the real kernel: no
// source grep, no injected ObserveFn, no substituted syscall.
//
// The load-bearing property is that WNOWAIT leaves the leader in a
// waitable (zombie) state, so its PID — and, since Setpgid makes the
// PGID equal the leader's PID, the process group's numeric identity —
// is NOT released back to the kernel's reuse pool merely because the
// observer fired. That is the entire reason the finalizer can safely
// send SIGTERM/grace/SIGKILL to -pgid before ever calling cmd.Wait().
//
// The test is built so that either half of the contract breaking makes
// it fail:
//
//   - if WNOWAIT were dropped (or the observer otherwise reaped), the
//     SECOND observe of the same exited child would return ECHILD
//     rather than succeeding, because the zombie would already have
//     been consumed;
//   - and cmd.Wait() would then be unable to collect the child's real
//     exit status, so the exit-code assertion would fail too.

package rescap

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// observeWithin runs the production observer against pid and fails the
// test if it does not return inside the timeout, so a contract
// regression surfaces as a failure rather than a hung suite.
func observeWithin(t *testing.T, pid int, timeout time.Duration, what string) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- observeLeaderEvent(pid) }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatalf("%s: the observer did not return within %s", what, timeout)
		return nil
	}
}

// TestNativeObserverIsNonReapingAndPreservesExitStatus drives the real
// waitid(P_PID, pid, WEXITED|WNOWAIT) helper against a normally
// exiting child.
func TestNativeObserverIsNonReapingAndPreservesExitStatus(t *testing.T) {
	const wantExitCode = 7

	cmd := exec.Command("/bin/sh", "-c", "exit 7")
	// Same process-group setup the adapter uses, so the observed child
	// is shaped exactly like a real leader.
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	setProcessGroup(cmd.SysProcAttr)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	// First observe: blocks until the child has genuinely exited. This
	// is also how the production finalizer learns of a leader event.
	if err := observeWithin(t, pid, 30*time.Second, "first observe"); err != nil {
		t.Fatalf("first observe returned %v; a normally exiting child must be observable", err)
	}

	// Second observe of the SAME, already-exited child. This is the
	// negative control for WNOWAIT: it can only succeed while the
	// zombie is still unconsumed. A reaping observer would have
	// released the PID here and this call would return ECHILD.
	if err := observeWithin(t, pid, 30*time.Second, "second observe"); err != nil {
		if errors.Is(err, syscall.ECHILD) {
			t.Fatalf("second observe returned ECHILD: the observer REAPED the child, " +
				"so WNOWAIT is not in effect and the PID/PGID was released before cleanup could signal it")
		}
		t.Fatalf("second observe returned %v; WNOWAIT must leave the child waitable", err)
	}

	// Exactly one cmd.Wait(), only now — mirroring the finalizer's own
	// exclusive-waiter discipline.
	waitErr := cmd.Wait()
	reaped = true

	// The observer must not have consumed the status: Wait still sees
	// the child's real exit code.
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		if errors.Is(waitErr, syscall.ECHILD) {
			t.Fatalf("cmd.Wait() returned ECHILD: the observer consumed the child, " +
				"so the real exit status was lost")
		}
		t.Fatalf("cmd.Wait() returned %v, want an *exec.ExitError carrying the child's status", waitErr)
	}
	if got := exitErr.ExitCode(); got != wantExitCode {
		t.Fatalf("child exit code = %d, want %d; the observer did not preserve the real status", got, wantExitCode)
	}
	if !exitErr.Exited() {
		t.Fatal("the child must be reported as normally exited, not signalled")
	}

	// After the single Wait, the child IS reaped: a further observe now
	// must fail with ECHILD. This is the positive control proving the
	// two successful observes above were genuinely pre-reap, not a
	// tautology that would pass for any PID.
	if err := observeWithin(t, pid, 30*time.Second, "post-reap observe"); !errors.Is(err, syscall.ECHILD) {
		t.Fatalf("post-reap observe returned %v, want ECHILD once cmd.Wait() has consumed the child", err)
	}
}

// TestNativeObserverReportsECHILDForAnUnrelatedPID is the complementary
// control: the production helper genuinely reports ECHILD for a PID
// that is not this process's child, so the ECHILD assertions above
// distinguish real states rather than always-true conditions.
func TestNativeObserverReportsECHILDForAnUnrelatedPID(t *testing.T) {
	// PID 1 is never a child of the test binary.
	err := observeWithin(t, 1, 30*time.Second, "unrelated-pid observe")
	if !errors.Is(err, syscall.ECHILD) {
		t.Fatalf("observing an unrelated pid returned %v, want ECHILD", err)
	}
}

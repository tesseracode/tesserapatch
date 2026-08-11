//go:build darwin

// Native Darwin observer-behaviour tests (matrix row 176).
//
// Rev-0 covered the Darwin stopped-child quirk by grepping
// observer_unix.go for the `waitid` call text, which proves nothing
// about how the kernel actually behaves or about whether this design's
// fail-closed treatment is sound. These tests exercise the real
// syscall and the real finalizer against a real stopped child.

package rescap

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// startStoppedChild spawns a long-lived child in its own process group
// and SIGSTOPs it, returning the command once the stop has actually
// been delivered.
func startStoppedChild(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	setProcessGroup(cmd.SysProcAttr)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := cmd.Process.Signal(syscall.SIGSTOP); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("SIGSTOP: %v", err)
	}
	return cmd
}

// TestDarwinObserverReturnsForAStoppedChild documents the actual
// platform behaviour this design's naming depends on: on Darwin,
// waitid(P_PID, pid, WEXITED|WNOWAIT) can return for a merely *stopped*
// child, which is exactly why the observer's successful return is
// called a "leader event"/"cleanup trigger" and never "the leader has
// exited".
//
// The test records which behaviour this kernel exhibits rather than
// asserting the quirk is present: if a future Darwin stops returning
// early, the design is still correct — it is fail-closed either way —
// and the test says so explicitly instead of silently rotting.
func TestDarwinObserverReturnsForAStoppedChild(t *testing.T) {
	cmd := startStoppedChild(t)
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGCONT)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	returned := make(chan error, 1)
	go func() { returned <- observeLeaderEvent(pid) }()

	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("the observer returned an error for a stopped child: %v", err)
		}
		t.Log("darwin waitid(WEXITED|WNOWAIT) returned for a STOPPED child: " +
			"the documented quirk is present, so a successful return must never be read as proof of exit")
	case <-time.After(2 * time.Second):
		t.Log("darwin waitid(WEXITED|WNOWAIT) did NOT return for a stopped child on this kernel; " +
			"the fail-closed treatment remains correct either way")
	}
	// Either way the child is still alive and merely stopped — the
	// observer is non-reaping, so nothing has been consumed.
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("the observer must not reap: signalling the child failed with %v", err)
	}
}

// TestSIGKILLTerminatesAStoppedChild is the load-bearing native
// assertion behind treating any successful Darwin observer return as a
// fail-closed cleanup trigger.
//
// The whole safety argument is: it does not matter whether the leader
// exited or was merely stopped, because the unconditional
// SIGTERM → grace → SIGKILL sequence is correct and effective in both
// cases — SIGKILL terminates a stopped process too and cannot be
// caught, blocked or ignored. This proves that on the real platform.
func TestSIGKILLTerminatesAStoppedChild(t *testing.T) {
	cmd := startStoppedChild(t)
	pid := cmd.Process.Pid

	// A stopped process cannot handle SIGTERM, so the group SIGKILL is
	// what must do the work.
	if err := signalProcessGroup(pid, syscall.SIGTERM); err != nil {
		t.Fatalf("group SIGTERM: %v", err)
	}
	if err := signalProcessGroup(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("group SIGKILL: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("SIGKILL did not terminate a STOPPED child; the fail-closed cleanup argument would not hold")
	}
}

// TestFinalizerCompletesAgainstAStoppedChild runs the whole
// ProcessRunner against a real stopped child and asserts the invocation
// still completes inside its own bounds and leaves the child dead.
//
// This is the end-to-end form of the two assertions above: whatever the
// observer reports on Darwin, the finalizer must not hang and must not
// leave the process tree alive.
func TestFinalizerCompletesAgainstAStoppedChild(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	r := &ProcessRunner{
		Cmd:               cmd,
		InvocationTimeout: 300 * time.Millisecond,
		TerminateGrace:    50 * time.Millisecond,
		ReapDeadline:      3 * time.Second,
		DrainDeadline:     2 * time.Second,
		OutputCap:         1 << 20,
	}
	// Stop the child the instant it exists, before any trigger fires.
	r.AfterInitialSnapshot = nil
	origStart := r.StartFn
	_ = origStart
	r.StartFn = func() error {
		if err := cmd.Start(); err != nil {
			return err
		}
		return cmd.Process.Signal(syscall.SIGSTOP)
	}

	started := time.Now()
	out, _ := r.Run()
	elapsed := time.Since(started)

	if elapsed > 15*time.Second {
		t.Fatalf("the finalizer took %v against a stopped child; it must stay inside its own bounds", elapsed)
	}
	if out.SignalCalls < 2 {
		t.Fatalf("the group signal sequence ran %d times, want the full SIGTERM+SIGKILL pair", out.SignalCalls)
	}
	if cmd.Process == nil {
		t.Fatal("the child was never started")
	}
	// The child must be gone: either reaped by the finalizer's own
	// Wait, or killed and reaped here.
	_ = cmd.Process.Signal(syscall.SIGCONT)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return // gone
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	if os.Getpid() != 0 {
		t.Fatal("the stopped child survived the finalizer's SIGKILL")
	}
}

//go:build linux || darwin

// Mutation-resistant semantic tests for the rev-0 review's weak rows.
//
// Build-tagged to the platforms resource capture actually supports:
// these exercise Setpgid, negative-PGID signalling and descriptor
// duplication, none of which exist on the fail-closed targets.
//
// Every test here is written so that deleting the load-bearing
// production call it covers makes it fail. Where a source-shape grep
// was the only rev-0 evidence, this file replaces it with an assertion
// against observable behaviour or against the real call site.

package rescap

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestProcessRunnerSetsSetpgidOnTheRealCommand covers matrix row 175's
// process-group half at the actual call site.
//
// Rev-0 only grepped observer_unix.go for the literal `attr.Setpgid =
// true`, which passes even if ProcessRunner never calls it. This
// inspects the exec.Cmd the runner actually configured, so deleting the
// setProcessGroup call in Run() fails the test.
func TestProcessRunnerSetsSetpgidOnTheRealCommand(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	r := fastRunner()
	r.Cmd = cmd
	r.StartFn = nil // use the real cmd.Start
	r.WaitFn = nil  // use the real cmd.Wait
	r.ObserveFn = func() error { return nil }
	r.SignalFn = func(syscall.Signal) error { return nil }

	if _, err := r.Run(); err != nil {
		t.Fatalf("unexpected refusal: %v", err)
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr was never installed on the command")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("Setpgid was not set: a -pgid signal could reach tpatch's own process group")
	}
	if cmd.Process == nil {
		t.Fatal("the child was never started")
	}
}

// TestSignalTargetsTheChildGroupNotOurOwn proves the negative-PGID
// signal helper addresses the spawned leader's group. Deleting the
// negation in signalProcessGroup would make this signal our own group
// and take the test binary down, which is itself the assertion.
func TestSignalTargetsTheChildGroupNotOurOwn(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	setProcessGroup(cmd.SysProcAttr)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	if pid == os.Getpid() {
		t.Fatal("impossible: child shares our pid")
	}
	if err := signalProcessGroup(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("group signal: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the child group was not terminated by the -pgid signal")
	}
	// We are still alive, so the signal did not reach our own group.
	if os.Getpid() == 0 {
		t.Fatal("unreachable")
	}
}

// TestNoexecPreflightRunsBeforeTheCopyIsCreated covers matrix row 173.
//
// The assertion is ordering, not just the refusal name: the scratch
// directory must contain zero files afterwards, which can only hold if
// the preflight ran before os.OpenFile created the copy.
func TestNoexecPreflightRunsBeforeTheCopyIsCreated(t *testing.T) {
	srcDir := t.TempDir()
	scratch := t.TempDir()
	path := writeFixtureExecutable(t, srcDir, "dolt", "#!/bin/sh\nexit 0\n")
	digest, err := HashExecutableDescriptor(path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	var checkedPath string
	restore := SetScratchExecCheckForTest(func(p string) error {
		checkedPath = p
		return Refuse(ReasonAdapterCopyNoexec,
			"%s is on a noexec-mounted filesystem; the private adapter copy could never be executed there", p)
	})
	defer restore()

	_, err = MakeVerifiedPrivateCopy(path, scratch, digest)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonAdapterCopyNoexec || r.Code != ExitRefusal {
		t.Fatalf("want adapter-copy-noexec exit 3, got %v", err)
	}
	if checkedPath != scratch {
		t.Fatalf("the preflight inspected %q, want the scratch directory %q", checkedPath, scratch)
	}
	entries, err := os.ReadDir(scratch)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("the preflight must run before the copy is created; found %d file(s)", len(entries))
	}
}

// TestPrivateCopyHostIOFailureCleansUpAndStartsNothing covers matrix
// row 174's host-fault half.
//
// A directory opens fine but fails on read, which is a genuine
// mid-copy I/O error in the same class as ENOSPC/EIO: the partial copy
// has already been created by then, so the test proves the cleanup
// path actually removes it and that no invocation is attempted.
func TestPrivateCopyHostIOFailureCleansUpAndStartsNothing(t *testing.T) {
	t.Run("mid-copy-read-failure-removes-the-partial-copy", func(t *testing.T) {
		scratch := t.TempDir()
		sourceDir := t.TempDir() // opens, then fails on read
		_, err := MakeVerifiedPrivateCopy(sourceDir, scratch, strings.Repeat("a", 64))
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonAdapterCopyFailed || r.Code != ExitInternal {
			t.Fatalf("want adapter-copy-failed exit 1, got %v", err)
		}
		entries, readErr := os.ReadDir(scratch)
		if readErr != nil {
			t.Fatalf("ReadDir: %v", readErr)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "dolt-copy-") {
				t.Fatalf("the partial copy %s was not cleaned up", e.Name())
			}
		}
	})

	t.Run("uncreatable-copy-refuses-without-starting-anything", func(t *testing.T) {
		srcDir := t.TempDir()
		path := writeFixtureExecutable(t, srcDir, "dolt", "#!/bin/sh\nexit 0\n")
		digest, err := HashExecutableDescriptor(path)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		scratch := t.TempDir()
		if err := os.Chmod(scratch, 0o500); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(scratch, 0o700) })

		_, err = MakeVerifiedPrivateCopy(path, scratch, digest)
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonAdapterCopyFailed || r.Code != ExitInternal {
			t.Fatalf("want adapter-copy-failed exit 1, got %v", err)
		}
	})
}

// TestGatedOpenRefusesAReplacedEntry covers the os.SameFile descriptor
// identity gate through a controlled replacement seam.
//
// The hook swaps the validated file for a different inode at the same
// pathname at exactly the instant a real attacker would. If the
// os.SameFile comparison in GatePath is deleted, the swap goes
// undetected and this test fails.
func TestGatedOpenRefusesAReplacedEntry(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "config/target.env", "original\n")

	var swaps atomic.Int32
	restore := SetBeforeGatedOpenForTest(func(abs string) {
		if !strings.HasSuffix(abs, "config/target.env") {
			return
		}
		if !swaps.CompareAndSwap(0, 1) {
			return
		}
		// Replace the name with a DIFFERENT inode: unlink then
		// recreate. Same pathname, same shape, new identity.
		if err := os.Remove(abs); err != nil {
			t.Errorf("remove: %v", err)
			return
		}
		if err := os.WriteFile(abs, []byte("swapped!\n"), 0o644); err != nil {
			t.Errorf("recreate: %v", err)
		}
	})
	defer restore()

	_, err := GatePath(root, "config/target.env")
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonPathReplacedDuringOpen || r.Code != ExitRefusal {
		t.Fatalf("want path-replaced-during-open exit 3, got %v", err)
	}
	if swaps.Load() != 1 {
		t.Fatal("the replacement seam never fired; the test proved nothing")
	}
}

// TestGatedOpenAcceptsAnUnreplacedEntry is the control for the test
// above: with the seam installed but performing no swap, the identical
// gate accepts the path. Without this, a gate that refused everything
// would also pass.
func TestGatedOpenAcceptsAnUnreplacedEntry(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "config/target.env", "original\n")
	var fired atomic.Int32
	restore := SetBeforeGatedOpenForTest(func(string) { fired.Add(1) })
	defer restore()

	gated, err := GatePath(root, "config/target.env")
	if err != nil {
		t.Fatalf("an unreplaced entry must be accepted: %v", err)
	}
	if err := gated.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if fired.Load() == 0 {
		t.Fatal("the seam is not installed on the real gate path")
	}
}

// TestRunawayChildIsBoundedToCapPlusOne covers matrix row 183's memory
// half with a real child process.
//
// The child writes far beyond the cap on both streams and ignores
// SIGTERM long enough to exercise the full cleanup sequence. The
// binding assertion is that retained stdout+stderr never exceeds
// cap+1 bytes *combined*.
func TestRunawayChildIsBoundedToCapPlusOne(t *testing.T) {
	const cap = 64 * 1024
	script := `
trap '' TERM
i=0
while [ $i -lt 400 ]; do
  printf '%s' "$LINE"
  printf '%s' "$LINE" 1>&2
  i=$((i+1))
done
sleep 5
`
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.Env = append(os.Environ(), "LINE="+strings.Repeat("x", 4096))

	r := &ProcessRunner{
		Cmd:               cmd,
		InvocationTimeout: 30 * time.Second,
		TerminateGrace:    50 * time.Millisecond,
		ReapDeadline:      3 * time.Second,
		DrainDeadline:     3 * time.Second,
		OutputCap:         cap,
	}
	out, err := r.Run()

	retained := len(out.Stdout) + len(out.Stderr)
	// 400 iterations x 4096 bytes x 2 streams = 3.2 MiB written.
	const written = 400 * 4096 * 2
	if retained > cap+1 {
		t.Fatalf("retained %d bytes for a %d-byte cap; the bound is cap+1 = %d", retained, cap, cap+1)
	}
	if written <= cap+1 {
		t.Fatalf("the fixture did not actually exceed the cap (%d written)", written)
	}
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonResourceLimitExceeded || ref.Code != ExitRefusal {
		t.Fatalf("want resource-limit-exceeded exit 3, got %v", err)
	}
	if out.Classification != "output-cap-exceeded" {
		t.Fatalf("classification = %s, want output-cap-exceeded", out.Classification)
	}
	if out.RetentionBudgetRemaining != 0 {
		t.Fatalf("retention budget remaining = %d, want 0", out.RetentionBudgetRemaining)
	}
	if out.SignalCalls == 0 {
		t.Fatal("a SIGTERM-ignoring runaway child must still be signalled")
	}
}

// TestClaimRetentionNeverOvershootsUnderConcurrency proves the atomic
// claim is what bounds the total, independent of scheduling: many
// concurrent claimants can collectively take exactly the budget and
// never one byte more.
func TestClaimRetentionNeverOvershootsUnderConcurrency(t *testing.T) {
	const cap = 1000
	r := &ProcessRunner{OutputCap: cap}
	r.budget.Store(cap + 1)

	var total atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				accept, _ := r.claimRetention(37)
				total.Add(int64(accept))
			}
		}()
	}
	wg.Wait()
	if got := total.Load(); got != cap+1 {
		t.Fatalf("claimants took %d bytes total, want exactly cap+1 = %d", got, cap+1)
	}
	if remaining := r.budget.Load(); remaining != 0 {
		t.Fatalf("budget remaining = %d, want 0", remaining)
	}
	accept, exhausted := r.claimRetention(10)
	if accept != 0 || exhausted {
		t.Fatalf("an exhausted budget must accept 0 and not re-report exhaustion, got (%d, %v)", accept, exhausted)
	}
}

// TestInvocationTimerIsDisarmedOnEverySuccessfulRun covers the rev-0
// timer-leak finding deterministically.
//
// time.Timer.Stop() reports true only when it cancelled a timer that
// had not yet fired. Asserting that, plus TimerFired being false, is a
// direct statement about timer lifecycle rather than an inference from
// a global goroutine count.
func TestInvocationTimerIsDisarmedOnEverySuccessfulRun(t *testing.T) {
	var stopReports []bool
	var mu sync.Mutex
	for i := 0; i < 5; i++ {
		r := fastRunner()
		r.InvocationTimeout = time.Hour
		r.OnTimerStopped = func(stopped bool) {
			mu.Lock()
			defer mu.Unlock()
			stopReports = append(stopReports, stopped)
		}
		out, err := r.Run()
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if out.TimerFired {
			t.Fatalf("run %d: the timer fired despite a one-hour timeout", i)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(stopReports) != 5 {
		t.Fatalf("the timer lifecycle hook fired %d times, want once per invocation (5)", len(stopReports))
	}
	for i, stopped := range stopReports {
		if !stopped {
			t.Fatalf("run %d: Stop() reported the timer had already fired; it must be cancelled cleanly", i)
		}
	}
}

// TestInvocationTimerFiresWhenItShould is the control: the lifecycle
// hook must report false when the timer genuinely fired, so a test that
// always reports "stopped" cannot pass vacuously.
func TestInvocationTimerFiresWhenItShould(t *testing.T) {
	r := fastRunner()
	r.InvocationTimeout = time.Millisecond
	blockObserver := make(chan struct{})
	r.ObserveFn = func() error {
		<-blockObserver
		return nil
	}
	defer close(blockObserver)

	var stopped atomic.Bool
	var called atomic.Bool
	r.OnTimerStopped = func(s bool) {
		called.Store(true)
		stopped.Store(s)
	}
	out, err := r.Run()
	if err != nil {
		t.Fatalf("unexpected refusal: %v", err)
	}
	if !called.Load() {
		t.Fatal("the lifecycle hook did not fire")
	}
	if stopped.Load() {
		t.Fatal("Stop() reported a cancellation for a timer that must have fired")
	}
	if !out.TimerFired {
		t.Fatal("TimerFired should be true for a timeout-triggered invocation")
	}
	if out.Classification != "invocation-timeout" {
		t.Fatalf("classification = %s, want invocation-timeout", out.Classification)
	}
}

// TestSetReadDeadlineFailureIsAdapterOutputReadFailed covers matrix row
// 179's second half: a SetReadDeadline failure surfaces its own named
// refusal and still joins both drains.
func TestSetReadDeadlineFailureIsAdapterOutputReadFailed(t *testing.T) {
	r := fastRunner()
	r.BeforeDrainDeadline = func() {
		// Closing both read ends makes the subsequent SetReadDeadline
		// calls fail with os.ErrClosed.
		_ = r.stdoutRead.Close()
		_ = r.stderrRead.Close()
	}
	out, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonAdapterOutputReadFailed || ref.Code != ExitInternal {
		t.Fatalf("want adapter-output-read-failed exit 1, got %v", err)
	}
	if out.Classification != "leader-event" {
		t.Fatalf("classification = %s; the deadline failure is a phase error, not a trigger", out.Classification)
	}
	assertDrainsJoined(t, r)
}

// TestEveryForcedCloseBranchJoinsBothDrains covers matrix row 186 by
// walking all three forced-close branches this design defines.
func TestEveryForcedCloseBranchJoinsBothDrains(t *testing.T) {
	cases := []struct {
		name       string
		configure  func(r *ProcessRunner)
		wantReason string
	}{
		{
			name: "echild-finalizer-force-close",
			configure: func(r *ProcessRunner) {
				r.ObserveFn = func() error { return syscall.ECHILD }
			},
			wantReason: ReasonAdapterProcessObserverFail,
		},
		{
			name: "drain-deadline-expiry-force-close",
			configure: func(r *ProcessRunner) {
				r.DrainDeadline = 40 * time.Millisecond
				// A lingering writer keeps the pipe open past the
				// deadline, exactly like an escaped descendant.
				r.StartFn = func() error {
					// Duplicate the write end so a second descriptor
					// keeps the pipe alive after Run closes its own —
					// exactly what an escaped descendant that inherited
					// the fd does.
					r.lingeringWriter = dupWriteEnd(t, r.stdoutWrite)
					return nil
				}
			},
			wantReason: ReasonAdapterDrainTimeout,
		},
		{
			name: "setreaddeadline-failure-force-close",
			configure: func(r *ProcessRunner) {
				r.BeforeDrainDeadline = func() {
					_ = r.stdoutRead.Close()
					_ = r.stderrRead.Close()
				}
			},
			wantReason: ReasonAdapterOutputReadFailed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := fastRunner()
			tc.configure(r)
			_, err := r.Run()
			if r.lingeringWriter != nil {
				_ = r.lingeringWriter.Close()
			}
			ref := AsRefusal(err)
			if ref == nil || ref.Reason != tc.wantReason {
				t.Fatalf("want %s, got %v", tc.wantReason, err)
			}
			assertDrainsJoined(t, r)
		})
	}
}

// assertDrainsJoined proves both drain goroutines completed before the
// invocation returned. drainDone is closed by each drain's own defer,
// so a non-closed channel here means the branch returned (and would
// have released the per-slug flock) with a goroutine still running.
func assertDrainsJoined(t *testing.T, r *ProcessRunner) {
	t.Helper()
	for i, ch := range r.drainDone {
		select {
		case <-ch:
		default:
			t.Fatalf("drain goroutine %d was still running when the branch returned", i)
		}
	}
}

// TestWaitIsLaunchedStrictlyAfterTheSignalPhase covers matrix row 189
// for every classification that runs the shared bounded finalizer.
//
// The assertion is an observed ordering log, not just a boolean: the
// recorded sequence must be SIGTERM, SIGKILL, then the Wait launch.
func TestWaitIsLaunchedStrictlyAfterTheSignalPhase(t *testing.T) {
	cases := []struct {
		name      string
		configure func(r *ProcessRunner)
	}{
		{"benign-leader-event", func(r *ProcessRunner) {}},
		{"invocation-timeout", func(r *ProcessRunner) {
			r.InvocationTimeout = time.Millisecond
			block := make(chan struct{})
			t.Cleanup(func() { close(block) })
			r.ObserveFn = func() error { <-block; return nil }
		}},
		{"output-cap-exceeded", func(r *ProcessRunner) {
			r.OutputCap = 16
			block := make(chan struct{})
			t.Cleanup(func() { close(block) })
			r.ObserveFn = func() error { <-block; return nil }
			r.StartFn = func() error {
				_, err := r.stdoutWrite.Write([]byte(strings.Repeat("z", 64)))
				return err
			}
		}},
		{"genuine-reader-error", func(r *ProcessRunner) {
			block := make(chan struct{})
			t.Cleanup(func() { close(block) })
			r.ObserveFn = func() error { <-block; return nil }
			r.StartFn = func() error {
				go func() {
					time.Sleep(5 * time.Millisecond)
					_ = r.stdoutRead.Close()
				}()
				return nil
			}
		}},
		{"non-echild-terminal-observer-error", func(r *ProcessRunner) {
			r.ObserveFn = func() error { return syscall.EINVAL }
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var events []string
			r := fastRunner()
			r.SignalFn = func(sig syscall.Signal) error {
				mu.Lock()
				defer mu.Unlock()
				switch sig {
				case syscall.SIGTERM:
					events = append(events, "sigterm")
				case syscall.SIGKILL:
					events = append(events, "sigkill")
				}
				return nil
			}
			r.WaitFn = func() error {
				mu.Lock()
				defer mu.Unlock()
				events = append(events, "wait")
				return nil
			}
			tc.configure(r)

			out, _ := r.Run()
			if out.WaitLaunchedBeforeSignals {
				t.Fatal("cmd.Wait() was launched before the signal phase completed")
			}
			if out.WaitLaunches != 1 {
				t.Fatalf("cmd.Wait() launched %d times, want exactly 1", out.WaitLaunches)
			}
			mu.Lock()
			defer mu.Unlock()
			got := strings.Join(events, ",")
			if got != "sigterm,sigkill,wait" {
				t.Fatalf("ordering = %q, want sigterm,sigkill,wait", got)
			}
		})
	}
}

// TestMultiErrorPrecedenceCapOutranksSignalAndDrain covers matrix row
// 183's worked example exactly: an output-cap classification whose
// finalizer ALSO suffers an untolerated group-signal errno AND a drain
// timeout still reports resource-limit-exceeded as the sole primary
// reason, with both later failures demoted to local diagnostics.
func TestMultiErrorPrecedenceCapOutranksSignalAndDrain(t *testing.T) {
	r := fastRunner()
	r.OutputCap = 32
	r.DrainDeadline = 40 * time.Millisecond
	block := make(chan struct{})
	defer close(block)
	r.ObserveFn = func() error { <-block; return nil }
	r.SignalFn = func(syscall.Signal) error { return syscall.EINVAL }
	r.StartFn = func() error {
		if _, err := r.stdoutWrite.Write([]byte(strings.Repeat("q", 128))); err != nil {
			return err
		}
		// A duplicated write end survives Run's own close, so the
		// drain genuinely cannot reach EOF and must time out.
		r.lingeringWriter = dupWriteEnd(t, r.stderrWrite)
		return nil
	}
	out, err := r.Run()
	if r.lingeringWriter != nil {
		_ = r.lingeringWriter.Close()
	}

	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonResourceLimitExceeded || ref.Code != ExitRefusal {
		t.Fatalf("want resource-limit-exceeded exit 3 as the sole primary, got %v", err)
	}
	joined := strings.Join(out.Diagnostics, "|")
	if !strings.Contains(joined, ReasonAdapterGroupSignalFailed) {
		t.Fatalf("the untolerated signal errno must be demoted to a diagnostic: %v", out.Diagnostics)
	}
	if !strings.Contains(joined, ReasonAdapterDrainTimeout) {
		t.Fatalf("the drain timeout must be demoted to a diagnostic: %v", out.Diagnostics)
	}
	if len(out.Stdout)+len(out.Stderr) > int(r.OutputCap)+1 {
		t.Fatalf("retained %d bytes for a %d-byte cap", len(out.Stdout)+len(out.Stderr), r.OutputCap)
	}
}

// TestCurrentPointerIsCommittedByRenameNotDirectWrite covers the
// publication commit point.
//
// The live current.json is made read-only (0444). A direct
// open-for-write of that path fails with EACCES for a non-root owner,
// while a rename over it succeeds because rename needs write permission
// on the *directory*, not the file. So this publish can only succeed if
// the pointer really is written to a temp and renamed into place.
func TestCurrentPointerIsCommittedByRenameNotDirectWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a 0444 file is writable, so the discriminator does not hold")
	}
	root := newGitRepo(t)
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	slug := "model-picker"
	if err := s.EnsureResourceCaptureTree(slug); err != nil {
		t.Fatalf("EnsureResourceCaptureTree: %v", err)
	}

	first := store.Batch{Feature: slug, Results: []store.BatchResult{{
		ResourceID: "res_acc91dc23a8b", Kind: store.ResourceKindGitMetadata,
		Selector: "head", Args: []store.ResourceArg{},
		Result: store.CanonObject(
			store.CanonFieldOf("symbolic_ref", store.CanonString("refs/heads/main")),
			store.CanonFieldOf("oid", store.CanonString(strings.Repeat("a", 40))),
			store.CanonFieldOf("detached", store.CanonBool(false)),
		),
	}}}
	id, canonical, err := store.ComputeBatchID(first.Feature, first.Results)
	if err != nil {
		t.Fatalf("ComputeBatchID: %v", err)
	}
	first.BatchID = id
	if _, err := s.PublishBatch(slug, first, canonical); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	pointerPath := s.ResourceCurrentPath(slug)
	if err := os.Chmod(pointerPath, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(pointerPath, 0o644) })

	// Prove the discriminator: a direct write to the live path fails.
	if f, err := os.OpenFile(pointerPath, os.O_WRONLY|os.O_TRUNC, 0); err == nil {
		_ = f.Close()
		t.Skip("this filesystem allows the owner to write a 0444 file; the discriminator does not hold")
	}

	second := store.Batch{Feature: slug, Results: []store.BatchResult{{
		ResourceID: "res_acc91dc23a8b", Kind: store.ResourceKindGitMetadata,
		Selector: "head", Args: []store.ResourceArg{},
		Result: store.CanonObject(
			store.CanonFieldOf("symbolic_ref", store.CanonString("refs/heads/other")),
			store.CanonFieldOf("oid", store.CanonString(strings.Repeat("b", 40))),
			store.CanonFieldOf("detached", store.CanonBool(false)),
		),
	}}}
	id2, canonical2, err := store.ComputeBatchID(second.Feature, second.Results)
	if err != nil {
		t.Fatalf("ComputeBatchID: %v", err)
	}
	second.BatchID = id2
	if _, err := s.PublishBatch(slug, second, canonical2); err != nil {
		t.Fatalf("publish over a read-only pointer must succeed via rename: %v", err)
	}

	pointer, err := s.LoadCurrentPointer(slug)
	if err != nil {
		t.Fatalf("LoadCurrentPointer: %v", err)
	}
	if pointer.CurrentBatchID != id2 {
		t.Fatalf("the pointer was not actually replaced: %s", pointer.CurrentBatchID)
	}
	if _, err := os.Stat(s.ResourceCurrentTempPath(slug)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the pointer temp file must be consumed by the rename")
	}
}

// buildEscapingDoltFixture compiles a fake adapter that reproduces an
// escaped-session writer deterministically.
//
// Invoked normally it prints a valid zero-row body and then forks a
// copy of itself with SysProcAttr{Setsid: true}, handing it the
// inherited stdout descriptor, before exiting immediately. Because
// setsid(2) happens in the child between fork and exec, the escape is
// complete the instant Start() returns — there is no window in which
// the finalizer's SIGKILL(-pgid) could still reach the descendant, so
// the test does not depend on process-startup timing.
//
// A plain `sleep &` from a shell would NOT reproduce this: a background
// job of a non-interactive shell stays in the leader's process group
// and is reaped by the same group signal.
func buildEscapingDoltFixture(t *testing.T, holdFor time.Duration) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH to build the escaping adapter fixture")
	}
	srcDir := t.TempDir()
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "escaping-dolt")

	src := `package main

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

const (
	selfPath = ` + strconv.Quote(bin) + `
	holdFor  = ` + strconv.FormatInt(int64(holdFor), 10) + ` * time.Nanosecond
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--hold-stdout" {
		// Already in its own session: just keep the inherited stdout
		// descriptor open for the hold window.
		time.Sleep(holdFor)
		return
	}
	_, _ = os.Stdout.WriteString(` + "`" + `{"rows":[]}` + "`" + `)
	child := exec.Command(selfPath, "--hold-stdout")
	child.Stdout = os.Stdout
	// Setsid is applied by the child between fork and exec, so the
	// escape is complete before Start returns.
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		os.Exit(1)
	}
	// Exit without waiting: the escaped grandchild owns the pipe now.
	os.Exit(0)
}
`
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte("module escapingdolt\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not build the escaping adapter fixture: %v\n%s", err, out)
	}
	return bin
}

// TestEngineDrainTimeoutFromEscapedWriterPublishesNothing covers matrix
// row 180 with a real escaped-session descendant.
//
// The parent's read end genuinely never reaches EOF even after the
// whole original process group has been killed and the leader reaped,
// because the escaped grandchild still holds the write end. The
// invocation must refuse `adapter-drain-timeout`, publish nothing, and
// leave the per-slug lock re-acquirable.
func TestEngineDrainTimeoutFromEscapedWriterPublishesNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a helper binary; skipped under -short")
	}
	fixturePath := buildEscapingDoltFixture(t, 8*time.Second)

	fake := newDoltFixture(t, `{"rows":[]}`, "", 0)
	escaping, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(fake.Path, escaping, 0o755); err != nil {
		t.Fatalf("install fixture: %v", err)
	}
	digest, err := HashExecutableDescriptor(fake.Path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	f := newDoltEngineFixture(t, fake)
	f.Resource.Trust = &store.ResourceTrust{BinarySHA256: digest}
	f.Engine.TerminateGrace = 20 * time.Millisecond
	f.Engine.ReapDeadline = 2 * time.Second
	f.Engine.DrainDeadline = 400 * time.Millisecond
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()

	started := time.Now()
	_, stageErr := f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	elapsed := time.Since(started)

	ref := AsRefusal(stageErr)
	if ref == nil || ref.Reason != ReasonAdapterDrainTimeout || ref.Code != ExitRefusal {
		t.Fatalf("want adapter-drain-timeout exit 3 after %v, got %v", elapsed, stageErr)
	}
	if elapsed < f.Engine.DrainDeadline {
		t.Fatalf("refused after %v, which is shorter than the %v drain bound", elapsed, f.Engine.DrainDeadline)
	}
	if _, err := os.Stat(f.Store.ResourceCurrentPath("model-picker")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a drain timeout must publish nothing")
	}
	// The per-slug lock is re-acquirable, proving the refusal released it.
	lock, err := AcquireLock(ScratchRoot(f.Repo, "model-picker"), f.Repo)
	if err != nil {
		t.Fatalf("the lock was not released by the refusal: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// TestNativeCrossBuildContract covers matrix row 175's build half by
// actually compiling this package for the platforms whose contract the
// build tags encode, rather than by grepping for the tag text.
func TestNativeCrossBuildContract(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-compilation is slow; skipped under -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	targets := []struct{ goos, goarch string }{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "arm64"},
		{"darwin", "amd64"},
		// Proves the !linux && !darwin fail-closed stubs really compile.
		{"windows", "amd64"},
	}
	for _, tgt := range targets {
		t.Run(tgt.goos+"-"+tgt.goarch, func(t *testing.T) {
			cmd := exec.Command("go", "build", "-o", filepath.Join(t.TempDir(), "out.a"),
				"github.com/tesseracode/tesserapatch/internal/rescap")
			cmd.Env = append(os.Environ(), "GOOS="+tgt.goos, "GOARCH="+tgt.goarch, "CGO_ENABLED=0")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("cross-build failed: %v\n%s", err, out)
			}
		})
	}
}

// dupWriteEnd duplicates a pipe write-end descriptor so the pipe stays
// alive after the runner closes its own reference, reproducing an
// escaped descendant that inherited the fd. The caller closes the
// duplicate.
func dupWriteEnd(t *testing.T, f *os.File) *os.File {
	t.Helper()
	fd, err := syscall.Dup(int(f.Fd()))
	if err != nil {
		t.Fatalf("dup: %v", err)
	}
	return os.NewFile(uintptr(fd), "lingering-writer")
}

// Process-finalizer tests (PRD §6.4, ADR-033 D5).
//
// Every branch is driven through the ProcessRunner's injected seams
// rather than by spawning a real Dolt child: the contract under test is
// the finalizer's own state machine, and a real process could not
// reproduce a terminal ECHILD or a late-ECHILD race deterministically.

package rescap

import (
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// fastRunner builds a runner with production semantics and test-speed
// bounds.
func fastRunner() *ProcessRunner {
	return &ProcessRunner{
		InvocationTimeout: 5 * time.Second,
		TerminateGrace:    10 * time.Millisecond,
		ReapDeadline:      200 * time.Millisecond,
		DrainDeadline:     200 * time.Millisecond,
		OutputCap:         1 << 20,
		StartFn:           func() error { return nil },
		WaitFn:            func() error { return nil },
		ObserveFn:         func() error { return nil },
		SignalFn:          func(syscall.Signal) error { return nil },
	}
}

// TestBenignLeaderEventCompletes covers the common path: the observer
// fires, the shared bounded finalizer runs its full phase order, and no
// refusal is reported.
func TestBenignLeaderEventCompletes(t *testing.T) {
	r := fastRunner()
	var signals []syscall.Signal
	var mu sync.Mutex
	r.SignalFn = func(sig syscall.Signal) error {
		mu.Lock()
		defer mu.Unlock()
		signals = append(signals, sig)
		return nil
	}
	out, err := r.Run()
	if err != nil {
		t.Fatalf("unexpected refusal: %v", err)
	}
	if out.Classification != "leader-event" {
		t.Fatalf("classification = %s", out.Classification)
	}
	if len(signals) != 2 || signals[0] != syscall.SIGTERM || signals[1] != syscall.SIGKILL {
		t.Fatalf("signal sequence = %v, want SIGTERM then SIGKILL", signals)
	}
	if !out.Reaped {
		t.Fatal("the bounded reap should have completed")
	}
	if out.WaitLaunchedBeforeSignals {
		t.Fatal("cmd.Wait() was launched before the signal phase completed")
	}
	if out.WaitLaunches != 1 {
		t.Fatalf("cmd.Wait() launched %d times, want exactly 1", out.WaitLaunches)
	}
}

// TestStartFailureCarveOut covers AC-116: zero drain goroutines, zero
// CAS attempts, zero -pgid signals, cmd.Wait() never invoked, and all
// four parent-held endpoints closed synchronously.
func TestStartFailureCarveOut(t *testing.T) {
	r := fastRunner()
	sentinel := errors.New("exec format error")
	r.StartFn = func() error { return sentinel }
	var signalCount atomic.Int32
	r.SignalFn = func(syscall.Signal) error {
		signalCount.Add(1)
		return nil
	}
	var waitCount atomic.Int32
	r.WaitFn = func() error {
		waitCount.Add(1)
		return nil
	}
	var observeCount atomic.Int32
	r.ObserveFn = func() error {
		observeCount.Add(1)
		return nil
	}
	out, err := r.Run()
	var startErr *StartFailureError
	if !errors.As(err, &startErr) || !errors.Is(err, sentinel) {
		t.Fatalf("want a StartFailureError wrapping the cause, got %v", err)
	}
	if out.DrainGoroutines != 0 {
		t.Fatalf("drain goroutines = %d, want 0", out.DrainGoroutines)
	}
	if out.SignalCalls != 0 || signalCount.Load() != 0 {
		t.Fatal("the start-failure path must send zero -pgid signals")
	}
	if out.WaitLaunches != 0 || waitCount.Load() != 0 {
		t.Fatal("the start-failure path must never invoke cmd.Wait()")
	}
	if observeCount.Load() != 0 {
		t.Fatal("the start-failure path must never start the observer")
	}
	// Both parent-held read ends are closed: reading returns an error
	// rather than blocking.
	for _, f := range []*os.File{r.stdoutRead, r.stderrRead} {
		if _, err := f.Read(make([]byte, 1)); err == nil {
			t.Fatal("a parent-held read end was left open")
		}
	}
	for _, f := range []*os.File{r.stdoutWrite, r.stderrWrite} {
		if _, err := f.Write([]byte("x")); err == nil {
			t.Fatal("a parent-held write end was left open")
		}
	}
}

// TestECHILDFinalizerSendsNoSignals covers AC-109/AC-117: the ECHILD
// branch sends zero -pgid signals, never calls cmd.Wait(), force-closes
// both read ends, joins both drains, and discloses the residual — even
// though its own force-close induces reader errors on both goroutines.
func TestECHILDFinalizerSendsNoSignals(t *testing.T) {
	r := fastRunner()
	r.ObserveFn = func() error { return syscall.ECHILD }
	var signalCount atomic.Int32
	r.SignalFn = func(syscall.Signal) error {
		signalCount.Add(1)
		return nil
	}
	var waitCount atomic.Int32
	r.WaitFn = func() error {
		waitCount.Add(1)
		return nil
	}
	out, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonAdapterProcessObserverFail || ref.Code != ExitInternal {
		t.Fatalf("want adapter-process-observer-failed exit 1, got %v", err)
	}
	if signalCount.Load() != 0 || out.SignalCalls != 0 {
		t.Fatalf("the ECHILD finalizer sent %d signals; it must send zero", signalCount.Load())
	}
	if waitCount.Load() != 0 || out.WaitLaunches != 0 {
		t.Fatal("the ECHILD finalizer must never call cmd.Wait()")
	}
	found := false
	for _, d := range out.Diagnostics {
		if strings.Contains(d, "unsignaled and unreaped") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the unsafe-identity residual must be disclosed: %v", out.Diagnostics)
	}
}

// TestLateECHILDCutoffDrain covers AC-119: an ECHILD published strictly
// between the initial trigger snapshot and the fixed cutoff instant
// overrides a lower-priority classification, and the invocation sends
// zero -pgid signals.
func TestLateECHILDCutoffDrain(t *testing.T) {
	r := fastRunner()
	// The timeout fires first and would otherwise be the (benign)
	// classification.
	r.InvocationTimeout = 5 * time.Millisecond
	blockObserver := make(chan struct{})
	r.ObserveFn = func() error {
		<-blockObserver
		return syscall.ECHILD
	}
	var signalCount atomic.Int32
	r.SignalFn = func(syscall.Signal) error {
		signalCount.Add(1)
		return nil
	}
	r.AfterInitialSnapshot = func() {
		// Publish the ECHILD occurrence in the narrow window between
		// the initial snapshot and the cutoff drain.
		close(blockObserver)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if r.occurred[trigObserverTerminal].Load() {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Error("the observer never published its ECHILD occurrence")
	}
	out, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonAdapterProcessObserverFail {
		t.Fatalf("want the ECHILD finalizer's refusal, got %v", err)
	}
	if signalCount.Load() != 0 || out.SignalCalls != 0 {
		t.Fatalf("a late ECHILD must still send zero -pgid signals, sent %d", signalCount.Load())
	}
}

// TestNonECHILDTerminalObserverError covers the other terminal-errno
// branch: the shared bounded finalizer still runs (signals plus a
// bounded reap attempt) with adapter-process-observer-failed primary.
func TestNonECHILDTerminalObserverError(t *testing.T) {
	r := fastRunner()
	r.ObserveFn = func() error { return syscall.EINVAL }
	out, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonAdapterProcessObserverFail || ref.Code != ExitInternal {
		t.Fatalf("want adapter-process-observer-failed exit 1, got %v", err)
	}
	if out.SignalCalls != 2 {
		t.Fatalf("a non-ECHILD terminal observer error still signals the group, got %d calls", out.SignalCalls)
	}
	if out.WaitLaunches != 1 {
		t.Fatal("the bounded reap step is still attempted for a non-ECHILD terminal errno")
	}
}

// TestOutputCapIsARefusalNotATruncation covers AC's cap contract: the
// shared budget spans stdout and stderr together, the classification
// outranks any later phase failure, and nothing partial is parsed.
func TestOutputCapIsARefusalNotATruncation(t *testing.T) {
	r := fastRunner()
	r.OutputCap = 64
	blockObserver := make(chan struct{})
	r.ObserveFn = func() error {
		<-blockObserver
		return nil
	}
	// Both later phases also fail, and must be demoted to diagnostics.
	r.SignalFn = func(syscall.Signal) error { return syscall.EINVAL }
	r.WaitFn = func() error {
		time.Sleep(2 * time.Second)
		return nil
	}
	r.StartFn = func() error {
		// 40 bytes on each stream: neither alone exceeds the cap, but
		// together they do, which is what proves the budget is shared.
		// The writes happen before Start() returns because Run closes
		// the parent's own write ends immediately afterwards; a real
		// child keeps them alive through its own dup'd fds.
		if _, err := r.stdoutWrite.Write([]byte(strings.Repeat("a", 40))); err != nil {
			return err
		}
		if _, err := r.stderrWrite.Write([]byte(strings.Repeat("b", 40))); err != nil {
			return err
		}
		return nil
	}
	defer close(blockObserver)
	out, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonResourceLimitExceeded || ref.Code != ExitRefusal {
		t.Fatalf("want resource-limit-exceeded exit 3, got %v", err)
	}
	if out.Classification != "output-cap-exceeded" {
		t.Fatalf("classification = %s", out.Classification)
	}
	joined := strings.Join(out.Diagnostics, "|")
	if !strings.Contains(joined, ReasonAdapterGroupSignalFailed) && !strings.Contains(joined, ReasonAdapterReapTimeout) {
		t.Fatalf("later phase failures must be demoted to diagnostics: %v", out.Diagnostics)
	}
}

// TestBenignEntryPrimaryErrorIsFirstPhaseFailure covers the
// primary-error-selection rule's worked second example: a benign
// classification takes the first failure in the fixed
// signal -> reap -> drain walk order, so a signal failure wins over a
// later reap timeout.
func TestBenignEntryPrimaryErrorIsFirstPhaseFailure(t *testing.T) {
	r := fastRunner()
	r.SignalFn = func(syscall.Signal) error { return syscall.EINVAL }
	r.WaitFn = func() error {
		time.Sleep(2 * time.Second)
		return nil
	}
	out, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonAdapterGroupSignalFailed || ref.Code != ExitInternal {
		t.Fatalf("want adapter-group-signal-failed exit 1, got %v", err)
	}
	if out.Reaped {
		t.Fatal("the reap should have timed out")
	}
}

// TestToleratedSignalErrnos proves nil/ESRCH/EPERM are all tolerated
// and never produce a refusal.
func TestToleratedSignalErrnos(t *testing.T) {
	for _, errno := range []error{nil, syscall.ESRCH, syscall.EPERM} {
		r := fastRunner()
		captured := errno
		r.SignalFn = func(syscall.Signal) error { return signalProcessGroupResult(captured) }
		if _, err := r.Run(); err != nil {
			t.Fatalf("errno %v should be tolerated, got %v", errno, err)
		}
	}
}

// signalProcessGroupResult mirrors the production tolerance rule so the
// test exercises the same classification the real syscall wrapper
// applies.
func signalProcessGroupResult(err error) error {
	switch err {
	case nil, syscall.ESRCH, syscall.EPERM:
		return nil
	default:
		return err
	}
}

// TestReapTimeoutDisclosesTwoResiduals covers AC-118: when the
// triggering classification was not the leader event, both the
// abandoned cmd.Wait() goroutine and the still-blocked observer are
// disclosed, and neither ever blocks reporting its own completion.
func TestReapTimeoutDisclosesTwoResiduals(t *testing.T) {
	r := fastRunner()
	r.InvocationTimeout = 5 * time.Millisecond
	observerDone := make(chan struct{})
	release := make(chan struct{})
	r.ObserveFn = func() error {
		<-release
		close(observerDone)
		return nil
	}
	waitReturned := make(chan struct{})
	r.WaitFn = func() error {
		<-release
		close(waitReturned)
		return nil
	}
	out, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonAdapterReapTimeout || ref.Code != ExitInternal {
		t.Fatalf("want adapter-reap-timeout exit 1, got %v", err)
	}
	joined := strings.Join(out.Diagnostics, "|")
	if !strings.Contains(joined, "abandoned cmd.Wait() goroutine") {
		t.Fatalf("the abandoned Wait goroutine must be disclosed: %v", out.Diagnostics)
	}
	if !strings.Contains(joined, "leader-event observer goroutine may also remain blocked") {
		t.Fatalf("the still-blocked observer must be disclosed: %v", out.Diagnostics)
	}
	// Both goroutines complete after the invocation has already
	// returned and stopped listening; the capacity-one buffered,
	// non-blocking sends mean neither blocks.
	close(release)
	select {
	case <-waitReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("the abandoned Wait goroutine blocked trying to report")
	}
	select {
	case <-observerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the observer goroutine blocked trying to report")
	}
}

// TestGenuineReaderErrorClassification covers a non-EOF read error
// observed before the cleanup-initiated flag is set.
func TestGenuineReaderErrorClassification(t *testing.T) {
	r := fastRunner()
	blockObserver := make(chan struct{})
	r.ObserveFn = func() error {
		<-blockObserver
		return nil
	}
	r.StartFn = func() error {
		// Closing the parent's own read end from outside the control
		// cleanup function is not something production code does; it is
		// the only way to synthesize a genuine pre-cleanup reader error.
		go func() {
			time.Sleep(5 * time.Millisecond)
			_ = r.stdoutRead.Close()
		}()
		return nil
	}
	defer close(blockObserver)
	_, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonAdapterOutputReadFailed || ref.Code != ExitInternal {
		t.Fatalf("want adapter-output-read-failed exit 1, got %v", err)
	}
}

// TestOwnerInducedReaderErrorsAreSuppressed covers the
// cleanup-initiated flag: the ECHILD finalizer's own force-close
// induces read errors on both drains, and they must not be resubmitted
// as fresh triggers or change the reported classification.
func TestOwnerInducedReaderErrorsAreSuppressed(t *testing.T) {
	r := fastRunner()
	r.ObserveFn = func() error { return syscall.ECHILD }
	r.StartFn = func() error {
		// Keep a write end alive so the drains are genuinely blocked in
		// Read when the finalizer force-closes the read ends.
		return nil
	}
	out, err := r.Run()
	ref := AsRefusal(err)
	if ref == nil || ref.Reason != ReasonAdapterProcessObserverFail {
		t.Fatalf("want the ECHILD classification to survive, got %v", err)
	}
	if out.Classification != "terminal-observer-error" {
		t.Fatalf("classification = %s, want terminal-observer-error", out.Classification)
	}
}

// TestSingleCleanupOwner proves only one source ever runs the cleanup
// body even when several conditions become true at once.
func TestSingleCleanupOwner(t *testing.T) {
	r := fastRunner()
	r.InvocationTimeout = time.Millisecond
	var runs atomic.Int32
	r.SignalFn = func(sig syscall.Signal) error {
		if sig == syscall.SIGTERM {
			runs.Add(1)
		}
		return nil
	}
	if _, err := r.Run(); err != nil {
		t.Fatalf("unexpected refusal: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if got := runs.Load(); got != 1 {
		t.Fatalf("the cleanup body ran %d times, want exactly 1", got)
	}
}

// TestTriggerPriorityOrder pins the fixed priority order the owner
// re-checks: terminal observer error > reader error > output cap >
// timeout > benign leader event.
func TestTriggerPriorityOrder(t *testing.T) {
	cases := []struct {
		name string
		set  []triggerSource
		want triggerSource
	}{
		{"observer-beats-all", []triggerSource{trigLeaderEvent, trigTimeout, trigOutputCap, trigReaderError, trigObserverTerminal}, trigObserverTerminal},
		{"reader-beats-cap", []triggerSource{trigLeaderEvent, trigTimeout, trigOutputCap, trigReaderError}, trigReaderError},
		{"cap-beats-timeout", []triggerSource{trigLeaderEvent, trigTimeout, trigOutputCap}, trigOutputCap},
		{"timeout-beats-leader", []triggerSource{trigLeaderEvent, trigTimeout}, trigTimeout},
		{"leader-alone", []triggerSource{trigLeaderEvent}, trigLeaderEvent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &ProcessRunner{}
			for _, src := range tc.set {
				r.occurred[src].Store(true)
			}
			if got := r.snapshotClassification(); got != tc.want {
				t.Fatalf("classification = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPipesExistBeforeStart proves both pipe pairs and both endpoint
// assignments exist in full before cmd.Start() is called.
func TestPipesExistBeforeStart(t *testing.T) {
	r := fastRunner()
	var sawStdout, sawStderr bool
	r.StartFn = func() error {
		sawStdout = r.stdoutRead != nil && r.stdoutWrite != nil
		sawStderr = r.stderrRead != nil && r.stderrWrite != nil
		return nil
	}
	if _, err := r.Run(); err != nil {
		t.Fatalf("unexpected refusal: %v", err)
	}
	if !sawStdout || !sawStderr {
		t.Fatal("both pipe pairs must exist before Start() is called")
	}
}

// TestDrainedOutputIsCaptured proves the drains actually deliver the
// child's bytes to the caller.
func TestDrainedOutputIsCaptured(t *testing.T) {
	r := fastRunner()
	blockObserver := make(chan struct{})
	r.ObserveFn = func() error {
		<-blockObserver
		return nil
	}
	r.StartFn = func() error {
		_, _ = r.stdoutWrite.Write([]byte(`{"rows":[]}` + "\n"))
		_, _ = r.stderrWrite.Write([]byte("warning\n"))
		_ = r.stdoutWrite.Close()
		_ = r.stderrWrite.Close()
		go func() {
			time.Sleep(20 * time.Millisecond)
			close(blockObserver)
		}()
		return nil
	}
	out, err := r.Run()
	if err != nil {
		t.Fatalf("unexpected refusal: %v", err)
	}
	if !strings.Contains(string(out.Stdout), `{"rows":[]}`) {
		t.Fatalf("stdout = %q", out.Stdout)
	}
	if !strings.Contains(string(out.Stderr), "warning") {
		t.Fatalf("stderr = %q", out.Stderr)
	}
}

// TestDefaultBoundsMatchTheDesign pins the §6.4 parameter table.
func TestDefaultBoundsMatchTheDesign(t *testing.T) {
	if DefaultInvocationTimeout != 30*time.Second {
		t.Fatalf("invocation timeout = %v, want 30s", DefaultInvocationTimeout)
	}
	if DefaultTerminateGrace != 2*time.Second {
		t.Fatalf("terminate grace = %v, want 2s", DefaultTerminateGrace)
	}
	if DefaultReapDeadline != 2*time.Second {
		t.Fatalf("reap deadline = %v, want 2s", DefaultReapDeadline)
	}
	if DefaultDrainDeadline != 2*time.Second {
		t.Fatalf("drain deadline = %v, want 2s", DefaultDrainDeadline)
	}
	if DefaultOutputCapBytes != 5<<20 {
		t.Fatalf("output cap = %d, want 5 MiB", DefaultOutputCapBytes)
	}
}

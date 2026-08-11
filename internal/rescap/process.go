// Single-owner bounded process finalizer for
// PRD-feature-resource-claims-and-capture-adapters §6.4 / ADR-033 D5.
//
// One unified cleanup sequence serves every invocation, success or
// kill-triggered alike. Five sources can trigger it — the leader-event
// observer, a terminal observer error, a genuine non-EOF pipe-reader
// error, the output-cap-exceeded signal, and the invocation timeout —
// and every one of them attempts the same ownership CAS *before*
// taking any action at all, not merely before signalling but before
// even a single Close or SetReadDeadline call.
//
// Ownership acquisition is a genuine race and is not made
// deterministic. The *reported* classification is: each source also
// records its own occurrence non-blockingly, and the owner re-examines
// all five recorded-occurrence flags in one fixed priority order the
// instant the CAS resolves.

package rescap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Trigger classifications, highest priority first. The priority order
// is fixed and is what makes the reported reason reproducible even
// though which goroutine wins the CAS is arbitrary.
type triggerSource int

const (
	trigObserverTerminal triggerSource = iota
	trigReaderError
	trigOutputCap
	trigTimeout
	trigLeaderEvent
	triggerSourceCount
)

func (t triggerSource) String() string {
	switch t {
	case trigObserverTerminal:
		return "terminal-observer-error"
	case trigReaderError:
		return "pipe-reader-error"
	case trigOutputCap:
		return "output-cap-exceeded"
	case trigTimeout:
		return "invocation-timeout"
	case trigLeaderEvent:
		return "leader-event"
	default:
		return "unknown"
	}
}

// Fixed bounds (§6.4's parameter table). They are struct fields rather
// than constants so tests can drive every branch without sleeping for
// the production durations.
const (
	DefaultInvocationTimeout = 30 * time.Second
	DefaultTerminateGrace    = 2 * time.Second
	DefaultReapDeadline      = 2 * time.Second
	DefaultDrainDeadline     = 2 * time.Second
	DefaultOutputCapBytes    = 5 << 20
	// drainJoinSlack is the defensive margin the join waits beyond the
	// drain deadline. The deadline itself is what unblocks the reads;
	// this only covers scheduling delay between a drain returning and
	// its done-channel closing.
	drainJoinSlack = 500 * time.Millisecond
)

// ProcessRunner runs one child process under the finalizer contract.
//
// Every function-valued field is a seam the tests drive directly; in
// production they are wired to the real exec.Cmd and the real
// build-tagged syscalls.
type ProcessRunner struct {
	Cmd *exec.Cmd

	InvocationTimeout time.Duration
	TerminateGrace    time.Duration
	ReapDeadline      time.Duration
	DrainDeadline     time.Duration
	OutputCap         int64

	// StartFn, WaitFn, ObserveFn and SignalFn default to the real
	// cmd.Start/cmd.Wait, the build-tagged waitid observer, and the
	// build-tagged negative-PGID kill.
	StartFn   func() error
	WaitFn    func() error
	ObserveFn func() error
	SignalFn  func(sig syscall.Signal) error

	// AfterInitialSnapshot runs strictly between the initial priority
	// re-check and the fixed cutoff-drain instant. AC-119 uses it to
	// publish a late ECHILD in exactly that window.
	AfterInitialSnapshot func()

	// OnTimerStopped is a deterministic lifecycle hook invoked exactly
	// once per invocation, as the invocation timer is torn down. Its
	// argument is time.Timer.Stop()'s own report: true when the timer
	// was cancelled before firing (the normal, non-timeout path), false
	// when it had already fired. It exists so a test can assert the
	// timer is genuinely disarmed rather than inferring it from flaky
	// global goroutine counts.
	OnTimerStopped func(stopped bool)

	// BeforeDrainDeadline runs immediately before the shared finalizer
	// applies its read deadline to both owned pipe read ends. It exists
	// so a test can force the SetReadDeadline-failure branch by closing
	// the read ends at exactly that instant.
	BeforeDrainDeadline func()

	// signalCalls counts every -pgid signal attempt, including ones
	// whose errno was tolerated. The ECHILD and Start-failure paths
	// must leave it at zero.
	signalCalls atomic.Int32
	// waitLaunches counts cmd.Wait() launches so AC-120 can prove Wait
	// is never launched before the signal phase completes.
	waitLaunches atomic.Int32
	// signalPhaseDone is set once SIGTERM/grace/SIGKILL have all run.
	signalPhaseDone atomic.Bool
	// waitLaunchedBeforeSignalPhase records a contract violation if it
	// ever happens.
	waitBeforeSignals atomic.Bool

	timerFired atomic.Bool

	drainDeadlineHit [2]atomic.Bool

	occurred     [triggerSourceCount]atomic.Bool
	echild       atomic.Bool
	observerErr  atomic.Value
	readerErr    atomic.Value
	owned        atomic.Bool
	cleanupBegun atomic.Bool

	ownerCh chan struct{}

	stdoutRead, stderrRead   *os.File
	stdoutWrite, stderrWrite *os.File
	// lingeringWriter lets a test retain a write end past Start so the
	// drain genuinely cannot reach EOF, reproducing an escaped
	// descendant. Production never sets it.
	lingeringWriter      *os.File
	stdoutBuf, stderrBuf []byte
	drainDone            [2]chan struct{}

	budget atomic.Int64
}

// ProcessOutcome is everything the finalizer establishes about one
// invocation.
type ProcessOutcome struct {
	Stdout []byte
	Stderr []byte

	// Classification is the trigger the fixed priority order selected.
	Classification string
	// Primary is the single reported refusal, or nil when the
	// invocation completed benignly.
	Primary *Refusal
	// Reaped reports whether cmd.Wait() returned inside its bound.
	Reaped bool
	// WaitErr is cmd.Wait()'s own error when the reap completed.
	WaitErr error
	// Diagnostics collects every demoted, non-primary failure. They
	// are local-only and never tracked.
	Diagnostics []string
	// SignalCalls is the number of -pgid signal attempts made.
	SignalCalls int
	// DrainGoroutines is the number of drain goroutines spawned (0 on
	// the Start-failure path).
	DrainGoroutines int
	// WaitLaunches counts cmd.Wait() launches.
	WaitLaunches int
	// WaitLaunchedBeforeSignals records an exclusive-waiter contract
	// violation, and must always be false.
	WaitLaunchedBeforeSignals bool
	// TimerFired reports whether the invocation timer actually fired.
	TimerFired bool
	// RetentionBudgetRemaining is the unclaimed portion of the shared
	// cap-plus-one retention budget. Zero means the cap was reached.
	RetentionBudgetRemaining int64
}

// StartFailureError marks a cmd.Start() failure. It is deliberately a
// distinct type: the Start-failure path is wholly separate from both
// finalizers and never reaches either.
type StartFailureError struct{ Err error }

// Error satisfies the error interface.
func (e *StartFailureError) Error() string { return fmt.Sprintf("start failed: %v", e.Err) }

// Unwrap exposes the underlying cause.
func (e *StartFailureError) Unwrap() error { return e.Err }

func (p *ProcessRunner) applyDefaults() {
	if p.InvocationTimeout == 0 {
		p.InvocationTimeout = DefaultInvocationTimeout
	}
	if p.TerminateGrace == 0 {
		p.TerminateGrace = DefaultTerminateGrace
	}
	if p.ReapDeadline == 0 {
		p.ReapDeadline = DefaultReapDeadline
	}
	if p.DrainDeadline == 0 {
		p.DrainDeadline = DefaultDrainDeadline
	}
	if p.OutputCap == 0 {
		p.OutputCap = DefaultOutputCapBytes
	}
	if p.StartFn == nil && p.Cmd != nil {
		p.StartFn = p.Cmd.Start
	}
	if p.WaitFn == nil && p.Cmd != nil {
		p.WaitFn = p.Cmd.Wait
	}
	if p.ObserveFn == nil {
		p.ObserveFn = func() error {
			if p.Cmd == nil || p.Cmd.Process == nil {
				return errors.New("no started process to observe")
			}
			return observeLeaderEvent(p.Cmd.Process.Pid)
		}
	}
	if p.SignalFn == nil {
		p.SignalFn = func(sig syscall.Signal) error {
			if p.Cmd == nil || p.Cmd.Process == nil {
				return errors.New("no started process to signal")
			}
			return signalProcessGroup(p.Cmd.Process.Pid, sig)
		}
	}
	p.budget.Store(p.OutputCap + 1)
	p.ownerCh = make(chan struct{}, 1)
}

// Run executes the whole contract and returns once the finalizer has
// completed. It never blocks longer than the configured bounds.
func (p *ProcessRunner) Run() (ProcessOutcome, error) {
	p.applyDefaults()

	// Both pipe pairs, and both endpoint assignments, exist in full
	// before cmd.Start() is ever called.
	var err error
	p.stdoutRead, p.stdoutWrite, err = os.Pipe()
	if err != nil {
		return ProcessOutcome{}, Internal(ReasonAdapterOutputReadFailed, "creating stdout pipe: %v", err)
	}
	p.stderrRead, p.stderrWrite, err = os.Pipe()
	if err != nil {
		_ = p.stdoutRead.Close()
		_ = p.stdoutWrite.Close()
		return ProcessOutcome{}, Internal(ReasonAdapterOutputReadFailed, "creating stderr pipe: %v", err)
	}
	if p.Cmd != nil {
		p.Cmd.Stdout = p.stdoutWrite
		p.Cmd.Stderr = p.stderrWrite
		if p.Cmd.SysProcAttr == nil {
			p.Cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		setProcessGroup(p.Cmd.SysProcAttr)
	}

	if startErr := p.StartFn(); startErr != nil {
		// Start-failure carve-out: all four parent-held endpoints are
		// closed directly and synchronously by this same goroutine.
		// Zero drain goroutines, zero ownership CAS attempts, zero
		// -pgid signals, and cmd.Wait() is never invoked.
		_ = p.stdoutRead.Close()
		_ = p.stdoutWrite.Close()
		_ = p.stderrRead.Close()
		_ = p.stderrWrite.Close()
		return ProcessOutcome{
			Classification:  "start-failure",
			SignalCalls:     int(p.signalCalls.Load()),
			DrainGoroutines: 0,
			WaitLaunches:    int(p.waitLaunches.Load()),
		}, &StartFailureError{Err: startErr}
	}

	// The child's dup'd fds keep the pipes alive; this process drops
	// its own write-end references so an EOF is actually reachable.
	_ = p.stdoutWrite.Close()
	_ = p.stderrWrite.Close()

	p.drainDone[0] = make(chan struct{})
	p.drainDone[1] = make(chan struct{})
	go p.drain(0, p.stdoutRead, &p.stdoutBuf)
	go p.drain(1, p.stderrRead, &p.stderrBuf)

	go func() {
		obsErr := p.ObserveFn()
		if obsErr != nil {
			p.observerErr.Store(errHolder{obsErr})
			if isNoChildError(obsErr) {
				p.echild.Store(true)
			}
			p.record(trigObserverTerminal)
			return
		}
		p.record(trigLeaderEvent)
	}()

	// The invocation timer fires its trigger directly from the runtime
	// timer goroutine rather than from a dedicated receiver blocked on
	// timer.C. Rev-0 used `go func(){ <-timer.C; ... }()`, which leaks
	// that receiver forever on every successful invocation: Stop()
	// prevents the send, so the receiver blocks on a channel nothing
	// will ever write to. AfterFunc has no receiver to strand, and
	// Stop() reports whether the callback was cancelled before firing,
	// which the lifecycle hook below exposes to tests.
	timer := time.AfterFunc(p.InvocationTimeout, func() {
		p.timerFired.Store(true)
		p.record(trigTimeout)
	})
	// Teardown is deferred (not run at ownership resolution) so the
	// timer stays armed through the priority re-check, exactly as
	// rev-0 sequenced it.
	defer func() {
		if h := p.OnTimerStopped; h != nil {
			h(timer.Stop())
			return
		}
		timer.Stop()
	}()

	<-p.ownerCh
	return p.finalize()
}

// errHolder boxes an error so atomic.Value never sees two concrete
// types.
type errHolder struct{ err error }

// record sets a source's recorded-occurrence flag and then attempts
// the ownership CAS. Both steps happen for every source, including
// both terminal-observer-error branches: terminal observer errors are
// not a side path outside the guard.
func (p *ProcessRunner) record(src triggerSource) {
	p.occurred[src].Store(true)
	if p.owned.CompareAndSwap(false, true) {
		p.ownerCh <- struct{}{}
	}
}

// snapshotClassification re-examines all five recorded-occurrence
// flags in the fixed priority order and returns the highest-priority
// one currently set.
func (p *ProcessRunner) snapshotClassification() triggerSource {
	for src := trigObserverTerminal; src < triggerSourceCount; src++ {
		if p.occurred[src].Load() {
			return src
		}
	}
	return trigLeaderEvent
}

// finalize runs the classification selection, the cutoff drain, and
// whichever finalizer the resulting classification selects.
func (p *ProcessRunner) finalize() (ProcessOutcome, error) {
	classification := p.snapshotClassification()

	if p.AfterInitialSnapshot != nil {
		p.AfterInitialSnapshot()
	}

	// Cutoff drain: exactly one additional deterministic read of the
	// terminal-observer-error flag, strictly after the initial
	// classification is selected and strictly before the shared
	// finalizer's first -pgid syscall. A genuine ECHILD found here
	// overrides any lower-priority classification, guaranteeing zero
	// negative-PGID signals have been sent up to this point.
	if classification != trigObserverTerminal && p.occurred[trigObserverTerminal].Load() && p.echild.Load() {
		classification = trigObserverTerminal
	}

	// The cleanup-initiated flag is set the instant ownership is
	// acted on: before any SetReadDeadline, before any Close, before
	// the ECHILD finalizer's force-close. From here on, a drain
	// goroutine's non-EOF error is attributable to the owner's own
	// action and only unblocks that goroutine's join.
	p.cleanupBegun.Store(true)

	if classification == trigObserverTerminal && p.echild.Load() {
		return p.finalizeECHILD()
	}
	return p.finalizeShared(classification)
}

// finalizeECHILD implements the ECHILD-specific finalizer: no -pgid
// signal of any kind, and cmd.Wait() is never called. The kernel no
// longer considers this PID a child, so its numeric PID/PGID could
// already have been recycled to an unrelated process group and
// signalling it would risk hitting that group entirely.
func (p *ProcessRunner) finalizeECHILD() (ProcessOutcome, error) {
	_ = p.stdoutRead.Close()
	_ = p.stderrRead.Close()
	// Join-only: both read ends are already closed, so there is no
	// open descriptor left to set a deadline on, and this branch never
	// attempts SetReadDeadline.
	p.joinDrains(p.DrainDeadline)

	primary := Internal(ReasonAdapterProcessObserverFail,
		"the leader-event observer returned ECHILD; no signal was sent and the leader was not reaped")
	out := p.outcome(trigObserverTerminal, primary)
	out.Diagnostics = append(out.Diagnostics,
		"ECHILD finalizer: leader and any descendants remain alive, unsignaled and unreaped (disclosed residual)")
	return out, primary
}

// finalizeShared implements the shared bounded finalizer used by every
// other classification.
func (p *ProcessRunner) finalizeShared(classification triggerSource) (ProcessOutcome, error) {
	var diagnostics []string
	var phaseErr *Refusal

	recordPhase := func(r *Refusal) {
		if phaseErr == nil {
			phaseErr = r
		} else {
			diagnostics = append(diagnostics, r.Error())
		}
	}

	// Phase 1-3: signals. Any untolerated errno is recorded without
	// halting the sequence.
	p.signalCalls.Add(1)
	if err := p.SignalFn(syscall.SIGTERM); err != nil {
		recordPhase(Internal(ReasonAdapterGroupSignalFailed, "SIGTERM to the process group: %v", err))
	}
	time.Sleep(p.TerminateGrace)
	p.signalCalls.Add(1)
	if err := p.SignalFn(syscall.SIGKILL); err != nil {
		recordPhase(Internal(ReasonAdapterGroupSignalFailed, "SIGKILL to the process group: %v", err))
	}
	p.signalPhaseDone.Store(true)

	// Phase 4: bounded reap. cmd.Wait() is launched here and only
	// here — strictly after the signal phase completes, which is the
	// exclusive-waiter invariant the cutoff-drain guarantee rests on.
	waitCh := make(chan error, 1)
	p.waitLaunches.Add(1)
	if !p.signalPhaseDone.Load() {
		p.waitBeforeSignals.Store(true)
	}
	go func() {
		err := p.WaitFn()
		select {
		case waitCh <- err:
		default:
		}
	}()
	reaped := false
	var waitErr error
	select {
	case waitErr = <-waitCh:
		reaped = true
	case <-time.After(p.ReapDeadline):
		recordPhase(Internal(ReasonAdapterReapTimeout,
			"cmd.Wait() did not return within %s; proceeding without a second Wait", p.ReapDeadline))
		diagnostics = append(diagnostics,
			"reap timeout: the abandoned cmd.Wait() goroutine is never joined or cancelled (disclosed residual)")
		if classification != trigLeaderEvent {
			diagnostics = append(diagnostics,
				"reap timeout: the leader-event observer goroutine may also remain blocked in its own waitid call (disclosed residual)")
		}
	}

	// Phase 5: bounded pipe-drain finalization, normatively after
	// phase 4 resolves one way or the other.
	if h := p.BeforeDrainDeadline; h != nil {
		h()
	}
	deadline := time.Now().Add(p.DrainDeadline)
	deadlineErr := false
	if err := p.stdoutRead.SetReadDeadline(deadline); err != nil {
		deadlineErr = true
	}
	if err := p.stderrRead.SetReadDeadline(deadline); err != nil {
		deadlineErr = true
	}
	switch {
	case deadlineErr:
		_ = p.stdoutRead.Close()
		_ = p.stderrRead.Close()
		p.joinDrains(p.DrainDeadline)
		recordPhase(Internal(ReasonAdapterOutputReadFailed,
			"SetReadDeadline failed on an owned pipe read end"))
	default:
		// The deadline is what unblocks a drain still waiting on a
		// write end an escaped descendant holds open, so BOTH drains
		// always return here — one via io.EOF, the other possibly via
		// os.ErrDeadlineExceeded. The join therefore cannot be the
		// timeout detector (rev-0 measured the join and so could never
		// observe an expiry); the drains themselves record which
		// terminal condition ended them, and that is what distinguishes
		// a clean drain from an expired one.
		joined := p.joinDrains(p.DrainDeadline + drainJoinSlack)
		expired := p.drainDeadlineHit[0].Load() || p.drainDeadlineHit[1].Load()
		_ = p.stdoutRead.Close()
		_ = p.stderrRead.Close()
		if !joined {
			p.joinDrains(p.DrainDeadline)
		}
		if expired || !joined {
			recordPhase(Refuse(ReasonAdapterDrainTimeout,
				"a pipe drain did not complete within %s; publishing nothing", p.DrainDeadline))
		}
	}

	// Primary-error selection: a non-benign classification's own named
	// refusal always wins; a benign entry takes the first cleanup-phase
	// failure in the fixed signal -> reap -> drain walk order.
	var primary *Refusal
	switch classification {
	case trigObserverTerminal:
		primary = Internal(ReasonAdapterProcessObserverFail,
			"the leader-event observer failed terminally: %v", p.storedObserverErr())
	case trigReaderError:
		primary = Internal(ReasonAdapterOutputReadFailed,
			"a pipe drain failed before cleanup began: %v", p.storedReaderErr())
	case trigOutputCap:
		primary = Refuse(ReasonResourceLimitExceeded,
			"combined stdout+stderr exceeded the %d-byte cap; no partial output is parsed or scanned", p.OutputCap)
	default:
		primary = phaseErr
	}
	if primary != nil && phaseErr != nil && primary != phaseErr {
		diagnostics = append(diagnostics, phaseErr.Error())
	}

	out := p.outcome(classification, primary)
	out.Reaped = reaped
	out.WaitErr = waitErr
	out.Diagnostics = append(out.Diagnostics, diagnostics...)
	if primary != nil {
		return out, primary
	}
	return out, nil
}

func (p *ProcessRunner) storedObserverErr() error {
	if v, ok := p.observerErr.Load().(errHolder); ok {
		return v.err
	}
	return nil
}

func (p *ProcessRunner) storedReaderErr() error {
	if v, ok := p.readerErr.Load().(errHolder); ok {
		return v.err
	}
	return nil
}

func (p *ProcessRunner) outcome(classification triggerSource, primary *Refusal) ProcessOutcome {
	return ProcessOutcome{
		Stdout:                    p.stdoutBuf,
		Stderr:                    p.stderrBuf,
		TimerFired:                p.timerFired.Load(),
		RetentionBudgetRemaining:  p.budget.Load(),
		Classification:            classification.String(),
		Primary:                   primary,
		SignalCalls:               int(p.signalCalls.Load()),
		DrainGoroutines:           2,
		WaitLaunches:              int(p.waitLaunches.Load()),
		WaitLaunchedBeforeSignals: p.waitBeforeSignals.Load(),
	}
}

// joinDrains waits up to timeout for both drain goroutines and reports
// whether both completed. It is the bounded-join helper every
// forced-close branch uses before returning or releasing the lock.
func (p *ProcessRunner) joinDrains(timeout time.Duration) bool {
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(2)
	for i := range p.drainDone {
		ch := p.drainDone[i]
		go func() {
			defer wg.Done()
			<-ch
		}()
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// claimRetention atomically reserves up to n bytes of the shared
// retention budget and reports how many of them may actually be
// appended, plus whether this claim exhausted the budget.
//
// The budget starts at cap+1 and is shared by both drains, so the
// combined retained stdout+stderr length can never exceed cap+1 — not
// cap+1 *per stream*, and not cap+1 *per read*. The CAS loop is what
// makes two concurrent chunks unable to overshoot: each competitor
// re-reads the remaining budget and only ever takes what is left.
//
// Exhaustion is reported exactly once (by whichever claim consumes the
// final byte), so the output-cap trigger is recorded once rather than
// on every subsequent read.
func (p *ProcessRunner) claimRetention(n int) (accept int, exhausted bool) {
	if n <= 0 {
		return 0, false
	}
	for {
		remaining := p.budget.Load()
		if remaining <= 0 {
			return 0, false
		}
		take := int64(n)
		if take > remaining {
			take = remaining
		}
		if p.budget.CompareAndSwap(remaining, remaining-take) {
			return int(take), remaining-take == 0
		}
	}
}

// drain reads one pipe under the shared cap-plus-one retention budget.
//
// Reading continues past the cap so the child can always finish writing
// and exit cleanly, but **retention stops**: once the shared budget is
// exhausted, further bytes are read and discarded rather than appended.
// Rev-0 appended every byte it read, so a runaway child could pin
// arbitrarily much memory under a 5 MiB cap; rev-1 bounds the retained
// total at exactly cap+1 bytes across both streams.
//
// The drain goroutines never close anything: only the control cleanup
// function does.
func (p *ProcessRunner) drain(idx int, r *os.File, dst *[]byte) {
	defer close(p.drainDone[idx])
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			accept, exhausted := p.claimRetention(n)
			if accept > 0 {
				*dst = append(*dst, buf[:accept]...)
			}
			if exhausted {
				// The cap-plus-one'th byte has now actually been read.
				// Everything after this point is drained and discarded;
				// no partial or truncated output is ever handed to the
				// parser or the redaction scanner.
				p.record(trigOutputCap)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errors.Is(err, os.ErrDeadlineExceeded) {
				// The owner's own drain deadline expired while this
				// drain was still blocked, which means a writer — an
				// escaped descendant, typically — still held the pipe
				// open. Record it so the finalizer can tell an expired
				// drain apart from a clean one; the error itself is
				// owner-induced and is never a fresh trigger.
				p.drainDeadlineHit[idx].Store(true)
				return
			}
			if p.cleanupBegun.Load() {
				// Attributable to the owner's own SetReadDeadline or
				// force-close: a join-completion signal, never a fresh
				// trigger.
				return
			}
			p.readerErr.Store(errHolder{err})
			p.record(trigReaderError)
			return
		}
	}
}

//go:build linux || darwin

// Rev-2 mechanism tests: exact host errnos during the private copy and
// the descriptor-identity ABA fixture.
//
// These are separated from hardening_test.go because each replaces a
// rev-1 test the review found insufficiently specific: generic
// directory/permission failures instead of the named ENOSPC/EIO, and a
// replacement fixture that the redundant pathname recheck also caught.

package rescap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// failingCopyTarget wraps the real private-copy destination and fails
// at one precise step with one precise errno. Everything else passes
// through to the real file, so the partial copy genuinely exists on
// disk when the failure fires and the cleanup path is really exercised.
type failingCopyTarget struct {
	inner       privateCopyTarget
	failWrite   error
	failSync    error
	bytesBefore int
	written     int
}

func (f *failingCopyTarget) Write(p []byte) (int, error) {
	if f.failWrite != nil && f.written >= f.bytesBefore {
		return 0, f.failWrite
	}
	n, err := f.inner.Write(p)
	f.written += n
	return n, err
}

func (f *failingCopyTarget) Sync() error {
	if f.failSync != nil {
		return f.failSync
	}
	return f.inner.Sync()
}

func (f *failingCopyTarget) Chmod(mode os.FileMode) error { return f.inner.Chmod(mode) }
func (f *failingCopyTarget) Stat() (os.FileInfo, error)   { return f.inner.Stat() }
func (f *failingCopyTarget) Close() error                 { return f.inner.Close() }

// TestPrivateCopyExactHostErrnosCleanUpAndStartNothing covers matrix row
// 174 with the exact errnos §6.1 step 4 names: ENOSPC and EIO from the
// streamed copy and from Sync.
//
// Rev-1 used a directory-read failure and an unwritable scratch
// directory, which the review correctly rejected as not-the-named
// condition. Each case here injects one specific syscall errno at one
// specific step and asserts the full contract: adapter-copy-failed
// (exit 1, a host fault rather than a policy refusal), the partial
// private copy removed, and zero processes started.
func TestPrivateCopyExactHostErrnosCleanUpAndStartNothing(t *testing.T) {
	cases := []struct {
		name      string
		failWrite error
		failSync  error
		partial   int
	}{
		{name: "enospc-mid-write", failWrite: syscall.ENOSPC, partial: 16},
		{name: "enospc-on-first-write", failWrite: syscall.ENOSPC, partial: 0},
		{name: "eio-mid-write", failWrite: syscall.EIO, partial: 16},
		{name: "eio-on-sync", failSync: syscall.EIO},
		{name: "enospc-on-sync", failSync: syscall.ENOSPC},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srcDir := t.TempDir()
			scratch := t.TempDir()
			// A source large enough that a mid-write failure leaves a
			// genuinely partial copy behind.
			body := "#!/bin/sh\n" + strings.Repeat("# padding\n", 4096) + "exit 0\n"
			path := writeFixtureExecutable(t, srcDir, "dolt", body)
			digest, err := HashExecutableDescriptor(path)
			if err != nil {
				t.Fatalf("hash: %v", err)
			}

			var observed *failingCopyTarget
			restore := SetPrivateCopyTargetWrapperForTest(func(inner privateCopyTarget) privateCopyTarget {
				observed = &failingCopyTarget{
					inner:       inner,
					failWrite:   tc.failWrite,
					failSync:    tc.failSync,
					bytesBefore: tc.partial,
				}
				return observed
			})
			defer restore()

			copyFile, err := MakeVerifiedPrivateCopy(path, scratch, digest)
			if copyFile != nil {
				copyFile.Remove()
				t.Fatal("no private copy may be returned when the host write fails")
			}
			if observed == nil {
				t.Fatal("the injection seam was never reached; the test proved nothing")
			}

			r := AsRefusal(err)
			if r == nil || r.Reason != ReasonAdapterCopyFailed || r.Code != ExitInternal {
				t.Fatalf("want adapter-copy-failed exit 1, got %v", err)
			}
			// The exact injected errno must survive into the message,
			// proving the failure really was the named host fault.
			wantErrno := tc.failWrite
			if wantErrno == nil {
				wantErrno = tc.failSync
			}
			if !strings.Contains(r.Error(), wantErrno.Error()) {
				t.Fatalf("refusal %q does not carry the injected errno %v", r.Error(), wantErrno)
			}

			// The partial copy is removed, so the scratch tree is clean.
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
	}
}

// TestPrivateCopyHostFailureStartsNoProcess proves the host-fault path
// never reaches an invocation, using the engine so a real process start
// would be observable through the fixture's own side effect.
func TestPrivateCopyHostFailureStartsNoProcess(t *testing.T) {
	fake := newDoltFixture(t, `{"rows":[]}`, "", 0)
	f := newDoltEngineFixture(t, fake)
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()
	restoreCopy := SetPrivateCopyTargetWrapperForTest(func(inner privateCopyTarget) privateCopyTarget {
		return &failingCopyTarget{inner: inner, failWrite: syscall.ENOSPC}
	})
	defer restoreCopy()

	_, err := f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonAdapterCopyFailed || r.Code != ExitInternal {
		t.Fatalf("want adapter-copy-failed exit 1, got %v", err)
	}
	if _, statErr := os.Stat(fake.ObservedAt); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("a host copy failure must start no process at all")
	}
	if _, statErr := os.Stat(f.Store.ResourceCurrentPath("model-picker")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("a host copy failure must publish nothing")
	}
}

// TestPrivateCopySucceedsWithoutInjection is the control: the seam is
// inert unless a test installs it.
func TestPrivateCopySucceedsWithoutInjection(t *testing.T) {
	srcDir := t.TempDir()
	scratch := t.TempDir()
	path := writeFixtureExecutable(t, srcDir, "dolt", "#!/bin/sh\nexit 0\n")
	digest, err := HashExecutableDescriptor(path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	copyFile, err := MakeVerifiedPrivateCopy(path, scratch, digest)
	if err != nil {
		t.Fatalf("the production path must succeed with no wrapper installed: %v", err)
	}
	defer copyFile.Remove()
	if copyFile.Digest != digest {
		t.Fatal("the verified copy must carry the streamed digest")
	}
}

// TestSameFileDescriptorGateIsTheLoadBearingCheck covers the
// descriptor-identity mechanism specifically (ADR-033 §9.1 step 5).
//
// Rev-1's version swapped the entry and left it swapped, so the later
// redundant pathname re-Lstat also caught it — deleting the os.SameFile
// comparison would still have produced a refusal and the test would
// still have passed. It proved the gate refuses, not that the
// descriptor check is what refuses.
//
// This is an ABA fixture instead. The pathname is swapped for a
// different inode before the open, and RESTORED at the seam that sits
// strictly after the descriptor comparison and strictly before the
// pathname recheck. So:
//
//   - with the production os.SameFile check, the refusal has already
//     happened by the time the restore seam would run (it never fires)
//     and the wrong descriptor is rejected;
//   - without it, the restore makes the pathname recheck observe the
//     ORIGINAL entry, the gate accepts, and the caller is handed a
//     descriptor on the attacker's file.
//
// A scratch mutation probe deleting the os.SameFile guard was run
// against this test and it failed by accepting the swapped descriptor,
// confirming the mechanism is what is under test.
func TestSameFileDescriptorGateIsTheLoadBearingCheck(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "config/target.env", "original\n")
	abs := filepath.Join(root, "config", "target.env")
	stash := filepath.Join(root, "config", ".original.env")

	var swapped, restored atomic.Int32
	restoreSwap := SetBeforeGatedOpenForTest(func(p string) {
		if p != abs || !swapped.CompareAndSwap(0, 1) {
			return
		}
		// Move the original aside (preserving its inode) and put a
		// DIFFERENT inode at the same pathname.
		if err := os.Rename(abs, stash); err != nil {
			t.Errorf("stash original: %v", err)
			return
		}
		if err := os.WriteFile(abs, []byte("attacker\n"), 0o644); err != nil {
			t.Errorf("plant replacement: %v", err)
		}
	})
	defer restoreSwap()

	restoreSeam := SetAfterDescriptorIdentityCheckForTest(func(p string) {
		if p != abs || !restored.CompareAndSwap(0, 1) {
			return
		}
		// Put the ORIGINAL inode back before the pathname recheck can
		// look. Only the descriptor comparison can have noticed.
		if err := os.Remove(abs); err != nil {
			t.Errorf("remove replacement: %v", err)
			return
		}
		if err := os.Rename(stash, abs); err != nil {
			t.Errorf("restore original: %v", err)
		}
	})
	defer restoreSeam()

	gated, err := GatePath(root, "config/target.env")
	if gated != nil {
		defer func() { _ = gated.Close() }()
	}

	if swapped.Load() != 1 {
		t.Fatal("the pre-open swap never fired; the test proved nothing")
	}
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonPathReplacedDuringOpen || r.Code != ExitRefusal {
		t.Fatalf("want path-replaced-during-open exit 3, got %v (gated=%v)", err, gated)
	}
	if restored.Load() != 0 {
		t.Fatal("the descriptor comparison did not refuse before the restore seam; " +
			"the pathname recheck, not os.SameFile, would have been the deciding check")
	}
}

// TestGatedOpenAcceptsAnUnreplacedEntry is the control for the test
// above: with both seams installed but performing no swap, the
// identical gate accepts the path. Without this, a gate that refused
// everything would also pass.
func TestGatedOpenAcceptsAnUnreplacedEntry(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "config/target.env", "original\n")
	var beforeFired, afterFired atomic.Int32
	restoreBefore := SetBeforeGatedOpenForTest(func(string) { beforeFired.Add(1) })
	defer restoreBefore()
	restoreAfter := SetAfterDescriptorIdentityCheckForTest(func(string) { afterFired.Add(1) })
	defer restoreAfter()

	gated, err := GatePath(root, "config/target.env")
	if err != nil {
		t.Fatalf("an unreplaced entry must be accepted: %v", err)
	}
	if err := gated.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if beforeFired.Load() == 0 {
		t.Fatal("the pre-open seam is not installed on the real gate path")
	}
	if afterFired.Load() == 0 {
		t.Fatal("the post-descriptor-check seam is not installed on the real gate path")
	}
}

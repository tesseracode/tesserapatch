//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestAcquireCreatesNoResidueAndReleasePermitsReacquire(t *testing.T) {
	workspace := t.TempDir()
	before := directoryNames(t, workspace)

	authority, err := Acquire(workspace)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if got := directoryNames(t, workspace); !equalStrings(got, before) {
		t.Fatalf("acquisition changed workspace: before=%v after=%v", before, got)
	}

	contender, err := Acquire(workspace)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("contender unexpectedly acquired authority")
	}
	assertCode(t, err, CodeTransactionInProgress)

	if err := authority.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	reacquired, err := Acquire(workspace)
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	if err := reacquired.Release(); err != nil {
		t.Fatalf("release reacquired authority: %v", err)
	}
	if got := directoryNames(t, workspace); !equalStrings(got, before) {
		t.Fatalf("release left residue: before=%v after=%v", before, got)
	}
}

func TestAuthorityLifetimeSurvivesGCAndCallerLocals(t *testing.T) {
	workspace := t.TempDir()
	authority := acquireInNestedScope(t, workspace)

	for range 8 {
		runtime.GC()
	}
	contender, err := Acquire(workspace)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("GC released a live authority")
	}
	assertCode(t, err, CodeTransactionInProgress)

	if err := authority.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestAliasContendsOnSameDirectoryInode(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(workspace, alias); err != nil {
		t.Fatal(err)
	}

	authority, err := Acquire(workspace)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = authority.Release() }()

	contender, err := Acquire(alias)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("alias unexpectedly acquired the held inode")
	}
	assertCode(t, err, CodeTransactionInProgress)
}

func TestOriginalPathValidationDistinguishesPublicationBoundary(t *testing.T) {
	tests := []struct {
		name             string
		afterPublication bool
		want             Code
	}{
		{name: "before", want: CodeWorkspaceRootChanged},
		{name: "after", afterPublication: true, want: CodeWorkspaceRootReplacedAfterPublication},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()
			workspace := filepath.Join(parent, "workspace")
			moved := filepath.Join(parent, "moved")
			if err := os.Mkdir(workspace, 0o700); err != nil {
				t.Fatal(err)
			}
			authority, err := Acquire(workspace)
			if err != nil {
				t.Fatalf("Acquire: %v", err)
			}
			defer func() { _ = authority.Release() }()

			if err := os.Rename(workspace, moved); err != nil {
				t.Fatal(err)
			}
			assertCode(t, authority.ValidateOriginalPath(test.afterPublication), test.want)
			if err := authority.WithRoot(func(root *os.Root) error {
				_, statErr := root.Stat(".")
				return statErr
			}); err != nil {
				t.Fatalf("held root did not remain usable after rename: %v", err)
			}
		})
	}
}

func TestOriginalPathMissingAndRecreatedAreRefused(t *testing.T) {
	for _, recreate := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "recreated"}[recreate], func(t *testing.T) {
			parent := t.TempDir()
			workspace := filepath.Join(parent, "workspace")
			moved := filepath.Join(parent, "moved")
			if err := os.Mkdir(workspace, 0o700); err != nil {
				t.Fatal(err)
			}
			authority, err := Acquire(workspace)
			if err != nil {
				t.Fatalf("Acquire: %v", err)
			}
			defer func() { _ = authority.Release() }()
			if err := os.Rename(workspace, moved); err != nil {
				t.Fatal(err)
			}
			if recreate {
				if err := os.Mkdir(workspace, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			assertCode(t, authority.ValidateOriginalPath(false), CodeWorkspaceRootChanged)
			assertCode(t, authority.ValidateOriginalPath(true), CodeWorkspaceRootReplacedAfterPublication)
		})
	}
}

func TestReleaseExactlyOnceAndControlAfterCloseFailClosed(t *testing.T) {
	authority, err := Acquire(t.TempDir())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := authority.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	assertCode(t, authority.Release(), CodeDirectoryFlockUnavailable)
	assertCode(t, authority.WithRoot(func(*os.Root) error { return nil }), CodeDirectoryFlockUnavailable)
	if err := lockHeldDirectory(authority.directory); err == nil {
		t.Fatal("Control after release unexpectedly succeeded")
	}
}

func TestCloseDuringInFlightControlFailsClosedWithoutLostLockClaim(t *testing.T) {
	workspace := t.TempDir()
	authority, err := Acquire(workspace)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	raw, err := authority.directory.SyscallConn()
	if err != nil {
		_ = authority.Release()
		t.Fatalf("SyscallConn: %v", err)
	}
	controlEntered := make(chan struct{})
	releaseControl := make(chan struct{})
	controlDone := make(chan error, 1)
	go func() {
		controlDone <- raw.Control(func(uintptr) {
			close(controlEntered)
			<-releaseControl
		})
	}()
	select {
	case <-controlEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("Control callback did not start")
	}

	closeStarted := make(chan struct{})
	closeDone := make(chan error, 1)
	go func() {
		close(closeStarted)
		closeDone <- authority.directory.Close()
	}()
	<-closeStarted
	runtime.Gosched()

	var closeErr error
	closeReturned := false
	select {
	case closeErr = <-closeDone:
		closeReturned = true
	case <-time.After(50 * time.Millisecond):
	}

	contender, err := Acquire(workspace)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("Close during in-flight Control allowed a contender to claim the held inode")
	}
	assertCode(t, err, CodeTransactionInProgress)

	close(releaseControl)
	select {
	case err := <-controlDone:
		if err != nil {
			t.Fatalf("Control: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Control deadlocked after callback release")
	}
	if !closeReturned {
		select {
		case closeErr = <-closeDone:
		case <-time.After(5 * time.Second):
			t.Fatal("Close deadlocked after callback release")
		}
	}
	if closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	err = authority.Release()
	assertCode(t, err, CodeDirectoryFlockUnavailable)
	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code == CodeTransactionInProgress {
		t.Fatalf("concurrent-close misuse reported contention: %v", err)
	}
}

func TestOnlyWouldBlockAndAgainAreContention(t *testing.T) {
	for _, lockErr := range []error{syscall.EWOULDBLOCK, syscall.EAGAIN, syscall.EPERM} {
		t.Run(lockErr.Error(), func(t *testing.T) {
			ops := defaultAuthorityOps
			ops.lock = func(*os.File) error { return lockErr }
			authority, err := acquireWithOps(t.TempDir(), ops)
			if authority != nil {
				_ = authority.Release()
				t.Fatal("injected lock error acquired authority")
			}
			if errors.Is(lockErr, syscall.EWOULDBLOCK) || errors.Is(lockErr, syscall.EAGAIN) {
				assertCode(t, err, CodeTransactionInProgress)
			} else {
				assertCode(t, err, CodeDirectoryFlockUnavailable)
			}
		})
	}
}

func acquireInNestedScope(t *testing.T, workspace string) *WorkspaceAuthority {
	t.Helper()
	authority, err := Acquire(workspace)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	return authority
}

func directoryNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func assertCode(t *testing.T, err error, want Code) {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatalf("expected intentlock.Error code %q, got %T %v", want, err, err)
	}
	if typed.Code != want {
		t.Fatalf("error code = %q, want %q (%v)", typed.Code, want, err)
	}
}

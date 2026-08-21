//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"os"
	"testing"
)

func s7InstallLockAttemptCounter(t *testing.T) *int {
	t.Helper()
	previous := failLockAcquire
	attempts := 0
	failLockAcquire = func() error {
		attempts++
		return nil
	}
	t.Cleanup(func() { failLockAcquire = previous })
	return &attempts
}

func s7AssertDeniedClassStopsBeforeFlock(t *testing.T, class string, attempts *int) {
	t.Helper()
	before := *attempts
	authority, err := AcquireWithFilesystemClassifier(
		t.TempDir(),
		func(*os.File) (string, bool, error) { return class, true, nil },
	)
	if authority != nil {
		_ = authority.Release()
		t.Fatal("denied filesystem returned authority")
	}
	assertCode(t, err, CodeLockFilesystemUnsupported)
	typed, ok := err.(*Error)
	if !ok || typed.Class != class {
		t.Fatalf("denied class error = %#v, want class %q", err, class)
	}
	if *attempts != before {
		t.Fatalf("denied class %q reached flock", class)
	}
}

func s7AssertNonDeniedClassStillRequiresRealFlock(t *testing.T, class string, attempts *int) {
	t.Helper()
	workspace := t.TempDir()
	before := *attempts
	authority, err := AcquireWithFilesystemClassifier(
		workspace,
		func(*os.File) (string, bool, error) { return class, false, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if *attempts != before+1 {
		t.Fatalf("non-denied class %q reached flock %d time(s), want one", class, *attempts-before)
	}
	contender, err := Acquire(workspace)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("non-denied classification bypassed the real directory flock")
	}
	assertCode(t, err, CodeTransactionInProgress)
	if *attempts != before+2 {
		t.Fatalf("real contender for %q reached flock %d total time(s), want two", class, *attempts-before)
	}
	if err := authority.Release(); err != nil {
		t.Fatal(err)
	}
}

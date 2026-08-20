package intentlock

import (
	"errors"
	"syscall"
	"testing"
)

func TestNamedInjectionSeamsReachAuthorityBoundaries(t *testing.T) {
	if !AuthoritySupported {
		t.Skip("workspace authority is unsupported on this target")
	}
	oldFailAcquire := failLockAcquire
	oldBeforeIdentity := beforeRootIdentityCheck
	oldBeforeRelease := beforeLockRelease
	oldAfterRelease := afterLockRelease
	t.Cleanup(func() {
		failLockAcquire = oldFailAcquire
		beforeRootIdentityCheck = oldBeforeIdentity
		beforeLockRelease = oldBeforeRelease
		afterLockRelease = oldAfterRelease
	})

	failCalls := 0
	failLockAcquire = func() error {
		failCalls++
		return syscall.EWOULDBLOCK
	}
	authority, err := Acquire(t.TempDir())
	if authority != nil {
		_ = authority.Release()
		t.Fatal("failLockAcquire still acquired authority")
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeTransactionInProgress ||
		failCalls != 1 {
		t.Fatalf("acquire seam = authority=%v err=%v fail=%d", authority, err, failCalls)
	}

	failLockAcquire = nil
	rootChecks := 0
	releaseEvents := []string{}
	beforeRootIdentityCheck = func(path string) {
		if path == "" {
			t.Fatal("beforeRootIdentityCheck received an empty path")
		}
		rootChecks++
	}
	beforeLockRelease = func() { releaseEvents = append(releaseEvents, "before") }
	afterLockRelease = func() { releaseEvents = append(releaseEvents, "after") }
	authority, err = Acquire(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.ValidateOriginalPath(false); err != nil {
		t.Fatal(err)
	}
	if err := authority.Release(); err != nil {
		t.Fatal(err)
	}
	if rootChecks != 1 ||
		len(releaseEvents) != 2 || releaseEvents[0] != "before" || releaseEvents[1] != "after" {
		t.Fatalf("authority seam order = root=%d release=%v", rootChecks, releaseEvents)
	}
}

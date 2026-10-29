//go:build !linux && !darwin && !windows

package candidatelock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnsupportedTryAcquireRefusesWithoutWrites(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "locks", "candidate.lock")
	if Supported {
		t.Fatal("unsupported build unexpectedly reports Supported=true")
	}
	_, err := TryAcquire(lockPath)
	if err == nil {
		t.Fatal("TryAcquire succeeded on unsupported target")
	}
	if !IsCode(err, CodeLockUnsupported) {
		t.Fatalf("error = %v, want %s", err, CodeLockUnsupported)
	}
	if _, statErr := os.Stat(filepath.Dir(lockPath)); !os.IsNotExist(statErr) {
		t.Fatalf("unsupported target created lock directory: %v", statErr)
	}
}

//go:build windows

package candidatelock

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCandidateLockNativeWindows(t *testing.T) {
	if !Supported {
		t.Fatal("windows build must report candidate lock support")
	}
	t.Run("CLW-001-acquire-release", func(t *testing.T) {
		lockPath := filepath.Join(t.TempDir(), "candidate.lock")
		handle, err := TryAcquire(lockPath)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		if err := handle.Release(); err != nil {
			t.Fatalf("release: %v", err)
		}
	})

	t.Run("CLW-002-contention", func(t *testing.T) {
		lockPath := filepath.Join(t.TempDir(), "candidate.lock")
		first, err := TryAcquire(lockPath)
		if err != nil {
			t.Fatalf("acquire first: %v", err)
		}
		defer first.Release()
		second, err := TryAcquire(lockPath)
		if err == nil {
			_ = second.Release()
			t.Fatal("second lock unexpectedly acquired")
		}
		if !IsCode(err, CodeCandidateLockHeld) {
			t.Fatalf("second lock error = %v, want %s", err, CodeCandidateLockHeld)
		}
	})

	t.Run("CLW-003-crash-release", func(t *testing.T) {
		lockDir := t.TempDir()
		lockPath := filepath.Join(lockDir, "candidate.lock")
		ready := filepath.Join(lockDir, "ready")
		cmd := exec.Command(os.Args[0], "-test.run=TestCandidateLockHelperProcess", "--", lockPath, ready)
		cmd.Env = append(os.Environ(), "TPATCH_CANDIDATE_LOCK_HELPER=1")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start helper: %v", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(ready); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("helper did not become ready")
			}
			time.Sleep(25 * time.Millisecond)
		}
		if _, err := TryAcquire(lockPath); !IsCode(err, CodeCandidateLockHeld) {
			t.Fatalf("lock should be held while helper is alive, err=%v", err)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("helper wait: %v", err)
		}
		handle, err := TryAcquire(lockPath)
		if err != nil {
			t.Fatalf("reacquire after helper exit: %v", err)
		}
		if err := handle.Release(); err != nil {
			t.Fatalf("release after helper exit: %v", err)
		}
	})
}

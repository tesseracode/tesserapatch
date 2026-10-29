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
		reacquire, err := TryAcquire(lockPath)
		if err != nil {
			t.Fatalf("same-process reacquire: %v", err)
		}
		if err := reacquire.Release(); err != nil {
			t.Fatalf("release reacquire: %v", err)
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
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("stdin pipe: %v", err)
		}
		cmd.Env = append(os.Environ(), "TPATCH_CANDIDATE_LOCK_HELPER=1")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start helper: %v", err)
		}
		t.Cleanup(func() {
			_ = stdin.Close()
			if cmd.ProcessState == nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})

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
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("kill helper: %v", err)
		}
		_ = stdin.Close()
		if err := cmd.Wait(); err == nil {
			t.Fatal("helper exited cleanly after forced kill")
		}
		handle, err := TryAcquire(lockPath)
		if err != nil {
			t.Fatalf("reacquire after helper crash: %v", err)
		}
		if err := handle.Release(); err != nil {
			t.Fatalf("release after helper crash: %v", err)
		}
	})

	t.Run("CLW-004-unc-refusal", func(t *testing.T) {
		_, err := TryAcquire(`\\\\localhost\\share\\candidate.lock`)
		if !IsCode(err, CodeLockFilesystemUnsupported) {
			t.Fatalf("UNC refusal err=%v, want %s", err, CodeLockFilesystemUnsupported)
		}
	})

	t.Run("CLW-005-borrow-release-exclusion", func(t *testing.T) {
		lockPath := filepath.Join(t.TempDir(), "candidate.lock")
		handle, err := TryAcquire(lockPath)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		releaseBorrow, err := handle.Borrow(lockPath)
		if err != nil {
			t.Fatalf("borrow: %v", err)
		}
		if err := handle.Release(); err == nil {
			t.Fatal("release unexpectedly succeeded while borrowed")
		}
		if _, err := TryAcquire(lockPath); !IsCode(err, CodeCandidateLockHeld) {
			t.Fatalf("lock became available while borrowed, err=%v", err)
		}
		releaseBorrow()
		if err := handle.Release(); err != nil {
			t.Fatalf("release after borrow: %v", err)
		}
		reacquire, err := TryAcquire(lockPath)
		if err != nil {
			t.Fatalf("reacquire: %v", err)
		}
		if err := reacquire.Release(); err != nil {
			t.Fatalf("release reacquire: %v", err)
		}
	})
}

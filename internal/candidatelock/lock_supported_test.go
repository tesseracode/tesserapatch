//go:build linux || darwin || windows

package candidatelock

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTryAcquireContentionAndReacquire(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "candidate.lock")

	first, err := TryAcquire(lockPath)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	defer func() {
		if releaseErr := first.Release(); releaseErr != nil {
			t.Fatalf("release first lock: %v", releaseErr)
		}
	}()

	second, err := TryAcquire(lockPath)
	if err == nil {
		_ = second.Release()
		t.Fatal("second lock unexpectedly acquired")
	}
	if !IsCode(err, CodeCandidateLockHeld) {
		t.Fatalf("second lock error = %v, want %s", err, CodeCandidateLockHeld)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}
	first = nil

	third, err := TryAcquire(lockPath)
	if err != nil {
		t.Fatalf("third lock after release: %v", err)
	}
	if err := third.Release(); err != nil {
		t.Fatalf("release third lock: %v", err)
	}
}

func TestTryAcquireFilesystemRefusalDoesNotWrite(t *testing.T) {
	original := acquireFilesystemClassifier
	acquireFilesystemClassifier = func(string) error {
		return &Error{
			Code:   CodeLockFilesystemUnsupported,
			Detail: "synthetic refusal",
		}
	}
	t.Cleanup(func() { acquireFilesystemClassifier = original })

	lockPath := filepath.Join(t.TempDir(), "locks", "candidate.lock")
	_, err := TryAcquire(lockPath)
	if err == nil {
		t.Fatal("filesystem refusal did not fire")
	}
	if !IsCode(err, CodeLockFilesystemUnsupported) {
		t.Fatalf("error = %v, want %s", err, CodeLockFilesystemUnsupported)
	}
	if _, statErr := os.Stat(filepath.Dir(lockPath)); !os.IsNotExist(statErr) {
		t.Fatalf("lock directory was created despite refusal: %v", statErr)
	}
}

func TestCandidateLockHelperProcess(t *testing.T) {
	if os.Getenv("TPATCH_CANDIDATE_LOCK_HELPER") != "1" {
		return
	}
	args := os.Args
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "missing helper args")
		os.Exit(2)
	}
	lockPath := args[len(args)-2]
	readyPath := args[len(args)-1]
	lock, err := TryAcquire(lockPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	if err := os.WriteFile(readyPath, []byte("ready"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	time.Sleep(1250 * time.Millisecond)
	_ = lock
	os.Exit(0)
}

func TestTryAcquireHeldByOtherProcessThenReacquire(t *testing.T) {
	lockDir := t.TempDir()
	lockPath := filepath.Join(lockDir, "candidate.lock")
	ready := filepath.Join(lockDir, "ready")

	cmd := exec.Command(os.Args[0], "-test.run=TestCandidateLockHelperProcess", "--", lockPath, ready)
	cmd.Env = append(os.Environ(), "TPATCH_CANDIDATE_LOCK_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}

	second, err := TryAcquire(lockPath)
	if err == nil {
		_ = second.Release()
		t.Fatal("lock acquisition succeeded while helper held lock")
	}
	if !IsCode(err, CodeCandidateLockHeld) {
		t.Fatalf("error = %v, want %s", err, CodeCandidateLockHeld)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper wait: %v", err)
	}

	third, err := TryAcquire(lockPath)
	if err != nil {
		t.Fatalf("reacquire after helper exit: %v", err)
	}
	if err := third.Release(); err != nil {
		t.Fatalf("release third lock: %v", err)
	}
}

func TestErrorCodeMatcher(t *testing.T) {
	err := &Error{Code: CodeCandidateLockHeld}
	if !IsCode(err, CodeCandidateLockHeld) {
		t.Fatal("IsCode did not match direct error")
	}
	if IsCode(fmt.Errorf("wrap: %w", err), CodeLockUnsupported) {
		t.Fatal("IsCode matched wrong code")
	}
	if IsCode(syscall.EINVAL, CodeCandidateLockHeld) {
		t.Fatal("IsCode matched non-lock error")
	}
}

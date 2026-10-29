//go:build linux || darwin || windows

package candidatelock

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	acquireFilesystemClassifier = func(string, *os.File) error {
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

func TestTryAcquireUsesLocalFilesystemClassifier(t *testing.T) {
	dir := t.TempDir()
	probe, err := openClassifierFile(dir)
	if err != nil {
		t.Fatalf("open classifier file: %v", err)
	}
	defer probe.Close()
	if err := acquireFilesystemClassifier(dir, probe); err != nil {
		t.Fatalf("classifier rejected local test filesystem: %v", err)
	}
}

func TestCanonicalLockPathPreservesPathBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows path normalization trims trailing spaces at the filesystem layer")
	}
	path := filepath.Join(t.TempDir(), "candidate-lock ")
	canonical, err := canonicalLockPath(path)
	if err != nil {
		t.Fatalf("canonicalLockPath: %v", err)
	}
	if !strings.HasSuffix(canonical, "candidate-lock ") {
		t.Fatalf("canonical path lost trailing space bytes: %q", canonical)
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
	_ = lock
	_, _ = os.Stdin.Read(make([]byte, 1))
	os.Exit(0)
}

func TestTryAcquireHeldByOtherProcessCrashReleaseAndReacquire(t *testing.T) {
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
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_, _ = cmd.Process.Wait()
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

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill helper: %v", err)
	}
	_ = stdin.Close()
	waitErr := cmd.Wait()
	if waitErr == nil {
		t.Fatal("helper exited cleanly; expected forced termination")
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		if runtime.GOOS != "windows" && exitErr.ExitCode() == 0 {
			t.Fatalf("helper exit code=%d, want non-zero after forced kill", exitErr.ExitCode())
		}
	}

	third, err := TryAcquire(lockPath)
	if err != nil {
		t.Fatalf("reacquire after helper crash: %v", err)
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

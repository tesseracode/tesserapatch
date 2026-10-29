//go:build linux || darwin

package candidate

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestCandidateReadFileRefusesFIFOWithoutBlocking(t *testing.T) {
	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012350", head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	defer func() {
		if !candidateRoot.closed {
			_ = candidateRoot.Prune(featureLock, globalLock)
		}
	}()

	fifo := filepath.Join(candidateRoot.WorktreePath(), "blocked.fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, readErr := candidateRoot.ReadFile("blocked.fifo")
		done <- readErr
	}()

	select {
	case readErr := <-done:
		if readErr == nil {
			t.Fatal("FIFO read unexpectedly succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadFile blocked on FIFO")
	}
}

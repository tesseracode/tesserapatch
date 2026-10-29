//go:build linux || darwin

package candidatelock

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestTryAcquireRejectsSymlinkLeaf(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.lock")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "candidate.lock")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := TryAcquire(link); err == nil {
		t.Fatal("TryAcquire accepted a symlink leaf")
	}
}

func TestTryAcquireRejectsFIFOLeaf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate.lock")
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	if _, err := TryAcquire(path); err == nil {
		t.Fatal("TryAcquire accepted a FIFO leaf")
	}
}

func TestTryAcquireRejectsDirectoryLeaf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate.lock")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir leaf: %v", err)
	}
	if _, err := TryAcquire(path); err == nil {
		t.Fatal("TryAcquire accepted a directory leaf")
	}
}

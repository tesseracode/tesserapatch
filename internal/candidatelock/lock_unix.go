//go:build linux || darwin

package candidatelock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Supported reports whether this build target has a native advisory lock.
const Supported = true

var acquireFilesystemClassifier = classifyLockFilesystem

// TryAcquire takes a nonblocking exclusive advisory lock on path.
func TryAcquire(path string) (*Handle, error) {
	lockPath, err := canonicalLockPath(path)
	if err != nil {
		return nil, err
	}
	ancestor, err := nearestExistingAncestor(filepath.Dir(lockPath))
	if err != nil {
		return nil, err
	}
	if err := acquireFilesystemClassifier(ancestor); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, &Error{
				Code:   CodeCandidateLockHeld,
				Detail: "another process already holds this candidate lock",
			}
		}
		return nil, fmt.Errorf("acquire advisory lock: %w", err)
	}
	return &Handle{file: file, path: lockPath}, nil
}

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
	parentPath := filepath.Dir(lockPath)
	leaf := filepath.Base(lockPath)

	ancestor, err := nearestExistingAncestor(parentPath)
	if err != nil {
		return nil, err
	}
	ancestorFile, err := openClassifierFile(ancestor)
	if err != nil {
		return nil, err
	}
	if err := acquireFilesystemClassifier(ancestor, ancestorFile); err != nil {
		_ = ancestorFile.Close()
		return nil, err
	}
	if err := ancestorFile.Close(); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	parentRoot, parentDir, err := openLockParent(parentPath)
	if err != nil {
		return nil, err
	}
	defer parentDir.Close()
	defer parentRoot.Close()

	if err := acquireFilesystemClassifier(parentPath, parentDir); err != nil {
		return nil, err
	}
	file, err := openLockLeaf(parentRoot, leaf)
	if err != nil {
		return nil, err
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
	return &Handle{state: &lockState{file: file, path: lockPath}}, nil
}

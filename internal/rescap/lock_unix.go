//go:build linux || darwin

// Kernel advisory locking and the filesystem-type preflight for
// PRD-feature-resource-claims-and-capture-adapters §7.2.
//
// A kernel flock(2) on an open file description has none of the
// ABA/staleness problems of a PID-plus-rename lock protocol: it is not
// data a process reads and reasons about, it is a kernel-tracked
// association between one open file description and one inode, and it
// is released automatically when the last referencing descriptor is
// closed — including on SIGKILL, crash, or power loss.

package rescap

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// LockSupported reports whether this build target has a real flock
// implementation. It is true for linux/darwin and false everywhere
// else (see lock_unsupported.go).
const LockSupported = true

// Lock is a held per-slug advisory lock.
type Lock struct {
	file *os.File
	path string
}

// Path returns the lock file's path.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Release closes the descriptor, which is what releases the kernel
// lock. There is no separate unlock step that could fail or leave a
// stale artifact behind.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f.Close()
}

// AcquireLock takes the exclusive, nonblocking advisory lock for a
// scratch root. The caller is responsible for having already run the
// local ignore/untracked gate: this function creates the lock file,
// which is itself the first piece of scratch state any mutator writes.
//
// Sequence (§7.2): statfs preflight on the nearest existing ancestor,
// MkdirAll + whole-chain fsync, O_CREATE|O_RDWR 0600 open, then
// LOCK_EX|LOCK_NB. Contention refuses immediately — no wait, no retry.
func AcquireLock(scratchRoot, repoRoot string) (*Lock, error) {
	ancestor, err := store.NearestExistingAncestor(scratchRoot)
	if err != nil {
		return nil, Refuse(ReasonResourceLockFSUnsupported,
			"no existing ancestor of %s could be inspected for its filesystem type", scratchRoot)
	}
	if err := CheckFilesystemSupported(ancestor); err != nil {
		return nil, err
	}
	if err := store.MkdirAllAndSyncChain(scratchRoot, repoRoot, 0o700); err != nil {
		return nil, Internal(ReasonAdapterCopyFailed, "creating scratch root %s: %v", scratchRoot, err)
	}
	lockPath := filepath.Join(scratchRoot, ".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, Internal(ReasonAdapterCopyFailed, "opening %s: %v", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, Refuse(ReasonCaptureInProgress,
				"another tpatch resource mutation already holds %s", lockPath)
		}
		return nil, Internal(ReasonAdapterCopyFailed, "flock %s: %v", lockPath, err)
	}
	return &Lock{file: f, path: lockPath}, nil
}

//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"errors"
	"os"
	"syscall"
)

// AuthoritySupported reports whether this target provides real directory
// authority.
const AuthoritySupported = true

var defaultAuthorityOps = authorityOps{
	openRoot: os.OpenRoot,
	openDir: func(root *os.Root) (*os.File, error) {
		return root.Open(".")
	},
	classify:     classifyHeldFilesystem,
	lock:         lockHeldDirectory,
	unlock:       unlockHeldDirectory,
	fileIdentity: identityFromFile,
	pathIdentity: identityFromPath,
	closeFile:    func(file *os.File) error { return file.Close() },
	closeRoot:    func(root *os.Root) error { return root.Close() },
}

// Acquire opens, classifies, locks, and retains one workspace-root authority.
func Acquire(discoveredWorkspacePath string) (*WorkspaceAuthority, error) {
	return acquireWithOps(discoveredWorkspacePath, defaultAuthorityOps)
}

func acquireWithOps(discoveredWorkspacePath string, ops authorityOps) (*WorkspaceAuthority, error) {
	root, err := ops.openRoot(discoveredWorkspacePath)
	if err != nil {
		return nil, authorityError("open-root", "workspace root authority could not be opened")
	}
	directory, err := ops.openDir(root)
	if err != nil {
		_ = ops.closeRoot(root)
		return nil, authorityError("open-directory", "workspace root directory handle could not be opened")
	}
	closeHandles := func() {
		_ = ops.closeFile(directory)
		_ = ops.closeRoot(root)
	}

	identity, err := ops.fileIdentity(directory)
	if err != nil {
		closeHandles()
		return nil, authorityError("identity-capture", "workspace root identity could not be captured")
	}

	class, denied, err := ops.classify(directory)
	if err != nil {
		closeHandles()
		return nil, authorityError("filesystem-classification", "workspace root filesystem could not be classified")
	}
	if denied {
		closeHandles()
		return nil, &Error{
			Code:   CodeLockFilesystemUnsupported,
			Class:  sanitizeFilesystemClass(class),
			Detail: "the detected workspace filesystem does not support this authority",
		}
	}

	var lockErr error
	if failLockAcquire != nil {
		lockErr = failLockAcquire()
	}
	if lockErr == nil {
		lockErr = ops.lock(directory)
	}
	if lockErr != nil {
		closeHandles()
		if errors.Is(lockErr, syscall.EWOULDBLOCK) || errors.Is(lockErr, syscall.EAGAIN) {
			return nil, &Error{
				Code:   CodeTransactionInProgress,
				Class:  "workspace",
				Detail: "another transaction holds the workspace authority",
			}
		}
		return nil, authorityError("flock", "workspace directory authority could not be locked")
	}

	return &WorkspaceAuthority{
		root:         root,
		directory:    directory,
		originalPath: discoveredWorkspacePath,
		identity:     identity,
		ops:          ops,
	}, nil
}

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

func acquireWithFilesystemClassifier(
	discoveredWorkspacePath string,
	classifier FilesystemClassifier,
) (*WorkspaceAuthority, error) {
	if classifier == nil {
		return nil, authorityError(
			"invalid-test-dependency",
			"filesystem classifier test dependency is nil",
		)
	}
	ops := defaultAuthorityOps
	ops.classify = classifier
	return acquireWithOps(discoveredWorkspacePath, ops)
}

func acquireWithStageHook(
	discoveredWorkspacePath string,
	hook AuthorityStageHook,
) (*WorkspaceAuthority, error) {
	if hook == nil {
		return nil, authorityError(
			"invalid-test-dependency",
			"authority stage test dependency is nil",
		)
	}
	ops := defaultAuthorityOps
	openRoot := ops.openRoot
	ops.openRoot = func(path string) (*os.Root, error) {
		if err := hook("open-root"); err != nil {
			return nil, err
		}
		return openRoot(path)
	}
	openDir := ops.openDir
	ops.openDir = func(root *os.Root) (*os.File, error) {
		if err := hook("open-directory"); err != nil {
			return nil, err
		}
		return openDir(root)
	}
	classify := ops.classify
	ops.classify = func(file *os.File) (string, bool, error) {
		if err := hook("fstatfs"); err != nil {
			return "", false, err
		}
		return classify(file)
	}
	lock := ops.lock
	ops.lock = func(file *os.File) error {
		if err := hook("flock"); err != nil {
			return err
		}
		return lock(file)
	}
	return acquireWithOps(discoveredWorkspacePath, ops)
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

package intentlock

import (
	"os"
	"runtime"
)

type nativeIdentity struct {
	device uint64
	inode  uint64
}

type authorityOps struct {
	openRoot     func(string) (*os.Root, error)
	openDir      func(*os.Root) (*os.File, error)
	classify     func(*os.File) (string, bool, error)
	lock         func(*os.File) error
	unlock       func(*os.File) error
	fileIdentity func(*os.File) (nativeIdentity, error)
	pathIdentity func(string) (nativeIdentity, error)
	closeFile    func(*os.File) error
	closeRoot    func(*os.Root) error
}

// WorkspaceAuthority owns one workspace-root inode for a single invocation.
//
// It is single-goroutine owned: acquisition, rooted use, validation, and the
// one Release call must be performed by that goroutine.
type WorkspaceAuthority struct {
	root         *os.Root
	directory    *os.File
	originalPath string
	identity     nativeIdentity
	released     bool
	ops          authorityOps
}

// WithRoot runs fn with the retained rooted handle. The handle must not be
// closed or retained by fn.
func (a *WorkspaceAuthority) WithRoot(fn func(*os.Root) error) error {
	if a == nil || a.root == nil || a.released {
		return authorityError("authority-released", "workspace authority is not available")
	}
	if fn == nil {
		return authorityError("invalid-root-operation", "rooted operation is nil")
	}
	return fn(a.root)
}

// ValidateOriginalPath confirms that the originally discovered path still
// resolves to the retained root inode.
func (a *WorkspaceAuthority) ValidateOriginalPath(afterPublication bool) error {
	if a == nil || a.root == nil || a.directory == nil || a.released {
		return authorityError("authority-released", "workspace authority is not available")
	}
	if beforeRootIdentityCheck != nil {
		beforeRootIdentityCheck(a.originalPath)
	}
	identity, err := a.ops.pathIdentity(a.originalPath)
	if err == nil && identity == a.identity {
		return nil
	}
	code := CodeWorkspaceRootChanged
	detail := "the original workspace path no longer identifies the held workspace root"
	if afterPublication {
		code = CodeWorkspaceRootReplacedAfterPublication
		detail = "the original workspace path was replaced after publication began"
	}
	return &Error{Code: code, Class: "original-path-changed", Detail: detail}
}

// Released reports whether Release has already begun.
func (a *WorkspaceAuthority) Released() bool {
	return a == nil || a.released
}

// Release unlocks and closes the retained handles exactly once.
func (a *WorkspaceAuthority) Release() error {
	if a == nil {
		return authorityError("nil-authority", "workspace authority is nil")
	}
	if a.released {
		return authorityError("authority-already-released", "workspace authority was already released")
	}
	if beforeLockRelease != nil {
		beforeLockRelease()
	}
	a.released = true

	var first error
	if err := a.ops.unlock(a.directory); err != nil {
		first = authorityError("unlock", "workspace directory unlock failed")
	}
	if err := a.ops.closeFile(a.directory); err != nil && first == nil {
		first = authorityError("close-directory", "workspace directory handle close failed")
	}
	if err := a.ops.closeRoot(a.root); err != nil && first == nil {
		first = authorityError("close-root", "workspace root handle close failed")
	}
	if afterLockRelease != nil {
		afterLockRelease()
	}
	runtime.KeepAlive(a)
	return first
}

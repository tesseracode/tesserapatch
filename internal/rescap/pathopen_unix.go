//go:build !windows

// O_NOFOLLOW-based final-component hardening for
// PRD-feature-resource-claims-and-capture-adapters §9.1 step 4.
//
// O_NOFOLLOW binds only the *final* component, which is exactly what
// this design claims: a symlink that appears at the final component
// between the ancestor walk and the open fails the open with ELOOP and
// is refused the same as any other symlink component. Ancestor
// components are covered by the walk, not by this flag.

package rescap

import (
	"errors"
	"os"
	"syscall"
)

// openNoFollow opens path read-only, refusing to follow a symlink at
// the final component.
func openNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
}

// isSymlinkLoopError reports whether an open failed because the final
// component was a symlink.
func isSymlinkLoopError(err error) bool { return errors.Is(err, syscall.ELOOP) }

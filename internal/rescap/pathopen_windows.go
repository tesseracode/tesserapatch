//go:build windows

// Windows has no O_NOFOLLOW, and resource capture is explicitly
// unsupported there (PRD §7.2's hard "unsupported" contract). These
// definitions exist only so the package compiles for the target: the
// lock layer refuses before any gated path is ever opened.

package rescap

import "os"

// openNoFollow opens path read-only. The final-component symlink
// hardening O_NOFOLLOW provides is unavailable on this target, which is
// one of the reasons resource capture is refused here outright.
func openNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY, 0)
}

// isSymlinkLoopError always reports false on this target.
func isSymlinkLoopError(err error) bool { return false }

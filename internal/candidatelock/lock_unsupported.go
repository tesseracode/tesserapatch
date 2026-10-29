//go:build !linux && !darwin && !windows

package candidatelock

// Supported reports whether this build target has a native advisory lock.
const Supported = false

// TryAcquire refuses on targets without verified native support.
func TryAcquire(string) (*Handle, error) {
	return nil, &Error{
		Code:   CodeLockUnsupported,
		Detail: "candidate locking is supported only on linux, darwin, and windows",
	}
}

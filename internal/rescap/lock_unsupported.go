//go:build !linux && !darwin

// Fail-closed locking stub for every non-linux/darwin build target.
//
// PRD-feature-resource-claims-and-capture-adapters §7.2 makes this an
// explicit "unsupported" contract, not a best-effort fallback: Windows,
// AIX, Solaris and any future target refuse before touching the
// filesystem at all. Lifting the restriction requires both CI coverage
// for the new target and a real locking primitive for it.

package rescap

// LockSupported reports whether this build target has a real flock
// implementation.
const LockSupported = false

// Lock is the stub lock handle. It is never constructed on an
// unsupported host.
type Lock struct{}

// Path returns the empty string on an unsupported host.
func (l *Lock) Path() string { return "" }

// Release is a no-op on an unsupported host.
func (l *Lock) Release() error { return nil }

// AcquireLock refuses unconditionally on an unsupported host, without
// creating or opening anything.
func AcquireLock(scratchRoot, repoRoot string) (*Lock, error) {
	return nil, Refuse(ReasonResourceLockUnsupported,
		"resource capture requires a linux or darwin host; this build target has no verified flock(2) primitive")
}

// CheckFilesystemSupported refuses unconditionally on an unsupported
// host. The build-tag refusal above already fires first; this exists so
// the symbol set is identical across build targets.
func CheckFilesystemSupported(path string) error {
	return Refuse(ReasonResourceLockUnsupported,
		"resource capture requires a linux or darwin host; this build target has no verified statfs preflight")
}

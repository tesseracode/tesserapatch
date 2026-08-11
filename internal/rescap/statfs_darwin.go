//go:build darwin

// Darwin filesystem-type preflight for
// PRD-feature-resource-claims-and-capture-adapters §7.2.
//
// Darwin reports a filesystem name rather than a magic number.
// Statfs_t.Fstypename is accurately an array of signed 8-bit integers
// ([16]int8, not a []byte), so converting it to a comparable Go string
// requires an explicit per-element signed-to-unsigned cast and a trim
// at the first NUL.

package rescap

import (
	"syscall"
)

// darwinAllowedFilesystems is the exact allowlist.
var darwinAllowedFilesystems = map[string]struct{}{
	"apfs":  {},
	"hfs":   {},
	"tmpfs": {},
}

// darwinDeniedFilesystems names representative known-network/known-FUSE
// entries for a more actionable refusal message. Any unrecognized
// Fstypename refuses identically to a denylisted one.
var darwinDeniedFilesystems = map[string]struct{}{
	"nfs":     {},
	"smbfs":   {},
	"webdav":  {},
	"osxfuse": {},
	"macfuse": {},
}

// FstypenameString converts Darwin's [16]int8 Fstypename array to a Go
// string, trimming at the first NUL. Exported so the unit-test seam can
// exercise the signed-byte conversion without mounting a filesystem.
func FstypenameString(raw []int8) string {
	buf := make([]byte, 0, len(raw))
	for _, c := range raw {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
	}
	return string(buf)
}

// ClassifyFilesystemName reports whether flock is trusted on a Darwin
// filesystem name.
func ClassifyFilesystemName(name string) (denied bool, allowed bool) {
	if _, ok := darwinAllowedFilesystems[name]; ok {
		return false, true
	}
	_, denied = darwinDeniedFilesystems[name]
	return denied, false
}

// CheckFilesystemSupported refuses unless path's filesystem name is on
// the exact allowlist.
func CheckFilesystemSupported(path string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path, &buf); err != nil {
		return Refuse(ReasonResourceLockFSUnsupported,
			"statfs(%s) failed: %v", path, err)
	}
	name := FstypenameString(buf.Fstypename[:])
	if _, allowed := ClassifyFilesystemName(name); allowed {
		return nil
	}
	if name == "" {
		name = "an unnamed filesystem"
	}
	return Refuse(ReasonResourceLockFSUnsupported,
		"%s lives on %s; resource capture requires .tpatch/local/ on a recognized local filesystem", path, name)
}

// NoexecFlagSet reports whether a mount-flags value has Darwin's
// MNT_NOEXEC bit set (sys/mount.h).
func NoexecFlagSet(flags uint64) bool { return flags&mntNoexec != 0 }

const mntNoexec uint64 = 0x00000004

// CheckScratchExecutable refuses when the scratch filesystem is
// mounted noexec, before the private Dolt-binary copy is created at
// all (§6.1 capture-time step 3).
func CheckScratchExecutable(path string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path, &buf); err != nil {
		return Refuse(ReasonAdapterCopyNoexec,
			"statfs(%s) failed while checking for a noexec mount: %v", path, err)
	}
	if NoexecFlagSet(uint64(buf.Flags)) {
		return Refuse(ReasonAdapterCopyNoexec,
			"%s is on a noexec-mounted filesystem; the private adapter copy could never be executed there", path)
	}
	return nil
}

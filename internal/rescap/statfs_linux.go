//go:build linux

// Linux filesystem-type preflight for
// PRD-feature-resource-claims-and-capture-adapters §7.2.
//
// flock(2)'s advisory-lock semantics are only guaranteed for genuinely
// local filesystems. Rather than silently degrading on a mount where
// even the single-host guarantee may not hold, the preflight fails
// closed against an exact allowlist.

package rescap

import (
	"fmt"
	"syscall"
)

// Linux superblock magic numbers. Statfs_t.Type's width and signedness
// is architecture-dependent (int64 on amd64/arm64, int32 on 386/arm,
// uint32 on s390x), so every comparison happens against a single
// normalized uint32 value.
const (
	magicEXT     uint32 = 0xEF53
	magicXFS     uint32 = 0x58465342
	magicBTRFS   uint32 = 0x9123683E
	magicTMPFS   uint32 = 0x01021994
	magicOVERLAY uint32 = 0x794C7630
	magicNFS     uint32 = 0x6969
	magicCIFS    uint32 = 0xFF534D42
	magicSMB2    uint32 = 0xFE534D42
	magicFUSE    uint32 = 0x65735546
)

// linuxAllowedFilesystems is the exact allowlist. overlayfs is allowed
// because it is the default storage driver for Docker/Podman and
// behaves as a genuinely local advisory lock; denying it would make
// resource capture unusable in the container CI this project runs in.
var linuxAllowedFilesystems = map[uint32]string{
	magicEXT:     "ext2/ext3/ext4",
	magicXFS:     "xfs",
	magicBTRFS:   "btrfs",
	magicTMPFS:   "tmpfs",
	magicOVERLAY: "overlayfs",
}

// linuxDeniedFilesystems names the known-network/known-FUSE types for a
// more actionable refusal message. Anything not on the allowlist is
// refused regardless of whether it appears here.
var linuxDeniedFilesystems = map[uint32]string{
	magicNFS:  "nfs",
	magicCIFS: "cifs",
	magicSMB2: "smb2",
	magicFUSE: "fuse",
}

// normalizeStatfsType collapses Linux's architecture-dependent
// Statfs_t.Type field to a single uint32 so the allow/deny comparison
// is architecture-agnostic by construction.
func normalizeStatfsType[T ~int32 | ~int64 | ~uint32 | ~uint64](raw T) uint32 {
	return uint32(uint64(raw))
}

// ClassifyFilesystemMagic reports the recognized name for a normalized
// magic number and whether flock is trusted on it. Exported for the
// architecture-coverage unit test seam, which feeds fixture values
// representative of each width/signedness class rather than requiring
// four cross-architecture CI runners.
func ClassifyFilesystemMagic(fsType uint32) (name string, allowed bool) {
	if n, ok := linuxAllowedFilesystems[fsType]; ok {
		return n, true
	}
	if n, ok := linuxDeniedFilesystems[fsType]; ok {
		return n, false
	}
	return "", false
}

// CheckFilesystemSupported refuses unless path's filesystem is on the
// exact allowlist.
func CheckFilesystemSupported(path string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path, &buf); err != nil {
		return Refuse(ReasonResourceLockFSUnsupported,
			"statfs(%s) failed: %v", path, err)
	}
	fsType := normalizeStatfsType(buf.Type)
	name, allowed := ClassifyFilesystemMagic(fsType)
	if allowed {
		return nil
	}
	if name == "" {
		name = fmt.Sprintf("unrecognized magic 0x%X", fsType)
	}
	return Refuse(ReasonResourceLockFSUnsupported,
		"%s lives on %s; resource capture requires .tpatch/local/ on a recognized local filesystem", path, name)
}

// NoexecFlagSet reports whether a mount-flags value has Linux's
// ST_NOEXEC bit set ("Execution of programs is disallowed on this
// filesystem", statfs(2) f_flags, present since Linux 2.6.36).
func NoexecFlagSet(flags uint64) bool { return flags&stNoexec != 0 }

const stNoexec uint64 = 0x8

// CheckScratchExecutable refuses when the scratch filesystem is
// mounted noexec, before the private Dolt-binary copy is created at
// all (§6.1 capture-time step 3): creating an executable-intent copy
// on a filesystem the OS already marked non-executable can only fail
// later, and more confusingly, at cmd.Start().
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

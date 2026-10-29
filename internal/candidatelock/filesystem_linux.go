//go:build linux

package candidatelock

import (
	"fmt"
	"syscall"
)

const (
	linuxMagicEXT4    uint32 = 0xEF53
	linuxMagicXFS     uint32 = 0x58465342
	linuxMagicBTRFS   uint32 = 0x9123683E
	linuxMagicTMPFS   uint32 = 0x01021994
	linuxMagicOVERLAY uint32 = 0x794C7630
)

var linuxAllowedFilesystems = map[uint32]string{
	linuxMagicEXT4:    "ext",
	linuxMagicXFS:     "xfs",
	linuxMagicBTRFS:   "btrfs",
	linuxMagicTMPFS:   "tmpfs",
	linuxMagicOVERLAY: "overlay",
}

func normalizeLinuxStatfsType[T ~int32 | ~int64 | ~uint32 | ~uint64](value T) uint32 {
	return uint32(uint64(value))
}

func classifyLockFilesystem(path string) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return fmt.Errorf("statfs(%s): %w", path, err)
	}
	magic := normalizeLinuxStatfsType(stat.Type)
	if _, ok := linuxAllowedFilesystems[magic]; ok {
		return nil
	}
	return &Error{
		Code:   CodeLockFilesystemUnsupported,
		Detail: fmt.Sprintf("filesystem class 0x%08x is not in the local-filesystem allowlist", magic),
	}
}

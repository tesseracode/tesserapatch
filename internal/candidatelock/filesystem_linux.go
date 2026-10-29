//go:build linux

package candidatelock

import (
	"fmt"
	"os"
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

func classifyLinuxFilesystemType(magic uint32) error {
	if _, ok := linuxAllowedFilesystems[magic]; ok {
		return nil
	}
	return &Error{
		Code:   CodeLockFilesystemUnsupported,
		Detail: fmt.Sprintf("filesystem class 0x%08x is not in the local-filesystem allowlist", magic),
	}
}

func classifyLockFilesystem(path string, directory *os.File) error {
	if directory == nil {
		return fmt.Errorf("classify lock filesystem for %s: nil directory handle", path)
	}
	var stat syscall.Statfs_t
	if err := syscall.Fstatfs(int(directory.Fd()), &stat); err != nil {
		return fmt.Errorf("fstatfs(%s): %w", path, err)
	}
	magic := uint32(uint64(stat.Type))
	return classifyLinuxFilesystemType(magic)
}

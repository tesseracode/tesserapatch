//go:build linux && !android

package intentlock

import (
	"fmt"
	"os"
	"syscall"
)

const (
	linuxMagicNFS  uint32 = 0x00006969
	linuxMagicSMB  uint32 = 0x0000517B
	linuxMagicCIFS uint32 = 0xFF534D42
	linuxMagicSMB2 uint32 = 0xFE534D42
	linuxMagicFUSE uint32 = 0x65735546
)

func normalizeLinuxStatfsType[T ~int32 | ~int64 | ~uint32 | ~uint64](value T) uint32 {
	return uint32(uint64(value))
}

func classifyLinuxFilesystem(fsType uint32) (string, bool) {
	switch fsType {
	case linuxMagicNFS:
		return "nfs", true
	case linuxMagicSMB:
		return "smb", true
	case linuxMagicCIFS:
		return "cifs", true
	case linuxMagicSMB2:
		return "smb2", true
	case linuxMagicFUSE:
		return "fuse", true
	default:
		return fmt.Sprintf("0x%08x", fsType), false
	}
}

func classifyHeldFilesystem(file *os.File) (string, bool, error) {
	raw, err := file.SyscallConn()
	if err != nil {
		return "", false, err
	}
	var stat syscall.Statfs_t
	var statErr error
	if err := raw.Control(func(descriptor uintptr) {
		statErr = syscall.Fstatfs(int(descriptor), &stat)
	}); err != nil {
		return "", false, err
	}
	if statErr != nil {
		return "", false, statErr
	}
	name, denied := classifyLinuxFilesystem(normalizeLinuxStatfsType(stat.Type))
	return name, denied, nil
}

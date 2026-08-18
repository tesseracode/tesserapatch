//go:build darwin && !ios

package intentlock

import (
	"os"
	"syscall"
)

func darwinFilesystemName(raw []int8) string {
	name := make([]byte, 0, len(raw))
	for _, value := range raw {
		if value == 0 {
			break
		}
		name = append(name, byte(value))
	}
	return string(name)
}

func classifyDarwinFilesystem(name string) (string, bool) {
	switch name {
	case "nfs", "smbfs", "webdav", "macfuse", "osxfuse":
		return name, true
	default:
		return name, false
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
	name, denied := classifyDarwinFilesystem(darwinFilesystemName(stat.Fstypename[:]))
	return name, denied, nil
}

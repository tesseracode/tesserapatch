//go:build darwin

package candidatelock

import (
	"fmt"
	"syscall"
)

var darwinAllowedFilesystems = map[string]struct{}{
	"apfs":  {},
	"hfs":   {},
	"tmpfs": {},
}

const darwinMNTLocal uint32 = 0x00001000

func darwinFilesystemName(raw []int8) string {
	buf := make([]byte, 0, len(raw))
	for _, c := range raw {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
	}
	return string(buf)
}

func classifyLockFilesystem(path string) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return fmt.Errorf("statfs(%s): %w", path, err)
	}
	if stat.Flags&darwinMNTLocal == 0 {
		return &Error{
			Code:   CodeLockFilesystemUnsupported,
			Detail: "filesystem is not local (MNT_LOCAL is not set)",
		}
	}
	name := darwinFilesystemName(stat.Fstypename[:])
	if _, ok := darwinAllowedFilesystems[name]; ok {
		return nil
	}
	if name == "" {
		name = "unknown"
	}
	return &Error{
		Code:   CodeLockFilesystemUnsupported,
		Detail: fmt.Sprintf("filesystem %q is not in the local-filesystem allowlist", name),
	}
}

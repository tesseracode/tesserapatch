//go:build darwin

package candidatelock

import (
	"fmt"
	"os"
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

func classifyDarwinFilesystem(name string, local bool) error {
	if !local {
		return &Error{
			Code:   CodeLockFilesystemUnsupported,
			Detail: "filesystem is not local (MNT_LOCAL is not set)",
		}
	}
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

func classifyLockFilesystem(path string, directory *os.File) error {
	if directory == nil {
		return fmt.Errorf("classify lock filesystem for %s: nil directory handle", path)
	}
	var stat syscall.Statfs_t
	if err := syscall.Fstatfs(int(directory.Fd()), &stat); err != nil {
		return fmt.Errorf("fstatfs(%s): %w", path, err)
	}
	return classifyDarwinFilesystem(
		darwinFilesystemName(stat.Fstypename[:]),
		stat.Flags&darwinMNTLocal != 0,
	)
}

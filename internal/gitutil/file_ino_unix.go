//go:build !windows

package gitutil

import (
	"io/fs"
	"syscall"
)

// fileInoFromInfo extracts the inode number on platforms that have one.
func fileInoFromInfo(info fs.FileInfo) (uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Ino), true
}

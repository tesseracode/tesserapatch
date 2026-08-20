//go:build linux && !android && loong64

package intentpub

import "syscall"

const unlinkatTrap = syscall.SYS_UNLINKAT

func platformStatAt(directory uintptr, name string) (syscall.Stat_t, error) {
	var stat syscall.Stat_t
	err := syscall.Fstatat(int(directory), name, &stat, 0x100)
	return stat, err
}

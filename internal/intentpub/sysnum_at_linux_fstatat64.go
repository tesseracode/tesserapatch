//go:build linux && !android && (386 || arm || mips || mipsle)

package intentpub

import "syscall"

const unlinkatTrap = syscall.SYS_UNLINKAT

func platformStatAt(directory uintptr, name string) (syscall.Stat_t, error) {
	return rawStatAt(directory, name, syscall.SYS_FSTATAT64, 0x100)
}

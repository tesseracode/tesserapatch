//go:build linux && !android && (arm64 || riscv64)

package intentpub

import "syscall"

const unlinkatTrap = syscall.SYS_UNLINKAT

func platformStatAt(directory uintptr, name string) (syscall.Stat_t, error) {
	return rawStatAt(directory, name, syscall.SYS_FSTATAT, 0x100)
}

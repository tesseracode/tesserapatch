//go:build linux && !android && (mips64 || mips64le)

package intentpub

import "syscall"

const unlinkatTrap = syscall.SYS_UNLINKAT

func platformStatAt(directory uintptr, name string) (syscall.Stat_t, error) {
	var stat syscall.Stat_t
	// The stdlib wrapper converts the private mips64 kernel stat layout into
	// the public syscall.Stat_t; passing the public layout to newfstatat is unsafe.
	err := syscall.Fstatat(int(directory), name, &stat, 0x100)
	return stat, err
}

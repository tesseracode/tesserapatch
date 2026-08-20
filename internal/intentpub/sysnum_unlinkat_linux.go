//go:build linux && !android && (amd64 || ppc64 || ppc64le || s390x)

package intentpub

import "syscall"

const unlinkatTrap = syscall.SYS_UNLINKAT

func platformStatAt(directory uintptr, name string) (syscall.Stat_t, error) {
	return rawStatAt(directory, name, syscall.SYS_NEWFSTATAT, 0x100)
}

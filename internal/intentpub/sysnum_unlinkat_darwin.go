//go:build darwin && !ios

package intentpub

import "syscall"

const unlinkatTrap = 472

func platformStatAt(directory uintptr, name string) (syscall.Stat_t, error) {
	return rawStatAt(directory, name, 470, 0x20)
}

//go:build unix

package intentpub

import "syscall"

func openFlags() int {
	return syscall.O_NONBLOCK
}

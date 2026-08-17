//go:build unix

package intent

import "syscall"

func openFlags() int {
	return syscall.O_NONBLOCK
}

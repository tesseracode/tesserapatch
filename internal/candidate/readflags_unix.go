//go:build linux || darwin

package candidate

import "syscall"

func candidateReadOpenFlags() int {
	return syscall.O_NONBLOCK
}

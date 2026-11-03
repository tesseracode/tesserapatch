//go:build linux || darwin

package workflow

import (
	"os"
	"syscall"
)

func operationCandidateReadFlags() int {
	return os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
}

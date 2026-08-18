//go:build unix

package intent

import "syscall"

// openFlags returns the extra open flags for the final leaf on this target.
// It never includes a write, create, truncate or append bit.
//
// O_NONBLOCK bounds the open(2) of a FIFO or blocking character device that
// replaces the leaf after the pre-open kind check refused the stable case
// (PRD §7.4.3). Its scope is the open: it bounds no read and no Lstat, and it
// is not a termination guarantee (PRD §7.4.2, ADR-034 D16).
func openFlags() int {
	return syscall.O_NONBLOCK
}

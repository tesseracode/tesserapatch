//go:build !windows

package gitutil

import (
	"syscall"
	"testing"
)

// makeFifoForTest creates a FIFO at path, reporting whether the
// platform allowed it. Kept in a `!windows` file so the symbol is never
// compiled on platforms without it — a runtime skip would still fail
// `GOOS=windows go vet` (GH #7 rev-9 F3).
func makeFifoForTest(t *testing.T, path string) bool {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Logf("Mkfifo: %v", err)
		return false
	}
	return true
}

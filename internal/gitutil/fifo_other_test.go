//go:build windows

package gitutil

import "testing"

// makeFifoForTest reports that FIFOs are unavailable on this platform.
func makeFifoForTest(*testing.T, string) bool { return false }

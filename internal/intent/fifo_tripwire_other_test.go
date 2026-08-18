//go:build !unix

package intent

import (
	"errors"
	"testing"
)

// fifoOpenTripwire is non-Windows-only per AVP-200. On Windows there is no
// FIFO and no `O_NONBLOCK`, so the guard half is vacuously satisfied and the
// sensitivity half reports that the fixture is inapplicable — which is a
// non-nil error, exactly as the meta-check requires.
func fifoOpenTripwire(t *testing.T, withNonblock bool) error {
	t.Helper()
	if withNonblock {
		return nil
	}
	return errors.New("the FIFO tripwire is a non-Windows row (AVP-200); no blocking open exists on this target")
}

//go:build windows

package cli

import "testing"

func TestS7WindowsPlatformRows(t *testing.T) {
	t.Run("PIB-409", s7TestPlatformMutationRefusal)
	t.Run("PIB-417", s7TestPlatformCheckCompatibility)
}

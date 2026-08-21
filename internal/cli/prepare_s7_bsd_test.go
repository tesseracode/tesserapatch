//go:build freebsd || openbsd || netbsd || dragonfly

package cli

import "testing"

const s7BSDNativeLimitation = "BSD command rows are cross-compiled in the blocking guard; CI has no native BSD runner, so the identical production predicate branch is executed by TestS7BSDPlatformPredicateSeamRuntime."

func TestS7BSDPlatformRows(t *testing.T) {
	t.Run("PIB-409", s7TestPlatformMutationRefusal)
	t.Run("PIB-417", s7TestPlatformCheckCompatibility)
}

package intentlock

import "testing"

func TestS6InjectionSeamsNilAtInitialization(t *testing.T) {
	t.Run("PIB-232-intentlock-runtime-nil", func(t *testing.T) {
		if failLockAcquire != nil ||
			beforeLockRelease != nil ||
			afterLockRelease != nil ||
			beforeRootIdentityCheck != nil {
			t.Fatal("an intentlock injection seam was non-nil at initialization")
		}
	})
}

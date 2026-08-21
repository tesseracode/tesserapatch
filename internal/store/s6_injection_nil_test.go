package store

import "testing"

func TestS6InjectionSeamsNilAtInitialization(t *testing.T) {
	t.Run("PIB-232-store-runtime-nil", func(t *testing.T) {
		if beforeBlobWrite != nil ||
			afterBlobWrite != nil ||
			beforeBlobRemove != nil ||
			afterPurgeBlobRemove != nil ||
			beforePendingTombstoneCAS != nil ||
			failPurgeAfterFirstMutation != nil ||
			beforePurgeIndexCAS != nil ||
			afterPurgeIndexRename != nil ||
			beforePurgeBlobRemove != nil ||
			afterPurgeBlobRevalidate != nil ||
			failPurgeBetweenHashes != nil ||
			failOrphanRemoveAfterFirst != nil {
			t.Fatal("a store injection seam was non-nil at initialization")
		}
	})
}

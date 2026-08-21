package intentpub

import "testing"

func TestS6InjectionSeamsNilAtInitialization(t *testing.T) {
	t.Run("PIB-232-intentpub-runtime-nil", func(t *testing.T) {
		if beforeJournalWrite != nil ||
			afterJournalWrite != nil ||
			beforeControlWriteRename != nil ||
			beforeEntryCAS != nil ||
			beforeRename != nil ||
			afterRename != nil ||
			beforeStatusRename != nil ||
			afterStatusRename != nil ||
			beforeFinalVerify != nil ||
			beforeJournalClear != nil ||
			failFsync != nil ||
			failRename != nil {
			t.Fatal("an intentpub injection seam was non-nil at initialization")
		}
	})
}

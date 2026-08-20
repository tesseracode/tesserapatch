package intentpub

var (
	beforeJournalWrite       func(string)
	afterJournalWrite        func(string)
	beforeControlWriteRename func(string)
	beforeEntryCAS           func(int)
	beforeRename             func(int)
	afterRename              func(int)
	beforeStatusRename       func(string)
	afterStatusRename        func(string)
	beforeFinalVerify        func()
	beforeJournalClear       func(string)
	failFsync                func(string) error
	failRename               func(string) error
	afterTempContentRead     func(string)
)

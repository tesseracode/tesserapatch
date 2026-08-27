package store

var (
	beforeBlobWrite             func(string)
	afterBlobWrite              func(string)
	beforeBlobRemove            func(string)
	afterPurgeBlobRemove        func(string)
	beforePendingTombstoneCAS   func(string)
	failPurgeAfterFirstMutation func() error
	beforePurgeIndexCAS         func(string)
	afterPurgeIndexRename       func(string)
	afterPurgeIndexDecode       func(string)
	beforePurgeBlobRemove       func(string)
	afterPurgeBlobRevalidate    func(string)
	failPurgeBetweenHashes      func() error
	failOrphanRemoveAfterFirst  func() error
)

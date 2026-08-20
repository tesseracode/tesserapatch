package intentlock

var (
	failLockAcquire         func() error
	beforeRootIdentityCheck func(string)
	beforeLockRelease       func()
	afterLockRelease        func()
)

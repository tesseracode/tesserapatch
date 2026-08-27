package intentlock

import "os"

// FilesystemClassifier is a raw filesystem classifier used by internal tests.
type FilesystemClassifier func(*os.File) (class string, denied bool, err error)

// AuthorityStageHook observes or fails one raw authority acquisition stage.
// It is a test dependency; production callers must use Acquire.
type AuthorityStageHook func(stage string) error

// AcquireWithFilesystemClassifier runs normal authority acquisition with only
// the filesystem classifier replaced. Production code must use Acquire.
func AcquireWithFilesystemClassifier(
	discoveredWorkspacePath string,
	classifier FilesystemClassifier,
) (*WorkspaceAuthority, error) {
	return acquireWithFilesystemClassifier(discoveredWorkspacePath, classifier)
}

// AcquireWithStageHook runs normal authority acquisition with stage-local test
// instrumentation. Production code must use Acquire.
func AcquireWithStageHook(
	discoveredWorkspacePath string,
	hook AuthorityStageHook,
) (*WorkspaceAuthority, error) {
	return acquireWithStageHook(discoveredWorkspacePath, hook)
}

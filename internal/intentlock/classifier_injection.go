package intentlock

import "os"

// FilesystemClassifier is a raw filesystem classifier used by internal tests.
type FilesystemClassifier func(*os.File) (class string, denied bool, err error)

// AcquireWithFilesystemClassifier runs normal authority acquisition with only
// the filesystem classifier replaced. Production code must use Acquire.
func AcquireWithFilesystemClassifier(
	discoveredWorkspacePath string,
	classifier FilesystemClassifier,
) (*WorkspaceAuthority, error) {
	return acquireWithFilesystemClassifier(discoveredWorkspacePath, classifier)
}

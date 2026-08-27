//go:build android || ios || (!linux && !darwin)

package intentlock

// AuthoritySupported reports whether this target provides real directory
// authority.
const AuthoritySupported = false

// Acquire fails before opening the workspace on unsupported targets.
func Acquire(string) (*WorkspaceAuthority, error) {
	return unsupportedAuthorityError()
}

func acquireWithFilesystemClassifier(
	_ string,
	_ FilesystemClassifier,
) (*WorkspaceAuthority, error) {
	return unsupportedAuthorityError()
}

func acquireWithStageHook(
	_ string,
	_ AuthorityStageHook,
) (*WorkspaceAuthority, error) {
	return unsupportedAuthorityError()
}

func unsupportedAuthorityError() (*WorkspaceAuthority, error) {
	return nil, &Error{
		Code:   CodePrepareUnsupportedPlatform,
		Class:  "platform",
		Detail: "workspace mutation is supported only on non-mobile linux and darwin",
	}
}

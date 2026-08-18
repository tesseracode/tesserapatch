//go:build android || ios || (!linux && !darwin)

package intentlock

// AuthoritySupported reports whether this target provides real directory
// authority.
const AuthoritySupported = false

// Acquire fails before opening the workspace on unsupported targets.
func Acquire(string) (*WorkspaceAuthority, error) {
	return nil, &Error{
		Code:   CodePrepareUnsupportedPlatform,
		Class:  "platform",
		Detail: "workspace mutation is supported only on non-mobile linux and darwin",
	}
}

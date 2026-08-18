package intentlock

import "fmt"

// Code is a stable authority refusal code.
type Code string

const (
	CodeTransactionInProgress                 Code = "transaction-in-progress"
	CodeLockFilesystemUnsupported             Code = "lock-filesystem-unsupported"
	CodeDirectoryFlockUnavailable             Code = "directory-flock-unavailable"
	CodePrepareUnsupportedPlatform            Code = "prepare-unsupported-platform"
	CodeWorkspaceRootChanged                  Code = "workspace-root-changed"
	CodeWorkspaceRootReplacedAfterPublication Code = "workspace-root-replaced-after-publication"
)

// Error is the path-safe, typed failure returned by this package.
type Error struct {
	Code   Code
	Class  string
	Detail string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Class == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Detail)
	}
	return fmt.Sprintf("%s (%s): %s", e.Code, e.Class, e.Detail)
}

func authorityError(class, detail string) error {
	return &Error{
		Code:   CodeDirectoryFlockUnavailable,
		Class:  class,
		Detail: detail,
	}
}

func sanitizeFilesystemClass(class string) string {
	if class == "" || len(class) > 64 {
		return "unknown"
	}
	for _, character := range class {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '.', character == '_', character == '-':
		default:
			return "unknown"
		}
	}
	return class
}

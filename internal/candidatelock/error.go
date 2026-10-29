package candidatelock

import (
	"errors"
	"fmt"
)

// Code is a stable refusal class for candidate locking.
type Code string

const (
	CodeCandidateLockHeld         Code = "candidate-lock-held"
	CodeLockUnsupported           Code = "lock-unsupported"
	CodeLockFilesystemUnsupported Code = "lock-filesystem-unsupported"
)

// Error is the typed lock refusal returned by this package.
type Error struct {
	Code   Code
	Detail string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

// IsCode reports whether err carries the requested refusal code.
func IsCode(err error, code Code) bool {
	var lockErr *Error
	return errors.As(err, &lockErr) && lockErr.Code == code
}

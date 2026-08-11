// Named refusal taxonomy for resource capture
// (PRD-feature-resource-claims-and-capture-adapters §11's exit-code
// table, ADR-033).
//
// Every refusal this design defines has exactly one name and exactly
// one exit code. The CLI turns a *Refusal into an *cli.ExitCodeError so
// the 1/2/3 semantics are real at the process boundary.

package rescap

import "fmt"

// Exit codes (§11).
const (
	// ExitInternal is exit 1: internal/host faults and data-integrity
	// conditions.
	ExitInternal = 1
	// ExitValidation is exit 2: input the caller can fix.
	ExitValidation = 2
	// ExitRefusal is exit 3: a state/policy refusal.
	ExitRefusal = 3
)

// Exit-1 reason names.
const (
	ReasonTrackedBatchMissing        = "tracked-batch-missing"
	ReasonAdapterCopyFailed          = "adapter-copy-failed"
	ReasonAdapterProcessObserverFail = "adapter-process-observer-failed"
	ReasonAdapterGroupSignalFailed   = "adapter-group-signal-failed"
	ReasonAdapterReapTimeout         = "adapter-reap-timeout"
	ReasonAdapterOutputReadFailed    = "adapter-output-read-failed"
	ReasonNoResourcesDeclared        = "no-resources-declared"
	ReasonResourceDomainIncomplete   = "resource-domain-incomplete"
)

// Exit-2 reason names.
const (
	ReasonDoltArgumentRefused     = "dolt-argument-refused"
	ReasonDoltTrustFlagRequired   = "dolt-trust-flag-required"
	ReasonAdapterMissingAtAdd     = "adapter-missing-at-add"
	ReasonDoltContractUnsupported = "dolt-contract-unsupported"
	ReasonResourceNotDoltAdapter  = "resource-not-dolt-adapter"
	ReasonInvalidDeclaration      = "invalid-declaration"
)

// Exit-3 reason names.
const (
	ReasonNotIgnored                = "not-ignored"
	ReasonTrackedAndIgnored         = "tracked-and-ignored"
	ReasonGitIgnoreCheckError       = "git-ignore-check-error"
	ReasonGitLsFilesError           = "git-ls-files-error"
	ReasonSymlinkComponentRefused   = "symlink-component-refused"
	ReasonPathMissing               = "path-missing"
	ReasonPathReplacedDuringOpen    = "path-replaced-during-open"
	ReasonPathOutsideRepo           = "path-outside-repo"
	ReasonResourceLimitExceeded     = "resource-limit-exceeded"
	ReasonRedactionRefused          = "redaction-refused"
	ReasonAdapterMissing            = "adapter-missing"
	ReasonAdapterExecutableInRepo   = "adapter-executable-in-repo"
	ReasonAdapterBinaryUntrusted    = "adapter-binary-untrusted"
	ReasonDoltTrustRequired         = "dolt-trust-required"
	ReasonAdapterCopyNoexec         = "adapter-copy-noexec"
	ReasonDBPathIdentityChanged     = "db-path-identity-changed"
	ReasonDoltQueryError            = "dolt-query-error"
	ReasonDoltJSONParseError        = "dolt-json-parse-error"
	ReasonLocalRootNotIgnored       = "local-root-not-ignored"
	ReasonLocalPathTracked          = "local-path-tracked"
	ReasonCaptureInProgress         = "capture-in-progress"
	ReasonResourceLockUnsupported   = "resource-lock-unsupported"
	ReasonResourceLockFSUnsupported = "resource-lock-filesystem-unsupported"
	ReasonBatchIDCollision          = "batch-id-collision"
	ReasonBatchFileCorrupt          = "batch-file-corrupt"
	ReasonResourcesFileCorrupt      = "resources-file-corrupt"
	ReasonResourceIDCollision       = "resource-id-collision"
	ReasonIndexEntryMissing         = "index-entry-missing"
	ReasonAdapterDrainTimeout       = "adapter-drain-timeout"
	ReasonNoSuchResource            = "no-such-resource"
	ReasonAmbiguousResourcePrefix   = "ambiguous-resource-prefix"
)

// Refusal is a named outcome carrying its binding exit code.
type Refusal struct {
	Reason string
	Code   int
	Detail string
	Cause  error
}

// Error satisfies the error interface.
func (r *Refusal) Error() string {
	if r == nil {
		return ""
	}
	if r.Detail == "" {
		return r.Reason
	}
	return fmt.Sprintf("%s: %s", r.Reason, r.Detail)
}

// Unwrap exposes an underlying cause for errors.Is/As.
func (r *Refusal) Unwrap() error {
	if r == nil {
		return nil
	}
	return r.Cause
}

// ExitCode returns the binding process exit code.
func (r *Refusal) ExitCode() int {
	if r == nil {
		return 0
	}
	return r.Code
}

// Internal builds an exit-1 refusal.
func Internal(reason, format string, args ...any) *Refusal {
	return &Refusal{Reason: reason, Code: ExitInternal, Detail: fmt.Sprintf(format, args...)}
}

// Invalid builds an exit-2 validation refusal.
func Invalid(reason, format string, args ...any) *Refusal {
	return &Refusal{Reason: reason, Code: ExitValidation, Detail: fmt.Sprintf(format, args...)}
}

// Refuse builds an exit-3 state/policy refusal.
func Refuse(reason, format string, args ...any) *Refusal {
	return &Refusal{Reason: reason, Code: ExitRefusal, Detail: fmt.Sprintf(format, args...)}
}

// AsRefusal unwraps a chain looking for a *Refusal.
func AsRefusal(err error) *Refusal {
	for err != nil {
		if r, ok := err.(*Refusal); ok {
			return r
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return nil
		}
		err = u.Unwrap()
	}
	return nil
}

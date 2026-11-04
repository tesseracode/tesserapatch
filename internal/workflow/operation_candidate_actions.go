package workflow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// OperationCandidateActionError carries a stable terminal-action refusal code.
type OperationCandidateActionError struct {
	Code   string
	Detail string
	Err    error
}

func (e *OperationCandidateActionError) Error() string {
	if e == nil {
		return ""
	}
	message := e.Code
	if e.Detail != "" {
		message += ": " + e.Detail
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *OperationCandidateActionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// OperationCandidateActionCode extracts a terminal-action refusal code.
func OperationCandidateActionCode(err error) (string, bool) {
	var actionErr *OperationCandidateActionError
	if errors.As(err, &actionErr) {
		return actionErr.Code, true
	}
	for _, code := range []candidatelock.Code{
		candidatelock.CodeCandidateLockHeld,
		candidatelock.CodeLockUnsupported,
		candidatelock.CodeLockFilesystemUnsupported,
	} {
		if candidatelock.IsCode(err, code) {
			return string(code), true
		}
	}
	return "", false
}

func candidateActionError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := OperationCandidateActionCode(err); ok {
		return err
	}
	message := err.Error()
	code := message
	if index := strings.IndexByte(code, ':'); index >= 0 {
		code = code[:index]
	}
	switch code {
	case "candidate-pending", "shadow-pending", "candidate-absent",
		"candidate-not-pending", "candidate-already-accepted",
		"candidate-feature-state-changed", "head-diverged",
		"candidate-cas-failed", "candidate-path-occupied",
		"candidate-binding-changed", "candidate-test-failed",
		"candidate-lock-held", "lock-unsupported",
		"lock-filesystem-unsupported", "recovery-required",
		"coverage-publication-failed", "acceptance-rolled-back",
		"acceptance-rollback-failed":
		detail := strings.TrimSpace(strings.TrimPrefix(message, code))
		detail = strings.TrimSpace(strings.TrimPrefix(detail, ":"))
		return &OperationCandidateActionError{Code: code, Detail: detail}
	default:
		return err
	}
}

func candidateActionErrorForSlug(err error, slug string) error {
	err = candidateActionError(err)
	code, ok := OperationCandidateActionCode(err)
	if !ok || code != "shadow-pending" {
		return err
	}
	return &OperationCandidateActionError{
		Code:   code,
		Detail: fmt.Sprintf("run `tpatch reconcile --accept %s` or `--reject %s` first", slug, slug),
	}
}

// DiffOperationCandidate returns the persisted review diff without mutation.
func DiffOperationCandidate(s *store.Store, slug string) ([]byte, error) {
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return nil, candidateActionError(err)
	}
	if refusal.Reason != "" {
		return nil, &OperationCandidateActionError{Code: refusal.Reason}
	}
	defer lock.Release()

	if err := operationCandidateJournalPreflightHeld(s, slug, lock); err != nil {
		return nil, err
	}
	reference, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return nil, err
	}
	defer reference.Close()
	_, pointer, err := operationCandidateActionPreflight(s, slug, "candidate-action", reference, true)
	if err != nil {
		return nil, candidateActionErrorForSlug(err, slug)
	}
	_, state, root, upstream, err := readOperationCandidateArtifacts(s.Root, reference, slug, pointer)
	if err != nil {
		return nil, candidateActionError(err)
	}
	defer upstream.Close()
	defer root.Close()
	if state.Status != candidate.CandidateStatusPending {
		return nil, &OperationCandidateActionError{Code: "candidate-not-pending"}
	}
	patch, err := root.ReadPatch()
	if err != nil {
		return nil, err
	}
	return patch, nil
}

// RejectOperationCandidate records the rejection, prunes the worktree and
// clears the live pointer.
func RejectOperationCandidate(s *store.Store, slug string) (string, error) {
	if err := refuseOperationCandidateJournal(s, slug); err != nil {
		return "", err
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return "", err
	}
	if pointer := status.Reconcile.OperationCandidate; pointer != nil {
		reference, err := openOperationCandidateReference(s.Root, "")
		if err != nil {
			return "", err
		}
		_, state, readErr := reference.ReadCandidateMetadata(slug, pointer.CandidateID)
		closeErr := reference.Close()
		if readErr != nil {
			return "", candidateActionError(readErr)
		}
		if closeErr != nil {
			return "", closeErr
		}
		if state.Status != candidate.CandidateStatusPending {
			return "", &OperationCandidateActionError{Code: "candidate-not-pending"}
		}
	}
	id, err := rejectPendingOperationCandidate(s, slug)
	return id, candidateActionErrorForSlug(err, slug)
}

// ClearOperationCandidateRejections truncates only the slug's rejection log.
func ClearOperationCandidateRejections(s *store.Store, slug string) error {
	return candidateActionErrorForSlug(clearOperationCandidateRejectionsLocked(s, slug), slug)
}

// RefuseResolverActionWhenCandidatePending preserves resolver-only terminal
// actions instead of silently routing them to an operation candidate.
func RefuseResolverActionWhenCandidatePending(s *store.Store, slug string) error {
	if err := refuseReconcileCandidateRecovery(s, []string{slug}); err != nil {
		return candidateActionError(err)
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return err
	}
	if operationCandidateCoexistenceRefusal(status, "resolver-action") == "" {
		return nil
	}
	return &OperationCandidateActionError{
		Code:   "candidate-pending",
		Detail: fmt.Sprintf("run `tpatch reconcile --reject-candidate %s` or `tpatch reconcile --accept-candidate %s` first", slug, slug),
	}
}

// AcceptOperationCandidate runs the explicit, journaled acceptance transaction.
func AcceptOperationCandidate(s *store.Store, slug string) error {
	return candidateActionErrorForSlug(acceptOperationCandidate(s, slug), slug)
}

// ResumeOperationCandidateRecovery is the only marker-driven recovery entry.
func ResumeOperationCandidateRecovery(s *store.Store, slug string) error {
	return candidateActionError(resumeOperationCandidateRecovery(s, slug))
}

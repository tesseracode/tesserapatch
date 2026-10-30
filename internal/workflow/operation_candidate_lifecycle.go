package workflow

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type operationCandidatePreflightResult struct {
	DerivationEnabled bool
	Reason            string
	RecoveryPath      string
	StaleCandidateID  string
}

type operationCandidateReuseResult struct {
	Reusable    bool
	RefusalCode string
}

type operationCandidatePrepared struct {
	Preparation        candidate.CandidatePreparation
	CandidateID        string
	SourceFeatureState string
	ReferenceCommit    string
	UpstreamCommit     string
}

var operationCandidateLifecycleInspectHook = func() {}

func runOperationCandidatePreflight(
	s *store.Store,
	slug string,
	current candidate.CandidateStalenessInput,
) (operationCandidatePreflightResult, error) {
	lock, lockResult, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	if lockResult.Reason != "" {
		return lockResult, nil
	}
	defer func() { _ = lock.Release() }()
	operationCandidateLifecycleInspectHook()

	reference, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	defer func() { _ = reference.Close() }()

	recoveryPath, err := findOperationCandidateRecoveryJournal(reference, slug)
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	if recoveryPath != "" {
		return operationCandidatePreflightResult{
			Reason:       "recovery-required",
			RecoveryPath: recoveryPath,
		}, nil
	}

	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	if status.Reconcile.OperationCandidate != nil {
		staleID, err := stalePendingOperationCandidate(s, slug, status, current, lock)
		if err != nil {
			return operationCandidatePreflightResult{}, err
		}
		if staleID != "" {
			status, err = s.LoadFeatureStatus(slug)
			if err != nil {
				return operationCandidatePreflightResult{}, err
			}
		}
		if status.State != store.StateApplied && status.State != store.StateBlocked {
			return operationCandidatePreflightResult{
				Reason:           "candidate-feature-state-unsupported",
				StaleCandidateID: staleID,
			}, nil
		}
		return operationCandidatePreflightResult{
			DerivationEnabled: true,
			StaleCandidateID:  staleID,
		}, nil
	}
	if status.State != store.StateApplied && status.State != store.StateBlocked {
		return operationCandidatePreflightResult{Reason: "candidate-feature-state-unsupported"}, nil
	}
	return operationCandidatePreflightResult{DerivationEnabled: true}, nil
}

func stalePendingOperationCandidate(
	s *store.Store,
	slug string,
	status store.FeatureStatus,
	current candidate.CandidateStalenessInput,
	featureLock *candidatelock.Handle,
) (string, error) {
	pointer := store.CloneOperationCandidateSummary(status.Reconcile.OperationCandidate)
	if err := store.ValidateOperationCandidateSummary(pointer); err != nil {
		return "", err
	}
	if pointer == nil || pointer.Status == candidate.CandidateStatusAccepted || pointer.Status == candidate.CandidateStatusRejected {
		return "", nil
	}
	if pointer.Status != candidate.CandidateStatusPending {
		return "", nil
	}
	if status.Reconcile.ShadowPath != "" {
		return "", fmt.Errorf("shadow-pending")
	}
	reference, err := openOperationCandidateReference(s.Root, current.UpstreamCommit)
	if err != nil {
		return "", err
	}
	defer func() { _ = reference.Close() }()
	artifact, state, root, err := readOperationCandidateArtifacts(reference, slug, pointer)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	if state.Status == candidate.CandidateStatusStale {
		releaseMutation, err := borrowOperationCandidateFeatureMutation(featureLock, s.Root, slug)
		if err != nil {
			return "", err
		}
		defer releaseMutation()
		if err := pruneOperationCandidateWorktree(s.Root, root, featureLock); err != nil {
			return "", err
		}
		status.Reconcile.OperationCandidate = nil
		if err := s.SaveFeatureStatus(status); err != nil {
			return "", err
		}
		return artifact.CandidateID, nil
	}
	if state.Status != candidate.CandidateStatusPending {
		return "", nil
	}
	current.SourceFeatureState = string(status.State)
	needsStale, err := candidate.CandidateNeedsStaleMark(artifact, state, current)
	if err != nil {
		return "", err
	}
	if !needsStale {
		return "", nil
	}
	releaseMutation, err := borrowOperationCandidateFeatureMutation(featureLock, s.Root, slug)
	if err != nil {
		return "", err
	}
	defer releaseMutation()
	state.Status = candidate.CandidateStatusStale
	if err := root.WriteState(state); err != nil {
		return "", err
	}
	if err := pruneOperationCandidateWorktree(s.Root, root, featureLock); err != nil {
		return "", err
	}
	status.Reconcile.OperationCandidate = nil
	if err := s.SaveFeatureStatus(status); err != nil {
		return "", err
	}
	return artifact.CandidateID, nil
}

func acquireOperationCandidateFeatureLock(repoRoot, slug string) (*candidatelock.Handle, operationCandidatePreflightResult, error) {
	lockPath := filepath.Join(repoRoot, ".tpatch", "locks", "reconcile", slug+".lock")
	lock, err := candidatelock.TryAcquire(lockPath)
	if err == nil {
		return lock, operationCandidatePreflightResult{}, nil
	}
	switch {
	case candidatelock.IsCode(err, candidatelock.CodeCandidateLockHeld):
		return nil, operationCandidatePreflightResult{Reason: "candidate-lock-held"}, nil
	case candidatelock.IsCode(err, candidatelock.CodeLockUnsupported):
		return nil, operationCandidatePreflightResult{Reason: "lock-unsupported"}, nil
	case candidatelock.IsCode(err, candidatelock.CodeLockFilesystemUnsupported):
		return nil, operationCandidatePreflightResult{Reason: "lock-filesystem-unsupported"}, nil
	default:
		return nil, operationCandidatePreflightResult{}, err
	}
}

func findOperationCandidateRecoveryJournal(reference *candidate.ReferenceRoot, slug string) (string, error) {
	return reference.FindCandidateRecoveryJournal(slug)
}

func readOperationCandidateArtifacts(reference *candidate.ReferenceRoot, slug string, pointer *store.OperationCandidateSummary) (candidate.CandidateArtifact, candidate.CandidateState, *candidate.CandidateRoot, error) {
	if err := store.ValidateOperationCandidateSummary(pointer); err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, err
	}
	expectedPath, err := candidate.CandidatePointerPath(slug, pointer.CandidateID)
	if err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, err
	}
	if pointer.Path != expectedPath {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, fmt.Errorf("candidate pointer path mismatch: have %s want %s", pointer.Path, expectedPath)
	}
	root, err := candidate.OpenExistingCandidateRoot(reference, slug, pointer.CandidateID)
	if err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, err
	}
	artifact, err := root.ReadArtifact()
	if err != nil {
		_ = root.Close()
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, err
	}
	state, err := root.ReadState()
	if err != nil {
		_ = root.Close()
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, err
	}
	if artifact.CandidateID != pointer.CandidateID || state.CandidateID != pointer.CandidateID {
		_ = root.Close()
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, fmt.Errorf("candidate pointer/id mismatch")
	}
	if pointer.SourceFeatureState != artifact.SourceFeatureState {
		_ = root.Close()
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, fmt.Errorf("candidate pointer state drift")
	}
	if pointer.Status != state.Status &&
		!(pointer.Status == candidate.CandidateStatusPending && (state.Status == candidate.CandidateStatusStale || state.Status == candidate.CandidateStatusRejected)) {
		_ = root.Close()
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, fmt.Errorf("candidate pointer state drift")
	}
	return artifact, state, root, nil
}

func pruneOperationCandidateWorktree(repoRoot string, root *candidate.CandidateRoot, featureLock *candidatelock.Handle) error {
	if root == nil {
		return fmt.Errorf("candidate root is required")
	}
	globalPath := filepath.Join(repoRoot, ".tpatch", "locks", "worktree-mutation.lock")
	global, err := candidatelock.TryAcquire(globalPath)
	if err != nil {
		return err
	}
	pruneErr := root.Prune(featureLock, global)
	releaseErr := global.Release()
	if pruneErr != nil {
		return pruneErr
	}
	return releaseErr
}

func operationCandidateReuseCheck(preparation candidate.CandidatePreparation, existingArtifact candidate.CandidateArtifact, existingState candidate.CandidateState, derived candidate.CandidateArtifact, root *candidate.CandidateRoot) (operationCandidateReuseResult, error) {
	decision, err := candidate.CompareExistingCandidate(existingArtifact, existingState, derived)
	if err != nil {
		return operationCandidateReuseResult{}, err
	}
	if !decision.Reusable {
		return operationCandidateReuseResult{RefusalCode: decision.RefusalCode}, nil
	}
	if root == nil {
		return operationCandidateReuseResult{}, fmt.Errorf("reusable candidate root is required")
	}
	result, err := preparation.ValidateReusableCandidate(root)
	if err != nil {
		return operationCandidateReuseResult{}, err
	}
	if !reflect.DeepEqual(result, existingArtifact.Result) {
		return operationCandidateReuseResult{RefusalCode: "candidate-integrity-drift"}, nil
	}
	return operationCandidateReuseResult{Reusable: true}, nil
}

func appendRejectedOperationCandidateWithReference(reference *candidate.ReferenceRoot, slug string, artifact candidate.CandidateArtifact) error {
	record := candidate.RejectedRecordFromArtifact(artifact)
	return reference.AppendRejectedRecord(slug, record)
}

func appendRejectedOperationCandidate(repoRoot, slug string, artifact candidate.CandidateArtifact) error {
	reference, err := openOperationCandidateReference(repoRoot, artifact.UpstreamCommit)
	if err != nil {
		return err
	}
	defer func() { _ = reference.Close() }()
	return appendRejectedOperationCandidateWithReference(reference, slug, artifact)
}

func readRejectedOperationCandidates(reference *candidate.ReferenceRoot, slug string) ([]candidate.CandidateRejectedRecord, error) {
	return reference.ReadRejectedRecords(slug)
}

func isOperationCandidateRejected(reference *candidate.ReferenceRoot, slug, candidateID string) (bool, error) {
	rows, err := readRejectedOperationCandidates(reference, slug)
	if err != nil {
		return false, err
	}
	return candidate.RejectedContainsCandidateID(rows, candidateID), nil
}

func clearOperationCandidateRejections(reference *candidate.ReferenceRoot, slug string) error {
	return reference.ClearRejectedRecords(slug)
}

func rejectPendingOperationCandidate(s *store.Store, slug string) (string, error) {
	lock, lockResult, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return "", err
	}
	if lockResult.Reason != "" {
		return "", fmt.Errorf("%s", lockResult.Reason)
	}
	defer func() { _ = lock.Release() }()
	reference, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return "", err
	}
	defer func() { _ = reference.Close() }()
	status, pointer, err := operationCandidateActionPreflight(s, slug, "candidate-action", reference, true)
	if err != nil {
		return "", err
	}
	releaseMutation, err := borrowOperationCandidateFeatureMutation(lock, s.Root, slug)
	if err != nil {
		return "", err
	}
	defer releaseMutation()
	artifact, state, root, err := readOperationCandidateArtifacts(reference, slug, pointer)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()

	if state.Status == candidate.CandidateStatusAccepted {
		return "", fmt.Errorf("candidate-not-pending")
	}
	if state.Status != candidate.CandidateStatusPending && state.Status != candidate.CandidateStatusStale && state.Status != candidate.CandidateStatusRejected {
		return "", fmt.Errorf("candidate-not-pending")
	}
	if state.Status != candidate.CandidateStatusRejected {
		rejected, err := isOperationCandidateRejected(reference, slug, artifact.CandidateID)
		if err != nil {
			return "", err
		}
		if !rejected {
			if err := appendRejectedOperationCandidate(s.Root, slug, artifact); err != nil {
				return "", err
			}
		}
		state.Status = candidate.CandidateStatusRejected
		if err := root.WriteState(state); err != nil {
			return "", err
		}
	}
	if err := pruneOperationCandidateWorktree(s.Root, root, lock); err != nil {
		return "", err
	}
	status.Reconcile.OperationCandidate = nil
	if err := s.SaveFeatureStatus(status); err != nil {
		return "", err
	}
	return artifact.CandidateID, nil
}

func clearOperationCandidateRejectionsLocked(s *store.Store, slug string) error {
	lock, lockResult, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return err
	}
	if lockResult.Reason != "" {
		return fmt.Errorf("%s", lockResult.Reason)
	}
	defer func() { _ = lock.Release() }()
	reference, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return err
	}
	defer func() { _ = reference.Close() }()
	if _, _, err := operationCandidateActionPreflight(s, slug, "candidate-action", reference, false); err != nil {
		return err
	}
	releaseMutation, err := borrowOperationCandidateFeatureMutation(lock, s.Root, slug)
	if err != nil {
		return err
	}
	defer releaseMutation()
	return clearOperationCandidateRejections(reference, slug)
}

func prepareOperationCandidate(
	s *store.Store,
	slug string,
	eligibility operationCandidateEligibility,
	patch string,
	effects []gitutil.PatchEffect,
	effectSHA256ByOrdinal map[int]string,
) (operationCandidatePrepared, error) {
	lock, lockResult, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	if lockResult.Reason != "" {
		return operationCandidatePrepared{}, fmt.Errorf("%s", lockResult.Reason)
	}
	defer func() { _ = lock.Release() }()

	reference, err := openOperationCandidateReference(s.Root, eligibility.referenceCommit)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	defer func() { _ = reference.Close() }()
	if recoveryPath, err := findOperationCandidateRecoveryJournal(reference, slug); err != nil {
		return operationCandidatePrepared{}, err
	} else if recoveryPath != "" {
		return operationCandidatePrepared{}, fmt.Errorf("recovery-required")
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	if refusal := operationCandidateCoexistenceRefusal(status, "candidate-derivation"); refusal != "" {
		return operationCandidatePrepared{}, fmt.Errorf("%s", refusal)
	}
	if status.State != store.StateApplied && status.State != store.StateBlocked {
		return operationCandidatePrepared{}, fmt.Errorf("candidate-feature-state-unsupported")
	}
	upstream, err := openOperationCandidateReference(s.Root, eligibility.upstreamCommit)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	defer func() { _ = upstream.Close() }()
	preparation, err := candidate.PrepareCandidateArtifact(candidate.CandidatePrepareInput{
		Feature:               slug,
		SourceFeatureState:    string(status.State),
		UpstreamCommit:        eligibility.upstreamCommit,
		ReferenceCommit:       eligibility.referenceCommit,
		Bindings:              operationCandidateBindings(eligibility),
		Reference:             reference,
		Upstream:              upstream,
		Patch:                 patch,
		Effects:               effects,
		EffectSHA256ByOrdinal: effectSHA256ByOrdinal,
	})
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	candidateID, err := preparation.CandidateID()
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	rejected, err := isOperationCandidateRejected(reference, slug, candidateID)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	if rejected {
		return operationCandidatePrepared{}, fmt.Errorf("candidate-rejected")
	}
	return operationCandidatePrepared{
		Preparation:        preparation,
		CandidateID:        candidateID,
		SourceFeatureState: string(status.State),
		ReferenceCommit:    eligibility.referenceCommit,
		UpstreamCommit:     eligibility.upstreamCommit,
	}, nil
}

func publishOperationCandidate(
	s *store.Store,
	slug string,
	prepared operationCandidatePrepared,
	execution candidate.ExecutionResult,
) (candidate.CandidateArtifact, error) {
	lock, lockResult, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if lockResult.Reason != "" {
		return candidate.CandidateArtifact{}, fmt.Errorf("%s", lockResult.Reason)
	}
	defer func() { _ = lock.Release() }()

	reference, err := openOperationCandidateReference(s.Root, prepared.ReferenceCommit)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer func() { _ = reference.Close() }()
	if _, _, err := operationCandidateActionPreflight(s, slug, "candidate-derivation", reference, false); err != nil {
		return candidate.CandidateArtifact{}, err
	}
	releaseMutation, err := borrowOperationCandidateFeatureMutation(lock, s.Root, slug)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer releaseMutation()

	artifact, err := candidate.BuildCandidateArtifact(candidate.CandidateMaterializeInput{
		Preparation: prepared.Preparation,
		Execution:   execution,
	})
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if execution.Root == nil {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate execution root is required")
	}
	if artifact.CandidateID != prepared.CandidateID {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate id mismatch while publishing")
	}
	if err := execution.Root.WriteArtifact(artifact); err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if err := execution.Root.WritePatch([]byte(execution.Diff)); err != nil {
		return candidate.CandidateArtifact{}, err
	}
	state := candidate.CandidateState{
		SchemaVersion:    candidate.CandidateSchemaVersion,
		CandidateID:      artifact.CandidateID,
		Status:           candidate.CandidateStatusPending,
		AcceptValidation: candidate.CandidateAcceptValidationNone,
	}
	if err := execution.Root.WriteState(state); err != nil {
		return candidate.CandidateArtifact{}, err
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	path, err := candidate.CandidatePointerPath(slug, artifact.CandidateID)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	status.Reconcile.OperationCandidate = &store.OperationCandidateSummary{
		CandidateID:        artifact.CandidateID,
		Path:               path,
		Status:             candidate.CandidateStatusPending,
		SourceFeatureState: artifact.SourceFeatureState,
	}
	if err := s.SaveFeatureStatus(status); err != nil {
		return candidate.CandidateArtifact{}, err
	}
	return artifact, nil
}

func operationCandidateCoexistenceRefusal(status store.FeatureStatus, action string) string {
	hasCandidate := status.Reconcile.OperationCandidate != nil &&
		status.Reconcile.OperationCandidate.Status == candidate.CandidateStatusPending
	hasShadow := status.Reconcile.ShadowPath != ""
	switch action {
	case "resolver-action":
		if hasCandidate {
			return "candidate-pending"
		}
	case "candidate-action", "candidate-derivation":
		if hasShadow {
			return "shadow-pending"
		}
	}
	return ""
}

func borrowOperationCandidateFeatureMutation(lock *candidatelock.Handle, repoRoot, slug string) (func(), error) {
	if lock == nil {
		return nil, fmt.Errorf("feature lock handle is required")
	}
	lockPath := filepath.Join(repoRoot, ".tpatch", "locks", "reconcile", slug+".lock")
	return lock.Borrow(lockPath)
}

func openOperationCandidateReference(repoRoot, commitHint string) (*candidate.ReferenceRoot, error) {
	commit := strings.TrimSpace(commitHint)
	if commit == "" {
		head, err := repoHEADCommit(repoRoot)
		if err != nil {
			return nil, err
		}
		commit = head
	}
	reference, err := candidate.OpenReferenceRoot(repoRoot, commit)
	if err != nil && commitHint == "" {
		head, headErr := repoHEADCommit(repoRoot)
		if headErr != nil {
			return nil, err
		}
		return candidate.OpenReferenceRoot(repoRoot, head)
	}
	return reference, err
}

func repoHEADCommit(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot
	cmd.Env = gitutil.HardenedNoLazyGitEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository HEAD commit: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	commit := strings.TrimSpace(string(out))
	if commit == "" {
		return "", fmt.Errorf("resolve repository HEAD commit: empty output")
	}
	return commit, nil
}

func operationCandidateActionPreflight(s *store.Store, slug, action string, reference *candidate.ReferenceRoot, requirePointer bool) (store.FeatureStatus, *store.OperationCandidateSummary, error) {
	recoveryPath, err := findOperationCandidateRecoveryJournal(reference, slug)
	if err != nil {
		return store.FeatureStatus{}, nil, err
	}
	if recoveryPath != "" {
		return store.FeatureStatus{}, nil, fmt.Errorf("recovery-required")
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return store.FeatureStatus{}, nil, err
	}
	if refusal := operationCandidateCoexistenceRefusal(status, action); refusal != "" {
		return store.FeatureStatus{}, nil, fmt.Errorf("%s", refusal)
	}
	pointer := store.CloneOperationCandidateSummary(status.Reconcile.OperationCandidate)
	if err := store.ValidateOperationCandidateSummary(pointer); err != nil {
		return store.FeatureStatus{}, nil, err
	}
	if requirePointer && pointer == nil {
		return store.FeatureStatus{}, nil, fmt.Errorf("candidate-absent")
	}
	return status, pointer, nil
}

func operationCandidateBindings(eligibility operationCandidateEligibility) candidate.CandidateBindings {
	return candidate.CandidateBindings{
		CaptureEventSHA256: eligibility.captureEventSHA256,
		CoverageSHA256:     eligibility.coverageSHA256,
		PatchSHA256:        eligibility.patchSHA256,
		RecipeSHA256:       eligibility.recipeSHA256,
	}
}

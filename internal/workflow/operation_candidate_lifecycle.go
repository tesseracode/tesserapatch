package workflow

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
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

// Only prepareOperationCandidateHeld mints this in-process value. Finalization
// recomputes it under the same hold used for execution and publication.
type operationCandidatePrepared struct {
	owner        *store.Store
	repoPath     string
	repoIdentity os.FileInfo
	slug         string
	invocation   operationCandidateInvocation
	eligibility  operationCandidateEligibility
	derivation   candidate.CandidateDerivation
	candidateID  string
}

func (operationCandidatePrepared) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("candidate preparation is ephemeral")
}

var operationCandidateLifecycleInspectHook = func() {}

func runOperationCandidatePreflight(s *store.Store, slug string, current candidate.CandidateStalenessInput) (operationCandidatePreflightResult, error) {
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil || refusal.Reason != "" {
		return refusal, err
	}
	defer lock.Release()
	return runOperationCandidatePreflightHeld(s, slug, current, lock, false)
}

func runReconcileCandidatePreflight(s *store.Store, slug, upstreamCommit string) (operationCandidatePreflightResult, error) {
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil || refusal.Reason != "" {
		return refusal, err
	}
	defer lock.Release()
	return runOperationCandidatePreflightHeld(s, slug, candidate.CandidateStalenessInput{UpstreamCommit: upstreamCommit}, lock, true)
}

func runOperationCandidatePreflightHeld(s *store.Store, slug string, current candidate.CandidateStalenessInput, lock *candidatelock.Handle, snapshotBindings bool) (operationCandidatePreflightResult, error) {
	release, err := borrowOperationCandidateFeatureMutation(lock, s.Root, slug)
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	defer release()
	operationCandidateLifecycleInspectHook()
	reference, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	defer reference.Close()
	recovery, err := findOperationCandidateRecoveryJournal(reference, slug)
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	if recovery != "" {
		return operationCandidatePreflightResult{Reason: "recovery-required", RecoveryPath: recovery}, nil
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	if err := inspectOperationCandidateInventory(reference, slug, status.Reconcile.OperationCandidate); err != nil {
		return operationCandidatePreflightResult{}, err
	}
	if snapshotBindings && status.Reconcile.OperationCandidate != nil {
		current.Bindings, err = snapshotOperationCandidateBindings(s, slug)
		if err != nil {
			return operationCandidatePreflightResult{Reason: "capture-event-unusable"}, nil
		}
	}
	staleID, err := stalePendingOperationCandidate(s, slug, status, current, lock)
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	status, err = s.LoadFeatureStatus(slug)
	if err != nil {
		return operationCandidatePreflightResult{}, err
	}
	if status.State != store.StateApplied && status.State != store.StateBlocked {
		return operationCandidatePreflightResult{Reason: "candidate-feature-state-unsupported", StaleCandidateID: staleID}, nil
	}
	return operationCandidatePreflightResult{DerivationEnabled: true, StaleCandidateID: staleID}, nil
}

func inspectOperationCandidateInventory(reference *candidate.ReferenceRoot, slug string, pointer *store.OperationCandidateSummary) error {
	if err := store.ValidateOperationCandidateSummary(pointer); err != nil {
		return err
	}
	if pointer != nil && pointer.Status != candidate.CandidateStatusPending {
		return fmt.Errorf("candidate-integrity-drift: live pointer is not pending")
	}
	rows, err := reference.ListCandidateStates(slug)
	if err != nil {
		return fmt.Errorf("candidate-integrity-drift: %w", err)
	}
	found := false
	for _, row := range rows {
		owned := pointer != nil && pointer.CandidateID == row.CandidateID
		if !owned && (row.Status == candidate.CandidateStatusAccepted || row.Status == candidate.CandidateStatusRejected) {
			continue
		}
		artifact, _, err := reference.ReadCandidateMetadata(slug, row.CandidateID)
		if err != nil {
			return fmt.Errorf("candidate-integrity-drift: %w", err)
		}
		if owned {
			expectedPath, _ := candidate.CandidatePointerPath(slug, row.CandidateID)
			if pointer.Path != expectedPath || pointer.SourceFeatureState != artifact.SourceFeatureState ||
				row.Status == candidate.CandidateStatusAccepted {
				return fmt.Errorf("candidate-integrity-drift: pointer/metadata mismatch")
			}
			found = true
		}
		if row.Status == candidate.CandidateStatusPending && !owned {
			return fmt.Errorf("candidate-integrity-drift: orphan pending candidate %s", row.CandidateID)
		}
		if row.Status == candidate.CandidateStatusStale && !owned {
			present, err := reference.CandidateWorktreePresent(slug, row.CandidateID)
			if err != nil {
				return err
			}
			if present {
				return fmt.Errorf("candidate-integrity-drift: unfinished stale candidate %s", row.CandidateID)
			}
		}
	}
	if pointer != nil && !found {
		return fmt.Errorf("candidate-integrity-drift: pointer has no metadata")
	}
	return nil
}

func stalePendingOperationCandidate(s *store.Store, slug string, status store.FeatureStatus, current candidate.CandidateStalenessInput, featureLock *candidatelock.Handle) (string, error) {
	pointer := store.CloneOperationCandidateSummary(status.Reconcile.OperationCandidate)
	if pointer == nil {
		return "", nil
	}
	release, err := borrowOperationCandidateFeatureMutation(featureLock, s.Root, slug)
	if err != nil {
		return "", err
	}
	defer release()
	reference, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return "", err
	}
	defer reference.Close()
	artifact, state, err := reference.ReadCandidateMetadata(slug, pointer.CandidateID)
	if err != nil {
		return "", err
	}
	if state.Status == candidate.CandidateStatusAccepted {
		return "", fmt.Errorf("candidate-integrity-drift")
	}
	if state.Status == candidate.CandidateStatusRejected {
		records, err := reference.ReadRejectedRecords(slug)
		if err != nil {
			return "", err
		}
		if !slices.Contains(records, candidate.RejectedRecordFromArtifact(artifact)) {
			return "", fmt.Errorf("candidate-integrity-drift: incomplete rejection without its record")
		}
		if err := pruneOperationCandidateIfPresent(s, reference, artifact, featureLock); err != nil {
			return "", err
		}
		status.Reconcile.OperationCandidate = nil
		return artifact.CandidateID, s.SaveFeatureStatus(status)
	}
	current.SourceFeatureState = string(status.State)
	stale := state.Status == candidate.CandidateStatusStale
	if !stale {
		stale, err = candidate.CandidateNeedsStaleMark(artifact, state, current)
		if err != nil {
			return "", err
		}
	}
	if !stale {
		present, err := reference.CandidateWorktreePresent(slug, artifact.CandidateID)
		if err != nil {
			return "", err
		}
		if !present {
			return "", fmt.Errorf("candidate-integrity-drift: pending worktree is absent")
		}
		return "", nil
	}
	if err := pruneOperationCandidateIfPresent(s, reference, artifact, featureLock); err != nil {
		return "", err
	}
	state.Status = candidate.CandidateStatusStale
	if err := reference.WriteCandidateState(slug, artifact.CandidateID, state, featureLock); err != nil {
		return "", err
	}
	status.Reconcile.OperationCandidate = nil
	if err := s.SaveFeatureStatus(status); err != nil {
		return "", err
	}
	return artifact.CandidateID, nil
}

func acquireOperationCandidateFeatureLock(repoRoot, slug string) (*candidatelock.Handle, operationCandidatePreflightResult, error) {
	if _, err := candidate.CandidateSlugRelative(slug); err != nil {
		return nil, operationCandidatePreflightResult{}, err
	}
	lock, err := candidatelock.TryAcquire(filepath.Join(repoRoot, ".tpatch", "locks", "reconcile", slug+".lock"))
	if err == nil {
		return lock, operationCandidatePreflightResult{}, nil
	}
	for _, code := range []candidatelock.Code{candidatelock.CodeCandidateLockHeld, candidatelock.CodeLockUnsupported, candidatelock.CodeLockFilesystemUnsupported} {
		if candidatelock.IsCode(err, code) {
			return nil, operationCandidatePreflightResult{Reason: string(code)}, nil
		}
	}
	return nil, operationCandidatePreflightResult{}, err
}

func findOperationCandidateRecoveryJournal(reference *candidate.ReferenceRoot, slug string) (string, error) {
	return reference.FindCandidateRecoveryJournal(slug)
}

func readOperationCandidateArtifacts(repoRoot string, reference *candidate.ReferenceRoot, slug string, pointer *store.OperationCandidateSummary) (candidate.CandidateArtifact, candidate.CandidateState, *candidate.CandidateRoot, *candidate.ReferenceRoot, error) {
	if err := store.ValidateOperationCandidateSummary(pointer); err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, nil, err
	}
	if pointer == nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, nil, fmt.Errorf("candidate-absent")
	}
	expected, err := candidate.CandidatePointerPath(slug, pointer.CandidateID)
	if err != nil || pointer.Path != expected {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, nil, nil, fmt.Errorf("candidate pointer path mismatch")
	}
	artifact, state, err := reference.ReadCandidateMetadata(slug, pointer.CandidateID)
	if err != nil {
		return artifact, state, nil, nil, err
	}
	if pointer.SourceFeatureState != artifact.SourceFeatureState || pointer.Status != state.Status {
		return artifact, state, nil, nil, fmt.Errorf("candidate pointer state drift")
	}
	upstream, err := openOperationCandidateReference(repoRoot, artifact.UpstreamCommit)
	if err != nil {
		return artifact, state, nil, nil, err
	}
	root, err := candidate.OpenExistingCandidateRoot(upstream, slug, pointer.CandidateID)
	if err != nil {
		_ = upstream.Close()
		return artifact, state, nil, nil, err
	}
	return artifact, state, root, upstream, nil
}

func pruneOperationCandidateWorktree(repoRoot string, root *candidate.CandidateRoot, featureLock *candidatelock.Handle) error {
	if root == nil {
		return fmt.Errorf("candidate root is required")
	}
	global, err := candidatelock.TryAcquire(filepath.Join(repoRoot, ".tpatch", "locks", "worktree-mutation.lock"))
	if err != nil {
		return err
	}
	pruneErr := root.Prune(featureLock, global)
	return errors.Join(pruneErr, global.Release())
}

func pruneOperationCandidateIfPresent(s *store.Store, reference *candidate.ReferenceRoot, artifact candidate.CandidateArtifact, lock *candidatelock.Handle) error {
	present, err := reference.CandidateWorktreePresent(artifact.Feature, artifact.CandidateID)
	if err != nil || !present {
		return err
	}
	upstream, err := openOperationCandidateReference(s.Root, artifact.UpstreamCommit)
	if err != nil {
		return err
	}
	defer upstream.Close()
	root, err := candidate.OpenExistingCandidateRoot(upstream, artifact.Feature, artifact.CandidateID)
	if err != nil {
		return err
	}
	defer root.Close()
	return pruneOperationCandidateWorktree(s.Root, root, lock)
}

func appendRejectedOperationCandidate(repoRoot, slug string, artifact candidate.CandidateArtifact, lock *candidatelock.Handle) error {
	reference, err := openOperationCandidateReference(repoRoot, artifact.UpstreamCommit)
	if err != nil {
		return err
	}
	defer reference.Close()
	return reference.AppendRejectedRecord(slug, candidate.RejectedRecordFromArtifact(artifact), lock)
}

func readRejectedOperationCandidates(reference *candidate.ReferenceRoot, slug string) ([]candidate.CandidateRejectedRecord, error) {
	return reference.ReadRejectedRecords(slug)
}

func isOperationCandidateRejected(reference *candidate.ReferenceRoot, slug, candidateID string) (bool, error) {
	rows, err := reference.ReadRejectedRecords(slug)
	if err != nil {
		return false, err
	}
	return candidate.RejectedContainsCandidateID(rows, candidateID), nil
}

func clearOperationCandidateRejections(reference *candidate.ReferenceRoot, slug string, lock *candidatelock.Handle) error {
	return reference.ClearRejectedRecords(slug, lock)
}

func rejectPendingOperationCandidate(s *store.Store, slug string) (string, error) {
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return "", err
	}
	if refusal.Reason != "" {
		return "", fmt.Errorf("%s", refusal.Reason)
	}
	defer lock.Release()
	release, err := borrowOperationCandidateFeatureMutation(lock, s.Root, slug)
	if err != nil {
		return "", err
	}
	defer release()
	reference, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return "", err
	}
	defer reference.Close()
	status, pointer, err := operationCandidateActionPreflight(s, slug, "candidate-action", reference, true)
	if err != nil {
		return "", err
	}
	artifact, state, err := reference.ReadCandidateMetadata(slug, pointer.CandidateID)
	if err != nil {
		return "", err
	}
	if state.Status == candidate.CandidateStatusAccepted {
		return "", fmt.Errorf("candidate-not-pending")
	}
	// Acquire/prune before changing terminal metadata: global contention must
	// leave the original pending pair and its worktree untouched.
	if err := pruneOperationCandidateIfPresent(s, reference, artifact, lock); err != nil {
		return "", err
	}
	rejected, err := isOperationCandidateRejected(reference, slug, artifact.CandidateID)
	if err != nil {
		return "", err
	}
	if !rejected {
		if err := appendRejectedOperationCandidate(s.Root, slug, artifact, lock); err != nil {
			return "", err
		}
	}
	state.Status = candidate.CandidateStatusRejected
	if err := reference.WriteCandidateState(slug, artifact.CandidateID, state, lock); err != nil {
		return "", err
	}
	status.Reconcile.OperationCandidate = nil
	if err := s.SaveFeatureStatus(status); err != nil {
		return "", err
	}
	return artifact.CandidateID, nil
}

func clearOperationCandidateRejectionsLocked(s *store.Store, slug string) error {
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return err
	}
	if refusal.Reason != "" {
		return fmt.Errorf("%s", refusal.Reason)
	}
	defer lock.Release()
	reference, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return err
	}
	defer reference.Close()
	if _, _, err := operationCandidateActionPreflight(s, slug, "candidate-action", reference, false); err != nil {
		return err
	}
	return clearOperationCandidateRejections(reference, slug, lock)
}

func prepareOperationCandidate(s *store.Store, slug string, invocation operationCandidateInvocation) (operationCandidatePrepared, error) {
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	if refusal.Reason != "" {
		return operationCandidatePrepared{}, fmt.Errorf("%s", refusal.Reason)
	}
	defer lock.Release()
	return prepareOperationCandidateHeld(s, slug, invocation, lock)
}

func prepareOperationCandidateHeld(s *store.Store, slug string, invocation operationCandidateInvocation, lock *candidatelock.Handle) (operationCandidatePrepared, error) {
	release, err := borrowOperationCandidateFeatureMutation(lock, s.Root, slug)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	defer release()
	repoIdentity, err := os.Stat(s.Root)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	metadata, err := openOperationCandidateReference(s.Root, "")
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	defer metadata.Close()
	status, pointer, err := operationCandidateActionPreflight(s, slug, "candidate-derivation", metadata, false)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	if pointer != nil {
		bindings, err := snapshotOperationCandidateBindings(s, slug)
		if err != nil {
			return operationCandidatePrepared{}, err
		}
		current := candidate.CandidateStalenessInput{UpstreamCommit: invocation.upstreamCommit,
			SourceFeatureState: string(status.State), Bindings: bindings}
		if _, err := stalePendingOperationCandidate(s, slug, status, current, lock); err != nil {
			return operationCandidatePrepared{}, err
		}
	}
	eligibility, err := assessOperationCandidateEligibility(s, slug, invocation)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	if !eligibility.enabled || !eligibility.eligible || eligibility.inputs == nil {
		if eligibility.reason != "" {
			return operationCandidatePrepared{}, fmt.Errorf("%s: %s", eligibility.reason, eligibility.detail)
		}
		return operationCandidatePrepared{}, fmt.Errorf("operation candidate derivation is disabled")
	}
	reference, err := openOperationCandidateReference(s.Root, eligibility.referenceCommit)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	defer reference.Close()
	upstream, err := openOperationCandidateReference(s.Root, eligibility.upstreamCommit)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	defer upstream.Close()
	inputs := eligibility.inputs
	derivation, err := candidate.DescribeDerivation(reference, upstream, inputs.patch, inputs.effects, inputs.effectHashes)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	id, _, _, err := candidate.ComputeCandidateID(candidate.CandidateIdentityInput{
		SchemaVersion: candidate.CandidateSchemaVersion, Feature: slug, SourceFeatureState: string(eligibility.sourceFeatureState),
		UpstreamCommit: eligibility.upstreamCommit, ReferenceCommit: eligibility.referenceCommit,
		Bindings: operationCandidateBindings(eligibility), DerivationMode: candidate.CandidateDerivationModeSingle,
		Operations: derivation.Operations, EffectMap: derivation.EffectMap,
	})
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	rejected, err := isOperationCandidateRejected(reference, slug, id)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	if rejected {
		return operationCandidatePrepared{}, fmt.Errorf("candidate-rejected")
	}
	info, err := os.Stat(s.Root)
	if err != nil {
		return operationCandidatePrepared{}, err
	}
	if !os.SameFile(repoIdentity, info) {
		return operationCandidatePrepared{}, fmt.Errorf("candidate preparation repository changed")
	}
	invocation.slugs = slices.Clone(invocation.slugs)
	return operationCandidatePrepared{owner: s, repoPath: s.Root, repoIdentity: info, slug: slug, invocation: invocation,
		eligibility: eligibility, derivation: derivation, candidateID: id}, nil
}

func sameOperationCandidatePreparation(a, b operationCandidatePrepared) bool {
	return a.owner == b.owner && a.repoPath == b.repoPath && a.repoIdentity != nil && b.repoIdentity != nil &&
		os.SameFile(a.repoIdentity, b.repoIdentity) && a.slug == b.slug && a.candidateID == b.candidateID &&
		a.eligibility.sourceFeatureState == b.eligibility.sourceFeatureState &&
		a.eligibility.referenceCommit == b.eligibility.referenceCommit && a.eligibility.upstreamCommit == b.eligibility.upstreamCommit &&
		operationCandidateBindings(a.eligibility) == operationCandidateBindings(b.eligibility)
}

func checkOperationCandidatePreparationRoot(s *store.Store, slug string, prepared operationCandidatePrepared) error {
	if prepared.owner != s || prepared.slug != slug || prepared.repoPath != s.Root || prepared.repoIdentity == nil {
		return fmt.Errorf("candidate preparation repository/feature mismatch")
	}
	info, err := os.Stat(s.Root)
	if err != nil {
		return err
	}
	if !os.SameFile(info, prepared.repoIdentity) {
		return fmt.Errorf("candidate preparation repository identity changed")
	}
	return nil
}

func operationCandidateArtifact(prepared operationCandidatePrepared, execution candidate.ExecutionResult) (candidate.CandidateArtifact, error) {
	treeHash, err := candidate.CandidateTreeIntegritySHA256(execution.TreeHash)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	artifact := candidate.CandidateArtifact{
		SchemaVersion: candidate.CandidateSchemaVersion, CandidateID: prepared.candidateID, Feature: prepared.slug,
		SourceFeatureState: string(prepared.eligibility.sourceFeatureState), UpstreamCommit: prepared.eligibility.upstreamCommit,
		ReferenceCommit: prepared.eligibility.referenceCommit, DerivationMode: candidate.CandidateDerivationModeSingle,
		Bindings: operationCandidateBindings(prepared.eligibility), Derivation: prepared.derivation,
		Result: candidate.CandidateResult{CandidateDiffSHA256: CoverageSHA256([]byte(execution.Diff)),
			CandidateTreeSHA256: treeHash, Validated: candidate.CandidateValidationStructural},
	}
	return artifact, artifact.Validate()
}

func operationCandidateReuseCheck(prepared operationCandidatePrepared, root *candidate.CandidateRoot, lock *candidatelock.Handle) (candidate.CandidateArtifact, error) {
	if prepared.owner == nil || prepared.eligibility.inputs == nil {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate preparation is absent")
	}
	release, err := borrowOperationCandidateFeatureMutation(lock, prepared.repoPath, prepared.slug)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer release()
	if err := checkOperationCandidatePreparationRoot(prepared.owner, prepared.slug, prepared); err != nil {
		return candidate.CandidateArtifact{}, err
	}
	fresh, err := prepareOperationCandidateHeld(prepared.owner, prepared.slug, prepared.invocation, lock)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if !sameOperationCandidatePreparation(prepared, fresh) {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate-integrity-drift")
	}
	relative, err := candidate.CandidateWorktreeRelative(fresh.slug, fresh.candidateID)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	expectedPath, err := filepath.Abs(filepath.Join(fresh.repoPath, filepath.FromSlash(relative)))
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if root == nil || root.CandidateID() != fresh.candidateID || root.Commit() != fresh.eligibility.upstreamCommit ||
		filepath.Clean(root.WorktreePath()) != filepath.Clean(expectedPath) {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate root identity mismatch")
	}
	existing, err := root.ReadArtifact()
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	state, err := root.ReadState()
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if state.Status == candidate.CandidateStatusAccepted {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate-already-accepted")
	}
	if state.Status == candidate.CandidateStatusRejected {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate-rejected")
	}
	if err := matchOperationCandidateIdentity(fresh, existing); err != nil {
		return candidate.CandidateArtifact{}, err
	}
	reference, err := openOperationCandidateReference(prepared.repoPath, prepared.eligibility.referenceCommit)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer reference.Close()
	upstream, err := openOperationCandidateReference(prepared.repoPath, prepared.eligibility.upstreamCommit)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer upstream.Close()
	in := fresh.eligibility.inputs
	execution, err := candidate.ValidateExistingCandidate(reference, upstream, root, in.patch, in.effects, fresh.slug, fresh.candidateID, lock)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	derived, err := operationCandidateArtifact(fresh, execution)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	decision, err := candidate.CompareExistingCandidate(existing, state, derived)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if !decision.Reusable {
		return candidate.CandidateArtifact{}, fmt.Errorf("%s", decision.RefusalCode)
	}
	return existing, nil
}

func matchOperationCandidateIdentity(prepared operationCandidatePrepared, artifact candidate.CandidateArtifact) error {
	actual, err := candidate.IdentityTupleFromArtifact(artifact)
	if err != nil {
		return err
	}
	expected, err := candidate.IdentityTupleFromInput(candidate.CandidateIdentityInput{
		SchemaVersion: candidate.CandidateSchemaVersion, Feature: prepared.slug,
		SourceFeatureState: string(prepared.eligibility.sourceFeatureState),
		ReferenceCommit:    prepared.eligibility.referenceCommit, UpstreamCommit: prepared.eligibility.upstreamCommit,
		Bindings: operationCandidateBindings(prepared.eligibility), DerivationMode: candidate.CandidateDerivationModeSingle,
		Operations: prepared.derivation.Operations, EffectMap: prepared.derivation.EffectMap,
	})
	if err != nil {
		return err
	}
	if artifact.CandidateID != prepared.candidateID || !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("candidate-id-collision")
	}
	return nil
}

// Finalization owns one continuous feature hold. Callers cannot inject an
// execution result, an eligibility assertion, or a successful validation callback.
func finalizeOperationCandidate(s *store.Store, slug string, expected operationCandidatePrepared) (candidate.CandidateArtifact, error) {
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if refusal.Reason != "" {
		return candidate.CandidateArtifact{}, fmt.Errorf("%s", refusal.Reason)
	}
	defer lock.Release()
	return finalizeOperationCandidateHeld(s, slug, expected, lock)
}

func finalizeOperationCandidateHeld(s *store.Store, slug string, expected operationCandidatePrepared, lock *candidatelock.Handle) (candidate.CandidateArtifact, error) {
	release, err := borrowOperationCandidateFeatureMutation(lock, s.Root, slug)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer release()
	if err := checkOperationCandidatePreparationRoot(s, slug, expected); err != nil {
		return candidate.CandidateArtifact{}, err
	}
	prepared, err := prepareOperationCandidateHeld(s, slug, expected.invocation, lock)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if !sameOperationCandidatePreparation(expected, prepared) {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate-integrity-drift: preparation changed")
	}
	reference, err := openOperationCandidateReference(s.Root, prepared.eligibility.referenceCommit)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer reference.Close()
	upstream, err := openOperationCandidateReference(s.Root, prepared.eligibility.upstreamCommit)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer upstream.Close()
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	if status.Reconcile.OperationCandidate != nil && status.Reconcile.OperationCandidate.CandidateID != prepared.candidateID {
		return candidate.CandidateArtifact{}, fmt.Errorf("candidate-pending")
	}
	var prior *candidate.CandidateArtifact
	var artifact candidate.CandidateArtifact
	state, metadataErr := upstream.ReadCandidateState(slug, prepared.candidateID)
	if metadataErr == nil {
		switch state.Status {
		case candidate.CandidateStatusAccepted:
			return candidate.CandidateArtifact{}, fmt.Errorf("candidate-already-accepted")
		case candidate.CandidateStatusRejected:
			return candidate.CandidateArtifact{}, fmt.Errorf("candidate-rejected")
		}
		artifact, _, err = upstream.ReadCandidateMetadata(slug, prepared.candidateID)
		if err != nil {
			return candidate.CandidateArtifact{}, err
		}
		if err := matchOperationCandidateIdentity(prepared, artifact); err != nil {
			return candidate.CandidateArtifact{}, err
		}
		prior = &artifact
		present, err := upstream.CandidateWorktreePresent(slug, prepared.candidateID)
		if err != nil {
			return candidate.CandidateArtifact{}, err
		}
		if present {
			root, err := candidate.OpenExistingCandidateRoot(upstream, slug, prepared.candidateID)
			if err != nil {
				return candidate.CandidateArtifact{}, err
			}
			defer root.Close()
			return operationCandidateReuseCheck(prepared, root, lock)
		}
	} else if !errors.Is(metadataErr, fs.ErrNotExist) {
		return candidate.CandidateArtifact{}, metadataErr
	}
	in := prepared.eligibility.inputs
	execution, err := candidate.Execute(reference, upstream, in.patch, in.effects, slug, prepared.candidateID, lock)
	if err != nil {
		return candidate.CandidateArtifact{}, err
	}
	defer execution.Root.Close()
	fail := func(cause error) (candidate.CandidateArtifact, error) {
		stale := candidate.CandidateState{SchemaVersion: candidate.CandidateSchemaVersion, CandidateID: prepared.candidateID,
			Status: candidate.CandidateStatusStale, AcceptValidation: candidate.CandidateAcceptValidationNone}
		markErr := execution.Root.WriteState(stale, lock)
		pruneErr := pruneOperationCandidateWorktree(s.Root, execution.Root, lock)
		return candidate.CandidateArtifact{}, errors.Join(cause, markErr, pruneErr)
	}
	// Recheck source bindings after execution too: non-cooperating artifact
	// editors must not turn a once-valid preparation into fresh authority.
	latest, err := assessOperationCandidateEligibility(s, slug, prepared.invocation)
	if err != nil {
		return fail(err)
	}
	if !latest.eligible || operationCandidateBindings(latest) != operationCandidateBindings(prepared.eligibility) ||
		latest.sourceFeatureState != prepared.eligibility.sourceFeatureState {
		return fail(fmt.Errorf("candidate-integrity-drift: inputs changed during execution"))
	}
	artifact, err = operationCandidateArtifact(prepared, execution)
	if err != nil {
		return fail(err)
	}
	if prior != nil {
		decision, err := candidate.CompareExistingCandidate(*prior, state, artifact)
		if err != nil {
			return fail(err)
		}
		if !decision.Reusable {
			return fail(fmt.Errorf("%s", decision.RefusalCode))
		}
		artifact = *prior
	} else {
		if err := candidate.WriteCandidateMetadata(execution.Root, artifact, lock); err != nil {
			return fail(err)
		}
		if err := execution.Root.WritePatch([]byte(execution.Diff), lock); err != nil {
			return fail(err)
		}
	}
	pending := candidate.CandidateState{SchemaVersion: candidate.CandidateSchemaVersion, CandidateID: artifact.CandidateID,
		Status: candidate.CandidateStatusPending, AcceptValidation: candidate.CandidateAcceptValidationNone}
	if err := execution.Root.WriteState(pending, lock); err != nil {
		return fail(err)
	}
	path, _ := candidate.CandidatePointerPath(slug, artifact.CandidateID)
	status.Reconcile.OperationCandidate = &store.OperationCandidateSummary{CandidateID: artifact.CandidateID, Path: path,
		Status: candidate.CandidateStatusPending, SourceFeatureState: artifact.SourceFeatureState}
	if err := s.SaveFeatureStatus(status); err != nil {
		return fail(err)
	}
	return artifact, nil
}

func operationCandidateCoexistenceRefusal(status store.FeatureStatus, action string) string {
	hasCandidate := status.Reconcile.OperationCandidate != nil && status.Reconcile.OperationCandidate.Status == candidate.CandidateStatusPending
	switch action {
	case "resolver-action":
		if hasCandidate {
			return "candidate-pending"
		}
	case "candidate-action", "candidate-derivation":
		if status.Reconcile.ShadowPath != "" {
			return "shadow-pending"
		}
	}
	return ""
}

func borrowOperationCandidateFeatureMutation(lock *candidatelock.Handle, repoRoot, slug string) (func(), error) {
	if lock == nil {
		return nil, fmt.Errorf("feature lock handle is required")
	}
	return lock.Borrow(filepath.Join(repoRoot, ".tpatch", "locks", "reconcile", slug+".lock"))
}

func openOperationCandidateReference(repoRoot, commitHint string) (*candidate.ReferenceRoot, error) {
	commit := strings.TrimSpace(commitHint)
	if commit == "" {
		var err error
		commit, err = repoHEADCommit(repoRoot)
		if err != nil {
			return nil, err
		}
	}
	return candidate.OpenReferenceRoot(repoRoot, commit)
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
	recovery, err := reference.FindCandidateRecoveryJournal(slug)
	if err != nil {
		return store.FeatureStatus{}, nil, err
	}
	if recovery != "" {
		return store.FeatureStatus{}, nil, fmt.Errorf("recovery-required")
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return store.FeatureStatus{}, nil, err
	}
	if refusal := operationCandidateCoexistenceRefusal(status, action); refusal != "" {
		return store.FeatureStatus{}, nil, fmt.Errorf("%s", refusal)
	}
	if err := inspectOperationCandidateInventory(reference, slug, status.Reconcile.OperationCandidate); err != nil {
		return store.FeatureStatus{}, nil, err
	}
	pointer := store.CloneOperationCandidateSummary(status.Reconcile.OperationCandidate)
	if requirePointer && pointer == nil {
		return store.FeatureStatus{}, nil, fmt.Errorf("candidate-absent")
	}
	return status, pointer, nil
}

func operationCandidateBindings(eligibility operationCandidateEligibility) candidate.CandidateBindings {
	return candidate.CandidateBindings{CaptureEventSHA256: eligibility.captureEventSHA256, CoverageSHA256: eligibility.coverageSHA256,
		PatchSHA256: eligibility.patchSHA256, RecipeSHA256: eligibility.recipeSHA256}
}

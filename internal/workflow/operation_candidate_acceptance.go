package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func acceptOperationCandidate(s *store.Store, slug string) error {
	// 1. These caller-owned holds span all fourteen steps, including cleanup.
	featureLock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return err
	}
	if refusal.Reason != "" {
		return &OperationCandidateActionError{Code: refusal.Reason}
	}
	defer featureLock.Release()
	globalLock, err := candidatelock.TryAcquire(filepath.Join(s.Root, ".tpatch", "locks", "worktree-mutation.lock"))
	if err != nil {
		return err
	}
	defer globalLock.Release()
	if err := operationCandidateAcceptanceHook(candidateStepLocks, "held", ""); err != nil {
		return err
	}

	// 2. Journal inspection precedes status, immutable inputs and worktree reads.
	disk, err := openOperationCandidateDisk(s.Root)
	if err != nil {
		return err
	}
	defer disk.root.Close()
	journals, err := operationCandidateRecoveryIDs(disk, slug)
	if err != nil || len(journals) != 0 {
		return &OperationCandidateActionError{Code: "recovery-required", Detail: "step 2", Err: err}
	}
	if err := operationCandidateAcceptanceHook(candidateStepRecoveryCheck, "checked", ""); err != nil {
		return err
	}

	// 3. No stale transition is permitted on this terminal action.
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return &OperationCandidateActionError{Code: "candidate-absent", Err: err}
	}
	if code := operationCandidateCoexistenceRefusal(status, "candidate-action"); code != "" {
		return candidateActionErrorForSlug(fmt.Errorf("%s", code), slug)
	}
	pointer := status.Reconcile.OperationCandidate
	if pointer == nil {
		return &OperationCandidateActionError{Code: "candidate-absent"}
	}
	if err := store.ValidateOperationCandidateSummary(pointer); err != nil {
		return acceptanceBindingError(candidateStepPending, err)
	}
	directory, err := candidate.CandidateDirectoryRelative(slug, pointer.CandidateID)
	if err != nil || pointer.Path != directory {
		return acceptanceBindingError(candidateStepPending, fmt.Errorf("candidate pointer namespace mismatch"))
	}
	metadata, err := disk.read(path.Join(directory, candidate.CandidateMetadataName))
	if err != nil || !metadata.Present {
		return acceptanceBindingError(candidateStepPending, errors.Join(fmt.Errorf("candidate metadata unavailable"), err))
	}
	artifact, err := candidate.DecodeCandidateArtifact(metadata.Data)
	if err != nil {
		return acceptanceBindingError(candidateStepPending, err)
	}
	stateImage, err := disk.read(path.Join(directory, candidate.CandidateStateName))
	if err != nil || !stateImage.Present {
		return acceptanceBindingError(candidateStepPending, errors.Join(fmt.Errorf("candidate state unavailable"), err))
	}
	state, err := candidate.DecodeCandidateState(stateImage.Data)
	if err != nil {
		return acceptanceBindingError(candidateStepPending, err)
	}
	if state.Status == candidate.CandidateStatusAccepted {
		return &OperationCandidateActionError{Code: "candidate-already-accepted"}
	}
	if state.Status != candidate.CandidateStatusPending {
		return &OperationCandidateActionError{Code: "candidate-not-pending"}
	}
	if artifact.Feature != slug || artifact.CandidateID != pointer.CandidateID || state.CandidateID != artifact.CandidateID ||
		pointer.SourceFeatureState != artifact.SourceFeatureState || pointer.Status != state.Status {
		return acceptanceBindingError(candidateStepPending, fmt.Errorf("candidate metadata/pointer mismatch"))
	}
	if string(status.State) != artifact.SourceFeatureState || (status.State != store.StateApplied && status.State != store.StateBlocked) {
		return &OperationCandidateActionError{Code: "candidate-feature-state-changed",
			Detail: fmt.Sprintf("step 3: bound %s, current %s", artifact.SourceFeatureState, status.State)}
	}
	if err := operationCandidateAcceptanceHook(candidateStepPending, "checked", ""); err != nil {
		return err
	}

	// 4. E1–E15 and every proof are recomputed, not supplied by candidate.json.
	eligibility, err := assessOperationCandidateEligibility(s, slug, operationCandidateInvocation{
		slugs: []string{slug}, upstreamCommit: artifact.UpstreamCommit,
	})
	if err != nil {
		return acceptanceBindingError(candidateStepRederive, err)
	}
	if !eligibility.eligible || eligibility.inputs == nil {
		return acceptanceBindingError(candidateStepRederive, fmt.Errorf("%s: %s", eligibility.reason, eligibility.detail))
	}
	reference, err := candidate.OpenReferenceRoot(s.Root, eligibility.referenceCommit)
	if err != nil {
		return acceptanceBindingError(candidateStepRederive, err)
	}
	defer reference.Close()
	upstream, err := candidate.OpenReferenceRoot(s.Root, artifact.UpstreamCommit)
	if err != nil {
		return acceptanceBindingError(candidateStepRederive, err)
	}
	defer upstream.Close()
	derivation, err := candidate.DescribeDerivation(reference, upstream, eligibility.inputs.patch, eligibility.inputs.effects, eligibility.inputs.effectHashes)
	if err != nil {
		return acceptanceBindingError(candidateStepRederive, err)
	}
	id, _, _, err := candidate.ComputeCandidateID(candidate.CandidateIdentityInput{
		SchemaVersion: candidate.CandidateSchemaVersion, Feature: slug, SourceFeatureState: string(eligibility.sourceFeatureState),
		UpstreamCommit: eligibility.upstreamCommit, ReferenceCommit: eligibility.referenceCommit,
		Bindings: operationCandidateBindings(eligibility), DerivationMode: candidate.CandidateDerivationModeSingle,
		Operations: derivation.Operations, EffectMap: derivation.EffectMap,
	})
	if err != nil || id != artifact.CandidateID || artifact.ReferenceCommit != eligibility.referenceCommit ||
		artifact.Bindings != operationCandidateBindings(eligibility) || !reflect.DeepEqual(artifact.Derivation, derivation) {
		return acceptanceBindingError(candidateStepRederive, errors.Join(fmt.Errorf("re-derived immutable identity differs"), err))
	}
	rejected, err := isOperationCandidateRejected(reference, slug, id)
	if err != nil {
		return acceptanceBindingError(candidateStepRederive, err)
	}
	if rejected {
		return &OperationCandidateActionError{Code: "candidate-not-pending", Detail: "candidate is in the rejection audit"}
	}
	root, err := candidate.OpenExistingCandidateRoot(upstream, slug, id)
	if err != nil {
		return acceptanceBindingError(candidateStepRederive, err)
	}
	defer func() { _ = root.Close() }()
	execution, err := candidate.ValidateExistingCandidate(reference, upstream, root, eligibility.inputs.patch, eligibility.inputs.effects, slug, id, featureLock)
	if err != nil {
		return acceptanceBindingError(candidateStepRederive, err)
	}
	if err := matchOperationCandidateResult(artifact, execution); err != nil {
		return acceptanceBindingError(candidateStepRederive, err)
	}
	reviewDiff, err := disk.read(path.Join(directory, candidate.CandidatePatchName))
	if err != nil || !reviewDiff.Present || !bytes.Equal(reviewDiff.Data, []byte(execution.Diff)) {
		return acceptanceBindingError(candidateStepRederive, fmt.Errorf("candidate review diff differs from the re-derived result"))
	}
	if err := operationCandidateAcceptanceHook(candidateStepRederive, "checked", ""); err != nil {
		return err
	}

	// 5. Rebuild under the same two holds, then run only the configured tests.
	if err := operationCandidateAcceptanceHook(candidateStepRematerialize, "before", ""); err != nil {
		return acceptanceBindingError(candidateStepRematerialize, err)
	}
	execution, err = candidate.RematerializeForAcceptance(reference, upstream, root, eligibility.inputs.patch, eligibility.inputs.effects, slug, id, featureLock, globalLock)
	if err != nil {
		return acceptanceBindingError(candidateStepRematerialize, err)
	}
	root = execution.Root
	cfg, err := s.LoadConfig()
	if err != nil {
		return err
	}
	test, err := RunTestCommandInShadow(root.WorktreePath(), ValidationConfig{TestCommand: cfg.TestCommand})
	if err != nil || (test.Ran && !test.Passed) {
		return &OperationCandidateActionError{Code: "candidate-test-failed", Detail: fmt.Sprintf("step 5: exit %d: %s", test.ExitCode, test.Stderr), Err: err}
	}
	execution, err = candidate.ValidateExistingCandidate(reference, upstream, root, eligibility.inputs.patch, eligibility.inputs.effects, slug, id, featureLock)
	if err != nil {
		return acceptanceBindingError(candidateStepRematerialize, err)
	}
	if err := matchOperationCandidateResult(artifact, execution); err != nil {
		return acceptanceBindingError(candidateStepRematerialize, err)
	}
	latest, err := assessOperationCandidateEligibility(s, slug, operationCandidateInvocation{
		slugs: []string{slug}, upstreamCommit: artifact.UpstreamCommit,
	})
	if latest.sourceFeatureState != "" && string(latest.sourceFeatureState) != artifact.SourceFeatureState {
		return &OperationCandidateActionError{Code: "candidate-feature-state-changed",
			Detail: fmt.Sprintf("step 5: bound %s, current %s", artifact.SourceFeatureState, latest.sourceFeatureState)}
	}
	if err != nil || !latest.eligible || latest.inputs == nil ||
		operationCandidateBindings(latest) != artifact.Bindings || string(latest.sourceFeatureState) != artifact.SourceFeatureState {
		return acceptanceBindingError(candidateStepRematerialize, errors.Join(fmt.Errorf("bound inputs changed during rematerialization/tests"), err))
	}
	currentMetadata, err := disk.read(path.Join(directory, candidate.CandidateMetadataName))
	if err != nil {
		return acceptanceBindingError(candidateStepRematerialize, err)
	}
	currentArtifact, err := candidate.DecodeCandidateArtifact(currentMetadata.Data)
	if err != nil || !reflect.DeepEqual(currentArtifact, artifact) {
		return acceptanceBindingError(candidateStepRematerialize, fmt.Errorf("immutable candidate metadata changed during validation"))
	}
	currentState, err := disk.read(path.Join(directory, candidate.CandidateStateName))
	if err != nil {
		return acceptanceBindingError(candidateStepRematerialize, err)
	}
	checkedState, err := candidate.DecodeCandidateState(currentState.Data)
	if err != nil || checkedState != state {
		return &OperationCandidateActionError{Code: "candidate-not-pending", Detail: "candidate state changed during validation", Err: err}
	}
	currentDiff, err := disk.read(path.Join(directory, candidate.CandidatePatchName))
	if err != nil || !currentDiff.Present || !bytes.Equal(currentDiff.Data, []byte(execution.Diff)) {
		return acceptanceBindingError(candidateStepRematerialize, fmt.Errorf("immutable review diff changed during validation"))
	}
	changes, err := candidate.AcceptanceChanges(upstream, execution, featureLock, globalLock)
	if err != nil {
		return acceptanceBindingError(candidateStepRematerialize, err)
	}
	transaction, err := newOperationCandidateTransaction(s, artifact, status, changes, featureLock, globalLock)
	if err != nil {
		return err
	}
	defer transaction.close()

	// 6. Ancestry is diagnostic; exact per-path CAS supplies write authority.
	if err := transaction.before(candidateStepPreconditions); err != nil {
		return err
	}
	if err := operationCandidateAncestor(s.Root, artifact.UpstreamCommit); err != nil {
		return err
	}
	if err := transaction.checkCAS(false); err != nil {
		return err
	}
	now := time.Now().UTC()

	// 7–9. No live or canonical feature path is mutated during preparation.
	if err := transaction.before(candidateStepStage); err != nil {
		return err
	}
	if err := transaction.requireFreshPreparation(); err != nil {
		return fmt.Errorf("acceptance step 7, before journal preparation: %w", err)
	}
	publication, err := operationCandidatePublication(s, transaction, upstream, execution, now)
	if err == nil {
		_, err = PublishCoverage(s, publication)
	}
	if err != nil {
		return transaction.preparationFailed(err)
	}
	if err := transaction.before(candidateStepSnapshot); err != nil {
		return transaction.preparationFailed(err)
	}
	if err := transaction.snapshot(state, execution.Root); err != nil {
		return transaction.preparationFailed(err)
	}
	if err := transaction.before(candidateStepJournal); err != nil {
		return transaction.preparationFailed(err)
	}
	if err := transaction.prepareJournal(); err != nil {
		if transaction.prepared {
			return transaction.failed(err)
		}
		return transaction.preparationFailed(err)
	}

	// 10. The recheck and first live write have no intervening mutation hook.
	if err := transaction.before(candidateStepApply); err != nil {
		return transaction.failed(err)
	}
	if err := transaction.applyLive(); err != nil {
		if code, ok := OperationCandidateActionCode(err); ok && code == "candidate-cas-failed" {
			return err // No write occurred. Never erase the intervening editor's work.
		}
		return transaction.failed(err)
	}

	// 11. The shared publisher consumes the already-staged complete variant.
	if err := transaction.before(candidateStepPublish); err != nil {
		return transaction.failed(err)
	}
	if _, err := PublishCoverage(s, publication); err != nil {
		return transaction.failed(err)
	}

	// 12. Status is written without SaveFeatureStatus's unjournaled FEATURES.md.
	if err := transaction.before(candidateStepAccepted); err != nil {
		return transaction.failed(err)
	}
	state.Status, state.AcceptValidation = candidate.CandidateStatusAccepted, candidate.CandidateAcceptValidationStruct
	if test.Ran {
		state.AcceptValidation = candidate.CandidateAcceptValidationWithRun
	}
	if err := transaction.acceptState(state, now); err != nil {
		return transaction.failed(err)
	}
	if err := transaction.validatePublishedBase(); err != nil {
		return transaction.failed(err)
	}

	// 13. Once installed, even a failed durability acknowledgement must not
	// select rollback; explicit recovery reads the actual marker.
	if err := transaction.before(candidateStepCommit); err != nil {
		return transaction.failed(err)
	}
	if err := transaction.markCommitted(); err != nil {
		if transaction.committed {
			return fmt.Errorf("acceptance step 13: commit marker installed, durability failed; explicit recovery required: %w", err)
		}
		return transaction.failed(err)
	}

	// 14. Any cleanup failure keeps the journal and the accepted live result.
	if err := transaction.before(candidateStepCleanup); err != nil {
		return fmt.Errorf("acceptance step 14: recovery required: %w", err)
	}
	if err := execution.Root.Close(); err != nil {
		return fmt.Errorf("acceptance step 14: candidate handle close failed; journal retained: %w", err)
	}
	return transaction.cleanupCommitted(upstream)
}

func acceptanceBindingError(step operationCandidateAcceptStep, err error) error {
	return &OperationCandidateActionError{Code: "candidate-binding-changed", Detail: fmt.Sprintf("step %d", step), Err: err}
}

func matchOperationCandidateResult(artifact candidate.CandidateArtifact, execution candidate.ExecutionResult) error {
	tree, err := candidate.CandidateTreeIntegritySHA256(execution.TreeHash)
	if err != nil {
		return err
	}
	if artifact.Result.CandidateDiffSHA256 != store.SHA256HexString(execution.Diff) || artifact.Result.CandidateTreeSHA256 != tree {
		return fmt.Errorf("candidate diff/tree fingerprints differ from re-derived result")
	}
	return nil
}

func operationCandidateAncestor(root, upstream string) error {
	head, err := repoHEADCommit(root)
	if err != nil {
		return err
	}
	command := exec.Command("git", "merge-base", "--is-ancestor", upstream, head)
	command.Dir = root
	command.Env = gitutil.HardenedNoLazyGitEnv("GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return &OperationCandidateActionError{Code: "head-diverged", Detail: fmt.Sprintf("step 6: upstream %s is not an ancestor of HEAD %s", upstream, head)}
		}
		return fmt.Errorf("acceptance ancestry check: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (t *operationCandidateTransaction) acceptState(state candidate.CandidateState, now time.Time) error {
	body, err := candidate.EncodeCandidateState(state)
	if err != nil {
		return err
	}
	if _, err := t.replace(path.Join(t.directory, candidate.CandidateStateName), body, fs.FileMode(t.journal.PendingStateMode)); err != nil {
		return err
	}
	if err := operationCandidateAcceptanceHook(t.step, "accepted-state", "state.json"); err != nil {
		return err
	}
	status := t.status
	status.State = store.StateApplied
	status.Apply.BaseCommit = t.artifact.UpstreamCommit
	status.Apply.CompletedAt = now.Format(time.RFC3339)
	status.Apply.HasPatch, status.Apply.HasRecipe = true, true
	status.LastCommand, status.UpdatedAt = "reconcile --accept-candidate", now.Format(time.RFC3339)
	status.Reconcile.OperationCandidate = store.CloneOperationCandidateSummary(status.Reconcile.OperationCandidate)
	status.Reconcile.OperationCandidate.Status = candidate.CandidateStatusAccepted
	body, err = json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	mode := t.journal.Snapshots[len(t.journal.Snapshots)-1].Mode
	if _, err := t.replace(path.Join(t.featureDir, "status.json"), append(body, '\n'), fs.FileMode(mode)); err != nil {
		return err
	}
	return operationCandidateAcceptanceHook(t.step, "accepted-status", "status.json")
}

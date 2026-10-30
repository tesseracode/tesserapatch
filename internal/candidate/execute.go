package candidate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/safety"
)

type ExecutionResult struct {
	Root      *CandidateRoot
	Plan      Plan
	EffectMap []EffectResolution
	TreeHash  string
	Diff      string
}

type EffectResolution struct {
	EffectOrdinal     int
	Path              string
	State             string
	OperationOrdinals []int
}

// Execute generates, applies and validates one candidate worktree end-to-end.
func Execute(
	reference, upstream *ReferenceRoot,
	patch string,
	effects []gitutil.PatchEffect,
	slug, candidateID string,
	featureLock *candidatelock.Handle,
) (ExecutionResult, error) {
	plan, err := derive(reference, upstream, patch, effects)
	if err != nil {
		return ExecutionResult{}, err
	}
	if _, _, conflicts := plan.Counts(); conflicts != 0 {
		return ExecutionResult{}, &refusal{code: "candidate-postcondition-failed", detail: "derivation includes conflicts"}
	}
	return executeWithPlan(reference, upstream, patch, effects, plan, slug, candidateID, featureLock)
}

func executeWithPlan(
	reference, upstream *ReferenceRoot,
	patch string,
	effects []gitutil.PatchEffect,
	plan Plan,
	slug, candidateID string,
	featureLock *candidatelock.Handle,
) (ExecutionResult, error) {
	repoRoot, err := resolveExecutionRepoRoot(reference, upstream)
	if err != nil {
		return ExecutionResult{}, err
	}
	if !slugPattern.MatchString(slug) || !candidateIDPattern.MatchString(candidateID) {
		return ExecutionResult{}, fmt.Errorf("invalid candidate slug or id")
	}
	releaseFeature, err := borrowExecutionFeatureLock(upstream, slug, featureLock)
	if err != nil {
		return ExecutionResult{}, err
	}
	defer releaseFeature()

	createGlobal, err := acquireExecutionGlobalLock(upstream.repo)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("acquire worktree mutation lock for candidate creation: %w", err)
	}
	candidateRoot, createErr := CreateCandidateRoot(repoRoot, slug, candidateID, upstream.Commit(), featureLock, createGlobal)
	releaseCreateGlobalErr := createGlobal.Release()
	if createErr != nil {
		if releaseCreateGlobalErr != nil {
			return ExecutionResult{}, fmt.Errorf("%w (release worktree mutation lock: %v)", createErr, releaseCreateGlobalErr)
		}
		return ExecutionResult{}, createErr
	}
	if releaseCreateGlobalErr != nil {
		return abandonCandidate(candidateRoot, featureLock, fmt.Errorf("release worktree mutation lock after candidate creation: %w", releaseCreateGlobalErr))
	}

	abandon := func(cause error) (ExecutionResult, error) {
		return abandonCandidate(candidateRoot, featureLock, cause)
	}
	if err := applyCandidateOperations(candidateRoot, plan.operations); err != nil {
		return abandon(err)
	}
	validation, err := validateCandidate(reference, upstream, candidateRoot, patch, effects, plan)
	if err != nil {
		return abandon(err)
	}
	return ExecutionResult{
		Root:      candidateRoot,
		Plan:      plan,
		EffectMap: validation.effectMap,
		TreeHash:  validation.treeHash,
		Diff:      validation.diff,
	}, nil
}

// ValidateExistingCandidate runs the production derivation and full S4
// validator against the exact repository/feature/ID named by the caller.
func ValidateExistingCandidate(reference, upstream *ReferenceRoot, root *CandidateRoot, patch string, effects []gitutil.PatchEffect, slug, candidateID string, featureLock *candidatelock.Handle) (ExecutionResult, error) {
	if _, err := resolveExecutionRepoRoot(reference, upstream); err != nil {
		return ExecutionResult{}, err
	}
	if err := root.ensureAvailable(); err != nil {
		return ExecutionResult{}, err
	}
	if root.slug != slug || root.candidateID != candidateID || root.commit != upstream.commit ||
		root.repo.path != upstream.repo.path || !os.SameFile(root.repo.identity, upstream.repo.identity) {
		return ExecutionResult{}, fmt.Errorf("candidate root repository/feature/identity mismatch")
	}
	release, err := root.borrowFeatureLock(featureLock)
	if err != nil {
		return ExecutionResult{}, err
	}
	defer release()
	plan, err := derive(reference, upstream, patch, effects)
	if err != nil {
		return ExecutionResult{}, err
	}
	result, err := validateCandidate(reference, upstream, root, patch, effects, plan)
	if err != nil {
		return ExecutionResult{}, err
	}
	return ExecutionResult{Root: root, Plan: plan, EffectMap: result.effectMap, TreeHash: result.treeHash, Diff: result.diff}, nil
}

func abandonCandidate(candidateRoot *CandidateRoot, featureLock *candidatelock.Handle, cause error) (ExecutionResult, error) {
	if candidateRoot == nil {
		if cause == nil {
			return ExecutionResult{}, fmt.Errorf("candidate root is nil")
		}
		return ExecutionResult{}, cause
	}
	pruneGlobal, lockErr := acquireExecutionGlobalLock(candidateRoot.repo)
	if lockErr != nil {
		closeErr := closeCandidateRoot(candidateRoot)
		lockWrap := fmt.Errorf("acquire worktree mutation lock for candidate prune: %w", lockErr)
		if closeErr != nil {
			lockWrap = fmt.Errorf("%w (close candidate root: %v)", lockWrap, closeErr)
		}
		if cause == nil {
			return ExecutionResult{}, lockWrap
		}
		return ExecutionResult{}, fmt.Errorf("%w (candidate abandon failed: %v)", cause, lockWrap)
	}
	pruneErr := candidateRoot.Prune(featureLock, pruneGlobal)
	releaseErr := pruneGlobal.Release()
	if pruneErr == nil && releaseErr == nil {
		return ExecutionResult{}, cause
	}
	cleanupErr := pruneErr
	if cleanupErr == nil {
		cleanupErr = fmt.Errorf("release worktree mutation lock after candidate prune: %w", releaseErr)
	} else if releaseErr != nil && !errors.Is(releaseErr, cleanupErr) {
		cleanupErr = fmt.Errorf("%v (release worktree mutation lock after candidate prune: %v)", cleanupErr, releaseErr)
	}
	if cause == nil {
		return ExecutionResult{}, cleanupErr
	}
	return ExecutionResult{}, fmt.Errorf("%w (candidate abandon failed: %v)", cause, cleanupErr)
}

func resolveExecutionRepoRoot(reference, upstream *ReferenceRoot) (string, error) {
	if reference == nil || upstream == nil {
		return "", fmt.Errorf("derivation roots are required")
	}
	if err := reference.ensureAvailable(); err != nil {
		return "", err
	}
	if err := upstream.ensureAvailable(); err != nil {
		return "", err
	}
	if reference.repo == nil || upstream.repo == nil {
		return "", fmt.Errorf("derivation roots are required")
	}
	if reference.repo.path != upstream.repo.path || !os.SameFile(reference.repo.identity, upstream.repo.identity) {
		return "", fmt.Errorf("derivation roots must reference the same repository root")
	}
	return reference.repo.path, nil
}

func borrowExecutionFeatureLock(upstream *ReferenceRoot, slug string, featureLock *candidatelock.Handle) (func(), error) {
	if err := upstream.ensureAvailable(); err != nil {
		return nil, err
	}
	repoRoot := upstream.repo.path
	featurePath := filepath.Join(repoRoot, ".tpatch", "locks", "reconcile", slug+".lock")
	if err := safety.EnsureSafeRepoPath(repoRoot, featurePath); err != nil {
		return nil, fmt.Errorf("feature lock path escaped repository root: %w", err)
	}
	release, err := featureLock.Borrow(featurePath)
	if err != nil {
		return nil, err
	}
	return release, nil
}

func acquireExecutionGlobalLock(repo *heldRoot) (*candidatelock.Handle, error) {
	if _, err := repo.validateCurrentPath(false); err != nil {
		return nil, err
	}
	repoRoot := repo.path
	globalPath := filepath.Join(repoRoot, ".tpatch", "locks", "worktree-mutation.lock")
	if err := safety.EnsureSafeRepoPath(repoRoot, globalPath); err != nil {
		return nil, fmt.Errorf("worktree mutation lock path escaped repository root: %w", err)
	}
	return candidatelock.TryAcquire(globalPath)
}

func closeCandidateRoot(root *CandidateRoot) error {
	if root == nil || root.closed {
		return nil
	}
	return root.Close()
}

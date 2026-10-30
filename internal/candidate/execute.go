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
	releaseFeature, err := borrowExecutionFeatureLock(repoRoot, slug, featureLock)
	if err != nil {
		return ExecutionResult{}, err
	}
	defer releaseFeature()

	createGlobal, err := acquireExecutionGlobalLock(repoRoot)
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
		return abandonCandidate(candidateRoot, featureLock, repoRoot, fmt.Errorf("release worktree mutation lock after candidate creation: %w", releaseCreateGlobalErr))
	}

	abandon := func(cause error) (ExecutionResult, error) {
		return abandonCandidate(candidateRoot, featureLock, repoRoot, cause)
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

func abandonCandidate(candidateRoot *CandidateRoot, featureLock *candidatelock.Handle, repoRoot string, cause error) (ExecutionResult, error) {
	if candidateRoot == nil {
		if cause == nil {
			return ExecutionResult{}, fmt.Errorf("candidate root is nil")
		}
		return ExecutionResult{}, cause
	}
	pruneGlobal, lockErr := acquireExecutionGlobalLock(repoRoot)
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

func borrowExecutionFeatureLock(repoRoot, slug string, featureLock *candidatelock.Handle) (func(), error) {
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

func acquireExecutionGlobalLock(repoRoot string) (*candidatelock.Handle, error) {
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

package candidate

import (
	"fmt"

	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
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
	repoRoot, slug, candidateID string,
	featureLock, worktreeMutationLock *candidatelock.Handle,
) (ExecutionResult, error) {
	plan, err := derive(reference, upstream, patch, effects)
	if err != nil {
		return ExecutionResult{}, err
	}
	if _, _, conflicts := plan.Counts(); conflicts != 0 {
		return ExecutionResult{}, &refusal{code: "candidate-postcondition-failed", detail: "derivation includes conflicts"}
	}
	return executeWithPlan(reference, upstream, patch, effects, plan, repoRoot, slug, candidateID, featureLock, worktreeMutationLock)
}

func executeWithPlan(
	reference, upstream *ReferenceRoot,
	patch string,
	effects []gitutil.PatchEffect,
	plan Plan,
	repoRoot, slug, candidateID string,
	featureLock, worktreeMutationLock *candidatelock.Handle,
) (ExecutionResult, error) {
	if reference == nil || upstream == nil {
		return ExecutionResult{}, fmt.Errorf("derivation roots are required")
	}
	candidateRoot, err := CreateCandidateRoot(repoRoot, slug, candidateID, upstream.Commit(), featureLock, worktreeMutationLock)
	if err != nil {
		return ExecutionResult{}, err
	}
	abandon := func(cause error) (ExecutionResult, error) {
		pruneErr := candidateRoot.Prune(featureLock, worktreeMutationLock)
		if pruneErr == nil {
			return ExecutionResult{}, cause
		}
		if cause == nil {
			return ExecutionResult{}, pruneErr
		}
		return ExecutionResult{}, fmt.Errorf("%w (candidate abandon failed: %v)", cause, pruneErr)
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

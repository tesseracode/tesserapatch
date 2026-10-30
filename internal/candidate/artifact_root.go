package candidate

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

const (
	CandidateMetadataName = "candidate.json"
	CandidateStateName    = "state.json"
	CandidatePatchName    = "candidate.patch"
	CandidateWorktreeName = "worktree"
	CandidateJournalName  = "accept-journal.json"
	CandidateRejectedName = "rejected.jsonl"
)

func CandidateSlugRelative(slug string) (string, error) {
	if !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("candidate slug must be kebab-case: %q", slug)
	}
	return path.Join(".tpatch", "reconcile-candidates", slug), nil
}

func CandidateDirectoryRelative(slug, candidateID string) (string, error) {
	root, err := CandidateSlugRelative(slug)
	if err != nil {
		return "", err
	}
	if !candidateIDPattern.MatchString(candidateID) {
		return "", fmt.Errorf("candidate id must match rc_<12 lowercase hex>: %q", candidateID)
	}
	return path.Join(root, candidateID), nil
}

func CandidatePointerPath(slug, candidateID string) (string, error) {
	return CandidateDirectoryRelative(slug, candidateID)
}

func CandidateWorktreeRelative(slug, candidateID string) (string, error) {
	root, err := CandidateDirectoryRelative(slug, candidateID)
	if err != nil {
		return "", err
	}
	return path.Join(root, CandidateWorktreeName), nil
}

func CandidateRejectedLogRelative(slug string) (string, error) {
	root, err := CandidateSlugRelative(slug)
	if err != nil {
		return "", err
	}
	return path.Join(root, CandidateRejectedName), nil
}

func CandidateRecoveryJournalRelative(slug, candidateID string) (string, error) {
	root, err := CandidateDirectoryRelative(slug, candidateID)
	if err != nil {
		return "", err
	}
	return path.Join(root, CandidateJournalName), nil
}

// OpenExistingCandidateRoot reopens an already-created candidate namespace
// through a typed reference capability; no bare repo root forwarding is used.
func OpenExistingCandidateRoot(reference *ReferenceRoot, slug, candidateID string) (_ *CandidateRoot, returnedErr error) {
	if reference == nil {
		return nil, fmt.Errorf("reference root is nil")
	}
	if err := reference.ensureAvailable(); err != nil {
		return nil, err
	}
	if !slugPattern.MatchString(slug) {
		return nil, fmt.Errorf("candidate slug must be kebab-case: %q", slug)
	}
	if !candidateIDPattern.MatchString(candidateID) {
		return nil, fmt.Errorf("candidate id must match rc_<12 lowercase hex>: %q", candidateID)
	}
	parentRel, err := CandidateDirectoryRelative(slug, candidateID)
	if err != nil {
		return nil, err
	}
	worktreeRel := path.Join(parentRel, CandidateWorktreeName)
	worktreePath := filepath.Join(reference.repo.path, filepath.FromSlash(worktreeRel))
	if err := safety.EnsureSafeRepoPath(reference.repo.path, worktreePath); err != nil {
		return nil, fmt.Errorf("candidate worktree path escaped repository root: %w", err)
	}
	parent, err := reference.repo.openNestedRoot(parentRel)
	if err != nil {
		return nil, err
	}
	defer func() {
		if returnedErr != nil {
			_ = parent.close()
		}
	}()
	worktree, err := parent.openNestedRoot(CandidateWorktreeName)
	if err != nil {
		return nil, err
	}
	candidateRoot := &CandidateRoot{
		repo:         reference.repo,
		ownsRepo:     false,
		parent:       parent,
		worktree:     worktree,
		repoRoot:     reference.repo.path,
		slug:         slug,
		candidateID:  candidateID,
		commit:       reference.commit,
		parentRel:    parentRel,
		worktreeRel:  worktreeRel,
		worktreePath: worktreePath,
	}
	if err := candidateRoot.ensureLockPathSafety(); err != nil {
		_ = candidateRoot.Close()
		return nil, err
	}
	return candidateRoot, nil
}

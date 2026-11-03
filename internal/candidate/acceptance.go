package candidate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// RematerializeForAcceptance borrows both transaction-wide holds. Unlike
// Execute, it must never acquire or release a worktree-mutation lock itself.
func RematerializeForAcceptance(reference, upstream *ReferenceRoot, previous *CandidateRoot, patch string, effects []gitutil.PatchEffect, slug, candidateID string, featureLock, globalLock *candidatelock.Handle) (ExecutionResult, error) {
	repoRoot, err := resolveExecutionRepoRoot(reference, upstream)
	if err != nil {
		return ExecutionResult{}, err
	}
	if previous == nil || previous.slug != slug || previous.candidateID != candidateID ||
		previous.commit != upstream.commit || previous.repoRoot != repoRoot {
		return ExecutionResult{}, fmt.Errorf("acceptance candidate root identity mismatch")
	}
	borrowed, err := previous.borrowMutationLocks(featureLock, globalLock)
	if err != nil {
		return ExecutionResult{}, err
	}
	defer borrowed.release()
	state, err := previous.ReadState()
	if err != nil {
		return ExecutionResult{}, err
	}
	if state.Status != CandidateStatusPending {
		return ExecutionResult{}, fmt.Errorf("candidate-not-pending")
	}
	plan, err := derive(reference, upstream, patch, effects)
	if err != nil {
		return ExecutionResult{}, err
	}
	if _, applicable, conflicts := plan.Counts(); applicable == 0 || conflicts != 0 {
		return ExecutionResult{}, fmt.Errorf("candidate-postcondition-failed: no complete applicable plan")
	}
	if err := previous.Prune(featureLock, globalLock); err != nil {
		return ExecutionResult{}, err
	}
	root, err := CreateCandidateRoot(repoRoot, slug, candidateID, upstream.Commit(), featureLock, globalLock)
	if err != nil {
		return ExecutionResult{}, err
	}
	fail := func(cause error) (ExecutionResult, error) {
		return ExecutionResult{}, errors.Join(cause, root.Prune(featureLock, globalLock))
	}
	if err := applyCandidateOperations(root, plan.operations); err != nil {
		return fail(err)
	}
	result, err := validateCandidate(reference, upstream, root, patch, effects, plan)
	if err != nil {
		return fail(err)
	}
	return ExecutionResult{Root: root, Plan: plan, EffectMap: result.effectMap, TreeHash: result.treeHash, Diff: result.diff}, nil
}

// AcceptanceChange contains independently re-derived raw images, not a stored
// recipe body or a copy of a potentially edited worktree file.
type AcceptanceChange struct {
	Path    string
	Present bool
	Before  []byte
	After   []byte
}

// AcceptanceChanges replays the ephemeral operations against their immutable
// upstream. The caller still owes live CAS and the journaled write transaction.
func AcceptanceChanges(upstream *ReferenceRoot, execution ExecutionResult, featureLock, globalLock *candidatelock.Handle) ([]AcceptanceChange, error) {
	root := execution.Root
	if upstream == nil || root == nil {
		return nil, fmt.Errorf("acceptance roots are required")
	}
	if _, err := resolveExecutionRepoRoot(upstream, upstream); err != nil {
		return nil, err
	}
	if root.repoRoot != upstream.repo.path || root.commit != upstream.commit {
		return nil, fmt.Errorf("acceptance root belongs to another upstream")
	}
	borrowed, err := root.borrowMutationLocks(featureLock, globalLock)
	if err != nil {
		return nil, err
	}
	defer borrowed.release()
	paths := touchedPaths(execution.Plan)
	snapshot, err := upstream.snapshotPaths(paths)
	if err != nil {
		return nil, err
	}
	changes := make([]AcceptanceChange, 0, len(paths))
	for _, name := range paths {
		entry, known := snapshot.entry(name)
		if !known || (entry.Present && entry.Mode != gitutil.ModeRegular) {
			return nil, fmt.Errorf("candidate-mode-drift: %s", name)
		}
		change := AcceptanceChange{Path: filepath.ToSlash(name), Present: entry.Present}
		if entry.Present {
			change.Before, err = upstream.ReadFile(name)
			if err != nil {
				return nil, err
			}
		}
		current := bytes.Clone(change.Before)
		for _, op := range execution.Plan.operations {
			if op.path != name {
				continue
			}
			switch op.kind {
			case "create-file":
				if entry.Present {
					return nil, fmt.Errorf("candidate-postcondition-failed: occupied creation %s", name)
				}
				current = bytes.Clone(op.payload)
			case "insert-by-boundary":
				if !entry.Present || op.byteOffset < 0 || op.byteOffset > len(current) {
					return nil, fmt.Errorf("candidate-postcondition-failed: insertion %s", name)
				}
				next := make([]byte, 0, len(current)+len(op.payload))
				next = append(next, current[:op.byteOffset]...)
				next = append(next, op.payload...)
				current = append(next, current[op.byteOffset:]...)
			default:
				return nil, fmt.Errorf("candidate-operation-kind-unsupported: %s", op.kind)
			}
		}
		actual, err := root.ReadFile(name)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(actual, current) {
			return nil, fmt.Errorf("candidate-postcondition-failed: %s", name)
		}
		change.After = current
		changes = append(changes, change)
	}
	if len(changes) == 0 {
		return nil, fmt.Errorf("candidate-postcondition-failed: empty acceptance")
	}
	return changes, nil
}

// AcceptanceDirectoryIdentity survives process restart and child deletion.
// Neither path spelling nor a fresh os.SameFile observation proves ownership.
type AcceptanceDirectoryIdentity struct {
	Platform string `json:"platform"`
	Volume   string `json:"volume"`
	File     string `json:"file"`
}

func (identity AcceptanceDirectoryIdentity) validate() error {
	if identity.Platform != runtime.GOOS {
		return fmt.Errorf("acceptance directory identity belongs to another platform")
	}
	for _, value := range []string{identity.Volume, identity.File} {
		number, err := strconv.ParseUint(value, 16, 64)
		if err != nil || fmt.Sprintf("%016x", number) != value {
			return fmt.Errorf("acceptance directory identity is not canonical")
		}
	}
	if identity.File == "0000000000000000" {
		return fmt.Errorf("acceptance directory has no stable native identity")
	}
	return nil
}

// CheckRoot compares a held directory with its original, journaled identity.
func (identity AcceptanceDirectoryIdentity) CheckRoot(root *os.Root) error {
	if err := identity.validate(); err != nil {
		return err
	}
	current, err := readAcceptanceDirectoryIdentity(root)
	if err != nil {
		return err
	}
	if current != identity {
		return fmt.Errorf("acceptance directory identity differs from its original reservation")
	}
	return nil
}

type AcceptanceOwnership struct {
	Worktree         AcceptanceDirectoryIdentity `json:"worktree"`
	RegistrationName string                      `json:"registration_name"`
	Registration     AcceptanceDirectoryIdentity `json:"registration"`
	Head             string                      `json:"head"`
	GitDir           string                      `json:"gitdir"`
	CommonDir        string                      `json:"commondir"`
}

// Validate derives the backlink from immutable candidate identity; the
// registration name alone is never permission to delete a registry entry.
func (ownership AcceptanceOwnership) Validate(repoRoot, slug, candidateID, commit string) error {
	if err := ownership.Worktree.validate(); err != nil {
		return err
	}
	if err := ownership.Registration.validate(); err != nil {
		return err
	}
	name, err := normalizeRelativePath(ownership.RegistrationName)
	if err != nil || name != ownership.RegistrationName || strings.Contains(name, "/") ||
		ownership.Worktree == ownership.Registration || ownership.Head != commit+"\n" ||
		filepath.ToSlash(strings.TrimSpace(ownership.CommonDir)) != "../.." {
		return fmt.Errorf("acceptance registration ownership is malformed")
	}
	directory, err := CandidateDirectoryRelative(slug, candidateID)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return err
	}
	expected := filepath.Join(resolved, filepath.FromSlash(directory), CandidateWorktreeName, ".git")
	if filepath.Clean(strings.TrimSpace(ownership.GitDir)) != expected {
		return fmt.Errorf("acceptance registration backlink belongs to another worktree")
	}
	return nil
}

// CaptureAcceptanceOwnership binds the rematerialized root while it is still
// pending. The initial acceptance journal preserves it before any live write.
func (r *CandidateRoot) CaptureAcceptanceOwnership(featureLock, globalLock *candidatelock.Handle) (AcceptanceOwnership, error) {
	var ownership AcceptanceOwnership
	borrowed, err := r.borrowMutationLocks(featureLock, globalLock)
	if err != nil {
		return ownership, err
	}
	defer borrowed.release()
	if err := r.validateReopenedCandidateAuthority(r.commit); err != nil {
		return ownership, err
	}
	state, err := r.ReadState()
	if err != nil || state.Status != CandidateStatusPending {
		return ownership, errors.Join(fmt.Errorf("cleanup ownership requires the pending rematerialized root"), err)
	}
	gitfile, err := readRootedRegularFile(r.worktree.root, ".git")
	if err != nil {
		return ownership, err
	}
	gitdir, err := parseWorktreeGitfile(r.worktree.path, string(gitfile))
	if err != nil {
		return ownership, err
	}
	gitdir, err = filepath.EvalSymlinks(gitdir)
	if err != nil {
		return ownership, err
	}
	resolved, err := filepath.EvalSymlinks(r.repoRoot)
	if err != nil {
		return ownership, err
	}
	relative, err := filepath.Rel(resolved, gitdir)
	if err != nil {
		return ownership, err
	}
	relative, err = normalizeRelativePath(relative)
	if err != nil || !strings.HasPrefix(relative, ".git/worktrees/") {
		return ownership, fmt.Errorf("cleanup registration is outside the worktree registry")
	}
	ownership.RegistrationName = strings.TrimPrefix(relative, ".git/worktrees/")
	if strings.Contains(ownership.RegistrationName, "/") {
		return ownership, fmt.Errorf("cleanup registration is not one registry entry")
	}
	registration, err := r.repo.openNestedRoot(relative)
	if err != nil {
		return ownership, err
	}
	defer registration.close()
	for _, field := range []struct {
		name  string
		value *string
	}{{"HEAD", &ownership.Head}, {"gitdir", &ownership.GitDir}, {"commondir", &ownership.CommonDir}} {
		raw, err := readRootedRegularFile(registration.root, field.name)
		if err != nil {
			return ownership, err
		}
		*field.value = string(raw)
	}
	ownership.Worktree, err = readAcceptanceDirectoryIdentity(r.worktree.root)
	if err != nil {
		return ownership, err
	}
	ownership.Registration, err = readAcceptanceDirectoryIdentity(registration.root)
	if err != nil {
		return ownership, err
	}
	if _, err := r.worktree.validateCurrentPath(false); err != nil {
		return ownership, err
	}
	if _, err := registration.validateCurrentPath(false); err != nil {
		return ownership, err
	}
	return ownership, ownership.Validate(r.repoRoot, r.slug, r.candidateID, r.commit)
}

// PruneAcceptanceWorktree removes only the originally captured registration.
// Partial deletion may remove its control files, but cannot change its native
// identity. The callback fsyncs the still-held registry directory on each OS.
func (r *ReferenceRoot) PruneAcceptanceWorktree(slug, candidateID string, featureLock, globalLock *candidatelock.Handle, ownership AcceptanceOwnership, syncDirectory func(*os.Root) error) error {
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	releaseFeature, err := r.borrowFeatureLock(slug, featureLock)
	if err != nil {
		return err
	}
	defer releaseFeature()
	if globalLock == nil || globalLock == featureLock {
		return fmt.Errorf("acceptance prune requires a distinct global capability")
	}
	globalPath := filepath.Join(r.repo.path, ".tpatch", "locks", "worktree-mutation.lock")
	if err := ensureLockPathBoundToRepo(r.repo, r.repo.path, globalPath); err != nil {
		return err
	}
	releaseGlobal, err := globalLock.Borrow(globalPath)
	if err != nil {
		return err
	}
	defer releaseGlobal()
	directory, err := CandidateDirectoryRelative(slug, candidateID)
	if err != nil {
		return err
	}
	if _, err := readRootedRegularFile(r.repo.root, path.Join(directory, CandidateJournalName)); err != nil {
		return fmt.Errorf("acceptance prune requires its journal: %w", err)
	}
	artifact, state, err := r.ReadCandidateMetadata(slug, candidateID)
	if err != nil || artifact.UpstreamCommit != r.commit || state.Status != CandidateStatusAccepted {
		return errors.Join(fmt.Errorf("acceptance prune metadata mismatch"), err)
	}
	if syncDirectory == nil {
		return fmt.Errorf("acceptance prune requires directory durability")
	}
	if err := ownership.Validate(r.repo.path, slug, candidateID, r.commit); err != nil {
		return err
	}
	relative := path.Join(directory, CandidateWorktreeName)
	if err := validateRootedDirectoryPath(r.repo.root, directory, false); err != nil {
		return err
	}
	if _, err := r.repo.root.Lstat(relative); err == nil {
		return fmt.Errorf("candidate worktree must be relocated before registration pruning")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	registry, err := r.repo.openNestedRoot(".git/worktrees")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err == nil {
		defer registry.close()
		if _, err := registry.root.Lstat(ownership.RegistrationName); err == nil {
			if err := validateRootedDirectoryPath(registry.root, ownership.RegistrationName, false); err != nil {
				return err
			}
			registration, err := registry.openNestedRoot(ownership.RegistrationName)
			if err != nil {
				return err
			}
			defer registration.close()
			if err := ownership.Registration.CheckRoot(registration.root); err != nil {
				return err
			}
			for _, file := range []struct{ name, expected string }{
				{"HEAD", ownership.Head}, {"gitdir", ownership.GitDir}, {"commondir", ownership.CommonDir},
			} {
				body, err := readRootedRegularFile(registration.root, file.name)
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err != nil || string(body) != file.expected {
					return errors.Join(fmt.Errorf("acceptance registration %s changed ownership", file.name), err)
				}
			}
			if _, err := registration.validateCurrentPath(false); err != nil {
				return err
			}
			if _, err := registry.validateCurrentPath(false); err != nil {
				return err
			}
			if err := registry.root.RemoveAll(ownership.RegistrationName); err != nil {
				return err
			}
			if err := registration.close(); err != nil {
				return err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := syncDirectory(registry.root); err != nil {
			return err
		}
	}
	resolved, err := filepath.EvalSymlinks(r.repo.path)
	if err != nil {
		return err
	}
	expected := filepath.Join(resolved, filepath.FromSlash(relative))
	entries, err := registeredWorktrees(r.repo)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if filepath.Clean(entry.path) != expected {
			continue
		}
		return fmt.Errorf("candidate worktree has an unowned replacement registration")
	}
	return nil
}

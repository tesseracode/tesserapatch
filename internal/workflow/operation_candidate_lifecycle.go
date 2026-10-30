package workflow

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
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

	recoveryPath, err := findOperationCandidateRecoveryJournal(s.Root, slug)
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
	artifact, state, err := readOperationCandidateArtifacts(s.Root, slug, pointer)
	if err != nil {
		return "", err
	}
	needsStale, err := candidate.CandidateNeedsStaleMark(artifact, state, current)
	if err != nil {
		return "", err
	}
	if !needsStale {
		return "", nil
	}
	state.Status = candidate.CandidateStatusStale
	stateRaw, err := candidate.EncodeCandidateState(state)
	if err != nil {
		return "", err
	}
	statePath := filepath.Join(s.Root, filepath.FromSlash(pointer.Path), candidate.CandidateStateName)
	if err := os.WriteFile(statePath, stateRaw, 0o644); err != nil {
		return "", err
	}
	status.Reconcile.OperationCandidate.Status = candidate.CandidateStatusStale
	if err := s.SaveFeatureStatus(status); err != nil {
		return "", err
	}
	if err := pruneOperationCandidateWorktree(s.Root, slug, pointer.CandidateID, artifact.UpstreamCommit, featureLock); err != nil {
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

func findOperationCandidateRecoveryJournal(repoRoot, slug string) (string, error) {
	slugRel, err := candidate.CandidateSlugRelative(slug)
	if err != nil {
		return "", err
	}
	slugAbs := filepath.Join(repoRoot, filepath.FromSlash(slugRel))
	info, err := os.Stat(slugAbs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("candidate slug root is not a directory: %s", slugRel)
	}
	var found string
	walkErr := filepath.WalkDir(slugAbs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != candidate.CandidateJournalName {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		found = filepath.ToSlash(rel)
		return fs.SkipAll
	})
	if walkErr != nil && !errors.Is(walkErr, fs.SkipAll) {
		return "", walkErr
	}
	return found, nil
}

func readOperationCandidateArtifacts(repoRoot, slug string, pointer *store.OperationCandidateSummary) (candidate.CandidateArtifact, candidate.CandidateState, error) {
	if err := store.ValidateOperationCandidateSummary(pointer); err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, err
	}
	expectedPath, err := candidate.CandidatePointerPath(slug, pointer.CandidateID)
	if err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, err
	}
	if pointer.Path != expectedPath {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, fmt.Errorf("candidate pointer path mismatch: have %s want %s", pointer.Path, expectedPath)
	}
	base := filepath.Join(repoRoot, filepath.FromSlash(pointer.Path))
	rawArtifact, err := os.ReadFile(filepath.Join(base, candidate.CandidateMetadataName))
	if err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, err
	}
	artifact, err := candidate.DecodeCandidateArtifact(rawArtifact)
	if err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, err
	}
	rawState, err := os.ReadFile(filepath.Join(base, candidate.CandidateStateName))
	if err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, err
	}
	state, err := candidate.DecodeCandidateState(rawState)
	if err != nil {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, err
	}
	if artifact.CandidateID != pointer.CandidateID || state.CandidateID != pointer.CandidateID {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, fmt.Errorf("candidate pointer/id mismatch")
	}
	if pointer.Status != state.Status || pointer.SourceFeatureState != artifact.SourceFeatureState {
		return candidate.CandidateArtifact{}, candidate.CandidateState{}, fmt.Errorf("candidate pointer state drift")
	}
	return artifact, state, nil
}

func pruneOperationCandidateWorktree(repoRoot, slug, candidateID, upstreamCommit string, featureLock *candidatelock.Handle) error {
	worktreeRel, err := candidate.CandidateWorktreeRelative(slug, candidateID)
	if err != nil {
		return err
	}
	worktreeAbs := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))
	if _, err := os.Stat(worktreeAbs); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	reference, err := candidate.OpenReferenceRoot(repoRoot, upstreamCommit)
	if err != nil {
		return err
	}
	defer func() { _ = reference.Close() }()
	root, err := candidate.OpenExistingCandidateRoot(reference, slug, candidateID)
	if err != nil {
		return err
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

func operationCandidateReuseCheck(existingArtifact candidate.CandidateArtifact, existingState candidate.CandidateState, derived candidate.CandidateArtifact, validate func() error) (operationCandidateReuseResult, error) {
	decision, err := candidate.CompareExistingCandidate(existingArtifact, existingState, derived)
	if err != nil {
		return operationCandidateReuseResult{}, err
	}
	if !decision.Reusable {
		return operationCandidateReuseResult{RefusalCode: decision.RefusalCode}, nil
	}
	if validate == nil {
		return operationCandidateReuseResult{}, fmt.Errorf("reuse validation function is required")
	}
	if err := validate(); err != nil {
		return operationCandidateReuseResult{}, err
	}
	return operationCandidateReuseResult{Reusable: true}, nil
}

func appendRejectedOperationCandidate(repoRoot, slug string, artifact candidate.CandidateArtifact) error {
	record := candidate.RejectedRecordFromArtifact(artifact)
	body, err := candidate.EncodeRejectedRecord(record)
	if err != nil {
		return err
	}
	pathRel, err := candidate.CandidateRejectedLogRelative(slug)
	if err != nil {
		return err
	}
	pathAbs := filepath.Join(repoRoot, filepath.FromSlash(pathRel))
	file, err := os.OpenFile(pathAbs, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		return err
	}
	return nil
}

func readRejectedOperationCandidates(repoRoot, slug string) ([]candidate.CandidateRejectedRecord, error) {
	pathRel, err := candidate.CandidateRejectedLogRelative(slug)
	if err != nil {
		return nil, err
	}
	pathAbs := filepath.Join(repoRoot, filepath.FromSlash(pathRel))
	file, err := os.Open(pathAbs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	var rows []candidate.CandidateRejectedRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		row, err := candidate.DecodeRejectedRecord([]byte(line))
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func isOperationCandidateRejected(repoRoot, slug, candidateID string) (bool, error) {
	rows, err := readRejectedOperationCandidates(repoRoot, slug)
	if err != nil {
		return false, err
	}
	return candidate.RejectedContainsCandidateID(rows, candidateID), nil
}

func clearOperationCandidateRejections(repoRoot, slug string) error {
	pathRel, err := candidate.CandidateRejectedLogRelative(slug)
	if err != nil {
		return err
	}
	pathAbs := filepath.Join(repoRoot, filepath.FromSlash(pathRel))
	if err := os.MkdirAll(filepath.Dir(pathAbs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(pathAbs, []byte{}, 0o644)
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

	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return "", err
	}
	pointer := store.CloneOperationCandidateSummary(status.Reconcile.OperationCandidate)
	if err := store.ValidateOperationCandidateSummary(pointer); err != nil {
		return "", err
	}
	if pointer == nil {
		return "", fmt.Errorf("candidate-absent")
	}
	artifact, state, err := readOperationCandidateArtifacts(s.Root, slug, pointer)
	if err != nil {
		return "", err
	}
	if state.Status == candidate.CandidateStatusAccepted {
		return "", fmt.Errorf("candidate-not-pending")
	}
	if state.Status != candidate.CandidateStatusPending && state.Status != candidate.CandidateStatusStale {
		return "", fmt.Errorf("candidate-not-pending")
	}
	if err := appendRejectedOperationCandidate(s.Root, slug, artifact); err != nil {
		return "", err
	}
	state.Status = candidate.CandidateStatusRejected
	stateRaw, err := candidate.EncodeCandidateState(state)
	if err != nil {
		return "", err
	}
	statePath := filepath.Join(s.Root, filepath.FromSlash(pointer.Path), candidate.CandidateStateName)
	if err := os.WriteFile(statePath, stateRaw, 0o644); err != nil {
		return "", err
	}
	if err := pruneOperationCandidateWorktree(s.Root, slug, pointer.CandidateID, artifact.UpstreamCommit, lock); err != nil {
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
	return clearOperationCandidateRejections(s.Root, slug)
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

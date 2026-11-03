package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const operationCandidatePruneDirectory = ".accept-prune"

func operationCandidateRecoveryIDs(disk *operationCandidateDisk, slug string) ([]string, error) {
	directory, err := candidate.CandidateSlugRelative(slug)
	if err != nil {
		return nil, err
	}
	root, _, check, err := disk.parent(path.Join(directory, "entry"), false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, err := dir.ReadDir(-1)
	closeErr := dir.Close()
	if err != nil || closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	var ids []string
	for _, entry := range entries {
		if _, err := candidate.CandidateDirectoryRelative(slug, entry.Name()); err != nil {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("candidate recovery namespace is not a regular directory")
		}
		// Presence is sufficient to block an ordinary command. A corrupt or
		// symlinked journal is still recovery evidence, never permission to run.
		_, err := root.Lstat(path.Join(entry.Name(), candidate.CandidateJournalName))
		if err == nil {
			ids = append(ids, entry.Name())
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	if err := check(); err != nil {
		return nil, err
	}
	sort.Strings(ids)
	return ids, nil
}

func refuseOperationCandidateJournal(s *store.Store, slug string) error {
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil {
		return candidateActionError(err)
	}
	if refusal.Reason != "" {
		return &OperationCandidateActionError{Code: refusal.Reason}
	}
	defer lock.Release()
	return operationCandidateJournalPreflightHeld(s, slug, lock)
}

func operationCandidateJournalPreflightHeld(s *store.Store, slug string, lock *candidatelock.Handle) error {
	release, err := borrowOperationCandidateFeatureMutation(lock, s.Root, slug)
	if err != nil {
		return err
	}
	defer release()
	disk, err := openOperationCandidateDisk(s.Root)
	if err != nil {
		return err
	}
	defer disk.root.Close()
	ids, err := operationCandidateRecoveryIDs(disk, slug)
	if err != nil || len(ids) != 0 {
		return &OperationCandidateActionError{Code: "recovery-required", Detail: "run --resume-candidate-recovery " + slug, Err: err}
	}
	return nil
}

// RefuseReconcileCandidateRecovery precedes CLI filtering, read-only exits,
// dirty-tree checks and provider probing. Only explicit recovery may bypass it.
// This read-only probe grants no mutation authority; held actions recheck.
func RefuseReconcileCandidateRecovery(s *store.Store, slugs []string) error {
	return refuseReconcileCandidateRecovery(s, slugs)
}

func refuseReconcileCandidateRecovery(s *store.Store, slugs []string) error {
	disk, err := openOperationCandidateDisk(s.Root)
	if err != nil {
		return err
	}
	defer disk.root.Close()
	if len(slugs) == 0 {
		root, _, check, err := disk.parent(".tpatch/reconcile-candidates/entry", false)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrOperationCandidateRecoveryRequired, err)
		}
		defer root.Close()
		dir, err := root.Open(".")
		if err != nil {
			return fmt.Errorf("%w: inspect candidate namespace: %v", ErrOperationCandidateRecoveryRequired, err)
		}
		entries, err := dir.ReadDir(-1)
		if err := errors.Join(err, dir.Close()); err != nil {
			return fmt.Errorf("%w: inspect candidate namespace: %v", ErrOperationCandidateRecoveryRequired, err)
		}
		for _, entry := range entries {
			if _, err := candidate.CandidateSlugRelative(entry.Name()); err == nil {
				slugs = append(slugs, entry.Name())
			}
		}
		if err := check(); err != nil {
			return fmt.Errorf("%w: inspect candidate namespace: %v", ErrOperationCandidateRecoveryRequired, err)
		}
		sort.Strings(slugs)
	}
	for _, slug := range slugs {
		if _, err := candidate.CandidateSlugRelative(slug); err != nil {
			return err
		}
		ids, err := operationCandidateRecoveryIDs(disk, slug)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrOperationCandidateRecoveryRequired, slug, err)
		}
		if len(ids) != 0 {
			return fmt.Errorf("%w: %s", ErrOperationCandidateRecoveryRequired, slug)
		}
	}
	return nil
}

func resumeOperationCandidateRecovery(s *store.Store, slug string) error {
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
	disk, err := openOperationCandidateDisk(s.Root)
	if err != nil {
		return err
	}
	defer disk.root.Close()
	ids, err := operationCandidateRecoveryIDs(disk, slug)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return &OperationCandidateActionError{Code: "candidate-absent", Detail: "no acceptance journal"}
	}
	if len(ids) != 1 {
		return fmt.Errorf("candidate recovery: multiple journals; refusing ambiguous restoration")
	}
	directory, _ := candidate.CandidateDirectoryRelative(slug, ids[0])
	image, err := disk.read(path.Join(directory, candidate.CandidateJournalName))
	if err != nil || !image.Present {
		return errors.Join(fmt.Errorf("candidate recovery: journal unavailable"), err)
	}
	journal, err := decodeOperationCandidateJournal(image.Data)
	if err != nil {
		return fmt.Errorf("candidate recovery: malformed journal: %w", err)
	}
	metadata, err := disk.read(path.Join(directory, candidate.CandidateMetadataName))
	if err != nil || !metadata.Present {
		return errors.Join(fmt.Errorf("candidate recovery: immutable metadata unavailable"), err)
	}
	artifact, err := candidate.DecodeCandidateArtifact(metadata.Data)
	if err != nil || artifact.Feature != slug || artifact.CandidateID != ids[0] {
		return errors.Join(fmt.Errorf("candidate recovery: immutable metadata mismatch"), err)
	}
	var changes []candidate.AcceptanceChange
	for _, name := range operationCandidateArtifactPaths(artifact) {
		changes = append(changes, candidate.AcceptanceChange{Path: name})
	}
	transaction, err := newOperationCandidateTransaction(s, artifact, store.FeatureStatus{}, changes, featureLock, globalLock)
	if err != nil {
		return err
	}
	defer transaction.close()
	if err := transaction.validateJournal(journal); err != nil {
		return fmt.Errorf("candidate recovery: %w", err)
	}
	transaction.journal, transaction.prepared = journal, true
	transaction.committed = journal.CommitMarker == "committed"
	transaction.step = candidateStepCleanup
	if err := transaction.disk.removeFile(path.Join(directory, ".accept-write")); err != nil {
		return fmt.Errorf("candidate recovery: interrupted write cleanup: %w", err)
	}
	if !transaction.committed {
		if err := transaction.rollback(); err != nil {
			return fmt.Errorf("candidate recovery: rollback failed; journal retained: %w", err)
		}
		return nil
	}
	upstream, err := candidate.OpenReferenceRoot(s.Root, artifact.UpstreamCommit)
	if err != nil {
		return fmt.Errorf("candidate recovery: committed worktree cleanup: %w", err)
	}
	defer upstream.Close()
	return transaction.cleanupCommitted(upstream)
}

func (t *operationCandidateTransaction) requireFreshPreparation() error {
	if t.prepared || t.step != candidateStepStage {
		return fmt.Errorf("preparation cleanup requires an unjournaled pending transaction")
	}
	parent, leaf, check, err := t.disk.parent(path.Join(t.directory, operationCandidatePruneDirectory), false)
	if err != nil {
		return err
	}
	_, inspectErr := parent.Lstat(leaf)
	checkErr := check()
	_ = parent.Close()
	if !errors.Is(inspectErr, fs.ErrNotExist) || checkErr != nil {
		return errors.Join(fmt.Errorf("unjournaled candidate has reserved prune residue; refusing ambiguous ownership"), inspectErr, checkErr)
	}
	// A crash before step 9 has no live writes to recover. Only these derived,
	// reserved preparation names can be discarded on a validated pending retry.
	return t.cleanupTransactionData(false)
}

func (t *operationCandidateTransaction) preparationFailed(cause error) error {
	step := t.step
	cleanupErr := t.cleanupTransactionData(false)
	if cleanupErr != nil {
		return fmt.Errorf("acceptance step %d failed before journal preparation; live paths were not written: %w (preparation cleanup: %v)", step, cause, cleanupErr)
	}
	if _, ok := OperationCandidateActionCode(cause); ok {
		return cause
	}
	return fmt.Errorf("acceptance step %d failed before journal preparation; live paths were not written: %w", step, cause)
}

func (t *operationCandidateTransaction) failed(cause error) error {
	step := t.step
	if t.committed {
		return fmt.Errorf("acceptance step %d: committed cleanup requires explicit recovery: %w", step, cause)
	}
	if err := t.rollback(); err != nil {
		return &OperationCandidateActionError{Code: "acceptance-rollback-failed",
			Detail: fmt.Sprintf("step %d failed: %v; rollback failed; journal retained", step, cause), Err: err}
	}
	code := "acceptance-rolled-back"
	if errors.Is(cause, ErrCoveragePublication) {
		code = "coverage-publication-failed"
	}
	return &OperationCandidateActionError{Code: code, Detail: fmt.Sprintf("step %d", step), Err: cause}
}

func (t *operationCandidateTransaction) rollback() error {
	if !t.prepared || t.committed || t.journal.CommitMarker != "" {
		return fmt.Errorf("rollback requires a prepared no-marker journal")
	}
	if err := t.validateJournal(t.journal); err != nil {
		return err
	}
	t.restoring = true
	defer func() { t.restoring = false }()
	for i, image := range t.journal.Snapshots {
		target := ""
		if i < len(t.changes) {
			target = t.changes[i].Path
		} else {
			target = path.Join(t.featureDir, operationCandidateFeatureFiles[i-len(t.changes)])
		}
		if err := operationCandidateAcceptanceHook(t.step, "restore", target); err != nil {
			return err
		}
		if image.Present {
			if _, err := t.replace(target, image.Data, fs.FileMode(image.Mode)); err != nil {
				return err
			}
		} else {
			if err := t.authorizeWrite(target); err != nil {
				return err
			}
			if err := t.disk.removeFile(target); err != nil {
				return err
			}
		}
	}
	if _, err := t.replace(path.Join(t.directory, candidate.CandidateStateName), []byte(t.journal.PendingState), fs.FileMode(t.journal.PendingStateMode)); err != nil {
		return err
	}
	for i := len(t.journal.MissingDirectories) - 1; i >= 0; i-- {
		if err := t.disk.removeEmptyDirectory(t.journal.MissingDirectories[i], true); err != nil {
			return err
		}
	}
	return t.cleanupTransactionData(true)
}

func (t *operationCandidateTransaction) cleanupCommitted(upstream *candidate.ReferenceRoot) error {
	if !t.prepared || !t.committed || t.journal.CommitMarker != "committed" {
		return fmt.Errorf("cleanup requires the committed acceptance journal")
	}
	if t.journal.PruneOwnership == nil {
		return fmt.Errorf("acceptance step 14: original cleanup ownership is missing; journal retained")
	}
	if err := t.relocatePruneWorktree(); err != nil {
		return fmt.Errorf("acceptance step 14: worktree relocation failed; journal retained: %w", err)
	}
	if err := upstream.PruneAcceptanceWorktree(t.slug, t.id, t.featureLock, t.globalLock, *t.journal.PruneOwnership, operationCandidateSyncDirectory); err != nil {
		return fmt.Errorf("acceptance step 14: prune failed; journal retained: %w", err)
	}
	if err := operationCandidateAcceptanceHook(t.step, "worktree-unregistered", operationCandidatePruneDirectory); err != nil {
		return err
	}
	if err := t.removePrunedWorktree(); err != nil {
		return fmt.Errorf("acceptance step 14: reserved worktree cleanup failed; journal retained: %w", err)
	}
	if err := operationCandidateAcceptanceHook(t.step, "pruned", "worktree"); err != nil {
		return err
	}
	image, err := t.disk.read(path.Join(t.featureDir, "status.json"))
	if err != nil {
		return err
	}
	var status store.FeatureStatus
	if !image.Present || json.Unmarshal(image.Data, &status) != nil || status.Slug != t.slug {
		return fmt.Errorf("acceptance cleanup: feature status is unavailable; journal retained")
	}
	if pointer := status.Reconcile.OperationCandidate; pointer != nil {
		if pointer.CandidateID != t.id || pointer.Path != t.directory || pointer.Status != candidate.CandidateStatusAccepted {
			return fmt.Errorf("acceptance cleanup: pointer belongs to another transition; journal retained")
		}
		status.Reconcile.OperationCandidate = nil
		body, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		if _, err := t.replace(path.Join(t.featureDir, "status.json"), append(body, '\n'), fs.FileMode(image.Mode)); err != nil {
			return err
		}
	}
	return t.cleanupTransactionData(true)
}

// Original native identities were fsynced in the initial step-9 journal.
// Neither the reserved name nor a freshly opened root establishes ownership.
func (t *operationCandidateTransaction) relocatePruneWorktree() error {
	if t.release == nil || !t.committed || t.step != candidateStepCleanup || t.journal.PruneOwnership == nil {
		return fmt.Errorf("worktree relocation requires the committed held transaction")
	}
	parent, _, check, err := t.disk.parent(path.Join(t.directory, "entry"), false)
	if err != nil {
		return err
	}
	defer parent.Close()
	worktree, worktreeErr := parent.Lstat(candidate.CandidateWorktreeName)
	pruned, pruneErr := parent.Lstat(operationCandidatePruneDirectory)
	if worktreeErr != nil && !errors.Is(worktreeErr, fs.ErrNotExist) {
		return worktreeErr
	}
	if pruneErr != nil && !errors.Is(pruneErr, fs.ErrNotExist) {
		return pruneErr
	}
	if pruneErr == nil {
		if !pruned.IsDir() || pruned.Mode()&os.ModeSymlink != 0 || worktreeErr == nil {
			return fmt.Errorf("ambiguous reserved prune subtree")
		}
		held, err := parent.OpenRoot(operationCandidatePruneDirectory)
		if err != nil {
			return err
		}
		defer held.Close()
		if err := t.journal.PruneOwnership.Worktree.CheckRoot(held); err != nil {
			return err
		}
		return check()
	}
	if errors.Is(worktreeErr, fs.ErrNotExist) {
		return check()
	}
	if !worktree.IsDir() || worktree.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("candidate worktree is not a regular directory")
	}
	held, err := parent.OpenRoot(candidate.CandidateWorktreeName)
	if err != nil {
		return err
	}
	defer held.Close()
	if err := t.journal.PruneOwnership.Worktree.CheckRoot(held); err != nil {
		return err
	}
	actual, err := held.Stat(".")
	if err != nil || !os.SameFile(worktree, actual) {
		return fmt.Errorf("candidate worktree identity changed before relocation")
	}
	if err := check(); err != nil {
		return err
	}
	current, err := parent.Lstat(candidate.CandidateWorktreeName)
	if err != nil || !os.SameFile(worktree, current) {
		return fmt.Errorf("candidate worktree identity changed before relocation")
	}
	if err := parent.Rename(candidate.CandidateWorktreeName, operationCandidatePruneDirectory); err != nil {
		return err
	}
	if err := operationCandidateSyncDirectory(parent); err != nil {
		return err
	}
	return operationCandidateAcceptanceHook(t.step, "worktree-relocated", operationCandidatePruneDirectory)
}

func (t *operationCandidateTransaction) removePrunedWorktree() error {
	if t.release == nil || !t.committed || t.step != candidateStepCleanup || t.journal.PruneOwnership == nil {
		return fmt.Errorf("reserved subtree removal requires the committed held transaction")
	}
	parent, leaf, check, err := t.disk.parent(path.Join(t.directory, operationCandidatePruneDirectory), false)
	if err != nil {
		return err
	}
	defer parent.Close()
	info, err := parent.Lstat(leaf)
	if errors.Is(err, fs.ErrNotExist) {
		return operationCandidateSyncDirectory(parent)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("reserved prune subtree is not a regular directory")
	}
	owned, err := parent.OpenRoot(leaf)
	if err != nil {
		return err
	}
	defer owned.Close()
	if err := t.journal.PruneOwnership.Worktree.CheckRoot(owned); err != nil {
		return err
	}
	actual, err := owned.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		return fmt.Errorf("reserved prune subtree identity changed")
	}
	if err := check(); err != nil {
		return err
	}
	current, err := parent.Lstat(leaf)
	if err != nil || !os.SameFile(info, current) {
		return fmt.Errorf("reserved prune subtree identity changed before removal")
	}
	// The leaf is fixed and its original native identity is journal-bound.
	// os.Root does not traverse symlink children.
	if err := parent.RemoveAll(leaf); err != nil {
		return err
	}
	if err := owned.Close(); err != nil {
		return err
	}
	return operationCandidateSyncDirectory(parent)
}

func (t *operationCandidateTransaction) cleanupTransactionData(removeJournal bool) error {
	if t.release == nil {
		return fmt.Errorf("transaction cleanup has no live lock borrow")
	}
	var snapshots []string
	for i := 0; i < len(t.changes)+len(operationCandidateFeatureFiles); i++ {
		snapshots = append(snapshots, path.Base(t.snapshotPath(i)))
	}
	if err := t.removeKnownDirectory(path.Join(t.directory, "snapshots"), snapshots); err != nil {
		return err
	}
	if err := operationCandidateAcceptanceHook(t.step, "snapshots-removed", "snapshots"); err != nil {
		return err
	}
	var staged []string
	for _, name := range operationCandidateFeatureFiles[:len(operationCandidateFeatureFiles)-1] {
		staged = append(staged, path.Base(name))
	}
	if err := t.removeKnownDirectory(path.Join(t.directory, "staged"), staged); err != nil {
		return err
	}
	if err := t.disk.removeFile(path.Join(t.directory, ".accept-write")); err != nil {
		return err
	}
	if !removeJournal {
		return nil
	}
	parent, leaf, check, err := t.disk.parent(path.Join(t.directory, operationCandidatePruneDirectory), false)
	if err != nil {
		return err
	}
	_, inspectErr := parent.Lstat(leaf)
	checkErr := check()
	_ = parent.Close()
	if !errors.Is(inspectErr, fs.ErrNotExist) || checkErr != nil {
		return fmt.Errorf("reserved prune cleanup is incomplete; journal retained")
	}
	if err := operationCandidateAcceptanceHook(t.step, "before-journal-removal", candidate.CandidateJournalName); err != nil {
		return err
	}
	journalPath := path.Join(t.directory, candidate.CandidateJournalName)
	if err := t.disk.removeFile(journalPath); err != nil {
		// If unlink succeeded but its sync failed, put the self-contained
		// journal back before returning a failed resume/cleanup.
		body, encodeErr := json.MarshalIndent(t.journal, "", "  ")
		if encodeErr != nil {
			return errors.Join(err, encodeErr)
		}
		_, restoreErr := t.disk.replace(journalPath, append(body, '\n'), 0o600, t.directory)
		return errors.Join(err, restoreErr)
	}
	return nil
}

func (t *operationCandidateTransaction) removeKnownDirectory(name string, allowed []string) error {
	root, _, check, err := t.disk.parent(path.Join(name, "entry"), false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	closeErr := dir.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	for _, entry := range entries {
		if !slices.Contains(allowed, entry.Name()) || !entry.Type().IsRegular() {
			return fmt.Errorf("acceptance cleanup refuses an unregistered transaction entry: %s", path.Join(name, entry.Name()))
		}
	}
	if err := check(); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := t.disk.removeFile(path.Join(name, entry.Name())); err != nil {
			return err
		}
	}
	return t.disk.removeEmptyDirectory(name, false)
}

func (d *operationCandidateDisk) removeEmptyDirectory(name string, keepForeignChildren bool) error {
	parent, leaf, check, err := d.parent(name, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()
	info, err := parent.Lstat(leaf)
	if errors.Is(err, fs.ErrNotExist) {
		return operationCandidateSyncDirectory(parent)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("acceptance cleanup target is not a directory")
	}
	if err := check(); err != nil {
		return err
	}
	if err := parent.Remove(leaf); err != nil {
		if keepForeignChildren {
			dir, openErr := parent.Open(leaf)
			if openErr == nil {
				entries, readErr := dir.ReadDir(1)
				_ = dir.Close()
				if len(entries) != 0 && readErr == nil {
					return nil
				}
			}
		}
		return err
	}
	return operationCandidateSyncDirectory(parent)
}

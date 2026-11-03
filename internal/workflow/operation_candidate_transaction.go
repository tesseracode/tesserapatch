package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type operationCandidateAcceptStep int

const (
	candidateStepLocks operationCandidateAcceptStep = iota + 1
	candidateStepRecoveryCheck
	candidateStepPending
	candidateStepRederive
	candidateStepRematerialize
	candidateStepPreconditions
	candidateStepStage
	candidateStepSnapshot
	candidateStepJournal
	candidateStepApply
	candidateStepPublish
	candidateStepAccepted
	candidateStepCommit
	candidateStepCleanup
)

var operationCandidatePreparedSteps = []operationCandidateAcceptStep{
	candidateStepLocks, candidateStepRecoveryCheck, candidateStepPending,
	candidateStepRederive, candidateStepRematerialize, candidateStepPreconditions,
	candidateStepStage, candidateStepSnapshot, candidateStepJournal,
	candidateStepApply, candidateStepPublish, candidateStepAccepted,
	candidateStepCommit, candidateStepCleanup,
}

// This registry drives staging, snapshotting, publication and restoration.
// Status is deliberately not published until step 12.
var operationCandidateFeatureFiles = []string{
	"artifacts/post-apply.patch",
	"artifacts/apply-recipe.json",
	"artifacts/recipe-provenance.json",
	"artifacts/patch-generations.json",
	"artifacts/recipe-capture-event.json",
	"artifacts/recipe-coverage.json",
	"status.json",
}

var operationCandidateAcceptanceHook = func(operationCandidateAcceptStep, string, string) error { return nil }

type operationCandidateSnapshot struct {
	Scope   string `json:"scope"`
	Path    string `json:"path"`
	Present bool   `json:"present"`
	Mode    uint32 `json:"mode"`
	Data    []byte `json:"data"`
	SHA256  string `json:"sha256"`
}

// Journal snapshots are also retained inline. Removing snapshots/ can therefore
// never strand a no-marker journal without its restoration data.
type operationCandidateJournal struct {
	Version            int                            `json:"version"`
	Feature            string                         `json:"feature"`
	CandidateID        string                         `json:"candidate_id"`
	MetadataSHA256     string                         `json:"metadata_sha256"`
	PreparedSteps      []operationCandidateAcceptStep `json:"prepared_steps"`
	Snapshots          []operationCandidateSnapshot   `json:"snapshots"`
	MissingDirectories []string                       `json:"missing_directories"`
	PendingState       string                         `json:"pending_state"`
	PendingStateMode   uint32                         `json:"pending_state_mode"`
	ApplyBefore        store.ApplySummary             `json:"apply_before"`
	StateBefore        store.FeatureState             `json:"state_before"`
	CommitMarker       string                         `json:"commit_marker"`
	PruneOwnership     *candidate.AcceptanceOwnership `json:"prune_ownership"`
}

type operationCandidateTransaction struct {
	disk        *operationCandidateDisk
	featureLock *candidatelock.Handle
	globalLock  *candidatelock.Handle
	release     func()
	slug        string
	id          string
	directory   string
	featureDir  string
	artifact    candidate.CandidateArtifact
	changes     []candidate.AcceptanceChange
	status      store.FeatureStatus
	journal     operationCandidateJournal
	prepared    bool
	committed   bool
	restoring   bool
	step        operationCandidateAcceptStep
	staged      map[string][]byte
}

func newOperationCandidateTransaction(s *store.Store, artifact candidate.CandidateArtifact, status store.FeatureStatus, changes []candidate.AcceptanceChange, featureLock, globalLock *candidatelock.Handle) (*operationCandidateTransaction, error) {
	directory, err := candidate.CandidateDirectoryRelative(artifact.Feature, artifact.CandidateID)
	if err != nil {
		return nil, err
	}
	disk, err := openOperationCandidateDisk(s.Root)
	if err != nil {
		return nil, err
	}
	release, err := borrowOperationCandidateTransactionLocks(s.Root, artifact.Feature, featureLock, globalLock)
	if err != nil {
		_ = disk.root.Close()
		return nil, err
	}
	t := &operationCandidateTransaction{
		disk: disk, featureLock: featureLock, globalLock: globalLock, release: release,
		slug: artifact.Feature, id: artifact.CandidateID, directory: directory,
		featureDir: path.Join(".tpatch", "features", artifact.Feature),
		artifact:   artifact, changes: changes, status: status, staged: make(map[string][]byte),
	}
	if err := t.validatePaths(); err != nil {
		t.close()
		return nil, err
	}
	return t, nil
}

func borrowOperationCandidateTransactionLocks(root, slug string, featureLock, globalLock *candidatelock.Handle) (func(), error) {
	if featureLock == nil || globalLock == nil || featureLock == globalLock {
		return nil, fmt.Errorf("acceptance requires distinct feature and global capabilities")
	}
	releaseFeature, err := featureLock.Borrow(filepath.Join(root, ".tpatch", "locks", "reconcile", slug+".lock"))
	if err != nil {
		return nil, err
	}
	releaseGlobal, err := globalLock.Borrow(filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock"))
	if err != nil {
		releaseFeature()
		return nil, err
	}
	return func() { releaseGlobal(); releaseFeature() }, nil
}

func (t *operationCandidateTransaction) close() {
	if t.release != nil {
		t.release()
		t.release = nil
	}
	if t.disk != nil {
		_ = t.disk.root.Close()
	}
}

func (t *operationCandidateTransaction) validatePaths() error {
	if err := t.artifact.Validate(); err != nil {
		return err
	}
	id, _, _, err := candidate.ComputeCandidateID(candidate.CandidateIdentityInput{
		SchemaVersion: t.artifact.SchemaVersion, Feature: t.artifact.Feature, SourceFeatureState: t.artifact.SourceFeatureState,
		UpstreamCommit: t.artifact.UpstreamCommit, ReferenceCommit: t.artifact.ReferenceCommit,
		Bindings: t.artifact.Bindings, DerivationMode: t.artifact.DerivationMode,
		Operations: t.artifact.Derivation.Operations, EffectMap: t.artifact.Derivation.EffectMap,
	})
	if err != nil || id != t.id {
		return fmt.Errorf("acceptance namespace does not match immutable metadata identity")
	}
	paths := operationCandidateArtifactPaths(t.artifact)
	if len(paths) == 0 || len(paths) != len(t.changes) {
		return fmt.Errorf("candidate path set differs from re-derived operations")
	}
	for i, name := range paths {
		if err := operationCandidateLivePath(name); err != nil {
			return err
		}
		if name != t.changes[i].Path {
			return fmt.Errorf("candidate path set differs from re-derived operations")
		}
		if i > 0 && strings.HasPrefix(name, paths[i-1]+"/") {
			return fmt.Errorf("candidate paths overlap")
		}
	}
	return nil
}

func operationCandidateArtifactPaths(artifact candidate.CandidateArtifact) []string {
	var paths []string
	for _, op := range artifact.Derivation.Operations {
		paths = append(paths, op.Path)
	}
	sort.Strings(paths)
	return slices.Compact(paths)
}

func operationCandidateLivePath(name string) error {
	if err := operationCandidateRelativePath(name); err != nil {
		return err
	}
	for _, part := range strings.Split(name, "/") {
		if strings.EqualFold(part, ".git") || strings.EqualFold(part, ".tpatch") {
			return fmt.Errorf("candidate live path enters a control namespace: %s", name)
		}
	}
	return nil
}

func operationCandidateRelativePath(name string) error {
	if name == "" || name == "." || !fs.ValidPath(name) || path.Clean(name) != name ||
		filepath.IsAbs(name) || filepath.VolumeName(name) != "" ||
		strings.ContainsAny(name, "\\:\x00") {
		return fmt.Errorf("candidate path is not a normalized relative path: %q", name)
	}
	return nil
}

func (t *operationCandidateTransaction) before(step operationCandidateAcceptStep) error {
	t.step = step
	return operationCandidateAcceptanceHook(step, "before", "")
}

func (t *operationCandidateTransaction) checkCAS(recheck bool) error {
	for _, change := range t.changes {
		image, err := t.disk.read(change.Path)
		code := "candidate-cas-failed"
		if !recheck && !change.Present {
			code = "candidate-path-occupied"
		}
		if err != nil || image.Present != change.Present ||
			(change.Present && (!bytes.Equal([]byte(image.Data), change.Before) || !operationCandidateRegularMode(fs.FileMode(image.Mode)))) {
			return &OperationCandidateActionError{Code: code, Detail: fmt.Sprintf("step %d: %s does not match upstream %s", t.step, change.Path, t.artifact.UpstreamCommit), Err: err}
		}
	}
	return nil
}

func operationCandidateRegularMode(mode fs.FileMode) bool {
	if !mode.IsRegular() || mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		return mode.Perm() == 0o666
	}
	return mode.Perm() == 0o644
}

func (t *operationCandidateTransaction) authorizeWrite(relative string) error {
	if t.release == nil {
		return fmt.Errorf("acceptance mutation has no live lock borrow")
	}
	if err := t.disk.check(); err != nil {
		return err
	}
	for _, change := range t.changes {
		if relative == change.Path && (t.restoring || t.step == candidateStepApply) && t.prepared && !t.committed {
			return nil
		}
	}
	for _, name := range operationCandidateFeatureFiles {
		if relative != path.Join(t.featureDir, name) {
			continue
		}
		if t.restoring && t.prepared && !t.committed {
			return nil
		}
		if t.prepared && ((name == "status.json" && (t.step == candidateStepAccepted || t.step == candidateStepCleanup)) ||
			(name != "status.json" && t.step == candidateStepPublish && !t.committed)) {
			return nil
		}
	}
	if relative == path.Join(t.directory, candidate.CandidateStateName) && t.prepared && !t.committed &&
		(t.restoring || t.step == candidateStepAccepted) {
		return nil
	}
	if relative == path.Join(t.directory, candidate.CandidateJournalName) &&
		(t.step == candidateStepJournal || (t.prepared && t.step == candidateStepCommit)) {
		return nil
	}
	if t.step == candidateStepStage {
		for _, name := range operationCandidateFeatureFiles[:len(operationCandidateFeatureFiles)-1] {
			if relative == t.stagedPath(name) {
				return nil
			}
		}
	}
	if t.step == candidateStepSnapshot {
		for i := range t.journal.Snapshots {
			if relative == t.snapshotPath(i) {
				return nil
			}
		}
	}
	return fmt.Errorf("unjournaled acceptance write refused: step %d path %s", t.step, relative)
}

func (t *operationCandidateTransaction) replace(relative string, data []byte, mode fs.FileMode) (bool, error) {
	if err := t.authorizeWrite(relative); err != nil {
		return false, err
	}
	return t.disk.replace(relative, data, mode, t.directory)
}

func (t *operationCandidateTransaction) stagedPath(name string) string {
	return path.Join(t.directory, "staged", path.Base(name))
}

func (t *operationCandidateTransaction) snapshotPath(index int) string {
	return path.Join(t.directory, "snapshots", fmt.Sprintf("%06d.json", index))
}

func (t *operationCandidateTransaction) writePublicationFile(name string, data []byte) error {
	if !slices.Contains(operationCandidateFeatureFiles[:len(operationCandidateFeatureFiles)-1], name) {
		return fmt.Errorf("unregistered acceptance publication: %s", name)
	}
	switch t.step {
	case candidateStepStage:
		if _, exists := t.staged[name]; exists {
			return fmt.Errorf("acceptance artifact staged twice: %s", name)
		}
		if err := operationCandidateAcceptanceHook(t.step, "stage", name); err != nil {
			return err
		}
		if _, err := t.replace(t.stagedPath(name), data, 0o600); err != nil {
			return err
		}
		t.staged[name] = bytes.Clone(data)
	case candidateStepPublish:
		staged, err := t.disk.read(t.stagedPath(name))
		if err != nil || !staged.Present || !bytes.Equal([]byte(staged.Data), data) ||
			!bytes.Equal(t.staged[name], data) {
			return errors.Join(fmt.Errorf("staged artifact changed: %s", name), err)
		}
		if err := operationCandidateAcceptanceHook(t.step, "publish", name); err != nil {
			return err
		}
		if _, err := t.replace(path.Join(t.featureDir, name), data, 0o644); err != nil {
			return err
		}
		return operationCandidateAcceptanceHook(t.step, "published", name)
	default:
		return fmt.Errorf("publication outside acceptance steps 7/11")
	}
	return nil
}

func (t *operationCandidateTransaction) snapshot(state candidate.CandidateState, root *candidate.CandidateRoot) error {
	metadata, err := candidate.EncodeCandidateArtifact(t.artifact)
	if err != nil {
		return err
	}
	pending, err := t.disk.read(path.Join(t.directory, candidate.CandidateStateName))
	if err != nil || !pending.Present {
		return errors.Join(fmt.Errorf("pending state disappeared before snapshot"), err)
	}
	checked, err := candidate.DecodeCandidateState(pending.Data)
	if err != nil || checked != state {
		return errors.Join(fmt.Errorf("pending state changed before snapshot"), err)
	}
	if !fs.FileMode(pending.Mode).IsRegular() {
		return fmt.Errorf("pending state is not a regular file")
	}
	ownership, err := root.CaptureAcceptanceOwnership(t.featureLock, t.globalLock)
	if err != nil {
		return err
	}
	t.journal = operationCandidateJournal{
		Version: 1, Feature: t.slug, CandidateID: t.id, MetadataSHA256: store.SHA256HexBytes(metadata),
		PreparedSteps: slices.Clone(operationCandidatePreparedSteps), Snapshots: []operationCandidateSnapshot{},
		MissingDirectories: []string{}, PendingState: string(pending.Data), PendingStateMode: pending.Mode,
		ApplyBefore: t.status.Apply, StateBefore: t.status.State, PruneOwnership: &ownership,
	}
	for _, change := range t.changes {
		image, err := t.disk.read(change.Path)
		if err != nil {
			return err
		}
		// An editor between steps 6 and 8 must not become the rollback baseline.
		if image.Present != change.Present || (image.Present &&
			(!bytes.Equal([]byte(image.Data), change.Before) || !operationCandidateRegularMode(fs.FileMode(image.Mode)))) {
			return &OperationCandidateActionError{Code: "candidate-cas-failed", Detail: "step 8: " + change.Path}
		}
		image.Scope, image.Path = "worktree", change.Path
		t.journal.Snapshots = append(t.journal.Snapshots, image)
		if !change.Present {
			missing, err := t.disk.missingParents(change.Path)
			if err != nil {
				return err
			}
			t.journal.MissingDirectories = append(t.journal.MissingDirectories, missing...)
		}
	}
	for _, name := range operationCandidateFeatureFiles {
		image, err := t.disk.read(path.Join(t.featureDir, name))
		if err != nil {
			return err
		}
		image.Scope, image.Path = "feature", name
		t.journal.Snapshots = append(t.journal.Snapshots, image)
	}
	sort.Strings(t.journal.MissingDirectories)
	t.journal.MissingDirectories = slices.Compact(t.journal.MissingDirectories)
	if err := t.validateJournal(t.journal); err != nil {
		return err
	}
	// Preserve unrelated status fields observed at the snapshot boundary.
	if err := json.Unmarshal(t.journal.Snapshots[len(t.journal.Snapshots)-1].Data, &t.status); err != nil {
		return err
	}
	for i, image := range t.journal.Snapshots {
		data, err := json.Marshal(image)
		if err != nil {
			return err
		}
		if _, err := t.replace(t.snapshotPath(i), append(data, '\n'), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (t *operationCandidateTransaction) prepareJournal() error {
	if err := t.validateJournal(t.journal); err != nil {
		return err
	}
	for _, name := range operationCandidateFeatureFiles[:len(operationCandidateFeatureFiles)-1] {
		data, ok := t.staged[name]
		if !ok {
			return fmt.Errorf("acceptance staging is incomplete: %s", name)
		}
		image, err := t.disk.read(t.stagedPath(name))
		if err != nil || !image.Present || !bytes.Equal([]byte(image.Data), data) {
			return fmt.Errorf("acceptance staged artifact changed: %s", name)
		}
	}
	body, err := json.MarshalIndent(t.journal, "", "  ")
	if err != nil {
		return err
	}
	installed, err := t.replace(path.Join(t.directory, candidate.CandidateJournalName), append(body, '\n'), 0o600)
	// A directory-sync error after rename still leaves recovery evidence.
	t.prepared = installed
	return err
}

func (t *operationCandidateTransaction) applyLive() error {
	if err := t.checkCAS(true); err != nil {
		return err
	}
	for _, change := range t.changes {
		if _, err := t.replace(change.Path, change.After, 0o644); err != nil {
			return err
		}
		if err := operationCandidateAcceptanceHook(t.step, "written", change.Path); err != nil {
			return err
		}
	}
	return nil
}

func (t *operationCandidateTransaction) markCommitted() error {
	t.journal.CommitMarker = "committed"
	body, err := json.MarshalIndent(t.journal, "", "  ")
	if err != nil {
		return err
	}
	installed, err := t.replace(path.Join(t.directory, candidate.CandidateJournalName), append(body, '\n'), 0o600)
	t.committed = installed
	if !installed {
		t.journal.CommitMarker = ""
	}
	return err
}

func (t *operationCandidateTransaction) validateJournal(j operationCandidateJournal) error {
	metadata, err := candidate.EncodeCandidateArtifact(t.artifact)
	if err != nil {
		return err
	}
	if j.Version != 1 || j.Feature != t.slug || j.CandidateID != t.id ||
		j.MetadataSHA256 != store.SHA256HexBytes(metadata) ||
		!slices.Equal(j.PreparedSteps, operationCandidatePreparedSteps) ||
		(j.CommitMarker != "" && j.CommitMarker != "committed") {
		return fmt.Errorf("acceptance journal identity/step inventory mismatch")
	}
	if j.PruneOwnership != nil {
		if err := j.PruneOwnership.Validate(t.disk.path, t.slug, t.id, t.artifact.UpstreamCommit); err != nil {
			return err
		}
	} else if j.CommitMarker == "committed" {
		return fmt.Errorf("committed acceptance journal lacks original cleanup ownership")
	}
	pending, err := candidate.DecodeCandidateState([]byte(j.PendingState))
	if err != nil || pending.CandidateID != t.id || pending.Status != candidate.CandidateStatusPending ||
		!fs.FileMode(j.PendingStateMode).IsRegular() {
		return fmt.Errorf("acceptance journal lacks its pending state")
	}
	if len(j.Snapshots) != len(t.changes)+len(operationCandidateFeatureFiles) || j.MissingDirectories == nil {
		return fmt.Errorf("acceptance snapshot inventory mismatch")
	}
	for i, image := range j.Snapshots {
		scope, name := "worktree", ""
		if i < len(t.changes) {
			name = t.changes[i].Path
		} else {
			scope, name = "feature", operationCandidateFeatureFiles[i-len(t.changes)]
		}
		if image.Scope != scope || image.Path != name || image.Data == nil || image.SHA256 != store.SHA256HexBytes(image.Data) ||
			(!image.Present && (image.Mode != 0 || len(image.Data) != 0)) ||
			(image.Present && !fs.FileMode(image.Mode).IsRegular()) {
			return fmt.Errorf("acceptance snapshot does not match registered target %s", name)
		}
	}
	statusImage := j.Snapshots[len(j.Snapshots)-1]
	var status store.FeatureStatus
	if !statusImage.Present || json.Unmarshal([]byte(statusImage.Data), &status) != nil ||
		status.Slug != t.slug || status.State != j.StateBefore || status.Apply != j.ApplyBefore ||
		string(status.State) != t.artifact.SourceFeatureState ||
		status.Reconcile.OperationCandidate == nil {
		return fmt.Errorf("acceptance status snapshot does not bind source state")
	}
	pointer := status.Reconcile.OperationCandidate
	if pointer.CandidateID != t.id || pointer.Path != t.directory ||
		pointer.SourceFeatureState != t.artifact.SourceFeatureState || pointer.Status != candidate.CandidateStatusPending {
		return fmt.Errorf("acceptance status snapshot pointer mismatch")
	}
	if !slices.IsSorted(j.MissingDirectories) || len(slices.Compact(slices.Clone(j.MissingDirectories))) != len(j.MissingDirectories) {
		return fmt.Errorf("acceptance absent-directory inventory is not unique and sorted")
	}
	for _, directory := range j.MissingDirectories {
		if err := operationCandidateLivePath(directory); err != nil {
			return err
		}
		owned := false
		for i, change := range t.changes {
			if !j.Snapshots[i].Present && strings.HasPrefix(change.Path, directory+"/") {
				owned = true
			}
		}
		if !owned {
			return fmt.Errorf("acceptance absent directory is not a creation ancestor")
		}
	}
	return nil
}

func decodeOperationCandidateJournal(raw []byte) (operationCandidateJournal, error) {
	var journal operationCandidateJournal
	var node store.CanonNode
	if err := node.UnmarshalJSON(raw); err != nil {
		return journal, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&journal); err != nil {
		return journal, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return journal, fmt.Errorf("acceptance journal has trailing input")
	}
	// Compare declared JSON keys recursively, including omitted ApplySummary
	// fields. CanonNode rejects duplicate members before encoding/json sees them.
	canonical, err := json.Marshal(journal)
	if err != nil {
		return journal, err
	}
	var expected store.CanonNode
	if err := expected.UnmarshalJSON(canonical); err != nil {
		return journal, err
	}
	normalizeOperationCandidateJournalObjectOrder(&node)
	normalizeOperationCandidateJournalObjectOrder(&expected)
	if !reflect.DeepEqual(node, expected) {
		return journal, fmt.Errorf("acceptance journal has missing, null or noncanonical typed members")
	}
	return journal, nil
}

func normalizeOperationCandidateJournalObjectOrder(node *store.CanonNode) {
	for i := range node.Array {
		normalizeOperationCandidateJournalObjectOrder(&node.Array[i])
	}
	for i := range node.Object {
		normalizeOperationCandidateJournalObjectOrder(&node.Object[i].Value)
	}
	slices.SortFunc(node.Object, func(a, b store.CanonField) int { return strings.Compare(a.Key, b.Key) })
}

type operationCandidateDisk struct {
	path     string
	root     *os.Root
	identity os.FileInfo
}

func openOperationCandidateDisk(root string) (*operationCandidateDisk, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := safety.EnsureSafeRepoPath(absolute, absolute); err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if err != nil || info == nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(fmt.Errorf("acceptance repository root is not a safe directory"), err)
	}
	held, err := os.OpenRoot(absolute)
	if err != nil {
		return nil, err
	}
	actual, err := held.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		_ = held.Close()
		return nil, fmt.Errorf("acceptance repository root changed")
	}
	return &operationCandidateDisk{path: absolute, root: held, identity: info}, nil
}

func (d *operationCandidateDisk) check() error {
	info, err := os.Lstat(d.path)
	if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, d.identity) {
		return fmt.Errorf("acceptance repository root identity changed")
	}
	return nil
}

// All leaf opens use a held, component-checked parent. os.Root alone would
// confine symlinks but would still follow an in-root symlink to another target.
func (d *operationCandidateDisk) parent(name string, create bool) (*os.Root, string, func() error, error) {
	if err := operationCandidateRelativePath(name); err != nil {
		return nil, "", nil, err
	}
	if err := d.check(); err != nil {
		return nil, "", nil, err
	}
	held, err := d.root.OpenRoot(".")
	if err != nil {
		return nil, "", nil, err
	}
	var identities []os.FileInfo
	var relatives []string
	relative := ""
	parts := strings.Split(name, "/")
	for _, part := range parts[:len(parts)-1] {
		relative = path.Join(relative, part)
		info, err := held.Lstat(part)
		if errors.Is(err, fs.ErrNotExist) && create {
			err = held.Mkdir(part, 0o755)
			if err == nil {
				err = operationCandidateSyncDirectory(held)
			}
			if err == nil {
				info, err = held.Lstat(part)
			}
		}
		if err != nil {
			_ = held.Close()
			return nil, "", nil, err
		}
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			_ = held.Close()
			return nil, "", nil, fmt.Errorf("acceptance parent is not a regular directory: %s", relative)
		}
		next, err := held.OpenRoot(part)
		_ = held.Close()
		if err != nil {
			return nil, "", nil, err
		}
		actual, err := next.Stat(".")
		if err != nil || !os.SameFile(actual, info) {
			_ = next.Close()
			return nil, "", nil, fmt.Errorf("acceptance parent was replaced: %s", relative)
		}
		held = next
		identities, relatives = append(identities, info), append(relatives, relative)
	}
	check := func() error {
		if err := d.check(); err != nil {
			return err
		}
		for i, relative := range relatives {
			info, err := d.root.Lstat(relative)
			if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, identities[i]) {
				return fmt.Errorf("acceptance parent identity changed: %s", relative)
			}
		}
		return nil
	}
	return held, parts[len(parts)-1], check, nil
}

func (d *operationCandidateDisk) read(name string) (operationCandidateSnapshot, error) {
	absent := operationCandidateSnapshot{Data: []byte{}, SHA256: store.SHA256HexString("")}
	parent, leaf, check, err := d.parent(name, false)
	if errors.Is(err, fs.ErrNotExist) {
		return absent, nil
	}
	if err != nil {
		return absent, err
	}
	defer parent.Close()
	info, err := parent.Lstat(leaf)
	if errors.Is(err, fs.ErrNotExist) {
		return absent, check()
	}
	if err != nil {
		return absent, err
	}
	if !info.Mode().IsRegular() {
		return absent, fmt.Errorf("acceptance target is not a regular file: %s", name)
	}
	file, err := parent.OpenFile(leaf, operationCandidateReadFlags(), 0)
	if err != nil {
		return absent, err
	}
	defer file.Close()
	actual, err := file.Stat()
	if err != nil || !os.SameFile(info, actual) || info.Mode() != actual.Mode() || !actual.Mode().IsRegular() {
		return absent, fmt.Errorf("acceptance file identity changed: %s", name)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return absent, err
	}
	after, err := parent.Lstat(leaf)
	if err != nil || !os.SameFile(info, after) || info.Mode() != after.Mode() {
		return absent, fmt.Errorf("acceptance file changed while reading: %s", name)
	}
	if err := check(); err != nil {
		return absent, err
	}
	if data == nil {
		data = []byte{}
	}
	return operationCandidateSnapshot{Present: true, Mode: uint32(info.Mode()), Data: data, SHA256: store.SHA256HexBytes(data)}, nil
}

func (d *operationCandidateDisk) replace(name string, data []byte, mode fs.FileMode, directory string) (installed bool, retErr error) {
	parent, leaf, check, err := d.parent(name, true)
	if err != nil {
		return false, err
	}
	defer parent.Close()
	if info, err := parent.Lstat(leaf); err == nil {
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("acceptance refuses replacing a nonregular file: %s", name)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	temporaryPath := path.Join(directory, ".accept-write")
	writer, temporary, checkWriter, err := d.parent(temporaryPath, false)
	if err != nil {
		return false, err
	}
	defer writer.Close()
	file, err := writer.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false, err
	}
	identity, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return false, err
	}
	defer func() {
		_ = file.Close()
		if !installed {
			if current, err := writer.Lstat(temporary); err == nil && os.SameFile(identity, current) {
				retErr = errors.Join(retErr, writer.Remove(temporary))
			}
		}
	}()
	if _, err := file.Write(data); err != nil {
		return false, err
	}
	if err := file.Chmod(mode); err != nil {
		return false, err
	}
	if err := file.Sync(); err != nil {
		return false, err
	}
	if err := file.Close(); err != nil {
		return false, err
	}
	if err := check(); err != nil {
		return false, err
	}
	if err := checkWriter(); err != nil {
		return false, err
	}
	current, err := writer.Lstat(temporary)
	if err != nil || !os.SameFile(identity, current) {
		return false, fmt.Errorf("acceptance write file was replaced")
	}
	if info, err := parent.Lstat(leaf); err == nil && !info.Mode().IsRegular() {
		return false, fmt.Errorf("acceptance destination became nonregular")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := d.root.Rename(temporaryPath, name); err != nil {
		return false, err
	}
	return true, errors.Join(operationCandidateSyncDirectory(parent), operationCandidateSyncDirectory(writer))
}

func (d *operationCandidateDisk) missingParents(name string) ([]string, error) {
	var missing []string
	for directory := path.Dir(name); directory != "."; directory = path.Dir(directory) {
		info, err := d.root.Lstat(directory)
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, directory)
		} else if err != nil {
			return nil, err
		} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("candidate creation ancestor is not a directory")
		}
	}
	return missing, nil
}

func (d *operationCandidateDisk) removeFile(name string) error {
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
	if !info.Mode().IsRegular() {
		return fmt.Errorf("acceptance cleanup refuses a nonregular file: %s", name)
	}
	if err := check(); err != nil {
		return err
	}
	if err := parent.Remove(leaf); err != nil {
		return err
	}
	return operationCandidateSyncDirectory(parent)
}

package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type rocS5LifecycleFixture struct {
	store          *store.Store
	slug           string
	upstreamCommit string
	artifact       candidate.CandidateArtifact
	pointerPath    string
}

func newROCS5LifecycleFixture(t *testing.T) *rocS5LifecycleFixture {
	t.Helper()
	s := slice2Store(t)
	git(t, s.Root, "init", "-q")
	git(t, s.Root, "config", "user.email", "test@example.invalid")
	git(t, s.Root, "config", "user.name", "test")
	writeRepoFile(t, s, "tracked.txt", []byte("v1\n"))
	git(t, s.Root, "add", "tracked.txt")
	git(t, s.Root, "commit", "-qm", "base")
	upstream := strings.TrimSpace(git(t, s.Root, "rev-parse", "HEAD"))
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "Lifecycle fixture"})
	if err != nil {
		t.Fatal(err)
	}
	feature.State = store.StateBlocked
	feature.Notes = "preserve-notes"
	if err := s.SaveFeatureStatus(feature); err != nil {
		t.Fatal(err)
	}
	artifact := rocS5LifecycleArtifact(t, feature.Slug, "blocked", upstream, upstream, strings.Repeat("4", 64))
	pointerPath, err := candidate.CandidatePointerPath(feature.Slug, artifact.CandidateID)
	if err != nil {
		t.Fatal(err)
	}

	featureLockPath := filepath.Join(s.Root, ".tpatch", "locks", "reconcile", feature.Slug+".lock")
	globalLockPath := filepath.Join(s.Root, ".tpatch", "locks", "worktree-mutation.lock")
	featureLock, err := candidatelock.TryAcquire(featureLockPath)
	if err != nil {
		t.Fatal(err)
	}
	globalLock, err := candidatelock.TryAcquire(globalLockPath)
	if err != nil {
		_ = featureLock.Release()
		t.Fatal(err)
	}
	root, err := candidate.CreateCandidateRoot(s.Root, feature.Slug, artifact.CandidateID, upstream, featureLock, globalLock)
	if err != nil {
		_ = globalLock.Release()
		_ = featureLock.Release()
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := globalLock.Release(); err != nil {
		t.Fatal(err)
	}
	if err := featureLock.Release(); err != nil {
		t.Fatal(err)
	}

	artifactRaw, err := candidate.EncodeCandidateArtifact(artifact)
	if err != nil {
		t.Fatal(err)
	}
	stateRaw, err := candidate.EncodeCandidateState(candidate.CandidateState{
		SchemaVersion:    candidate.CandidateSchemaVersion,
		CandidateID:      artifact.CandidateID,
		Status:           candidate.CandidateStatusPending,
		AcceptValidation: candidate.CandidateAcceptValidationNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(s.Root, filepath.FromSlash(pointerPath))
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, candidate.CandidateMetadataName), artifactRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, candidate.CandidateStateName), stateRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, candidate.CandidatePatchName), []byte("diff --git a/a b/a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := s.LoadFeatureStatus(feature.Slug)
	if err != nil {
		t.Fatal(err)
	}
	status.Reconcile.OperationCandidate = &store.OperationCandidateSummary{
		CandidateID:        artifact.CandidateID,
		Path:               pointerPath,
		Status:             candidate.CandidateStatusPending,
		SourceFeatureState: artifact.SourceFeatureState,
	}
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	return &rocS5LifecycleFixture{
		store:          s,
		slug:           feature.Slug,
		upstreamCommit: upstream,
		artifact:       artifact,
		pointerPath:    pointerPath,
	}
}

func rocS5LifecycleArtifact(t *testing.T, slug, sourceState, upstream, reference, recipeHash string) candidate.CandidateArtifact {
	t.Helper()
	operations := []candidate.CandidateOperation{
		{
			Ordinal:         1,
			Kind:            "create-file",
			Path:            "new.txt",
			EffectOrdinal:   2,
			EffectSHA256:    strings.Repeat("b", 64),
			RunOrdinals:     []int{},
			InsertionSHA256: strings.Repeat("9", 64),
			InsertionSize:   4,
			Classification:  "applicable",
		},
	}
	effectMap := []candidate.CandidateEffectMapEntry{
		{
			EffectOrdinal:     1,
			EffectSHA256:      strings.Repeat("a", 64),
			Path:              "tracked.txt",
			Resolution:        "absorbed-present",
			OperationOrdinals: []int{},
			ReasonCodes:       []string{},
		},
		{
			EffectOrdinal:     2,
			EffectSHA256:      strings.Repeat("b", 64),
			Path:              "new.txt",
			Resolution:        "candidate-applicable",
			OperationOrdinals: []int{1},
			ReasonCodes:       []string{},
		},
	}
	candidateID, _, _, err := candidate.ComputeCandidateID(candidate.CandidateIdentityInput{
		SchemaVersion:      candidate.CandidateSchemaVersion,
		Feature:            slug,
		SourceFeatureState: sourceState,
		UpstreamCommit:     upstream,
		ReferenceCommit:    reference,
		Bindings: candidate.CandidateBindings{
			CaptureEventSHA256: strings.Repeat("1", 64),
			CoverageSHA256:     strings.Repeat("2", 64),
			PatchSHA256:        strings.Repeat("3", 64),
			RecipeSHA256:       recipeHash,
		},
		DerivationMode: candidate.CandidateDerivationModeSingle,
		Operations:     operations,
		EffectMap:      effectMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact := candidate.CandidateArtifact{
		SchemaVersion:      candidate.CandidateSchemaVersion,
		CandidateID:        candidateID,
		Feature:            slug,
		SourceFeatureState: sourceState,
		UpstreamCommit:     upstream,
		ReferenceCommit:    reference,
		DerivationMode:     candidate.CandidateDerivationModeSingle,
		Bindings: candidate.CandidateBindings{
			CaptureEventSHA256: strings.Repeat("1", 64),
			CoverageSHA256:     strings.Repeat("2", 64),
			PatchSHA256:        strings.Repeat("3", 64),
			RecipeSHA256:       recipeHash,
		},
		Derivation: candidate.CandidateDerivation{
			EffectCount:              2,
			AbsorbedPresentCount:     1,
			CandidateApplicableCount: 1,
			ConflictCount:            0,
			Operations:               operations,
			EffectMap:                effectMap,
		},
		Result: candidate.CandidateResult{
			CandidateDiffSHA256: strings.Repeat("d", 64),
			CandidateTreeSHA256: strings.Repeat("e", 64),
			Validated:           candidate.CandidateValidationStructural,
		},
	}
	if err := artifact.Validate(); err != nil {
		t.Fatal(err)
	}
	return artifact
}

func TestROCS5PreflightLockFirstNoInspection(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	lockPath := filepath.Join(f.store.Root, ".tpatch", "locks", "reconcile", f.slug+".lock")
	held, err := candidatelock.TryAcquire(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Release() }()
	called := false
	old := operationCandidateLifecycleInspectHook
	t.Cleanup(func() { operationCandidateLifecycleInspectHook = old })
	operationCandidateLifecycleInspectHook = func() { called = true }
	got, err := runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit:     f.artifact.UpstreamCommit,
		SourceFeatureState: f.artifact.SourceFeatureState,
		Bindings:           f.artifact.Bindings,
	})
	if err != nil || got.Reason != "candidate-lock-held" || called {
		t.Fatalf("lock-first preflight failed: result=%+v err=%v inspect=%v", got, err, called)
	}
}

func TestROCS5PreflightRecoveryBeforeStale(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	journal := filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateJournalName)
	if err := os.WriteFile(journal, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit:     strings.Repeat("f", 40),
		SourceFeatureState: "applied",
		Bindings: candidate.CandidateBindings{
			CaptureEventSHA256: strings.Repeat("1", 64),
			CoverageSHA256:     strings.Repeat("2", 64),
			PatchSHA256:        strings.Repeat("3", 64),
			RecipeSHA256:       strings.Repeat("9", 64),
		},
	})
	if err != nil || got.Reason != "recovery-required" {
		t.Fatalf("expected recovery-required: result=%+v err=%v", got, err)
	}
	rawState, err := os.ReadFile(filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateStateName))
	if err != nil {
		t.Fatal(err)
	}
	state, err := candidate.DecodeCandidateState(rawState)
	if err != nil || state.Status != candidate.CandidateStatusPending {
		t.Fatalf("recovery check mutated pending state: %+v err=%v", state, err)
	}
	if _, err := os.Stat(filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateWorktreeName)); err != nil {
		t.Fatalf("recovery check pruned worktree unexpectedly: %v", err)
	}
}

func TestROCS5PreflightStalesPendingCandidateAndPreservesStateNotes(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	got, err := runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit:     f.artifact.UpstreamCommit,
		SourceFeatureState: f.artifact.SourceFeatureState,
		Bindings: candidate.CandidateBindings{
			CaptureEventSHA256: f.artifact.Bindings.CaptureEventSHA256,
			CoverageSHA256:     f.artifact.Bindings.CoverageSHA256,
			PatchSHA256:        f.artifact.Bindings.PatchSHA256,
			RecipeSHA256:       strings.Repeat("9", 64),
		},
	})
	if err != nil || !got.DerivationEnabled || got.StaleCandidateID != f.artifact.CandidateID {
		t.Fatalf("staleness preflight failed: result=%+v err=%v", got, err)
	}
	rawState, err := os.ReadFile(filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateStateName))
	if err != nil {
		t.Fatal(err)
	}
	state, err := candidate.DecodeCandidateState(rawState)
	if err != nil || state.Status != candidate.CandidateStatusStale {
		t.Fatalf("candidate state not marked stale: %+v err=%v", state, err)
	}
	status, err := f.store.LoadFeatureStatus(f.slug)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != store.StateBlocked || status.Notes != "preserve-notes" {
		t.Fatalf("preflight changed feature lifecycle metadata: %+v", status)
	}
	if status.Reconcile.OperationCandidate != nil {
		t.Fatalf("stale preflight did not clear pointer: %+v", status.Reconcile.OperationCandidate)
	}
	if _, err := os.Stat(filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateWorktreeName)); !os.IsNotExist(err) {
		t.Fatalf("stale preflight failed to prune worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateMetadataName)); err != nil {
		t.Fatalf("stale preflight removed immutable candidate metadata: %v", err)
	}
}

func TestROCS5RejectAndRejectedLogLifecycle(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	rejectedID, err := rejectPendingOperationCandidate(f.store, f.slug)
	if err != nil || rejectedID != f.artifact.CandidateID {
		t.Fatalf("reject transition failed: id=%s err=%v", rejectedID, err)
	}
	status, err := f.store.LoadFeatureStatus(f.slug)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != store.StateBlocked || status.Notes != "preserve-notes" || status.Reconcile.OperationCandidate != nil {
		t.Fatalf("reject changed lifecycle metadata or pointer unexpectedly: %+v", status)
	}
	rawState, err := os.ReadFile(filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateStateName))
	if err != nil {
		t.Fatal(err)
	}
	state, err := candidate.DecodeCandidateState(rawState)
	if err != nil || state.Status != candidate.CandidateStatusRejected {
		t.Fatalf("reject did not persist state=rejected: %+v err=%v", state, err)
	}
	if _, err := os.Stat(filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateWorktreeName)); !os.IsNotExist(err) {
		t.Fatalf("reject failed to prune worktree: %v", err)
	}
	rows, err := readRejectedOperationCandidates(f.store.Root, f.slug)
	if err != nil || len(rows) != 1 || rows[0].CandidateID != f.artifact.CandidateID {
		t.Fatalf("rejected.jsonl metadata mismatch: rows=%+v err=%v", rows, err)
	}
	blob, err := os.ReadFile(filepath.Join(f.store.Root, ".tpatch", "reconcile-candidates", f.slug, candidate.CandidateRejectedName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(blob), "timestamp") || strings.Contains(string(blob), "body") {
		t.Fatalf("rejected metadata leaked mutable/body fields: %s", blob)
	}
	artifact2 := rocS5LifecycleArtifact(t, f.slug, "blocked", f.upstreamCommit, f.upstreamCommit, strings.Repeat("f", 64))
	same, err := isOperationCandidateRejected(f.store.Root, f.slug, f.artifact.CandidateID)
	if err != nil || !same {
		t.Fatalf("rejected id lookup failed for matching id: %t err=%v", same, err)
	}
	changed, err := isOperationCandidateRejected(f.store.Root, f.slug, artifact2.CandidateID)
	if err != nil || changed {
		t.Fatalf("changed bindings should mint non-rejected id: %t err=%v", changed, err)
	}
	if err := clearOperationCandidateRejectionsLocked(f.store, f.slug); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(f.store.Root, ".tpatch", "reconcile-candidates", f.slug, candidate.CandidateRejectedName))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("clear rejections did not truncate log: %q", after)
	}
}

func TestROCS5ReuseValidationAndTamperedArtifactRefusal(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	state := candidate.CandidateState{
		SchemaVersion:    candidate.CandidateSchemaVersion,
		CandidateID:      f.artifact.CandidateID,
		Status:           candidate.CandidateStatusPending,
		AcceptValidation: candidate.CandidateAcceptValidationNone,
	}
	calls := 0
	reuse, err := operationCandidateReuseCheck(f.artifact, state, f.artifact, func() error {
		calls++
		return nil
	})
	if err != nil || !reuse.Reusable || calls != 1 {
		t.Fatalf("reuse should require and run validation callback once: reuse=%+v calls=%d err=%v", reuse, calls, err)
	}
	collision := f.artifact
	collision.Bindings.RecipeSHA256 = strings.Repeat("f", 64)
	reuse, err = operationCandidateReuseCheck(f.artifact, state, collision, func() error {
		calls++
		return nil
	})
	if err != nil || reuse.RefusalCode != "candidate-id-collision" || calls != 1 {
		t.Fatalf("collision should refuse before validation callback: reuse=%+v calls=%d err=%v", reuse, calls, err)
	}
	path := filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateMetadataName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	node["unknown"] = true
	bad, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit:     f.artifact.UpstreamCommit,
		SourceFeatureState: f.artifact.SourceFeatureState,
		Bindings:           f.artifact.Bindings,
	}); err == nil {
		t.Fatal("tampered candidate metadata unexpectedly granted authority")
	}
}

func TestROCS5CoexistenceRefusals(t *testing.T) {
	status := store.FeatureStatus{}
	status.Reconcile.OperationCandidate = &store.OperationCandidateSummary{Status: candidate.CandidateStatusPending}
	if code := operationCandidateCoexistenceRefusal(status, "resolver-action"); code != "candidate-pending" {
		t.Fatalf("resolver coexistence refusal mismatch: %q", code)
	}
	status.Reconcile.ShadowPath = "shadow"
	if code := operationCandidateCoexistenceRefusal(status, "candidate-action"); code != "shadow-pending" {
		t.Fatalf("candidate coexistence refusal mismatch: %q", code)
	}
}

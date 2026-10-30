package workflow

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
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

func TestROCS5RecoveryScanIgnoresUnrelatedWorktreeSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture is not portable on windows")
	}
	f := newROCS5LifecycleFixture(t)
	outside := filepath.Join(f.store.Root, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateWorktreeName)
	if err := os.Symlink(outside, filepath.Join(worktree, "linked-outside.txt")); err != nil {
		t.Fatal(err)
	}
	got, err := runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit:     f.artifact.UpstreamCommit,
		SourceFeatureState: f.artifact.SourceFeatureState,
		Bindings:           f.artifact.Bindings,
	})
	if err != nil || !got.DerivationEnabled || got.Reason != "" {
		t.Fatalf("worktree symlink should not block metadata recovery scan: result=%+v err=%v", got, err)
	}
	journal := filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateJournalName)
	if err := os.WriteFile(journal, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit:     f.artifact.UpstreamCommit,
		SourceFeatureState: f.artifact.SourceFeatureState,
		Bindings:           f.artifact.Bindings,
	})
	if err != nil || got.Reason != "recovery-required" {
		t.Fatalf("metadata journal must still block derivation: result=%+v err=%v", got, err)
	}
}

func TestROCS5PreflightStalesPendingCandidateAndPreservesStateNotes(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	writeRepoFile(t, f.store, "tracked.txt", []byte("v2\n"))
	git(t, f.store.Root, "add", "tracked.txt")
	git(t, f.store.Root, "commit", "-qm", "upstream-advanced")
	liveHead := strings.TrimSpace(git(t, f.store.Root, "rev-parse", "HEAD"))
	if liveHead == f.artifact.UpstreamCommit {
		t.Fatal("fixture failed to advance live HEAD")
	}
	got, err := runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit:     liveHead,
		SourceFeatureState: f.artifact.SourceFeatureState,
		Bindings:           f.artifact.Bindings,
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
	writeRepoFile(t, f.store, "tracked.txt", []byte("v2\n"))
	git(t, f.store.Root, "add", "tracked.txt")
	git(t, f.store.Root, "commit", "-qm", "live-head-advanced")
	liveHead := strings.TrimSpace(git(t, f.store.Root, "rev-parse", "HEAD"))
	if liveHead == f.artifact.UpstreamCommit {
		t.Fatal("fixture failed to advance live HEAD")
	}
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
	reference, err := openOperationCandidateReference(f.store.Root, f.upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reference.Close() }()
	rows, err := readRejectedOperationCandidates(reference, f.slug)
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
	same, err := isOperationCandidateRejected(reference, f.slug, f.artifact.CandidateID)
	if err != nil || !same {
		t.Fatalf("rejected id lookup failed for matching id: %t err=%v", same, err)
	}
	changed, err := isOperationCandidateRejected(reference, f.slug, artifact2.CandidateID)
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
	f := rocS2NewFixture(t, true)
	prepared, err := prepareOperationCandidate(f.s, f.slug, f.invocation)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := finalizeOperationCandidate(f.s, f.slug, prepared)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := finalizeOperationCandidate(f.s, f.slug, prepared)
	if err != nil || reused.CandidateID != artifact.CandidateID {
		t.Fatalf("real validated reuse failed: %+v %v", reused, err)
	}
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := openOperationCandidateReference(f.s.Root, f.invocation.upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	_, _, existingRoot, existingReference, err := readOperationCandidateArtifacts(f.s.Root, reference, f.slug, status.Reconcile.OperationCandidate)
	if err != nil {
		t.Fatal(err)
	}
	defer existingRoot.Close()
	defer existingReference.Close()
	reuseLock, err := candidatelock.TryAcquire(filepath.Join(f.s.Root, ".tpatch", "locks", "reconcile", f.slug+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer reuseLock.Release()
	path := filepath.Join(f.s.Root, filepath.FromSlash(status.Reconcile.OperationCandidate.Path), candidate.CandidateMetadataName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	resultNode, ok := node["result"].(map[string]any)
	if !ok {
		t.Fatal("candidate metadata fixture missing result object")
	}
	resultNode["validated"] = "tests"
	bad, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := operationCandidateReuseCheck(prepared, existingRoot, reuseLock); err == nil {
		t.Fatal("tampered artifact granted reuse")
	}
	if err := os.RemoveAll(existingRoot.WorktreePath()); err != nil {
		t.Fatal(err)
	}
	if _, err := operationCandidateReuseCheck(prepared, existingRoot, reuseLock); err == nil {
		t.Fatal("reuse unexpectedly accepted missing candidate worktree")
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

func TestROCS5ResumePreparationUsesRealEligibility(t *testing.T) {
	for _, kind := range []string{"disabled", "forged-event", "forged-descriptor"} {
		t.Run(kind, func(t *testing.T) {
			f := rocS2NewFixture(t, true)
			switch kind {
			case "disabled":
				cfg, _ := f.s.LoadConfig()
				cfg.OperationCandidateEnabled = false
				if err := f.s.SaveConfig(cfg); err != nil {
					t.Fatal(err)
				}
			case "forged-event":
				f.event.CoverageSHA256 = strings.Repeat("f", 64)
				raw, _ := json.Marshal(f.event)
				f.write(t, "recipe-capture-event.json", raw)
			case "forged-descriptor":
				f.coverage.Effects[0].EffectSHA256 = strings.Repeat("f", 64)
				f.save(t)
			}
			if _, err := prepareOperationCandidate(f.s, f.slug, f.invocation); err == nil {
				t.Fatal("unproved eligibility granted preparation")
			}
			if _, err := os.Stat(filepath.Join(f.s.Root, ".tpatch", "reconcile-candidates", f.slug)); !os.IsNotExist(err) {
				t.Fatalf("refused preparation created candidate namespace: %v", err)
			}
		})
	}
}

func TestROCS5ResumeFinalizationRechecksStateBindingsAndRejection(t *testing.T) {
	for _, kind := range []string{"source-state", "recipe", "rejection", "recovery", "shadow", "other-feature"} {
		t.Run(kind, func(t *testing.T) {
			f := rocS2NewFixture(t, true)
			prepared, err := prepareOperationCandidate(f.s, f.slug, f.invocation)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := json.Marshal(prepared); err == nil {
				t.Fatal("preparation serialized")
			}
			slug := f.slug
			switch kind {
			case "source-state":
				f.changeStatus(t, func(s *store.FeatureStatus) { s.State = store.StateBlocked })
			case "recipe":
				f.recipe = append(f.recipe, '\n')
				f.rebindRecipe()
				f.save(t)
			case "rejection":
				lock, _, err := acquireOperationCandidateFeatureLock(f.s.Root, f.slug)
				if err != nil {
					t.Fatal(err)
				}
				ref, err := openOperationCandidateReference(f.s.Root, prepared.eligibility.upstreamCommit)
				if err != nil {
					t.Fatal(err)
				}
				b := operationCandidateBindings(prepared.eligibility)
				err = ref.AppendRejectedRecord(f.slug, candidate.CandidateRejectedRecord{
					CandidateID: prepared.candidateID, SourceFeatureState: string(prepared.eligibility.sourceFeatureState),
					UpstreamCommit: prepared.eligibility.upstreamCommit, ReferenceCommit: prepared.eligibility.referenceCommit,
					CaptureEventSHA256: b.CaptureEventSHA256, CoverageSHA256: b.CoverageSHA256,
					PatchSHA256: b.PatchSHA256, RecipeSHA256: b.RecipeSHA256,
				}, lock)
				_ = ref.Close()
				_ = lock.Release()
				if err != nil {
					t.Fatal(err)
				}
			case "recovery":
				dir := filepath.Join(f.s.Root, ".tpatch", "reconcile-candidates", f.slug, prepared.candidateID)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, candidate.CandidateJournalName), []byte("{}"), 0644); err != nil {
					t.Fatal(err)
				}
			case "shadow":
				f.changeStatus(t, func(s *store.FeatureStatus) { s.Reconcile.ShadowPath = "shadow" })
			case "other-feature":
				slug = "other-feature"
			}
			if _, err := finalizeOperationCandidate(f.s, slug, prepared); err == nil {
				t.Fatal("stale or unrelated preparation published")
			}
			tree := filepath.Join(f.s.Root, ".tpatch", "reconcile-candidates", slug, prepared.candidateID, "worktree")
			if _, err := os.Stat(tree); !os.IsNotExist(err) {
				t.Fatalf("refusal created a worktree: %v", err)
			}
		})
	}
}

func TestROCS5ResumeRejectGlobalContentionPreservesPendingPair(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	global, err := candidatelock.TryAcquire(filepath.Join(f.store.Root, ".tpatch", "locks", "worktree-mutation.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rejectPendingOperationCandidate(f.store, f.slug); err == nil {
		t.Fatal("contended prune succeeded")
	}
	ref, err := openOperationCandidateReference(f.store.Root, f.upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer ref.Close()
	_, state, err := ref.ReadCandidateMetadata(f.slug, f.artifact.CandidateID)
	if err != nil || state.Status != candidate.CandidateStatusPending {
		t.Fatalf("contention changed terminal state: %+v %v", state, err)
	}
	status, err := f.store.LoadFeatureStatus(f.slug)
	if err != nil || status.Reconcile.OperationCandidate == nil || status.Reconcile.OperationCandidate.Status != candidate.CandidateStatusPending {
		t.Fatalf("contention changed pointer: %+v %v", status, err)
	}
	if err := global.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := rejectPendingOperationCandidate(f.store, f.slug); err != nil {
		t.Fatal(err)
	}
	status, _ = f.store.LoadFeatureStatus(f.slug)
	if status.Reconcile.OperationCandidate != nil {
		t.Fatal("rejection retry left pointer")
	}
}

func TestROCS5ResumeRejectStatusFailureCanFinishUnderNextHold(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires enforced Unix directory permissions")
	}
	f := newROCS5LifecycleFixture(t)
	dir := filepath.Join(f.store.Root, ".tpatch", "features", f.slug)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	if _, err := rejectPendingOperationCandidate(f.store, f.slug); err == nil {
		t.Fatal("status failure was swallowed")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit: f.upstreamCommit, SourceFeatureState: "blocked", Bindings: f.artifact.Bindings,
	})
	if err != nil || !got.DerivationEnabled {
		t.Fatalf("failed rejection did not finish safely: %+v %v", got, err)
	}
	status, _ := f.store.LoadFeatureStatus(f.slug)
	if status.Reconcile.OperationCandidate != nil {
		t.Fatal("incomplete rejection still has a pointer")
	}
	if _, err := os.Stat(filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), "worktree")); !os.IsNotExist(err) {
		t.Fatalf("old worktree retained after retry: %v", err)
	}
}

func TestROCS5ResumeOrphanPendingNeverEnablesDerivation(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	status, _ := f.store.LoadFeatureStatus(f.slug)
	status.Reconcile.OperationCandidate = nil
	if err := f.store.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	got, err := runOperationCandidatePreflight(f.store, f.slug, candidate.CandidateStalenessInput{
		UpstreamCommit: f.upstreamCommit, SourceFeatureState: "blocked", Bindings: f.artifact.Bindings,
	})
	if err == nil || got.DerivationEnabled {
		t.Fatalf("orphan pending was invisible: %+v %v", got, err)
	}
}

func TestROCS5ResumePublicationFailureDoesNotLeavePendingOrphan(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires enforced Unix directory permissions")
	}
	f := rocS2NewFixture(t, true)
	prepared, err := prepareOperationCandidate(f.s, f.slug, f.invocation)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.s.Root, ".tpatch", "features", f.slug)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	if _, err := finalizeOperationCandidate(f.s, f.slug, prepared); err == nil {
		t.Fatal("pointer write failure was swallowed")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	status, _ := f.s.LoadFeatureStatus(f.slug)
	if status.Reconcile.OperationCandidate != nil {
		t.Fatal("failed publication left pointer")
	}
	ref, err := openOperationCandidateReference(f.s.Root, f.invocation.upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer ref.Close()
	_, state, err := ref.ReadCandidateMetadata(f.slug, prepared.candidateID)
	if err != nil || state.Status != candidate.CandidateStatusStale {
		t.Fatalf("failed publication left pending metadata: %+v %v", state, err)
	}
	present, err := ref.CandidateWorktreePresent(f.slug, prepared.candidateID)
	if err != nil || present {
		t.Fatalf("failed publication retained worktree: %t %v", present, err)
	}
	// A fully validated retry can reuse the retained immutable audit.
	if _, err := finalizeOperationCandidate(f.s, f.slug, prepared); err != nil {
		t.Fatal(err)
	}
}

func reconstructLifecycleEffects(t *testing.T, repoRoot, baseCommit, patch string) []gitutil.PatchEffect {
	t.Helper()
	rows, err := patchobs.ReconstructOffline(repoRoot, baseCommit, patch)
	if err != nil {
		t.Fatal(err)
	}
	effects := make([]gitutil.PatchEffect, len(rows))
	for i, row := range rows {
		e := row.Effect
		e.PreimageObserved, e.PreimagePresent, e.PreimageSHA256 = true, row.Pre.Present, row.Pre.SHA256
		e.PostimageObserved, e.PostimagePresent, e.PostimageSHA256 = true, row.Post.Present, row.Post.SHA256
		e.OldMode, e.NewMode = row.Pre.Mode, row.Post.Mode
		entry := patchobs.EffectObservation{Effect: e}
		entry.Bytes.Preimage, entry.Bytes.Postimage = row.Pre.Bytes, row.Post.Bytes
		e.ContentKind, e.ObjectKind = patchobs.ClassifyEffectObservation(e, entry.Bytes)
		effects[i] = e
	}
	return effects
}

func lifecycleGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
	return string(out)
}

func acquireFeatureAndGlobalLocks(t *testing.T, repoRoot, slug string) (*candidatelock.Handle, *candidatelock.Handle) {
	t.Helper()
	featureLockPath := filepath.Join(repoRoot, ".tpatch", "locks", "reconcile", slug+".lock")
	globalLockPath := filepath.Join(repoRoot, ".tpatch", "locks", "worktree-mutation.lock")
	featureLock, err := candidatelock.TryAcquire(featureLockPath)
	if err != nil {
		t.Fatal(err)
	}
	globalLock, err := candidatelock.TryAcquire(globalLockPath)
	if err != nil {
		_ = featureLock.Release()
		t.Fatal(err)
	}
	return featureLock, globalLock
}

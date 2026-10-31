package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const rocS6Base = "head\nremove\nend\n"
const rocS6Feature = "head\nremove\nfeature\nend\n"
const rocS6UpstreamDeletion = "head\nend\n"

func rocS6PhaseFixture(t *testing.T, upstream string) *rocS2Fixture {
	t.Helper()
	s := slice2Store(t)
	git(t, s.Root, "init", "-q")
	git(t, s.Root, "config", "user.email", "test@example.invalid")
	git(t, s.Root, "config", "user.name", "test")
	git(t, s.Root, "config", "commit.gpgsign", "false")
	writeRepoFile(t, s, "a.txt", []byte(rocS6Base))
	git(t, s.Root, "add", "a.txt")
	git(t, s.Root, "commit", "-qm", "base")
	base := strings.TrimSpace(gitOutput(t, s.Root, "rev-parse", "HEAD"))
	status, err := s.AddFeature(store.AddFeatureInput{Title: "S6 phase fixture"})
	if err != nil {
		t.Fatal(err)
	}
	status.State, status.Notes = store.StateBlocked, "keep-s6-notes"
	status.Apply.BaseCommit = base
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.OperationCandidateEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, s, "a.txt", []byte(rocS6Feature))
	patch := gitOutput(t, s.Root, "diff", "--binary", base, "--", "a.txt")
	obs := rgaS3Observe(t, patch, rgaS3Image{pre: rocS6Base, post: rocS6Feature})
	obs.RepoRoot, obs.Slug, obs.Reference.Commit = s.Root, status.Slug, base
	inputs := rgaS3Inputs(t, obs, rgaS3Write("a.txt", rocS6Base, rocS6Feature, false))
	inputs.Events.RecipeRegenerated = true
	event, raw, bindings := rgaS5EventPair(t, inputs)
	coverage, err := DecodeRecipeCoverage(raw)
	if err != nil || coverage.CoverageStatus != CoverageComplete {
		t.Fatalf("phase fixture coverage: %v", err)
	}
	writeRepoFile(t, s, "a.txt", []byte(upstream))
	git(t, s.Root, "add", "a.txt")
	git(t, s.Root, "commit", "-qm", "upstream", "--allow-empty")
	commit := strings.TrimSpace(gitOutput(t, s.Root, "rev-parse", "HEAD"))
	f := &rocS2Fixture{s: s, slug: status.Slug, coverage: coverage, event: event,
		patch: bindings.Patch.Bytes, recipe: bindings.Recipe.Bytes,
		invocation: operationCandidateInvocation{slugs: []string{status.Slug}, upstreamCommit: commit}}
	f.save(t)
	return f
}

func rocS6Run(t *testing.T, f *rocS2Fixture, prov provider.Provider, cfg provider.Config, opts ReconcileOptions) ReconcileResult {
	t.Helper()
	results, err := RunReconcile(context.Background(), f.s, []string{f.slug}, "HEAD", prov, cfg, opts)
	if err != nil || len(results) != 1 {
		t.Fatalf("reconcile results=%v err=%v", results, err)
	}
	return results[0]
}

func TestROCS6AllPresentUsesImmutableUpstreamAndFreshAuthority(t *testing.T) {
	f := rocS6PhaseFixture(t, rocS6Feature)
	if err := os.Remove(filepath.Join(f.s.Root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	result := rocS6Run(t, f, nil, provider.Config{}, ReconcileOptions{})
	if result.Outcome != store.ReconcileUpstreamed || result.Phase != "phase-2-operation-level" || result.ReviewVerdict != "confirmed-upstreamed" {
		t.Fatalf("all-present result: %+v", result)
	}
	if len(result.Evidence) == 0 || !qualifyingOperationAttempt(result.Evidence[0], &result) {
		t.Fatalf("qualifying phase2 evidence absent: %+v", result.Evidence)
	}
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil || status.State != store.StateUpstreamMerged || status.Reconcile.OperationCandidate != nil {
		t.Fatalf("all-present status=%+v err=%v", status, err)
	}
	if _, err := os.Stat(filepath.Join(f.s.Root, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("all-present classification changed the live worktree")
	}
}

func TestROCS6CandidateReadyPreservesStateAndSubstitutesEvidence(t *testing.T) {
	f := rocS6PhaseFixture(t, rocS6UpstreamDeletion)
	result := rocS6Run(t, f, nil, provider.Config{}, ReconcileOptions{})
	if result.Outcome != store.ReconcileStillNeeded || result.Phase != "phase-2-operation-candidate" {
		t.Fatalf("expected actual textual-conflict candidate arm: %+v", result)
	}
	if len(result.Evidence) < 2 || result.Evidence[0].Phase != store.EvidencePhase2 || result.Evidence[1].Phase != store.EvidencePhase4 ||
		result.Evidence[0].RawReconcileVerdict != string(store.ReconcileStillNeeded) ||
		result.Evidence[1].RawReconcileVerdict != string(store.ReconcileBlocked) ||
		result.Evidence[0].AttemptID == result.Evidence[1].AttemptID {
		t.Fatalf("candidate evidence pair: %+v", result.Evidence)
	}
	for _, entry := range result.Evidence[2:] {
		if entry.EvidenceKind == store.EvidenceKindRecipeOperationMatch || entry.EvidenceKind == store.EvidenceKindForwardApply ||
			entry.EvidenceKind == store.EvidenceKindPathRestructure || entry.EvidenceKind == store.EvidenceKindBlockedClassification ||
			entry.EvidenceKind == store.EvidenceKindManualReview {
			t.Fatalf("duplicated verdict or changed supporting firing condition: %+v", entry)
		}
	}
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil || status.State != store.StateBlocked || status.Notes != "keep-s6-notes" || status.Reconcile.OperationCandidate == nil {
		t.Fatalf("candidate status=%+v err=%v", status, err)
	}
	path := filepath.Join(f.s.Root, filepath.FromSlash(status.Reconcile.OperationCandidate.Path), "worktree", "a.txt")
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "head\nfeature\nend\n" {
		t.Fatalf("candidate contents=%q err=%v", body, err)
	}
	live, err := os.ReadFile(filepath.Join(f.s.Root, "a.txt"))
	if err != nil || string(live) != rocS6UpstreamDeletion {
		t.Fatalf("candidate generation changed live content: %q %v", live, err)
	}
}

type rocS6Provider struct {
	calls int
	probe func()
}

func (p *rocS6Provider) Check(context.Context, provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *rocS6Provider) Generate(context.Context, provider.Config, provider.GenerateRequest) (string, error) {
	p.calls++
	if p.probe != nil {
		p.probe()
	}
	return `{"decision":"upstreamed","reasoning":"fixture"}`, nil
}

func TestROCS6StrongerArmsDiscardCandidatePlan(t *testing.T) {
	for _, name := range []string{"strict", "live-markers", "provider", "resolve"} {
		t.Run(name, func(t *testing.T) {
			upstream := rocS6UpstreamDeletion
			if name == "strict" || name == "live-markers" {
				upstream = rocS6Base
			}
			f := rocS6PhaseFixture(t, upstream)
			var prov provider.Provider
			cfg := provider.Config{}
			opts := ReconcileOptions{Resolve: name == "resolve"}
			var scripted *rocS6Provider
			if name == "provider" {
				scripted = &rocS6Provider{probe: func() {
					lock, err := candidatelock.TryAcquire(filepath.Join(f.s.Root, ".tpatch", "locks", "reconcile", f.slug+".lock"))
					if err != nil {
						t.Errorf("feature lock held during provider call: %v", err)
						return
					}
					_ = lock.Release()
				}}
				prov, cfg = scripted, provider.Config{BaseURL: "http://unused", Model: "fixture"}
			}
			if name == "live-markers" {
				writeRepoFile(t, f.s, "leftover.txt", []byte("<<<<<<< HEAD\nleft\n=======\nright\n>>>>>>> other\n"))
			}
			result := rocS6Run(t, f, prov, cfg, opts)
			status, err := f.s.LoadFeatureStatus(f.slug)
			if err != nil || status.Reconcile.OperationCandidate != nil || result.Phase == "phase-2-operation-candidate" {
				t.Fatalf("stronger arm persisted a candidate: %+v %+v %v", result, status, err)
			}
			if scripted != nil && scripted.calls != 1 {
				t.Fatalf("phase3 calls=%d", scripted.calls)
			}
			if name == "strict" && result.Outcome != store.ReconcileReapplied {
				t.Fatalf("strict arm changed: %+v", result)
			}
			if name == "live-markers" && result.Phase != "phase-4-live-conflict-markers" {
				t.Fatalf("marker promotion changed: %+v", result)
			}
		})
	}
}

func TestROCS6GateOnLegacyFallsThroughWithoutAuthority(t *testing.T) {
	f := rocS0Fixture(t, "phase2-legacy-rejected")
	cfg, err := f.store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.OperationCandidateEnabled = true
	if err := f.store.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	results, err := RunReconcile(context.Background(), f.store, f.slugs, "HEAD", f.provider, f.config, f.options)
	if err != nil || len(results) != 1 || results[0].Phase == "phase-2-operation-level" {
		t.Fatalf("legacy evaluator retained authority: %+v %v", results, err)
	}
}

func rocS6TestProof(s *store.Store, result *ReconcileResult, snapshot *operationPhaseSnapshot) *phase2AuthorityProof {
	return &phase2AuthorityProof{
		owner: s, result: result, repoPath: snapshot.repoPath, repoIdentity: snapshot.repoIdentity,
		upstreamCommit: snapshot.eligibility.upstreamCommit, referenceCommit: snapshot.eligibility.referenceCommit,
		captureEventSHA256: snapshot.eligibility.captureEventSHA256, coverageSHA256: snapshot.eligibility.coverageSHA256,
		patchSHA256: snapshot.eligibility.patchSHA256, recipeSHA256: snapshot.eligibility.recipeSHA256,
		sourceFeatureState: string(snapshot.eligibility.sourceFeatureState), gatesPassed: snapshot.eligibility.gatesPassed,
		effectCount: snapshot.effectCount, effectDigest: snapshot.effectDigest, presentCount: snapshot.presentCount,
		applicableCount: snapshot.applicableCount, conflictCount: snapshot.conflictCount, operationCount: snapshot.operationCount,
	}
}

func TestROCS6ConfirmationRejectsAlteredProofAndEntry(t *testing.T) {
	f := rocS6PhaseFixture(t, rocS6Feature)
	snapshot, err := captureOperationPhaseSnapshot(f.s, f.slug, f.invocation)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*phase2AuthorityProof, *ReconcileResult){
		"other-result": func(p *phase2AuthorityProof, r *ReconcileResult) { copy := *r; p.result = &copy },
		"other-store": func(p *phase2AuthorityProof, _ *ReconcileResult) { copy := *f.s; p.owner = &copy },
		"binding": func(p *phase2AuthorityProof, _ *ReconcileResult) { p.captureEventSHA256 = strings.Repeat("0", 64) },
		"gate": func(p *phase2AuthorityProof, _ *ReconcileResult) { p.gatesPassed[3] = false },
		"effect-digest": func(p *phase2AuthorityProof, _ *ReconcileResult) { p.effectDigest = strings.Repeat("0", 64) },
		"operations": func(p *phase2AuthorityProof, _ *ReconcileResult) { p.operationCount = 1 },
		"present-count": func(p *phase2AuthorityProof, _ *ReconcileResult) { p.presentCount = 0 },
		"source-state": func(p *phase2AuthorityProof, _ *ReconcileResult) { p.sourceFeatureState = "applied" },
		"entry-origin": func(_ *phase2AuthorityProof, r *ReconcileResult) { r.Evidence[0].MatchOrigin = store.EvidenceMatchOriginFork },
		"entry-presence": func(_ *phase2AuthorityProof, r *ReconcileResult) { r.Evidence[0].PreReconcilePresence = store.EvidencePresenceUnknown },
		"extra-ref": func(_ *phase2AuthorityProof, r *ReconcileResult) { r.Evidence[0].UpstreamCommitRefs = append(r.Evidence[0].UpstreamCommitRefs, f.invocation.upstreamCommit) },
		"duplicate-entry": func(_ *phase2AuthorityProof, r *ReconcileResult) { r.Evidence = append(r.Evidence, r.Evidence[0]) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			result := &ReconcileResult{Slug: f.slug, UpstreamRef: "HEAD", UpstreamCommit: f.invocation.upstreamCommit,
				Outcome: store.ReconcileUpstreamed, Phase: "phase-2-operation-level"}
			result.Evidence = []store.ReconcileEvidence{qualifyingOperationPhaseEvidence(f.s, f.slug, result)}
			proof := rocS6TestProof(f.s, result, snapshot)
			mutate(proof, result)
			applyUpstreamedConfirmationGate(f.s, f.slug, result, proof)
			if result.Outcome != store.ReconcileBlocked || result.ReviewVerdict != "rejected-upstreamed" {
				t.Fatalf("altered authority confirmed: %+v", result)
			}
		})
	}
}

func TestROCS6MissingAuthorityPublicationNeverRetires(t *testing.T) {
	f := rocS6PhaseFixture(t, rocS6Feature)
	if err := os.Remove(filepath.Join(f.s.Root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.s.ReconcileEvidencePath(f.slug), 0755); err != nil {
		t.Fatal(err)
	}
	result := rocS6Run(t, f, nil, provider.Config{}, ReconcileOptions{})
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil || status.State == store.StateUpstreamMerged || result.Outcome != store.ReconcileBlocked {
		t.Fatalf("unpublished authority retired feature: %+v %+v %v", result, status, err)
	}
}

func TestROCS6RecoveryRefusesBeforePipelineMutation(t *testing.T) {
	f := newROCS5LifecycleFixture(t)
	journal := filepath.Join(f.store.Root, filepath.FromSlash(f.pointerPath), candidate.CandidateJournalName)
	if err := os.WriteFile(journal, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := f.store.LoadFeatureStatus(f.slug)
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunReconcile(context.Background(), f.store, []string{f.slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if !errors.Is(err, ErrOperationCandidateRecoveryRequired) {
		t.Fatalf("recovery did not refuse whole command: %v", err)
	}
	after, err := f.store.LoadFeatureStatus(f.slug)
	if err != nil || after.State != before.State || after.Notes != before.Notes || after.Reconcile.OperationCandidate == nil {
		t.Fatalf("recovery refusal mutated lifecycle: %+v %v", after, err)
	}
}

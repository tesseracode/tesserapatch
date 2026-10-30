package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type rocS2Fixture struct {
	s             *store.Store
	slug          string
	invocation    operationCandidateInvocation
	coverage      RecipeCoverage
	event         RecipeCaptureEvent
	patch, recipe []byte
}

func rocS2NewFixture(t *testing.T, creation bool) *rocS2Fixture {
	t.Helper()
	s := slice2Store(t)
	git(t, s.Root, "init", "-q")
	git(t, s.Root, "config", "user.email", "test@example.invalid")
	git(t, s.Root, "config", "user.name", "test")
	git(t, s.Root, "config", "commit.gpgsign", "false")
	if !creation {
		writeRepoFile(t, s, "a.txt", []byte("old\n"))
		git(t, s.Root, "add", "a.txt")
	}
	git(t, s.Root, "commit", "-qm", "base", "--allow-empty")
	base := gitOutput(t, s.Root, "rev-parse", "HEAD")
	status, err := s.AddFeature(store.AddFeatureInput{Title: "S2 eligibility"})
	if err != nil {
		t.Fatal(err)
	}
	status.State = store.StateApplied
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
	patch := rgaS3ModifyPatch
	pre := "old\n"
	if creation {
		patch, pre = rgaS3AddPatch, ""
	}
	obs := rgaS3Observe(t, patch, rgaS3Image{pre: pre, post: "new\n"})
	obs.RepoRoot, obs.Slug, obs.Reference.Commit = s.Root, status.Slug, strings.TrimSpace(base)
	in := rgaS3Inputs(t, obs, rgaS3Write("a.txt", pre, "new\n", creation))
	in.Events.RecipeRegenerated = true
	event, raw, bindings := rgaS5EventPair(t, in)
	coverage, err := DecodeRecipeCoverage(raw)
	if err != nil || coverage.CoverageStatus != CoverageComplete {
		t.Fatalf("fixture coverage: %v %+v", err, coverage)
	}
	f := &rocS2Fixture{s: s, slug: status.Slug, coverage: coverage, event: event,
		patch: bindings.Patch.Bytes, recipe: bindings.Recipe.Bytes,
		invocation: operationCandidateInvocation{slugs: []string{status.Slug}, upstreamCommit: strings.TrimSpace(base)}}
	f.save(t)
	return f
}

func (f *rocS2Fixture) write(t *testing.T, name string, raw []byte) {
	t.Helper()
	if err := f.s.WriteArtifact(f.slug, name, string(raw)); err != nil {
		t.Fatal(err)
	}
}

func (f *rocS2Fixture) save(t *testing.T) {
	t.Helper()
	raw, err := json.Marshal(f.coverage)
	if err != nil {
		t.Fatal(err)
	}
	f.event.CoverageSHA256 = CoverageSHA256(raw)
	f.write(t, "recipe-coverage.json", raw)
	raw, err = json.Marshal(f.event)
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, "recipe-capture-event.json", raw)
	f.write(t, "post-apply.patch", f.patch)
	f.write(t, "apply-recipe.json", f.recipe)
}

func (f *rocS2Fixture) rebindRecipe() {
	f.coverage.RecipeSHA256 = CoverageSHA256(f.recipe)
	f.event.RecipeSHA256 = f.coverage.RecipeSHA256
}

func (f *rocS2Fixture) changeRecipe(t *testing.T, mutate func(*ApplyRecipe)) {
	t.Helper()
	recipe, err := decodeCoverageRecipe(f.recipe)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&recipe)
	f.recipe, err = EncodeRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	f.rebindRecipe()
}

func (f *rocS2Fixture) changeStatus(t *testing.T, mutate func(*store.FeatureStatus)) {
	t.Helper()
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&status)
	if err := f.s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
}

func TestROCS2EligibilityReasonsAndFirstMatch(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		gate         int
		mutate       func(*testing.T, *rocS2Fixture)
		after        func(*testing.T, *rocS2Fixture)
	}{
		{name: "coverage-contradiction", gate: 1, reason: "coverage-unusable", mutate: func(t *testing.T, f *rocS2Fixture) { f.coverage.RecipeDecodable = false }},
		{name: "coverage-owner", gate: 2, reason: "coverage-owner-mismatch", mutate: func(t *testing.T, f *rocS2Fixture) { f.coverage.Feature = "other" }},
		{name: "incomplete-before-stale", gate: 3, reason: "coverage-incomplete", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.coverage.CoverageStatus, f.coverage.CrossBaseStatus = CoverageIncomplete, CrossBaseUnsupported
			f.coverage.Reasons = []string{"recipe-stale-marker-present"}
			f.write(t, "recipe-stale.json", []byte("{}"))
		}},
		{name: "event-decode-before-owner", gate: 4, reason: "capture-event-unusable", mutate: func(t *testing.T, f *rocS2Fixture) { f.event.SchemaVersion, f.event.Feature = 99, "other" }},
		{name: "event-absent", gate: 4, reason: "capture-event-unusable", after: func(t *testing.T, f *rocS2Fixture) {
			if err := os.Remove(filepath.Join(f.s.Root, ".tpatch", "features", f.slug, "artifacts", "recipe-capture-event.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "event-owner-before-pair", gate: 4, reason: "capture-event-owner-mismatch", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.event.Feature = "other"
			f.event.Capture.Mode = patchobs.CaptureModeStagedIndex
		}},
		{name: "event-descriptor", gate: 4, reason: "capture-event-pair-mismatch", mutate: func(t *testing.T, f *rocS2Fixture) { f.event.Capture.Mode = patchobs.CaptureModeStagedIndex }},
		{name: "event-presence-reverse", gate: 5, reason: "binding-presence-drift", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.event.RecipePresent, f.event.RecipeSHA256, f.event.Event.RecipeRegenerated = false, "", false
		}},
		{name: "missing-recipe-before-owner", gate: 5, reason: "binding-presence-drift", after: func(t *testing.T, f *rocS2Fixture) {
			if err := os.Remove(filepath.Join(f.s.Root, ".tpatch", "features", f.slug, "artifacts", "apply-recipe.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "hash-before-decode", gate: 6, reason: "binding-hash-drift", mutate: func(t *testing.T, f *rocS2Fixture) { f.recipe = []byte("invalid") }},
		{name: "raw-recipe-format", gate: 6, reason: "binding-hash-drift", mutate: func(t *testing.T, f *rocS2Fixture) { f.recipe = append(f.recipe, '\n') }},
		{name: "decode-before-owner", gate: 7, reason: "recipe-undecodable", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.recipe = []byte(`{"feature":"other","operations":null}`)
			f.rebindRecipe()
		}},
		{name: "recipe-owner", gate: 8, reason: "recipe-owner-mismatch", mutate: func(t *testing.T, f *rocS2Fixture) { f.changeRecipe(t, func(r *ApplyRecipe) { r.Feature = "other" }) }},
		{name: "reference-pair", gate: 9, reason: "reference-not-reconstructable", mutate: func(t *testing.T, f *rocS2Fixture) { f.event.Reference.Commit = strings.Repeat("f", 40) }},
		{name: "reference-missing-local", gate: 9, reason: "reference-not-reconstructable", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.event.Reference.Commit, f.coverage.Reference.Commit = strings.Repeat("f", 40), strings.Repeat("f", 40)
		}},
		{name: "upstream-missing-local", gate: 9, reason: "reference-not-reconstructable", mutate: func(t *testing.T, f *rocS2Fixture) { f.invocation.upstreamCommit = strings.Repeat("f", 40) }},
		{name: "fragment-drift", gate: 10, reason: "effect-binding-drift", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.coverage.Effects[0].PatchFragmentSHA256 = strings.Repeat("f", 64)
			f.event.Observations[0].PatchFragmentSHA256 = strings.Repeat("f", 64)
			var err error
			f.coverage.Effects[0].EffectSHA256, err = CoverageEffectSHA256(f.coverage.Effects[0])
			if err != nil {
				t.Fatal(err)
			}
		}},
		{name: "event-facts", gate: 10, reason: "effect-binding-drift", mutate: func(t *testing.T, f *rocS2Fixture) { f.event.Event.StaleMarkerPresent = true }},
		{name: "unsupported-witness", gate: 11, reason: "recipe-unsupported", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.changeRecipe(t, func(r *ApplyRecipe) { r.Operations[0].CreatedBy = "parent" })
		}},
		{name: "stale-before-unsupported", gate: 11, reason: "recipe-stale-marker-present", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.changeRecipe(t, func(r *ApplyRecipe) { r.Operations[0].CreatedBy = "parent" })
			f.write(t, "recipe-stale.json", []byte("{}"))
		}},
		{name: "superseded", gate: 12, reason: "supersession-not-authority", mutate: func(t *testing.T, f *rocS2Fixture) {
			peer, err := f.s.AddFeature(store.AddFeatureInput{Title: "replacement"})
			if err != nil {
				t.Fatal(err)
			}
			peer.State = store.StateApplied
			peer.DependsOn = []store.Dependency{{Slug: f.slug, Kind: store.DependencyKindSupersedes}}
			if err := f.s.SaveFeatureStatus(peer); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "parent-blocked", gate: 13, reason: "parent-blocked", mutate: func(t *testing.T, f *rocS2Fixture) {
			parent, err := f.s.AddFeature(store.AddFeatureInput{Title: "parent"})
			if err != nil {
				t.Fatal(err)
			}
			parent.State = store.StateBlocked
			if err := f.s.SaveFeatureStatus(parent); err != nil {
				t.Fatal(err)
			}
			f.changeStatus(t, func(s *store.FeatureStatus) {
				s.DependsOn = []store.Dependency{{Slug: parent.Slug, Kind: store.DependencyKindHard}}
			})
		}},
		{name: "multiple-slugs", gate: 14, reason: "derivation-mode-unsupported", mutate: func(t *testing.T, f *rocS2Fixture) { f.invocation.slugs = append(f.invocation.slugs, "other") }},
		{name: "cumulative", gate: 14, reason: "derivation-mode-unsupported", mutate: func(t *testing.T, f *rocS2Fixture) { f.invocation.cumulativeLegacy = true }},
		{name: "shadow", gate: 15, reason: "shadow-pending", mutate: func(t *testing.T, f *rocS2Fixture) {
			f.changeStatus(t, func(s *store.FeatureStatus) { s.Reconcile.ShadowPath = "pending-shadow" })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := rocS2NewFixture(t, false)
			if tc.mutate != nil {
				tc.mutate(t, f)
			}
			f.save(t)
			if tc.after != nil {
				tc.after(t, f)
			}
			got, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
			if err != nil || got.eligible || got.gate != tc.gate || got.reason != tc.reason {
				t.Fatalf("got %+v err=%v; want E%d/%s", got, err, tc.gate, tc.reason)
			}
			for i, passed := range got.gatesPassed {
				if passed != (i < tc.gate-1) {
					t.Fatalf("E%d pass flag=%v", i+1, passed)
				}
			}
		})
	}
}

func TestROCS2EligiblePairCanonicalIdentityAndNoLiveWrites(t *testing.T) {
	for _, creation := range []bool{false, true} {
		f := rocS2NewFixture(t, creation)
		f.coverage.Capture.Mode, f.event.Capture.Mode = patchobs.CaptureModeNoCapture, patchobs.CaptureModeNoCapture
		f.save(t)
		writeRepoFile(t, f.s, "a.txt", []byte("live tree is not evidence\n"))
		before := gitOutput(t, f.s.Root, "status", "--porcelain")
		first, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
		if err != nil || !first.eligible {
			t.Fatalf("valid pair refused: %+v %v", first, err)
		}
		for i, passed := range first.gatesPassed {
			if !passed {
				t.Fatalf("E%d not evaluated", i+1)
			}
		}
		raw, err := json.MarshalIndent(f.event, "", "\t")
		if err != nil {
			t.Fatal(err)
		}
		f.write(t, "recipe-capture-event.json", raw)
		second, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
		if err != nil || !second.eligible || first.captureEventSHA256 != second.captureEventSHA256 {
			t.Fatalf("formatting changed event identity: %+v %v", second, err)
		}
		f.event.Event.PatchRewritten = !f.event.Event.PatchRewritten
		f.save(t)
		third, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
		if err != nil || !third.eligible || first.captureEventSHA256 == third.captureEventSHA256 {
			t.Fatalf("semantic event change failed identity binding: %+v %v", third, err)
		}
		if gitOutput(t, f.s.Root, "status", "--porcelain") != before ||
			strings.TrimSpace(gitOutput(t, f.s.Root, "rev-parse", "HEAD")) != f.invocation.upstreamCommit {
			t.Fatal("eligibility changed live worktree, index or refs")
		}
		live, err := os.ReadFile(filepath.Join(f.s.Root, "a.txt"))
		if err != nil || string(live) != "live tree is not evidence\n" {
			t.Fatalf("eligibility rewrote live bytes: %v", err)
		}
		if _, err := os.Stat(filepath.Join(f.s.Root, ".tpatch", "reconcile-candidates")); !os.IsNotExist(err) {
			t.Fatalf("eligibility created candidate artifacts: %v", err)
		}
		if creation {
			f.event.ParentCreatedPaths = []string{"a.txt"}
			f.save(t)
			got, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
			if err != nil || got.reason != "recipe-unsupported" || got.gate != 11 {
				t.Fatalf("captured parent exclusion ignored: %+v %v", got, err)
			}
		}
	}
}

func TestROCS2PreflightAndUnreadableArtifacts(t *testing.T) {
	f := rocS2NewFixture(t, false)
	cfg, _ := f.s.LoadConfig()
	cfg.OperationCandidateEnabled = false
	if err := f.s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	old := ReadRecipeArtifact
	t.Cleanup(func() { ReadRecipeArtifact = old })
	ReadRecipeArtifact = func(string) RecipeArtifactRead {
		t.Fatal("preflight read bound artifacts")
		return RecipeArtifactRead{}
	}
	got, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
	if err != nil || got.enabled || got.eligible {
		t.Fatalf("disabled gate: %+v %v", got, err)
	}
	cfg.OperationCandidateEnabled = true
	if err := f.s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	for _, state := range []store.FeatureState{
		store.StateRequested, store.StateAnalyzed, store.StateDefined, store.StateImplementing,
		store.StateActive, store.StateReconciling, store.StateReconcilingShadow,
		store.StateUpstreamMerged, store.StateRejected, store.StateUnapplied,
	} {
		f.changeStatus(t, func(s *store.FeatureStatus) { s.State = state })
		got, err = assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
		if err != nil || got.reason != "candidate-feature-state-unsupported" || got.gate != 0 {
			t.Fatalf("state %s preflight: %+v %v", state, got, err)
		}
	}
	f.changeStatus(t, func(s *store.FeatureStatus) { s.State = store.StateBlocked })
	for _, tc := range []struct {
		file, reason string
		gate         int
	}{
		{"recipe-coverage.json", "coverage-unusable", 1},
		{"recipe-capture-event.json", "capture-event-unusable", 4},
		{"apply-recipe.json", "binding-presence-drift", 5},
		{"recipe-stale.json", "recipe-stale-marker-present", 11},
	} {
		ReadRecipeArtifact = func(path string) RecipeArtifactRead {
			if filepath.Base(path) == tc.file {
				return RecipeArtifactRead{Exists: true, Err: errors.New("fixture access denied")}
			}
			return old(path)
		}
		got, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
		if err != nil || got.reason != tc.reason || got.gate != tc.gate || !strings.Contains(got.detail, "fixture access denied") {
			t.Fatalf("unreadable %s: %+v %v", tc.file, got, err)
		}
	}
	ReadRecipeArtifact = old
	got, err = assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
	if err != nil || !got.eligible {
		t.Fatalf("blocked feature should qualify: %+v %v", got, err)
	}
}

func TestROCS2CapturePairSubcheckOrder(t *testing.T) {
	f := rocS2NewFixture(t, false)
	base := rgaS5CloneEvent(f.event)
	for first, field := range []string{"coverage_sha256", "capture.mode", "capture.pathspecs", "capture.claim_ids"} {
		f.event = rgaS5CloneEvent(base)
		if first == 0 {
			f.event.CoverageSHA256 = strings.Repeat("f", 64)
		}
		if first <= 1 {
			f.event.Capture.Mode = patchobs.CaptureModeStagedIndex
		}
		if first <= 2 {
			f.event.Capture.Pathspecs = []string{"a.txt"}
		}
		f.event.Capture.ClaimIDs = []string{"claim-a"}
		raw, err := EncodeRecipeCaptureEvent(f.event)
		if err != nil {
			t.Fatal(err)
		}
		f.write(t, "recipe-capture-event.json", raw)
		got, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
		if err != nil || got.reason != "capture-event-pair-mismatch" || got.detail != field+" differs" {
			t.Fatalf("first field %s: %+v %v", field, got, err)
		}
	}
}

func TestROCS2StrictDecoderReuseRejectsMalformedBindings(t *testing.T) {
	f := rocS2NewFixture(t, false)
	originalRecipe := bytes.Clone(f.recipe)
	for _, name := range []string{"recipe-coverage.json", "recipe-capture-event.json", "apply-recipe.json"} {
		for _, mutation := range []string{"unknown", "null", "duplicate"} {
			f.recipe = bytes.Clone(originalRecipe)
			f.rebindRecipe()
			f.save(t)
			path := filepath.Join(f.s.Root, ".tpatch", "features", f.slug, "artifacts", name)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "unknown":
				raw = bytes.Replace(raw, []byte("{"), []byte(`{"unknown":true,`), 1)
			case "duplicate":
				raw = bytes.Replace(raw, []byte("{"), []byte(`{"feature":"duplicate",`), 1)
			case "null":
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(raw, &fields); err != nil {
					t.Fatal(err)
				}
				fields["feature"] = json.RawMessage("null")
				raw, err = json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
			}
			if name == "apply-recipe.json" {
				f.recipe = raw
				f.rebindRecipe()
				f.save(t)
			}
			f.write(t, name, raw)
			got, err := assessOperationCandidateEligibility(f.s, f.slug, f.invocation)
			want := map[string]string{"recipe-coverage.json": "coverage-unusable", "recipe-capture-event.json": "capture-event-unusable", "apply-recipe.json": "recipe-undecodable"}[name]
			if err != nil || got.reason != want {
				t.Fatalf("%s malformed input: %+v %v", name, got, err)
			}
		}
	}
}

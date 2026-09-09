package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestRGAS4ImplementBothWriteArmsAndFailures(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, failure := range []string{"", "recipe", "provenance", "state", "coverage", "state+coverage"} {
			t.Run(strings.Join([]string{map[bool]string{false: "valid", true: "raw"}[raw], failure}, "/"), func(t *testing.T) {
				s, root, slug := s1ImplementFixture(t)
				rawBytes := "the provider returned undecodable bytes\n"
				if raw {
					old := implementRecipeContentHook
					implementRecipeContentHook = func(string) string { return rawBytes }
					t.Cleanup(func() { implementRecipeContentHook = old })
				}
				for _, item := range strings.Split(failure, "+") {
					path := ""
					switch item {
					case "recipe":
						path = filepath.Join(s.TpatchDir(), "features", slug, "artifacts", "apply-recipe.json")
					case "provenance":
						path = filepath.Join(s.TpatchDir(), "features", slug, "artifacts", "recipe-provenance.json")
					case "state":
						path = filepath.Join(s.TpatchDir(), "features", slug, "status.json")
						if err := os.Remove(path); err != nil {
							t.Fatal(err)
						}
					case "coverage":
						path = filepath.Join(s.TpatchDir(), "features", slug, "artifacts", "recipe-coverage.json")
					}
					if path != "" {
						if err := os.MkdirAll(path, 0o755); err != nil {
							t.Fatal(err)
						}
					}
				}
				err := RunImplement(context.Background(), s, slug, nil, provider.Config{})
				if (err != nil) != (failure != "") {
					t.Fatalf("failure=%s raw=%v err=%v", failure, raw, err)
				}
				if failure == "recipe" {
					rgaS0AssertNoCoverageArtifact(t, root, slug)
					return
				}
				if strings.Contains(failure, "coverage") {
					if !strings.Contains(err.Error(), "publish coverage") {
						t.Fatalf("publication error hidden: %v", err)
					}
					if failure == "state+coverage" {
						stateIndex := strings.Index(err.Error(), "status.json")
						coverageIndex := strings.Index(err.Error(), "publish coverage")
						var pathErr *fs.PathError
						if stateIndex < 0 || stateIndex >= coverageIndex || !errors.As(err, &pathErr) {
							t.Fatalf("primary state cause must precede chained coverage cause: %v", err)
						}
					}
					return
				}
				c := rgaS4ReadCoverage(t, s, slug)
				if c.Producer != patchobs.ProducerImplement || c.Capture.Mode != patchobs.CaptureModeNoCapture ||
					c.PatchPresent || c.CoverageStatus != CoverageIncomplete || !c.RecipePresent ||
					c.RecipeDecodable == raw || !slices.Contains(c.Reasons, "canonical-patch-missing") {
					t.Fatalf("P6 publication: %+v", c)
				}
				if raw && (c.RecipeSHA256 != CoverageSHA256([]byte(rawBytes)) || !slices.Contains(c.Reasons, "recipe-undecodable")) {
					t.Fatalf("raw write not bound truthfully: %+v", c)
				}
				if !raw && slices.Contains(c.Reasons, "recipe-undecodable") {
					t.Fatal("valid reserialized arm was classified undecodable")
				}
			})
		}
	}
}

func TestRGAS4RefreshSamePatchWritePublishesWithoutGeneration(t *testing.T) {
	s, root, slug := s1ImplementFixture(t)
	target := filepath.Join(root, "a.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rgaS2Git(t, root, "add", "a.txt")
	rgaS2Git(t, root, "commit", "-qm", "base")
	head, err := gitutil.HeadCommit(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch, err := gitutil.DiffFromCommitForPaths(root, head, []string{"a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	if err := RefreshAfterAccept(s, slug, head, patch); err != nil {
		t.Fatal(err)
	}
	c := rgaS4ReadCoverage(t, s, slug)
	if c.Producer != patchobs.ProducerReconcileAccept || c.Capture.Mode != patchobs.CaptureModeReconcile {
		t.Fatalf("same-patch refresh did not publish P3: %+v", c)
	}
	if _, err := s.ReadFeatureFile(slug, "artifacts/patch-generations.json"); !os.IsNotExist(err) {
		t.Fatalf("same-patch refresh appended generation: %v", err)
	}
	coveragePath := filepath.Join(s.TpatchDir(), "features", slug, "artifacts", "recipe-coverage.json")
	if err := os.Remove(coveragePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(coveragePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RefreshAfterAccept(s, slug, head, patch); err == nil || !strings.Contains(err.Error(), "publish coverage") {
		t.Fatalf("P3 publication failure must reach caller: %v", err)
	}
}

func TestRGAS4AcceptShadowPropagatesPublicationFailure(t *testing.T) {
	s, root, slug := s1ImplementFixture(t)
	head, err := gitutil.HeadCommit(root)
	if err != nil {
		t.Fatal(err)
	}
	patch := "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -1 +1,2 @@\n # Test\n+original\n"
	if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	shadow, err := gitutil.CreateShadow(root, slug, head)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gitutil.PruneShadow(root, slug) })
	if err := os.WriteFile(filepath.Join(shadow, "README.md"), []byte("# Test\nresolved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState(slug, store.StateReconcilingShadow, "reconcile", "fixture"); err != nil {
		t.Fatal(err)
	}
	state, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	state.Reconcile.ShadowPath = shadow
	state.Reconcile.UpstreamCommit = head
	state.Reconcile.ResolveSession = "s4-publication-failure"
	if err := s.SaveFeatureStatus(state); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(s.TpatchDir(), "features", slug, "status.json")
	statusBefore, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(s.TpatchDir(), "features", slug, "artifacts", "recipe-coverage.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := AcceptShadow(s, slug, []string{"README.md"}, head, AcceptOptions{})
	if !errors.Is(err, ErrCoveragePublication) {
		t.Fatalf("AcceptShadow swallowed owed publication failure: result=%+v err=%v", result, err)
	}
	if result == nil || result.Pruned || result.RefreshWarning != "" ||
		!slices.Equal(result.AcceptedFiles, []string{"README.md"}) {
		t.Fatalf("publication failure was converted into completed acceptance: %+v", result)
	}
	statusAfter, readErr := os.ReadFile(statusPath)
	if readErr != nil || !bytes.Equal(statusBefore, statusAfter) {
		t.Fatalf("failed acceptance changed state or cleared its recovery pointer: %v", readErr)
	}
	if got, readErr := os.ReadFile(filepath.Join(shadow, "README.md")); readErr != nil || string(got) != "# Test\nresolved\n" {
		t.Fatalf("failed publication pruned or damaged the recovery shadow: %q %v", got, readErr)
	}
	if got, readErr := os.ReadFile(filepath.Join(root, "README.md")); readErr != nil || string(got) != "# Test\nresolved\n" {
		t.Fatalf("accepted files must remain on disk without a false completion claim: %q %v", got, readErr)
	}
	canonical, readErr := s.ReadFeatureFile(slug, "artifacts/post-apply.patch")
	if readErr != nil || canonical == patch || !strings.Contains(canonical, "+resolved") {
		t.Fatalf("fixture did not reach the successful P3 bound write: %q %v", canonical, readErr)
	}
}

type rgaS4WorkflowRecorder func(patchobs.Observation)

func (f rgaS4WorkflowRecorder) Record(obs patchobs.Observation) { f(obs) }

func TestRGAS4ImplementProvenanceUsesPreWriteReference(t *testing.T) {
	s, root, slug := s1ImplementFixture(t)
	before, err := gitutil.HeadCommit(root)
	if err != nil {
		t.Fatal(err)
	}
	restore := patchobs.SetRecorder(rgaS4WorkflowRecorder(func(obs patchobs.Observation) {
		if obs.Producer != patchobs.ProducerImplement {
			t.Fatalf("unexpected producer at implement seam: %s", obs.Producer)
		}
		rgaS2Git(t, root, "commit", "--allow-empty", "-qm", "reference moved after observation")
	}))
	t.Cleanup(restore)
	if err := RunImplement(context.Background(), s, slug, nil, provider.Config{}); err != nil {
		t.Fatal(err)
	}
	after, err := gitutil.HeadCommit(root)
	if err != nil || before == after {
		t.Fatal("fixture did not move HEAD")
	}
	raw, err := s.ReadFeatureFile(slug, "artifacts/recipe-provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var provenance RecipeProvenance
	if err := json.Unmarshal([]byte(raw), &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.BaseCommit != before {
		t.Fatalf("provenance borrowed a post-write reference: %+v", provenance)
	}
	rgaS4ReadCoverage(t, s, slug)
}

func TestRGAS4AcceptShadowPreservesBestEffortPreEventWarnings(t *testing.T) {
	for _, upstream := range []string{"", ErrCoveragePublication.Error()} {
		t.Run(map[bool]string{true: "no-upstream", false: "ordinary-error-containing-publication-text"}[upstream == ""], func(t *testing.T) {
			s, root, slug := s1ImplementFixture(t)
			head, err := gitutil.HeadCommit(root)
			if err != nil {
				t.Fatal(err)
			}
			patch := "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -1 +1,2 @@\n # Test\n+original\n"
			if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
				t.Fatal(err)
			}
			shadow, err := gitutil.CreateShadow(root, slug, head)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = gitutil.PruneShadow(root, slug) })
			if err := os.WriteFile(filepath.Join(shadow, "README.md"), []byte("# Test\nresolved\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := s.MarkFeatureState(slug, store.StateReconcilingShadow, "reconcile", "fixture"); err != nil {
				t.Fatal(err)
			}
			result, err := AcceptShadow(s, slug, []string{"README.md"}, upstream, AcceptOptions{})
			if err != nil || result == nil || result.RefreshWarning == "" || !result.Pruned {
				t.Fatalf("unrelated pre-event warning became a publication failure: result=%+v err=%v", result, err)
			}
			if upstream != "" && !strings.Contains(result.RefreshWarning, upstream) {
				t.Fatal("fixture did not preserve the publication-shaped ordinary diagnostic")
			}
			state, err := s.LoadFeatureStatus(slug)
			if err != nil || state.State != store.StateApplied {
				t.Fatalf("legacy best-effort acceptance did not complete: %+v %v", state, err)
			}
			if _, err := os.Stat(shadow); !os.IsNotExist(err) {
				t.Fatalf("successful best-effort acceptance did not prune shadow: %v", err)
			}
			if actual, err := s.ReadFeatureFile(slug, "artifacts/post-apply.patch"); err != nil || actual != patch {
				t.Fatal("pre-event refresh refusal changed the canonical artifact")
			}
			rgaS0AssertNoCoverageArtifact(t, root, slug)
		})
	}
}

func TestRGAS4AutoAcceptPublicationFailureReachesRunReconcile(t *testing.T) {
	for _, scenario := range []string{"publication-failure", "successful-auto-accept", "legacy-blocked"} {
		t.Run(scenario, func(t *testing.T) {
			s, slug := buildConflictFixture(t)
			t.Cleanup(func() { _ = gitutil.PruneShadow(s.Root, slug) })
			cfg := provider.Config{Type: "openai-compatible", BaseURL: "http://x", Model: "m", AuthEnv: "X"}
			var prov provider.Provider = &scriptedProvider{
				responses: []string{`{"verdict":"unclear"}`},
				keyed:     map[string]string{"shared.txt": "a\nB-merged\nc\n"},
			}
			slugs := []string{slug}
			const runtimeErrorSlug = "runtime-error"
			if scenario == "publication-failure" {
				if err := os.Mkdir(filepath.Join(s.TpatchDir(), "features", slug, "artifacts", "recipe-coverage.json"), 0o755); err != nil {
					t.Fatal(err)
				}
				// This feature passes planning but has no canonical patch,
				// so its legacy error occurs inside reconcileFeature.
				if _, err := s.AddFeature(store.AddFeatureInput{
					Slug: runtimeErrorSlug, Title: "Runtime error", Request: "no patch fixture",
				}); err != nil {
					t.Fatal(err)
				}
				if err := s.MarkFeatureState(runtimeErrorSlug, store.StateApplied, "apply", "fixture without a recorded patch"); err != nil {
					t.Fatal(err)
				}
				slugs = append(slugs, runtimeErrorSlug)
				if planned, err := PlanReconcile(s, slugs); err != nil || len(planned) != len(slugs) ||
					!slices.Contains(planned, slug) || !slices.Contains(planned, runtimeErrorSlug) {
					t.Fatalf("fixture must pass planning before exercising P3: planned=%v err=%v", planned, err)
				}
			}
			if scenario == "legacy-blocked" {
				prov = nil
			}
			results, err := RunReconcile(context.Background(), s, slugs, "HEAD", prov, cfg, ReconcileOptions{Resolve: true, Apply: true})
			if len(results) != len(slugs) {
				t.Fatalf("structured results were dropped: %+v %v", results, err)
			}
			bySlug := make(map[string]ReconcileResult, len(results))
			for _, result := range results {
				bySlug[result.Slug] = result
			}
			result, found := bySlug[slug]
			if !found {
				t.Fatalf("P3 result missing: %+v", results)
			}
			state, stateErr := s.LoadFeatureStatus(slug)
			if stateErr != nil {
				t.Fatal(stateErr)
			}
			switch scenario {
			case "publication-failure":
				if !errors.Is(err, ErrCoveragePublication) || result.Outcome != store.ReconcileBlockedRequiresHuman ||
					result.ShadowPath == "" || result.ResolveSession == "" {
					t.Fatalf("auto-accept publication cause/recovery lost: result=%+v err=%v", result, err)
				}
				if state.State != store.StateBlocked || state.Reconcile.ShadowPath != result.ShadowPath ||
					state.Reconcile.ResolveSession != result.ResolveSession {
					t.Fatalf("blocked recovery state was not preserved: %+v", state)
				}
				if _, statErr := os.Stat(result.ShadowPath); statErr != nil {
					t.Fatalf("auto-accept pruned the recovery shadow: %v", statErr)
				}
				if got, readErr := os.ReadFile(filepath.Join(s.Root, "shared.txt")); readErr != nil || string(got) != "a\nB-merged\nc\n" {
					t.Fatalf("fixture did not reach the bound-write event: %q %v", got, readErr)
				}
				if patch, readErr := s.ReadFeatureFile(slug, "artifacts/post-apply.patch"); readErr != nil || !strings.Contains(patch, "+B-merged") {
					t.Fatalf("fixture did not reach the successful P3 canonical write: %q %v", patch, readErr)
				}
				legacy, found := bySlug[runtimeErrorSlug]
				if !found || legacy.Outcome != store.ReconcileBlocked || legacy.Phase != "error" ||
					!strings.Contains(strings.Join(legacy.Notes, "\n"), "no recorded patch") {
					t.Fatalf("ordinary runtime error semantics changed: %+v", legacy)
				}
			case "successful-auto-accept":
				if err != nil || result.Outcome != store.ReconcileReapplied || state.State != store.StateApplied ||
					state.Reconcile.ShadowPath != "" {
					t.Fatalf("successful auto-accept changed: %+v %+v %v", result, state, err)
				}
				if c := rgaS4ReadCoverage(t, s, slug); c.Producer != patchobs.ProducerReconcileAccept {
					t.Fatalf("successful auto-accept failed to publish P3: %+v", c)
				}
			case "legacy-blocked":
				if err != nil || result.Outcome != store.ReconcileBlockedRequiresHuman {
					t.Fatalf("ordinary blocked outcome became a returned error: %+v %v", result, err)
				}
				rgaS0AssertNoCoverageArtifact(t, s.Root, slug)
			}
		})
	}
}

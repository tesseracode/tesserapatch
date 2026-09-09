package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func rgaS4ReadCoverage(t *testing.T, s *store.Store, slug string) RecipeCoverage {
	t.Helper()
	raw, err := s.ReadFeatureFile(slug, "artifacts/recipe-coverage.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := DecodeRecipeCoverage([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	patch, patchErr := s.ReadFeatureFile(slug, "artifacts/post-apply.patch")
	recipe, recipeErr := s.ReadFeatureFile(slug, "artifacts/apply-recipe.json")
	if c.PatchPresent != (patchErr == nil) || (c.PatchPresent && c.PatchSHA256 != CoverageSHA256([]byte(patch))) ||
		c.RecipePresent != (recipeErr == nil) || (c.RecipePresent && c.RecipeSHA256 != CoverageSHA256([]byte(recipe))) {
		t.Fatalf("publication does not bind exact readable artifacts: %+v", c)
	}
	return c
}

func rgaS4PublicationFixture(t *testing.T) (*store.Store, CoveragePublicationInput) {
	t.Helper()
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Slug: "s3", Title: "S4", Request: "S4"}); err != nil {
		t.Fatal(err)
	}
	obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
	obs.RepoRoot = root
	derived, err := DeriveRecipe(obs)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("s3", "post-apply.patch", string(obs.PatchBytes)); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("s3", "apply-recipe.json", string(derived.CanonicalBytes())); err != nil {
		t.Fatal(err)
	}
	return s, ObserveCoveragePublication(s, obs)
}

func TestRGAS4PublishAllProducersAndFailures(t *testing.T) {
	for _, producer := range []patchobs.ProducerID{
		patchobs.ProducerRecord, patchobs.ProducerFeaturePatch, patchobs.ProducerReconcileAccept,
		patchobs.ProducerCycle, patchobs.ProducerApplyDone, patchobs.ProducerImplement, patchobs.ProducerEdit,
	} {
		t.Run(string(producer), func(t *testing.T) {
			s, in := rgaS4PublicationFixture(t)
			in.Observation.Producer = producer
			if producer == patchobs.ProducerImplement || producer == patchobs.ProducerEdit {
				in.Observation.Capture = patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeNoCapture}
			}
			c, err := PublishCoverage(s, in)
			if err != nil || c.CoverageStatus != CoverageComplete || c.Producer != producer {
				t.Fatalf("publication: %+v %v", c, err)
			}
			rgaS4ReadCoverage(t, s, "s3")
			first, _ := s.ReadFeatureFile("s3", "artifacts/recipe-coverage.json")
			if _, err := PublishCoverage(s, in); err != nil {
				t.Fatal(err)
			}
			second, _ := s.ReadFeatureFile("s3", "artifacts/recipe-coverage.json")
			if first != second {
				t.Fatal("identical event inputs must encode byte-identically")
			}
			target := filepath.Join(s.TpatchDir(), "features", "s3", "artifacts", "recipe-coverage.json")
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := PublishCoverage(s, in); err == nil || !errors.Is(err, ErrCoveragePublication) {
				t.Fatalf("publication failure hidden: %v", err)
			}
			leaks, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".recipe-coverage.json.tmp-*"))
			if err != nil || len(leaks) != 0 {
				t.Fatalf("atomic replacement leaked files: %v %v", leaks, err)
			}
		})
	}
}

func TestRGAS4PrecomputedRecipePlanCoverageLastAndRecovery(t *testing.T) {
	s, in := rgaS4PublicationFixture(t)
	for _, name := range []string{"apply-recipe.json", "recipe-provenance.json"} {
		_ = os.Remove(filepath.Join(s.TpatchDir(), "features", "s3", "artifacts", name))
	}
	in = ObserveCoveragePublication(s, in.Observation)
	in.DeferRecipeWrites = true
	out, err := AutogenRecipeForRecord(s, in.Observation, true, false, in)
	if err != nil || out.Action != AutogenGenerated || !out.OriginProved {
		t.Fatalf("plan: %+v %v", out, err)
	}
	if _, err := s.ReadFeatureFile("s3", "artifacts/apply-recipe.json"); !os.IsNotExist(err) {
		t.Fatalf("planning must not write recipe: %v", err)
	}
	in.Autogen = &out
	provPath := filepath.Join(s.TpatchDir(), "features", "s3", "artifacts", "recipe-provenance.json")
	if err := os.Mkdir(provPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishCoverage(s, in); err == nil || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("expected failure between recipe and provenance: %v", err)
	}
	recipe, err := s.ReadFeatureFile("s3", "artifacts/apply-recipe.json")
	if err != nil || !bytes.Equal([]byte(recipe), out.Recipe.Bytes) {
		t.Fatalf("recipe must land before failing provenance: %v", err)
	}
	rgaS0AssertNoCoverageArtifact(t, s.Root, "s3")
	if err := os.Remove(provPath); err != nil {
		t.Fatal(err)
	}
	retry := ObserveCoveragePublication(s, in.Observation)
	retry.DeferRecipeWrites = true
	repaired, err := AutogenRecipeForRecord(s, retry.Observation, true, false, retry)
	if err != nil || repaired.Action != AutogenNoop || !repaired.OriginProved {
		t.Fatalf("D6 noop recovery: %+v %v", repaired, err)
	}
	retry.Autogen = &repaired
	if _, err := PublishCoverage(s, retry); err != nil {
		t.Fatal(err)
	}
	rgaS4ReadCoverage(t, s, "s3")
	first, _ := os.ReadFile(provPath)
	retry = ObserveCoveragePublication(s, in.Observation)
	retry.DeferRecipeWrites = true
	out, err = AutogenRecipeForRecord(s, retry.Observation, false, false, retry)
	if err != nil {
		t.Fatal(err)
	}
	retry.Autogen = &out
	if _, err := PublishCoverage(s, retry); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(provPath)
	if !bytes.Equal(first, second) || out.ProvenanceWritten {
		t.Fatal("truthful existing provenance timestamp/bytes must converge unchanged")
	}
}

func TestRGAS4IncompletePreservationAndExactRewriteReasons(t *testing.T) {
	s, in := rgaS4PublicationFixture(t)
	manual := `{"feature":"s3","operations":[{"type":"write-file","path":"a.txt","content":"wrong\n"}]}`
	if err := s.WriteArtifact("s3", "apply-recipe.json", manual); err != nil {
		t.Fatal(err)
	}
	in = ObserveCoveragePublication(s, in.Observation)
	in.Observation.Producer = patchobs.ProducerFeaturePatch
	for _, rewrite := range []bool{false, true} {
		in.Events.PatchRewritten = rewrite
		c, err := PublishCoverage(s, in)
		if err != nil {
			t.Fatal(err)
		}
		for _, reason := range []string{"producer-patch-rewrite", "recipe-not-regenerated"} {
			if slices.Contains(c.Reasons, reason) != rewrite {
				t.Fatalf("rewrite=%v reasons=%v", rewrite, c.Reasons)
			}
		}
		if got, _ := s.ReadFeatureFile("s3", "artifacts/apply-recipe.json"); got != manual {
			t.Fatal("publisher rewrote preserved recipe")
		}
	}
	deletion := rgaS3Observe(t, rgaS3DeletePatch, rgaS3Image{pre: "old\n"})
	deletion.RepoRoot = s.Root
	in = ObserveCoveragePublication(s, deletion)
	in.DeferRecipeWrites = true
	out, err := AutogenRecipeForRecord(s, deletion, true, true, in)
	if err != nil || out.Action != AutogenStale || out.RecipeWritten || out.OriginProved {
		t.Fatalf("incomplete regeneration must preserve recipe: %+v %v", out, err)
	}
	if string(out.Recipe.Bytes) != manual {
		t.Fatal("incomplete plan lost preserved bytes")
	}
}

func TestRGAS4P2CheckpointUsesSemanticCoverageWithoutOriginClaim(t *testing.T) {
	for _, rewritten := range []bool{false, true} {
		t.Run(map[bool]string{false: "checkpoint", true: "patch-writing-negative-control"}[rewritten], func(t *testing.T) {
			s, in := rgaS4PublicationFixture(t)
			var compact bytes.Buffer
			if err := json.Compact(&compact, in.Recipe.Bytes); err != nil {
				t.Fatal(err)
			}
			if err := s.WriteArtifactAtomic("s3", "apply-recipe.json", compact.String()); err != nil {
				t.Fatal(err)
			}
			in.Observation.Producer = patchobs.ProducerFeaturePatch
			in = ObserveCoveragePublication(s, in.Observation)
			in.Events.PatchRewritten = rewritten
			if rewritten {
				if err := s.WriteArtifactAtomic("s3", "post-apply.patch", string(in.Observation.PatchBytes)); err != nil {
					t.Fatal(err)
				}
			}
			featureDir := filepath.Join(s.TpatchDir(), "features", "s3")
			before := snapshotTreeForRefresh(t, featureDir)
			c, err := PublishCoverage(s, in)
			if rewritten {
				if !errors.Is(err, ErrCoveragePublication) || !strings.Contains(err.Error(), "patch-writing event") {
					t.Fatalf("P2 writing event lost its D16 requirement: %+v %v", c, err)
				}
				rgaS0AssertNoCoverageArtifact(t, s.Root, "s3")
			} else {
				if err != nil || c.CoverageStatus != CoverageComplete || len(c.Reasons) != 0 {
					t.Fatalf("coverage-only checkpoint must use the unchanged semantic predicates: %+v %v", c, err)
				}
				rgaS4ReadCoverage(t, s, "s3")
			}
			after := snapshotTreeForRefresh(t, featureDir)
			delete(after, "artifacts/recipe-coverage.json")
			if !reflect.DeepEqual(before, after) {
				t.Fatal("P2 origin policy changed a non-coverage artifact or fabricated provenance")
			}
		})
	}
}

func TestRGAS4PublishUsesCapturedImagesAndRejectsChangedPlan(t *testing.T) {
	s, in := rgaS4PublicationFixture(t)
	target := filepath.Join(s.Root, "a.txt")
	if err := os.WriteFile(target, []byte("late worktree mutation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := PublishCoverage(s, in)
	if err != nil || c.CoverageStatus != CoverageComplete {
		t.Fatalf("publisher reread a late worktree mutation: %+v %v", c, err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "late worktree mutation\n" {
		t.Fatal("publisher executed recipe operations")
	}
	recipePath := filepath.Join(s.TpatchDir(), "features", "s3", "artifacts", "apply-recipe.json")
	if err := os.Remove(recipePath); err != nil {
		t.Fatal(err)
	}
	in = ObserveCoveragePublication(s, in.Observation)
	in.DeferRecipeWrites = true
	out, err := AutogenRecipeForRecord(s, in.Observation, true, false, in)
	if err != nil {
		t.Fatal(err)
	}
	out.Recipe.Bytes = append(out.Recipe.Bytes, '\n')
	in.Autogen = &out
	if _, err := PublishCoverage(s, in); !errors.Is(err, ErrCoveragePublication) {
		t.Fatalf("publisher accepted bytes that differ from its planned write: %v", err)
	}
	if _, err := os.Stat(recipePath); !os.IsNotExist(err) {
		t.Fatal("refused plan wrote a recipe")
	}
}

func TestRGAS4UnknownMarkerObservationCannotInventAReason(t *testing.T) {
	s, in := rgaS4PublicationFixture(t)
	artifacts := filepath.Join(s.TpatchDir(), "features", "s3", "artifacts")
	saved := artifacts + "-saved"
	if err := os.Rename(artifacts, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifacts, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	in = ObserveCoveragePublication(s, in.Observation)
	if in.Events.StaleMarkerPresent {
		t.Fatalf("unestablished marker presence must not be encoded as present: %+v", in)
	}
	// Some platforms classify a non-directory ancestor as path-not-found.
	// Supply the unavailable outcome explicitly there to exercise the same
	// publication guard rather than relying on an OS-specific errno.
	if in.observationErr == nil {
		in.observationErr = errors.New("injected unavailable stale-marker observation")
	}
	if _, err := PublishCoverage(s, in); !errors.Is(err, ErrCoveragePublication) || !errors.Is(err, in.observationErr) {
		t.Fatalf("snapshot failure must survive publication wrapping: %v", err)
	}
	if got, err := os.ReadFile(artifacts); err != nil || string(got) != "not a directory" {
		t.Fatal("publication changed an unobserved artifact boundary")
	}
	if _, err := os.Stat(filepath.Join(saved, "recipe-coverage.json")); !os.IsNotExist(err) {
		t.Fatal("failed observation invented a coverage record")
	}
}

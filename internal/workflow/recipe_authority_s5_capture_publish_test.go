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

func rgaS5ReadPublishedPair(t *testing.T, s *store.Store, slug string) (RecipeCaptureEvent, []byte) {
	t.Helper()
	rawE, err := s.ReadFeatureFile(slug, "artifacts/recipe-capture-event.json")
	if err != nil {
		t.Fatal(err)
	}
	e, err := DecodeRecipeCaptureEvent([]byte(rawE))
	if err != nil {
		t.Fatal(err)
	}
	rawC, err := s.ReadFeatureFile(slug, "artifacts/recipe-coverage.json")
	if err != nil {
		t.Fatal(err)
	}
	patch, patchErr := s.ReadFeatureFile(slug, "artifacts/post-apply.patch")
	recipe, recipeErr := s.ReadFeatureFile(slug, "artifacts/apply-recipe.json")
	bindings := RecipeCaptureBindings{
		RepoRoot: s.Root, Feature: slug,
		Patch:  CoverageArtifact{Present: patchErr == nil, Bytes: []byte(patch)},
		Recipe: CoverageArtifact{Present: recipeErr == nil, Bytes: []byte(recipe)},
	}
	if err := ValidateRecipeCaptureEventPair(e, []byte(rawC), bindings); err != nil {
		t.Fatal(err)
	}
	return e, []byte(rawC)
}

func rgaS5DurablePublicationFixture(t *testing.T) (*store.Store, CoveragePublicationInput) {
	t.Helper()
	s, in := rgaS4PublicationFixture(t)
	mustGit(t, s.Root, "init", "-q")
	mustGit(t, s.Root, "config", "user.name", "S5 evidence")
	mustGit(t, s.Root, "config", "user.email", "s5@example.invalid")
	mustGit(t, s.Root, "config", "commit.gpgsign", "false")
	writeRepoFile(t, s, "a.txt", []byte("old\n"))
	mustGit(t, s.Root, "add", "a.txt")
	mustGit(t, s.Root, "commit", "-qm", "captured base")
	in.Observation.Reference.Commit = gitHeadOf(t, s.Root)
	writeRepoFile(t, s, "a.txt", []byte("new\n"))
	return s, ObserveCoveragePublication(s, in.Observation)
}

func TestRGAS5CapturePublicationAllProducerPairs(t *testing.T) {
	for _, producer := range []patchobs.ProducerID{
		patchobs.ProducerRecord, patchobs.ProducerFeaturePatch, patchobs.ProducerReconcileAccept,
		patchobs.ProducerCycle, patchobs.ProducerApplyDone, patchobs.ProducerImplement, patchobs.ProducerEdit,
	} {
		t.Run(string(producer), func(t *testing.T) {
			s, in := rgaS4PublicationFixture(t)
			in.Observation.Producer = producer
			if producer == patchobs.ProducerImplement || producer == patchobs.ProducerEdit {
				obs := rgaS3Observe(t, rgaS3ModifyPatch)
				obs.RepoRoot, obs.Producer = s.Root, producer
				obs.Reference.Kind, obs.Reference.Commit = patchobs.ReferenceKindUnavailable, ""
				obs.Capture = patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeNoCapture}
				in.Observation = obs
			}
			if producer == patchobs.ProducerEdit {
				in.Events.BoundArtifactEdited = true
			}
			var order []string
			in.beforePairWrite = func(name string) error {
				order = append(order, name)
				if _, err := s.ReadFeatureFile("s3", "artifacts/recipe-coverage.json"); !os.IsNotExist(err) {
					t.Fatalf("C was published before its final boundary: %v", err)
				}
				if name == "recipe-coverage.json" {
					rawE, err := s.ReadFeatureFile("s3", "artifacts/recipe-capture-event.json")
					if err != nil {
						t.Fatal("C boundary reached before E was durable")
					}
					if _, err := DecodeRecipeCaptureEvent([]byte(rawE)); err != nil {
						t.Fatal(err)
					}
				}
				return nil
			}
			coverage, err := PublishCoverage(s, in)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(order, []string{"recipe-capture-event.json", "recipe-coverage.json"}) {
				t.Fatalf("pair order/cardinality: %v", order)
			}
			e, rawC := rgaS5ReadPublishedPair(t, s, "s3")
			if e.Capture.Mode == patchobs.CaptureModeNoCapture && coverage.CoverageStatus != CoverageIncomplete {
				t.Fatal("unavailable no-capture publication became complete")
			}
			if err := ValidateRecipeCaptureEventSource(e, rawC, RecipeCoverageInput{
				Observation: in.Observation, Recipe: in.Recipe, Events: in.Events,
			}); err != nil {
				t.Fatal(err)
			}
			in.beforePairWrite = nil
			firstE, _ := s.ReadFeatureFile("s3", "artifacts/recipe-capture-event.json")
			if _, err := PublishCoverage(s, in); err != nil {
				t.Fatal(err)
			}
			secondE, _ := s.ReadFeatureFile("s3", "artifacts/recipe-capture-event.json")
			secondC, _ := s.ReadFeatureFile("s3", "artifacts/recipe-coverage.json")
			if firstE != secondE || !bytes.Equal(rawC, []byte(secondC)) {
				t.Fatal("identical event rerun changed the deterministic pair")
			}
		})
	}
}

func TestRGAS5CapturePublicationBoundariesAndRecovery(t *testing.T) {
	for _, boundary := range []string{"recipe-capture-event.json", "recipe-coverage.json"} {
		t.Run(boundary, func(t *testing.T) {
			s, in := rgaS4PublicationFixture(t)
			if _, err := PublishCoverage(s, in); err != nil {
				t.Fatal(err)
			}
			oldE, oldC := rgaS5ReadPublishedPair(t, s, "s3")
			in.Observation.Capture.Mode = patchobs.CaptureModeStagedIndex
			cause := errors.New("injected pair boundary failure")
			var attempted []string
			in.beforePairWrite = func(name string) error {
				attempted = append(attempted, name)
				if name == boundary {
					return cause
				}
				return nil
			}
			c, err := PublishCoverage(s, in)
			if !errors.Is(err, cause) || !errors.Is(err, ErrCoveragePublication) || !strings.Contains(err.Error(), boundary) {
				t.Fatalf("publication path/cause chain was lost: %v", err)
			}
			var output bytes.Buffer
			if reportErr := ReportCoverageStatus(&output, c, err); !errors.Is(reportErr, cause) || output.Len() != 0 {
				t.Fatal("failed publication emitted success-shaped status")
			}
			currentC, _ := s.ReadFeatureFile("s3", "artifacts/recipe-coverage.json")
			if !bytes.Equal(oldC, []byte(currentC)) {
				t.Fatal("failed boundary replaced C")
			}
			if boundary == "recipe-capture-event.json" {
				if !slices.Equal(attempted, []string{boundary}) {
					t.Fatalf("E failure did not prevent C attempt: %v", attempted)
				}
				stillE, _ := rgaS5ReadPublishedPair(t, s, "s3")
				if !reflect.DeepEqual(oldE, stillE) {
					t.Fatal("E failure changed the previous complete file")
				}
			} else {
				rawE, _ := s.ReadFeatureFile("s3", "artifacts/recipe-capture-event.json")
				e, decodeErr := DecodeRecipeCaptureEvent([]byte(rawE))
				if decodeErr != nil {
					t.Fatal("E was partially encoded at the C failure boundary")
				}
				bindings := RecipeCaptureBindings{
					RepoRoot: s.Root, Feature: "s3",
					Patch: CoverageArtifact{Present: true, Bytes: in.Observation.PatchBytes}, Recipe: in.Recipe,
				}
				if err := ValidateRecipeCaptureEventPair(e, oldC, bindings); err == nil {
					t.Fatal("the actual pair validator accepted a mixed publication")
				}
			}
			in.beforePairWrite = nil
			if _, err := PublishCoverage(s, in); err != nil {
				t.Fatal(err)
			}
			repaired, _ := rgaS5ReadPublishedPair(t, s, "s3")
			if repaired.Capture.Mode != patchobs.CaptureModeStagedIndex {
				t.Fatal("real producer rerun did not publish the new captured event")
			}
		})
	}
}

func TestRGAS5CapturePublicationAtomicRenameFailures(t *testing.T) {
	for _, boundary := range []string{"recipe-capture-event.json", "recipe-coverage.json"} {
		t.Run(boundary, func(t *testing.T) {
			s, in := rgaS4PublicationFixture(t)
			artifacts := filepath.Join(s.TpatchDir(), "features", "s3", "artifacts")
			if err := os.Mkdir(filepath.Join(artifacts, boundary), 0o755); err != nil {
				t.Fatal(err)
			}
			_, err := PublishCoverage(s, in)
			if !errors.Is(err, ErrCoveragePublication) || !strings.Contains(err.Error(), boundary) {
				t.Fatalf("real atomic rename failure was not propagated: %v", err)
			}
			if boundary == "recipe-capture-event.json" {
				if _, err := s.ReadFeatureFile("s3", "artifacts/recipe-coverage.json"); !os.IsNotExist(err) {
					t.Fatal("E rename failure did not suppress C")
				}
			} else {
				raw, err := s.ReadFeatureFile("s3", "artifacts/recipe-capture-event.json")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := DecodeRecipeCaptureEvent([]byte(raw)); err != nil {
					t.Fatal("C failure did not retain a whole E")
				}
			}
			leftovers, err := filepath.Glob(filepath.Join(artifacts, "."+boundary+".tmp-*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("atomic rename leaked partial evidence: %v %v", leftovers, err)
			}
		})
	}
}

func TestRGAS5SamePatchCaptureChangeWithoutGenerationAppend(t *testing.T) {
	s, in := rgaS5DurablePublicationFixture(t)
	in.Generation = &PatchGenerationInput{
		Kind: store.PatchGenerationKindRecord, Patch: string(in.Observation.PatchBytes),
		BaseCommit: in.Observation.Reference.Commit,
		Capture:    store.GenerationCapture{Mode: "working-tree-all", Pathspecs: []string{}, ClaimIDs: []string{}},
	}
	if _, err := PublishCoverage(s, in); err != nil {
		t.Fatal(err)
	}
	firstE, firstC := rgaS5ReadPublishedPair(t, s, "s3")
	firstGeneration, err := os.ReadFile(s.PatchGenerationsPath("s3"))
	if err != nil {
		t.Fatal(err)
	}
	in.Observation.Capture.Mode = patchobs.CaptureModeStagedIndex
	in.Generation.Capture.Mode = "staged-index"
	if _, err := PublishCoverage(s, in); err != nil {
		t.Fatal(err)
	}
	secondE, secondC := rgaS5ReadPublishedPair(t, s, "s3")
	secondGeneration, err := os.ReadFile(s.PatchGenerationsPath("s3"))
	if err != nil || !bytes.Equal(firstGeneration, secondGeneration) {
		t.Fatal("same patch-byte publication altered generation history")
	}
	if reflect.DeepEqual(firstE, secondE) || bytes.Equal(firstC, secondC) ||
		firstE.PatchSHA256 != secondE.PatchSHA256 || firstE.RecipeSHA256 != secondE.RecipeSHA256 {
		t.Fatal("new independent capture was suppressed by the unchanged generation")
	}
}

func TestRGAS5P2CheckpointChangesExactlyPair(t *testing.T) {
	s, in := rgaS4PublicationFixture(t)
	in.Observation.Producer = patchobs.ProducerFeaturePatch
	var compact bytes.Buffer
	if err := json.Compact(&compact, in.Recipe.Bytes); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("s3", "apply-recipe.json", compact.String()); err != nil {
		t.Fatal(err)
	}
	in = ObserveCoveragePublication(s, in.Observation)
	featureDir := filepath.Join(s.TpatchDir(), "features", "s3")
	before := snapshotTreeForRefresh(t, featureDir)
	c, err := PublishCoverage(s, in)
	if err != nil || c.CoverageStatus != CoverageComplete {
		t.Fatalf("P2 checkpoint acquired a writing-event origin demand: %+v %v", c, err)
	}
	after := snapshotTreeForRefresh(t, featureDir)
	for _, name := range []string{"artifacts/recipe-capture-event.json", "artifacts/recipe-coverage.json"} {
		if _, exists := after[name]; !exists {
			t.Fatalf("checkpoint omitted %s", name)
		}
		delete(after, name)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("P2 checkpoint changed an artifact outside E/C")
	}
	e, _ := rgaS5ReadPublishedPair(t, s, "s3")
	if e.Event.PatchRewritten || e.Event.RecipeRegenerated {
		t.Fatal("coverage-only checkpoint invented an earlier write event")
	}
}

func TestRGAS5NoCaptureCarryRequiresPriorPairAndFrozenImages(t *testing.T) {
	for _, corrupt := range []string{"", "missing-e", "malformed-e", "wrong-owner", "raw-c-change", "capture-change"} {
		t.Run(corrupt, func(t *testing.T) {
			s, in := rgaS5DurablePublicationFixture(t)
			if _, err := PublishCoverage(s, in); err != nil {
				t.Fatal(err)
			}
			e, rawC := rgaS5ReadPublishedPair(t, s, "s3")
			eventPath := filepath.Join(s.TpatchDir(), "features", "s3", "artifacts", "recipe-capture-event.json")
			switch corrupt {
			case "missing-e":
				if err := os.Remove(eventPath); err != nil {
					t.Fatal(err)
				}
			case "malformed-e":
				if err := s.WriteArtifact("s3", "recipe-capture-event.json", "{"); err != nil {
					t.Fatal(err)
				}
			case "wrong-owner", "capture-change":
				if corrupt == "wrong-owner" {
					e.Feature = "other"
				} else {
					e.Capture.Mode = patchobs.CaptureModeStagedIndex
				}
				raw, err := EncodeRecipeCaptureEvent(e)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.WriteArtifact("s3", "recipe-capture-event.json", string(raw)); err != nil {
					t.Fatal(err)
				}
			case "raw-c-change":
				if err := s.WriteArtifact("s3", "recipe-coverage.json", string(rawC)+"\n"); err != nil {
					t.Fatal(err)
				}
			}
			publication := ObserveCoveragePublication(s, patchobs.Observation{Producer: patchobs.ProducerEdit, RepoRoot: s.Root, Slug: "s3"})
			after := patchobs.Observe(patchobs.Input{
				Producer: patchobs.ProducerEdit, RepoRoot: s.Root, Slug: "s3",
				Patch: string(in.Observation.PatchBytes), PatchPresent: true,
				Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeNoCapture},
			})
			publication.Observation = after
			publication.Events.BoundArtifactEdited = true
			// The editor can outlive unrelated changes. Reference carry may
			// only use the context frozen before it started.
			writeRepoFile(t, s, "a.txt", []byte("late live mutation\n"))
			publication = ReconstructEditedCoverage(s, publication)
			c, err := PublishCoverage(s, publication)
			if err != nil {
				t.Fatal(err)
			}
			actualE, _ := rgaS5ReadPublishedPair(t, s, "s3")
			wantKind := patchobs.ReferenceKindCommit
			wantStatus := CoverageComplete
			if corrupt != "" {
				wantKind, wantStatus = patchobs.ReferenceKindUnavailable, CoverageIncomplete
			}
			if c.Reference.Kind != wantKind || c.CoverageStatus != wantStatus ||
				actualE.Capture.Mode != patchobs.CaptureModeNoCapture {
				t.Fatalf("prior evidence validation/frozen context lost: %+v", c)
			}
		})
	}
}

func TestRGAS5NoCaptureCannotCopyUnprovedCommit(t *testing.T) {
	s, in := rgaS4PublicationFixture(t)
	in.Observation.Producer = patchobs.ProducerImplement
	in.Observation.Capture = patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeNoCapture}
	if _, err := PublishCoverage(s, in); !errors.Is(err, ErrCoveragePublication) ||
		!strings.Contains(err.Error(), "validated frozen prior evidence pair") {
		t.Fatalf("no-capture producer copied a commit without independent evidence: %v", err)
	}
	for _, name := range []string{"recipe-capture-event.json", "recipe-coverage.json"} {
		if _, err := s.ReadFeatureFile("s3", "artifacts/"+name); !os.IsNotExist(err) {
			t.Fatalf("unproved prior reference wrote %s", name)
		}
	}
}

func TestRGAS5NoCapturePublicationBindsUndecodableRecipeBytes(t *testing.T) {
	s, in := rgaS4PublicationFixture(t)
	rawRecipe := "{not decodable}\r\n"
	if err := s.WriteArtifact("s3", "apply-recipe.json", rawRecipe); err != nil {
		t.Fatal(err)
	}
	obs := patchobs.Observe(patchobs.Input{
		Producer: patchobs.ProducerImplement, RepoRoot: s.Root, Slug: "s3",
		Patch: string(in.Observation.PatchBytes), PatchPresent: true,
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeNoCapture},
	})
	in = ObserveCoveragePublication(s, obs)
	c, err := PublishCoverage(s, in)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := rgaS5ReadPublishedPair(t, s, "s3")
	if c.RecipeDecodable || c.CoverageStatus != CoverageIncomplete ||
		!slices.Contains(c.Reasons, "recipe-undecodable") ||
		!e.RecipePresent || e.RecipeSHA256 != CoverageSHA256([]byte(rawRecipe)) {
		t.Fatalf("raw P6 bytes lost their truthful incomplete binding: %+v %+v", c, e)
	}
}

func TestRGAS5ObservationAndSameByteFailureDoNotClaimNewEvent(t *testing.T) {
	s, in := rgaS4PublicationFixture(t)
	if _, err := PublishCoverage(s, in); err != nil {
		t.Fatal(err)
	}
	featureDir := filepath.Join(s.TpatchDir(), "features", "s3")
	before := snapshotTreeForRefresh(t, featureDir)
	_ = ObserveCoveragePublication(s, in.Observation)
	after := snapshotTreeForRefresh(t, featureDir)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("observation repaired or published evidence without an event")
	}
	cause := errors.New("same-byte event failed before E")
	in.beforePairWrite = func(string) error { return cause }
	c, err := PublishCoverage(s, in)
	if !errors.Is(err, cause) {
		t.Fatal("same-byte interruption was hidden")
	}
	var output bytes.Buffer
	if err := ReportCoverageStatus(&output, c, err); !errors.Is(err, cause) || output.Len() != 0 {
		t.Fatal("retained old pair was reported as this failed event's completion")
	}
	rgaS5ReadPublishedPair(t, s, "s3")
	if after = snapshotTreeForRefresh(t, featureDir); !reflect.DeepEqual(before, after) {
		t.Fatal("pre-E interruption modified the previous coherent pair")
	}
}

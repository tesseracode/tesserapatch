package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/provider"
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
		Upper:      store.GenerationUpper{Kind: "working-tree", Ref: "working-tree"},
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
	in.Generation.Upper = store.GenerationUpper{Kind: "index", Ref: "index"}
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

func TestRGAS5ImplementPrimaryErrorsSuppressPairStatus(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, failure := range []string{"", "state", "provenance", "state+event", "state+coverage"} {
			t.Run(fmt.Sprintf("raw=%v/%s", raw, failure), func(t *testing.T) {
				s, _, slug := s1ImplementFixture(t)
				if raw {
					previous := implementRecipeContentHook
					implementRecipeContentHook = func(string) string { return "undecodable provider response\n" }
					t.Cleanup(func() { implementRecipeContentHook = previous })
				}
				feature := filepath.Join(s.TpatchDir(), "features", slug)
				for _, part := range strings.Split(failure, "+") {
					var path string
					switch part {
					case "state":
						path = filepath.Join(feature, "status.json")
						if err := os.Remove(path); err != nil {
							t.Fatal(err)
						}
					case "provenance":
						path = filepath.Join(feature, "artifacts", "recipe-provenance.json")
					case "event":
						path = filepath.Join(feature, "artifacts", "recipe-capture-event.json")
					case "coverage":
						path = filepath.Join(feature, "artifacts", "recipe-coverage.json")
					}
					if path != "" {
						if err := os.Mkdir(path, 0o755); err != nil {
							t.Fatal(err)
						}
					}
				}
				output, err := rgaS4CaptureCoverageStderr(t, func() error {
					return RunImplement(context.Background(), s, slug, nil, provider.Config{})
				})
				if (err != nil) != (failure != "") {
					t.Fatalf("primary/publication outcome changed: %v", err)
				}
				statusLines := strings.Count(output, "recipe coverage:")
				if (failure == "" && statusLines != 1) || (failure != "" && statusLines != 0) {
					t.Fatalf("completion status ignored the enclosing result: %q error=%v", output, err)
				}
				pairFailure := strings.Contains(failure, "+")
				if errors.Is(err, ErrCoveragePublication) != pairFailure {
					t.Fatalf("publication sentinel was swallowed or manufactured: %v", err)
				}
				if strings.Contains(failure, "state") {
					var cause *os.PathError
					if !errors.As(err, &cause) || !strings.Contains(cause.Path, "status.json") {
						t.Fatalf("primary state read cause was lost: %v", err)
					}
				}
				switch failure {
				case "state+event", "state+coverage":
					artifact := "recipe-capture-event.json"
					if failure == "state+coverage" {
						artifact = "recipe-coverage.json"
					}
					primary := strings.Index(err.Error(), "status.json")
					secondary := strings.Index(err.Error(), artifact)
					if primary < 0 || secondary <= primary {
						t.Fatalf("primary cause must precede publication cause: %v", err)
					}
					if failure == "state+event" {
						if _, readErr := s.ReadFeatureFile(slug, "artifacts/recipe-coverage.json"); !os.IsNotExist(readErr) {
							t.Fatal("E failure did not prevent C publication")
						}
					} else {
						event, readErr := s.ReadFeatureFile(slug, "artifacts/recipe-capture-event.json")
						if readErr != nil {
							t.Fatal(readErr)
						}
						if _, decodeErr := DecodeRecipeCaptureEvent([]byte(event)); decodeErr != nil {
							t.Fatal("state/C failure did not retain whole E")
						}
					}
				default:
					e, c := rgaS5ReadPublishedPair(t, s, slug)
					coverage, decodeErr := DecodeRecipeCoverage(c)
					if decodeErr != nil || e.Capture.Mode != patchobs.CaptureModeNoCapture ||
						!e.RecipePresent || coverage.RecipeDecodable == raw {
						t.Fatalf("primary error cancelled the owed truthful P6 pair: %+v %v", coverage, decodeErr)
					}
				}
			})
		}
	}
}

func TestRGAS5RefreshPrimaryErrorsSuppressPairStatus(t *testing.T) {
	for _, boundary := range []string{"", "recipe-capture-event.json", "recipe-coverage.json"} {
		t.Run(boundary, func(t *testing.T) {
			s, in := rgaS5DurablePublicationFixture(t)
			feature := filepath.Join(s.TpatchDir(), "features", "s3")
			patches := filepath.Join(feature, "patches")
			if err := os.Remove(patches); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := os.WriteFile(patches, []byte("block the numbered audit write"), 0o644); err != nil {
				t.Fatal(err)
			}
			if boundary != "" {
				if err := os.Mkdir(filepath.Join(feature, "artifacts", boundary), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			output, err := rgaS4CaptureCoverageStderr(t, func() error {
				return RefreshAfterAccept(s, "s3", in.Observation.Reference.Commit, string(in.Observation.PatchBytes))
			})
			if err == nil || !strings.Contains(err.Error(), "write numbered reconcile patch") {
				t.Fatalf("fixture did not reach the primary post-patch failure: %v", err)
			}
			if strings.Contains(output, "recipe coverage:") {
				t.Fatalf("failed refresh reported successful completion: %q", output)
			}
			var primary *os.PathError
			if !errors.As(err, &primary) || primary.Path != patches {
				t.Fatalf("the underlying primary audit-write cause was lost: %v", err)
			}
			if boundary == "" {
				e, _ := rgaS5ReadPublishedPair(t, s, "s3")
				if !e.Event.PatchRewritten || e.Capture.Mode != patchobs.CaptureModeReconcile {
					t.Fatal("primary audit error cancelled the owed P3 event")
				}
			} else {
				if !errors.Is(err, ErrCoveragePublication) ||
					strings.Index(err.Error(), boundary) <= strings.Index(err.Error(), "write numbered reconcile patch") {
					t.Fatalf("primary/publication failures were not chained in order: %v", err)
				}
				if boundary == "recipe-capture-event.json" {
					if _, readErr := s.ReadFeatureFile("s3", "artifacts/recipe-coverage.json"); !os.IsNotExist(readErr) {
						t.Fatal("E failure did not prevent C publication after primary failure")
					}
				}
			}
		})
	}
}

func rgaS5CallerClosureSource(src, function string, causes []string) error {
	file, err := rgaS0Parse(function, src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, function)
	if fn == nil {
		return fmt.Errorf("caller %s is missing", function)
	}
	var reports, observations, editors []*ast.CallExpr
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch strings.TrimPrefix(rgaS0CallName(call), "workflow.") {
		case "ReportCoverageStatus":
			reports = append(reports, call)
		case "ObserveCoveragePublication":
			observations = append(observations, call)
		case "openInEditor":
			editors = append(editors, call)
		}
		return true
	})
	if len(reports) != 1 || len(reports[0].Args) != 3 {
		return fmt.Errorf("caller needs exactly one three-argument status reporter")
	}
	report := reports[0]
	join, ok := report.Args[2].(*ast.CallExpr)
	if !ok || rgaS0CallName(join) != "errors.Join" || len(join.Args) != len(causes) {
		return fmt.Errorf("status reporter must receive the combined primary/publication result")
	}
	for i, cause := range causes {
		id, ok := join.Args[i].(*ast.Ident)
		if !ok || id.Name != cause {
			return fmt.Errorf("cause %d must be %s", i, cause)
		}
	}
	propagates := false
	if function == "RefreshAfterAccept" {
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			deferred, ok := node.(*ast.DeferStmt)
			if !ok {
				return true
			}
			ast.Inspect(deferred.Call, func(node ast.Node) bool {
				assign, ok := node.(*ast.AssignStmt)
				if ok && len(assign.Lhs) == 1 && len(assign.Rhs) == 1 && assign.Rhs[0] == report {
					id, ok := assign.Lhs[0].(*ast.Ident)
					propagates = ok && id.Name == "retErr"
				}
				return true
			})
			return false
		})
	} else {
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			ret, ok := node.(*ast.ReturnStmt)
			propagates = propagates || (ok && len(ret.Results) == 1 && ret.Results[0] == report)
			return true
		})
	}
	if !propagates {
		return fmt.Errorf("caller discards the combined reporting result")
	}
	if function != "runEditWithObservation" {
		return nil
	}
	if len(observations) != 1 || len(editors) != 1 || observations[0].Pos() >= editors[0].Pos() ||
		len(observations[0].Args) != 2 {
		return fmt.Errorf("P7 must freeze its context before invoking the editor")
	}
	identity, ok := observations[0].Args[1].(*ast.CompositeLit)
	if !ok {
		return fmt.Errorf("P7 requires an explicit observation identity")
	}
	fields := map[string]ast.Expr{}
	for _, entry := range identity.Elts {
		keyValue, ok := entry.(*ast.KeyValueExpr)
		if !ok {
			return fmt.Errorf("P7 identity must use named fields")
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			return fmt.Errorf("P7 identity key is not a field")
		}
		fields[key.Name] = keyValue.Value
	}
	for field, want := range map[string]string{"Producer": "patchobs.ProducerEdit", "RepoRoot": "s.Root"} {
		selector, ok := fields[field].(*ast.SelectorExpr)
		if !ok {
			return fmt.Errorf("P7 identity lacks %s", field)
		}
		base, ok := selector.X.(*ast.Ident)
		if !ok || base.Name+"."+selector.Sel.Name != want {
			return fmt.Errorf("P7 identity has the wrong %s", field)
		}
	}
	slug, ok := fields["Slug"].(*ast.Ident)
	if !ok || slug.Name != "slug" {
		return fmt.Errorf("P7 identity must retain the requested slug")
	}
	return nil
}

func TestRGAS5PrimaryAwareCallerWiringAndMutations(t *testing.T) {
	for _, caller := range []struct {
		path, function string
		causes         []string
	}{
		{"internal/cli/c1.go", "runEditWithObservation", []string{"editErr", "observationErr", "coverageErr"}},
		{"internal/workflow/implement.go", "RunImplement", []string{"stateErr", "provenanceErr", "coverageErr"}},
		{"internal/workflow/refresh.go", "RefreshAfterAccept", []string{"retErr", "coverageErr"}},
	} {
		t.Run(caller.function, func(t *testing.T) {
			src := rgaS0ReadRepoFile(t, caller.path)
			validate := func(source string) error { return rgaS5CallerClosureSource(source, caller.function, caller.causes) }
			if err := validate(src); err != nil {
				t.Fatal(err)
			}
			joined := "errors.Join(" + strings.Join(caller.causes, ", ") + ")"
			reversed := slices.Clone(caller.causes)
			slices.Reverse(reversed)
			for name, bad := range map[string]string{
				"primary-ignored-by-status": strings.Replace(src, joined, "coverageErr", 1),
				"primary-after-publication": strings.Replace(src, joined, "errors.Join("+strings.Join(reversed, ", ")+")", 1),
			} {
				t.Run(name, func(t *testing.T) {
					if bad == src || validate(bad) == nil {
						t.Fatal("the actual caller-wiring validator accepted its wrong-input mutation")
					}
				})
			}
			if caller.function == "runEditWithObservation" {
				bad := strings.Replace(src, "Producer: patchobs.ProducerEdit", "Producer: patchobs.ProducerRecord", 1)
				if bad == src || validate(bad) == nil {
					t.Fatal("P7 caller could bypass pre-editor reference validation")
				}
				start := strings.Index(src, "\tif observed {\n")
				editor := strings.Index(src, "\teditErr := openInEditor")
				if start < 0 || editor <= start {
					t.Fatal("could not locate the actual pre-editor observation block")
				}
				block := src[start:editor]
				without := src[:start] + src[editor:]
				end := start + strings.Index(without[start:], "\n") + 1
				late := without[:end] + block + without[end:]
				if validate(late) == nil {
					t.Fatal("P7 caller validator accepted reference reconstruction after the editor")
				}
			}
		})
	}
}

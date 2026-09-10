package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func rgaS5ReadFixture(t *testing.T) (*store.Store, RecipeCoverageInput) {
	t.Helper()
	s := setupVerifyFeature(t, "s5")
	if err := os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, s.Root, "add", "a.txt")
	mustGit(t, s.Root, "commit", "-qm", "reference")
	if err := os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatal(err)
	}
	obs := patchobs.Observe(patchobs.Input{RepoRoot: s.Root, Slug: "s5", Producer: patchobs.ProducerRecord,
		Patch: rgaS3ModifyPatch, PatchPresent: true, PreimageRef: "HEAD",
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll}})
	in := rgaS3Inputs(t, obs, rgaS3Write("a.txt", "old\n", "new\n", false))
	return s, in
}

func rgaS5ReadSnapshot(t *testing.T, in RecipeCoverageInput) RecipeCoverageSnapshot {
	t.Helper()
	e, c, bindings := rgaS5EventPair(t, in)
	raw, err := EncodeRecipeCaptureEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	return RecipeCoverageSnapshot{
		Coverage: RecipeArtifactRead{Exists: true, Bytes: c}, Event: RecipeArtifactRead{Exists: true, Bytes: raw},
		Patch:  RecipeArtifactRead{Exists: bindings.Patch.Present, Bytes: bindings.Patch.Bytes, Err: bindings.Patch.ReadError},
		Recipe: RecipeArtifactRead{Exists: bindings.Recipe.Present, Bytes: bindings.Recipe.Bytes, Err: bindings.Recipe.ReadError},
	}
}

func TestRGAS5ReadSixRungsAndSevenExecuteCases(t *testing.T) {
	s, complete := rgaS5ReadFixture(t)
	noRecipe := complete
	noRecipe.Recipe = CoverageArtifact{}
	badRecipe := complete
	badRecipe.Recipe = CoverageArtifact{Present: true, Bytes: []byte("{broken")}
	incomplete := complete
	incomplete.Events.StaleMarkerPresent = true
	status, err := s.LoadFeatureStatus("s5")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                          string
		in                            RecipeCoverageInput
		mutate                        func(*RecipeCoverageSnapshot)
		rung                          int
		refusal, warning, legacyError bool
	}{
		{"malformed", complete, func(s *RecipeCoverageSnapshot) { s.Coverage.Bytes = []byte("{}") }, 1, true, false, false},
		{"binding", complete, func(s *RecipeCoverageSnapshot) { s.Patch.Bytes = append(append([]byte{}, s.Patch.Bytes...), '\n') }, 2, true, false, false},
		{"no-recipe", noRecipe, nil, 3, true, false, false},
		{"undecodable", badRecipe, nil, 3, true, false, false},
		{"incomplete-decodable", incomplete, nil, 3, false, true, false},
		{"legacy-no-recipe", noRecipe, func(s *RecipeCoverageSnapshot) { s.Coverage = RecipeArtifactRead{} }, 5, false, false, true},
		{"legacy-recipe", complete, func(s *RecipeCoverageSnapshot) { s.Coverage = RecipeArtifactRead{} }, 5, false, false, false},
		{"complete", complete, nil, 6, false, false, false},
		{"current-marker", complete, func(s *RecipeCoverageSnapshot) { s.Marker.Exists = true }, 4, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := rgaS5ReadSnapshot(t, tc.in)
			if tc.mutate != nil {
				tc.mutate(&snap)
			}
			a := AssessRecipeCoverage(s.Root, "s5", snap, nil)
			if a.Rung != tc.rung {
				t.Fatalf("rung=%d: %s", a.Rung, a.explanation())
			}
			_, warning, refusal, err := RecipeExecutePreflight(s.Root, status, snap)
			if (refusal != "") != tc.refusal || (warning != "") != tc.warning || (err != nil) != tc.legacyError {
				t.Fatalf("warning=%q refusal=%q error=%v", warning, refusal, err)
			}
			if strings.Contains(warning+refusal, "old\n") || strings.Contains(warning+refusal, "new\n") {
				t.Fatal("diagnostic exposed source bodies")
			}
		})
	}
}

func TestRGAS5ReadPresenceOwnerAndCompanionFailures(t *testing.T) {
	s, in := rgaS5ReadFixture(t)
	for _, tc := range []struct {
		name, code string
		mutate     func(*RecipeCoverageSnapshot)
	}{
		{"unreadable-c", "recipe-coverage-malformed", func(s *RecipeCoverageSnapshot) { s.Coverage.Err = errors.New("injected EIO") }},
		{"absent-e", "recipe-coverage-capture-evidence-invalid", func(s *RecipeCoverageSnapshot) { s.Event = RecipeArtifactRead{} }},
		{"unreadable-e", "recipe-coverage-capture-evidence-invalid", func(s *RecipeCoverageSnapshot) { s.Event.Err = errors.New("injected EACCES") }},
		{"malformed-e", "recipe-coverage-capture-evidence-invalid", func(s *RecipeCoverageSnapshot) { s.Event.Bytes = []byte("{}") }},
		{"unreadable-r", "recipe-coverage-recipe-changed", func(s *RecipeCoverageSnapshot) { s.Recipe = RecipeArtifactRead{Exists: true, Err: errors.New("EIO")} }},
		{"missing-p", "recipe-coverage-patch-changed", func(s *RecipeCoverageSnapshot) { s.Patch = RecipeArtifactRead{} }},
		{"raw-r-space", "recipe-coverage-recipe-changed", func(s *RecipeCoverageSnapshot) { s.Recipe.Bytes = append([]byte(" "), s.Recipe.Bytes...) }},
		{"raw-c-space", "recipe-coverage-capture-evidence-invalid", func(s *RecipeCoverageSnapshot) { s.Coverage.Bytes = append([]byte(" "), s.Coverage.Bytes...) }},
		{"orphan-e", "recipe-coverage-missing", func(s *RecipeCoverageSnapshot) { s.Coverage = RecipeArtifactRead{}; s.Event.Err = errors.New("EIO") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := rgaS5ReadSnapshot(t, in)
			if a := AssessRecipeCoverage(s.Root, "s5", snap, nil); a.Rung != 6 {
				t.Fatal(a.explanation())
			}
			tc.mutate(&snap)
			if a := AssessRecipeCoverage(s.Root, "s5", snap, nil); a.Code != tc.code {
				t.Fatalf("%s: %s", a.Code, a.explanation())
			}
		})
	}
	snap := rgaS5ReadSnapshot(t, in)
	if a := AssessRecipeCoverage(s.Root, "other-owner", snap, nil); a.Code != "recipe-coverage-owner-mismatch" {
		t.Fatal(a.explanation())
	}
	recipe, err := decodeCoverageRecipe(in.Recipe.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	recipe.Feature = "other-owner"
	in.Recipe.Bytes, err = EncodeRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	a := AssessRecipeCoverage(s.Root, "s5", rgaS5ReadSnapshot(t, in), nil)
	if a.Rung != 3 || !strings.Contains(strings.Join(a.Reasons, ","), "recipe-owner-mismatch") {
		t.Fatal(a.explanation())
	}
}

func TestRGAS5ReadRejectsWrongReferenceTypeAndLostObject(t *testing.T) {
	s, in := rgaS5ReadFixture(t)
	good := rgaS5ReadSnapshot(t, in)
	wrong := in
	wrong.Observation.Reference.Commit = strings.TrimSpace(mustGit(t, s.Root, "rev-parse", "HEAD^{tree}"))
	if a := AssessRecipeCoverage(s.Root, "s5", rgaS5ReadSnapshot(t, wrong), nil); a.Code != "recipe-coverage-reference-stale" {
		t.Fatal("tree object masqueraded as commit: " + a.explanation())
	}
	blob := strings.TrimSpace(mustGit(t, s.Root, "rev-parse", "HEAD:a.txt"))
	if err := os.Remove(filepath.Join(s.Root, ".git", "objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	if a := AssessRecipeCoverage(s.Root, "s5", good, nil); a.Code != "recipe-coverage-reference-stale" {
		t.Fatal("lost reference blob became absence or limited proof: " + a.explanation())
	}
}

func TestRGAS5ReadLimitedBinaryStillChecksMixedText(t *testing.T) {
	s, _ := rgaS5ReadFixture(t)
	if err := os.WriteFile(filepath.Join(s.Root, "b.bin"), []byte("old\x00binary"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, s.Root, "add", "b.bin")
	mustGit(t, s.Root, "commit", "-qm", "binary reference")
	if err := os.WriteFile(filepath.Join(s.Root, "b.bin"), []byte("new\x00binary"), 0644); err != nil {
		t.Fatal(err)
	}
	patch := rgaS3ModifyPatch + "diff --git a/b.bin b/b.bin\nindex 123..456 100644\nBinary files a/b.bin and b/b.bin differ\n"
	obs := patchobs.Observe(patchobs.Input{Producer: patchobs.ProducerRecord, RepoRoot: s.Root, Slug: "s5",
		Patch: patch, PatchPresent: true, PreimageRef: "HEAD",
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll}})
	in := rgaS3Inputs(t, obs, rgaS3Write("a.txt", "old\n", "new\n", false))
	a := AssessRecipeCoverage(s.Root, "s5", rgaS5ReadSnapshot(t, in), nil)
	if a.Rung != 3 || len(a.Limitations) == 0 || !strings.Contains(strings.Join(a.Reasons, ","), "effect-binary-unsupported") {
		t.Fatalf("legitimate observed binary was not limited incomplete: %s", a.explanation())
	}
	if !in.Observation.Effects[1].Effect.PostimageObserved {
		t.Fatal("fixture did not observe binary postimage")
	}
	// A consistent but false whole-body text claim in the SAME mixed pair
	// must fail independently of the binary limitation.
	in.Observation.Effects[0].Bytes.Postimage = []byte("false postimage\n")
	in.Observation.Effects[0].Effect.PostimageSHA256 = CoverageSHA256(in.Observation.Effects[0].Bytes.Postimage)
	in.Observation.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(in.Observation.Effects)
	a = AssessRecipeCoverage(s.Root, "s5", rgaS5ReadSnapshot(t, in), nil)
	if a.Code != "recipe-coverage-reference-stale" {
		t.Fatal("mixed text mismatch laundered through binary limitation: " + a.explanation())
	}
}

func TestRGAS5ReadHistoricalParentExclusionAndReadableRestore(t *testing.T) {
	s, _ := rgaS5ReadFixture(t)
	path := filepath.Join(s.Root, "parent.txt")
	if err := os.WriteFile(path, []byte("new\n"), 0644); err != nil {
		t.Fatal(err)
	}
	patch := strings.ReplaceAll(rgaS3AddPatch, "a.txt", "parent.txt")
	obs := patchobs.Observe(patchobs.Input{Producer: patchobs.ProducerRecord, RepoRoot: s.Root, Slug: "s5",
		Patch: patch, PatchPresent: true, PreimageRef: "HEAD", ParentCreatedPaths: []string{"parent.txt"},
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll}})
	in := rgaS3Inputs(t, obs, rgaS3Write("parent.txt", "", "new\n", true))
	a := AssessRecipeCoverage(s.Root, "s5", rgaS5ReadSnapshot(t, in), nil)
	if a.Rung != 3 || !strings.Contains(strings.Join(a.Reasons, ","), "parent-created-target-unsupported") {
		t.Fatal("historical parent exclusion erased: " + a.explanation())
	}
	in.Recipe = CoverageArtifact{ReadError: errors.New("portable unreadable recipe")}
	snap := rgaS5ReadSnapshot(t, in)
	snap.Recipe.Exists = true
	status, err := s.LoadFeatureStatus("s5")
	if err != nil {
		t.Fatal(err)
	}
	_, _, refusal, err := RecipeExecutePreflight(s.Root, status, snap)
	if err != nil || !strings.Contains(refusal, "unreadable: portable unreadable recipe") || !strings.Contains(refusal, "recipe-generation-incomplete") {
		t.Fatalf("truthful false/unreadable presence misclassified: %s %v", refusal, err)
	}
	snap.Recipe = RecipeArtifactRead{Exists: true, Bytes: []byte("{}")}
	if a := AssessRecipeCoverage(s.Root, "s5", snap, nil); a.Code != "recipe-coverage-recipe-changed" {
		t.Fatal(a.explanation())
	}
}

func TestRGAS5ReadHistoricalProofAndArtifactSeam(t *testing.T) {
	s, in := rgaS5ReadFixture(t)
	snap := rgaS5ReadSnapshot(t, in)
	if err := os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("unrelated worktree drift\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if a := AssessRecipeCoverage(s.Root, "s5", snap, nil); a.Rung != 6 {
		t.Fatal(a.explanation())
	}
	denied := errors.New("below Git floor")
	if a := AssessRecipeCoverage(s.Root, "s5", snap, func() error { return denied }); a.Rung != 2 || !strings.Contains(a.Detail, denied.Error()) {
		t.Fatal(a.explanation())
	}
	in.Observation.Reference.Commit = strings.Repeat("a", 40)
	if a := AssessRecipeCoverage(s.Root, "s5", rgaS5ReadSnapshot(t, in), nil); a.Code != "recipe-coverage-reference-stale" {
		t.Fatal(a.explanation())
	}
	original := ReadRecipeArtifact
	t.Cleanup(func() { ReadRecipeArtifact = original })
	counts := map[string]int{}
	ReadRecipeArtifact = func(path string) RecipeArtifactRead {
		counts[filepath.Base(path)]++
		if filepath.Base(path) == "recipe-coverage.json" {
			return RecipeArtifactRead{Exists: true, Err: errors.New("portable injected EIO")}
		}
		return RecipeArtifactRead{}
	}
	captured := SnapshotRecipeCoverage(s.Root, "s5")
	a := AssessRecipeCoverage(s.Root, "s5", captured, nil)
	if a.Rung != 1 || !strings.Contains(a.Detail, "portable injected EIO") {
		t.Fatal(a.explanation())
	}
	for name, count := range counts {
		if count != 1 {
			t.Fatalf("%s read %d times", name, count)
		}
	}
	path := filepath.Join(s.Root, "dangling")
	if err := os.Symlink("missing-target", path); err == nil {
		got := readRecipeArtifact(path)
		if !got.Exists || got.Err == nil {
			t.Fatal("dangling entry collapsed to absence")
		}
	}
}

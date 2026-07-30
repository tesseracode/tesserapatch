package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// slice3Feature seeds a feature with an override RequestedAt so
// later-touch ordering can be tested deterministically. Bypasses
// AddFeature's `now` stamp by rewriting status.json after creation.
func slice3Feature(t *testing.T, s *store.Store, slug, requestedAt string) {
	t.Helper()
	_, err := s.AddFeature(store.AddFeatureInput{Title: slug, Request: slug, Slug: slug})
	if err != nil {
		t.Fatalf("AddFeature %s: %v", slug, err)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus %s: %v", slug, err)
	}
	st.RequestedAt = requestedAt
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatalf("SaveFeatureStatus %s: %v", slug, err)
	}
}

// slice3WriteRecipe writes an apply-recipe.json for `slug` with the
// provided operations. Deliberately uses raw json.Marshal so tests can
// omit or include the `preimage_hash` field as needed.
func slice3WriteRecipe(t *testing.T, s *store.Store, slug string, ops []RecipeOperation) {
	t.Helper()
	body := ApplyRecipe{Feature: slug, Operations: ops}
	// Marshal via the same struct the CLI uses.
	if err := os.MkdirAll(filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(slug, "apply-recipe.json", jsonMustMarshal(t, body)); err != nil {
		t.Fatalf("WriteArtifact recipe %s: %v", slug, err)
	}
}

func jsonMustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestSlice3_LaterTouchDetectsAndRefuses — PRD-write-file-recipe-safety
// §4.2 later-touch detection: an older feature's write-file targeting a
// path that a LATER feature already touched refuses with an actionable
// message naming the later feature. Wave β Slice 3 makes this apply-
// time refusal-class per the Wave β dispatch (tighter than ADR-029 D6's
// warn baseline).
func TestSlice3_LaterTouchDetectsAndRefuses(t *testing.T) {
	s := slice2Store(t)

	// Two features. `older` has an older RequestedAt than `newer`.
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	slice3Feature(t, s, "newer", "2026-02-01T00:00:00Z")

	// Newer feature has a recipe that already touched src/a.txt.
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "newer's snapshot\n"},
	})

	// Set up disk state matching newer's outcome (i.e. src/a.txt exists
	// with newer's bytes). The older feature's recipe below claims a
	// preimage_hash that matches the current bytes — so the preimage
	// gate itself is satisfied, and only later-touch protects the fix.
	current := []byte("newer's snapshot\n")
	writeRepoFile(t, s, "src/a.txt", current)

	olderRecipe := ApplyRecipe{
		Feature: "older",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf(current)), Content: "older's old snapshot\n"},
		},
	}
	result := ExecuteRecipe(s, olderRecipe)
	if result.Success {
		t.Fatalf("expected later-touch refusal even with matching preimage; got success")
	}
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "later feature") {
		t.Errorf("error should name 'later feature'; got: %s", joined)
	}
	if !strings.Contains(joined, "newer") {
		t.Errorf("error should name the later feature slug 'newer'; got: %s", joined)
	}
	if !strings.Contains(joined, "silently revert") {
		t.Errorf("error should mention 'silently revert' (PRD §1 class); got: %s", joined)
	}
	// D3 all-or-nothing: file was not modified despite matching preimage.
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != string(current) {
		t.Errorf("D3 all-or-nothing violated: file mutated after later-touch reject; got %q", got)
	}
}

// TestSlice3_EarlierTouchNotLater confirms the ordering predicate: if
// another feature touched the path EARLIER, no drift is reported. Only
// later features trigger the gate.
func TestSlice3_EarlierTouchNotLater(t *testing.T) {
	s := slice2Store(t)
	slice3Feature(t, s, "earlier", "2026-01-01T00:00:00Z")
	slice3Feature(t, s, "current", "2026-06-01T00:00:00Z")

	// Earlier feature's recipe touched src/a.txt.
	slice3WriteRecipe(t, s, "earlier", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "e\n"},
	})

	writeRepoFile(t, s, "src/a.txt", []byte("existing\n"))
	rec := ApplyRecipe{
		Feature: "current",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("existing\n"))), Content: "n\n"},
		},
	}
	result := ExecuteRecipe(s, rec)
	if !result.Success {
		t.Fatalf("no later-touch expected when other feature is earlier; got errors: %v", result.Errors)
	}
}

// TestSlice3_UnrelatedPathIgnored confirms path scoping: a later
// feature that touched a DIFFERENT path does not trigger the gate.
func TestSlice3_UnrelatedPathIgnored(t *testing.T) {
	s := slice2Store(t)
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	slice3Feature(t, s, "newer", "2026-02-01T00:00:00Z")

	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/other.txt", PreimageHash: ptr(""), Content: "z"},
	})

	writeRepoFile(t, s, "src/a.txt", []byte("hello\n"))
	rec := ApplyRecipe{
		Feature: "older",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("hello\n"))), Content: "goodbye\n"},
		},
	}
	result := ExecuteRecipe(s, rec)
	if !result.Success {
		t.Fatalf("no later-touch expected on unrelated path; got errors: %v", result.Errors)
	}
}

// TestSlice3_MissingRequestedAtSkipsCheck — if the current feature has
// no RequestedAt (legacy status.json), we cannot order features and
// the later-touch gate is skipped. The preimage gate still applies.
func TestSlice3_MissingRequestedAtSkipsCheck(t *testing.T) {
	s := slice2Store(t)
	// Both features share slugs, but current has empty RequestedAt.
	_, err := s.AddFeature(store.AddFeatureInput{Title: "current", Slug: "current"})
	if err != nil {
		t.Fatal(err)
	}
	st, _ := s.LoadFeatureStatus("current")
	st.RequestedAt = ""
	s.SaveFeatureStatus(st)
	slice3Feature(t, s, "peer", "2030-01-01T00:00:00Z")
	slice3WriteRecipe(t, s, "peer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "z"},
	})
	writeRepoFile(t, s, "src/a.txt", []byte("hello\n"))
	rec := ApplyRecipe{
		Feature: "current",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("hello\n"))), Content: "n"},
		},
	}
	result := ExecuteRecipe(s, rec)
	if !result.Success {
		t.Fatalf("ordering-indeterminate should skip later-touch (best-effort); got errors: %v", result.Errors)
	}
}

// TestSlice3_DeterministicSlugSelection locks in PRD §5 note 4:
// "Sort path warnings by path then feature slug for deterministic
// output." When multiple later features touch the same path, the
// error message must name a deterministic slug (alphabetically first).
func TestSlice3_DeterministicSlugSelection(t *testing.T) {
	s := slice2Store(t)
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	slice3Feature(t, s, "later-b", "2026-02-01T00:00:00Z")
	slice3Feature(t, s, "later-a", "2026-03-01T00:00:00Z")

	// Both later features touched src/a.txt.
	for _, sl := range []string{"later-a", "later-b"} {
		slice3WriteRecipe(t, s, sl, []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "x"},
		})
	}

	writeRepoFile(t, s, "src/a.txt", []byte("v0\n"))
	rec := ApplyRecipe{
		Feature: "older",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("v0\n"))), Content: "revert\n"},
		},
	}
	// ListFeatures sorts by slug so `later-a` appears before `later-b`
	// and is picked first for the reported later-slug.
	result := ExecuteRecipe(s, rec)
	if result.Success {
		t.Fatalf("expected later-touch failure")
	}
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "later-a") {
		t.Errorf("expected deterministic pick of alphabetically-first slug 'later-a'; got: %s", joined)
	}
}

// TestSlice3_UsesPatchGenerationsTouchedPaths verifies the PRD §4.2
// "prefer deterministic artifacts" contract: if a later feature has a
// patch-generations manifest with touched_paths, those paths ALSO gate
// the current feature's write-file (not just recipe op paths). This
// covers the audit-trail case where a feature has already been
// recorded and its raw recipe has been superseded by the manifest.
func TestSlice3_UsesPatchGenerationsTouchedPaths(t *testing.T) {
	s := slice2Store(t)
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	slice3Feature(t, s, "recorded-newer", "2026-02-01T00:00:00Z")

	// recorded-newer has NO apply-recipe.json — only a manifest with
	// touched_paths. Only the patch-generations source can flag it.
	manifest := store.PatchGenerationsManifest{
		Version:           store.PatchGenerationsManifestVersion,
		Feature:           "recorded-newer",
		CurrentGeneration: 1,
		Generations: []store.PatchGeneration{{
			Generation:          1,
			GenerationID:        "pg_abcdef012345",
			Kind:                "record",
			PatchSHA256:         "p",
			GitPatchID:          "g",
			GitPatchIDAlgorithm: store.PatchIDAlgorithmStable,
			RecipeSHA256:        "r",
			CanonicalPatch:      "artifacts/post-apply.patch",
			AuditPatch:          "patches/001-record.patch",
			BaseCommit:          "b",
			Upper: store.GenerationUpper{
				Kind:   "working-tree",
				Ref:    "working-tree",
				Commit: "",
			},
			Capture: store.GenerationCapture{
				Mode:      "working-tree-all",
				Pathspecs: []string{},
				ClaimIDs:  []string{},
			},
			TouchedPaths: []string{"src/a.txt"},
			Dependencies: []store.GenerationDependency{},
			Refs:         &store.GenerationRefs{},
		}},
	}
	if err := store.SavePatchGenerations(s, manifest); err != nil {
		t.Fatalf("SavePatchGenerations: %v", err)
	}

	writeRepoFile(t, s, "src/a.txt", []byte("v0\n"))
	rec := ApplyRecipe{
		Feature: "older",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("v0\n"))), Content: "revert\n"},
		},
	}
	result := ExecuteRecipe(s, rec)
	if result.Success {
		t.Fatalf("expected later-touch detection via patch-generations.touched_paths")
	}
	if !strings.Contains(strings.Join(result.Errors, "\n"), "recorded-newer") {
		t.Errorf("expected error to name 'recorded-newer'; got: %v", result.Errors)
	}
}

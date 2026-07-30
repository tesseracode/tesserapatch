package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// slice4SeedSupersession creates two features: `historical` (older,
// state=applied, RequestedAt=old) and `newer` (later, state=applied,
// RequestedAt=new). `newer` declares a `supersedes → historical` edge,
// so `IsFeatureSuperseded("historical")` returns ("newer", true).
//
// State=applied is one of Wave α's supersederHealthyStates, so `newer`
// is a HEALTHY superseder in the base scenario. Callers wanting a
// STALE-superseder scenario mutate `newer`'s state afterward.
func slice4SeedSupersession(t *testing.T, s *store.Store) {
	t.Helper()
	// Order matters: newer must have a later RequestedAt than
	// historical so Slice 3 later-touch detection also fires.
	slice3Feature(t, s, "historical", "2026-01-01T00:00:00Z")
	slice3Feature(t, s, "newer", "2026-06-01T00:00:00Z")

	// Both features start applied so the healthy check passes.
	for _, sl := range []string{"historical", "newer"} {
		st, _ := s.LoadFeatureStatus(sl)
		st.State = store.StateApplied
		s.SaveFeatureStatus(st)
	}

	// newer supersedes historical.
	st, _ := s.LoadFeatureStatus("newer")
	st.DependsOn = []store.Dependency{
		{Slug: "historical", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatalf("save newer with supersedes edge: %v", err)
	}
}

// TestSlice4_SupersededPreimageMismatchDowngrades — PRD-feature-supersession §4.5
// clause 1: when the current feature is superseded by an active healthy
// superseder, preimage mismatch drift downgrades from Error (Slice 2)
// to Warning-with-note. Execution proceeds despite the drift.
func TestSlice4_SupersededPreimageMismatchDowngrades(t *testing.T) {
	s := slice2Store(t)
	slice4SeedSupersession(t, s)

	writeRepoFile(t, s, "src/a.txt", []byte("current bytes\n"))

	// historical (superseded) recipe has a stale preimage_hash — under
	// Slice 2 alone this would refuse execution.
	recipe := ApplyRecipe{
		Feature: "historical",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("STALE"))), Content: "historical snapshot\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if !result.Success {
		t.Fatalf("superseded feature should downgrade preimage drift; got errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected downgrade warning; got none")
	}
	joined := strings.Join(result.Warnings, "\n")
	if !strings.Contains(joined, "downgraded") {
		t.Errorf("warning should mark drift as 'downgraded'; got: %s", joined)
	}
	if !strings.Contains(joined, "superseded by \"newer\"") {
		t.Errorf("warning should name the superseder; got: %s", joined)
	}
	if !strings.Contains(joined, "PRD-feature-supersession §4.5") {
		t.Errorf("warning should cite PRD-feature-supersession §4.5 / ADR-029 D7 provenance; got: %s", joined)
	}
	// Execution DID proceed: file rewritten to historical's snapshot.
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != "historical snapshot\n" {
		t.Errorf("superseded downgrade should still execute the write; got %q", got)
	}
}

// TestSlice4_SupersededLaterTouchDowngrades — PRD-feature-supersession §4.5
// clause 1: later-touch drift on a superseded historical feature is
// warning-class, not refusal. The check STILL RUNS (drift is reported),
// only the severity flips.
func TestSlice4_SupersededLaterTouchDowngrades(t *testing.T) {
	s := slice2Store(t)
	slice4SeedSupersession(t, s)

	// newer's recipe touches src/a.txt so historical's write-file on
	// the same path triggers later-touch.
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "newer snapshot\n"},
	})

	pre := []byte("current bytes\n")
	writeRepoFile(t, s, "src/a.txt", pre)

	// historical's recipe: matching preimage (no Slice 2 drift), so the
	// only drift is Slice 3 later-touch.
	recipe := ApplyRecipe{
		Feature: "historical",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf(pre)), Content: "historical snapshot\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if !result.Success {
		t.Fatalf("superseded feature should downgrade later-touch drift; got errors: %v", result.Errors)
	}
	joined := strings.Join(result.Warnings, "\n")
	if !strings.Contains(joined, "later feature \"newer\"") {
		t.Errorf("later-touch signal should still be reported (just downgraded); got warnings: %s", joined)
	}
	if !strings.Contains(joined, "downgraded") {
		t.Errorf("warning should mark drift as 'downgraded'; got: %s", joined)
	}
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != "historical snapshot\n" {
		t.Errorf("superseded downgrade should still execute; got %q", got)
	}
}

// TestSlice4_NotSupersededHardRejectsRetained locks in the negative
// direction: when NO supersession edge exists, drift remains hard-
// reject (Slice 2/3 baseline). This is the primary safety guarantee
// against silent reverts — supersession-downgrade is a controlled
// exception.
func TestSlice4_NotSupersededHardRejectsRetained(t *testing.T) {
	s := slice2Store(t)
	// Two features but NO supersession edge.
	slice3Feature(t, s, "historical", "2026-01-01T00:00:00Z")
	slice3Feature(t, s, "unrelated", "2026-06-01T00:00:00Z")

	writeRepoFile(t, s, "src/a.txt", []byte("current bytes\n"))

	recipe := ApplyRecipe{
		Feature: "historical",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("STALE"))), Content: "n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if result.Success {
		t.Fatalf("no supersession edge → drift must remain hard-reject")
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected Errors entry")
	}
	// D3 all-or-nothing: file unchanged.
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != "current bytes\n" {
		t.Errorf("D3 violated; got %q", got)
	}
}

// TestSlice4_StaleSupersederStillDowngrades locks in the Wave α R4
// runtime flip: supersession excludes the historical from the default
// replay set whether the superseder is healthy OR stale (per PRD §4.5.3
// clause 3 + Wave α acceptance). Wave β inherits that semantics: when
// the ONLY superseder is stale (state=requested for example), the
// historical STILL has drift downgraded. Rationale: the graph reports
// the stale-superseder problem separately (Wave α `stale-superseder`
// label); the historical's drift should still not silently block
// audit paths.
func TestSlice4_StaleSupersederStillDowngrades(t *testing.T) {
	s := slice2Store(t)
	slice4SeedSupersession(t, s)

	// Flip newer to a NON-healthy state so it becomes stale.
	// StateRequested is not in supersederHealthyStates.
	st, _ := s.LoadFeatureStatus("newer")
	st.State = store.StateRequested
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}

	writeRepoFile(t, s, "src/a.txt", []byte("current bytes\n"))
	recipe := ApplyRecipe{
		Feature: "historical",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("STALE"))), Content: "historical snapshot\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if !result.Success {
		t.Fatalf("stale-superseder should STILL trigger the Wave α R4 runtime flip and downgrade drift; got errors: %v", result.Errors)
	}
	joined := strings.Join(result.Warnings, "\n")
	if !strings.Contains(joined, "downgraded") {
		t.Errorf("stale-superseder path should still emit downgrade warning; got: %s", joined)
	}
	if !strings.Contains(joined, "superseded by \"newer\"") {
		t.Errorf("stale-superseder still named as superseder; got: %s", joined)
	}
}

// TestSlice4_SupersederFeatureItselfHardRejects — the ACTIVE superseder
// is NOT superseded by anyone, so its own drift remains hard-reject.
// Slice 4 must not accidentally suppress drift on the wrong side of
// the edge (per ADR-029 D7 "The active superseder remains subject to
// normal refusal/failure semantics").
func TestSlice4_SupersederFeatureItselfHardRejects(t *testing.T) {
	s := slice2Store(t)
	slice4SeedSupersession(t, s)

	writeRepoFile(t, s, "src/a.txt", []byte("current bytes\n"))

	// newer's recipe on the same path with stale preimage — since
	// newer supersedes historical (not the other way around), newer
	// itself is NOT superseded and drift stays hard-reject.
	recipe := ApplyRecipe{
		Feature: "newer",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("STALE"))), Content: "n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if result.Success {
		t.Fatalf("active superseder's own drift must NOT downgrade; got success")
	}
}

// TestSlice4_SupersededPathSafetyStillHardReject locks in the safety-
// boundary invariant from runWriteFilePreimagePrecheck: the ONLY
// unconditional reject is a path safety violation (../ escape). Even
// for a superseded feature, path-safety violations must NEVER downgrade
// — that would be a security regression.
func TestSlice4_SupersededPathSafetyStillHardReject(t *testing.T) {
	s := slice2Store(t)
	slice4SeedSupersession(t, s)

	recipe := ApplyRecipe{
		Feature: "historical",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "../escape.txt", PreimageHash: ptr(""), Content: "pwn"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if result.Success {
		t.Fatalf("path safety must NEVER downgrade; got success")
	}
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "path safety") {
		t.Errorf("expected 'path safety' in errors; got: %s", joined)
	}
}

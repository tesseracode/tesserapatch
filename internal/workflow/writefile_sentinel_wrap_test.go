package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// v0.12.0 Wave β rev-1 Slice R5 (F-M1 fix) — sentinel-error wrap-and-
// return contract. The docstring on ErrWriteFilePreimageMismatch and
// ErrWriteFileLaterTouch promises that `errors.Is` matches at every
// refusal / drift-warn callsite; the rev-0 shipped code declared them
// but never returned wrapped instances, so the promise was hollow.
// These regression tests lock the wire-up so future changes can't
// silently drop it again.

// TestSliceR5_PreimageMismatchWrappedForEffective — the effective-
// feature preimage-mismatch drift path (write-file with mismatched
// on-disk hash, no supersession) MUST populate WrappedErrors with a
// sentinel-wrapped entry that errors.Is matches on
// ErrWriteFilePreimageMismatch.
func TestSliceR5_PreimageMismatchWrappedForEffective(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "src/a.txt", []byte("current\n"))
	sum := sha256.Sum256([]byte("stale\n"))
	stale := PreimageHashPrefix + hex.EncodeToString(sum[:])
	slice3Feature(t, s, "demo", "2026-01-01T00:00:00Z")
	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(stale), Content: "next\n"},
		},
	}
	res := runWriteFilePreimagePrecheck(s, recipe)

	if len(res.Errors) == 0 {
		t.Fatalf("expected effective preimage-mismatch to populate Errors")
	}
	if len(res.WrappedErrors) != len(res.Errors) {
		t.Fatalf("WrappedErrors must pair 1:1 with preimage Errors; got %d wrapped for %d string errors",
			len(res.WrappedErrors), len(res.Errors))
	}
	matched := false
	for _, werr := range res.WrappedErrors {
		if errors.Is(werr, ErrWriteFilePreimageMismatch) {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("no WrappedErrors entry unwrapped to ErrWriteFilePreimageMismatch; got %v", res.WrappedErrors)
	}
	// Cross-sentinel check: the preimage-drift error MUST NOT match
	// the later-touch sentinel.
	for _, werr := range res.WrappedErrors {
		if errors.Is(werr, ErrWriteFileLaterTouch) {
			t.Errorf("preimage-drift wrapped error should NOT match ErrWriteFileLaterTouch; entry: %v", werr)
		}
	}
}

// TestSliceR5_LaterTouchWrappedForEffective — the effective-feature
// later-touch warn path (Slice 3 detector fires, no supersession) MUST
// populate WrappedWarnings with a sentinel-wrapped entry that
// errors.Is matches on ErrWriteFileLaterTouch.
func TestSliceR5_LaterTouchWrappedForEffective(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "src/a.txt", []byte("current\n"))
	// older is the one applying — same as TestSlice3_LaterTouchWarnsAndProceeds.
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	slice3Feature(t, s, "newer", "2026-06-01T00:00:00Z")
	// newer touches src/a.txt so the later-touch detector fires when
	// older's recipe is prechecked.
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("current\n"))), Content: "n\n"},
	})
	recipe := ApplyRecipe{
		Feature: "older",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("current\n"))), Content: "older-writes\n"},
		},
	}
	res := runWriteFilePreimagePrecheck(s, recipe)

	if len(res.Warnings) == 0 {
		t.Fatalf("expected later-touch to populate Warnings; got none. Errors=%v", res.Errors)
	}
	if len(res.WrappedWarnings) == 0 {
		t.Fatalf("expected WrappedWarnings to be populated alongside Warnings")
	}
	matched := false
	for _, werr := range res.WrappedWarnings {
		if errors.Is(werr, ErrWriteFileLaterTouch) {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("no WrappedWarnings entry unwrapped to ErrWriteFileLaterTouch; got %v", res.WrappedWarnings)
	}
	// And the later-touch wrapped-warn MUST NOT accidentally match
	// the preimage sentinel.
	for _, werr := range res.WrappedWarnings {
		if errors.Is(werr, ErrWriteFilePreimageMismatch) {
			t.Errorf("later-touch wrapped warning should NOT match ErrWriteFilePreimageMismatch; entry: %v", werr)
		}
	}
	// Execution proceeds — no drift-class error was emitted.
	if len(res.Errors) != 0 || len(res.WrappedErrors) != 0 {
		t.Errorf("later-touch is warn-class per D6; must not emit Errors. Got Errors=%v WrappedErrors=%v",
			res.Errors, res.WrappedErrors)
	}
}

// TestSliceR5_SupersededPreimageDriftWrappedInWarnings — Slice 4
// downgrade path: a superseded feature with a stale preimage lands
// its drift in Warnings (not Errors), but the underlying drift class
// is STILL preimage-mismatch and the wrapped warn entry MUST match
// ErrWriteFilePreimageMismatch via errors.Is. This mirrors the
// downgrade semantics: severity is warn, class is unchanged.
func TestSliceR5_SupersededPreimageDriftWrappedInWarnings(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "src/a.txt", []byte("current\n"))
	sum := sha256.Sum256([]byte("stale\n"))
	stale := PreimageHashPrefix + hex.EncodeToString(sum[:])
	slice3Feature(t, s, "target", "2026-01-01T00:00:00Z")
	// Add a healthy superseder that declares supersedes on target.
	slice3Feature(t, s, "superseder", "2026-06-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("superseder")
	st.State = store.StateApplied
	st.DependsOn = []store.Dependency{{Slug: "target", Kind: store.DependencyKindSupersedes}}
	s.SaveFeatureStatus(st)
	recipe := ApplyRecipe{
		Feature: "target",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(stale), Content: "n\n"},
		},
	}
	res := runWriteFilePreimagePrecheck(s, recipe)

	// Downgrade path: drift lands in Warnings, not Errors.
	if len(res.Errors) != 0 {
		t.Errorf("superseded feature drift should NOT populate Errors; got %v", res.Errors)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("expected downgraded drift in Warnings; got none")
	}
	if len(res.WrappedWarnings) == 0 {
		t.Fatalf("expected WrappedWarnings to carry the downgraded drift")
	}
	matched := false
	for _, werr := range res.WrappedWarnings {
		if errors.Is(werr, ErrWriteFilePreimageMismatch) {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("no WrappedWarnings entry unwrapped to ErrWriteFilePreimageMismatch (downgrade should preserve class); got %v", res.WrappedWarnings)
	}
}

// TestSliceR5_SentinelIdentityDistinctness — the two sentinels are
// distinct error values. errors.Is(A, B) must be false in both
// directions so downstream callers can safely branch on them.
func TestSliceR5_SentinelIdentityDistinctness(t *testing.T) {
	if errors.Is(ErrWriteFilePreimageMismatch, ErrWriteFileLaterTouch) {
		t.Error("ErrWriteFilePreimageMismatch must not match ErrWriteFileLaterTouch under errors.Is")
	}
	if errors.Is(ErrWriteFileLaterTouch, ErrWriteFilePreimageMismatch) {
		t.Error("ErrWriteFileLaterTouch must not match ErrWriteFilePreimageMismatch under errors.Is")
	}
	// Reflexive: each must match itself.
	if !errors.Is(ErrWriteFilePreimageMismatch, ErrWriteFilePreimageMismatch) {
		t.Error("ErrWriteFilePreimageMismatch sentinel identity broken")
	}
	if !errors.Is(ErrWriteFileLaterTouch, ErrWriteFileLaterTouch) {
		t.Error("ErrWriteFileLaterTouch sentinel identity broken")
	}
}

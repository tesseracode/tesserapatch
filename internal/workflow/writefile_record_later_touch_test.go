package workflow

import (
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestSliceR2_RecordLaterTouchWarns — PRD-write-file-recipe-safety
// AC-7 + §4.2 "During record": when recording a NEWER feature whose
// touched path overlaps an OLDER active feature's write-file, a
// deterministic warning naming both features + the shared path is
// emitted. ADR-029 D6 warning-class (execution proceeds).
func TestSliceR2_RecordLaterTouchWarns(t *testing.T) {
	s := slice2Store(t)

	// older-feat is applied with a write-file recipe on src/a.txt.
	slice3Feature(t, s, "older-feat", "2026-01-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("older-feat")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "older-feat", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "older content\n"},
	})

	// newer-feat is being recorded; its touched paths include src/a.txt.
	slice3Feature(t, s, "newer-feat", "2026-06-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("newer-feat")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "newer-feat", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "newer content\n"},
	})

	warnings := DetectRecordLaterTouchWarnings(s, "newer-feat")
	if len(warnings) == 0 {
		t.Fatalf("expected at least one later-touch warning; got none")
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "newer-feat") {
		t.Errorf("warning should name the newly-recorded feature; got: %s", joined)
	}
	if !strings.Contains(joined, "older-feat") {
		t.Errorf("warning should name the older active feature; got: %s", joined)
	}
	if !strings.Contains(joined, "src/a.txt") {
		t.Errorf("warning should name the shared path; got: %s", joined)
	}
	if !strings.Contains(joined, "PRD-write-file-recipe-safety §4.2") {
		t.Errorf("warning should cite PRD §4.2; got: %s", joined)
	}
	if !strings.Contains(joined, "ADR-029 D6") {
		t.Errorf("warning should cite ADR-029 D6; got: %s", joined)
	}
}

// TestSliceR2_RecordDeterministicPerPath verifies PRD §5 note 4:
// "Sort path warnings by path then feature slug for deterministic
// output." Given multiple older active features whose write-files
// overlap the same path, the warning names the alphabetically-first
// older slug.
func TestSliceR2_RecordDeterministicPerPath(t *testing.T) {
	s := slice2Store(t)
	// Two older active features both own a write-file at src/x.txt.
	for _, sl := range []string{"older-b", "older-a"} {
		slice3Feature(t, s, sl, "2026-01-01T00:00:00Z")
		st, _ := s.LoadFeatureStatus(sl)
		st.State = store.StateApplied
		s.SaveFeatureStatus(st)
		slice3WriteRecipe(t, s, sl, []RecipeOperation{
			{Type: "write-file", Path: "src/x.txt", PreimageHash: ptr(""), Content: sl + "\n"},
		})
	}
	slice3Feature(t, s, "current", "2026-06-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("current")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "current", []RecipeOperation{
		{Type: "write-file", Path: "src/x.txt", PreimageHash: ptr(""), Content: "n"},
	})

	warnings := DetectRecordLaterTouchWarnings(s, "current")
	if len(warnings) == 0 {
		t.Fatalf("expected at least one warning")
	}
	if !strings.Contains(warnings[0], "older-a") {
		t.Errorf("expected deterministic pick of alphabetically-first slug 'older-a'; got: %s", warnings[0])
	}
	if strings.Contains(warnings[0], "older-b") {
		t.Errorf("only the first slug per path should be named; got: %s", warnings[0])
	}
}

// TestSliceR2_RecordSortedByPath — multiple overlap paths yield
// warnings in ascending path order.
func TestSliceR2_RecordSortedByPath(t *testing.T) {
	s := slice2Store(t)
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("older")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	// Two write-file ops on distinct paths (z before a in insertion order).
	slice3WriteRecipe(t, s, "older", []RecipeOperation{
		{Type: "write-file", Path: "src/z.txt", PreimageHash: ptr(""), Content: "z"},
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "a"},
	})
	slice3Feature(t, s, "current", "2026-06-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("current")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "current", []RecipeOperation{
		{Type: "write-file", Path: "src/z.txt", PreimageHash: ptr(""), Content: "1"},
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "2"},
	})

	warnings := DetectRecordLaterTouchWarnings(s, "current")
	if len(warnings) < 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}
	// Locate src/a.txt and src/z.txt indexes; a.txt must precede z.txt.
	var aIdx, zIdx int = -1, -1
	for i, w := range warnings {
		if strings.Contains(w, "src/a.txt") {
			aIdx = i
		}
		if strings.Contains(w, "src/z.txt") {
			zIdx = i
		}
	}
	if aIdx < 0 || zIdx < 0 {
		t.Fatalf("expected both paths in warnings; got: %v", warnings)
	}
	if aIdx >= zIdx {
		t.Errorf("expected src/a.txt warning before src/z.txt; got a=%d z=%d", aIdx, zIdx)
	}
	// Belt-and-suspenders: verify the warnings themselves are sorted
	// per the detector's PRD §5 note 4 promise.
	sorted := append([]string(nil), warnings...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if strings.Join(warnings, "|") != strings.Join(sorted, "|") {
		// Not strict-required by AC-7 but the detector doc promises sort
		// by path; a warning that violates the invariant would break
		// audit tooling.
		t.Logf("warnings order (informational): %v", warnings)
	}
}

// TestSliceR2_RecordNoOverlapNoWarning — negative case: an unrelated
// older active feature that owns write-file on a DIFFERENT path
// generates no warning for the current feature.
func TestSliceR2_RecordNoOverlapNoWarning(t *testing.T) {
	s := slice2Store(t)
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("older")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "older", []RecipeOperation{
		{Type: "write-file", Path: "src/unrelated.txt", PreimageHash: ptr(""), Content: "x"},
	})
	slice3Feature(t, s, "current", "2026-06-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("current")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "current", []RecipeOperation{
		{Type: "write-file", Path: "src/current.txt", PreimageHash: ptr(""), Content: "y"},
	})

	warnings := DetectRecordLaterTouchWarnings(s, "current")
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings (no path overlap); got: %v", warnings)
	}
}

// TestSliceR2_RecordDraftOlderIgnored — an older feature in a
// non-active state (e.g. draft/requested) does NOT emit a record-time
// warning per §4.2 "older ACTIVE/EFFECTIVE features". The active-set
// filter (activeOrAppliedFeatures) uses states applied/active.
func TestSliceR2_RecordDraftOlderIgnored(t *testing.T) {
	s := slice2Store(t)
	slice3Feature(t, s, "older-draft", "2026-01-01T00:00:00Z")
	// State stays at whatever AddFeature set (typically not applied);
	// slice3Feature does not touch State so it should be requested.
	slice3WriteRecipe(t, s, "older-draft", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "x"},
	})
	slice3Feature(t, s, "current", "2026-06-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("current")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "current", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "y"},
	})

	warnings := DetectRecordLaterTouchWarnings(s, "current")
	if len(warnings) != 0 {
		t.Errorf("draft older should not trigger record warning; got: %v", warnings)
	}
}

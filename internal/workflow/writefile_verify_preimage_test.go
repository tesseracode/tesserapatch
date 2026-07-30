package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// v0.12.0 Wave β rev-1 Slice R4 regression suite —
// PRD-write-file-recipe-safety AC-9 + §5:130 + ADR-029 D6/D7/D8.
//
// V10 (CheckWriteFilePreimageFresh) fails-block on stale write-file
// preimages for effective features, downgrades to warn for superseded
// features, and passes when preimages match on disk. It has no side
// effects on shadow state — the check reads on-disk files directly.

// sliceR4WritePreimage — helper: write a file to the repo root and
// return its sha256 in the ADR-029 D1 display form.
func sliceR4WritePreimage(t *testing.T, s *store.Store, relPath, body string) string {
	t.Helper()
	dir := filepath.Dir(filepath.Join(s.Root, relPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, relPath), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(body))
	return PreimageHashPrefix + hex.EncodeToString(sum[:])
}

// findCheck locates a V-check result by ID; fails the test if absent.
// (Local helper: verify_slice_c_test.go already defines a `findCheck`
// that takes a *VerifyReport, so this V10 file uses a distinct name
// to avoid the redeclaration.)
func findCheckR4(t *testing.T, checks []store.VerifyCheckResult, id string) store.VerifyCheckResult {
	t.Helper()
	for _, c := range checks {
		if c.ID == id {
			return c
		}
	}
	ids := make([]string, 0, len(checks))
	for _, c := range checks {
		ids = append(ids, c.ID)
	}
	t.Fatalf("check %q not found in report; got IDs %v", id, ids)
	return store.VerifyCheckResult{}
}

// TestSliceR4_PreimageFreshPassesWhenMatching — baseline: an effective
// feature whose recipe preimage matches the on-disk file passes V10.
func TestSliceR4_PreimageFreshPassesWhenMatching(t *testing.T) {
	slug := "fresh-ok"
	s := setupVerifyFeature(t, slug)
	writeIntentFiles(t, s, slug)
	hash := sliceR4WritePreimage(t, s, "src/a.txt", "current\n")
	recipe := ApplyRecipe{
		Feature:    slug,
		Operations: []RecipeOperation{{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hash), Content: "next\n"}},
	}
	writeVerifyRecipe(t, s, slug, recipe)

	report, err := RunVerify(s, slug, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerify: %v", err)
	}
	c := findCheckR4(t, report.Checks, CheckWriteFilePreimageFresh)
	if !c.Passed || c.Skipped {
		t.Errorf("expected V10 passed=true, skipped=false; got %+v", c)
	}
	if c.Severity != SeverityBlock {
		t.Errorf("expected V10 severity=block on effective feature; got %s", c.Severity)
	}
}

// TestSliceR4_PreimageFreshBlocksOnMismatchForEffective — AC-9 core:
// effective feature + stale preimage → V10 failure at block severity
// (PRD §7.2 "v1 blocks only on preimage mismatch"; ADR-029 D6).
func TestSliceR4_PreimageFreshBlocksOnMismatchForEffective(t *testing.T) {
	slug := "stale-effective"
	s := setupVerifyFeature(t, slug)
	writeIntentFiles(t, s, slug)
	// Compute a hash for "expected preimage" body, then write a
	// DIFFERENT body to disk to force mismatch.
	sum := sha256.Sum256([]byte("expected\n"))
	expected := PreimageHashPrefix + hex.EncodeToString(sum[:])
	os.MkdirAll(filepath.Join(s.Root, "src"), 0o755)
	if err := os.WriteFile(filepath.Join(s.Root, "src/a.txt"), []byte("drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recipe := ApplyRecipe{
		Feature:    slug,
		Operations: []RecipeOperation{{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(expected), Content: "next\n"}},
	}
	writeVerifyRecipe(t, s, slug, recipe)

	report, err := RunVerify(s, slug, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerify: %v", err)
	}
	c := findCheckR4(t, report.Checks, CheckWriteFilePreimageFresh)
	if c.Passed {
		t.Errorf("expected V10 failure on stale effective preimage; got %+v", c)
	}
	if c.Severity != SeverityBlock {
		t.Errorf("effective feature V10 severity must be block; got %s", c.Severity)
	}
	if !strings.Contains(c.Remediation, "src/a.txt") {
		t.Errorf("remediation should name the drifted path; got %q", c.Remediation)
	}
	// Report verdict must reflect the block-severity failure.
	if report.Verdict != "failed" {
		t.Errorf("expected report.Verdict=failed on effective V10 fail; got %q", report.Verdict)
	}
}

// TestSliceR4_PreimageFreshDowngradesForSuperseded — AC-9 + ADR-029
// D7 + Slice 4 pattern: same stale-preimage condition on a feature
// superseded by an active superseder produces a warn-severity result
// and does NOT flip the report verdict to failed on V10 alone.
func TestSliceR4_PreimageFreshDowngradesForSuperseded(t *testing.T) {
	slug := "stale-superseded"
	s := setupVerifyFeature(t, slug)
	writeIntentFiles(t, s, slug)

	// Force a stale preimage on the target.
	sum := sha256.Sum256([]byte("expected\n"))
	expected := PreimageHashPrefix + hex.EncodeToString(sum[:])
	os.MkdirAll(filepath.Join(s.Root, "src"), 0o755)
	if err := os.WriteFile(filepath.Join(s.Root, "src/a.txt"), []byte("drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recipe := ApplyRecipe{
		Feature:    slug,
		Operations: []RecipeOperation{{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(expected), Content: "next\n"}},
	}
	writeVerifyRecipe(t, s, slug, recipe)

	// Register an active healthy superseder that declares
	// `supersedes` on the target — matches Slice 4 fixture pattern.
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "superseder", Slug: "superseder", Request: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState("superseder", store.StateApplied, "apply", ""); err != nil {
		t.Fatal(err)
	}
	sup, _ := s.LoadFeatureStatus("superseder")
	sup.DependsOn = []store.Dependency{{Slug: slug, Kind: store.DependencyKindSupersedes}}
	if err := s.SaveFeatureStatus(sup); err != nil {
		t.Fatal(err)
	}

	report, err := RunVerify(s, slug, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerify: %v", err)
	}
	c := findCheckR4(t, report.Checks, CheckWriteFilePreimageFresh)
	if c.Passed {
		t.Errorf("expected V10 to still surface the failing signal; got passed=true")
	}
	if c.Severity != SeverityWarn {
		t.Errorf("superseded feature V10 severity must be warn; got %s", c.Severity)
	}
	if !strings.Contains(c.Remediation, "superseder") {
		t.Errorf("remediation should name the superseder in the downgrade suffix; got %q", c.Remediation)
	}
	if !strings.Contains(c.Remediation, "PRD-feature-supersession §4.5") {
		t.Errorf("remediation should cite PRD-feature-supersession §4.5; got %q", c.Remediation)
	}
	// A warn-severity V10 failure MUST NOT flip the report verdict
	// to failed — Slice 4 supersession-controls-severity coupling.
	if report.Verdict == "failed" && !anyBlockFailedExcluding(report.Checks, CheckWriteFilePreimageFresh) {
		t.Errorf("report.Verdict flipped to failed solely because of superseded-V10 warn; got %+v", report.Checks)
	}
}

// TestSliceR4_PreimageFreshSkipsWhenRecipeAbsent — V10 skips (and does
// not fail) when V2 (recipe_parses) skipped/failed: the check needs a
// parseable recipe to iterate operations.
func TestSliceR4_PreimageFreshSkipsWhenRecipeAbsent(t *testing.T) {
	slug := "no-recipe"
	s := setupVerifyFeature(t, slug)
	writeIntentFiles(t, s, slug)
	// Intentionally do NOT write apply-recipe.json.
	report, err := RunVerify(s, slug, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerify: %v", err)
	}
	c := findCheckR4(t, report.Checks, CheckWriteFilePreimageFresh)
	if !c.Skipped || !c.Passed {
		t.Errorf("expected V10 skipped=true, passed=true when recipe absent; got %+v", c)
	}
	if !strings.Contains(c.Reason, "V2") {
		t.Errorf("skip reason should cite V2 dependency; got %q", c.Reason)
	}
}

// anyBlockFailedExcluding: helper mirroring anyBlockFailed but
// excluding one check by ID. Lets the superseded-downgrade test assert
// "V10 warn didn't cause the failed verdict" independently of unrelated
// block-severity checks that might also be failing in the fixture.
func anyBlockFailedExcluding(checks []store.VerifyCheckResult, excludeID string) bool {
	for _, c := range checks {
		if c.ID == excludeID {
			continue
		}
		if c.Severity == SeverityBlock && !c.Passed && !c.Skipped {
			return true
		}
	}
	return false
}

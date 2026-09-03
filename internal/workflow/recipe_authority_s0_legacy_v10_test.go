package workflow

// GH #15 / ADR-036 slice S0 — frozen evidence: the pre-v0.17.0 legacy
// repository fixture and the downstream V10 / preimage-provenance case.
//
// PRD-recipe-generation-authority §8 "S0 - Frozen evidence" requires:
//
//   - "Freeze a pre-v0.17.0 legacy repository fixture that must stay
//     verify-green through the whole wave, **including a feature carrying
//     `recipe-stale.json` and no coverage**."
//   - "Promote the ... downstream V10 case into fixtures."
//
// The V10 case comes from
// `docs/state-of-the-art/case-studies/copilot-api-cumulative-verify-2026-08/summary.md`
// §4 "Preimage diagnosis correction": four recent features failed V10 not
// because a preimage hash was measured stale, but because
// `artifacts/recipe-provenance.json` was absent while every non-empty
// `preimage_hash` matched the bytes at the feature's recorded base. That
// distinction is the whole finding, so the fixture below proves BOTH
// halves: the terminal reason is provenance unavailability, AND the
// preimage would have matched at the recorded base.
//
// Both rows reuse the shipped verify helpers (`setupVerifyFeature`,
// `writeIntent`, `writeVerifyRecipe`, `writeVerifyProvenance`,
// `sliceR4WritePreimage`, `findCheck`) rather than introducing a
// competing runner.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// rgaS0LegacyStaleMarker is the exact `recipe-stale.json` sidecar shape
// `AutogenRecipeForRecord` writes today (`RecipeStaleness`). The timestamp
// is a fixed historical value: S0 must not depend on wall-clock time.
const rgaS0LegacyStaleMarker = `{
  "stale": true,
  "reason": "patch touches files absent from recipe: src/legacy.txt",
  "detected_at": "2026-06-01T00:00:00Z"
}
`

// rgaS0MarkerPath returns the on-disk location of the stale sidecar.
func rgaS0MarkerPath(root, slug string) string {
	return filepath.Join(root, ".tpatch", "features", slug, "artifacts", "recipe-stale.json")
}

// rgaS0CoveragePath returns the on-disk location the (unshipped) coverage
// record would occupy.
func rgaS0CoveragePath(root, slug string) string {
	return filepath.Join(root, ".tpatch", "features", slug, "artifacts", "recipe-coverage.json")
}

// rgaS0CheckSummary renders a stable, order-independent projection of a
// verify report so two runs can be compared without depending on
// `verified_at` or any other wall-clock field.
func rgaS0CheckSummary(t *testing.T, report *VerifyReport) string {
	t.Helper()
	var b strings.Builder
	for _, c := range report.Checks {
		b.WriteString(c.ID)
		b.WriteString("=")
		if c.Passed {
			b.WriteString("pass")
		} else {
			b.WriteString("fail")
		}
		if c.Skipped {
			b.WriteString("/skipped")
		}
		b.WriteString("/")
		b.WriteString(c.Severity)
		b.WriteString(";")
	}
	return b.String()
}

// TestRGAS0LegacyPreV017FeatureStaysVerifyGreen freezes the migration
// fixture: a feature recorded before v0.17.0 semantics carries a canonical
// patch and a legacy recipe (write-file with NO `preimage_hash`, the
// ADR-029 D4 path), carries a `recipe-stale.json` marker, and carries no
// coverage record at all — and verify is green with exit 0.
//
// The marker's presence must not change the verdict today. §6.12 makes it
// warning-class and consumer-ineligible in S5; until then, a green run is
// the contract this fixture holds still.
func TestRGAS0LegacyPreV017FeatureStaysVerifyGreen(t *testing.T) {
	slug := "legacy-pre-v017"
	s := setupVerifyFeature(t, slug)
	writeIntent(t, s, slug)
	writeVerifyRecipe(t, s, slug, ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
		{Type: "write-file", Path: "feature.ts", Content: "export const x = 1;\n"},
	}})
	if err := s.WriteArtifact(slug, "post-apply.patch", equivalentRecipePatch); err != nil {
		t.Fatalf("write canonical patch: %v", err)
	}

	// Baseline WITHOUT the marker, so the marker's effect is measured
	// rather than assumed.
	baseline, err := RunVerify(s, slug, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("baseline RunVerify: %v", err)
	}
	if baseline.Verdict != "passed" || baseline.ExitCode != 0 {
		t.Fatalf("the legacy fixture must be verify-green before the marker: verdict=%q exit=%d", baseline.Verdict, baseline.ExitCode)
	}

	// Now add the pre-v0.17.0 stale marker and re-run.
	if err := s.WriteArtifact(slug, "recipe-stale.json", rgaS0LegacyStaleMarker); err != nil {
		t.Fatalf("write recipe-stale.json: %v", err)
	}

	// The marker state must be REAL, not merely asserted.
	markerBytes, err := os.ReadFile(rgaS0MarkerPath(s.Root, slug))
	if err != nil {
		t.Fatalf("the stale marker must exist on disk: %v", err)
	}
	var marker RecipeStaleness
	if uerr := json.Unmarshal(markerBytes, &marker); uerr != nil {
		t.Fatalf("stale marker is not the shipped RecipeStaleness shape: %v", uerr)
	}
	if !marker.Stale || marker.Reason == "" {
		t.Fatalf("stale marker must record stale=true with a reason, got %+v", marker)
	}
	if _, terr := time.Parse(time.RFC3339, marker.DetectedAt); terr != nil {
		t.Fatalf("detected_at must be RFC3339-shaped, got %q (%v)", marker.DetectedAt, terr)
	}
	// And there must be NO coverage record — that is the legacy state.
	if _, err := os.Stat(rgaS0CoveragePath(s.Root, slug)); !os.IsNotExist(err) {
		t.Fatalf("legacy fixture must carry no coverage record; stat err = %v", err)
	}

	withMarker, err := RunVerify(s, slug, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerify with marker: %v", err)
	}
	if withMarker.Verdict != "passed" {
		t.Fatalf("the stale marker must not turn the verdict into a failure today; got %q", withMarker.Verdict)
	}
	if withMarker.ExitCode != 0 {
		t.Fatalf("exit code changed from 0 to %d because of the marker", withMarker.ExitCode)
	}
	if got, want := rgaS0CheckSummary(t, withMarker), rgaS0CheckSummary(t, baseline); got != want {
		t.Fatalf("the marker changed the check matrix:\n got %s\nwant %s", got, want)
	}
	if len(withMarker.Advisories) != len(baseline.Advisories) {
		t.Fatalf("the marker introduced %d advisory delta", len(withMarker.Advisories)-len(baseline.Advisories))
	}

	// No verify surface names the marker today. §6.12 introduces
	// `recipe-coverage-stale-marker`; S5 must add it deliberately.
	for _, c := range withMarker.Checks {
		if strings.Contains(c.Remediation, "recipe-stale") || strings.Contains(c.Reason, "recipe-stale") {
			t.Errorf("check %s already mentions the stale marker: %+v", c.ID, c)
		}
	}
}

// TestRGAS0LegacyFixtureIsSensitive proves the row above is a measurement:
// the same pipeline DOES go red when the legacy recipe stops matching its
// patch, so "green" is not the fixture's only reachable outcome.
func TestRGAS0LegacyFixtureIsSensitive(t *testing.T) {
	slug := "legacy-sensitivity"
	s := setupVerifyFeature(t, slug)
	writeIntent(t, s, slug)
	writeVerifyRecipe(t, s, slug, ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
		{Type: "write-file", Path: "feature.ts", Content: "export const x = 1;\n"},
	}})
	// A canonical patch that cannot apply at the closure baseline.
	if err := s.WriteArtifact(slug, "post-apply.patch", "this is not a valid patch\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(slug, "recipe-stale.json", rgaS0LegacyStaleMarker); err != nil {
		t.Fatal(err)
	}
	report, err := RunVerify(s, slug, VerifyOptions{NoWrite: true})
	if report == nil {
		t.Fatalf("RunVerify returned no report: %v", err)
	}
	if err == nil && report.Verdict == "passed" {
		t.Fatal("an unusable canonical patch must not verify green; the legacy row would prove nothing")
	}
}

// TestRGAS0DownstreamV10ProvenanceUnavailable freezes the downstream
// case-study row. A recent feature whose recipe carries a REAL
// `preimage_hash` that MATCHES the bytes at its recorded base still fails
// V10, terminally, because `artifacts/recipe-provenance.json` is absent —
// and the remediation is R24 verbatim, naming `absent` as the condition.
//
// This is the exact shape §4 of the copilot-api study corrects: not a
// measured stale hash, a missing manual-provenance publication. GH #19
// owns the repair; S0 owns the record that the repair has not happened.
//
// The row that makes this a promotion of the STUDY rather than a
// duplicate of AC-L99 is the second half: publishing the provenance the
// manual path omits flips the SAME fixture green, which is the study's
// "all 11 non-empty preimage_hash values match the bytes at each
// feature's recorded base_commit" finding, made executable.
func TestRGAS0DownstreamV10ProvenanceUnavailable(t *testing.T) {
	const slug = "downstream-v10"
	const preimageBody = "parent-seed\n"
	const targetPath = "src/parent.txt"

	c := newChainFixture(t)
	c.Feature(slug, store.StateApplied)
	c.Artifacts(slug,
		"diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+FEATURE\n",
		ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
			{Type: "write-file", Path: targetPath, Content: "parent-seed\nFEATURE\n",
				PreimageHash: strPtr(sha256Display(preimageBody))},
		}})

	// Path B, exactly as the study describes it: the shipped skill tells
	// an agent to author a recipe including preimages, and
	// `implement --manual` checkpoints it without writing provenance.
	provenancePath := filepath.Join(c.Root, ".tpatch", "features", slug, "artifacts", "recipe-provenance.json")
	if _, err := os.Stat(provenancePath); !os.IsNotExist(err) {
		t.Fatalf("fixture precondition: provenance must be absent; stat err = %v", err)
	}

	report := c.Verify(slug)
	v10 := checkByID(t, report, CheckWriteFilePreimageFresh)
	if v10.Passed || v10.Skipped {
		t.Fatalf("V10 must fail for a preimage-bearing recipe with no provenance; got %+v", v10)
	}
	if v10.Severity != SeverityBlock {
		t.Errorf("V10 severity = %q, want %q for an effective feature", v10.Severity, SeverityBlock)
	}
	if want := remediationR24(1, targetPath, "absent", slug); v10.Remediation != want {
		t.Fatalf("R24 not verbatim:\n got %q\nwant %q", v10.Remediation, want)
	}
	if report.FailedAt != FailedAtRecipeProvenance {
		t.Fatalf("failed_at = %q, want %q", report.FailedAt, FailedAtRecipeProvenance)
	}
	// The terminal reason is provenance availability, NOT a stale hash:
	// no remediation mentions an observed-versus-expected comparison.
	if strings.Contains(v10.Remediation, "observed") {
		t.Errorf("the study's correction says this is not a measured stale hash: %q", v10.Remediation)
	}
	rgaS0AssertNoCoverageArtifact(t, c.Root, slug)

	// Second half: publish the provenance the manual path omits. The
	// preimage was correct all along, so the SAME fixture goes green.
	writeProvenance(t, c.Store, slug, c.Base, true)
	repaired := c.Verify(slug)
	fixed := checkByID(t, repaired, CheckWriteFilePreimageFresh)
	if !fixed.Passed || fixed.Skipped {
		t.Fatalf("with provenance published the preimage matches at the recorded base; got %+v", fixed)
	}
	if repaired.FailedAt == FailedAtRecipeProvenance {
		t.Fatalf("failed_at must clear once provenance is available; got %q", repaired.FailedAt)
	}
	rgaS0AssertNoCoverageArtifact(t, c.Root, slug)
}

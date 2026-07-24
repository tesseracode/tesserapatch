package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestDoctorD1ReportsMalformedUnsupportedMetadata(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Bad Metadata", Slug: "bad-metadata"}); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(root, ".tpatch", "features", "bad-metadata", "status.json")
	before, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.TrimSuffix(string(before), "\n")
	bad = strings.TrimSuffix(bad, "}") + ",\n  \"unknown_field\": true\n}\n"
	if err := os.WriteFile(statusPath, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tpatch", "features", "bad-metadata", "feature.yaml"), []byte("legacy: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D1"}})
	if err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report, "D1", "feature-metadata-unsupported-field", "bad-metadata")
	assertFinding(t, report, "D1", "legacy-feature-yaml-unsupported", "bad-metadata")
	after, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(bad), after) {
		t.Fatal("D1 dry-run rewrote status.json")
	}
}

func TestDoctorD2ReportsMissingAndMalformedPatchGenerations(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Missing Manifest", Slug: "missing-manifest"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tpatch", "features", "missing-manifest", "artifacts", "post-apply.patch"), []byte("diff --git a/a b/a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Bad Manifest", Slug: "bad-manifest"}); err != nil {
		t.Fatal(err)
	}
	manifestPath := s.PatchGenerationsPath("bad-manifest")
	if err := os.WriteFile(manifestPath, []byte("{\n  \"version\": 99,\n  \"feature\": \"bad-manifest\",\n  \"current_generation\": 0,\n  \"generations\": []\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D2"}})
	if err != nil {
		t.Fatal(err)
	}
	missing := assertFinding(t, report, "D2", "patch-generations-missing", "missing-manifest")
	if missing.Remediation != "run tpatch feature patch refresh missing-manifest" {
		t.Fatalf("unexpected remediation: %q", missing.Remediation)
	}
	malformed := assertFinding(t, report, "D2", "patch-generations-malformed", "bad-manifest")
	if malformed.Line != 1 {
		t.Fatalf("malformed manifest line = %d, want 1", malformed.Line)
	}
}

func TestDoctorPerCheckErrorsDoNotAbortOtherChecks(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	featureDir := filepath.Join(root, ".tpatch", "features", "broken-and-missing")
	if err := os.MkdirAll(filepath.Join(featureDir, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "status.json"), []byte("{not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "artifacts", "post-apply.patch"), []byte("diff --git a/a b/a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunDoctor(s, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertFinding(t, report, "D1", "feature-metadata-malformed", "broken-and-missing")
	assertFinding(t, report, "D2", "patch-generations-missing", "broken-and-missing")
}

func TestDoctorHardInvariantMissingWorkspace(t *testing.T) {
	s := &store.Store{Root: t.TempDir()}
	_, err := RunDoctor(s, DoctorOptions{Fix: true})
	if err == nil {
		t.Fatal("expected hard invariant error")
	}
	var hard *DoctorHardInvariantError
	if !errors.As(err, &hard) {
		t.Fatalf("got %T, want DoctorHardInvariantError", err)
	}
}

func TestDoctorJSONDeterministicAndDecodesDTO(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Missing Manifest", Slug: "missing-manifest"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tpatch", "features", "missing-manifest", "artifacts", "post-apply.patch"), []byte("diff --git a/a b/a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := RunDoctor(s, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WriteDoctorJSON(&first, report); err != nil {
		t.Fatal(err)
	}
	if err := WriteDoctorJSON(&second, report); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatalf("JSON report is not deterministic:\n%s\n---\n%s", first.String(), second.String())
	}
	if strings.Contains(first.String(), "2026-") || strings.Contains(first.String(), "T22:") {
		t.Fatalf("JSON report appears to contain a wall-clock timestamp: %s", first.String())
	}
	var decoded DoctorReport
	dec := json.NewDecoder(strings.NewReader(first.String()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("report does not decode into DoctorReport DTO: %v\n%s", err, first.String())
	}
	if decoded.SchemaVersion != DoctorReportSchemaVersion {
		t.Fatalf("schema_version = %d", decoded.SchemaVersion)
	}
}

func TestDoctorExitCodes(t *testing.T) {
	clean := DoctorReport{Summary: DoctorSummary{}, Fix: false}
	if got := DoctorExitCode(clean); got != 0 {
		t.Fatalf("clean exit = %d", got)
	}
	drift := DoctorReport{Summary: DoctorSummary{Findings: 1}, Fix: false}
	if got := DoctorExitCode(drift); got != 1 {
		t.Fatalf("dry-run drift exit = %d", got)
	}
	fixErr := DoctorReport{Summary: DoctorSummary{Errors: 1}, Fix: true}
	if got := DoctorExitCode(fixErr); got != 2 {
		t.Fatalf("--fix partial failure exit = %d", got)
	}
}

func assertFinding(t *testing.T, report DoctorReport, checkID, code, feature string) DoctorFinding {
	t.Helper()
	for _, f := range report.Findings {
		if f.CheckID == checkID && f.Code == code && f.Feature == feature {
			return f
		}
	}
	t.Fatalf("missing finding %s/%s/%s in %#v", checkID, code, feature, report.Findings)
	return DoctorFinding{}
}

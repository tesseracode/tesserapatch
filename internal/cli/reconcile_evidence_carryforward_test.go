package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestStatusLoadsWhenEvidenceArtifactAbsent verifies PRD 1 §6 acceptance
// ("Existing status round-trips remain backward-compatible when the feature
// has no evidence artifact"). A feature with a ReconcileSummary in status.json
// but NO reconcile-evidence.jsonl on disk must load cleanly via
// store.LoadFeatureStatus and via `tpatch status --json`, with the
// evidence_artifact field omitted from the runtime payload.
func TestStatusLoadsWhenEvidenceArtifactAbsent(t *testing.T) {
	dir, slug, s := cliEvidenceFixture(t, "no-evidence file", map[string]string{
		"new.txt": "brand new\n",
	})

	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus failed for pre-evidence feature: %v", err)
	}
	status.Reconcile.Outcome = store.ReconcileUpstreamed
	status.Reconcile.UpstreamRef = "HEAD"
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatalf("SaveFeatureStatus failed: %v", err)
	}

	evidencePath := filepath.Join(dir, ".tpatch", "features", slug, "artifacts", "reconcile-evidence.jsonl")
	if _, err := os.Stat(evidencePath); !os.IsNotExist(err) {
		t.Fatalf("expected no evidence artifact yet, got err=%v", err)
	}

	if _, err := s.LoadFeatureStatus(slug); err != nil {
		t.Fatalf("LoadFeatureStatus failed when evidence is absent: %v", err)
	}

	statusOut, statusErr, err := runCLIForEvidence("status", "--path", dir, "--json")
	if err != nil {
		t.Fatalf("status --json failed when evidence is absent: %v\nstderr=%s\nstdout=%s", err, statusErr, statusOut)
	}
	var payload any
	if err := json.Unmarshal([]byte(statusOut), &payload); err != nil {
		t.Fatalf("status output is not valid JSON: %v\n%s", err, statusOut)
	}
	if strings.Contains(statusOut, `"evidence_artifact"`) {
		t.Fatalf("expected evidence_artifact field to be omitted when artifact is absent:\n%s", statusOut)
	}
}

func TestStatusLoadsWithEmptyReviewVerdict(t *testing.T) {
	dir, slug, s := cliEvidenceFixture(t, "empty review verdict", map[string]string{
		"new.txt": "brand new\n",
	})

	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus failed for fixture: %v", err)
	}
	status.Reconcile.Outcome = store.ReconcileBlocked
	status.Reconcile.UpstreamRef = "HEAD"
	status.Reconcile.ReviewVerdict = ""
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatalf("SaveFeatureStatus failed: %v", err)
	}

	loaded, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus failed with empty ReviewVerdict: %v", err)
	}
	if loaded.Reconcile.ReviewVerdict != "" || loaded.Reconcile.Outcome != store.ReconcileBlocked {
		t.Fatalf("unexpected reconcile summary after empty-review load: %+v", loaded.Reconcile)
	}
	statusOut, statusErr, err := runCLIForEvidence("status", "--path", dir, "--json")
	if err != nil {
		t.Fatalf("status --json failed with empty ReviewVerdict: %v\nstderr=%s\nstdout=%s", err, statusErr, statusOut)
	}
	if strings.Contains(statusOut, `"review_verdict"`) {
		t.Fatalf("empty review_verdict should remain omitted for back-compat:\n%s", statusOut)
	}
}

func TestReconcileLazilyCreatesEvidenceAndRevisionArtifacts(t *testing.T) {
	dir, slug := cliOperationUpstreamedCandidateFixture(t)
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	evidencePath := s.ReconcileEvidencePath(slug)
	revisionsPath := s.ReconcileRevisionsPath(slug)
	for _, path := range []string{evidencePath, revisionsPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected artifact to be absent before reconcile at %s, err=%v", path, err)
		}
	}

	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed without pre-existing evidence/revision artifacts: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	for _, path := range []string{evidencePath, revisionsPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected artifact to be lazily created at %s: %v", path, err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			t.Fatalf("artifact is empty at %s", path)
		}
		for _, line := range strings.Split(trimmed, "\n") {
			if !json.Valid([]byte(line)) {
				t.Fatalf("artifact contains invalid JSONL at %s:\n%s", path, data)
			}
		}
	}
}

// TestStatusLoadsWhenEvidenceArtifactMalformed verifies PRD 1 §6 acceptance
// ("Corrupt evidence artifacts fail with an explicit warning/error and do not
// prevent status.json from loading"). A feature with a malformed
// reconcile-evidence.jsonl on disk must still load via store.LoadFeatureStatus
// and via `tpatch status --json`. The runtime evidence_artifact field must be
// gracefully omitted (evidenceArtifactRef returns empty string on read
// failure), and the malformed file must remain on disk for operator inspection.
func TestStatusLoadsWhenEvidenceArtifactMalformed(t *testing.T) {
	dir, slug, s := cliEvidenceFixture(t, "malformed evidence", map[string]string{
		"new.txt": "brand new\n",
	})

	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus failed for fixture: %v", err)
	}
	status.Reconcile.Outcome = store.ReconcileUpstreamed
	status.Reconcile.UpstreamRef = "HEAD"
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatalf("SaveFeatureStatus failed: %v", err)
	}

	evidencePath := filepath.Join(dir, ".tpatch", "features", slug, "artifacts", "reconcile-evidence.jsonl")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	malformed := []byte(`{"schema_version":`)
	if err := os.WriteFile(evidencePath, malformed, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.LoadFeatureStatus(slug); err != nil {
		t.Fatalf("LoadFeatureStatus failed when evidence is malformed: %v", err)
	}

	statusOut, statusErr, err := runCLIForEvidence("status", "--path", dir, "--json")
	if err != nil {
		t.Fatalf("status --json failed when evidence is malformed: %v\nstderr=%s\nstdout=%s", err, statusErr, statusOut)
	}
	var payload any
	if err := json.Unmarshal([]byte(statusOut), &payload); err != nil {
		t.Fatalf("status output is not valid JSON: %v\n%s", err, statusOut)
	}
	if strings.Contains(statusOut, `"evidence_artifact"`) {
		t.Fatalf("expected evidence_artifact field to be omitted when artifact is malformed:\n%s", statusOut)
	}

	got, readErr := os.ReadFile(evidencePath)
	if readErr != nil {
		t.Fatalf("malformed evidence file removed unexpectedly: %v", readErr)
	}
	if string(got) != string(malformed) {
		t.Fatalf("malformed evidence file content was modified; want %q got %q", malformed, got)
	}
}

// TestReconcileEvidenceOutputDoesNotLeakFeatureTitle is the rev-2 carry-forward
// strengthened privacy test (D10 / PRD 1 §6.6). The original rev-2 privacy
// tests seeded secrets into file content only; this test seeds a secret into a
// plausible non-content metadata vector (the feature title, which lives in
// status.json and could leak through rendering paths) and verifies that
// reconcile/status output does NOT surface it.
func TestReconcileEvidenceOutputDoesNotLeakFeatureTitle(t *testing.T) {
	secretTitle := "TITLE_SECRET_DO_NOT_LEAK_XYZ123"
	dir, slug, _ := cliEvidenceFixture(t, secretTitle, map[string]string{
		"new.txt": "ordinary new file\n",
	})

	humanOut, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, humanOut)
	}

	evidencePath := filepath.Join(dir, ".tpatch", "features", slug, "artifacts", "reconcile-evidence.jsonl")
	evidenceBytes, readErr := os.ReadFile(evidencePath)
	if readErr != nil {
		t.Fatalf("evidence artifact missing: %v", readErr)
	}
	if strings.Contains(string(evidenceBytes), secretTitle) {
		t.Fatalf("evidence artifact leaked feature title:\n%s", string(evidenceBytes))
	}

	// The verdict-line itself legitimately echoes the feature title (pre-existing
	// behavior, unrelated to evidence). The leak vector under test is the rev-2
	// evidence-hint rendering — assert the hint lines themselves do not embed
	// the title.
	for _, line := range strings.Split(humanOut, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "evidence:") {
			continue
		}
		if strings.Contains(trimmed, secretTitle) {
			t.Fatalf("evidence hint line leaked feature title: %q", trimmed)
		}
	}
}

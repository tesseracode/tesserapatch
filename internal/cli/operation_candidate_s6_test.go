package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestROCS6ReviewLabelsEvidenceWithoutChangingRevisionJSON(t *testing.T) {
	dir, slug, s := cliEvidenceFixture(t, "S6 evidence classes", map[string]string{"new.txt": "new\n"})
	entry := store.ReconcileEvidence{
		SchemaVersion: store.ReconcileEvidenceSchemaVersion, FeatureSlug: slug,
		UpstreamRef: "HEAD", UpstreamCommit: strings.Repeat("a", 40), BaseCommit: strings.Repeat("b", 40),
		RawReconcileVerdict: "still_needed", Phase: store.EvidencePhase2,
		EvidenceKind: store.EvidenceKindRecipeOperationMatch, Confidence: store.EvidenceConfidenceHigh,
		MatchedPaths: []string{}, MatchedOperations: []string{}, UpstreamCommitRefs: []string{},
		MatchOrigin: store.EvidenceMatchOriginUpstream, PreReconcilePresence: store.EvidencePresencePresent,
		RequiresConfirmation: true, ReasonCode: "phase-2-operation-candidate",
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		t.Fatal(err)
	}
	first := entry
	terminal := first
	terminal.Phase = store.EvidencePhase4
	terminal.EvidenceKind = store.EvidenceKindForwardApply
	terminal.RawReconcileVerdict, terminal.ReasonCode = "blocked", "phase-4-forward-apply-conflicts"
	terminal.AttemptID = store.ComputeAttemptID(terminal)
	if err := store.AppendReconcileEvidence(s, slug, terminal); err != nil {
		t.Fatal(err)
	}
	entry.Phase = store.EvidencePhase35
	entry.EvidenceKind = store.EvidenceKindManualReview
	entry.MatchedOperations = []string{"confirmation-gate"}
	entry.ReasonCode = "missing-upstream-commit-ref"
	entry.AttemptID = store.ComputeAttemptID(entry)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		t.Fatal(err)
	}
	writeSession := func(outcome string, evidence []store.ReconcileEvidence) {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"outcome": outcome, "phase": "phase-4-blocked", "evidence": evidence})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.WriteArtifact(slug, "reconcile-session.json", string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	writeSession("still_needed", []store.ReconcileEvidence{first, terminal, entry})
	out, stderr, err := runCLIForEvidence("reconcile", "review", "list", "--path", dir, slug)
	if err != nil || !strings.Contains(out, "class=verdict-bearing phase=phase-2") || !strings.Contains(out, "class=supporting phase=phase-3.5") {
		t.Fatalf("evidence classes missing: %q %q %v", out, stderr, err)
	}
	if strings.Index(out, entry.AttemptID) > strings.Index(out, terminal.AttemptID) ||
		!strings.Contains(out, "returned outcome: still_needed") || !strings.Contains(out, "phase-local verdict=blocked") {
		t.Fatalf("phase ordering or returned/phase-local distinction lost: %q", out)
	}
	out, stderr, err = runCLIForEvidence("reconcile", "review", "list", "--path", dir, "--json", slug)
	if err != nil || strings.Contains(out, `"evidence"`) || !strings.Contains(out, `"revisions"`) {
		t.Fatalf("revision JSON envelope changed: %q %q %v", out, stderr, err)
	}
	latest := first
	latest.Phase = store.EvidencePhase4
	latest.EvidenceKind = store.EvidenceKindForwardApply
	latest.RawReconcileVerdict, latest.ReasonCode = "blocked", "phase-4-blocked"
	latest.AttemptID = store.ComputeAttemptID(latest)
	if err := store.AppendReconcileEvidence(s, slug, latest); err != nil {
		t.Fatal(err)
	}
	writeSession("blocked", []store.ReconcileEvidence{latest})
	out, stderr, err = runCLIForEvidence("reconcile", "review", "list", "--path", dir, slug)
	if err != nil || strings.Contains(out, first.AttemptID) || strings.Contains(out, entry.AttemptID) ||
		!strings.Contains(out, latest.AttemptID) || !strings.Contains(out, "returned outcome: blocked") {
		t.Fatalf("latest invocation mixed with history: %q %q %v", out, stderr, err)
	}
	out, stderr, err = runCLIForEvidence("reconcile", "review", "list", "--path", dir, "--all", slug)
	if err != nil || !strings.Contains(out, "historical evidence") || !strings.Contains(out, first.AttemptID) {
		t.Fatalf("explicit history missing: %q %q %v", out, stderr, err)
	}
}

func TestROCS6PlainReconcileRecoveryRefusesWithExitThree(t *testing.T) {
	dir, slug, _ := cliEvidenceFixture(t, "S6 recovery refusal", map[string]string{"new.txt": "new\n"})
	path := filepath.Join(dir, ".tpatch", "reconcile-candidates", slug, "rc_0123456789ab")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "accept-journal.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	var coded *ExitCodeError
	if !errors.As(err, &coded) || coded.Code != 3 || !strings.Contains(err.Error(), "recovery-required") {
		t.Fatalf("recovery refusal: %q %q %v", out, stderr, err)
	}
}

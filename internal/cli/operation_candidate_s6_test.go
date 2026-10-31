package cli

import (
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
	entry.Phase = store.EvidencePhase35
	entry.EvidenceKind = store.EvidenceKindManualReview
	entry.MatchedOperations = []string{"confirmation-gate"}
	entry.ReasonCode = "missing-upstream-commit-ref"
	entry.AttemptID = store.ComputeAttemptID(entry)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := runCLIForEvidence("reconcile", "review", "list", "--path", dir, slug)
	if err != nil || !strings.Contains(out, "class=verdict-bearing phase=phase-2") || !strings.Contains(out, "class=supporting phase=phase-3.5") {
		t.Fatalf("evidence classes missing: %q %q %v", out, stderr, err)
	}
	out, stderr, err = runCLIForEvidence("reconcile", "review", "list", "--path", dir, "--json", slug)
	if err != nil || strings.Contains(out, `"evidence"`) || !strings.Contains(out, `"revisions"`) {
		t.Fatalf("revision JSON envelope changed: %q %q %v", out, stderr, err)
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
	out, stderr, err := runCLIForEvidence("reconcile", "--path", dir, "--upstream-ref", "HEAD", slug)
	var coded *ExitCodeError
	if !errors.As(err, &coded) || coded.Code != 3 || !strings.Contains(err.Error(), "recovery-required") {
		t.Fatalf("recovery refusal: %q %q %v", out, stderr, err)
	}
}

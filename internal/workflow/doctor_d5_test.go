package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestDoctorD5CleanEvidenceAndMissingEvidenceClasses(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	saveDoctorD5Feature(t, s, "clean", func(st *store.FeatureStatus) {
		st.State = store.StateApplied
		st.Reconcile.AttemptedAt = "2026-07-28T00:00:00Z"
		st.Reconcile.Outcome = store.ReconcileReapplied
	})
	if err := store.AppendReconcileEvidence(s, "clean", doctorD5Evidence("clean")); err != nil {
		t.Fatal(err)
	}
	clean, err := RunDoctor(s, DoctorOptions{Checks: []string{"D5"}})
	if err != nil {
		t.Fatal(err)
	}
	if clean.Summary.Findings != 0 || clean.Summary.Warnings != 0 || clean.Summary.Errors != 0 {
		t.Fatalf("clean D5 findings: %#v %#v", clean.Summary, clean.Findings)
	}

	saveDoctorD5Feature(t, s, "missing-modern", func(st *store.FeatureStatus) {
		st.State = store.StateApplied
		st.Reconcile.AttemptedAt = "2026-07-28T00:00:00Z"
		st.Reconcile.Outcome = store.ReconcileReapplied
	})
	saveDoctorD5Feature(t, s, "pre-adr", func(st *store.FeatureStatus) {
		st.State = store.StateApplied
	})
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D5"}})
	if err != nil {
		t.Fatal(err)
	}
	missing := assertFinding(t, report, "D5", "reconcile-evidence-missing", "missing-modern")
	if missing.Severity != "drift" || missing.Remediation != "run tpatch reconcile missing-modern" {
		t.Fatalf("modern missing evidence finding = %#v", missing)
	}
	grace := assertFinding(t, report, "D5", "reconcile-evidence-missing-pre-adr025", "pre-adr")
	if grace.Severity != "warning" {
		t.Fatalf("pre-ADR grace severity = %#v", grace)
	}
}

func TestDoctorD5MalformedJSONLLineNumbersAndContinuation(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	saveDoctorD5Feature(t, s, "bad-evidence", func(st *store.FeatureStatus) { st.State = store.StateApplied })
	saveDoctorD5Feature(t, s, "other-feature", func(st *store.FeatureStatus) { st.State = store.StateApplied })
	if err := store.AppendReconcileEvidence(s, "other-feature", doctorD5Evidence("other-feature")); err != nil {
		t.Fatal(err)
	}
	good1 := doctorD5EvidenceLine(t, "bad-evidence")
	second := doctorD5Evidence("bad-evidence")
	second.UpstreamCommit = strings.Repeat("b", 40)
	second.AttemptID = store.ComputeAttemptID(second)
	good2 := doctorD5EvidenceLineForEntry(t, second)
	body := good1 + "\n" + `{"schema_version":` + "\n" + good2 + "\n"
	if err := os.WriteFile(s.ReconcileEvidencePath("bad-evidence"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D5"}})
	if err != nil {
		t.Fatal(err)
	}
	f := assertFinding(t, report, "D5", "reconcile-evidence-malformed", "bad-evidence")
	if f.Line != 2 {
		t.Fatalf("malformed evidence line = %d, want 2: %#v", f.Line, f)
	}
	if strings.Contains(f.Message, good1) || strings.Contains(f.Message, good2) {
		t.Fatalf("D5 leaked full evidence content in finding: %s", f.Message)
	}
	if got := len(report.Findings); got != 1 {
		t.Fatalf("D5 should continue past malformed line and other feature without extra findings, got %#v", report.Findings)
	}
}

func TestDoctorD5MalformedRevisionsJSONL(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	saveDoctorD5Feature(t, s, "bad-revisions", func(st *store.FeatureStatus) { st.State = store.StateApplied })
	path := s.ReconcileRevisionsPath("bad-revisions")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := doctorD5RevisionLine(t, "bad-revisions") + "\n" + `{"schema_version":` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D5"}})
	if err != nil {
		t.Fatal(err)
	}
	f := assertFinding(t, report, "D5", "reconcile-revisions-malformed", "bad-revisions")
	if f.Line != 2 {
		t.Fatalf("malformed revisions line = %d, want 2: %#v", f.Line, f)
	}
}

func saveDoctorD5Feature(t *testing.T, s *store.Store, slug string, mutate func(*store.FeatureStatus)) {
	t.Helper()
	st, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: slug})
	if err != nil {
		t.Fatal(err)
	}
	mutate(&st)
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
}

func doctorD5Evidence(slug string) store.ReconcileEvidence {
	entry := store.ReconcileEvidence{
		SchemaVersion:        store.ReconcileEvidenceSchemaVersion,
		FeatureSlug:          slug,
		UpstreamRef:          "origin/main",
		UpstreamCommit:       strings.Repeat("a", 40),
		BaseCommit:           strings.Repeat("0", 40),
		RawReconcileVerdict:  string(store.ReconcileReapplied),
		Phase:                store.EvidencePhase4,
		EvidenceKind:         store.EvidenceKindForwardApply,
		Confidence:           store.EvidenceConfidenceHigh,
		MatchedPaths:         []string{},
		MatchedOperations:    []string{},
		MatchOrigin:          store.EvidenceMatchOriginUpstream,
		UpstreamCommitRefs:   []string{"origin/main"},
		PreReconcilePresence: store.EvidencePresencePresent,
		RequiresConfirmation: false,
		ReasonCode:           "doctor-test",
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	return entry
}

func doctorD5EvidenceLine(t *testing.T, slug string) string {
	t.Helper()
	return doctorD5EvidenceLineForEntry(t, doctorD5Evidence(slug))
}

func doctorD5EvidenceLineForEntry(t *testing.T, entry store.ReconcileEvidence) string {
	t.Helper()
	s := &store.Store{Root: t.TempDir()}
	if err := store.AppendReconcileEvidence(s, entry.FeatureSlug, entry); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.ReconcileEvidencePath(entry.FeatureSlug))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func doctorD5RevisionLine(t *testing.T, slug string) string {
	t.Helper()
	entry := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         slug,
		EvidenceAttemptID:   "re_abcdef123456",
		RawReconcileVerdict: string(store.ReconcileUpstreamed),
		ReviewVerdict:       store.ReviewVerdictConfirmed,
		FinalFeatureState:   store.StateUpstreamMerged,
		ActionTaken:         store.ReconcileActionConfirmedRetired,
		ReasonCode:          "doctor-test",
		ValidationRefs:      []store.ValidationRef{},
	}
	entry.EntryID = store.ComputeRevisionID(entry)
	s := &store.Store{Root: t.TempDir()}
	if err := store.AppendReconcileRevision(s, slug, entry); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.ReconcileRevisionsPath(slug))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

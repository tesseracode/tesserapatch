package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReconcileRevisionRoundTripAndStableID(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	entry := sampleRevision("demo")
	if err := AppendReconcileRevision(s, "demo", entry); err != nil {
		t.Fatalf("AppendReconcileRevision: %v", err)
	}
	got, err := LoadReconcileRevisions(s, "demo")
	if err != nil {
		t.Fatalf("LoadReconcileRevisions: %v", err)
	}
	if len(got) != 1 || got[0].EntryID == "" || got[0].ReviewVerdict != ReviewVerdictConfirmed {
		t.Fatalf("unexpected revision: %+v", got)
	}
	if ok, _ := regexp.MatchString(`^rr_[0-9a-f]{12}$`, got[0].EntryID); !ok {
		t.Fatalf("entry_id has wrong shape: %s", got[0].EntryID)
	}
	if id := ComputeRevisionID(got[0]); id != got[0].EntryID {
		t.Fatalf("entry_id not stable: got %s want %s", id, got[0].EntryID)
	}
}

func TestReconcileRevisionMalformedLineAndWriterRefusal(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	path := s.ReconcileRevisionsPath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	line, _ := marshalReconcileRevisionLine(sampleRevision("demo"))
	body := string(line) + "\n" + `{"schema_version":` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadReconcileRevisions(s, "demo")
	if !errors.Is(err, ErrMalformedRevision) || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected malformed revision with line number, got %v", err)
	}
	if err := AppendReconcileRevision(s, "demo", sampleRevision("demo")); !errors.Is(err, ErrMalformedRevision) {
		t.Fatalf("writer must refuse corrupt revision file, got %v", err)
	}
}

func TestReconcileRevisionPrivacyNoSourceLeak(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	secret := "SECRET_REVISION_SOURCE_DO_NOT_LEAK"
	entry := sampleRevision("demo")
	if strings.Contains(string(mustRevisionLine(t, entry)), secret) {
		t.Fatal("fixture unexpectedly contains secret")
	}
	if err := AppendReconcileRevision(s, "demo", entry); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.ReconcileRevisionsPath("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(secret)) {
		t.Fatalf("revision artifact leaked source body: %s", data)
	}
}

func sampleRevision(slug string) ReconcileRevision {
	entry := ReconcileRevision{
		SchemaVersion:       ReconcileRevisionSchemaVersion,
		FeatureSlug:         slug,
		EvidenceAttemptID:   "re_abcdef123456",
		RawReconcileVerdict: string(ReconcileUpstreamed),
		ReviewVerdict:       ReviewVerdictConfirmed,
		FinalFeatureState:   StateUpstreamMerged,
		ActionTaken:         ReconcileActionConfirmedRetired,
		ReasonCode:          "confirmed-upstreamed",
		ValidationRefs:      []ValidationRef{},
	}
	entry.EntryID = ComputeRevisionID(entry)
	return entry
}

func mustRevisionLine(t *testing.T, entry ReconcileRevision) []byte {
	t.Helper()
	line, err := marshalReconcileRevisionLine(entry)
	if err != nil {
		t.Fatal(err)
	}
	return line
}

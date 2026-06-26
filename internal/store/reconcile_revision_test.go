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
	first := sampleRevision("demo")
	second := sampleRevision("demo")
	second.EvidenceAttemptID = "re_123456abcdef"
	second.EntryID = ComputeRevisionID(second)
	firstLine, _ := marshalReconcileRevisionLine(first)
	secondLine, _ := marshalReconcileRevisionLine(second)
	body := string(firstLine) + "\n" + `{"schema_version":` + "\n" + string(secondLine) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadReconcileRevisions(s, "demo")
	if !errors.Is(err, ErrMalformedRevision) || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected malformed revision with line number, got %v", err)
	}
	valid, corrupt, err := LoadReconcileRevisionsLenient(path)
	if err != nil {
		t.Fatalf("LoadReconcileRevisionsLenient returned I/O error: %v", err)
	}
	if len(valid) != 2 || len(corrupt) != 1 || corrupt[0].Line != 2 {
		t.Fatalf("expected 2 valid entries and line-2 corrupt entry, got valid=%+v corrupt=%+v", valid, corrupt)
	}
	if valid[0].EntryID != first.EntryID || valid[1].EntryID != second.EntryID {
		t.Fatalf("valid entries were not preserved around corrupt line: %+v", valid)
	}
	if err := AppendReconcileRevision(s, "demo", sampleRevision("demo")); !errors.Is(err, ErrMalformedRevision) {
		t.Fatalf("writer must refuse corrupt revision file, got %v", err)
	}
}

func TestReconcileRevisionPrivacyNoSourceLeak(t *testing.T) {
	secret := "SECRET_REVISION_METADATA_DO_NOT_LEAK"
	root := filepath.Join(t.TempDir(), secret+"-repo-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	s := &Store{Root: root}
	slug := "secret-revision-metadata-do-not-leak"
	entry := sampleRevision(slug)
	if strings.Contains(string(mustRevisionLine(t, entry)), secret) {
		t.Fatal("fixture unexpectedly contains exact secret")
	}
	if err := AppendReconcileRevision(s, slug, entry); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.ReconcileRevisionsPath(slug))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(secret)) {
		t.Fatalf("revision artifact leaked secret metadata vector: %s", data)
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

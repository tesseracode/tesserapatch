package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestReconcileEvidenceRoundTrip(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	entry := sampleEvidence("demo")
	if err := AppendReconcileEvidence(s, "demo", entry); err != nil {
		t.Fatalf("AppendReconcileEvidence: %v", err)
	}
	got, err := LoadReconcileEvidence(s, "demo")
	if err != nil {
		t.Fatalf("LoadReconcileEvidence: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].AttemptID == "" || got[0].FeatureSlug != "demo" || got[0].ReasonCode != entry.ReasonCode {
		t.Fatalf("fields not preserved: %+v", got[0])
	}
}

func TestReconcileEvidenceDeterminismSameLogicalInput(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	entry := sampleEvidence("demo")
	entry.MatchedPaths = []string{"z.go", "a.go"}
	if err := AppendReconcileEvidence(s, "demo", entry); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	path := s.ReconcileEvidencePath("demo")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entry.MatchedPaths = []string{"a.go", "z.go"}
	if err := AppendReconcileEvidence(s, "demo", entry); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("same logical input changed file:\nfirst=%s\nsecond=%s", first, second)
	}
}

func TestReconcileEvidenceSortedKeys(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := AppendReconcileEvidence(s, "demo", sampleEvidence("demo")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.ReconcileEvidencePath("demo"))
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(data))
	wantOrder := []string{"attempt_id", "base_commit", "confidence", "evidence_kind", "feature_slug", "match_origin", "matched_operations", "matched_paths", "pre_reconcile_presence", "raw_reconcile_verdict", "reason_code", "requires_confirmation", "schema_version", "upstream_commit", "upstream_commit_refs", "upstream_ref"}
	last := -1
	for _, key := range wantOrder {
		idx := strings.Index(line, `"`+key+`":`)
		if idx < 0 {
			t.Fatalf("key %q missing from %s", key, line)
		}
		if idx < last {
			t.Fatalf("key %q appeared out of order in %s", key, line)
		}
		last = idx
	}
}

func TestReconcileEvidenceSortedArrays(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	entry := sampleEvidence("demo")
	entry.MatchedPaths = []string{"z", "a"}
	entry.MatchedOperations = []string{"op-9", "op-1"}
	entry.UpstreamCommitRefs = []string{"c", "b"}
	if err := AppendReconcileEvidence(s, "demo", entry); err != nil {
		t.Fatal(err)
	}
	got, err := LoadReconcileEvidence(s, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[0].MatchedPaths, []string{"a", "z"}) || !reflect.DeepEqual(got[0].MatchedOperations, []string{"op-1", "op-9"}) || !reflect.DeepEqual(got[0].UpstreamCommitRefs, []string{"b", "c"}) {
		t.Fatalf("arrays not sorted: %+v", got[0])
	}
}

func TestComputeAttemptIDContentAddressedDeterministic(t *testing.T) {
	a := sampleEvidence("demo")
	b := sampleEvidence("demo")
	a.MatchedPaths = []string{"z", "a"}
	b.MatchedPaths = []string{"a", "z"}
	id1 := ComputeAttemptID(a)
	id2 := ComputeAttemptID(b)
	if id1 != id2 {
		t.Fatalf("reordered arrays changed attempt_id: %s != %s", id1, id2)
	}
	if ok, _ := regexp.MatchString(`^re_[0-9a-f]{12}$`, id1); !ok {
		t.Fatalf("attempt_id has wrong shape: %s", id1)
	}
	a.AttemptID = "re_deadbeef0000"
	if got := ComputeAttemptID(a); got != id1 {
		t.Fatalf("attempt_id must be excluded from hash input: got %s want %s", got, id1)
	}
}

func TestReconcileEvidenceMalformedLineAndWriterRefusal(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	path := s.ReconcileEvidencePath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	good := sampleEvidenceLine(t, "demo")
	body := good + "\n" + `{"schema_version":` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadReconcileEvidence(s, "demo")
	if !errors.Is(err, ErrMalformedEvidence) || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected ErrMalformedEvidence with line number, got %v", err)
	}
	if err := AppendReconcileEvidence(s, "demo", sampleEvidence("demo")); !errors.Is(err, ErrMalformedEvidence) {
		t.Fatalf("writer must refuse corrupt file with sentinel, got %v", err)
	}
}

func TestReconcileEvidenceStrictEnums(t *testing.T) {
	for name, mutate := range map[string]func(*ReconcileEvidence){
		"phase":                  func(e *ReconcileEvidence) { e.Phase = "phase-x" },
		"evidence_kind":          func(e *ReconcileEvidence) { e.EvidenceKind = "surprise" },
		"confidence":             func(e *ReconcileEvidence) { e.Confidence = "certain" },
		"match_origin":           func(e *ReconcileEvidence) { e.MatchOrigin = "elsewhere" },
		"pre_reconcile_presence": func(e *ReconcileEvidence) { e.PreReconcilePresence = "maybe" },
	} {
		t.Run(name, func(t *testing.T) {
			s := &Store{Root: t.TempDir()}
			entry := sampleEvidence("demo")
			mutate(&entry)
			writeRawEvidence(t, s, "demo", entry)
			_, err := LoadReconcileEvidence(s, "demo")
			if !errors.Is(err, ErrMalformedEvidence) {
				t.Fatalf("expected malformed enum, got %v", err)
			}
		})
	}
}

func TestReconcileEvidenceUnknownSchemaVersion(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	entry := sampleEvidence("demo")
	entry.SchemaVersion = 2
	writeRawEvidence(t, s, "demo", entry)
	before, _ := os.ReadFile(s.ReconcileEvidencePath("demo"))
	_, err := LoadReconcileEvidence(s, "demo")
	if !errors.Is(err, ErrMalformedEvidence) || !strings.Contains(err.Error(), "unsupported schema_version") {
		t.Fatalf("expected unsupported version sentinel, got %v", err)
	}
	after, _ := os.ReadFile(s.ReconcileEvidencePath("demo"))
	if !bytes.Equal(before, after) {
		t.Fatal("reader modified raw evidence file")
	}
}

func TestReconcileEvidenceUnknownField(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	path := s.ReconcileEvidencePath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSuffix(sampleEvidenceLine(t, "demo"), "}") + `,"unexpected":"nope"}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadReconcileEvidence(s, "demo")
	if !errors.Is(err, ErrMalformedEvidence) || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestReconcileEvidenceDuplicateIDHandling(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	entry := sampleEvidence("demo")
	if err := AppendReconcileEvidence(s, "demo", entry); err != nil {
		t.Fatal(err)
	}
	path := s.ReconcileEvidencePath("demo")
	one, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(append([]byte(nil), one...), one...), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadReconcileEvidence(s, "demo")
	if err != nil || len(got) != 1 {
		t.Fatalf("byte-identical duplicate should no-op on load, got len=%d err=%v", len(got), err)
	}
	diff := sampleEvidence("demo")
	diff.AttemptID = got[0].AttemptID
	diff.ReasonCode = "different"
	line, _ := marshalReconcileEvidenceLine(diff)
	if err := os.WriteFile(path, append(append([]byte(nil), one...), append(line, '\n')...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadReconcileEvidence(s, "demo")
	if !errors.Is(err, ErrMalformedEvidence) || !strings.Contains(err.Error(), "differing payload") {
		t.Fatalf("expected duplicate differing payload malformed, got %v", err)
	}
}

func TestReconcileEvidenceRefsOmitAndPreserve(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	entry := sampleEvidence("demo")
	entry.Refs = &EvidenceRefs{}
	if err := AppendReconcileEvidence(s, "demo", entry); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(s.ReconcileEvidencePath("demo"))
	if strings.Contains(string(data), `"refs"`) {
		t.Fatalf("empty refs must be omitted: %s", data)
	}

	s2 := &Store{Root: t.TempDir()}
	entry = sampleEvidence("demo")
	entry.Refs = &EvidenceRefs{PatchGenerationID: "pg_abc123def456", PatchGenerationsPath: "artifacts/patch-generations.json", Anchors: "anchors.json"}
	if err := AppendReconcileEvidence(s2, "demo", entry); err != nil {
		t.Fatal(err)
	}
	got, err := LoadReconcileEvidence(s2, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Refs == nil || got[0].Refs.PatchGenerationID != "pg_abc123def456" || got[0].Refs.Anchors != "anchors.json" {
		t.Fatalf("refs not preserved: %+v", got[0].Refs)
	}
}

func TestReconcileEvidencePrivacyNoSourceLeak(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	secret := "SECRET_SOURCE_BODY_DO_NOT_LEAK"
	entry := sampleEvidence("demo")
	entry.MatchedPaths = []string{"src/secret.go"}
	entry.MatchedOperations = []string{"op-secret"}
	if strings.Contains(sampleEvidenceLine(t, "demo"), secret) {
		t.Fatal("test fixture unexpectedly contains secret")
	}
	if err := AppendReconcileEvidence(s, "demo", entry); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.ReconcileEvidencePath("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("raw source bytes leaked into evidence: %s", data)
	}
}

func TestPatchIDMatchEvidenceFields(t *testing.T) {
	match := PatchIDMatch{OurPatchID: "patch123", MatchedUpstreamSHA: "abc", AdditionalMatches: []string{"z", "a"}, ScannedRange: "base..tip", ScannedCount: 7}
	entry := PatchIDMatchEvidenceFields(sampleEvidence("demo"), match)
	if entry.EvidenceKind != EvidenceKindPatchIDMatch || entry.GitPatchID != "patch123" || entry.GitPatchIDAlgorithm != "git-patch-id-stable" || entry.MatchedUpstreamSHA != "abc" || entry.ScannedRange != "base..tip" || entry.ScannedCount != 7 {
		t.Fatalf("patch-id fields not populated: %+v", entry)
	}
	if err := AppendReconcileEvidence(&Store{Root: t.TempDir()}, "demo", entry); err != nil {
		t.Fatalf("patch-id evidence did not validate: %v", err)
	}
}

func TestReconcileEvidenceAtomicWriteNoTempOnSuccess(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := AppendReconcileEvidence(s, "demo", sampleEvidence("demo")); err != nil {
		t.Fatal(err)
	}
	path := s.ReconcileEvidencePath("demo")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("final evidence file missing: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file should not remain after successful atomic write, err=%v", err)
	}
}

func TestReconcileEvidenceRejectsNonNewlineTerminatedFinalObject(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	path := s.ReconcileEvidencePath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(sampleEvidenceLine(t, "demo")), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadReconcileEvidence(s, "demo")
	if !errors.Is(err, ErrMalformedEvidence) || !strings.Contains(err.Error(), "newline") {
		t.Fatalf("expected newline termination error, got %v", err)
	}
}

func sampleEvidence(slug string) ReconcileEvidence {
	entry := ReconcileEvidence{
		SchemaVersion:        ReconcileEvidenceSchemaVersion,
		FeatureSlug:          slug,
		UpstreamRef:          "origin/main",
		UpstreamCommit:       "abcdef1234567890",
		BaseCommit:           "123456abcdef7890",
		RawReconcileVerdict:  string(ReconcileUpstreamed),
		Phase:                EvidencePhase2,
		EvidenceKind:         EvidenceKindRecipeOperationMatch,
		Confidence:           EvidenceConfidenceLow,
		MatchedPaths:         []string{"src/a.go"},
		MatchedOperations:    []string{"op-1"},
		MatchOrigin:          EvidenceMatchOriginUnknown,
		UpstreamCommitRefs:   []string{},
		PreReconcilePresence: EvidencePresencePresent,
		RequiresConfirmation: true,
		ReasonCode:           "match-origin-unknown",
	}
	entry.AttemptID = ComputeAttemptID(entry)
	return entry
}

func sampleEvidenceLine(t *testing.T, slug string) string {
	t.Helper()
	line, err := marshalReconcileEvidenceLine(sampleEvidence(slug))
	if err != nil {
		t.Fatal(err)
	}
	return string(line)
}

func writeRawEvidence(t *testing.T, s *Store, slug string, entry ReconcileEvidence) {
	t.Helper()
	path := s.ReconcileEvidencePath(slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(evidenceOrderedMap(entry))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

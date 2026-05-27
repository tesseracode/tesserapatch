package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	ReconcileRevisionSchemaVersion = 1
	reconcileRevisionsFileName     = "reconcile-revisions.jsonl"
)

var ErrMalformedRevision = errors.New("reconcile-revisions.jsonl: malformed revision")

type ReconcileReviewVerdict string

const (
	ReviewVerdictConfirmed     ReconcileReviewVerdict = "confirmed"
	ReviewVerdictFalsePositive ReconcileReviewVerdict = "false-positive"
	ReviewVerdictFalseNegative ReconcileReviewVerdict = "false-negative"
	ReviewVerdictInconclusive  ReconcileReviewVerdict = "inconclusive"
	ReviewVerdictDeferred      ReconcileReviewVerdict = "deferred"
)

type ReconcileActionTaken string

const (
	ReconcileActionNone                 ReconcileActionTaken = "none"
	ReconcileActionConfirmedRetired     ReconcileActionTaken = "confirmed-retired"
	ReconcileActionReapplied            ReconcileActionTaken = "reapplied"
	ReconcileActionReappliedAndRecorded ReconcileActionTaken = "reapplied-and-recorded"
	ReconcileActionImplemented          ReconcileActionTaken = "implemented"
	ReconcileActionDeferred             ReconcileActionTaken = "deferred"
	ReconcileActionSkipped              ReconcileActionTaken = "skipped"
	ReconcileActionCleanupNeeded        ReconcileActionTaken = "cleanup-needed"
)

type ValidationRef struct {
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Result string `json:"result"`
}

type ReconcileRevision struct {
	SchemaVersion       int                    `json:"schema_version"`
	EntryID             string                 `json:"entry_id"`
	FeatureSlug         string                 `json:"feature_slug"`
	EvidenceAttemptID   string                 `json:"evidence_attempt_id"`
	RawReconcileVerdict string                 `json:"raw_reconcile_verdict"`
	ReviewVerdict       ReconcileReviewVerdict `json:"review_verdict"`
	FinalFeatureState   FeatureState           `json:"final_feature_state"`
	ActionTaken         ReconcileActionTaken   `json:"action_taken"`
	ReasonCode          string                 `json:"reason_code"`
	ValidationRefs      []ValidationRef        `json:"validation_refs"`
	SupersedesEntryID   string                 `json:"supersedes_entry_id,omitempty"`
	Refs                *EvidenceRefs          `json:"refs,omitempty"`
}

func (s *Store) ReconcileRevisionsPath(slug string) string {
	return filepath.Join(s.featureArtifactsDir(slug), reconcileRevisionsFileName)
}

func ComputeRevisionID(entry ReconcileRevision) string {
	entry = normalizeReconcileRevision(entry)
	entry.EntryID = ""
	data, _ := canonicalRevisionIdentityJSON(entry)
	sum := sha256.Sum256(data)
	return "rr_" + hex.EncodeToString(sum[:])[:12]
}

func AppendReconcileRevision(s *Store, slug string, entry ReconcileRevision) error {
	path := s.ReconcileRevisionsPath(slug)
	if _, err := LoadReconcileRevisions(s, slug); err != nil {
		return err
	}
	entry.FeatureSlug = slug
	entry = normalizeReconcileRevision(entry)
	if entry.EntryID == "" {
		entry.EntryID = ComputeRevisionID(entry)
	}
	if err := validateReconcileRevision(slug, entry); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedRevision, err)
	}
	line, err := marshalReconcileRevisionLine(entry)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	var existing []byte
	if data, err := os.ReadFile(path); err == nil {
		existing = data
		for _, oldLine := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
			old, err := decodeReconcileRevisionLine(oldLine)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrMalformedRevision, err)
			}
			if old.EntryID == entry.EntryID {
				oldBytes, _ := marshalReconcileRevisionLine(old)
				if bytes.Equal(oldBytes, bytes.TrimSuffix(line, []byte("\n"))) {
					return nil
				}
				return fmt.Errorf("%w: entry_id %q has differing payload", ErrMalformedRevision, entry.EntryID)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data := append(append([]byte(nil), existing...), line...)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func LoadReconcileRevisions(s *Store, slug string) ([]ReconcileRevision, error) {
	path := s.ReconcileRevisionsPath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []ReconcileRevision{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return []ReconcileRevision{}, nil
	}
	if data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("%w: line %d: final object is not newline-terminated", ErrMalformedRevision, bytes.Count(data, []byte("\n"))+1)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	entries := make([]ReconcileRevision, 0, len(lines))
	seen := map[string][]byte{}
	for i, line := range lines {
		lineNo := i + 1
		if len(bytes.TrimSpace(line)) == 0 {
			return nil, fmt.Errorf("%w: line %d: empty line", ErrMalformedRevision, lineNo)
		}
		entry, err := decodeReconcileRevisionLine(line)
		if err != nil {
			return nil, fmt.Errorf("%w: line %d: %v", ErrMalformedRevision, lineNo, err)
		}
		entry = normalizeReconcileRevision(entry)
		if err := validateReconcileRevision(slug, entry); err != nil {
			return nil, fmt.Errorf("%w: line %d: %w", ErrMalformedRevision, lineNo, err)
		}
		canon, _ := marshalReconcileRevisionLine(entry)
		if prev, ok := seen[entry.EntryID]; ok {
			if !bytes.Equal(prev, canon) {
				return nil, fmt.Errorf("%w: line %d: duplicate entry_id %q has differing payload", ErrMalformedRevision, lineNo, entry.EntryID)
			}
			continue
		}
		seen[entry.EntryID] = canon
		entries = append(entries, entry)
	}
	return entries, nil
}

func normalizeReconcileRevision(entry ReconcileRevision) ReconcileRevision {
	if entry.SchemaVersion == 0 {
		entry.SchemaVersion = ReconcileRevisionSchemaVersion
	}
	if entry.ValidationRefs == nil {
		entry.ValidationRefs = []ValidationRef{}
	}
	sort.Slice(entry.ValidationRefs, func(i, j int) bool {
		if entry.ValidationRefs[i].Kind != entry.ValidationRefs[j].Kind {
			return entry.ValidationRefs[i].Kind < entry.ValidationRefs[j].Kind
		}
		if entry.ValidationRefs[i].Value != entry.ValidationRefs[j].Value {
			return entry.ValidationRefs[i].Value < entry.ValidationRefs[j].Value
		}
		return entry.ValidationRefs[i].Result < entry.ValidationRefs[j].Result
	})
	if entry.Refs != nil && entry.Refs.empty() {
		entry.Refs = nil
	}
	return entry
}

func decodeReconcileRevisionLine(line []byte) (ReconcileRevision, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return ReconcileRevision{}, err
	}
	allowed := map[string]bool{
		"schema_version": true, "entry_id": true, "feature_slug": true, "evidence_attempt_id": true,
		"raw_reconcile_verdict": true, "review_verdict": true, "final_feature_state": true, "action_taken": true,
		"reason_code": true, "validation_refs": true, "supersedes_entry_id": true, "refs": true,
	}
	for k := range raw {
		if !allowed[k] {
			return ReconcileRevision{}, fmt.Errorf("unknown field %q", k)
		}
	}
	for _, k := range []string{"schema_version", "entry_id", "feature_slug", "evidence_attempt_id", "raw_reconcile_verdict", "review_verdict", "final_feature_state", "action_taken", "reason_code", "validation_refs"} {
		if _, ok := raw[k]; !ok {
			return ReconcileRevision{}, fmt.Errorf("missing required field %q", k)
		}
	}
	var entry ReconcileRevision
	dec = json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entry); err != nil {
		return ReconcileRevision{}, err
	}
	return entry, nil
}

func validateReconcileRevision(slug string, entry ReconcileRevision) error {
	if entry.SchemaVersion != ReconcileRevisionSchemaVersion {
		return fmt.Errorf("unsupported schema_version %d (expected %d)", entry.SchemaVersion, ReconcileRevisionSchemaVersion)
	}
	if entry.EntryID == "" || entry.FeatureSlug != slug || entry.RawReconcileVerdict == "" || entry.ReasonCode == "" {
		return fmt.Errorf("required field is empty or mismatched")
	}
	if entry.ValidationRefs == nil {
		return fmt.Errorf("validation_refs is a required array")
	}
	if !validReviewVerdict(entry.ReviewVerdict) {
		return fmt.Errorf("review_verdict %q is not recognized", entry.ReviewVerdict)
	}
	if !validActionTaken(entry.ActionTaken) {
		return fmt.Errorf("action_taken %q is not recognized", entry.ActionTaken)
	}
	if entry.FinalFeatureState != "" && !ValidFeatureState(entry.FinalFeatureState) {
		return fmt.Errorf("final_feature_state %q is not recognized", entry.FinalFeatureState)
	}
	return nil
}

func marshalReconcileRevisionLine(entry ReconcileRevision) ([]byte, error) {
	entry = normalizeReconcileRevision(entry)
	return json.Marshal(revisionOrderedMap(entry))
}

func canonicalRevisionIdentityJSON(entry ReconcileRevision) ([]byte, error) {
	m := revisionOrderedMap(entry)
	delete(m, "entry_id")
	return json.Marshal(m)
}

func revisionOrderedMap(entry ReconcileRevision) map[string]any {
	m := map[string]any{
		"schema_version":        entry.SchemaVersion,
		"entry_id":              entry.EntryID,
		"feature_slug":          entry.FeatureSlug,
		"evidence_attempt_id":   entry.EvidenceAttemptID,
		"raw_reconcile_verdict": entry.RawReconcileVerdict,
		"review_verdict":        entry.ReviewVerdict,
		"final_feature_state":   entry.FinalFeatureState,
		"action_taken":          entry.ActionTaken,
		"reason_code":           entry.ReasonCode,
		"validation_refs":       revisionValidationRefs(entry.ValidationRefs),
	}
	if entry.SupersedesEntryID != "" {
		m["supersedes_entry_id"] = entry.SupersedesEntryID
	}
	if entry.Refs != nil && !entry.Refs.empty() {
		m["refs"] = refsOrderedMap(*entry.Refs)
	}
	return m
}

func revisionValidationRefs(refs []ValidationRef) []map[string]any {
	if refs == nil {
		refs = []ValidationRef{}
	}
	out := make([]map[string]any, 0, len(refs))
	for _, r := range refs {
		out = append(out, map[string]any{"kind": r.Kind, "value": r.Value, "result": r.Result})
	}
	return out
}

func validReviewVerdict(v ReconcileReviewVerdict) bool {
	switch v {
	case ReviewVerdictConfirmed, ReviewVerdictFalsePositive, ReviewVerdictFalseNegative, ReviewVerdictInconclusive, ReviewVerdictDeferred:
		return true
	default:
		return false
	}
}

func validActionTaken(v ReconcileActionTaken) bool {
	switch v {
	case ReconcileActionNone, ReconcileActionConfirmedRetired, ReconcileActionReapplied, ReconcileActionReappliedAndRecorded, ReconcileActionImplemented, ReconcileActionDeferred, ReconcileActionSkipped, ReconcileActionCleanupNeeded:
		return true
	default:
		return false
	}
}

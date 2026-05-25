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
	ReconcileEvidenceSchemaVersion = 1
	reconcileEvidenceFileName      = "reconcile-evidence.jsonl"
)

// ErrMalformedEvidence signals that reconcile-evidence.jsonl failed JSONL,
// schema, enum, or duplicate-ID validation. Writers refuse to append to this
// error class while status.json remains the current state source.
var ErrMalformedEvidence = errors.New("reconcile-evidence.jsonl: malformed evidence")

type ReconcileEvidencePhase string

const (
	EvidencePhase1  ReconcileEvidencePhase = "phase-1"
	EvidencePhase15 ReconcileEvidencePhase = "phase-1.5"
	EvidencePhase2  ReconcileEvidencePhase = "phase-2"
	EvidencePhase3  ReconcileEvidencePhase = "phase-3"
	EvidencePhase35 ReconcileEvidencePhase = "phase-3.5"
	EvidencePhase4  ReconcileEvidencePhase = "phase-4"
)

type ReconcileEvidenceKind string

const (
	EvidenceKindReverseApply          ReconcileEvidenceKind = "reverse-apply"
	EvidenceKindPatchIDMatch          ReconcileEvidenceKind = "patch-id-match"
	EvidenceKindRecipeOperationMatch  ReconcileEvidenceKind = "recipe-operation-match"
	EvidenceKindProviderSemantic      ReconcileEvidenceKind = "provider-semantic"
	EvidenceKindForwardApply          ReconcileEvidenceKind = "forward-apply"
	EvidenceKindFileNovelty           ReconcileEvidenceKind = "file-novelty"
	EvidenceKindHunkOverlap           ReconcileEvidenceKind = "hunk-overlap"
	EvidenceKindBlockedClassification ReconcileEvidenceKind = "blocked-classification"
	EvidenceKindPathRestructure       ReconcileEvidenceKind = "path-restructure"
	EvidenceKindManualReview          ReconcileEvidenceKind = "manual-review"
	EvidenceKindUnknown               ReconcileEvidenceKind = "unknown"
)

type ReconcileEvidenceConfidence string

const (
	EvidenceConfidenceHigh    ReconcileEvidenceConfidence = "high"
	EvidenceConfidenceMedium  ReconcileEvidenceConfidence = "medium"
	EvidenceConfidenceLow     ReconcileEvidenceConfidence = "low"
	EvidenceConfidenceUnknown ReconcileEvidenceConfidence = "unknown"
)

type ReconcileEvidenceMatchOrigin string

const (
	EvidenceMatchOriginUpstream       ReconcileEvidenceMatchOrigin = "upstream"
	EvidenceMatchOriginFork           ReconcileEvidenceMatchOrigin = "fork"
	EvidenceMatchOriginSiblingFeature ReconcileEvidenceMatchOrigin = "sibling-feature"
	EvidenceMatchOriginUnknown        ReconcileEvidenceMatchOrigin = "unknown"
	EvidenceMatchOriginMixed          ReconcileEvidenceMatchOrigin = "mixed"
)

type ReconcileEvidencePresence string

const (
	EvidencePresencePresent    ReconcileEvidencePresence = "present"
	EvidencePresenceAbsent     ReconcileEvidencePresence = "absent"
	EvidencePresenceUnknown    ReconcileEvidencePresence = "unknown"
	EvidencePresenceNotChecked ReconcileEvidencePresence = "not-checked"
)

type EvidenceRefs struct {
	PatchGenerationID    string `json:"patch_generation_id,omitempty"`
	PatchGenerationsPath string `json:"patch_generations_path,omitempty"`
	Anchors              string `json:"anchors,omitempty"`
	Fingerprints         string `json:"fingerprints,omitempty"`
	Relations            string `json:"relations,omitempty"`
	VectorManifest       string `json:"vector_manifest,omitempty"`
}

type ReconcileEvidence struct {
	SchemaVersion        int                          `json:"schema_version"`
	FeatureSlug          string                       `json:"feature_slug"`
	AttemptID            string                       `json:"attempt_id"`
	UpstreamRef          string                       `json:"upstream_ref"`
	UpstreamCommit       string                       `json:"upstream_commit"`
	BaseCommit           string                       `json:"base_commit"`
	RawReconcileVerdict  string                       `json:"raw_reconcile_verdict"`
	Phase                ReconcileEvidencePhase       `json:"phase"`
	EvidenceKind         ReconcileEvidenceKind        `json:"evidence_kind"`
	Confidence           ReconcileEvidenceConfidence  `json:"confidence"`
	MatchedPaths         []string                     `json:"matched_paths"`
	MatchedOperations    []string                     `json:"matched_operations"`
	MatchOrigin          ReconcileEvidenceMatchOrigin `json:"match_origin"`
	UpstreamCommitRefs   []string                     `json:"upstream_commit_refs"`
	PreReconcilePresence ReconcileEvidencePresence    `json:"pre_reconcile_presence"`
	RequiresConfirmation bool                         `json:"requires_confirmation"`
	ReasonCode           string                       `json:"reason_code"`
	GitPatchID           string                       `json:"git_patch_id,omitempty"`
	GitPatchIDAlgorithm  string                       `json:"git_patch_id_algorithm,omitempty"`
	MatchedUpstreamSHA   string                       `json:"matched_upstream_sha,omitempty"`
	ScannedRange         string                       `json:"scanned_range,omitempty"`
	ScannedCount         int                          `json:"scanned_count,omitempty"`
	AdditionalMatches    []string                     `json:"additional_matches,omitempty"`
	Refs                 *EvidenceRefs                `json:"refs,omitempty"`
}

func (s *Store) ReconcileEvidencePath(slug string) string {
	return filepath.Join(s.featureArtifactsDir(slug), reconcileEvidenceFileName)
}

func ComputeAttemptID(entry ReconcileEvidence) string {
	entry = normalizeReconcileEvidence(entry)
	entry.AttemptID = ""
	data, _ := canonicalEvidenceIdentityJSON(entry)
	sum := sha256.Sum256(data)
	return "re_" + hex.EncodeToString(sum[:])[:12]
}

func PatchIDMatchEvidenceFields(entry ReconcileEvidence, match PatchIDMatch) ReconcileEvidence {
	entry.EvidenceKind = EvidenceKindPatchIDMatch
	entry.GitPatchID = match.OurPatchID
	entry.GitPatchIDAlgorithm = PatchIDAlgorithmStable
	entry.MatchedUpstreamSHA = match.MatchedUpstreamSHA
	entry.ScannedRange = match.ScannedRange
	entry.ScannedCount = match.ScannedCount
	entry.AdditionalMatches = append([]string(nil), match.AdditionalMatches...)
	return entry
}

func AppendReconcileEvidence(s *Store, slug string, entry ReconcileEvidence) error {
	path := s.ReconcileEvidencePath(slug)
	if _, err := LoadReconcileEvidence(s, slug); err != nil {
		return err
	}
	entry.FeatureSlug = slug
	entry = normalizeReconcileEvidence(entry)
	if entry.AttemptID == "" {
		entry.AttemptID = ComputeAttemptID(entry)
	}
	if err := validateReconcileEvidence(slug, entry); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedEvidence, err)
	}
	line, err := marshalReconcileEvidenceLine(entry)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	var existing []byte
	if data, err := os.ReadFile(path); err == nil {
		existing = data
		for _, oldLine := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
			old, err := decodeReconcileEvidenceLine(oldLine)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrMalformedEvidence, err)
			}
			if old.AttemptID == entry.AttemptID {
				oldBytes, _ := marshalReconcileEvidenceLine(old)
				if bytes.Equal(oldBytes, bytes.TrimSuffix(line, []byte("\n"))) {
					return nil
				}
				return fmt.Errorf("%w: attempt_id %q has differing payload", ErrMalformedEvidence, entry.AttemptID)
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

func LoadReconcileEvidence(s *Store, slug string) ([]ReconcileEvidence, error) {
	path := s.ReconcileEvidencePath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []ReconcileEvidence{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return []ReconcileEvidence{}, nil
	}
	if data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("%w: line %d: final object is not newline-terminated", ErrMalformedEvidence, bytes.Count(data, []byte("\n"))+1)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	entries := make([]ReconcileEvidence, 0, len(lines))
	seen := map[string][]byte{}
	for i, line := range lines {
		lineNo := i + 1
		if len(bytes.TrimSpace(line)) == 0 {
			return nil, fmt.Errorf("%w: line %d: empty line", ErrMalformedEvidence, lineNo)
		}
		entry, err := decodeReconcileEvidenceLine(line)
		if err != nil {
			return nil, fmt.Errorf("%w: line %d: %v", ErrMalformedEvidence, lineNo, err)
		}
		entry = normalizeReconcileEvidence(entry)
		if err := validateReconcileEvidence(slug, entry); err != nil {
			return nil, fmt.Errorf("%w: line %d: %w", ErrMalformedEvidence, lineNo, err)
		}
		canon, _ := marshalReconcileEvidenceLine(entry)
		if prev, ok := seen[entry.AttemptID]; ok {
			if !bytes.Equal(prev, canon) {
				return nil, fmt.Errorf("%w: line %d: duplicate attempt_id %q has differing payload", ErrMalformedEvidence, lineNo, entry.AttemptID)
			}
			continue
		}
		seen[entry.AttemptID] = canon
		entries = append(entries, entry)
	}
	return entries, nil
}

func normalizeReconcileEvidence(entry ReconcileEvidence) ReconcileEvidence {
	if entry.SchemaVersion == 0 {
		entry.SchemaVersion = ReconcileEvidenceSchemaVersion
	}
	entry.MatchedPaths = sortedStrings(entry.MatchedPaths)
	entry.MatchedOperations = sortedStrings(entry.MatchedOperations)
	entry.UpstreamCommitRefs = sortedStrings(entry.UpstreamCommitRefs)
	entry.AdditionalMatches = sortedStrings(entry.AdditionalMatches)
	if entry.Refs != nil && entry.Refs.empty() {
		entry.Refs = nil
	}
	return entry
}

func decodeReconcileEvidenceLine(line []byte) (ReconcileEvidence, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return ReconcileEvidence{}, err
	}
	allowed := map[string]bool{
		"schema_version": true, "feature_slug": true, "attempt_id": true, "upstream_ref": true, "upstream_commit": true,
		"base_commit": true, "raw_reconcile_verdict": true, "phase": true, "evidence_kind": true, "confidence": true,
		"matched_paths": true, "matched_operations": true, "match_origin": true, "upstream_commit_refs": true,
		"pre_reconcile_presence": true, "requires_confirmation": true, "reason_code": true, "git_patch_id": true,
		"git_patch_id_algorithm": true, "matched_upstream_sha": true, "scanned_range": true, "scanned_count": true,
		"additional_matches": true, "refs": true,
	}
	for k := range raw {
		if !allowed[k] {
			return ReconcileEvidence{}, fmt.Errorf("unknown field %q", k)
		}
	}
	for _, k := range []string{"schema_version", "feature_slug", "attempt_id", "upstream_ref", "upstream_commit", "base_commit", "raw_reconcile_verdict", "phase", "evidence_kind", "confidence", "matched_paths", "matched_operations", "match_origin", "upstream_commit_refs", "pre_reconcile_presence", "requires_confirmation", "reason_code"} {
		if _, ok := raw[k]; !ok {
			return ReconcileEvidence{}, fmt.Errorf("missing required field %q", k)
		}
	}
	var entry ReconcileEvidence
	dec = json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entry); err != nil {
		return ReconcileEvidence{}, err
	}
	return entry, nil
}

func validateReconcileEvidence(slug string, entry ReconcileEvidence) error {
	if entry.SchemaVersion != ReconcileEvidenceSchemaVersion {
		return fmt.Errorf("unsupported schema_version %d (expected %d)", entry.SchemaVersion, ReconcileEvidenceSchemaVersion)
	}
	if entry.FeatureSlug != slug {
		return fmt.Errorf("feature_slug %q does not match %q", entry.FeatureSlug, slug)
	}
	if entry.AttemptID == "" {
		return fmt.Errorf("attempt_id is required")
	}
	if entry.UpstreamRef == "" || entry.UpstreamCommit == "" || entry.BaseCommit == "" || entry.RawReconcileVerdict == "" || entry.ReasonCode == "" {
		return fmt.Errorf("required string field is empty")
	}
	if entry.MatchedPaths == nil || entry.MatchedOperations == nil || entry.UpstreamCommitRefs == nil {
		return fmt.Errorf("matched_paths, matched_operations, and upstream_commit_refs are required arrays")
	}
	if !validEvidencePhase(entry.Phase) {
		return fmt.Errorf("phase %q is not recognized", entry.Phase)
	}
	if !validEvidenceKind(entry.EvidenceKind) {
		return fmt.Errorf("evidence_kind %q is not recognized", entry.EvidenceKind)
	}
	if !validEvidenceConfidence(entry.Confidence) {
		return fmt.Errorf("confidence %q is not recognized", entry.Confidence)
	}
	if !validEvidenceMatchOrigin(entry.MatchOrigin) {
		return fmt.Errorf("match_origin %q is not recognized", entry.MatchOrigin)
	}
	if !validEvidencePresence(entry.PreReconcilePresence) {
		return fmt.Errorf("pre_reconcile_presence %q is not recognized", entry.PreReconcilePresence)
	}
	if entry.EvidenceKind == EvidenceKindPatchIDMatch {
		if entry.GitPatchID == "" || entry.GitPatchIDAlgorithm != PatchIDAlgorithmStable || entry.MatchedUpstreamSHA == "" || entry.ScannedRange == "" || entry.ScannedCount == 0 {
			return fmt.Errorf("patch-id-match evidence requires git_patch_id, git_patch_id_algorithm=%q, matched_upstream_sha, scanned_range, and scanned_count", PatchIDAlgorithmStable)
		}
	}
	return nil
}

func marshalReconcileEvidenceLine(entry ReconcileEvidence) ([]byte, error) {
	entry = normalizeReconcileEvidence(entry)
	return json.Marshal(evidenceOrderedMap(entry))
}

func canonicalEvidenceIdentityJSON(entry ReconcileEvidence) ([]byte, error) {
	m := evidenceOrderedMap(entry)
	delete(m, "attempt_id")
	return json.Marshal(m)
}

func evidenceOrderedMap(entry ReconcileEvidence) map[string]any {
	m := map[string]any{
		"schema_version":         entry.SchemaVersion,
		"feature_slug":           entry.FeatureSlug,
		"attempt_id":             entry.AttemptID,
		"upstream_ref":           entry.UpstreamRef,
		"upstream_commit":        entry.UpstreamCommit,
		"base_commit":            entry.BaseCommit,
		"raw_reconcile_verdict":  entry.RawReconcileVerdict,
		"phase":                  entry.Phase,
		"evidence_kind":          entry.EvidenceKind,
		"confidence":             entry.Confidence,
		"matched_paths":          nonNilStrings(entry.MatchedPaths),
		"matched_operations":     nonNilStrings(entry.MatchedOperations),
		"match_origin":           entry.MatchOrigin,
		"upstream_commit_refs":   nonNilStrings(entry.UpstreamCommitRefs),
		"pre_reconcile_presence": entry.PreReconcilePresence,
		"requires_confirmation":  entry.RequiresConfirmation,
		"reason_code":            entry.ReasonCode,
	}
	if entry.GitPatchID != "" {
		m["git_patch_id"] = entry.GitPatchID
	}
	if entry.GitPatchIDAlgorithm != "" {
		m["git_patch_id_algorithm"] = entry.GitPatchIDAlgorithm
	}
	if entry.MatchedUpstreamSHA != "" {
		m["matched_upstream_sha"] = entry.MatchedUpstreamSHA
	}
	if entry.ScannedRange != "" {
		m["scanned_range"] = entry.ScannedRange
	}
	if entry.ScannedCount != 0 {
		m["scanned_count"] = entry.ScannedCount
	}
	if len(entry.AdditionalMatches) > 0 {
		m["additional_matches"] = nonNilStrings(entry.AdditionalMatches)
	}
	if entry.Refs != nil && !entry.Refs.empty() {
		m["refs"] = refsOrderedMap(*entry.Refs)
	}
	return m
}

func refsOrderedMap(refs EvidenceRefs) map[string]any {
	m := map[string]any{}
	if refs.PatchGenerationID != "" {
		m["patch_generation_id"] = refs.PatchGenerationID
	}
	if refs.PatchGenerationsPath != "" {
		m["patch_generations_path"] = refs.PatchGenerationsPath
	}
	if refs.Anchors != "" {
		m["anchors"] = refs.Anchors
	}
	if refs.Fingerprints != "" {
		m["fingerprints"] = refs.Fingerprints
	}
	if refs.Relations != "" {
		m["relations"] = refs.Relations
	}
	if refs.VectorManifest != "" {
		m["vector_manifest"] = refs.VectorManifest
	}
	return m
}

func (r EvidenceRefs) empty() bool {
	return r.PatchGenerationID == "" && r.PatchGenerationsPath == "" && r.Anchors == "" && r.Fingerprints == "" && r.Relations == "" && r.VectorManifest == ""
}

func sortedStrings(in []string) []string {
	if in == nil {
		return nil
	}
	if len(in) == 0 {
		return []string{}
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func validEvidencePhase(v ReconcileEvidencePhase) bool {
	switch v {
	case EvidencePhase1, EvidencePhase15, EvidencePhase2, EvidencePhase3, EvidencePhase35, EvidencePhase4:
		return true
	default:
		return false
	}
}

func validEvidenceKind(v ReconcileEvidenceKind) bool {
	switch v {
	case EvidenceKindReverseApply, EvidenceKindPatchIDMatch, EvidenceKindRecipeOperationMatch, EvidenceKindProviderSemantic, EvidenceKindForwardApply, EvidenceKindFileNovelty, EvidenceKindHunkOverlap, EvidenceKindBlockedClassification, EvidenceKindPathRestructure, EvidenceKindManualReview, EvidenceKindUnknown:
		return true
	default:
		return false
	}
}

func validEvidenceConfidence(v ReconcileEvidenceConfidence) bool {
	switch v {
	case EvidenceConfidenceHigh, EvidenceConfidenceMedium, EvidenceConfidenceLow, EvidenceConfidenceUnknown:
		return true
	default:
		return false
	}
}

func validEvidenceMatchOrigin(v ReconcileEvidenceMatchOrigin) bool {
	switch v {
	case EvidenceMatchOriginUpstream, EvidenceMatchOriginFork, EvidenceMatchOriginSiblingFeature, EvidenceMatchOriginUnknown, EvidenceMatchOriginMixed:
		return true
	default:
		return false
	}
}

func validEvidencePresence(v ReconcileEvidencePresence) bool {
	switch v {
	case EvidencePresencePresent, EvidencePresenceAbsent, EvidencePresenceUnknown, EvidencePresenceNotChecked:
		return true
	default:
		return false
	}
}

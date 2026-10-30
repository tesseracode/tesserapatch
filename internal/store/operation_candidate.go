package store

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

const (
	operationCandidateStatusPending  = "pending"
	operationCandidateStatusStale    = "stale"
	operationCandidateStatusRejected = "rejected"
	operationCandidateStatusAccepted = "accepted"
)

func CarryForwardOperationCandidate(next, loaded ReconcileSummary) ReconcileSummary {
	next.OperationCandidate = CloneOperationCandidateSummary(loaded.OperationCandidate)
	return next
}

func CloneOperationCandidateSummary(value *OperationCandidateSummary) *OperationCandidateSummary {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func ValidateOperationCandidateSummary(value *OperationCandidateSummary) error {
	if value == nil {
		return nil
	}
	if value.CandidateID == "" || value.Path == "" || value.Status == "" || value.SourceFeatureState == "" {
		return fmt.Errorf("operation candidate summary fields are required")
	}
	if len(value.CandidateID) != 15 || !strings.HasPrefix(value.CandidateID, "rc_") {
		return fmt.Errorf("operation candidate summary candidate_id must match rc_<12hex>")
	}
	for _, r := range strings.TrimPrefix(value.CandidateID, "rc_") {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return fmt.Errorf("operation candidate summary candidate_id must be lowercase hex")
		}
	}
	switch value.Status {
	case operationCandidateStatusPending, operationCandidateStatusStale, operationCandidateStatusRejected, operationCandidateStatusAccepted:
	default:
		return fmt.Errorf("operation candidate summary has invalid status %q", value.Status)
	}
	switch FeatureState(value.SourceFeatureState) {
	case StateApplied, StateBlocked:
	default:
		return fmt.Errorf("operation candidate summary has invalid source_feature_state %q", value.SourceFeatureState)
	}
	if strings.Contains(value.Path, "\\") || strings.HasPrefix(value.Path, "/") || strings.Contains(value.Path, ":") {
		return fmt.Errorf("operation candidate summary path must be slash-normalized and repository-relative")
	}
	if path.Clean(value.Path) != value.Path || value.Path == "." || value.Path == "" || strings.HasPrefix(value.Path, "../") || strings.Contains(value.Path, "/../") {
		return fmt.Errorf("operation candidate summary path must be canonical")
	}
	if !fs.ValidPath(value.Path) {
		return fmt.Errorf("operation candidate summary path is not a valid relative path")
	}
	return nil
}

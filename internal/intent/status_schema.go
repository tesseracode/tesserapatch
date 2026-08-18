package intent

import (
	"encoding/json"
	"time"
)

// The types below are a deliberate, local mirror of `store.FeatureStatus`
// and every shape reachable from it. `internal/intent` must not import
// `internal/store` (AVP-087, AVP-150), yet `status-malformed` is defined as
// "the bytes would not decode into the tpatch status document" — not merely
// "the `state` member is not a string". Decoding only `state` accepted, for
// example, `{"state":"defined","apply":7}`, which `store.LoadFeatureStatus`
// rejects.
//
// The mirror is kept honest mechanically rather than by review: the
// AST parity guard (`TestAVPStatusSchemaParity`, AVP-093's sibling in
// `status_schema_guard_test.go`) parses `internal/store/types.go` and
// `internal/store/status.go`, walks `FeatureStatus` transitively, and fails
// when a field, JSON name, `omitempty` flag or normalized Go type drifts
// from the mirror below. A sensitivity fixture proves the guard fails on a
// mutated store shape.
//
// Unknown JSON keys are deliberately still accepted: `store` decodes with a
// plain `json.Unmarshal`, so the mirror does too (`DisallowUnknownFields` is
// never set). Forward-compatible documents written by a newer tpatch stay
// readable, exactly as they are for every other reader in the codebase.

type statusEvidenceRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type statusDivergenceDetail struct {
	Path            string `json:"path"`
	DivergentReason string `json:"divergent_reason"`
}

type statusRejection struct {
	Reason     string              `json:"reason"`
	Note       string              `json:"note"`
	Actor      string              `json:"actor"`
	Related    string              `json:"related,omitempty"`
	Evidence   []statusEvidenceRef `json:"evidence,omitempty"`
	RejectedAt time.Time           `json:"rejected_at"`
	PriorState string              `json:"prior_state"`
}

type statusRejectionHistoryEntry struct {
	RejectedAt        time.Time                `json:"rejected_at"`
	RejectedBy        string                   `json:"rejected_by"`
	Reason            string                   `json:"reason"`
	RejectNote        string                   `json:"reject_note"`
	PriorState        string                   `json:"prior_state,omitempty"`
	Related           string                   `json:"related,omitempty"`
	RejectEvidence    []statusEvidenceRef      `json:"reject_evidence,omitempty"`
	ReopenedAt        time.Time                `json:"reopened_at"`
	ReopenedBy        string                   `json:"reopened_by"`
	ReopenNote        string                   `json:"reopen_note"`
	ReopenEvidence    []statusEvidenceRef      `json:"reopen_evidence,omitempty"`
	EvidenceIntegrity string                   `json:"evidence_integrity,omitempty"`
	DivergenceDetail  []statusDivergenceDetail `json:"divergence_detail,omitempty"`
}

type statusVerifyRecord struct {
	VerifiedAt         string            `json:"verified_at"`
	Passed             bool              `json:"passed"`
	RecipeHashAtVerify string            `json:"recipe_hash_at_verify,omitempty"`
	PatchHashAtVerify  string            `json:"patch_hash_at_verify,omitempty"`
	ParentSnapshot     map[string]string `json:"parent_snapshot,omitempty"`
}

type statusDependency struct {
	Slug        string `json:"slug"`
	Kind        string `json:"kind"`
	SatisfiedBy string `json:"satisfied_by,omitempty"`
}

type statusApplySummary struct {
	PreparedAt  string `json:"prepared_at,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	BaseCommit  string `json:"base_commit,omitempty"`
	HasPatch    bool   `json:"has_patch,omitempty"`
	HasRecipe   bool   `json:"has_recipe,omitempty"`
}

type statusPatchIDMatch struct {
	OurPatchID         string   `json:"our_patch_id"`
	MatchedUpstreamSHA string   `json:"matched_upstream_sha"`
	AdditionalMatches  []string `json:"additional_matches,omitempty"`
	ScannedRange       string   `json:"scanned_range"`
	ScannedCount       int      `json:"scanned_count"`
}

type statusReconcileSummary struct {
	AttemptedAt    string              `json:"attempted_at,omitempty"`
	UpstreamRef    string              `json:"upstream_ref,omitempty"`
	UpstreamCommit string              `json:"upstream_commit,omitempty"`
	Outcome        string              `json:"outcome,omitempty"`
	ReviewVerdict  string              `json:"review_verdict,omitempty"`
	ShadowPath     string              `json:"shadow_path,omitempty"`
	ResolveSession string              `json:"resolve_session_id,omitempty"`
	ResolvedFiles  int                 `json:"resolved_files,omitempty"`
	FailedFiles    int                 `json:"failed_files,omitempty"`
	SkippedFiles   int                 `json:"skipped_files,omitempty"`
	Labels         []string            `json:"labels,omitempty"`
	PatchIDMatch   *statusPatchIDMatch `json:"patch_id_match,omitempty"`
}

// statusDocument mirrors store.FeatureStatus field for field.
type statusDocument struct {
	ID               string                        `json:"id"`
	Slug             string                        `json:"slug"`
	Title            string                        `json:"title"`
	State            string                        `json:"state"`
	Compatibility    string                        `json:"compatibility"`
	RequestedAt      string                        `json:"requested_at"`
	UpdatedAt        string                        `json:"updated_at"`
	LastCommand      string                        `json:"last_command"`
	Notes            string                        `json:"notes,omitempty"`
	Apply            statusApplySummary            `json:"apply"`
	Reconcile        statusReconcileSummary        `json:"reconcile"`
	DependsOn        []statusDependency            `json:"depends_on,omitempty"`
	Verify           *statusVerifyRecord           `json:"verify,omitempty"`
	Rejection        *statusRejection              `json:"rejection,omitempty"`
	RejectionHistory []statusRejectionHistoryEntry `json:"rejection_history,omitempty"`
}

// decodeStatusDocument reports whether data is a JSON object that decodes
// cleanly into the full mirrored status schema, and returns the decoded
// lifecycle state literal. A false result is exactly the `status-malformed`
// population of §9.4.2 — no partial acceptance, no per-field salvage.
func decodeStatusDocument(data []byte) (string, bool) {
	if !jsonObject(data) {
		return "", false
	}
	var document statusDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return "", false
	}
	return document.State, true
}

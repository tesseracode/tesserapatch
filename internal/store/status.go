package store

import "time"

// This file carries the `rejected` lifecycle sub-model that hangs off
// FeatureStatus (`internal/store/types.go`). See
// docs/prds/PRD-rejected-feature-state.md §6 and
// docs/adrs/ADR-031-rejected-feature-state-data-model.md D1/D3/D5.

// EvidenceRef is one content-hashed evidence reference attached to a
// rejection (or to a reopen). `Path` is the operator-supplied path,
// normalized to a repo-relative, forward-slash form (ADR-031 D3
// addendum); `SHA256` is the lowercase-hex ASCII encoding of the raw
// SHA-256 digest of that file's bytes at the moment the record was
// written (`^[0-9a-f]{64}$` — never uppercase hex, never base64).
//
// Storing the hash alongside the path is what makes the audit trail
// survive later in-place rewrites of the cited file: `analysis.md`,
// `artifacts/apply-recipe.json`, `artifacts/post-apply.patch` and
// friends are all truncating writes on re-run, so evidence integrity is
// *detected* at reopen time rather than *assumed* from path location.
type EvidenceRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Evidence-integrity verdicts recorded on a reopen history entry
// (ADR-031 D3 addendum). `clean` is the verified case and is
// deliberately OMITTED from the serialized record (the field is
// `omitempty`), so a clean reopen round-trips without the field at all.
const (
	EvidenceIntegrityClean     = "clean"
	EvidenceIntegrityDivergent = "divergent"
)

// Closed taxonomy of per-element divergence reasons (PRD §6,
// "Reopen-time integrity check"). Exactly one applies per divergent
// evidence element.
const (
	// DivergentReasonHashMismatch — path safety re-passed, still a
	// regular in-repo file, but the content hash differs.
	DivergentReasonHashMismatch = "hash-mismatch"
	// DivergentReasonMissing — the path no longer resolves to any file.
	DivergentReasonMissing = "missing"
	// DivergentReasonNonRegular — the path now resolves to something
	// that is not a regular file (directory, device, socket).
	DivergentReasonNonRegular = "non-regular"
	// DivergentReasonPathSafetyFailed — the path-safety re-check itself
	// fails at reopen time (absolute path, `..` escape, or a symlink
	// resolving outside the repository root). No hash is EVER attempted
	// in this case: the file's bytes are never read, so a divergence
	// entry here can never itself become a path-safety violation.
	DivergentReasonPathSafetyFailed = "path-safety-failed-at-reopen"
	// DivergentReasonUnreadable — path safety re-passed and the file is
	// still a regular in-repo file, but it cannot be opened.
	DivergentReasonUnreadable = "unreadable"
)

// Rejection history actions (ADR-031 D5). The history array is
// append-only and unbounded: every `reject` appends one entry with
// action `reject`, every `reopen` appends one entry with action
// `reopen`. No entry is ever overwritten or truncated.
const (
	RejectionActionReject = "reject"
	RejectionActionReopen = "reopen"
)

// DivergenceDetail names one evidence element whose reopen-time
// integrity re-check did not pass, together with its reason drawn from
// the closed taxonomy above.
type DivergenceDetail struct {
	Path            string `json:"path"`
	DivergentReason string `json:"divergent_reason"`
}

// RejectionHistoryEntry is one append-only audit entry in
// RejectionStatus.History. Reject entries carry `reason` (and the
// `prior_state` the feature was rejected from); reopen entries carry the
// historical-evidence verification verdict (`evidence_integrity` plus
// `divergence_detail` when divergent). Both carry the operator's
// mandatory `note`, the resolved `actor`, and the UTC `timestamp`.
type RejectionHistoryEntry struct {
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Timestamp time.Time `json:"timestamp"`
	Note      string    `json:"note"`

	// Reason is set on `reject` entries only (closed enum, see
	// rejection_reason.go).
	Reason string `json:"reason,omitempty"`

	// PriorState is set on `reject` entries only: the FeatureState the
	// feature held immediately before the rejection.
	PriorState FeatureState `json:"prior_state,omitempty"`

	// Related is the optional free-form `--related` reference (a feature
	// slug or a `GH#N` pointer). Not validated against the store.
	Related string `json:"related,omitempty"`

	// Evidence holds the NEW evidence attached by this action: the
	// mandatory `--evidence` list on `reject`, or the optional
	// `--evidence` list on `reopen` (which may legitimately be empty —
	// the REOPEN-EVIDENCE-OPTIONAL contract, PRD §5 rev-3).
	Evidence []EvidenceRef `json:"evidence,omitempty"`

	// EvidenceIntegrity is set on `reopen` entries only, and ONLY when
	// the verdict is `divergent`. A clean verification omits the field
	// entirely (ADR-031 D3 addendum).
	EvidenceIntegrity string `json:"evidence_integrity,omitempty"`

	// DivergenceDetail enumerates the divergent historical evidence
	// elements. Non-empty exactly when EvidenceIntegrity is `divergent`.
	DivergenceDetail []DivergenceDetail `json:"divergence_detail,omitempty"`
}

// RejectionStatus is the current rejection record for a feature plus its
// full append-only history. It is written by `tpatch reject` and updated
// (never cleared) by `tpatch reopen`.
//
// The top-level fields describe the MOST RECENT rejection; History is
// the complete, unbounded audit log of every reject/reopen action ever
// taken on this feature, oldest first.
type RejectionStatus struct {
	Reason     string                  `json:"reason"`
	Note       string                  `json:"note"`
	Actor      string                  `json:"actor"`
	Related    string                  `json:"related,omitempty"`
	Evidence   []EvidenceRef           `json:"evidence,omitempty"`
	RejectedAt time.Time               `json:"rejected_at"`
	PriorState FeatureState            `json:"prior_state"`
	History    []RejectionHistoryEntry `json:"history,omitempty"`
}

// RejectableStates is the shared, single-source-of-truth set of source
// states from which `tpatch reject` is permitted (ADR-031 D4
// Alternative 2, PRD §5). Rejection is a PRE-implementation terminal
// only: `implementing` onward — including `blocked` and
// `upstream_merged` — is refused outright with exit code 3. Post-
// implementation retirement is explicitly out of scope (ADR-031 D6).
var RejectableStates = map[FeatureState]struct{}{
	StateRequested: {},
	StateAnalyzed:  {},
	StateDefined:   {},
}

// IsRejectableState reports whether `tpatch reject` may fire from the
// given source state. Callers that need the refusal message should
// render RejectableStateList().
func IsRejectableState(state FeatureState) bool {
	_, ok := RejectableStates[state]
	return ok
}

// RejectableStateList returns the reject-eligible states in a stable,
// human-facing order for error messages and help text.
func RejectableStateList() []FeatureState {
	return []FeatureState{StateRequested, StateAnalyzed, StateDefined}
}

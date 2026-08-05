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

// DivergenceDetail names one evidence element whose reopen-time
// integrity re-check did not pass, together with its reason drawn from
// the closed taxonomy above.
type DivergenceDetail struct {
	Path            string `json:"path"`
	DivergentReason string `json:"divergent_reason"`
}

// RejectionHistoryEntry is one COMPLETED reject→reopen cycle
// (PRD §6 `history`, ADR-031 D5). It is appended by `tpatch reopen` and
// by nothing else: `tpatch reject` writes the live record to
// FeatureStatus.Rejection instead, and the reopen that closes the cycle
// snapshots those fields into the reject-half of this entry before
// clearing Rejection.
//
// The consequence, and the invariant the CLI enforces:
//
//	after N reject→reopen cycles  len(FeatureStatus.RejectionHistory) == N
//	while currently rejected      FeatureStatus.Rejection != nil
//	after a reopen                FeatureStatus.Rejection == nil
//
// Appending one entry per *action* (as an earlier draft did) would
// double-count every cycle and leave the live `Rejection` sub-object
// duplicating the most recent history entry — see ADR-031 D5.
type RejectionHistoryEntry struct {
	// ─── reject half: snapshotted from FeatureStatus.Rejection at
	// reopen time, so no rejection record is ever lost.
	RejectedAt time.Time `json:"rejected_at"`
	RejectedBy string    `json:"rejected_by"`
	Reason     string    `json:"reason"`
	RejectNote string    `json:"reject_note"`
	// PriorState is the FeatureState the feature held immediately
	// before the rejection that opened this cycle.
	PriorState FeatureState `json:"prior_state,omitempty"`
	// Related is the optional free-form `--related` reference recorded
	// at reject time (a feature slug or a `GH#N` pointer). Not
	// validated against the store.
	Related        string        `json:"related,omitempty"`
	RejectEvidence []EvidenceRef `json:"reject_evidence,omitempty"`

	// ─── reopen half: recorded by the reopen that closed the cycle.
	ReopenedAt time.Time `json:"reopened_at"`
	ReopenedBy string    `json:"reopened_by"`
	ReopenNote string    `json:"reopen_note"`
	// ReopenEvidence holds the optional `--evidence` list attached to
	// the reopen, which may legitimately be empty — the
	// REOPEN-EVIDENCE-OPTIONAL contract (PRD §5 rev-3).
	ReopenEvidence []EvidenceRef `json:"reopen_evidence,omitempty"`

	// EvidenceIntegrity is the reopen-time verdict on the historical
	// (reject-half) evidence, and is set ONLY when the verdict is
	// `divergent`. A clean verification omits the field entirely
	// (ADR-031 D3 addendum).
	EvidenceIntegrity string `json:"evidence_integrity,omitempty"`

	// DivergenceDetail enumerates the divergent historical evidence
	// elements. Non-empty exactly when EvidenceIntegrity is `divergent`.
	DivergenceDetail []DivergenceDetail `json:"divergence_detail,omitempty"`
}

// RejectionStatus is the LIVE rejection record for a feature: it is
// written by `tpatch reject` and cleared by `tpatch reopen`, which folds
// it into a RejectionHistoryEntry first so nothing is lost. A non-nil
// RejectionStatus therefore means "this feature is rejected right now";
// the completed-cycle audit log lives in
// FeatureStatus.RejectionHistory.
type RejectionStatus struct {
	Reason     string        `json:"reason"`
	Note       string        `json:"note"`
	Actor      string        `json:"actor"`
	Related    string        `json:"related,omitempty"`
	Evidence   []EvidenceRef `json:"evidence,omitempty"`
	RejectedAt time.Time     `json:"rejected_at"`
	PriorState FeatureState  `json:"prior_state"`
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

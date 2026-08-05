package store

import "strings"

// RejectionReason is the closed enum of `--reason` codes accepted by
// `tpatch reject` (ADR-031 D2 Alternative 1, PRD-rejected-feature-state
// §6). The vocabulary is deliberately finite so `status --json`
// consumers, the FEATURES.md renderer, and future duplicate-detection
// tooling can rely on a stable classification rather than free text.
//
// Adding an eighth value later is a small, backward-compatible change:
// an old binary simply never writes it, and a new binary reading an old
// file never sees it (ADR-031 D2 rationale).
type RejectionReason = string

// The seven authoritative reason codes (PRD §6 field table).
const (
	// ReasonNotABug — the reported behavior is working as intended.
	ReasonNotABug RejectionReason = "not-a-bug"
	// ReasonPremiseDisproved — the premise the request rested on was
	// empirically falsified.
	ReasonPremiseDisproved RejectionReason = "premise-disproved"
	// ReasonObsolete — the request no longer applies to the current
	// upstream.
	ReasonObsolete RejectionReason = "obsolete"
	// ReasonOutOfScope — legitimate, but outside what this fork patches.
	ReasonOutOfScope RejectionReason = "out-of-scope"
	// ReasonUnsafe — implementing it would introduce an unacceptable
	// safety/security risk.
	ReasonUnsafe RejectionReason = "unsafe"
	// ReasonDuplicate — another tracked feature already covers it.
	ReasonDuplicate RejectionReason = "duplicate"
	// ReasonSuperseded — a different approach replaced this one.
	ReasonSuperseded RejectionReason = "superseded"
)

// ValidRejectionReasons is the closed membership set. Kept as a map so
// validation is a single lookup; RejectionReasonList() is the ordered
// rendering used in error messages and help text.
var ValidRejectionReasons = map[RejectionReason]bool{
	ReasonNotABug:          true,
	ReasonPremiseDisproved: true,
	ReasonObsolete:         true,
	ReasonOutOfScope:       true,
	ReasonUnsafe:           true,
	ReasonDuplicate:        true,
	ReasonSuperseded:       true,
}

// IsValidRejectionReason reports whether r is one of the seven codes.
// Any other value (including the empty string) is a validation error,
// exit code 2 (ADR-031 D4 addendum).
func IsValidRejectionReason(r RejectionReason) bool {
	return ValidRejectionReasons[r]
}

// RejectionReasonList returns the reason codes in their canonical,
// stable order for deterministic error messages and `--help` output.
func RejectionReasonList() []RejectionReason {
	return []RejectionReason{
		ReasonNotABug,
		ReasonPremiseDisproved,
		ReasonObsolete,
		ReasonOutOfScope,
		ReasonUnsafe,
		ReasonDuplicate,
		ReasonSuperseded,
	}
}

// RejectionReasonsJoined renders the closed enum as a comma-separated
// list, matching the PRD §8 invalid-reason error envelope verbatim.
func RejectionReasonsJoined() string {
	return strings.Join(RejectionReasonList(), ", ")
}

package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ─── PRD-rejected-feature-state §9 — store-layer half of the matrix ──────────
//
// This file covers the data-model, reason-enum, actor-resolution and
// Rule 7 dependency-guard items. The CLI-observable half (transitions,
// evidence hashing, reopen, exit codes, --help golden strings) lives in
// internal/cli/reject_test.go.

// ─── §9 item: StateRejected is a first-class, valid FeatureState ─────────────

func TestRejected_IsValidFeatureState(t *testing.T) {
	if !ValidFeatureState(StateRejected) {
		t.Fatalf("StateRejected must be accepted by ValidFeatureState")
	}
	if StateRejected != "rejected" {
		t.Fatalf("wire value must be %q, got %q", "rejected", StateRejected)
	}
}

// The reject-eligible set is PRE-implementation only (ADR-031 D4/D6).
func TestRejected_RejectableStateSet(t *testing.T) {
	allowed := []FeatureState{StateRequested, StateAnalyzed, StateDefined}
	refused := []FeatureState{
		StateImplementing, StateApplied, StateActive, StateReconciling,
		StateReconcilingShadow, StateBlocked, StateUpstreamMerged, StateRejected,
	}
	for _, st := range allowed {
		if !IsRejectableState(st) {
			t.Errorf("state %q must be reject-eligible", st)
		}
	}
	for _, st := range refused {
		if IsRejectableState(st) {
			t.Errorf("state %q must NOT be reject-eligible", st)
		}
	}
	if got := RejectableStateList(); len(got) != len(allowed) {
		t.Fatalf("RejectableStateList len = %d, want %d", len(got), len(allowed))
	}
}

// ─── §9 item: the reason enum is CLOSED ─────────────────────────────────────

func TestRejectionReason_ClosedEnum(t *testing.T) {
	want := []string{
		"not-a-bug", "premise-disproved", "obsolete", "out-of-scope",
		"unsafe", "duplicate", "superseded",
	}
	for _, r := range want {
		if !IsValidRejectionReason(r) {
			t.Errorf("reason %q must be valid", r)
		}
	}
	if len(ValidRejectionReasons) != len(want) {
		t.Fatalf("enum size = %d, want %d (the enum is CLOSED: adding a value is a contract change)",
			len(ValidRejectionReasons), len(want))
	}
	for _, bad := range []string{"", "NOT-A-BUG", "wont-fix", "deferred", "design-rejected", "replaced-by", "  duplicate  "} {
		if IsValidRejectionReason(bad) {
			t.Errorf("reason %q must be rejected by the closed enum", bad)
		}
	}
	// The rendered list is used in --help and in exit-2 messages; it
	// must enumerate every value so an operator can self-correct.
	joined := RejectionReasonsJoined()
	for _, r := range want {
		if !strings.Contains(joined, r) {
			t.Errorf("RejectionReasonsJoined() missing %q: %s", r, joined)
		}
	}
}

// ─── §9 item: evidence refs round-trip and are hash-shaped ──────────────────

var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestRejectionStatus_JSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	in := &RejectionStatus{
		Reason:     "premise-disproved",
		Note:       "upstream already handles this",
		Actor:      "dev@example.com",
		Related:    "GH#41",
		Evidence:   []EvidenceRef{{Path: "analysis.md", SHA256: strings.Repeat("a", 64)}},
		RejectedAt: ts,
		PriorState: StateAnalyzed,
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out RejectionStatus
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	again, err := json.Marshal(&out)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(data) != string(again) {
		t.Fatalf("round-trip is not byte-identical:\n%s\n%s", data, again)
	}
	if !sha256HexRe.MatchString(out.Evidence[0].SHA256) {
		t.Fatalf("evidence sha256 must match ^[0-9a-f]{64}$, got %q", out.Evidence[0].SHA256)
	}
	if out.PriorState != StateAnalyzed {
		t.Fatalf("prior_state lost in round-trip: %q", out.PriorState)
	}
}

// One history entry == one COMPLETED reject→reopen cycle (PRD §6,
// ADR-031 D5). The entry carries both halves, and it round-trips
// byte-identically through JSON.
func TestRejectionHistoryEntry_CompletedCycleRoundTrip(t *testing.T) {
	ts := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	in := RejectionHistoryEntry{
		RejectedAt:     ts,
		RejectedBy:     "dev@example.com",
		Reason:         "premise-disproved",
		RejectNote:     "upstream already handles this",
		PriorState:     StateAnalyzed,
		Related:        "GH#41",
		RejectEvidence: []EvidenceRef{{Path: "analysis.md", SHA256: strings.Repeat("a", 64)}},
		ReopenedAt:     ts.Add(time.Hour),
		ReopenedBy:     "other@example.com",
		ReopenNote:     "upstream reverted it",
		ReopenEvidence: []EvidenceRef{{Path: "artifacts/revert.md", SHA256: strings.Repeat("c", 64)}},

		EvidenceIntegrity: EvidenceIntegrityDivergent,
		DivergenceDetail:  []DivergenceDetail{{Path: "analysis.md", DivergentReason: DivergentReasonHashMismatch}},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{
		"rejected_at", "rejected_by", "reason", "reject_note", "prior_state", "related",
		"reject_evidence", "reopened_at", "reopened_by", "reopen_note", "reopen_evidence",
		"evidence_integrity", "divergence_detail",
	} {
		if !strings.Contains(string(data), "\""+key+"\"") {
			t.Errorf("history entry missing %q: %s", key, data)
		}
	}
	var out RejectionHistoryEntry
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	again, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(data) != string(again) {
		t.Fatalf("round-trip is not byte-identical:\n%s\n%s", data, again)
	}
	if !out.RejectedAt.Equal(ts) || !out.ReopenedAt.Equal(ts.Add(time.Hour)) {
		t.Fatalf("timestamps drifted: %+v", out)
	}
}

// A feature that has never completed a cycle must omit
// `rejection_history` entirely, so pre-v0.13.0 fixtures round-trip
// byte-identical (ADR-031 D7).
func TestFeatureStatus_RejectionHistoryOmittedWhenEmpty(t *testing.T) {
	data, err := json.Marshal(FeatureStatus{Slug: "x", State: StateRequested})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "rejection_history") {
		t.Fatalf("empty history must be omitted, got %s", data)
	}
}

// A CLEAN reopen omits evidence_integrity entirely (ADR-031 D3
// addendum) — the field must never serialize as "clean".
func TestRejectionHistoryEntry_CleanIntegrityIsOmitted(t *testing.T) {
	e := RejectionHistoryEntry{
		RejectedAt: time.Unix(0, 0).UTC(), RejectedBy: "a", Reason: "duplicate", RejectNote: "n",
		ReopenedAt: time.Unix(1, 0).UTC(), ReopenedBy: "a", ReopenNote: "n",
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "evidence_integrity") {
		t.Fatalf("clean reopen must omit evidence_integrity, got %s", data)
	}
	if strings.Contains(string(data), "divergence_detail") {
		t.Fatalf("clean reopen must omit divergence_detail, got %s", data)
	}
	// ...and the constant still exists for callers that want the
	// positive verdict in memory.
	if EvidenceIntegrityClean != "clean" || EvidenceIntegrityDivergent != "divergent" {
		t.Fatalf("integrity constants drifted")
	}
}

// A feature with no rejection must not carry a `rejection` key: every
// pre-v0.13.0 status.json stays byte-identical (ADR-031 D1).
func TestFeatureStatus_RejectionOmittedWhenAbsent(t *testing.T) {
	data, err := json.Marshal(FeatureStatus{Slug: "x", State: StateRequested})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "rejection") {
		t.Fatalf("non-rejected feature must omit the rejection key, got %s", data)
	}
}

// End-to-end persistence through the store: the sub-object survives a
// Save/Load cycle with every field intact.
func TestFeatureStatus_RejectionPersistsThroughStore(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{"alpha": StateAnalyzed})
	st, err := s.LoadFeatureStatus("alpha")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 6, 9, 30, 0, 0, time.UTC)
	st.State = StateRejected
	st.Rejection = &RejectionStatus{
		Reason:     "duplicate",
		Note:       "same as beta",
		Actor:      "dev@example.com",
		Evidence:   []EvidenceRef{{Path: "request.md", SHA256: strings.Repeat("b", 64)}},
		RejectedAt: now,
		PriorState: StateAnalyzed,
	}
	st.RejectionHistory = []RejectionHistoryEntry{{
		RejectedAt: now.Add(-time.Hour), RejectedBy: "dev@example.com", Reason: "obsolete",
		RejectNote: "first cycle", PriorState: StateRequested,
		ReopenedAt: now.Add(-time.Minute), ReopenedBy: "dev@example.com", ReopenNote: "back",
	}}
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatalf("SaveFeatureStatus: %v", err)
	}
	got, err := s.LoadFeatureStatus("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateRejected {
		t.Fatalf("state = %q, want rejected", got.State)
	}
	if got.Rejection == nil {
		t.Fatal("rejection sub-object lost through the store")
	}
	if got.Rejection.Reason != "duplicate" || got.Rejection.PriorState != StateAnalyzed {
		t.Fatalf("rejection fields lost: %+v", got.Rejection)
	}
	if !got.Rejection.RejectedAt.Equal(now) {
		t.Fatalf("rejected_at drifted: %v", got.Rejection.RejectedAt)
	}
	if len(got.RejectionHistory) != 1 || got.RejectionHistory[0].Reason != "obsolete" {
		t.Fatalf("history lost: %+v", got.RejectionHistory)
	}
	if got.RejectionHistory[0].ReopenNote != "back" {
		t.Fatalf("reopen half lost: %+v", got.RejectionHistory[0])
	}
}

// ─── §9 item 23: actor resolution precedence (4 tiers) ──────────────────────

func TestResolveActor_PrecedenceChain(t *testing.T) {
	restore := gitConfigUserEmail
	t.Cleanup(func() { gitConfigUserEmail = restore })

	// Tier 1: --actor beats everything.
	gitConfigUserEmail = func(string) string { return "git@example.com" }
	t.Setenv(ActorEnvVar, "env@example.com")
	if got := ResolveActor("flag@example.com"); got != "flag@example.com" {
		t.Errorf("tier 1: got %q, want flag@example.com", got)
	}
	// Whitespace-only --actor falls through rather than winning.
	if got := ResolveActor("   "); got != "env@example.com" {
		t.Errorf("tier 1 whitespace: got %q, want env@example.com", got)
	}

	// Tier 2: $TPATCH_ACTOR beats git config.
	if got := ResolveActor(""); got != "env@example.com" {
		t.Errorf("tier 2: got %q, want env@example.com", got)
	}

	// Tier 3: git config user.email.
	os.Unsetenv(ActorEnvVar)
	if got := ResolveActor(""); got != "git@example.com" {
		t.Errorf("tier 3: got %q, want git@example.com", got)
	}

	// Tier 4: "unknown".
	gitConfigUserEmail = func(string) string { return "" }
	if got := ResolveActor(""); got != ActorUnknown {
		t.Errorf("tier 4: got %q, want %q", got, ActorUnknown)
	}
	if ActorUnknown != "unknown" {
		t.Fatalf("terminal fallback must be the literal %q", "unknown")
	}
}

// The git-config tier must read the REPOSITORY being operated on, not
// the process working directory.
func TestResolveActorIn_UsesRepoRoot(t *testing.T) {
	restore := gitConfigUserEmail
	t.Cleanup(func() { gitConfigUserEmail = restore })
	os.Unsetenv(ActorEnvVar)

	var sawRoot string
	gitConfigUserEmail = func(root string) string {
		sawRoot = root
		return "repo@example.com"
	}
	if got := ResolveActorIn("", "/some/repo"); got != "repo@example.com" {
		t.Fatalf("got %q", got)
	}
	if sawRoot != "/some/repo" {
		t.Fatalf("git-config tier consulted %q, want /some/repo", sawRoot)
	}
}

// ─── §9 item 22: Rule 7 — dependency-order symmetry ─────────────────────────
//
// Both independent orders must converge on the same invariant ("a
// rejected feature never has live dependents"), for all three edge
// kinds, with no per-kind carve-out (ADR-031 D8).

func TestValidateDependencies_Rule7_RejectedParentRefused(t *testing.T) {
	for _, kind := range []string{DependencyKindHard, DependencyKindSoft, DependencyKindSupersedes} {
		t.Run(kind, func(t *testing.T) {
			s := newStoreWith(t, map[string]FeatureState{
				"parent": StateRequested,
				"child":  StateRequested,
			})
			// Order B: reject the parent FIRST, then try to add the edge.
			p, err := s.LoadFeatureStatus("parent")
			if err != nil {
				t.Fatal(err)
			}
			p.State = StateRejected
			p.Rejection = &RejectionStatus{Reason: "out-of-scope", Note: "n", Actor: "a", PriorState: StateRequested}
			if err := s.SaveFeatureStatus(p); err != nil {
				t.Fatal(err)
			}

			err = ValidateDependencies(s, "child", []Dependency{{Slug: "parent", Kind: kind}})
			if !errors.Is(err, ErrRejectedParent) {
				t.Fatalf("kind=%s: want ErrRejectedParent, got %v", kind, err)
			}
			// The message must name the parent and hand the operator the
			// exact recovery command.
			msg := err.Error()
			for _, want := range []string{`"parent"`, "rejected", "tpatch reopen parent"} {
				if !strings.Contains(msg, want) {
					t.Errorf("kind=%s: message missing %q: %s", kind, want, msg)
				}
			}
			// The reason is surfaced so the operator can judge whether
			// reopening is appropriate.
			if !strings.Contains(msg, "out-of-scope") {
				t.Errorf("kind=%s: message should surface the rejection reason: %s", kind, msg)
			}
		})
	}
}

// A rejection record with no reason (defensively constructed) must not
// break the message rendering.
func TestValidateDependencies_Rule7_NoReasonRecord(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{"parent": StateRequested, "child": StateRequested})
	p, _ := s.LoadFeatureStatus("parent")
	p.State = StateRejected
	if err := s.SaveFeatureStatus(p); err != nil {
		t.Fatal(err)
	}
	err := ValidateDependencies(s, "child", []Dependency{{Slug: "parent", Kind: DependencyKindHard}})
	if !errors.Is(err, ErrRejectedParent) {
		t.Fatalf("want ErrRejectedParent, got %v", err)
	}
}

// Rule 7 is scoped to the PARENT side only: a rejected CHILD adding an
// edge onto a healthy parent is not this rule's business (the reject
// command's own dependents check owns the other direction).
func TestValidateDependencies_Rule7_HealthyParentStillAllowed(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{
		"parent": StateApplied,
		"child":  StateRequested,
	})
	if err := ValidateDependencies(s, "child", []Dependency{{Slug: "parent", Kind: DependencyKindHard}}); err != nil {
		t.Fatalf("healthy parent must still validate, got %v", err)
	}
}

// ─── FEATURES.md rendering: rejected features move to their own table ───────

func TestRefreshFeaturesIndex_RejectedSection(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{
		"alpha": StateRequested,
		"beta":  StateRequested,
	})
	b, _ := s.LoadFeatureStatus("beta")
	b.State = StateRejected
	b.Rejection = &RejectionStatus{
		Reason:     "obsolete",
		Note:       "no longer needed\nsecond line | with a pipe",
		Actor:      "dev@example.com",
		Evidence:   []EvidenceRef{{Path: "analysis.md", SHA256: strings.Repeat("c", 64)}},
		PriorState: StateRequested,
	}
	if err := s.SaveFeatureStatus(b); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshFeaturesIndex(); err != nil {
		t.Fatalf("RefreshFeaturesIndex: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(s.Root, ".tpatch", "FEATURES.md"))
	if err != nil {
		t.Fatalf("read FEATURES.md: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "## Rejected") {
		t.Fatalf("missing Rejected section:\n%s", got)
	}
	if !strings.Contains(got, "obsolete") {
		t.Fatalf("Rejected section must carry the reason:\n%s", got)
	}
	// The rejected slug must NOT appear above the Rejected header.
	idx := strings.Index(got, "## Rejected")
	if strings.Contains(got[:idx], "beta") {
		t.Fatalf("rejected feature must not appear in the active table:\n%s", got[:idx])
	}
	if !strings.Contains(got[:idx], "alpha") {
		t.Fatalf("active feature missing from the active table:\n%s", got[:idx])
	}
	// Free-form note text must not be able to break the markdown table.
	for _, line := range strings.Split(got[idx:], "\n") {
		if strings.HasPrefix(line, "| beta ") && strings.Count(line, "|") != 5 {
			t.Fatalf("rejected row has %d pipes, want 5 (note not escaped/collapsed): %q",
				strings.Count(line, "|"), line)
		}
	}
}

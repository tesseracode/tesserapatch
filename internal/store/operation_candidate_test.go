package store

import (
	"encoding/json"
	"testing"
)

func TestOperationCandidateSummaryOmitEmptyParity(t *testing.T) {
	without := ReconcileSummary{Outcome: ReconcileBlocked}
	withNil := ReconcileSummary{Outcome: ReconcileBlocked, OperationCandidate: nil}
	a, err := json.Marshal(without)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(withNil)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != `{"outcome":"blocked"}` || string(a) != string(b) {
		t.Fatalf("omitempty parity changed: without=%s withNil=%s", a, b)
	}
}

func TestOperationCandidateSummaryRoundTripAndValidation(t *testing.T) {
	summary := OperationCandidateSummary{
		CandidateID:        "rc_0123456789ab",
		Path:               ".tpatch/reconcile-candidates/demo/rc_0123456789ab",
		Status:             "pending",
		SourceFeatureState: string(StateBlocked),
	}
	if err := ValidateOperationCandidateSummary(&summary); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(ReconcileSummary{OperationCandidate: &summary})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"operation_candidate":{"candidate_id":"rc_0123456789ab","path":".tpatch/reconcile-candidates/demo/rc_0123456789ab","status":"pending","source_feature_state":"blocked"}}`
	if string(body) != want {
		t.Fatalf("operation candidate summary bytes changed:\nwant: %s\ngot:  %s", want, body)
	}
	var decoded ReconcileSummary
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOperationCandidateSummary(decoded.OperationCandidate); err != nil {
		t.Fatal(err)
	}
}

func TestOperationCandidateSummaryValidationRefusals(t *testing.T) {
	for _, tc := range []OperationCandidateSummary{
		{CandidateID: "rc_0123456789ab", Path: "/abs", Status: "pending", SourceFeatureState: "applied"},
		{CandidateID: "rc_0123456789ab", Path: ".tpatch\\bad", Status: "pending", SourceFeatureState: "applied"},
		{CandidateID: "rc_0123456789ab", Path: ".tpatch/reconcile-candidates/../bad", Status: "pending", SourceFeatureState: "applied"},
		{CandidateID: "rc_0123456789ab", Path: ".tpatch/reconcile-candidates/demo/rc_0123456789ab", Status: "queued", SourceFeatureState: "applied"},
		{CandidateID: "rc_0123456789ab", Path: ".tpatch/reconcile-candidates/demo/rc_0123456789ab", Status: "pending", SourceFeatureState: "requested"},
		{CandidateID: "rc_deadbeef", Path: ".tpatch/reconcile-candidates/demo/rc_deadbeef", Status: "pending", SourceFeatureState: "applied"},
	} {
		value := tc
		if err := ValidateOperationCandidateSummary(&value); err == nil {
			t.Fatalf("validation accepted %+v", value)
		}
	}
}

func TestCarryForwardOperationCandidate(t *testing.T) {
	loaded := ReconcileSummary{
		Outcome: ReconcileBlocked,
		OperationCandidate: &OperationCandidateSummary{
			CandidateID:        "rc_0123456789ab",
			Path:               ".tpatch/reconcile-candidates/demo/rc_0123456789ab",
			Status:             "pending",
			SourceFeatureState: "blocked",
		},
	}
	next := ReconcileSummary{Outcome: ReconcileUpstreamed}
	merged := CarryForwardOperationCandidate(next, loaded)
	if merged.OperationCandidate == nil || merged.OperationCandidate.CandidateID != loaded.OperationCandidate.CandidateID {
		t.Fatalf("carry-forward lost pointer: %+v", merged)
	}
	if merged.Outcome != ReconcileUpstreamed {
		t.Fatalf("carry-forward changed unrelated fields: %+v", merged)
	}
	if merged.OperationCandidate == loaded.OperationCandidate {
		t.Fatal("carry-forward returned alias instead of copy")
	}
}

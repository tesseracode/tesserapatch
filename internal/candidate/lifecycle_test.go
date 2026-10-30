package candidate

import (
	"strings"
	"testing"
)

func TestBuildCandidateArtifactExpectedIDAndTreeHashSemantics(t *testing.T) {
	artifact := testCandidateArtifact(t, "applied")
	if len(artifact.Result.CandidateTreeSHA256) != 64 || artifact.Result.CandidateTreeSHA256 == strings.Repeat("c", 40) {
		t.Fatalf("candidate_tree_sha256 is not SHA-256 integrity hash: %q", artifact.Result.CandidateTreeSHA256)
	}
	_, err := BuildCandidateArtifact(CandidateMaterializeInput{
		Feature:            artifact.Feature,
		SourceFeatureState: artifact.SourceFeatureState,
		UpstreamCommit:     artifact.UpstreamCommit,
		ReferenceCommit:    artifact.ReferenceCommit,
		Bindings:           artifact.Bindings,
		Execution: ExecutionResult{
			Plan:      Plan{operations: []operation{}, effects: []effectClassification{}},
			EffectMap: []EffectResolution{},
			TreeHash:  strings.Repeat("a", 40),
			Diff:      "",
		},
		EffectSHA256ByOrdinal: map[int]string{},
		ExpectedCandidateID:   "rc_deadbeefcafe",
	})
	if err == nil {
		t.Fatal("expected candidate id mismatch refusal")
	}
}

func TestCompareExistingCandidateDecisionMatrix(t *testing.T) {
	derived := testCandidateArtifact(t, "applied")
	state := CandidateState{
		SchemaVersion:    CandidateSchemaVersion,
		CandidateID:      derived.CandidateID,
		Status:           CandidateStatusPending,
		AcceptValidation: CandidateAcceptValidationNone,
	}
	decision, err := CompareExistingCandidate(derived, state, derived)
	if err != nil || !decision.Reusable || decision.RefusalCode != "" {
		t.Fatalf("pending equal candidate should be reusable: decision=%+v err=%v", decision, err)
	}
	acceptedState := state
	acceptedState.Status = CandidateStatusAccepted
	decision, err = CompareExistingCandidate(derived, acceptedState, derived)
	if err != nil || decision.RefusalCode != "candidate-already-accepted" {
		t.Fatalf("accepted candidate must refuse before identity compare: decision=%+v err=%v", decision, err)
	}
	collision := derived
	collision.Bindings.RecipeSHA256 = strings.Repeat("f", 64)
	decision, err = CompareExistingCandidate(derived, state, collision)
	if err != nil || decision.RefusalCode != "candidate-id-collision" {
		t.Fatalf("identity drift should report collision: decision=%+v err=%v", decision, err)
	}
	integrityDrift := derived
	integrityDrift.Result.CandidateDiffSHA256 = strings.Repeat("9", 64)
	decision, err = CompareExistingCandidate(derived, state, integrityDrift)
	if err != nil || decision.RefusalCode != "candidate-integrity-drift" {
		t.Fatalf("result drift should report integrity drift: decision=%+v err=%v", decision, err)
	}
}

func TestCandidateNeedsStaleMark(t *testing.T) {
	artifact := testCandidateArtifact(t, "blocked")
	state := CandidateState{
		SchemaVersion:    CandidateSchemaVersion,
		CandidateID:      artifact.CandidateID,
		Status:           CandidateStatusPending,
		AcceptValidation: CandidateAcceptValidationNone,
	}
	stale, err := CandidateNeedsStaleMark(artifact, state, CandidateStalenessInput{
		UpstreamCommit:     artifact.UpstreamCommit,
		SourceFeatureState: artifact.SourceFeatureState,
		Bindings:           artifact.Bindings,
	})
	if err != nil || stale {
		t.Fatalf("matching bindings should stay pending: stale=%t err=%v", stale, err)
	}
	for _, mutate := range []CandidateStalenessInput{
		{UpstreamCommit: strings.Repeat("f", 40), SourceFeatureState: artifact.SourceFeatureState, Bindings: artifact.Bindings},
		{UpstreamCommit: artifact.UpstreamCommit, SourceFeatureState: "applied", Bindings: artifact.Bindings},
		{UpstreamCommit: artifact.UpstreamCommit, SourceFeatureState: artifact.SourceFeatureState, Bindings: CandidateBindings{
			CaptureEventSHA256: strings.Repeat("1", 64),
			CoverageSHA256:     strings.Repeat("2", 64),
			PatchSHA256:        strings.Repeat("3", 64),
			RecipeSHA256:       strings.Repeat("9", 64),
		}},
	} {
		stale, err = CandidateNeedsStaleMark(artifact, state, mutate)
		if err != nil || !stale {
			t.Fatalf("drift should mark stale: stale=%t err=%v mutate=%+v", stale, err, mutate)
		}
	}
	state.Status = CandidateStatusAccepted
	stale, err = CandidateNeedsStaleMark(artifact, state, CandidateStalenessInput{
		UpstreamCommit:     strings.Repeat("f", 40),
		SourceFeatureState: artifact.SourceFeatureState,
		Bindings:           artifact.Bindings,
	})
	if err != nil || stale {
		t.Fatalf("accepted state must not be stale-pruned by preflight: stale=%t err=%v", stale, err)
	}
}

func TestRejectedRecordStrictMetadataOnly(t *testing.T) {
	artifact := testCandidateArtifact(t, "applied")
	record := RejectedRecordFromArtifact(artifact)
	raw, err := EncodeRejectedRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRejectedRecord(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != record {
		t.Fatalf("rejected record changed after round-trip: got=%+v want=%+v", decoded, record)
	}
	mutated := strings.Replace(strings.TrimSpace(string(raw)), "{", `{"timestamp":"now",`, 1)
	if _, err := DecodeRejectedRecord([]byte(mutated)); err == nil {
		t.Fatal("rejected decoder accepted extra timestamp/body fields")
	}
	if !RejectedContainsCandidateID([]CandidateRejectedRecord{record}, artifact.CandidateID) {
		t.Fatal("rejected id lookup failed")
	}
}

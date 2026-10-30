package candidate

import (
	"strings"
	"testing"
)

func TestDescribeDerivationContainsMetadataOnly(t *testing.T) {
	_, reference, upstream, patch, effects := executionFixture(t)
	effectHashes := make(map[int]string, len(effects))
	for i, effect := range effects {
		_ = effect
		nibble := string(rune('a' + (i % 6)))
		effectHashes[i+1] = strings.Repeat(nibble, 64)
	}
	metadata, err := DescribeDerivation(reference, upstream, patch, effects, effectHashes)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.EffectCount != len(effects) || metadata.CandidateApplicableCount == 0 || metadata.ConflictCount != 0 {
		t.Fatalf("unexpected derivation metadata: %+v", metadata)
	}
	if _, err := DescribeDerivation(reference, upstream, patch, effects, nil); err == nil {
		t.Fatal("missing descriptor map accepted")
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
	base := testCandidateArtifactWithOrdinalPermutation(t, false)
	swapped := testCandidateArtifactWithOrdinalPermutation(t, true)
	baseState := state
	baseState.CandidateID = base.CandidateID
	decision, err = CompareExistingCandidate(base, baseState, swapped)
	if err != nil || !decision.Reusable {
		t.Fatalf("canonical identity tuple compare should allow ordinal permutation: decision=%+v err=%v", decision, err)
	}
}

func testCandidateArtifactWithOrdinalPermutation(t *testing.T, swap bool) CandidateArtifact {
	t.Helper()
	artifact := testCandidateArtifact(t, "applied")
	artifact.Derivation.Operations = []CandidateOperation{
		{
			Ordinal:         1,
			Kind:            "create-file",
			Path:            "new.txt",
			EffectOrdinal:   2,
			EffectSHA256:    strings.Repeat("b", 64),
			RunOrdinals:     []int{},
			InsertionSHA256: strings.Repeat("8", 64),
			InsertionSize:   4,
			Classification:  "applicable",
		},
		{
			Ordinal:         2,
			Kind:            "create-file",
			Path:            "new.txt",
			EffectOrdinal:   2,
			EffectSHA256:    strings.Repeat("b", 64),
			RunOrdinals:     []int{},
			InsertionSHA256: strings.Repeat("9", 64),
			InsertionSize:   4,
			Classification:  "applicable",
		},
	}
	artifact.Derivation.EffectMap = []CandidateEffectMapEntry{
		{
			EffectOrdinal:     1,
			EffectSHA256:      strings.Repeat("a", 64),
			Path:              "existing.txt",
			Resolution:        "absorbed-present",
			OperationOrdinals: []int{},
			ReasonCodes:       []string{},
		},
		{
			EffectOrdinal:     2,
			EffectSHA256:      strings.Repeat("b", 64),
			Path:              "new.txt",
			Resolution:        "candidate-applicable",
			OperationOrdinals: []int{1, 2},
			ReasonCodes:       []string{},
		},
	}
	artifact.Derivation.EffectCount = 2
	artifact.Derivation.AbsorbedPresentCount = 1
	artifact.Derivation.CandidateApplicableCount = 1
	artifact.Derivation.ConflictCount = 0
	if swap {
		left := artifact.Derivation.Operations[0]
		right := artifact.Derivation.Operations[1]
		left.Ordinal = 2
		right.Ordinal = 1
		artifact.Derivation.Operations = []CandidateOperation{right, left}
	}
	id, _, _, err := ComputeCandidateID(CandidateIdentityInput{
		SchemaVersion:      artifact.SchemaVersion,
		Feature:            artifact.Feature,
		SourceFeatureState: artifact.SourceFeatureState,
		UpstreamCommit:     artifact.UpstreamCommit,
		ReferenceCommit:    artifact.ReferenceCommit,
		Bindings:           artifact.Bindings,
		DerivationMode:     artifact.DerivationMode,
		Operations:         artifact.Derivation.Operations,
		EffectMap:          artifact.Derivation.EffectMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact.CandidateID = id
	if err := artifact.Validate(); err != nil {
		t.Fatal(err)
	}
	return artifact
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
		{UpstreamCommit: artifact.UpstreamCommit, SourceFeatureState: "rejected", Bindings: artifact.Bindings},
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

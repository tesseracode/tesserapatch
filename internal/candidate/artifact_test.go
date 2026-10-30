package candidate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCandidateArtifactRoundTripStrictly(t *testing.T) {
	artifact := testCandidateArtifact(t, "applied")
	raw, err := EncodeCandidateArtifact(artifact)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCandidateArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.CandidateID != artifact.CandidateID || decoded.Result != artifact.Result {
		t.Fatalf("artifact round-trip changed payload:\nwant=%+v\ngot=%+v", artifact, decoded)
	}
	state := CandidateState{
		SchemaVersion:    CandidateSchemaVersion,
		CandidateID:      artifact.CandidateID,
		Status:           CandidateStatusPending,
		AcceptValidation: CandidateAcceptValidationNone,
	}
	stateRaw, err := EncodeCandidateState(state)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeCandidateState(stateRaw); err != nil || got != state {
		t.Fatalf("state round-trip mismatch: %+v err=%v", got, err)
	}
}

func TestCandidateArtifactStrictDecodeRefusals(t *testing.T) {
	artifact := testCandidateArtifact(t, "applied")
	raw, err := EncodeCandidateArtifact(artifact)
	if err != nil {
		t.Fatal(err)
	}
	replaceOnce := func(name, old, new string) []byte {
		t.Helper()
		out := strings.Replace(string(raw), old, new, 1)
		if out == string(raw) {
			t.Fatalf("%s mutation anchor missing: %q", name, old)
		}
		return []byte(out)
	}
	nullArrayBody := func() []byte {
		var node map[string]any
		if err := json.Unmarshal(raw, &node); err != nil {
			t.Fatal(err)
		}
		derivation := node["derivation"].(map[string]any)
		derivation["operations"] = nil
		body, err := json.Marshal(node)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}()
	for _, mutation := range []struct {
		name string
		body []byte
	}{
		{"unknown-top-level", replaceOnce("unknown-top-level", "{", `{"unknown":true,`)},
		{"duplicate-top-level", replaceOnce("duplicate-top-level", `"feature": "demo-feature",`, `"feature": "x", "feature": "demo-feature",`)},
		{"unknown-nested", replaceOnce("unknown-nested", `"bindings": {`, `"bindings": {"extra":"x",`)},
		{"null-array", nullArrayBody},
		{"status-leak", replaceOnce("status-leak", `"result": {`, `"result": {"status":"pending",`)},
		{"offset-leak", replaceOnce("offset-leak", `"effect_ordinal": 2,`, `"effect_ordinal": 2, "offset": 10,`)},
		{"classification-resolution-contradiction", replaceOnce("classification-resolution-contradiction", `"classification": "applicable"`, `"classification": "conflict"`)},
	} {
		if _, err := DecodeCandidateArtifact(mutation.body); err == nil {
			t.Fatalf("strict decoder accepted %s", mutation.name)
		}
	}
}

func TestCandidateArtifactValidationRefusals(t *testing.T) {
	artifact := testCandidateArtifact(t, "applied")
	for _, mutate := range []func(*CandidateArtifact){
		func(a *CandidateArtifact) { a.Derivation.EffectCount++ },
		func(a *CandidateArtifact) { a.Derivation.EffectMap[1].OperationOrdinals = []int{} },
		func(a *CandidateArtifact) { a.Derivation.Operations[0].Path = `a\bad.txt` },
		func(a *CandidateArtifact) { a.Derivation.Operations[0].Path = "../bad.txt" },
		func(a *CandidateArtifact) { a.Derivation.EffectMap[1].ReasonCodes = nil },
		func(a *CandidateArtifact) { a.Derivation.Operations[0].Classification = "conflict" },
		func(a *CandidateArtifact) { a.Result.Validated = "tests" },
	} {
		mutated := artifact
		mutate(&mutated)
		if err := mutated.Validate(); err == nil {
			t.Fatalf("validation accepted %+v", mutated)
		}
	}
	state := CandidateState{
		SchemaVersion:    CandidateSchemaVersion,
		CandidateID:      artifact.CandidateID,
		Status:           "queued",
		AcceptValidation: CandidateAcceptValidationNone,
	}
	if err := state.Validate(); err == nil {
		t.Fatal("state validation accepted unknown status")
	}
}

func testCandidateArtifact(t *testing.T, sourceState string) CandidateArtifact {
	t.Helper()
	operations := []CandidateOperation{
		{
			Ordinal:         1,
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
	effectMap := []CandidateEffectMapEntry{
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
			OperationOrdinals: []int{1},
			ReasonCodes:       []string{},
		},
	}
	bindings := CandidateBindings{
		CaptureEventSHA256: strings.Repeat("1", 64),
		CoverageSHA256:     strings.Repeat("2", 64),
		PatchSHA256:        strings.Repeat("3", 64),
		RecipeSHA256:       strings.Repeat("4", 64),
	}
	candidateID, _, _, err := ComputeCandidateID(CandidateIdentityInput{
		SchemaVersion:      CandidateSchemaVersion,
		Feature:            "demo-feature",
		SourceFeatureState: sourceState,
		UpstreamCommit:     strings.Repeat("a", 40),
		ReferenceCommit:    strings.Repeat("b", 40),
		Bindings:           bindings,
		DerivationMode:     CandidateDerivationModeSingle,
		Operations:         operations,
		EffectMap:          effectMap,
	})
	if err != nil {
		t.Fatalf("compute fixture candidate id: %v", err)
	}
	artifact := CandidateArtifact{
		SchemaVersion:      CandidateSchemaVersion,
		CandidateID:        candidateID,
		Feature:            "demo-feature",
		SourceFeatureState: sourceState,
		UpstreamCommit:     strings.Repeat("a", 40),
		ReferenceCommit:    strings.Repeat("b", 40),
		DerivationMode:     CandidateDerivationModeSingle,
		Bindings:           bindings,
		Derivation: CandidateDerivation{
			EffectCount:              2,
			AbsorbedPresentCount:     1,
			CandidateApplicableCount: 1,
			ConflictCount:            0,
			Operations:               operations,
			EffectMap:                effectMap,
		},
		Result: CandidateResult{
			CandidateDiffSHA256: strings.Repeat("d", 64),
			CandidateTreeSHA256: strings.Repeat("e", 64),
			Validated:           CandidateValidationStructural,
		},
	}
	if err := artifact.Validate(); err != nil {
		t.Fatalf("validate fixture candidate artifact: %v", err)
	}
	return artifact
}

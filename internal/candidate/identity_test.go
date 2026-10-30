package candidate

import (
	"strings"
	"testing"
)

func TestCandidateIDDeterministicAndSourceStateSensitive(t *testing.T) {
	input := CandidateIdentityInput{
		SchemaVersion:      CandidateSchemaVersion,
		Feature:            "demo-feature",
		SourceFeatureState: "applied",
		UpstreamCommit:     strings.Repeat("a", 40),
		ReferenceCommit:    strings.Repeat("b", 40),
		Bindings: CandidateBindings{
			CaptureEventSHA256: strings.Repeat("1", 64),
			CoverageSHA256:     strings.Repeat("2", 64),
			PatchSHA256:        strings.Repeat("3", 64),
			RecipeSHA256:       strings.Repeat("4", 64),
		},
		DerivationMode: CandidateDerivationModeSingle,
		Operations: []CandidateOperation{
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
		},
		EffectMap: []CandidateEffectMapEntry{
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
		},
	}
	first, opProofs1, effectProofs1, err := ComputeCandidateID(input)
	if err != nil {
		t.Fatal(err)
	}
	second, opProofs2, effectProofs2, err := ComputeCandidateID(input)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || strings.Count(first, "rc_") != 1 || len(first) != 15 {
		t.Fatalf("candidate id is not deterministic/shape-stable: %q vs %q", first, second)
	}
	if strings.Join(opProofs1, "\n") != strings.Join(opProofs2, "\n") || strings.Join(effectProofs1, "\n") != strings.Join(effectProofs2, "\n") {
		t.Fatal("proof hash lists are not deterministic")
	}
	input.SourceFeatureState = "blocked"
	changed, _, _, err := ComputeCandidateID(input)
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatalf("source_feature_state did not rotate candidate id: %q", first)
	}
}

func TestCandidateIdentityProofSensitivity(t *testing.T) {
	base := CandidateOperation{
		Ordinal:             1,
		Kind:                "insert-by-boundary",
		Path:                "a.txt",
		EffectOrdinal:       1,
		EffectSHA256:        strings.Repeat("a", 64),
		RunOrdinals:         []int{1, 2},
		InsertionSHA256:     strings.Repeat("b", 64),
		InsertionSize:       4,
		BoundaryProofSHA256: strings.Repeat("c", 64),
		Classification:      "applicable",
	}
	hash, err := CandidateOperationProofHash(base)
	if err != nil {
		t.Fatal(err)
	}
	mutant := base
	mutant.RunOrdinals = []int{1, 3}
	changed, err := CandidateOperationProofHash(mutant)
	if err != nil {
		t.Fatal(err)
	}
	if hash == changed {
		t.Fatal("operation proof hash did not react to run_ordinals change")
	}
}

func TestBoundaryProofSHA256DeterministicAndSensitive(t *testing.T) {
	input := CandidateBoundaryProofInput{
		Path:                 "a.txt",
		BaseBoundaryIndex:    1,
		UpstreamLeftIndex:    2,
		UpstreamRightIndex:   3,
		BaseRecordCount:      5,
		UpstreamRecordCount:  7,
		AlignmentLength:      4,
		InsertionRecordCount: 2,
		InsertionSHA256:      strings.Repeat("d", 64),
	}
	first, err := BoundaryProofSHA256(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BoundaryProofSHA256(input)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("boundary proof hash changed across identical inputs: %s vs %s", first, second)
	}
	input.AlignmentLength++
	changed, err := BoundaryProofSHA256(input)
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatal("boundary proof hash ignored alignment_length change")
	}
}

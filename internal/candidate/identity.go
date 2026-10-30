package candidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type CandidateIdentityInput struct {
	SchemaVersion      int
	Feature            string
	SourceFeatureState string
	UpstreamCommit     string
	ReferenceCommit    string
	Bindings           CandidateBindings
	DerivationMode     string
	Operations         []CandidateOperation
	EffectMap          []CandidateEffectMapEntry
}

type CandidateBoundaryProofInput struct {
	Path                 string
	BaseBoundaryIndex    int
	UpstreamLeftIndex    int
	UpstreamRightIndex   int
	BaseRecordCount      int
	UpstreamRecordCount  int
	AlignmentLength      int
	InsertionRecordCount int
	InsertionSHA256      string
}

type CandidateIdentityTuple struct {
	SchemaVersion      int      `json:"schema_version"`
	Feature            string   `json:"feature"`
	SourceFeatureState string   `json:"source_feature_state"`
	UpstreamCommit     string   `json:"upstream_commit"`
	ReferenceCommit    string   `json:"reference_commit"`
	CaptureEventSHA256 string   `json:"capture_event_sha256"`
	CoverageSHA256     string   `json:"coverage_sha256"`
	PatchSHA256        string   `json:"patch_sha256"`
	RecipeSHA256       string   `json:"recipe_sha256"`
	DerivationMode     string   `json:"derivation_mode"`
	OperationProofs    []string `json:"operation_proof_hashes"`
	EffectMapProofs    []string `json:"effect_map_proof_hashes"`
}

func (input CandidateIdentityInput) Validate() error {
	artifact := CandidateArtifact{
		SchemaVersion:      input.SchemaVersion,
		CandidateID:        "rc_0123456789ab",
		Feature:            input.Feature,
		SourceFeatureState: input.SourceFeatureState,
		UpstreamCommit:     input.UpstreamCommit,
		ReferenceCommit:    input.ReferenceCommit,
		DerivationMode:     input.DerivationMode,
		Bindings:           input.Bindings,
		Derivation: CandidateDerivation{
			EffectCount:              len(input.EffectMap),
			AbsorbedPresentCount:     countResolution(input.EffectMap, "absorbed-present"),
			CandidateApplicableCount: countResolution(input.EffectMap, "candidate-applicable"),
			ConflictCount:            0,
			Operations:               input.Operations,
			EffectMap:                input.EffectMap,
		},
		Result: CandidateResult{
			CandidateDiffSHA256: strings.Repeat("0", 64),
			CandidateTreeSHA256: strings.Repeat("0", 64),
			Validated:           CandidateValidationStructural,
		},
	}
	if err := artifact.Validate(); err != nil {
		return err
	}
	return nil
}

func ComputeCandidateID(input CandidateIdentityInput) (string, []string, []string, error) {
	tuple, err := IdentityTupleFromInput(input)
	if err != nil {
		return "", nil, nil, err
	}
	sum, err := hashCanonicalJSON(tuple)
	if err != nil {
		return "", nil, nil, err
	}
	return "rc_" + sum[:12], tuple.OperationProofs, tuple.EffectMapProofs, nil
}

func IdentityTupleFromInput(input CandidateIdentityInput) (CandidateIdentityTuple, error) {
	if err := input.Validate(); err != nil {
		return CandidateIdentityTuple{}, err
	}
	operationProofs := make([]string, 0, len(input.Operations))
	for _, op := range input.Operations {
		hash, err := CandidateOperationProofHash(op)
		if err != nil {
			return CandidateIdentityTuple{}, err
		}
		operationProofs = append(operationProofs, hash)
	}
	effectProofs := make([]string, 0, len(input.EffectMap))
	for _, row := range input.EffectMap {
		hash, err := CandidateEffectMapProofHash(row)
		if err != nil {
			return CandidateIdentityTuple{}, err
		}
		effectProofs = append(effectProofs, hash)
	}
	sort.Strings(operationProofs)
	sort.Strings(effectProofs)
	return CandidateIdentityTuple{
		SchemaVersion:      input.SchemaVersion,
		Feature:            input.Feature,
		SourceFeatureState: input.SourceFeatureState,
		UpstreamCommit:     input.UpstreamCommit,
		ReferenceCommit:    input.ReferenceCommit,
		CaptureEventSHA256: input.Bindings.CaptureEventSHA256,
		CoverageSHA256:     input.Bindings.CoverageSHA256,
		PatchSHA256:        input.Bindings.PatchSHA256,
		RecipeSHA256:       input.Bindings.RecipeSHA256,
		DerivationMode:     input.DerivationMode,
		OperationProofs:    operationProofs,
		EffectMapProofs:    effectProofs,
	}, nil
}

func IdentityTupleFromArtifact(artifact CandidateArtifact) (CandidateIdentityTuple, error) {
	if err := artifact.Validate(); err != nil {
		return CandidateIdentityTuple{}, err
	}
	return IdentityTupleFromInput(CandidateIdentityInput{
		SchemaVersion:      artifact.SchemaVersion,
		Feature:            artifact.Feature,
		SourceFeatureState: artifact.SourceFeatureState,
		UpstreamCommit:     artifact.UpstreamCommit,
		ReferenceCommit:    artifact.ReferenceCommit,
		Bindings:           artifact.Bindings,
		DerivationMode:     artifact.DerivationMode,
		Operations:         append([]CandidateOperation(nil), artifact.Derivation.Operations...),
		EffectMap:          append([]CandidateEffectMapEntry(nil), artifact.Derivation.EffectMap...),
	})
}

func CandidateOperationProofHash(operation CandidateOperation) (string, error) {
	if err := operation.Validate(); err != nil {
		return "", err
	}
	proof := struct {
		Kind                string `json:"kind"`
		Path                string `json:"path"`
		EffectOrdinal       int    `json:"effect_ordinal"`
		EffectSHA256        string `json:"effect_sha256"`
		RunOrdinals         []int  `json:"run_ordinals"`
		InsertionSHA256     string `json:"insertion_sha256"`
		InsertionSize       int    `json:"insertion_size"`
		BoundaryProofSHA256 string `json:"boundary_proof_sha256"`
	}{
		Kind:                operation.Kind,
		Path:                operation.Path,
		EffectOrdinal:       operation.EffectOrdinal,
		EffectSHA256:        operation.EffectSHA256,
		RunOrdinals:         operation.RunOrdinals,
		InsertionSHA256:     operation.InsertionSHA256,
		InsertionSize:       operation.InsertionSize,
		BoundaryProofSHA256: operation.BoundaryProofSHA256,
	}
	return hashCanonicalJSON(proof)
}

func CandidateEffectMapProofHash(entry CandidateEffectMapEntry) (string, error) {
	if err := entry.Validate(); err != nil {
		return "", err
	}
	proof := struct {
		EffectOrdinal     int      `json:"effect_ordinal"`
		EffectSHA256      string   `json:"effect_sha256"`
		Path              string   `json:"path"`
		Resolution        string   `json:"resolution"`
		OperationOrdinals []int    `json:"operation_ordinals"`
		ReasonCodes       []string `json:"reason_codes"`
	}{
		EffectOrdinal:     entry.EffectOrdinal,
		EffectSHA256:      entry.EffectSHA256,
		Path:              entry.Path,
		Resolution:        entry.Resolution,
		OperationOrdinals: entry.OperationOrdinals,
		ReasonCodes:       entry.ReasonCodes,
	}
	return hashCanonicalJSON(proof)
}

func BoundaryProofSHA256(input CandidateBoundaryProofInput) (string, error) {
	if err := validateCanonicalRelativePath(input.Path); err != nil {
		return "", err
	}
	for name, value := range map[string]int{
		"base_boundary_index":    input.BaseBoundaryIndex,
		"upstream_left_index":    input.UpstreamLeftIndex,
		"upstream_right_index":   input.UpstreamRightIndex,
		"base_record_count":      input.BaseRecordCount,
		"upstream_record_count":  input.UpstreamRecordCount,
		"alignment_length":       input.AlignmentLength,
		"insertion_record_count": input.InsertionRecordCount,
	} {
		if value < 0 {
			return "", fmt.Errorf("%s must be nonnegative", name)
		}
	}
	if input.UpstreamLeftIndex > input.UpstreamRightIndex {
		return "", fmt.Errorf("upstream_left_index cannot exceed upstream_right_index")
	}
	if !sha256HexPattern.MatchString(input.InsertionSHA256) {
		return "", fmt.Errorf("insertion_sha256 must be a 64-character lowercase SHA-256")
	}
	proof := struct {
		Path                 string `json:"path"`
		BaseBoundaryIndex    int    `json:"base_boundary_index"`
		UpstreamLeftIndex    int    `json:"upstream_left_index"`
		UpstreamRightIndex   int    `json:"upstream_right_index"`
		BaseRecordCount      int    `json:"base_record_count"`
		UpstreamRecordCount  int    `json:"upstream_record_count"`
		AlignmentLength      int    `json:"alignment_length"`
		InsertionRecordCount int    `json:"insertion_record_count"`
		InsertionSHA256      string `json:"insertion_sha256"`
	}{
		Path:                 input.Path,
		BaseBoundaryIndex:    input.BaseBoundaryIndex,
		UpstreamLeftIndex:    input.UpstreamLeftIndex,
		UpstreamRightIndex:   input.UpstreamRightIndex,
		BaseRecordCount:      input.BaseRecordCount,
		UpstreamRecordCount:  input.UpstreamRecordCount,
		AlignmentLength:      input.AlignmentLength,
		InsertionRecordCount: input.InsertionRecordCount,
		InsertionSHA256:      input.InsertionSHA256,
	}
	return hashCanonicalJSON(proof)
}

func CandidateTreeIntegritySHA256(treeObjectID string) (string, error) {
	if !regexpTreeObjectID.MatchString(treeObjectID) {
		return "", fmt.Errorf("tree object id must be 40 or 64 lowercase hex characters")
	}
	sum := sha256.Sum256([]byte("git-tree-object-id:" + treeObjectID))
	return hex.EncodeToString(sum[:]), nil
}

var regexpTreeObjectID = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

func hashCanonicalJSON(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func countResolution(rows []CandidateEffectMapEntry, resolution string) int {
	count := 0
	for _, row := range rows {
		if row.Resolution == resolution {
			count++
		}
	}
	return count
}

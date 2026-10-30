package candidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

type CandidateExistingDecision struct {
	Reusable    bool
	RefusalCode string
}

type CandidateStalenessInput struct {
	UpstreamCommit     string
	SourceFeatureState string
	Bindings           CandidateBindings
}

type CandidateRejectedRecord struct {
	CandidateID        string `json:"candidate_id"`
	SourceFeatureState string `json:"source_feature_state"`
	UpstreamCommit     string `json:"upstream_commit"`
	ReferenceCommit    string `json:"reference_commit"`
	CaptureEventSHA256 string `json:"capture_event_sha256"`
	CoverageSHA256     string `json:"coverage_sha256"`
	PatchSHA256        string `json:"patch_sha256"`
	RecipeSHA256       string `json:"recipe_sha256"`
}

// DescribeDerivation returns metadata, not eligibility or execution authority.
// The workflow owner supplies descriptor hashes from its validated E/C snapshot.
func DescribeDerivation(reference, upstream *ReferenceRoot, patch string, effects []gitutil.PatchEffect, effectHashes map[int]string) (CandidateDerivation, error) {
	plan, err := derive(reference, upstream, patch, effects)
	if err != nil {
		return CandidateDerivation{}, err
	}
	if _, applicable, conflicts := plan.Counts(); conflicts != 0 || applicable == 0 {
		return CandidateDerivation{}, fmt.Errorf("candidate requires applicable effects and no conflicts")
	}
	effectMap, err := buildEffectResolutionMap(plan)
	if err != nil {
		return CandidateDerivation{}, err
	}
	return buildCandidateDerivation(plan, effectMap, effectHashes)
}

func CompareExistingCandidate(existingArtifact CandidateArtifact, existingState CandidateState, derived CandidateArtifact) (CandidateExistingDecision, error) {
	if err := existingArtifact.Validate(); err != nil {
		return CandidateExistingDecision{}, err
	}
	if err := existingState.Validate(); err != nil {
		return CandidateExistingDecision{}, err
	}
	if err := derived.Validate(); err != nil {
		return CandidateExistingDecision{}, err
	}
	if existingState.CandidateID != existingArtifact.CandidateID || derived.CandidateID == "" {
		return CandidateExistingDecision{}, fmt.Errorf("candidate state/id mismatch")
	}
	if existingState.Status == CandidateStatusAccepted {
		return CandidateExistingDecision{RefusalCode: "candidate-already-accepted"}, nil
	}
	if !sameIdentitySubset(existingArtifact, derived) {
		return CandidateExistingDecision{RefusalCode: "candidate-id-collision"}, nil
	}
	if !reflect.DeepEqual(existingArtifact.Result, derived.Result) {
		return CandidateExistingDecision{RefusalCode: "candidate-integrity-drift"}, nil
	}
	if existingState.Status == CandidateStatusPending || existingState.Status == CandidateStatusStale {
		return CandidateExistingDecision{Reusable: true}, nil
	}
	return CandidateExistingDecision{RefusalCode: "candidate-not-pending"}, nil
}

func CandidateNeedsStaleMark(artifact CandidateArtifact, state CandidateState, current CandidateStalenessInput) (bool, error) {
	if err := artifact.Validate(); err != nil {
		return false, err
	}
	if err := state.Validate(); err != nil {
		return false, err
	}
	if state.Status != CandidateStatusPending {
		return false, nil
	}
	if err := current.Bindings.Validate(); err != nil {
		return false, err
	}
	if err := validateCommit(current.UpstreamCommit); err != nil {
		return false, err
	}
	if current.SourceFeatureState == "" {
		return false, fmt.Errorf("current source_feature_state is required")
	}
	return artifact.UpstreamCommit != current.UpstreamCommit ||
		artifact.SourceFeatureState != current.SourceFeatureState ||
		artifact.Bindings != current.Bindings, nil
}

func RejectedRecordFromArtifact(artifact CandidateArtifact) CandidateRejectedRecord {
	return CandidateRejectedRecord{
		CandidateID:        artifact.CandidateID,
		SourceFeatureState: artifact.SourceFeatureState,
		UpstreamCommit:     artifact.UpstreamCommit,
		ReferenceCommit:    artifact.ReferenceCommit,
		CaptureEventSHA256: artifact.Bindings.CaptureEventSHA256,
		CoverageSHA256:     artifact.Bindings.CoverageSHA256,
		PatchSHA256:        artifact.Bindings.PatchSHA256,
		RecipeSHA256:       artifact.Bindings.RecipeSHA256,
	}
}

func (record CandidateRejectedRecord) Validate() error {
	if !candidateIDPattern.MatchString(record.CandidateID) {
		return fmt.Errorf("candidate rejection: invalid candidate_id %q", record.CandidateID)
	}
	if record.SourceFeatureState != "applied" && record.SourceFeatureState != "blocked" {
		return fmt.Errorf("candidate rejection: invalid source_feature_state %q", record.SourceFeatureState)
	}
	for name, value := range map[string]string{
		"upstream_commit": record.UpstreamCommit, "reference_commit": record.ReferenceCommit,
		"capture_event_sha256": record.CaptureEventSHA256, "coverage_sha256": record.CoverageSHA256,
		"patch_sha256": record.PatchSHA256, "recipe_sha256": record.RecipeSHA256,
	} {
		switch name {
		case "upstream_commit", "reference_commit":
			if err := validateCommit(value); err != nil {
				return fmt.Errorf("candidate rejection: %w", err)
			}
		default:
			if !sha256HexPattern.MatchString(value) {
				return fmt.Errorf("candidate rejection: %s must be a 64-character lowercase SHA-256", name)
			}
		}
	}
	return nil
}

func EncodeRejectedRecord(record CandidateRejectedRecord) ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func DecodeRejectedRecord(raw []byte) (CandidateRejectedRecord, error) {
	var record CandidateRejectedRecord
	if err := strictDecodeJSON(raw, &record, "candidate rejection"); err != nil {
		return CandidateRejectedRecord{}, err
	}
	if err := record.Validate(); err != nil {
		return CandidateRejectedRecord{}, err
	}
	return record, nil
}

func RejectedContainsCandidateID(records []CandidateRejectedRecord, candidateID string) bool {
	for _, row := range records {
		if row.CandidateID == candidateID {
			return true
		}
	}
	return false
}

func buildCandidateDerivation(plan Plan, effectMap []EffectResolution, effectSHA256ByOrdinal map[int]string) (CandidateDerivation, error) {
	if effectSHA256ByOrdinal == nil {
		return CandidateDerivation{}, fmt.Errorf("effect proof mapping is required")
	}
	classificationByEffect := make(map[int]string, len(plan.effects))
	for _, effect := range plan.effects {
		classificationByEffect[effect.ordinal] = string(effect.state)
	}
	operations := make([]CandidateOperation, 0, len(plan.operations))
	for ordinal, op := range plan.operations {
		effectSHA, ok := effectSHA256ByOrdinal[op.effectOrdinal]
		if !ok {
			return CandidateDerivation{}, fmt.Errorf("missing effect_sha256 for effect ordinal %d", op.effectOrdinal)
		}
		payloadSum := sha256.Sum256(op.payload)
		row := CandidateOperation{
			Ordinal: ordinal + 1, Kind: op.kind, Path: op.path,
			EffectOrdinal: op.effectOrdinal, EffectSHA256: effectSHA,
			RunOrdinals:     append([]int(nil), op.runOrdinals...),
			InsertionSHA256: hex.EncodeToString(payloadSum[:]), InsertionSize: len(op.payload),
			Classification: classificationByEffect[op.effectOrdinal],
		}
		if row.RunOrdinals == nil {
			row.RunOrdinals = []int{}
		}
		if row.Classification == "" {
			row.Classification = "applicable"
		}
		if op.kind == "insert-by-boundary" {
			boundaryHash, err := BoundaryProofSHA256(CandidateBoundaryProofInput{
				Path: op.path, BaseBoundaryIndex: op.boundary,
				UpstreamLeftIndex: op.left, UpstreamRightIndex: op.right,
				BaseRecordCount: op.baseRecordCount, UpstreamRecordCount: op.upstreamRecordCount,
				AlignmentLength: op.alignmentLength, InsertionRecordCount: op.insertionRecordCount,
				InsertionSHA256: row.InsertionSHA256,
			})
			if err != nil {
				return CandidateDerivation{}, err
			}
			row.BoundaryProofSHA256 = boundaryHash
		}
		if err := row.Validate(); err != nil {
			return CandidateDerivation{}, err
		}
		operations = append(operations, row)
	}
	mapped := make([]CandidateEffectMapEntry, 0, len(effectMap))
	for _, entry := range effectMap {
		effectSHA, ok := effectSHA256ByOrdinal[entry.EffectOrdinal]
		if !ok {
			return CandidateDerivation{}, fmt.Errorf("missing effect_sha256 for effect ordinal %d", entry.EffectOrdinal)
		}
		row := CandidateEffectMapEntry{
			EffectOrdinal: entry.EffectOrdinal, EffectSHA256: effectSHA, Path: entry.Path,
			Resolution: entry.State, OperationOrdinals: append([]int(nil), entry.OperationOrdinals...),
			ReasonCodes: []string{},
		}
		if row.OperationOrdinals == nil {
			row.OperationOrdinals = []int{}
		}
		if err := row.Validate(); err != nil {
			return CandidateDerivation{}, err
		}
		mapped = append(mapped, row)
	}
	sort.Slice(mapped, func(i, j int) bool { return mapped[i].EffectOrdinal < mapped[j].EffectOrdinal })
	absorbed := 0
	for _, entry := range mapped {
		if entry.Resolution == "absorbed-present" {
			absorbed++
		}
	}
	derivation := CandidateDerivation{
		EffectCount: len(mapped), AbsorbedPresentCount: absorbed,
		CandidateApplicableCount: len(mapped) - absorbed, ConflictCount: 0,
		Operations: operations, EffectMap: mapped,
	}
	if err := derivation.Validate(); err != nil {
		return CandidateDerivation{}, err
	}
	return derivation, nil
}

func sameIdentitySubset(left, right CandidateArtifact) bool {
	leftTuple, leftErr := IdentityTupleFromArtifact(left)
	rightTuple, rightErr := IdentityTupleFromArtifact(right)
	return leftErr == nil && rightErr == nil && reflect.DeepEqual(leftTuple, rightTuple)
}

func RejectableCandidateStatuses() []string {
	return []string{CandidateStatusPending, CandidateStatusStale}
}

func IsRejectableCandidateStatus(status string) bool {
	return slices.Contains(RejectableCandidateStatuses(), status)
}

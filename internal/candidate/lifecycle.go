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

type CandidatePrepareInput struct {
	Feature               string
	SourceFeatureState    string
	UpstreamCommit        string
	ReferenceCommit       string
	Bindings              CandidateBindings
	Reference             *ReferenceRoot
	Upstream              *ReferenceRoot
	Patch                 string
	Effects               []gitutil.PatchEffect
	EffectSHA256ByOrdinal map[int]string
}

type CandidatePreparation struct {
	authority *candidatePreparationAuthority
}

type candidatePreparationAuthority struct {
	feature            string
	sourceFeatureState string
	upstreamCommit     string
	referenceCommit    string
	bindings           CandidateBindings
	patch              string
	effects            []gitutil.PatchEffect
	plan               Plan
	effectMap          []EffectResolution
	derivation         CandidateDerivation
	candidateID        string
}

type CandidateMaterializeInput struct {
	Preparation CandidatePreparation
	Execution   ExecutionResult
}

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

func BuildCandidateArtifact(input CandidateMaterializeInput) (CandidateArtifact, error) {
	authority, err := input.Preparation.requireAuthority()
	if err != nil {
		return CandidateArtifact{}, err
	}
	if input.Execution.Root == nil {
		return CandidateArtifact{}, fmt.Errorf("candidate execution root is required")
	}
	if input.Execution.Root.CandidateID() != authority.candidateID {
		return CandidateArtifact{}, fmt.Errorf("candidate execution root/id mismatch")
	}
	if input.Execution.Root.Commit() != authority.upstreamCommit {
		return CandidateArtifact{}, fmt.Errorf("candidate execution root upstream mismatch")
	}
	if !reflect.DeepEqual(input.Execution.Plan, authority.plan) {
		return CandidateArtifact{}, fmt.Errorf("candidate execution plan differs from prepared authority")
	}
	if !reflect.DeepEqual(input.Execution.EffectMap, authority.effectMap) {
		return CandidateArtifact{}, fmt.Errorf("candidate execution effect_map differs from prepared authority")
	}
	validationResult, err := validatePreparedCandidateAgainstRoot(authority, input.Execution.Root)
	if err != nil {
		return CandidateArtifact{}, err
	}
	if input.Execution.TreeHash == "" || input.Execution.Diff == "" {
		return CandidateArtifact{}, fmt.Errorf("candidate execution tree/diff output is required")
	}
	if input.Execution.TreeHash != validationResult.treeHash || input.Execution.Diff != validationResult.diff {
		return CandidateArtifact{}, fmt.Errorf("candidate execution tree/diff drifted from production validation")
	}
	treeSHA, err := CandidateTreeIntegritySHA256(validationResult.treeHash)
	if err != nil {
		return CandidateArtifact{}, err
	}
	diffSum := sha256.Sum256([]byte(validationResult.diff))
	artifact := CandidateArtifact{
		SchemaVersion:      CandidateSchemaVersion,
		CandidateID:        authority.candidateID,
		Feature:            authority.feature,
		SourceFeatureState: authority.sourceFeatureState,
		UpstreamCommit:     authority.upstreamCommit,
		ReferenceCommit:    authority.referenceCommit,
		DerivationMode:     CandidateDerivationModeSingle,
		Bindings:           authority.bindings,
		Derivation:         authority.derivation,
		Result: CandidateResult{
			CandidateDiffSHA256: hex.EncodeToString(diffSum[:]),
			CandidateTreeSHA256: treeSHA,
			Validated:           CandidateValidationStructural,
		},
	}
	if err := artifact.Validate(); err != nil {
		return CandidateArtifact{}, err
	}
	return artifact, nil
}

func PrepareCandidateArtifact(input CandidatePrepareInput) (CandidatePreparation, error) {
	if input.Reference == nil || input.Upstream == nil {
		return CandidatePreparation{}, fmt.Errorf("typed reference and upstream roots are required")
	}
	if err := input.Reference.ensureAvailable(); err != nil {
		return CandidatePreparation{}, err
	}
	if err := input.Upstream.ensureAvailable(); err != nil {
		return CandidatePreparation{}, err
	}
	if input.Reference.Commit() != input.ReferenceCommit || input.Upstream.Commit() != input.UpstreamCommit {
		return CandidatePreparation{}, fmt.Errorf("prepare authority commit mismatch")
	}
	plan, err := derive(input.Reference, input.Upstream, input.Patch, input.Effects)
	if err != nil {
		return CandidatePreparation{}, err
	}
	if _, _, conflicts := plan.Counts(); conflicts != 0 {
		return CandidatePreparation{}, &refusal{code: "candidate-postcondition-failed", detail: "derivation includes conflicts"}
	}
	effectMap, err := buildEffectResolutionMap(plan)
	if err != nil {
		return CandidatePreparation{}, err
	}
	derivation, err := buildCandidateDerivation(plan, effectMap, input.EffectSHA256ByOrdinal)
	if err != nil {
		return CandidatePreparation{}, err
	}
	candidateID, _, _, err := ComputeCandidateID(CandidateIdentityInput{
		SchemaVersion:      CandidateSchemaVersion,
		Feature:            input.Feature,
		SourceFeatureState: input.SourceFeatureState,
		UpstreamCommit:     input.UpstreamCommit,
		ReferenceCommit:    input.ReferenceCommit,
		Bindings:           input.Bindings,
		DerivationMode:     CandidateDerivationModeSingle,
		Operations:         derivation.Operations,
		EffectMap:          derivation.EffectMap,
	})
	if err != nil {
		return CandidatePreparation{}, err
	}
	return CandidatePreparation{
		authority: &candidatePreparationAuthority{
			feature:            input.Feature,
			sourceFeatureState: input.SourceFeatureState,
			upstreamCommit:     input.UpstreamCommit,
			referenceCommit:    input.ReferenceCommit,
			bindings:           input.Bindings,
			patch:              input.Patch,
			effects:            append([]gitutil.PatchEffect(nil), input.Effects...),
			plan:               clonePlan(plan),
			effectMap:          cloneEffectResolution(effectMap),
			derivation:         cloneCandidateDerivation(derivation),
			candidateID:        candidateID,
		},
	}, nil
}

func (prep CandidatePreparation) CandidateID() (string, error) {
	authority, err := prep.requireAuthority()
	if err != nil {
		return "", err
	}
	return authority.candidateID, nil
}

func (prep CandidatePreparation) ValidateReusableCandidate(root *CandidateRoot) (CandidateResult, error) {
	authority, err := prep.requireAuthority()
	if err != nil {
		return CandidateResult{}, err
	}
	validationResult, err := validatePreparedCandidateAgainstRoot(authority, root)
	if err != nil {
		return CandidateResult{}, err
	}
	treeSHA, err := CandidateTreeIntegritySHA256(validationResult.treeHash)
	if err != nil {
		return CandidateResult{}, err
	}
	diffSum := sha256.Sum256([]byte(validationResult.diff))
	return CandidateResult{
		CandidateDiffSHA256: hex.EncodeToString(diffSum[:]),
		CandidateTreeSHA256: treeSHA,
		Validated:           CandidateValidationStructural,
	}, nil
}

func (prep CandidatePreparation) DerivedArtifactIdentity() (CandidateIdentityInput, error) {
	if prep.authority == nil {
		return CandidateIdentityInput{}, fmt.Errorf("candidate preparation authority is required")
	}
	authority := prep.authority
	return CandidateIdentityInput{
		SchemaVersion:      CandidateSchemaVersion,
		Feature:            authority.feature,
		SourceFeatureState: authority.sourceFeatureState,
		UpstreamCommit:     authority.upstreamCommit,
		ReferenceCommit:    authority.referenceCommit,
		Bindings:           authority.bindings,
		DerivationMode:     CandidateDerivationModeSingle,
		Operations:         append([]CandidateOperation(nil), authority.derivation.Operations...),
		EffectMap:          append([]CandidateEffectMapEntry(nil), authority.derivation.EffectMap...),
	}, nil
}

func (prep CandidatePreparation) requireAuthority() (*candidatePreparationAuthority, error) {
	if prep.authority == nil {
		return nil, fmt.Errorf("candidate preparation authority is required")
	}
	identity, err := prep.DerivedArtifactIdentity()
	if err != nil {
		return nil, err
	}
	candidateID, _, _, err := ComputeCandidateID(identity)
	if err != nil {
		return nil, err
	}
	if candidateID != prep.authority.candidateID {
		return nil, fmt.Errorf("candidate preparation authority drifted")
	}
	return prep.authority, nil
}

func validatePreparedCandidateAgainstRoot(authority *candidatePreparationAuthority, root *CandidateRoot) (validationResult, error) {
	if authority == nil {
		return validationResult{}, fmt.Errorf("candidate preparation authority is required")
	}
	if root == nil {
		return validationResult{}, fmt.Errorf("candidate root is required")
	}
	if root.CandidateID() != authority.candidateID {
		return validationResult{}, fmt.Errorf("candidate root/id mismatch")
	}
	if root.Commit() != authority.upstreamCommit {
		return validationResult{}, fmt.Errorf("candidate root upstream mismatch")
	}
	reference, err := OpenReferenceRoot(root.repoRoot, authority.referenceCommit)
	if err != nil {
		return validationResult{}, err
	}
	defer func() { _ = reference.Close() }()
	upstream, err := OpenReferenceRoot(root.repoRoot, authority.upstreamCommit)
	if err != nil {
		return validationResult{}, err
	}
	defer func() { _ = upstream.Close() }()
	return validateCandidate(reference, upstream, root, authority.patch, authority.effects, authority.plan)
}

func clonePlan(in Plan) Plan {
	out := Plan{
		operations: make([]operation, len(in.operations)),
		effects:    make([]effectClassification, len(in.effects)),
	}
	copy(out.operations, in.operations)
	copy(out.effects, in.effects)
	for i := range out.operations {
		out.operations[i].payload = append([]byte(nil), out.operations[i].payload...)
		out.operations[i].runOrdinals = append([]int(nil), out.operations[i].runOrdinals...)
	}
	return out
}

func cloneEffectResolution(in []EffectResolution) []EffectResolution {
	out := make([]EffectResolution, len(in))
	copy(out, in)
	for i := range out {
		out[i].OperationOrdinals = append([]int(nil), out[i].OperationOrdinals...)
	}
	return out
}

func cloneCandidateDerivation(in CandidateDerivation) CandidateDerivation {
	out := in
	out.Operations = append([]CandidateOperation(nil), in.Operations...)
	out.EffectMap = append([]CandidateEffectMapEntry(nil), in.EffectMap...)
	for i := range out.Operations {
		out.Operations[i].RunOrdinals = append([]int(nil), out.Operations[i].RunOrdinals...)
		if out.Operations[i].RunOrdinals == nil {
			out.Operations[i].RunOrdinals = []int{}
		}
	}
	for i := range out.EffectMap {
		out.EffectMap[i].OperationOrdinals = append([]int(nil), out.EffectMap[i].OperationOrdinals...)
		out.EffectMap[i].ReasonCodes = append([]string(nil), out.EffectMap[i].ReasonCodes...)
		if out.EffectMap[i].OperationOrdinals == nil {
			out.EffectMap[i].OperationOrdinals = []int{}
		}
		if out.EffectMap[i].ReasonCodes == nil {
			out.EffectMap[i].ReasonCodes = []string{}
		}
	}
	return out
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
		"upstream_commit":      record.UpstreamCommit,
		"reference_commit":     record.ReferenceCommit,
		"capture_event_sha256": record.CaptureEventSHA256,
		"coverage_sha256":      record.CoverageSHA256,
		"patch_sha256":         record.PatchSHA256,
		"recipe_sha256":        record.RecipeSHA256,
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
			Ordinal:         ordinal + 1,
			Kind:            op.kind,
			Path:            op.path,
			EffectOrdinal:   op.effectOrdinal,
			EffectSHA256:    effectSHA,
			RunOrdinals:     append([]int(nil), op.runOrdinals...),
			InsertionSHA256: hex.EncodeToString(payloadSum[:]),
			InsertionSize:   len(op.payload),
			Classification:  classificationByEffect[op.effectOrdinal],
		}
		if row.RunOrdinals == nil {
			row.RunOrdinals = []int{}
		}
		if row.Classification == "" {
			row.Classification = "applicable"
		}
		if op.kind == "insert-by-boundary" {
			boundaryHash, err := BoundaryProofSHA256(CandidateBoundaryProofInput{
				Path:                 op.path,
				BaseBoundaryIndex:    op.boundary,
				UpstreamLeftIndex:    op.left,
				UpstreamRightIndex:   op.right,
				BaseRecordCount:      op.baseRecordCount,
				UpstreamRecordCount:  op.upstreamRecordCount,
				AlignmentLength:      op.alignmentLength,
				InsertionRecordCount: op.insertionRecordCount,
				InsertionSHA256:      row.InsertionSHA256,
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
			EffectOrdinal:     entry.EffectOrdinal,
			EffectSHA256:      effectSHA,
			Path:              entry.Path,
			Resolution:        entry.State,
			OperationOrdinals: append([]int(nil), entry.OperationOrdinals...),
			ReasonCodes:       []string{},
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
		EffectCount:              len(mapped),
		AbsorbedPresentCount:     absorbed,
		CandidateApplicableCount: len(mapped) - absorbed,
		ConflictCount:            0,
		Operations:               operations,
		EffectMap:                mapped,
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

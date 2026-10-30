package candidate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tesseracode/tesserapatch/internal/store"
)

const (
	CandidateSchemaVersion           = 1
	CandidateDerivationModeSingle    = "single-slug"
	CandidateValidationStructural    = "structural"
	CandidateStatusPending           = "pending"
	CandidateStatusStale             = "stale"
	CandidateStatusRejected          = "rejected"
	CandidateStatusAccepted          = "accepted"
	CandidateAcceptValidationNone    = "none"
	CandidateAcceptValidationStruct  = "structural"
	CandidateAcceptValidationWithRun = "structural+tests"
)

var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type CandidateBindings struct {
	CaptureEventSHA256 string `json:"capture_event_sha256"`
	CoverageSHA256     string `json:"coverage_sha256"`
	PatchSHA256        string `json:"patch_sha256"`
	RecipeSHA256       string `json:"recipe_sha256"`
}

type CandidateOperation struct {
	Ordinal             int    `json:"ordinal"`
	Kind                string `json:"kind"`
	Path                string `json:"path"`
	EffectOrdinal       int    `json:"effect_ordinal"`
	EffectSHA256        string `json:"effect_sha256"`
	RunOrdinals         []int  `json:"run_ordinals"`
	InsertionSHA256     string `json:"insertion_sha256"`
	InsertionSize       int    `json:"insertion_size"`
	BoundaryProofSHA256 string `json:"boundary_proof_sha256"`
	Classification      string `json:"classification"`
}

type CandidateEffectMapEntry struct {
	EffectOrdinal     int      `json:"effect_ordinal"`
	EffectSHA256      string   `json:"effect_sha256"`
	Path              string   `json:"path"`
	Resolution        string   `json:"resolution"`
	OperationOrdinals []int    `json:"operation_ordinals"`
	ReasonCodes       []string `json:"reason_codes"`
}

type CandidateDerivation struct {
	EffectCount              int                       `json:"effect_count"`
	AbsorbedPresentCount     int                       `json:"absorbed_present_count"`
	CandidateApplicableCount int                       `json:"candidate_applicable_count"`
	ConflictCount            int                       `json:"conflict_count"`
	Operations               []CandidateOperation      `json:"operations"`
	EffectMap                []CandidateEffectMapEntry `json:"effect_map"`
}

type CandidateResult struct {
	CandidateDiffSHA256 string `json:"candidate_diff_sha256"`
	CandidateTreeSHA256 string `json:"candidate_tree_sha256"`
	Validated           string `json:"validated"`
}

type CandidateArtifact struct {
	SchemaVersion      int                 `json:"schema_version"`
	CandidateID        string              `json:"candidate_id"`
	Feature            string              `json:"feature"`
	SourceFeatureState string              `json:"source_feature_state"`
	UpstreamCommit     string              `json:"upstream_commit"`
	ReferenceCommit    string              `json:"reference_commit"`
	DerivationMode     string              `json:"derivation_mode"`
	Bindings           CandidateBindings   `json:"bindings"`
	Derivation         CandidateDerivation `json:"derivation"`
	Result             CandidateResult     `json:"result"`
}

type CandidateState struct {
	SchemaVersion    int    `json:"schema_version"`
	CandidateID      string `json:"candidate_id"`
	Status           string `json:"status"`
	AcceptValidation string `json:"accept_validation"`
}

func DecodeCandidateArtifact(raw []byte) (CandidateArtifact, error) {
	var artifact CandidateArtifact
	if err := strictDecodeJSON(raw, &artifact, "candidate"); err != nil {
		return CandidateArtifact{}, err
	}
	if err := artifact.Validate(); err != nil {
		return CandidateArtifact{}, err
	}
	return artifact, nil
}

func EncodeCandidateArtifact(artifact CandidateArtifact) ([]byte, error) {
	if err := artifact.Validate(); err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func DecodeCandidateState(raw []byte) (CandidateState, error) {
	var state CandidateState
	if err := strictDecodeJSON(raw, &state, "candidate state"); err != nil {
		return CandidateState{}, err
	}
	if err := state.Validate(); err != nil {
		return CandidateState{}, err
	}
	return state, nil
}

func EncodeCandidateState(state CandidateState) ([]byte, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func (artifact CandidateArtifact) Validate() error {
	if artifact.SchemaVersion != CandidateSchemaVersion {
		return fmt.Errorf("candidate: unsupported schema_version %d", artifact.SchemaVersion)
	}
	if !candidateIDPattern.MatchString(artifact.CandidateID) {
		return fmt.Errorf("candidate: invalid candidate_id %q", artifact.CandidateID)
	}
	if !slugPattern.MatchString(artifact.Feature) {
		return fmt.Errorf("candidate: invalid feature %q", artifact.Feature)
	}
	if artifact.SourceFeatureState != "applied" && artifact.SourceFeatureState != "blocked" {
		return fmt.Errorf("candidate: invalid source_feature_state %q", artifact.SourceFeatureState)
	}
	if err := validateCommit(artifact.UpstreamCommit); err != nil {
		return fmt.Errorf("candidate: %w", err)
	}
	if err := validateCommit(artifact.ReferenceCommit); err != nil {
		return fmt.Errorf("candidate: %w", err)
	}
	if artifact.DerivationMode != CandidateDerivationModeSingle {
		return fmt.Errorf("candidate: invalid derivation_mode %q", artifact.DerivationMode)
	}
	if err := artifact.Bindings.Validate(); err != nil {
		return err
	}
	if err := artifact.Derivation.Validate(); err != nil {
		return err
	}
	if err := artifact.Result.Validate(); err != nil {
		return err
	}
	return nil
}

func (bindings CandidateBindings) Validate() error {
	for name, value := range map[string]string{
		"capture_event_sha256": bindings.CaptureEventSHA256,
		"coverage_sha256":      bindings.CoverageSHA256,
		"patch_sha256":         bindings.PatchSHA256,
		"recipe_sha256":        bindings.RecipeSHA256,
	} {
		if !sha256HexPattern.MatchString(value) {
			return fmt.Errorf("candidate: bindings.%s must be a 64-character lowercase SHA-256", name)
		}
	}
	return nil
}

func (result CandidateResult) Validate() error {
	for name, value := range map[string]string{
		"candidate_diff_sha256": result.CandidateDiffSHA256,
		"candidate_tree_sha256": result.CandidateTreeSHA256,
	} {
		if !sha256HexPattern.MatchString(value) {
			return fmt.Errorf("candidate: result.%s must be a 64-character lowercase SHA-256", name)
		}
	}
	if result.Validated != CandidateValidationStructural {
		return fmt.Errorf("candidate: result.validated must be %q", CandidateValidationStructural)
	}
	return nil
}

func (state CandidateState) Validate() error {
	if state.SchemaVersion != CandidateSchemaVersion {
		return fmt.Errorf("candidate state: unsupported schema_version %d", state.SchemaVersion)
	}
	if !candidateIDPattern.MatchString(state.CandidateID) {
		return fmt.Errorf("candidate state: invalid candidate_id %q", state.CandidateID)
	}
	switch state.Status {
	case CandidateStatusPending, CandidateStatusStale, CandidateStatusRejected, CandidateStatusAccepted:
	default:
		return fmt.Errorf("candidate state: invalid status %q", state.Status)
	}
	switch state.AcceptValidation {
	case CandidateAcceptValidationNone, CandidateAcceptValidationStruct, CandidateAcceptValidationWithRun:
	default:
		return fmt.Errorf("candidate state: invalid accept_validation %q", state.AcceptValidation)
	}
	return nil
}

func (derivation CandidateDerivation) Validate() error {
	if derivation.EffectCount < 0 || derivation.AbsorbedPresentCount < 0 ||
		derivation.CandidateApplicableCount < 0 || derivation.ConflictCount < 0 {
		return fmt.Errorf("candidate: derivation counts must be nonnegative")
	}
	if derivation.Operations == nil || derivation.EffectMap == nil {
		return fmt.Errorf("candidate: derivation operations/effect_map must be non-null arrays")
	}
	if !sortedUniquePositiveIntField(derivation.Operations, func(row CandidateOperation) int { return row.Ordinal }) {
		return fmt.Errorf("candidate: derivation.operations must be sorted by ordinal with no duplicates")
	}
	if !sortedUniquePositiveIntField(derivation.EffectMap, func(row CandidateEffectMapEntry) int { return row.EffectOrdinal }) {
		return fmt.Errorf("candidate: derivation.effect_map must be sorted by effect_ordinal with no duplicates")
	}
	effects := make(map[int]CandidateEffectMapEntry, len(derivation.EffectMap))
	applicableByEffect := make(map[int][]int, len(derivation.EffectMap))
	absorbedCount, applicableCount := 0, 0
	for _, row := range derivation.EffectMap {
		if err := row.Validate(); err != nil {
			return err
		}
		effects[row.EffectOrdinal] = row
		switch row.Resolution {
		case "absorbed-present":
			absorbedCount++
		case "candidate-applicable":
			applicableCount++
		default:
			return fmt.Errorf("candidate: invalid effect_map resolution %q", row.Resolution)
		}
	}
	for _, op := range derivation.Operations {
		if err := op.Validate(); err != nil {
			return err
		}
		row, ok := effects[op.EffectOrdinal]
		if !ok {
			return fmt.Errorf("candidate: operation ordinal %d references unknown effect_ordinal %d", op.Ordinal, op.EffectOrdinal)
		}
		if row.Path != op.Path || row.EffectSHA256 != op.EffectSHA256 {
			return fmt.Errorf("candidate: operation ordinal %d contradicts effect_map proof binding", op.Ordinal)
		}
		applicableByEffect[op.EffectOrdinal] = append(applicableByEffect[op.EffectOrdinal], op.Ordinal)
	}
	for _, row := range derivation.EffectMap {
		ops := append([]int(nil), applicableByEffect[row.EffectOrdinal]...)
		if ops == nil {
			ops = []int{}
		}
		if !slices.Equal(ops, row.OperationOrdinals) {
			return fmt.Errorf("candidate: effect_map ordinal %d operation_ordinals contradict operation set", row.EffectOrdinal)
		}
		if row.Resolution == "absorbed-present" && len(row.OperationOrdinals) != 0 {
			return fmt.Errorf("candidate: absorbed-present effect %d cannot carry operation_ordinals", row.EffectOrdinal)
		}
		if row.Resolution == "candidate-applicable" && len(row.OperationOrdinals) == 0 {
			return fmt.Errorf("candidate: candidate-applicable effect %d requires operation_ordinals", row.EffectOrdinal)
		}
	}
	if derivation.EffectCount != len(derivation.EffectMap) {
		return fmt.Errorf("candidate: derivation.effect_count=%d mismatches effect_map length=%d", derivation.EffectCount, len(derivation.EffectMap))
	}
	if derivation.AbsorbedPresentCount != absorbedCount {
		return fmt.Errorf("candidate: absorbed_present_count=%d mismatches effect_map=%d", derivation.AbsorbedPresentCount, absorbedCount)
	}
	if derivation.CandidateApplicableCount != applicableCount {
		return fmt.Errorf("candidate: candidate_applicable_count=%d mismatches effect_map=%d", derivation.CandidateApplicableCount, applicableCount)
	}
	if derivation.ConflictCount != derivation.EffectCount-absorbedCount-applicableCount {
		return fmt.Errorf("candidate: conflict_count contradicts effect_map partition")
	}
	return nil
}

func (operation CandidateOperation) Validate() error {
	if operation.Ordinal <= 0 {
		return fmt.Errorf("candidate: operation ordinal must be positive")
	}
	switch operation.Kind {
	case "create-file", "insert-by-boundary":
	default:
		return fmt.Errorf("candidate: invalid operation kind %q", operation.Kind)
	}
	if err := validateCanonicalRelativePath(operation.Path); err != nil {
		return fmt.Errorf("candidate: operation path: %w", err)
	}
	if operation.EffectOrdinal <= 0 {
		return fmt.Errorf("candidate: effect_ordinal must be positive")
	}
	if !sha256HexPattern.MatchString(operation.EffectSHA256) {
		return fmt.Errorf("candidate: effect_sha256 must be a 64-character lowercase SHA-256")
	}
	if operation.RunOrdinals == nil {
		return fmt.Errorf("candidate: run_ordinals must be a non-null array")
	}
	if !sortedUniquePositiveInts(operation.RunOrdinals) {
		return fmt.Errorf("candidate: run_ordinals must be sorted positive unique values")
	}
	if !sha256HexPattern.MatchString(operation.InsertionSHA256) {
		return fmt.Errorf("candidate: insertion_sha256 must be a 64-character lowercase SHA-256")
	}
	if operation.InsertionSize < 0 {
		return fmt.Errorf("candidate: insertion_size must be nonnegative")
	}
	switch operation.Classification {
	case "present", "applicable", "conflict":
	default:
		return fmt.Errorf("candidate: invalid classification %q", operation.Classification)
	}
	switch operation.Kind {
	case "create-file":
		if len(operation.RunOrdinals) != 0 {
			return fmt.Errorf("candidate: create-file cannot carry run_ordinals")
		}
		if operation.BoundaryProofSHA256 != "" {
			return fmt.Errorf("candidate: create-file cannot carry boundary_proof_sha256")
		}
	case "insert-by-boundary":
		if len(operation.RunOrdinals) == 0 {
			return fmt.Errorf("candidate: insert-by-boundary requires run_ordinals")
		}
		if !sha256HexPattern.MatchString(operation.BoundaryProofSHA256) {
			return fmt.Errorf("candidate: insert-by-boundary boundary_proof_sha256 must be a 64-character lowercase SHA-256")
		}
	}
	return nil
}

func (entry CandidateEffectMapEntry) Validate() error {
	if entry.EffectOrdinal <= 0 {
		return fmt.Errorf("candidate: effect_map effect_ordinal must be positive")
	}
	if !sha256HexPattern.MatchString(entry.EffectSHA256) {
		return fmt.Errorf("candidate: effect_map effect_sha256 must be a 64-character lowercase SHA-256")
	}
	if err := validateCanonicalRelativePath(entry.Path); err != nil {
		return fmt.Errorf("candidate: effect_map path: %w", err)
	}
	switch entry.Resolution {
	case "absorbed-present", "candidate-applicable":
	default:
		return fmt.Errorf("candidate: invalid effect_map resolution %q", entry.Resolution)
	}
	if entry.OperationOrdinals == nil {
		return fmt.Errorf("candidate: effect_map operation_ordinals must be a non-null array")
	}
	if !sortedUniquePositiveInts(entry.OperationOrdinals) {
		return fmt.Errorf("candidate: effect_map operation_ordinals must be sorted positive unique values")
	}
	if entry.ReasonCodes == nil {
		return fmt.Errorf("candidate: effect_map reason_codes must be a non-null array")
	}
	if !sortedUniqueStrings(entry.ReasonCodes) {
		return fmt.Errorf("candidate: effect_map reason_codes must be sorted unique values")
	}
	return nil
}

func strictDecodeJSON(raw []byte, out any, label string) error {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return fmt.Errorf("%s: invalid UTF-8 or JSON", label)
	}
	if err := ensureNoDuplicateObjectKeys(raw); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	var node store.CanonNode
	if err := node.UnmarshalJSON(raw); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	target := reflect.TypeOf(out)
	if target.Kind() != reflect.Pointer || target.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("%s: strict decoder requires pointer to struct", label)
	}
	if err := strictRequiredShape(node, target.Elem(), label); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

func ensureNoDuplicateObjectKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := decodeNoDuplicatesValue(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON tokens")
		}
		return err
	}
	return nil
}

func decodeNoDuplicatesValue(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key must be string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := decodeNoDuplicatesValue(dec); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil {
			return err
		}
		if endDelim, ok := end.(json.Delim); !ok || endDelim != '}' {
			return fmt.Errorf("malformed object terminator")
		}
	case '[':
		for dec.More() {
			if err := decodeNoDuplicatesValue(dec); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil {
			return err
		}
		if endDelim, ok := end.(json.Delim); !ok || endDelim != ']' {
			return fmt.Errorf("malformed array terminator")
		}
	default:
		return fmt.Errorf("unexpected delimiter %q", delim)
	}
	return nil
}

func strictRequiredShape(node store.CanonNode, typ reflect.Type, at string) error {
	switch typ.Kind() {
	case reflect.Struct:
		if node.Kind != store.CanonKindObject || len(node.Object) != typ.NumField() {
			return fmt.Errorf("%s: every declared field is required and no unknown fields are allowed", at)
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			name := field.Tag.Get("json")
			value, ok := node.Field(name)
			if !ok {
				return fmt.Errorf("%s: missing field %s", at, name)
			}
			if err := strictRequiredShape(value, field.Type, at+"."+name); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if node.Kind != store.CanonKindArray {
			return fmt.Errorf("%s: expected non-null array", at)
		}
		for _, item := range node.Array {
			if err := strictRequiredShape(item, typ.Elem(), at+"[]"); err != nil {
				return err
			}
		}
	case reflect.String:
		if node.Kind != store.CanonKindString {
			return fmt.Errorf("%s: expected string", at)
		}
	case reflect.Bool:
		if node.Kind != store.CanonKindBool {
			return fmt.Errorf("%s: expected boolean", at)
		}
	case reflect.Int:
		if node.Kind != store.CanonKindUint {
			return fmt.Errorf("%s: expected nonnegative integer", at)
		}
	default:
		return fmt.Errorf("%s: unsupported schema type", at)
	}
	return nil
}

func validateCanonicalRelativePath(value string) error {
	if value == "" {
		return fmt.Errorf("path is required")
	}
	if strings.Contains(value, "\\") {
		return fmt.Errorf("path must use slash separators")
	}
	if strings.Contains(value, ":") {
		return fmt.Errorf("path contains unsupported colon separator")
	}
	if strings.HasPrefix(value, "/") {
		return fmt.Errorf("path must be repository-relative")
	}
	clean := path.Clean(value)
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return fmt.Errorf("path must remain inside the repository")
	}
	if clean != value {
		return fmt.Errorf("path must be slash-normalized without dot segments")
	}
	if !fs.ValidPath(clean) {
		return fmt.Errorf("path is not a valid relative path")
	}
	return nil
}

func sortedUniquePositiveInts(values []int) bool {
	for i, value := range values {
		if value <= 0 {
			return false
		}
		if i > 0 && values[i-1] >= value {
			return false
		}
	}
	return true
}

func sortedUniqueStrings(values []string) bool {
	for i, value := range values {
		if value == "" {
			return false
		}
		if i > 0 && values[i-1] >= value {
			return false
		}
	}
	return true
}

func sortedUniquePositiveIntField[T any](rows []T, value func(T) int) bool {
	for i := 1; i < len(rows); i++ {
		if value(rows[i-1]) >= value(rows[i]) {
			return false
		}
	}
	if len(rows) > 0 && value(rows[0]) <= 0 {
		return false
	}
	return true
}

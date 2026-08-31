package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/redact"
)

const IntentArchiveSchemaVersion = 1

const (
	IntentArchiveModeRegenerate       = "regenerate"
	IntentArchiveRepairCWD            = "workspace-root"
	IntentArchivePurgeStateConsistent = "index-decodes-and-is-internally-consistent"
)

type IntentArchiveArtifactID string

const (
	IntentArchiveArtifactAnalysis        IntentArchiveArtifactID = "analysis"
	IntentArchiveArtifactAnalysisSidecar IntentArchiveArtifactID = "analysis_sidecar"
	IntentArchiveArtifactExploration     IntentArchiveArtifactID = "exploration"
	IntentArchiveArtifactSpec            IntentArchiveArtifactID = "spec"
)

var intentArchiveArtifactOrder = [...]IntentArchiveArtifactID{
	IntentArchiveArtifactAnalysis,
	IntentArchiveArtifactAnalysisSidecar,
	IntentArchiveArtifactExploration,
	IntentArchiveArtifactSpec,
}

type IntentArchiveIndex struct {
	SchemaVersion int                       `json:"schema_version"`
	Feature       string                    `json:"feature"`
	Generations   []IntentArchiveGeneration `json:"generations"`
}

type IntentArchiveGeneration struct {
	GenerationID string                     `json:"generation_id"`
	Mode         string                     `json:"mode"`
	Replaced     []IntentArchiveReplacement `json:"replaced"`
}

type IntentArchiveReplacement struct {
	ArtifactID    IntentArchiveArtifactID `json:"artifact_id"`
	Path          string                  `json:"path"`
	ContentSHA256 string                  `json:"content_sha256"`
	Blob          string                  `json:"blob"`
	SizeBytes     int64                   `json:"size_bytes"`
	Purged        bool                    `json:"purged"`
	PurgePending  bool                    `json:"purge_pending"`
}

type intentArchiveGenerationBody struct {
	Feature  string                                  `json:"feature"`
	Mode     string                                  `json:"mode"`
	Replaced []intentArchiveImmutableReplacementBody `json:"replaced"`
}

type intentArchiveImmutableReplacementBody struct {
	ArtifactID    IntentArchiveArtifactID `json:"artifact_id"`
	Path          string                  `json:"path"`
	ContentSHA256 string                  `json:"content_sha256"`
	SizeBytes     int64                   `json:"size_bytes"`
}

type IntentArchiveErrorCode string

const (
	IntentArchiveCodeIndexCorrupt               IntentArchiveErrorCode = "archive-index-corrupt"
	IntentArchiveCodeVersionUnsupported         IntentArchiveErrorCode = "archive-index-version-unsupported"
	IntentArchiveCodeIndexForeign               IntentArchiveErrorCode = "archive-index-foreign"
	IntentArchiveCodeIndexPathEscape            IntentArchiveErrorCode = "archive-index-path-escape"
	IntentArchiveCodeGenerationMismatch         IntentArchiveErrorCode = "archive-index-generation-mismatch"
	IntentArchiveCodeGenerationCollision        IntentArchiveErrorCode = "archive-generation-id-collision"
	IntentArchiveCodeContentSensitive           IntentArchiveErrorCode = "archive-content-refused-sensitive"
	IntentArchiveCodeBlobDangling               IntentArchiveErrorCode = "archive-blob-dangling"
	IntentArchiveCodeBlobCorrupt                IntentArchiveErrorCode = "archive-blob-corrupt"
	IntentArchiveCodeIndexStorageInconsistent   IntentArchiveErrorCode = "archive-index-storage-inconsistent"
	IntentArchiveCodeRecoveryPending            IntentArchiveErrorCode = "recovery-pending"
	IntentArchiveCodePurgeEvidenceDivergent     IntentArchiveErrorCode = "archive-purge-evidence-divergent"
	IntentArchiveCodeIndexChanged               IntentArchiveErrorCode = "archive-index-changed"
	IntentArchiveCodePurgeIndexChanged          IntentArchiveErrorCode = "archive-purge-index-changed"
	IntentArchiveCodeBlobShared                 IntentArchiveErrorCode = "archive-blob-shared"
	IntentArchiveCodeSelectorInvalid            IntentArchiveErrorCode = "archive-selector-invalid"
	IntentArchiveCodePurgePartial               IntentArchiveErrorCode = "archive-purge-partial"
	IntentArchiveCodeStorageFailed              IntentArchiveErrorCode = "archive-storage-failed"
	IntentArchiveCodeRegenerateGenerationFailed IntentArchiveErrorCode = "regenerate-generation-failed"
)

type IntentArchiveError struct {
	Code         IntentArchiveErrorCode
	Hash         string
	GenerationID string
	ArtifactID   IntentArchiveArtifactID
	Class        string
	Detail       string
	ExitClass    int
	Committed    bool
}

func (e *IntentArchiveError) Error() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(string(e.Code))
	if e.Hash != "" {
		b.WriteString(" (")
		b.WriteString(e.Hash)
		b.WriteByte(')')
	}
	if e.GenerationID != "" {
		b.WriteString(" (")
		b.WriteString(e.GenerationID)
		b.WriteByte(')')
	}
	if e.ArtifactID != "" {
		b.WriteString(" (")
		b.WriteString(string(e.ArtifactID))
		b.WriteByte(')')
	}
	if e.Class != "" {
		b.WriteString(" [")
		b.WriteString(e.Class)
		b.WriteByte(']')
	}
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	return b.String()
}

func intentArchiveError(code IntentArchiveErrorCode, detail string, exitClass int) *IntentArchiveError {
	return &IntentArchiveError{Code: code, Detail: detail, ExitClass: exitClass}
}

func NewIntentArchiveIndex(feature string) (IntentArchiveIndex, error) {
	if !validIntentArchiveSlug(feature) {
		return IntentArchiveIndex{}, intentArchiveError(IntentArchiveCodeIndexPathEscape, "the feature slug is invalid", 3)
	}
	return IntentArchiveIndex{
		SchemaVersion: IntentArchiveSchemaVersion,
		Feature:       feature,
		Generations:   []IntentArchiveGeneration{},
	}, nil
}

func IntentArchiveRootRel(feature string) (string, error) {
	if !validIntentArchiveSlug(feature) {
		return "", intentArchiveError(IntentArchiveCodeIndexPathEscape, "the feature slug is invalid", 3)
	}
	return ".tpatch/features/" + feature + "/artifacts/intent-archive", nil
}

func IntentArchiveIndexRel(feature string) (string, error) {
	root, err := IntentArchiveRootRel(feature)
	if err != nil {
		return "", err
	}
	return root + "/index.json", nil
}

func IntentArchiveBlobsRel(feature string) (string, error) {
	root, err := IntentArchiveRootRel(feature)
	if err != nil {
		return "", err
	}
	return root + "/blobs", nil
}

func IntentArchiveBlobRel(feature, hash string) (string, error) {
	blobs, err := IntentArchiveBlobsRel(feature)
	if err != nil {
		return "", err
	}
	if !validIntentArchiveHash(hash) {
		return "", intentArchiveError(IntentArchiveCodeIndexPathEscape, "the blob hash is invalid", 3)
	}
	return blobs + "/" + hash + ".blob", nil
}

func IntentArchiveArtifactPath(id IntentArchiveArtifactID) (string, error) {
	switch id {
	case IntentArchiveArtifactAnalysis:
		return "analysis.md", nil
	case IntentArchiveArtifactAnalysisSidecar:
		return "artifacts/analysis.json", nil
	case IntentArchiveArtifactExploration:
		return "exploration.md", nil
	case IntentArchiveArtifactSpec:
		return "spec.md", nil
	default:
		return "", intentArchiveError(IntentArchiveCodeIndexCorrupt, "the artifact id is not in the closed intent set", 3)
	}
}

func DecodeIntentArchiveIndex(data []byte, expectedFeature string) (IntentArchiveIndex, error) {
	if !validIntentArchiveSlug(expectedFeature) {
		return IntentArchiveIndex{}, intentArchiveError(IntentArchiveCodeIndexPathEscape, "the expected feature slug is invalid", 3)
	}
	if err := validateIntentArchiveJSONTokens(data); err != nil {
		return IntentArchiveIndex{}, intentArchiveError(IntentArchiveCodeIndexCorrupt, "index.json does not match the exact version-1 wire schema", 3)
	}
	var index IntentArchiveIndex
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&index); err != nil {
		return IntentArchiveIndex{}, intentArchiveError(IntentArchiveCodeIndexCorrupt, "index.json does not decode", 3)
	}
	if err := requireIntentArchiveJSONEOF(decoder); err != nil {
		return IntentArchiveIndex{}, intentArchiveError(IntentArchiveCodeIndexCorrupt, "index.json carries trailing content", 3)
	}
	if err := ValidateIntentArchiveIndex(index, expectedFeature); err != nil {
		return IntentArchiveIndex{}, err
	}
	return index, nil
}

func EncodeIntentArchiveIndex(index IntentArchiveIndex) ([]byte, error) {
	if err := ValidateIntentArchiveIndex(index, index.Feature); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, intentArchiveError(IntentArchiveCodeIndexCorrupt, "index.json could not be encoded", 3)
	}
	return append(data, '\n'), nil
}

func ValidateIntentArchiveIndex(index IntentArchiveIndex, expectedFeature string) error {
	if index.SchemaVersion > IntentArchiveSchemaVersion {
		return intentArchiveError(IntentArchiveCodeVersionUnsupported, "index.json requires a newer tpatch version", 3)
	}
	if index.SchemaVersion != IntentArchiveSchemaVersion {
		return intentArchiveError(IntentArchiveCodeIndexCorrupt, "index.json has an invalid schema version", 3)
	}
	if !validIntentArchiveSlug(expectedFeature) || index.Feature != expectedFeature {
		return intentArchiveError(IntentArchiveCodeIndexForeign, "index.json belongs to another feature", 3)
	}
	if index.Generations == nil {
		return intentArchiveError(IntentArchiveCodeIndexCorrupt, "generations must be a non-null array", 3)
	}
	seenGenerations := make(map[string]struct{}, len(index.Generations))
	hashSizes := make(map[string]int64)
	for _, generation := range index.Generations {
		if !validIntentArchiveHash(generation.GenerationID) {
			return intentArchiveError(IntentArchiveCodeIndexCorrupt, "generation_id must be a full lowercase SHA-256", 3)
		}
		if _, exists := seenGenerations[generation.GenerationID]; exists {
			return intentArchiveError(IntentArchiveCodeIndexCorrupt, "generation_id values must be unique", 3)
		}
		seenGenerations[generation.GenerationID] = struct{}{}
		if generation.Mode != IntentArchiveModeRegenerate {
			return intentArchiveError(IntentArchiveCodeIndexCorrupt, "generation mode must be regenerate", 3)
		}
		if generation.Replaced == nil || len(generation.Replaced) == 0 {
			return intentArchiveError(IntentArchiveCodeIndexCorrupt, "replaced must be a non-null, non-empty array", 3)
		}
		seenArtifacts := make(map[IntentArchiveArtifactID]struct{}, len(generation.Replaced))
		previousArtifact := ""
		for replacementIndex, replacement := range generation.Replaced {
			wantPath, err := IntentArchiveArtifactPath(replacement.ArtifactID)
			if err != nil {
				return err
			}
			if _, exists := seenArtifacts[replacement.ArtifactID]; exists {
				return intentArchiveError(IntentArchiveCodeIndexCorrupt, "artifact ids must be unique within a generation", 3)
			}
			seenArtifacts[replacement.ArtifactID] = struct{}{}
			if replacementIndex > 0 && previousArtifact >= string(replacement.ArtifactID) {
				return intentArchiveError(IntentArchiveCodeIndexCorrupt, "replaced entries must be sorted by artifact_id", 3)
			}
			previousArtifact = string(replacement.ArtifactID)
			if replacement.Path != wantPath || !validIntentArchiveFeatureRelativePath(replacement.Path) {
				err := intentArchiveError(IntentArchiveCodeIndexPathEscape, "a replacement path is not the canonical feature-relative artifact path", 3)
				err.ArtifactID = replacement.ArtifactID
				return err
			}
			if !validIntentArchiveHash(replacement.ContentSHA256) || replacement.SizeBytes < 0 {
				return intentArchiveError(IntentArchiveCodeIndexCorrupt, "a replacement has an invalid immutable content identity", 3)
			}
			if err := validateIntentArchiveWireState(replacement); err != nil {
				return err
			}
			if size, exists := hashSizes[replacement.ContentSHA256]; exists && size != replacement.SizeBytes {
				return intentArchiveError(IntentArchiveCodeIndexCorrupt, "one content hash records inconsistent sizes", 3)
			}
			hashSizes[replacement.ContentSHA256] = replacement.SizeBytes
		}
		recomputed, _, err := ComputeIntentArchiveGenerationID(expectedFeature, generation.Replaced)
		if err != nil || recomputed != generation.GenerationID {
			bindErr := intentArchiveError(IntentArchiveCodeGenerationMismatch, "generation_id does not reproduce from the immutable body", 3)
			bindErr.GenerationID = generation.GenerationID
			return bindErr
		}
	}
	return nil
}

func ComputeIntentArchiveGenerationID(feature string, replacements []IntentArchiveReplacement) (string, []byte, error) {
	if !validIntentArchiveSlug(feature) || replacements == nil || len(replacements) == 0 {
		return "", nil, intentArchiveError(IntentArchiveCodeIndexCorrupt, "the immutable generation body is invalid", 3)
	}
	sorted := append([]IntentArchiveReplacement(nil), replacements...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].ArtifactID < sorted[j].ArtifactID
	})
	body := intentArchiveGenerationBody{
		Feature:  feature,
		Mode:     IntentArchiveModeRegenerate,
		Replaced: make([]intentArchiveImmutableReplacementBody, 0, len(sorted)),
	}
	seen := make(map[IntentArchiveArtifactID]struct{}, len(sorted))
	for _, replacement := range sorted {
		wantPath, err := IntentArchiveArtifactPath(replacement.ArtifactID)
		if err != nil {
			return "", nil, err
		}
		if _, exists := seen[replacement.ArtifactID]; exists {
			return "", nil, intentArchiveError(IntentArchiveCodeIndexCorrupt, "the immutable body repeats an artifact id", 3)
		}
		seen[replacement.ArtifactID] = struct{}{}
		if replacement.Path != wantPath || !validIntentArchiveHash(replacement.ContentSHA256) || replacement.SizeBytes < 0 {
			return "", nil, intentArchiveError(IntentArchiveCodeIndexCorrupt, "the immutable replacement body is invalid", 3)
		}
		body.Replaced = append(body.Replaced, intentArchiveImmutableReplacementBody{
			ArtifactID:    replacement.ArtifactID,
			Path:          replacement.Path,
			ContentSHA256: replacement.ContentSHA256,
			SizeBytes:     replacement.SizeBytes,
		})
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return "", nil, intentArchiveError(IntentArchiveCodeIndexCorrupt, "the immutable generation body could not be encoded", 3)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), canonical, nil
}

var intentArchiveCandidateGenerationID = func(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func SetIntentArchiveGenerationIDDeriverForTest(fn func([]byte) string) func() {
	previous := intentArchiveCandidateGenerationID
	intentArchiveCandidateGenerationID = fn
	return func() {
		intentArchiveCandidateGenerationID = previous
	}
}

type intentArchiveJSONKind uint8

const (
	intentArchiveJSONString intentArchiveJSONKind = iota + 1
	intentArchiveJSONInteger
	intentArchiveJSONBool
	intentArchiveJSONGenerations
	intentArchiveJSONReplacements
)

type intentArchiveJSONField struct {
	name string
	kind intentArchiveJSONKind
}

var intentArchiveIndexFields = [...]intentArchiveJSONField{
	{name: "schema_version", kind: intentArchiveJSONInteger},
	{name: "feature", kind: intentArchiveJSONString},
	{name: "generations", kind: intentArchiveJSONGenerations},
}

var intentArchiveGenerationFields = [...]intentArchiveJSONField{
	{name: "generation_id", kind: intentArchiveJSONString},
	{name: "mode", kind: intentArchiveJSONString},
	{name: "replaced", kind: intentArchiveJSONReplacements},
}

var intentArchiveReplacementFields = [...]intentArchiveJSONField{
	{name: "artifact_id", kind: intentArchiveJSONString},
	{name: "path", kind: intentArchiveJSONString},
	{name: "content_sha256", kind: intentArchiveJSONString},
	{name: "blob", kind: intentArchiveJSONString},
	{name: "size_bytes", kind: intentArchiveJSONInteger},
	{name: "purged", kind: intentArchiveJSONBool},
	{name: "purge_pending", kind: intentArchiveJSONBool},
}

func validateIntentArchiveJSONTokens(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return errors.New("index is not an object")
	}
	if err := validateIntentArchiveJSONObject(decoder, intentArchiveIndexFields[:]); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON token")
		}
		return err
	}
	return nil
}

func validateIntentArchiveJSONObject(decoder *json.Decoder, schema []intentArchiveJSONField) error {
	seen := make([]bool, len(schema))
	seenFolded := make(map[string]struct{}, len(schema))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("object key is not a string")
		}
		index, alias := intentArchiveJSONFieldIndex(schema, key)
		if index < 0 || alias || seen[index] {
			return errors.New("unknown, aliased, or duplicate object key")
		}
		folded := strings.ToLower(key)
		if _, duplicate := seenFolded[folded]; duplicate {
			return errors.New("duplicate case-aliased object key")
		}
		seen[index] = true
		seenFolded[folded] = struct{}{}
		value, err := decoder.Token()
		if err != nil {
			return err
		}
		if err := validateIntentArchiveJSONValue(decoder, value, schema[index].kind); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return errors.New("unterminated object")
	}
	for _, present := range seen {
		if !present {
			return errors.New("missing object key")
		}
	}
	return nil
}

func validateIntentArchiveJSONValue(decoder *json.Decoder, value json.Token, kind intentArchiveJSONKind) error {
	switch kind {
	case intentArchiveJSONString:
		if _, ok := value.(string); !ok {
			return errors.New("value is not a string")
		}
	case intentArchiveJSONInteger:
		number, ok := value.(json.Number)
		if !ok || !isIntentArchiveUnsignedInteger(number.String()) {
			return errors.New("value is not a non-negative integer")
		}
	case intentArchiveJSONBool:
		if _, ok := value.(bool); !ok {
			return errors.New("value is not a boolean")
		}
	case intentArchiveJSONGenerations:
		if value != json.Delim('[') {
			return errors.New("generations is not an array")
		}
		for decoder.More() {
			start, err := decoder.Token()
			if err != nil || start != json.Delim('{') {
				return errors.New("generation is not an object")
			}
			if err := validateIntentArchiveJSONObject(decoder, intentArchiveGenerationFields[:]); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unterminated generations array")
		}
	case intentArchiveJSONReplacements:
		if value != json.Delim('[') {
			return errors.New("replaced is not an array")
		}
		for decoder.More() {
			start, err := decoder.Token()
			if err != nil || start != json.Delim('{') {
				return errors.New("replacement is not an object")
			}
			if err := validateIntentArchiveJSONObject(decoder, intentArchiveReplacementFields[:]); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unterminated replacements array")
		}
	default:
		return errors.New("unsupported schema node")
	}
	return nil
}

func intentArchiveJSONFieldIndex(schema []intentArchiveJSONField, key string) (int, bool) {
	for index, field := range schema {
		if field.name == key {
			return index, false
		}
		if strings.EqualFold(field.name, key) {
			return index, true
		}
	}
	return -1, false
}

func requireIntentArchiveJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("trailing JSON value")
	}
	return err
}

func isIntentArchiveUnsignedInteger(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validateIntentArchiveWireState(replacement IntentArchiveReplacement) error {
	retained := replacement.Blob == replacement.ContentSHA256 && !replacement.Purged && !replacement.PurgePending
	pending := replacement.Blob == replacement.ContentSHA256 && !replacement.Purged && replacement.PurgePending
	tombstoned := replacement.Blob == "" && replacement.Purged && !replacement.PurgePending
	if retained || pending || tombstoned {
		return nil
	}
	err := intentArchiveError(IntentArchiveCodeIndexCorrupt, "a replacement has an invalid archive wire state", 3)
	err.ArtifactID = replacement.ArtifactID
	return err
}

func validIntentArchiveSlug(slug string) bool {
	if len(slug) == 0 || len(slug) > 60 || !fs.ValidPath(slug) || strings.Contains(slug, "/") {
		return false
	}
	segmentStart := true
	for _, character := range []byte(slug) {
		if character == '-' {
			if segmentStart {
				return false
			}
			segmentStart = true
			continue
		}
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')) {
			return false
		}
		segmentStart = false
	}
	if segmentStart {
		return false
	}
	switch strings.ToUpper(slug) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return false
	default:
		return true
	}
}

func validIntentArchiveHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validIntentArchiveFeatureRelativePath(value string) bool {
	return value != "" && value != "." && fs.ValidPath(value) && !strings.HasPrefix(value, "../")
}

type IntentArchiveWireState string

const (
	IntentArchiveWireRetained       IntentArchiveWireState = "retained"
	IntentArchiveWireRemovalPending IntentArchiveWireState = "removal-pending"
	IntentArchiveWireTombstoned     IntentArchiveWireState = "tombstoned"
)

func (replacement IntentArchiveReplacement) WireState() IntentArchiveWireState {
	switch {
	case replacement.Blob == replacement.ContentSHA256 && !replacement.Purged && replacement.PurgePending:
		return IntentArchiveWireRemovalPending
	case replacement.Blob == "" && replacement.Purged && !replacement.PurgePending:
		return IntentArchiveWireTombstoned
	default:
		return IntentArchiveWireRetained
	}
}

type IntentArchiveBlobState string

const (
	IntentArchiveBlobAbsent         IntentArchiveBlobState = "absent"
	IntentArchiveBlobPresentCorrect IntentArchiveBlobState = "present-regular-hash-correct"
	IntentArchiveBlobUnidentifiable IntentArchiveBlobState = "present-unidentifiable"
)

type IntentArchiveDisposition string

const (
	IntentArchiveDispositionHealthyRetained   IntentArchiveDisposition = "healthy-retained"
	IntentArchiveDispositionHealthyPurged     IntentArchiveDisposition = "healthy-purged"
	IntentArchiveDispositionPendingRemove     IntentArchiveDisposition = "pending-remove"
	IntentArchiveDispositionPendingFinalize   IntentArchiveDisposition = "pending-finalize"
	IntentArchiveDispositionDanglingReference IntentArchiveDisposition = "dangling-reference"
	IntentArchiveDispositionResidue           IntentArchiveDisposition = "unreferenced-residue"
	IntentArchiveDispositionMixedReference    IntentArchiveDisposition = "mixed-reference"
	IntentArchiveDispositionCorruptObject     IntentArchiveDisposition = "corrupt-object"
)

type IntentArchiveAction string

const (
	IntentArchiveActionNone                IntentArchiveAction = "none"
	IntentArchiveActionRoutePendingOwner   IntentArchiveAction = "route-pending-owner"
	IntentArchiveActionPurgeHash           IntentArchiveAction = "purge-hash"
	IntentArchiveActionPurgeOrphans        IntentArchiveAction = "purge-orphans"
	IntentArchiveActionRemoveCorruptObject IntentArchiveAction = "remove-corrupt-object"
)

type IntentArchiveRepairClass string

const (
	IntentArchiveRepairCorruptObject       IntentArchiveRepairClass = "corrupt-object"
	IntentArchiveRepairDanglingReference   IntentArchiveRepairClass = "dangling-reference"
	IntentArchiveRepairMixedReference      IntentArchiveRepairClass = "mixed-reference"
	IntentArchiveRepairUnreferencedResidue IntentArchiveRepairClass = "unreferenced-residue"
)

var intentArchiveRepairClassOrder = [...]IntentArchiveRepairClass{
	IntentArchiveRepairCorruptObject,
	IntentArchiveRepairDanglingReference,
	IntentArchiveRepairMixedReference,
	IntentArchiveRepairUnreferencedResidue,
}

type IntentArchiveTuple struct {
	WireState IntentArchiveWireState
	BlobState IntentArchiveBlobState
	Owned     bool
	Live      bool
}

type IntentArchiveTupleResult struct {
	Reachable   bool
	Disposition IntentArchiveDisposition
	Action      IntentArchiveAction
	Code        IntentArchiveErrorCode
	RepairClass IntentArchiveRepairClass
	ExitClass   int
}

func IntentArchiveTupleReachable(tuple IntentArchiveTuple) bool {
	switch tuple.BlobState {
	case IntentArchiveBlobAbsent, IntentArchiveBlobPresentCorrect, IntentArchiveBlobUnidentifiable:
	default:
		return false
	}
	if tuple.Owned && !tuple.Live {
		return false
	}
	switch tuple.WireState {
	case IntentArchiveWireRetained:
		return tuple.Live
	case IntentArchiveWireRemovalPending:
		return tuple.Owned && tuple.Live
	case IntentArchiveWireTombstoned:
		return !tuple.Owned || tuple.Live
	default:
		return false
	}
}

func ClassifyIntentArchiveTuple(tuple IntentArchiveTuple) IntentArchiveTupleResult {
	if !IntentArchiveTupleReachable(tuple) {
		return IntentArchiveTupleResult{}
	}
	result := IntentArchiveTupleResult{Reachable: true}
	if tuple.Owned {
		switch tuple.BlobState {
		case IntentArchiveBlobPresentCorrect, IntentArchiveBlobUnidentifiable:
			result.Disposition = IntentArchiveDispositionPendingRemove
			result.Action = IntentArchiveActionRoutePendingOwner
		case IntentArchiveBlobAbsent:
			result.Disposition = IntentArchiveDispositionPendingFinalize
			result.Action = IntentArchiveActionRoutePendingOwner
		}
		return result
	}
	if tuple.BlobState == IntentArchiveBlobUnidentifiable {
		result.Disposition = IntentArchiveDispositionCorruptObject
		result.Action = IntentArchiveActionRemoveCorruptObject
		result.Code = IntentArchiveCodeBlobCorrupt
		result.RepairClass = IntentArchiveRepairCorruptObject
		result.ExitClass = 3
		return result
	}
	switch tuple.WireState {
	case IntentArchiveWireRetained:
		if tuple.BlobState == IntentArchiveBlobAbsent {
			result.Disposition = IntentArchiveDispositionDanglingReference
			result.Action = IntentArchiveActionPurgeHash
			result.Code = IntentArchiveCodeBlobDangling
			result.RepairClass = IntentArchiveRepairDanglingReference
			result.ExitClass = 3
		} else {
			result.Disposition = IntentArchiveDispositionHealthyRetained
			result.Action = IntentArchiveActionNone
		}
	case IntentArchiveWireRemovalPending:
		result.Disposition = IntentArchiveDispositionPendingFinalize
		result.Action = IntentArchiveActionRoutePendingOwner
	case IntentArchiveWireTombstoned:
		switch tuple.BlobState {
		case IntentArchiveBlobAbsent:
			if tuple.Live {
				result.Disposition = IntentArchiveDispositionDanglingReference
				result.Action = IntentArchiveActionPurgeHash
				result.Code = IntentArchiveCodeBlobDangling
				result.RepairClass = IntentArchiveRepairDanglingReference
				result.ExitClass = 3
			} else {
				result.Disposition = IntentArchiveDispositionHealthyPurged
				result.Action = IntentArchiveActionNone
			}
		case IntentArchiveBlobPresentCorrect:
			if tuple.Live {
				result.Disposition = IntentArchiveDispositionMixedReference
				result.Action = IntentArchiveActionPurgeHash
				result.Code = IntentArchiveCodeIndexStorageInconsistent
				result.RepairClass = IntentArchiveRepairMixedReference
				result.ExitClass = 3
			} else {
				result.Disposition = IntentArchiveDispositionResidue
				result.Action = IntentArchiveActionPurgeOrphans
				result.Code = IntentArchiveCodeIndexStorageInconsistent
				result.RepairClass = IntentArchiveRepairUnreferencedResidue
				result.ExitClass = 3
			}
		}
	}
	return result
}

type IntentArchiveIdentityToken string

type IntentArchiveBlobObservation struct {
	Hash      string
	Path      string
	State     IntentArchiveBlobState
	SizeBytes int64
	Identity  IntentArchiveIdentityToken
}

type IntentArchiveReferenceReport struct {
	GenerationID string
	ArtifactID   IntentArchiveArtifactID
	Path         string
	Hash         string
	SizeBytes    int64
	WireState    IntentArchiveWireState
	Disposition  IntentArchiveDisposition
	Action       IntentArchiveAction
	Code         IntentArchiveErrorCode
	RepairClass  IntentArchiveRepairClass
}

type IntentArchiveHashReport struct {
	Hash         string
	Owned        bool
	Live         bool
	Unreferenced bool
	Blob         IntentArchiveBlobObservation
	References   []IntentArchiveReferenceReport
	RepairClass  IntentArchiveRepairClass
}

type IntentArchiveOrphan struct {
	Hash      string
	Path      string
	SizeBytes int64
}

type IntentArchiveRepairInstance struct {
	Hash             string
	Path             string
	GenerationIDs    []string
	ResultingClasses []IntentArchiveRepairClass
}

type IntentArchiveRepairClassReport struct {
	Rank      int
	Class     IntentArchiveRepairClass
	Blocking  bool
	Hashes    []string
	Paths     []string
	Instances []IntentArchiveRepairInstance
}

type IntentArchiveInspection struct {
	Hashes        []IntentArchiveHashReport
	References    []IntentArchiveReferenceReport
	Orphans       []IntentArchiveOrphan
	PendingHashes []string
	Classes       []IntentArchiveRepairClassReport
	Consistent    bool
}

func InspectIntentArchive(index IntentArchiveIndex, blobObservations []IntentArchiveBlobObservation) (IntentArchiveInspection, error) {
	report := IntentArchiveInspection{
		Hashes:        []IntentArchiveHashReport{},
		References:    []IntentArchiveReferenceReport{},
		Orphans:       []IntentArchiveOrphan{},
		PendingHashes: []string{},
		Classes:       []IntentArchiveRepairClassReport{},
		Consistent:    true,
	}
	if err := ValidateIntentArchiveIndex(index, index.Feature); err != nil {
		return report, err
	}
	blobsDir, _ := IntentArchiveBlobsRel(index.Feature)
	type indexedReference struct {
		generationID string
		replacement  IntentArchiveReplacement
	}
	refsByHash := make(map[string][]indexedReference)
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			refsByHash[replacement.ContentSHA256] = append(refsByHash[replacement.ContentSHA256], indexedReference{
				generationID: generation.GenerationID,
				replacement:  replacement,
			})
		}
	}
	observedByHash := make(map[string]IntentArchiveBlobObservation)
	invalidObjects := make([]IntentArchiveBlobObservation, 0)
	seenPaths := make(map[string]struct{}, len(blobObservations))
	for _, observation := range blobObservations {
		if !validIntentArchiveRootRelativePath(observation.Path) || path.Dir(observation.Path) != blobsDir {
			return report, intentArchiveError(IntentArchiveCodeIndexPathEscape, "a blob observation escaped the archive blob directory", 3)
		}
		if _, duplicate := seenPaths[observation.Path]; duplicate {
			return report, intentArchiveError(IntentArchiveCodeBlobCorrupt, "the blob directory observation repeated a path", 3)
		}
		seenPaths[observation.Path] = struct{}{}
		switch observation.State {
		case IntentArchiveBlobAbsent:
			if !validIntentArchiveHash(observation.Hash) {
				return report, intentArchiveError(IntentArchiveCodeBlobCorrupt, "an absent blob observation has no valid hash", 3)
			}
		case IntentArchiveBlobPresentCorrect:
			if !validIntentArchiveHash(observation.Hash) || observation.SizeBytes < 0 {
				return report, intentArchiveError(IntentArchiveCodeBlobCorrupt, "a present blob observation has an invalid identity", 3)
			}
		case IntentArchiveBlobUnidentifiable:
			if observation.Hash != "" && !validIntentArchiveHash(observation.Hash) {
				return report, intentArchiveError(IntentArchiveCodeBlobCorrupt, "an unidentifiable blob observation has an invalid hash", 3)
			}
		default:
			return report, intentArchiveError(IntentArchiveCodeBlobCorrupt, "a blob observation has an unknown state", 3)
		}
		if observation.Hash == "" {
			invalidObjects = append(invalidObjects, observation)
			continue
		}
		wantPath, _ := IntentArchiveBlobRel(index.Feature, observation.Hash)
		if observation.Path != wantPath {
			return report, intentArchiveError(IntentArchiveCodeIndexPathEscape, "a blob observation does not use its canonical managed path", 3)
		}
		if _, duplicate := observedByHash[observation.Hash]; duplicate {
			return report, intentArchiveError(IntentArchiveCodeBlobCorrupt, "the blob directory observation repeated a hash", 3)
		}
		observedByHash[observation.Hash] = observation
	}

	hashSet := make(map[string]struct{}, len(refsByHash)+len(observedByHash))
	for hash := range refsByHash {
		hashSet[hash] = struct{}{}
	}
	for hash := range observedByHash {
		hashSet[hash] = struct{}{}
	}
	hashes := make([]string, 0, len(hashSet))
	for hash := range hashSet {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)

	classInstances := make(map[IntentArchiveRepairClass][]IntentArchiveRepairInstance)
	for _, hash := range hashes {
		references := append([]indexedReference(nil), refsByHash[hash]...)
		sort.SliceStable(references, func(i, j int) bool {
			if references[i].generationID != references[j].generationID {
				return references[i].generationID < references[j].generationID
			}
			return references[i].replacement.ArtifactID < references[j].replacement.ArtifactID
		})
		owned := false
		live := false
		hasRetained := false
		hasTombstone := false
		expectedSize := int64(-1)
		for _, reference := range references {
			if expectedSize < 0 {
				expectedSize = reference.replacement.SizeBytes
			}
			switch reference.replacement.WireState() {
			case IntentArchiveWireRetained:
				live = true
				hasRetained = true
			case IntentArchiveWireRemovalPending:
				owned = true
				live = true
			case IntentArchiveWireTombstoned:
				hasTombstone = true
			}
		}
		observation, exists := observedByHash[hash]
		if !exists {
			blobRel, _ := IntentArchiveBlobRel(index.Feature, hash)
			observation = IntentArchiveBlobObservation{
				Hash:  hash,
				Path:  blobRel,
				State: IntentArchiveBlobAbsent,
			}
		}
		if observation.State == IntentArchiveBlobPresentCorrect && expectedSize >= 0 && observation.SizeBytes != expectedSize {
			observation.State = IntentArchiveBlobUnidentifiable
		}
		hashReport := IntentArchiveHashReport{
			Hash:         hash,
			Owned:        owned,
			Live:         live,
			Unreferenced: !live,
			Blob:         observation,
			References:   []IntentArchiveReferenceReport{},
		}
		for _, reference := range references {
			tuple := IntentArchiveTuple{
				WireState: reference.replacement.WireState(),
				BlobState: observation.State,
				Owned:     owned,
				Live:      live,
			}
			classification := ClassifyIntentArchiveTuple(tuple)
			if !classification.Reachable {
				return report, intentArchiveError(IntentArchiveCodeIndexCorrupt, "a validated reference produced an unreachable storage tuple", 3)
			}
			item := IntentArchiveReferenceReport{
				GenerationID: reference.generationID,
				ArtifactID:   reference.replacement.ArtifactID,
				Path:         reference.replacement.Path,
				Hash:         hash,
				SizeBytes:    reference.replacement.SizeBytes,
				WireState:    reference.replacement.WireState(),
				Disposition:  classification.Disposition,
				Action:       classification.Action,
				Code:         classification.Code,
				RepairClass:  classification.RepairClass,
			}
			hashReport.References = append(hashReport.References, item)
			report.References = append(report.References, item)
		}

		switch {
		case owned:
			report.PendingHashes = append(report.PendingHashes, hash)
		case observation.State == IntentArchiveBlobUnidentifiable:
			hashReport.RepairClass = IntentArchiveRepairCorruptObject
		case observation.State == IntentArchiveBlobAbsent && hasRetained:
			hashReport.RepairClass = IntentArchiveRepairDanglingReference
		case observation.State == IntentArchiveBlobPresentCorrect && hasRetained && hasTombstone:
			hashReport.RepairClass = IntentArchiveRepairMixedReference
		case observation.State == IntentArchiveBlobPresentCorrect && !live:
			hashReport.RepairClass = IntentArchiveRepairUnreferencedResidue
		}
		if hashReport.RepairClass != "" {
			generationIDs := make([]string, 0, len(references))
			for _, reference := range references {
				generationIDs = append(generationIDs, reference.generationID)
			}
			instance := IntentArchiveRepairInstance{
				Hash:             hash,
				Path:             observation.Path,
				GenerationIDs:    sortedUniqueStrings(generationIDs),
				ResultingClasses: []IntentArchiveRepairClass{},
			}
			if hashReport.RepairClass == IntentArchiveRepairCorruptObject && hasRetained {
				instance.ResultingClasses = []IntentArchiveRepairClass{IntentArchiveRepairDanglingReference}
			}
			classInstances[hashReport.RepairClass] = append(classInstances[hashReport.RepairClass], instance)
		}
		if hashReport.RepairClass == IntentArchiveRepairUnreferencedResidue {
			report.Orphans = append(report.Orphans, IntentArchiveOrphan{
				Hash:      hash,
				Path:      observation.Path,
				SizeBytes: observation.SizeBytes,
			})
		}
		report.Hashes = append(report.Hashes, hashReport)
	}

	sort.SliceStable(invalidObjects, func(i, j int) bool {
		return invalidObjects[i].Path < invalidObjects[j].Path
	})
	for _, object := range invalidObjects {
		classInstances[IntentArchiveRepairCorruptObject] = append(classInstances[IntentArchiveRepairCorruptObject], IntentArchiveRepairInstance{
			Path:             object.Path,
			GenerationIDs:    []string{},
			ResultingClasses: []IntentArchiveRepairClass{},
		})
	}
	for rank, class := range intentArchiveRepairClassOrder {
		instances := classInstances[class]
		if len(instances) == 0 {
			continue
		}
		sort.SliceStable(instances, func(i, j int) bool {
			if instances[i].Hash != instances[j].Hash {
				return instances[i].Hash < instances[j].Hash
			}
			return instances[i].Path < instances[j].Path
		})
		classReport := IntentArchiveRepairClassReport{
			Rank:      rank + 1,
			Class:     class,
			Blocking:  class == IntentArchiveRepairCorruptObject,
			Hashes:    []string{},
			Paths:     []string{},
			Instances: instances,
		}
		for _, instance := range instances {
			if instance.Hash != "" {
				classReport.Hashes = append(classReport.Hashes, instance.Hash)
			}
			classReport.Paths = append(classReport.Paths, instance.Path)
		}
		classReport.Hashes = sortedUniqueStrings(classReport.Hashes)
		classReport.Paths = sortedUniqueStrings(classReport.Paths)
		report.Classes = append(report.Classes, classReport)
	}
	report.Orphans = sortedIntentArchiveOrphans(report.Orphans)
	report.PendingHashes = sortedUniqueStrings(report.PendingHashes)
	report.Consistent = len(report.Classes) == 0 && len(report.PendingHashes) == 0
	return report, nil
}

func sortedIntentArchiveOrphans(orphans []IntentArchiveOrphan) []IntentArchiveOrphan {
	if orphans == nil {
		return []IntentArchiveOrphan{}
	}
	sort.SliceStable(orphans, func(i, j int) bool {
		if orphans[i].Hash != orphans[j].Hash {
			return orphans[i].Hash < orphans[j].Hash
		}
		return orphans[i].Path < orphans[j].Path
	})
	return orphans
}

func sortedUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	out := sorted[:0]
	for _, value := range sorted {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}

func validIntentArchiveRootRelativePath(value string) bool {
	return value != "" && value != "." && fs.ValidPath(value)
}

type IntentArchiveIndexCapture struct {
	Exists   bool
	Raw      []byte
	Identity IntentArchiveIdentityToken
}

func (capture IntentArchiveIndexCapture) Equal(other IntentArchiveIndexCapture) bool {
	return capture.Exists == other.Exists &&
		capture.Identity == other.Identity &&
		bytes.Equal(capture.Raw, other.Raw)
}

type IntentArchiveBlobKind string

const (
	IntentArchiveBlobKindAbsent          IntentArchiveBlobKind = "absent"
	IntentArchiveBlobKindRegular         IntentArchiveBlobKind = "regular"
	IntentArchiveBlobKindSymlink         IntentArchiveBlobKind = "symlink"
	IntentArchiveBlobKindDirectory       IntentArchiveBlobKind = "directory"
	IntentArchiveBlobKindFIFO            IntentArchiveBlobKind = "fifo"
	IntentArchiveBlobKindDevice          IntentArchiveBlobKind = "device"
	IntentArchiveBlobKindOtherNonRegular IntentArchiveBlobKind = "other-non-regular"
	IntentArchiveBlobKindNonRegular      IntentArchiveBlobKind = IntentArchiveBlobKindOtherNonRegular
)

type IntentArchiveBlobProbe struct {
	Kind      IntentArchiveBlobKind
	SHA256    string
	SizeBytes int64
	Identity  IntentArchiveIdentityToken
}

func intentArchiveBlobKindUnidentifiable(kind IntentArchiveBlobKind) bool {
	switch kind {
	case IntentArchiveBlobKindSymlink,
		IntentArchiveBlobKindDirectory,
		IntentArchiveBlobKindFIFO,
		IntentArchiveBlobKindDevice,
		IntentArchiveBlobKindOtherNonRegular:
		return true
	default:
		return false
	}
}

type IntentArchiveStoragePhase string

const (
	IntentArchiveStoragePhaseNone            IntentArchiveStoragePhase = ""
	IntentArchiveStoragePhaseValidated       IntentArchiveStoragePhase = "validated"
	IntentArchiveStoragePhaseWritten         IntentArchiveStoragePhase = "written"
	IntentArchiveStoragePhaseSynced          IntentArchiveStoragePhase = "synced"
	IntentArchiveStoragePhaseRenamed         IntentArchiveStoragePhase = "renamed"
	IntentArchiveStoragePhaseRemoved         IntentArchiveStoragePhase = "removed"
	IntentArchiveStoragePhaseDirectorySynced IntentArchiveStoragePhase = "directory-synced"
)

type IntentArchiveMutationResult struct {
	Committed bool
	Reused    bool
	Phase     IntentArchiveStoragePhase
}

type IntentArchiveStorage interface {
	CaptureIndex(indexRel string) (IntentArchiveIndexCapture, error)
	EnumerateBlobs(blobsRel string) ([]string, error)
	ProbeBlob(blobRel string) (IntentArchiveBlobProbe, error)
	PreflightIndexCAS(indexRel string, expected IntentArchiveIdentityToken) error
	PreflightBlobRemove(blobRel string, expected IntentArchiveIdentityToken) error
	PublishBlob(blobRel, contentSHA256 string, data []byte) (IntentArchiveMutationResult, error)
	CASIndex(indexRel string, expected IntentArchiveIdentityToken, canonical []byte) (IntentArchiveMutationResult, error)
	RemoveBlob(blobRel string, expected IntentArchiveIdentityToken) (IntentArchiveMutationResult, error)
	SyncDirectory(dirRel string) error
}

type IntentArchiveSnapshot struct {
	Feature          string
	IndexCapture     IntentArchiveIndexCapture
	Index            IntentArchiveIndex
	BlobObservations []IntentArchiveBlobObservation
	Inspection       IntentArchiveInspection
}

func CaptureIntentArchive(storage IntentArchiveStorage, feature string) (IntentArchiveSnapshot, error) {
	snapshot := IntentArchiveSnapshot{
		Feature:          feature,
		BlobObservations: []IntentArchiveBlobObservation{},
	}
	if storage == nil {
		return snapshot, intentArchiveError(IntentArchiveCodeStorageFailed, "archive storage is unavailable", 3)
	}
	indexRel, err := IntentArchiveIndexRel(feature)
	if err != nil {
		return snapshot, err
	}
	capture, err := storage.CaptureIndex(indexRel)
	if err != nil {
		return snapshot, intentArchiveStorageError(err, "capture-index", false, 3)
	}
	capture.Raw = append([]byte(nil), capture.Raw...)
	snapshot.IndexCapture = capture
	if capture.Exists {
		index, decodeErr := DecodeIntentArchiveIndex(capture.Raw, feature)
		if decodeErr != nil {
			return snapshot, decodeErr
		}
		snapshot.Index = index
	} else {
		if len(capture.Raw) != 0 {
			return snapshot, intentArchiveError(IntentArchiveCodeIndexCorrupt, "an absent index capture carried raw bytes", 3)
		}
		index, newErr := NewIntentArchiveIndex(feature)
		if newErr != nil {
			return snapshot, newErr
		}
		snapshot.Index = index
	}
	observations, err := ObserveIntentArchiveBlobs(storage, snapshot.Index)
	if err != nil {
		return snapshot, err
	}
	snapshot.BlobObservations = observations
	inspection, err := InspectIntentArchive(snapshot.Index, observations)
	if err != nil {
		return snapshot, err
	}
	snapshot.Inspection = inspection
	return snapshot, nil
}

func ObserveIntentArchiveBlobs(storage IntentArchiveStorage, index IntentArchiveIndex) ([]IntentArchiveBlobObservation, error) {
	if storage == nil {
		return nil, intentArchiveError(IntentArchiveCodeStorageFailed, "archive storage is unavailable", 3)
	}
	if err := ValidateIntentArchiveIndex(index, index.Feature); err != nil {
		return nil, err
	}
	blobsRel, _ := IntentArchiveBlobsRel(index.Feature)
	entries, err := storage.EnumerateBlobs(blobsRel)
	if err != nil {
		return nil, intentArchiveStorageError(err, "enumerate-blobs", false, 3)
	}
	entrySet := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if !validIntentArchiveRootRelativePath(entry) || path.Dir(entry) != blobsRel {
			return nil, intentArchiveError(IntentArchiveCodeIndexPathEscape, "blob enumeration returned a path outside the managed directory", 3)
		}
		if _, duplicate := entrySet[entry]; duplicate {
			return nil, intentArchiveError(IntentArchiveCodeBlobCorrupt, "blob enumeration returned a duplicate managed path", 3)
		}
		entrySet[entry] = struct{}{}
	}
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			blobRel, _ := IntentArchiveBlobRel(index.Feature, replacement.ContentSHA256)
			entrySet[blobRel] = struct{}{}
		}
	}
	paths := make([]string, 0, len(entrySet))
	for entry := range entrySet {
		paths = append(paths, entry)
	}
	sort.Strings(paths)
	observations := make([]IntentArchiveBlobObservation, 0, len(paths))
	for _, blobRel := range paths {
		hash, validName := intentArchiveHashFromBlobPath(blobsRel, blobRel)
		probe, probeErr := storage.ProbeBlob(blobRel)
		if probeErr != nil {
			return nil, intentArchiveStorageError(probeErr, "probe-blob", false, 3)
		}
		observation := IntentArchiveBlobObservation{
			Hash:      hash,
			Path:      blobRel,
			SizeBytes: probe.SizeBytes,
			Identity:  probe.Identity,
		}
		switch probe.Kind {
		case IntentArchiveBlobKindAbsent:
			if !validName {
				continue
			}
			observation.State = IntentArchiveBlobAbsent
		case IntentArchiveBlobKindRegular:
			if validName && probe.SHA256 == hash && probe.SizeBytes >= 0 {
				observation.State = IntentArchiveBlobPresentCorrect
			} else {
				observation.State = IntentArchiveBlobUnidentifiable
				if !validName {
					observation.Hash = ""
				}
			}
		case IntentArchiveBlobKindSymlink,
			IntentArchiveBlobKindDirectory,
			IntentArchiveBlobKindFIFO,
			IntentArchiveBlobKindDevice,
			IntentArchiveBlobKindOtherNonRegular:
			observation.State = IntentArchiveBlobUnidentifiable
			if !validName {
				observation.Hash = ""
			}
		default:
			return nil, intentArchiveError(IntentArchiveCodeBlobCorrupt, "blob probing returned an unknown object kind", 3)
		}
		observations = append(observations, observation)
	}
	sort.SliceStable(observations, func(i, j int) bool {
		if observations[i].Hash != observations[j].Hash {
			return observations[i].Hash < observations[j].Hash
		}
		return observations[i].Path < observations[j].Path
	})
	return observations, nil
}

func intentArchiveHashFromBlobPath(blobsRel, blobRel string) (string, bool) {
	if path.Dir(blobRel) != blobsRel {
		return "", false
	}
	base := path.Base(blobRel)
	if len(base) != 69 || !strings.HasSuffix(base, ".blob") {
		return "", false
	}
	hash := strings.TrimSuffix(base, ".blob")
	return hash, validIntentArchiveHash(hash)
}

func intentArchiveStorageError(err error, class string, committed bool, exitClass int) *IntentArchiveError {
	var typed *IntentArchiveError
	if errors.As(err, &typed) {
		copy := *typed
		copy.Committed = copy.Committed || committed
		if copy.ExitClass == 0 {
			copy.ExitClass = exitClass
		}
		return &copy
	}
	return &IntentArchiveError{
		Code:      IntentArchiveCodeStorageFailed,
		Class:     class,
		Detail:    "the archive storage operation failed",
		ExitClass: exitClass,
		Committed: committed,
	}
}

type IntentArchiveReplacementInput struct {
	ArtifactID IntentArchiveArtifactID
	Path       string
	PriorBytes []byte
}

type IntentArchiveSensitiveFinding struct {
	ArtifactID IntentArchiveArtifactID
	Classes    []string
}

type IntentArchiveAppendOutcome string

const (
	IntentArchiveAppendNew       IntentArchiveAppendOutcome = "append"
	IntentArchiveAppendNoOp      IntentArchiveAppendOutcome = "no-op"
	IntentArchiveAppendRehydrate IntentArchiveAppendOutcome = "rehydrate"
)

type IntentArchiveBlobCandidate struct {
	Hash        string
	Rel         string
	SizeBytes   int64
	ArtifactIDs []IntentArchiveArtifactID
	Data        []byte
}

type IntentArchivePreimageReference struct {
	ArtifactID    IntentArchiveArtifactID
	ContentSHA256 string
	BlobRel       string
}

type IntentArchiveAppendPlan struct {
	feature       string
	outcome       IntentArchiveAppendOutcome
	generationID  string
	indexPreimage IntentArchiveIndexCapture
	blobs         []IntentArchiveBlobCandidate
	indexBytes    []byte
	preimages     []IntentArchivePreimageReference
	inputs        []IntentArchiveReplacementInput
	seal          [sha256.Size]byte
}

func (plan IntentArchiveAppendPlan) PreimageFor(artifactID IntentArchiveArtifactID) (IntentArchivePreimageReference, bool) {
	for _, reference := range plan.preimages {
		if reference.ArtifactID == artifactID {
			return reference, true
		}
	}
	return IntentArchivePreimageReference{}, false
}

func (plan IntentArchiveAppendPlan) Feature() string {
	return plan.feature
}

func (plan IntentArchiveAppendPlan) Outcome() IntentArchiveAppendOutcome {
	return plan.outcome
}

func (plan IntentArchiveAppendPlan) GenerationID() string {
	return plan.generationID
}

func (plan IntentArchiveAppendPlan) IndexPreimage() IntentArchiveIndexCapture {
	return cloneIntentArchiveIndexCapture(plan.indexPreimage)
}

func (plan IntentArchiveAppendPlan) Blobs() []IntentArchiveBlobCandidate {
	return cloneIntentArchiveBlobCandidates(plan.blobs)
}

func (plan IntentArchiveAppendPlan) IndexBytes() []byte {
	return append([]byte(nil), plan.indexBytes...)
}

func (plan IntentArchiveAppendPlan) Preimages() []IntentArchivePreimageReference {
	return append([]IntentArchivePreimageReference(nil), plan.preimages...)
}

type IntentArchiveBlobPublishResult struct {
	Hash      string
	Rel       string
	Committed bool
	Reused    bool
	Phase     IntentArchiveStoragePhase
}

type IntentArchiveAppendResult struct {
	Outcome         IntentArchiveAppendOutcome
	GenerationID    string
	BlobResults     []IntentArchiveBlobPublishResult
	NewOrphanHashes []string
	Committed       bool
	Phase           IntentArchiveStoragePhase
}

func PlanIntentArchiveAppend(storage IntentArchiveStorage, feature string, replacements []IntentArchiveReplacementInput) (IntentArchiveAppendPlan, error) {
	snapshot, err := CaptureIntentArchive(storage, feature)
	if err != nil {
		return IntentArchiveAppendPlan{}, err
	}
	return BuildIntentArchiveAppendPlan(snapshot, replacements)
}

func BuildIntentArchiveAppendPlan(snapshot IntentArchiveSnapshot, inputs []IntentArchiveReplacementInput) (IntentArchiveAppendPlan, error) {
	plan := IntentArchiveAppendPlan{
		feature:       snapshot.Feature,
		indexPreimage: cloneIntentArchiveIndexCapture(snapshot.IndexCapture),
		blobs:         []IntentArchiveBlobCandidate{},
		preimages:     []IntentArchivePreimageReference{},
		inputs:        cloneIntentArchiveReplacementInputs(inputs),
	}
	if err := ValidateIntentArchiveIndex(snapshot.Index, snapshot.Feature); err != nil {
		return plan, err
	}
	inspection, err := InspectIntentArchive(snapshot.Index, snapshot.BlobObservations)
	if err != nil {
		return plan, err
	}
	snapshot.Inspection = inspection
	if err := validateIntentArchiveAppendInspection(snapshot.Inspection); err != nil {
		return plan, err
	}
	if len(inputs) == 0 {
		return plan, intentArchiveError(IntentArchiveCodeIndexCorrupt, "an archive generation requires at least one replacement", 3)
	}
	findings := scanIntentArchiveReplacementInputs(inputs)
	if len(findings) != 0 {
		err := intentArchiveError(IntentArchiveCodeContentSensitive, formatIntentArchiveSensitiveFindings(findings), 3)
		err.ArtifactID = findings[0].ArtifactID
		err.Class = strings.Join(findings[0].Classes, ",")
		return plan, err
	}
	sortedInputs := append([]IntentArchiveReplacementInput(nil), inputs...)
	sort.SliceStable(sortedInputs, func(i, j int) bool {
		return sortedInputs[i].ArtifactID < sortedInputs[j].ArtifactID
	})
	seenArtifacts := make(map[IntentArchiveArtifactID]struct{}, len(sortedInputs))
	replacements := make([]IntentArchiveReplacement, 0, len(sortedInputs))
	candidateByHash := make(map[string]IntentArchiveBlobCandidate)
	targetHashes := make(map[string]struct{})
	for _, input := range sortedInputs {
		wantPath, err := IntentArchiveArtifactPath(input.ArtifactID)
		if err != nil {
			return plan, err
		}
		if _, duplicate := seenArtifacts[input.ArtifactID]; duplicate {
			return plan, intentArchiveError(IntentArchiveCodeIndexCorrupt, "replacement inputs repeat an artifact id", 3)
		}
		seenArtifacts[input.ArtifactID] = struct{}{}
		if input.Path != wantPath {
			pathErr := intentArchiveError(IntentArchiveCodeIndexPathEscape, "a replacement input is not its canonical feature-relative path", 3)
			pathErr.ArtifactID = input.ArtifactID
			return plan, pathErr
		}
		content := append([]byte(nil), input.PriorBytes...)
		sum := sha256.Sum256(content)
		hash := hex.EncodeToString(sum[:])
		blobRel, _ := IntentArchiveBlobRel(snapshot.Feature, hash)
		replacement := IntentArchiveReplacement{
			ArtifactID:    input.ArtifactID,
			Path:          input.Path,
			ContentSHA256: hash,
			Blob:          hash,
			SizeBytes:     int64(len(content)),
			Purged:        false,
			PurgePending:  false,
		}
		replacements = append(replacements, replacement)
		targetHashes[hash] = struct{}{}
		if existing, duplicate := candidateByHash[hash]; duplicate {
			if !bytes.Equal(existing.Data, content) {
				return plan, intentArchiveError(IntentArchiveCodeGenerationCollision, "distinct prior bytes produced one content hash", 3)
			}
			existing.ArtifactIDs = append(existing.ArtifactIDs, input.ArtifactID)
			sort.SliceStable(existing.ArtifactIDs, func(i, j int) bool {
				return existing.ArtifactIDs[i] < existing.ArtifactIDs[j]
			})
			candidateByHash[hash] = existing
		} else {
			candidateByHash[hash] = IntentArchiveBlobCandidate{
				Hash:        hash,
				Rel:         blobRel,
				SizeBytes:   int64(len(content)),
				ArtifactIDs: []IntentArchiveArtifactID{input.ArtifactID},
				Data:        content,
			}
		}
		plan.preimages = append(plan.preimages, IntentArchivePreimageReference{
			ArtifactID:    input.ArtifactID,
			ContentSHA256: hash,
			BlobRel:       blobRel,
		})
	}
	actualID, canonicalBody, err := ComputeIntentArchiveGenerationID(snapshot.Feature, replacements)
	if err != nil {
		return plan, err
	}
	candidateID := intentArchiveCandidateGenerationID(canonicalBody)
	if !validIntentArchiveHash(candidateID) {
		return plan, intentArchiveError(IntentArchiveCodeGenerationCollision, "the generation id derivation returned an invalid digest", 3)
	}
	plan.generationID = candidateID

	existingIndex := -1
	for index, generation := range snapshot.Index.Generations {
		if generation.GenerationID == candidateID {
			existingIndex = index
			break
		}
	}
	if existingIndex >= 0 {
		stored := snapshot.Index.Generations[existingIndex]
		_, storedBody, bodyErr := ComputeIntentArchiveGenerationID(snapshot.Feature, stored.Replaced)
		if bodyErr != nil || !bytes.Equal(storedBody, canonicalBody) || candidateID != actualID {
			collision := intentArchiveError(IntentArchiveCodeGenerationCollision, "an existing generation id has a distinct immutable body", 3)
			collision.GenerationID = candidateID
			return plan, collision
		}
	}

	next := cloneIntentArchiveIndex(snapshot.Index)
	rehydrated := false
	for generationIndex := range next.Generations {
		for replacementIndex := range next.Generations[generationIndex].Replaced {
			replacement := &next.Generations[generationIndex].Replaced[replacementIndex]
			if _, selected := targetHashes[replacement.ContentSHA256]; !selected {
				continue
			}
			if replacement.WireState() == IntentArchiveWireTombstoned {
				setIntentArchiveReplacementState(replacement, IntentArchiveWireRetained)
				rehydrated = true
			}
		}
	}
	if existingIndex >= 0 {
		if !rehydrated {
			plan.outcome = IntentArchiveAppendNoOp
			indexBytes, encodeErr := EncodeIntentArchiveIndex(snapshot.Index)
			if encodeErr != nil {
				return plan, encodeErr
			}
			plan.indexBytes = indexBytes
			plan.seal = sealIntentArchiveAppendPlan(plan)
			return plan, nil
		}
		plan.outcome = IntentArchiveAppendRehydrate
	} else {
		generation := IntentArchiveGeneration{
			GenerationID: candidateID,
			Mode:         IntentArchiveModeRegenerate,
			Replaced:     replacements,
		}
		next.Generations = append(next.Generations, generation)
		plan.outcome = IntentArchiveAppendNew
	}
	membership := intentArchiveRetainedDiffMembership(snapshot.Index, next)
	for hash, artifactIDs := range membership {
		candidate, exists := candidateByHash[hash]
		if !exists {
			return plan, intentArchiveError(IntentArchiveCodeIndexCorrupt, "the planned index introduces retained bytes without a blob candidate", 3)
		}
		candidate.ArtifactIDs = append([]IntentArchiveArtifactID(nil), artifactIDs...)
		plan.blobs = append(plan.blobs, candidate)
	}
	sort.SliceStable(plan.blobs, func(i, j int) bool {
		return plan.blobs[i].Hash < plan.blobs[j].Hash
	})
	indexBytes, err := EncodeIntentArchiveIndex(next)
	if err != nil {
		return plan, err
	}
	plan.indexBytes = indexBytes
	plan.seal = sealIntentArchiveAppendPlan(plan)
	return plan, nil
}

func scanIntentArchiveReplacementInputs(inputs []IntentArchiveReplacementInput) []IntentArchiveSensitiveFinding {
	findings := make([]IntentArchiveSensitiveFinding, 0)
	for _, input := range inputs {
		classes := redact.Scan(input.PriorBytes)
		if len(classes) == 0 {
			continue
		}
		findings = append(findings, IntentArchiveSensitiveFinding{
			ArtifactID: input.ArtifactID,
			Classes:    append([]string(nil), classes...),
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].ArtifactID < findings[j].ArtifactID
	})
	return findings
}

func formatIntentArchiveSensitiveFindings(findings []IntentArchiveSensitiveFinding) string {
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		classes := append([]string(nil), finding.Classes...)
		sort.Strings(classes)
		parts = append(parts, string(finding.ArtifactID)+":"+strings.Join(classes, ","))
	}
	return "sensitive content classes matched " + strings.Join(parts, ";")
}

func validateIntentArchiveAppendInspection(inspection IntentArchiveInspection) error {
	for _, hash := range inspection.Hashes {
		if hash.Owned {
			err := intentArchiveError(IntentArchiveCodeRecoveryPending, "a purge transaction owns an archive content hash", 3)
			err.Hash = hash.Hash
			return err
		}
	}
	if len(inspection.Classes) == 0 {
		return nil
	}
	first := inspection.Classes[0]
	err := intentArchiveError(intentArchiveCodeForRepairClass(first.Class), "the archive storage observation must be repaired before publication", 3)
	err.Class = string(first.Class)
	if len(first.Hashes) != 0 {
		err.Hash = first.Hashes[0]
	}
	return err
}

func intentArchiveCodeForRepairClass(class IntentArchiveRepairClass) IntentArchiveErrorCode {
	switch class {
	case IntentArchiveRepairCorruptObject:
		return IntentArchiveCodeBlobCorrupt
	case IntentArchiveRepairDanglingReference:
		return IntentArchiveCodeBlobDangling
	default:
		return IntentArchiveCodeIndexStorageInconsistent
	}
}

func PublishIntentArchiveBlobs(storage IntentArchiveStorage, plan IntentArchiveAppendPlan) (IntentArchiveAppendResult, error) {
	result := IntentArchiveAppendResult{
		Outcome:         plan.outcome,
		GenerationID:    plan.generationID,
		BlobResults:     []IntentArchiveBlobPublishResult{},
		NewOrphanHashes: []string{},
	}
	if storage == nil {
		return result, intentArchiveError(IntentArchiveCodeStorageFailed, "archive storage is unavailable", 3)
	}
	if err := validateIntentArchiveAppendPlanSeal(plan); err != nil {
		return result, err
	}
	if !validIntentArchiveSlug(plan.feature) ||
		!validIntentArchiveHash(plan.generationID) ||
		len(plan.indexBytes) == 0 {
		return result, intentArchiveError(IntentArchiveCodeIndexCorrupt, "the archive append plan is invalid", 3)
	}
	if _, err := DecodeIntentArchiveIndex(plan.indexBytes, plan.feature); err != nil {
		return result, err
	}
	for _, candidate := range plan.blobs {
		probe, probeErr := storage.ProbeBlob(candidate.Rel)
		if probeErr != nil {
			return result, intentArchiveStorageError(probeErr, "probe-blob", false, 3)
		}
		if intentArchiveBlobKindUnidentifiable(probe.Kind) ||
			(probe.Kind == IntentArchiveBlobKindRegular &&
				(probe.SHA256 != candidate.Hash || probe.SizeBytes != candidate.SizeBytes)) {
			corrupt := intentArchiveError(IntentArchiveCodeBlobCorrupt, "an existing managed blob is not the immutable candidate content", 3)
			corrupt.Hash = candidate.Hash
			return result, corrupt
		}
	}
	snapshot, err := CaptureIntentArchive(storage, plan.feature)
	if err != nil {
		return result, err
	}
	if !snapshot.IndexCapture.Equal(plan.indexPreimage) {
		return result, intentArchiveError(IntentArchiveCodeIndexChanged, "index.json changed after archive planning", 5)
	}
	if err := validateIntentArchiveAppendPlanAgainstSnapshot(plan, snapshot); err != nil {
		return result, err
	}
	if plan.outcome == IntentArchiveAppendNoOp {
		return result, nil
	}

	blobsDir, _ := IntentArchiveBlobsRel(plan.feature)
	for _, candidate := range plan.blobs {
		if beforeBlobWrite != nil {
			beforeBlobWrite(candidate.Rel)
		}
		mutation, publishErr := storage.PublishBlob(candidate.Rel, candidate.Hash, candidate.Data)
		result.BlobResults = append(result.BlobResults, IntentArchiveBlobPublishResult{
			Hash:      candidate.Hash,
			Rel:       candidate.Rel,
			Committed: mutation.Committed,
			Reused:    mutation.Reused,
			Phase:     mutation.Phase,
		})
		if mutation.Committed {
			result.Committed = true
			result.NewOrphanHashes = append(result.NewOrphanHashes, candidate.Hash)
			result.Phase = mutation.Phase
			if afterBlobWrite != nil {
				afterBlobWrite(candidate.Rel)
			}
		}
		if publishErr != nil {
			result.NewOrphanHashes = sortedUniqueStrings(result.NewOrphanHashes)
			return result, normalizeIntentArchiveAppendFailure(publishErr, "publish-blob", result.Committed)
		}
		if !mutation.Committed && !mutation.Reused {
			result.NewOrphanHashes = sortedUniqueStrings(result.NewOrphanHashes)
			return result, normalizeIntentArchiveAppendFailure(
				errors.New("blob publish returned no terminal truth"),
				"publish-blob",
				result.Committed,
			)
		}
	}
	if result.Committed {
		if err := storage.SyncDirectory(blobsDir); err != nil {
			result.NewOrphanHashes = sortedUniqueStrings(result.NewOrphanHashes)
			return result, normalizeIntentArchiveAppendFailure(err, "sync-blobs-directory", true)
		}
		result.Phase = IntentArchiveStoragePhaseDirectorySynced
	}
	result.NewOrphanHashes = sortedUniqueStrings(result.NewOrphanHashes)
	return result, nil
}

func validateIntentArchiveAppendPlanSeal(plan IntentArchiveAppendPlan) error {
	if plan.seal != sealIntentArchiveAppendPlan(plan) {
		return intentArchiveError(IntentArchiveCodeIndexCorrupt, "the archive append plan changed after construction", 3)
	}
	return nil
}

func validateIntentArchiveAppendPlanAgainstSnapshot(plan IntentArchiveAppendPlan, snapshot IntentArchiveSnapshot) error {
	if !snapshot.IndexCapture.Equal(plan.indexPreimage) {
		return intentArchiveError(IntentArchiveCodeIndexChanged, "index.json changed after archive planning", 5)
	}
	rebuilt, err := BuildIntentArchiveAppendPlan(snapshot, plan.inputs)
	if err != nil {
		return err
	}
	if rebuilt.seal != plan.seal {
		return intentArchiveError(IntentArchiveCodeIndexCorrupt, "the archive append plan does not match a complete rebuild", 3)
	}
	plannedIndex, err := DecodeIntentArchiveIndex(plan.indexBytes, plan.feature)
	if err != nil {
		return err
	}
	return validateIntentArchiveAppendCandidateCoverage(snapshot.Index, plannedIndex, plan.blobs)
}

func validateIntentArchiveAppendCandidateCoverage(before, after IntentArchiveIndex, candidates []IntentArchiveBlobCandidate) error {
	membership := intentArchiveRetainedDiffMembership(before, after)
	if len(membership) != len(candidates) {
		return intentArchiveError(IntentArchiveCodeIndexCorrupt, "the blob candidates do not exactly cover the retained index diff", 3)
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, duplicate := seen[candidate.Hash]; duplicate {
			return intentArchiveError(IntentArchiveCodeIndexCorrupt, "the archive append plan repeats a blob candidate", 3)
		}
		seen[candidate.Hash] = struct{}{}
		wantArtifacts, exists := membership[candidate.Hash]
		if !exists {
			return intentArchiveError(IntentArchiveCodeIndexCorrupt, "the archive append plan carries a blob outside the retained index diff", 3)
		}
		wantRel, relErr := IntentArchiveBlobRel(after.Feature, candidate.Hash)
		sum := sha256.Sum256(candidate.Data)
		if relErr != nil ||
			candidate.Rel != wantRel ||
			candidate.SizeBytes != int64(len(candidate.Data)) ||
			hex.EncodeToString(sum[:]) != candidate.Hash ||
			!equalIntentArchiveArtifactIDSets(candidate.ArtifactIDs, wantArtifacts) {
			return intentArchiveError(IntentArchiveCodeIndexCorrupt, "an archive blob candidate is not canonical for the retained index diff", 3)
		}
		if classes := redact.Scan(candidate.Data); len(classes) != 0 {
			err := intentArchiveError(IntentArchiveCodeContentSensitive, "sensitive content matched during archive execution", 3)
			err.Class = strings.Join(classes, ",")
			if len(candidate.ArtifactIDs) != 0 {
				err.ArtifactID = candidate.ArtifactIDs[0]
			}
			return err
		}
	}
	return nil
}

func intentArchiveRetainedDiffMembership(before, after IntentArchiveIndex) map[string][]IntentArchiveArtifactID {
	type referenceKey struct {
		generationID string
		artifactID   IntentArchiveArtifactID
	}
	beforeStates := make(map[referenceKey]IntentArchiveWireState)
	for _, generation := range before.Generations {
		for _, replacement := range generation.Replaced {
			beforeStates[referenceKey{
				generationID: generation.GenerationID,
				artifactID:   replacement.ArtifactID,
			}] = replacement.WireState()
		}
	}
	membershipSets := make(map[string]map[IntentArchiveArtifactID]struct{})
	for _, generation := range after.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.WireState() != IntentArchiveWireRetained {
				continue
			}
			key := referenceKey{
				generationID: generation.GenerationID,
				artifactID:   replacement.ArtifactID,
			}
			if state, exists := beforeStates[key]; exists && state == IntentArchiveWireRetained {
				continue
			}
			if membershipSets[replacement.ContentSHA256] == nil {
				membershipSets[replacement.ContentSHA256] = make(map[IntentArchiveArtifactID]struct{})
			}
			membershipSets[replacement.ContentSHA256][replacement.ArtifactID] = struct{}{}
		}
	}
	membership := make(map[string][]IntentArchiveArtifactID, len(membershipSets))
	for hash, artifactSet := range membershipSets {
		artifactIDs := make([]IntentArchiveArtifactID, 0, len(artifactSet))
		for artifactID := range artifactSet {
			artifactIDs = append(artifactIDs, artifactID)
		}
		sort.SliceStable(artifactIDs, func(i, j int) bool {
			return artifactIDs[i] < artifactIDs[j]
		})
		membership[hash] = artifactIDs
	}
	return membership
}

func equalIntentArchiveArtifactIDSets(left, right []IntentArchiveArtifactID) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]IntentArchiveArtifactID(nil), left...)
	rightCopy := append([]IntentArchiveArtifactID(nil), right...)
	sort.SliceStable(leftCopy, func(i, j int) bool { return leftCopy[i] < leftCopy[j] })
	sort.SliceStable(rightCopy, func(i, j int) bool { return rightCopy[i] < rightCopy[j] })
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}

func cloneIntentArchiveIndexCapture(capture IntentArchiveIndexCapture) IntentArchiveIndexCapture {
	capture.Raw = append([]byte(nil), capture.Raw...)
	return capture
}

func cloneIntentArchiveReplacementInputs(inputs []IntentArchiveReplacementInput) []IntentArchiveReplacementInput {
	if inputs == nil {
		return []IntentArchiveReplacementInput{}
	}
	out := make([]IntentArchiveReplacementInput, len(inputs))
	for index, input := range inputs {
		out[index] = input
		out[index].PriorBytes = append([]byte(nil), input.PriorBytes...)
	}
	return out
}

func cloneIntentArchiveBlobCandidates(candidates []IntentArchiveBlobCandidate) []IntentArchiveBlobCandidate {
	if candidates == nil {
		return []IntentArchiveBlobCandidate{}
	}
	out := make([]IntentArchiveBlobCandidate, len(candidates))
	for index, candidate := range candidates {
		out[index] = candidate
		out[index].ArtifactIDs = append([]IntentArchiveArtifactID(nil), candidate.ArtifactIDs...)
		out[index].Data = append([]byte(nil), candidate.Data...)
	}
	return out
}

type intentArchiveAppendPlanSealBody struct {
	Feature       string
	Outcome       IntentArchiveAppendOutcome
	GenerationID  string
	IndexPreimage IntentArchiveIndexCapture
	Blobs         []IntentArchiveBlobCandidate
	IndexBytes    []byte
	Preimages     []IntentArchivePreimageReference
	Inputs        []IntentArchiveReplacementInput
}

func sealIntentArchiveAppendPlan(plan IntentArchiveAppendPlan) [sha256.Size]byte {
	body := intentArchiveAppendPlanSealBody{
		Feature:       plan.feature,
		Outcome:       plan.outcome,
		GenerationID:  plan.generationID,
		IndexPreimage: cloneIntentArchiveIndexCapture(plan.indexPreimage),
		Blobs:         cloneIntentArchiveBlobCandidates(plan.blobs),
		IndexBytes:    append([]byte(nil), plan.indexBytes...),
		Preimages:     append([]IntentArchivePreimageReference(nil), plan.preimages...),
		Inputs:        cloneIntentArchiveReplacementInputs(plan.inputs),
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		panic("intent archive append plan seal contains an unsupported value")
	}
	return sha256.Sum256(canonical)
}

func normalizeIntentArchiveAppendFailure(err error, class string, committed bool) *IntentArchiveError {
	typed := intentArchiveStorageError(err, class, committed, intentArchiveExitAfterMutation(committed))
	if !committed {
		return typed
	}
	typed.Committed = true
	if typed.ExitClass >= 6 {
		return typed
	}
	typed.ExitClass = 5
	switch typed.Code {
	case IntentArchiveCodeIndexChanged, IntentArchiveCodePurgeIndexChanged:
		typed.Code = IntentArchiveCodeIndexChanged
	default:
		typed.Code = IntentArchiveCodeRegenerateGenerationFailed
	}
	return typed
}

func intentArchiveExitAfterMutation(committed bool) int {
	if committed {
		return 5
	}
	return 3
}

func cloneIntentArchiveIndex(index IntentArchiveIndex) IntentArchiveIndex {
	out := IntentArchiveIndex{
		SchemaVersion: index.SchemaVersion,
		Feature:       index.Feature,
		Generations:   make([]IntentArchiveGeneration, len(index.Generations)),
	}
	for i, generation := range index.Generations {
		out.Generations[i] = IntentArchiveGeneration{
			GenerationID: generation.GenerationID,
			Mode:         generation.Mode,
			Replaced:     append([]IntentArchiveReplacement(nil), generation.Replaced...),
		}
	}
	return out
}

func setIntentArchiveReplacementState(replacement *IntentArchiveReplacement, state IntentArchiveWireState) {
	switch state {
	case IntentArchiveWireRetained:
		replacement.Blob = replacement.ContentSHA256
		replacement.Purged = false
		replacement.PurgePending = false
	case IntentArchiveWireRemovalPending:
		replacement.Blob = replacement.ContentSHA256
		replacement.Purged = false
		replacement.PurgePending = true
	case IntentArchiveWireTombstoned:
		replacement.Blob = ""
		replacement.Purged = true
		replacement.PurgePending = false
	}
}

type IntentArchiveSelectorKind string

const (
	IntentArchiveSelectorBlob       IntentArchiveSelectorKind = "blob"
	IntentArchiveSelectorGeneration IntentArchiveSelectorKind = "generation"
	IntentArchiveSelectorAll        IntentArchiveSelectorKind = "all"
	IntentArchiveSelectorOrphans    IntentArchiveSelectorKind = "orphans"
)

type IntentArchivePurgeSelector struct {
	Blobs       []string
	Generations []string
	All         bool
	Orphans     bool
}

type IntentArchiveReferenceTarget struct {
	GenerationID string
	ArtifactID   IntentArchiveArtifactID
	Hash         string
	Path         string
	WireState    IntentArchiveWireState
}

type IntentArchiveRepairStageKind string

const (
	IntentArchiveRepairStageManual IntentArchiveRepairStageKind = "manual-prerequisite"
	IntentArchiveRepairStagePurge  IntentArchiveRepairStageKind = "purge-invocation"
)

type IntentArchiveRepairCommand struct {
	Warning  string
	Argv     []string
	Rendered string
}

type IntentArchiveRepairStage struct {
	Ordinal           int
	Class             IntentArchiveRepairClass
	Kind              IntentArchiveRepairStageKind
	Hashes            []string
	Paths             []string
	Repair            string
	RepairCWD         string
	ResultingClasses  []IntentArchiveRepairClass
	Commands          []IntentArchiveRepairCommand
	PredictedHashes   []string
	AfterPrerequisite bool
}

type IntentArchiveNextRepairStage struct {
	Ordinal int
	Kind    IntentArchiveRepairStageKind
	Class   IntentArchiveRepairClass
}

type IntentArchiveRemainingRepairs struct {
	RerunRequired   bool
	RepairedClass   IntentArchiveRepairClass
	StagesRemaining int
	NextStage       *IntentArchiveNextRepairStage
	Stages          []IntentArchiveRepairStage
}

type IntentArchivePurgeOutcome string

const (
	IntentArchivePurgePlanned          IntentArchivePurgeOutcome = "planned"
	IntentArchivePurgeRecoveryRequired IntentArchivePurgeOutcome = "recovery-required"
	IntentArchivePurgePurged           IntentArchivePurgeOutcome = "purged"
	IntentArchivePurgeNoOp             IntentArchivePurgeOutcome = "no-op"
	IntentArchivePurgeRecovered        IntentArchivePurgeOutcome = "recovered"
	IntentArchivePurgePartial          IntentArchivePurgeOutcome = "purge-partial"
)

type IntentArchivePurgePlan struct {
	Feature               string
	Selector              IntentArchivePurgeSelector
	SelectorKind          IntentArchiveSelectorKind
	Preview               bool
	Outcome               IntentArchivePurgeOutcome
	Action                string
	RequiresYes           bool
	RecoveryRequired      bool
	PendingHashes         []string
	IndexPreimage         IntentArchiveIndexCapture
	Hashes                []string
	GenerationIDs         []string
	References            []IntentArchiveReferenceTarget
	BlobRemovals          []IntentArchiveBlobObservation
	StructuralBlastRadius bool
	AdmittedRepairClass   IntentArchiveRepairClass
	ObservedClasses       []IntentArchiveRepairClassReport
	RemainingRepairs      *IntentArchiveRemainingRepairs
	ResultingClasses      []IntentArchiveRepairClassReport
}

type intentArchivePurgeSelection struct {
	hashes        []string
	generations   []string
	references    []IntentArchiveReferenceTarget
	removals      []IntentArchiveBlobObservation
	sharedRefusal *IntentArchiveError
}

func (selector IntentArchivePurgeSelector) normalized(index IntentArchiveIndex) (IntentArchivePurgeSelector, IntentArchiveSelectorKind, error) {
	normalized := IntentArchivePurgeSelector{
		Blobs:       sortedUniqueStrings(selector.Blobs),
		Generations: sortedUniqueStrings(selector.Generations),
		All:         selector.All,
		Orphans:     selector.Orphans,
	}
	kinds := 0
	var kind IntentArchiveSelectorKind
	if len(normalized.Blobs) != 0 {
		kinds++
		kind = IntentArchiveSelectorBlob
	}
	if len(normalized.Generations) != 0 {
		kinds++
		kind = IntentArchiveSelectorGeneration
	}
	if normalized.All {
		kinds++
		kind = IntentArchiveSelectorAll
	}
	if normalized.Orphans {
		kinds++
		kind = IntentArchiveSelectorOrphans
	}
	if kinds != 1 {
		return normalized, "", intentArchiveError(IntentArchiveCodeSelectorInvalid, "exactly one purge selector is required", 1)
	}
	hashSet := intentArchiveIndexHashSet(index)
	generationSet := make(map[string]struct{}, len(index.Generations))
	for _, generation := range index.Generations {
		generationSet[generation.GenerationID] = struct{}{}
	}
	for _, hash := range normalized.Blobs {
		if !validIntentArchiveHash(hash) {
			return normalized, "", intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --blob selector is not a lowercase SHA-256", 3)
		}
		if _, exists := hashSet[hash]; !exists {
			err := intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --blob selector does not name an indexed content hash", 3)
			err.Hash = hash
			return normalized, "", err
		}
	}
	for _, generationID := range normalized.Generations {
		if !validIntentArchiveHash(generationID) {
			return normalized, "", intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --generation selector is not a lowercase SHA-256", 3)
		}
		if _, exists := generationSet[generationID]; !exists {
			err := intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --generation selector does not name an archive generation", 3)
			err.GenerationID = generationID
			return normalized, "", err
		}
	}
	return normalized, kind, nil
}

func PlanIntentArchivePurge(storage IntentArchiveStorage, feature string, selector IntentArchivePurgeSelector, confirmed bool) (IntentArchivePurgePlan, error) {
	snapshot, err := CaptureIntentArchive(storage, feature)
	if err != nil {
		return IntentArchivePurgePlan{}, err
	}
	return BuildIntentArchivePurgePlan(snapshot, selector, confirmed)
}

func PreviewIntentArchivePurge(storage IntentArchiveStorage, feature string, selector IntentArchivePurgeSelector) (IntentArchivePurgePlan, error) {
	return PlanIntentArchivePurge(storage, feature, selector, false)
}

func BuildIntentArchivePurgePlan(snapshot IntentArchiveSnapshot, selector IntentArchivePurgeSelector, confirmed bool) (IntentArchivePurgePlan, error) {
	plan := IntentArchivePurgePlan{
		Feature:          snapshot.Feature,
		Preview:          !confirmed,
		Action:           "none",
		PendingHashes:    []string{},
		Hashes:           []string{},
		GenerationIDs:    []string{},
		References:       []IntentArchiveReferenceTarget{},
		BlobRemovals:     []IntentArchiveBlobObservation{},
		ObservedClasses:  cloneIntentArchiveClassReports(snapshot.Inspection.Classes),
		ResultingClasses: []IntentArchiveRepairClassReport{},
		IndexPreimage:    snapshot.IndexCapture,
	}
	plan.IndexPreimage.Raw = append([]byte(nil), snapshot.IndexCapture.Raw...)
	if err := ValidateIntentArchiveIndex(snapshot.Index, snapshot.Feature); err != nil {
		return plan, err
	}
	inspection, err := InspectIntentArchive(snapshot.Index, snapshot.BlobObservations)
	if err != nil {
		return plan, err
	}
	snapshot.Inspection = inspection
	plan.ObservedClasses = cloneIntentArchiveClassReports(inspection.Classes)
	normalized, kind, err := selector.normalized(snapshot.Index)
	if err != nil {
		return plan, err
	}
	plan.Selector = normalized
	plan.SelectorKind = kind
	plan.StructuralBlastRadius = kind == IntentArchiveSelectorAll
	if len(snapshot.Inspection.PendingHashes) != 0 {
		plan.Outcome = IntentArchivePurgeRecoveryRequired
		plan.RecoveryRequired = true
		plan.RequiresYes = true
		plan.PendingHashes = append([]string(nil), snapshot.Inspection.PendingHashes...)
		plan.RemainingRepairs = buildIntentArchiveRemainingRepairs(snapshot.Feature, snapshot.Inspection.Classes, "")
		return plan, nil
	}

	selection, selectErr := selectIntentArchivePurgeTargets(snapshot, normalized, kind)
	if selectErr != nil {
		return plan, selectErr
	}
	plan.Hashes = selection.hashes
	plan.GenerationIDs = selection.generations
	plan.References = selection.references
	plan.BlobRemovals = selection.removals
	plan.RequiresYes = len(selection.hashes) != 0 || len(selection.removals) != 0
	admitted, admissionErr := admitIntentArchiveRepair(snapshot.Inspection, kind, selection.hashes)
	if admissionErr != nil {
		plan.ResultingClasses = cloneIntentArchiveClassReports(snapshot.Inspection.Classes)
		plan.RemainingRepairs = buildIntentArchiveRemainingRepairs(snapshot.Feature, snapshot.Inspection.Classes, "")
		if !confirmed {
			plan.Outcome = IntentArchivePurgePlanned
			return plan, nil
		}
		return plan, admissionErr
	}
	if selection.sharedRefusal != nil {
		return plan, selection.sharedRefusal
	}
	plan.AdmittedRepairClass = admitted
	plan.ResultingClasses = resultingIntentArchiveClasses(snapshot.Inspection.Classes, admitted)
	if len(plan.ResultingClasses) != 0 {
		plan.RemainingRepairs = buildIntentArchiveRemainingRepairs(snapshot.Feature, plan.ResultingClasses, admitted)
	}
	if !confirmed {
		plan.Outcome = IntentArchivePurgePlanned
		return plan, nil
	}
	if len(selection.hashes) == 0 && len(selection.removals) == 0 {
		plan.Outcome = IntentArchivePurgeNoOp
	} else {
		plan.Outcome = IntentArchivePurgePurged
	}
	return plan, nil
}

func selectIntentArchivePurgeTargets(snapshot IntentArchiveSnapshot, selector IntentArchivePurgeSelector, kind IntentArchiveSelectorKind) (intentArchivePurgeSelection, error) {
	selection := intentArchivePurgeSelection{
		hashes:      []string{},
		generations: []string{},
		references:  []IntentArchiveReferenceTarget{},
		removals:    []IntentArchiveBlobObservation{},
	}
	hashSet := make(map[string]struct{})
	generationSet := make(map[string]struct{})
	switch kind {
	case IntentArchiveSelectorBlob:
		for _, hash := range selector.Blobs {
			hashSet[hash] = struct{}{}
		}
	case IntentArchiveSelectorGeneration:
		for _, generationID := range selector.Generations {
			generationSet[generationID] = struct{}{}
		}
		for _, generation := range snapshot.Index.Generations {
			if _, selected := generationSet[generation.GenerationID]; !selected {
				continue
			}
			for _, replacement := range generation.Replaced {
				hashSet[replacement.ContentSHA256] = struct{}{}
			}
		}
		for _, generation := range snapshot.Index.Generations {
			if _, selected := generationSet[generation.GenerationID]; selected {
				continue
			}
			for _, replacement := range generation.Replaced {
				if _, selectedHash := hashSet[replacement.ContentSHA256]; selectedHash && selection.sharedRefusal == nil {
					err := intentArchiveError(IntentArchiveCodeBlobShared, "a generation selection shares this hash; use --blob for this hash or preview --all", 3)
					err.Hash = replacement.ContentSHA256
					err.GenerationID = generation.GenerationID
					selection.sharedRefusal = err
				}
			}
		}
	case IntentArchiveSelectorAll:
		for _, generation := range snapshot.Index.Generations {
			generationSet[generation.GenerationID] = struct{}{}
			for _, replacement := range generation.Replaced {
				hashSet[replacement.ContentSHA256] = struct{}{}
			}
		}
		for _, observation := range snapshot.BlobObservations {
			if validIntentArchiveHash(observation.Hash) && observation.State == IntentArchiveBlobPresentCorrect {
				hashSet[observation.Hash] = struct{}{}
			}
		}
	case IntentArchiveSelectorOrphans:
		for _, orphan := range snapshot.Inspection.Orphans {
			hashSet[orphan.Hash] = struct{}{}
		}
	default:
		return selection, intentArchiveError(IntentArchiveCodeSelectorInvalid, "the purge selector kind is invalid", 3)
	}
	hashes := make([]string, 0, len(hashSet))
	for hash := range hashSet {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)
	generations := make([]string, 0, len(generationSet))
	for generationID := range generationSet {
		generations = append(generations, generationID)
	}
	sort.Strings(generations)

	references := make([]IntentArchiveReferenceTarget, 0)
	for _, generation := range snapshot.Index.Generations {
		for _, replacement := range generation.Replaced {
			if _, selected := hashSet[replacement.ContentSHA256]; !selected {
				continue
			}
			if kind == IntentArchiveSelectorOrphans {
				continue
			}
			references = append(references, IntentArchiveReferenceTarget{
				GenerationID: generation.GenerationID,
				ArtifactID:   replacement.ArtifactID,
				Hash:         replacement.ContentSHA256,
				Path:         replacement.Path,
				WireState:    replacement.WireState(),
			})
		}
	}
	sort.SliceStable(references, func(i, j int) bool {
		if references[i].Hash != references[j].Hash {
			return references[i].Hash < references[j].Hash
		}
		if references[i].GenerationID != references[j].GenerationID {
			return references[i].GenerationID < references[j].GenerationID
		}
		return references[i].ArtifactID < references[j].ArtifactID
	})
	observed := make(map[string]IntentArchiveBlobObservation, len(snapshot.BlobObservations))
	for _, observation := range snapshot.BlobObservations {
		if observation.Hash != "" {
			observed[observation.Hash] = observation
		}
	}
	removals := make([]IntentArchiveBlobObservation, 0)
	for _, hash := range hashes {
		if observation, exists := observed[hash]; exists && observation.State == IntentArchiveBlobPresentCorrect {
			removals = append(removals, observation)
		}
	}
	sort.SliceStable(removals, func(i, j int) bool {
		return removals[i].Path < removals[j].Path
	})
	selection.hashes = hashes
	selection.generations = generations
	selection.references = references
	selection.removals = removals
	return selection, nil
}

func admitIntentArchiveRepair(inspection IntentArchiveInspection, kind IntentArchiveSelectorKind, selectedHashes []string) (IntentArchiveRepairClass, error) {
	if len(inspection.Classes) == 0 {
		return "", nil
	}
	if inspection.Classes[0].Class == IntentArchiveRepairCorruptObject {
		return "", intentArchiveInspectionError(inspection, "a corrupt managed object blocks every confirmed selector")
	}
	selected := sortedUniqueStrings(selectedHashes)
	var chosen IntentArchiveRepairClass
	switch kind {
	case IntentArchiveSelectorOrphans:
		if class := intentArchiveClassReport(inspection.Classes, IntentArchiveRepairUnreferencedResidue); class != nil &&
			equalStringSets(selected, class.Hashes) {
			chosen = IntentArchiveRepairUnreferencedResidue
		}
	case IntentArchiveSelectorBlob:
		for _, class := range []IntentArchiveRepairClass{
			IntentArchiveRepairDanglingReference,
			IntentArchiveRepairMixedReference,
		} {
			report := intentArchiveClassReport(inspection.Classes, class)
			if report != nil && equalStringSets(selected, report.Hashes) {
				chosen = class
				break
			}
		}
	case IntentArchiveSelectorAll:
		if len(inspection.Classes) == 1 {
			chosen = inspection.Classes[0].Class
		}
	case IntentArchiveSelectorGeneration:
		chosen = ""
	}
	if chosen == "" {
		return "", intentArchiveInspectionError(inspection, "the confirmed selection does not completely and exclusively cover one repair class")
	}
	return chosen, nil
}

func intentArchiveInspectionError(inspection IntentArchiveInspection, detail string) *IntentArchiveError {
	if len(inspection.Classes) == 0 {
		return intentArchiveError(IntentArchiveCodeIndexStorageInconsistent, detail, 3)
	}
	first := inspection.Classes[0]
	err := intentArchiveError(intentArchiveCodeForRepairClass(first.Class), detail, 3)
	err.Class = string(first.Class)
	if len(first.Hashes) != 0 {
		err.Hash = first.Hashes[0]
	}
	return err
}

func intentArchiveClassReport(classes []IntentArchiveRepairClassReport, class IntentArchiveRepairClass) *IntentArchiveRepairClassReport {
	for index := range classes {
		if classes[index].Class == class {
			copy := classes[index]
			return &copy
		}
	}
	return nil
}

func resultingIntentArchiveClasses(classes []IntentArchiveRepairClassReport, repaired IntentArchiveRepairClass) []IntentArchiveRepairClassReport {
	out := make([]IntentArchiveRepairClassReport, 0, len(classes))
	for _, class := range classes {
		if class.Class == repaired {
			continue
		}
		out = append(out, cloneIntentArchiveClassReport(class))
	}
	return out
}

func buildIntentArchiveRemainingRepairs(feature string, classes []IntentArchiveRepairClassReport, repaired IntentArchiveRepairClass) *IntentArchiveRemainingRepairs {
	if len(classes) == 0 {
		return nil
	}
	classByName := make(map[IntentArchiveRepairClass]IntentArchiveRepairClassReport, len(classes))
	for _, class := range classes {
		classByName[class.Class] = cloneIntentArchiveClassReport(class)
	}
	corrupt, hasCorrupt := classByName[IntentArchiveRepairCorruptObject]
	predictedByClass := make(map[IntentArchiveRepairClass]map[string]struct{})
	if hasCorrupt {
		delete(classByName, IntentArchiveRepairCorruptObject)
		for _, instance := range corrupt.Instances {
			for _, resultingClass := range instance.ResultingClasses {
				if instance.Hash == "" {
					continue
				}
				if predictedByClass[resultingClass] == nil {
					predictedByClass[resultingClass] = make(map[string]struct{})
				}
				predictedByClass[resultingClass][instance.Hash] = struct{}{}
			}
			if !containsIntentArchiveRepairClass(instance.ResultingClasses, IntentArchiveRepairDanglingReference) || instance.Hash == "" {
				continue
			}
			dangling := classByName[IntentArchiveRepairDanglingReference]
			dangling.Rank = 2
			dangling.Class = IntentArchiveRepairDanglingReference
			dangling.Hashes = append(dangling.Hashes, instance.Hash)
			dangling.Paths = append(dangling.Paths, instance.Path)
			dangling.Instances = append(dangling.Instances, IntentArchiveRepairInstance{
				Hash:             instance.Hash,
				Path:             instance.Path,
				GenerationIDs:    append([]string(nil), instance.GenerationIDs...),
				ResultingClasses: []IntentArchiveRepairClass{},
			})
			classByName[IntentArchiveRepairDanglingReference] = dangling
		}
		classByName[IntentArchiveRepairCorruptObject] = corrupt
	}
	stages := make([]IntentArchiveRepairStage, 0, len(classByName))
	for _, className := range intentArchiveRepairClassOrder {
		class, exists := classByName[className]
		if !exists {
			continue
		}
		class.Hashes = sortedUniqueStrings(class.Hashes)
		class.Paths = sortedUniqueStrings(class.Paths)
		predictedHashes := sortedIntentArchiveStringSet(predictedByClass[className])
		stage := IntentArchiveRepairStage{
			Class:             className,
			Hashes:            append([]string(nil), class.Hashes...),
			Paths:             append([]string(nil), class.Paths...),
			RepairCWD:         IntentArchiveRepairCWD,
			ResultingClasses:  []IntentArchiveRepairClass{},
			Commands:          []IntentArchiveRepairCommand{},
			PredictedHashes:   predictedHashes,
			AfterPrerequisite: len(predictedHashes) != 0,
		}
		switch className {
		case IntentArchiveRepairCorruptObject:
			stage.Kind = IntentArchiveRepairStageManual
			stage.Repair, stage.Commands = intentArchiveCorruptRepair(feature, class.Paths)
			for _, instance := range class.Instances {
				stage.ResultingClasses = append(stage.ResultingClasses, instance.ResultingClasses...)
			}
			stage.ResultingClasses = sortedUniqueRepairClasses(stage.ResultingClasses)
		case IntentArchiveRepairDanglingReference, IntentArchiveRepairMixedReference:
			stage.Kind = IntentArchiveRepairStagePurge
			command := intentArchiveBlobPurgeRepairCommand(feature, class.Hashes)
			stage.Repair = command.Rendered
			stage.Commands = []IntentArchiveRepairCommand{command}
		case IntentArchiveRepairUnreferencedResidue:
			stage.Kind = IntentArchiveRepairStagePurge
			command := intentArchiveOrphanPurgeRepairCommand(feature)
			stage.Repair = command.Rendered
			stage.Commands = []IntentArchiveRepairCommand{command}
		}
		stages = append(stages, stage)
	}
	for index := range stages {
		stages[index].Ordinal = index + 1
	}
	remaining := &IntentArchiveRemainingRepairs{
		RerunRequired:   true,
		RepairedClass:   repaired,
		StagesRemaining: len(stages),
		Stages:          stages,
	}
	if len(stages) != 0 {
		remaining.NextStage = &IntentArchiveNextRepairStage{
			Ordinal: stages[0].Ordinal,
			Kind:    stages[0].Kind,
			Class:   stages[0].Class,
		}
	}
	return remaining
}

func intentArchiveCorruptRepair(feature string, paths []string) (string, []IntentArchiveRepairCommand) {
	blobsRel, _ := IntentArchiveBlobsRel(feature)
	sorted := sortedUniqueStrings(paths)
	blocks := make([]string, 0, len(sorted))
	commands := make([]IntentArchiveRepairCommand, 0, len(sorted))
	for _, item := range sorted {
		if !validIntentArchiveRootRelativePath(item) || path.Dir(item) != blobsRel {
			continue
		}
		warning := "WARNING: destructive archive repair. The next command permanently deletes whatever object is at the single managed path " +
			quoteIntentArchivePOSIXShell(item) +
			", including directory contents, with no undo. Stop before running it if you need to preserve that object with tooling appropriate to its type. Removing archived bytes does not remove copies already present in Git history."
		command := IntentArchiveRepairCommand{
			Warning:  warning,
			Argv:     []string{"rm", "-rf", "--", item},
			Rendered: "rm -rf -- " + quoteIntentArchivePOSIXShell(item),
		}
		commands = append(commands, command)
		blocks = append(blocks, warning+"\n"+command.Rendered)
	}
	return strings.Join(blocks, "\n"), commands
}

func intentArchiveBlobPurgeRepairCommand(feature string, hashes []string) IntentArchiveRepairCommand {
	argv := []string{"tpatch", "feature", "intent-archive", "purge", feature}
	for _, hash := range sortedUniqueStrings(hashes) {
		argv = append(argv, "--blob", hash)
	}
	argv = append(argv, "--yes")
	return IntentArchiveRepairCommand{
		Argv:     append([]string(nil), argv...),
		Rendered: strings.Join(argv, " "),
	}
}

func intentArchiveOrphanPurgeRepairCommand(feature string) IntentArchiveRepairCommand {
	argv := []string{"tpatch", "feature", "intent-archive", "purge", feature, "--orphans", "--yes"}
	return IntentArchiveRepairCommand{
		Argv:     append([]string(nil), argv...),
		Rendered: strings.Join(argv, " "),
	}
}

func quoteIntentArchivePOSIXShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func sortedIntentArchiveStringSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func cloneIntentArchiveClassReports(classes []IntentArchiveRepairClassReport) []IntentArchiveRepairClassReport {
	if classes == nil {
		return []IntentArchiveRepairClassReport{}
	}
	out := make([]IntentArchiveRepairClassReport, len(classes))
	for index, class := range classes {
		out[index] = cloneIntentArchiveClassReport(class)
	}
	return out
}

func cloneIntentArchiveClassReport(class IntentArchiveRepairClassReport) IntentArchiveRepairClassReport {
	out := class
	out.Hashes = append([]string(nil), class.Hashes...)
	out.Paths = append([]string(nil), class.Paths...)
	out.Instances = make([]IntentArchiveRepairInstance, len(class.Instances))
	for index, instance := range class.Instances {
		out.Instances[index] = instance
		out.Instances[index].GenerationIDs = append([]string(nil), instance.GenerationIDs...)
		out.Instances[index].ResultingClasses = append([]IntentArchiveRepairClass(nil), instance.ResultingClasses...)
	}
	return out
}

func sortedUniqueRepairClasses(classes []IntentArchiveRepairClass) []IntentArchiveRepairClass {
	seen := make(map[IntentArchiveRepairClass]struct{}, len(classes))
	for _, class := range classes {
		if class != "" {
			seen[class] = struct{}{}
		}
	}
	out := make([]IntentArchiveRepairClass, 0, len(seen))
	for _, class := range intentArchiveRepairClassOrder {
		if _, exists := seen[class]; exists {
			out = append(out, class)
		}
	}
	return out
}

func containsIntentArchiveRepairClass(classes []IntentArchiveRepairClass, want IntentArchiveRepairClass) bool {
	for _, class := range classes {
		if class == want {
			return true
		}
	}
	return false
}

func equalStringSets(left, right []string) bool {
	left = sortedUniqueStrings(left)
	right = sortedUniqueStrings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func intentArchiveIndexHashSet(index IntentArchiveIndex) map[string]struct{} {
	hashes := make(map[string]struct{})
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			hashes[replacement.ContentSHA256] = struct{}{}
		}
	}
	return hashes
}

type IntentArchivePurgeResume string

const (
	IntentArchiveResumePendingRecoveryThenCompletion IntentArchivePurgeResume = "pending-recovery-then-completion"
	IntentArchiveResumeCompletionOnly                IntentArchivePurgeResume = "completion-only"
	IntentArchiveResumeOrphanScan                    IntentArchivePurgeResume = "orphan-scan"
)

type IntentArchivePurgeResult struct {
	Outcome                      IntentArchivePurgeOutcome
	Action                       string
	CompletedHashes              []string
	FinalizedHashes              []string
	RemovedBlobs                 []string
	PendingHash                  string
	RemainingHashes              []string
	Resume                       IntentArchivePurgeResume
	State                        string
	Committed                    bool
	RemovalRaceResidualDisclosed bool
	RemainingRepairs             *IntentArchiveRemainingRepairs
}

func PendingIntentArchiveHashes(index IntentArchiveIndex) []string {
	hashes := make([]string, 0)
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.WireState() == IntentArchiveWireRemovalPending {
				hashes = append(hashes, replacement.ContentSHA256)
			}
		}
	}
	return sortedUniqueStrings(hashes)
}

func ExecuteIntentArchivePurge(storage IntentArchiveStorage, plan IntentArchivePurgePlan) (IntentArchivePurgeResult, error) {
	result := IntentArchivePurgeResult{
		Outcome:          plan.Outcome,
		Action:           "none",
		CompletedHashes:  []string{},
		FinalizedHashes:  []string{},
		RemovedBlobs:     []string{},
		RemainingHashes:  []string{},
		RemainingRepairs: plan.RemainingRepairs,
	}
	if plan.Preview {
		return result, intentArchiveError(IntentArchiveCodeSelectorInvalid, "a purge preview cannot be executed", 3)
	}
	if plan.RecoveryRequired {
		err := intentArchiveError(IntentArchiveCodeRecoveryPending, "pending purge hashes must be recovered before a new selector is processed", 3)
		if len(plan.PendingHashes) != 0 {
			err.Hash = plan.PendingHashes[0]
		}
		return result, err
	}
	if storage == nil || !validIntentArchiveSlug(plan.Feature) {
		return result, intentArchiveError(IntentArchiveCodeStorageFailed, "archive storage is unavailable", 3)
	}
	if plan.Outcome == IntentArchivePurgeNoOp && plan.SelectorKind != IntentArchiveSelectorOrphans {
		blobsRel, _ := IntentArchiveBlobsRel(plan.Feature)
		if err := storage.SyncDirectory(blobsRel); err != nil {
			result.Outcome = IntentArchivePurgePartial
			result.Resume = IntentArchiveResumeCompletionOnly
			result.State = IntentArchivePurgeStateConsistent
			result.Committed = true
			partial := intentArchiveStorageError(err, "sync-blobs-directory", true, 5)
			partial.Code = IntentArchiveCodePurgePartial
			partial.Detail = "the no-op purge could not establish the blob-directory durability barrier"
			return result, partial
		}
		archiveRel, _ := IntentArchiveRootRel(plan.Feature)
		if err := storage.SyncDirectory(archiveRel); err != nil {
			result.Outcome = IntentArchivePurgePartial
			result.Resume = IntentArchiveResumeCompletionOnly
			result.State = IntentArchivePurgeStateConsistent
			result.Committed = true
			partial := intentArchiveStorageError(err, "sync-archive-directory", true, 5)
			partial.Code = IntentArchiveCodePurgePartial
			partial.Detail = "the no-op purge could not establish the archive-directory durability barrier"
			return result, partial
		}
		result.Outcome = IntentArchivePurgeNoOp
		return result, nil
	}
	snapshot, err := CaptureIntentArchive(storage, plan.Feature)
	if err != nil {
		return result, err
	}
	if !snapshot.IndexCapture.Equal(plan.IndexPreimage) {
		return result, intentArchiveError(IntentArchiveCodePurgeIndexChanged, "index.json changed after purge planning", 3)
	}
	fresh, err := BuildIntentArchivePurgePlan(snapshot, plan.Selector, true)
	if err != nil {
		return result, err
	}
	if fresh.RecoveryRequired {
		pendingErr := intentArchiveError(IntentArchiveCodeRecoveryPending, "pending purge hashes appeared after purge planning", 3)
		if len(fresh.PendingHashes) != 0 {
			pendingErr.Hash = fresh.PendingHashes[0]
		}
		return result, pendingErr
	}
	if !equalStringSets(fresh.Hashes, plan.Hashes) ||
		!equalIntentArchiveBlobRemovalSets(fresh.BlobRemovals, plan.BlobRemovals) {
		return result, intentArchiveError(IntentArchiveCodePurgeIndexChanged, "the purge selection changed after planning", 3)
	}
	if err := preflightIntentArchivePurge(storage, snapshot, fresh); err != nil {
		return result, err
	}
	if fresh.SelectorKind == IntentArchiveSelectorOrphans {
		return executeIntentArchiveOrphanPurge(storage, snapshot, fresh)
	}
	return executeIntentArchiveHashPurge(storage, snapshot, fresh)
}

func preflightIntentArchivePurge(storage IntentArchiveStorage, snapshot IntentArchiveSnapshot, plan IntentArchivePurgePlan) error {
	indexRel, _ := IntentArchiveIndexRel(plan.Feature)
	if len(plan.References) != 0 {
		if err := storage.PreflightIndexCAS(indexRel, snapshot.IndexCapture.Identity); err != nil {
			return intentArchiveStorageError(err, "preflight-index-cas", false, 3)
		}
	}
	for _, observation := range plan.BlobRemovals {
		if observation.State != IntentArchiveBlobPresentCorrect {
			err := intentArchiveError(IntentArchiveCodeBlobCorrupt, "a selected blob is not a regular hash-correct object", 3)
			err.Hash = observation.Hash
			return err
		}
		probe, err := storage.ProbeBlob(observation.Path)
		if err != nil {
			return intentArchiveStorageError(err, "preflight-probe-blob", false, 3)
		}
		if probe.Kind != IntentArchiveBlobKindRegular ||
			probe.SHA256 != observation.Hash ||
			probe.SizeBytes != observation.SizeBytes ||
			probe.Identity != observation.Identity {
			corrupt := intentArchiveError(IntentArchiveCodeBlobCorrupt, "a selected blob changed during purge preflight", 3)
			corrupt.Hash = observation.Hash
			return corrupt
		}
		if err := storage.PreflightBlobRemove(observation.Path, probe.Identity); err != nil {
			return intentArchiveStorageError(err, "preflight-blob-remove", false, 3)
		}
	}
	return nil
}

func executeIntentArchiveOrphanPurge(storage IntentArchiveStorage, snapshot IntentArchiveSnapshot, plan IntentArchivePurgePlan) (IntentArchivePurgeResult, error) {
	result := IntentArchivePurgeResult{
		Outcome:          IntentArchivePurgePurged,
		Action:           "none",
		CompletedHashes:  []string{},
		FinalizedHashes:  []string{},
		RemovedBlobs:     []string{},
		RemainingHashes:  []string{},
		RemainingRepairs: plan.RemainingRepairs,
	}
	indexRel, _ := IntentArchiveIndexRel(plan.Feature)
	blobsRel, _ := IntentArchiveBlobsRel(plan.Feature)
	if err := storage.SyncDirectory(blobsRel); err != nil {
		if len(plan.BlobRemovals) == 0 {
			result.Outcome = IntentArchivePurgePartial
			result.Resume = IntentArchiveResumeOrphanScan
			result.State = IntentArchivePurgeStateConsistent
			result.Committed = true
			partial := intentArchiveStorageError(err, "sync-blobs-directory", true, 5)
			partial.Code = IntentArchiveCodePurgePartial
			partial.Detail = "the no-op orphan scan could not establish the blob-directory durability barrier"
			return result, partial
		}
		return result, intentArchiveStorageError(err, "sync-blobs-directory", false, 3)
	}
	for index, observation := range plan.BlobRemovals {
		current, err := storage.CaptureIndex(indexRel)
		if err != nil {
			return intentArchiveOrphanFailure(result, plan.BlobRemovals, index, intentArchiveStorageError(err, "capture-index", result.Committed, intentArchiveExitAfterMutation(result.Committed)))
		}
		if !current.Equal(snapshot.IndexCapture) {
			changed := intentArchiveError(IntentArchiveCodePurgeIndexChanged, "index.json changed before orphan removal", intentArchiveExitAfterMutation(result.Committed))
			changed.Committed = result.Committed
			return intentArchiveOrphanFailure(result, plan.BlobRemovals, index, changed)
		}
		probe, err := storage.ProbeBlob(observation.Path)
		if err != nil {
			return intentArchiveOrphanFailure(result, plan.BlobRemovals, index, intentArchiveStorageError(err, "probe-orphan", result.Committed, intentArchiveExitAfterMutation(result.Committed)))
		}
		if probe.Kind != IntentArchiveBlobKindRegular ||
			probe.SHA256 != observation.Hash ||
			probe.Identity != observation.Identity {
			corrupt := intentArchiveError(IntentArchiveCodeBlobCorrupt, "an orphan changed before removal", intentArchiveExitAfterMutation(result.Committed))
			corrupt.Hash = observation.Hash
			corrupt.Committed = result.Committed
			return intentArchiveOrphanFailure(result, plan.BlobRemovals, index, corrupt)
		}
		if afterPurgeBlobRevalidate != nil {
			afterPurgeBlobRevalidate(observation.Path)
		}
		authorized := intentArchiveHashUnreferenced(snapshot.Index, observation.Hash)
		mutation, err := removeIntentArchiveBlob(storage, observation.Path, observation.Hash, probe.Identity, authorized)
		result.RemovalRaceResidualDisclosed = true
		if mutation.Committed {
			result.Committed = true
			result.RemovedBlobs = append(result.RemovedBlobs, observation.Path)
		}
		if mutation.Committed {
			if syncErr := storage.SyncDirectory(blobsRel); syncErr != nil {
				return intentArchiveOrphanFailure(result, plan.BlobRemovals, index, intentArchiveStorageError(syncErr, "sync-blobs-directory", true, 5))
			}
			result.CompletedHashes = append(result.CompletedHashes, observation.Hash)
			if index == 0 && failOrphanRemoveAfterFirst != nil {
				if injected := failOrphanRemoveAfterFirst(); injected != nil {
					return intentArchiveOrphanFailure(result, plan.BlobRemovals, index+1, intentArchiveStorageError(injected, "after-first-orphan", true, 5))
				}
			}
		}
		if err != nil {
			failedIndex := index
			if mutation.Committed {
				failedIndex++
			}
			return intentArchiveOrphanFailure(result, plan.BlobRemovals, failedIndex, intentArchiveStorageError(err, "remove-orphan", result.Committed, intentArchiveExitAfterMutation(result.Committed)))
		}
	}
	if len(plan.BlobRemovals) == 0 {
		result.Outcome = IntentArchivePurgeNoOp
	}
	result.CompletedHashes = sortedUniqueStrings(result.CompletedHashes)
	result.RemovedBlobs = sortedUniqueStrings(result.RemovedBlobs)
	return result, nil
}

func intentArchiveOrphanFailure(result IntentArchivePurgeResult, removals []IntentArchiveBlobObservation, failedIndex int, cause error) (IntentArchivePurgeResult, error) {
	if !result.Committed {
		return result, cause
	}
	result.Outcome = IntentArchivePurgePartial
	result.Resume = IntentArchiveResumeOrphanScan
	result.State = IntentArchivePurgeStateConsistent
	result.RemainingHashes = []string{}
	for index := failedIndex; index < len(removals); index++ {
		result.RemainingHashes = append(result.RemainingHashes, removals[index].Hash)
	}
	result.RemainingHashes = sortedUniqueStrings(result.RemainingHashes)
	partial := intentArchiveError(IntentArchiveCodePurgePartial, "the orphan scan stopped after at least one removal and must resume from current storage", 5)
	partial.Committed = true
	return result, partial
}

func executeIntentArchiveHashPurge(storage IntentArchiveStorage, snapshot IntentArchiveSnapshot, plan IntentArchivePurgePlan) (IntentArchivePurgeResult, error) {
	result := IntentArchivePurgeResult{
		Outcome:          IntentArchivePurgePurged,
		Action:           "none",
		CompletedHashes:  []string{},
		FinalizedHashes:  []string{},
		RemovedBlobs:     []string{},
		RemainingHashes:  []string{},
		RemainingRepairs: plan.RemainingRepairs,
	}
	currentCapture := snapshot.IndexCapture
	for index, hash := range plan.Hashes {
		context := intentArchiveTransitionNewSelection
		if result.Committed {
			context = intentArchiveTransitionPriorProgress
		}
		execution, err := executeIntentArchivePurgeHash(storage, plan.Feature, hash, currentCapture, context)
		if execution.Removed {
			result.RemovedBlobs = append(result.RemovedBlobs, execution.BlobRel)
			result.RemovalRaceResidualDisclosed = true
		}
		if execution.Mutated {
			result.Committed = true
		}
		if execution.Completed {
			result.CompletedHashes = append(result.CompletedHashes, hash)
		}
		if execution.Capture.Exists || execution.Capture.Identity != "" {
			currentCapture = execution.Capture
		}
		if err == nil && execution.Mutated &&
			index+1 < len(plan.Hashes) && failPurgeBetweenHashes != nil {
			err = failPurgeBetweenHashes()
		}
		if err != nil {
			var typed *IntentArchiveError
			if errors.As(err, &typed) && typed.Committed {
				result.Committed = true
			}
			if typed != nil && typed.Code == IntentArchiveCodePurgeEvidenceDivergent {
				typed.Committed = typed.Committed || result.Committed
				result.Committed = typed.Committed
				intentArchiveSetDivergentProgress(&result, execution, hash, plan.Hashes[index+1:])
				return result, typed
			}
			if !result.Committed {
				return result, err
			}
			result.Outcome = IntentArchivePurgePartial
			result.State = IntentArchivePurgeStateConsistent
			if execution.Pending {
				result.PendingHash = hash
				result.Resume = IntentArchiveResumePendingRecoveryThenCompletion
				result.RemainingHashes = append([]string(nil), plan.Hashes[index+1:]...)
			} else {
				result.Resume = IntentArchiveResumeCompletionOnly
				start := index
				if execution.Completed {
					start = index + 1
				}
				result.RemainingHashes = append([]string(nil), plan.Hashes[start:]...)
			}
			partial := intentArchiveError(IntentArchiveCodePurgePartial, "the purge stopped after its first mutation and can be resumed", 5)
			partial.Hash = hash
			partial.Committed = true
			result.CompletedHashes = sortedUniqueStrings(result.CompletedHashes)
			result.RemainingHashes = sortedUniqueStrings(result.RemainingHashes)
			result.RemovedBlobs = sortedUniqueStrings(result.RemovedBlobs)
			return result, partial
		}
	}
	if !result.Committed {
		result.Outcome = IntentArchivePurgeNoOp
	}
	result.CompletedHashes = sortedUniqueStrings(result.CompletedHashes)
	result.RemovedBlobs = sortedUniqueStrings(result.RemovedBlobs)
	return result, nil
}

type intentArchiveHashExecution struct {
	Capture       IntentArchiveIndexCapture
	Mutated       bool
	Removed       bool
	Completed     bool
	Pending       bool
	OwnedRecovery bool
	BlobRel       string
}

type intentArchivePurgeTransitionContext uint8

const (
	intentArchiveTransitionNewSelection intentArchivePurgeTransitionContext = iota
	intentArchiveTransitionPriorProgress
	intentArchiveTransitionOwnedRecovery
)

func (context intentArchivePurgeTransitionContext) committed() bool {
	return context != intentArchiveTransitionNewSelection
}

func executeIntentArchivePurgeHash(storage IntentArchiveStorage, feature, hash string, expected IntentArchiveIndexCapture, context intentArchivePurgeTransitionContext) (intentArchiveHashExecution, error) {
	execution := intentArchiveHashExecution{
		Capture:       expected,
		Pending:       context == intentArchiveTransitionOwnedRecovery,
		OwnedRecovery: context == intentArchiveTransitionOwnedRecovery,
	}
	blobRel, _ := IntentArchiveBlobRel(feature, hash)
	execution.BlobRel = blobRel
	currentCapture, currentIndex, err := captureIntentArchiveIndexOnly(storage, feature)
	if err != nil {
		execution.Capture = currentCapture
		return execution, promoteIntentArchivePurgeDivergence(err, hash, context.committed())
	}
	if !currentCapture.Equal(expected) {
		execution.Capture = currentCapture
		intentArchiveSetHashExecutionEvidence(&execution, currentIndex, hash)
		if context != intentArchiveTransitionOwnedRecovery || !execution.Pending {
			if context.committed() {
				return execution, intentArchiveCommittedIndexChangeError(currentIndex, hash, "index.json identity changed after purge evidence became committed", execution.OwnedRecovery)
			}
			changed := intentArchiveError(IntentArchiveCodePurgeIndexChanged, "index.json changed before the per-hash transition", 3)
			changed.Hash = hash
			return execution, changed
		}
	}
	execution.Capture = currentCapture
	probe, err := storage.ProbeBlob(blobRel)
	if err != nil {
		return execution, intentArchiveStorageError(err, "probe-blob", context.committed(), intentArchiveExitAfterMutation(context.committed()))
	}
	switch probe.Kind {
	case IntentArchiveBlobKindSymlink,
		IntentArchiveBlobKindDirectory,
		IntentArchiveBlobKindFIFO,
		IntentArchiveBlobKindDevice,
		IntentArchiveBlobKindOtherNonRegular:
		return execution, intentArchiveUnidentifiablePurgeError(hash, context == intentArchiveTransitionOwnedRecovery)
	case IntentArchiveBlobKindRegular:
		expectedSize, _ := intentArchiveExpectedSize(currentIndex, hash)
		if probe.SHA256 != hash || (expectedSize >= 0 && probe.SizeBytes != expectedSize) {
			return execution, intentArchiveUnidentifiablePurgeError(hash, context == intentArchiveTransitionOwnedRecovery)
		}
	case IntentArchiveBlobKindAbsent:
		return executeIntentArchiveAbsentTombstone(storage, feature, hash, execution, context)
	default:
		return execution, intentArchiveUnidentifiablePurgeError(hash, context == intentArchiveTransitionOwnedRecovery)
	}

	references := intentArchiveReferencesForHash(currentIndex, hash)
	if len(references) == 0 {
		revalidatedCapture, revalidatedIndex, captureErr := captureIntentArchiveIndexOnly(storage, feature)
		if captureErr != nil {
			execution.Capture = revalidatedCapture
			return execution, promoteIntentArchivePurgeDivergence(captureErr, hash, context.committed())
		}
		if !revalidatedCapture.Equal(currentCapture) {
			execution.Capture = revalidatedCapture
			intentArchiveSetHashExecutionEvidence(&execution, revalidatedIndex, hash)
			if context.committed() {
				return execution, intentArchiveCommittedIndexChangeError(revalidatedIndex, hash, "index.json identity changed after an earlier purge mutation", execution.OwnedRecovery)
			}
			return execution, intentArchiveError(IntentArchiveCodePurgeIndexChanged, "index.json changed before unreferenced blob removal", 3)
		}
		revalidatedProbe, probeErr := storage.ProbeBlob(blobRel)
		if probeErr != nil {
			return execution, intentArchiveStorageError(probeErr, "revalidate-unreferenced-blob", context.committed(), intentArchiveExitAfterMutation(context.committed()))
		}
		if revalidatedProbe.Kind == IntentArchiveBlobKindAbsent {
			return executeIntentArchiveAbsentTombstone(storage, feature, hash, execution, context)
		}
		if revalidatedProbe.Kind != IntentArchiveBlobKindRegular || revalidatedProbe.SHA256 != hash {
			return execution, intentArchiveUnidentifiablePurgeError(hash, context == intentArchiveTransitionOwnedRecovery)
		}
		if afterPurgeBlobRevalidate != nil {
			afterPurgeBlobRevalidate(blobRel)
		}
		authorized := intentArchiveHashUnreferenced(revalidatedIndex, hash)
		mutation, removeErr := removeIntentArchiveBlob(storage, blobRel, hash, revalidatedProbe.Identity, authorized)
		execution.Mutated = mutation.Committed
		execution.Removed = mutation.Committed
		if mutation.Committed {
			blobsRel, _ := IntentArchiveBlobsRel(feature)
			if syncErr := storage.SyncDirectory(blobsRel); syncErr != nil {
				return execution, intentArchiveStorageError(syncErr, "sync-blobs-directory", true, 5)
			}
			execution.Completed = true
		}
		if removeErr != nil {
			committed := execution.Mutated || context.committed()
			return execution, intentArchiveStorageError(removeErr, "remove-unreferenced-blob", committed, intentArchiveExitAfterMutation(committed))
		}
		return execution, nil
	}

	claimed, changed := setIntentArchiveHashState(currentIndex, hash, IntentArchiveWireRemovalPending)
	claimCapture := currentCapture
	if changed {
		nextCapture, mutation, claimErr := publishIntentArchiveIndex(storage, feature, currentCapture, claimed)
		execution.Mutated = mutation.Committed
		execution.Pending = mutation.Committed
		execution.Capture = nextCapture
		if claimErr != nil {
			return execution, normalizeIntentArchivePostMutationIndexError(
				storage,
				feature,
				hash,
				currentCapture,
				&execution,
				claimErr,
				execution.Mutated || context.committed(),
				mutation.Committed,
			)
		}
		if mutation.Committed && !context.committed() && failPurgeAfterFirstMutation != nil {
			if injected := failPurgeAfterFirstMutation(); injected != nil {
				return execution, intentArchiveStorageError(injected, "after-first-mutation", true, 5)
			}
		}
		claimCapture = nextCapture
		context = intentArchiveTransitionOwnedRecovery
	} else {
		execution.Pending = true
		context = intentArchiveTransitionOwnedRecovery
		archiveRel, _ := IntentArchiveRootRel(feature)
		if syncErr := storage.SyncDirectory(archiveRel); syncErr != nil {
			return execution, intentArchiveStorageError(syncErr, "sync-archive-directory", true, 5)
		}
	}

	revalidatedCapture, revalidatedIndex, err := captureIntentArchiveIndexOnly(storage, feature)
	if err != nil {
		execution.Capture = revalidatedCapture
		return execution, promoteIntentArchivePurgeDivergence(err, hash, true)
	}
	execution.Capture = revalidatedCapture
	if !revalidatedCapture.Equal(claimCapture) {
		intentArchiveSetHashExecutionEvidence(&execution, revalidatedIndex, hash)
		return execution, intentArchiveCommittedIndexChangeError(revalidatedIndex, hash, "index.json identity changed after the global claim", execution.OwnedRecovery)
	}
	if !allIntentArchiveReferencesPending(revalidatedIndex, hash) {
		intentArchiveSetHashExecutionEvidence(&execution, revalidatedIndex, hash)
		return execution, intentArchiveCommittedIndexChangeError(revalidatedIndex, hash, "same-hash ownership changed after the global claim", execution.OwnedRecovery)
	}
	revalidatedProbe, err := storage.ProbeBlob(blobRel)
	if err != nil {
		return execution, intentArchiveStorageError(err, "revalidate-blob", true, 5)
	}
	if intentArchiveBlobKindUnidentifiable(revalidatedProbe.Kind) ||
		(revalidatedProbe.Kind == IntentArchiveBlobKindRegular &&
			(revalidatedProbe.SHA256 != hash || revalidatedProbe.SizeBytes != intentArchiveMustExpectedSize(revalidatedIndex, hash))) {
		divergent := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, "the owned blob is no longer identifiable", 6)
		divergent.Hash = hash
		divergent.Committed = true
		return execution, divergent
	}
	if revalidatedProbe.Kind == IntentArchiveBlobKindAbsent {
		return executeIntentArchiveAbsentTombstone(storage, feature, hash, execution, context)
	}
	if afterPurgeBlobRevalidate != nil {
		afterPurgeBlobRevalidate(blobRel)
	}
	tombstoned, _ := setIntentArchiveHashState(revalidatedIndex, hash, IntentArchiveWireTombstoned)
	if revalidatedProbe.Kind == IntentArchiveBlobKindRegular {
		if !allIntentArchiveReferencesPending(revalidatedIndex, hash) {
			return execution, intentArchiveError(IntentArchiveCodePurgeIndexChanged, "blob removal was not authorized by an all-pending index", 5)
		}
		mutation, removeErr := removeIntentArchiveBlob(storage, blobRel, hash, revalidatedProbe.Identity, true)
		execution.Mutated = execution.Mutated || mutation.Committed
		execution.Removed = mutation.Committed
		if mutation.Committed {
			blobsRel, _ := IntentArchiveBlobsRel(feature)
			if syncErr := storage.SyncDirectory(blobsRel); syncErr != nil {
				return execution, intentArchiveStorageError(syncErr, "sync-blobs-directory", true, 5)
			}
		}
		if removeErr != nil {
			return execution, intentArchiveStorageError(removeErr, "remove-blob", true, 5)
		}
	} else {
		return execution, intentArchiveUnidentifiablePurgeError(hash, true)
	}
	if beforePendingTombstoneCAS != nil {
		beforePendingTombstoneCAS(hash)
	}
	nextCapture, mutation, tombstoneErr := publishIntentArchiveIndex(storage, feature, revalidatedCapture, tombstoned)
	execution.Mutated = execution.Mutated || mutation.Committed
	execution.Capture = nextCapture
	execution.Pending = !mutation.Committed || tombstoneErr != nil
	execution.Completed = mutation.Committed && tombstoneErr == nil
	if tombstoneErr != nil {
		if observedCapture, observedIndex, captureErr := captureIntentArchiveIndexOnly(storage, feature); captureErr == nil {
			execution.Capture = observedCapture
			execution.Pending = intentArchiveHashHasState(observedIndex, hash, IntentArchiveWireRemovalPending)
			execution.Completed = intentArchiveHashAllTombstoned(observedIndex, hash)
		}
		return execution, normalizeIntentArchivePostMutationIndexError(
			storage,
			feature,
			hash,
			revalidatedCapture,
			&execution,
			tombstoneErr,
			true,
			mutation.Committed,
		)
	}
	execution.Pending = false
	return execution, nil
}

func executeIntentArchiveAbsentTombstone(
	storage IntentArchiveStorage,
	feature, hash string,
	execution intentArchiveHashExecution,
	context intentArchivePurgeTransitionContext,
) (intentArchiveHashExecution, error) {
	blobsRel, _ := IntentArchiveBlobsRel(feature)
	validatedCapture := execution.Capture
	committed := execution.Mutated || context.committed()
	if err := storage.SyncDirectory(blobsRel); err != nil {
		return execution, intentArchiveStorageError(err, "sync-blobs-directory", committed, intentArchiveExitAfterMutation(committed))
	}
	blobRel, _ := IntentArchiveBlobRel(feature, hash)
	probe, err := storage.ProbeBlob(blobRel)
	if err != nil {
		return execution, intentArchiveStorageError(err, "reprobe-absent-blob", committed, intentArchiveExitAfterMutation(committed))
	}
	latestCapture, latestIndex, captureErr := captureIntentArchiveIndexOnly(storage, feature)
	execution.Capture = latestCapture
	if captureErr != nil {
		return execution, promoteIntentArchivePurgeDivergence(captureErr, hash, committed)
	}
	intentArchiveSetHashExecutionEvidence(&execution, latestIndex, hash)
	if !latestCapture.Equal(validatedCapture) {
		switch context {
		case intentArchiveTransitionNewSelection:
			changed := intentArchiveError(IntentArchiveCodePurgeIndexChanged, "index.json changed before the new selector's first transition", 3)
			changed.Hash = hash
			return execution, changed
		case intentArchiveTransitionPriorProgress:
			return execution, intentArchiveCommittedIndexChangeError(latestIndex, hash, "index.json identity changed after an earlier purge mutation", execution.OwnedRecovery)
		case intentArchiveTransitionOwnedRecovery:
		default:
			return execution, intentArchiveError(IntentArchiveCodePurgeIndexChanged, "the purge transition context is invalid", intentArchiveExitAfterMutation(committed))
		}
	}
	if probe.Kind != IntentArchiveBlobKindAbsent {
		if intentArchiveBlobKindUnidentifiable(probe.Kind) ||
			(probe.Kind == IntentArchiveBlobKindRegular &&
				(probe.SHA256 != hash ||
					(intentArchiveMustExpectedSize(latestIndex, hash) >= 0 &&
						probe.SizeBytes != intentArchiveMustExpectedSize(latestIndex, hash)))) {
			return execution, intentArchiveUnidentifiablePurgeError(hash, context == intentArchiveTransitionOwnedRecovery)
		}
		if execution.Pending {
			return execution, intentArchiveCommittedIndexChangeError(latestIndex, hash, "the blob appeared after its absence durability barrier", execution.OwnedRecovery)
		}
		changed := intentArchiveError(IntentArchiveCodePurgeIndexChanged, "the blob appeared after its absence durability barrier; retry from the present-object path", intentArchiveExitAfterMutation(committed))
		changed.Hash = hash
		changed.Committed = committed
		return execution, changed
	}
	if context == intentArchiveTransitionOwnedRecovery && !intentArchiveHashHasState(latestIndex, hash, IntentArchiveWireRemovalPending) {
		if intentArchiveHashAllTombstoned(latestIndex, hash) {
			execution.Completed = true
			execution.Pending = false
			archiveRel, _ := IntentArchiveRootRel(feature)
			if err := storage.SyncDirectory(archiveRel); err != nil {
				return execution, intentArchiveStorageError(err, "sync-archive-directory", true, 5)
			}
			return execution, nil
		}
		return execution, intentArchiveCommittedIndexChangeError(latestIndex, hash, "the adopted pending ownership disappeared before absent-object finalization", execution.OwnedRecovery)
	}
	tombstoned, changed := setIntentArchiveHashState(latestIndex, hash, IntentArchiveWireTombstoned)
	if !changed {
		execution.Completed = true
		execution.Pending = false
		archiveRel, _ := IntentArchiveRootRel(feature)
		if err := storage.SyncDirectory(archiveRel); err != nil {
			return execution, intentArchiveStorageError(err, "sync-archive-directory", true, 5)
		}
		return execution, nil
	}
	if beforePendingTombstoneCAS != nil {
		beforePendingTombstoneCAS(hash)
	}
	nextCapture, mutation, tombstoneErr := publishIntentArchiveIndex(storage, feature, latestCapture, tombstoned)
	execution.Mutated = execution.Mutated || mutation.Committed
	execution.Capture = nextCapture
	execution.Pending = !mutation.Committed || tombstoneErr != nil
	execution.Completed = mutation.Committed && tombstoneErr == nil
	if tombstoneErr != nil {
		if observedCapture, observedIndex, observedErr := captureIntentArchiveIndexOnly(storage, feature); observedErr == nil {
			execution.Capture = observedCapture
			intentArchiveSetHashExecutionEvidence(&execution, observedIndex, hash)
		}
		return execution, normalizeIntentArchivePostMutationIndexError(
			storage,
			feature,
			hash,
			latestCapture,
			&execution,
			tombstoneErr,
			committed || mutation.Committed,
			mutation.Committed,
		)
	}
	execution.Pending = false
	return execution, nil
}

func RecoverPendingPurge(storage IntentArchiveStorage, feature string) (IntentArchivePurgeResult, error) {
	result := IntentArchivePurgeResult{
		Outcome:         IntentArchivePurgeNoOp,
		Action:          "none",
		CompletedHashes: []string{},
		FinalizedHashes: []string{},
		RemovedBlobs:    []string{},
		RemainingHashes: []string{},
	}
	if storage == nil {
		return result, intentArchiveError(IntentArchiveCodeStorageFailed, "archive storage is unavailable", 3)
	}
	capture, index, err := captureIntentArchiveIndexOnly(storage, feature)
	if err != nil {
		return result, err
	}
	pending := PendingIntentArchiveHashes(index)
	blobsRel, _ := IntentArchiveBlobsRel(feature)
	if len(pending) == 0 {
		if syncErr := storage.SyncDirectory(blobsRel); syncErr != nil {
			result.Outcome = IntentArchivePurgePartial
			result.Resume = IntentArchiveResumeCompletionOnly
			result.State = IntentArchivePurgeStateConsistent
			result.Committed = true
			partial := intentArchiveStorageError(syncErr, "sync-blobs-directory", true, 5)
			partial.Code = IntentArchiveCodePurgePartial
			partial.Detail = "finalized purge removals could not be made durable"
			return result, partial
		}
		archiveRel, _ := IntentArchiveRootRel(feature)
		if syncErr := storage.SyncDirectory(archiveRel); syncErr != nil {
			result.Outcome = IntentArchivePurgePartial
			result.Resume = IntentArchiveResumeCompletionOnly
			result.State = IntentArchivePurgeStateConsistent
			result.Committed = true
			partial := intentArchiveStorageError(syncErr, "sync-archive-directory", true, 5)
			partial.Code = IntentArchiveCodePurgePartial
			partial.Detail = "finalized purge tombstones could not be made durable"
			return result, partial
		}
		return result, nil
	}
	archiveRel, _ := IntentArchiveRootRel(feature)
	if syncErr := storage.SyncDirectory(archiveRel); syncErr != nil {
		return classifyIntentArchivePendingRecoveryInterruption(storage, feature, pending[0], "pending purge recovery could not establish prior index durability")
	}
	indexRel, _ := IntentArchiveIndexRel(feature)
	if err := storage.PreflightIndexCAS(indexRel, capture.Identity); err != nil {
		return classifyIntentArchivePendingRecoveryInterruption(storage, feature, pending[0], "pending purge recovery could not establish index CAS capability")
	}
	for _, hash := range pending {
		blobRel, _ := IntentArchiveBlobRel(feature, hash)
		probe, probeErr := storage.ProbeBlob(blobRel)
		if probeErr != nil {
			return classifyIntentArchivePendingRecoveryInterruption(storage, feature, hash, "pending purge recovery could not probe owned evidence")
		}
		if intentArchiveBlobKindUnidentifiable(probe.Kind) ||
			(probe.Kind == IntentArchiveBlobKindRegular &&
				(probe.SHA256 != hash || probe.SizeBytes != intentArchiveMustExpectedSize(index, hash))) {
			result.Committed = true
			result.PendingHash = hash
			result.RemainingHashes = intentArchiveStringsAfter(pending, hash)
			divergent := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, "an owned blob is present but unidentifiable", 6)
			divergent.Hash = hash
			divergent.Committed = true
			return result, divergent
		}
		if probe.Kind == IntentArchiveBlobKindRegular {
			if err := storage.PreflightBlobRemove(blobRel, probe.Identity); err != nil {
				return classifyIntentArchivePendingRecoveryInterruption(storage, feature, hash, "pending purge recovery could not establish blob removal capability")
			}
		}
	}

	currentCapture := capture
	result.Outcome = IntentArchivePurgeRecovered
	result.Committed = true
	for index, hash := range pending {
		execution, executionErr := executeIntentArchivePurgeHash(storage, feature, hash, currentCapture, intentArchiveTransitionOwnedRecovery)
		if execution.Removed {
			result.RemovedBlobs = append(result.RemovedBlobs, execution.BlobRel)
			result.RemovalRaceResidualDisclosed = true
		}
		if execution.Completed {
			result.CompletedHashes = append(result.CompletedHashes, hash)
			result.FinalizedHashes = append(result.FinalizedHashes, hash)
		}
		if execution.Capture.Exists || execution.Capture.Identity != "" {
			currentCapture = execution.Capture
		}
		if executionErr != nil {
			var typed *IntentArchiveError
			if errors.As(executionErr, &typed) && typed.Code == IntentArchiveCodePurgeEvidenceDivergent {
				typed.Committed = true
				intentArchiveSetDivergentProgress(&result, execution, hash, pending[index+1:])
				return result, typed
			}
			result.Outcome = IntentArchivePurgePartial
			result.State = IntentArchivePurgeStateConsistent
			if execution.Pending {
				result.PendingHash = hash
				result.Resume = IntentArchiveResumePendingRecoveryThenCompletion
				result.RemainingHashes = append([]string(nil), pending[index+1:]...)
			} else {
				result.Resume = IntentArchiveResumeCompletionOnly
				start := index
				if execution.Completed {
					start = index + 1
				}
				result.RemainingHashes = append([]string(nil), pending[start:]...)
			}
			partial := intentArchiveError(IntentArchiveCodePurgePartial, "pending purge recovery stopped and must be resumed", 5)
			partial.Hash = hash
			partial.Committed = true
			result.CompletedHashes = sortedUniqueStrings(result.CompletedHashes)
			result.FinalizedHashes = sortedUniqueStrings(result.FinalizedHashes)
			result.RemainingHashes = sortedUniqueStrings(result.RemainingHashes)
			result.RemovedBlobs = sortedUniqueStrings(result.RemovedBlobs)
			return result, partial
		}
	}
	result.CompletedHashes = sortedUniqueStrings(result.CompletedHashes)
	result.FinalizedHashes = sortedUniqueStrings(result.FinalizedHashes)
	result.RemovedBlobs = sortedUniqueStrings(result.RemovedBlobs)
	return result, nil
}

func classifyIntentArchivePendingRecoveryInterruption(
	storage IntentArchiveStorage,
	feature, hash, detail string,
) (IntentArchivePurgeResult, error) {
	result := IntentArchivePurgeResult{
		Outcome:         IntentArchivePurgePartial,
		Action:          "none",
		CompletedHashes: []string{},
		FinalizedHashes: []string{},
		RemovedBlobs:    []string{},
		RemainingHashes: []string{},
		Committed:       true,
	}
	_, latestIndex, captureErr := captureIntentArchiveIndexOnly(storage, feature)
	blobRel, _ := IntentArchiveBlobRel(feature, hash)
	probe, probeErr := storage.ProbeBlob(blobRel)
	if captureErr != nil {
		promoted := promoteIntentArchivePurgeDivergence(captureErr, hash, true)
		var typed *IntentArchiveError
		if errors.As(promoted, &typed) && typed.Code == IntentArchiveCodePurgeEvidenceDivergent {
			return result, typed
		}
		result.PendingHash = hash
		result.Resume = IntentArchiveResumePendingRecoveryThenCompletion
		partial := intentArchiveError(IntentArchiveCodePurgePartial, detail+"; current index evidence could not be recaptured", 5)
		partial.Hash = hash
		partial.Committed = true
		return result, partial
	}
	result.State = IntentArchivePurgeStateConsistent
	latestPending := PendingIntentArchiveHashes(latestIndex)
	if intentArchiveHashHasState(latestIndex, hash, IntentArchiveWireRemovalPending) {
		result.PendingHash = hash
		result.Resume = IntentArchiveResumePendingRecoveryThenCompletion
		result.RemainingHashes = intentArchiveStringsExcept(latestPending, hash)
	} else {
		result.Resume = IntentArchiveResumeCompletionOnly
		result.RemainingHashes = sortedUniqueStrings(append([]string{hash}, latestPending...))
	}
	if probeErr != nil {
		partial := intentArchiveError(IntentArchiveCodePurgePartial, detail+"; current blob evidence could not be reprobed", 5)
		partial.Hash = hash
		partial.Committed = true
		return result, partial
	}
	expectedSize := intentArchiveMustExpectedSize(latestIndex, hash)
	if intentArchiveBlobKindUnidentifiable(probe.Kind) ||
		(probe.Kind == IntentArchiveBlobKindRegular &&
			(probe.SHA256 != hash || (expectedSize >= 0 && probe.SizeBytes != expectedSize))) {
		result.State = ""
		return result, intentArchivePurgeEvidenceDivergent(hash, "owned purge evidence is present but unidentifiable after a preflight interruption")
	}
	if result.PendingHash != "" {
		partial := intentArchiveError(IntentArchiveCodePurgePartial, detail+"; retry must reclaim every current same-hash reference", 5)
		partial.Hash = hash
		partial.Committed = true
		return result, partial
	}
	result.State = ""
	divergent := intentArchivePurgeEvidenceDivergent(hash, detail+"; pending ownership was removed by a concurrent index change")
	divergent.Committed = true
	return result, divergent
}

func captureIntentArchiveIndexOnly(storage IntentArchiveStorage, feature string) (IntentArchiveIndexCapture, IntentArchiveIndex, error) {
	indexRel, err := IntentArchiveIndexRel(feature)
	if err != nil {
		return IntentArchiveIndexCapture{}, IntentArchiveIndex{}, err
	}
	capture, err := storage.CaptureIndex(indexRel)
	if err != nil {
		return capture, IntentArchiveIndex{}, intentArchiveStorageError(err, "capture-index", false, 3)
	}
	capture.Raw = append([]byte(nil), capture.Raw...)
	if !capture.Exists {
		if len(capture.Raw) != 0 {
			return capture, IntentArchiveIndex{}, intentArchiveError(IntentArchiveCodeIndexCorrupt, "an absent index capture carried raw bytes", 3)
		}
		index, newErr := NewIntentArchiveIndex(feature)
		return capture, index, newErr
	}
	index, err := DecodeIntentArchiveIndex(capture.Raw, feature)
	if err == nil && afterPurgeIndexDecode != nil {
		afterPurgeIndexDecode(indexRel)
	}
	return capture, index, err
}

func publishIntentArchiveIndex(storage IntentArchiveStorage, feature string, current IntentArchiveIndexCapture, next IntentArchiveIndex) (IntentArchiveIndexCapture, IntentArchiveMutationResult, error) {
	var mutation IntentArchiveMutationResult
	indexRel, _ := IntentArchiveIndexRel(feature)
	canonical, err := EncodeIntentArchiveIndex(next)
	if err != nil {
		return current, mutation, err
	}
	if beforePurgeIndexCAS != nil {
		beforePurgeIndexCAS(indexRel)
	}
	mutation, err = storage.CASIndex(indexRel, current.Identity, canonical)
	if mutation.Committed && afterPurgeIndexRename != nil {
		afterPurgeIndexRename(indexRel)
	}
	if err != nil {
		typed := intentArchiveStorageError(err, "cas-index", mutation.Committed, intentArchiveExitAfterMutation(mutation.Committed))
		if !mutation.Committed && !typed.Committed && typed.ExitClass == 5 {
			typed.ExitClass = 3
		}
		if typed.Code == IntentArchiveCodeStorageFailed {
			typed.Code = IntentArchiveCodePurgeIndexChanged
		}
		return current, mutation, typed
	}
	if !mutation.Committed {
		return current, mutation, intentArchiveError(IntentArchiveCodePurgeIndexChanged, "index CAS returned without a committed publication", 3)
	}
	archiveRel, _ := IntentArchiveRootRel(feature)
	if err := storage.SyncDirectory(archiveRel); err != nil {
		return current, mutation, intentArchiveStorageError(err, "sync-archive-directory", true, 5)
	}
	captured, decoded, err := captureIntentArchiveIndexOnly(storage, feature)
	if err != nil {
		return captured, mutation, err
	}
	if !bytes.Equal(captured.Raw, canonical) {
		return captured, mutation, intentArchiveError(IntentArchiveCodePurgeIndexChanged, "index.json diverged after CAS publication", 5)
	}
	if err := ValidateIntentArchiveIndex(decoded, feature); err != nil {
		return captured, mutation, err
	}
	return captured, mutation, nil
}

func removeIntentArchiveBlob(storage IntentArchiveStorage, blobRel, hash string, identity IntentArchiveIdentityToken, authorized bool) (IntentArchiveMutationResult, error) {
	if !authorized {
		return IntentArchiveMutationResult{}, intentArchiveError(IntentArchiveCodePurgeIndexChanged, "blob removal lacks a validated index authorization", 3)
	}
	if beforePurgeBlobRemove != nil {
		beforePurgeBlobRemove(blobRel)
	}
	if beforeBlobRemove != nil {
		beforeBlobRemove(blobRel)
	}
	mutation, err := storage.RemoveBlob(blobRel, identity)
	if mutation.Committed && afterPurgeBlobRemove != nil {
		afterPurgeBlobRemove(blobRel)
	}
	if err == nil && !mutation.Committed {
		return mutation, intentArchiveError(IntentArchiveCodeStorageFailed, "blob removal returned without committed truth", 3)
	}
	return mutation, err
}

func intentArchiveUnidentifiablePurgeError(hash string, owned bool) *IntentArchiveError {
	if owned {
		err := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, "the owned blob is present but unidentifiable", 6)
		err.Hash = hash
		err.Committed = true
		return err
	}
	err := intentArchiveError(IntentArchiveCodeBlobCorrupt, "a selected blob is present but unidentifiable", 3)
	err.Hash = hash
	return err
}

func promoteIntentArchivePurgeDivergence(err error, hash string, committed bool) error {
	if !committed {
		return err
	}
	var typed *IntentArchiveError
	if !errors.As(err, &typed) {
		return err
	}
	switch typed.Code {
	case IntentArchiveCodeIndexCorrupt,
		IntentArchiveCodeVersionUnsupported,
		IntentArchiveCodeIndexForeign,
		IntentArchiveCodeIndexPathEscape,
		IntentArchiveCodeGenerationMismatch,
		IntentArchiveCodeGenerationCollision:
		divergent := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, "index.json stopped strict-decoding after the purge transaction began", 6)
		divergent.Hash = hash
		divergent.Committed = true
		return divergent
	default:
		return err
	}
}

func normalizeIntentArchivePostMutationIndexError(
	storage IntentArchiveStorage,
	feature, hash string,
	expected IntentArchiveIndexCapture,
	execution *intentArchiveHashExecution,
	err error,
	committed, mutationCommitted bool,
) error {
	promoted := promoteIntentArchivePurgeDivergence(err, hash, committed)
	var typed *IntentArchiveError
	if !committed ||
		!errors.As(promoted, &typed) ||
		typed.Code == IntentArchiveCodePurgeEvidenceDivergent ||
		(typed.Code != IntentArchiveCodePurgeIndexChanged && typed.Code != IntentArchiveCodeIndexChanged) {
		return promoted
	}
	latestCapture, latestIndex, captureErr := captureIntentArchiveIndexOnly(storage, feature)
	execution.Capture = latestCapture
	if captureErr != nil {
		return promoteIntentArchivePurgeDivergence(captureErr, hash, true)
	}
	intentArchiveSetHashExecutionEvidence(execution, latestIndex, hash)
	if mutationCommitted || !latestCapture.Equal(expected) {
		return intentArchiveCommittedIndexChangeError(latestIndex, hash, "index.json identity diverged after a purge mutation", execution.OwnedRecovery)
	}
	return promoted
}

func intentArchiveCommittedIndexChangeError(index IntentArchiveIndex, hash, detail string, ownedRecovery bool) *IntentArchiveError {
	if intentArchiveHashHasState(index, hash, IntentArchiveWireRemovalPending) {
		partial := intentArchiveError(IntentArchiveCodePurgePartial, detail, 5)
		partial.Hash = hash
		partial.Committed = true
		return partial
	}
	if ownedRecovery {
		return intentArchivePurgeEvidenceDivergent(hash, detail+"; the owned hash no longer has pending references")
	}
	changed := intentArchiveError(IntentArchiveCodePurgeIndexChanged, detail, 5)
	changed.Hash = hash
	changed.Committed = true
	return changed
}

func intentArchivePurgeEvidenceDivergent(hash, detail string) *IntentArchiveError {
	divergent := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, detail, 6)
	divergent.Hash = hash
	divergent.Committed = true
	return divergent
}

func intentArchiveSetHashExecutionEvidence(execution *intentArchiveHashExecution, index IntentArchiveIndex, hash string) {
	execution.Pending = intentArchiveHashHasState(index, hash, IntentArchiveWireRemovalPending)
	execution.Completed = intentArchiveHashAllTombstoned(index, hash)
}

func intentArchiveSetDivergentProgress(result *IntentArchivePurgeResult, execution intentArchiveHashExecution, hash string, later []string) {
	result.Outcome = IntentArchivePurgePartial
	result.State = ""
	if execution.Pending {
		result.PendingHash = hash
		result.Resume = IntentArchiveResumePendingRecoveryThenCompletion
		result.RemainingHashes = append([]string(nil), later...)
	} else {
		result.PendingHash = ""
		result.Resume = IntentArchiveResumeCompletionOnly
		result.RemainingHashes = append([]string{hash}, later...)
		if execution.Completed {
			result.RemainingHashes = append([]string(nil), later...)
		}
	}
	result.CompletedHashes = sortedUniqueStrings(result.CompletedHashes)
	result.FinalizedHashes = sortedUniqueStrings(result.FinalizedHashes)
	result.RemainingHashes = sortedUniqueStrings(result.RemainingHashes)
	result.RemovedBlobs = sortedUniqueStrings(result.RemovedBlobs)
}

func setIntentArchiveHashState(index IntentArchiveIndex, hash string, state IntentArchiveWireState) (IntentArchiveIndex, bool) {
	next := cloneIntentArchiveIndex(index)
	changed := false
	for generationIndex := range next.Generations {
		for replacementIndex := range next.Generations[generationIndex].Replaced {
			replacement := &next.Generations[generationIndex].Replaced[replacementIndex]
			if replacement.ContentSHA256 != hash || replacement.WireState() == state {
				continue
			}
			setIntentArchiveReplacementState(replacement, state)
			changed = true
		}
	}
	return next, changed
}

func allIntentArchiveReferencesPending(index IntentArchiveIndex, hash string) bool {
	found := false
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 != hash {
				continue
			}
			found = true
			if replacement.WireState() != IntentArchiveWireRemovalPending {
				return false
			}
		}
	}
	return found
}

func intentArchiveHashUnreferenced(index IntentArchiveIndex, hash string) bool {
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == hash &&
				(replacement.WireState() == IntentArchiveWireRetained ||
					replacement.WireState() == IntentArchiveWireRemovalPending) {
				return false
			}
		}
	}
	return true
}

func intentArchiveReferencesForHash(index IntentArchiveIndex, hash string) []IntentArchiveReferenceTarget {
	references := make([]IntentArchiveReferenceTarget, 0)
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 != hash {
				continue
			}
			references = append(references, IntentArchiveReferenceTarget{
				GenerationID: generation.GenerationID,
				ArtifactID:   replacement.ArtifactID,
				Hash:         hash,
				Path:         replacement.Path,
				WireState:    replacement.WireState(),
			})
		}
	}
	return references
}

func intentArchiveExpectedSize(index IntentArchiveIndex, hash string) (int64, bool) {
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == hash {
				return replacement.SizeBytes, true
			}
		}
	}
	return -1, false
}

func intentArchiveMustExpectedSize(index IntentArchiveIndex, hash string) int64 {
	size, ok := intentArchiveExpectedSize(index, hash)
	if !ok {
		return -1
	}
	return size
}

func intentArchiveHashHasState(index IntentArchiveIndex, hash string, state IntentArchiveWireState) bool {
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == hash && replacement.WireState() == state {
				return true
			}
		}
	}
	return false
}

func intentArchiveHashAllTombstoned(index IntentArchiveIndex, hash string) bool {
	found := false
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 != hash {
				continue
			}
			found = true
			if replacement.WireState() != IntentArchiveWireTombstoned {
				return false
			}
		}
	}
	return found
}

func intentArchiveStringsAfter(values []string, value string) []string {
	for index, candidate := range values {
		if candidate == value {
			return append([]string(nil), values[index+1:]...)
		}
	}
	return []string{}
}

func intentArchiveStringsExcept(values []string, value string) []string {
	filtered := make([]string, 0, len(values))
	for _, candidate := range values {
		if candidate != value {
			filtered = append(filtered, candidate)
		}
	}
	return sortedUniqueStrings(filtered)
}

func equalIntentArchiveBlobRemovalSets(left, right []IntentArchiveBlobObservation) bool {
	if len(left) != len(right) {
		return false
	}
	leftPaths := make([]string, 0, len(left))
	rightPaths := make([]string, 0, len(right))
	for _, observation := range left {
		leftPaths = append(leftPaths, observation.Hash+"\x00"+observation.Path+"\x00"+string(observation.Identity))
	}
	for _, observation := range right {
		rightPaths = append(rightPaths, observation.Hash+"\x00"+observation.Path+"\x00"+string(observation.Identity))
	}
	sort.Strings(leftPaths)
	sort.Strings(rightPaths)
	return equalStringSets(leftPaths, rightPaths)
}

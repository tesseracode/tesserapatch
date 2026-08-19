package intentpub

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

func BuildJournal(plan Plan, runNonce string) (Journal, error) {
	if !validHex(runNonce, 16) {
		return Journal{}, transactionError(CodeJournalCorrupt, "", "run-nonce", "the run nonce must be 16 lowercase hexadecimal characters", 6)
	}
	if err := validatePlanShape(plan.slug, plan.mode, plan.stageRel, plan.entries); err != nil {
		return Journal{}, err
	}
	digest, err := PlanDigest(plan.entries)
	if err != nil {
		return Journal{}, err
	}
	if plan.planDigest != "" && plan.planDigest != digest {
		return Journal{}, transactionError(CodeJournalForged, "", "plan-digest", "the frozen plan digest does not match its entries", 6)
	}
	tempSuffixes := make(map[string]ArtifactID, len(plan.entries))
	for _, entry := range plan.entries {
		suffix := canonicalTempSuffix(runNonce, entry)
		if _, exists := tempSuffixes[suffix]; exists {
			return Journal{}, transactionError(CodeInvalidPlan, entry.ArtifactID, "temp-collision", "journal-bound canonical temporary names collide with another entry", 5)
		}
		tempSuffixes[suffix] = entry.ArtifactID
	}
	return Journal{
		Version:    JournalVersion,
		Slug:       plan.slug,
		Mode:       plan.mode,
		RunNonce:   runNonce,
		PlanDigest: digest,
		StageRel:   plan.stageRel,
		Entries:    cloneEntries(plan.entries),
	}, nil
}

func EncodeJournal(journal Journal) ([]byte, error) {
	if journal.Entries == nil || len(journal.Entries) == 0 {
		return nil, transactionError(CodeJournalCorrupt, "", "encode-entries", "the transaction journal cannot encode null or empty entries", 6)
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return nil, transactionError(CodeJournalCorrupt, "", "encode", "the transaction journal could not be encoded", 6)
	}
	return append(data, '\n'), nil
}

type journalValueKind uint8

const (
	journalString journalValueKind = iota + 1
	journalInteger
	journalBoolean
	journalIdentity
	journalEntries
)

type journalField struct {
	name     string
	kind     journalValueKind
	required bool
}

var journalTopFields = [...]journalField{
	{name: "version", kind: journalInteger, required: true},
	{name: "slug", kind: journalString, required: true},
	{name: "mode", kind: journalString, required: true},
	{name: "run_nonce", kind: journalString, required: true},
	{name: "plan_digest", kind: journalString, required: true},
	{name: "stage_rel", kind: journalString, required: true},
	{name: "entries", kind: journalEntries, required: true},
}

var journalEntryFields = [...]journalField{
	{name: "artifact_id", kind: journalString, required: true},
	{name: "rel", kind: journalString, required: true},
	{name: "action", kind: journalString, required: true},
	{name: "preimage", kind: journalIdentity, required: true},
	{name: "preimage_blob", kind: journalString},
	{name: "preimage_blob_rel", kind: journalString},
	{name: "preimage_raw_rel", kind: journalString},
	{name: "new_image", kind: journalIdentity, required: true},
	{name: "staged_rel", kind: journalString, required: true},
}

var journalIdentityFields = [...]journalField{
	{name: "exists", kind: journalBoolean, required: true},
	{name: "sha256", kind: journalString, required: true},
	{name: "size", kind: journalInteger, required: true},
	{name: "mode", kind: journalInteger, required: true},
}

// DecodeJournal performs the exact-key structural validation and then J1-J10
// before returning any recovery-capable data.
func DecodeJournal(data []byte, expectedSlug string) (Journal, error) {
	if err := validateJournalTokens(data); err != nil {
		return Journal{}, journalBindError(CodeJournalCorrupt, "json-schema")
	}
	var journal Journal
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&journal); err != nil {
		return Journal{}, journalBindError(CodeJournalCorrupt, "json-shape")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Journal{}, journalBindError(CodeJournalCorrupt, "trailing-content")
	}
	if journal.Version != JournalVersion {
		return Journal{}, journalBindError(CodeJournalVersionMismatch, "version")
	}
	if journal.Slug != expectedSlug {
		return Journal{}, journalBindError(CodeJournalForeign, "slug")
	}
	if journal.Mode != ModeGenerate && journal.Mode != ModeRegenerate {
		return Journal{}, journalBindError(CodeJournalCorrupt, "mode")
	}
	if !validHex(journal.RunNonce, 16) {
		return Journal{}, journalBindError(CodeJournalCorrupt, "run-nonce")
	}
	if journal.Entries == nil || len(journal.Entries) == 0 {
		return Journal{}, journalBindError(CodeJournalCorrupt, "entries")
	}
	if !validStageRel(journal.Slug, journal.StageRel) {
		return Journal{}, journalBindError(CodeJournalPathEscape, "stage-path")
	}
	for _, entry := range journal.Entries {
		if !validRootRel(entry.Rel) || !contained(featureRel(journal.Slug), entry.Rel) ||
			!validRootRel(entry.StagedRel) || !contained(journal.StageRel, entry.StagedRel) {
			return Journal{}, journalBindError(CodeJournalPathEscape, "entry-path")
		}
		if entry.PreimageRawRel != "" &&
			(!validRootRel(entry.PreimageRawRel) || !contained(laneRel(journal.Slug), entry.PreimageRawRel)) {
			return Journal{}, journalBindError(CodeJournalPathEscape, "raw-preimage-path")
		}
		if entry.PreimageBlobRel != "" &&
			(!validRootRel(entry.PreimageBlobRel) || !contained(featureRel(journal.Slug), entry.PreimageBlobRel)) {
			return Journal{}, journalBindError(CodeJournalPathEscape, "archive-preimage-path")
		}
	}
	if err := validatePlanShape(journal.Slug, journal.Mode, journal.StageRel, journal.Entries); err != nil {
		return Journal{}, journalBindError(CodeJournalCorrupt, "entry-shape")
	}
	digest, err := PlanDigest(journal.Entries)
	if err != nil || journal.PlanDigest != digest || !validHash(journal.PlanDigest) {
		return Journal{}, journalBindError(CodeJournalForged, "plan-digest")
	}
	return journal, nil
}

func validateJournalTokens(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return errors.New("journal is not an object")
	}
	if err := validateJSONObjectSchema(decoder, journalTopFields[:]); err != nil {
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

func validateJSONObjectSchema(decoder *json.Decoder, schema []journalField) error {
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
		index, alias := journalFieldIndex(schema, key)
		if alias || index < 0 {
			return errors.New("unknown or case-aliased object key")
		}
		folded := strings.ToLower(key)
		if seen[index] {
			return errors.New("duplicate object key")
		}
		if _, duplicateAlias := seenFolded[folded]; duplicateAlias {
			return errors.New("duplicate case-aliased object key")
		}
		seen[index] = true
		seenFolded[folded] = struct{}{}
		value, err := decoder.Token()
		if err != nil {
			return err
		}
		if err := validateJournalValue(decoder, value, schema[index].kind); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return errors.New("unterminated object")
	}
	for index, field := range schema {
		if field.required && !seen[index] {
			return errors.New("missing object key")
		}
	}
	return nil
}

func journalFieldIndex(schema []journalField, key string) (index int, alias bool) {
	for index, field := range schema {
		if key == field.name {
			return index, false
		}
	}
	for _, field := range schema {
		if strings.EqualFold(key, field.name) {
			return -1, true
		}
	}
	return -1, false
}

func validateJournalValue(decoder *json.Decoder, token json.Token, kind journalValueKind) error {
	switch kind {
	case journalString:
		if _, ok := token.(string); !ok {
			return errors.New("expected string")
		}
	case journalInteger:
		number, ok := token.(json.Number)
		if !ok {
			return errors.New("expected integer")
		}
		if _, err := strconv.ParseInt(string(number), 10, 64); err != nil {
			return errors.New("expected integer")
		}
	case journalBoolean:
		if _, ok := token.(bool); !ok {
			return errors.New("expected boolean")
		}
	case journalIdentity:
		if token != json.Delim('{') {
			return errors.New("expected identity object")
		}
		return validateJSONObjectSchema(decoder, journalIdentityFields[:])
	case journalEntries:
		if token != json.Delim('[') {
			return errors.New("expected entries array")
		}
		count := 0
		for decoder.More() {
			entryToken, err := decoder.Token()
			if err != nil || entryToken != json.Delim('{') {
				return errors.New("expected non-null entry object")
			}
			if err := validateJSONObjectSchema(decoder, journalEntryFields[:]); err != nil {
				return err
			}
			count++
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') || count == 0 {
			return errors.New("entries must be a non-empty array")
		}
	default:
		return errors.New("unknown journal schema kind")
	}
	return nil
}

func PlanFromJournal(journal Journal) (Plan, error) {
	plan, err := NewPlan(journal.Slug, journal.Mode, journal.StageRel, journal.Entries)
	if err != nil {
		return Plan{}, err
	}
	if plan.Digest() != journal.PlanDigest {
		return Plan{}, journalBindError(CodeJournalForged, "plan-digest")
	}
	return plan, nil
}

func JournalRel(slug string) string {
	return laneRel(slug) + "/journal.json"
}

func JournalMarkerRel(slug string) string {
	return laneRel(slug) + "/journal.clearing.json"
}

func journalBindError(code Code, class string) *Error {
	detail := "the transaction journal failed strict binding"
	if code == CodeJournalVersionMismatch {
		detail = "the transaction journal was written by another build"
	}
	return transactionError(code, "", class, detail, 6)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("additional JSON value")
	}
	return err
}

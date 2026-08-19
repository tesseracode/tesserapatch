package intentpub

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPlanDigestDeterministicAndComplete(t *testing.T) {
	entries := []Entry{
		planFixtureEntry(t, ArtifactAnalysis, ActionReplace, "digest"),
		planFixtureEntry(t, ArtifactSpec, ActionReplace, "digest"),
		planFixtureEntry(t, ArtifactExploration, ActionReplace, "digest"),
		planFixtureEntry(t, ArtifactAnalysisSidecar, ActionReplace, "digest"),
		planFixtureEntry(t, ArtifactArchiveIndex, ActionReplace, "digest"),
		planFixtureEntry(t, ArtifactStatus, ActionReplace, "digest"),
	}
	first, err := CanonicalPlanEncoding(entries)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalPlanEncoding(entries)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || first[0] != '[' || bytes.Contains(first, []byte("null")) {
		t.Fatalf("canonical encoding is not a stable non-null array: %s", first)
	}
	if bytes.Contains(first, []byte("{\"")) && bytes.Contains(first, []byte("map[")) {
		t.Fatalf("canonical encoding unexpectedly contains a map rendering: %s", first)
	}

	baseDigest, err := PlanDigest(entries)
	if err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		mutate func(*Entry)
	}{
		{"artifact-id", func(entry *Entry) { entry.ArtifactID += "-changed" }},
		{"canonical-rel", func(entry *Entry) { entry.Rel += "-changed" }},
		{"action", func(entry *Entry) {
			if entry.Action == ActionCreate {
				entry.Action = ActionReplace
			} else {
				entry.Action = ActionCreate
			}
		}},
		{"preimage-exists", func(entry *Entry) { entry.Preimage.Exists = !entry.Preimage.Exists }},
		{"preimage-sha256", func(entry *Entry) { entry.Preimage.SHA256 = alternateHash(entry.Preimage.SHA256, "a") }},
		{"preimage-size", func(entry *Entry) { entry.Preimage.Size++ }},
		{"preimage-mode", func(entry *Entry) { entry.Preimage.Mode ^= 1 }},
		{"new-image-exists", func(entry *Entry) { entry.NewImage.Exists = !entry.NewImage.Exists }},
		{"new-image-sha256", func(entry *Entry) { entry.NewImage.SHA256 = alternateHash(entry.NewImage.SHA256, "b") }},
		{"new-image-size", func(entry *Entry) { entry.NewImage.Size++ }},
		{"new-image-mode", func(entry *Entry) { entry.NewImage.Mode ^= 1 }},
		{"preimage-blob", func(entry *Entry) { entry.PreimageBlob += "-changed" }},
		{"preimage-blob-rel", func(entry *Entry) { entry.PreimageBlobRel += "-changed" }},
		{"preimage-raw-rel", func(entry *Entry) { entry.PreimageRawRel += "-changed" }},
		{"staged-rel", func(entry *Entry) { entry.StagedRel += "-changed" }},
	}
	for _, mutation := range mutations {
		for index := range entries {
			name := fmt.Sprintf("%s-entry-%d", mutation.name, index)
			t.Run(name, func(t *testing.T) {
				changed := cloneEntries(entries)
				mutation.mutate(&changed[index])
				assertCanonicalPlanMutation(t, first, baseDigest, changed)
			})
		}
	}
	for index := range entries {
		t.Run(fmt.Sprintf("remove-entry-%d", index), func(t *testing.T) {
			changed := append(cloneEntries(entries[:index]), entries[index+1:]...)
			assertCanonicalPlanMutation(t, first, baseDigest, changed)
		})
	}
	for index := 0; index+1 < len(entries); index++ {
		t.Run(fmt.Sprintf("swap-entries-%d-%d", index, index+1), func(t *testing.T) {
			changed := cloneEntries(entries)
			changed[index], changed[index+1] = changed[index+1], changed[index]
			assertCanonicalPlanMutation(t, first, baseDigest, changed)
		})
	}
	t.Run("append-entry", func(t *testing.T) {
		appended := entries[len(entries)-1]
		appended.ArtifactID += "-appended"
		assertCanonicalPlanMutation(t, first, baseDigest, append(cloneEntries(entries), appended))
	})
	t.Run("duplicate-entry", func(t *testing.T) {
		assertCanonicalPlanMutation(t, first, baseDigest, append(cloneEntries(entries), entries[0]))
	})
}

func alternateHash(current, seed string) string {
	candidate := strings.Repeat(seed, 64)
	if candidate != current {
		return candidate
	}
	return strings.Repeat("f", 64)
}

func assertCanonicalPlanMutation(t *testing.T, baseEncoding []byte, baseDigest string, changed []Entry) {
	t.Helper()
	encoded, err := CanonicalPlanEncoding(changed)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encoded, baseEncoding) {
		t.Fatal("mutation did not change canonical plan bytes")
	}
	digest, err := PlanDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if digest == baseDigest {
		t.Fatal("mutation was not covered by plan_digest")
	}
}

func TestPlanRejectsOrderDuplicatesAndModeIncoherence(t *testing.T) {
	valid := testCreatePlan(t).Entries()
	tests := []struct {
		name    string
		mode    Mode
		entries []Entry
	}{
		{"duplicate", ModeGenerate, append(cloneEntries(valid[:1]), valid[0])},
		{"out-of-order", ModeGenerate, []Entry{valid[1], valid[0], valid[len(valid)-1]}},
		{"status-not-last", ModeGenerate, []Entry{valid[len(valid)-1], valid[0]}},
		{"manual", ModeManual, valid},
		{"generate-replace", ModeGenerate, replaceEntry(valid)},
		{"analysis-without-sidecar", ModeGenerate, []Entry{valid[0], valid[len(valid)-1]}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewPlan(testSlug, test.mode, testStageRel, test.entries); err == nil {
				t.Fatal("invalid plan was accepted")
			}
		})
	}
}

func TestJournalBoundCanonicalTempSuffixesAreStableAndDistinct(t *testing.T) {
	entries := []Entry{
		planFixtureEntry(t, ArtifactAnalysis, ActionReplace, "temps"),
		planFixtureEntry(t, ArtifactSpec, ActionReplace, "temps"),
		planFixtureEntry(t, ArtifactExploration, ActionReplace, "temps"),
		planFixtureEntry(t, ArtifactAnalysisSidecar, ActionReplace, "temps"),
		planFixtureEntry(t, ArtifactArchiveIndex, ActionReplace, "temps"),
		planFixtureEntry(t, ArtifactStatus, ActionReplace, "temps"),
	}
	seen := make(map[string]ArtifactID, len(entries))
	for _, entry := range entries {
		suffix := canonicalTempSuffix("0123456789abcdef", entry)
		if !validHex(suffix, 12) || suffix != canonicalTempSuffix("0123456789abcdef", entry) {
			t.Fatalf("%s suffix is not stable 12-hex: %q", entry.ArtifactID, suffix)
		}
		if other, exists := seen[suffix]; exists {
			t.Fatalf("%s and %s share suffix %q", other, entry.ArtifactID, suffix)
		}
		seen[suffix] = entry.ArtifactID
		if suffix == canonicalTempSuffix("fedcba9876543210", entry) {
			t.Fatalf("%s suffix is not bound to the run nonce", entry.ArtifactID)
		}
	}
}

func TestDecodeJournalJ1ThroughJ10(t *testing.T) {
	plan := testCreatePlan(t)
	journal, err := BuildJournal(plan, "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	valid, err := EncodeJournal(journal)
	if err != nil {
		t.Fatal(err)
	}
	entryUnknown := bytes.Replace(valid, []byte(`"artifact_id":`), []byte(`"mystery": 1, "artifact_id":`), 1)
	identityUnknown := bytes.Replace(valid, []byte(`"exists":`), []byte(`"mystery": 1, "exists":`), 1)
	duplicateKey := bytes.Replace(valid, []byte(`"version": 1`), []byte(`"version": 1, "version": 1`), 1)
	nullEntries := replaceJSONField(t, valid, "entries", nil)
	emptyEntries := replaceJSONField(t, valid, "entries", []any{})
	wrongVersion := replaceJSONField(t, valid, "version", float64(2))
	foreignSlug := replaceJSONField(t, valid, "slug", "other")
	manualMode := replaceJSONField(t, valid, "mode", "manual")
	badNonce := replaceJSONField(t, valid, "run_nonce", "ABCDEF0123456789")
	pathEscape := replaceEntryField(t, valid, 0, "rel", "../outside")
	duplicateID := replaceEntryField(t, valid, 1, "artifact_id", "analysis")
	forged := replaceEntryIdentityHash(t, valid, 0, strings.Repeat("a", 64))
	nullIdentity := replaceEntryField(t, valid, 0, "preimage", nil)

	tests := []struct {
		name string
		bind string
		data []byte
		code Code
	}{
		{"valid", "", valid, ""},
		{"trailing-value", "J1", append(append([]byte(nil), valid...), []byte(`{}`)...), CodeJournalCorrupt},
		{"unknown-top", "J2", bytes.Replace(valid, []byte(`"version":`), []byte(`"unknown": 1, "version":`), 1), CodeJournalCorrupt},
		{"unknown-entry", "", entryUnknown, CodeJournalCorrupt},
		{"unknown-identity", "", identityUnknown, CodeJournalCorrupt},
		{"duplicate-key", "", duplicateKey, CodeJournalCorrupt},
		{"version", "J3", wrongVersion, CodeJournalVersionMismatch},
		{"foreign", "J4", foreignSlug, CodeJournalForeign},
		{"manual", "J5", manualMode, CodeJournalCorrupt},
		{"nonce", "J6", badNonce, CodeJournalCorrupt},
		{"path", "J7", pathEscape, CodeJournalPathEscape},
		{"forged", "J8", forged, CodeJournalForged},
		{"duplicate-id", "J9", duplicateID, CodeJournalCorrupt},
		{"null-entries", "J10", nullEntries, CodeJournalCorrupt},
		{"empty-entries", "", emptyEntries, CodeJournalCorrupt},
		{"null-identity", "", nullIdentity, CodeJournalCorrupt},
	}
	covered := make(map[string]bool)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeJournal(test.data, testSlug)
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var typed *Error
			if !errors.As(err, &typed) || typed.Code != test.code {
				t.Fatalf("error = %#v, want code %s", err, test.code)
			}
		})
		if test.bind != "" {
			covered[test.bind] = true
		}
	}
	for index := 1; index <= 10; index++ {
		bind := fmt.Sprintf("J%d", index)
		if !covered[bind] {
			t.Fatalf("%s has no rejecting sensitivity fixture", bind)
		}
	}
}

func TestJournalExactSchemaKeysTypesAndNulls(t *testing.T) {
	journal, err := BuildJournal(testCreatePlan(t), "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	valid, err := EncodeJournal(journal)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
	}{
		{"top-case-alias", bytes.Replace(valid, []byte(`"version"`), []byte(`"VERSION"`), 1)},
		{"top-duplicate-alias", bytes.Replace(valid, []byte(`"version": 1`), []byte(`"version": 1, "VERSION": 1`), 1)},
		{"entry-case-alias", bytes.Replace(valid, []byte(`"artifact_id"`), []byte(`"ARTIFACT_ID"`), 1)},
		{"entry-duplicate-alias", bytes.Replace(valid, []byte(`"artifact_id": "analysis"`), []byte(`"artifact_id": "analysis", "ARTIFACT_ID": "analysis"`), 1)},
		{"identity-case-alias", bytes.Replace(valid, []byte(`"exists"`), []byte(`"EXISTS"`), 1)},
		{"identity-duplicate-alias", bytes.Replace(valid, []byte(`"exists": false`), []byte(`"exists": false, "EXISTS": false`), 1)},
		{"missing-top", removeJSONField(t, valid, "plan_digest")},
		{"missing-entry", removeEntryField(t, valid, 0, "action")},
		{"missing-identity", removeIdentityField(t, valid, 0, "preimage", "mode")},
		{"wrong-top-type", replaceJSONField(t, valid, "version", "1")},
		{"wrong-entries-type", replaceJSONField(t, valid, "entries", map[string]any{})},
		{"null-entry", replaceEntriesValue(t, valid, 0, nil)},
		{"wrong-entry-type", replaceEntriesValue(t, valid, 0, "entry")},
		{"wrong-identity-type", replaceEntryField(t, valid, 0, "preimage", []any{})},
		{"null-identity", replaceEntryField(t, valid, 0, "preimage", nil)},
		{"wrong-bool", replaceIdentityField(t, valid, 0, "preimage", "exists", "false")},
		{"fractional-size", replaceIdentityField(t, valid, 0, "new_image", "size", 1.5)},
		{"negative-mode", replaceIdentityField(t, valid, 0, "new_image", "mode", -1)},
		{"null-optional-string", replaceEntryField(t, valid, 0, "preimage_blob", nil)},
		{"trailing-scalar", append(append([]byte(nil), valid...), []byte(` true`)...)}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeJournal(test.data, testSlug)
			assertCode(t, err, CodeJournalCorrupt)
		})
	}
}

func TestJournalWireHasNoForbiddenTransactionFields(t *testing.T) {
	journal, err := BuildJournal(testCreatePlan(t), "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	data, err := EncodeJournal(journal)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`"created_at"`, `"timestamp"`, `"phase"`, `"provider"`,
		`"provenance"`, `"content"`, `"prompt"`, `"transcript"`,
	} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatalf("journal contains forbidden field %s: %s", forbidden, data)
		}
	}
	if bytes.Contains(data, []byte(`"entries": null`)) {
		t.Fatalf("journal entries encoded as null: %s", data)
	}
}

func replaceJSONField(t *testing.T, data []byte, key string, value any) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object[key] = value
	return marshalTestJSON(t, object)
}

func replaceEntryField(t *testing.T, data []byte, index int, key string, value any) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	entries := object["entries"].([]any)
	entry := entries[index].(map[string]any)
	entry[key] = value
	return marshalTestJSON(t, object)
}

func replaceEntryIdentityHash(t *testing.T, data []byte, index int, hash string) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	entry := object["entries"].([]any)[index].(map[string]any)
	identity := entry["new_image"].(map[string]any)
	identity["sha256"] = hash
	return marshalTestJSON(t, object)
}

func removeJSONField(t *testing.T, data []byte, key string) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	delete(object, key)
	return marshalTestJSON(t, object)
}

func removeEntryField(t *testing.T, data []byte, index int, key string) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	entry := object["entries"].([]any)[index].(map[string]any)
	delete(entry, key)
	return marshalTestJSON(t, object)
}

func removeIdentityField(t *testing.T, data []byte, index int, identityKey, key string) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	identity := object["entries"].([]any)[index].(map[string]any)[identityKey].(map[string]any)
	delete(identity, key)
	return marshalTestJSON(t, object)
}

func replaceIdentityField(t *testing.T, data []byte, index int, identityKey, key string, value any) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	identity := object["entries"].([]any)[index].(map[string]any)[identityKey].(map[string]any)
	identity[key] = value
	return marshalTestJSON(t, object)
}

func replaceEntriesValue(t *testing.T, data []byte, index int, value any) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object["entries"].([]any)[index] = value
	return marshalTestJSON(t, object)
}

func marshalTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func replaceEntry(entries []Entry) []Entry {
	out := cloneEntries(entries)
	out[0].Action = ActionReplace
	out[0].Preimage = out[0].NewImage
	out[0].PreimageBlob = out[0].Preimage.SHA256
	out[0].PreimageBlobRel = featureRel(testSlug) + "/artifacts/intent-archive/blobs/" + out[0].Preimage.SHA256 + ".blob"
	return out
}

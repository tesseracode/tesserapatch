package store

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestIntentArchiveGenerationGoldenAndCanonicalWire(t *testing.T) {
	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "old analysis", IntentArchiveWireRetained)
	id, body, err := ComputeIntentArchiveGenerationID("demo", []IntentArchiveReplacement{replacement})
	if err != nil {
		t.Fatal(err)
	}
	const wantBody = `{"feature":"demo","mode":"regenerate","replaced":[{"artifact_id":"analysis","path":"analysis.md","content_sha256":"6d674626ff8c44c6d8a1cc6d92601b82e387cc9af2bf67a5ccb8d23ff7949e84","size_bytes":12}]}`
	const wantID = "90ada070441a57d4252035fef2c1c42bc42cc6004a3e5b8a081597355b33735f"
	if string(body) != wantBody {
		t.Fatalf("body = %s\nwant   %s", body, wantBody)
	}
	if id != wantID {
		t.Fatalf("id = %s, want %s", id, wantID)
	}
	generation := archiveGeneration(t, "demo",
		archiveReplacement(t, IntentArchiveArtifactSpec, "spec", IntentArchiveWireRetained),
		replacement,
		archiveReplacement(t, IntentArchiveArtifactAnalysisSidecar, "{}", IntentArchiveWireRetained),
	)
	index := archiveIndex(t, "demo", generation)
	wire, err := EncodeIntentArchiveIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(wire, []byte("}\n")) {
		t.Fatal("canonical index must end in one newline")
	}
	text := string(wire)
	if strings.Count(text, `"feature"`) != 1 {
		t.Fatalf("feature must appear only at the top-level index:\n%s", text)
	}
	keys := []string{
		`"schema_version"`, `"feature"`, `"generations"`,
		`"generation_id"`, `"mode"`, `"replaced"`,
		`"artifact_id"`, `"path"`, `"content_sha256"`, `"blob"`,
		`"size_bytes"`, `"purged"`, `"purge_pending"`,
	}
	cursor := 0
	for _, key := range keys {
		next := strings.Index(text[cursor:], key)
		if next < 0 {
			t.Fatalf("canonical wire does not contain %s after byte %d", key, cursor)
		}
		cursor += next + len(key)
	}
	if strings.Contains(text, "timestamp") ||
		strings.Contains(text, "created_at") ||
		strings.Contains(text, "provider") ||
		strings.Contains(text, "provenance") ||
		strings.Contains(text, "generator") {
		t.Fatalf("forbidden tracked field in canonical wire:\n%s", text)
	}
	decoded, err := DecodeIntentArchiveIndex(wire, "demo")
	if err != nil {
		t.Fatal(err)
	}
	again, err := EncodeIntentArchiveIndex(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire, again) {
		t.Fatal("decode/encode did not round-trip byte-identically")
	}
	if decoded.Generations == nil || decoded.Generations[0].Replaced == nil {
		t.Fatal("wire arrays decoded as nil")
	}
}

func TestIntentArchiveStrictDecoderSensitivity(t *testing.T) {
	generation := archiveGeneration(t, "demo",
		archiveReplacement(t, IntentArchiveArtifactAnalysis, "analysis", IntentArchiveWireRetained),
	)
	valid, err := EncodeIntentArchiveIndex(archiveIndex(t, "demo", generation))
	if err != nil {
		t.Fatal(err)
	}
	compact := func(value any) []byte {
		t.Helper()
		data, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return data
	}
	invalidWire := generation.Replaced[0]
	invalidWire.Blob = ""
	invalidWire.Purged = false
	invalidWire.PurgePending = false
	invalidGeneration := generation
	invalidGeneration.Replaced = []IntentArchiveReplacement{invalidWire}
	cases := []struct {
		name string
		data []byte
		code IntentArchiveErrorCode
	}{
		{"trailing-value", append(append([]byte(nil), valid...), []byte(`{}`)...), IntentArchiveCodeIndexCorrupt},
		{"unknown-top", []byte(strings.Replace(string(valid), `"feature": "demo",`, `"feature": "demo", "extra": 1,`, 1)), IntentArchiveCodeIndexCorrupt},
		{"unknown-generation", []byte(strings.Replace(string(valid), `"mode": "regenerate",`, `"mode": "regenerate", "extra": 1,`, 1)), IntentArchiveCodeIndexCorrupt},
		{"forbidden-generation-feature", []byte(strings.Replace(string(valid), `"mode": "regenerate",`, `"feature": "demo", "mode": "regenerate",`, 1)), IntentArchiveCodeIndexCorrupt},
		{"unknown-replacement", []byte(strings.Replace(string(valid), `"path": "analysis.md",`, `"path": "analysis.md", "extra": 1,`, 1)), IntentArchiveCodeIndexCorrupt},
		{"duplicate-top", []byte(strings.Replace(string(valid), `"schema_version": 1,`, `"schema_version": 1, "schema_version": 1,`, 1)), IntentArchiveCodeIndexCorrupt},
		{"duplicate-generation", []byte(strings.Replace(string(valid), `"mode": "regenerate",`, `"mode": "regenerate", "mode": "regenerate",`, 1)), IntentArchiveCodeIndexCorrupt},
		{"duplicate-replacement", []byte(strings.Replace(string(valid), `"purged": false,`, `"purged": false, "purged": false,`, 1)), IntentArchiveCodeIndexCorrupt},
		{"case-alias", []byte(strings.Replace(string(valid), `"schema_version": 1,`, `"Schema_Version": 1,`, 1)), IntentArchiveCodeIndexCorrupt},
		{"nested-case-alias", []byte(strings.Replace(string(valid), `"artifact_id": "analysis",`, `"Artifact_ID": "analysis",`, 1)), IntentArchiveCodeIndexCorrupt},
		{"null-string", []byte(strings.Replace(string(valid), `"feature": "demo"`, `"feature": null`, 1)), IntentArchiveCodeIndexCorrupt},
		{"null-generations", []byte(`{"schema_version":1,"feature":"demo","generations":null}`), IntentArchiveCodeIndexCorrupt},
		{"null-replaced", []byte(`{"schema_version":1,"feature":"demo","generations":[{"generation_id":"` + generation.GenerationID + `","mode":"regenerate","replaced":null}]}`), IntentArchiveCodeIndexCorrupt},
		{"null-generation", []byte(`{"schema_version":1,"feature":"demo","generations":[null]}`), IntentArchiveCodeIndexCorrupt},
		{"higher-version", []byte(strings.Replace(string(valid), `"schema_version": 1`, `"schema_version": 2`, 1)), IntentArchiveCodeVersionUnsupported},
		{"lower-version", []byte(strings.Replace(string(valid), `"schema_version": 1`, `"schema_version": 0`, 1)), IntentArchiveCodeIndexCorrupt},
		{"foreign-feature", []byte(strings.Replace(string(valid), `"feature": "demo"`, `"feature": "other"`, 1)), IntentArchiveCodeIndexForeign},
		{"invalid-wire-state", compact(IntentArchiveIndex{SchemaVersion: 1, Feature: "demo", Generations: []IntentArchiveGeneration{invalidGeneration}}), IntentArchiveCodeIndexCorrupt},
		{"negative-size", []byte(strings.Replace(string(valid), `"size_bytes": 8`, `"size_bytes": -1`, 1)), IntentArchiveCodeIndexCorrupt},
		{"path-escape", []byte(strings.Replace(string(valid), `"path": "analysis.md"`, `"path": "../analysis.md"`, 1)), IntentArchiveCodeIndexPathEscape},
		{"unknown-artifact", []byte(strings.Replace(string(valid), `"artifact_id": "analysis"`, `"artifact_id": "notes"`, 1)), IntentArchiveCodeIndexCorrupt},
		{"generation-mismatch", []byte(strings.Replace(string(valid), generation.GenerationID, strings.Repeat("a", 64), 1)), IntentArchiveCodeGenerationMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeIntentArchiveIndex(tc.data, "demo")
			assertArchiveCode(t, err, tc.code)
		})
	}
}

func TestIntentArchiveWireStatesAndOrdering(t *testing.T) {
	for _, state := range []IntentArchiveWireState{
		IntentArchiveWireRetained,
		IntentArchiveWireRemovalPending,
		IntentArchiveWireTombstoned,
	} {
		replacement := archiveReplacement(t, IntentArchiveArtifactSpec, "same", state)
		if replacement.WireState() != state {
			t.Fatalf("WireState() = %q, want %q", replacement.WireState(), state)
		}
		if err := validateIntentArchiveWireState(replacement); err != nil {
			t.Fatalf("valid state %q refused: %v", state, err)
		}
	}
	base := archiveReplacement(t, IntentArchiveArtifactSpec, "same", IntentArchiveWireRetained)
	invalid := []IntentArchiveReplacement{
		func() IntentArchiveReplacement { r := base; r.Blob = ""; return r }(),
		func() IntentArchiveReplacement { r := base; r.Purged = true; return r }(),
		func() IntentArchiveReplacement { r := base; r.Purged = true; r.PurgePending = true; return r }(),
		func() IntentArchiveReplacement {
			r := base
			r.Blob = ""
			r.Purged = true
			r.PurgePending = true
			return r
		}(),
	}
	for index, replacement := range invalid {
		if err := validateIntentArchiveWireState(replacement); err == nil {
			t.Fatalf("invalid wire state %d accepted: %+v", index, replacement)
		}
	}

	unsorted := []IntentArchiveReplacement{
		archiveReplacement(t, IntentArchiveArtifactSpec, "spec", IntentArchiveWireRetained),
		archiveReplacement(t, IntentArchiveArtifactAnalysis, "analysis", IntentArchiveWireRetained),
	}
	id, _, err := ComputeIntentArchiveGenerationID("demo", unsorted)
	if err != nil {
		t.Fatal(err)
	}
	index := IntentArchiveIndex{
		SchemaVersion: 1,
		Feature:       "demo",
		Generations: []IntentArchiveGeneration{{
			GenerationID: id,
			Mode:         IntentArchiveModeRegenerate,
			Replaced:     unsorted,
		}},
	}
	assertArchiveCode(t, ValidateIntentArchiveIndex(index, "demo"), IntentArchiveCodeIndexCorrupt)
}

func TestIntentArchiveSafeRelativePaths(t *testing.T) {
	for _, slug := range []string{"", "../demo", "/demo", "Demo", "a--b", "con"} {
		if _, err := IntentArchiveIndexRel(slug); err == nil {
			t.Fatalf("unsafe slug %q accepted", slug)
		}
	}
	indexRel, err := IntentArchiveIndexRel("safe-feature")
	if err != nil {
		t.Fatal(err)
	}
	if indexRel != ".tpatch/features/safe-feature/artifacts/intent-archive/index.json" {
		t.Fatalf("index rel = %q", indexRel)
	}
	if _, err := IntentArchiveBlobRel("safe-feature", strings.Repeat("A", 64)); err == nil {
		t.Fatal("uppercase blob hash accepted")
	}
	hash := strings.Repeat("a", 64)
	blobRel, err := IntentArchiveBlobRel("safe-feature", hash)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(blobRel, "/") || strings.Contains(blobRel, "..") {
		t.Fatalf("blob path is not safe repo-relative: %q", blobRel)
	}
}

package store

// Direct §18.53 sensitivity fixtures for the archive-wire rows.
//
// `TestIntentArchiveGenerationGoldenAndCanonicalWire` is a baseline-only body:
// it proves the shipped bytes are canonical but never mutates them, so it
// cannot serve as a wrong-input fixture. The two tests below drive the *same*
// shipped validators — the canonical encoder and the strict decoder — over the
// shipped wire and then over deliberately wrong versions of it.

import (
	"fmt"
	"strings"
	"testing"
)

// TestIntentArchiveCanonicalWireGuardAndSensitivity owns PIB-161: index.json
// carries no wall-clock field, its key order is fixed, and `replaced` is sorted
// by artifact_id.
func TestIntentArchiveCanonicalWireGuardAndSensitivity(t *testing.T) {
	analysis := archiveReplacement(t, IntentArchiveArtifactAnalysis, "analysis", IntentArchiveWireRetained)
	spec := archiveReplacement(t, IntentArchiveArtifactSpec, "spec", IntentArchiveWireRetained)
	generation := archiveGeneration(t, "demo", spec, analysis)
	index := archiveIndex(t, "demo", generation)
	wire, err := EncodeIntentArchiveIndex(index)
	if err != nil {
		t.Fatalf("PIB-161: the shipped index was not encodable: %v", err)
	}
	if err := validateIntentArchiveCanonicalWire(string(wire)); err != nil {
		t.Fatalf("PIB-161: the shipped canonical wire failed its own guard: %v", err)
	}

	// A wall-clock key injected into the canonical bytes must be rejected by the
	// same strict decoder that accepts the shipped bytes.
	clocked := strings.Replace(
		string(wire), `"feature": "demo",`, `"feature": "demo", "created_at": "2024-01-02T03:04:05Z",`, 1,
	)
	if clocked == string(wire) {
		t.Fatal("PIB-161: the wall-clock mutation anchor is missing")
	}
	if err := validateIntentArchiveCanonicalWire(clocked); err == nil {
		t.Fatal("PIB-161: the wire guard accepted a wall-clock field")
	}
	if _, err := DecodeIntentArchiveIndex([]byte(clocked), "demo"); err == nil {
		t.Fatal("PIB-161: the strict decoder accepted a wall-clock field")
	}

	// `replaced` out of artifact_id order must be rejected by the same encoder
	// that produced the shipped bytes.
	unsorted := generation
	unsorted.Replaced = []IntentArchiveReplacement{spec, analysis}
	misordered := IntentArchiveIndex{
		SchemaVersion: IntentArchiveSchemaVersion,
		Feature:       "demo",
		Generations:   []IntentArchiveGeneration{unsorted},
	}
	if _, err := EncodeIntentArchiveIndex(misordered); err == nil {
		t.Fatal("PIB-161: the canonical encoder accepted replaced entries out of artifact_id order")
	}
}

// validateIntentArchiveCanonicalWire is the wire-shape half of PIB-161: no
// wall-clock or provenance key, and the fixed key order of §9's schema.
func validateIntentArchiveCanonicalWire(text string) error {
	if text == "" {
		return fmt.Errorf("the canonical wire is empty")
	}
	for _, forbidden := range []string{
		`"created_at"`, `"timestamp"`, `"updated_at"`, `"provider"`, `"provenance"`, `"generator"`,
	} {
		if strings.Contains(text, forbidden) {
			return fmt.Errorf("the canonical wire carries %s", forbidden)
		}
	}
	cursor := 0
	for _, key := range []string{
		`"schema_version"`, `"feature"`, `"generations"`,
		`"generation_id"`, `"mode"`, `"replaced"`,
		`"artifact_id"`, `"path"`, `"content_sha256"`, `"blob"`,
		`"size_bytes"`, `"purged"`, `"purge_pending"`,
	} {
		next := strings.Index(text[cursor:], key)
		if next < 0 {
			return fmt.Errorf("the canonical key order no longer reaches %s", key)
		}
		cursor += next + len(key)
	}
	return nil
}

// TestIntentArchiveStrictDecoderInjectedKeySensitivity owns PIB-341: a valid
// index plus one injected unknown key must fail, and "decoding succeeded" is
// not enough to satisfy the guard.
func TestIntentArchiveStrictDecoderInjectedKeySensitivity(t *testing.T) {
	generation := archiveGeneration(t, "demo",
		archiveReplacement(t, IntentArchiveArtifactAnalysis, "analysis", IntentArchiveWireRetained),
	)
	valid, err := EncodeIntentArchiveIndex(archiveIndex(t, "demo", generation))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeIntentArchiveIndex(valid, "demo")
	if err != nil {
		t.Fatalf("PIB-341: the shipped index was rejected by its own strict decoder: %v", err)
	}
	if decoded.Feature != "demo" || len(decoded.Generations) != 1 {
		t.Fatalf("PIB-341: the strict decoder returned %#v", decoded)
	}
	injected := strings.Replace(
		string(valid), `"artifact_id": "analysis",`, `"artifact_id": "analysis", "author": "x",`, 1,
	)
	if injected == string(valid) {
		t.Fatal("PIB-341: the unknown-key mutation anchor is missing")
	}
	if _, err := DecodeIntentArchiveIndex([]byte(injected), "demo"); err == nil {
		t.Fatal("PIB-341: the strict decoder accepted one injected unknown key")
	}
	nested := strings.Replace(
		string(valid), `"mode": "regenerate",`, `"mode": "regenerate", "agent": "x",`, 1,
	)
	if nested == string(valid) {
		t.Fatal("PIB-341: the nested unknown-key mutation anchor is missing")
	}
	if _, err := DecodeIntentArchiveIndex([]byte(nested), "demo"); err == nil {
		t.Fatal("PIB-341: the strict decoder accepted a nested unknown key")
	}
}

package store

// Literal owning acceptance subtests for the §18.28 archive-index rows.
//
// `TestIntentArchiveStrictDecoderSensitivity` registers its cases with
// `t.Run(tc.name, …)` over a positional struct table, so its case labels exist
// only at run time and no static acceptance identity can name them. Each leaf
// below is a literal `t.Run("PIB-NNN…", …)` that drives the *shipped* strict
// decoder over the canonical bytes and then over one deliberately wrong version
// of them, asserting the exact refusal code and exit class §18 names for the row.

import (
	"encoding/json"
	"strings"
	"testing"
)

// pibRowArchiveIndexFixture returns the canonical index wire the shipped
// encoder produces for a single retained analysis replacement, plus the
// generation it carries, so every leaf mutates real evidence.
func pibRowArchiveIndexFixture(t *testing.T) ([]byte, IntentArchiveGeneration) {
	t.Helper()
	generation := archiveGeneration(t, "demo",
		archiveReplacement(t, IntentArchiveArtifactAnalysis, "analysis", IntentArchiveWireRetained),
	)
	wire, err := EncodeIntentArchiveIndex(archiveIndex(t, "demo", generation))
	if err != nil {
		t.Fatal(err)
	}
	return wire, generation
}

// pibRowAssertArchiveRefusal asserts the shipped bytes still decode and the
// mutated bytes are refused with the exact code at exit class 3.
func pibRowAssertArchiveRefusal(
	t *testing.T,
	row string,
	valid, mutated []byte,
	code IntentArchiveErrorCode,
) {
	t.Helper()
	if string(valid) == string(mutated) {
		t.Fatalf("%s: the mutation anchor is missing, so the fixture proves nothing", row)
	}
	if _, err := DecodeIntentArchiveIndex(valid, "demo"); err != nil {
		t.Fatalf("%s: the shipped index was rejected by its own strict decoder: %v", row, err)
	}
	_, err := DecodeIntentArchiveIndex(mutated, "demo")
	typed := assertArchiveCode(t, err, code)
	if typed.ExitClass != 3 {
		t.Fatalf("%s: %s exit class = %d, want 3", row, code, typed.ExitClass)
	}
}

func TestPIBRowIntentArchiveIndexStrictDecoderBinds(t *testing.T) {
	t.Run("PIB-331", func(t *testing.T) {
		valid, _ := pibRowArchiveIndexFixture(t)
		trailing := append(append([]byte(nil), valid...), []byte(`{}`)...)
		pibRowAssertArchiveRefusal(t, "PIB-331", valid, trailing, IntentArchiveCodeIndexCorrupt)
	})

	t.Run("PIB-332-unknown-top", func(t *testing.T) {
		valid, _ := pibRowArchiveIndexFixture(t)
		mutated := []byte(strings.Replace(
			string(valid), `"feature": "demo",`, `"feature": "demo", "extra": 1,`, 1,
		))
		pibRowAssertArchiveRefusal(t, "PIB-332", valid, mutated, IntentArchiveCodeIndexCorrupt)
	})

	t.Run("PIB-332-unknown-generation", func(t *testing.T) {
		valid, _ := pibRowArchiveIndexFixture(t)
		mutated := []byte(strings.Replace(
			string(valid), `"mode": "regenerate",`, `"mode": "regenerate", "extra": 1,`, 1,
		))
		pibRowAssertArchiveRefusal(t, "PIB-332", valid, mutated, IntentArchiveCodeIndexCorrupt)
	})

	t.Run("PIB-335-null-generations", func(t *testing.T) {
		valid, _ := pibRowArchiveIndexFixture(t)
		mutated := []byte(`{"schema_version":1,"feature":"demo","generations":null}`)
		pibRowAssertArchiveRefusal(t, "PIB-335", valid, mutated, IntentArchiveCodeIndexCorrupt)
	})

	t.Run("PIB-335-null-replaced", func(t *testing.T) {
		valid, generation := pibRowArchiveIndexFixture(t)
		mutated := []byte(`{"schema_version":1,"feature":"demo","generations":[{"generation_id":"` +
			generation.GenerationID + `","mode":"regenerate","replaced":null}]}`)
		pibRowAssertArchiveRefusal(t, "PIB-335", valid, mutated, IntentArchiveCodeIndexCorrupt)
	})

	t.Run("PIB-336-duplicate-key", func(t *testing.T) {
		valid, _ := pibRowArchiveIndexFixture(t)
		mutated := []byte(strings.Replace(
			string(valid), `"mode": "regenerate",`, `"mode": "regenerate", "mode": "regenerate",`, 1,
		))
		pibRowAssertArchiveRefusal(t, "PIB-336", valid, mutated, IntentArchiveCodeIndexCorrupt)
	})

	t.Run("PIB-336-malformed-generation-id", func(t *testing.T) {
		valid, generation := pibRowArchiveIndexFixture(t)
		mutated := []byte(strings.Replace(string(valid), generation.GenerationID, "not-a-sha", 1))
		pibRowAssertArchiveRefusal(t, "PIB-336", valid, mutated, IntentArchiveCodeIndexCorrupt)
	})

	t.Run("PIB-337", func(t *testing.T) {
		valid, generation := pibRowArchiveIndexFixture(t)
		blobless := generation.Replaced[0]
		blobless.Blob = ""
		blobless.Purged = false
		blobless.PurgePending = false
		unpublished := generation
		unpublished.Replaced = []IntentArchiveReplacement{blobless}
		mutated, err := json.Marshal(IntentArchiveIndex{
			SchemaVersion: IntentArchiveSchemaVersion,
			Feature:       "demo",
			Generations:   []IntentArchiveGeneration{unpublished},
		})
		if err != nil {
			t.Fatal(err)
		}
		pibRowAssertArchiveRefusal(t, "PIB-337", valid, mutated, IntentArchiveCodeIndexCorrupt)

		// The mirrored half of the row: a retained blob that also claims to be
		// purged is refused by the same shipped wire-state validator.
		purged := generation.Replaced[0]
		purged.Purged = true
		if err := validateIntentArchiveWireState(generation.Replaced[0]); err != nil {
			t.Fatalf("PIB-337: the shipped retained wire state was refused: %v", err)
		}
		if err := validateIntentArchiveWireState(purged); err == nil {
			t.Fatal("PIB-337: the wire-state validator accepted a purged replacement that keeps its blob")
		}
	})

	t.Run("PIB-338", func(t *testing.T) {
		valid, _ := pibRowArchiveIndexFixture(t)
		mutated := []byte(strings.Replace(
			string(valid), `"path": "analysis.md"`, `"path": "../analysis.md"`, 1,
		))
		pibRowAssertArchiveRefusal(t, "PIB-338", valid, mutated, IntentArchiveCodeIndexPathEscape)
	})

	t.Run("PIB-339", func(t *testing.T) {
		valid, _ := pibRowArchiveIndexFixture(t)
		mutated := []byte(strings.Replace(
			string(valid), `"artifact_id": "analysis"`, `"artifact_id": "notes"`, 1,
		))
		pibRowAssertArchiveRefusal(t, "PIB-339", valid, mutated, IntentArchiveCodeIndexCorrupt)
	})

	t.Run("PIB-340", func(t *testing.T) {
		valid, generation := pibRowArchiveIndexFixture(t)
		mutated := []byte(strings.Replace(
			string(valid), generation.GenerationID, strings.Repeat("a", 64), 1,
		))
		pibRowAssertArchiveRefusal(
			t, "PIB-340", valid, mutated, IntentArchiveCodeGenerationMismatch,
		)
	})

	t.Run("PIB-341-unknown-replacement", func(t *testing.T) {
		valid, _ := pibRowArchiveIndexFixture(t)
		mutated := []byte(strings.Replace(
			string(valid), `"path": "analysis.md",`, `"path": "analysis.md", "extra": 1,`, 1,
		))
		pibRowAssertArchiveRefusal(t, "PIB-341", valid, mutated, IntentArchiveCodeIndexCorrupt)
	})
}

package intentpub

// Literal owning acceptance subtests for the aggregate-ledger rows whose only
// prior evidence was a table-driven subtest whose name is produced at run time.
//
// `TestStageV1ThroughV6AndArtifactSensitivity`, `TestRecoveryEvidenceTableAndIdempotency`,
// `TestRecoveryCP5CP6CP8Fixtures` and `TestDecodeJournalJ1ThroughJ10` all register
// their cases with `t.Run(test.name, …)` over positional struct literals. Those
// labels exist only at run time, so no static identity can name them and the
// aggregate ledger must not pretend otherwise.
//
// Every leaf below is a literal `t.Run("PIB-NNN…", …)` whose body drives the
// real shipped validator — `Stage`, `Execute`/`Recover`, `DecodeJournal` — over
// the row's own fixture and asserts the exact observable §18 names for it: the
// refusal code, its exit class, its class token and the zero-mutation property.
// Nothing here asserts that a fixture label exists.

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// §18.7 F — staged-output validation V1…V5.
// ---------------------------------------------------------------------------

// pibRowStageRefusal drives the shipped staging validator over exactly one
// deliberately invalid staged input against a workspace that already holds
// canonical bytes for that artifact, and reports the typed refusal together
// with everything the row needs to prove zero canonical mutation.
func pibRowStageRefusal(t *testing.T, input StageInput) (*Error, StageResult, bool, string) {
	t.Helper()
	_, authority := acquireWorkspace(t)
	canonical := canonicalRel(testSlug, input.ArtifactID)
	rootWrite(t, authority, canonical, []byte("canonical-before"), 0o644)
	result, err := Stage(
		authority, testSlug, []StageInput{input}, Options{RandomHex12: sequenceHex()},
	)
	var typed *Error
	if !errors.As(err, &typed) {
		typed = nil
	}
	return typed, result, rootExists(t, authority, laneRel(testSlug)),
		string(rootRead(t, authority, canonical))
}

func TestPIBRowStagedOutputValidationRefusals(t *testing.T) {
	t.Run("PIB-082", func(t *testing.T) {
		typed, result, staged, canonical := pibRowStageRefusal(t, StageInput{
			ArtifactID: ArtifactSpec, Rel: "spec.md", Data: nil, Mode: 0o644,
		})
		if typed == nil || typed.Code != CodeStagedOutputInvalid ||
			typed.ExitClass != 2 || typed.Class != "v1-nonempty" {
			t.Fatalf("PIB-082: empty staged spec.md refusal = %#v", typed)
		}
		if canonical != "canonical-before" {
			t.Fatalf("PIB-082: the refusal mutated the canonical artifact: %q", canonical)
		}
		if result.StageRel != "" || staged {
			t.Fatalf("PIB-082: the refusal left staging state behind: %#v", result)
		}
	})

	t.Run("PIB-083", func(t *testing.T) {
		typed, _, _, canonical := pibRowStageRefusal(t, StageInput{
			ArtifactID: ArtifactSpec, Rel: "spec.md", Data: []byte(" \n\t"), Mode: 0o644,
		})
		if typed == nil || typed.Code != CodeStagedOutputInvalid ||
			typed.ExitClass != 2 || typed.Class != "v1-nonempty" {
			t.Fatalf("PIB-083: whitespace-only staged spec.md refusal = %#v", typed)
		}
		if canonical != "canonical-before" {
			t.Fatalf("PIB-083: the refusal mutated the canonical artifact: %q", canonical)
		}
	})

	t.Run("PIB-084", func(t *testing.T) {
		typed, _, _, canonical := pibRowStageRefusal(t, StageInput{
			ArtifactID: ArtifactAnalysis,
			Rel:        "analysis.md",
			Data:       make([]byte, MaxArtifactBytes+1),
			Mode:       0o644,
		})
		if typed == nil || typed.Code != CodeStagedOutputInvalid ||
			typed.ExitClass != 2 || typed.Class != "v2-size" {
			t.Fatalf("PIB-084: oversize staged analysis.md refusal = %#v", typed)
		}
		if canonical != "canonical-before" {
			t.Fatalf("PIB-084: the refusal mutated the canonical artifact: %q", canonical)
		}
	})

	t.Run("PIB-085", func(t *testing.T) {
		typed, _, _, canonical := pibRowStageRefusal(t, StageInput{
			ArtifactID: ArtifactExploration,
			Rel:        "exploration.md",
			Data:       []byte("ok\x00bad"),
			Mode:       0o644,
		})
		if typed == nil || typed.Code != CodeStagedOutputInvalid ||
			typed.ExitClass != 2 || typed.Class != "v3-nul" {
			t.Fatalf("PIB-085: NUL-carrying staged bytes refusal = %#v", typed)
		}
		if canonical != "canonical-before" {
			t.Fatalf("PIB-085: the refusal mutated the canonical artifact: %q", canonical)
		}
	})

	t.Run("PIB-086", func(t *testing.T) {
		typed, _, _, canonical := pibRowStageRefusal(t, StageInput{
			ArtifactID: ArtifactExploration,
			Rel:        "exploration.md",
			Data:       []byte{0xff, 'x'},
			Mode:       0o644,
		})
		if typed == nil || typed.Code != CodeStagedOutputInvalid ||
			typed.ExitClass != 2 || typed.Class != "v4-utf8" {
			t.Fatalf("PIB-086: invalid-UTF-8 staged bytes refusal = %#v", typed)
		}
		if canonical != "canonical-before" {
			t.Fatalf("PIB-086: the refusal mutated the canonical artifact: %q", canonical)
		}
	})

	t.Run("PIB-087", func(t *testing.T) {
		typed, _, _, canonical := pibRowStageRefusal(t, StageInput{
			ArtifactID: ArtifactAnalysisSidecar,
			Rel:        "analysis.json",
			Data:       []byte(`[1,2,3]`),
			Mode:       0o644,
		})
		if typed == nil || typed.Code != CodeStagedOutputInvalid ||
			typed.ExitClass != 2 || typed.Class != "v5-json-object" {
			t.Fatalf("PIB-087: array staged sidecar refusal = %#v", typed)
		}
		if canonical != "canonical-before" {
			t.Fatalf("PIB-087: the refusal mutated the canonical artifact: %q", canonical)
		}
	})

	t.Run("PIB-088", func(t *testing.T) {
		typed, _, _, canonical := pibRowStageRefusal(t, StageInput{
			ArtifactID: ArtifactAnalysisSidecar,
			Rel:        "analysis.json",
			Data:       []byte(`{`),
			Mode:       0o644,
		})
		if typed == nil || typed.Code != CodeStagedOutputInvalid ||
			typed.ExitClass != 2 || typed.Class != "v5-json-object" {
			t.Fatalf("PIB-088: malformed staged sidecar refusal = %#v", typed)
		}
		if canonical != "canonical-before" {
			t.Fatalf("PIB-088: the refusal mutated the canonical artifact: %q", canonical)
		}
	})
}

// ---------------------------------------------------------------------------
// §18.10 I — crash-phase recovery evidence.
// ---------------------------------------------------------------------------

func TestPIBRowRecoveryCrashPhaseEvidence(t *testing.T) {
	t.Run("PIB-119", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		if _, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point != PointAfterJournalDurable {
					return nil
				}
				return errors.New("crash")
			},
		}); err == nil {
			t.Fatal("PIB-119: the CP3 crash hook did not stop execution")
		}
		for _, entry := range plan.Entries() {
			if entry.Action == ActionCreate && rootExists(t, authority, entry.Rel) {
				t.Fatalf("PIB-119: CP3 published %s before the journal was applied", entry.Rel)
			}
		}
		status := string(rootRead(t, authority, canonicalRel(testSlug, ArtifactStatus)))
		if status != `{"state":"requested"}` {
			t.Fatalf("PIB-119: CP3 changed the status preimage: %q", status)
		}
		recovered, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
		if err != nil {
			t.Fatalf("PIB-119: CP3 recovery failed: %v", err)
		}
		if recovered.Outcome != OutcomeRecovered || len(recovered.Restored) != 0 ||
			recovered.Completed {
			t.Fatalf("PIB-119: CP3 recovery = %#v", recovered)
		}
		if rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("PIB-119: CP3 recovery left the journal behind")
		}
		second, err := Recover(authority, testSlug, Options{})
		if err != nil || second.Outcome != OutcomeRecoveryAbsent {
			t.Fatalf("PIB-119: CP3 second recovery = %#v, %v", second, err)
		}
	})

	t.Run("PIB-120", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		renames := 0
		if _, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterEntryRename {
					renames++
					if renames == 2 {
						return errors.New("crash")
					}
				}
				return nil
			},
		}); err == nil {
			t.Fatal("PIB-120: the CP4 crash hook did not stop execution")
		}
		recovered, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
		if err != nil {
			t.Fatalf("PIB-120: CP4 recovery failed: %v", err)
		}
		if recovered.Outcome != OutcomeRecovered || len(recovered.Restored) != 2 ||
			recovered.Completed {
			t.Fatalf("PIB-120: CP4 recovery = %#v", recovered)
		}
		if rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("PIB-120: CP4 recovery left the journal behind")
		}
		second, err := Recover(authority, testSlug, Options{})
		if err != nil || second.Outcome != OutcomeRecoveryAbsent {
			t.Fatalf("PIB-120: CP4 second recovery = %#v, %v", second, err)
		}
	})

	t.Run("PIB-121", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageRegeneratePlan(t, authority)
		renames := 0
		_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterEntryRename {
					renames++
					if renames == 4 {
						return errors.New("crash")
					}
				}
				return nil
			},
		})
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != CodeCrashInjected {
			t.Fatalf("PIB-121: CP5 crash injection = %#v", err)
		}
		recovered, recoverErr := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
		if recoverErr != nil {
			t.Fatalf("PIB-121: CP5 recovery failed: %v", recoverErr)
		}
		if len(recovered.Restored) != 4 || recovered.Completed {
			t.Fatalf("PIB-121: CP5 recovery = %#v", recovered)
		}
		assertRegeneratePreimages(t, authority)
		if rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("PIB-121: CP5 recovery left the journal behind")
		}
	})

	t.Run("PIB-122", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageRegeneratePlan(t, authority)
		renames := 0
		_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterEntryRename {
					renames++
					if renames == 5 {
						return errors.New("crash")
					}
				}
				return nil
			},
		})
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != CodeCrashInjected {
			t.Fatalf("PIB-122: CP6 crash injection = %#v", err)
		}
		recovered, recoverErr := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
		if recoverErr != nil {
			t.Fatalf("PIB-122: CP6 recovery failed: %v", recoverErr)
		}
		if len(recovered.Restored) != 5 || recovered.Completed {
			t.Fatalf("PIB-122: CP6 recovery = %#v", recovered)
		}
		assertRegeneratePreimages(t, authority)
		index := string(rootRead(t, authority, canonicalRel(testSlug, ArtifactArchiveIndex)))
		if index != `{"old":"index"}` {
			t.Fatalf("PIB-122: CP6 recovery did not restore index.json: %q", index)
		}
	})

	t.Run("PIB-123", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		if _, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point != PointAfterAllRenames {
					return nil
				}
				return errors.New("crash")
			},
		}); err == nil {
			t.Fatal("PIB-123: the CP7 crash hook did not stop execution")
		}
		published := map[string]string{}
		for _, entry := range plan.Entries() {
			if rootExists(t, authority, entry.Rel) {
				published[entry.Rel] = string(rootRead(t, authority, entry.Rel))
			}
		}
		if len(published) != len(plan.Entries()) {
			t.Fatalf("PIB-123: CP7 left %d of %d canonical files, want the all-new tree",
				len(published), len(plan.Entries()))
		}
		recovered, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
		if err != nil {
			t.Fatalf("PIB-123: CP7 recovery failed: %v", err)
		}
		if recovered.Outcome != OutcomeRecovered || len(recovered.Restored) != 0 ||
			!recovered.Completed {
			t.Fatalf("PIB-123: CP7 recovery = %#v", recovered)
		}
		for rel, want := range published {
			if got := string(rootRead(t, authority, rel)); got != want {
				t.Fatalf("PIB-123: CP7 recovery undid %s\n got %q\nwant %q", rel, got, want)
			}
		}
		if rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("PIB-123: CP7 recovery left the journal behind")
		}
	})
}

// ---------------------------------------------------------------------------
// §18.24 X — the strict journal decoder, J1…J10.
// ---------------------------------------------------------------------------

// pibRowValidJournal returns the canonical journal bytes the shipped encoder
// produces for the reference plan, so every leaf below mutates real evidence
// rather than a hand-written fixture.
func pibRowValidJournal(t *testing.T) []byte {
	t.Helper()
	journal, err := BuildJournal(testCreatePlan(t), "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeJournal(journal)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

// pibRowAssertJournalRefusal asserts the shipped bytes still decode and the
// mutated bytes are refused with the exact §10.4 code at exit class 6.
func pibRowAssertJournalRefusal(t *testing.T, row string, valid, mutated []byte, code Code) {
	t.Helper()
	if bytes.Equal(valid, mutated) {
		t.Fatalf("%s: the mutation anchor is missing, so the fixture proves nothing", row)
	}
	if _, err := DecodeJournal(valid, testSlug); err != nil {
		t.Fatalf("%s: the shipped journal was rejected by its own decoder: %v", row, err)
	}
	_, err := DecodeJournal(mutated, testSlug)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != code || typed.ExitClass != 6 {
		t.Fatalf("%s: mutated journal = %#v, want %s at exit 6", row, err, code)
	}
}

func TestPIBRowJournalStrictDecoderBinds(t *testing.T) {
	t.Run("PIB-298", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		trailing := append(append([]byte(nil), valid...), []byte(`{}`)...)
		pibRowAssertJournalRefusal(t, "PIB-298", valid, trailing, CodeJournalCorrupt)
	})

	t.Run("PIB-299-unknown-top", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := bytes.Replace(valid, []byte(`"version":`), []byte(`"unknown": 1, "version":`), 1)
		pibRowAssertJournalRefusal(t, "PIB-299", valid, mutated, CodeJournalCorrupt)
	})

	t.Run("PIB-299-unknown-entry", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := bytes.Replace(
			valid, []byte(`"artifact_id":`), []byte(`"mystery": 1, "artifact_id":`), 1,
		)
		pibRowAssertJournalRefusal(t, "PIB-299", valid, mutated, CodeJournalCorrupt)
	})

	t.Run("PIB-299-unknown-identity", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := bytes.Replace(valid, []byte(`"exists":`), []byte(`"mystery": 1, "exists":`), 1)
		pibRowAssertJournalRefusal(t, "PIB-299", valid, mutated, CodeJournalCorrupt)
	})

	t.Run("PIB-300", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := replaceJSONField(t, valid, "version", float64(2))
		pibRowAssertJournalRefusal(t, "PIB-300", valid, mutated, CodeJournalVersionMismatch)
		_, err := DecodeJournal(mutated, testSlug)
		var typed *Error
		if !errors.As(err, &typed) ||
			!strings.Contains(typed.Detail, "written by another build") {
			t.Fatalf("PIB-300: version mismatch detail = %#v", err)
		}
	})

	t.Run("PIB-301", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := replaceJSONField(t, valid, "slug", "other")
		pibRowAssertJournalRefusal(t, "PIB-301", valid, mutated, CodeJournalForeign)
	})

	t.Run("PIB-302", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := replaceEntryField(t, valid, 0, "rel", "../outside")
		pibRowAssertJournalRefusal(t, "PIB-302", valid, mutated, CodeJournalPathEscape)
	})

	t.Run("PIB-303", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := replaceEntryIdentityHash(t, valid, 0, strings.Repeat("a", 64))
		pibRowAssertJournalRefusal(t, "PIB-303", valid, mutated, CodeJournalForged)
	})

	t.Run("PIB-304-duplicate-id", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := replaceEntryField(t, valid, 1, "artifact_id", "analysis")
		pibRowAssertJournalRefusal(t, "PIB-304", valid, mutated, CodeJournalCorrupt)
	})

	t.Run("PIB-304-null-entries", func(t *testing.T) {
		valid := pibRowValidJournal(t)
		mutated := replaceJSONField(t, valid, "entries", nil)
		pibRowAssertJournalRefusal(t, "PIB-304", valid, mutated, CodeJournalCorrupt)
	})
}

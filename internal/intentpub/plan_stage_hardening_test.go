package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPlanModeSetsExhaustive(t *testing.T) {
	t.Run("generate-all-subsets", func(t *testing.T) {
		validSets := map[string]bool{
			"exploration,status":                                true,
			"spec,exploration,status":                           true,
			"analysis,spec,exploration,analysis_sidecar,status": true,
		}
		for mask := 1; mask < 1<<len(artifactOrder); mask++ {
			entries := make([]Entry, 0, len(artifactOrder))
			for index, id := range artifactOrder {
				if mask&(1<<index) == 0 {
					continue
				}
				action := ActionCreate
				if id == ArtifactStatus {
					action = ActionReplace
				}
				entries = append(entries, planFixtureEntry(t, id, action, "generate"))
			}
			signature := artifactSignature(entries)
			_, err := NewPlan(testSlug, ModeGenerate, testStageRel, entries)
			if validSets[signature] {
				if err != nil {
					t.Fatalf("%s rejected: %v", signature, err)
				}
			} else if err == nil {
				t.Fatalf("incoherent generate set accepted: %s", signature)
			}
		}
	})

	t.Run("regenerate-presence-matrix", func(t *testing.T) {
		intentIDs := []ArtifactID{
			ArtifactAnalysis,
			ArtifactSpec,
			ArtifactExploration,
			ArtifactAnalysisSidecar,
		}
		for mask := 0; mask < 1<<len(intentIDs); mask++ {
			entries := make([]Entry, 0, 6)
			for index, id := range intentIDs {
				action := ActionCreate
				if mask&(1<<index) != 0 {
					action = ActionReplace
				}
				entries = append(entries, planFixtureEntry(t, id, action, "regenerate"))
			}
			if mask != 0 {
				indexAction := ActionCreate
				if mask&1 != 0 {
					indexAction = ActionReplace
				}
				entries = append(entries, planFixtureEntry(t, ArtifactArchiveIndex, indexAction, "regenerate"))
			}
			entries = append(entries, planFixtureEntry(t, ArtifactStatus, ActionReplace, "regenerate"))
			if _, err := NewPlan(testSlug, ModeRegenerate, testStageRel, entries); err != nil {
				t.Fatalf("presence mask %04b rejected: %v", mask, err)
			}
		}
	})
}

func TestPlanRejectsEveryModeSetIncoherence(t *testing.T) {
	full := []Entry{
		planFixtureEntry(t, ArtifactAnalysis, ActionReplace, "full"),
		planFixtureEntry(t, ArtifactSpec, ActionReplace, "full"),
		planFixtureEntry(t, ArtifactExploration, ActionReplace, "full"),
		planFixtureEntry(t, ArtifactAnalysisSidecar, ActionReplace, "full"),
		planFixtureEntry(t, ArtifactArchiveIndex, ActionReplace, "full"),
		planFixtureEntry(t, ArtifactStatus, ActionReplace, "full"),
	}
	tests := []struct {
		name    string
		mode    Mode
		entries []Entry
	}{
		{"regenerate-missing-analysis", ModeRegenerate, cloneEntries(full[1:])},
		{"regenerate-missing-spec", ModeRegenerate, append(cloneEntries(full[:1]), full[2:]...)},
		{"regenerate-missing-exploration", ModeRegenerate, append(cloneEntries(full[:2]), full[3:]...)},
		{"regenerate-missing-sidecar", ModeRegenerate, append(cloneEntries(full[:3]), full[4:]...)},
		{"regenerate-missing-index", ModeRegenerate, append(cloneEntries(full[:4]), full[5])},
		{"regenerate-index-from-status-only", ModeRegenerate, []Entry{
			planFixtureEntry(t, ArtifactAnalysis, ActionCreate, "status-only"),
			planFixtureEntry(t, ArtifactSpec, ActionCreate, "status-only"),
			planFixtureEntry(t, ArtifactExploration, ActionCreate, "status-only"),
			planFixtureEntry(t, ArtifactAnalysisSidecar, ActionCreate, "status-only"),
			planFixtureEntry(t, ArtifactArchiveIndex, ActionCreate, "status-only"),
			planFixtureEntry(t, ArtifactStatus, ActionReplace, "status-only"),
		}},
		{"generate-status-create", ModeGenerate, []Entry{
			planFixtureEntry(t, ArtifactExploration, ActionCreate, "status-create"),
			planFixtureEntry(t, ArtifactStatus, ActionCreate, "status-create"),
		}},
		{"generate-status-not-last", ModeGenerate, []Entry{
			planFixtureEntry(t, ArtifactStatus, ActionReplace, "status-first"),
			planFixtureEntry(t, ArtifactExploration, ActionCreate, "status-first"),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPlan(testSlug, test.mode, testStageRel, test.entries)
			assertCode(t, err, CodeInvalidPlan)
		})
	}
}

func TestPlanAllowsSemanticNoOpReplaceForEveryArtifact(t *testing.T) {
	entries := []Entry{
		planFixtureEntry(t, ArtifactAnalysis, ActionReplace, "no-op"),
		planFixtureEntry(t, ArtifactSpec, ActionReplace, "no-op"),
		planFixtureEntry(t, ArtifactExploration, ActionReplace, "no-op"),
		planFixtureEntry(t, ArtifactAnalysisSidecar, ActionReplace, "no-op"),
		planFixtureEntry(t, ArtifactArchiveIndex, ActionReplace, "no-op"),
		planFixtureEntry(t, ArtifactStatus, ActionReplace, "no-op"),
	}
	for index := range entries {
		entries[index].NewImage = entries[index].Preimage
	}
	plan, err := NewPlan(testSlug, ModeRegenerate, testStageRel, entries)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Entries(); len(got) != 6 ||
		!equalArtifactIDs(entryIDs(got[:4]), []ArtifactID{
			ArtifactAnalysis, ArtifactSpec, ArtifactExploration, ArtifactAnalysisSidecar,
		}) {
		t.Fatalf("regenerate plan omitted intent artifacts: %#v", got)
	}
}

func TestRecoveryDualMatchDoesNotRestoreSemanticNoOps(t *testing.T) {
	_, authority := acquireWorkspace(t)
	base := stageRegeneratePlan(t, authority)
	entries := base.Entries()
	indexEntry := &entries[4]
	rootWrite(t, authority, indexEntry.StagedRel, []byte(`{"old":"index"}`), 0o600)
	indexEntry.NewImage = indexEntry.Preimage
	plan, err := NewPlan(testSlug, ModeRegenerate, base.StageRel(), entries)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
			if point == PointAfterJournalDurable {
				return errors.New("crash")
			}
			return nil
		},
	})
	assertCode(t, err, CodeCrashInjected)

	analysis := entries[0]
	rootWrite(t, authority, analysis.Rel, rootRead(t, authority, analysis.StagedRel), fs.FileMode(analysis.NewImage.Mode))
	recovered, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recovered.Restored, []ArtifactID{ArtifactAnalysis}) {
		t.Fatalf("dual-match archive index was incorrectly restored: %#v", recovered)
	}
	if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactArchiveIndex))) != `{"old":"index"}` {
		t.Fatal("semantic no-op archive index changed during recovery")
	}
}

func TestStageV1ThroughV6AndArtifactSensitivity(t *testing.T) {
	tests := []struct {
		name  string
		input StageInput
		class string
	}{
		{"v1-empty", StageInput{ArtifactID: ArtifactSpec, Rel: "spec.md", Data: nil, Mode: 0o644}, "v1-nonempty"},
		{"v1-whitespace", StageInput{ArtifactID: ArtifactSpec, Rel: "spec.md", Data: []byte(" \n\t"), Mode: 0o644}, "v1-nonempty"},
		{"v2-oversize", StageInput{ArtifactID: ArtifactAnalysis, Rel: "analysis.md", Data: make([]byte, MaxArtifactBytes+1), Mode: 0o644}, "v2-size"},
		{"v2-status-oversize", StageInput{ArtifactID: ArtifactStatus, Rel: "status.json", Data: make([]byte, MaxArtifactBytes+1), Mode: 0o644}, "v2-size"},
		{"v3-nul", StageInput{ArtifactID: ArtifactExploration, Rel: "exploration.md", Data: []byte("ok\x00bad"), Mode: 0o644}, "v3-nul"},
		{"v3-status-nul", StageInput{ArtifactID: ArtifactStatus, Rel: "status.json", Data: []byte("ok\x00bad"), Mode: 0o644}, "v3-nul"},
		{"v4-utf8", StageInput{ArtifactID: ArtifactExploration, Rel: "exploration.md", Data: []byte{0xff, 'x'}, Mode: 0o644}, "v4-utf8"},
		{"v4-index-utf8", StageInput{ArtifactID: ArtifactArchiveIndex, Rel: "index.json", Data: []byte{0xff, 'x'}, Mode: 0o644}, "v4-utf8"},
		{"v5-array", StageInput{ArtifactID: ArtifactAnalysisSidecar, Rel: "analysis.json", Data: []byte(`[1,2,3]`), Mode: 0o644}, "v5-json-object"},
		{"v5-malformed", StageInput{ArtifactID: ArtifactAnalysisSidecar, Rel: "analysis.json", Data: []byte(`{`), Mode: 0o644}, "v5-json-object"},
		{"v5-trailing", StageInput{ArtifactID: ArtifactAnalysisSidecar, Rel: "analysis.json", Data: []byte(`{} {}`), Mode: 0o644}, "v5-json-object"},
		{"v5-null", StageInput{ArtifactID: ArtifactAnalysisSidecar, Rel: "analysis.json", Data: []byte(`null`), Mode: 0o644}, "v5-json-object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			result, err := Stage(authority, testSlug, []StageInput{test.input}, Options{RandomHex12: sequenceHex()})
			var typed *Error
			if !errors.As(err, &typed) || typed.Code != CodeStagedOutputInvalid ||
				typed.ExitClass != 2 || typed.Class != test.class {
				t.Fatalf("error = %#v", err)
			}
			if result.StageRel != "" || rootExists(t, authority, laneRel(testSlug)) {
				t.Fatalf("invalid staged output created staging state: %#v", result)
			}
		})
	}

	t.Run("v5-encoding-json-object-key-semantics", func(t *testing.T) {
		for _, data := range [][]byte{
			[]byte(`{}`),
			[]byte(`{"SUMMARY":1}`),
			[]byte(`{"":1}`),
			[]byte(`{"summary":1,"summary":2}`),
			[]byte(`{"summary":1,"SUMMARY":2}`),
			[]byte(`{"compatibility":{"status":1,"status":2}}`),
		} {
			_, authority := acquireWorkspace(t)
			result, err := Stage(authority, testSlug, []StageInput{{
				ArtifactID: ArtifactAnalysisSidecar,
				Rel:        "analysis.json",
				Data:       data,
				Mode:       0o644,
			}}, Options{RandomHex12: sequenceHex()})
			if err != nil {
				t.Fatalf("%s rejected: %v", data, err)
			}
			if len(result.Files) != 1 {
				t.Fatalf("%s produced %#v", data, result)
			}
		}
	})

	t.Run("v6-identity", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		state := &stageMutationState{}
		_, err := Stage(authority, testSlug, []StageInput{{
			ArtifactID: ArtifactSpec,
			Rel:        "spec.md",
			Data:       []byte("valid"),
			Mode:       0o644,
		}}, Options{
			RandomHex12: sequenceHex(),
			RootOpsFactory: func(root *os.Root) RootOps {
				return &stageMutationOps{RootOps: NewRootOps(root), state: state}
			},
		})
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != CodeEntryChanged ||
			typed.Class != "v6-staged-identity" || typed.ExitClass != 5 {
			t.Fatalf("error = %#v", err)
		}
	})

	t.Run("artifact-specific-nonempty", func(t *testing.T) {
		for _, input := range []StageInput{
			{ArtifactID: ArtifactArchiveIndex, Rel: "index.json", Data: nil, Mode: 0o644},
			{ArtifactID: ArtifactStatus, Rel: "status.json", Data: []byte(" \n"), Mode: 0o644},
		} {
			_, authority := acquireWorkspace(t)
			result, err := Stage(authority, testSlug, []StageInput{input}, Options{RandomHex12: sequenceHex()})
			if err != nil {
				t.Fatalf("%s should not receive Markdown V1: %v", input.ArtifactID, err)
			}
			if result.Files[0].Identity.Mode != 0o600 || result.Files[0].NewImage.Mode != 0o644 {
				t.Fatalf("staged/final identities = %#v", result.Files[0])
			}
		}
	})
}

func planFixtureEntry(t *testing.T, id ArtifactID, action Action, salt string) Entry {
	t.Helper()
	newImage, err := identityForBytes([]byte("new-"+string(id)+"-"+salt), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{
		ArtifactID: id,
		Rel:        canonicalRel(testSlug, id),
		Action:     action,
		Preimage:   AbsentIdentity(),
		NewImage:   newImage,
		StagedRel:  testStageRel + "/" + stagedBase(id),
	}
	if action != ActionReplace {
		return entry
	}
	preimage, err := identityForBytes([]byte("old-"+string(id)+"-"+salt), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	entry.Preimage = preimage
	switch id {
	case ArtifactArchiveIndex:
		entry.PreimageRawRel = laneRel(testSlug) + "/index.preimage.json"
	case ArtifactStatus:
		entry.PreimageRawRel = laneRel(testSlug) + "/status.preimage.json"
	default:
		entry.PreimageBlob = preimage.SHA256
		entry.PreimageBlobRel = featureRel(testSlug) + "/artifacts/intent-archive/blobs/" + preimage.SHA256 + ".blob"
	}
	return entry
}

func artifactSignature(entries []Entry) string {
	parts := make([]string, len(entries))
	for index, entry := range entries {
		parts[index] = string(entry.ArtifactID)
	}
	return strings.Join(parts, ",")
}

type stageMutationState struct {
	readCount int
}

type stageMutationOps struct {
	RootOps
	state *stageMutationState
}

func (ops *stageMutationOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	if strings.HasSuffix(name, "/spec.md") && flag&os.O_RDONLY == os.O_RDONLY &&
		flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE) == 0 {
		ops.state.readCount++
		if ops.state.readCount == 2 {
			file, err := ops.RootOps.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0)
			if err != nil {
				return nil, err
			}
			if _, err := file.Write([]byte("other")); err != nil {
				_ = file.Close()
				return nil, err
			}
			if err := file.Sync(); err != nil {
				_ = file.Close()
				return nil, err
			}
			if err := file.Close(); err != nil {
				return nil, err
			}
		}
	}
	return ops.RootOps.OpenFile(name, flag, mode)
}

package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func TestExecutePublishesFixedOrderStatusLastAndRetainsAuthority(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	var renames []string
	options := Options{
		RandomHex12: fixedHex("222222222222"),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &recordingOps{RootOps: NewRootOps(root), renames: &renames}
		},
	}

	result, err := Execute(authority, plan, "0123456789abcdef", nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomePublished || result.ExitClass != 0 || !result.Completed {
		t.Fatalf("result = %#v", result)
	}
	var canonical []string
	for _, rel := range renames {
		if strings.HasPrefix(rel, featureRel(testSlug)+"/") {
			canonical = append(canonical, rel)
		}
	}

	want := []string{
		canonicalRel(testSlug, ArtifactAnalysis),
		canonicalRel(testSlug, ArtifactSpec),
		canonicalRel(testSlug, ArtifactExploration),
		canonicalRel(testSlug, ArtifactAnalysisSidecar),
		canonicalRel(testSlug, ArtifactStatus),
	}
	if strings.Join(canonical, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rename order = %v, want %v", canonical, want)
	}
	if authority.Released() {
		t.Fatal("Execute released caller-owned authority")
	}
	if rootExists(t, authority, JournalRel(testSlug)) || rootExists(t, authority, plan.StageRel()) {
		t.Fatal("successful transaction retained journal or stage")
	}
}

func TestRegeneratePublishesIndexImmediatelyBeforeStatus(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageRegeneratePlan(t, authority)
	var renames []string
	result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &recordingOps{RootOps: NewRootOps(root), renames: &renames}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var canonical []string
	for _, rel := range renames {
		if strings.HasPrefix(rel, featureRel(testSlug)+"/") {
			canonical = append(canonical, rel)
		}
	}
	want := make([]string, 0, len(artifactOrder))
	for _, id := range artifactOrder {
		want = append(want, canonicalRel(testSlug, id))
	}
	if !reflect.DeepEqual(canonical, want) {
		t.Fatalf("canonical order=%v, want %v", canonical, want)
	}
	if result.Published[len(result.Published)-2] != ArtifactArchiveIndex ||
		result.Published[len(result.Published)-1] != ArtifactStatus {
		t.Fatalf("published order = %v", result.Published)
	}
}

func TestExecuteAcceptsByteIdenticalRegenerateBundle(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageSemanticNoOpRegeneratePlan(t, authority)
	before := make(map[ArtifactID]Identity, len(plan.Entries()))
	for _, entry := range plan.Entries() {
		before[entry.ArtifactID] = captureForTest(t, authority, entry.Rel)
	}

	result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomePublished || !result.Completed || len(result.Published) != 0 {
		t.Fatalf("semantic no-op result = %#v", result)
	}
	for _, entry := range plan.Entries() {
		if current := captureForTest(t, authority, entry.Rel); !current.Equal(before[entry.ArtifactID]) {
			t.Fatalf("%s changed during byte-identical regenerate", entry.ArtifactID)
		}
	}
	if rootExists(t, authority, JournalRel(testSlug)) || rootExists(t, authority, plan.StageRel()) {
		t.Fatal("semantic no-op publication retained transaction residue")
	}
}

func TestExecuteSetAndPerEntryCAS(t *testing.T) {
	t.Run("set-level", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		rootWrite(t, authority, canonicalRel(testSlug, ArtifactAnalysis), []byte("editor"), 0o644)
		result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{RandomHex12: fixedHex("333333333333")})
		assertCode(t, err, CodeEntryAppeared)
		if result.Outcome != OutcomeFailed || rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatalf("set-level mismatch armed a transaction: %#v", result)
		}
		if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactAnalysis))) != "editor" {
			t.Fatal("set-level CAS changed editor bytes")
		}
	})

	t.Run("per-entry", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		options := Options{
			RandomHex12: fixedHex("444444444444"),
			Hook: func(point CrashPoint, root *os.Root, entry *Entry) error {
				if point == PointBeforeEntryCAS && entry != nil && entry.ArtifactID == ArtifactSpec {
					return writeRootFile(root, entry.Rel, []byte("editor-spec"), 0o644)
				}
				return nil
			},
		}
		result, err := Execute(authority, plan, "0123456789abcdef", nil, options)
		assertCode(t, err, CodeUndoCASMismatch)
		if result.Outcome != OutcomeFailed || result.ExitClass != 6 {
			t.Fatalf("result = %#v", result)
		}
		if rootExists(t, authority, canonicalRel(testSlug, ArtifactAnalysis)) {
			t.Fatal("published prefix was not rolled back")
		}
		if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactSpec))) != "editor-spec" {
			t.Fatal("editor bytes were not preserved")
		}
		if !rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("final rollback set mismatch removed journal evidence")
		}
	})
}

func TestExecuteRollbackAndOrphanResult(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	state := &renameFailureState{failCanonicalAt: 2}
	options := Options{
		RandomHex12: fixedHex("555555555555"),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &failingRenameOps{RootOps: NewRootOps(root), state: state}
		},
	}
	orphans := []string{"sha256-a", "sha256-b"}
	result, err := Execute(authority, plan, "0123456789abcdef", orphans, options)
	assertCode(t, err, CodeRootedWrite)
	if result.Outcome != OutcomeRolledBack || result.ExitClass != 5 {
		t.Fatalf("result = %#v", result)
	}
	if strings.Join(result.Orphans, ",") != strings.Join(orphans, ",") {
		t.Fatalf("orphans = %v, want %v", result.Orphans, orphans)
	}
	for _, entry := range plan.Entries() {
		if entry.Action == ActionCreate && rootExists(t, authority, entry.Rel) {
			t.Fatalf("%s survived successful rollback", entry.ArtifactID)
		}
		if entry.ArtifactID == ArtifactStatus &&
			string(rootRead(t, authority, entry.Rel)) != `{"state":"requested"}` {
			t.Fatal("status preimage was not restored")
		}
	}
	if rootExists(t, authority, JournalRel(testSlug)) {
		t.Fatal("journal survived successful rollback")
	}
}

func TestUndoCASPreservesThirdPartyBytesAndEvidence(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	edited := false
	options := Options{
		RandomHex12: fixedHex("666666666666"),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &failingRenameOps{
				RootOps: NewRootOps(root),
				state:   &renameFailureState{failCanonicalAt: 2},
			}
		},
		Hook: func(point CrashPoint, root *os.Root, entry *Entry) error {
			if point == PointAfterEntryRename && entry != nil && entry.ArtifactID == ArtifactAnalysis && !edited {
				edited = true
				return writeRootFile(root, entry.Rel, []byte("third-party"), 0o644)
			}
			return nil
		},
	}
	result, err := Execute(authority, plan, "0123456789abcdef", nil, options)
	assertCode(t, err, CodeUndoCASMismatch)
	if result.ExitClass != 6 || result.Outcome != OutcomeFailed {
		t.Fatalf("result = %#v", result)
	}
	if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactAnalysis))) != "third-party" {
		t.Fatal("rollback clobbered third-party bytes")
	}
	if !rootExists(t, authority, JournalRel(testSlug)) {
		t.Fatal("rollback failure did not retain journal evidence")
	}
}

func TestRollbackFinalSetDetectsEarlierRestoreRewrittenAfterLaterUndo(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	undoCount := 0
	result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &failingRenameOps{
				RootOps: NewRootOps(root),
				state:   &renameFailureState{failCanonicalAt: 3},
			}
		},
		Hook: func(point CrashPoint, root *os.Root, entry *Entry) error {
			if point != PointAfterUndo || entry == nil {
				return nil
			}
			undoCount++
			if undoCount == 2 {
				return writeRootFile(root, canonicalRel(testSlug, ArtifactSpec), []byte("rewritten-after-restore"), 0o644)
			}
			return nil
		},
	})
	assertCode(t, err, CodeUndoCASMismatch)
	assertResultErrorExitAgreement(t, result, err, 6)
	if !reflect.DeepEqual(result.Restored, []ArtifactID{ArtifactAnalysis}) {
		t.Fatalf("final-set divergent entry reported restored: %v", result.Restored)
	}
	if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactSpec))) != "rewritten-after-restore" {
		t.Fatal("final set verification changed divergent bytes")
	}
	if !rootExists(t, authority, JournalRel(testSlug)) {
		t.Fatal("final set divergence removed rollback evidence")
	}
}

func TestFinalVerificationPreservesDivergence(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	options := Options{
		RandomHex12: fixedHex("777777777777"),
		Hook: func(point CrashPoint, root *os.Root, _ *Entry) error {
			if point == PointBeforeFinalVerify {
				return writeRootFile(root, canonicalRel(testSlug, ArtifactStatus), []byte("external-status"), 0o644)
			}
			return nil
		},
	}
	result, err := Execute(authority, plan, "0123456789abcdef", nil, options)
	assertCode(t, err, CodePostPublicationDivergence)
	if result.ExitClass != 6 || string(rootRead(t, authority, canonicalRel(testSlug, ArtifactStatus))) != "external-status" {
		t.Fatalf("final divergence result = %#v", result)
	}
	if !rootExists(t, authority, JournalRel(testSlug)) {
		t.Fatal("final divergence did not preserve journal")
	}
}

func TestRollbackRestoresArchiveAndRawMetadataPreimages(t *testing.T) {
	_, authority := acquireWorkspace(t)
	oldIntent := map[ArtifactID][]byte{
		ArtifactAnalysis:        []byte("old-analysis"),
		ArtifactSpec:            []byte("old-spec"),
		ArtifactExploration:     []byte("old-exploration"),
		ArtifactAnalysisSidecar: []byte(`{"old":"analysis"}`),
	}
	oldIndex := []byte(`{"old":"index"}`)
	oldStatus := []byte(`{"old":"status"}`)
	for id, data := range oldIntent {
		rootWrite(t, authority, canonicalRel(testSlug, id), data, 0o644)
	}
	rootWrite(t, authority, canonicalRel(testSlug, ArtifactArchiveIndex), oldIndex, 0o644)
	rootWrite(t, authority, canonicalRel(testSlug, ArtifactStatus), oldStatus, 0o644)

	stage, err := Stage(authority, testSlug, []StageInput{
		{ArtifactID: ArtifactAnalysis, Rel: "analysis.md", Data: []byte("new-analysis"), Mode: 0o644},
		{ArtifactID: ArtifactSpec, Rel: "spec.md", Data: []byte("new-spec"), Mode: 0o644},
		{ArtifactID: ArtifactExploration, Rel: "exploration.md", Data: []byte("new-exploration"), Mode: 0o644},
		{ArtifactID: ArtifactAnalysisSidecar, Rel: "analysis.json", Data: []byte(`{"new":"analysis"}`), Mode: 0o644},
		{ArtifactID: ArtifactArchiveIndex, Rel: "index.json", Data: []byte(`{"new":"index"}`), Mode: 0o644},
		{ArtifactID: ArtifactStatus, Rel: "status.json", Data: []byte(`{"new":"status"}`), Mode: 0o644},
	}, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	indexPre := captureForTest(t, authority, canonicalRel(testSlug, ArtifactArchiveIndex))
	statusPre := captureForTest(t, authority, canonicalRel(testSlug, ArtifactStatus))
	indexRaw, err := WriteRawPreimage(authority, testSlug, ArtifactArchiveIndex, oldIndex, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	statusRaw, err := WriteRawPreimage(authority, testSlug, ArtifactStatus, oldStatus, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	byID := stagedByID(stage)
	entries := make([]Entry, 0, 6)
	for _, id := range []ArtifactID{ArtifactAnalysis, ArtifactSpec, ArtifactExploration, ArtifactAnalysisSidecar} {
		preimage := captureForTest(t, authority, canonicalRel(testSlug, id))
		blobRel := featureRel(testSlug) + "/artifacts/intent-archive/blobs/" + preimage.SHA256 + ".blob"
		if _, err := DurableWrite(authority, WriteRequest{Rel: blobRel, Data: oldIntent[id], Mode: 0o644}, Options{RandomHex12: sequenceHex()}); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, Entry{
			ArtifactID:      id,
			Rel:             canonicalRel(testSlug, id),
			Action:          ActionReplace,
			Preimage:        preimage,
			PreimageBlob:    preimage.SHA256,
			PreimageBlobRel: blobRel,
			NewImage:        byID[id].NewImage,
			StagedRel:       byID[id].Rel,
		})
	}
	entries = append(entries,
		Entry{ArtifactID: ArtifactArchiveIndex, Rel: canonicalRel(testSlug, ArtifactArchiveIndex), Action: ActionReplace, Preimage: indexPre, PreimageRawRel: indexRaw, NewImage: byID[ArtifactArchiveIndex].NewImage, StagedRel: byID[ArtifactArchiveIndex].Rel},
		Entry{ArtifactID: ArtifactStatus, Rel: canonicalRel(testSlug, ArtifactStatus), Action: ActionReplace, Preimage: statusPre, PreimageRawRel: statusRaw, NewImage: byID[ArtifactStatus].NewImage, StagedRel: byID[ArtifactStatus].Rel},
	)
	plan, err := NewPlan(testSlug, ModeRegenerate, stage.StageRel, entries)
	if err != nil {
		t.Fatal(err)
	}
	state := &renameFailureState{failDestination: canonicalRel(testSlug, ArtifactStatus)}
	result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &failingRenameOps{RootOps: NewRootOps(root), state: state}
		},
	})
	assertCode(t, err, CodeRootedWrite)
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("result = %#v", result)
	}
	for id, data := range oldIntent {
		if string(rootRead(t, authority, canonicalRel(testSlug, id))) != string(data) {
			t.Fatalf("%s was not restored", id)
		}
	}
	if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactArchiveIndex))) != string(oldIndex) ||
		string(rootRead(t, authority, canonicalRel(testSlug, ArtifactStatus))) != string(oldStatus) {
		t.Fatal("rollback did not restore archive/raw metadata preimages")
	}
}

func TestRecoveryEvidenceTableAndIdempotency(t *testing.T) {
	tests := []struct {
		name         string
		crashPoint   CrashPoint
		afterCount   int
		wantRestore  int
		wantComplete bool
	}{
		{"cp3-all-preimages", PointAfterJournalDurable, 0, 0, false},
		{"cp4-prefix", PointAfterEntryRename, 2, 2, false},
		{"cp7-all-new", PointAfterAllRenames, 0, 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			plan := stageCreatePlan(t, authority)
			count := 0
			options := Options{
				RandomHex12: sequenceHex(),
				Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
					if point != test.crashPoint {
						return nil
					}
					if point == PointAfterEntryRename {
						count++
						if count != test.afterCount {
							return nil
						}
					}
					return errors.New("crash")
				},
			}
			if _, err := Execute(authority, plan, "0123456789abcdef", nil, options); err == nil {
				t.Fatal("crash hook did not stop execution")
			}
			recovered, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
			if err != nil {
				t.Fatal(err)
			}
			if recovered.Outcome != OutcomeRecovered || len(recovered.Restored) != test.wantRestore || recovered.Completed != test.wantComplete {
				t.Fatalf("recovered = %#v", recovered)
			}
			second, err := Recover(authority, testSlug, Options{})
			if err != nil || second.Outcome != OutcomeRecoveryAbsent {
				t.Fatalf("second recovery = %#v, %v", second, err)
			}
		})
	}
}

func TestRecoveryDivergenceAndNonPrefixPreserveEvidence(t *testing.T) {
	t.Run("third-party", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		count := 0
		options := Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterEntryRename {
					count++
					if count == 2 {
						return errors.New("crash")
					}
				}
				return nil
			},
		}
		_, _ = Execute(authority, plan, "0123456789abcdef", nil, options)
		rootWrite(t, authority, canonicalRel(testSlug, ArtifactAnalysis), []byte("third-party"), 0o644)
		_, err := Recover(authority, testSlug, Options{})
		assertCode(t, err, CodeRecoveryDivergent)
		if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactAnalysis))) != "third-party" ||
			!rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("divergent recovery changed bytes or removed evidence")
		}
	})

	t.Run("mixed-arbitrary-order", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		options := Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterJournalDurable {
					return errors.New("crash")
				}
				return nil
			},
		}
		_, _ = Execute(authority, plan, "0123456789abcdef", nil, options)
		spec := plan.Entries()[1]
		rootWrite(t, authority, spec.Rel, []byte("new-"+string(spec.ArtifactID)), 0o644)
		recovered, err := Recover(authority, testSlug, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if recovered.Outcome != OutcomeRecovered || len(recovered.Restored) != 1 ||
			rootExists(t, authority, spec.Rel) || rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatalf("arbitrary mixture was not recovered: %#v", recovered)
		}
	})
}

func stageCreatePlan(t *testing.T, authority *intentlock.WorkspaceAuthority) Plan {
	t.Helper()
	oldStatus := []byte(`{"state":"requested"}`)
	rootWrite(t, authority, canonicalRel(testSlug, ArtifactStatus), oldStatus, 0o644)
	rootMkdirAll(t, authority, featureRel(testSlug)+"/artifacts", 0o755)
	inputs := []StageInput{
		{ArtifactID: ArtifactAnalysis, Rel: "analysis.md", Data: []byte("new-analysis"), Mode: 0o644},
		{ArtifactID: ArtifactSpec, Rel: "spec.md", Data: []byte("new-spec"), Mode: 0o644},
		{ArtifactID: ArtifactExploration, Rel: "exploration.md", Data: []byte("new-exploration"), Mode: 0o644},
		{ArtifactID: ArtifactAnalysisSidecar, Rel: "analysis.json", Data: []byte(`{"new":"analysis"}`), Mode: 0o644},
		{ArtifactID: ArtifactStatus, Rel: "status.json", Data: []byte(`{"state":"defined"}`), Mode: 0o644},
	}
	stage, err := Stage(authority, testSlug, inputs, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	statusPreimage := captureForTest(t, authority, canonicalRel(testSlug, ArtifactStatus))
	statusRaw, err := WriteRawPreimage(authority, testSlug, ArtifactStatus, oldStatus, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]Entry, 0, len(stage.Files))
	for _, file := range stage.Files {
		entry := Entry{
			ArtifactID: file.ArtifactID,
			Rel:        canonicalRel(testSlug, file.ArtifactID),
			Action:     ActionCreate,
			Preimage:   AbsentIdentity(),
			NewImage:   file.NewImage,
			StagedRel:  file.Rel,
		}
		if file.ArtifactID == ArtifactStatus {
			entry.Action = ActionReplace
			entry.Preimage = statusPreimage
			entry.PreimageRawRel = statusRaw
		}
		entries = append(entries, entry)
	}
	plan, err := NewPlan(testSlug, ModeGenerate, stage.StageRel, entries)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func stagedByID(stage StageResult) map[ArtifactID]StagedFile {
	result := make(map[ArtifactID]StagedFile, len(stage.Files))
	for _, file := range stage.Files {
		result[file.ArtifactID] = file
	}
	return result
}

func captureForTest(t *testing.T, authority *intentlock.WorkspaceAuthority, rel string) Identity {
	t.Helper()
	identity, err := CaptureIdentity(authority, rel, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func fixedHex(value string) func() (string, error) {
	return func() (string, error) { return value, nil }
}

func sequenceHex() func() (string, error) {
	counter := 0
	return func() (string, error) {
		counter++
		return strings.Repeat("0", 11) + string("0123456789abcdef"[counter%16]), nil
	}
}

func assertCode(t *testing.T, err error, code Code) {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("error = %#v, want %s", err, code)
	}
}

func writeRootFile(root *os.Root, rel string, data []byte, mode fs.FileMode) error {
	file, err := root.OpenFile(rel, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

type recordingOps struct {
	RootOps
	renames *[]string
}

func (ops *recordingOps) Rename(oldName, newName string) error {
	*ops.renames = append(*ops.renames, newName)
	return ops.RootOps.Rename(oldName, newName)
}

type renameFailureState struct {
	canonicalCount  int
	failCanonicalAt int
	failDestination string
	failed          bool
}

type failingRenameOps struct {
	RootOps
	state *renameFailureState
}

func (ops *failingRenameOps) Rename(oldName, newName string) error {
	if strings.HasPrefix(newName, featureRel(testSlug)+"/") {
		ops.state.canonicalCount++
		if !ops.state.failed &&
			((ops.state.failCanonicalAt > 0 && ops.state.canonicalCount == ops.state.failCanonicalAt) ||
				(ops.state.failDestination != "" && newName == ops.state.failDestination)) {
			ops.state.failed = true
			return errors.New("injected rename failure")
		}
	}
	return ops.RootOps.Rename(oldName, newName)
}

package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"testing"
)

func TestNamedInjectionSeamsReachPublicationBoundaries(t *testing.T) {
	oldBeforeJournalWrite := beforeJournalWrite
	oldAfterJournalWrite := afterJournalWrite
	oldBeforeControlWriteRename := beforeControlWriteRename
	oldBeforeEntryCAS := beforeEntryCAS
	oldBeforeRename := beforeRename
	oldAfterRename := afterRename
	oldBeforeStatusRename := beforeStatusRename
	oldAfterStatusRename := afterStatusRename
	oldBeforeFinalVerify := beforeFinalVerify
	oldBeforeJournalClear := beforeJournalClear
	oldFailFsync := failFsync
	oldFailRename := failRename
	t.Cleanup(func() {
		beforeJournalWrite = oldBeforeJournalWrite
		afterJournalWrite = oldAfterJournalWrite
		beforeControlWriteRename = oldBeforeControlWriteRename
		beforeEntryCAS = oldBeforeEntryCAS
		beforeRename = oldBeforeRename
		afterRename = oldAfterRename
		beforeStatusRename = oldBeforeStatusRename
		afterStatusRename = oldAfterStatusRename
		beforeFinalVerify = oldBeforeFinalVerify
		beforeJournalClear = oldBeforeJournalClear
		failFsync = oldFailFsync
		failRename = oldFailRename
	})
	_, authority := acquireWorkspace(t)
	journalWrites := []string{}
	journalWritten := []string{}
	controlRenames := []string{}
	entryCAS := []int{}
	renameBefore := []int{}
	renameAfter := []int{}
	statusBefore := []string{}
	statusAfter := []string{}
	finalVerify := 0
	journalClear := []string{}
	order := []string{}
	beforeJournalWrite = func(rel string) { journalWrites = append(journalWrites, rel) }
	afterJournalWrite = func(rel string) { journalWritten = append(journalWritten, rel) }
	beforeControlWriteRename = func(rel string) { controlRenames = append(controlRenames, rel) }
	beforeEntryCAS = func(index int) {
		entryCAS = append(entryCAS, index)
		order = append(order, "named")
	}
	beforeRename = func(index int) { renameBefore = append(renameBefore, index) }
	afterRename = func(index int) { renameAfter = append(renameAfter, index) }
	beforeStatusRename = func(rel string) { statusBefore = append(statusBefore, rel) }
	afterStatusRename = func(rel string) { statusAfter = append(statusAfter, rel) }
	beforeFinalVerify = func() { finalVerify++ }
	beforeJournalClear = func(rel string) { journalClear = append(journalClear, rel) }
	plan := stageCreatePlan(t, authority)

	result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
			if point == PointBeforeEntryCAS {
				order = append(order, "options")
			}
			return nil
		},
	})
	if err != nil || result.Outcome != OutcomePublished {
		t.Fatalf("publication = result=%+v err=%v", result, err)
	}
	wantIndexes := make([]int, len(plan.Entries()))
	for index := range wantIndexes {
		wantIndexes[index] = index
	}
	if !reflect.DeepEqual(entryCAS, wantIndexes) ||
		!reflect.DeepEqual(renameBefore, wantIndexes) ||
		!reflect.DeepEqual(renameAfter, wantIndexes) {
		t.Fatalf("indexed seams = cas=%v before=%v after=%v want=%v", entryCAS, renameBefore, renameAfter, wantIndexes)
	}
	for index := 0; index < len(order); index += 2 {
		if index+1 >= len(order) || order[index] != "named" || order[index+1] != "options" {
			t.Fatalf("named/Options.Hook order = %v", order)
		}
	}

	wantControl := []string{
		laneRel(testSlug) + "/stage-000000000001/analysis.md",
		laneRel(testSlug) + "/stage-000000000001/spec.md",
		laneRel(testSlug) + "/stage-000000000001/exploration.md",
		laneRel(testSlug) + "/stage-000000000001/analysis.json",
		laneRel(testSlug) + "/stage-000000000001/status.json",
		laneRel(testSlug) + "/status.preimage.json",
		JournalRel(testSlug),
		JournalMarkerRel(testSlug),
	}
	if len(journalWrites) != 1 || journalWrites[0] != JournalRel(testSlug) ||
		!reflect.DeepEqual(journalWritten, journalWrites) ||
		!reflect.DeepEqual(controlRenames, wantControl) ||
		len(statusBefore) != 1 || len(statusAfter) != 1 ||
		statusBefore[0] != canonicalRel(testSlug, ArtifactStatus) ||
		!reflect.DeepEqual(statusAfter, statusBefore) ||
		finalVerify != 1 ||
		len(journalClear) != 1 || journalClear[0] != JournalMarkerRel(testSlug) {
		t.Fatalf(
			"non-indexed seams = journal-before=%v journal-after=%v control=%v status=%v/%v final=%d clear=%v",
			journalWrites, journalWritten, controlRenames, statusBefore, statusAfter, finalVerify, journalClear,
		)
	}

	failFsyncCalls := 0
	failFsync = func(path string) error {
		if path == "" {
			t.Fatal("failFsync received an empty path")
		}
		failFsyncCalls++
		if failFsyncCalls == 1 {
			return errors.New("injected fsync failure")
		}
		return nil
	}
	rel := ".tpatch/local/intent-prepare/injection/fsync.json"
	writeResult, writeErr := DurableWrite(authority, WriteRequest{
		Rel: rel, Data: []byte("body"), Mode: 0o600, Role: WriteRoleOrdinaryCanonical,
	}, Options{RandomHex12: fixedHex("111111111111")})
	assertCode(t, writeErr, CodeRootedWrite)
	if writeResult.Committed || failFsyncCalls == 0 {
		t.Fatalf("failFsync = result=%+v calls=%d err=%v", writeResult, failFsyncCalls, writeErr)
	}

	failFsync = nil
	failRenameCalls := 0
	failRename = func(path string) error {
		if path == "" {
			t.Fatal("failRename received an empty path")
		}
		failRenameCalls++
		return errors.New("injected rename failure")
	}
	rel = ".tpatch/local/intent-prepare/injection/rename.json"
	writeResult, writeErr = DurableWrite(authority, WriteRequest{
		Rel: rel, Data: []byte("body"), Mode: 0o600, Role: WriteRoleOrdinaryCanonical,
	}, Options{RandomHex12: fixedHex("222222222222")})
	assertCode(t, writeErr, CodeRootedWrite)
	if writeResult.Committed || failRenameCalls != 1 {
		t.Fatalf("failRename = result=%+v calls=%d err=%v", writeResult, failRenameCalls, writeErr)
	}
}

func TestWriteRoleSeamSequencesRegenerateManualAndRecovery(t *testing.T) {
	t.Run("regenerate", func(t *testing.T) {
		recorded := installWriteRoleRecorders(t)
		_, authority := acquireWorkspace(t)
		plan := stageRegeneratePlan(t, authority)
		result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
		})
		if err != nil || result.Outcome != OutcomePublished {
			t.Fatalf("regenerate = result=%+v err=%v", result, err)
		}
		stage := laneRel(testSlug) + "/stage-000000000001/"
		wantControl := []string{
			stage + "analysis.md",
			stage + "spec.md",
			stage + "exploration.md",
			stage + "analysis.json",
			stage + "index.json",
			stage + "status.json",
			laneRel(testSlug) + "/index.preimage.json",
			laneRel(testSlug) + "/status.preimage.json",
			JournalRel(testSlug),
			JournalMarkerRel(testSlug),
		}
		wantIndexes := []int{0, 1, 2, 3, 4, 5}
		wantStatus := []string{canonicalRel(testSlug, ArtifactStatus)}
		if !reflect.DeepEqual(recorded.control, wantControl) ||
			!reflect.DeepEqual(recorded.statusBefore, wantStatus) ||
			!reflect.DeepEqual(recorded.statusAfter, wantStatus) ||
			!reflect.DeepEqual(recorded.indexBefore, wantIndexes) ||
			!reflect.DeepEqual(recorded.indexAfter, wantIndexes) {
			t.Fatalf("regenerate roles = %+v", recorded)
		}
	})

	t.Run("manual-status", func(t *testing.T) {
		recorded := installWriteRoleRecorders(t)
		_, authority := acquireWorkspace(t)
		rel := canonicalRel(testSlug, ArtifactStatus)
		rootMkdirAll(t, authority, pathDir(rel), 0o755)
		rootWrite(t, authority, rel, []byte("before"), 0o644)
		expected := captureForTest(t, authority, rel)
		result, err := DurableWrite(authority, WriteRequest{
			Rel:        rel,
			Data:       []byte("after"),
			Mode:       0o644,
			Expected:   identityPointer(expected),
			ArtifactID: ArtifactStatus,
			Role:       WriteRoleCanonicalStatus,
		}, Options{RandomHex12: fixedHex("123456789abc")})
		wantStatus := []string{rel}
		if err != nil || !result.Committed ||
			!reflect.DeepEqual(recorded.statusBefore, wantStatus) ||
			!reflect.DeepEqual(recorded.statusAfter, wantStatus) ||
			len(recorded.control) != 0 ||
			len(recorded.indexBefore) != 0 || len(recorded.indexAfter) != 0 {
			t.Fatalf("manual status roles = result=%+v recorded=%+v err=%v", result, recorded, err)
		}
	})

	t.Run("recovery-status-rollback", func(t *testing.T) {
		recorded := installWriteRoleRecorders(t)
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		journal, err := BuildJournal(plan, "0123456789abcdef")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := PersistJournal(authority, journal, Options{RandomHex12: sequenceHex()}); err != nil {
			t.Fatal(err)
		}
		status, ok := findEntry(plan.Entries(), ArtifactStatus)
		if !ok {
			t.Fatal("recovery plan has no status entry")
		}
		rootWrite(t, authority, status.Rel, rootRead(t, authority, status.StagedRel), fs.FileMode(status.NewImage.Mode))
		recorded.reset()

		result, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
		wantStatus := []string{status.Rel}
		wantControl := []string{JournalMarkerRel(testSlug)}
		if err != nil || result.Outcome != OutcomeRecovered ||
			!reflect.DeepEqual(result.Restored, []ArtifactID{ArtifactStatus}) ||
			!reflect.DeepEqual(recorded.statusBefore, wantStatus) ||
			!reflect.DeepEqual(recorded.statusAfter, wantStatus) ||
			!reflect.DeepEqual(recorded.control, wantControl) ||
			len(recorded.indexBefore) != 0 || len(recorded.indexAfter) != 0 {
			t.Fatalf("recovery roles = result=%+v recorded=%+v err=%v", result, recorded, err)
		}
	})
}

func TestWriteRoleValidationFailsClosed(t *testing.T) {
	tests := []WriteRequest{
		{
			Rel:  ".tpatch/features/roles/invalid.json",
			Data: []byte("invalid"),
			Mode: 0o644,
			Role: WriteRole(255),
		},
		{
			Rel:        ".tpatch/features/roles/status.json",
			Data:       []byte("status"),
			Mode:       0o644,
			ArtifactID: ArtifactStatus,
			Role:       WriteRoleOrdinaryCanonical,
		},
		{
			Rel:        ".tpatch/features/roles/analysis.md",
			Data:       []byte("analysis"),
			Mode:       0o644,
			ArtifactID: ArtifactAnalysis,
			Role:       WriteRoleCanonicalStatus,
		},
	}
	for index, request := range tests {
		_, authority := acquireWorkspace(t)
		_, err := DurableWrite(authority, request, Options{RandomHex12: sequenceHex()})
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != CodeInvalidPlan || typed.Class != "write-role" {
			t.Fatalf("case %d error = %#v", index, err)
		}
		if rootExists(t, authority, request.Rel) {
			t.Fatalf("case %d invalid role wrote %s", index, request.Rel)
		}
	}

	_, authority := acquireWorkspace(t)
	var omitted WriteRequest
	omitted.Rel = ".tpatch/features/roles/omitted.json"
	omitted.Data = []byte("omitted")
	omitted.Mode = 0o644
	_, err := DurableWrite(authority, omitted, Options{RandomHex12: sequenceHex()})
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeInvalidPlan || typed.Class != "write-role" ||
		rootExists(t, authority, omitted.Rel) {
		t.Fatalf("omitted role = error=%#v exists=%t", err, rootExists(t, authority, omitted.Rel))
	}
}

type writeRoleRecorders struct {
	control      []string
	statusBefore []string
	statusAfter  []string
	indexBefore  []int
	indexAfter   []int
}

func installWriteRoleRecorders(t *testing.T) *writeRoleRecorders {
	t.Helper()
	oldControl := beforeControlWriteRename
	oldStatusBefore := beforeStatusRename
	oldStatusAfter := afterStatusRename
	oldIndexBefore := beforeRename
	oldIndexAfter := afterRename
	t.Cleanup(func() {
		beforeControlWriteRename = oldControl
		beforeStatusRename = oldStatusBefore
		afterStatusRename = oldStatusAfter
		beforeRename = oldIndexBefore
		afterRename = oldIndexAfter
	})
	recorded := &writeRoleRecorders{}
	beforeControlWriteRename = func(rel string) { recorded.control = append(recorded.control, rel) }
	beforeStatusRename = func(rel string) { recorded.statusBefore = append(recorded.statusBefore, rel) }
	afterStatusRename = func(rel string) { recorded.statusAfter = append(recorded.statusAfter, rel) }
	beforeRename = func(index int) { recorded.indexBefore = append(recorded.indexBefore, index) }
	afterRename = func(index int) { recorded.indexAfter = append(recorded.indexAfter, index) }
	return recorded
}

func (recorded *writeRoleRecorders) reset() {
	recorded.control = nil
	recorded.statusBefore = nil
	recorded.statusAfter = nil
	recorded.indexBefore = nil
	recorded.indexAfter = nil
}

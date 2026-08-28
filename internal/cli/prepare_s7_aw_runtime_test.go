//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type s7AWMutationSpyStorage struct {
	store.IntentArchiveStorage
	removed   *[]string
	indexCASs *int
}

func (spy *s7AWMutationSpyStorage) RemoveBlob(
	blobRel string,
	expected store.IntentArchiveIdentityToken,
) (store.IntentArchiveMutationResult, error) {
	if spy.removed != nil {
		*spy.removed = append(*spy.removed, blobRel)
	}
	return spy.IntentArchiveStorage.RemoveBlob(blobRel, expected)
}

func (spy *s7AWMutationSpyStorage) CASIndex(
	indexRel string,
	expected store.IntentArchiveIdentityToken,
	canonical []byte,
) (store.IntentArchiveMutationResult, error) {
	if spy.indexCASs != nil {
		(*spy.indexCASs)++
	}
	return spy.IntentArchiveStorage.CASIndex(indexRel, expected, canonical)
}

func s7AWRunPurgeWithMutationSpy(
	t *testing.T,
	archive s7AVRepairArchive,
	selector []string,
) (int, string, string, intentArchivePurgeReport, []string, int) {
	t.Helper()
	removed := []string{}
	indexCASs := 0
	previous := intentArchiveNewStorage
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		root *os.Root,
	) store.IntentArchiveStorage {
		return &s7AWMutationSpyStorage{
			IntentArchiveStorage: previous(authority, root),
			removed:              &removed,
			indexCASs:            &indexCASs,
		}
	}
	restore := s7AVRestoreSeam(t, func() { intentArchiveNewStorage = previous })
	code, stdout, stderr, _ := runPrepare(
		t,
		s7ASPurgeArgs(
			archive.root, archive.slug, selector, true, true, true,
		)...,
	)
	restore()
	return code, stdout, stderr, decodeIntentArchivePurgeReport(t, stdout),
		removed, indexCASs
}

func s7AWRepairAdvisoryCount(report intentArchivePurgeReport) int {
	count := 0
	for _, advisory := range report.Advisories {
		if advisory.Code == "archive-repairs-remaining" {
			count++
		}
	}
	return count
}

func s7AWAssertPrepareRoutes(
	t *testing.T,
	archive s7AVRepairArchive,
	wantRoutes []string,
	forbidRoutes []string,
) {
	t.Helper()
	before := archive.tree(t)
	code, stdout, _, _ := runPrepare(
		t, "--path", archive.root, "prepare", archive.slug, "--json", "--quiet",
	)
	if code != 3 {
		t.Fatalf("ordinary prepare exit=%d, want 3\n%s", code, stdout)
	}
	for _, route := range wantRoutes {
		if !strings.Contains(stdout, route) {
			t.Fatalf("ordinary prepare omitted route %q\n%s", route, stdout)
		}
	}
	for _, route := range forbidRoutes {
		if strings.Contains(stdout, route) {
			t.Fatalf("ordinary prepare retained completed route %q\n%s", route, stdout)
		}
	}
	if !bytes.Equal(before, archive.tree(t)) {
		t.Fatal("ordinary prepare changed the archive while repair work remained")
	}
}

func s7AWAssertRepairStage(
	t *testing.T,
	label string,
	stage intentArchiveRepairStageReport,
	ordinal int,
	kind string,
	class string,
	hashes []string,
	paths []string,
	repair string,
	resulting []string,
	afterPrerequisite bool,
) {
	t.Helper()
	sortedHashes := append([]string(nil), hashes...)
	sortedPaths := append([]string(nil), paths...)
	sort.Strings(sortedHashes)
	sort.Strings(sortedPaths)
	if stage.Ordinal != ordinal ||
		stage.Kind != kind ||
		stage.Class != class ||
		!reflect.DeepEqual(stage.Hashes, sortedHashes) ||
		!reflect.DeepEqual(stage.Paths, sortedPaths) ||
		stage.Repair != repair ||
		stage.RepairCWD != store.IntentArchiveRepairCWD ||
		!reflect.DeepEqual(stage.ResultingClasses, resulting) ||
		stage.AfterPrerequisite != afterPrerequisite {
		t.Fatalf("%s stage = %#v", label, stage)
	}
}

// ─── PIB-552 ──────────────────────────────────────────────────────────────────

func TestS7AWTwoClassSequentialRepairContracts(t *testing.T) {
	archive := s7AVWriteRepairArchive(
		t, "PIB-552", s7AVRepairSpec{residues: 3, mixed: 2, ready: true},
	)
	residueRepair := s7AVOrphanPurgeCommand(archive.slug)
	mixedRepair := s7AVBlobPurgeCommand(archive.slug, archive.mixed)

	s7AWAssertPrepareRoutes(
		t, archive,
		[]string{residueRepair, mixedRepair},
		nil,
	)

	indexBefore := archive.indexBytes(t)
	mixedBefore := map[string][]byte{}
	for _, hash := range archive.mixed {
		data, err := os.ReadFile(archive.abs(archive.blobRel[hash]))
		if err != nil {
			t.Fatal(err)
		}
		mixedBefore[hash] = data
	}

	invocations := 0
	code, stdout, stderr, first, removed, indexCASs :=
		s7AWRunPurgeWithMutationSpy(t, archive, []string{"--orphans"})
	invocations++
	if code != 0 || stderr != "" ||
		first.Outcome != string(store.IntentArchivePurgePurged) ||
		first.Action != "none" {
		t.Fatalf("PIB-552 first repair = exit:%d stderr:%q report:%#v\n%s",
			code, stderr, first, stdout)
	}
	wantRemoved := []string{}
	for _, hash := range archive.residues {
		wantRemoved = append(wantRemoved, archive.blobRel[hash])
		if _, err := os.Lstat(archive.abs(archive.blobRel[hash])); !os.IsNotExist(err) {
			t.Fatalf("PIB-552 residue %s survived: %v", hash, err)
		}
	}
	sort.Strings(removed)
	sort.Strings(wantRemoved)
	if !reflect.DeepEqual(removed, wantRemoved) || indexCASs != 0 {
		t.Fatalf("PIB-552 first write set = removed:%v index-CAS:%d, want %v/0",
			removed, indexCASs, wantRemoved)
	}
	if !bytes.Equal(indexBefore, archive.indexBytes(t)) {
		t.Fatal("PIB-552 first repair changed index.json bytes")
	}
	for hash, before := range mixedBefore {
		after, err := os.ReadFile(archive.abs(archive.blobRel[hash]))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("PIB-552 first repair changed mixed hash %s: %v", hash, err)
		}
	}
	if first.RemainingRepairs == nil ||
		first.RemainingRepairs.RepairedClass != string(store.IntentArchiveRepairUnreferencedResidue) ||
		first.RemainingRepairs.StagesRemaining != 1 ||
		first.RemainingRepairs.NextStage == nil ||
		first.RemainingRepairs.NextStage.Ordinal != 1 ||
		first.RemainingRepairs.NextStage.Kind != string(store.IntentArchiveRepairStagePurge) ||
		first.RemainingRepairs.NextStage.Class != string(store.IntentArchiveRepairMixedReference) ||
		len(first.RemainingRepairs.Stages) != 1 ||
		s7AWRepairAdvisoryCount(first) != 1 {
		t.Fatalf("PIB-552 first remaining repairs = %#v advisories=%#v",
			first.RemainingRepairs, first.Advisories)
	}
	mixedPaths := []string{}
	for _, hash := range archive.mixed {
		mixedPaths = append(mixedPaths, archive.blobRel[hash])
	}
	s7AWAssertRepairStage(
		t, "PIB-552 first",
		first.RemainingRepairs.Stages[0],
		1,
		string(store.IntentArchiveRepairStagePurge),
		string(store.IntentArchiveRepairMixedReference),
		archive.mixed,
		mixedPaths,
		mixedRepair,
		[]string{},
		false,
	)

	s7AWAssertPrepareRoutes(
		t, archive,
		[]string{mixedRepair},
		[]string{residueRepair},
	)

	code, stdout, stderr, second, removed, _ :=
		s7AWRunPurgeWithMutationSpy(t, archive, s7AVBlobSelector(archive.mixed))
	invocations++
	if code != 0 || stderr != "" ||
		second.Outcome != string(store.IntentArchivePurgePurged) ||
		second.Action != "none" ||
		second.RemainingRepairs != nil ||
		s7AWRepairAdvisoryCount(second) != 0 {
		t.Fatalf("PIB-552 second repair = exit:%d stderr:%q report:%#v\n%s",
			code, stderr, second, stdout)
	}
	sort.Strings(removed)
	sort.Strings(mixedPaths)
	if !reflect.DeepEqual(removed, mixedPaths) {
		t.Fatalf("PIB-552 second repair removed %v, want %v", removed, mixedPaths)
	}
	for _, hash := range archive.mixed {
		if _, err := os.Lstat(archive.abs(archive.blobRel[hash])); !os.IsNotExist(err) {
			t.Fatalf("PIB-552 mixed blob %s survived: %v", hash, err)
		}
		_, index := readIntentArchiveCLIIndex(t, archive.root, archive.slug)
		for _, state := range s7ATWireStates(index, hash) {
			if state != store.IntentArchiveWireTombstoned {
				t.Fatalf("PIB-552 mixed hash %s retained wire state %s", hash, state)
			}
		}
	}
	if invocations != 2 {
		t.Fatalf("PIB-552 repair invocation count = %d, want 2", invocations)
	}
	s7AVAssertArchiveRepaired(t, archive, "PIB-552")
}

// ─── PIB-553 ──────────────────────────────────────────────────────────────────

func s7AWAssertCorruptFirstRefusal(
	t *testing.T,
	archive s7AVRepairArchive,
	selector []string,
	wantRemaining *intentArchiveRemainingRepairsReport,
) *intentArchiveRemainingRepairsReport {
	t.Helper()
	before := archive.tree(t)
	code, stdout, _, _ := runPrepare(
		t,
		s7ASPurgeArgs(
			archive.root, archive.slug, selector, true, true, true,
		)...,
	)
	if code != 3 {
		t.Fatalf("PIB-553 selector %v exit=%d, want 3\n%s", selector, code, stdout)
	}
	if !bytes.Equal(before, archive.tree(t)) {
		t.Fatalf("PIB-553 selector %v changed the archive before the prerequisite", selector)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	if report.Refusal == nil ||
		report.Refusal.Code != string(store.IntentArchiveCodeBlobCorrupt) ||
		report.RemainingRepairs == nil {
		t.Fatalf("PIB-553 selector %v refusal = %#v / %#v",
			selector, report.Refusal, report.RemainingRepairs)
	}
	if wantRemaining != nil && !reflect.DeepEqual(report.RemainingRepairs, wantRemaining) {
		t.Fatalf("PIB-553 selector %v changed the shared stage plan:\n got %#v\nwant %#v",
			selector, report.RemainingRepairs, wantRemaining)
	}
	return report.RemainingRepairs
}

func s7AWRunThreeClassOrder(t *testing.T, order string) int {
	t.Helper()
	archive := s7AVWriteRepairArchive(
		t, "PIB-553-"+order,
		s7AVRepairSpec{residues: 2, dangling: 2, corrupt: true, ready: true},
	)
	oldDangling := append([]string(nil), archive.dangling...)
	allDangling := append(append([]string(nil), archive.dangling...), archive.corrupt)
	sort.Strings(allDangling)
	danglingRepair := s7AVBlobPurgeCommand(archive.slug, allDangling)
	residueRepair := s7AVOrphanPurgeCommand(archive.slug)

	var remaining *intentArchiveRemainingRepairsReport
	for _, selector := range [][]string{
		{"--orphans"},
		s7AVBlobSelector(oldDangling),
		{"--blob", archive.corrupt},
		{"--generation", archive.generations[archive.corrupt]},
		{"--all"},
	} {
		remaining = s7AWAssertCorruptFirstRefusal(t, archive, selector, remaining)
	}
	if remaining == nil ||
		remaining.RepairedClass != "" ||
		remaining.StagesRemaining != 3 ||
		len(remaining.Stages) != 3 ||
		remaining.NextStage == nil ||
		remaining.NextStage.Ordinal != 1 ||
		remaining.NextStage.Kind != string(store.IntentArchiveRepairStageManual) ||
		remaining.NextStage.Class != string(store.IntentArchiveRepairCorruptObject) {
		t.Fatalf("PIB-553 initial stage plan = %#v", remaining)
	}
	corruptPath := archive.blobRel[archive.corrupt]
	warning, removal := s7AVExtractRemovalLine(remaining.Stages[0].Repair)
	if err := s7AVValidatePrintedRemoval(s7AVPrintedProcedure{
		label:          "PIB-553 corrupt prerequisite",
		block:          remaining.Stages[0].Repair + "\n" + s7AVGitHistoryCaveat,
		warning:        warning,
		removeCommand:  removal,
		blobRel:        corruptPath,
		historyCaveats: []string{s7AVGitHistoryCaveat},
	}); err != nil {
		t.Fatal(err)
	}
	s7AWAssertRepairStage(
		t, "PIB-553 corrupt",
		remaining.Stages[0],
		1,
		string(store.IntentArchiveRepairStageManual),
		string(store.IntentArchiveRepairCorruptObject),
		[]string{archive.corrupt},
		[]string{corruptPath},
		remaining.Stages[0].Repair,
		[]string{string(store.IntentArchiveRepairDanglingReference)},
		false,
	)
	danglingPaths := []string{}
	for _, hash := range allDangling {
		danglingPaths = append(danglingPaths, archive.blobRel[hash])
	}
	s7AWAssertRepairStage(
		t, "PIB-553 dangling",
		remaining.Stages[1],
		2,
		string(store.IntentArchiveRepairStagePurge),
		string(store.IntentArchiveRepairDanglingReference),
		allDangling,
		danglingPaths,
		danglingRepair,
		[]string{},
		true,
	)
	residuePaths := []string{}
	for _, hash := range archive.residues {
		residuePaths = append(residuePaths, archive.blobRel[hash])
	}
	s7AWAssertRepairStage(
		t, "PIB-553 residue",
		remaining.Stages[2],
		3,
		string(store.IntentArchiveRepairStagePurge),
		string(store.IntentArchiveRepairUnreferencedResidue),
		archive.residues,
		residuePaths,
		residueRepair,
		[]string{},
		false,
	)

	s7AVExecutePrintedRemoval(t, "PIB-553 manual prerequisite", archive.root, removal)
	if _, err := os.Lstat(archive.abs(corruptPath)); !os.IsNotExist(err) {
		t.Fatalf("PIB-553 manual prerequisite left the corrupt object: %v", err)
	}

	// Once the prerequisite has run, --all is still withheld while the
	// dangling and residue classes coexist.
	beforeAll := archive.tree(t)
	allCode, allStdout, _, _ := runPrepare(
		t,
		s7ASPurgeArgs(
			archive.root, archive.slug, []string{"--all"}, true, true, true,
		)...,
	)
	if allCode != 3 || !bytes.Equal(beforeAll, archive.tree(t)) {
		t.Fatalf("PIB-553 post-prerequisite --all = exit:%d\n%s", allCode, allStdout)
	}

	// A selection copied before the prerequisite omits its newly dangling hash
	// and must not partially repair the enlarged class.
	beforePartial := archive.tree(t)
	partialCode, partialStdout, _, _ := runPrepare(
		t,
		s7ASPurgeArgs(
			archive.root, archive.slug, s7AVBlobSelector(oldDangling), true, true, true,
		)...,
	)
	if partialCode != 3 || !bytes.Equal(beforePartial, archive.tree(t)) {
		t.Fatalf("PIB-553 stale dangling selection = exit:%d\n%s", partialCode, partialStdout)
	}

	invocations := 0
	runDangling := func() intentArchivePurgeReport {
		code, stdout, stderr, report, removed, _ :=
			s7AWRunPurgeWithMutationSpy(t, archive, s7AVBlobSelector(allDangling))
		invocations++
		if code != 0 || stderr != "" ||
			report.Outcome != string(store.IntentArchivePurgePurged) ||
			len(removed) != 0 {
			t.Fatalf("PIB-553 dangling repair = exit:%d stderr:%q removed:%v report:%#v\n%s",
				code, stderr, removed, report, stdout)
		}
		return report
	}
	runResidue := func() intentArchivePurgeReport {
		code, stdout, stderr, report, removed, indexCASs :=
			s7AWRunPurgeWithMutationSpy(t, archive, []string{"--orphans"})
		invocations++
		wantRemoved := append([]string(nil), residuePaths...)
		sort.Strings(removed)
		sort.Strings(wantRemoved)
		if code != 0 || stderr != "" ||
			report.Outcome != string(store.IntentArchivePurgePurged) ||
			!reflect.DeepEqual(removed, wantRemoved) || indexCASs != 0 {
			t.Fatalf("PIB-553 residue repair = exit:%d stderr:%q removed:%v index-CAS:%d report:%#v\n%s",
				code, stderr, removed, indexCASs, report, stdout)
		}
		return report
	}

	var first, second intentArchivePurgeReport
	switch order {
	case "dangling-then-residue":
		first = runDangling()
		second = runResidue()
	case "residue-then-dangling":
		first = runResidue()
		second = runDangling()
	default:
		t.Fatalf("unknown PIB-553 repair order %q", order)
	}
	if first.RemainingRepairs == nil ||
		first.RemainingRepairs.StagesRemaining != 1 ||
		s7AWRepairAdvisoryCount(first) != 1 {
		t.Fatalf("PIB-553 first repair did not report the untouched class: %#v", first)
	}
	if second.RemainingRepairs != nil || s7AWRepairAdvisoryCount(second) != 0 {
		t.Fatalf("PIB-553 final repair left a stage: %#v", second)
	}
	if invocations != 2 {
		t.Fatalf("PIB-553 tpatch repair invocations = %d, want 2", invocations)
	}
	s7AVAssertArchiveRepaired(t, archive, "PIB-553 "+order)
	return invocations
}

func TestS7AWThreeClassCorruptFirstRepairContracts(t *testing.T) {
	for _, order := range []string{
		"dangling-then-residue",
		"residue-then-dangling",
	} {
		if invocations := s7AWRunThreeClassOrder(t, order); invocations != 2 {
			t.Fatalf("PIB-553 %s repair invocations = %d, want 2", order, invocations)
		}
	}
}

// ─── PIB-556 ──────────────────────────────────────────────────────────────────

type s7AWRemainingExpectation struct {
	exit          int
	repairedClass string
	classes       []string
}

func s7AWAssertRemainingCarrier(
	t *testing.T,
	label string,
	root string,
	stdout string,
	human string,
	report intentArchivePurgeReport,
	want s7AWRemainingExpectation,
) {
	t.Helper()
	remaining := report.RemainingRepairs
	if remaining == nil || !remaining.RerunRequired ||
		remaining.StagesRemaining != len(remaining.Stages) ||
		remaining.StagesRemaining != len(want.classes) ||
		remaining.RepairedClass != want.repairedClass ||
		remaining.NextStage == nil ||
		len(remaining.Stages) == 0 {
		t.Fatalf("%s remaining repairs = %#v", label, remaining)
	}
	if remaining.NextStage.Ordinal != remaining.Stages[0].Ordinal ||
		remaining.NextStage.Kind != remaining.Stages[0].Kind ||
		remaining.NextStage.Class != remaining.Stages[0].Class {
		t.Fatalf("%s next_stage does not name stages[0]: %#v / %#v",
			label, remaining.NextStage, remaining.Stages[0])
	}
	previousHumanStage := -1
	for index, stage := range remaining.Stages {
		if stage.Ordinal != index+1 ||
			stage.Class != want.classes[index] ||
			stage.RepairCWD != store.IntentArchiveRepairCWD ||
			stage.Repair == "" ||
			!sort.StringsAreSorted(stage.Hashes) ||
			!sort.StringsAreSorted(stage.Paths) {
			t.Fatalf("%s stage %d = %#v", label, index, stage)
		}
		humanStage := strings.Index(
			human,
			fmt.Sprintf("stage %d: %s (%s)", stage.Ordinal, stage.Class, stage.Kind),
		)
		if humanStage <= previousHumanStage {
			t.Fatalf("%s human stage order diverges at ordinal %d\n%s",
				label, stage.Ordinal, human)
		}
		previousHumanStage = humanStage
		if stage.Kind == string(store.IntentArchiveRepairStageManual) {
			if stage.Class != string(store.IntentArchiveRepairCorruptObject) ||
				len(stage.ResultingClasses) == 0 {
				t.Fatalf("%s invalid manual stage = %#v", label, stage)
			}
		} else if stage.Kind != string(store.IntentArchiveRepairStagePurge) ||
			len(stage.ResultingClasses) != 0 {
			t.Fatalf("%s invalid purge stage = %#v", label, stage)
		}
		for _, path := range stage.Paths {
			wantPrefix := ".tpatch/features/" + report.Slug + "/artifacts/intent-archive/blobs/"
			if !strings.HasPrefix(path, wantPrefix) ||
				strings.Contains(stdout, root) ||
				strings.Contains(human, root) {
				t.Fatalf("%s unsafe stage path %q", label, path)
			}
		}
		for _, token := range []string{
			fmt.Sprintf("stage %d: %s (%s)", stage.Ordinal, stage.Class, stage.Kind),
			"repair_cwd: " + stage.RepairCWD,
			fmt.Sprintf("after prerequisite: %t", stage.AfterPrerequisite),
		} {
			if !strings.Contains(human, token) {
				t.Fatalf("%s human carrier omitted %q\n%s", label, token, human)
			}
		}
		for _, hash := range stage.Hashes {
			if !strings.Contains(human, "hash: "+hash) {
				t.Fatalf("%s human carrier omitted hash %s\n%s", label, hash, human)
			}
		}
		for _, path := range stage.Paths {
			if !strings.Contains(human, "path: "+path) {
				t.Fatalf("%s human carrier omitted path %s\n%s", label, path, human)
			}
		}
		for _, line := range strings.Split(stage.Repair, "\n") {
			if !strings.Contains(human, strings.TrimSpace(line)) {
				t.Fatalf("%s human carrier omitted repair line %q\n%s",
					label, line, human)
			}
		}
	}
	if strings.Contains(human, prepareRetryHeader) {
		t.Fatalf("%s labelled a remaining repair as a retry\n%s", label, human)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(envelope["remaining_repairs"], &raw); err != nil {
		t.Fatalf("%s raw remaining_repairs: %v", label, err)
	}
	wantKeys := []string{"next_stage", "rerun_required", "stages", "stages_remaining"}
	if want.repairedClass != "" {
		wantKeys = append(wantKeys, "repaired_class")
	}
	sort.Strings(wantKeys)
	gotKeys := make([]string, 0, len(raw))
	for key := range raw {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("%s remaining_repairs keys = %v, want %v", label, gotKeys, wantKeys)
	}
	var nextFields map[string]json.RawMessage
	if err := json.Unmarshal(raw["next_stage"], &nextFields); err != nil {
		t.Fatalf("%s raw next_stage: %v", label, err)
	}
	nextKeys := make([]string, 0, len(nextFields))
	for key := range nextFields {
		nextKeys = append(nextKeys, key)
	}
	sort.Strings(nextKeys)
	if !reflect.DeepEqual(nextKeys, []string{"class", "kind", "ordinal"}) {
		t.Fatalf("%s next_stage keys = %v", label, nextKeys)
	}
	for index, stageRaw := range func() []json.RawMessage {
		var stages []json.RawMessage
		if err := json.Unmarshal(raw["stages"], &stages); err != nil {
			t.Fatalf("%s raw stages: %v", label, err)
		}
		return stages
	}() {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(stageRaw, &fields); err != nil {
			t.Fatal(err)
		}
		wantStageKeys := []string{
			"after_prerequisite", "class", "hashes", "kind", "ordinal",
			"paths", "repair", "repair_cwd", "resulting_classes",
		}
		gotStageKeys := make([]string, 0, len(fields))
		for key := range fields {
			gotStageKeys = append(gotStageKeys, key)
		}
		sort.Strings(gotStageKeys)
		if !reflect.DeepEqual(gotStageKeys, wantStageKeys) {
			t.Fatalf("%s stage %d keys = %v, want %v",
				label, index, gotStageKeys, wantStageKeys)
		}
		for _, arrayField := range []string{"hashes", "paths", "resulting_classes"} {
			if bytes.Equal(bytes.TrimSpace(fields[arrayField]), []byte("null")) {
				t.Fatalf("%s stage %d field %s is null", label, index, arrayField)
			}
		}
	}
	if want.exit == 0 {
		if s7AWRepairAdvisoryCount(report) != 1 {
			t.Fatalf("%s exit-0 carrier advisory count = %d, want 1",
				label, s7AWRepairAdvisoryCount(report))
		}
	} else if s7AWRepairAdvisoryCount(report) != 0 {
		t.Fatalf("%s exit-3 carrier emitted archive-repairs-remaining", label)
	}
}

func s7AWCaptureRemainingCarrier(
	t *testing.T,
	archive s7AVRepairArchive,
	selector []string,
) (int, string, string, intentArchivePurgeReport) {
	t.Helper()
	args := []string{
		"--path", archive.root,
		"feature", "intent-archive", "purge", archive.slug,
	}
	args = append(args, selector...)
	args = append(args, "--yes", "--json")
	code, stdout, stderr, _ := runPrepare(t, args...)
	return code, stdout, stderr, decodeIntentArchivePurgeReport(t, stdout)
}

func TestS7AWRemainingRepairsCarrierContracts(t *testing.T) {
	// The exit-3 carrier observes all four classes in one real archive.
	refused := s7AVWriteRepairArchive(
		t, "PIB-556-refused",
		s7AVRepairSpec{
			residues: 1, dangling: 1, mixed: 1, corrupt: true, ready: true,
		},
	)
	code, stdout, human, report := s7AWCaptureRemainingCarrier(
		t, refused, []string{"--orphans"},
	)
	if code != 3 || report.Refusal == nil {
		t.Fatalf("PIB-556 refusal carrier = exit:%d report:%#v\n%s", code, report, stdout)
	}
	s7AWAssertRemainingCarrier(
		t, "PIB-556 refusal", refused.root, stdout, human, report,
		s7AWRemainingExpectation{
			exit: 3,
			classes: []string{
				string(store.IntentArchiveRepairCorruptObject),
				string(store.IntentArchiveRepairDanglingReference),
				string(store.IntentArchiveRepairMixedReference),
				string(store.IntentArchiveRepairUnreferencedResidue),
			},
		},
	)

	// Exit 0 can carry every non-blocking class. The corrupt class appears on
	// the refusal form because its manual prerequisite forbids an exit-0 class
	// repair while it remains present.
	admitted := []struct {
		name     string
		spec     s7AVRepairSpec
		selector func(s7AVRepairArchive) []string
		repaired string
		classes  []string
	}{
		{
			name:     "residue-leaves-dangling-and-mixed",
			spec:     s7AVRepairSpec{residues: 1, dangling: 1, mixed: 1, ready: true},
			selector: func(s7AVRepairArchive) []string { return []string{"--orphans"} },
			repaired: string(store.IntentArchiveRepairUnreferencedResidue),
			classes: []string{
				string(store.IntentArchiveRepairDanglingReference),
				string(store.IntentArchiveRepairMixedReference),
			},
		},
		{
			name: "dangling-leaves-residue",
			spec: s7AVRepairSpec{residues: 1, dangling: 1, ready: true},
			selector: func(archive s7AVRepairArchive) []string {
				return s7AVBlobSelector(archive.dangling)
			},
			repaired: string(store.IntentArchiveRepairDanglingReference),
			classes:  []string{string(store.IntentArchiveRepairUnreferencedResidue)},
		},
	}
	for _, fixture := range admitted {
		archive := s7AVWriteRepairArchive(t, "PIB-556-"+fixture.name, fixture.spec)
		code, stdout, human, report := s7AWCaptureRemainingCarrier(
			t, archive, fixture.selector(archive),
		)
		if code != 0 || report.Outcome != string(store.IntentArchivePurgePurged) {
			t.Fatalf("PIB-556 %s = exit:%d report:%#v\n%s",
				fixture.name, code, report, stdout)
		}
		s7AWAssertRemainingCarrier(
			t, "PIB-556 "+fixture.name, archive.root, stdout, human, report,
			s7AWRemainingExpectation{
				exit:          0,
				repairedClass: fixture.repaired,
				classes:       fixture.classes,
			},
		)
	}
}

// ─── PIB-560 ──────────────────────────────────────────────────────────────────

type s7AWFixtureKindObservation struct {
	kind       string
	real       bool
	seam       bool
	limitation string
}

func s7AWConstructBlobKind(
	t *testing.T,
	fixture s7ARDivergenceFixture,
	kind string,
) (bool, string) {
	t.Helper()
	blobPath := filepath.Join(fixture.root, filepath.FromSlash(fixture.blobRel))
	if err := os.Remove(blobPath); err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "regular":
		if err := os.WriteFile(blobPath, []byte("PIB-560 hash-wrong regular\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		relative, err := filepath.Rel(filepath.Dir(blobPath), fixture.targetPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(relative, blobPath); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.MkdirAll(blobPath, 0o700); err != nil {
			t.Fatal(err)
		}
		s7AVDirectoryWithTwoFiles(t, blobPath)
	case "fifo":
		if err := syscall.Mkfifo(blobPath, 0o600); err != nil {
			t.Fatal(err)
		}
	case "device":
		var null syscall.Stat_t
		if err := syscall.Stat("/dev/null", &null); err != nil {
			t.Fatalf("PIB-560 stat /dev/null: %v", err)
		}
		if err := syscall.Mknod(
			blobPath, uint32(syscall.S_IFCHR|0o600), int(null.Rdev),
		); err == nil {
			return false, ""
		} else if !errors.Is(err, syscall.EPERM) &&
			!errors.Is(err, syscall.EACCES) &&
			!errors.Is(err, syscall.ENOTSUP) &&
			!errors.Is(err, syscall.EOPNOTSUPP) &&
			!errors.Is(err, syscall.ENOSYS) &&
			!errors.Is(err, syscall.EROFS) &&
			!errors.Is(err, syscall.EINVAL) {
			t.Fatalf("PIB-560 mknod failed for a reason other than host privilege/support: %v", err)
		}
		if err := os.WriteFile(blobPath, []byte("PIB-560 device seam placeholder\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		limitation := "LIMITATION: unprivileged host could not create a device node; " +
			"the device-only injected kind seam covers classification and rendering, " +
			"but no real rm -rf -- is executed against a real device node here."
		t.Log(limitation)
		return true, limitation
	default:
		t.Fatalf("unknown PIB-560 object kind %q", kind)
	}
	return false, ""
}

func s7AWValidateFixtureKinds(observations []s7AWFixtureKindObservation) error {
	want := []string{"regular", "symlink", "directory", "fifo", "device"}
	if len(observations) != len(want) {
		return fmt.Errorf("nonregular fixture count = %d, want %d", len(observations), len(want))
	}
	for index, kind := range want {
		observation := observations[index]
		if observation.kind != kind {
			return fmt.Errorf("fixture %d kind = %q, want %q", index, observation.kind, kind)
		}
		if kind != "device" {
			if !observation.real || observation.seam || observation.limitation != "" {
				return fmt.Errorf("%s used the injected kind seam", kind)
			}
			continue
		}
		if observation.real == observation.seam {
			return errors.New("device fixture must be exactly one of real mknod or injected kind")
		}
		if observation.seam && !strings.HasPrefix(observation.limitation, "LIMITATION:") {
			return errors.New("device seam silently omitted its stated limitation")
		}
	}
	return nil
}

func TestS7AWNonRegularFixtureConstructionContracts(t *testing.T) {
	observations := []s7AWFixtureKindObservation{}
	for _, kind := range []string{"regular", "symlink", "directory", "fifo", "device"} {
		fixture := s7AROwnedDivergenceFixture(t)
		seam, limitation := s7AWConstructBlobKind(t, fixture, kind)
		blobPath := filepath.Join(fixture.root, filepath.FromSlash(fixture.blobRel))
		info, err := os.Lstat(blobPath)
		if err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "regular":
			if !info.Mode().IsRegular() {
				t.Fatalf("PIB-560 regular fixture mode = %v", info.Mode())
			}
		case "symlink":
			if info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("PIB-560 symlink fixture mode = %v", info.Mode())
			}
		case "directory":
			if !info.IsDir() {
				t.Fatalf("PIB-560 directory fixture mode = %v", info.Mode())
			}
		case "fifo":
			if info.Mode()&os.ModeNamedPipe == 0 {
				t.Fatalf("PIB-560 FIFO fixture mode = %v", info.Mode())
			}
		case "device":
			if !seam && info.Mode()&os.ModeDevice == 0 {
				t.Fatalf("PIB-560 device fixture mode = %v", info.Mode())
			}
		}

		restore := func() {}
		if seam {
			restore = s7AVRestoreSeam(t, s7ARInstallDeviceProbe(t, fixture.blobRel))
		}
		code, stdout, stderr := s7APRunFromWorkspace(t, fixture.root, []string{
			"feature", "intent-archive", "purge", fixture.slug,
			"--blob", fixture.hash, "--yes", "--json",
		})
		restore()
		report := decodeIntentArchivePurgeReport(t, stdout)
		if code != 6 || report.Divergence == nil ||
			report.Divergence.Kind != "blob" ||
			report.Divergence.RemoveCommand != "rm -rf -- "+fixture.blobRel ||
			strings.Contains(stdout, "--abandon-transaction") {
			t.Fatalf("PIB-560 %s route = exit:%d stderr:%q report:%#v\n%s",
				kind, code, stderr, report, stdout)
		}
		if err := s7AVValidatePrintedRemoval(s7AVDivergenceProcedure(
			"PIB-560 "+kind, report, stderr, fixture.blobRel,
		)); err != nil {
			t.Fatalf("PIB-560 %s printed route: %v", kind, err)
		}
		if !seam {
			s7AVExecutePrintedRemoval(
				t, "PIB-560 "+kind, fixture.root, report.Divergence.RemoveCommand,
			)
			if _, err := os.Lstat(blobPath); !os.IsNotExist(err) {
				t.Fatalf("PIB-560 %s real removal left the object: %v", kind, err)
			}
		}
		observations = append(observations, s7AWFixtureKindObservation{
			kind:       kind,
			real:       !seam,
			seam:       seam,
			limitation: limitation,
		})
	}
	if err := s7AWValidateFixtureKinds(observations); err != nil {
		t.Fatal(err)
	}
	if err := s7AWValidateDeviceSeamSource(avpRepoRoot(t)); err != nil {
		t.Fatal(err)
	}

	allInjected := append([]s7AWFixtureKindObservation(nil), observations...)
	for index := range allInjected {
		allInjected[index].real = false
		allInjected[index].seam = true
		allInjected[index].limitation = "LIMITATION: injected"
	}
	if err := s7AWValidateFixtureKinds(allInjected); err == nil {
		t.Fatal("PIB-560 validator accepted an injected-kind run for all five kinds")
	}
	if err := s7AWValidateFixtureKinds(observations[:4]); err == nil {
		t.Fatal("PIB-560 validator accepted a silently skipped device kind")
	}
}

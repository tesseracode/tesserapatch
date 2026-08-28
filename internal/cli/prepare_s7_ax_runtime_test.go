//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// ─── shared AX fixture machinery ──────────────────────────────────────────────

// s7AXTreeSnapshot is readTree's guarantee extended over non-regular managed
// objects. readTree opens every non-directory entry with os.ReadFile, which
// blocks forever on a FIFO, so the AX fixtures — whose whole subject is an
// object tpatch cannot identify — need a snapshot that records a non-regular
// entry by lstat identity instead of by content. Regular files still contribute
// their exact bytes, so the "byte-identical tree" assertion is not weakened.
func s7AXTreeSnapshot(t *testing.T, root string) []byte {
	t.Helper()
	var output bytes.Buffer
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			output.WriteString("D " + relative + "\n")
		case info.Mode().IsRegular():
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			output.WriteString("F " + relative + "\x00")
			output.Write(data)
			output.WriteByte('\n')
		default:
			output.WriteString(
				"O " + relative + "\x00" +
					strconv.FormatUint(uint64(info.Mode()), 10) + ":" +
					strconv.FormatInt(info.Size(), 10) + "\n",
			)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func s7AXArchiveTree(t *testing.T, archive s7AVRepairArchive) []byte {
	t.Helper()
	return s7AXTreeSnapshot(t, filepath.Join(archive.root, ".tpatch"))
}

// s7AXUnreferencedCorruptSpec builds the fixture rev-12 lacked: an archive whose
// corrupt managed object is a FIFO at a canonical blob path whose hash every
// reference tombstones, beside `residues` globally unreferenced residues.
type s7AXUnreferencedCorruptSpec struct {
	residues int
	ready    bool
}

func s7AXWriteUnreferencedCorruptArchive(
	t *testing.T,
	label string,
	spec s7AXUnreferencedCorruptSpec,
) s7AVRepairArchive {
	t.Helper()
	var root, slug string
	if spec.ready {
		root, slug = prepareS4Workspace(t, "S7 AX "+label)
		prepareS4WriteReadyBundle(t, root, slug, true)
	} else {
		root, slug = intentArchiveCLIWorkspace(t)
	}
	archive := s7AVRepairArchive{
		root:        root,
		slug:        slug,
		blobRel:     map[string]string{},
		generations: map[string]string{},
	}
	record := func(hash string) {
		rel, err := store.IntentArchiveBlobRel(slug, hash)
		if err != nil {
			t.Fatal(err)
		}
		archive.blobRel[hash] = rel
	}

	generations := []store.IntentArchiveGeneration{}
	blobs := map[string][]byte{}
	for index := 0; index < spec.residues; index++ {
		data := []byte(fmt.Sprintf("PIB-562 %s residue %d\n", label, index))
		replacement := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireTombstoned,
		)
		generation := intentArchiveCLIGeneration(t, slug, replacement)
		generations = append(generations, generation)
		blobs[replacement.ContentSHA256] = data
		archive.residues = append(archive.residues, replacement.ContentSHA256)
		archive.generations[replacement.ContentSHA256] = generation.GenerationID
		record(replacement.ContentSHA256)
	}

	// Two tombstoned references to the same hash, in two generations, so the
	// corrupt object's own hash is globally unreferenced by construction rather
	// than by the absence of a second reference.
	corruptData := []byte("PIB-562 " + label + " unreferenced corrupt object\n")
	first := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactExploration, corruptData, store.IntentArchiveWireTombstoned,
	)
	second := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, corruptData, store.IntentArchiveWireTombstoned,
	)
	firstGen := intentArchiveCLIGeneration(t, slug, first)
	secondGen := intentArchiveCLIGeneration(t, slug, second)
	generations = append(generations, firstGen, secondGen)
	archive.corrupt = first.ContentSHA256
	archive.generations[first.ContentSHA256] = firstGen.GenerationID
	record(first.ContentSHA256)

	writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, generations...), blobs)
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	archive.indexRel = indexRel

	corruptPath := archive.abs(archive.blobRel[archive.corrupt])
	if err := syscall.Mkfifo(corruptPath, 0o600); err != nil {
		t.Fatalf("PIB-562 FIFO construction failed: %v", err)
	}
	info, err := os.Lstat(corruptPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("PIB-562 corrupt fixture mode = %v, want a real FIFO", info.Mode())
	}
	return archive
}

// s7AXSelectorsOverArchive is the closed confirmed-selector set §9.3.1 ranks
// against: --orphans, one --blob per observed hash, one --generation per
// observed generation, and --all.
func s7AXSelectorsOverArchive(archive s7AVRepairArchive) [][]string {
	selectors := [][]string{{"--orphans"}}
	hashes := []string{}
	for hash := range archive.blobRel {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)
	for _, hash := range hashes {
		selectors = append(selectors, []string{"--blob", hash})
	}
	generations := []string{}
	for _, generation := range archive.generations {
		generations = append(generations, generation)
	}
	sort.Strings(generations)
	for _, generation := range generations {
		selectors = append(selectors, []string{"--generation", generation})
	}
	return append(selectors, []string{"--all"})
}

// s7AXAssertBlockedSelector runs one confirmed selector against an archive that
// holds a corrupt object and proves the rank-1 block: exit 3, a byte-identical
// tree, zero removals and zero index writes, and a remaining_repairs plan whose
// first stage is the corrupt class's manual prerequisite.
func s7AXAssertBlockedSelector(
	t *testing.T,
	label string,
	archive s7AVRepairArchive,
	selector []string,
) *intentArchiveRemainingRepairsReport {
	t.Helper()
	before := s7AXArchiveTree(t, archive)
	code, stdout, _, report, removed, indexCASs :=
		s7AWRunPurgeWithMutationSpy(t, archive, selector)
	if code != 3 {
		t.Fatalf("%s selector %v exit=%d, want 3\n%s", label, selector, code, stdout)
	}
	if len(removed) != 0 || indexCASs != 0 {
		t.Fatalf("%s selector %v wrote: removed=%v index-CAS=%d",
			label, selector, removed, indexCASs)
	}
	if !bytes.Equal(before, s7AXArchiveTree(t, archive)) {
		t.Fatalf("%s selector %v changed the tree while a corrupt object was present",
			label, selector)
	}
	if report.Outcome != "refused" || report.Refusal == nil ||
		report.Refusal.Code != string(store.IntentArchiveCodeBlobCorrupt) {
		t.Fatalf("%s selector %v refusal = %#v\n%s", label, selector, report.Refusal, stdout)
	}
	remaining := report.RemainingRepairs
	if remaining == nil || remaining.RepairedClass != "" ||
		len(remaining.Stages) == 0 ||
		remaining.Stages[0].Class != string(store.IntentArchiveRepairCorruptObject) ||
		remaining.Stages[0].Kind != string(store.IntentArchiveRepairStageManual) ||
		remaining.NextStage == nil ||
		remaining.NextStage.Ordinal != remaining.Stages[0].Ordinal ||
		remaining.NextStage.Kind != remaining.Stages[0].Kind ||
		remaining.NextStage.Class != remaining.Stages[0].Class {
		t.Fatalf("%s selector %v plan = %#v\n%s", label, selector, remaining, stdout)
	}
	return remaining
}

// s7AXStageIndex maps a plan's stages by class token.
func s7AXStageIndex(
	remaining *intentArchiveRemainingRepairsReport,
) map[string]intentArchiveRepairStageReport {
	stages := map[string]intentArchiveRepairStageReport{}
	if remaining == nil {
		return stages
	}
	for _, stage := range remaining.Stages {
		stages[stage.Class] = stage
	}
	return stages
}

func s7AXBlobPaths(archive s7AVRepairArchive, hashes []string) []string {
	paths := []string{}
	for _, hash := range hashes {
		paths = append(paths, archive.blobRel[hash])
	}
	sort.Strings(paths)
	return paths
}

// ─── PIB-562 ──────────────────────────────────────────────────────────────────

func TestS7AXCorruptPlusResidueRepairContracts(t *testing.T) {
	archive := s7AXWriteUnreferencedCorruptArchive(
		t, "PIB-562", s7AXUnreferencedCorruptSpec{residues: 2, ready: true},
	)
	corruptPath := archive.blobRel[archive.corrupt]
	residuePaths := s7AXBlobPaths(archive, archive.residues)
	residueRepair := s7AVOrphanPurgeCommand(archive.slug)

	// Pre-prerequisite: every byte of the archive, including both residue blobs
	// and index.json, is unchanged by the refusal.
	indexBefore := archive.indexBytes(t)
	residueBefore := map[string][]byte{}
	for _, hash := range archive.residues {
		data, err := os.ReadFile(archive.abs(archive.blobRel[hash]))
		if err != nil {
			t.Fatal(err)
		}
		residueBefore[hash] = data
	}

	// `--orphans --yes` does not remove the two residues it can identify: the
	// rank-1 corrupt object blocks it, zero-write, at exit 3.
	remaining := s7AXAssertBlockedSelector(t, "PIB-562", archive, []string{"--orphans"})
	if remaining.StagesRemaining != 2 || len(remaining.Stages) != 2 {
		t.Fatalf("PIB-562 stages_remaining = %d / %d stages, want 2/2",
			remaining.StagesRemaining, len(remaining.Stages))
	}
	if !bytes.Equal(indexBefore, archive.indexBytes(t)) {
		t.Fatal("PIB-562 refusal changed index.json bytes")
	}
	for hash, before := range residueBefore {
		after, err := os.ReadFile(archive.abs(archive.blobRel[hash]))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("PIB-562 refusal changed residue blob %s: %v", hash, err)
		}
	}

	// The corrupt object's hash is globally unreferenced, so its removal leaves
	// a clean hash rather than a dangling one: resulting_classes is empty.
	s7AWAssertRepairStage(
		t, "PIB-562 corrupt",
		remaining.Stages[0],
		1,
		string(store.IntentArchiveRepairStageManual),
		string(store.IntentArchiveRepairCorruptObject),
		[]string{archive.corrupt},
		[]string{corruptPath},
		remaining.Stages[0].Repair,
		[]string{},
		false,
	)
	s7AWAssertRepairStage(
		t, "PIB-562 residue",
		remaining.Stages[1],
		2,
		string(store.IntentArchiveRepairStagePurge),
		string(store.IntentArchiveRepairUnreferencedResidue),
		archive.residues,
		residuePaths,
		residueRepair,
		[]string{},
		false,
	)

	// The corrupt stage never offers --orphans --yes as its own repair, and the
	// printed prerequisite is the single exact-path type-total removal.
	warning, removal := s7AVExtractRemovalLine(remaining.Stages[0].Repair)
	if err := s7AVValidatePrintedRemoval(s7AVPrintedProcedure{
		label:          "PIB-562 corrupt prerequisite",
		block:          remaining.Stages[0].Repair + "\n" + s7AVGitHistoryCaveat,
		warning:        warning,
		removeCommand:  removal,
		blobRel:        corruptPath,
		historyCaveats: []string{s7AVGitHistoryCaveat},
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(remaining.Stages[0].Repair, "--orphans") {
		t.Fatalf("PIB-562 corrupt stage named --orphans as its repair:\n%s",
			remaining.Stages[0].Repair)
	}

	// `list` and `doctor` render the corrupt object and both residues in one
	// pass and never name --orphans --yes as the corrupt object's repair.
	s7AXAssertOnePassInventory(t, archive, corruptPath, residuePaths)

	// Every other confirmed selector is blocked on the identical plan.
	for _, selector := range s7AXSelectorsOverArchive(archive) {
		if reflect.DeepEqual(selector, []string{"--orphans"}) {
			continue
		}
		other := s7AXAssertBlockedSelector(t, "PIB-562", archive, selector)
		if !reflect.DeepEqual(other, remaining) {
			t.Fatalf("PIB-562 selector %v changed the shared plan:\n got %#v\nwant %#v",
				selector, other, remaining)
		}
	}

	// Stage 1: the operator runs the printed manual prerequisite.
	stages := 0
	s7AVExecutePrintedRemoval(t, "PIB-562 manual prerequisite", archive.root, removal)
	stages++
	if _, err := os.Lstat(archive.abs(corruptPath)); !os.IsNotExist(err) {
		t.Fatalf("PIB-562 prerequisite left the corrupt object: %v", err)
	}

	// X11 now classifies that hash as ordinary purged storage in no repair
	// class: the only remaining class is the residue class.
	freedCode, freedStdout, _, _ := runPrepare(t,
		"--path", archive.root, "feature", "intent-archive", "list", archive.slug,
		"--json", "--quiet",
	)
	if freedCode != 0 {
		t.Fatalf("PIB-562 post-prerequisite list exit=%d, want 0\n%s", freedCode, freedStdout)
	}
	freedReport := decodeIntentArchiveListReport(t, freedStdout)
	if len(freedReport.CorruptObjects) != 0 || len(freedReport.Orphans) != 2 {
		t.Fatalf("PIB-562 post-prerequisite inventory = %d corrupt / %d orphans, want 0/2",
			len(freedReport.CorruptObjects), len(freedReport.Orphans))
	}
	freedReferences := 0
	for _, generation := range freedReport.Generations {
		for _, entry := range generation.Entries {
			if entry.ContentSHA256 != archive.corrupt {
				continue
			}
			freedReferences++
			if entry.Storage != "purged" || entry.Repair != "" {
				t.Fatalf("PIB-562 freed hash entry = storage:%q repair:%q, want ordinary purged storage in no class",
					entry.Storage, entry.Repair)
			}
		}
	}
	if freedReferences != 2 {
		t.Fatalf("PIB-562 freed hash rendered %d references, want 2", freedReferences)
	}

	// Stage 2: one tpatch invocation clears both residues.
	invocations := 0
	code, stdout, stderr, report, removed, indexCASs :=
		s7AWRunPurgeWithMutationSpy(t, archive, []string{"--orphans"})
	invocations++
	stages++
	sort.Strings(removed)
	if code != 0 || stderr != "" ||
		report.Outcome != string(store.IntentArchivePurgePurged) ||
		report.Action != "none" ||
		report.RemainingRepairs != nil ||
		s7AWRepairAdvisoryCount(report) != 0 ||
		!reflect.DeepEqual(removed, residuePaths) ||
		indexCASs != 0 {
		t.Fatalf("PIB-562 residue repair = exit:%d stderr:%q removed:%v index-CAS:%d report:%#v\n%s",
			code, stderr, removed, indexCASs, report, stdout)
	}
	if !bytes.Equal(indexBefore, archive.indexBytes(t)) {
		t.Fatal("PIB-562 residue repair rewrote index.json")
	}
	for _, hash := range archive.residues {
		if _, err := os.Lstat(archive.abs(archive.blobRel[hash])); !os.IsNotExist(err) {
			t.Fatalf("PIB-562 residue %s survived: %v", hash, err)
		}
	}
	if stages != 2 || invocations != 1 {
		t.Fatalf("PIB-562 totals = %d stage(s) / %d tpatch invocation(s), want 2/1",
			stages, invocations)
	}
	s7AVAssertArchiveRepaired(t, archive, "PIB-562")
}

// s7AXAssertOnePassInventory proves `list` and `doctor` each render the corrupt
// object and every residue in a single pass, and that neither offers
// `--orphans --yes` as the corrupt object's own repair.
//
// `list` takes the highest observed exit, which is 3 for a corrupt object.
// `doctor`'s D9 renders the identical observation set as warning findings and
// exits 0 — that is D16's warning-only rule, restated by the accepted rev-17
// erratum for PIB-543, carried into PIB-562's own row by the proposed rev-19
// erratum, and implemented by workflow.DoctorExitCode, which counts warnings
// outside Summary.Findings.
func s7AXAssertOnePassInventory(
	t *testing.T,
	archive s7AVRepairArchive,
	corruptPath string,
	residuePaths []string,
) {
	t.Helper()
	code, stdout, _, _ := runPrepare(t,
		"--path", archive.root, "feature", "intent-archive", "list", archive.slug,
		"--json", "--quiet",
	)
	if code != 3 {
		t.Fatalf("PIB-562 list exit=%d, want 3\n%s", code, stdout)
	}
	listReport := decodeIntentArchiveListReport(t, stdout)
	corruptEntries := 0
	for _, generation := range listReport.Generations {
		for _, entry := range generation.Entries {
			if entry.Storage != "corrupt" {
				continue
			}
			corruptEntries++
			if entry.BlobPath != corruptPath {
				t.Fatalf("PIB-562 list corrupt entry path = %q, want %q", entry.BlobPath, corruptPath)
			}
			if strings.Contains(entry.Repair, "--orphans") || entry.Retry != "" {
				t.Fatalf("PIB-562 list named a tpatch selector for the corrupt object: repair=%q retry=%q",
					entry.Repair, entry.Retry)
			}
			if !strings.Contains(entry.Repair, "rm -rf -- ") {
				t.Fatalf("PIB-562 list corrupt repair is not the manual prerequisite: %q", entry.Repair)
			}
		}
	}
	if corruptEntries != 2 {
		t.Fatalf("PIB-562 list rendered %d corrupt references, want both tombstoned references\n%s",
			corruptEntries, stdout)
	}
	renderedOrphans := []string{}
	for _, orphan := range listReport.Orphans {
		renderedOrphans = append(renderedOrphans, orphan.Path)
		if orphan.Repair != s7AVOrphanPurgeCommand(archive.slug) {
			t.Fatalf("PIB-562 list orphan repair = %q", orphan.Repair)
		}
	}
	sort.Strings(renderedOrphans)
	if !reflect.DeepEqual(renderedOrphans, residuePaths) {
		t.Fatalf("PIB-562 list orphans = %v, want %v", renderedOrphans, residuePaths)
	}

	doctorCode, doctorStdout, doctorStderr, _ := runPrepare(t,
		"--path", archive.root, "doctor", "--json", "--check", "D9",
	)
	if doctorCode != 0 || doctorStderr != "" {
		t.Fatalf("PIB-562 doctor exit=%d stderr=%q, want D16's warning-only 0\n%s",
			doctorCode, doctorStderr, doctorStdout)
	}
	doctorReport := s7AXDecodeDoctorReport(t, doctorStdout)
	corruptFindings := 0
	residueFindings := 0
	for _, finding := range doctorReport.Findings {
		if finding.CheckID != "D9" {
			continue
		}
		if finding.Severity != "warning" {
			t.Fatalf("PIB-562 doctor finding %q severity = %q, want warning",
				finding.Tag, finding.Severity)
		}
		switch finding.Tag {
		case string(store.IntentArchiveRepairCorruptObject):
			corruptFindings++
			if !strings.Contains(finding.Message, corruptPath) {
				t.Fatalf("PIB-562 doctor omitted the corrupt path:\n%s", finding.Message)
			}
			if strings.Contains(finding.Remediation, "--orphans") {
				t.Fatalf("PIB-562 doctor named --orphans for the corrupt object: %q", finding.Remediation)
			}
		case string(store.IntentArchiveRepairUnreferencedResidue):
			residueFindings++
			for _, residuePath := range residuePaths {
				if !strings.Contains(finding.Message, residuePath) {
					t.Fatalf("PIB-562 doctor omitted residue %s:\n%s", residuePath, finding.Message)
				}
			}
			if finding.Remediation != s7AVOrphanPurgeCommand(archive.slug) {
				t.Fatalf("PIB-562 doctor residue remediation = %q", finding.Remediation)
			}
		}
	}
	if corruptFindings != 1 || residueFindings != 1 {
		t.Fatalf("PIB-562 doctor one-pass findings = %d corrupt / %d residue, want 1/1\n%s",
			corruptFindings, residueFindings, doctorStdout)
	}
}

func s7AXDecodeDoctorReport(t *testing.T, stdout string) workflow.DoctorReport {
	t.Helper()
	var report workflow.DoctorReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("PIB-562 decode doctor report: %v\n%s", err, stdout)
	}
	return report
}

// ─── PIB-563 ──────────────────────────────────────────────────────────────────

type s7AXMergeCase struct {
	label            string
	spec             s7AVRepairSpec
	wantStages       int
	wantInvocations  int
	wantSecondClass  store.IntentArchiveRepairClass
	corruptJoinsBase bool
}

// s7AXMergeOutcome is what one merge case observes, returned to the contract
// body so the top-level target asserts the counts and the merged class
// membership directly instead of delegating every binding to the helper.
type s7AXMergeOutcome struct {
	label           string
	stagesPlanned   int
	stagesExecuted  int
	invocations     int
	mergedDangling  []string
	repairedClasses []store.IntentArchiveRepairClass
	staleRefused    bool
	repaired        bool
}

func TestS7AXCorruptClassMergeRepairContracts(t *testing.T) {
	cases := []s7AXMergeCase{
		{
			label:            "corrupt-plus-mixed",
			spec:             s7AVRepairSpec{mixed: 2, corrupt: true, ready: true},
			wantStages:       3,
			wantInvocations:  2,
			wantSecondClass:  store.IntentArchiveRepairMixedReference,
			corruptJoinsBase: false,
		},
		{
			label:            "corrupt-plus-dangling",
			spec:             s7AVRepairSpec{dangling: 2, corrupt: true, ready: true},
			wantStages:       2,
			wantInvocations:  1,
			wantSecondClass:  store.IntentArchiveRepairDanglingReference,
			corruptJoinsBase: true,
		},
	}
	outcomes := make([]s7AXMergeOutcome, 0, len(cases))
	for _, fixture := range cases {
		outcome := s7AXRunMergeCase(t, fixture)
		if outcome.label != fixture.label || !outcome.repaired {
			t.Fatalf("PIB-563 %s did not run to a repaired archive: %#v", fixture.label, outcome)
		}
		if outcome.stagesPlanned != fixture.wantStages ||
			outcome.stagesExecuted != fixture.wantStages {
			t.Fatalf("PIB-563 %s stages planned/executed = %d/%d, want %d/%d",
				fixture.label, outcome.stagesPlanned, outcome.stagesExecuted,
				fixture.wantStages, fixture.wantStages)
		}
		if outcome.invocations != fixture.wantInvocations ||
			len(outcome.repairedClasses) != fixture.wantInvocations {
			t.Fatalf("PIB-563 %s tpatch invocations = %d over classes %v, want %d",
				fixture.label, outcome.invocations, outcome.repairedClasses,
				fixture.wantInvocations)
		}
		last := outcome.repairedClasses[len(outcome.repairedClasses)-1]
		if last != fixture.wantSecondClass {
			t.Fatalf("PIB-563 %s last repaired class = %q, want %q",
				fixture.label, last, fixture.wantSecondClass)
		}
		if outcome.repairedClasses[0] != store.IntentArchiveRepairDanglingReference {
			t.Fatalf("PIB-563 %s first repaired class = %q, want the merged dangling class",
				fixture.label, outcome.repairedClasses[0])
		}
		if outcome.staleRefused != fixture.corruptJoinsBase {
			t.Fatalf("PIB-563 %s stale-selection refusal observed = %t, want %t",
				fixture.label, outcome.staleRefused, fixture.corruptJoinsBase)
		}
		wantMerged := fixture.spec.dangling + 1
		if len(outcome.mergedDangling) != wantMerged {
			t.Fatalf("PIB-563 %s merged dangling class holds %d hash(es) %v, want %d",
				fixture.label, len(outcome.mergedDangling), outcome.mergedDangling, wantMerged)
		}
		outcomes = append(outcomes, outcome)
	}

	// The two cases are genuinely distinct: the corrupt object joins the base
	// dangling class in exactly one of them, and the two differ in both their
	// stage counts and their invocation counts.
	if len(outcomes) != 2 {
		t.Fatalf("PIB-563 observed %d merge cases, want 2", len(outcomes))
	}
	if outcomes[0].stagesExecuted == outcomes[1].stagesExecuted ||
		outcomes[0].invocations == outcomes[1].invocations ||
		outcomes[0].staleRefused == outcomes[1].staleRefused {
		t.Fatalf("PIB-563 no longer separates the two merge shapes: %#v", outcomes)
	}
}

func s7AXRunMergeCase(t *testing.T, fixture s7AXMergeCase) s7AXMergeOutcome {
	t.Helper()
	outcome := s7AXMergeOutcome{label: fixture.label}
	archive := s7AVWriteRepairArchive(t, "PIB-563-"+fixture.label, fixture.spec)
	corruptPath := archive.blobRel[archive.corrupt]

	// Pre-prerequisite class membership, as the archive's own scan sees it.
	preDangling := append([]string(nil), archive.dangling...)
	sort.Strings(preDangling)
	mergedDangling := append(append([]string(nil), archive.dangling...), archive.corrupt)
	sort.Strings(mergedDangling)
	outcome.mergedDangling = mergedDangling

	// Every confirmed selector refuses exit 3 zero-write until the manual
	// prerequisite has run, and every refusal carries the same plan.
	var remaining *intentArchiveRemainingRepairsReport
	for _, selector := range s7AXSelectorsOverArchive(archive) {
		observed := s7AXAssertBlockedSelector(t, "PIB-563 "+fixture.label, archive, selector)
		if remaining != nil && !reflect.DeepEqual(observed, remaining) {
			t.Fatalf("PIB-563 %s selector %v changed the shared plan:\n got %#v\nwant %#v",
				fixture.label, selector, observed, remaining)
		}
		remaining = observed
	}
	if remaining.StagesRemaining != fixture.wantStages ||
		len(remaining.Stages) != fixture.wantStages {
		t.Fatalf("PIB-563 %s stages_remaining = %d / %d stages, want %d",
			fixture.label, remaining.StagesRemaining, len(remaining.Stages), fixture.wantStages)
	}
	outcome.stagesPlanned = remaining.StagesRemaining

	// The corrupt object is named with its exact path and its predicted class:
	// a retained reference to its hash survives, so it becomes dangling.
	s7AWAssertRepairStage(
		t, "PIB-563 "+fixture.label+" corrupt",
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

	// The merge is asserted before the prerequisite runs: the pre-prerequisite
	// report already enumerates the hash the prerequisite will free inside the
	// dangling class's own stage, and never prints the pre-prerequisite
	// membership as an admitted selection.
	stages := s7AXStageIndex(remaining)
	danglingStage, hasDangling := stages[string(store.IntentArchiveRepairDanglingReference)]
	if !hasDangling {
		t.Fatalf("PIB-563 %s plan has no dangling stage: %#v", fixture.label, remaining)
	}
	if !danglingStage.AfterPrerequisite {
		t.Fatalf("PIB-563 %s dangling stage is not marked after_prerequisite: %#v",
			fixture.label, danglingStage)
	}
	if !reflect.DeepEqual(danglingStage.Hashes, mergedDangling) {
		t.Fatalf("PIB-563 %s dangling stage hashes = %v, want the merged %v",
			fixture.label, danglingStage.Hashes, mergedDangling)
	}
	staleSelection := s7AVBlobPurgeCommand(archive.slug, preDangling)
	if fixture.corruptJoinsBase && danglingStage.Repair == staleSelection {
		t.Fatalf("PIB-563 %s printed the pre-prerequisite membership as its repair: %q",
			fixture.label, danglingStage.Repair)
	}
	if danglingStage.Repair != s7AVBlobPurgeCommand(archive.slug, mergedDangling) {
		t.Fatalf("PIB-563 %s dangling repair = %q", fixture.label, danglingStage.Repair)
	}

	// Stage 1: the printed manual prerequisite.
	_, removal := s7AVExtractRemovalLine(remaining.Stages[0].Repair)
	s7AVExecutePrintedRemoval(t, "PIB-563 "+fixture.label, archive.root, removal)
	if _, err := os.Lstat(archive.abs(corruptPath)); !os.IsNotExist(err) {
		t.Fatalf("PIB-563 %s prerequisite left the corrupt object: %v", fixture.label, err)
	}
	executedStages := 1

	// A selection computed from the pre-prerequisite scan leaves an instance of
	// its own class uncovered and is refused exit 3 zero-write.
	if fixture.corruptJoinsBase {
		before := s7AXArchiveTree(t, archive)
		code, stdout, _, staleReport, removed, indexCASs :=
			s7AWRunPurgeWithMutationSpy(t, archive, s7AVBlobSelector(preDangling))
		if code != 3 || len(removed) != 0 || indexCASs != 0 ||
			staleReport.Outcome != "refused" ||
			!bytes.Equal(before, s7AXArchiveTree(t, archive)) {
			t.Fatalf("PIB-563 %s stale incomplete selection = exit:%d removed:%v index-CAS:%d\n%s",
				fixture.label, code, removed, indexCASs, stdout)
		}
		outcome.staleRefused = true
	}

	// The report printed after the prerequisite enumerates the whole merged
	// class.
	afterCode, afterStdout, _, _ := runPrepare(t,
		s7ASPurgeArgs(archive.root, archive.slug, []string{"--generation", archive.generations[archive.corrupt]}, true, true, true)...,
	)
	if afterCode != 3 {
		t.Fatalf("PIB-563 %s post-prerequisite probe exit=%d, want 3\n%s",
			fixture.label, afterCode, afterStdout)
	}
	afterStages := s7AXStageIndex(decodeIntentArchivePurgeReport(t, afterStdout).RemainingRepairs)
	afterDangling, present := afterStages[string(store.IntentArchiveRepairDanglingReference)]
	if !present || !reflect.DeepEqual(afterDangling.Hashes, mergedDangling) ||
		afterDangling.AfterPrerequisite {
		t.Fatalf("PIB-563 %s post-prerequisite dangling stage = %#v, want the merged %v",
			fixture.label, afterDangling, mergedDangling)
	}

	// The remaining stages are one confirmed purge invocation each.
	invocations := 0
	runClass := func(
		selector []string, label string, class store.IntentArchiveRepairClass,
	) intentArchivePurgeReport {
		code, stdout, stderr, report, _, _ :=
			s7AWRunPurgeWithMutationSpy(t, archive, selector)
		invocations++
		executedStages++
		outcome.repairedClasses = append(outcome.repairedClasses, class)
		if code != 0 || stderr != "" ||
			report.Outcome != string(store.IntentArchivePurgePurged) ||
			report.Action != "none" {
			t.Fatalf("PIB-563 %s %s = exit:%d stderr:%q report:%#v\n%s",
				fixture.label, label, code, stderr, report, stdout)
		}
		return report
	}

	runClass(
		s7AVBlobSelector(mergedDangling), "dangling repair",
		store.IntentArchiveRepairDanglingReference,
	)
	if fixture.wantSecondClass == store.IntentArchiveRepairMixedReference {
		runClass(
			s7AVBlobSelector(archive.mixed), "mixed repair",
			store.IntentArchiveRepairMixedReference,
		)
	}
	if executedStages != fixture.wantStages || invocations != fixture.wantInvocations {
		t.Fatalf("PIB-563 %s totals = %d stage(s) / %d tpatch invocation(s), want %d/%d",
			fixture.label, executedStages, invocations,
			fixture.wantStages, fixture.wantInvocations)
	}
	s7AVAssertArchiveRepaired(t, archive, "PIB-563 "+fixture.label)
	outcome.stagesExecuted = executedStages
	outcome.invocations = invocations
	outcome.repaired = true
	return outcome
}

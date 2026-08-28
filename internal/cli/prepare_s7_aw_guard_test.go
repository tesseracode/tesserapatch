//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func s7AWFunctionCase(function *ast.FuncDecl, constant string) *ast.CaseClause {
	if function == nil {
		return nil
	}
	var found *ast.CaseClause
	ast.Inspect(function.Body, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expression := range clause.List {
			if ident, ok := expression.(*ast.Ident); ok && ident.Name == constant {
				if found != nil {
					found = nil
					return false
				}
				found = clause
			}
		}
		return true
	})
	return found
}

// ─── PIB-554 ──────────────────────────────────────────────────────────────────

func s7AWValidateClassCollapseSource(sources map[string]string) error {
	if err := s7AVValidateAdmissionPredicate(sources); err != nil {
		return err
	}
	program, err := s7AVParseProgram(sources, []string{s7AVStoreArchiveSource})
	if err != nil {
		return err
	}
	inspect := program.function("InspectIntentArchive")
	if inspect == nil {
		return errors.New("InspectIntentArchive is missing")
	}
	hashLoop := (*ast.RangeStmt)(nil)
	ast.Inspect(inspect.Body, func(node ast.Node) bool {
		loop, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if ident, ok := loop.X.(*ast.Ident); ok && ident.Name == "hashes" {
			hashLoop = loop
			return false
		}
		return true
	})
	if hashLoop == nil {
		return errors.New("the classifier no longer has one per-hash collapse loop")
	}

	classAssignments := 0
	classSetAppends := 0
	directClassSetAppends := 0
	ast.Inspect(hashLoop.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, left := range assignment.Lhs {
			selector, ok := left.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "RepairClass" {
				classAssignments++
			}
			index, ok := left.(*ast.IndexExpr)
			if !ok {
				continue
			}
			base, ok := index.X.(*ast.Ident)
			if !ok || base.Name != "classInstances" {
				continue
			}
			classSetAppends++
			switch key := index.Index.(type) {
			case *ast.SelectorExpr:
				if key.Sel.Name != "RepairClass" {
					directClassSetAppends++
				}
			case *ast.Ident:
				if key.Name != "hashReport" {
					directClassSetAppends++
				}
			default:
				directClassSetAppends++
			}
		}
		return true
	})
	if classAssignments != 4 {
		return fmt.Errorf("the per-hash collapse assigns %d class arms, want exactly 4", classAssignments)
	}
	if classSetAppends != 1 || directClassSetAppends != 0 {
		return fmt.Errorf(
			"one hash is inserted into %d class sets (%d direct constant-key insertions)",
			classSetAppends, directClassSetAppends,
		)
	}

	build := program.function("BuildIntentArchivePurgePlan")
	if build == nil {
		return errors.New("BuildIntentArchivePurgePlan is missing")
	}
	inspectCalls := s7AVCalls(build.Body, "InspectIntentArchive")
	selectCalls := s7AVCalls(build.Body, "selectIntentArchivePurgeTargets")
	admitCalls := s7AVCalls(build.Body, "admitIntentArchiveRepair")
	if len(inspectCalls) != 1 || len(selectCalls) != 1 || len(admitCalls) != 1 {
		return fmt.Errorf(
			"class-collapse/selection call counts = inspect:%d select:%d admit:%d, want 1/1/1",
			len(inspectCalls), len(selectCalls), len(admitCalls),
		)
	}
	if inspectCalls[0].Pos() > selectCalls[0].Pos() ||
		selectCalls[0].Pos() > admitCalls[0].Pos() {
		return errors.New("class membership is computed after selection, so collapse cannot constrain admission")
	}
	return nil
}

type s7AWOverlapFixture struct {
	root    string
	slug    string
	hash    string
	blobRel string
}

func s7AWWriteOverlapFixture(t *testing.T, owned bool) s7AWOverlapFixture {
	t.Helper()
	root, slug := prepareS4Workspace(t, "S7 AW PIB 554")
	prepareS4WriteReadyBundle(t, root, slug, true)
	data := []byte("PIB-554 mixed and unidentifiable\n")
	retained := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained,
	)
	tombstoned := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireTombstoned,
	)
	generations := []store.IntentArchiveGeneration{
		intentArchiveCLIGeneration(t, slug, retained),
		intentArchiveCLIGeneration(t, slug, tombstoned),
	}
	if owned {
		pending := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactExploration, data, store.IntentArchiveWireRemovalPending,
		)
		generations = append(generations, intentArchiveCLIGeneration(t, slug, pending))
	}
	writeIntentArchiveCLIFixture(
		t, root, slug, intentArchiveCLIIndex(t, slug, generations...),
		map[string][]byte{retained.ContentSHA256: data},
	)
	blobRel, err := store.IntentArchiveBlobRel(slug, retained.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	blobPath := filepath.Join(root, filepath.FromSlash(blobRel))
	if err := os.Remove(blobPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(blobPath, 0o700); err != nil {
		t.Fatal(err)
	}
	s7AVDirectoryWithTwoFiles(t, blobPath)
	return s7AWOverlapFixture{
		root: root, slug: slug, hash: retained.ContentSHA256, blobRel: blobRel,
	}
}

func s7AWMoveCollapseAfterSelection(t *testing.T, source string) string {
	t.Helper()
	block := "\tinspection, err := InspectIntentArchive(snapshot.Index, snapshot.BlobObservations)\n" +
		"\tif err != nil {\n" +
		"\t\treturn plan, err\n" +
		"\t}\n" +
		"\tsnapshot.Inspection = inspection\n" +
		"\tplan.ObservedClasses = cloneIntentArchiveClassReports(inspection.Classes)\n"
	if !strings.Contains(source, block) {
		t.Fatal("PIB-554 collapse block anchor is missing")
	}
	mutated := strings.Replace(
		source, block,
		"\tinspection := snapshot.Inspection\n\tvar err error\n",
		1,
	)
	anchor := "\tif selectErr != nil {\n\t\treturn plan, selectErr\n\t}\n"
	insert := anchor +
		"\tinspection, err = InspectIntentArchive(snapshot.Index, snapshot.BlobObservations)\n" +
		"\tif err != nil {\n" +
		"\t\treturn plan, err\n" +
		"\t}\n" +
		"\tsnapshot.Inspection = inspection\n" +
		"\tplan.ObservedClasses = cloneIntentArchiveClassReports(inspection.Classes)\n"
	replaced := strings.Replace(mutated, anchor, insert, 1)
	if replaced == mutated {
		t.Fatal("PIB-554 selection anchor is missing")
	}
	return replaced
}

func TestS7AWClassCollapseGuard(t *testing.T) {
	sources := s7AVRepoSources(t, s7AVStoreArchiveSource)
	if err := s7AWValidateClassCollapseSource(sources); err != nil {
		t.Fatalf("PIB-554 baseline source validation failed: %v", err)
	}

	overlap := s7AWWriteOverlapFixture(t, false)
	code, stdout, _, _ := runPrepare(
		t, "--path", overlap.root,
		"feature", "intent-archive", "list", overlap.slug, "--json", "--quiet",
	)
	if code != 3 {
		t.Fatalf("PIB-554 overlap list exit=%d, want 3\n%s", code, stdout)
	}
	list := decodeIntentArchiveListReport(t, stdout)
	corrupt := 0
	for _, generation := range list.Generations {
		for _, entry := range generation.Entries {
			if entry.ContentSHA256 != overlap.hash {
				continue
			}
			if entry.Storage == "mixed-reference" {
				t.Fatalf("PIB-554 overlapping hash leaked into mixed-reference\n%s", stdout)
			}
			if entry.Storage == "corrupt" {
				corrupt++
			}
		}
	}
	if corrupt != 2 {
		t.Fatalf("PIB-554 corrupt reference count = %d, want 2\n%s", corrupt, stdout)
	}
	before := readTree(t, filepath.Join(overlap.root, ".tpatch"))
	code, stdout, _, _ = runPrepare(
		t, "--path", overlap.root,
		"feature", "intent-archive", "purge", overlap.slug,
		"--blob", overlap.hash, "--yes", "--json", "--quiet",
	)
	if code != 3 ||
		decodeIntentArchivePurgeReport(t, stdout).Refusal == nil ||
		!bytes.Equal(before, readTree(t, filepath.Join(overlap.root, ".tpatch"))) {
		t.Fatalf("PIB-554 mixed-looking selector escaped corrupt precedence: exit=%d\n%s", code, stdout)
	}

	owned := s7AWWriteOverlapFixture(t, true)
	code, stdout, _, _ = runPrepare(
		t, "--path", owned.root, "prepare", owned.slug, "--json", "--quiet",
	)
	prepareReport := prepareS4Report(t, stdout)
	if code != 3 || prepareReport.Refusal == nil ||
		prepareReport.Refusal.Code != string(store.IntentArchiveCodeRecoveryPending) ||
		strings.Contains(stdout, string(store.IntentArchiveCodeBlobCorrupt)) {
		t.Fatalf("PIB-554 owned overlap did not escape to its owner: exit=%d report=%#v\n%s",
			code, prepareReport, stdout)
	}

	fixtures := []struct {
		name      string
		mutate    func(string) string
		wantClass string
	}{
		{
			name: "one-hash-in-two-class-sets",
			mutate: func(source string) string {
				old := "\t\t\tclassInstances[hashReport.RepairClass] = append(classInstances[hashReport.RepairClass], instance)\n"
				new := old +
					"\t\t\tif hashReport.RepairClass == IntentArchiveRepairCorruptObject {\n" +
					"\t\t\t\tclassInstances[IntentArchiveRepairMixedReference] = append(classInstances[IntentArchiveRepairMixedReference], instance)\n" +
					"\t\t\t}\n"
				return strings.Replace(source, old, new, 1)
			},
			wantClass: "inserted into",
		},
		{
			name: "mixed-directory-demoted",
			mutate: func(source string) string {
				old := "		case observation.State == IntentArchiveBlobUnidentifiable:\n" +
					"			hashReport.RepairClass = IntentArchiveRepairCorruptObject\n"
				new := "		case observation.State == IntentArchiveBlobUnidentifiable && hasRetained && hasTombstone:\n" +
					"			hashReport.RepairClass = IntentArchiveRepairMixedReference\n" +
					"		case observation.State == IntentArchiveBlobUnidentifiable:\n" +
					"			hashReport.RepairClass = IntentArchiveRepairCorruptObject\n"
				return strings.Replace(source, old, new, 1)
			},
			wantClass: "corrupt-object class second",
		},
		{
			name: "collapse-after-selection",
			mutate: func(source string) string {
				return s7AWMoveCollapseAfterSelection(t, source)
			},
			wantClass: "computed after selection",
		},
	}
	for _, fixture := range fixtures {
		mutated := map[string]string{
			s7AVStoreArchiveSource: fixture.mutate(sources[s7AVStoreArchiveSource]),
		}
		if mutated[s7AVStoreArchiveSource] == sources[s7AVStoreArchiveSource] {
			t.Fatalf("PIB-554 sensitivity %q changed nothing", fixture.name)
		}
		err := s7AWValidateClassCollapseSource(mutated)
		if err == nil || !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-554 sensitivity %q: want %q, got %v",
				fixture.name, fixture.wantClass, err)
		}
	}
}

// ─── PIB-555 ──────────────────────────────────────────────────────────────────

func s7AWValidateNonDegradationSource(sources map[string]string) error {
	if err := s7AVValidateAdmissionPredicate(sources); err != nil {
		return err
	}
	program, err := s7AVParseProgram(sources, []string{s7AVStoreArchiveSource})
	if err != nil {
		return err
	}
	selectTargets := program.function("selectIntentArchivePurgeTargets")
	blobCase := s7AWFunctionCase(selectTargets, "IntentArchiveSelectorBlob")
	orphanCase := s7AWFunctionCase(selectTargets, "IntentArchiveSelectorOrphans")
	if blobCase == nil || orphanCase == nil {
		return errors.New("the purge target selector lost its blob or orphan arm")
	}
	blobNames := s7AVIdentNames(blobCase)
	if blobNames["Blobs"] == 0 || blobNames["Generations"] != 0 ||
		blobNames["Replaced"] != 0 || blobNames["Orphans"] != 0 {
		return errors.New("the repeated --blob selector widens beyond its explicit selected hashes")
	}
	orphanNames := s7AVIdentNames(orphanCase)
	if orphanNames["Orphans"] == 0 || orphanNames["Hash"] == 0 ||
		orphanNames["Generations"] != 0 || orphanNames["Replaced"] != 0 {
		return errors.New("--orphans is not derived only from the global orphan set")
	}

	orphanExecution := program.function("executeIntentArchiveOrphanPurge")
	if orphanExecution == nil {
		return errors.New("executeIntentArchiveOrphanPurge is missing")
	}
	for _, forbidden := range []string{
		"CASIndex", "publishIntentArchiveIndex", "setIntentArchiveHashState",
	} {
		if len(s7AVCalls(orphanExecution.Body, forbidden)) != 0 {
			return fmt.Errorf("--orphans rewrites the index through %s", forbidden)
		}
	}
	orphanRanges := 0
	ast.Inspect(orphanExecution.Body, func(node ast.Node) bool {
		loop, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		selector, ok := loop.X.(*ast.SelectorExpr)
		base, baseOK := selector.X.(*ast.Ident)
		if ok && baseOK && base.Name == "plan" && selector.Sel.Name == "BlobRemovals" {
			orphanRanges++
		}
		return true
	})
	if orphanRanges != 1 {
		return fmt.Errorf("--orphans ranges over its exact blob-removal set %d times, want 1", orphanRanges)
	}

	hashExecution := program.function("executeIntentArchiveHashPurge")
	if hashExecution == nil {
		return errors.New("executeIntentArchiveHashPurge is missing")
	}
	hashRanges := 0
	exactHashCalls := 0
	ast.Inspect(hashExecution.Body, func(node ast.Node) bool {
		loop, ok := node.(*ast.RangeStmt)
		if ok {
			selector, selectorOK := loop.X.(*ast.SelectorExpr)
			base, baseOK := selector.X.(*ast.Ident)
			if selectorOK && baseOK && base.Name == "plan" && selector.Sel.Name == "Hashes" {
				hashRanges++
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || s7AVCallName(call) != "executeIntentArchivePurgeHash" {
			return true
		}
		for _, argument := range call.Args {
			if ident, ok := argument.(*ast.Ident); ok && ident.Name == "hash" {
				exactHashCalls++
			}
		}
		return true
	})
	if hashRanges != 1 || exactHashCalls != 1 {
		return fmt.Errorf(
			"hash purge write set = %d plan.Hashes loops / %d current-hash calls, want 1/1",
			hashRanges, exactHashCalls,
		)
	}
	return nil
}

func TestS7AWNonDegradationWriteSetGuard(t *testing.T) {
	sources := s7AVRepoSources(t, s7AVStoreArchiveSource)
	if err := s7AWValidateNonDegradationSource(sources); err != nil {
		t.Fatalf("PIB-555 baseline source validation failed: %v", err)
	}

	orphanArchive := s7AVWriteRepairArchive(
		t, "PIB-555-orphans",
		s7AVRepairSpec{residues: 2, mixed: 2, ready: true},
	)
	indexBefore := orphanArchive.indexBytes(t)
	mixedBefore := map[string][]byte{}
	for _, hash := range orphanArchive.mixed {
		body, err := os.ReadFile(orphanArchive.abs(orphanArchive.blobRel[hash]))
		if err != nil {
			t.Fatal(err)
		}
		mixedBefore[hash] = body
	}
	code, stdout, stderr, _, removed, indexCASs :=
		s7AWRunPurgeWithMutationSpy(t, orphanArchive, []string{"--orphans"})
	wantOrphanRemovals := []string{}
	for _, hash := range orphanArchive.residues {
		wantOrphanRemovals = append(wantOrphanRemovals, orphanArchive.blobRel[hash])
	}
	sort.Strings(removed)
	sort.Strings(wantOrphanRemovals)
	if code != 0 || stderr != "" ||
		!reflect.DeepEqual(removed, wantOrphanRemovals) || indexCASs != 0 {
		t.Fatalf("PIB-555 orphan write set = exit:%d stderr:%q removed:%v index-CAS:%d\n%s",
			code, stderr, removed, indexCASs, stdout)
	}
	if !bytes.Equal(indexBefore, orphanArchive.indexBytes(t)) {
		t.Fatal("PIB-555 --orphans changed index bytes")
	}
	for hash, before := range mixedBefore {
		after, err := os.ReadFile(orphanArchive.abs(orphanArchive.blobRel[hash]))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("PIB-555 --orphans degraded mixed hash %s: %v", hash, err)
		}
	}

	hashArchive := s7AVWriteRepairArchive(
		t, "PIB-555-blob",
		s7AVRepairSpec{dangling: 1, mixed: 2, ready: true},
	)
	beforeDangling := hashArchive.tree(t)
	code, stdout, stderr, _, removed, _ =
		s7AWRunPurgeWithMutationSpy(t, hashArchive, s7AVBlobSelector(hashArchive.mixed))
	wantMixedRemovals := []string{}
	for _, hash := range hashArchive.mixed {
		wantMixedRemovals = append(wantMixedRemovals, hashArchive.blobRel[hash])
	}
	sort.Strings(removed)
	sort.Strings(wantMixedRemovals)
	if code != 0 || stderr != "" ||
		!reflect.DeepEqual(removed, wantMixedRemovals) {
		t.Fatalf("PIB-555 blob write set = exit:%d stderr:%q removed:%v\n%s",
			code, stderr, removed, stdout)
	}
	_, afterIndex := readIntentArchiveCLIIndex(t, hashArchive.root, hashArchive.slug)
	for _, hash := range hashArchive.dangling {
		for _, state := range s7ATWireStates(afterIndex, hash) {
			if state != store.IntentArchiveWireRetained {
				t.Fatalf("PIB-555 selected mixed hashes reclassified dangling hash %s to %s", hash, state)
			}
		}
	}
	if bytes.Equal(beforeDangling, hashArchive.tree(t)) {
		t.Fatal("PIB-555 selected mixed repair performed no real mutation")
	}

	fixtures := []struct {
		name      string
		old       string
		new       string
		wantClass string
	}{
		{
			name: "orphans-rewrites-index",
			old: "\t\tRemainingRepairs: plan.RemainingRepairs,\n" +
				"\t}\n" +
				"\tindexRel, _ := IntentArchiveIndexRel(plan.Feature)\n",
			new: "\t\tRemainingRepairs: plan.RemainingRepairs,\n" +
				"\t}\n" +
				"\tindexRel, _ := IntentArchiveIndexRel(plan.Feature)\n" +
				"\t_, _ = storage.CASIndex(indexRel, snapshot.IndexCapture.Identity, snapshot.IndexCapture.Raw)\n",
			wantClass: "rewrites the index",
		},
		{
			name:      "all-admitted-with-second-class",
			old:       "\t\tif len(inspection.Classes) == 1 {\n",
			new:       "\t\tif len(inspection.Classes) >= 1 {\n",
			wantClass: "not exactly one class",
		},
		{
			name: "blob-widens-to-shared-generation",
			old: "	case IntentArchiveSelectorBlob:\n" +
				"		for _, hash := range selector.Blobs {\n" +
				"			hashSet[hash] = struct{}{}\n" +
				"		}\n",
			new: "	case IntentArchiveSelectorBlob:\n" +
				"		for _, hash := range selector.Blobs {\n" +
				"			hashSet[hash] = struct{}{}\n" +
				"		}\n" +
				"		for _, generation := range snapshot.Index.Generations {\n" +
				"			for _, replacement := range generation.Replaced {\n" +
				"				if _, selected := hashSet[replacement.ContentSHA256]; selected {\n" +
				"					for _, shared := range generation.Replaced {\n" +
				"						hashSet[shared.ContentSHA256] = struct{}{}\n" +
				"					}\n" +
				"				}\n" +
				"			}\n" +
				"		}\n",
			wantClass: "widens beyond",
		},
	}
	for _, fixture := range fixtures {
		mutated := s7AVMutate(
			t, sources, s7AVStoreArchiveSource, fixture.old, fixture.new, 1,
		)
		err := s7AWValidateNonDegradationSource(mutated)
		if err == nil || !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-555 sensitivity %q: want %q, got %v",
				fixture.name, fixture.wantClass, err)
		}
	}
}

// ─── PIB-557 ──────────────────────────────────────────────────────────────────

func s7AWValidateAllDisclosure(text string) error {
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	normalized = strings.NewReplacer(
		"`", "", "**", "", "*", "", "<", "", ">", "",
	).Replace(normalized)
	required := [][]string{
		{"tombstones every reference"},
		{"every generation"},
		{"claims", "selects every retained reference"},
		{"removes every blob"},
		{"no recoverable bytes"},
		{"identical content"},
		{"unconfirmed preview is the default", "preview first, which is the default"},
		{"shows the full selection"},
		{"repeated --blob", "--blob hash --yes over the pending hashes"},
		{"touching nothing else", "touches nothing else"},
	}
	for _, alternatives := range required {
		found := false
		for _, phrase := range alternatives {
			if strings.Contains(normalized, phrase) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("--all disclosure omits one of %q", alternatives)
		}
	}
	return nil
}

func s7AWReportAllRetry(report intentArchivePurgeReport) string {
	switch {
	case report.PendingPurge != nil:
		return report.PendingPurge.Retry
	case report.Recovery != nil:
		return report.Recovery.Retry
	case report.PurgeProgress != nil:
		return report.PurgeProgress.Retry
	default:
		return report.Retry
	}
}

func s7AWReportAllHashes(report intentArchivePurgeReport) []string {
	hashes := append([]string(nil), report.Hashes...)
	if report.PendingPurge != nil {
		for _, pending := range report.PendingPurge.PendingHashes {
			hashes = append(hashes, pending.Hash)
		}
	}
	if report.Recovery != nil {
		hashes = append(hashes, report.Recovery.FinalizedHashes...)
	}
	if report.PurgeProgress != nil {
		hashes = append(hashes, report.PurgeProgress.CompletedHashes...)
		if report.PurgeProgress.PendingHash != "" {
			hashes = append(hashes, report.PurgeProgress.PendingHash)
		}
		hashes = append(hashes, report.PurgeProgress.RemainingHashes...)
	}
	return sortedUniqueIntentArchiveStrings(hashes)
}

func s7AWValidateAllCarrier(
	report intentArchivePurgeReport,
	human string,
	requireRetry bool,
) error {
	if report.Selector != string(store.IntentArchiveSelectorAll) {
		return fmt.Errorf("selector = %q, want all", report.Selector)
	}
	if err := s7AWValidateAllDisclosure(report.BlastRadius); err != nil {
		return err
	}
	if !strings.Contains(human, report.BlastRadius) {
		return errors.New("human carrier omitted the structured --all disclosure")
	}
	hashes := s7AWReportAllHashes(report)
	if len(hashes) == 0 {
		return errors.New("--all carrier enumerated no hashes")
	}
	for _, hash := range hashes {
		if !strings.Contains(human, hash) {
			return fmt.Errorf("--all carrier did not render enumerable hash %s", hash)
		}
	}
	retry := s7AWReportAllRetry(report)
	if !requireRetry {
		return nil
	}
	if !strings.Contains(retry, " --all --yes") {
		return fmt.Errorf("selector-preserving retry = %q, want unchanged --all", retry)
	}
	disclosureAt := strings.Index(human, report.BlastRadius)
	headerAt := strings.Index(human, prepareRetryHeader)
	retryAt := strings.Index(human, retry)
	if disclosureAt < 0 || headerAt < 0 || retryAt < 0 ||
		disclosureAt > headerAt || headerAt > retryAt {
		return errors.New("--all disclosure is not above the retry heading and unchanged retry")
	}
	afterHeader := human[headerAt+len(prepareRetryHeader):]
	lines := strings.Split(strings.TrimLeft(afterHeader, "\r\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != retry {
		return errors.New("something appears between the retry heading and the preserved --all command")
	}
	return nil
}

func s7AWValidateSharedAllEscalation(
	report intentArchivePurgeReport,
	human string,
	hash string,
) error {
	if report.Refusal == nil ||
		report.Refusal.Code != string(store.IntentArchiveCodeBlobShared) {
		return errors.New("shared-reference carrier is not archive-blob-shared")
	}
	remediation := report.Refusal.Remediation
	if err := s7AWValidateAllDisclosure(remediation); err != nil {
		return err
	}
	preview := "tpatch feature intent-archive purge " + report.Slug + " --all"
	confirmed := preview + " --yes"
	narrow := "tpatch feature intent-archive purge " + report.Slug + " --blob " + hash + " --yes"
	if strings.Index(remediation, preview) < 0 ||
		strings.Index(remediation, preview) > strings.Index(remediation, confirmed) ||
		report.Refusal.Retry != narrow ||
		!strings.Contains(remediation, narrow) ||
		!strings.Contains(human, remediation) {
		return errors.New("shared-reference escalation lost preview/confirm/narrow ordering")
	}
	return nil
}

func s7AWValidateAllEmitterSource(source string) error {
	program, err := s7AVParseProgram(
		map[string]string{s7AVCLIArchiveSource: source},
		[]string{s7AVCLIArchiveSource},
	)
	if err != nil {
		return err
	}
	newReport := program.function("newIntentArchivePurgeReport")
	retry := program.function("intentArchivePurgeRetry")
	refusal := program.function("intentArchiveRefusalFromError")
	if newReport == nil || retry == nil || refusal == nil {
		return errors.New("one of the three --all output producers is missing")
	}
	if s7AVIdentNames(newReport)["BlastRadius"] != 1 ||
		s7AVIdentNames(newReport)["all"] == 0 {
		return errors.New("the --all report producer is not bound to the all selector")
	}
	retryAll := false
	ast.Inspect(retry.Body, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		names := s7AVIdentNames(clause)
		literals := []string{}
		ast.Inspect(clause, func(current ast.Node) bool {
			literal, ok := current.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr == nil {
				literals = append(literals, value)
			}
			return true
		})
		if names["all"] != 0 && reflect.DeepEqual(literals, []string{"--all"}) {
			retryAll = true
		}
		return true
	})
	if !retryAll {
		return errors.New("the selector-preserving retry does not append exactly --all")
	}
	shared := false
	ast.Inspect(refusal.Body, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok || s7AVIdentNames(clause)["IntentArchiveCodeBlobShared"] == 0 {
			return true
		}
		names := s7AVIdentNames(clause)
		shared = names["allPreview"] != 0 &&
			names["allConfirmed"] != 0 &&
			names["intentArchiveAllBlastRadius"] != 0
		return true
	})
	if !shared {
		return errors.New("archive-blob-shared is not bound to the common --all disclosure")
	}

	derived := map[string]bool{}
	for name, function := range program.functions() {
		hasAll := false
		hasOutput := name == "intentArchivePurgeRetry"
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.BasicLit:
				if typed.Kind == token.STRING {
					value, unquoteErr := strconv.Unquote(typed.Value)
					hasAll = hasAll || (unquoteErr == nil && strings.Contains(value, "--all"))
				}
			case *ast.SelectorExpr:
				switch typed.Sel.Name {
				case "BlastRadius", "Remediation", "Retry":
					hasOutput = true
				}
			case *ast.Ident:
				if typed.Name == "intentArchiveAllBlastRadius" {
					hasAll = true
				}
			}
			return true
		})
		if hasAll && hasOutput {
			derived[name] = true
		}
	}
	for _, producer := range []string{
		"newIntentArchivePurgeReport",
		"intentArchiveRefusalFromError",
		"intentArchivePurgeRetry",
	} {
		if !derived[producer] {
			return fmt.Errorf("source derivation omitted --all producer %s", producer)
		}
	}
	if len(derived) != 3 {
		return fmt.Errorf("source derives unguarded --all output producers: %v", derived)
	}
	return nil
}

func s7AWMarkdownFences(document string) []string {
	lines := strings.Split(document, "\n")
	blocks := []string{}
	var current []string
	fenced := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if fenced {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
			}
			fenced = !fenced
			continue
		}
		if fenced {
			current = append(current, line)
		}
	}
	return blocks
}

func s7AWValidateAllExamples(prd, adr string) error {
	examples := 0
	allToken := regexp.MustCompile(`(^|[[:space:]])--all([^[:alnum:]-]|$)`)
	for _, document := range []string{prd, adr} {
		for _, block := range s7AWMarkdownFences(document) {
			if !strings.Contains(block, "tpatch feature intent-archive purge") ||
				!allToken.MatchString(block) ||
				strings.Contains(block, "(--blob") {
				continue
			}
			examples++
			if strings.Contains(block, `"retry"`) {
				if !strings.Contains(block, `"selector": "all"`) ||
					!strings.Contains(block, "--all --yes") {
					return errors.New("JSON --all example does not carry selector/retry parity")
				}
				continue
			}
			if err := s7AWValidateAllDisclosure(block); err != nil {
				return fmt.Errorf("human --all example: %w", err)
			}
			header := strings.Index(block, prepareRetryHeader)
			command := strings.Index(block, "tpatch feature intent-archive purge")
			if header < 0 || command < header {
				return errors.New("human --all example does not preserve heading/command order")
			}
		}
	}
	if examples != 2 {
		return fmt.Errorf("derived --all worked examples = %d, want 2", examples)
	}
	for _, section := range []struct {
		label string
		body  string
		start string
		end   string
	}{
		{
			label: "PRD sequential-admission disclosure",
			body:  prd,
			start: "**`--all --yes` is admitted only under an explicit",
			end:   "**Repairs are counted in *stages*",
		},
	} {
		begin := strings.Index(section.body, section.start)
		finish := strings.Index(section.body, section.end)
		if begin < 0 || finish <= begin {
			return fmt.Errorf("%s anchors are missing", section.label)
		}
		if err := s7AWValidateAllDisclosure(section.body[begin:finish]); err != nil {
			return fmt.Errorf("%s: %w", section.label, err)
		}
	}
	if !strings.Contains(adr, "**`--all --yes` is the one selector") ||
		!strings.Contains(adr, "PIB-557") {
		return errors.New("ADR D16 does not bind its --all emitter rule to PIB-557")
	}
	return nil
}

func s7AWSharedFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	data := []byte("PIB-557 shared reference\n")
	first := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained,
	)
	second := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireRetained,
	)
	g1 := intentArchiveCLIGeneration(t, slug, first)
	g2 := intentArchiveCLIGeneration(t, slug, second)
	writeIntentArchiveCLIFixture(
		t, root, slug, intentArchiveCLIIndex(t, slug, g1, g2),
		map[string][]byte{first.ContentSHA256: data},
	)
	return root, slug, first.ContentSHA256, g1.GenerationID
}

func TestS7AWAllEmitterAuthorityGuard(t *testing.T) {
	source := s6RepoFile(t, s7AVCLIArchiveSource)
	if err := s7AWValidateAllEmitterSource(source); err != nil {
		t.Fatalf("PIB-557 baseline source validation failed: %v", err)
	}
	if err := s7AVValidateAdmissionPredicate(
		s7AVRepoSources(t, s7AVStoreArchiveSource),
	); err != nil {
		t.Fatalf("PIB-557 --all repair admission validation failed: %v", err)
	}

	type carrier struct {
		name         string
		report       intentArchivePurgeReport
		human        string
		requireRetry bool
	}
	carriers := []carrier{}

	previewArchive := s7AVWriteRepairArchive(
		t, "PIB-557-preview", s7AVRepairSpec{dangling: 1, ready: true},
	)
	code, stdout, human, _ := runPrepare(
		t, "--path", previewArchive.root,
		"feature", "intent-archive", "purge", previewArchive.slug,
		"--all", "--json",
	)
	if code != 0 {
		t.Fatalf("PIB-557 all preview exit=%d\n%s", code, stdout)
	}
	carriers = append(carriers, carrier{
		name: "ordinary-preview", report: decodeIntentArchivePurgeReport(t, stdout),
		human: human, requireRetry: true,
	})

	pendingRoot, pendingSlug := intentArchiveCLIWorkspace(t)
	s7ASWritePendingArchiveFixture(t, pendingRoot, pendingSlug, 2)
	code, stdout, human, _ = runPrepare(
		t, "--path", pendingRoot,
		"feature", "intent-archive", "purge", pendingSlug,
		"--all", "--json",
	)
	if code != 0 {
		t.Fatalf("PIB-557 pending preview exit=%d\n%s", code, stdout)
	}
	carriers = append(carriers, carrier{
		name: "pending-preview", report: decodeIntentArchivePurgeReport(t, stdout),
		human: human, requireRetry: true,
	})

	partial := s7APRunPartialPurge(t)
	var partialHuman bytes.Buffer
	writeIntentArchivePurgeHuman(&partialHuman, partial.report)
	carriers = append(carriers, carrier{
		name: "partial-retry", report: partial.report,
		human: partialHuman.String(), requireRetry: true,
	})
	recoveryArgv, err := s7APParseRenderedCommand(partial.report.PurgeProgress.Retry)
	if err != nil {
		t.Fatal(err)
	}
	recoveryCode, recoveryOut, recoveryErr := s7APRunFromWorkspace(
		t, partial.root, recoveryArgv,
	)
	recovery := decodeIntentArchivePurgeReport(t, recoveryOut)
	if recoveryCode != 0 || recoveryErr != "" ||
		recovery.Outcome != string(store.IntentArchivePurgeRecovered) {
		t.Fatalf("PIB-557 recovery carrier = exit:%d stderr:%q report:%#v",
			recoveryCode, recoveryErr, recovery)
	}
	var recoveryHuman bytes.Buffer
	writeIntentArchivePurgeHuman(&recoveryHuman, recovery)
	carriers = append(carriers, carrier{
		name: "terminal-recovery", report: recovery,
		human: recoveryHuman.String(), requireRetry: true,
	})

	for _, item := range carriers {
		if err := s7AWValidateAllCarrier(
			item.report, item.human, item.requireRetry,
		); err != nil {
			t.Fatalf("PIB-557 %s: %v\n%s", item.name, err, item.human)
		}
	}

	sharedRoot, sharedSlug, sharedHash, generation := s7AWSharedFixture(t)
	code, stdout, human, _ = runPrepare(
		t, "--path", sharedRoot,
		"feature", "intent-archive", "purge", sharedSlug,
		"--generation", generation, "--yes", "--json",
	)
	shared := decodeIntentArchivePurgeReport(t, stdout)
	if code != 3 {
		t.Fatalf("PIB-557 shared escalation exit=%d\n%s", code, stdout)
	}
	if err := s7AWValidateSharedAllEscalation(
		shared, human, sharedHash,
	); err != nil {
		t.Fatalf("PIB-557 shared escalation: %v\n%s", err, human)
	}

	if err := s7AWValidateAllExamples(
		s7AVRepoDocument(t, s7AVPRDRelPath),
		s7AVRepoDocument(t, s7AVADRRelPath),
	); err != nil {
		t.Fatal(err)
	}

	baseline := carriers[0]
	for _, fixture := range []struct {
		name   string
		mutate func(*intentArchivePurgeReport, *string)
	}{
		{
			name: "bare-all-command",
			mutate: func(report *intentArchivePurgeReport, human *string) {
				*human = strings.ReplaceAll(*human, report.BlastRadius+"\n", "")
				report.BlastRadius = ""
			},
		},
		{
			name: "disclosure-in-later-paragraph",
			mutate: func(report *intentArchivePurgeReport, human *string) {
				*human = strings.ReplaceAll(*human, report.BlastRadius+"\n", "")
				*human += report.BlastRadius + "\n"
			},
		},
		{
			name: "narrower-alternative-omitted",
			mutate: func(report *intentArchivePurgeReport, human *string) {
				old := report.BlastRadius
				report.BlastRadius = strings.Replace(
					old,
					"; repeated --blob selectors over the hashes listed in this report cover the same work while touching nothing else",
					"",
					1,
				)
				*human = strings.Replace(*human, old, report.BlastRadius, 1)
			},
		},
		{
			name: "preserved-retry-silently-narrowed",
			mutate: func(report *intentArchivePurgeReport, human *string) {
				old := report.Retry
				report.Retry = strings.Replace(
					old, " --all --yes", " --blob "+report.Hashes[0]+" --yes", 1,
				)
				*human = strings.Replace(*human, old, report.Retry, 1)
			},
		},
	} {
		report := baseline.report
		human := baseline.human
		fixture.mutate(&report, &human)
		if err := s7AWValidateAllCarrier(report, human, true); err == nil {
			t.Fatalf("PIB-557 sensitivity fixture %q was accepted", fixture.name)
		}
	}
}

// ─── PIB-558 ──────────────────────────────────────────────────────────────────

func s7AWValidateOwnedCorruptRouteSource(sources map[string]string, document string) error {
	if err := s7AVValidateDispositionTotality(
		map[string]string{s7AVStoreArchiveSource: sources[s7AVStoreArchiveSource]},
		document,
	); err != nil {
		return err
	}
	program, err := s7AVParseProgram(
		sources,
		[]string{s7AVStoreArchiveSource, s7AVCLIArchiveSource},
	)
	if err != nil {
		return err
	}
	failure := program.function("emitIntentArchivePurgeFailure")
	refusal := program.function("intentArchiveRefusalFromError")
	if failure == nil || refusal == nil {
		return errors.New("owned-corrupt CLI routing functions are missing")
	}
	divergentFailure := false
	ast.Inspect(failure.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		names := s7AVIdentNames(branch)
		if names["IntentArchiveCodePurgeEvidenceDivergent"] == 0 {
			return true
		}
		divergentFailure =
			names["buildIntentArchiveDivergence"] != 0 &&
				names["emitIntentArchivePurgeReport"] != 0
		literals := []int{}
		ast.Inspect(branch.Body, func(current ast.Node) bool {
			literal, ok := current.(*ast.BasicLit)
			if ok && literal.Kind == token.INT {
				value, parseErr := strconv.Atoi(literal.Value)
				if parseErr == nil {
					literals = append(literals, value)
				}
			}
			return true
		})
		divergentFailure = divergentFailure && slicesContainInt(literals, 6)
		return true
	})
	if !divergentFailure {
		return errors.New("owned unidentifiable evidence is not emitted through the exit-6 divergence procedure")
	}

	divergentRefusal := false
	ast.Inspect(refusal.Body, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok || s7AVIdentNames(clause)["IntentArchiveCodePurgeEvidenceDivergent"] == 0 {
			return true
		}
		text := ""
		ast.Inspect(clause, func(current ast.Node) bool {
			literal, ok := current.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr == nil {
				text += " " + value
			}
			return true
		})
		divergentRefusal =
			strings.Contains(text, "archive-specific divergence procedure") &&
				!strings.Contains(text, "--abandon-transaction")
		return true
	})
	if !divergentRefusal {
		return errors.New("owned-corrupt exit 6 is routed to abandon instead of the archive procedure")
	}
	return nil
}

func slicesContainInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func s7AWRunOwnedRecoveryFixture(
	t *testing.T,
	absent bool,
) intentArchivePurgeReport {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	fixture := s7ASWritePendingArchiveFixture(t, root, slug, 1)
	if absent {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(fixture.blobRels[fixture.hashes[0]]))); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root,
		"feature", "intent-archive", "purge", slug,
		"--blob", fixture.hashes[0], "--yes", "--json", "--quiet",
	)
	report := decodeIntentArchivePurgeReport(t, stdout)
	if code != 0 || stderr != "" ||
		report.Outcome != string(store.IntentArchivePurgeRecovered) ||
		report.Recovery == nil ||
		!reflect.DeepEqual(report.Recovery.FinalizedHashes, fixture.hashes) {
		t.Fatalf("PIB-558 owned recovery absent=%t = exit:%d stderr:%q report:%#v\n%s",
			absent, code, stderr, report, stdout)
	}
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	for _, state := range s7ATWireStates(index, fixture.hashes[0]) {
		if state != store.IntentArchiveWireTombstoned {
			t.Fatalf("PIB-558 owned recovery absent=%t left state %s", absent, state)
		}
	}
	return report
}

func TestS7AWOwnedCorruptRouteGuard(t *testing.T) {
	sources := s7AVRepoSources(t, s7AVStoreArchiveSource, s7AVCLIArchiveSource)
	document := s7AVRepoDocument(t, s7AVPRDRelPath)
	if err := s7AWValidateOwnedCorruptRouteSource(sources, document); err != nil {
		t.Fatalf("PIB-558 baseline source validation failed: %v", err)
	}

	owned := s7AWWriteOverlapFixture(t, true)
	code, stdout, _, _ := runPrepare(
		t, "--path", owned.root,
		"feature", "intent-archive", "list", owned.slug, "--json", "--quiet",
	)
	if code != 0 {
		t.Fatalf("PIB-558 owned-corrupt list exit=%d, want 0\n%s", code, stdout)
	}
	list := decodeIntentArchiveListReport(t, stdout)
	ownedReferences := 0
	for _, generation := range list.Generations {
		for _, entry := range generation.Entries {
			if entry.ContentSHA256 != owned.hash {
				continue
			}
			ownedReferences++
			if entry.Storage != "pending-remove" ||
				strings.Contains(entry.Repair, "archive-blob-corrupt") {
				t.Fatalf("PIB-558 owned reference route = %#v", entry)
			}
		}
	}
	if ownedReferences != 3 {
		t.Fatalf("PIB-558 owned reference count = %d, want retained/pending/tombstoned", ownedReferences)
	}
	code, stdout, stderr := s7APRunFromWorkspace(t, owned.root, []string{
		"feature", "intent-archive", "purge", owned.slug,
		"--blob", owned.hash, "--yes", "--json",
	})
	divergent := decodeIntentArchivePurgeReport(t, stdout)
	if code != 6 || divergent.Refusal == nil ||
		divergent.Refusal.Code != string(store.IntentArchiveCodePurgeEvidenceDivergent) ||
		divergent.Divergence == nil ||
		divergent.Divergence.Kind != "blob" ||
		divergent.Divergence.RemoveCommand != "rm -rf -- "+owned.blobRel ||
		strings.Contains(stdout+stderr, "--abandon-transaction") ||
		strings.Contains(stdout, string(store.IntentArchiveCodeBlobCorrupt)) ||
		strings.Contains(stdout, string(store.IntentArchiveCodeBlobDangling)) {
		t.Fatalf("PIB-558 owned-corrupt route = exit:%d stderr:%q report:%#v\n%s",
			code, stderr, divergent, stdout)
	}

	s7AWRunOwnedRecoveryFixture(t, false)
	s7AWRunOwnedRecoveryFixture(t, true)

	nonOwned := s7AWWriteOverlapFixture(t, false)
	code, stdout, _, _ = runPrepare(
		t, "--path", nonOwned.root,
		"feature", "intent-archive", "purge", nonOwned.slug,
		"--blob", nonOwned.hash, "--yes", "--json", "--quiet",
	)
	nonOwnedReport := decodeIntentArchivePurgeReport(t, stdout)
	if code != 3 || nonOwnedReport.Refusal == nil ||
		nonOwnedReport.Refusal.Code != string(store.IntentArchiveCodeBlobCorrupt) ||
		nonOwnedReport.Divergence != nil {
		t.Fatalf("PIB-558 non-owned corrupt population widened to exit 6: %#v", nonOwnedReport)
	}

	fixtures := []struct {
		name      string
		source    string
		old       string
		new       string
		wantClass string
	}{
		{
			name:   "pending-unsafe-keeps-exit-three",
			source: s7AVStoreArchiveSource,
			old: "\tif owned {\n" +
				"\t\terr := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, \"the owned blob is present but unidentifiable\", 6)\n",
			new: "\tif owned {\n" +
				"\t\terr := intentArchiveError(IntentArchiveCodeIndexStorageInconsistent, \"the owned blob is present but unidentifiable\", 3)\n",
			wantClass: "owned arm",
		},
		{
			name:   "owned-tombstone-split-to-corrupt",
			source: s7AVStoreArchiveSource,
			old:    "\tif tuple.Owned {\n",
			new: "\tif tuple.Owned && !(tuple.WireState == IntentArchiveWireTombstoned && " +
				"tuple.BlobState == IntentArchiveBlobUnidentifiable) {\n",
			wantClass: "owned tuple",
		},
		{
			name:      "exit-six-routed-to-abandon",
			source:    s7AVCLIArchiveSource,
			old:       "\"Follow the archive-specific divergence procedure in this report; prepare recovery modes cannot repair an archive purge.\"",
			new:       "\"Run tpatch prepare \" + slug + \" --abandon-transaction --yes.\"",
			wantClass: "routed to abandon",
		},
	}
	for _, fixture := range fixtures {
		mutated := map[string]string{}
		for name, body := range sources {
			mutated[name] = body
		}
		mutated[fixture.source] = strings.Replace(
			mutated[fixture.source], fixture.old, fixture.new, 1,
		)
		if mutated[fixture.source] == sources[fixture.source] {
			t.Fatalf("PIB-558 sensitivity %q changed nothing", fixture.name)
		}
		err := s7AWValidateOwnedCorruptRouteSource(mutated, document)
		if err == nil || !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-558 sensitivity %q: want %q, got %v",
				fixture.name, fixture.wantClass, err)
		}
	}
}

// ─── PIB-559 ──────────────────────────────────────────────────────────────────

type s7AWTokenBlock struct {
	label   string
	text    string
	allowCP bool
}

func s7AWTokenBlockCommands(text string) []string {
	var commands []string
	isCommandName := func(name string) bool {
		if name == "tpatch" || name == "rm" || name == "find" {
			return true
		}
		return slices.Contains(s7AVForbiddenCommandTokens, name)
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		candidates := []string{trimmed}
		if index := strings.LastIndex(trimmed, ": "); index >= 0 {
			tail := strings.TrimSpace(trimmed[index+2:])
			fields := strings.Fields(tail)
			if len(fields) != 0 && isCommandName(fields[0]) {
				candidates = append(candidates, tail)
			}
		}
		for _, candidate := range candidates {
			argv, err := s7AVShellSplit(candidate)
			if err != nil || len(argv) < 2 || !s7AVLooksLikeInvocation(argv) {
				continue
			}
			first := argv[0]
			if first == "" || first[0] < 'a' || first[0] > 'z' {
				continue
			}
			commands = append(commands, candidate)
			break
		}
	}
	return commands
}

func s7AWValidateTokenBlock(block s7AWTokenBlock) error {
	if block.text == "" {
		return fmt.Errorf("%s: empty emitted block", block.label)
	}
	allow := map[string]bool{"tpatch": true, "rm": true}
	if block.allowCP {
		allow["cp"] = true
	}
	prose := block.text
	for _, line := range s7AWTokenBlockCommands(block.text) {
		if strings.Contains(line, " | ") ||
			strings.Contains(line, "; ") ||
			strings.Contains(line, " && ") ||
			strings.Contains(line, " || ") {
			return fmt.Errorf("%s: chained structural command line %q", block.label, line)
		}
		argv, err := s7AVShellSplit(line)
		if err != nil {
			return fmt.Errorf("%s: %w", block.label, err)
		}
		if len(argv) == 0 {
			return fmt.Errorf("%s: empty structural command line", block.label)
		}
		if !allow[argv[0]] {
			return fmt.Errorf("%s: command argv[0] %q is outside the closed allowlist", block.label, argv[0])
		}
		prose = strings.ReplaceAll(prose, line, "")
	}
	for _, line := range strings.Split(prose, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasSuffix(trimmed, ":") {
			continue
		}
		argv, err := s7AVShellSplit(trimmed)
		if err != nil || len(argv) < 2 || !s7AVLooksLikeInvocation(argv) {
			continue
		}
		if !allow[argv[0]] {
			return fmt.Errorf("%s: command argv[0] %q is outside the closed allowlist", block.label, argv[0])
		}
	}
	for _, forbidden := range s7AVForbiddenCommandTokens {
		if forbidden == "cp" && block.allowCP {
			continue
		}
		inline := regexp.MustCompile("`\\s*" + regexp.QuoteMeta(forbidden) + "\\b")
		if inline.MatchString(prose) {
			return fmt.Errorf("%s: %q appears as inline command code", block.label, forbidden)
		}
		adjacent := regexp.MustCompile(
			`(^|[^0-9A-Za-z_./-])` + regexp.QuoteMeta(forbidden) +
				`[ \t]+(-{1,2}[A-Za-z]|[^\s]*/[^\s]*)`,
		)
		if adjacent.MatchString(prose) {
			return fmt.Errorf("%s: %q appears in command-invocation prose", block.label, forbidden)
		}
	}
	return nil
}

func s7AWFunctionContainsArchiveRemoval(function *ast.FuncDecl) bool {
	if function == nil || function.Body == nil {
		return false
	}
	hasRemoval := false
	hasArchive := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.BasicLit:
			if typed.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(typed.Value)
			if err != nil {
				return true
			}
			hasRemoval = hasRemoval || strings.Contains(value, "rm -rf --")
			hasArchive = hasArchive ||
				strings.Contains(value, "archive") ||
				strings.Contains(value, "managed blob")
		case *ast.Ident:
			hasArchive = hasArchive ||
				typed.Name == "IntentArchiveRepairCorruptObject" ||
				typed.Name == "IntentArchiveCodePurgeEvidenceDivergent"
		}
		return true
	})
	return hasRemoval && hasArchive
}

func s7AWDeriveArchiveRemovalEmitters(
	sources map[string]string,
) (map[string]bool, error) {
	emitters := map[string]bool{}
	for name, body := range sources {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, body, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !s7AWFunctionContainsArchiveRemoval(function) {
				continue
			}
			emitters[name+":"+function.Name.Name] = true
		}
	}
	return emitters, nil
}

func TestS7AWForbiddenCommandTokenizerGuard(t *testing.T) {
	s7ATInstallDeterministicProvider(t)
	sources := map[string]string{
		"internal/store/intent_archive.go":       s6RepoFile(t, "internal/store/intent_archive.go"),
		"internal/cli/feature_intent_archive.go": s6RepoFile(t, "internal/cli/feature_intent_archive.go"),
		"internal/cli/prepare_publish.go":        s6RepoFile(t, "internal/cli/prepare_publish.go"),
		"internal/workflow/doctor_d9.go":         s6RepoFile(t, "internal/workflow/doctor_d9.go"),
	}
	derived, err := s7AWDeriveArchiveRemovalEmitters(sources)
	if err != nil {
		t.Fatal(err)
	}

	nonOwned := s7AWWriteOverlapFixture(t, false)
	blobCommand := "tpatch feature intent-archive purge " + nonOwned.slug +
		" --blob " + nonOwned.hash + " --yes"
	blocksByEmitter := map[string][]s7AWTokenBlock{}

	code, stdout, _, _ := runPrepare(
		t, "--path", nonOwned.root, "prepare", nonOwned.slug, "--json", "--quiet",
	)
	prepareReport := prepareS4Report(t, stdout)
	if code != 3 || prepareReport.Refusal == nil {
		t.Fatalf("PIB-559 prepare fixture = exit:%d report:%#v", code, prepareReport)
	}
	blocksByEmitter["internal/cli/prepare_publish.go:prepareStoreArchiveFailure"] =
		append(
			blocksByEmitter["internal/cli/prepare_publish.go:prepareStoreArchiveFailure"],
			s7AWTokenBlock{
				label: "prepare refusal",
				text:  prepareReport.Refusal.Remediation,
			},
		)
	multiClass := s7AVWriteRepairArchive(
		t, "PIB-559-prepare-multi",
		s7AVRepairSpec{residues: 1, corrupt: true, ready: true},
	)
	code, stdout, _, _ = runPrepare(
		t, "--path", multiClass.root, "prepare", multiClass.slug, "--json", "--quiet",
	)
	multiPrepare := prepareS4Report(t, stdout)
	if code != 3 || multiPrepare.Refusal == nil {
		t.Fatalf("PIB-559 multi-class prepare fixture = exit:%d report:%#v", code, multiPrepare)
	}
	if commands := s7AWTokenBlockCommands(multiPrepare.Refusal.Remediation); len(commands) != 3 {
		t.Fatalf("PIB-559 multi-class remediation commands = %v, want corrupt, dangling and residue", commands)
	}
	blocksByEmitter["internal/cli/feature_intent_archive.go:intentArchiveCorruptRemovalText"] =
		append(
			blocksByEmitter["internal/cli/feature_intent_archive.go:intentArchiveCorruptRemovalText"],
			s7AWTokenBlock{
				label: "multi-class prepare refusal",
				text:  multiPrepare.Refusal.Remediation,
			},
		)

	code, stdout, _, _ = runPrepare(
		t, "--path", nonOwned.root,
		"feature", "intent-archive", "list", nonOwned.slug, "--json", "--quiet",
	)
	list := decodeIntentArchiveListReport(t, stdout)
	if code != 3 {
		t.Fatalf("PIB-559 list fixture exit=%d\n%s", code, stdout)
	}
	listRepair := ""
	listRetry := ""
	for _, generation := range list.Generations {
		for _, entry := range generation.Entries {
			if entry.ContentSHA256 == nonOwned.hash {
				listRepair = entry.Repair
				listRetry = entry.Retry
			}
		}
	}
	if listRepair == "" || listRetry != blobCommand {
		t.Fatalf("PIB-559 list fixture repair/retry = %q / %q", listRepair, listRetry)
	}
	listBlock := s7AWTokenBlock{
		label: "list corrupt route",
		text:  listRepair + "\n" + listRetry + "\n" + list.HistoryDisclosure,
	}
	blocksByEmitter["internal/cli/feature_intent_archive.go:intentArchiveCorruptRemovalText"] =
		append(
			blocksByEmitter["internal/cli/feature_intent_archive.go:intentArchiveCorruptRemovalText"],
			listBlock,
		)

	doctorJSON, doctorErr := runDoctorCLI(
		t, nonOwned.root, "doctor", "--json", "--check", "D9",
	)
	if doctorErr != nil {
		t.Fatal(doctorErr)
	}
	var doctor workflow.DoctorReport
	if err := json.Unmarshal([]byte(doctorJSON), &doctor); err != nil {
		t.Fatal(err)
	}
	doctorRepair := ""
	for _, finding := range doctor.Findings {
		if finding.Tag == string(store.IntentArchiveRepairCorruptObject) {
			doctorRepair = finding.Remediation
		}
	}
	if doctorRepair == "" {
		t.Fatal("PIB-559 doctor fixture emitted no corrupt route")
	}
	blocksByEmitter["internal/workflow/doctor_d9.go:doctorD9ArchiveClassRemediation"] =
		append(
			blocksByEmitter["internal/workflow/doctor_d9.go:doctorD9ArchiveClassRemediation"],
			s7AWTokenBlock{
				label: "doctor corrupt route",
				text:  doctorRepair,
			},
		)

	code, stdout, _, _ = runPrepare(
		t, "--path", nonOwned.root,
		"feature", "intent-archive", "purge", nonOwned.slug,
		"--blob", nonOwned.hash, "--yes", "--json", "--quiet",
	)
	refusal := decodeIntentArchivePurgeReport(t, stdout)
	if code != 3 || refusal.RemainingRepairs == nil ||
		len(refusal.RemainingRepairs.Stages) == 0 {
		t.Fatalf("PIB-559 remaining-stage fixture = exit:%d report:%#v", code, refusal)
	}
	stageRepairs := []string{}
	for _, stage := range refusal.RemainingRepairs.Stages {
		stageRepairs = append(stageRepairs, stage.Repair)
	}
	blocksByEmitter["internal/store/intent_archive.go:intentArchiveCorruptRepair"] =
		append(
			blocksByEmitter["internal/store/intent_archive.go:intentArchiveCorruptRepair"],
			s7AWTokenBlock{
				label: "remaining repair stage",
				text:  strings.Join(stageRepairs, "\n") + "\n" + s7AVGitHistoryCaveat,
			},
		)

	owned := s7AWWriteOverlapFixture(t, true)
	code, stdout, stderr := s7APRunFromWorkspace(t, owned.root, []string{
		"feature", "intent-archive", "purge", owned.slug,
		"--blob", owned.hash, "--yes", "--json",
	})
	divergent := decodeIntentArchivePurgeReport(t, stdout)
	if code != 6 || divergent.Divergence == nil {
		t.Fatalf("PIB-559 divergence fixture = exit:%d stderr:%q report:%#v",
			code, stderr, divergent)
	}
	blocksByEmitter["internal/cli/feature_intent_archive.go:buildIntentArchiveDivergence"] =
		append(
			blocksByEmitter["internal/cli/feature_intent_archive.go:buildIntentArchiveDivergence"],
			s7AWTokenBlock{
				label: "owned divergence route",
				text: divergent.Divergence.Warning + "\n" +
					divergent.Divergence.RemoveCommand + "\n" +
					divergent.Divergence.Cost + "\n" +
					divergent.Divergence.Retry,
			},
		)

	if len(derived) != 5 {
		t.Fatalf("PIB-559 derived archive removal emitters = %v, want 5", derived)
	}
	for emitter := range derived {
		blocks := blocksByEmitter[emitter]
		if len(blocks) == 0 {
			t.Fatalf("PIB-559 derived emitter %s has no real emitted block", emitter)
		}
		for _, block := range blocks {
			if err := s7AWValidateTokenBlock(block); err != nil {
				t.Fatalf("PIB-559 %s: %v\n%s", emitter, err, block.text)
			}
		}
	}
	for emitter := range blocksByEmitter {
		if !derived[emitter] {
			t.Fatalf("PIB-559 runtime block claims non-derived emitter %s", emitter)
		}
	}
	prd := s7AVRepoDocument(t, s7AVPRDRelPath)
	documentedProcedures := 0
	for _, block := range s7AWMarkdownFences(prd) {
		if !strings.Contains(block, "rm -rf --") {
			continue
		}
		documentedProcedures++
		if err := s7AWValidateTokenBlock(s7AWTokenBlock{
			label: "documented archive procedure",
			text:  block,
		}); err != nil {
			t.Fatalf("PIB-559 documented procedure: %v\n%s", err, block)
		}
	}
	if documentedProcedures != 2 {
		t.Fatalf("PIB-559 documented procedure count = %d, want 2", documentedProcedures)
	}
	if !strings.Contains(prd, "**The residual is disclosed rather than claimed closed.**") ||
		!strings.Contains(prd, "is outside the tokenizer's reach") {
		t.Fatal("PIB-559 §10.7 residual disclosure is missing or contradicted")
	}
	for name, body := range sources {
		if strings.Contains(body, "the prose channel is fully guarded") {
			t.Fatalf("PIB-559 %s claims the disclosed residual is closed", name)
		}
	}

	mustPass := s7AWTokenBlock{
		label: "mandatory Git-history caveat",
		text: "WARNING: destructive archive repair.\nrm -rf -- " + nonOwned.blobRel +
			"\nIt is still in this repository's Git history.",
	}
	if err := s7AWValidateTokenBlock(mustPass); err != nil {
		t.Fatalf("PIB-559 must-pass Git-history caveat failed: %v", err)
	}
	successRoot, successSlug := prepareS4Workspace(t, "S7 AW PIB 559 cp success")
	prepareS4WriteReadyBundle(t, successRoot, successSlug, true)
	successCode, successHuman, successStderr, _ := runPrepare(
		t, "--path", successRoot, "prepare", successSlug,
		"--regenerate", "--allow-heuristic",
	)
	if successCode != 0 {
		t.Fatalf("PIB-559 success-report fixture = exit:%d stderr:%q\n%s",
			successCode, successStderr, successHuman)
	}
	cpCommands := []string{}
	for _, line := range strings.Split(successHuman, "\n") {
		if index := strings.Index(line, "cp "); index >= 0 {
			cpCommands = append(cpCommands, strings.TrimSpace(line[index:]))
		}
	}
	if len(cpCommands) == 0 {
		t.Fatalf("PIB-559 §9.5 success report emitted no cp restore\n%s", successHuman)
	}
	if err := s7AWValidateTokenBlock(s7AWTokenBlock{
		label:   "§9.5 successful archive report",
		text:    successHuman,
		allowCP: true,
	}); err != nil {
		t.Fatalf("PIB-559 scoped cp success form failed: %v", err)
	}

	base := s7AWTokenBlock{
		label: "semantic sensitivity",
		text: "WARNING: destructive archive repair.\nrm -rf -- " + nonOwned.blobRel +
			"\nIt is still in this repository's Git history.",
	}
	fixtures := []struct {
		name    string
		addText string
	}{
		{
			name: "rev-11-preservation-menu",
			addText: "\nPreserve it with cp -R for a directory, cp -P/readlink for a symlink, " +
				"or git show for a version-controlled original.",
		},
		{
			name:    "cp-success-line-moved-here",
			addText: "\ncp saved.preimage " + nonOwned.blobRel,
		},
		{
			name:    "mv-aside",
			addText: "\nmv " + nonOwned.blobRel + " preserved.blob",
		},
		{
			name:    "tar-preservation",
			addText: "\ntar -cf preserved.tar " + nonOwned.blobRel,
		},
		{
			name:    "allowlist-catches-unnamed-find",
			addText: "\nfind . -name '*.blob' -delete",
		},
	}
	for _, fixture := range fixtures {
		candidate := base
		candidate.label = fixture.name
		candidate.text += fixture.addText
		if err := s7AWValidateTokenBlock(candidate); err == nil {
			t.Fatalf("PIB-559 sensitivity fixture %q was accepted", fixture.name)
		}
	}
}

func s7AWValidateDeviceSeamSource(repoRoot string) error {
	directories := []string{"internal/cli"}
	declarations := 0
	typeDeclarations := 0
	deviceAssignments := 0
	calls := 0
	for _, directory := range directories {
		entries, err := os.ReadDir(filepath.Join(repoRoot, filepath.FromSlash(directory)))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			path := filepath.Join(repoRoot, filepath.FromSlash(directory), entry.Name())
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && function.Name.Name == "s7ARInstallDeviceProbe" {
					declarations++
					if !strings.HasSuffix(entry.Name(), "_test.go") {
						return errors.New("device-kind injection seam has a production declaration")
					}
				}
				if ok && function.Name.Name == "ProbeBlob" &&
					function.Recv != nil && len(function.Recv.List) == 1 {
					receiver := function.Recv.List[0].Type
					if pointer, pointerOK := receiver.(*ast.StarExpr); pointerOK {
						receiver = pointer.X
					}
					ident, identOK := receiver.(*ast.Ident)
					if identOK && ident.Name == "s7ARDeviceProbeStorage" {
						ast.Inspect(function.Body, func(node ast.Node) bool {
							assignment, assignOK := node.(*ast.AssignStmt)
							if !assignOK {
								return true
							}
							for _, right := range assignment.Rhs {
								selector, selectorOK := right.(*ast.SelectorExpr)
								if !selectorOK {
									continue
								}
								base, baseOK := selector.X.(*ast.Ident)
								if baseOK && base.Name == "store" &&
									selector.Sel.Name == "IntentArchiveBlobKindDevice" {
									deviceAssignments++
								}
							}
							return true
						})
					}
				}
				generic, ok := declaration.(*ast.GenDecl)
				if !ok || generic.Tok != token.TYPE {
					continue
				}
				for _, specification := range generic.Specs {
					typed, ok := specification.(*ast.TypeSpec)
					if ok && typed.Name.Name == "s7ARDeviceProbeStorage" {
						typeDeclarations++
						if !strings.HasSuffix(entry.Name(), "_test.go") {
							return errors.New("device-kind injection interface has a production declaration")
						}
					}
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || s7AVCallName(call) != "s7ARInstallDeviceProbe" {
					return true
				}
				calls++
				if !strings.HasSuffix(entry.Name(), "_test.go") {
					calls = -1
					return false
				}
				return true
			})
			if calls < 0 {
				return errors.New("device-kind injection seam has a production call site")
			}
		}
	}
	if declarations != 1 || typeDeclarations != 1 ||
		deviceAssignments != 1 || calls == 0 {
		return fmt.Errorf(
			"device-kind seam function/type/assignment/calls = %d/%d/%d/%d, want 1/1/1/nonzero test-only",
			declarations, typeDeclarations, deviceAssignments, calls,
		)
	}
	return nil
}

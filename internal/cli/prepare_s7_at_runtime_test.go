//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// s7ATMixedFixture holds a fixture where one generation retains hash h
// while another generation tombstones hash h, with the blob present and correct.
type s7ATMixedFixture struct {
	root     string
	slug     string
	hash     string
	g1ID     string
	g2ID     string
	blobRel  string
	indexRel string
	data     []byte
}

func s7ATWriteMixedArchiveFixture(t *testing.T, root, slug string, data []byte) s7ATMixedFixture {
	t.Helper()
	retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
	tombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireTombstoned)
	g1 := intentArchiveCLIGeneration(t, slug, retained)
	g2 := intentArchiveCLIGeneration(t, slug, tombstoned)
	index := intentArchiveCLIIndex(t, slug, g1, g2)
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{retained.ContentSHA256: data})
	blobRel, err := store.IntentArchiveBlobRel(slug, retained.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	return s7ATMixedFixture{
		root:     root,
		slug:     slug,
		hash:     retained.ContentSHA256,
		g1ID:     g1.GenerationID,
		g2ID:     g2.GenerationID,
		blobRel:  blobRel,
		indexRel: indexRel,
		data:     append([]byte(nil), data...),
	}
}

func s7ATWireStates(
	index store.IntentArchiveIndex,
	hash string,
) []store.IntentArchiveWireState {
	states := []store.IntentArchiveWireState{}
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == hash {
				states = append(states, replacement.WireState())
			}
		}
	}
	return states
}

func s7ATInstallDeterministicProvider(t *testing.T) {
	t.Helper()
	attempt := &s7AQRetryAttempt{}
	restore := s6InstallProviderFixture(
		t,
		&s7AQRetryProvider{attempt: attempt},
		provider.Config{
			Type:    "openai-compatible",
			BaseURL: "https://s7-at.invalid",
			Model:   "s7-at-deterministic",
		},
	)
	t.Cleanup(restore)
}

// ─── PIB-531 ──────────────────────────────────────────────────────────────────

func TestS7ATMixedResiduePrepareRefusalContracts(t *testing.T) {
	s7ATInstallDeterministicProvider(t)
	root, slug := intentArchiveCLIWorkspace(t)
	fixture := s7ATWriteMixedArchiveFixture(t, root, slug, []byte("PIB-531 mixed retained+tombstoned\n"))
	before := readTree(t, filepath.Join(root, ".tpatch"))

	forbidden := []string{"--orphans", "archive-purge-evidence-divergent", "--abandon-transaction"}
	wantRepair := "tpatch feature intent-archive purge " + slug + " --blob " + fixture.hash + " --yes"

	// prepare and --regenerate must exit 3 archive-index-storage-inconsistent
	for _, tc := range []struct {
		surface string
		mode    prepareMode
		args    []string
	}{
		{
			surface: "prepare",
			mode:    prepareModeGenerate,
			args:    []string{"--path", root, "prepare", slug, "--json", "--quiet"},
		},
		{
			surface: "regenerate",
			mode:    prepareModeRegenerate,
			args:    []string{"--path", root, "prepare", slug, "--regenerate", "--json", "--quiet"},
		},
	} {
		code, stdout, _, _ := runPrepare(t, tc.args...)
		report := prepareS4Report(t, stdout)
		if code != 3 {
			t.Fatalf("%s exit = %d, want 3\n%s", tc.surface, code, stdout)
		}
		if report.Mode != tc.mode || report.Slug != slug || report.Outcome != "refused" {
			t.Fatalf("%s envelope = mode:%s slug:%q outcome:%q", tc.surface, report.Mode, report.Slug, report.Outcome)
		}
		if report.Refusal == nil || report.Refusal.Code != string(store.IntentArchiveCodeIndexStorageInconsistent) {
			t.Fatalf("%s refusal code = %v", tc.surface, report.Refusal)
		}
		if !strings.Contains(report.Refusal.Remediation, wantRepair) {
			t.Fatalf("%s remediation missing repair\ngot: %q\nwant contains: %q", tc.surface, report.Refusal.Remediation, wantRepair)
		}
		for _, needle := range forbidden {
			if strings.Contains(stdout, needle) {
				t.Fatalf("%s output contains forbidden %q\n%s", tc.surface, needle, stdout)
			}
		}
		s7ASAssertNoLeak(t, stdout, root)
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatalf("%s mutated .tpatch", tc.surface)
		}
	}

	// feature intent-archive list exits 3 with mixed-reference storage
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
	)
	wantListStderr := "error: feature intent-archive list " + slug +
		": refused archive-index-storage-inconsistent\n"
	if code != 3 || stderr != wantListStderr {
		t.Fatalf("list = %d stderr=%q\n%s", code, stderr, stdout)
	}
	listReport := decodeIntentArchiveListReport(t, stdout)
	if listReport.Command != intentArchiveCommandList || listReport.Slug != slug {
		t.Fatalf("list report envelope = %#v", listReport)
	}
	if listReport.Refusal == nil || listReport.Refusal.Code != string(store.IntentArchiveCodeIndexStorageInconsistent) {
		t.Fatalf("list refusal = %#v", listReport.Refusal)
	}
	foundMixed := false
	for _, gen := range listReport.Generations {
		for _, entry := range gen.Entries {
			if entry.ContentSHA256 == fixture.hash && entry.Storage == "mixed-reference" {
				foundMixed = true
				if entry.BlobPath != fixture.blobRel {
					t.Fatalf("list entry blob_path = %q, want %q", entry.BlobPath, fixture.blobRel)
				}
				if entry.Repair != wantRepair {
					t.Fatalf("list entry repair = %q, want %q", entry.Repair, wantRepair)
				}
				if entry.RetryCWD != store.IntentArchiveRepairCWD {
					t.Fatalf("list entry retry_cwd = %q, want %q", entry.RetryCWD, store.IntentArchiveRepairCWD)
				}
				if len(entry.TombstoneGenerationIDs) == 0 || len(entry.LiveGenerationIDs) == 0 {
					t.Fatalf("list entry generation ids: live=%v tombstone=%v", entry.LiveGenerationIDs, entry.TombstoneGenerationIDs)
				}
			}
		}
	}
	if !foundMixed {
		t.Fatalf("list did not render mixed-reference entry\n%s", stdout)
	}
	for _, needle := range forbidden {
		if strings.Contains(stdout, needle) {
			t.Fatalf("list output contains forbidden %q\n%s", needle, stdout)
		}
	}
	s7ASAssertNoLeak(t, stdout, root)
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("list mutated .tpatch")
	}

	// doctor D9 reports mixed-reference
	structured, err := runDoctorCLI(t, root, "doctor", "--json", "--check", "D9")
	if err != nil {
		t.Fatalf("doctor = %v\n%s", err, structured)
	}
	var doctorReport workflow.DoctorReport
	if err := json.Unmarshal([]byte(structured), &doctorReport); err != nil {
		t.Fatalf("decode doctor: %v\n%s", err, structured)
	}
	if doctorReport.Summary.ChecksRun != 1 || len(doctorReport.Findings) == 0 {
		t.Fatalf("doctor report = %#v", doctorReport)
	}
	foundDoctorMixed := false
	for _, finding := range doctorReport.Findings {
		if finding.CheckID == "D9" &&
			finding.Tag == string(store.IntentArchiveRepairMixedReference) {
			foundDoctorMixed = true
			if !strings.Contains(finding.Remediation, wantRepair) {
				t.Fatalf("doctor remediation = %q, want contains %q", finding.Remediation, wantRepair)
			}
		}
	}
	if !foundDoctorMixed {
		t.Fatalf("doctor did not detect mixed-reference\n%s", structured)
	}
	for _, needle := range forbidden {
		if strings.Contains(structured, needle) {
			t.Fatalf("doctor output contains forbidden %q\n%s", needle, structured)
		}
	}
	s7ASAssertNoLeak(t, structured, root)
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("doctor mutated .tpatch")
	}
}

// ─── PIB-532 ──────────────────────────────────────────────────────────────────

func TestS7ATMixedResidueBlobRepairContracts(t *testing.T) {
	s7ATInstallDeterministicProvider(t)
	root, slug := prepareS4Workspace(t, "S7 AT PIB 532")
	prepareS4WriteReadyBundle(t, root, slug, true)
	analysisPath := filepath.Join(root, ".tpatch", "features", slug, "analysis.md")
	mixedBytes := []byte("PIB-532 mixed repair bytes\n")
	if err := os.WriteFile(analysisPath, mixedBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := s7ATWriteMixedArchiveFixture(t, root, slug, mixedBytes)
	indexBefore, parsedBefore := readIntentArchiveCLIIndex(t, root, slug)
	beforeStates := s7ATWireStates(parsedBefore, fixture.hash)
	if !reflect.DeepEqual(beforeStates, []store.IntentArchiveWireState{
		store.IntentArchiveWireRetained,
		store.IntentArchiveWireTombstoned,
	}) {
		t.Fatalf("pre-repair wire states = %v", beforeStates)
	}

	// Install spies to observe the CAS order
	decodedBeforeRemoval := false
	removeBeforeDecode := false
	removedPaths := []string{}
	casWrites := []string{} // records: "claim", "remove", "tombstone"
	claimAllPending := false
	claimAfterDecode := false
	tombstoneAll := false

	previousHook := s7APBeforePurgeBlobRemove
	previousDecodeHook := s7ASAfterPurgeIndexDecode
	previousAfterCASHook := s7APAfterPurgeIndexRename
	previousFactory := intentArchiveNewStorage

	s7ASAfterPurgeIndexDecode = func(indexRel string) {
		if indexRel == fixture.indexRel {
			decodedBeforeRemoval = true
		}
	}

	casPhase := 0
	s7APAfterPurgeIndexRename = func(indexRel string) {
		if indexRel != fixture.indexRel {
			return
		}
		casPhase++
		_, parsed := readIntentArchiveCLIIndex(t, root, slug)
		switch casPhase {
		case 1:
			casWrites = append(casWrites, "claim")
			claimAfterDecode = decodedBeforeRemoval
			states := s7ATWireStates(parsed, fixture.hash)
			claimAllPending = len(states) == len(beforeStates)
			for _, state := range states {
				claimAllPending = claimAllPending &&
					state == store.IntentArchiveWireRemovalPending
			}
		case 2:
			casWrites = append(casWrites, "tombstone")
			states := s7ATWireStates(parsed, fixture.hash)
			tombstoneAll = len(states) == len(beforeStates)
			for _, state := range states {
				tombstoneAll = tombstoneAll &&
					state == store.IntentArchiveWireTombstoned
			}
		}
	}

	s7APBeforePurgeBlobRemove = func(blobRel string) {
		casWrites = append(casWrites, "remove")
	}

	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rootFS *os.Root,
	) store.IntentArchiveStorage {
		return &s7ASRemoveSpyStorage{
			IntentArchiveStorage: previousFactory(authority, rootFS),
			removed:              &removedPaths,
			decodeObserved:       &decodedBeforeRemoval,
			removeBeforeDecode:   &removeBeforeDecode,
		}
	}
	t.Cleanup(func() {
		s7APBeforePurgeBlobRemove = previousHook
		s7ASAfterPurgeIndexDecode = previousDecodeHook
		s7APAfterPurgeIndexRename = previousAfterCASHook
		intentArchiveNewStorage = previousFactory
	})

	// Run the exact repair command
	repairArgv := []string{"feature", "intent-archive", "purge", slug, "--blob", fixture.hash, "--yes"}
	code, stdout, stderr := s7APRunFromWorkspace(t, root, repairArgv)
	if code != 0 || stderr != "" {
		t.Fatalf("repair exit=%d stderr=%q\n%s", code, stderr, stdout)
	}

	// Verify strict decode before first mutation
	if !decodedBeforeRemoval {
		t.Fatal("strict index decode was not observed before first mutation")
	}
	if removeBeforeDecode {
		t.Fatal("blob removal ran before the post-decode seam fired")
	}

	// Verify CAS order: claim → remove → tombstone
	wantOrder := []string{"claim", "remove", "tombstone"}
	if !reflect.DeepEqual(casWrites, wantOrder) {
		t.Fatalf("CAS order = %v, want %v", casWrites, wantOrder)
	}

	// Verify the claim CAS made every reference pending (including the tombstoned one)
	if !claimAllPending {
		t.Fatal("claim CAS did not make every reference to h pending")
	}
	if !claimAfterDecode {
		t.Fatal("claim CAS committed before the successful strict decode")
	}

	// Verify the tombstone CAS saw every ref pending before tombstoning
	if !tombstoneAll {
		t.Fatal("tombstone CAS did not tombstone every same-hash reference")
	}

	// Verify removal
	if !reflect.DeepEqual(removedPaths, []string{fixture.blobRel}) {
		t.Fatalf("removed paths = %v, want [%s]", removedPaths, fixture.blobRel)
	}

	// Verify blob is gone
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(fixture.blobRel))); !os.IsNotExist(err) {
		t.Fatalf("blob still present after repair: %v", err)
	}

	// Verify every reference is tombstoned, none retained, none dangling
	_, parsedAfter := readIntentArchiveCLIIndex(t, root, slug)
	for _, gen := range parsedAfter.Generations {
		for _, repl := range gen.Replaced {
			if repl.ContentSHA256 == fixture.hash {
				if repl.WireState() != store.IntentArchiveWireTombstoned {
					t.Fatalf("after repair: ref in %s still %s", gen.GenerationID, repl.WireState())
				}
			}
		}
	}

	// Verify index strict-decodes and X11 is satisfied
	if err := store.ValidateIntentArchiveIndex(parsedAfter, slug); err != nil {
		t.Fatalf("index validation failed after repair: %v", err)
	}
	if validateS7ASX11Classification(store.ClassifyIntentArchiveTuple) != nil {
		t.Fatal("X11 classification failed after repair")
	}

	// Verify index was rewritten (not identical to before — wire states changed)
	indexAfterBytes, _ := readIntentArchiveCLIIndex(t, root, slug)
	if bytes.Equal(indexBefore, indexAfterBytes) {
		t.Fatal("index.json was not rewritten by repair")
	}

	// Verify regenerate proceeds after repair
	code, regenStdout, regenStderr, _ := runPrepare(
		t,
		"--path", root, "prepare", slug,
		"--regenerate", "--json", "--quiet",
	)
	regenReport := prepareS4Report(t, regenStdout)
	if code != 0 || regenStderr != "" || regenReport.Outcome != "published" {
		t.Fatalf("regenerate after repair = %d stderr=%q report=%#v\n%s", code, regenStderr, regenReport, regenStdout)
	}
}

// ─── PIB-534 ──────────────────────────────────────────────────────────────────

func TestS7ATPendingRepairMultiClassContracts(t *testing.T) {
	s7ATInstallDeterministicProvider(t)

	for _, selectorCase := range []struct {
		name         string
		selectorArgs func(fixture s7ATMixedFixture) []string
	}{
		{
			name: "orphans",
			selectorArgs: func(_ s7ATMixedFixture) []string {
				return []string{"--orphans"}
			},
		},
		{
			name: "blob",
			selectorArgs: func(f s7ATMixedFixture) []string {
				return []string{"--blob", f.hash}
			},
		},
	} {
		// Fixture: pending h1 + mixed h2
		root, slug := intentArchiveCLIWorkspace(t)
		pending := s7ASWritePendingArchiveFixture(t, root, slug, 1)
		mixedData := []byte("PIB-534 mixed h2 bytes\n")
		mixedRetained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, mixedData, store.IntentArchiveWireRetained)
		mixedTombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, mixedData, store.IntentArchiveWireTombstoned)
		mixedGen1 := intentArchiveCLIGeneration(t, slug, mixedRetained)
		mixedGen2 := intentArchiveCLIGeneration(t, slug, mixedTombstoned)

		_, existingIndex := readIntentArchiveCLIIndex(t, root, slug)
		existingIndex.Generations = append(existingIndex.Generations, mixedGen1, mixedGen2)
		encoded, err := store.EncodeIntentArchiveIndex(existingIndex)
		if err != nil {
			t.Fatal(err)
		}
		archiveDir := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive")
		if err := os.WriteFile(filepath.Join(archiveDir, "index.json"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(archiveDir, "blobs", mixedRetained.ContentSHA256+".blob"), mixedData, 0o644); err != nil {
			t.Fatal(err)
		}
		mixedHash := mixedRetained.ContentSHA256
		mixedBlobRel, _ := store.IntentArchiveBlobRel(slug, mixedHash)
		selectorArgs := selectorCase.selectorArgs(s7ATMixedFixture{hash: mixedHash})

		// Preview: exit 0 recovery-required, no effects
		acquires := 0
		writes := 0
		previousAcquire := intentArchiveAcquireAuthority
		previousFactory2 := intentArchiveNewStorage
		intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return previousAcquire(path)
		}
		intentArchiveNewStorage = func(
			authority *intentlock.WorkspaceAuthority,
			rootFS *os.Root,
		) store.IntentArchiveStorage {
			return &intentArchiveWriteSpyStorage{
				IntentArchiveStorage: previousFactory2(authority, rootFS),
				writes:               &writes,
			}
		}
		code, stdout, stderr, _ := runPrepare(
			t,
			s7ASPurgeArgs(root, slug, selectorArgs, false, true, true)...,
		)
		intentArchiveAcquireAuthority = previousAcquire
		intentArchiveNewStorage = previousFactory2
		if code != 0 || stderr != "" {
			t.Fatalf("%s preview = %d stderr=%q\n%s", selectorCase.name, code, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgeRecoveryRequired) {
			t.Fatalf("%s preview outcome = %q", selectorCase.name, report.Outcome)
		}
		if report.PendingPurge == nil || len(report.PendingPurge.PendingHashes) != 1 ||
			report.PendingPurge.PendingHashes[0].Hash != pending.hashes[0] {
			t.Fatalf("%s preview pending = %#v", selectorCase.name, report.PendingPurge)
		}
		if acquires != 0 || writes != 0 {
			t.Fatalf("%s preview effects: authority=%d writes=%d", selectorCase.name, acquires, writes)
		}

		// --yes: recovered h1 only, h2 untouched
		mixedBlobBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(mixedBlobRel)))
		if err != nil {
			t.Fatal(err)
		}
		_, parsedBefore2 := readIntentArchiveCLIIndex(t, root, slug)
		mixedStatesBefore := s7ATWireStates(parsedBefore2, mixedHash)

		code, stdout, stderr, _ = runPrepare(
			t,
			s7ASPurgeArgs(root, slug, selectorArgs, true, true, true)...,
		)
		if code != 0 || stderr != "" {
			t.Fatalf("%s confirmed = %d stderr=%q\n%s", selectorCase.name, code, stderr, stdout)
		}
		report = decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgeRecovered) {
			t.Fatalf("%s confirmed outcome = %q", selectorCase.name, report.Outcome)
		}
		if report.Recovery == nil || report.Recovery.Kind != "archive-purge-finalize" ||
			!reflect.DeepEqual(report.Recovery.FinalizedHashes, pending.hashes) {
			t.Fatalf("%s recovery = %#v", selectorCase.name, report.Recovery)
		}

		// h2's blob and index entry are byte-identical
		mixedBlobAfter, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(mixedBlobRel)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(mixedBlobBefore, mixedBlobAfter) {
			t.Fatalf("%s mixed blob changed after recovery", selectorCase.name)
		}
		_, parsedAfterRecovery := readIntentArchiveCLIIndex(t, root, slug)
		if !reflect.DeepEqual(
			s7ATWireStates(parsedAfterRecovery, mixedHash),
			mixedStatesBefore,
		) {
			t.Fatalf("%s mixed index state changed during recovery", selectorCase.name)
		}
		for _, state := range s7ATWireStates(
			parsedAfterRecovery, pending.hashes[0],
		) {
			if state != store.IntentArchiveWireTombstoned {
				t.Fatalf(
					"%s pending hash recovery state = %s",
					selectorCase.name, state,
				)
			}
		}

		// Retry is the operator's own selector
		wantRetry := s7ASRenderedRetry(
			slug, selectorArgs, true, true, true,
		)
		if report.Recovery.Retry != wantRetry ||
			report.Recovery.RetryCWD != store.IntentArchiveRepairCWD {
			t.Fatalf(
				"%s recovery retry = %q cwd=%q, want %q/%q",
				selectorCase.name,
				report.Recovery.Retry,
				report.Recovery.RetryCWD,
				wantRetry,
				store.IntentArchiveRepairCWD,
			)
		}

		// Rerun: repairs h2 or refuses with route
		rerunArgv, err := s7APParseRenderedCommand(report.Recovery.Retry)
		if err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr = s7APRunFromWorkspace(t, root, rerunArgv)

		if selectorCase.name == "blob" {
			// --blob h2 is the exact repair for mixed → should complete
			if code != 0 {
				t.Fatalf("%s rerun exit=%d stderr=%q\n%s", selectorCase.name, code, stderr, stdout)
			}
		} else {
			rerunReport := decodeIntentArchivePurgeReport(t, stdout)
			wantBlobRepair := "tpatch feature intent-archive purge " +
				slug + " --blob " + mixedHash + " --yes"
			if code != 3 || rerunReport.Refusal == nil ||
				rerunReport.Refusal.Code !=
					string(store.IntentArchiveCodeIndexStorageInconsistent) ||
				rerunReport.RemainingRepairs == nil ||
				len(rerunReport.RemainingRepairs.Stages) != 1 ||
				rerunReport.RemainingRepairs.Stages[0].Class !=
					string(store.IntentArchiveRepairMixedReference) ||
				rerunReport.RemainingRepairs.Stages[0].Repair != wantBlobRepair {
				t.Fatalf(
					"%s rerun = exit:%d report:%#v\n%s",
					selectorCase.name, code, rerunReport, stdout,
				)
			}
			repairArgv, err := s7APParseRenderedCommand(
				rerunReport.RemainingRepairs.Stages[0].Repair,
			)
			if err != nil {
				t.Fatal(err)
			}
			code, stdout, stderr = s7APRunFromWorkspace(t, root, repairArgv)
			if code != 0 || stderr != "" {
				t.Fatalf(
					"%s literal repair = exit:%d stderr:%q\n%s",
					selectorCase.name, code, stderr, stdout,
				)
			}
		}
	}

	// Third fixture: second repair class proves sequential untouched/reporting
	root3, slug3 := intentArchiveCLIWorkspace(t)
	pending3 := s7ASWritePendingArchiveFixture(t, root3, slug3, 1)
	// Mixed class: retained+tombstoned same hash
	mixedData3 := []byte("PIB-534 second fixture mixed\n")
	mixedRet3 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, mixedData3, store.IntentArchiveWireRetained)
	mixedTomb3 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, mixedData3, store.IntentArchiveWireTombstoned)
	mixedG1_3 := intentArchiveCLIGeneration(t, slug3, mixedRet3)
	mixedG2_3 := intentArchiveCLIGeneration(t, slug3, mixedTomb3)
	// Dangling class: retained with no blob
	danglingData3 := []byte("PIB-534 dangling reference\n")
	danglingRet3 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, danglingData3, store.IntentArchiveWireRetained)
	danglingGen3 := intentArchiveCLIGeneration(t, slug3, danglingRet3)
	danglingHash3 := danglingRet3.ContentSHA256

	_, existingIdx3 := readIntentArchiveCLIIndex(t, root3, slug3)
	existingIdx3.Generations = append(existingIdx3.Generations, mixedG1_3, mixedG2_3, danglingGen3)
	encoded3, err := store.EncodeIntentArchiveIndex(existingIdx3)
	if err != nil {
		t.Fatal(err)
	}
	archiveDir3 := filepath.Join(root3, ".tpatch", "features", slug3, "artifacts", "intent-archive")
	if err := os.WriteFile(filepath.Join(archiveDir3, "index.json"), encoded3, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archiveDir3, "blobs", mixedRet3.ContentSHA256+".blob"), mixedData3, 0o644); err != nil {
		t.Fatal(err)
	}
	// No blob for dangling — that's the point

	// Recover pending first
	code, stdout, stderr, _ := runPrepare(
		t,
		"--path", root3, "feature", "intent-archive", "purge", slug3,
		"--blob", mixedRet3.ContentSHA256, "--yes", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("third fixture recover = %d stderr=%q\n%s", code, stderr, stdout)
	}
	recoverReport := decodeIntentArchivePurgeReport(t, stdout)
	if recoverReport.Outcome != string(store.IntentArchivePurgeRecovered) ||
		!reflect.DeepEqual(recoverReport.Recovery.FinalizedHashes, pending3.hashes) {
		t.Fatalf("third fixture recovery = %#v", recoverReport)
	}

	// Rerun: repairs mixed class and reports dangling untouched
	rerunArgv3, err := s7APParseRenderedCommand(recoverReport.Recovery.Retry)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = s7APRunFromWorkspace(t, root3, rerunArgv3)
	if code != 0 {
		t.Fatalf("third fixture rerun exit=%d stderr=%q\n%s", code, stderr, stdout)
	}
	rerunReport3 := decodeIntentArchivePurgeReport(t, stdout)
	if rerunReport3.Outcome != string(store.IntentArchivePurgePurged) {
		t.Fatalf("third fixture rerun outcome = %q", rerunReport3.Outcome)
	}
	// Should report remaining repairs for the dangling class
	if rerunReport3.RemainingRepairs == nil || !rerunReport3.RemainingRepairs.RerunRequired {
		t.Fatalf("third fixture rerun should report remaining repairs: %#v", rerunReport3.RemainingRepairs)
	}
	foundDangling := false
	for _, stage := range rerunReport3.RemainingRepairs.Stages {
		if stage.Class == string(store.IntentArchiveRepairDanglingReference) {
			for _, h := range stage.Hashes {
				if h == danglingHash3 {
					foundDangling = true
					break
				}
			}
		}
	}
	if !foundDangling {
		t.Fatalf("third fixture rerun did not report dangling class: %#v", rerunReport3.RemainingRepairs)
	}
}

// ─── PIB-535 ──────────────────────────────────────────────────────────────────

func TestS7ATListStorageContracts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setup     func(t *testing.T, root, slug string) string
		wantExit  int
		wantCode  string
		storage   string
		repair    bool
		forbidden []string
	}{
		{
			name: "clean",
			setup: func(t *testing.T, root, slug string) string {
				t.Helper()
				data := []byte("PIB-535 clean archive\n")
				fixture := s7ASWriteCleanArchiveFixture(t, root, slug, data)
				return fixture.hash
			},
			wantExit: 0,
			storage:  "present",
			repair:   false,
		},
		{
			name: "residue",
			setup: func(t *testing.T, root, slug string) string {
				t.Helper()
				data := []byte("PIB-535 tombstone residue\n")
				fixture := s7ASWriteResidueFixture(t, root, slug, data)
				return fixture.hash
			},
			wantExit:  0,
			storage:   "orphan",
			repair:    true,
			forbidden: []string{"--blob"},
		},
		{
			name: "mixed",
			setup: func(t *testing.T, root, slug string) string {
				t.Helper()
				data := []byte("PIB-535 mixed ref\n")
				fixture := s7ATWriteMixedArchiveFixture(t, root, slug, data)
				return fixture.hash
			},
			wantExit:  3,
			wantCode:  string(store.IntentArchiveCodeIndexStorageInconsistent),
			storage:   "mixed-reference",
			repair:    true,
			forbidden: []string{"--orphans", "--abandon-transaction"},
		},
		{
			name: "dangling",
			setup: func(t *testing.T, root, slug string) string {
				t.Helper()
				data := []byte("PIB-535 dangling reference\n")
				retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
				gen := intentArchiveCLIGeneration(t, slug, retained)
				index := intentArchiveCLIIndex(t, slug, gen)
				// Write index but NO blob → dangling
				writeIntentArchiveCLIFixture(t, root, slug, index, nil)
				return retained.ContentSHA256
			},
			wantExit:  3,
			wantCode:  string(store.IntentArchiveCodeBlobDangling),
			storage:   "dangling",
			repair:    true,
			forbidden: []string{"--orphans", "--abandon-transaction"},
		},
		{
			name: "corrupt",
			setup: func(t *testing.T, root, slug string) string {
				t.Helper()
				data := []byte("PIB-535 correct data\n")
				retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
				gen := intentArchiveCLIGeneration(t, slug, retained)
				index := intentArchiveCLIIndex(t, slug, gen)
				// Write blob with WRONG data → corrupt
				writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{retained.ContentSHA256: []byte("wrong bytes\n")})
				return retained.ContentSHA256
			},
			wantExit:  3,
			wantCode:  string(store.IntentArchiveCodeBlobCorrupt),
			storage:   "corrupt",
			repair:    true,
			forbidden: []string{"--orphans", "--abandon-transaction"},
		},
	} {
		root, slug := intentArchiveCLIWorkspace(t)
		hash := tc.setup(t, root, slug)
		before := readTree(t, filepath.Join(root, ".tpatch"))

		acquires := 0
		writes := 0
		previousAcquire := intentArchiveAcquireAuthority
		previousFactory := intentArchiveNewStorage
		intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return previousAcquire(path)
		}
		intentArchiveNewStorage = func(
			authority *intentlock.WorkspaceAuthority,
			rootFS *os.Root,
		) store.IntentArchiveStorage {
			return &intentArchiveWriteSpyStorage{
				IntentArchiveStorage: previousFactory(authority, rootFS),
				writes:               &writes,
			}
		}

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
		)
		intentArchiveAcquireAuthority = previousAcquire
		intentArchiveNewStorage = previousFactory

		if code != tc.wantExit {
			t.Fatalf("%s list exit=%d, want %d, stderr=%q\n%s", tc.name, code, tc.wantExit, stderr, stdout)
		}
		wantStderr := ""
		if tc.wantCode != "" {
			wantStderr = "error: feature intent-archive list " + slug +
				": refused " + tc.wantCode + "\n"
		}
		if stderr != wantStderr {
			t.Fatalf("%s list stderr=%q, want %q", tc.name, stderr, wantStderr)
		}
		if acquires != 0 || writes != 0 {
			t.Fatalf("%s list effects: authority=%d writes=%d", tc.name, acquires, writes)
		}

		if hash != "" {
			listReport := decodeIntentArchiveListReport(t, stdout)
			if tc.wantCode != "" &&
				(listReport.Refusal == nil ||
					listReport.Refusal.Code != tc.wantCode) {
				t.Fatalf("%s list refusal = %#v", tc.name, listReport.Refusal)
			}
			found := false
			for _, gen := range listReport.Generations {
				for _, entry := range gen.Entries {
					if entry.ContentSHA256 == hash {
						if tc.name == "mixed" && entry.Storage == "present" {
							continue
						}
						found = true
						if entry.Storage != tc.storage {
							t.Fatalf("%s storage=%q, want %q", tc.name, entry.Storage, tc.storage)
						}
						if tc.repair && entry.Repair == "" {
							t.Fatalf("%s repair is empty for %s", tc.name, tc.storage)
						}
						if !tc.repair && entry.Repair != "" {
							t.Fatalf("%s unexpected repair=%q", tc.name, entry.Repair)
						}
						if tc.repair {
							switch tc.storage {
							case "orphan":
								want := "tpatch feature intent-archive purge " +
									slug + " --orphans --yes"
								if entry.Repair != want {
									t.Fatalf("%s repair=%q, want %q", tc.name, entry.Repair, want)
								}
							case "mixed-reference", "dangling":
								want := "tpatch feature intent-archive purge " +
									slug + " --blob " + hash + " --yes"
								if entry.Repair != want {
									t.Fatalf("%s repair=%q, want %q", tc.name, entry.Repair, want)
								}
							case "corrupt":
								blobRel, err := store.IntentArchiveBlobRel(slug, hash)
								if err != nil {
									t.Fatal(err)
								}
								wantRemove := "rm -rf -- '" + blobRel + "'"
								wantRetry := "tpatch feature intent-archive purge " +
									slug + " --blob " + hash + " --yes"
								if !strings.Contains(entry.Repair, wantRemove) ||
									!strings.Contains(
										entry.Repair,
										"Restoring each exact hash-correct managed object instead",
									) ||
									entry.Retry != wantRetry ||
									entry.RetryCWD != store.IntentArchiveRepairCWD {
									t.Fatalf(
										"%s corrupt presentation = repair:%q retry:%q cwd:%q",
										tc.name, entry.Repair, entry.Retry, entry.RetryCWD,
									)
								}
							}
						}
					}
				}
			}
			if !found {
				t.Fatalf("%s hash %s was not rendered", tc.name, hash)
			}
			for _, forbidden := range tc.forbidden {
				if strings.Contains(stdout, forbidden) {
					t.Fatalf("%s output contains forbidden %q\n%s", tc.name, forbidden, stdout)
				}
			}
		}

		s7ASAssertNoLeak(t, stdout, root)
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatalf("%s list mutated .tpatch", tc.name)
		}
	}
}

// ─── PIB-536 ──────────────────────────────────────────────────────────────────

func TestS7ATWorkspaceRootOpenFailureContracts(t *testing.T) {
	s7ATInstallDeterministicProvider(t)
	originalBeforeLock := beforeLockAcquire
	originalAcquire := prepareAcquireAuthority
	t.Cleanup(func() {
		beforeLockAcquire = originalBeforeLock
		prepareAcquireAuthority = originalAcquire
	})
	for _, failureStage := range []string{"open-root", "open-directory"} {
		for _, evidenceKind := range []string{"none", "journal", "preimage", "staging"} {
			root, slug := prepareS4Workspace(
				t, "S7 AT PIB-536 "+failureStage,
			)
			prepareS4WriteReadyBundle(t, root, slug, true)
			lane := filepath.Join(
				root, ".tpatch", "local", "intent-prepare", slug,
			)
			switch evidenceKind {
			case "journal":
				s6WriteJournalFixture(t, root, slug, "journal-corrupt")
			case "preimage", "staging":
				if err := os.MkdirAll(lane, 0o700); err != nil {
					t.Fatal(err)
				}
				name := "index.preimage.json"
				if evidenceKind == "staging" {
					name = "stage-0123456789ab"
				}
				if err := os.WriteFile(
					filepath.Join(lane, name), []byte("evidence\n"), 0o600,
				); err != nil {
					t.Fatal(err)
				}
			}
			before := readTree(t, filepath.Join(root, ".tpatch"))
			stages := []string{}
			beforeLockAcquire = func() {
				prepareAcquireAuthority = func(
					path string,
				) (*intentlock.WorkspaceAuthority, error) {
					return intentlock.AcquireWithStageHook(
						path,
						func(stage string) error {
							stages = append(stages, stage)
							if stage == failureStage {
								return errors.New(
									"injected " + failureStage + " failure",
								)
							}
							return nil
						},
					)
				}
			}
			code, stdout, stderr, _ := runPrepare(
				t, "--path", root, "prepare", slug, "--json", "--quiet",
			)
			beforeLockAcquire = originalBeforeLock
			prepareAcquireAuthority = originalAcquire
			report := prepareS4Report(t, stdout)
			label := failureStage + "/" + evidenceKind
			wantStages := []string{"open-root"}
			if failureStage == "open-directory" {
				wantStages = append(wantStages, "open-directory")
			}
			wantStderr := "error: prepare " + slug +
				": generate refused directory-flock-unavailable\n"
			if code != 3 || stderr != wantStderr ||
				report.Refusal == nil ||
				report.Refusal.Code != "directory-flock-unavailable" ||
				!reflect.DeepEqual(stages, wantStages) {
				t.Fatalf(
					"%s result = exit:%d stderr:%q stages:%v refusal:%#v\n%s",
					label, code, stderr, stages, report.Refusal, stdout,
				)
			}
			if strings.Contains(strings.Join(stages, ","), "fstatfs") ||
				strings.Contains(strings.Join(stages, ","), "flock") {
				t.Fatalf("%s reached filesystem syscalls: %v", label, stages)
			}
			for _, forbidden := range []string{
				"transaction-in-progress",
				"lock-filesystem-unsupported",
				"workspace-not-initialized",
			} {
				if strings.Contains(stdout, forbidden) {
					t.Fatalf("%s output contains %q\n%s", label, forbidden, stdout)
				}
			}
			wantLane := ".tpatch/local/intent-prepare/" + slug + "/"
			hasProcedure := strings.Contains(stdout, wantLane) &&
				strings.Contains(report.Refusal.Remediation, "rm -rf")
			wantProcedure := evidenceKind != "none"
			if hasProcedure != wantProcedure {
				t.Fatalf(
					"%s manual procedure presence = %t, want %t\n%s",
					label, hasProcedure, wantProcedure, stdout,
				)
			}
			s7ASAssertNoLeak(t, stdout, root)
			if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
				t.Fatalf("%s mutated .tpatch", label)
			}
		}
	}
}

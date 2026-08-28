//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func s7AUInstallDeterministicProvider(t *testing.T) {
	t.Helper()
	attempt := &s7AQRetryAttempt{}
	restore := s6InstallProviderFixture(
		t,
		&s7AQRetryProvider{attempt: attempt},
		provider.Config{
			Type:    "openai-compatible",
			BaseURL: "https://s7-au.invalid",
			Model:   "s7-au-deterministic",
		},
	)
	t.Cleanup(restore)
}

// ─── PIB-537 ──────────────────────────────────────────────────────────────────

// s7AUAbandonGateRowExit reads the exit class out of an abandon gate-table
// exit cell such as "`1`, pflag's own text".
func s7AUAbandonGateRowExit(t *testing.T, cell string) int {
	t.Helper()
	open := strings.Index(cell, "`")
	if open < 0 {
		t.Fatalf("PIB-537 gate exit cell %q has no backticked exit class", cell)
	}
	rest := cell[open+1:]
	closing := strings.Index(rest, "`")
	if closing <= 0 {
		t.Fatalf("PIB-537 gate exit cell %q has no closing backtick", cell)
	}
	exit, err := strconv.Atoi(rest[:closing])
	if err != nil {
		t.Fatalf("PIB-537 gate exit cell %q is not numeric: %v", cell, err)
	}
	return exit
}

func TestS7AUAbandonBooleanDomainContracts(t *testing.T) {
	// Gate table validation runs before provider install because the
	// deterministic provider changes HOME, which prevents cobra module
	// resolution in the type graph builder.
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	sources := s7ARProductionSourceSet(t)
	if err := validateS7ARAbandonGateTable(prd, sources); err != nil {
		t.Fatalf("PIB-537 gate table validation failed (PIB-511 authority): %v", err)
	}

	gateRows := s7ARExpectedAbandonGateRows()
	if len(gateRows) == 0 || len(gateRows[0]) != 4 {
		t.Fatalf("PIB-537 abandon gate row 1 = %#v, want four cells", gateRows)
	}
	mutexRow := gateRows[0]
	for _, clause := range []string{"mode mutex", "--check", "whatever boolean value it spells"} {
		if !strings.Contains(mutexRow[1], clause) {
			t.Fatalf("PIB-537 gate row 1 domain %q does not cover %q", mutexRow[1], clause)
		}
	}
	wantMutexExit := s7AUAbandonGateRowExit(t, mutexRow[2])

	s7AUInstallDeterministicProvider(t)

	// Abandon evidence workspace: the boolean-true arm previews a real pending
	// transaction journal, which is the only evidence that reaches §6.6's
	// preview contract instead of `no-pending-transaction`.
	abandonRoot, abandonSlug := prepareS4Workspace(t, "S7 AU PIB 537 abandon")
	prepareS4WriteReadyBundle(t, abandonRoot, abandonSlug, true)
	s6WriteJournalFixture(t, abandonRoot, abandonSlug, "")
	abandonLane := filepath.Join(abandonRoot, ".tpatch", "local", "intent-prepare", abandonSlug)
	journalRel := ".tpatch/local/intent-prepare/" + abandonSlug + "/journal.json"

	for _, tc := range []struct {
		name string
		args []string
	}{
		{
			name: "bare-abandon",
			args: []string{"--path", abandonRoot, "prepare", abandonSlug, "--abandon-transaction", "--json", "--quiet"},
		},
		{
			name: "abandon-equals-true",
			args: []string{"--path", abandonRoot, "prepare", abandonSlug, "--abandon-transaction=true", "--json", "--quiet"},
		},
	} {
		before := readTree(t, filepath.Join(abandonRoot, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(t, tc.args...)
		if code != 0 || stderr != "" {
			t.Fatalf("PIB-537 %s exit = %d stderr=%q\n%s", tc.name, code, stderr, stdout)
		}
		report := prepareS4Report(t, stdout)
		if report.Mode != prepareModeAbandon {
			t.Fatalf("PIB-537 %s mode = %s, want %s", tc.name, report.Mode, prepareModeAbandon)
		}
		if report.Outcome != "abandon-planned" || report.Refusal != nil ||
			report.Abandoned == nil || !reflect.DeepEqual(report.Abandoned.Moved, []string{journalRel}) {
			t.Fatalf("PIB-537 %s preview report = %#v\n%s", tc.name, report, stdout)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(abandonRoot, ".tpatch"))) {
			t.Fatalf("PIB-537 %s preview moved evidence", tc.name)
		}
		entries, err := os.ReadDir(abandonLane)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "abandoned-") {
				t.Fatalf("PIB-537 %s preview created %s", tc.name, entry.Name())
			}
		}
		for _, needle := range []string{
			"no-pending-transaction",
			"archive-purge-evidence-divergent",
		} {
			if strings.Contains(stdout, needle) {
				t.Fatalf("PIB-537 %s output contains forbidden %q", tc.name, needle)
			}
		}
	}

	// `--abandon-transaction=false` selects generate and is outside §6.6's
	// domain, so it runs on its own clean workspace with no abandon evidence.
	generateRoot, generateSlug := prepareS4Workspace(t, "S7 AU PIB 537 generate")
	prepareS4WriteReadyBundle(t, generateRoot, generateSlug, true)
	abandonBranches := 0
	previousBranch := beforeAbandonBranch
	beforeAbandonBranch = func() { abandonBranches++ }
	code, stdout, stderr, _ := runPrepare(t,
		"--path", generateRoot, "prepare", generateSlug,
		"--abandon-transaction=false", "--json", "--quiet",
	)
	beforeAbandonBranch = previousBranch
	if code != 0 || stderr != "" {
		t.Fatalf("PIB-537 abandon-equals-false exit = %d stderr=%q\n%s", code, stderr, stdout)
	}
	falseReport := prepareS4Report(t, stdout)
	if falseReport.Mode != prepareModeGenerate || falseReport.Outcome != "no-op" ||
		falseReport.Abandoned != nil || falseReport.Refusal != nil {
		t.Fatalf("PIB-537 abandon-equals-false report = %#v\n%s", falseReport, stdout)
	}
	if abandonBranches != 0 {
		t.Fatalf("PIB-537 abandon-equals-false opened the abandon lane %d times", abandonBranches)
	}
	for _, needle := range []string{
		"no-pending-transaction",
		"abandon-planned",
		"abandoned",
		"abandon-evidence-unsafe",
	} {
		if strings.Contains(stdout, needle) {
			t.Fatalf("PIB-537 abandon-equals-false output contains abandon code %q", needle)
		}
	}

	// `--check --abandon-transaction=false` stops at gate row 1 with pflag's
	// own mutex text, over the workspace that does hold abandon evidence.
	before := readTree(t, filepath.Join(abandonRoot, ".tpatch"))
	mutexCode, mutexStdout, mutexStderr, _ := runPrepare(t,
		"--path", abandonRoot, "prepare", abandonSlug,
		"--check", "--abandon-transaction=false", "--json", "--quiet",
	)
	if mutexCode != wantMutexExit {
		t.Fatalf("PIB-537 check-abandon-false-mutex exit = %d, want %d (gate row 1)\nstdout=%s\nstderr=%s",
			mutexCode, wantMutexExit, mutexStdout, mutexStderr)
	}
	if !strings.Contains(mutexStderr, "[check manual regenerate abandon-transaction]") {
		t.Fatalf("PIB-537 check-abandon-false-mutex stderr lacks pflag mutex text: %q", mutexStderr)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(abandonRoot, ".tpatch"))) {
		t.Fatal("PIB-537 check-abandon-false-mutex mutated .tpatch")
	}
	if strings.Contains(mutexStdout, "no-pending-transaction") ||
		strings.Contains(mutexStdout, "abandon-planned") {
		t.Fatalf("PIB-537 check-abandon-false-mutex emitted an abandon code\n%s", mutexStdout)
	}
}

// ─── PIB-538 ──────────────────────────────────────────────────────────────────

func TestS7AUManualModeRefusalContracts(t *testing.T) {
	s7AUInstallDeterministicProvider(t)

	type manualFixture struct {
		name     string
		setup    func(t *testing.T, root, slug string)
		wantCode string
	}
	fixtures := []manualFixture{
		{
			name: "pending",
			setup: func(t *testing.T, root, slug string) {
				t.Helper()
				s7ASWritePendingArchiveFixture(t, root, slug, 1)
			},
			wantCode: string(store.IntentArchiveCodeRecoveryPending),
		},
		{
			name: "dangling",
			setup: func(t *testing.T, root, slug string) {
				t.Helper()
				data := []byte("PIB-538 dangling\n")
				retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
				gen := intentArchiveCLIGeneration(t, slug, retained)
				writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, gen), nil)
			},
			wantCode: string(store.IntentArchiveCodeBlobDangling),
		},
		{
			name: "residue",
			setup: func(t *testing.T, root, slug string) {
				t.Helper()
				s7ASWriteResidueFixture(t, root, slug, []byte("PIB-538 residue\n"))
			},
			wantCode: string(store.IntentArchiveCodeIndexStorageInconsistent),
		},
		{
			name: "mixed",
			setup: func(t *testing.T, root, slug string) {
				t.Helper()
				s7ATWriteMixedArchiveFixture(t, root, slug, []byte("PIB-538 mixed\n"))
			},
			wantCode: string(store.IntentArchiveCodeIndexStorageInconsistent),
		},
		{
			name: "corrupt",
			setup: func(t *testing.T, root, slug string) {
				t.Helper()
				data := []byte("PIB-538 correct\n")
				retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
				gen := intentArchiveCLIGeneration(t, slug, retained)
				writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, gen), map[string][]byte{retained.ContentSHA256: []byte("wrong\n")})
			},
			wantCode: string(store.IntentArchiveCodeBlobCorrupt),
		},
	}

	for _, tc := range fixtures {
		root, slug := prepareS4Workspace(t, "S7 AU PIB 538 "+tc.name)
		prepareS4WriteReadyBundle(t, root, slug, true)
		tc.setup(t, root, slug)
		before := readTree(t, filepath.Join(root, ".tpatch"))

		writes := 0
		previousFactory := intentArchiveNewStorage
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
			t,
			"--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		intentArchiveNewStorage = previousFactory

		if code != 3 {
			t.Fatalf("PIB-538 %s manual exit = %d, want 3\nstderr=%q\n%s", tc.name, code, stderr, stdout)
		}
		report := prepareS4Report(t, stdout)
		if report.Mode != prepareModeManual {
			t.Fatalf("PIB-538 %s mode = %s, want manual", tc.name, report.Mode)
		}
		if report.Refusal == nil || report.Refusal.Code != tc.wantCode {
			t.Fatalf("PIB-538 %s refusal = %#v, want code %q", tc.name, report.Refusal, tc.wantCode)
		}

		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatalf("PIB-538 %s --manual mutated .tpatch", tc.name)
		}
		if writes != 0 {
			t.Fatalf("PIB-538 %s archive writes = %d, want 0", tc.name, writes)
		}

		for _, forbidden := range []string{"finalize", "repair"} {
			if strings.Contains(report.Refusal.Remediation, forbidden) {
				t.Fatalf("PIB-538 %s remediation contains %q\n%s", tc.name, forbidden, report.Refusal.Remediation)
			}
		}

		s7ASAssertNoLeak(t, stdout, root)
	}
}

// ─── PIB-539 ──────────────────────────────────────────────────────────────────

func TestS7AURetainedPendingSameHashContracts(t *testing.T) {
	s7AUInstallDeterministicProvider(t)

	root, slug := intentArchiveCLIWorkspace(t)
	data := []byte("PIB-539 retained+pending same hash\n")
	retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
	pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireRemovalPending)
	g1 := intentArchiveCLIGeneration(t, slug, retained)
	g2 := intentArchiveCLIGeneration(t, slug, pending)
	writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, g1, g2), map[string][]byte{retained.ContentSHA256: data})
	hash := retained.ContentSHA256
	blobRel, _ := store.IntentArchiveBlobRel(slug, hash)
	indexRel, _ := store.IntentArchiveIndexRel(slug)

	indexBefore, parsedBefore := readIntentArchiveCLIIndex(t, root, slug)
	beforeStates := s7ATWireStates(parsedBefore, hash)
	if len(beforeStates) != 2 {
		t.Fatalf("pre-repair wire states = %v, want 2 entries", beforeStates)
	}

	casWrites := []string{}
	claimAllPending := false
	removedPaths := []string{}
	tombstoneAll := false

	previousHook := s7APBeforePurgeBlobRemove
	previousAfterCAS := s7APAfterPurgeIndexRename
	previousFactory := intentArchiveNewStorage

	casPhase := 0
	s7APAfterPurgeIndexRename = func(ir string) {
		if ir != indexRel {
			return
		}
		casPhase++
		_, parsed := readIntentArchiveCLIIndex(t, root, slug)
		switch casPhase {
		case 1:
			casWrites = append(casWrites, "claim")
			states := s7ATWireStates(parsed, hash)
			claimAllPending = len(states) == len(beforeStates)
			for _, s := range states {
				claimAllPending = claimAllPending && s == store.IntentArchiveWireRemovalPending
			}
		case 2:
			casWrites = append(casWrites, "tombstone")
			states := s7ATWireStates(parsed, hash)
			tombstoneAll = len(states) == len(beforeStates)
			for _, s := range states {
				tombstoneAll = tombstoneAll && s == store.IntentArchiveWireTombstoned
			}
		}
	}
	s7APBeforePurgeBlobRemove = func(_ string) {
		casWrites = append(casWrites, "remove")
	}
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rootFS *os.Root,
	) store.IntentArchiveStorage {
		return &s7ASRemoveSpyStorage{
			IntentArchiveStorage: previousFactory(authority, rootFS),
			removed:              &removedPaths,
		}
	}
	t.Cleanup(func() {
		s7APBeforePurgeBlobRemove = previousHook
		s7APAfterPurgeIndexRename = previousAfterCAS
		intentArchiveNewStorage = previousFactory
	})

	repairArgv := []string{"feature", "intent-archive", "purge", slug, "--blob", hash, "--yes"}
	code, stdout, stderr := s7APRunFromWorkspace(t, root, repairArgv)
	if code != 0 || stderr != "" {
		t.Fatalf("repair exit=%d stderr=%q\n%s", code, stderr, stdout)
	}

	wantOrder := []string{"claim", "remove", "tombstone"}
	if !reflect.DeepEqual(casWrites, wantOrder) {
		t.Fatalf("CAS order = %v, want %v", casWrites, wantOrder)
	}

	if len(casWrites) > 0 && casWrites[0] == "remove" {
		t.Fatal("blob removal ran before the claim CAS")
	}

	if !claimAllPending {
		t.Fatal("claim CAS did not make every reference to h pending (all-pending)")
	}

	if !tombstoneAll {
		t.Fatal("tombstone CAS did not tombstone every same-hash reference")
	}

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel))); !os.IsNotExist(err) {
		t.Fatal("blob still present after repair")
	}
	if !reflect.DeepEqual(removedPaths, []string{blobRel}) {
		t.Fatalf("removed paths = %v, want [%s]", removedPaths, blobRel)
	}

	_, parsedAfter := readIntentArchiveCLIIndex(t, root, slug)
	for _, gen := range parsedAfter.Generations {
		for _, repl := range gen.Replaced {
			if repl.ContentSHA256 == hash && repl.WireState() != store.IntentArchiveWireTombstoned {
				t.Fatalf("after repair: ref still %s", repl.WireState())
			}
		}
	}

	indexAfter, _ := readIntentArchiveCLIIndex(t, root, slug)
	if bytes.Equal(indexBefore, indexAfter) {
		t.Fatal("index.json was not rewritten by repair")
	}

	prepRoot, prepSlug := prepareS4Workspace(t, "S7 AU PIB 539 prep")
	prepareS4WriteReadyBundle(t, prepRoot, prepSlug, true)
	data2 := []byte("PIB-539 observer fixture\n")
	ret2 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data2, store.IntentArchiveWireRetained)
	pend2 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, data2, store.IntentArchiveWireRemovalPending)
	g1b := intentArchiveCLIGeneration(t, prepSlug, ret2)
	g2b := intentArchiveCLIGeneration(t, prepSlug, pend2)
	writeIntentArchiveCLIFixture(t, prepRoot, prepSlug, intentArchiveCLIIndex(t, prepSlug, g1b, g2b), map[string][]byte{ret2.ContentSHA256: data2})
	hash2 := ret2.ContentSHA256

	pCode, pStdout, _, _ := runPrepare(t, "--path", prepRoot, "prepare", prepSlug, "--json", "--quiet")
	if pCode != 3 {
		t.Fatalf("prepare over pending exit=%d\n%s", pCode, pStdout)
	}
	pReport := prepareS4Report(t, pStdout)
	if pReport.Refusal == nil || pReport.Refusal.Code != string(store.IntentArchiveCodeRecoveryPending) {
		t.Fatalf("prepare refusal = %#v", pReport.Refusal)
	}
	for _, forbidden := range []string{
		string(store.IntentArchiveCodeBlobDangling),
		string(store.IntentArchiveCodeBlobCorrupt),
		string(store.IntentArchiveCodeIndexStorageInconsistent),
	} {
		if strings.Contains(pStdout, forbidden) {
			t.Fatalf("prepare output contains %q, want only pending\n%s", forbidden, pStdout)
		}
	}

	pvCode, pvStdout, pvStderr, _ := runPrepare(t,
		s7ASPurgeArgs(prepRoot, prepSlug, []string{"--blob", hash2}, false, true, true)...,
	)
	if pvCode != 0 || pvStderr != "" {
		t.Fatalf("preview exit=%d stderr=%q\n%s", pvCode, pvStderr, pvStdout)
	}
	pvReport := decodeIntentArchivePurgeReport(t, pvStdout)
	if pvReport.Outcome != string(store.IntentArchivePurgeRecoveryRequired) {
		t.Fatalf("preview outcome = %q", pvReport.Outcome)
	}

	// Ownership produces no repair class, so `list` renders the owned hash as
	// pending-remove and exits 0; the exit-3 storage classes stay unreachable.
	lCode, lStdout, lStderr, _ := runPrepare(t,
		"--path", prepRoot, "feature", "intent-archive", "list", prepSlug, "--json", "--quiet",
	)
	if lCode != 0 || lStderr != "" {
		t.Fatalf("list exit=%d stderr=%q\n%s", lCode, lStderr, lStdout)
	}
	listReport := decodeIntentArchiveListReport(t, lStdout)
	if listReport.Outcome == "refused" || listReport.Refusal != nil {
		t.Fatalf("list refused an owned hash: %#v\n%s", listReport.Refusal, lStdout)
	}
	if len(listReport.Orphans) != 0 || len(listReport.CorruptObjects) != 0 {
		t.Fatalf("list reported non-pending storage classes: orphans=%d corrupt=%d\n%s",
			len(listReport.Orphans), len(listReport.CorruptObjects), lStdout)
	}
	wantPendingRepair := "tpatch feature intent-archive purge " + prepSlug + " --blob " + hash2 + " --yes"
	pendingEntries := 0
	for _, gen := range listReport.Generations {
		for _, entry := range gen.Entries {
			if entry.ContentSHA256 != hash2 {
				t.Fatalf("list reported an unexpected hash %q\n%s", entry.ContentSHA256, lStdout)
			}
			if entry.Storage != "pending-remove" {
				t.Fatalf("list storage = %q, want pending-remove\n%s", entry.Storage, lStdout)
			}
			if entry.Repair != wantPendingRepair || entry.Retry != wantPendingRepair ||
				entry.RetryCWD != store.IntentArchiveRepairCWD {
				t.Fatalf("list pending repair = %q/%q/%q, want %q",
					entry.Repair, entry.Retry, entry.RetryCWD, wantPendingRepair)
			}
			pendingEntries++
		}
	}
	if pendingEntries != 2 {
		t.Fatalf("list pending-remove entries = %d, want 2\n%s", pendingEntries, lStdout)
	}
}

// ─── PIB-541 ──────────────────────────────────────────────────────────────────

func TestS7AUListDoctorMultiObservationContracts(t *testing.T) {
	s7AUInstallDeterministicProvider(t)

	for _, order := range []string{"residue-first", "mixed-first"} {
		root, slug := intentArchiveCLIWorkspace(t)

		mixedData := []byte("PIB-541 mixed ref\n")
		mixedRetained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, mixedData, store.IntentArchiveWireRetained)
		mixedTombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, mixedData, store.IntentArchiveWireTombstoned)
		mixedGen1 := intentArchiveCLIGeneration(t, slug, mixedRetained)
		mixedGen2 := intentArchiveCLIGeneration(t, slug, mixedTombstoned)
		mixedHash := mixedRetained.ContentSHA256

		residueData := []byte("PIB-541 orphan residue\n")
		residueReplacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, residueData, store.IntentArchiveWireTombstoned)
		residueGen := intentArchiveCLIGeneration(t, slug, residueReplacement)
		residueHash := residueReplacement.ContentSHA256

		cleanData := []byte("PIB-541 clean ref\n")
		cleanRetained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, cleanData, store.IntentArchiveWireRetained)
		cleanGen := intentArchiveCLIGeneration(t, slug, cleanRetained)

		var generations []store.IntentArchiveGeneration
		if order == "residue-first" {
			generations = []store.IntentArchiveGeneration{residueGen, mixedGen1, mixedGen2, cleanGen}
		} else {
			generations = []store.IntentArchiveGeneration{mixedGen1, mixedGen2, residueGen, cleanGen}
		}
		index := intentArchiveCLIIndex(t, slug, generations...)
		blobs := map[string][]byte{
			mixedHash:                   mixedData,
			residueHash:                 residueData,
			cleanRetained.ContentSHA256: cleanData,
		}
		writeIntentArchiveCLIFixture(t, root, slug, index, blobs)

		wantOrphanRepair := "tpatch feature intent-archive purge " + slug + " --orphans --yes"
		wantMixedRepair := "tpatch feature intent-archive purge " + slug + " --blob " + mixedHash + " --yes"

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
		before := readTree(t, filepath.Join(root, ".tpatch"))
		lCode, lStdout, lStderr, _ := runPrepare(t,
			"--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
		)
		intentArchiveAcquireAuthority = previousAcquire
		intentArchiveNewStorage = previousFactory

		if lCode != 3 {
			t.Fatalf("PIB-541 %s list exit=%d stderr=%q\n%s", order, lCode, lStderr, lStdout)
		}
		listReport := decodeIntentArchiveListReport(t, lStdout)

		foundClean := false
		foundOrphan := false
		foundMixed := false
		for _, gen := range listReport.Generations {
			for _, entry := range gen.Entries {
				switch {
				case entry.ContentSHA256 == cleanRetained.ContentSHA256 && entry.Storage == "present":
					foundClean = true
				case entry.ContentSHA256 == residueHash && entry.Storage == "orphan":
					foundOrphan = true
					if entry.Repair != wantOrphanRepair {
						t.Fatalf("PIB-541 %s orphan repair = %q", order, entry.Repair)
					}
				case entry.ContentSHA256 == mixedHash && entry.Storage == "mixed-reference":
					foundMixed = true
					if entry.Repair != wantMixedRepair {
						t.Fatalf("PIB-541 %s mixed repair = %q", order, entry.Repair)
					}
				}
			}
		}
		if !foundClean || !foundOrphan || !foundMixed {
			t.Fatalf("PIB-541 %s list missing observations: clean=%t orphan=%t mixed=%t\n%s",
				order, foundClean, foundOrphan, foundMixed, lStdout)
		}

		if acquires != 0 || writes != 0 {
			t.Fatalf("PIB-541 %s list effects: authority=%d writes=%d", order, acquires, writes)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatalf("PIB-541 %s list mutated .tpatch", order)
		}

		structured, err := runDoctorCLI(t, root, "doctor", "--json", "--check", "D9")
		if err != nil {
			t.Fatalf("PIB-541 %s doctor = %v\n%s", order, err, structured)
		}
		var doctorReport workflow.DoctorReport
		if err := json.Unmarshal([]byte(structured), &doctorReport); err != nil {
			t.Fatalf("PIB-541 %s decode doctor: %v\n%s", order, err, structured)
		}
		doctorFoundOrphan := false
		doctorFoundMixed := false
		for _, finding := range doctorReport.Findings {
			if finding.CheckID == "D9" {
				if finding.Tag == string(store.IntentArchiveRepairUnreferencedResidue) {
					doctorFoundOrphan = true
				}
				if finding.Tag == string(store.IntentArchiveRepairMixedReference) {
					doctorFoundMixed = true
				}
			}
		}
		if !doctorFoundOrphan || !doctorFoundMixed {
			t.Fatalf("PIB-541 %s doctor missing observations: orphan=%t mixed=%t\n%s",
				order, doctorFoundOrphan, doctorFoundMixed, structured)
		}

		s7ASAssertNoLeak(t, lStdout, root)
		s7ASAssertNoLeak(t, structured, root)
	}
}

// ─── PIB-542 ──────────────────────────────────────────────────────────────────

// s7AUAllBlastRadius is the exact whole-archive disclosure the product prints
// beside every `--all` command line (PIB-557): the tombstone/removal blast
// radius, the preview-first default and the narrower repeated-`--blob`
// alternative, in one constant so a reworded disclosure fails here.
const s7AUAllBlastRadius = "The --all selector tombstones every reference in every generation and removes every blob in this archive. The unconfirmed preview is the default; repeated --blob selectors are the narrower alternative."

// s7AUCleanDisjointArchive is one PIB-542 clean control: an archive whose h₂ is
// clean, so every disjoint selector must succeed on it.
type s7AUCleanDisjointArchive struct {
	root   string
	slug   string
	h2Hash string
	h3Hash string
	id3    string
}

// s7AUWriteCleanDisjointArchive builds one independent clean control archive
// with canonical, unique artifact and generation data, and proves the control
// is clean before any selector runs against it.
func s7AUWriteCleanDisjointArchive(t *testing.T, label string) s7AUCleanDisjointArchive {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	h2Data := []byte("PIB-542 clean control h2 " + label + "\n")
	h3Data := []byte("PIB-542 clean control h3 " + label + "\n")
	h2 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, h2Data, store.IntentArchiveWireRetained)
	h3 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, h3Data, store.IntentArchiveWireRetained)
	h2Gen := intentArchiveCLIGeneration(t, slug, h2)
	h3Gen := intentArchiveCLIGeneration(t, slug, h3)
	if h2Gen.GenerationID == h3Gen.GenerationID {
		t.Fatalf("PIB-542 clean control %s produced colliding generation IDs", label)
	}
	writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, h2Gen, h3Gen), map[string][]byte{
		h2.ContentSHA256: h2Data,
		h3.ContentSHA256: h3Data,
	})

	code, stdout, stderr, _ := runPrepare(t,
		"--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("PIB-542 clean control %s is not clean: exit=%d stderr=%q\n%s", label, code, stderr, stdout)
	}
	report := decodeIntentArchiveListReport(t, stdout)
	if report.Refusal != nil || len(report.Orphans) != 0 || len(report.CorruptObjects) != 0 {
		t.Fatalf("PIB-542 clean control %s holds a storage observation: %#v\n%s", label, report.Refusal, stdout)
	}
	for _, gen := range report.Generations {
		for _, entry := range gen.Entries {
			if entry.Storage != "present" {
				t.Fatalf("PIB-542 clean control %s entry storage = %q, want present\n%s", label, entry.Storage, stdout)
			}
		}
	}
	return s7AUCleanDisjointArchive{
		root:   root,
		slug:   slug,
		h2Hash: h2.ContentSHA256,
		h3Hash: h3.ContentSHA256,
		id3:    h3Gen.GenerationID,
	}
}

func TestS7AUDisjointSelectorRefusalContracts(t *testing.T) {
	s7AUInstallDeterministicProvider(t)

	root, slug := intentArchiveCLIWorkspace(t)
	h2Data := []byte("PIB-542 mixed h2\n")
	h2Retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, h2Data, store.IntentArchiveWireRetained)
	h2Tombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, h2Data, store.IntentArchiveWireTombstoned)
	h2Gen1 := intentArchiveCLIGeneration(t, slug, h2Retained)
	h2Gen2 := intentArchiveCLIGeneration(t, slug, h2Tombstoned)
	h2Hash := h2Retained.ContentSHA256

	h3Data := []byte("PIB-542 clean h3\n")
	h3Retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, h3Data, store.IntentArchiveWireRetained)
	h3Gen := intentArchiveCLIGeneration(t, slug, h3Retained)
	h3Hash := h3Retained.ContentSHA256
	id3 := h3Gen.GenerationID

	index := intentArchiveCLIIndex(t, slug, h2Gen1, h2Gen2, h3Gen)
	blobs := map[string][]byte{
		h2Hash: h2Data,
		h3Hash: h3Data,
	}
	writeIntentArchiveCLIFixture(t, root, slug, index, blobs)
	wantRepair := "tpatch feature intent-archive purge " + slug + " --blob " + h2Hash + " --yes"
	h2Rel, err := store.IntentArchiveBlobRel(slug, h2Hash)
	if err != nil {
		t.Fatal(err)
	}
	h3Rel, err := store.IntentArchiveBlobRel(slug, h3Hash)
	if err != nil {
		t.Fatal(err)
	}
	h2Path := filepath.Join(root, filepath.FromSlash(h2Rel))
	h3Path := filepath.Join(root, filepath.FromSlash(h3Rel))
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}

	selectors := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{name: "orphans", args: []string{"--orphans"}, wantExit: 3},
		{name: "blob-h3", args: []string{"--blob", h3Hash}, wantExit: 3},
		{name: "generation-id3", args: []string{"--generation", id3}, wantExit: 3},
		{name: "all", args: []string{"--all"}, wantExit: 0},
	}
	for _, sel := range selectors {
		before := readTree(t, filepath.Join(root, ".tpatch"))

		// The decode spy proves the whole-index X11 scan ran before this
		// selector's verdict — before each of the three refusals and before
		// the admission. It is restored on the statement after the run, with
		// no failure path in between.
		decodes := 0
		previousDecode := s7ASAfterPurgeIndexDecode
		s7ASAfterPurgeIndexDecode = func(decoded string) {
			if decoded == indexRel {
				decodes++
			}
		}
		code, stdout, _, _ := runPrepare(t,
			s7ASPurgeArgs(root, slug, sel.args, true, true, true)...,
		)
		s7ASAfterPurgeIndexDecode = previousDecode

		if decodes == 0 {
			t.Fatalf("PIB-542 %s decided without decoding the whole index\n%s", sel.name, stdout)
		}
		if code != sel.wantExit {
			t.Fatalf("PIB-542 %s exit=%d, want %d\n%s", sel.name, code, sel.wantExit, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if sel.wantExit == 0 {
			// `--all --yes` is admitted because the mixed class is the
			// archive's only class and `--all` covers every instance of it
			// (ADR-035 D16). The healthy h₃ is in no class, so it supplies no
			// second one — and it is purged with the rest.
			if report.Outcome != string(store.IntentArchivePurgePurged) {
				t.Fatalf("PIB-542 all sole-class outcome = %q, want purged\n%s", report.Outcome, stdout)
			}
			if report.Action != "none" {
				t.Fatalf("PIB-542 all action = %q, want none\n%s", report.Action, stdout)
			}
			if report.Refusal != nil {
				t.Fatalf("PIB-542 all refused: %#v\n%s", report.Refusal, stdout)
			}
			if report.RemainingRepairs != nil {
				t.Fatalf("PIB-542 all left repairs outstanding: %#v\n%s", report.RemainingRepairs, stdout)
			}
			if report.BlastRadius != s7AUAllBlastRadius {
				t.Fatalf("PIB-542 all blast radius = %q, want %q", report.BlastRadius, s7AUAllBlastRadius)
			}
			for _, removed := range []struct {
				label string
				path  string
			}{{label: "h2", path: h2Path}, {label: "h3", path: h3Path}} {
				if _, err := os.Stat(removed.path); !os.IsNotExist(err) {
					t.Fatalf("PIB-542 all left the %s blob in place: %v", removed.label, err)
				}
			}
			s7ASAssertNoLeak(t, stdout, root)
			continue
		}
		if report.Refusal == nil || report.Refusal.Code != string(store.IntentArchiveCodeIndexStorageInconsistent) {
			t.Fatalf("PIB-542 %s refusal = %#v", sel.name, report.Refusal)
		}
		if !strings.Contains(stdout, wantRepair) {
			t.Fatalf("PIB-542 %s missing h2 repair route\n%s", sel.name, stdout)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatalf("PIB-542 %s mutated .tpatch", sel.name)
		}
		for _, kept := range []struct {
			label string
			path  string
		}{{label: "h2", path: h2Path}, {label: "h3", path: h3Path}} {
			if _, err := os.Stat(kept.path); err != nil {
				t.Fatalf("PIB-542 %s removed the %s blob: %v", sel.name, kept.label, err)
			}
		}
		s7ASAssertNoLeak(t, stdout, root)
	}

	// Each clean control is an independent workspace: the four selectors are
	// mutating, so a shared fixture would let one selector's success corrupt
	// the next control's archive.
	for _, sel := range []string{"orphans", "blob-h3", "generation-id3", "all"} {
		control := s7AUWriteCleanDisjointArchive(t, sel)
		var args []string
		switch sel {
		case "orphans":
			args = []string{"--orphans"}
		case "blob-h3":
			args = []string{"--blob", control.h3Hash}
		case "generation-id3":
			args = []string{"--generation", control.id3}
		case "all":
			args = []string{"--all"}
		}
		code, stdout, stderr, _ := runPrepare(t,
			s7ASPurgeArgs(control.root, control.slug, args, true, true, true)...,
		)
		if code != 0 || stderr != "" {
			t.Fatalf("PIB-542 clean %s exit=%d stderr=%q\n%s", sel, code, stderr, stdout)
		}
		controlReport := decodeIntentArchivePurgeReport(t, stdout)
		if controlReport.Refusal != nil {
			t.Fatalf("PIB-542 clean %s refused: %#v\n%s", sel, controlReport.Refusal, stdout)
		}
		if sel == "blob-h3" || sel == "generation-id3" {
			h2Rel, _ := store.IntentArchiveBlobRel(control.slug, control.h2Hash)
			if _, err := os.Stat(filepath.Join(control.root, filepath.FromSlash(h2Rel))); err != nil {
				t.Fatalf("PIB-542 clean %s removed the disjoint h2 blob: %v", sel, err)
			}
		}
	}

	mixedRoot2, mixedSlug2 := intentArchiveCLIWorkspace(t)
	h2PrimeData := []byte("PIB-542 mixed h2-prime\n")
	h2PrimeRet := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, h2PrimeData, store.IntentArchiveWireRetained)
	h2PrimeTomb := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, h2PrimeData, store.IntentArchiveWireTombstoned)
	h2PrimeGen1 := intentArchiveCLIGeneration(t, mixedSlug2, h2PrimeRet)
	h2PrimeGen2 := intentArchiveCLIGeneration(t, mixedSlug2, h2PrimeTomb)
	h2PrimeHash := h2PrimeRet.ContentSHA256

	h2bData := []byte("PIB-542 mixed h2b\n")
	h2bRet := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, h2bData, store.IntentArchiveWireRetained)
	h2bTomb := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, h2bData, store.IntentArchiveWireTombstoned)
	h2bGen1 := intentArchiveCLIGeneration(t, mixedSlug2, h2bRet)
	h2bGen2 := intentArchiveCLIGeneration(t, mixedSlug2, h2bTomb)
	h2bHash := h2bRet.ContentSHA256

	mixedIndex2 := intentArchiveCLIIndex(t, mixedSlug2, h2PrimeGen1, h2PrimeGen2, h2bGen1, h2bGen2)
	writeIntentArchiveCLIFixture(t, mixedRoot2, mixedSlug2, mixedIndex2, map[string][]byte{
		h2PrimeHash: h2PrimeData,
		h2bHash:     h2bData,
	})

	code, stdout, _, _ := runPrepare(t,
		s7ASPurgeArgs(mixedRoot2, mixedSlug2, []string{"--blob", h2PrimeHash}, true, true, true)...,
	)
	if code != 3 {
		t.Fatalf("PIB-542 partial blob exit=%d, want 3\n%s", code, stdout)
	}

	code, stdout, stderr, _ := runPrepare(t,
		s7ASPurgeArgs(mixedRoot2, mixedSlug2, []string{"--blob", h2PrimeHash, "--blob", h2bHash}, true, true, true)...,
	)
	if code != 0 {
		t.Fatalf("PIB-542 full blob exit=%d stderr=%q\n%s", code, stderr, stdout)
	}

	boundaryRoot, boundarySlug := intentArchiveCLIWorkspace(t)
	bResidueData := []byte("PIB-542 boundary residue\n")
	bResidueRepl := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, bResidueData, store.IntentArchiveWireTombstoned)
	bResidueGen := intentArchiveCLIGeneration(t, boundarySlug, bResidueRepl)

	bMixedData := []byte("PIB-542 boundary mixed\n")
	bMixedRet := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, bMixedData, store.IntentArchiveWireRetained)
	bMixedTomb := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, bMixedData, store.IntentArchiveWireTombstoned)
	bMixedGen1 := intentArchiveCLIGeneration(t, boundarySlug, bMixedRet)
	bMixedGen2 := intentArchiveCLIGeneration(t, boundarySlug, bMixedTomb)
	bMixedHash := bMixedRet.ContentSHA256

	bH3Data := []byte("PIB-542 boundary h3\n")
	bH3Ret := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, bH3Data, store.IntentArchiveWireRetained)
	bH3Gen := intentArchiveCLIGeneration(t, boundarySlug, bH3Ret)
	bId3 := bH3Gen.GenerationID

	bIndex := intentArchiveCLIIndex(t, boundarySlug, bResidueGen, bMixedGen1, bMixedGen2, bH3Gen)
	writeIntentArchiveCLIFixture(t, boundaryRoot, boundarySlug, bIndex, map[string][]byte{
		bResidueRepl.ContentSHA256: bResidueData,
		bMixedHash:                 bMixedData,
		bH3Ret.ContentSHA256:       bH3Data,
	})

	for _, sel := range []struct {
		name string
		args []string
	}{
		{name: "blob-h3", args: []string{"--blob", bH3Ret.ContentSHA256}},
		{name: "generation-id3", args: []string{"--generation", bId3}},
		{name: "all", args: []string{"--all"}},
	} {
		code, stdout, _, _ := runPrepare(t,
			s7ASPurgeArgs(boundaryRoot, boundarySlug, sel.args, true, true, true)...,
		)
		if code != 3 {
			t.Fatalf("PIB-542 boundary %s exit=%d, want 3\n%s", sel.name, code, stdout)
		}
	}

	oCode, oStdout, oStderr, _ := runPrepare(t,
		s7ASPurgeArgs(boundaryRoot, boundarySlug, []string{"--orphans"}, true, true, true)...,
	)
	if oCode != 0 {
		t.Fatalf("PIB-542 boundary orphans exit=%d stderr=%q\n%s", oCode, oStderr, oStdout)
	}
	oReport := decodeIntentArchivePurgeReport(t, oStdout)
	if oReport.RemainingRepairs == nil || !oReport.RemainingRepairs.RerunRequired {
		t.Fatalf("PIB-542 boundary orphans missing remaining repairs\n%s", oStdout)
	}
}

// ─── PIB-543 ──────────────────────────────────────────────────────────────────

func TestS7AUCorruptBlobRouteContracts(t *testing.T) {
	s7AUInstallDeterministicProvider(t)

	forbiddenWords := []string{
		"cp", "git", "readlink", "mv", "rsync", "tar", "ln", "install", "dd", "chmod",
	}

	type corruptKind struct {
		name  string
		setup func(t *testing.T, blobPath string, correctData []byte)
		real  bool
	}
	kinds := []corruptKind{
		{
			name: "symlink",
			setup: func(t *testing.T, blobPath string, _ []byte) {
				t.Helper()
				os.Remove(blobPath)
				if err := os.Symlink("/dev/null", blobPath); err != nil {
					t.Fatal(err)
				}
			},
			real: true,
		},
		{
			name: "directory",
			setup: func(t *testing.T, blobPath string, _ []byte) {
				t.Helper()
				os.Remove(blobPath)
				if err := os.Mkdir(blobPath, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			real: true,
		},
		{
			name: "hash-wrong",
			setup: func(t *testing.T, blobPath string, _ []byte) {
				t.Helper()
				if err := os.WriteFile(blobPath, []byte("PIB-543 wrong hash data\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			real: true,
		},
		{
			name: "fifo",
			setup: func(t *testing.T, blobPath string, _ []byte) {
				t.Helper()
				os.Remove(blobPath)
				if err := syscall.Mkfifo(blobPath, 0o600); err != nil {
					t.Fatalf("FIFO creation failed: %v", err)
				}
			},
			real: true,
		},
		{
			name: "device-node",
			setup: func(t *testing.T, blobPath string, _ []byte) {
				t.Helper()
				os.Remove(blobPath)
				if err := os.WriteFile(blobPath, []byte("device seam placeholder\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			real: false,
		},
	}

	observers := []string{"prepare", "regenerate", "manual", "list", "doctor"}

	for _, kind := range kinds {
		correctData := []byte("PIB-543 correct data for " + kind.name + "\n")
		root, slug := prepareS4Workspace(t, "S7 AU PIB 543 "+kind.name)
		prepareS4WriteReadyBundle(t, root, slug, true)

		retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, correctData, store.IntentArchiveWireRetained)
		gen := intentArchiveCLIGeneration(t, slug, retained)
		writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, gen), map[string][]byte{retained.ContentSHA256: correctData})
		hash := retained.ContentSHA256
		blobRel, _ := store.IntentArchiveBlobRel(slug, hash)
		blobPath := filepath.Join(root, filepath.FromSlash(blobRel))

		kind.setup(t, blobPath, correctData)

		// The one kind the CI user cannot build on the real filesystem runs
		// through PIB-560's injected file-kind seam instead.
		var deviceRestore func()
		if !kind.real {
			deviceRestore = s7ARInstallDeviceProbe(t, blobRel)
		}

		before := snapshotTreeMetadata(
			t, "PIB-543 "+kind.name, filepath.Join(root, ".tpatch"),
		)

		wantRmRf := "rm -rf -- '" + blobRel + "'"
		wantRetry := "tpatch feature intent-archive purge " + slug + " --blob " + hash + " --yes"
		// `list` and `doctor` render the archive's own POSIX-quoted procedure;
		// the three mutating readers render the same removal + retry route as
		// refusal prose over the unquoted repo-relative managed path. Both
		// forms are product truth and both are asserted literally.
		wantRmRfPlain := "rm -rf -- " + blobRel
		wantMutatingRoute := "Warning: removing the managed object is destructive and has no undo. " +
			"Run " + wantRmRfPlain + ", then run " + wantRetry +
			", or restore the exact hash-correct blob and retry. " +
			"Committed bytes can remain in this repository's Git history."

		for _, obs := range observers {
			var obsCode int
			var obsStdout, obsStderr string
			switch obs {
			case "prepare":
				obsCode, obsStdout, obsStderr, _ = runPrepare(t,
					"--path", root, "prepare", slug, "--json", "--quiet",
				)
			case "regenerate":
				obsCode, obsStdout, obsStderr, _ = runPrepare(t,
					"--path", root, "prepare", slug, "--regenerate", "--json", "--quiet",
				)
			case "manual":
				obsCode, obsStdout, obsStderr, _ = runPrepare(t,
					"--path", root, "prepare", slug, "--manual", "--json", "--quiet",
				)
			case "list":
				obsCode, obsStdout, obsStderr, _ = runPrepare(t,
					"--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
				)
			case "doctor":
				// The doctor observer runs through the real command so the
				// exit code is the product's, not a fabricated one.
				obsCode, obsStdout, obsStderr, _ = runPrepare(t,
					"--path", root, "doctor", "--json", "--check", "D9",
				)
			}

			if obs != "doctor" && obsCode != 3 {
				t.Fatalf("PIB-543 %s/%s exit=%d, want 3\nstderr=%q\n%s", kind.name, obs, obsCode, obsStderr, obsStdout)
			}

			if obs == "doctor" {
				// D9 is warning-only: warnings raise no findings or errors, so
				// DoctorExitCode stays 0 while the corrupt route is rendered.
				if obsCode != 0 || obsStderr != "" {
					t.Fatalf("PIB-543 %s/doctor exit=%d stderr=%q\n%s", kind.name, obsCode, obsStderr, obsStdout)
				}
				var doctorReport workflow.DoctorReport
				if err := json.Unmarshal([]byte(obsStdout), &doctorReport); err != nil {
					t.Fatalf("PIB-543 %s/doctor decode: %v\n%s", kind.name, err, obsStdout)
				}
				corruptFindings := 0
				for _, finding := range doctorReport.Findings {
					if finding.CheckID != "D9" ||
						finding.Tag != string(store.IntentArchiveRepairCorruptObject) {
						continue
					}
					corruptFindings++
					if finding.Severity != "warning" {
						t.Fatalf("PIB-543 %s/doctor severity = %q, want warning", kind.name, finding.Severity)
					}
					if !strings.Contains(finding.Remediation, wantRmRf) {
						t.Fatalf("PIB-543 %s/doctor remediation lacks %q\n%s", kind.name, wantRmRf, finding.Remediation)
					}
					if !strings.Contains(finding.Remediation, wantRetry) {
						t.Fatalf("PIB-543 %s/doctor remediation lacks the retry %q\n%s", kind.name, wantRetry, finding.Remediation)
					}
					s7AUAssertNoForbiddenCommandWord(t,
						"PIB-543 "+kind.name+"/doctor", finding.Remediation, blobRel, forbiddenWords)
				}
				if corruptFindings != 1 {
					t.Fatalf("PIB-543 %s/doctor corrupt findings = %d, want 1\n%s", kind.name, corruptFindings, obsStdout)
				}
			}

			if obs == "list" {
				listReport := decodeIntentArchiveListReport(t, obsStdout)
				foundCorrupt := false
				for _, gen := range listReport.Generations {
					for _, entry := range gen.Entries {
						if entry.ContentSHA256 == hash && entry.Storage == "corrupt" {
							foundCorrupt = true
							if !strings.Contains(entry.Repair, wantRmRf) {
								t.Fatalf("PIB-543 %s/list corrupt repair missing rm -rf\n%s", kind.name, entry.Repair)
							}
							if entry.Retry != wantRetry {
								t.Fatalf("PIB-543 %s/list retry = %q", kind.name, entry.Retry)
							}
							if entry.RetryCWD != store.IntentArchiveRepairCWD {
								t.Fatalf("PIB-543 %s/list retry_cwd = %q", kind.name, entry.RetryCWD)
							}
						}
					}
				}
				if !foundCorrupt {
					t.Fatalf("PIB-543 %s/list missing corrupt entry\n%s", kind.name, obsStdout)
				}
			}

			if obs == "prepare" || obs == "regenerate" || obs == "manual" {
				report := prepareS4Report(t, obsStdout)
				if report.Refusal == nil ||
					report.Refusal.Code != string(store.IntentArchiveCodeBlobCorrupt) {
					t.Fatalf("PIB-543 %s/%s refusal = %#v\n%s", kind.name, obs, report.Refusal, obsStdout)
				}
				if report.Refusal.Remediation != wantMutatingRoute {
					t.Fatalf(
						"PIB-543 %s/%s remediation =\n%q\nwant\n%q",
						kind.name, obs, report.Refusal.Remediation, wantMutatingRoute,
					)
				}
				// The route is carried entirely by the remediation prose on
				// these three surfaces: no structured retry pair is emitted,
				// so an added one must be `workspace-root` and reviewed, not
				// silently introduced.
				if report.Refusal.Retry != "" || report.Refusal.RetryCWD != "" {
					t.Fatalf(
						"PIB-543 %s/%s carried a structured retry %q / retry_cwd %q",
						kind.name, obs, report.Refusal.Retry, report.Refusal.RetryCWD,
					)
				}
				s7AUAssertNoForbiddenCommandWord(t,
					"PIB-543 "+kind.name+"/"+obs, report.Refusal.Remediation, blobRel, forbiddenWords)
			}

			// The corrupt-object route never offers `--orphans` on any
			// observer: the hash is referenced, so residue is not its class.
			if strings.Contains(obsStdout, "--orphans") {
				t.Fatalf("PIB-543 %s/%s offered --orphans\n%s", kind.name, obs, obsStdout)
			}

			after := snapshotTreeMetadata(
				t, "PIB-543 "+kind.name, filepath.Join(root, ".tpatch"),
			)
			if before != after {
				t.Fatalf("PIB-543 %s/%s mutated .tpatch", kind.name, obs)
			}

			if obs != "doctor" {
				s7ASAssertNoLeak(t, obsStdout, root)
			}
		}

		if deviceRestore != nil {
			deviceRestore()
		}

		// Owned variant: a removal-pending reference to the same hash makes the
		// purge transaction the global owner of h.
		_, ownedIndex := readIntentArchiveCLIIndex(t, root, slug)
		pendingRepl := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, correctData, store.IntentArchiveWireRemovalPending)
		pendingGen := intentArchiveCLIGeneration(t, slug, pendingRepl)
		ownedIndex.Generations = append(ownedIndex.Generations, pendingGen)
		encoded, err := store.EncodeIntentArchiveIndex(ownedIndex)
		if err != nil {
			t.Fatal(err)
		}
		archiveDir := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive")
		if err := os.WriteFile(filepath.Join(archiveDir, "index.json"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}

		if !kind.real {
			deviceRestore = s7ARInstallDeviceProbe(t, blobRel)
		}

		// Ownership outranks the corrupt classification for every reader:
		// a mutating prepare refuses exit 3 recovery-pending and routes to the
		// owning transaction rather than to the corrupt-object procedure.
		oCode, oStdout, _, _ := runPrepare(t,
			"--path", root, "prepare", slug, "--json", "--quiet",
		)
		if oCode != 3 {
			t.Fatalf("PIB-543 %s owned prepare exit=%d, want 3\n%s", kind.name, oCode, oStdout)
		}
		oReport := prepareS4Report(t, oStdout)
		if oReport.Refusal == nil || oReport.Refusal.Code != string(store.IntentArchiveCodeRecoveryPending) {
			t.Fatalf("PIB-543 %s owned prepare refusal = %#v", kind.name, oReport.Refusal)
		}
		if strings.Contains(oStdout, string(store.IntentArchiveCodeBlobCorrupt)) {
			t.Fatalf("PIB-543 %s owned prepare emitted the exit-3 corrupt code\n%s", kind.name, oStdout)
		}

		// The confirmed purge is the one owner, and an owned unidentifiable
		// object is exactly exit 6 archive-purge-evidence-divergent.
		purgeDivCode, purgeDivStdout, purgeDivStderr := s7APRunFromWorkspace(t, root,
			[]string{"feature", "intent-archive", "purge", slug, "--blob", hash, "--yes", "--json", "--quiet"},
		)
		if purgeDivCode != 6 {
			t.Fatalf("PIB-543 %s owned purge exit=%d, want 6\nstderr=%q\n%s", kind.name, purgeDivCode, purgeDivStderr, purgeDivStdout)
		}
		divergentReport := decodeIntentArchivePurgeReport(t, purgeDivStdout)
		if divergentReport.Outcome != "refused" ||
			divergentReport.Refusal == nil ||
			divergentReport.Refusal.Code != string(store.IntentArchiveCodePurgeEvidenceDivergent) ||
			divergentReport.Divergence == nil ||
			divergentReport.Divergence.PendingHash != hash ||
			divergentReport.Divergence.Blob != blobRel {
			t.Fatalf("PIB-543 %s owned purge report = %#v\n%s", kind.name, divergentReport, purgeDivStdout)
		}
		if strings.Contains(purgeDivStdout, string(store.IntentArchiveCodeBlobCorrupt)) ||
			strings.Contains(purgeDivStdout, string(store.IntentArchiveCodeIndexStorageInconsistent)) {
			t.Fatalf("PIB-543 %s owned purge emitted an exit-3 code\n%s", kind.name, purgeDivStdout)
		}

		if deviceRestore != nil {
			deviceRestore()
		}

		// Repair route 1: the printed removal, then the confirmed purge.
		ownedIndex.Generations = ownedIndex.Generations[:len(ownedIndex.Generations)-1]
		encoded, err = store.EncodeIntentArchiveIndex(ownedIndex)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(archiveDir, "index.json"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}

		if err := os.RemoveAll(blobPath); err != nil {
			t.Fatal(err)
		}

		routeOneRemovals := []string{}
		previousStorage := intentArchiveNewStorage
		intentArchiveNewStorage = func(
			authority *intentlock.WorkspaceAuthority,
			rootFS *os.Root,
		) store.IntentArchiveStorage {
			return &s7ASRemoveSpyStorage{
				IntentArchiveStorage: previousStorage(authority, rootFS),
				removed:              &routeOneRemovals,
			}
		}
		pCode, pStdout, pStderr := s7APRunFromWorkspace(t, root,
			[]string{"feature", "intent-archive", "purge", slug, "--blob", hash, "--yes", "--json", "--quiet"},
		)
		intentArchiveNewStorage = previousStorage
		if pCode != 0 || pStderr != "" || len(routeOneRemovals) != 0 {
			t.Fatalf("PIB-543 %s post-remove purge exit=%d stderr=%q\n%s", kind.name, pCode, pStderr, pStdout)
		}
		_, routeOneIndex := readIntentArchiveCLIIndex(t, root, slug)
		for _, state := range s7ATWireStates(routeOneIndex, hash) {
			if state != store.IntentArchiveWireTombstoned {
				t.Fatalf("PIB-543 %s post-remove state=%s", kind.name, state)
			}
		}
		nextCode, nextStdout, nextStderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		if nextCode != 0 || nextStderr != "" {
			t.Fatalf(
				"PIB-543 %s post-repair prepare exit=%d stderr=%q\n%s",
				kind.name, nextCode, nextStderr, nextStdout,
			)
		}

		// Repair route 2: restoring the exact correct blob clears it instead,
		// on its own workspace whose managed blob path belongs to that slug.
		root2, slug2 := prepareS4Workspace(t, "S7 AU PIB 543 "+kind.name+" alt")
		prepareS4WriteReadyBundle(t, root2, slug2, true)
		retained2 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, correctData, store.IntentArchiveWireRetained)
		gen2 := intentArchiveCLIGeneration(t, slug2, retained2)
		writeIntentArchiveCLIFixture(t, root2, slug2, intentArchiveCLIIndex(t, slug2, gen2), map[string][]byte{retained2.ContentSHA256: correctData})
		blobRel2, err := store.IntentArchiveBlobRel(slug2, retained2.ContentSHA256)
		if err != nil {
			t.Fatal(err)
		}
		blobPath2 := filepath.Join(root2, filepath.FromSlash(blobRel2))
		if err := os.MkdirAll(filepath.Dir(blobPath2), 0o755); err != nil {
			t.Fatal(err)
		}

		kind.setup(t, blobPath2, correctData)
		if err := os.RemoveAll(blobPath2); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(blobPath2, correctData, 0o644); err != nil {
			t.Fatal(err)
		}

		indexBeforeRestore, _ := readIntentArchiveCLIIndex(t, root2, slug2)
		cCode, cStdout, cStderr, _ := runPrepare(t,
			"--path", root2, "feature", "intent-archive", "list", slug2,
			"--json", "--quiet",
		)
		cReport := decodeIntentArchiveListReport(t, cStdout)
		indexAfterRestore, _ := readIntentArchiveCLIIndex(t, root2, slug2)
		restoredBytes, restoredErr := os.ReadFile(blobPath2)
		if cCode != 0 || cStderr != "" || cReport.Refusal != nil ||
			restoredErr != nil || !bytes.Equal(restoredBytes, correctData) ||
			!bytes.Equal(indexBeforeRestore, indexAfterRestore) {
			t.Fatalf(
				"PIB-543 %s restore route exit=%d stderr=%q refusal=%#v blobErr=%v\n%s",
				kind.name, cCode, cStderr, cReport.Refusal, restoredErr, cStdout,
			)
		}
	}
}

// s7AUAssertNoForbiddenCommandWord rejects any forbidden command word in a
// printed procedure. The managed blob path is elided first because a hash or
// slug segment is data, not a command, and the check is case-sensitive so the
// required "Git history" caveat prose is not confused with the `git` command.
func s7AUAssertNoForbiddenCommandWord(t *testing.T, label, text, blobRel string, forbidden []string) {
	t.Helper()
	scanned := strings.ReplaceAll(text, blobRel, "")
	for _, word := range forbidden {
		pattern := regexp.MustCompile(`(^|[^0-9A-Za-z_])` + regexp.QuoteMeta(word) + `([^0-9A-Za-z_]|$)`)
		if pattern.MatchString(scanned) {
			t.Fatalf("%s procedure contains the forbidden command word %q:\n%s", label, word, text)
		}
	}
}

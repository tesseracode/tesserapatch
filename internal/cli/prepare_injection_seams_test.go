package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestNamedPrepareCLIInjectionSeamsReachBoundaries(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("workspace mutation authority is unsupported on this target")
	}
	oldBeforeAbandonBranch := beforeAbandonBranch
	oldBeforeAbandonMove := beforeAbandonMove
	oldAfterAbandonMove := afterAbandonMove
	oldBeforeLockAcquire := beforeLockAcquire
	oldBeforeRedactionScan := beforeRedactionScan
	oldAfterRecoveryComplete := afterRecoveryComplete
	oldBeforeManualStatusCAS := beforeManualStatusCAS
	oldBeforeIndexRewrite := beforeIndexRewrite
	oldBeforeRehydrateIndexRename := beforeRehydrateIndexRename
	t.Cleanup(func() {
		beforeAbandonBranch = oldBeforeAbandonBranch
		beforeAbandonMove = oldBeforeAbandonMove
		afterAbandonMove = oldAfterAbandonMove
		beforeLockAcquire = oldBeforeLockAcquire
		beforeRedactionScan = oldBeforeRedactionScan
		afterRecoveryComplete = oldAfterRecoveryComplete
		beforeManualStatusCAS = oldBeforeManualStatusCAS
		beforeIndexRewrite = oldBeforeIndexRewrite
		beforeRehydrateIndexRename = oldBeforeRehydrateIndexRename
	})

	abandonRoot, abandonSlug := prepareS4Workspace(t, "named abandon seams")
	lane := filepath.Join(abandonRoot, ".tpatch", "local", "intent-prepare", abandonSlug)
	if err := os.MkdirAll(filepath.Join(lane, "stage-0123456789ab"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	abandonEvents := []string{}
	beforeLockAcquire = func() { abandonEvents = append(abandonEvents, "lock") }
	beforeAbandonBranch = func() { abandonEvents = append(abandonEvents, "branch") }
	beforeAbandonMove = func(path string) {
		requireCLISeamValue(t, path)
		abandonEvents = append(abandonEvents, "before-move")
	}
	afterAbandonMove = func(path string) {
		requireCLISeamValue(t, path)
		abandonEvents = append(abandonEvents, "after-move")
	}
	code, _, _, _ := runPrepare(
		t, "--path", abandonRoot, "prepare", abandonSlug,
		"--abandon-transaction", "--yes", "--json", "--quiet",
	)
	if code != 0 ||
		len(abandonEvents) != 4 ||
		abandonEvents[0] != "lock" ||
		abandonEvents[1] != "branch" ||
		abandonEvents[2] != "before-move" ||
		abandonEvents[3] != "after-move" {
		t.Fatalf("abandon seams = code=%d events=%v", code, abandonEvents)
	}

	redactionCalls := 0
	beforeRedactionScan = func() { redactionCalls++ }
	prepareRedactionRefusal([]store.IntentArchiveReplacementInput{{
		ArtifactID: store.IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("ordinary bytes"),
	}})
	if redactionCalls != 1 {
		t.Fatalf("beforeRedactionScan calls = %d", redactionCalls)
	}

	manualRoot, manualSlug := prepareS4Workspace(t, "named manual status seam")
	prepareS4WriteReadyBundle(t, manualRoot, manualSlug, false)
	manualCalls := 0
	beforeManualStatusCAS = func() { manualCalls++ }
	code, _, _, _ = runPrepare(
		t, "--path", manualRoot, "prepare", manualSlug,
		"--manual", "--json", "--quiet",
	)
	if code != 0 || manualCalls != 1 {
		t.Fatalf("beforeManualStatusCAS = code=%d calls=%d", code, manualCalls)
	}

	indexRoot, indexSlug := intentArchiveCLIWorkspace(t)
	replacement := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactAnalysis,
		[]byte("rehydrate bytes"),
		store.IntentArchiveWireTombstoned,
	)
	writeIntentArchiveCLIFixture(
		t,
		indexRoot,
		indexSlug,
		intentArchiveCLIIndex(t, indexSlug, intentArchiveCLIGeneration(t, indexSlug, replacement)),
		nil,
	)
	authority, err := intentlock.Acquire(indexRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !authority.Released() {
			_ = authority.Release()
		}
	})
	storage := newPrepareArchiveStorage(authority, nil)
	appendPlan, err := store.PlanIntentArchiveAppend(storage, indexSlug, []store.IntentArchiveReplacementInput{{
		ArtifactID: store.IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("rehydrate bytes"),
	}})
	if err != nil || appendPlan.Outcome() != store.IntentArchiveAppendRehydrate {
		t.Fatalf("rehydrate plan = outcome=%q err=%v", appendPlan.Outcome(), err)
	}
	indexCalls := 0
	rehydrateCalls := 0
	beforeIndexRewrite = func(path string) {
		requireCLISeamValue(t, path)
		indexCalls++
	}
	beforeRehydrateIndexRename = func(path string) {
		requireCLISeamValue(t, path)
		rehydrateCalls++
	}
	options := prepareTransactionOptions(intentpub.Options{}, appendPlan, true)
	options.BeforeRename(intentpub.WriteRequest{
		Rel:        ".tpatch/features/" + indexSlug + "/artifacts/intent-archive/index.json",
		ArtifactID: intentpub.ArtifactArchiveIndex,
		Role:       intentpub.WriteRoleOrdinaryCanonical,
	})
	if indexCalls != 1 || rehydrateCalls != 1 {
		t.Fatalf("index rename seams = index=%d rehydrate=%d", indexCalls, rehydrateCalls)
	}
	if err := authority.Release(); err != nil {
		t.Fatal(err)
	}

	recoveryRoot, recoverySlug := prepareS4Workspace(t, "named recovery seam")
	prepareS5InterruptAfterJournal(t, recoveryRoot, recoverySlug)
	recoveryCalls := 0
	afterRecoveryComplete = func() { recoveryCalls++ }
	code, _, _, _ = runPrepare(
		t, "--path", recoveryRoot, "prepare", recoverySlug, "--json", "--quiet",
	)
	if code != 0 || recoveryCalls != 1 {
		t.Fatalf("afterRecoveryComplete = code=%d calls=%d", code, recoveryCalls)
	}
}

func requireCLISeamValue(t *testing.T, value string) {
	t.Helper()
	if value == "" {
		t.Fatal("CLI injection seam received an empty identifier")
	}
}

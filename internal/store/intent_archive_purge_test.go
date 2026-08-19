package store

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestIntentArchivePurgePreviewPendingAndSelectorPreservation(t *testing.T) {
	const feature = "demo"
	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "pending", IntentArchiveWireRemovalPending)
	generation := archiveGeneration(t, feature, replacement)
	index := archiveIndex(t, feature, generation)
	storage := newArchiveMemoryStorage(t, index)
	storage.putRegular(feature, replacement.ContentSHA256, []byte("pending"))
	selectors := []struct {
		name  string
		value IntentArchivePurgeSelector
		kind  IntentArchiveSelectorKind
	}{
		{"blob", IntentArchivePurgeSelector{Blobs: []string{replacement.ContentSHA256}}, IntentArchiveSelectorBlob},
		{"generation", IntentArchivePurgeSelector{Generations: []string{generation.GenerationID}}, IntentArchiveSelectorGeneration},
		{"all", IntentArchivePurgeSelector{All: true}, IntentArchiveSelectorAll},
		{"orphans", IntentArchivePurgeSelector{Orphans: true}, IntentArchiveSelectorOrphans},
	}
	for _, selector := range selectors {
		t.Run(selector.name, func(t *testing.T) {
			storage.calls = nil
			plan, err := PreviewIntentArchivePurge(storage, feature, selector.value)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Outcome != IntentArchivePurgeRecoveryRequired ||
				!plan.RecoveryRequired ||
				plan.SelectorKind != selector.kind ||
				len(plan.PendingHashes) != 1 ||
				plan.PendingHashes[0] != replacement.ContentSHA256 {
				t.Fatalf("pending preview = %+v", plan)
			}
			if len(storage.mutationCalls()) != 0 {
				t.Fatalf("preview mutated storage: %v", storage.calls)
			}
			if selector.kind == IntentArchiveSelectorAll && !plan.StructuralBlastRadius {
				t.Fatal("--all preview omitted its structural blast-radius marker")
			}
		})
	}
}

func TestIntentArchivePurgeSelectorValidationAndSharedGeneration(t *testing.T) {
	const feature = "demo"
	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireRetained)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireRetained)
	g1 := archiveGeneration(t, feature, first)
	g2 := archiveGeneration(t, feature, second)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature, g1, g2))
	storage.putRegular(feature, first.ContentSHA256, []byte("shared"))

	_, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{}, true)
	assertArchiveCode(t, err, IntentArchiveCodeSelectorInvalid)
	_, err = PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{first.ContentSHA256},
		All:   true,
	}, true)
	assertArchiveCode(t, err, IntentArchiveCodeSelectorInvalid)
	_, err = PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Generations: []string{g1.GenerationID},
	}, true)
	shared := assertArchiveCode(t, err, IntentArchiveCodeBlobShared)
	if shared.Hash != first.ContentSHA256 {
		t.Fatalf("shared hash = %q", shared.Hash)
	}
}

func TestIntentArchiveGlobalRefusalPrecedesSharedAndPopulatesPlan(t *testing.T) {
	const feature = "demo"
	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared-corrupt", IntentArchiveWireRetained)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "shared-corrupt", IntentArchiveWireRetained)
	g1 := archiveGeneration(t, feature, first)
	g2 := archiveGeneration(t, feature, second)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature, g1, g2))
	storage.putKind(feature, first.ContentSHA256, IntentArchiveBlobKindDirectory)
	selector := IntentArchivePurgeSelector{Generations: []string{g1.GenerationID}}

	storage.calls = nil
	confirmed, err := PlanIntentArchivePurge(storage, feature, selector, true)
	assertArchiveCode(t, err, IntentArchiveCodeBlobCorrupt)
	if confirmed.RemainingRepairs == nil ||
		len(confirmed.ObservedClasses) != 1 ||
		confirmed.ObservedClasses[0].Class != IntentArchiveRepairCorruptObject ||
		confirmed.RemainingRepairs.StagesRemaining != 2 ||
		confirmed.RemainingRepairs.Stages[0].Class != IntentArchiveRepairCorruptObject ||
		confirmed.RemainingRepairs.Stages[1].Class != IntentArchiveRepairDanglingReference {
		t.Fatalf("confirmed refusal plan = %+v", confirmed)
	}
	if len(storage.mutationCalls()) != 0 {
		t.Fatalf("global refusal mutated storage: %v", storage.calls)
	}

	preview, err := PreviewIntentArchivePurge(storage, feature, selector)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.RemainingRepairs, confirmed.RemainingRepairs) ||
		!reflect.DeepEqual(preview.ObservedClasses, confirmed.ObservedClasses) {
		t.Fatalf("preview/confirm refusal parity mismatch:\npreview=%+v\nconfirmed=%+v", preview, confirmed)
	}
}

func TestIntentArchiveAllSelectsEveryReferenceAndBlob(t *testing.T) {
	const feature = "demo"
	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "first", IntentArchiveWireRetained)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "second", IntentArchiveWireTombstoned)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, first, second),
	))
	storage.putRegular(feature, first.ContentSHA256, []byte("first"))
	storage.putRegular(feature, second.ContentSHA256, []byte("second"))
	unindexedHash := archiveHash("unindexed")
	storage.putRegular(feature, unindexedHash, []byte("unindexed"))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.StructuralBlastRadius ||
		len(plan.References) != 2 ||
		!equalStringSets(plan.Hashes, []string{first.ContentSHA256, second.ContentSHA256, unindexedHash}) ||
		len(plan.BlobRemovals) != 3 {
		t.Fatalf("--all plan = %+v", plan)
	}
}

func TestIntentArchiveSequentialTwoClassRepair(t *testing.T) {
	const feature = "demo"
	residue := archiveReplacement(t, IntentArchiveArtifactAnalysis, "residue", IntentArchiveWireTombstoned)
	mixedLive := archiveReplacement(t, IntentArchiveArtifactSpec, "mixed", IntentArchiveWireRetained)
	mixedTomb := archiveReplacement(t, IntentArchiveArtifactExploration, "mixed", IntentArchiveWireTombstoned)
	index := archiveIndex(t, feature,
		archiveGeneration(t, feature, residue),
		archiveGeneration(t, feature, mixedLive),
		archiveGeneration(t, feature, mixedTomb),
	)
	storage := newArchiveMemoryStorage(t, index)
	residueRel := storage.putRegular(feature, residue.ContentSHA256, []byte("residue"))
	mixedRel := storage.putRegular(feature, mixedLive.ContentSHA256, []byte("mixed"))
	beforeIndex := append([]byte(nil), storage.index...)

	preview, err := PreviewIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.RemainingRepairs == nil || preview.RemainingRepairs.StagesRemaining != 1 {
		t.Fatalf("preview remaining repairs = %+v", preview.RemainingRepairs)
	}
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.AdmittedRepairClass != IntentArchiveRepairUnreferencedResidue ||
		plan.RemainingRepairs == nil ||
		plan.RemainingRepairs.StagesRemaining != 1 ||
		plan.RemainingRepairs.Stages[0].Class != IntentArchiveRepairMixedReference {
		t.Fatalf("admitted residue plan = %+v", plan)
	}
	result, err := ExecuteIntentArchivePurge(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != IntentArchivePurgePurged || !result.Committed {
		t.Fatalf("residue result = %+v", result)
	}
	if _, exists := storage.blobs[residueRel]; exists {
		t.Fatal("residue blob survived admitted orphan repair")
	}
	if _, exists := storage.blobs[mixedRel]; !exists {
		t.Fatal("mixed live blob was removed by orphan repair")
	}
	if !bytes.Equal(storage.index, beforeIndex) {
		t.Fatal("orphan repair rewrote index.json")
	}

	mixedPlan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{mixedLive.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if mixedPlan.AdmittedRepairClass != IntentArchiveRepairMixedReference {
		t.Fatalf("mixed repair not admitted: %+v", mixedPlan)
	}
	if _, err := ExecuteIntentArchivePurge(storage, mixedPlan); err != nil {
		t.Fatal(err)
	}
	if _, exists := storage.blobs[mixedRel]; exists {
		t.Fatal("mixed blob survived its global hash purge")
	}
}

func TestIntentArchiveMultiInstanceClassCoverage(t *testing.T) {
	const feature = "demo"
	residues := []IntentArchiveReplacement{
		archiveReplacement(t, IntentArchiveArtifactAnalysis, "residue-one", IntentArchiveWireTombstoned),
		archiveReplacement(t, IntentArchiveArtifactSpec, "residue-two", IntentArchiveWireTombstoned),
		archiveReplacement(t, IntentArchiveArtifactExploration, "residue-three", IntentArchiveWireTombstoned),
	}
	mixedOneLive := archiveReplacement(t, IntentArchiveArtifactAnalysis, "mixed-one", IntentArchiveWireRetained)
	mixedOneTomb := archiveReplacement(t, IntentArchiveArtifactSpec, "mixed-one", IntentArchiveWireTombstoned)
	mixedTwoLive := archiveReplacement(t, IntentArchiveArtifactAnalysis, "mixed-two", IntentArchiveWireRetained)
	mixedTwoTomb := archiveReplacement(t, IntentArchiveArtifactExploration, "mixed-two", IntentArchiveWireTombstoned)
	generations := []IntentArchiveGeneration{
		archiveGeneration(t, feature, residues[0]),
		archiveGeneration(t, feature, residues[1]),
		archiveGeneration(t, feature, residues[2]),
		archiveGeneration(t, feature, mixedOneLive),
		archiveGeneration(t, feature, mixedOneTomb),
		archiveGeneration(t, feature, mixedTwoLive),
		archiveGeneration(t, feature, mixedTwoTomb),
	}
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature, generations...))
	for index, value := range []string{"residue-one", "residue-two", "residue-three"} {
		storage.putRegular(feature, residues[index].ContentSHA256, []byte(value))
	}
	storage.putRegular(feature, mixedOneLive.ContentSHA256, []byte("mixed-one"))
	storage.putRegular(feature, mixedTwoLive.ContentSHA256, []byte("mixed-two"))

	residuePlan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if residuePlan.AdmittedRepairClass != IntentArchiveRepairUnreferencedResidue ||
		len(residuePlan.BlobRemovals) != 3 {
		t.Fatalf("multi-residue plan = %+v", residuePlan)
	}
	partialMixed, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{mixedOneLive.ContentSHA256},
	}, true)
	assertArchiveCode(t, err, IntentArchiveCodeIndexStorageInconsistent)
	if partialMixed.AdmittedRepairClass != "" ||
		partialMixed.RemainingRepairs == nil ||
		partialMixed.RemainingRepairs.StagesRemaining != 2 {
		t.Fatalf("partial class refusal plan = %+v", partialMixed)
	}
	partialPreview, err := PreviewIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{mixedOneLive.ContentSHA256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(partialPreview.RemainingRepairs, partialMixed.RemainingRepairs) ||
		!reflect.DeepEqual(partialPreview.ObservedClasses, partialMixed.ObservedClasses) {
		t.Fatalf("partial preview/confirm mismatch:\npreview=%+v\nconfirmed=%+v", partialPreview, partialMixed)
	}
	fullMixed, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{mixedOneLive.ContentSHA256, mixedTwoLive.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if fullMixed.AdmittedRepairClass != IntentArchiveRepairMixedReference {
		t.Fatalf("full mixed class not admitted: %+v", fullMixed)
	}
}

func TestIntentArchiveCorruptFirstStageArithmeticAndBlocking(t *testing.T) {
	const feature = "demo"
	corrupt := archiveReplacement(t, IntentArchiveArtifactAnalysis, "corrupt", IntentArchiveWireRetained)
	dangling := archiveReplacement(t, IntentArchiveArtifactSpec, "dangling", IntentArchiveWireRetained)
	residue := archiveReplacement(t, IntentArchiveArtifactExploration, "residue", IntentArchiveWireTombstoned)
	index := archiveIndex(t, feature,
		archiveGeneration(t, feature, corrupt),
		archiveGeneration(t, feature, dangling),
		archiveGeneration(t, feature, residue),
	)
	storage := newArchiveMemoryStorage(t, index)
	storage.putNonRegular(feature, corrupt.ContentSHA256)
	storage.putRegular(feature, residue.ContentSHA256, []byte("residue"))
	preview, err := PreviewIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true})
	if err != nil {
		t.Fatal(err)
	}
	remaining := preview.RemainingRepairs
	if remaining == nil || remaining.StagesRemaining != 3 {
		t.Fatalf("remaining repairs = %+v", remaining)
	}
	if remaining.Stages[0].Kind != IntentArchiveRepairStageManual ||
		remaining.Stages[0].Class != IntentArchiveRepairCorruptObject ||
		remaining.Stages[1].Class != IntentArchiveRepairDanglingReference ||
		remaining.Stages[2].Class != IntentArchiveRepairUnreferencedResidue {
		t.Fatalf("stage order = %+v", remaining.Stages)
	}
	if !equalStringSets(remaining.Stages[1].Hashes, []string{corrupt.ContentSHA256, dangling.ContentSHA256}) {
		t.Fatalf("predicted dangling merge = %v", remaining.Stages[1].Hashes)
	}
	if remaining.Stages[0].AfterPrerequisite ||
		!remaining.Stages[1].AfterPrerequisite ||
		remaining.Stages[2].AfterPrerequisite ||
		!equalStringSets(remaining.Stages[1].PredictedHashes, []string{corrupt.ContentSHA256}) {
		t.Fatalf("per-stage prerequisite flags = %+v", remaining.Stages)
	}
	if len(remaining.Stages[0].Commands) != 1 ||
		!strings.HasPrefix(remaining.Stages[0].Repair, "WARNING: destructive archive repair.") ||
		strings.Index(remaining.Stages[0].Repair, remaining.Stages[0].Commands[0].Warning) >
			strings.Index(remaining.Stages[0].Repair, remaining.Stages[0].Commands[0].Rendered) {
		t.Fatalf("corrupt repair warning/command order = %+v", remaining.Stages[0])
	}
	selectors := []IntentArchivePurgeSelector{
		{Orphans: true},
		{Blobs: []string{dangling.ContentSHA256}},
		{Generations: []string{index.Generations[1].GenerationID}},
		{All: true},
	}
	for _, selector := range selectors {
		storage.calls = nil
		_, err = PlanIntentArchivePurge(storage, feature, selector, true)
		assertArchiveCode(t, err, IntentArchiveCodeBlobCorrupt)
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("corrupt-first refusal mutated storage for %+v: %v", selector, storage.calls)
		}
	}
}

func TestIntentArchiveAfterPrerequisiteOnlyMarksPredictedMembership(t *testing.T) {
	const feature = "demo"
	corruptClean := archiveReplacement(t, IntentArchiveArtifactAnalysis, "corrupt-clean", IntentArchiveWireTombstoned)
	dangling := archiveReplacement(t, IntentArchiveArtifactSpec, "dangling-existing", IntentArchiveWireRetained)
	mixedLive := archiveReplacement(t, IntentArchiveArtifactAnalysis, "mixed-existing", IntentArchiveWireRetained)
	mixedTomb := archiveReplacement(t, IntentArchiveArtifactExploration, "mixed-existing", IntentArchiveWireTombstoned)
	residue := archiveReplacement(t, IntentArchiveArtifactSpec, "residue-existing", IntentArchiveWireTombstoned)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, corruptClean),
		archiveGeneration(t, feature, dangling),
		archiveGeneration(t, feature, mixedLive),
		archiveGeneration(t, feature, mixedTomb),
		archiveGeneration(t, feature, residue),
	))
	storage.putKind(feature, corruptClean.ContentSHA256, IntentArchiveBlobKindSymlink)
	storage.putRegular(feature, mixedLive.ContentSHA256, []byte("mixed-existing"))
	storage.putRegular(feature, residue.ContentSHA256, []byte("residue-existing"))
	preview, err := PreviewIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.RemainingRepairs == nil || preview.RemainingRepairs.StagesRemaining != 4 {
		t.Fatalf("remaining repairs = %+v", preview.RemainingRepairs)
	}
	for _, stage := range preview.RemainingRepairs.Stages {
		if stage.AfterPrerequisite || len(stage.PredictedHashes) != 0 {
			t.Fatalf("independent stage was marked after prerequisite: %+v", stage)
		}
	}
}

func TestIntentArchiveCorruptRepairCommandsAreShellSafeAndExact(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("POSIX remediation is supported only on Linux and Darwin")
	}
	const feature = "demo"
	for _, objectKind := range []string{"regular", "symlink", "directory", "fifo"} {
		t.Run(objectKind, func(t *testing.T) {
			scratch, err := os.MkdirTemp(".", "intent-archive-shell-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if removeErr := os.RemoveAll(scratch); removeErr != nil {
					t.Errorf("cleanup: %v", removeErr)
				}
			})

			blobsRel, err := IntentArchiveBlobsRel(feature)
			if err != nil {
				t.Fatal(err)
			}
			objectName := "managed object;$(touch injected)'quoted'"
			objectRel := blobsRel + "/" + objectName
			objectPath := filepath.Join(scratch, filepath.FromSlash(objectRel))
			parentPath := filepath.Dir(objectPath)
			if err := os.MkdirAll(parentPath, 0o755); err != nil {
				t.Fatal(err)
			}
			siblingPath := filepath.Join(parentPath, "sibling")
			if err := os.WriteFile(siblingPath, []byte("sibling"), 0o600); err != nil {
				t.Fatal(err)
			}
			targetPath := filepath.Join(scratch, "symlink-target")
			targetLink, err := filepath.Abs(targetPath)
			if err != nil {
				t.Fatal(err)
			}
			switch objectKind {
			case "regular":
				err = os.WriteFile(objectPath, []byte("object"), 0o600)
			case "symlink":
				if err = os.WriteFile(targetPath, []byte("target"), 0o600); err == nil {
					err = os.Symlink(targetLink, objectPath)
				}
			case "directory":
				err = os.MkdirAll(filepath.Join(objectPath, "nested"), 0o755)
				if err == nil {
					err = os.WriteFile(filepath.Join(objectPath, "nested", "child"), []byte("child"), 0o600)
				}
			case "fifo":
				err = exec.Command("mkfifo", objectPath).Run()
			}
			if err != nil {
				t.Fatal(err)
			}

			remaining := buildIntentArchiveRemainingRepairs(feature, []IntentArchiveRepairClassReport{{
				Rank:     1,
				Class:    IntentArchiveRepairCorruptObject,
				Blocking: true,
				Paths:    []string{objectRel},
				Instances: []IntentArchiveRepairInstance{{
					Path: objectRel,
				}},
			}}, "")
			if remaining == nil ||
				len(remaining.Stages) != 1 ||
				len(remaining.Stages[0].Commands) != 1 {
				t.Fatalf("repair stage = %+v", remaining)
			}
			stage := remaining.Stages[0]
			command := stage.Commands[0]
			wantArgv := []string{"rm", "-rf", "--", objectRel}
			wantRendered := "rm -rf -- " + quoteIntentArchivePOSIXShell(objectRel)
			if !reflect.DeepEqual(command.Argv, wantArgv) ||
				command.Rendered != wantRendered ||
				strings.Contains(command.Rendered, "*") ||
				!strings.HasPrefix(stage.Repair, command.Warning+"\n") ||
				strings.Index(stage.Repair, command.Warning) >= strings.Index(stage.Repair, command.Rendered) {
				t.Fatalf("unsafe remediation command: stage=%+v command=%+v", stage, command)
			}

			shell := exec.Command("/bin/sh", "-c", command.Rendered)
			shell.Dir = scratch
			if output, runErr := shell.CombinedOutput(); runErr != nil {
				t.Fatalf("command failed: %v\n%s", runErr, output)
			}
			if _, statErr := os.Lstat(objectPath); !os.IsNotExist(statErr) {
				t.Fatalf("managed object still exists or stat failed: %v", statErr)
			}
			if data, readErr := os.ReadFile(siblingPath); readErr != nil || string(data) != "sibling" {
				t.Fatalf("sibling changed: data=%q err=%v", data, readErr)
			}
			if _, statErr := os.Stat(parentPath); statErr != nil {
				t.Fatalf("blob directory prefix was removed: %v", statErr)
			}
			if _, statErr := os.Stat(filepath.Join(scratch, "injected")); !os.IsNotExist(statErr) {
				t.Fatalf("shell command substitution escaped quoting: %v", statErr)
			}
			if objectKind == "symlink" {
				if data, readErr := os.ReadFile(targetPath); readErr != nil || string(data) != "target" {
					t.Fatalf("symlink target changed: data=%q err=%v", data, readErr)
				}
			}
		})
	}
}

func TestIntentArchiveConfirmedPurgeClaimsEveryReferenceThenRemovesThenTombstones(t *testing.T) {
	const feature = "demo"
	live := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireRetained)
	tomb := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireTombstoned)
	index := archiveIndex(t, feature,
		archiveGeneration(t, feature, live),
		archiveGeneration(t, feature, tomb),
	)
	storage := newArchiveMemoryStorage(t, index)
	blobRel := storage.putRegular(feature, live.ContentSHA256, []byte("shared"))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{live.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	storage.calls = nil
	result, err := ExecuteIntentArchivePurge(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RemovalRaceResidualDisclosed || result.Outcome != IntentArchivePurgePurged {
		t.Fatalf("result = %+v", result)
	}
	if _, exists := storage.blobs[blobRel]; exists {
		t.Fatal("selected blob survived purge")
	}
	var casIndexes []int
	for index, call := range storage.calls {
		if call == "cas-index" {
			casIndexes = append(casIndexes, index)
		}
	}
	removeIndex := callIndex(storage.calls, "remove:")
	if len(casIndexes) != 2 || removeIndex <= casIndexes[0] || removeIndex >= casIndexes[1] {
		t.Fatalf("call order = %v, want claim CAS -> remove -> tombstone CAS", storage.calls)
	}
	if len(storage.indexHistory) != 2 {
		t.Fatalf("index history entries = %d, want 2", len(storage.indexHistory))
	}
	claimed, err := DecodeIntentArchiveIndex(storage.indexHistory[0], feature)
	if err != nil {
		t.Fatal(err)
	}
	for _, generation := range claimed.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == live.ContentSHA256 &&
				replacement.WireState() != IntentArchiveWireRemovalPending {
				t.Fatalf("claim skipped same-hash reference: %+v", replacement)
			}
		}
	}
	final := storage.decodedIndex(t, feature)
	for _, generation := range final.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == live.ContentSHA256 &&
				replacement.WireState() != IntentArchiveWireTombstoned {
				t.Fatalf("final reference not tombstoned: %+v", replacement)
			}
		}
	}
}

func TestIntentArchiveDanglingRepairTombstonesWithoutRemoval(t *testing.T) {
	const feature = "demo"
	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "missing", IntentArchiveWireRetained)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, replacement),
	))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	storage.calls = nil
	result, err := ExecuteIntentArchivePurge(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != IntentArchivePurgePurged || callIndex(storage.calls, "remove:") >= 0 {
		t.Fatalf("dangling repair result=%+v calls=%v", result, storage.calls)
	}
	if len(storage.indexHistory) != 1 ||
		callIndex(storage.calls, "sync:blobs") < 0 ||
		callIndex(storage.calls, "sync:blobs") >= callIndex(storage.calls, "cas-index") {
		t.Fatalf("absent direct tombstone history=%d calls=%v", len(storage.indexHistory), storage.calls)
	}
	published, err := DecodeIntentArchiveIndex(storage.indexHistory[0], feature)
	if err != nil {
		t.Fatal(err)
	}
	if intentArchiveHashHasState(published, replacement.ContentSHA256, IntentArchiveWireRemovalPending) {
		t.Fatalf("absent direct tombstone published pending wire state: %+v", published)
	}
	final := storage.decodedIndex(t, feature)
	if final.Generations[0].Replaced[0].WireState() != IntentArchiveWireTombstoned {
		t.Fatalf("dangling reference not tombstoned: %+v", final)
	}
}

func TestIntentArchiveAbsentPathAppearanceNeverRemovesOnFirstObservation(t *testing.T) {
	const feature = "demo"
	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "appeared", IntentArchiveWireRetained)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, replacement),
	))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	blobRel, _ := IntentArchiveBlobRel(feature, replacement.ContentSHA256)
	storage.hooks["before:sync:blobs"] = func(memory *archiveMemoryStorage) {
		delete(memory.hooks, "before:sync:blobs")
		memory.putRegular(feature, replacement.ContentSHA256, []byte("appeared"))
	}
	storage.calls = nil
	result, err := ExecuteIntentArchivePurge(storage, plan)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgeIndexChanged)
	if typed.ExitClass != 3 ||
		result.Committed ||
		callIndex(storage.calls, "remove:") >= 0 ||
		callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("appearance result=%+v error=%+v calls=%v", result, typed, storage.calls)
	}
	if _, exists := storage.blobs[blobRel]; !exists {
		t.Fatal("blob that appeared after initial absence was removed")
	}

	retryPlan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	storage.calls = nil
	retry, err := ExecuteIntentArchivePurge(storage, retryPlan)
	if err != nil {
		t.Fatal(err)
	}
	if retry.Outcome != IntentArchivePurgePurged ||
		callIndex(storage.calls, "cas-index") < 0 ||
		callIndex(storage.calls, "remove:") < 0 {
		t.Fatalf("present-path retry result=%+v calls=%v", retry, storage.calls)
	}
}

func TestIntentArchiveAbsentAllTombstonedStillSyncsWithoutIndexRewrite(t *testing.T) {
	const feature = "demo"
	tombstoned := archiveReplacement(t, IntentArchiveArtifactAnalysis, "already-tombstoned", IntentArchiveWireTombstoned)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, tombstoned),
	))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{tombstoned.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	storage.calls = nil
	result, err := ExecuteIntentArchivePurge(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !equalStringSets(result.CompletedHashes, []string{tombstoned.ContentSHA256}) ||
		callIndex(storage.calls, "sync:blobs") < 0 ||
		callIndex(storage.calls, "sync:intent-archive") < 0 ||
		callIndex(storage.calls, "cas-index") >= 0 ||
		callIndex(storage.calls, "remove:") >= 0 {
		t.Fatalf("all-tombstoned absent result=%+v calls=%v", result, storage.calls)
	}
}

func TestRecoverPendingPurgeClaimsRetainedAndTombstonedReferences(t *testing.T) {
	const feature = "demo"
	retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "owned", IntentArchiveWireRetained)
	pending := archiveReplacement(t, IntentArchiveArtifactSpec, "owned", IntentArchiveWireRemovalPending)
	tombstoned := archiveReplacement(t, IntentArchiveArtifactExploration, "owned", IntentArchiveWireTombstoned)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, retained),
		archiveGeneration(t, feature, pending),
		archiveGeneration(t, feature, tombstoned),
	))
	blobRel := storage.putRegular(feature, retained.ContentSHA256, []byte("owned"))
	result, err := RecoverPendingPurge(storage, feature)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != IntentArchivePurgeRecovered ||
		len(result.FinalizedHashes) != 1 ||
		result.FinalizedHashes[0] != retained.ContentSHA256 {
		t.Fatalf("recovery result = %+v", result)
	}
	if _, exists := storage.blobs[blobRel]; exists {
		t.Fatal("owned blob survived recovery")
	}
	if len(storage.indexHistory) != 2 {
		t.Fatalf("recovery history = %d, want claim and tombstone", len(storage.indexHistory))
	}
	claim, err := DecodeIntentArchiveIndex(storage.indexHistory[0], feature)
	if err != nil {
		t.Fatal(err)
	}
	for _, generation := range claim.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.WireState() != IntentArchiveWireRemovalPending {
				t.Fatalf("global recovery claim skipped %+v", replacement)
			}
		}
	}
}

func TestRecoverPendingPurgeOwnedUnidentifiablePreservesEvidence(t *testing.T) {
	const feature = "demo"
	pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "owned", IntentArchiveWireRemovalPending)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, pending),
	))
	blobRel := storage.putNonRegular(feature, pending.ContentSHA256)
	beforeIndex := append([]byte(nil), storage.index...)
	result, err := RecoverPendingPurge(storage, feature)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgeEvidenceDivergent)
	if typed.ExitClass != 6 || !typed.Committed || !result.Committed {
		t.Fatalf("divergence result=%+v error=%+v", result, typed)
	}
	if !bytes.Equal(storage.index, beforeIndex) {
		t.Fatal("divergent recovery rewrote index")
	}
	if _, exists := storage.blobs[blobRel]; !exists {
		t.Fatal("divergent recovery removed evidence")
	}
}

func TestIntentArchiveNonRegularKindsShareClosedRoutes(t *testing.T) {
	kinds := []IntentArchiveBlobKind{
		IntentArchiveBlobKindSymlink,
		IntentArchiveBlobKindDirectory,
		IntentArchiveBlobKindFIFO,
		IntentArchiveBlobKindDevice,
		IntentArchiveBlobKindOtherNonRegular,
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			const feature = "demo"
			retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "kind", IntentArchiveWireRetained)
			storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
				archiveGeneration(t, feature, retained),
			))
			storage.putKind(feature, retained.ContentSHA256, kind)
			_, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
				Blobs: []string{retained.ContentSHA256},
			}, true)
			assertArchiveCode(t, err, IntentArchiveCodeBlobCorrupt)

			pending := retained
			setIntentArchiveReplacementState(&pending, IntentArchiveWireRemovalPending)
			storage = newArchiveMemoryStorage(t, archiveIndex(t, feature,
				archiveGeneration(t, feature, pending),
			))
			storage.putKind(feature, pending.ContentSHA256, kind)
			result, err := RecoverPendingPurge(storage, feature)
			typed := assertArchiveCode(t, err, IntentArchiveCodePurgeEvidenceDivergent)
			if typed.ExitClass != 6 || !result.Committed {
				t.Fatalf("owned %s result=%+v error=%+v", kind, result, typed)
			}
		})
	}
}

func TestRecoverPendingPurgeAbsentBlobFinalizesWithoutRemoval(t *testing.T) {
	const feature = "demo"
	pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "already-removed", IntentArchiveWireRemovalPending)
	retained := archiveReplacement(t, IntentArchiveArtifactSpec, "already-removed", IntentArchiveWireRetained)
	tombstoned := archiveReplacement(t, IntentArchiveArtifactExploration, "already-removed", IntentArchiveWireTombstoned)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, pending),
		archiveGeneration(t, feature, retained),
		archiveGeneration(t, feature, tombstoned),
	))
	storage.calls = nil
	result, err := RecoverPendingPurge(storage, feature)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != IntentArchivePurgeRecovered ||
		callIndex(storage.calls, "remove:") >= 0 ||
		len(result.FinalizedHashes) != 1 ||
		len(storage.indexHistory) != 1 {
		t.Fatalf("absent recovery result=%+v calls=%v", result, storage.calls)
	}
	published, err := DecodeIntentArchiveIndex(storage.indexHistory[0], feature)
	if err != nil {
		t.Fatal(err)
	}
	if intentArchiveHashHasState(published, pending.ContentSHA256, IntentArchiveWireRemovalPending) {
		t.Fatalf("absent recovery published intermediate pending state: %+v", published)
	}
	final := storage.decodedIndex(t, feature)
	for _, generation := range final.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.WireState() != IntentArchiveWireTombstoned {
				t.Fatalf("absent recovery left non-tombstone: %+v", replacement)
			}
		}
	}
}

func TestRecoverPendingPurgeAbsentBlobRequiresDirectorySyncBeforeTombstone(t *testing.T) {
	const feature = "demo"
	pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "already-removed-sync", IntentArchiveWireRemovalPending)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, pending),
	))
	beforeIndex := append([]byte(nil), storage.index...)
	storage.fail["sync:blobs"] = 1
	storage.calls = nil
	result, err := RecoverPendingPurge(storage, feature)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
	if typed.ExitClass != 5 ||
		!typed.Committed ||
		result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
		result.PendingHash != pending.ContentSHA256 ||
		!bytes.Equal(storage.index, beforeIndex) ||
		callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("sync refusal result=%+v error=%+v calls=%v", result, typed, storage.calls)
	}

	storage.calls = nil
	retry, err := RecoverPendingPurge(storage, feature)
	if err != nil {
		t.Fatal(err)
	}
	if retry.Outcome != IntentArchivePurgeRecovered ||
		callIndex(storage.calls, "sync:blobs") < 0 ||
		callIndex(storage.calls, "sync:blobs") >= callIndex(storage.calls, "cas-index") {
		t.Fatalf("sync retry result=%+v calls=%v", retry, storage.calls)
	}
	if storage.decodedIndex(t, feature).Generations[0].Replaced[0].WireState() != IntentArchiveWireTombstoned {
		t.Fatal("retry did not tombstone the already-absent owned hash")
	}
}

func TestIntentArchiveClaimDurabilityBarrierPrecedesOwnedRemoval(t *testing.T) {
	const feature = "demo"
	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "claim-durability", IntentArchiveWireRetained)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, replacement),
	))
	blobRel := storage.putRegular(feature, replacement.ContentSHA256, []byte("claim-durability"))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}

	storage.calls = nil
	storage.fail["sync:intent-archive"] = 1
	result, err := ExecuteIntentArchivePurge(storage, plan)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
	if typed.ExitClass != 5 ||
		!typed.Committed ||
		!result.Committed ||
		result.PendingHash != replacement.ContentSHA256 ||
		result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
		callIndex(storage.calls, "cas-index") < 0 ||
		callIndex(storage.calls, "sync:intent-archive") < callIndex(storage.calls, "cas-index") ||
		callIndex(storage.calls, "remove:") >= 0 {
		t.Fatalf("claim sync failure result=%+v error=%+v calls=%v", result, typed, storage.calls)
	}
	if _, exists := storage.blobs[blobRel]; !exists {
		t.Fatal("claim sync failure unlinked the blob")
	}
	pendingIndex := storage.decodedIndex(t, feature)
	if !allIntentArchiveReferencesPending(pendingIndex, replacement.ContentSHA256) {
		t.Fatal("claim sync failure did not retain pending evidence")
	}

	storage.calls = nil
	storage.fail["sync:intent-archive"] = 1
	retry, err := RecoverPendingPurge(storage, feature)
	typed = assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
	if typed.ExitClass != 5 ||
		!typed.Committed ||
		retry.PendingHash != replacement.ContentSHA256 ||
		retry.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
		callIndex(storage.calls, "sync:intent-archive") < 0 ||
		callIndex(storage.calls, "remove:") >= 0 {
		t.Fatalf("claim barrier retry failure result=%+v error=%+v calls=%v", retry, typed, storage.calls)
	}
	if !allIntentArchiveReferencesPending(storage.decodedIndex(t, feature), replacement.ContentSHA256) {
		t.Fatal("claim barrier retry failure lost pending evidence")
	}

	storage.calls = nil
	retry, err = RecoverPendingPurge(storage, feature)
	if err != nil {
		t.Fatal(err)
	}
	claimSync := callIndex(storage.calls, "sync:intent-archive")
	remove := callIndex(storage.calls, "remove:")
	if retry.Outcome != IntentArchivePurgeRecovered ||
		claimSync < 0 ||
		remove < 0 ||
		claimSync >= remove {
		t.Fatalf("claim retry result=%+v calls=%v", retry, storage.calls)
	}

	crashStorage := newArchiveMemoryStorage(t, pendingIndex)
	crashBlobRel := crashStorage.putRegular(feature, replacement.ContentSHA256, []byte("claim-durability"))
	crashStorage.calls = nil
	crashResult, err := RecoverPendingPurge(crashStorage, feature)
	if err != nil {
		t.Fatal(err)
	}
	crashSync := callIndex(crashStorage.calls, "sync:intent-archive")
	crashRemove := callIndex(crashStorage.calls, "remove:")
	if crashResult.Outcome != IntentArchivePurgeRecovered ||
		crashSync < 0 ||
		crashRemove < 0 ||
		crashSync >= crashRemove {
		t.Fatalf("crash recovery result=%+v calls=%v", crashResult, crashStorage.calls)
	}
	if _, exists := crashStorage.blobs[crashBlobRel]; exists {
		t.Fatal("crash recovery left the owned blob")
	}
}

func TestRecoverPendingPurgeRetriesCommittedTombstoneDirectorySync(t *testing.T) {
	const feature = "demo"
	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "finalized-durability", IntentArchiveWireRetained)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, replacement),
	))
	blobRel := storage.putRegular(feature, replacement.ContentSHA256, []byte("finalized-durability"))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	syncCount := 0
	storage.hooks["before:sync:intent-archive"] = func(memory *archiveMemoryStorage) {
		syncCount++
		if syncCount == 2 {
			memory.fail["sync:intent-archive"] = 1
		}
	}
	result, err := ExecuteIntentArchivePurge(storage, plan)
	assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
	if result.Resume != IntentArchiveResumeCompletionOnly ||
		result.State != IntentArchivePurgeStateConsistent ||
		!result.Committed ||
		result.PendingHash != "" ||
		len(result.CompletedHashes) != 1 {
		t.Fatalf("final tombstone sync failure result=%+v calls=%v", result, storage.calls)
	}
	if _, exists := storage.blobs[blobRel]; exists {
		t.Fatal("final tombstone sync failure did not preserve committed removal")
	}
	if !intentArchiveHashAllTombstoned(storage.decodedIndex(t, feature), replacement.ContentSHA256) {
		t.Fatal("final tombstone CAS did not commit")
	}
	var casCalls, archiveSyncCalls []int
	for index, call := range storage.calls {
		switch call {
		case "cas-index":
			casCalls = append(casCalls, index)
		case "sync:intent-archive":
			archiveSyncCalls = append(archiveSyncCalls, index)
		}
	}
	removeCall := callIndex(storage.calls, "remove:")
	blobSyncCall := callIndex(storage.calls, "sync:blobs")
	if len(casCalls) != 2 ||
		len(archiveSyncCalls) != 2 ||
		removeCall < 0 ||
		blobSyncCall <= removeCall ||
		casCalls[1] <= blobSyncCall ||
		archiveSyncCalls[1] <= casCalls[1] {
		t.Fatalf("durability order calls=%v", storage.calls)
	}

	pendingCrashIndex, err := DecodeIntentArchiveIndex(storage.indexHistory[0], feature)
	if err != nil {
		t.Fatal(err)
	}
	if !allIntentArchiveReferencesPending(pendingCrashIndex, replacement.ContentSHA256) {
		t.Fatalf("pre-tombstone crash index is not recoverable: %+v", pendingCrashIndex)
	}
	crashStorage := newArchiveMemoryStorage(t, pendingCrashIndex)
	crashResult, err := RecoverPendingPurge(crashStorage, feature)
	if err != nil {
		t.Fatal(err)
	}
	if crashResult.Outcome != IntentArchivePurgeRecovered ||
		!intentArchiveHashAllTombstoned(crashStorage.decodedIndex(t, feature), replacement.ContentSHA256) {
		t.Fatalf("crash recovery result=%+v calls=%v", crashResult, crashStorage.calls)
	}

	delete(storage.hooks, "before:sync:intent-archive")
	storage.fail["sync:intent-archive"] = 1
	storage.calls = nil
	retry, err := RecoverPendingPurge(storage, feature)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
	if typed.ExitClass != 5 ||
		!typed.Committed ||
		retry.Outcome != IntentArchivePurgePartial ||
		retry.State != IntentArchivePurgeStateConsistent ||
		!retry.Committed ||
		retry.Resume != IntentArchiveResumeCompletionOnly ||
		callIndex(storage.calls, "sync:blobs") < 0 ||
		callIndex(storage.calls, "sync:intent-archive") <= callIndex(storage.calls, "sync:blobs") ||
		callIndex(storage.calls, "probe:") >= 0 ||
		callIndex(storage.calls, "remove:") >= 0 ||
		callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("finalized barrier retry result=%+v error=%+v calls=%v", retry, typed, storage.calls)
	}

	storage.calls = nil
	retry, err = RecoverPendingPurge(storage, feature)
	if err != nil {
		t.Fatal(err)
	}
	if retry.Outcome != IntentArchivePurgeNoOp ||
		retry.Committed ||
		callIndex(storage.calls, "sync:blobs") < 0 ||
		callIndex(storage.calls, "sync:intent-archive") <= callIndex(storage.calls, "sync:blobs") ||
		callIndex(storage.calls, "probe:") >= 0 ||
		callIndex(storage.calls, "remove:") >= 0 ||
		callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("finalized recovery completion result=%+v calls=%v", retry, storage.calls)
	}
}

func TestRecoverPendingPurgeNonOwnedStatesAreNoOp(t *testing.T) {
	const feature = "demo"
	tests := []struct {
		name  string
		index IntentArchiveIndex
		setup func(*archiveMemoryStorage)
	}{
		{
			name: "tombstone-residue",
			index: func() IntentArchiveIndex {
				tombstoned := archiveReplacement(t, IntentArchiveArtifactAnalysis, "tombstone-residue", IntentArchiveWireTombstoned)
				return archiveIndex(t, feature, archiveGeneration(t, feature, tombstoned))
			}(),
			setup: func(storage *archiveMemoryStorage) {
				hash := archiveHash("tombstone-residue")
				storage.putRegular(feature, hash, []byte("tombstone-residue"))
			},
		},
		{
			name: "mixed-reference",
			index: func() IntentArchiveIndex {
				retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "mixed-reference", IntentArchiveWireRetained)
				tombstoned := archiveReplacement(t, IntentArchiveArtifactSpec, "mixed-reference", IntentArchiveWireTombstoned)
				return archiveIndex(t, feature,
					archiveGeneration(t, feature, retained),
					archiveGeneration(t, feature, tombstoned),
				)
			}(),
			setup: func(storage *archiveMemoryStorage) {
				hash := archiveHash("mixed-reference")
				storage.putRegular(feature, hash, []byte("mixed-reference"))
			},
		},
		{
			name: "dangling-reference",
			index: func() IntentArchiveIndex {
				retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "dangling-reference", IntentArchiveWireRetained)
				return archiveIndex(t, feature, archiveGeneration(t, feature, retained))
			}(),
		},
		{
			name: "corrupt-object",
			index: func() IntentArchiveIndex {
				retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "corrupt-object", IntentArchiveWireRetained)
				return archiveIndex(t, feature, archiveGeneration(t, feature, retained))
			}(),
			setup: func(storage *archiveMemoryStorage) {
				storage.putNonRegular(feature, archiveHash("corrupt-object"))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage := newArchiveMemoryStorage(t, test.index)
			if test.setup != nil {
				test.setup(storage)
			}
			beforeIndex := append([]byte(nil), storage.index...)
			beforeBlobs := len(storage.blobs)
			storage.calls = nil
			result, err := RecoverPendingPurge(storage, feature)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != IntentArchivePurgeNoOp ||
				result.Committed ||
				callIndex(storage.calls, "sync:blobs") < 0 ||
				callIndex(storage.calls, "sync:intent-archive") <= callIndex(storage.calls, "sync:blobs") ||
				callIndex(storage.calls, "probe:") >= 0 ||
				callIndex(storage.calls, "remove:") >= 0 ||
				callIndex(storage.calls, "cas-index") >= 0 ||
				!bytes.Equal(storage.index, beforeIndex) ||
				len(storage.blobs) != beforeBlobs {
				t.Fatalf("non-owned recovery result=%+v calls=%v", result, storage.calls)
			}
		})
	}
}

func TestRecoverPendingPurgeLeavesTombstoneResidueForOrphanSelector(t *testing.T) {
	const feature = "demo"
	tombstoned := archiveReplacement(t, IntentArchiveArtifactAnalysis, "tombstone-residue-repair", IntentArchiveWireTombstoned)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, tombstoned),
	))
	blobRel := storage.putRegular(feature, tombstoned.ContentSHA256, []byte("tombstone-residue-repair"))
	beforeIndex := append([]byte(nil), storage.index...)

	if result, err := RecoverPendingPurge(storage, feature); err != nil || result.Outcome != IntentArchivePurgeNoOp {
		t.Fatalf("recovery result=%+v error=%v", result, err)
	}
	if _, exists := storage.blobs[blobRel]; !exists {
		t.Fatal("recovery removed tombstone residue")
	}

	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ExecuteIntentArchivePurge(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != IntentArchivePurgePurged ||
		!result.Committed ||
		!bytes.Equal(storage.index, beforeIndex) {
		t.Fatalf("orphan repair result=%+v", result)
	}
	if _, exists := storage.blobs[blobRel]; exists {
		t.Fatal("orphan selector left tombstone residue")
	}
}

func TestRecoverPendingPurgeFreshClassificationAfterPreflightErrors(t *testing.T) {
	const feature = "demo"
	t.Run("index-identity-change-remains-owned", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "preflight-index", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
		))
		blobRel := storage.putRegular(feature, pending.ContentSHA256, []byte("preflight-index"))
		storage.hooks["before:preflight-index-cas"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "before:preflight-index-cas")
			latest := memory.decodedIndex(t, feature)
			latest.Generations = append(latest.Generations, archiveGeneration(t, feature,
				archiveReplacement(t, IntentArchiveArtifactSpec, "preflight-index", IntentArchiveWireRetained),
			))
			memory.externalSetIndex(t, latest)
		}
		storage.calls = nil
		result, err := RecoverPendingPurge(storage, feature)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if typed.ExitClass != 5 ||
			result.PendingHash != pending.ContentSHA256 ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.State != IntentArchivePurgeStateConsistent ||
			callIndex(storage.calls, "sync:intent-archive") < 0 ||
			callIndex(storage.calls, "sync:intent-archive") >= callIndex(storage.calls, "preflight-index-cas") ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("fresh index classification result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("preflight index identity change removed owned evidence")
		}

		retry, err := RecoverPendingPurge(storage, feature)
		if err != nil {
			t.Fatal(err)
		}
		if retry.Outcome != IntentArchivePurgeRecovered ||
			!intentArchiveHashAllTombstoned(storage.decodedIndex(t, feature), pending.ContentSHA256) {
			t.Fatalf("fresh index classification retry=%+v", retry)
		}
	})

	t.Run("blob-identity-change-remains-owned", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "preflight-blob", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
		))
		blobRel := storage.putRegular(feature, pending.ContentSHA256, []byte("preflight-blob"))
		storage.hooks["before:preflight-remove:"+path.Base(blobRel)] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "before:preflight-remove:"+path.Base(blobRel))
			latest := memory.decodedIndex(t, feature)
			latest.Generations = append(latest.Generations, archiveGeneration(t, feature,
				archiveReplacement(t, IntentArchiveArtifactSpec, "preflight-blob", IntentArchiveWireTombstoned),
			))
			memory.externalSetIndex(t, latest)
			blob := memory.blobs[blobRel]
			blob.version++
			memory.blobs[blobRel] = blob
		}
		result, err := RecoverPendingPurge(storage, feature)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if typed.ExitClass != 5 ||
			result.PendingHash != pending.ContentSHA256 ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.State != IntentArchivePurgeStateConsistent ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("fresh blob classification result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("preflight blob identity change removed owned evidence")
		}
		if _, err := RecoverPendingPurge(storage, feature); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("undecodable-index-is-divergent", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "preflight-corrupt", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
		))
		blobRel := storage.putRegular(feature, pending.ContentSHA256, []byte("preflight-corrupt"))
		storage.hooks["before:preflight-index-cas"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "before:preflight-index-cas")
			memory.index = []byte(`{"schema_version":1`)
			memory.indexVersion++
		}
		result, err := RecoverPendingPurge(storage, feature)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgeEvidenceDivergent)
		if typed.ExitClass != 6 ||
			result.State == IntentArchivePurgeStateConsistent ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("undecodable classification result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("undecodable preflight evidence was removed")
		}
	})

	t.Run("ownership-removed-is-divergent", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "preflight-removed", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
		))
		blobRel := storage.putRegular(feature, pending.ContentSHA256, []byte("preflight-removed"))
		storage.hooks["before:preflight-index-cas"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "before:preflight-index-cas")
			memory.externalSetIndex(t, archiveIndex(t, feature))
		}
		result, err := RecoverPendingPurge(storage, feature)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgeEvidenceDivergent)
		if typed.ExitClass != 6 ||
			result.PendingHash != "" ||
			result.Resume != IntentArchiveResumeCompletionOnly ||
			result.State == IntentArchivePurgeStateConsistent ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("ownership-removed result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("ownership-removed classification deleted the blob")
		}
	})

	t.Run("ownership-removed-after-preflight-is-divergent", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "post-preflight-removed", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
		))
		blobRel := storage.putRegular(feature, pending.ContentSHA256, []byte("post-preflight-removed"))
		hook := "before:preflight-remove:" + path.Base(blobRel)
		storage.hooks[hook] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, hook)
			memory.externalSetIndex(t, archiveIndex(t, feature))
		}
		result, err := RecoverPendingPurge(storage, feature)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgeEvidenceDivergent)
		if typed.ExitClass != 6 ||
			!typed.Committed ||
			result.State == IntentArchivePurgeStateConsistent ||
			result.PendingHash != "" ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("post-preflight ownership loss result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("post-preflight ownership loss deleted the blob")
		}
	})
}

func TestIntentArchiveConfirmedPurgeCorruptIndexAfterPriorMutationIsDivergent(t *testing.T) {
	const feature = "demo"
	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "prior-mutation-first", IntentArchiveWireRetained)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "prior-mutation-second", IntentArchiveWireRetained)
	ordered := []IntentArchiveReplacement{first, second}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ContentSHA256 < ordered[j].ContentSHA256
	})
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, first, second),
	))
	blobRels := map[string]string{
		first.ContentSHA256:  storage.putRegular(feature, first.ContentSHA256, []byte("prior-mutation-first")),
		second.ContentSHA256: storage.putRegular(feature, second.ContentSHA256, []byte("prior-mutation-second")),
	}
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{first.ContentSHA256, second.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	casCount := 0
	armed := false
	skippedOwnPublishCapture := false
	storage.hooks["after-index-cas"] = func(memory *archiveMemoryStorage) {
		casCount++
		if casCount == 2 {
			armed = true
		}
	}
	storage.hooks["before:capture-index"] = func(memory *archiveMemoryStorage) {
		if !armed {
			return
		}
		if !skippedOwnPublishCapture {
			skippedOwnPublishCapture = true
			return
		}
		delete(memory.hooks, "before:capture-index")
		memory.index = []byte(`{"schema_version":1`)
		memory.indexVersion++
	}

	result, err := ExecuteIntentArchivePurge(storage, plan)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgeEvidenceDivergent)
	if typed.ExitClass != 6 ||
		!typed.Committed ||
		!result.Committed ||
		result.State == IntentArchivePurgeStateConsistent ||
		result.Resume != IntentArchiveResumeCompletionOnly ||
		result.PendingHash != "" ||
		!equalStringSets(result.CompletedHashes, []string{ordered[0].ContentSHA256}) ||
		!equalStringSets(result.RemainingHashes, []string{ordered[1].ContentSHA256}) {
		t.Fatalf("post-mutation divergence result=%+v error=%+v calls=%v", result, typed, storage.calls)
	}
	if _, exists := storage.blobs[blobRels[ordered[0].ContentSHA256]]; exists {
		t.Fatal("first committed purge mutation was lost")
	}
	if _, exists := storage.blobs[blobRels[ordered[1].ContentSHA256]]; !exists {
		t.Fatal("post-mutation divergence removed the second blob")
	}
}

func TestIntentArchivePurgePartialBranchesAndRetryConvergence(t *testing.T) {
	t.Run("pending-recovery-then-completion", func(t *testing.T) {
		const feature = "demo"
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "partial", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, replacement),
		))
		storage.putRegular(feature, replacement.ContentSHA256, []byte("partial"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{replacement.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		storage.hooks["after-index-cas"] = func(memory *archiveMemoryStorage) {
			if fired {
				return
			}
			fired = true
			memory.fail["probe:"+replacement.ContentSHA256+".blob"] = 1
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.PendingHash != replacement.ContentSHA256 ||
			result.State != IntentArchivePurgeStateConsistent {
			t.Fatalf("partial result = %+v", result)
		}
		delete(storage.hooks, "after-index-cas")
		recovered, err := RecoverPendingPurge(storage, feature)
		if err != nil {
			t.Fatal(err)
		}
		if recovered.Outcome != IntentArchivePurgeRecovered {
			t.Fatalf("retry recovery = %+v", recovered)
		}
	})

	t.Run("completion-only", func(t *testing.T) {
		const feature = "demo"
		first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "first", IntentArchiveWireRetained)
		second := archiveReplacement(t, IntentArchiveArtifactSpec, "second", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, first, second),
		))
		storage.putRegular(feature, first.ContentSHA256, []byte("first"))
		storage.putRegular(feature, second.ContentSHA256, []byte("second"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{first.ContentSHA256, second.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		casCount := 0
		storage.hooks["after-index-cas"] = func(memory *archiveMemoryStorage) {
			casCount++
			if casCount == 2 {
				memory.fail["capture-index"] = 1
			}
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if result.Resume != IntentArchiveResumeCompletionOnly ||
			result.PendingHash != "" ||
			len(result.CompletedHashes) != 1 ||
			len(result.RemainingHashes) != 1 {
			t.Fatalf("completion-only result = %+v", result)
		}
		delete(storage.hooks, "after-index-cas")
		retry, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{first.ContentSHA256, second.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ExecuteIntentArchivePurge(storage, retry); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("orphan-scan-pre-remove-error", func(t *testing.T) {
		const feature = "demo"
		firstHash := archiveHash("orphan-one")
		secondHash := archiveHash("orphan-two")
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature))
		beforeIndex := append([]byte(nil), storage.index...)
		firstRel := storage.putRegular(feature, firstHash, []byte("orphan-one"))
		secondRel := storage.putRegular(feature, secondHash, []byte("orphan-two"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		paths := []string{firstRel, secondRel}
		sort.Strings(paths)
		storage.fail["remove:"+path.Base(paths[1])] = 1
		result, err := ExecuteIntentArchivePurge(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if result.Resume != IntentArchiveResumeOrphanScan ||
			result.PendingHash != "" ||
			len(result.CompletedHashes) != 1 ||
			len(result.RemainingHashes) != 1 ||
			result.CompletedHashes[0] != intentArchiveHashFromTestBlobPath(paths[0]) ||
			result.RemainingHashes[0] != intentArchiveHashFromTestBlobPath(paths[1]) ||
			!bytes.Equal(storage.index, beforeIndex) {
			t.Fatalf("orphan partial = %+v", result)
		}
		retry, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		storage.calls = nil
		if _, err := ExecuteIntentArchivePurge(storage, retry); err != nil {
			t.Fatal(err)
		}
		if callIndex(storage.calls, "sync:blobs") < 0 ||
			callIndex(storage.calls, "sync:blobs") >= callIndex(storage.calls, "remove:") {
			t.Fatalf("orphan retry did not sync before remaining removals: %v", storage.calls)
		}
		if len(storage.blobs) != 0 {
			t.Fatalf("orphan retry left blobs: %+v", storage.blobs)
		}
	})

	t.Run("orphan-scan-post-remove-error", func(t *testing.T) {
		const feature = "demo"
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature))
		beforeIndex := append([]byte(nil), storage.index...)
		paths := []string{
			storage.putRegular(feature, archiveHash("post-one"), []byte("post-one")),
			storage.putRegular(feature, archiveHash("post-two"), []byte("post-two")),
		}
		sort.Strings(paths)
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		storage.postError["remove:"+path.Base(paths[0])] = &IntentArchiveError{
			Code:      IntentArchiveCodeBlobCorrupt,
			ExitClass: 3,
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if result.Resume != IntentArchiveResumeOrphanScan ||
			len(result.CompletedHashes) != 1 ||
			result.CompletedHashes[0] != intentArchiveHashFromTestBlobPath(paths[0]) ||
			len(result.RemainingHashes) != 1 ||
			result.RemainingHashes[0] != intentArchiveHashFromTestBlobPath(paths[1]) ||
			!bytes.Equal(storage.index, beforeIndex) {
			t.Fatalf("post-remove partial = %+v", result)
		}
		retry, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		storage.calls = nil
		if _, err := ExecuteIntentArchivePurge(storage, retry); err != nil {
			t.Fatal(err)
		}
		if callIndex(storage.calls, "sync:blobs") < 0 ||
			callIndex(storage.calls, "sync:blobs") >= callIndex(storage.calls, "remove:") ||
			len(storage.blobs) != 0 ||
			!bytes.Equal(storage.index, beforeIndex) {
			t.Fatalf("post-remove retry calls=%v blobs=%+v", storage.calls, storage.blobs)
		}
	})

	t.Run("orphan-scan-unlink-committed-sync-failed", func(t *testing.T) {
		const feature = "demo"
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature))
		beforeIndex := append([]byte(nil), storage.index...)
		hash := archiveHash("sync-orphan")
		storage.putRegular(feature, hash, []byte("sync-orphan"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		storage.hooks["after-remove"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "after-remove")
			memory.fail["sync:blobs"] = 1
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if result.Resume != IntentArchiveResumeOrphanScan ||
			len(result.CompletedHashes) != 0 ||
			!equalStringSets(result.RemainingHashes, []string{hash}) ||
			!bytes.Equal(storage.index, beforeIndex) {
			t.Fatalf("sync-failed orphan result = %+v", result)
		}
		retry, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		if retry.Outcome != IntentArchivePurgeNoOp {
			t.Fatalf("absent orphan retry plan = %+v", retry)
		}
		storage.calls = nil
		retryResult, err := ExecuteIntentArchivePurge(storage, retry)
		if err != nil {
			t.Fatal(err)
		}
		if retryResult.Outcome != IntentArchivePurgeNoOp ||
			callIndex(storage.calls, "sync:blobs") < 0 ||
			callIndex(storage.calls, "remove:") >= 0 ||
			!bytes.Equal(storage.index, beforeIndex) {
			t.Fatalf("sync-establishing retry result=%+v calls=%v", retryResult, storage.calls)
		}
	})

	t.Run("all-unindexed-unlink-committed-sync-failed", func(t *testing.T) {
		const feature = "demo"
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature))
		hash := archiveHash("sync-all-unindexed")
		storage.putRegular(feature, hash, []byte("sync-all-unindexed"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		storage.hooks["after-remove"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "after-remove")
			memory.fail["sync:blobs"] = 1
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if typed.ExitClass != 5 ||
			len(result.CompletedHashes) != 0 ||
			!equalStringSets(result.RemainingHashes, []string{hash}) {
			t.Fatalf("--all sync failure result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}

		retryPlan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		if retryPlan.Outcome != IntentArchivePurgeNoOp {
			t.Fatalf("--all retry plan=%+v", retryPlan)
		}
		storage.calls = nil
		retry, err := ExecuteIntentArchivePurge(storage, retryPlan)
		if err != nil {
			t.Fatal(err)
		}
		if retry.Outcome != IntentArchivePurgeNoOp ||
			callIndex(storage.calls, "sync:blobs") < 0 ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("--all durability retry result=%+v calls=%v", retry, storage.calls)
		}
	})
}

func TestRecoverPendingPurgeInsertionWindowsPIB544(t *testing.T) {
	const feature = "demo"
	t.Run("before-claim-reread-is-included", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "before-reread", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
		))
		storage.putRegular(feature, pending.ContentSHA256, []byte("before-reread"))
		captures := 0
		storage.hooks["before:capture-index"] = func(memory *archiveMemoryStorage) {
			captures++
			if captures != 2 {
				return
			}
			delete(memory.hooks, "before:capture-index")
			latest := memory.decodedIndex(t, feature)
			latest.Generations = append(latest.Generations, archiveGeneration(t, feature,
				archiveReplacement(t, IntentArchiveArtifactSpec, "before-reread", IntentArchiveWireRetained),
			))
			memory.externalSetIndex(t, latest)
		}
		result, err := RecoverPendingPurge(storage, feature)
		if err != nil {
			t.Fatal(err)
		}
		if result.Outcome != IntentArchivePurgeRecovered ||
			callIndex(storage.calls, "remove:") < 0 ||
			!intentArchiveHashAllTombstoned(storage.decodedIndex(t, feature), pending.ContentSHA256) {
			t.Fatalf("before-reread result=%+v calls=%v", result, storage.calls)
		}
	})

	t.Run("between-reread-and-claim-cas-is-partial", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "between-reread-cas", IntentArchiveWireRemovalPending)
		retained := archiveReplacement(t, IntentArchiveArtifactSpec, "between-reread-cas", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
			archiveGeneration(t, feature, retained),
		))
		blobRel := storage.putRegular(feature, pending.ContentSHA256, []byte("between-reread-cas"))
		storage.hooks["before-index-cas"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "before-index-cas")
			latest := memory.decodedIndex(t, feature)
			latest.Generations = append(latest.Generations, archiveGeneration(t, feature,
				archiveReplacement(t, IntentArchiveArtifactExploration, "between-reread-cas", IntentArchiveWireRetained),
			))
			memory.externalSetIndex(t, latest)
		}
		result, err := RecoverPendingPurge(storage, feature)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if typed.ExitClass != 5 ||
			result.PendingHash != pending.ContentSHA256 ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.State != IntentArchivePurgeStateConsistent ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("between-reread/CAS result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("between-reread/CAS insertion allowed removal")
		}
		if _, err := RecoverPendingPurge(storage, feature); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("between-claim-cas-and-revalidate-is-partial", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "between-cas-revalidate", IntentArchiveWireRemovalPending)
		retained := archiveReplacement(t, IntentArchiveArtifactSpec, "between-cas-revalidate", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
			archiveGeneration(t, feature, retained),
		))
		blobRel := storage.putRegular(feature, pending.ContentSHA256, []byte("between-cas-revalidate"))
		fired := false
		storage.hooks["after-index-cas"] = func(memory *archiveMemoryStorage) {
			if fired {
				return
			}
			fired = true
			latest := memory.decodedIndex(t, feature)
			latest.Generations = append(latest.Generations, archiveGeneration(t, feature,
				archiveReplacement(t, IntentArchiveArtifactExploration, "between-cas-revalidate", IntentArchiveWireRetained),
			))
			memory.externalSetIndex(t, latest)
		}
		result, err := RecoverPendingPurge(storage, feature)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if typed.ExitClass != 5 ||
			result.PendingHash != pending.ContentSHA256 ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.State != IntentArchivePurgeStateConsistent ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("between-CAS/revalidate result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("between-CAS/revalidate insertion allowed removal")
		}
		delete(storage.hooks, "after-index-cas")
		if _, err := RecoverPendingPurge(storage, feature); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("after-removal-before-tombstone-retries-absent-path", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "after-remove-window", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
		))
		blobRel := storage.putRegular(feature, pending.ContentSHA256, []byte("after-remove-window"))
		storage.hooks["after-remove"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "after-remove")
			latest := memory.decodedIndex(t, feature)
			latest.Generations = append(latest.Generations, archiveGeneration(t, feature,
				archiveReplacement(t, IntentArchiveArtifactSpec, "after-remove-window", IntentArchiveWireRetained),
			))
			memory.externalSetIndex(t, latest)
		}
		result, err := RecoverPendingPurge(storage, feature)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if typed.ExitClass != 5 ||
			result.PendingHash != pending.ContentSHA256 ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			len(result.CompletedHashes) != 0 {
			t.Fatalf("after-remove window result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
		if _, exists := storage.blobs[blobRel]; exists {
			t.Fatal("after-remove window did not commit the unlink")
		}

		historyBeforeRetry := len(storage.indexHistory)
		storage.calls = nil
		retry, err := RecoverPendingPurge(storage, feature)
		if err != nil {
			t.Fatal(err)
		}
		if retry.Outcome != IntentArchivePurgeRecovered ||
			callIndex(storage.calls, "remove:") >= 0 ||
			len(storage.indexHistory) != historyBeforeRetry+1 ||
			!intentArchiveHashAllTombstoned(storage.decodedIndex(t, feature), pending.ContentSHA256) {
			t.Fatalf("after-remove absent retry=%+v calls=%v", retry, storage.calls)
		}
		published, err := DecodeIntentArchiveIndex(storage.indexHistory[len(storage.indexHistory)-1], feature)
		if err != nil {
			t.Fatal(err)
		}
		if intentArchiveHashHasState(published, pending.ContentSHA256, IntentArchiveWireRemovalPending) {
			t.Fatalf("absent retry republished pending state: %+v", published)
		}
	})
}

func TestIntentArchiveNewSelectionAbsentRecaptureRejectsInsertionPIB421(t *testing.T) {
	const feature = "demo"
	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "new-selection-absent", IntentArchiveWireRetained)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, replacement),
	))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}

	captures := 0
	var insertedIndex []byte
	storage.hooks["before:capture-index"] = func(memory *archiveMemoryStorage) {
		captures++
		if captures != 3 {
			return
		}
		delete(memory.hooks, "before:capture-index")
		latest := memory.decodedIndex(t, feature)
		latest.Generations = append(latest.Generations, archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactSpec, "new-selection-absent", IntentArchiveWireRetained),
		))
		memory.externalSetIndex(t, latest)
		insertedIndex = append([]byte(nil), memory.index...)
	}
	storage.calls = nil
	result, err := ExecuteIntentArchivePurge(storage, plan)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgeIndexChanged)
	if typed.ExitClass != 3 ||
		typed.Committed ||
		result.Committed ||
		len(storage.indexHistory) != 0 ||
		callIndex(storage.calls, "cas-index") >= 0 ||
		callIndex(storage.calls, "remove:") >= 0 ||
		!bytes.Equal(storage.index, insertedIndex) {
		t.Fatalf("new-selection insertion result=%+v error=%+v calls=%v", result, typed, storage.calls)
	}
}

func TestIntentArchivePurgeExternalWindowsAndRemovalResidual(t *testing.T) {
	t.Run("same-hash-insert-before-claim-cas", func(t *testing.T) {
		const feature = "demo"
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "window", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, replacement),
		))
		blobRel := storage.putRegular(feature, replacement.ContentSHA256, []byte("window"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{replacement.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		storage.hooks["before-index-cas"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "before-index-cas")
			current := memory.decodedIndex(t, feature)
			current.Generations = append(current.Generations,
				archiveGeneration(t, feature,
					archiveReplacement(t, IntentArchiveArtifactSpec, "window", IntentArchiveWireRetained),
				),
			)
			memory.externalSetIndex(t, current)
		}
		_, err = ExecuteIntentArchivePurge(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodePurgeIndexChanged)
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("pre-CAS insertion allowed blob removal")
		}
	})

	t.Run("insert-after-removal-resumes-absent-path", func(t *testing.T) {
		const feature = "demo"
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "after-remove", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, replacement),
		))
		storage.putRegular(feature, replacement.ContentSHA256, []byte("after-remove"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{replacement.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		storage.hooks["after-remove"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "after-remove")
			current := memory.decodedIndex(t, feature)
			current.Generations = append(current.Generations,
				archiveGeneration(t, feature,
					archiveReplacement(t, IntentArchiveArtifactSpec, "after-remove", IntentArchiveWireRetained),
				),
			)
			memory.externalSetIndex(t, current)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if typed.ExitClass != 5 ||
			result.State != IntentArchivePurgeStateConsistent ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.PendingHash != replacement.ContentSHA256 {
			t.Fatalf("after-remove partial = %+v error=%+v", result, typed)
		}
		recovered, err := RecoverPendingPurge(storage, feature)
		if err != nil {
			t.Fatal(err)
		}
		if recovered.Outcome != IntentArchivePurgeRecovered {
			t.Fatalf("absent-path recovery = %+v", recovered)
		}
		final := storage.decodedIndex(t, feature)
		for _, generation := range final.Generations {
			for _, ref := range generation.Replaced {
				if ref.ContentSHA256 == replacement.ContentSHA256 &&
					ref.WireState() != IntentArchiveWireTombstoned {
					t.Fatalf("inserted reference survived absent-path recovery: %+v", ref)
				}
			}
		}
	})

	t.Run("same-hash-insert-after-claim-before-revalidation", func(t *testing.T) {
		const feature = "demo"
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "after-claim", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, replacement),
		))
		blobRel := storage.putRegular(feature, replacement.ContentSHA256, []byte("after-claim"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{replacement.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		storage.hooks["after-index-cas"] = func(memory *archiveMemoryStorage) {
			if fired {
				return
			}
			fired = true
			current := memory.decodedIndex(t, feature)
			current.Generations = append(current.Generations,
				archiveGeneration(t, feature,
					archiveReplacement(t, IntentArchiveArtifactSpec, "after-claim", IntentArchiveWireRetained),
				),
			)
			memory.externalSetIndex(t, current)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		if typed.ExitClass != 5 ||
			result.State != IntentArchivePurgeStateConsistent ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.PendingHash != replacement.ContentSHA256 {
			t.Fatalf("after-claim result = %+v error=%+v", result, typed)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("blob removed despite a newly inserted non-pending reference")
		}
	})

	t.Run("replacement-after-revalidation-is-disclosed-residual", func(t *testing.T) {
		const feature = "demo"
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "residual", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, replacement),
		))
		blobRel := storage.putRegular(feature, replacement.ContentSHA256, []byte("residual"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{replacement.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		storage.hooks["after-remove-revalidate"] = func(memory *archiveMemoryStorage) {
			delete(memory.hooks, "after-remove-revalidate")
			memory.blobs[blobRel] = archiveMemoryBlob{
				kind:    IntentArchiveBlobKindNonRegular,
				version: 99,
			}
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		if err != nil {
			t.Fatal(err)
		}
		if !result.RemovalRaceResidualDisclosed {
			t.Fatal("revalidate-to-unlink residual was not represented")
		}
		if _, exists := storage.blobs[blobRel]; exists {
			t.Fatal("replacement in disclosed unlink window survived")
		}
	})

	t.Run("index-stops-decoding-after-claim", func(t *testing.T) {
		const feature = "demo"
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "index-divergence", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, replacement),
		))
		blobRel := storage.putRegular(feature, replacement.ContentSHA256, []byte("index-divergence"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{replacement.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		storage.hooks["after-index-cas"] = func(memory *archiveMemoryStorage) {
			if fired {
				return
			}
			fired = true
			memory.index = []byte(`{"schema_version":1`)
			memory.indexVersion++
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgeEvidenceDivergent)
		if typed.ExitClass != 6 || !typed.Committed || !result.Committed {
			t.Fatalf("index divergence result=%+v error=%+v", result, typed)
		}
		if _, exists := storage.blobs[blobRel]; !exists {
			t.Fatal("index divergence removed blob evidence")
		}
		if !bytes.Equal(storage.index, []byte(`{"schema_version":1`)) {
			t.Fatal("index divergence evidence was rewritten")
		}
	})
}

func intentArchiveHashFromTestBlobPath(blobPath string) string {
	return strings.TrimSuffix(path.Base(blobPath), ".blob")
}

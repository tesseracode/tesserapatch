package store

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestNamedIntentArchiveInjectionSeamsReachBoundaries(t *testing.T) {
	oldBeforeBlobWrite := beforeBlobWrite
	oldAfterBlobWrite := afterBlobWrite
	oldBeforeBlobRemove := beforeBlobRemove
	oldAfterPurgeBlobRemove := afterPurgeBlobRemove
	oldBeforePendingTombstoneCAS := beforePendingTombstoneCAS
	oldFailPurgeAfterFirstMutation := failPurgeAfterFirstMutation
	oldBeforePurgeIndexCAS := beforePurgeIndexCAS
	oldAfterPurgeIndexRename := afterPurgeIndexRename
	oldBeforePurgeBlobRemove := beforePurgeBlobRemove
	oldAfterPurgeBlobRevalidate := afterPurgeBlobRevalidate
	oldFailPurgeBetweenHashes := failPurgeBetweenHashes
	oldFailOrphanRemoveAfterFirst := failOrphanRemoveAfterFirst
	t.Cleanup(func() {
		beforeBlobWrite = oldBeforeBlobWrite
		afterBlobWrite = oldAfterBlobWrite
		beforeBlobRemove = oldBeforeBlobRemove
		afterPurgeBlobRemove = oldAfterPurgeBlobRemove
		beforePendingTombstoneCAS = oldBeforePendingTombstoneCAS
		failPurgeAfterFirstMutation = oldFailPurgeAfterFirstMutation
		beforePurgeIndexCAS = oldBeforePurgeIndexCAS
		afterPurgeIndexRename = oldAfterPurgeIndexRename
		beforePurgeBlobRemove = oldBeforePurgeBlobRemove
		afterPurgeBlobRevalidate = oldAfterPurgeBlobRevalidate
		failPurgeBetweenHashes = oldFailPurgeBetweenHashes
		failOrphanRemoveAfterFirst = oldFailOrphanRemoveAfterFirst
	})

	counts := map[string]int{}
	beforeBlobWrite = func(path string) {
		requireArchiveSeamPath(t, path)
		counts["before-blob-write"]++
	}
	afterBlobWrite = func(path string) {
		requireArchiveSeamPath(t, path)
		counts["after-blob-write"]++
	}
	beforeBlobRemove = func(path string) {
		requireArchiveSeamPath(t, path)
		counts["before-blob-remove"]++
	}
	afterPurgeBlobRemove = func(path string) {
		requireArchiveSeamPath(t, path)
		counts["after-purge-blob-remove"]++
	}
	beforePendingTombstoneCAS = func(hash string) {
		requireArchiveSeamHash(t, hash)
		counts["before-pending-tombstone-cas"]++
	}
	beforePurgeIndexCAS = func(path string) {
		requireArchiveSeamPath(t, path)
		counts["before-purge-index-cas"]++
	}
	afterPurgeIndexRename = func(path string) {
		requireArchiveSeamPath(t, path)
		counts["after-purge-index-rename"]++
	}
	beforePurgeBlobRemove = func(path string) {
		requireArchiveSeamPath(t, path)
		counts["before-purge-blob-remove"]++
	}
	afterPurgeBlobRevalidate = func(path string) {
		requireArchiveSeamPath(t, path)
		counts["after-purge-blob-revalidate"]++
	}

	appendStorage := newEmptyArchiveMemoryStorage()
	appendPlan, err := PlanIntentArchiveAppend(appendStorage, "demo", []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("archive seam bytes"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishIntentArchiveBlobs(appendStorage, appendPlan); err != nil {
		t.Fatal(err)
	}
	if counts["before-blob-write"] != 1 || counts["after-blob-write"] != 1 {
		t.Fatalf("blob write seams = %#v", counts)
	}

	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "purge seam bytes", IntentArchiveWireRetained)
	purgeStorage := newArchiveMemoryStorage(t, archiveIndex(t, "demo", archiveGeneration(t, "demo", replacement)))
	purgeStorage.putRegular("demo", replacement.ContentSHA256, []byte("purge seam bytes"))
	purgePlan, err := PlanIntentArchivePurge(purgeStorage, "demo", IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ExecuteIntentArchivePurge(purgeStorage, purgePlan)
	if err != nil || !result.Committed {
		t.Fatalf("purge seam execution = result=%+v err=%v", result, err)
	}
	for _, name := range []string{
		"before-blob-remove",
		"after-purge-blob-remove",
		"before-pending-tombstone-cas",
		"before-purge-index-cas",
		"after-purge-index-rename",
		"before-purge-blob-remove",
		"after-purge-blob-revalidate",
	} {
		if counts[name] == 0 {
			t.Fatalf("%s was not reached: %#v", name, counts)
		}
	}

	failAfterFirstCalls := 0
	failPurgeAfterFirstMutation = func() error {
		failAfterFirstCalls++
		return errors.New("after first mutation")
	}
	replacement = archiveReplacement(t, IntentArchiveArtifactAnalysis, "first mutation", IntentArchiveWireRetained)
	purgeStorage = newArchiveMemoryStorage(t, archiveIndex(t, "demo", archiveGeneration(t, "demo", replacement)))
	purgeStorage.putRegular("demo", replacement.ContentSHA256, []byte("first mutation"))
	purgePlan, err = PlanIntentArchivePurge(purgeStorage, "demo", IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	result, err = ExecuteIntentArchivePurge(purgeStorage, purgePlan)
	if err == nil || !result.Committed || failAfterFirstCalls != 1 {
		t.Fatalf("failPurgeAfterFirstMutation = result=%+v calls=%d err=%v", result, failAfterFirstCalls, err)
	}

	failPurgeAfterFirstMutation = nil
	betweenCalls := 0
	failPurgeBetweenHashes = func() error {
		betweenCalls++
		return errors.New("between hashes")
	}
	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "between one", IntentArchiveWireRetained)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "between two", IntentArchiveWireRetained)
	purgeStorage = newArchiveMemoryStorage(t, archiveIndex(t, "demo", archiveGeneration(t, "demo", first, second)))
	purgeStorage.putRegular("demo", first.ContentSHA256, []byte("between one"))
	purgeStorage.putRegular("demo", second.ContentSHA256, []byte("between two"))
	purgePlan, err = PlanIntentArchivePurge(purgeStorage, "demo", IntentArchivePurgeSelector{All: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	result, err = ExecuteIntentArchivePurge(purgeStorage, purgePlan)
	if err == nil || !result.Committed || betweenCalls != 1 {
		t.Fatalf("failPurgeBetweenHashes = result=%+v calls=%d err=%v", result, betweenCalls, err)
	}

	failPurgeBetweenHashes = nil
	orphanCalls := 0
	failOrphanRemoveAfterFirst = func() error {
		orphanCalls++
		return errors.New("after first orphan")
	}
	purgeStorage = newArchiveMemoryStorage(t, archiveIndex(t, "demo"))
	purgeStorage.putRegular("demo", archiveHash("orphan one"), []byte("orphan one"))
	purgeStorage.putRegular("demo", archiveHash("orphan two"), []byte("orphan two"))
	purgePlan, err = PlanIntentArchivePurge(purgeStorage, "demo", IntentArchivePurgeSelector{Orphans: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	result, err = ExecuteIntentArchivePurge(purgeStorage, purgePlan)
	if err == nil || !result.Committed || orphanCalls != 1 {
		t.Fatalf("failOrphanRemoveAfterFirst = result=%+v calls=%d err=%v", result, orphanCalls, err)
	}
}

func TestPurgeFailureSeamsOwnOnlyTheirPartialBranches(t *testing.T) {
	const feature = "demo"

	t.Run("removal-pending-claim", func(t *testing.T) {
		counts := installPurgeFailureSeams(t)
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "claim branch", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature, archiveGeneration(t, feature, replacement)))
		storage.putRegular(feature, replacement.ContentSHA256, []byte("claim branch"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{replacement.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		if err == nil || counts.claim != 1 || counts.between != 0 || counts.orphan != 0 ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.PendingHash != replacement.ContentSHA256 ||
			!intentArchiveHashHasState(storage.decodedIndex(t, feature), replacement.ContentSHA256, IntentArchiveWireRemovalPending) {
			t.Fatalf("claim branch = result=%+v counts=%+v err=%v", result, counts, err)
		}
	})

	t.Run("orphan-removal", func(t *testing.T) {
		counts := installPurgeFailureSeams(t)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature))
		first := archiveHash("owned orphan first")
		second := archiveHash("owned orphan second")
		storage.putRegular(feature, first, []byte("owned orphan first"))
		storage.putRegular(feature, second, []byte("owned orphan second"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{Orphans: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		if err == nil || counts.claim != 0 || counts.between != 0 || counts.orphan != 1 ||
			result.Resume != IntentArchiveResumeOrphanScan || result.PendingHash != "" ||
			len(PendingIntentArchiveHashes(storage.decodedIndex(t, feature))) != 0 {
			t.Fatalf("orphan branch = result=%+v counts=%+v err=%v", result, counts, err)
		}
	})

	t.Run("unreferenced-direct-removal", func(t *testing.T) {
		counts := installPurgeFailureSeams(t)
		first := archiveHash("unreferenced first")
		second := archiveHash("unreferenced second")
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature))
		storage.putRegular(feature, first, []byte("unreferenced first"))
		storage.putRegular(feature, second, []byte("unreferenced second"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		if err == nil || counts.claim != 0 || counts.between != 1 || counts.orphan != 0 ||
			result.Resume != IntentArchiveResumeCompletionOnly || result.PendingHash != "" ||
			len(PendingIntentArchiveHashes(storage.decodedIndex(t, feature))) != 0 {
			t.Fatalf("unreferenced branch = result=%+v counts=%+v err=%v", result, counts, err)
		}
	})

	t.Run("absent-blob-direct-tombstone", func(t *testing.T) {
		counts := installPurgeFailureSeams(t)
		first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "absent first", IntentArchiveWireRetained)
		second := archiveReplacement(t, IntentArchiveArtifactSpec, "absent second", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, first),
			archiveGeneration(t, feature, second),
		))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		if err == nil || counts.claim != 0 || counts.between != 1 || counts.orphan != 0 ||
			result.Resume != IntentArchiveResumeCompletionOnly || result.PendingHash != "" ||
			len(PendingIntentArchiveHashes(storage.decodedIndex(t, feature))) != 0 {
			t.Fatalf("absent branch = result=%+v counts=%+v err=%v", result, counts, err)
		}
	})
}

func TestAfterPurgeIndexRenameFiresForEveryCommittedCASResult(t *testing.T) {
	oldBefore := beforePurgeIndexCAS
	oldAfter := afterPurgeIndexRename
	t.Cleanup(func() {
		beforePurgeIndexCAS = oldBefore
		afterPurgeIndexRename = oldAfter
	})

	currentIndex := archiveIndex(t, "demo")
	currentStorage := newArchiveMemoryStorage(t, currentIndex)
	current, err := currentStorage.CaptureIndex("unused")
	if err != nil {
		t.Fatal(err)
	}
	next := archiveIndex(t, "demo", archiveGeneration(t, "demo",
		archiveReplacement(t, IntentArchiveArtifactAnalysis, "next", IntentArchiveWireRetained),
	))

	t.Run("committed-post-rename-error", func(t *testing.T) {
		events := []string{}
		beforePurgeIndexCAS = func(string) { events = append(events, "before") }
		afterPurgeIndexRename = func(string) { events = append(events, "after") }
		storage := &casFaultStorage{
			archiveMemoryStorage: newArchiveMemoryStorage(t, currentIndex),
			events:               &events,
			committedError:       errors.New("post-rename sync failure"),
		}
		_, mutation, err := publishIntentArchiveIndex(storage, "demo", current, next)
		var typed *IntentArchiveError
		if !errors.As(err, &typed) || typed.ExitClass != 5 || !typed.Committed ||
			!mutation.Committed || !reflect.DeepEqual(events, []string{"before", "cas", "after"}) {
			t.Fatalf("committed CAS = mutation=%+v events=%v err=%v", mutation, events, err)
		}
	})

	t.Run("pre-commit-error", func(t *testing.T) {
		events := []string{}
		beforePurgeIndexCAS = func(string) { events = append(events, "before") }
		afterPurgeIndexRename = func(string) { events = append(events, "after") }
		storage := &casFaultStorage{
			archiveMemoryStorage: newArchiveMemoryStorage(t, currentIndex),
			events:               &events,
			preCommitError:       errors.New("pre-commit CAS failure"),
		}
		_, mutation, err := publishIntentArchiveIndex(storage, "demo", current, next)
		var typed *IntentArchiveError
		if !errors.As(err, &typed) || typed.ExitClass != 3 || typed.Committed ||
			mutation.Committed || !reflect.DeepEqual(events, []string{"before", "cas"}) {
			t.Fatalf("pre-commit CAS = mutation=%+v events=%v err=%v", mutation, events, err)
		}
	})
}

func TestFailPurgeBetweenHashesRequiresCurrentHashMutation(t *testing.T) {
	const feature = "demo"

	t.Run("unmutated-first-mutated-last", func(t *testing.T) {
		first, second := orderedArchiveReplacements(
			t, IntentArchiveWireTombstoned, IntentArchiveWireRetained,
		)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, first),
			archiveGeneration(t, feature, second),
		))
		calls := installBetweenHashesFailure(t)
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		final := storage.decodedIndex(t, feature)
		if err != nil || *calls != 0 || !result.Committed ||
			!intentArchiveHashHasState(final, first.ContentSHA256, IntentArchiveWireTombstoned) ||
			!intentArchiveHashHasState(final, second.ContentSHA256, IntentArchiveWireTombstoned) {
			t.Fatalf("unmutated first = result=%+v calls=%d final=%+v err=%v", result, *calls, final, err)
		}
	})

	t.Run("mutated-first-with-remaining", func(t *testing.T) {
		first, second := orderedArchiveReplacements(
			t, IntentArchiveWireRetained, IntentArchiveWireRetained,
		)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, first),
			archiveGeneration(t, feature, second),
		))
		calls := installBetweenHashesFailure(t)
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		final := storage.decodedIndex(t, feature)
		if err == nil || *calls != 1 ||
			result.Resume != IntentArchiveResumeCompletionOnly || result.PendingHash != "" ||
			!intentArchiveHashHasState(final, first.ContentSHA256, IntentArchiveWireTombstoned) ||
			!intentArchiveHashHasState(final, second.ContentSHA256, IntentArchiveWireRetained) {
			t.Fatalf("mutated first = result=%+v calls=%d final=%+v err=%v", result, *calls, final, err)
		}
	})

	t.Run("last-mutated-hash", func(t *testing.T) {
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "last mutated", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, replacement),
		))
		calls := installBetweenHashesFailure(t)
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		if err != nil || *calls != 0 || !result.Committed {
			t.Fatalf("last mutation = result=%+v calls=%d err=%v", result, *calls, err)
		}
	})
}

func TestAppendFailureNormalizationPreservesCommittedExitSix(t *testing.T) {
	const feature = "demo"
	input := IntentArchiveReplacementInput{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("committed exit six"),
	}

	t.Run("committed-post-publication-divergence", func(t *testing.T) {
		base := newEmptyArchiveMemoryStorage()
		storage := &appendPublishFaultStorage{
			archiveMemoryStorage: base,
			committedError: &IntentArchiveError{
				Code:      IntentArchiveCodeStorageFailed,
				Class:     "post-publication-divergence:sync-directory",
				Detail:    "sanitized committed failure",
				ExitClass: 6,
				Committed: true,
			},
		}
		plan, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{input})
		if err != nil {
			t.Fatal(err)
		}
		result, err := PublishIntentArchiveBlobs(storage, plan)
		var typed *IntentArchiveError
		if !errors.As(err, &typed) ||
			typed.Code != IntentArchiveCodeStorageFailed ||
			typed.Class != "post-publication-divergence:sync-directory" ||
			typed.ExitClass != 6 || !typed.Committed ||
			!result.Committed ||
			result.Phase != IntentArchiveStoragePhaseRenamed ||
			len(result.NewOrphanHashes) != 1 ||
			result.NewOrphanHashes[0] != archiveHash("committed exit six") {
			t.Fatalf("committed append failure = result=%+v err=%#v", result, err)
		}
	})

	t.Run("precommit-storage-failure", func(t *testing.T) {
		base := newEmptyArchiveMemoryStorage()
		storage := &appendPublishFaultStorage{
			archiveMemoryStorage: base,
			preCommitError: &IntentArchiveError{
				Code:      IntentArchiveCodeStorageFailed,
				Class:     "publish-blob",
				Detail:    "sanitized precommit failure",
				ExitClass: 3,
			},
		}
		plan, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{input})
		if err != nil {
			t.Fatal(err)
		}
		result, err := PublishIntentArchiveBlobs(storage, plan)
		var typed *IntentArchiveError
		if !errors.As(err, &typed) || typed.ExitClass != 3 || typed.Committed ||
			result.Committed || len(result.NewOrphanHashes) != 0 {
			t.Fatalf("precommit append failure = result=%+v err=%#v", result, err)
		}
	})
}

type appendPublishFaultStorage struct {
	*archiveMemoryStorage
	preCommitError error
	committedError error
}

func (storage *appendPublishFaultStorage) PublishBlob(
	blobRel, contentSHA256 string,
	data []byte,
) (IntentArchiveMutationResult, error) {
	if storage.preCommitError != nil {
		return IntentArchiveMutationResult{}, storage.preCommitError
	}
	mutation, err := storage.archiveMemoryStorage.PublishBlob(blobRel, contentSHA256, data)
	if err != nil {
		return mutation, err
	}
	if storage.committedError != nil {
		return mutation, storage.committedError
	}
	return mutation, nil
}

func orderedArchiveReplacements(
	t *testing.T,
	firstState IntentArchiveWireState,
	secondState IntentArchiveWireState,
) (IntentArchiveReplacement, IntentArchiveReplacement) {
	t.Helper()
	for firstIndex := 0; firstIndex < 32; firstIndex++ {
		first := archiveReplacement(
			t,
			IntentArchiveArtifactAnalysis,
			fmt.Sprintf("ordered first %d", firstIndex),
			firstState,
		)
		for secondIndex := 0; secondIndex < 32; secondIndex++ {
			second := archiveReplacement(
				t,
				IntentArchiveArtifactSpec,
				fmt.Sprintf("ordered second %d", secondIndex),
				secondState,
			)
			if first.ContentSHA256 < second.ContentSHA256 {
				return first, second
			}
		}
	}
	t.Fatal("could not construct ordered archive hashes")
	return IntentArchiveReplacement{}, IntentArchiveReplacement{}
}

func installBetweenHashesFailure(t *testing.T) *int {
	t.Helper()
	old := failPurgeBetweenHashes
	t.Cleanup(func() { failPurgeBetweenHashes = old })
	calls := 0
	failPurgeBetweenHashes = func() error {
		calls++
		return errors.New("between hashes")
	}
	return &calls
}

type purgeFailureCounts struct {
	claim   int
	between int
	orphan  int
}

func installPurgeFailureSeams(t *testing.T) *purgeFailureCounts {
	t.Helper()
	oldClaim := failPurgeAfterFirstMutation
	oldBetween := failPurgeBetweenHashes
	oldOrphan := failOrphanRemoveAfterFirst
	t.Cleanup(func() {
		failPurgeAfterFirstMutation = oldClaim
		failPurgeBetweenHashes = oldBetween
		failOrphanRemoveAfterFirst = oldOrphan
	})
	counts := &purgeFailureCounts{}
	failPurgeAfterFirstMutation = func() error {
		counts.claim++
		return errors.New("claim seam")
	}
	failPurgeBetweenHashes = func() error {
		counts.between++
		return errors.New("between seam")
	}
	failOrphanRemoveAfterFirst = func() error {
		counts.orphan++
		return errors.New("orphan seam")
	}
	return counts
}

type casFaultStorage struct {
	*archiveMemoryStorage
	events         *[]string
	preCommitError error
	committedError error
}

func (storage *casFaultStorage) CASIndex(
	indexRel string,
	expected IntentArchiveIdentityToken,
	canonical []byte,
) (IntentArchiveMutationResult, error) {
	*storage.events = append(*storage.events, "cas")
	if storage.preCommitError != nil {
		return IntentArchiveMutationResult{}, storage.preCommitError
	}
	mutation, err := storage.archiveMemoryStorage.CASIndex(indexRel, expected, canonical)
	if err != nil {
		return mutation, err
	}
	if storage.committedError != nil {
		return mutation, storage.committedError
	}
	return mutation, nil
}

func requireArchiveSeamPath(t *testing.T, path string) {
	t.Helper()
	if path == "" {
		t.Fatal("archive injection seam received an empty path")
	}
}

func requireArchiveSeamHash(t *testing.T, hash string) {
	t.Helper()
	if !validIntentArchiveHash(hash) {
		t.Fatalf("archive injection seam received invalid hash %q", hash)
	}
}

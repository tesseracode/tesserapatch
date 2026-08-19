package store

import (
	"fmt"
	"testing"
)

func TestIntentArchiveTupleClassifierTotality(t *testing.T) {
	wireStates := []IntentArchiveWireState{
		IntentArchiveWireRetained,
		IntentArchiveWireRemovalPending,
		IntentArchiveWireTombstoned,
	}
	blobStates := []IntentArchiveBlobState{
		IntentArchiveBlobAbsent,
		IntentArchiveBlobPresentCorrect,
		IntentArchiveBlobUnidentifiable,
	}
	reachable := 0
	unreachable := 0
	seen := map[string]IntentArchiveTupleResult{}
	for _, wire := range wireStates {
		for _, blob := range blobStates {
			for _, owned := range []bool{false, true} {
				for _, live := range []bool{false, true} {
					tuple := IntentArchiveTuple{
						WireState: wire,
						BlobState: blob,
						Owned:     owned,
						Live:      live,
					}
					result := ClassifyIntentArchiveTuple(tuple)
					key := fmt.Sprintf("%s|%s|%t|%t", wire, blob, owned, live)
					if IntentArchiveTupleReachable(tuple) {
						reachable++
						if !result.Reachable || result.Disposition == "" || result.Action == "" {
							t.Fatalf("reachable tuple %s has incomplete result: %+v", key, result)
						}
						if _, duplicate := seen[key]; duplicate {
							t.Fatalf("tuple %s classified twice", key)
						}
						seen[key] = result
					} else {
						unreachable++
						if result.Reachable || result.Disposition != "" || result.Action != "" {
							t.Fatalf("unreachable tuple %s classified: %+v", key, result)
						}
					}
				}
			}
		}
	}
	if reachable != 18 || unreachable != 18 || len(seen) != 18 {
		t.Fatalf("reachable=%d unreachable=%d classified=%d, want 18/18/18", reachable, unreachable, len(seen))
	}

	expect := map[string]struct {
		disposition IntentArchiveDisposition
		action      IntentArchiveAction
		code        IntentArchiveErrorCode
		class       IntentArchiveRepairClass
		exit        int
	}{
		"retained|present-regular-hash-correct|false|true":       {IntentArchiveDispositionHealthyRetained, IntentArchiveActionNone, "", "", 0},
		"retained|present-regular-hash-correct|true|true":        {IntentArchiveDispositionPendingRemove, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"retained|absent|false|true":                             {IntentArchiveDispositionDanglingReference, IntentArchiveActionPurgeHash, IntentArchiveCodeBlobDangling, IntentArchiveRepairDanglingReference, 3},
		"retained|absent|true|true":                              {IntentArchiveDispositionPendingFinalize, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"retained|present-unidentifiable|false|true":             {IntentArchiveDispositionCorruptObject, IntentArchiveActionRemoveCorruptObject, IntentArchiveCodeBlobCorrupt, IntentArchiveRepairCorruptObject, 3},
		"retained|present-unidentifiable|true|true":              {IntentArchiveDispositionPendingRemove, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"removal-pending|present-regular-hash-correct|true|true": {IntentArchiveDispositionPendingRemove, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"removal-pending|absent|true|true":                       {IntentArchiveDispositionPendingFinalize, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"removal-pending|present-unidentifiable|true|true":       {IntentArchiveDispositionPendingRemove, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"tombstoned|absent|false|false":                          {IntentArchiveDispositionHealthyPurged, IntentArchiveActionNone, "", "", 0},
		"tombstoned|absent|false|true":                           {IntentArchiveDispositionDanglingReference, IntentArchiveActionPurgeHash, IntentArchiveCodeBlobDangling, IntentArchiveRepairDanglingReference, 3},
		"tombstoned|absent|true|true":                            {IntentArchiveDispositionPendingFinalize, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"tombstoned|present-regular-hash-correct|true|true":      {IntentArchiveDispositionPendingRemove, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"tombstoned|present-unidentifiable|true|true":            {IntentArchiveDispositionPendingRemove, IntentArchiveActionRoutePendingOwner, "", "", 0},
		"tombstoned|present-regular-hash-correct|false|false":    {IntentArchiveDispositionResidue, IntentArchiveActionPurgeOrphans, IntentArchiveCodeIndexStorageInconsistent, IntentArchiveRepairUnreferencedResidue, 3},
		"tombstoned|present-regular-hash-correct|false|true":     {IntentArchiveDispositionMixedReference, IntentArchiveActionPurgeHash, IntentArchiveCodeIndexStorageInconsistent, IntentArchiveRepairMixedReference, 3},
		"tombstoned|present-unidentifiable|false|false":          {IntentArchiveDispositionCorruptObject, IntentArchiveActionRemoveCorruptObject, IntentArchiveCodeBlobCorrupt, IntentArchiveRepairCorruptObject, 3},
		"tombstoned|present-unidentifiable|false|true":           {IntentArchiveDispositionCorruptObject, IntentArchiveActionRemoveCorruptObject, IntentArchiveCodeBlobCorrupt, IntentArchiveRepairCorruptObject, 3},
	}
	for key, want := range expect {
		got, ok := seen[key]
		if !ok {
			t.Fatalf("missing reachable tuple %s", key)
		}
		if got.Disposition != want.disposition ||
			got.Action != want.action ||
			got.Code != want.code ||
			got.RepairClass != want.class ||
			got.ExitClass != want.exit {
			t.Fatalf("%s = %+v, want disposition=%s action=%s code=%s class=%s exit=%d", key, got, want.disposition, want.action, want.code, want.class, want.exit)
		}
	}
}

func TestInspectIntentArchiveGlobalSameHashOwnershipAndLiveness(t *testing.T) {
	const feature = "demo"
	hash := archiveHash("shared")
	retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireRetained)
	tombstoned := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireTombstoned)
	index := archiveIndex(t, feature,
		archiveGeneration(t, feature, retained),
		archiveGeneration(t, feature, tombstoned),
	)
	report, err := InspectIntentArchive(index, []IntentArchiveBlobObservation{
		archiveObservation(feature, hash, IntentArchiveBlobPresentCorrect, int64(len("shared"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Classes) != 1 || report.Classes[0].Class != IntentArchiveRepairMixedReference {
		t.Fatalf("classes = %+v, want one mixed-reference class", report.Classes)
	}
	if len(report.Orphans) != 0 {
		t.Fatalf("live shared hash classified as orphan: %+v", report.Orphans)
	}
	if !report.Hashes[0].Live || report.Hashes[0].Owned || report.Hashes[0].Unreferenced {
		t.Fatalf("global facts = %+v", report.Hashes[0])
	}

	pending := tombstoned
	setIntentArchiveReplacementState(&pending, IntentArchiveWireRemovalPending)
	ownedIndex := archiveIndex(t, feature,
		archiveGeneration(t, feature, retained),
		archiveGeneration(t, feature, pending),
	)
	owned, err := InspectIntentArchive(ownedIndex, []IntentArchiveBlobObservation{
		archiveObservation(feature, hash, IntentArchiveBlobPresentCorrect, int64(len("shared"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(owned.PendingHashes) != 1 || owned.PendingHashes[0] != hash {
		t.Fatalf("pending hashes = %v", owned.PendingHashes)
	}
	if len(owned.Classes) != 0 {
		t.Fatalf("owned hash leaked into a repair class: %+v", owned.Classes)
	}
	for _, reference := range owned.References {
		if reference.Disposition != IntentArchiveDispositionPendingRemove {
			t.Fatalf("owned reference disposition = %q, want pending-remove", reference.Disposition)
		}
	}
}

func TestInspectIntentArchiveOrphansCorruptPrecedenceAndStableClasses(t *testing.T) {
	const feature = "demo"
	residueHash := archiveHash("residue")
	danglingHash := archiveHash("dangling")
	mixedHash := archiveHash("mixed")
	unindexedHash := archiveHash("unindexed")
	corruptHash := archiveHash("corrupt")
	index := archiveIndex(t, feature,
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactAnalysis, "residue", IntentArchiveWireTombstoned),
			archiveReplacement(t, IntentArchiveArtifactSpec, "dangling", IntentArchiveWireRetained),
		),
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactExploration, "mixed", IntentArchiveWireRetained),
		),
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactSpec, "mixed", IntentArchiveWireTombstoned),
		),
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactAnalysisSidecar, "corrupt", IntentArchiveWireRetained),
		),
	)
	blobsDir, _ := IntentArchiveBlobsRel(feature)
	report, err := InspectIntentArchive(index, []IntentArchiveBlobObservation{
		archiveObservation(feature, residueHash, IntentArchiveBlobPresentCorrect, int64(len("residue"))),
		archiveObservation(feature, danglingHash, IntentArchiveBlobAbsent, 0),
		archiveObservation(feature, mixedHash, IntentArchiveBlobPresentCorrect, int64(len("mixed"))),
		archiveObservation(feature, unindexedHash, IntentArchiveBlobPresentCorrect, int64(len("unindexed"))),
		archiveObservation(feature, corruptHash, IntentArchiveBlobUnidentifiable, 0),
		{Path: blobsDir + "/not-a-blob", State: IntentArchiveBlobUnidentifiable},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantClasses := []IntentArchiveRepairClass{
		IntentArchiveRepairCorruptObject,
		IntentArchiveRepairDanglingReference,
		IntentArchiveRepairMixedReference,
		IntentArchiveRepairUnreferencedResidue,
	}
	if len(report.Classes) != len(wantClasses) {
		t.Fatalf("classes = %+v", report.Classes)
	}
	for index, want := range wantClasses {
		if report.Classes[index].Class != want || report.Classes[index].Rank != index+1 {
			t.Fatalf("class[%d] = %+v, want %s rank %d", index, report.Classes[index], want, index+1)
		}
	}
	if !report.Classes[0].Blocking {
		t.Fatal("corrupt-object class is not blocking")
	}
	if len(report.Orphans) != 2 {
		t.Fatalf("orphans = %+v, want residue and unindexed hashes", report.Orphans)
	}
	orphanHashes := []string{report.Orphans[0].Hash, report.Orphans[1].Hash}
	if !equalStringSets(orphanHashes, []string{residueHash, unindexedHash}) {
		t.Fatalf("orphans = %+v, want residue and unindexed hashes", report.Orphans)
	}
	for _, class := range report.Classes {
		if class.Class == IntentArchiveRepairMixedReference && len(class.Hashes) == 1 && class.Hashes[0] == corruptHash {
			t.Fatal("corrupt observation also appeared in mixed-reference")
		}
	}
}

func TestOwnedUnidentifiableReadersRouteToOwnerWithoutExitSix(t *testing.T) {
	const feature = "demo"
	hash := archiveHash("owned")
	index := archiveIndex(t, feature,
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactAnalysis, "owned", IntentArchiveWireRetained),
		),
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactSpec, "owned", IntentArchiveWireRemovalPending),
		),
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactExploration, "owned", IntentArchiveWireTombstoned),
		),
	)
	report, err := InspectIntentArchive(index, []IntentArchiveBlobObservation{
		archiveObservation(feature, hash, IntentArchiveBlobUnidentifiable, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Classes) != 0 || len(report.PendingHashes) != 1 {
		t.Fatalf("owned corrupt report = %+v", report)
	}
	for _, reference := range report.References {
		if reference.Code != "" ||
			reference.Disposition != IntentArchiveDispositionPendingRemove ||
			reference.Action != IntentArchiveActionRoutePendingOwner {
			t.Fatalf("owned unidentifiable route = %+v", reference)
		}
	}
}

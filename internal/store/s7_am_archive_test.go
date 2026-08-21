package store

import (
	"bytes"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestS7PIB401PurgedGenerationRetainsAndValidatesImmutableIdentity(t *testing.T) {
	const feature = "demo"
	setup := s7PIB401BuildPurgedIdentity(t, feature)
	t.Run("purged-identity-roundtrip", func(t *testing.T) {
		if len(setup.wire) == 0 ||
			setup.replacement.WireState() != IntentArchiveWireTombstoned {
			t.Fatalf("purged identity setup is incomplete: %+v", setup.replacement)
		}
	})

	cases := []struct {
		name string
		edit func(IntentArchiveIndex) IntentArchiveIndex
		code IntentArchiveErrorCode
	}{
		{
			name: "missing-content-digest",
			edit: func(value IntentArchiveIndex) IntentArchiveIndex {
				value.Generations[0].Replaced[0].ContentSHA256 = ""
				return value
			},
			code: IntentArchiveCodeIndexCorrupt,
		},
		{
			name: "mismatched-content-digest",
			edit: func(value IntentArchiveIndex) IntentArchiveIndex {
				value.Generations[0].Replaced[0].ContentSHA256 = strings.Repeat("a", 64)
				return value
			},
			code: IntentArchiveCodeGenerationMismatch,
		},
		{
			name: "blob-purged-inconsistency",
			edit: func(value IntentArchiveIndex) IntentArchiveIndex {
				value.Generations[0].Replaced[0].Blob = "blobs/" + setup.replacement.ContentSHA256 + ".blob"
				return value
			},
			code: IntentArchiveCodeIndexCorrupt,
		},
		{
			name: "altered-immutable-path",
			edit: func(value IntentArchiveIndex) IntentArchiveIndex {
				value.Generations[0].Replaced[0].Path = "spec.md"
				return value
			},
			code: IntentArchiveCodeIndexPathEscape,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			baseline, err := DecodeIntentArchiveIndex(setup.wire, feature)
			if err != nil {
				t.Fatal(err)
			}
			altered := test.edit(baseline)
			alteredWire, err := EncodeIntentArchiveIndex(altered)
			if err == nil {
				_, err = DecodeIntentArchiveIndex(alteredWire, feature)
			}
			assertArchiveCode(t, err, test.code)
		})
	}
}

type s7PIB401PurgedIdentity struct {
	wire        []byte
	replacement IntentArchiveReplacement
}

func s7PIB401BuildPurgedIdentity(t *testing.T, feature string) s7PIB401PurgedIdentity {
	t.Helper()
	retained := archiveReplacement(
		t, IntentArchiveArtifactAnalysis, "prior analysis", IntentArchiveWireRetained,
	)
	storage := newArchiveMemoryStorage(
		t, archiveIndex(t, feature, archiveGeneration(t, feature, retained)),
	)
	storage.putRegular(feature, retained.ContentSHA256, []byte("prior analysis"))
	purge, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{retained.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExecuteIntentArchivePurge(storage, purge); err != nil {
		t.Fatal(err)
	}
	index := storage.decodedIndex(t, feature)
	replacement := index.Generations[0].Replaced[0]
	if replacement.WireState() != IntentArchiveWireTombstoned {
		t.Fatalf("confirmed purge did not create a tombstone: %+v", replacement)
	}
	wire, err := EncodeIntentArchiveIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeIntentArchiveIndex(wire, feature)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Generations[0].GenerationID != index.Generations[0].GenerationID ||
		decoded.Generations[0].Replaced[0].ContentSHA256 != replacement.ContentSHA256 {
		t.Fatalf("purged generation identity changed: before=%+v after=%+v", index, decoded)
	}
	return s7PIB401PurgedIdentity{wire: wire, replacement: replacement}
}

func TestS7PIB402TombstoneRehydrateAndPendingOwnershipConflict(t *testing.T) {
	const feature = "demo"
	tombstoned := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireTombstoned)
	secondTombstone := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireTombstoned)
	initial := archiveIndex(t, feature,
		archiveGeneration(t, feature, tombstoned),
		archiveGeneration(t, feature, secondTombstone),
	)
	storage := newArchiveMemoryStorage(t, initial)
	plan, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("shared"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Outcome() != IntentArchiveAppendRehydrate ||
		len(plan.Blobs()) != 1 ||
		len(plan.Blobs()[0].ArtifactIDs) != 2 {
		t.Fatalf("rehydration plan = outcome:%q blobs:%+v", plan.Outcome(), plan.Blobs())
	}
	storage.calls = nil
	result, err := PublishIntentArchiveBlobs(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	indexRel, err := IntentArchiveIndexRel(feature)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CASIndex(indexRel, plan.IndexPreimage().Identity, plan.IndexBytes()); err != nil {
		t.Fatal(err)
	}
	publish := callIndex(storage.calls, "publish:")
	cas := callIndex(storage.calls, "cas-index")
	if publish < 0 || cas <= publish || strings.Count(strings.Join(storage.calls, "\n"), "cas-index") != 1 ||
		!result.Committed || result.Outcome != IntentArchiveAppendRehydrate {
		t.Fatalf("rehydration publication order/result = calls:%v result:%+v", storage.calls, result)
	}
	final := storage.decodedIndex(t, feature)
	if len(final.Generations) != len(initial.Generations) {
		t.Fatalf("rehydration appended a duplicate generation: before=%d after=%d", len(initial.Generations), len(final.Generations))
	}
	revived := 0
	for _, generation := range final.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == tombstoned.ContentSHA256 {
				revived++
				if replacement.WireState() != IntentArchiveWireRetained {
					t.Fatalf("same-hash reference was not globally revived: %+v", replacement)
				}
			}
		}
	}
	report, err := InspectIntentArchive(final, []IntentArchiveBlobObservation{
		archiveObservation(feature, tombstoned.ContentSHA256, IntentArchiveBlobPresentCorrect, int64(len("shared"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if revived != 2 || !report.Consistent || len(report.Orphans) != 0 {
		t.Fatalf("rehydration left tombstone/orphan state: revived=%d report=%+v", revived, report)
	}

	pending := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireRemovalPending)
	pendingStorage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, tombstoned),
		archiveGeneration(t, feature, pending),
	))
	pendingStorage.putRegular(feature, pending.ContentSHA256, []byte("shared"))
	pendingStorage.calls = nil
	_, err = PlanIntentArchiveAppend(pendingStorage, feature, []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("shared"),
	}})
	typed := assertArchiveCode(t, err, IntentArchiveCodeRecoveryPending)
	if typed.ExitClass != 3 || len(pendingStorage.mutationCalls()) != 0 {
		t.Fatalf("globally pending hash bypassed recovery precedence: err=%+v calls=%v", typed, pendingStorage.calls)
	}
}

func TestS7PIB403RepeatedPurgeRehydratePreservesIDsOrderAndReferenceCount(t *testing.T) {
	const feature = "demo"
	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireRetained)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireRetained)
	initial := archiveIndex(t, feature,
		archiveGeneration(t, feature, first),
		archiveGeneration(t, feature, second),
	)
	storage := newArchiveMemoryStorage(t, initial)
	storage.putRegular(feature, first.ContentSHA256, []byte("shared"))
	selector := IntentArchivePurgeSelector{Blobs: []string{first.ContentSHA256}}

	firstPurge, err := PlanIntentArchivePurge(storage, feature, selector, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(firstPurge.References) != 2 {
		t.Fatalf("initial live reference count = %d, want 2", len(firstPurge.References))
	}
	if _, err := ExecuteIntentArchivePurge(storage, firstPurge); err != nil {
		t.Fatal(err)
	}

	rehydrate, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("shared"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishIntentArchiveBlobs(storage, rehydrate); err != nil {
		t.Fatal(err)
	}
	indexRel, err := IntentArchiveIndexRel(feature)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CASIndex(indexRel, rehydrate.IndexPreimage().Identity, rehydrate.IndexBytes()); err != nil {
		t.Fatal(err)
	}
	rehydrated := storage.decodedIndex(t, feature)
	if !reflect.DeepEqual(s7ArchiveImmutableProjection(initial), s7ArchiveImmutableProjection(rehydrated)) {
		t.Fatalf("purge/rehydrate changed generation IDs or lexical order\ninitial=%+v\nrehydrated=%+v", initial, rehydrated)
	}
	secondPurge, err := PlanIntentArchivePurge(storage, feature, selector, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPurge.Hashes) != 1 || len(secondPurge.References) != 2 {
		t.Fatalf("later purge selected hashes=%v references=%+v, want one hash/two live refs", secondPurge.Hashes, secondPurge.References)
	}
	if _, err := ExecuteIntentArchivePurge(storage, secondPurge); err != nil {
		t.Fatal(err)
	}
	if !intentArchiveHashAllTombstoned(storage.decodedIndex(t, feature), first.ContentSHA256) {
		t.Fatal("later purge did not count and tombstone every revived reference")
	}
}

func TestS7PIB404RehydrateRedactionAndCP13ResidueClassification(t *testing.T) {
	const feature = "demo"
	t.Run("redaction-preserves-tombstone-and-absent-blob", func(t *testing.T) {
		tombstone := archiveReplacement(t, IntentArchiveArtifactAnalysis, "prior", IntentArchiveWireTombstoned)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, tombstone),
		))
		before := append([]byte(nil), storage.index...)
		storage.calls = nil
		_, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("-----BEGIN PRIVATE KEY-----"),
		}})
		typed := assertArchiveCode(t, err, IntentArchiveCodeContentSensitive)
		if typed.ExitClass != 3 ||
			len(storage.mutationCalls()) != 0 ||
			len(storage.blobs) != 0 ||
			!bytes.Equal(storage.index, before) {
			t.Fatalf("redaction refusal changed tombstone/blob state: err=%+v calls=%v", typed, storage.calls)
		}
	})

	t.Run("unreferenced-cp13-blob-routes-to-orphans", func(t *testing.T) {
		tombstone := archiveReplacement(t, IntentArchiveArtifactAnalysis, "residue", IntentArchiveWireTombstoned)
		index := archiveIndex(t, feature, archiveGeneration(t, feature, tombstone))
		storage := newArchiveMemoryStorage(t, index)
		plan, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("residue"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := PublishIntentArchiveBlobs(storage, plan); err != nil {
			t.Fatal(err)
		}
		indexRel, err := IntentArchiveIndexRel(feature)
		if err != nil {
			t.Fatal(err)
		}
		storage.fail["cas-index"] = 1
		if _, err := storage.CASIndex(indexRel, plan.IndexPreimage().Identity, plan.IndexBytes()); err == nil {
			t.Fatal("CP13 pre-index-rename failure was not injected")
		}
		if !intentArchiveHashHasState(storage.decodedIndex(t, feature), tombstone.ContentSHA256, IntentArchiveWireTombstoned) {
			t.Fatal("CP13 changed the tombstoned index")
		}
		snapshot, err := CaptureIntentArchive(storage, feature)
		if err != nil {
			t.Fatal(err)
		}
		report := snapshot.Inspection
		if report.Consistent || len(report.Classes) != 1 ||
			report.Classes[0].Class != IntentArchiveRepairUnreferencedResidue ||
			len(report.Orphans) != 1 ||
			report.Orphans[0].Hash != tombstone.ContentSHA256 {
			t.Fatalf("unreferenced CP13 classification = %+v", report)
		}
		for _, reference := range report.References {
			if reference.Code != IntentArchiveCodeIndexStorageInconsistent ||
				reference.Action != IntentArchiveActionPurgeOrphans {
				t.Fatalf("unreferenced CP13 route = %+v", reference)
			}
		}
	})

	t.Run("live-cp13-blob-routes-to-confirmed-hash", func(t *testing.T) {
		retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireRetained)
		tombstone := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireTombstoned)
		index := archiveIndex(t, feature,
			archiveGeneration(t, feature, retained),
			archiveGeneration(t, feature, tombstone),
		)
		report, err := InspectIntentArchive(index, []IntentArchiveBlobObservation{
			archiveObservation(feature, retained.ContentSHA256, IntentArchiveBlobPresentCorrect, int64(len("shared"))),
		})
		if err != nil {
			t.Fatal(err)
		}
		if report.Consistent || len(report.Classes) != 1 ||
			report.Classes[0].Class != IntentArchiveRepairMixedReference ||
			len(report.Orphans) != 0 {
			t.Fatalf("live CP13 classification = %+v", report)
		}
		routed := 0
		for _, reference := range report.References {
			if reference.Action == IntentArchiveActionPurgeHash {
				routed++
				if reference.Code != IntentArchiveCodeIndexStorageInconsistent {
					t.Fatalf("live CP13 route = %+v", reference)
				}
			}
		}
		if routed != 1 {
			t.Fatalf("live CP13 confirmed-hash routes = %d, want 1: %+v", routed, report.References)
		}
	})
}

func TestS7PIB405ConcurrentIndexEditBeforeClaimRenamePreservesIndexAndBlobs(t *testing.T) {
	const feature = "demo"
	replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "selected", IntentArchiveWireRetained)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, replacement),
	))
	blobRel := storage.putRegular(feature, replacement.ContentSHA256, []byte("selected"))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var edited []byte
	storage.hooks["before-index-cas"] = func(memory *archiveMemoryStorage) {
		delete(memory.hooks, "before-index-cas")
		latest := memory.decodedIndex(t, feature)
		latest.Generations = append(latest.Generations,
			archiveGeneration(t, feature,
				archiveReplacement(t, IntentArchiveArtifactSpec, "operator-edit", IntentArchiveWireRetained),
			),
		)
		memory.externalSetIndex(t, latest)
		edited = append([]byte(nil), memory.index...)
	}
	storage.calls = nil
	result, err := ExecuteIntentArchivePurge(storage, plan)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgeIndexChanged)
	if typed.ExitClass != 3 || typed.Committed || result.Committed ||
		callIndex(storage.calls, "remove:") >= 0 ||
		!bytes.Equal(storage.index, edited) {
		t.Fatalf("pre-rename index edit = result:%+v err:%+v calls:%v", result, typed, storage.calls)
	}
	blob, exists := storage.blobs[blobRel]
	if !exists || !bytes.Equal(blob.data, []byte("selected")) {
		t.Fatalf("selected blob changed after index CAS refusal: exists=%v blob=%+v", exists, blob)
	}
}

func TestS7PIB425RehydrateExcludesPendingAndRefusesDanglingRetained(t *testing.T) {
	const feature = "demo"
	t.Run("multiple-tombstones-one-CAS", func(t *testing.T) {
		first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireTombstoned)
		second := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireTombstoned)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, first),
			archiveGeneration(t, feature, second),
		))
		plan, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("shared"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Outcome() != IntentArchiveAppendRehydrate {
			t.Fatalf("outcome = %q, want rehydrate", plan.Outcome())
		}
		storage.calls = nil
		if _, err := PublishIntentArchiveBlobs(storage, plan); err != nil {
			t.Fatal(err)
		}
		indexRel, err := IntentArchiveIndexRel(feature)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := storage.CASIndex(indexRel, plan.IndexPreimage().Identity, plan.IndexBytes()); err != nil {
			t.Fatal(err)
		}
		if strings.Count(strings.Join(storage.calls, "\n"), "cas-index") != 1 {
			t.Fatalf("rehydration index CAS calls = %v", storage.calls)
		}
		final := storage.decodedIndex(t, feature)
		revived := 0
		for _, generation := range final.Generations {
			for _, replacement := range generation.Replaced {
				if replacement.ContentSHA256 == first.ContentSHA256 {
					revived++
					if replacement.WireState() != IntentArchiveWireRetained {
						t.Fatalf("tombstoned target was not revived: %+v", replacement)
					}
				}
			}
		}
		report, err := InspectIntentArchive(final, []IntentArchiveBlobObservation{
			archiveObservation(feature, first.ContentSHA256, IntentArchiveBlobPresentCorrect, int64(len("shared"))),
		})
		if err != nil {
			t.Fatal(err)
		}
		if revived != 2 || !report.Consistent || len(report.Orphans) != 0 {
			t.Fatalf("rehydration state = revived:%d report:%+v", revived, report)
		}
	})

	t.Run("pending-hash-routes-to-owner", func(t *testing.T) {
		tombstone := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireTombstoned)
		pending := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, tombstone),
			archiveGeneration(t, feature, pending),
		))
		storage.putRegular(feature, pending.ContentSHA256, []byte("shared"))
		before := append([]byte(nil), storage.index...)
		storage.calls = nil
		_, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("shared"),
		}})
		typed := assertArchiveCode(t, err, IntentArchiveCodeRecoveryPending)
		if typed.ExitClass != 3 || len(storage.mutationCalls()) != 0 ||
			!bytes.Equal(before, storage.index) {
			t.Fatalf("pending owner precedence = err:%+v calls:%v", typed, storage.calls)
		}
		unchanged := storage.decodedIndex(t, feature)
		if !intentArchiveHashHasState(unchanged, pending.ContentSHA256, IntentArchiveWireRemovalPending) ||
			!intentArchiveHashHasState(unchanged, pending.ContentSHA256, IntentArchiveWireTombstoned) {
			t.Fatalf("pending/tombstoned references were consumed: %+v", unchanged)
		}
	})

	t.Run("dangling-retained-still-refuses", func(t *testing.T) {
		retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireRetained)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, retained),
		))
		before := append([]byte(nil), storage.index...)
		storage.calls = nil
		_, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("shared"),
		}})
		typed := assertArchiveCode(t, err, IntentArchiveCodeBlobDangling)
		if typed.ExitClass != 3 || len(storage.mutationCalls()) != 0 ||
			!bytes.Equal(before, storage.index) {
			t.Fatalf("dangling retained refusal = err:%+v calls:%v", typed, storage.calls)
		}
	})
}

func TestS7PIB429OrphanRevalidationProtectsNewlyReferencedBlob(t *testing.T) {
	const feature = "demo"
	body := []byte("orphan becomes referenced")
	replacement := archiveReplacement(
		t, IntentArchiveArtifactAnalysis, string(body), IntentArchiveWireRetained,
	)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature))
	blobRel := storage.putRegular(feature, replacement.ContentSHA256, body)
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Orphans: true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	hook := "before:preflight-remove:" + path.Base(blobRel)
	storage.hooks[hook] = func(memory *archiveMemoryStorage) {
		delete(memory.hooks, hook)
		memory.externalSetIndex(t, archiveIndex(
			t, feature, archiveGeneration(t, feature, replacement),
		))
	}
	storage.calls = nil
	result, err := ExecuteIntentArchivePurge(storage, plan)
	typed := assertArchiveCode(t, err, IntentArchiveCodePurgeIndexChanged)
	if typed.ExitClass != 3 || typed.Committed || result.Committed ||
		callIndex(storage.calls, "remove:") >= 0 {
		t.Fatalf("orphan revalidation = result:%+v err:%+v calls:%v", result, typed, storage.calls)
	}
	blob, exists := storage.blobs[blobRel]
	if !exists || !bytes.Equal(blob.data, body) {
		t.Fatalf("newly referenced blob changed: exists=%t blob=%+v", exists, blob)
	}
	final := storage.decodedIndex(t, feature)
	if len(final.Generations) != 1 ||
		final.Generations[0].Replaced[0].WireState() != IntentArchiveWireRetained {
		t.Fatalf("new external reference did not survive: %+v", final)
	}
}

func s7ArchiveImmutableProjection(index IntentArchiveIndex) []string {
	projection := []string{}
	for _, generation := range index.Generations {
		projection = append(projection, generation.GenerationID, string(generation.Mode))
		for _, replacement := range generation.Replaced {
			projection = append(projection,
				string(replacement.ArtifactID),
				replacement.Path,
				replacement.ContentSHA256,
				strconv.FormatInt(replacement.SizeBytes, 10),
			)
		}
	}
	return projection
}

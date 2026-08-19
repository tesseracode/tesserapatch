package store

import (
	"bytes"
	"errors"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/redact"
)

func TestIntentArchiveBlobPublicationBoundaryAndCanonicalNoOp(t *testing.T) {
	const feature = "demo"
	storage := newEmptyArchiveMemoryStorage()
	inputs := []IntentArchiveReplacementInput{
		{ArtifactID: IntentArchiveArtifactSpec, Path: "spec.md", PriorBytes: []byte("prior bytes")},
		{ArtifactID: IntentArchiveArtifactAnalysis, Path: "analysis.md", PriorBytes: []byte("prior bytes")},
	}
	plan, err := PlanIntentArchiveAppend(storage, feature, inputs)
	if err != nil {
		t.Fatal(err)
	}
	blobs := plan.Blobs()
	preimages := plan.Preimages()
	if plan.Outcome() != IntentArchiveAppendNew || len(blobs) != 1 || len(preimages) != 2 {
		t.Fatalf("plan outcome=%q blobs=%+v preimages=%+v", plan.Outcome(), blobs, preimages)
	}
	if preimages[0].ArtifactID != IntentArchiveArtifactAnalysis ||
		preimages[1].ArtifactID != IntentArchiveArtifactSpec {
		t.Fatalf("preimages not stable artifact order: %+v", preimages)
	}
	if plan.IndexPreimage().Exists || len(plan.IndexBytes()) == 0 {
		t.Fatalf("index publication model is incomplete: preimage=%+v bytes=%q", plan.IndexPreimage(), plan.IndexBytes())
	}
	plannedIndex, err := DecodeIntentArchiveIndex(plan.IndexBytes(), feature)
	if err != nil {
		t.Fatal(err)
	}
	if len(plannedIndex.Generations) != 1 || len(plannedIndex.Generations[0].Replaced) != 2 {
		t.Fatalf("planned index = %+v", plannedIndex)
	}

	copiedBlobs := plan.Blobs()
	copiedBlobs[0].Data[0] ^= 0xff
	copiedBytes := plan.IndexBytes()
	copiedBytes[0] ^= 0xff
	copiedPreimage := plan.IndexPreimage()
	copiedPreimage.Raw = append(copiedPreimage.Raw, 'x')
	if bytes.Equal(copiedBlobs[0].Data, plan.Blobs()[0].Data) ||
		bytes.Equal(copiedBytes, plan.IndexBytes()) ||
		len(plan.IndexPreimage().Raw) != 0 {
		t.Fatal("append plan getters exposed mutable internal storage")
	}

	storage.calls = nil
	result, err := PublishIntentArchiveBlobs(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != IntentArchiveAppendNew ||
		!result.Committed ||
		len(result.NewOrphanHashes) != 1 ||
		result.NewOrphanHashes[0] != blobs[0].Hash {
		t.Fatalf("blob publication result = %+v", result)
	}
	if callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("S3 blob publication called CASIndex: %v", storage.calls)
	}
	if storage.indexExists {
		t.Fatal("S3 blob publication published index.json")
	}
	publishIndex := callIndex(storage.calls, "publish:")
	if publishIndex < 0 {
		t.Fatalf("blob was not published: %v", storage.calls)
	}

	preimage := plan.IndexPreimage()
	if _, err := storage.CASIndex(".tpatch/features/demo/artifacts/intent-archive/index.json", preimage.Identity, plan.IndexBytes()); err != nil {
		t.Fatal(err)
	}
	if publishIndex >= callIndex(storage.calls, "cas-index") {
		t.Fatalf("blob publication did not precede later S1 index publication: %v", storage.calls)
	}

	duplicate, err := PlanIntentArchiveAppend(storage, feature, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.Outcome() != IntentArchiveAppendNoOp ||
		len(duplicate.Blobs()) != 0 ||
		!bytes.Equal(duplicate.IndexBytes(), storage.index) {
		t.Fatalf("duplicate plan outcome=%q blobs=%+v index=%q", duplicate.Outcome(), duplicate.Blobs(), duplicate.IndexBytes())
	}
	storage.calls = nil
	duplicateResult, err := PublishIntentArchiveBlobs(storage, duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if duplicateResult.Outcome != IntentArchiveAppendNoOp || len(storage.mutationCalls()) != 0 {
		t.Fatalf("duplicate result=%+v calls=%v", duplicateResult, storage.calls)
	}
	if callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("duplicate no-op called CASIndex: %v", storage.calls)
	}
}

func TestIntentArchiveAppendCollisionAndGlobalRehydrate(t *testing.T) {
	const feature = "demo"
	original := archiveReplacement(t, IntentArchiveArtifactAnalysis, "old", IntentArchiveWireRetained)
	existingGeneration := archiveGeneration(t, feature, original)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature, existingGeneration))
	storage.putRegular(feature, original.ContentSHA256, []byte("old"))
	snapshot, err := CaptureIntentArchive(storage, feature)
	if err != nil {
		t.Fatal(err)
	}
	restore := SetIntentArchiveGenerationIDDeriverForTest(func([]byte) string {
		return existingGeneration.GenerationID
	})
	_, err = BuildIntentArchiveAppendPlan(snapshot, []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactSpec,
		Path:       "spec.md",
		PriorBytes: []byte("different body"),
	}})
	restore()
	assertArchiveCode(t, err, IntentArchiveCodeGenerationCollision)

	shared := archiveHash("old")
	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "old", IntentArchiveWireTombstoned)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "old", IntentArchiveWireTombstoned)
	rehydrateIndex := archiveIndex(t, feature,
		archiveGeneration(t, feature, first),
		archiveGeneration(t, feature, second),
	)
	storage = newArchiveMemoryStorage(t, rehydrateIndex)
	inputs := []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("old"),
	}}
	plan, err := PlanIntentArchiveAppend(storage, feature, inputs)
	if err != nil {
		t.Fatal(err)
	}
	blobs := plan.Blobs()
	if plan.Outcome() != IntentArchiveAppendRehydrate ||
		len(blobs) != 1 ||
		!equalIntentArchiveArtifactIDSets(blobs[0].ArtifactIDs, []IntentArchiveArtifactID{
			IntentArchiveArtifactAnalysis,
			IntentArchiveArtifactSpec,
		}) {
		t.Fatalf("rehydrate plan outcome=%q blobs=%+v", plan.Outcome(), blobs)
	}
	beforeIndex := append([]byte(nil), storage.index...)
	storage.calls = nil
	result, err := PublishIntentArchiveBlobs(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != IntentArchiveAppendRehydrate ||
		len(result.NewOrphanHashes) != 1 ||
		!bytes.Equal(storage.index, beforeIndex) ||
		callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("rehydrate publication result=%+v calls=%v", result, storage.calls)
	}
	preimage := plan.IndexPreimage()
	if _, err := storage.CASIndex(".tpatch/features/demo/artifacts/intent-archive/index.json", preimage.Identity, plan.IndexBytes()); err != nil {
		t.Fatal(err)
	}
	index := storage.decodedIndex(t, feature)
	count := 0
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == shared {
				count++
				if replacement.WireState() != IntentArchiveWireRetained {
					t.Fatalf("same-hash reference not globally retained: %+v", replacement)
				}
			}
		}
	}
	if count != 2 {
		t.Fatalf("rehydrated %d references, want 2", count)
	}
}

func TestIntentArchiveBlobPublicationReportsReuseWithoutFalseOrphan(t *testing.T) {
	const feature = "demo"
	retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared-bytes", IntentArchiveWireRetained)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, retained),
	))
	storage.putRegular(feature, retained.ContentSHA256, []byte("shared-bytes"))
	plan, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactSpec,
		Path:       "spec.md",
		PriorBytes: []byte("shared-bytes"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := PublishIntentArchiveBlobs(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Committed ||
		len(result.NewOrphanHashes) != 0 ||
		len(result.BlobResults) != 1 ||
		!result.BlobResults[0].Reused ||
		result.BlobResults[0].Committed {
		t.Fatalf("reuse truth = %+v", result)
	}
	if callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("reuse called CASIndex: %v", storage.calls)
	}
}

func TestIntentArchiveAppendRefusesEveryOwnedHashWithoutMutation(t *testing.T) {
	const feature = "demo"
	for _, fixture := range []struct {
		name string
		kind IntentArchiveBlobKind
	}{
		{name: "present", kind: IntentArchiveBlobKindRegular},
		{name: "absent", kind: IntentArchiveBlobKindAbsent},
		{name: "corrupt", kind: IntentArchiveBlobKindDirectory},
	} {
		t.Run("selected-"+fixture.name, func(t *testing.T) {
			pending := archiveReplacement(t, IntentArchiveArtifactAnalysis, "owned", IntentArchiveWireRemovalPending)
			storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
				archiveGeneration(t, feature, pending),
			))
			switch fixture.kind {
			case IntentArchiveBlobKindRegular:
				storage.putRegular(feature, pending.ContentSHA256, []byte("owned"))
			case IntentArchiveBlobKindAbsent:
			default:
				storage.putKind(feature, pending.ContentSHA256, fixture.kind)
			}
			storage.calls = nil
			_, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
				ArtifactID: IntentArchiveArtifactAnalysis,
				Path:       "analysis.md",
				PriorBytes: []byte("owned"),
			}})
			typed := assertArchiveCode(t, err, IntentArchiveCodeRecoveryPending)
			if typed.Hash != pending.ContentSHA256 || len(storage.mutationCalls()) != 0 {
				t.Fatalf("owned refusal=%+v calls=%v", typed, storage.calls)
			}
		})
	}

	t.Run("unselected", func(t *testing.T) {
		pending := archiveReplacement(t, IntentArchiveArtifactSpec, "owned-other", IntentArchiveWireRemovalPending)
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, pending),
		))
		storage.putRegular(feature, pending.ContentSHA256, []byte("owned-other"))
		storage.calls = nil
		_, err := PlanIntentArchiveAppend(storage, feature, []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("-----BEGIN PRIVATE KEY-----"),
		}})
		assertArchiveCode(t, err, IntentArchiveCodeRecoveryPending)
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("unselected owned hash mutated storage: %v", storage.calls)
		}
	})
}

func TestIntentArchiveAppendPlanTamperAndFreshGlobalValidation(t *testing.T) {
	planType := reflect.TypeOf(IntentArchiveAppendPlan{})
	for index := 0; index < planType.NumField(); index++ {
		if planType.Field(index).PkgPath == "" {
			t.Fatalf("append plan field %q is externally mutable", planType.Field(index).Name)
		}
	}

	newPlan := func(t *testing.T, storage *archiveMemoryStorage) IntentArchiveAppendPlan {
		t.Helper()
		plan, err := PlanIntentArchiveAppend(storage, "demo", []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("candidate"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	t.Run("sealed-field-mutation", func(t *testing.T) {
		storage := newEmptyArchiveMemoryStorage()
		plan := newPlan(t, storage)
		plan.blobs[0].Data[0] ^= 0xff
		storage.calls = nil
		_, err := PublishIntentArchiveBlobs(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodeIndexCorrupt)
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("tampered plan mutated storage: %v", storage.calls)
		}
	})

	t.Run("resealed-incomplete-candidates", func(t *testing.T) {
		storage := newEmptyArchiveMemoryStorage()
		plan := newPlan(t, storage)
		plan.blobs = []IntentArchiveBlobCandidate{}
		plan.seal = sealIntentArchiveAppendPlan(plan)
		storage.calls = nil
		_, err := PublishIntentArchiveBlobs(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodeIndexCorrupt)
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("incomplete plan mutated storage: %v", storage.calls)
		}
	})

	t.Run("reseeded-index-diff", func(t *testing.T) {
		storage := newEmptyArchiveMemoryStorage()
		plan := newPlan(t, storage)
		empty, err := NewIntentArchiveIndex("demo")
		if err != nil {
			t.Fatal(err)
		}
		plan.indexBytes, err = EncodeIntentArchiveIndex(empty)
		if err != nil {
			t.Fatal(err)
		}
		plan.seal = sealIntentArchiveAppendPlan(plan)
		storage.calls = nil
		_, err = PublishIntentArchiveBlobs(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodeIndexCorrupt)
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("wrong index diff mutated storage: %v", storage.calls)
		}
	})

	t.Run("index-preimage-changed", func(t *testing.T) {
		storage := newEmptyArchiveMemoryStorage()
		plan := newPlan(t, storage)
		external := archiveIndex(t, "demo",
			archiveGeneration(t, "demo",
				archiveReplacement(t, IntentArchiveArtifactSpec, "external", IntentArchiveWireRetained),
			),
		)
		storage.externalSetIndex(t, external)
		storage.putRegular("demo", external.Generations[0].Replaced[0].ContentSHA256, []byte("external"))
		storage.calls = nil
		_, err := PublishIntentArchiveBlobs(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodeIndexChanged)
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("changed preimage mutated storage: %v", storage.calls)
		}
	})

	t.Run("global-x11-recaptured", func(t *testing.T) {
		storage := newEmptyArchiveMemoryStorage()
		plan := newPlan(t, storage)
		corruptHash := archiveHash("unrelated")
		storage.putKind("demo", corruptHash, IntentArchiveBlobKindFIFO)
		storage.calls = nil
		_, err := PublishIntentArchiveBlobs(storage, plan)
		assertArchiveCode(t, err, IntentArchiveCodeBlobCorrupt)
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("global X11 refusal mutated storage: %v", storage.calls)
		}
	})
}

func TestIntentArchiveAppendPostCommitFailuresAreExitFiveWithOrphans(t *testing.T) {
	t.Run("second-blob-precommit-failure", func(t *testing.T) {
		storage := newEmptyArchiveMemoryStorage()
		plan, err := PlanIntentArchiveAppend(storage, "demo", []IntentArchiveReplacementInput{
			{ArtifactID: IntentArchiveArtifactAnalysis, Path: "analysis.md", PriorBytes: []byte("first-blob")},
			{ArtifactID: IntentArchiveArtifactSpec, Path: "spec.md", PriorBytes: []byte("second-blob")},
		})
		if err != nil {
			t.Fatal(err)
		}
		blobs := plan.Blobs()
		storage.fail["publish:"+path.Base(blobs[1].Rel)] = 1
		result, err := PublishIntentArchiveBlobs(storage, plan)
		typed := assertArchiveCode(t, err, IntentArchiveCodeRegenerateGenerationFailed)
		if typed.ExitClass != 5 ||
			!typed.Committed ||
			!result.Committed ||
			len(result.NewOrphanHashes) != 1 ||
			result.NewOrphanHashes[0] != blobs[0].Hash ||
			callIndex(storage.calls, "cas-index") >= 0 {
			t.Fatalf("mid-blob result=%+v error=%+v calls=%v", result, typed, storage.calls)
		}
	})

	t.Run("typed-postcommit-error-is-promoted", func(t *testing.T) {
		storage := newEmptyArchiveMemoryStorage()
		plan, err := PlanIntentArchiveAppend(storage, "demo", []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("post-commit"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		blob := plan.Blobs()[0]
		storage.postError["publish:"+path.Base(blob.Rel)] = &IntentArchiveError{
			Code:      IntentArchiveCodeBlobCorrupt,
			ExitClass: 3,
		}
		result, err := PublishIntentArchiveBlobs(storage, plan)
		typed := assertArchiveCode(t, err, IntentArchiveCodeRegenerateGenerationFailed)
		if typed.ExitClass != 5 || !typed.Committed || len(result.NewOrphanHashes) != 1 {
			t.Fatalf("post-commit result=%+v error=%+v", result, typed)
		}
	})

	t.Run("directory-sync-failure", func(t *testing.T) {
		storage := newEmptyArchiveMemoryStorage()
		plan, err := PlanIntentArchiveAppend(storage, "demo", []IntentArchiveReplacementInput{{
			ArtifactID: IntentArchiveArtifactAnalysis,
			Path:       "analysis.md",
			PriorBytes: []byte("sync-failure"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		storage.fail["sync:blobs"] = 1
		result, err := PublishIntentArchiveBlobs(storage, plan)
		typed := assertArchiveCode(t, err, IntentArchiveCodeRegenerateGenerationFailed)
		if typed.ExitClass != 5 || !typed.Committed || len(result.NewOrphanHashes) != 1 {
			t.Fatalf("sync failure result=%+v error=%+v", result, typed)
		}
	})
}

func TestIntentArchiveAppendRejectsAllRedactionClassesBeforeMutation(t *testing.T) {
	fixtures := []struct {
		class string
		data  string
	}{
		{redact.ClassPrivateKey, "-----BEGIN PRIVATE KEY-----"},
		{redact.ClassConnectionURL, "mysql://host/db"},
		{redact.ClassEmailPII, "owner@example.invalid"},
		{redact.ClassCredentialAssignment, strings.Join([]string{"pass", "word"}, "") + ` = "hunter2hunter2hunter2"`},
		{redact.ClassBearerOrKeyToken, "Authorization: Bearer " + strings.Repeat("x", 24)},
		{redact.ClassHomeAbsolutePath, "/Users/example/private/file"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.class, func(t *testing.T) {
			storage := newEmptyArchiveMemoryStorage()
			snapshot, err := CaptureIntentArchive(storage, "demo")
			if err != nil {
				t.Fatal(err)
			}
			storage.calls = nil
			_, err = BuildIntentArchiveAppendPlan(snapshot, []IntentArchiveReplacementInput{{
				ArtifactID: IntentArchiveArtifactSpec,
				Path:       "spec.md",
				PriorBytes: []byte(fixture.data),
			}})
			typed := assertArchiveCode(t, err, IntentArchiveCodeContentSensitive)
			if !strings.Contains(typed.Class, fixture.class) || !strings.Contains(typed.Detail, fixture.class) {
				t.Fatalf("redaction error does not name class %q: %+v", fixture.class, typed)
			}
			if strings.Contains(typed.Error(), fixture.data) {
				t.Fatalf("redaction error leaked raw bytes: %q", typed.Error())
			}
			if len(storage.mutationCalls()) != 0 {
				t.Fatalf("redaction refusal mutated storage: %v", storage.calls)
			}
		})
	}
}

func TestIntentArchiveAppendRefusesCorruptExistingObjectWithoutOverwrite(t *testing.T) {
	storage := newEmptyArchiveMemoryStorage()
	hash := archiveHash("candidate")
	rel, _ := IntentArchiveBlobRel("demo", hash)
	storage.blobs[rel] = archiveMemoryBlob{
		kind:    IntentArchiveBlobKindRegular,
		data:    []byte("different"),
		version: 1,
	}
	before := append([]byte(nil), storage.blobs[rel].data...)
	_, err := PlanIntentArchiveAppend(storage, "demo", []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("candidate"),
	}})
	assertArchiveCode(t, err, IntentArchiveCodeBlobCorrupt)
	if !bytes.Equal(storage.blobs[rel].data, before) {
		t.Fatal("corrupt existing object was overwritten")
	}
	if callIndex(storage.calls, "publish:") >= 0 || callIndex(storage.calls, "cas-index") >= 0 {
		t.Fatalf("corrupt planning reached mutation calls: %v", storage.calls)
	}
}

func TestIntentArchiveAppendPrecommitStorageErrorRemainsExitThree(t *testing.T) {
	storage := newEmptyArchiveMemoryStorage()
	plan, err := PlanIntentArchiveAppend(storage, "demo", []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("precommit"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	blob := plan.Blobs()[0]
	storage.fail["probe:"+path.Base(blob.Rel)] = 1
	result, err := PublishIntentArchiveBlobs(storage, plan)
	var typed *IntentArchiveError
	if !errors.As(err, &typed) || typed.ExitClass != 3 || typed.Committed || result.Committed {
		t.Fatalf("precommit result=%+v error=%+v", result, typed)
	}
}

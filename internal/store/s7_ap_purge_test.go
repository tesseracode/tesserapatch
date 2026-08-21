//go:build (linux && !android) || (darwin && !ios)

package store

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestS7APPurgeContracts(t *testing.T) {
	const feature = "ap-purge"

	t.Run("PIB-457", func(t *testing.T) {
		replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "missing-ap", IntentArchiveWireRetained)
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
		final := storage.decodedIndex(t, feature)
		if result.Outcome != IntentArchivePurgePurged ||
			callIndex(storage.calls, "remove:") >= 0 ||
			len(storage.indexHistory) != 1 ||
			intentArchiveHashHasState(final, replacement.ContentSHA256, IntentArchiveWireRemovalPending) ||
			!intentArchiveHashAllTombstoned(final, replacement.ContentSHA256) {
			t.Fatalf("PIB-457 dangling purge = result:%+v calls:%v history:%d final:%+v",
				result, storage.calls, len(storage.indexHistory), final)
		}
	})

	t.Run("PIB-458", func(t *testing.T) {
		run := func(selector IntentArchivePurgeSelector) ([]string, IntentArchiveIndex) {
			replacement := archiveReplacement(t, IntentArchiveArtifactAnalysis, "same-helper", IntentArchiveWireRetained)
			storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
				archiveGeneration(t, feature, replacement),
			))
			plan, err := PlanIntentArchivePurge(storage, feature, selector, true)
			if err != nil {
				t.Fatal(err)
			}
			storage.calls = nil
			result, err := ExecuteIntentArchivePurge(storage, plan)
			if err != nil || result.Outcome != IntentArchivePurgePurged {
				t.Fatalf("PIB-458 selector %+v = result:%+v err:%v", selector, result, err)
			}
			return append([]string(nil), storage.calls...), storage.decodedIndex(t, feature)
		}
		hash := archiveHash("same-helper")
		narrowCalls, narrowIndex := run(IntentArchivePurgeSelector{Blobs: []string{hash}})
		allCalls, allIndex := run(IntentArchivePurgeSelector{All: true})
		if fmt.Sprint(narrowCalls) != fmt.Sprint(allCalls) ||
			!intentArchiveHashAllTombstoned(narrowIndex, hash) ||
			!intentArchiveHashAllTombstoned(allIndex, hash) ||
			callIndex(narrowCalls, "remove:") >= 0 {
			t.Fatalf("PIB-458 narrow/all routes differ: narrow=%v all=%v", narrowCalls, allCalls)
		}
	})

	t.Run("PIB-459", func(t *testing.T) {
		sourcePath := filepath.Join("intent_archive.go")
		source, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateS7APDanglingRepairSource(string(source)); err != nil {
			t.Fatal(err)
		}
		for name, wrong := range map[string]string{
			"pending-state": strings.Replace(
				string(source),
				"setIntentArchiveHashState(latestIndex, hash, IntentArchiveWireTombstoned)",
				"setIntentArchiveHashState(latestIndex, hash, IntentArchiveWireRemovalPending)",
				1,
			),
			"blob-removal": strings.Replace(
				string(source),
				"tombstoned, changed := setIntentArchiveHashState(latestIndex, hash, IntentArchiveWireTombstoned)",
				"_, _ = storage.RemoveBlob(blobRel, IntentArchiveBlobIdentity{})\n\t"+
					"tombstoned, changed := setIntentArchiveHashState(latestIndex, hash, IntentArchiveWireTombstoned)",
				1,
			),
		} {
			if err := validateS7APDanglingRepairSource(wrong); err == nil {
				t.Fatalf("PIB-459 same validator accepted %s alternative repair", name)
			}
		}
	})

	t.Run("PIB-460", func(t *testing.T) {
		first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "global-missing", IntentArchiveWireRetained)
		second := archiveReplacement(t, IntentArchiveArtifactSpec, "global-missing", IntentArchiveWireRetained)
		third := archiveReplacement(t, IntentArchiveArtifactExploration, "global-missing", IntentArchiveWireTombstoned)
		index := archiveIndex(t, feature,
			archiveGeneration(t, feature, first, second),
			archiveGeneration(t, feature, third),
		)
		beforeIDs := []string{index.Generations[0].GenerationID, index.Generations[1].GenerationID}
		storage := newArchiveMemoryStorage(t, index)
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
			Blobs: []string{first.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteIntentArchivePurge(storage, plan)
		if err != nil {
			t.Fatal(err)
		}
		final := storage.decodedIndex(t, feature)
		afterIDs := []string{final.Generations[0].GenerationID, final.Generations[1].GenerationID}
		references := intentArchiveReferencesForHash(final, first.ContentSHA256)
		if result.Outcome != IntentArchivePurgePurged ||
			fmt.Sprint(beforeIDs) != fmt.Sprint(afterIDs) ||
			len(references) != 3 ||
			!intentArchiveHashAllTombstoned(final, first.ContentSHA256) ||
			callIndex(storage.calls, "remove:") >= 0 {
			t.Fatalf("PIB-460 global tombstone = result:%+v ids:%v/%v refs:%+v calls:%v",
				result, beforeIDs, afterIDs, references, storage.calls)
		}
	})

	t.Run("PIB-465", func(t *testing.T) {
		failures := []string{}
		checkZeroMutation := func(name string, storage *archiveMemoryStorage, beforeIndex []byte, beforeBlobs map[string]archiveMemoryBlob) {
			t.Helper()
			if len(storage.indexHistory) != 0 || callIndex(storage.calls, "cas-index") >= 0 ||
				callIndex(storage.calls, "remove:") >= 0 || !bytes.Equal(storage.index, beforeIndex) ||
				fmt.Sprint(storage.blobs) != fmt.Sprint(beforeBlobs) {
				failures = append(failures, fmt.Sprintf("%s calls=%v history=%d", name, storage.calls, len(storage.indexHistory)))
			}
		}

		unknownStorage := newArchiveMemoryStorage(t, archiveIndex(t, feature))
		beforeUnknown := append([]byte(nil), unknownStorage.index...)
		beforeUnknownBlobs := cloneS7APMemoryBlobs(unknownStorage.blobs)
		_, err := PlanIntentArchivePurge(unknownStorage, feature, IntentArchivePurgeSelector{
			Blobs: []string{archiveHash("unknown")},
		}, true)
		if err == nil {
			failures = append(failures, "unknown selector was accepted")
		}
		checkZeroMutation("unknown-selector", unknownStorage, beforeUnknown, beforeUnknownBlobs)

		corrupt := archiveReplacement(t, IntentArchiveArtifactAnalysis, "correct", IntentArchiveWireRetained)
		corruptStorage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, corrupt),
		))
		corruptStorage.putNonRegular(feature, corrupt.ContentSHA256)
		beforeCorrupt := append([]byte(nil), corruptStorage.index...)
		beforeCorruptBlobs := cloneS7APMemoryBlobs(corruptStorage.blobs)
		_, err = PlanIntentArchivePurge(corruptStorage, feature, IntentArchivePurgeSelector{
			Blobs: []string{corrupt.ContentSHA256},
		}, true)
		if err == nil {
			failures = append(failures, "non-regular blob was accepted")
		}
		checkZeroMutation("non-regular", corruptStorage, beforeCorrupt, beforeCorruptBlobs)

		raced := archiveReplacement(t, IntentArchiveArtifactAnalysis, "race", IntentArchiveWireRetained)
		raceStorage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, raced),
		))
		raceStorage.putRegular(feature, raced.ContentSHA256, []byte("race"))
		racePlan, err := PlanIntentArchivePurge(raceStorage, feature, IntentArchivePurgeSelector{
			Blobs: []string{raced.ContentSHA256},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		changed := raceStorage.decodedIndex(t, feature)
		changed.Generations = append(changed.Generations,
			archiveGeneration(t, feature,
				archiveReplacement(t, IntentArchiveArtifactSpec, "other", IntentArchiveWireRetained),
			),
		)
		raceStorage.externalSetIndex(t, changed)
		beforeRace := append([]byte(nil), raceStorage.index...)
		beforeRaceBlobs := cloneS7APMemoryBlobs(raceStorage.blobs)
		raceStorage.calls = nil
		result, err := ExecuteIntentArchivePurge(raceStorage, racePlan)
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgeIndexChanged)
		if typed.ExitClass != 3 || result.Committed {
			failures = append(failures, fmt.Sprintf("preflight race = result:%+v err:%+v", result, typed))
		}
		checkZeroMutation("preflight-race", raceStorage, beforeRace, beforeRaceBlobs)
		if len(failures) != 0 {
			t.Fatalf("PIB-465 predictable preflight failures:\n%s", strings.Join(failures, "\n"))
		}
	})

	t.Run("PIB-467", func(t *testing.T) {
		first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "partial-first", IntentArchiveWireRetained)
		second := archiveReplacement(t, IntentArchiveArtifactSpec, "partial-second", IntentArchiveWireRetained)
		ordered := []IntentArchiveReplacement{first, second}
		sort.Slice(ordered, func(i, j int) bool {
			return ordered[i].ContentSHA256 < ordered[j].ContentSHA256
		})
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, first, second),
		))
		storage.putRegular(feature, first.ContentSHA256, []byte("partial-first"))
		storage.putRegular(feature, second.ContentSHA256, []byte("partial-second"))
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		previous := failPurgeAfterFirstMutation
		failPurgeAfterFirstMutation = func() error { return errors.New("PIB-467 injected first mutation failure") }
		result, err := ExecuteIntentArchivePurge(storage, plan)
		failPurgeAfterFirstMutation = previous
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgePartial)
		intermediate, decodeErr := DecodeIntentArchiveIndex(storage.index, feature)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if typed.ExitClass != 5 || !typed.Committed || !result.Committed ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.PendingHash != ordered[0].ContentSHA256 ||
			len(result.RemainingHashes) != 1 ||
			result.RemainingHashes[0] != ordered[1].ContentSHA256 ||
			!intentArchiveHashHasState(intermediate, ordered[0].ContentSHA256, IntentArchiveWireRemovalPending) {
			t.Fatalf("PIB-467 partial = result:%+v error:%+v", result, typed)
		}
		recovered, err := RecoverPendingPurge(storage, feature)
		if err != nil || recovered.Outcome != IntentArchivePurgeRecovered {
			t.Fatalf("PIB-467 recovery retry = %+v err=%v", recovered, err)
		}
		retry, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		completed, err := ExecuteIntentArchivePurge(storage, retry)
		if err != nil || completed.Outcome != IntentArchivePurgePurged {
			t.Fatalf("PIB-467 completion retry = %+v err=%v", completed, err)
		}
	})

	t.Run("PIB-468", func(t *testing.T) {
		first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "diverge-first", IntentArchiveWireRetained)
		second := archiveReplacement(t, IntentArchiveArtifactSpec, "diverge-second", IntentArchiveWireRetained)
		ordered := []IntentArchiveReplacement{first, second}
		sort.Slice(ordered, func(i, j int) bool {
			return ordered[i].ContentSHA256 < ordered[j].ContentSHA256
		})
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
			archiveGeneration(t, feature, first, second),
		))
		blobRels := map[string]string{
			first.ContentSHA256:  storage.putRegular(feature, first.ContentSHA256, []byte("diverge-first")),
			second.ContentSHA256: storage.putRegular(feature, second.ContentSHA256, []byte("diverge-second")),
		}
		plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{All: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		events := []string{}
		armed := false
		storage.hooks["after-index-cas"] = func(memory *archiveMemoryStorage) {
			events = append(events, "pending-cas")
			armed = true
		}
		wrapped := &s7APStoreHashWrongProbe{
			IntentArchiveStorage: storage,
			targetHash:           ordered[0].ContentSHA256,
			armed:                &armed,
			events:               &events,
		}
		result, err := ExecuteIntentArchivePurge(wrapped, plan)
		events = append(events, "production-exit-6")
		typed := assertArchiveCode(t, err, IntentArchiveCodePurgeEvidenceDivergent)
		intermediate, decodeErr := DecodeIntentArchiveIndex(storage.index, feature)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		_, firstExists := storage.blobs[blobRels[ordered[0].ContentSHA256]]
		_, secondExists := storage.blobs[blobRels[ordered[1].ContentSHA256]]
		if typed.ExitClass != 6 || !typed.Committed || !result.Committed ||
			result.State == IntentArchivePurgeStateConsistent ||
			result.Resume != IntentArchiveResumePendingRecoveryThenCompletion ||
			result.PendingHash != ordered[0].ContentSHA256 ||
			len(result.CompletedHashes) != 0 ||
			fmt.Sprint(result.RemainingHashes) != fmt.Sprint([]string{ordered[1].ContentSHA256}) ||
			!firstExists || !secondExists ||
			!intentArchiveHashHasState(
				intermediate, ordered[0].ContentSHA256, IntentArchiveWireRemovalPending,
			) ||
			!reflect.DeepEqual(events, []string{
				"pending-cas", "post-cas-evidence-read", "production-exit-6",
			}) {
			t.Fatalf("PIB-468 pending-window divergence = result:%+v error:%+v blobs:%t/%t events:%v",
				result, typed, firstExists, secondExists, events)
		}
	})
}

type s7APStoreHashWrongProbe struct {
	IntentArchiveStorage
	targetHash string
	armed      *bool
	events     *[]string
	injections int
}

func (storage *s7APStoreHashWrongProbe) ProbeBlob(
	blobRel string,
) (IntentArchiveBlobProbe, error) {
	probe, err := storage.IntentArchiveStorage.ProbeBlob(blobRel)
	if err != nil || !*storage.armed || storage.injections != 0 ||
		!strings.HasSuffix(blobRel, storage.targetHash+".blob") {
		return probe, err
	}
	*storage.events = append(*storage.events, "post-cas-evidence-read")
	storage.injections++
	probe.SHA256 = strings.Repeat("f", 64)
	if probe.SHA256 == storage.targetHash {
		probe.SHA256 = strings.Repeat("e", 64)
	}
	return probe, nil
}

func validateS7APDanglingRepairSource(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "intent_archive.go", source, 0)
	if err != nil {
		return err
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "executeIntentArchiveAbsentTombstone" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil {
		return errors.New("dangling absent-object transition is missing")
	}
	setTombstone, publish, syncBlobs := 0, 0, 0
	var forbidden string
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if forbidden != "" {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			name = fun.Name
		case *ast.SelectorExpr:
			name = fun.Sel.Name
		}
		switch name {
		case "setIntentArchiveHashState":
			if len(call.Args) != 3 {
				forbidden = "malformed state transition"
				return false
			}
			state, _ := call.Args[2].(*ast.Ident)
			if state == nil || state.Name != "IntentArchiveWireTombstoned" {
				forbidden = "dangling transition is not direct-to-tombstone"
				return false
			}
			setTombstone++
		case "publishIntentArchiveIndex":
			publish++
		case "RemoveBlob", "removeIntentArchiveBlob":
			forbidden = "dangling transition removes an absent blob"
		case "SyncDirectory":
			if len(call.Args) == 1 {
				if ident, _ := call.Args[0].(*ast.Ident); ident != nil && ident.Name == "blobsRel" {
					syncBlobs++
				}
			}
		}
		return forbidden == ""
	})
	if forbidden != "" {
		return errors.New(forbidden)
	}
	if setTombstone != 1 || publish != 1 || syncBlobs != 1 {
		return fmt.Errorf("dangling transition shape = tombstone:%d publish:%d sync-blobs:%d",
			setTombstone, publish, syncBlobs)
	}
	return nil
}

func cloneS7APMemoryBlobs(source map[string]archiveMemoryBlob) map[string]archiveMemoryBlob {
	clone := make(map[string]archiveMemoryBlob, len(source))
	for rel, blob := range source {
		copyBlob := blob
		copyBlob.data = append([]byte(nil), blob.data...)
		clone[rel] = copyBlob
	}
	return clone
}

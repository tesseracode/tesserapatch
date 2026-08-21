package store

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"testing"
)

func TestS7PIB443CrashMatrixRecoversEverySelectedHash(t *testing.T) {
	t.Run("all-before-removal-hash-0", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "all", "before-removal", 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("all-before-removal-hash-1", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "all", "before-removal", 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("all-after-removal-hash-0", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "all", "after-removal", 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("all-after-removal-hash-1", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "all", "after-removal", 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("all-after-tombstone-hash-0", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "all", "after-tombstone", 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("all-after-tombstone-hash-1", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "all", "after-tombstone", 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("generation-before-removal-hash-0", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "generation", "before-removal", 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("generation-before-removal-hash-1", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "generation", "before-removal", 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("generation-after-removal-hash-0", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "generation", "after-removal", 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("generation-after-removal-hash-1", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "generation", "after-removal", 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("generation-after-tombstone-hash-0", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "generation", "after-tombstone", 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("generation-after-tombstone-hash-1", func(t *testing.T) {
		if err := s7RunPIB443Cell(t, "generation", "after-tombstone", 1); err != nil {
			t.Fatal(err)
		}
	})
}

func s7RunPIB443Cell(t *testing.T, selectorKind, crashPoint string, targetIndex int) error {
	t.Helper()
	const feature = "demo"
	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "pib443-first", IntentArchiveWireRetained)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "pib443-second", IntentArchiveWireRetained)
	generation := archiveGeneration(t, feature, first, second)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature, generation))
	storage.putRegular(feature, first.ContentSHA256, []byte("pib443-first"))
	storage.putRegular(feature, second.ContentSHA256, []byte("pib443-second"))

	selector := IntentArchivePurgeSelector{}
	switch selectorKind {
	case "all":
		selector.All = true
	case "generation":
		selector.Generations = []string{generation.GenerationID}
	default:
		return fmt.Errorf("unknown selector %q", selectorKind)
	}
	plan, err := PlanIntentArchivePurge(storage, feature, selector, true)
	if err != nil {
		return err
	}
	if len(plan.Hashes) != 2 || !sort.StringsAreSorted(plan.Hashes) {
		return fmt.Errorf("%s selected hashes are not the exact lexical pair: %v", selectorKind, plan.Hashes)
	}
	if targetIndex < 0 || targetIndex >= len(plan.Hashes) {
		return fmt.Errorf("target index %d outside %v", targetIndex, plan.Hashes)
	}
	targetHash := plan.Hashes[targetIndex]
	blobRel, err := IntentArchiveBlobRel(feature, targetHash)
	if err != nil {
		return err
	}
	switch crashPoint {
	case "before-removal":
		storage.fail["remove:"+path.Base(blobRel)] = 1
	case "after-removal":
		storage.postError["remove:"+path.Base(blobRel)] = errors.New("PIB-443 crash after unlink")
	case "after-tombstone":
		storage.hooks["after-index-cas"] = func(memory *archiveMemoryStorage) {
			if intentArchiveHashAllTombstoned(memory.decodedIndex(t, feature), targetHash) {
				delete(memory.hooks, "after-index-cas")
				memory.fail["sync:intent-archive"] = 1
			}
		}
	default:
		return fmt.Errorf("unknown crash point %q", crashPoint)
	}

	result, executeErr := ExecuteIntentArchivePurge(storage, plan)
	typed := s7PIB443ArchiveError(executeErr)
	if typed == nil || typed.Code != IntentArchiveCodePurgePartial || typed.ExitClass != 5 ||
		!result.Committed {
		return fmt.Errorf("%s/%s/hash-%d crash result=%+v error=%+v calls=%v",
			selectorKind, crashPoint, targetIndex, result, typed, storage.calls)
	}
	crashIndex := storage.decodedIndex(t, feature)
	if err := ValidateIntentArchiveIndex(crashIndex, feature); err != nil {
		return fmt.Errorf("strict crash index decode: %w", err)
	}
	switch crashPoint {
	case "before-removal":
		if !intentArchiveHashHasState(crashIndex, targetHash, IntentArchiveWireRemovalPending) ||
			!s7PIB443BlobExists(storage, blobRel) {
			return fmt.Errorf("before-removal evidence lacks pending+present: index=%+v blobs=%v", crashIndex, storage.blobs)
		}
	case "after-removal":
		if !intentArchiveHashHasState(crashIndex, targetHash, IntentArchiveWireRemovalPending) ||
			s7PIB443BlobExists(storage, blobRel) {
			return fmt.Errorf("after-removal evidence lacks pending+absent: index=%+v blobs=%v", crashIndex, storage.blobs)
		}
	case "after-tombstone":
		if !intentArchiveHashAllTombstoned(crashIndex, targetHash) ||
			s7PIB443BlobExists(storage, blobRel) {
			return fmt.Errorf("after-tombstone evidence is not tombstoned+absent: index=%+v blobs=%v", crashIndex, storage.blobs)
		}
	}
	snapshot, err := CaptureIntentArchive(storage, feature)
	if err != nil {
		return fmt.Errorf("strict X11 crash capture: %w", err)
	}
	if len(snapshot.Inspection.Hashes) != 2 {
		return fmt.Errorf("strict X11 crash capture lost selected hashes: %+v", snapshot.Inspection)
	}

	storage.calls = nil
	recovery, err := RecoverPendingPurge(storage, feature)
	if err != nil {
		return fmt.Errorf("pending recovery: %w", err)
	}
	switch crashPoint {
	case "before-removal":
		if recovery.Outcome != IntentArchivePurgeRecovered ||
			s7PIB443FirstMutation(storage.calls) != "remove" {
			return fmt.Errorf("before-removal next action=%q recovery=%+v calls=%v",
				s7PIB443FirstMutation(storage.calls), recovery, storage.calls)
		}
	case "after-removal":
		if recovery.Outcome != IntentArchivePurgeRecovered ||
			s7PIB443FirstMutation(storage.calls) != "cas-index" ||
			callIndex(storage.calls, "remove:") >= 0 {
			return fmt.Errorf("after-removal next action=%q recovery=%+v calls=%v",
				s7PIB443FirstMutation(storage.calls), recovery, storage.calls)
		}
	case "after-tombstone":
		if recovery.Outcome != IntentArchivePurgeNoOp || s7PIB443FirstMutation(storage.calls) != "" {
			return fmt.Errorf("after-tombstone recovery repeated a completed hash: %+v calls=%v", recovery, storage.calls)
		}
	}

	retryPlan, err := PlanIntentArchivePurge(storage, feature, selector, true)
	if err != nil {
		return fmt.Errorf("same-selector retry plan: %w", err)
	}
	if retryPlan.SelectorKind != plan.SelectorKind {
		return fmt.Errorf("same selector changed from %q to %q", plan.SelectorKind, retryPlan.SelectorKind)
	}
	if _, err := ExecuteIntentArchivePurge(storage, retryPlan); err != nil {
		return fmt.Errorf("same-selector retry execute: %w", err)
	}
	final := storage.decodedIndex(t, feature)
	if err := ValidateIntentArchiveIndex(final, feature); err != nil {
		return fmt.Errorf("strict final index decode: %w", err)
	}
	for _, hash := range plan.Hashes {
		rel, _ := IntentArchiveBlobRel(feature, hash)
		if !intentArchiveHashAllTombstoned(final, hash) || s7PIB443BlobExists(storage, rel) {
			return fmt.Errorf("selected hash %s remained live after retry: index=%+v blobs=%v", hash, final, storage.blobs)
		}
	}
	finalSnapshot, err := CaptureIntentArchive(storage, feature)
	if err != nil || !finalSnapshot.Inspection.Consistent {
		return fmt.Errorf("strict X11 final capture=%+v err=%v", finalSnapshot.Inspection, err)
	}
	return nil
}

func s7PIB443ArchiveError(err error) *IntentArchiveError {
	var typed *IntentArchiveError
	if errors.As(err, &typed) {
		return typed
	}
	return nil
}

func s7PIB443BlobExists(storage *archiveMemoryStorage, rel string) bool {
	_, exists := storage.blobs[rel]
	return exists
}

func s7PIB443FirstMutation(calls []string) string {
	for _, call := range calls {
		switch {
		case strings.HasPrefix(call, "remove:"):
			return "remove"
		case call == "cas-index":
			return "cas-index"
		}
	}
	return ""
}

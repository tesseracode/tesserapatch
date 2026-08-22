//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type s7AQArchiveMutationSpy struct {
	store.IntentArchiveStorage
	writes *int
}

func (storage *s7AQArchiveMutationSpy) PublishBlob(
	rel, hash string,
	data []byte,
) (store.IntentArchiveMutationResult, error) {
	*storage.writes++
	return storage.IntentArchiveStorage.PublishBlob(rel, hash, data)
}

func (storage *s7AQArchiveMutationSpy) CASIndex(
	rel string,
	expected store.IntentArchiveIdentityToken,
	canonical []byte,
) (store.IntentArchiveMutationResult, error) {
	*storage.writes++
	return storage.IntentArchiveStorage.CASIndex(rel, expected, canonical)
}

func (storage *s7AQArchiveMutationSpy) RemoveBlob(
	rel string,
	expected store.IntentArchiveIdentityToken,
) (store.IntentArchiveMutationResult, error) {
	*storage.writes++
	return storage.IntentArchiveStorage.RemoveBlob(rel, expected)
}

func (storage *s7AQArchiveMutationSpy) SyncDirectory(rel string) error {
	*storage.writes++
	return storage.IntentArchiveStorage.SyncDirectory(rel)
}

func TestS7AQPendingPurgeContracts(t *testing.T) {
	t.Run("PIB-486", func(t *testing.T) {
		fixture := s7AQPendingArchiveFixture(t)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", fixture.root, "feature", "intent-archive", "purge", fixture.slug,
			"--blob", fixture.selectorHash, "--yes", "--json", "--quiet",
		)
		report := decodeIntentArchivePurgeReport(t, stdout)
		_, index := readIntentArchiveCLIIndex(t, fixture.root, fixture.slug)
		expectedIndex := s7AQIndexAfterHashes(
			fixture.index, fixture.pendingHashes,
		)
		if code != 0 || stderr != "" ||
			report.Outcome != string(store.IntentArchivePurgeRecovered) ||
			report.Recovery == nil ||
			report.Recovery.Kind != "archive-purge-finalize" ||
			!reflect.DeepEqual(report.Recovery.FinalizedHashes, fixture.pendingHashes) ||
			len(report.Hashes) != 0 || len(report.GenerationIDs) != 0 ||
			len(report.References) != 0 || len(report.Blobs) != 0 ||
			!reflect.DeepEqual(index, expectedIndex) ||
			!reflect.DeepEqual(
				s7AQArchiveBlobHashes(t, fixture.root, fixture.slug),
				s7AQSortedStrings([]string{fixture.selectorHash, fixture.unrelatedHash}),
			) ||
			!s7AQArchiveBlobsMatch(
				t, fixture, []string{fixture.selectorHash, fixture.unrelatedHash},
			) {
			t.Fatalf("PIB-486 terminal purge recovery = exit:%d stderr:%q report:%+v index:%+v",
				code, stderr, report, index)
		}
		selectorRel, _ := store.IntentArchiveBlobRel(fixture.slug, fixture.selectorHash)
		if _, err := os.Stat(filepath.Join(fixture.root, filepath.FromSlash(selectorRel))); err != nil {
			t.Fatalf("PIB-486 selector was processed during recovery: %v", err)
		}

		fixture = s7AQPendingArchiveFixture(t)
		before := snapshotTreeMetadata(t, "PIB-486 prepare", filepath.Join(fixture.root, ".tpatch"))
		code, stdout, stderr, _ = runPrepare(
			t, "--path", fixture.root, "prepare", fixture.slug, "--json", "--quiet",
		)
		prepareReport := prepareS4Report(t, stdout)
		after := snapshotTreeMetadata(t, "PIB-486 prepare", filepath.Join(fixture.root, ".tpatch"))
		if code != 3 || stderr == "" || prepareReport.Refusal == nil ||
			prepareReport.Refusal.Code != "recovery-pending" ||
			!s7AQContainsEvery(prepareReport.Refusal.Retry, fixture.pendingHashes) ||
			before != after {
			t.Fatalf("PIB-486 prepare pending owner = exit:%d stderr:%q report:%+v changed:%t",
				code, stderr, prepareReport, before != after)
		}
	})

	t.Run("PIB-489", func(t *testing.T) {
		observation := s7AQObservePendingJournalPurge(t, true)
		if observation.code != 3 || observation.stderr == "" ||
			observation.report.Refusal == nil ||
			observation.report.Refusal.Code != "recovery-pending" ||
			observation.before != observation.after ||
			observation.journalMarkers != 1 ||
			observation.journalDecodes != 0 ||
			observation.journalRenames != 0 ||
			observation.archiveWrites != 0 {
			t.Fatalf("PIB-489 confirmed purge pending journal = %+v", observation)
		}
	})

	t.Run("PIB-490", func(t *testing.T) {
		observation := s7AQObservePendingJournalPurge(t, false)
		if observation.code != 3 || observation.stderr == "" ||
			observation.report.Refusal == nil ||
			observation.report.Refusal.Code != "recovery-pending" ||
			observation.before != observation.after ||
			observation.authorityAcquires != 0 ||
			observation.journalMarkers != 1 ||
			observation.journalDecodes != 0 ||
			observation.journalRenames != 0 ||
			observation.archiveWrites != 0 ||
			len(observation.report.Hashes) != 0 ||
			len(observation.report.References) != 0 ||
			len(observation.report.Blobs) != 0 ||
			strings.Contains(observation.stdout, `"pending_purge"`) ||
			strings.Contains(strings.ToLower(observation.stdout), "plan:") {
			t.Fatalf("PIB-490 preview pending journal = %+v", observation)
		}
	})

	t.Run("PIB-491", func(t *testing.T) {
		fixture := s7AQPendingArchiveFixture(t)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", fixture.root, "feature", "intent-archive", "purge", fixture.slug,
			"--blob", fixture.selectorHash, "--yes", "--json", "--quiet",
		)
		recovered := decodeIntentArchivePurgeReport(t, stdout)
		_, recoveredIndex := readIntentArchiveCLIIndex(t, fixture.root, fixture.slug)
		if code != 0 || stderr != "" ||
			recovered.Outcome != string(store.IntentArchivePurgeRecovered) ||
			recovered.Recovery == nil ||
			!reflect.DeepEqual(recovered.Recovery.FinalizedHashes, fixture.pendingHashes) ||
			len(recovered.Hashes) != 0 ||
			strings.Contains(recovered.Recovery.Retry, fixture.root) ||
			recovered.Recovery.RetryCWD != "workspace-root" ||
			!reflect.DeepEqual(
				recoveredIndex,
				s7AQIndexAfterHashes(fixture.index, fixture.pendingHashes),
			) {
			t.Fatalf("PIB-491 terminal pending recovery = exit:%d stderr:%q report:%+v",
				code, stderr, recovered)
		}
		argv, err := s7APParseRenderedCommand(recovered.Recovery.Retry)
		if err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr = s7APRunFromWorkspace(t, fixture.root, argv)
		completed := decodeIntentArchivePurgeReport(t, stdout)
		_, completedIndex := readIntentArchiveCLIIndex(t, fixture.root, fixture.slug)
		expectedCompleted := s7AQIndexAfterHashes(
			fixture.index,
			append(append([]string{}, fixture.pendingHashes...), fixture.selectorHash),
		)
		if code != 0 || stderr != "" ||
			completed.Outcome != string(store.IntentArchivePurgePurged) ||
			completed.Recovery != nil ||
			!reflect.DeepEqual(completed.Hashes, []string{fixture.selectorHash}) ||
			!reflect.DeepEqual(completedIndex, expectedCompleted) ||
			!reflect.DeepEqual(
				s7AQArchiveBlobHashes(t, fixture.root, fixture.slug),
				[]string{fixture.unrelatedHash},
			) ||
			!s7AQArchiveBlobsMatch(t, fixture, []string{fixture.unrelatedHash}) {
			t.Fatalf("PIB-491 sanitized retry = exit:%d stderr:%q report:%+v",
				code, stderr, completed)
		}
	})
}

type s7AQPendingArchiveState struct {
	root           string
	slug           string
	pendingHashes  []string
	selectorHash   string
	unrelatedHash  string
	index          store.IntentArchiveIndex
	blobIdentities map[string]intentpub.Identity
}

func s7AQPendingArchiveFixture(t *testing.T) s7AQPendingArchiveState {
	t.Helper()
	root, slug := prepareS4Workspace(t, "AQ pending archive")
	pendingABytes := []byte("AQ pending A bytes\n")
	pendingBBytes := []byte("AQ pending B bytes\n")
	selectorBytes := []byte("AQ selected retained bytes\n")
	unrelatedBytes := []byte("AQ unrelated retained bytes\n")
	pendingA1 := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactAnalysis,
		pendingABytes,
		store.IntentArchiveWireRemovalPending,
	)
	pendingA2 := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactSpec,
		pendingABytes,
		store.IntentArchiveWireRemovalPending,
	)
	pendingB := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactExploration,
		pendingBBytes,
		store.IntentArchiveWireRemovalPending,
	)
	selector := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactAnalysis,
		selectorBytes,
		store.IntentArchiveWireRetained,
	)
	unrelated := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactSpec,
		unrelatedBytes,
		store.IntentArchiveWireRetained,
	)
	generations := []store.IntentArchiveGeneration{
		intentArchiveCLIGeneration(t, slug, pendingA1),
		intentArchiveCLIGeneration(t, slug, pendingA2),
		intentArchiveCLIGeneration(t, slug, pendingB),
		intentArchiveCLIGeneration(t, slug, selector),
		intentArchiveCLIGeneration(t, slug, unrelated),
	}
	sort.Slice(generations, func(i, j int) bool {
		return generations[i].GenerationID < generations[j].GenerationID
	})
	index := intentArchiveCLIIndex(t, slug, generations...)
	writeIntentArchiveCLIFixture(
		t,
		root,
		slug,
		index,
		map[string][]byte{
			pendingA1.ContentSHA256: pendingABytes,
			pendingB.ContentSHA256:  pendingBBytes,
			selector.ContentSHA256:  selectorBytes,
			unrelated.ContentSHA256: unrelatedBytes,
		},
	)
	pendingHashes := []string{pendingA1.ContentSHA256, pendingB.ContentSHA256}
	sort.Strings(pendingHashes)
	fixture := s7AQPendingArchiveState{
		root: root, slug: slug, pendingHashes: pendingHashes,
		selectorHash: selector.ContentSHA256, unrelatedHash: unrelated.ContentSHA256,
		index: index, blobIdentities: map[string]intentpub.Identity{},
	}
	for _, hash := range []string{
		pendingA1.ContentSHA256, pendingB.ContentSHA256,
		selector.ContentSHA256, unrelated.ContentSHA256,
	} {
		rel, err := store.IntentArchiveBlobRel(slug, hash)
		if err != nil {
			t.Fatal(err)
		}
		fixture.blobIdentities[hash] = s7APCaptureIdentity(t, root, rel)
	}
	return fixture
}

func s7AQIndexAfterHashes(
	index store.IntentArchiveIndex,
	hashes []string,
) store.IntentArchiveIndex {
	result := index
	result.Generations = append([]store.IntentArchiveGeneration(nil), index.Generations...)
	selected := map[string]bool{}
	for _, hash := range hashes {
		selected[hash] = true
	}
	for generationIndex := range result.Generations {
		generation := result.Generations[generationIndex]
		generation.Replaced = append([]store.IntentArchiveReplacement(nil), generation.Replaced...)
		for replacementIndex := range generation.Replaced {
			replacement := &generation.Replaced[replacementIndex]
			if selected[replacement.ContentSHA256] {
				replacement.Blob = ""
				replacement.PurgePending = false
				replacement.Purged = true
			}
		}
		result.Generations[generationIndex] = generation
	}
	return result
}

func s7AQArchiveBlobHashes(t *testing.T, root, slug string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(
		root, ".tpatch", "features", slug, "artifacts", "intent-archive", "blobs",
	))
	if err != nil {
		t.Fatal(err)
	}
	hashes := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected archive blob directory %s", entry.Name())
		}
		hashes = append(hashes, strings.TrimSuffix(entry.Name(), ".blob"))
	}
	sort.Strings(hashes)
	return hashes
}

func s7AQArchiveBlobsMatch(
	t *testing.T,
	fixture s7AQPendingArchiveState,
	present []string,
) bool {
	t.Helper()
	selected := map[string]bool{}
	for _, hash := range present {
		selected[hash] = true
	}
	for hash, before := range fixture.blobIdentities {
		rel, err := store.IntentArchiveBlobRel(fixture.slug, hash)
		if err != nil {
			t.Fatal(err)
		}
		after := s7APCaptureIdentity(t, fixture.root, rel)
		if selected[hash] {
			if !after.Equal(before) {
				return false
			}
		} else if after.Exists {
			return false
		}
	}
	return true
}

func s7AQContainsEvery(text string, values []string) bool {
	for _, value := range values {
		if !strings.Contains(text, value) {
			return false
		}
	}
	return true
}

type s7AQPendingJournalObservation struct {
	code              int
	stdout            string
	stderr            string
	report            intentArchivePurgeReport
	before            string
	after             string
	authorityAcquires int
	journalMarkers    int
	journalDecodes    int
	journalRenames    int
	archiveWrites     int
}

func s7AQObservePendingJournalPurge(
	t *testing.T,
	confirmed bool,
) s7AQPendingJournalObservation {
	t.Helper()
	root, slug := prepareS4Workspace(t, fmt.Sprintf("AQ pending journal %t", confirmed))
	data := []byte("AQ selected bytes\n")
	replacement := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactAnalysis,
		data,
		store.IntentArchiveWireRetained,
	)
	writeIntentArchiveCLIFixture(
		t,
		root,
		slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
		map[string][]byte{replacement.ContentSHA256: data},
	)
	journalRel := intentpub.JournalRel(slug)
	journalAbs := filepath.Join(root, filepath.FromSlash(journalRel))
	if err := os.MkdirAll(filepath.Dir(journalAbs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalAbs, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldJournals := intentArchiveJournals
	journalSpy := &intentArchiveJournalSpy{delegate: oldJournals}
	intentArchiveJournals = journalSpy
	oldAcquire := intentArchiveAcquireAuthority
	acquires := 0
	intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
		acquires++
		return oldAcquire(path)
	}
	oldStorage := intentArchiveNewStorage
	writes := 0
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rooted *os.Root,
	) store.IntentArchiveStorage {
		return &s7AQArchiveMutationSpy{
			IntentArchiveStorage: oldStorage(authority, rooted),
			writes:               &writes,
		}
	}
	defer func() {
		intentArchiveJournals = oldJournals
		intentArchiveAcquireAuthority = oldAcquire
		intentArchiveNewStorage = oldStorage
	}()

	before := snapshotTreeMetadata(t, "PIB-489/490", filepath.Join(root, ".tpatch"))
	args := []string{
		"--path", root, "feature", "intent-archive", "purge", slug,
		"--blob", replacement.ContentSHA256, "--json", "--quiet",
	}
	if confirmed {
		args = append(args, "--yes")
	}
	code, stdout, stderr, _ := runPrepare(t, args...)
	return s7AQPendingJournalObservation{
		code: code, stdout: stdout, stderr: stderr,
		report:            decodeIntentArchivePurgeReport(t, stdout),
		before:            before,
		after:             snapshotTreeMetadata(t, "PIB-489/490", filepath.Join(root, ".tpatch")),
		authorityAcquires: acquires,
		journalMarkers:    journalSpy.markers,
		journalDecodes:    journalSpy.decodes,
		journalRenames:    journalSpy.renames,
		archiveWrites:     writes,
	}
}

func s7AQBytesEqual(left, right []byte) bool {
	return bytes.Equal(left, right)
}

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func intentArchiveCLIWorkspace(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	repoStore, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	status, err := repoStore.AddFeature(store.AddFeatureInput{
		Title:   "Intent archive CLI",
		Request: "exercise the intent archive retention surface",
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, status.Slug
}

func intentArchiveCLIReplacement(
	t *testing.T,
	id store.IntentArchiveArtifactID,
	data []byte,
	state store.IntentArchiveWireState,
) store.IntentArchiveReplacement {
	t.Helper()
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	rel, err := store.IntentArchiveArtifactPath(id)
	if err != nil {
		t.Fatal(err)
	}
	replacement := store.IntentArchiveReplacement{
		ArtifactID:    id,
		Path:          rel,
		ContentSHA256: hash,
		SizeBytes:     int64(len(data)),
	}
	switch state {
	case store.IntentArchiveWireRetained:
		replacement.Blob = hash
	case store.IntentArchiveWireRemovalPending:
		replacement.Blob = hash
		replacement.PurgePending = true
	case store.IntentArchiveWireTombstoned:
		replacement.Purged = true
	default:
		t.Fatalf("unsupported wire state %q", state)
	}
	return replacement
}

func intentArchiveCLIGeneration(
	t *testing.T,
	slug string,
	replacements ...store.IntentArchiveReplacement,
) store.IntentArchiveGeneration {
	t.Helper()
	generationID, _, err := store.ComputeIntentArchiveGenerationID(slug, replacements)
	if err != nil {
		t.Fatal(err)
	}
	return store.IntentArchiveGeneration{
		GenerationID: generationID,
		Mode:         store.IntentArchiveModeRegenerate,
		Replaced:     replacements,
	}
}

func intentArchiveCLIIndex(
	t *testing.T,
	slug string,
	generations ...store.IntentArchiveGeneration,
) store.IntentArchiveIndex {
	t.Helper()
	if generations == nil {
		generations = []store.IntentArchiveGeneration{}
	}
	index := store.IntentArchiveIndex{
		SchemaVersion: store.IntentArchiveSchemaVersion,
		Feature:       slug,
		Generations:   generations,
	}
	if err := store.ValidateIntentArchiveIndex(index, slug); err != nil {
		t.Fatal(err)
	}
	return index
}

func writeIntentArchiveCLIFixture(
	t *testing.T,
	root, slug string,
	index store.IntentArchiveIndex,
	blobs map[string][]byte,
) {
	t.Helper()
	archive := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive")
	if err := os.MkdirAll(filepath.Join(archive, "blobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	encoded, err := store.EncodeIntentArchiveIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "index.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	for hash, data := range blobs {
		if err := os.WriteFile(filepath.Join(archive, "blobs", hash+".blob"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readIntentArchiveCLIIndex(t *testing.T, root, slug string) ([]byte, store.IntentArchiveIndex) {
	t.Helper()
	rel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	index, err := store.DecodeIntentArchiveIndex(raw, slug)
	if err != nil {
		t.Fatal(err)
	}
	return raw, index
}

func decodeIntentArchivePurgeReport(t *testing.T, output string) intentArchivePurgeReport {
	t.Helper()
	var report intentArchivePurgeReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("decode purge report: %v\n%s", err, output)
	}
	return report
}

func decodeIntentArchiveListReport(t *testing.T, output string) intentArchiveListReport {
	t.Helper()
	var report intentArchiveListReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("decode list report: %v\n%s", err, output)
	}
	return report
}

func assertIntentArchiveTpatchRetriesHeaded(t *testing.T, output string) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	for index, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "tpatch ") {
			continue
		}
		if index == 0 || lines[index-1] != prepareRetryHeader {
			t.Fatalf("bare tpatch command at line %d:\n%s", index+1, output)
		}
	}
}

func assertIntentArchiveNoGitTrace(t *testing.T, tracePath string) {
	t.Helper()
	if data, err := os.ReadFile(tracePath); err == nil {
		t.Fatalf("archive command spawned Git:\n%s", data)
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect Git process trace: %v", err)
	}
}

type intentArchiveJournalSpy struct {
	delegate intentArchiveJournalAccess
	markers  int
	decodes  int
	renames  int
}

func (spy *intentArchiveJournalSpy) Execute(
	root *os.Root,
	request intentArchiveJournalRequest,
) (intentArchiveJournalResult, error) {
	switch request.Operation {
	case intentArchiveJournalObserveMarker:
		spy.markers++
	case intentArchiveJournalDecode:
		spy.decodes++
	case intentArchiveJournalRename:
		spy.renames++
	}
	return spy.delegate.Execute(root, request)
}

type intentArchiveWriteSpyStorage struct {
	store.IntentArchiveStorage
	writes *int
}

func (storage *intentArchiveWriteSpyStorage) PublishBlob(
	blobRel, hash string,
	data []byte,
) (store.IntentArchiveMutationResult, error) {
	*storage.writes++
	return storage.IntentArchiveStorage.PublishBlob(blobRel, hash, data)
}

func (storage *intentArchiveWriteSpyStorage) CASIndex(
	indexRel string,
	expected store.IntentArchiveIdentityToken,
	canonical []byte,
) (store.IntentArchiveMutationResult, error) {
	*storage.writes++
	return storage.IntentArchiveStorage.CASIndex(indexRel, expected, canonical)
}

func (storage *intentArchiveWriteSpyStorage) RemoveBlob(
	blobRel string,
	expected store.IntentArchiveIdentityToken,
) (store.IntentArchiveMutationResult, error) {
	*storage.writes++
	return storage.IntentArchiveStorage.RemoveBlob(blobRel, expected)
}

func (storage *intentArchiveWriteSpyStorage) SyncDirectory(dirRel string) error {
	*storage.writes++
	return storage.IntentArchiveStorage.SyncDirectory(dirRel)
}

// PIB-345 and PIB-346: complete deterministic list truth with zero mutation.
func TestFeatureIntentArchiveListTruthAndZeroMutation(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	retainedBytes := []byte("retained archive bytes\n")
	orphanBytes := []byte("orphan archive bytes\n")
	retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, retainedBytes, store.IntentArchiveWireRetained)
	tombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, orphanBytes, store.IntentArchiveWireTombstoned)
	index := intentArchiveCLIIndex(t, slug,
		intentArchiveCLIGeneration(t, slug, retained),
		intentArchiveCLIGeneration(t, slug, tombstoned),
	)
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{
		retained.ContentSHA256:   retainedBytes,
		tombstoned.ContentSHA256: orphanBytes,
	})
	before := readTree(t, filepath.Join(root, ".tpatch"))
	acquires := 0
	writes := 0
	previousAcquire := intentArchiveAcquireAuthority
	previousFactory := intentArchiveNewStorage
	intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
		acquires++
		return previousAcquire(path)
	}
	intentArchiveNewStorage = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
		return &intentArchiveWriteSpyStorage{
			IntentArchiveStorage: previousFactory(authority, root),
			writes:               &writes,
		}
	}
	t.Cleanup(func() {
		intentArchiveAcquireAuthority = previousAcquire
		intentArchiveNewStorage = previousFactory
	})

	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("list = %d stderr=%q\n%s", code, stderr, stdout)
	}
	report := decodeIntentArchiveListReport(t, stdout)
	if report.Command != intentArchiveCommandList || report.Outcome != "listed" ||
		len(report.Generations) != 2 || len(report.Orphans) != 1 {
		t.Fatalf("list report = %#v", report)
	}
	first := report.Generations[0].Entries[0]
	if first.ContentSHA256 != retained.ContentSHA256 || first.SizeBytes != int64(len(retainedBytes)) ||
		!first.Present || first.Storage != "present" {
		t.Fatalf("retained entry = %#v", first)
	}
	second := report.Generations[1].Entries[0]
	if second.Storage != "orphan" || !second.Present ||
		second.Repair != "tpatch feature intent-archive purge "+slug+" --orphans --yes" {
		t.Fatalf("orphan entry = %#v", second)
	}
	if report.Orphans[0].Hash != tombstoned.ContentSHA256 ||
		report.Orphans[0].Path != ".tpatch/features/"+slug+"/artifacts/intent-archive/blobs/"+tombstoned.ContentSHA256+".blob" {
		t.Fatalf("orphans = %#v", report.Orphans)
	}
	humanCode, human, humanErr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug,
	)
	if humanCode != 0 || humanErr != "" ||
		!strings.Contains(human, "generation "+index.Generations[0].GenerationID) ||
		!strings.Contains(human, "blob: "+retained.ContentSHA256) ||
		!strings.Contains(human, "size: "+fmt.Sprint(len(retainedBytes))) ||
		!strings.Contains(human, "present: true") ||
		!strings.Contains(human, "\norphans:\n") {
		t.Fatalf("human list parity = %d stderr=%q\n%s", humanCode, humanErr, human)
	}
	assertIntentArchiveTpatchRetriesHeaded(t, human)
	if acquires != 0 || writes != 0 {
		t.Fatalf("list effects: authority=%d writes=%d", acquires, writes)
	}
	after := readTree(t, filepath.Join(root, ".tpatch"))
	if !bytes.Equal(before, after) {
		t.Fatal("list mutated the workspace")
	}
	lastKey := -1
	for _, key := range []string{
		`"schema_version"`, `"command"`, `"slug"`, `"outcome"`, `"index"`, `"generations"`, `"corrupt_objects"`, `"orphans"`,
	} {
		position := strings.Index(stdout, key)
		if position < 0 {
			t.Fatalf("fixed report key %s missing", key)
		}
		if position <= lastKey {
			t.Fatalf("fixed report key %s is out of order", key)
		}
		lastKey = position
	}
}

// PIB-347 and PIB-348: closed scope grammar and exact lock-free preview.
func TestFeatureIntentArchivePurgeScopeAndPreview(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	data := []byte("preview archive bytes\n")
	replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
	index := intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement))
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{replacement.ContentSHA256: data})

	t.Run("missing-scope-is-parse-error", func(t *testing.T) {
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
		)
		if code != 1 || stdout != "" {
			t.Fatalf("missing scope = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		for _, flag := range []string{"--blob", "--generation", "--orphans", "--all"} {
			if !strings.Contains(stderr, flag) {
				t.Fatalf("missing scope error omitted %s: %q", flag, stderr)
			}
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("missing-scope parse error mutated the workspace")
		}
	})

	t.Run("preview-is-exact-and-lock-free", func(t *testing.T) {
		before := readTree(t, filepath.Join(root, ".tpatch"))
		acquires := 0
		recoveries := 0
		writes := 0
		previousAcquire := intentArchiveAcquireAuthority
		previousFactory := intentArchiveNewStorage
		previousRecover := intentArchiveRecoverPurge
		intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return previousAcquire(path)
		}
		intentArchiveNewStorage = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
			return &intentArchiveWriteSpyStorage{
				IntentArchiveStorage: previousFactory(authority, root),
				writes:               &writes,
			}
		}
		intentArchiveRecoverPurge = func(
			storage store.IntentArchiveStorage,
			slug string,
		) (store.IntentArchivePurgeResult, error) {
			recoveries++
			return previousRecover(storage, slug)
		}
		t.Cleanup(func() {
			intentArchiveAcquireAuthority = previousAcquire
			intentArchiveNewStorage = previousFactory
			intentArchiveRecoverPurge = previousRecover
		})
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--all", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("preview = %d stderr=%q\n%s", code, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgePlanned) ||
			len(report.Hashes) != 1 || report.Hashes[0] != replacement.ContentSHA256 ||
			len(report.Blobs) != 1 || report.Blobs[0].Hash != replacement.ContentSHA256 ||
			report.Retry != "tpatch feature intent-archive purge "+slug+" --all --yes --json --quiet" ||
			report.RetryCWD != store.IntentArchiveRepairCWD {
			t.Fatalf("preview report = %#v", report)
		}
		if acquires != 0 || recoveries != 0 || writes != 0 {
			t.Fatalf("preview effects: authority=%d recovery=%d writes=%d", acquires, recoveries, writes)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("preview mutated the workspace")
		}
		code, confirmedOut, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--all", "--yes", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("confirmed preview plan = %d stderr=%q\n%s", code, stderr, confirmedOut)
		}
		confirmed := decodeIntentArchivePurgeReport(t, confirmedOut)
		if !reflect.DeepEqual(confirmed.Hashes, report.Hashes) ||
			len(confirmed.Blobs) != len(report.Blobs) ||
			!confirmed.Blobs[0].Removed ||
			confirmed.Blobs[0].Present {
			t.Fatalf("preview/confirmed mismatch: preview=%#v confirmed=%#v", report, confirmed)
		}
		if acquires != 1 || recoveries != 1 || writes == 0 {
			t.Fatalf("confirmed effects: authority=%d recovery=%d writes=%d", acquires, recoveries, writes)
		}
	})
}

// PIB-349 and PIB-350: one real authority and marker-only journal refusal.
func TestFeatureIntentArchivePurgeAuthorityAndPendingJournal(t *testing.T) {
	requireIntentArchiveAuthority(t)
	t.Run("contention", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("contention\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: data},
		)
		authority, err := intentlock.Acquire(root)
		if err != nil {
			t.Fatal(err)
		}
		defer authority.Release()
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--all", "--yes", "--json", "--quiet",
		)
		if code != 3 {
			t.Fatalf("contention exit = %d\n%s", code, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Refusal == nil || report.Refusal.Code != "transaction-in-progress" {
			t.Fatalf("contention report = %#v", report)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("contention changed the workspace")
		}
	})

	for _, confirmed := range []bool{false, true} {
		name := "preview"
		if confirmed {
			name = "confirmed"
		}
		t.Run("pending-journal-"+name, func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
			if err := os.MkdirAll(lane, 0o700); err != nil {
				t.Fatal(err)
			}
			journal := filepath.Join(lane, "journal.json")
			original := []byte("{not-json")
			if err := os.WriteFile(journal, original, 0o600); err != nil {
				t.Fatal(err)
			}
			before := readTree(t, filepath.Join(root, ".tpatch"))
			previousJournals := intentArchiveJournals
			spy := &intentArchiveJournalSpy{delegate: previousJournals}
			intentArchiveJournals = spy
			t.Cleanup(func() { intentArchiveJournals = previousJournals })
			args := []string{
				"--path", root, "feature", "intent-archive", "purge", slug,
				"--all", "--json", "--quiet",
			}
			if confirmed {
				args = append(args, "--yes")
			}
			code, stdout, _, _ := runPrepare(t, args...)
			if code != 3 {
				t.Fatalf("pending journal exit = %d\n%s", code, stdout)
			}
			report := decodeIntentArchivePurgeReport(t, stdout)
			if report.Refusal == nil || report.Refusal.Code != "recovery-pending" ||
				report.Refusal.Retry != "tpatch prepare "+slug ||
				!strings.Contains(report.Refusal.Remediation, "--abandon-transaction --yes") {
				t.Fatalf("pending journal report = %#v", report)
			}
			if spy.markers != 1 || spy.decodes != 0 || spy.renames != 0 {
				t.Fatalf("pending journal operations: markers=%d decodes=%d renames=%d", spy.markers, spy.decodes, spy.renames)
			}
			got, err := os.ReadFile(journal)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("journal changed: %v %q", err, got)
			}
			if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
				t.Fatal("pending-journal refusal changed the workspace")
			}
		})
	}

	t.Run("pending-journal-list", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("list ignores prepare journal\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: data},
		)
		journalAbs := filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug)))
		if err := os.MkdirAll(filepath.Dir(journalAbs), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(journalAbs, []byte("{not-json"), 0o600); err != nil {
			t.Fatal(err)
		}
		previousJournals := intentArchiveJournals
		spy := &intentArchiveJournalSpy{delegate: previousJournals}
		intentArchiveJournals = spy
		t.Cleanup(func() { intentArchiveJournals = previousJournals })
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("pending journal list = %d\n%s", code, stdout)
		}
		if spy.decodes != 0 || spy.renames != 0 {
			t.Fatalf("list journal operations: decodes=%d renames=%d", spy.decodes, spy.renames)
		}
	})

	t.Run("ownership-is-global-over-every-reference", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		expected := []byte("global owned bytes\n")
		pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, expected, store.IntentArchiveWireRemovalPending)
		retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, expected, store.IntentArchiveWireRetained)
		tombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, expected, store.IntentArchiveWireTombstoned)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug,
				intentArchiveCLIGeneration(t, slug, pending),
				intentArchiveCLIGeneration(t, slug, retained),
				intentArchiveCLIGeneration(t, slug, tombstoned),
			),
			map[string][]byte{pending.ContentSHA256: []byte("global wrong bytes\n")},
		)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug,
			"--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("global owned list = %d\n%s", code, stdout)
		}
		report := decodeIntentArchiveListReport(t, stdout)
		if len(report.Generations) != 3 || report.Refusal != nil {
			t.Fatalf("global owned report = %#v", report)
		}
		wantRepair := "tpatch feature intent-archive purge " + slug + " --blob " + pending.ContentSHA256 + " --yes"
		for _, generation := range report.Generations {
			entry := generation.Entries[0]
			if entry.Storage != "pending-remove" ||
				entry.Repair != wantRepair ||
				strings.Contains(entry.Availability, "not recoverable") {
				t.Fatalf("owned reference lost global ownership: %#v", entry)
			}
		}
		if strings.Contains(stdout, `"storage": "corrupt"`) ||
			strings.Contains(stdout, "mixed-reference") ||
			strings.Contains(stdout, "dangling") {
			t.Fatalf("owned hash was classified as a non-owned repair class:\n%s", stdout)
		}
	})
}

func requireIntentArchiveAuthority(t *testing.T) {
	t.Helper()
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
}

func TestFeatureIntentArchiveJournalOperationSeamSensitivity(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	journalAbs := filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug)))
	if err := os.MkdirAll(filepath.Dir(journalAbs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalAbs, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer rootHandle.Close()
	previousJournals := intentArchiveJournals
	spy := &intentArchiveJournalSpy{delegate: previousJournals}
	intentArchiveJournals = spy
	t.Cleanup(func() { intentArchiveJournals = previousJournals })
	if _, err := decodeIntentArchiveJournal(rootHandle, slug); err == nil {
		t.Fatal("invalid sensitivity journal unexpectedly decoded")
	}
	oldRel := ".tpatch/local/intent-prepare/" + slug + "/rename-source"
	newRel := ".tpatch/local/intent-prepare/" + slug + "/rename-destination"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(oldRel)), []byte("rename"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := renameIntentArchiveJournal(rootHandle, oldRel, newRel); err != nil {
		t.Fatal(err)
	}
	if spy.decodes != 1 || spy.renames != 1 {
		t.Fatalf("journal seam sensitivity: decodes=%d renames=%d", spy.decodes, spy.renames)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(newRel))); err != nil {
		t.Fatalf("journal rename sensitivity did not use real operation: %v", err)
	}
}

// PIB-351, PIB-354, PIB-355, PIB-357, PIB-358, and PIB-360.
func TestFeatureIntentArchivePurgeIdempotencyGenerationAllAndSharedHash(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	t.Run("blob-idempotency-and-secret-remediation", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		secret := []byte("accidentally retained value\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, secret, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: secret},
		)
		if code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
		); code != 0 || !strings.Contains(stdout, replacement.ContentSHA256) {
			t.Fatalf("list before secret purge = %d\n%s", code, stdout)
		}
		args := []string{
			"--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", replacement.ContentSHA256, "--yes", "--json", "--quiet",
		}
		if code, stdout, _, _ := runPrepare(t, args...); code != 0 {
			t.Fatalf("first purge = %d\n%s", code, stdout)
		}
		firstRaw, firstIndex := readIntentArchiveCLIIndex(t, root, slug)
		if firstIndex.Generations[0].Replaced[0].WireState() != store.IntentArchiveWireTombstoned {
			t.Fatalf("first purge index = %#v", firstIndex)
		}
		blobRel, _ := store.IntentArchiveBlobRel(slug, replacement.ContentSHA256)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel))); !os.IsNotExist(err) {
			t.Fatalf("secret blob still exists: %v", err)
		}
		if code, stdout, _, _ := runPrepare(t, args...); code != 0 {
			t.Fatalf("second purge = %d\n%s", code, stdout)
		}
		secondRaw, secondIndex := readIntentArchiveCLIIndex(t, root, slug)
		if !bytes.Equal(firstRaw, secondRaw) {
			t.Fatal("idempotent second purge rewrote index.json")
		}
		if err := store.ValidateIntentArchiveIndex(secondIndex, slug); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("generation-preserves-identity", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		analysisBytes := []byte("generation analysis\n")
		specBytes := []byte("generation spec\n")
		analysis := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, analysisBytes, store.IntentArchiveWireRetained)
		spec := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, specBytes, store.IntentArchiveWireRetained)
		generation := intentArchiveCLIGeneration(t, slug, analysis, spec)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, generation),
			map[string][]byte{analysis.ContentSHA256: analysisBytes, spec.ContentSHA256: specBytes},
		)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--generation", generation.GenerationID, "--yes", "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("generation purge = %d\n%s", code, stdout)
		}
		_, got := readIntentArchiveCLIIndex(t, root, slug)
		if got.Generations[0].GenerationID != generation.GenerationID {
			t.Fatalf("generation id changed: %#v", got)
		}
		for _, replacement := range got.Generations[0].Replaced {
			if replacement.WireState() != store.IntentArchiveWireTombstoned ||
				replacement.ContentSHA256 == "" {
				t.Fatalf("generation entry not tombstoned immutably: %#v", replacement)
			}
		}
	})

	t.Run("shared-hash-global-claim-and-all", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		sharedBytes := []byte("shared bytes\n")
		analysis := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, sharedBytes, store.IntentArchiveWireRetained)
		spec := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, sharedBytes, store.IntentArchiveWireRetained)
		g1 := intentArchiveCLIGeneration(t, slug, analysis)
		g2 := intentArchiveCLIGeneration(t, slug, spec)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, g1, g2),
			map[string][]byte{analysis.ContentSHA256: sharedBytes},
		)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--generation", g1.GenerationID, "--yes", "--json", "--quiet",
		)
		if code != 3 {
			t.Fatalf("shared generation exit = %d\n%s", code, stdout)
		}
		sharedReport := decodeIntentArchivePurgeReport(t, stdout)
		if sharedReport.Refusal == nil || sharedReport.Refusal.Code != string(store.IntentArchiveCodeBlobShared) ||
			!strings.Contains(sharedReport.Refusal.Retry, "--blob "+analysis.ContentSHA256) ||
			!strings.Contains(sharedReport.Refusal.Remediation, "--all") {
			t.Fatalf("shared refusal = %#v", sharedReport)
		}
		code, stdout, _, _ = runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", analysis.ContentSHA256, "--yes", "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("shared blob purge = %d\n%s", code, stdout)
		}
		_, got := readIntentArchiveCLIIndex(t, root, slug)
		for _, generation := range got.Generations {
			if generation.Replaced[0].WireState() != store.IntentArchiveWireTombstoned {
				t.Fatalf("shared reference survived: %#v", generation)
			}
		}

		rootSharedAll, slugSharedAll := intentArchiveCLIWorkspace(t)
		sharedAllAnalysis := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, sharedBytes, store.IntentArchiveWireRetained)
		sharedAllSpec := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, sharedBytes, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, rootSharedAll, slugSharedAll,
			intentArchiveCLIIndex(t, slugSharedAll,
				intentArchiveCLIGeneration(t, slugSharedAll, sharedAllAnalysis),
				intentArchiveCLIGeneration(t, slugSharedAll, sharedAllSpec),
			),
			map[string][]byte{sharedAllAnalysis.ContentSHA256: sharedBytes},
		)
		code, stdout, _, _ = runPrepare(
			t, "--path", rootSharedAll, "feature", "intent-archive", "purge", slugSharedAll,
			"--all", "--yes", "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("shared --all purge = %d\n%s", code, stdout)
		}
		_, sharedAllIndex := readIntentArchiveCLIIndex(t, rootSharedAll, slugSharedAll)
		for _, generation := range sharedAllIndex.Generations {
			if generation.Replaced[0].WireState() != store.IntentArchiveWireTombstoned {
				t.Fatalf("shared --all retained reference: %#v", generation)
			}
		}

		rootAll, slugAll := intentArchiveCLIWorkspace(t)
		aBytes := []byte("all-a\n")
		sBytes := []byte("all-s\n")
		a := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, aBytes, store.IntentArchiveWireRetained)
		s := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, sBytes, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, rootAll, slugAll,
			intentArchiveCLIIndex(t, slugAll, intentArchiveCLIGeneration(t, slugAll, a, s)),
			map[string][]byte{a.ContentSHA256: aBytes, s.ContentSHA256: sBytes},
		)
		code, stdout, _, _ = runPrepare(
			t, "--path", rootAll, "feature", "intent-archive", "purge", slugAll,
			"--all", "--yes", "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("--all purge = %d\n%s", code, stdout)
		}
		_, allIndex := readIntentArchiveCLIIndex(t, rootAll, slugAll)
		for _, replacement := range allIndex.Generations[0].Replaced {
			if replacement.WireState() != store.IntentArchiveWireTombstoned {
				t.Fatalf("--all retained entry: %#v", replacement)
			}
		}
		entries, err := os.ReadDir(filepath.Join(rootAll, ".tpatch", "features", slugAll, "artifacts", "intent-archive", "blobs"))
		if err != nil || len(entries) != 0 {
			t.Fatalf("--all blobs = %v %v", entries, err)
		}
	})
}

// Later corrections: preview zero lock, selector-preserving retry, and
// selector-independent pending recovery that returns terminally.
func TestFeatureIntentArchivePendingRecoveryIsTerminalForEverySelector(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	root, slug := intentArchiveCLIWorkspace(t)
	pendingBytes := []byte("pending owned bytes\n")
	orphanBytes := []byte("separate orphan bytes\n")
	pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, pendingBytes, store.IntentArchiveWireRemovalPending)
	index := intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, pending))
	orphanSum := sha256.Sum256(orphanBytes)
	orphanHash := hex.EncodeToString(orphanSum[:])
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{
		pending.ContentSHA256: pendingBytes,
		orphanHash:            orphanBytes,
	})

	acquires := 0
	previousAcquire := intentArchiveAcquireAuthority
	intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
		acquires++
		return previousAcquire(path)
	}
	t.Cleanup(func() { intentArchiveAcquireAuthority = previousAcquire })

	previewBefore := readTree(t, filepath.Join(root, ".tpatch"))
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("pending preview = %d stderr=%q\n%s", code, stderr, stdout)
	}
	preview := decodeIntentArchivePurgeReport(t, stdout)
	if preview.Outcome != string(store.IntentArchivePurgeRecoveryRequired) ||
		preview.PendingPurge == nil ||
		len(preview.PendingPurge.PendingHashes) != 1 ||
		preview.PendingPurge.PendingHashes[0].Hash != pending.ContentSHA256 ||
		preview.PendingPurge.Selector != string(store.IntentArchiveSelectorOrphans) ||
		preview.PendingPurge.Retry != "tpatch feature intent-archive purge "+slug+" --orphans --yes --json --quiet" {
		t.Fatalf("pending preview report = %#v", preview)
	}
	if acquires != 0 || !bytes.Equal(previewBefore, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatalf("pending preview effects: authority=%d", acquires)
	}
	humanCode, human, humanErr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug, "--orphans",
	)
	wantRetryBlock := "\n" + prepareRetryHeader + "\n  tpatch feature intent-archive purge " + slug + " --orphans --yes\n"
	if humanCode != 0 || humanErr != "" ||
		!strings.Contains(human, "pending hash: "+pending.ContentSHA256) ||
		!strings.Contains(human, "plan:  "+intentArchivePendingPlan) ||
		!strings.Contains(human, wantRetryBlock) {
		t.Fatalf("pending human parity = %d stderr=%q\n%s", humanCode, humanErr, human)
	}

	code, stdout, stderr, _ = runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--yes", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("terminal recovery = %d stderr=%q\n%s", code, stderr, stdout)
	}
	recovered := decodeIntentArchivePurgeReport(t, stdout)
	if recovered.Outcome != string(store.IntentArchivePurgeRecovered) ||
		recovered.Recovery == nil ||
		recovered.Recovery.Kind != "archive-purge-finalize" ||
		len(recovered.Recovery.FinalizedHashes) != 1 ||
		recovered.Recovery.FinalizedHashes[0] != pending.ContentSHA256 {
		t.Fatalf("recovery report = %#v", recovered)
	}
	if acquires != 1 {
		t.Fatalf("confirmed recovery authority acquisitions = %d", acquires)
	}
	orphanRel, _ := store.IntentArchiveBlobRel(slug, orphanHash)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(orphanRel))); err != nil {
		t.Fatalf("terminal recovery processed the new orphan selector: %v", err)
	}
	_, recoveredIndex := readIntentArchiveCLIIndex(t, root, slug)
	if recoveredIndex.Generations[0].Replaced[0].WireState() != store.IntentArchiveWireTombstoned {
		t.Fatalf("pending hash not finalized: %#v", recoveredIndex)
	}

	code, stdout, _, _ = runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--yes", "--json", "--quiet",
	)
	if code != 0 {
		t.Fatalf("orphan rerun = %d\n%s", code, stdout)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(orphanRel))); !os.IsNotExist(err) {
		t.Fatalf("orphan rerun did not remove orphan: %v", err)
	}
}

func TestFeatureIntentArchivePendingPreviewSuppressesLowerPrecedenceTruth(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	pendingBytes := []byte("bounded pending\n")
	orphanBytes := []byte("hidden lower precedence orphan\n")
	pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, pendingBytes, store.IntentArchiveWireRemovalPending)
	orphanSum := sha256.Sum256(orphanBytes)
	orphanHash := hex.EncodeToString(orphanSum[:])
	writeIntentArchiveCLIFixture(t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, pending)),
		map[string][]byte{
			pending.ContentSHA256: pendingBytes,
			orphanHash:            orphanBytes,
		},
	)
	before := readTree(t, filepath.Join(root, ".tpatch"))
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--all", "--json", "--quiet",
	)
	if code != 0 {
		t.Fatalf("bounded pending preview = %d\n%s", code, stdout)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	if report.Outcome != string(store.IntentArchivePurgeRecoveryRequired) ||
		report.PendingPurge == nil ||
		len(report.PendingPurge.PendingHashes) != 1 ||
		len(report.Hashes) != 0 ||
		len(report.GenerationIDs) != 0 ||
		len(report.References) != 0 ||
		len(report.Blobs) != 0 ||
		len(report.OrphanBlobs) != 0 ||
		len(report.Advisories) != 0 ||
		report.RemainingRepairs != nil ||
		strings.Contains(stdout, `"remaining_repairs"`) ||
		strings.Contains(stdout, orphanHash) {
		t.Fatalf("pending preview leaked lower-precedence truth: %#v\n%s", report, stdout)
	}
	humanCode, human, humanErr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug, "--all",
	)
	if humanCode != 0 || humanErr != "" ||
		strings.Contains(human, "orphan blobs:") ||
		strings.Contains(human, "Remaining repair") ||
		strings.Contains(human, orphanHash) {
		t.Fatalf("pending human leaked lower-precedence truth = %d stderr=%q\n%s", humanCode, humanErr, human)
	}
	assertIntentArchiveTpatchRetriesHeaded(t, human)
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("bounded pending preview changed the workspace")
	}
}

func TestFeatureIntentArchiveTerminalRecoveryPerformsNoPostRecoveryCapture(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	root, slug := intentArchiveCLIWorkspace(t)
	data := []byte("terminal bounded recovery\n")
	pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRemovalPending)
	writeIntentArchiveCLIFixture(t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, pending)),
		map[string][]byte{pending.ContentSHA256: data},
	)
	captures := 0
	previousCapture := intentArchiveCapture
	intentArchiveCapture = func(store.IntentArchiveStorage, string) (store.IntentArchiveSnapshot, error) {
		captures++
		return store.IntentArchiveSnapshot{}, errors.New("injected forbidden post-recovery capture")
	}
	t.Cleanup(func() { intentArchiveCapture = previousCapture })
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--all", "--yes", "--json", "--quiet",
	)
	if code != 0 {
		t.Fatalf("terminal bounded recovery = %d\n%s", code, stdout)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	if report.Outcome != string(store.IntentArchivePurgeRecovered) ||
		report.Recovery == nil ||
		len(report.Recovery.FinalizedHashes) != 1 ||
		captures != 0 {
		t.Fatalf("terminal recovery report=%#v captures=%d", report, captures)
	}
}

func TestFeatureIntentArchiveSelectorGatePrecedence(t *testing.T) {
	const malformed = "unsafe;$(touch selector-leak)"

	t.Run("workspace-precedes-selector", func(t *testing.T) {
		for _, confirmed := range []bool{false, true} {
			name := "preview"
			if confirmed {
				name = "confirmed"
			}
			t.Run(name, func(t *testing.T) {
				args := []string{
					"--path", t.TempDir(), "feature", "intent-archive", "purge", "missing-workspace",
					"--blob", malformed, "--json", "--quiet",
				}
				if confirmed {
					args = append(args, "--yes")
				}
				code, stdout, _, _ := runPrepare(t, args...)
				report := decodeIntentArchivePurgeReport(t, stdout)
				if code != 3 ||
					report.Refusal == nil ||
					report.Refusal.Code != "workspace-not-initialized" ||
					strings.Contains(stdout, malformed) {
					t.Fatalf("workspace precedence = %d report=%#v\n%s", code, report, stdout)
				}
			})
		}
	})

	t.Run("read-platform-precedes-selector", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		previous := intentArchiveReadBoundarySupported
		intentArchiveReadBoundarySupported = func() bool { return false }
		t.Cleanup(func() { intentArchiveReadBoundarySupported = previous })
		for _, confirmed := range []bool{false, true} {
			args := []string{
				"--path", root, "feature", "intent-archive", "purge", slug,
				"--blob", malformed, "--json", "--quiet",
			}
			if confirmed {
				args = append(args, "--yes")
			}
			code, stdout, _, _ := runPrepare(t, args...)
			report := decodeIntentArchivePurgeReport(t, stdout)
			if code != 3 ||
				report.Refusal == nil ||
				report.Refusal.Code != "workspace-unsupported-platform" ||
				strings.Contains(stdout, malformed) {
				t.Fatalf("read-platform precedence = %d report=%#v\n%s", code, report, stdout)
			}
		}
	})

	t.Run("mutation-platform-precedes-selector", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		previous := intentArchiveMutationAuthoritySupported
		intentArchiveMutationAuthoritySupported = func() bool { return false }
		t.Cleanup(func() { intentArchiveMutationAuthoritySupported = previous })
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", malformed, "--yes", "--json", "--quiet",
		)
		report := decodeIntentArchivePurgeReport(t, stdout)
		if code != 3 ||
			report.Refusal == nil ||
			report.Refusal.Code != "prepare-unsupported-platform" ||
			strings.Contains(stdout, malformed) {
			t.Fatalf("mutation-platform precedence = %d report=%#v\n%s", code, report, stdout)
		}
	})

	if intentlock.AuthoritySupported {
		t.Run("filesystem-gate-precedes-selector", func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			acquires := 0
			previous := intentArchiveAcquireAuthority
			intentArchiveAcquireAuthority = func(string) (*intentlock.WorkspaceAuthority, error) {
				acquires++
				return nil, &intentlock.Error{
					Code:  intentlock.CodeLockFilesystemUnsupported,
					Class: "network-fs",
				}
			}
			t.Cleanup(func() { intentArchiveAcquireAuthority = previous })
			code, stdout, _, _ := runPrepare(
				t, "--path", root, "feature", "intent-archive", "purge", slug,
				"--blob", malformed, "--yes", "--json", "--quiet",
			)
			report := decodeIntentArchivePurgeReport(t, stdout)
			if code != 3 ||
				acquires != 1 ||
				report.Refusal == nil ||
				report.Refusal.Code != string(intentlock.CodeLockFilesystemUnsupported) ||
				strings.Contains(stdout, malformed) {
				t.Fatalf(
					"filesystem precedence = %d acquires=%d report=%#v\n%s",
					code, acquires, report, stdout,
				)
			}
		})

		t.Run("authority-contention-precedes-selector", func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			authority, err := intentlock.Acquire(root)
			if err != nil {
				t.Fatal(err)
			}
			defer authority.Release()
			code, stdout, _, _ := runPrepare(
				t, "--path", root, "feature", "intent-archive", "purge", slug,
				"--blob", malformed, "--yes", "--json", "--quiet",
			)
			report := decodeIntentArchivePurgeReport(t, stdout)
			if code != 3 ||
				report.Refusal == nil ||
				report.Refusal.Code != "transaction-in-progress" ||
				strings.Contains(stdout, malformed) {
				t.Fatalf("authority precedence = %d report=%#v\n%s", code, report, stdout)
			}
		})
	}

	t.Run("pending-journal-precedes-selector", func(t *testing.T) {
		for _, confirmed := range []bool{false, true} {
			if confirmed && !intentlock.AuthoritySupported {
				continue
			}
			name := "preview"
			if confirmed {
				name = "confirmed"
			}
			t.Run(name, func(t *testing.T) {
				root, slug := intentArchiveCLIWorkspace(t)
				journal := filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug)))
				if err := os.MkdirAll(filepath.Dir(journal), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(journal, []byte("{not-json"), 0o600); err != nil {
					t.Fatal(err)
				}
				args := []string{
					"--path", root, "feature", "intent-archive", "purge", slug,
					"--blob", malformed, "--json", "--quiet",
				}
				if confirmed {
					args = append(args, "--yes")
				}
				code, stdout, _, _ := runPrepare(t, args...)
				report := decodeIntentArchivePurgeReport(t, stdout)
				if code != 3 ||
					report.Refusal == nil ||
					report.Refusal.Code != "recovery-pending" ||
					strings.Contains(stdout, malformed) {
					t.Fatalf("journal precedence = %d report=%#v\n%s", code, report, stdout)
				}
			})
		}
	})
}

func TestFeatureIntentArchiveSelectorLexicalSafetyPrecedesPendingPurgeRecovery(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flag  string
		value func(string) string
	}{
		{name: "blob-absolute", flag: "--blob", value: func(root string) string { return filepath.Join(root, "secret") }},
		{name: "blob-control", flag: "--blob", value: func(string) string { return strings.Repeat("a", 63) + "\n" }},
		{name: "generation-absolute", flag: "--generation", value: func(root string) string { return filepath.Join(root, "generation") }},
		{name: "generation-control", flag: "--generation", value: func(string) string { return strings.Repeat("b", 63) + "\t" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, confirmed := range []bool{false, true} {
				if confirmed && !intentlock.AuthoritySupported {
					continue
				}
				name := "preview"
				if confirmed {
					name = "confirmed"
				}
				t.Run(name, func(t *testing.T) {
					root, slug := intentArchiveCLIWorkspace(t)
					data := []byte("selector safety pending\n")
					pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRemovalPending)
					writeIntentArchiveCLIFixture(t, root, slug,
						intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, pending)),
						map[string][]byte{pending.ContentSHA256: data},
					)
					before := readTree(t, filepath.Join(root, ".tpatch"))
					previews := 0
					recoveries := 0
					previousPreview := intentArchivePreviewPurge
					previousRecover := intentArchiveRecoverPurge
					intentArchivePreviewPurge = func(
						storage store.IntentArchiveStorage,
						slug string,
						selector store.IntentArchivePurgeSelector,
					) (store.IntentArchivePurgePlan, error) {
						previews++
						return previousPreview(storage, slug, selector)
					}
					intentArchiveRecoverPurge = func(
						storage store.IntentArchiveStorage,
						slug string,
					) (store.IntentArchivePurgeResult, error) {
						recoveries++
						return previousRecover(storage, slug)
					}
					t.Cleanup(func() {
						intentArchivePreviewPurge = previousPreview
						intentArchiveRecoverPurge = previousRecover
					})
					unsafe := tc.value(root)
					args := []string{
						"--path", root, "feature", "intent-archive", "purge", slug,
						tc.flag, unsafe, "--json", "--quiet",
					}
					if confirmed {
						args = append(args, "--yes")
					}
					code, stdout, stderr, _ := runPrepare(t, args...)
					if code != 1 ||
						stdout != "" ||
						previews != 0 ||
						recoveries != 0 ||
						strings.Contains(stdout+stderr, unsafe) ||
						strings.Contains(stdout+stderr, root) ||
						strings.Contains(stdout+stderr, string(store.IntentArchiveCodeSelectorInvalid)) {
						t.Fatalf(
							"unsafe selector code=%d previews=%d recoveries=%d stdout=%q stderr=%q",
							code, previews, recoveries, stdout, stderr,
						)
					}
					if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
						t.Fatal("unsafe selector reached pending-purge observation, recovery, or mutation")
					}
					humanArgs := []string{
						"--path", root, "feature", "intent-archive", "purge", slug,
						tc.flag, unsafe,
					}
					if confirmed {
						humanArgs = append(humanArgs, "--yes")
					}
					humanCode, human, humanErr, _ := runPrepare(t, humanArgs...)
					if humanCode != 1 ||
						human != "" ||
						strings.Contains(human+humanErr, unsafe) ||
						strings.Contains(human+humanErr, root) ||
						strings.Contains(human+humanErr, prepareRetryHeader) ||
						strings.Contains(human+humanErr, string(store.IntentArchiveCodeSelectorInvalid)) ||
						previews != 0 ||
						recoveries != 0 {
						t.Fatalf(
							"unsafe selector human leaked argv = %d previews=%d recoveries=%d stdout=%q stderr=%q",
							humanCode, previews, recoveries, human, humanErr,
						)
					}
					authority, err := intentlock.Acquire(root)
					if err != nil {
						t.Fatalf("unsafe selector retained authority: %v", err)
					}
					if err := authority.Release(); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

func TestFeatureIntentArchiveRecoveryRetryUsesNormalizedSelectors(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	for _, selector := range []string{"blob", "generation"} {
		t.Run(selector, func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			pendingBytes := []byte("normalized pending " + selector + "\n")
			retainedBytes := []byte("normalized retained " + selector + "\n")
			pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, pendingBytes, store.IntentArchiveWireRemovalPending)
			retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, retainedBytes, store.IntentArchiveWireRetained)
			pendingGeneration := intentArchiveCLIGeneration(t, slug, pending)
			retainedGeneration := intentArchiveCLIGeneration(t, slug, retained)
			writeIntentArchiveCLIFixture(t, root, slug,
				intentArchiveCLIIndex(t, slug, pendingGeneration, retainedGeneration),
				map[string][]byte{
					pending.ContentSHA256:  pendingBytes,
					retained.ContentSHA256: retainedBytes,
				},
			)
			first := pending.ContentSHA256
			second := retained.ContentSHA256
			flag := "--blob"
			if selector == "generation" {
				first = pendingGeneration.GenerationID
				second = retainedGeneration.GenerationID
				flag = "--generation"
			}
			values := []string{second, first, second}
			args := []string{"--path", root, "feature", "intent-archive", "purge", slug}
			for _, value := range values {
				args = append(args, flag, value)
			}
			args = append(args, "--yes", "--json", "--quiet")
			code, stdout, _, _ := runPrepare(t, args...)
			if code != 0 {
				t.Fatalf("normalized recovery retry = %d\n%s", code, stdout)
			}
			report := decodeIntentArchivePurgeReport(t, stdout)
			wantValues := sortedUniqueIntentArchiveStrings([]string{first, second})
			wantOptions := intentArchivePurgeOptions{yes: true, asJSON: true, quiet: true}
			if selector == "blob" {
				wantOptions.blobs = wantValues
			} else {
				wantOptions.generations = wantValues
			}
			wantRetry := intentArchivePurgeRetry(slug, wantOptions, true)
			if report.Outcome != string(store.IntentArchivePurgeRecovered) ||
				report.Recovery == nil ||
				report.Recovery.Retry != wantRetry ||
				strings.Count(report.Recovery.Retry, second) != 1 {
				t.Fatalf("normalized recovery retry = %#v want %q", report.Recovery, wantRetry)
			}
		})
	}
}

func TestFeatureIntentArchiveMalformedSelectorPrecedesPendingPurgeRecovery(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	root, slug := intentArchiveCLIWorkspace(t)
	recoveryCalls := 0
	previous := intentArchiveRecoverPurge
	intentArchiveRecoverPurge = func(store.IntentArchiveStorage, string) (store.IntentArchivePurgeResult, error) {
		recoveryCalls++
		return store.IntentArchivePurgeResult{}, errors.New("recovery must not run")
	}
	t.Cleanup(func() { intentArchiveRecoverPurge = previous })
	before := readTree(t, filepath.Join(root, ".tpatch"))
	const malformed = "/absolute/\nselector"
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--blob", malformed, "--yes", "--json", "--quiet",
	)
	if code != 1 || stdout != "" || recoveryCalls != 0 ||
		strings.Contains(stdout+stderr, malformed) ||
		strings.Contains(stdout+stderr, string(store.IntentArchiveCodeSelectorInvalid)) {
		t.Fatalf("selector/recovery precedence = code=%d calls=%d stdout=%q stderr=%q",
			code, recoveryCalls, stdout, stderr)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("malformed selector changed the workspace before recovery")
	}
	authority, err := intentlock.Acquire(root)
	if err != nil {
		t.Fatalf("malformed selector retained authority: %v", err)
	}
	if err := authority.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureIntentArchiveSelectorValidationAndSequentialRepair(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	t.Run("malformed-selector", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			flag      string
			value     string
			confirmed bool
		}{
			{name: "blob-malformed-preview", flag: "--blob", value: "not-a-hash"},
			{name: "blob-control-confirmed", flag: "--blob", value: "bad\nselector", confirmed: true},
			{name: "generation-absolute-preview", flag: "--generation", value: "/unsafe/selector"},
			{name: "generation-control-confirmed", flag: "--generation", value: "\x00unsafe", confirmed: true},
		} {
			t.Run(test.name, func(t *testing.T) {
				root, slug := intentArchiveCLIWorkspace(t)
				before := readTree(t, filepath.Join(root, ".tpatch"))
				args := []string{
					"--path", root, "feature", "intent-archive", "purge", slug,
					test.flag, test.value, "--json", "--quiet",
				}
				if test.confirmed {
					args = append(args, "--yes")
				}
				code, stdout, stderr, _ := runPrepare(t, args...)
				if code != 1 || stdout != "" ||
					strings.Contains(stdout+stderr, test.value) ||
					strings.Contains(stdout+stderr, string(store.IntentArchiveCodeSelectorInvalid)) {
					t.Fatalf("malformed selector = code=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
					t.Fatal("malformed selector wrote to the workspace")
				}
				authority, err := intentlock.Acquire(root)
				if err != nil {
					t.Fatalf("malformed selector retained authority: %v", err)
				}
				if err := authority.Release(); err != nil {
					t.Fatal(err)
				}
			})
		}
	})

	t.Run("absent-archive-is-no-op", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		before := readTree(t, filepath.Join(root, ".tpatch"))
		recoveries := 0
		previousRecover := intentArchiveRecoverPurge
		intentArchiveRecoverPurge = func(
			storage store.IntentArchiveStorage,
			slug string,
		) (store.IntentArchivePurgeResult, error) {
			recoveries++
			return previousRecover(storage, slug)
		}
		t.Cleanup(func() { intentArchiveRecoverPurge = previousRecover })
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--all", "--yes", "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("absent archive = %d\n%s", code, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgeNoOp) ||
			report.Action != "none" ||
			recoveries != 0 {
			t.Fatalf("absent archive report=%#v recoveries=%d", report, recoveries)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("absent archive no-op created state")
		}
	})

	t.Run("partial-dangling-selection-uses-only-total-class-retry", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		firstBytes := []byte("dangling class first\n")
		secondBytes := []byte("dangling class second\n")
		first := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, firstBytes, store.IntentArchiveWireRetained)
		second := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, secondBytes, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug,
				intentArchiveCLIGeneration(t, slug, first),
				intentArchiveCLIGeneration(t, slug, second),
			),
			map[string][]byte{},
		)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", first.ContentSHA256, "--yes", "--json", "--quiet",
		)
		if code != 3 {
			t.Fatalf("partial dangling selection = %d\n%s", code, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Refusal == nil ||
			report.Refusal.Code != string(store.IntentArchiveCodeBlobDangling) ||
			report.Refusal.Retry != "" ||
			report.RemainingRepairs == nil ||
			len(report.RemainingRepairs.Stages) != 1 {
			t.Fatalf("partial dangling report = %#v", report)
		}
		stage := report.RemainingRepairs.Stages[0]
		wantRetry := intentArchiveBlobRetry(slug, []string{first.ContentSHA256, second.ContentSHA256})
		if stage.Class != string(store.IntentArchiveRepairDanglingReference) ||
			stage.Kind != string(store.IntentArchiveRepairStagePurge) ||
			stage.Repair != wantRetry {
			t.Fatalf("dangling stage = %#v want %q", stage, wantRetry)
		}
		fields := strings.Fields(stage.Repair)
		if len(fields) < 2 || fields[0] != "tpatch" {
			t.Fatalf("emitted dangling command is not executable: %q", stage.Repair)
		}
		executeArgs := append([]string{"--path", root}, fields[1:]...)
		executeArgs = append(executeArgs, "--json", "--quiet")
		code, stdout, _, _ = runPrepare(t, executeArgs...)
		if code != 0 {
			t.Fatalf("emitted total dangling retry self-refused = %d\n%s", code, stdout)
		}
		_, index := readIntentArchiveCLIIndex(t, root, slug)
		for _, generation := range index.Generations {
			if generation.Replaced[0].WireState() != store.IntentArchiveWireTombstoned {
				t.Fatalf("total dangling retry did not tombstone %#v", generation)
			}
		}
	})

	t.Run("repair-one-class-and-report-the-other", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		mixedBytes := []byte("mixed shared bytes\n")
		orphanBytes := []byte("repairable orphan\n")
		retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, mixedBytes, store.IntentArchiveWireRetained)
		tombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, mixedBytes, store.IntentArchiveWireTombstoned)
		orphanSum := sha256.Sum256(orphanBytes)
		orphanHash := hex.EncodeToString(orphanSum[:])
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug,
				intentArchiveCLIGeneration(t, slug, retained),
				intentArchiveCLIGeneration(t, slug, tombstoned),
			),
			map[string][]byte{
				retained.ContentSHA256: mixedBytes,
				orphanHash:             orphanBytes,
			},
		)
		indexBefore, _ := readIntentArchiveCLIIndex(t, root, slug)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--orphans", "--yes", "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("sequential orphan repair = %d\n%s", code, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgePurged) ||
			report.RemainingRepairs == nil ||
			report.RemainingRepairs.RepairedClass != string(store.IntentArchiveRepairUnreferencedResidue) ||
			report.RemainingRepairs.StagesRemaining != 1 ||
			report.RemainingRepairs.Stages[0].Class != string(store.IntentArchiveRepairMixedReference) {
			t.Fatalf("sequential repair report = %#v", report)
		}
		orphanRel, _ := store.IntentArchiveBlobRel(slug, orphanHash)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(orphanRel))); !os.IsNotExist(err) {
			t.Fatalf("orphan remains: %v", err)
		}
		mixedRel, _ := store.IntentArchiveBlobRel(slug, retained.ContentSHA256)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(mixedRel))); err != nil {
			t.Fatalf("mixed live blob was removed: %v", err)
		}
		indexAfter, _ := readIntentArchiveCLIIndex(t, root, slug)
		if !bytes.Equal(indexBefore, indexAfter) {
			t.Fatal("--orphans rewrote index.json")
		}
	})

	t.Run("corrupt-object-blocks-every-selector", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		expected := []byte("corrupt expected\n")
		orphanBytes := []byte("blocked orphan\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, expected, store.IntentArchiveWireRetained)
		orphanSum := sha256.Sum256(orphanBytes)
		orphanHash := hex.EncodeToString(orphanSum[:])
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{
				replacement.ContentSHA256: []byte("corrupt wrong bytes\n"),
				orphanHash:                orphanBytes,
			},
		)
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--orphans", "--yes", "--json", "--quiet",
		)
		if code != 3 {
			t.Fatalf("corrupt blocking class = %d\n%s", code, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Refusal == nil ||
			report.Refusal.Code != string(store.IntentArchiveCodeBlobCorrupt) ||
			report.RemainingRepairs == nil ||
			report.RemainingRepairs.StagesRemaining != 3 ||
			report.RemainingRepairs.Stages[0].Class != string(store.IntentArchiveRepairCorruptObject) ||
			report.RemainingRepairs.Stages[0].Kind != string(store.IntentArchiveRepairStageManual) ||
			!strings.Contains(report.RemainingRepairs.Stages[0].Repair, "No tpatch repair selector runs anywhere in this archive until that removal has happened.") {
			t.Fatalf("corrupt blocking report = %#v", report)
		}
		humanCode, human, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--orphans", "--yes",
		)
		if humanCode != 3 ||
			strings.Contains(human, "repair: tpatch ") ||
			strings.Contains(human, "Then run tpatch ") {
			t.Fatalf("corrupt blocking human = %d\n%s", humanCode, human)
		}
		assertIntentArchiveTpatchRetriesHeaded(t, human)
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("corrupt blocking-class refusal changed the archive")
		}
	})
}

func TestFeatureIntentArchiveRemainingRepairStageExactJSONSchema(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	root, slug := intentArchiveCLIWorkspace(t)
	expected := []byte("stage schema corrupt\n")
	replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, expected, store.IntentArchiveWireRetained)
	writeIntentArchiveCLIFixture(t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
		map[string][]byte{replacement.ContentSHA256: []byte("stage schema wrong\n")},
	)
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--all", "--yes", "--json", "--quiet",
	)
	if code != 3 {
		t.Fatalf("stage schema fixture = %d\n%s", code, stdout)
	}
	var raw struct {
		RemainingRepairs struct {
			Stages []map[string]json.RawMessage `json:"stages"`
		} `json:"remaining_repairs"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.RemainingRepairs.Stages) == 0 {
		t.Fatal("stage schema fixture emitted no stages")
	}
	wantKeys := []string{
		"after_prerequisite",
		"class",
		"hashes",
		"kind",
		"ordinal",
		"paths",
		"repair",
		"repair_cwd",
		"resulting_classes",
	}
	for index, stage := range raw.RemainingRepairs.Stages {
		gotKeys := make([]string, 0, len(stage))
		for key := range stage {
			gotKeys = append(gotKeys, key)
		}
		sort.Strings(gotKeys)
		if !reflect.DeepEqual(gotKeys, wantKeys) {
			t.Fatalf("stage %d keys = %v, want %v", index, gotKeys, wantKeys)
		}
		if _, exists := stage["retry"]; exists {
			t.Fatalf("stage %d leaked human-only retry", index)
		}
		if _, exists := stage["retry_cwd"]; exists {
			t.Fatalf("stage %d leaked human-only retry_cwd", index)
		}
	}
}

type intentArchiveFaultStorage struct {
	store.IntentArchiveStorage
	failBeforeCAS   bool
	failRemove      bool
	failCASAt       int
	casCalls        int
	failRemoveAt    int
	removeCalls     int
	probeCalls      int
	changeProbeAt   int
	captureCalls    int
	changeCaptureAt int
}

func (storage *intentArchiveFaultStorage) CASIndex(
	indexRel string,
	expected store.IntentArchiveIdentityToken,
	canonical []byte,
) (store.IntentArchiveMutationResult, error) {
	storage.casCalls++
	if storage.failBeforeCAS || storage.failCASAt != 0 && storage.casCalls == storage.failCASAt {
		return store.IntentArchiveMutationResult{}, errors.New("injected pre-index failure")
	}
	return storage.IntentArchiveStorage.CASIndex(indexRel, expected, canonical)
}

func (storage *intentArchiveFaultStorage) RemoveBlob(
	blobRel string,
	expected store.IntentArchiveIdentityToken,
) (store.IntentArchiveMutationResult, error) {
	storage.removeCalls++
	if storage.failRemove || storage.failRemoveAt != 0 && storage.removeCalls == storage.failRemoveAt {
		return store.IntentArchiveMutationResult{}, errors.New("injected pre-remove failure")
	}
	return storage.IntentArchiveStorage.RemoveBlob(blobRel, expected)
}

func (storage *intentArchiveFaultStorage) ProbeBlob(blobRel string) (store.IntentArchiveBlobProbe, error) {
	storage.probeCalls++
	probe, err := storage.IntentArchiveStorage.ProbeBlob(blobRel)
	if err == nil && storage.changeProbeAt != 0 && storage.probeCalls == storage.changeProbeAt {
		probe.Identity += ":externally-replaced"
	}
	return probe, err
}

func (storage *intentArchiveFaultStorage) CaptureIndex(indexRel string) (store.IntentArchiveIndexCapture, error) {
	storage.captureCalls++
	capture, err := storage.IntentArchiveStorage.CaptureIndex(indexRel)
	if err == nil && storage.changeCaptureAt != 0 && storage.captureCalls == storage.changeCaptureAt {
		capture.Identity += ":externally-changed"
	}
	return capture, err
}

// PIB-352 and PIB-353: the two sides of the first pending-index rename.
func TestFeatureIntentArchiveCrashBoundariesAndRecovery(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	t.Run("before-index-rewrite-is-zero-write", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("before rewrite\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: data},
		)
		before := readTree(t, filepath.Join(root, ".tpatch"))
		previousFactory := intentArchiveNewStorage
		intentArchiveNewStorage = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
			return &intentArchiveFaultStorage{
				IntentArchiveStorage: previousFactory(authority, root),
				failBeforeCAS:        authority != nil,
			}
		}
		t.Cleanup(func() { intentArchiveNewStorage = previousFactory })
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", replacement.ContentSHA256, "--yes", "--json", "--quiet",
		)
		if code != 3 {
			t.Fatalf("pre-index failure = %d\n%s", code, stdout)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("failure before the index rewrite changed the workspace")
		}
	})

	t.Run("pending-index-before-remove-recovers", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("pending before remove\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: data},
		)
		previousFactory := intentArchiveNewStorage
		intentArchiveNewStorage = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
			return &intentArchiveFaultStorage{
				IntentArchiveStorage: previousFactory(authority, root),
				failRemove:           authority != nil,
			}
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", replacement.ContentSHA256, "--yes", "--json", "--quiet",
		)
		intentArchiveNewStorage = previousFactory
		if code != 5 {
			t.Fatalf("pending crash = %d\n%s", code, stdout)
		}
		partial := decodeIntentArchivePurgeReport(t, stdout)
		if partial.Outcome != string(store.IntentArchivePurgePartial) ||
			partial.PurgeProgress == nil ||
			partial.PurgeProgress.Resume != string(store.IntentArchiveResumePendingRecoveryThenCompletion) ||
			partial.PurgeProgress.PendingHash != replacement.ContentSHA256 {
			t.Fatalf("pending crash report = %#v", partial)
		}
		_, pendingIndex := readIntentArchiveCLIIndex(t, root, slug)
		if pendingIndex.Generations[0].Replaced[0].WireState() != store.IntentArchiveWireRemovalPending {
			t.Fatalf("pending crash did not persist pending truth: %#v", pendingIndex)
		}
		blobRel, _ := store.IntentArchiveBlobRel(slug, replacement.ContentSHA256)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel))); err != nil {
			t.Fatalf("pending crash removed blob: %v", err)
		}
		code, stdout, _, _ = runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", replacement.ContentSHA256, "--yes", "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("pending recovery = %d\n%s", code, stdout)
		}
		recovered := decodeIntentArchivePurgeReport(t, stdout)
		if recovered.Outcome != string(store.IntentArchivePurgeRecovered) {
			t.Fatalf("pending recovery report = %#v", recovered)
		}
		_, finalIndex := readIntentArchiveCLIIndex(t, root, slug)
		if finalIndex.Generations[0].Replaced[0].WireState() != store.IntentArchiveWireTombstoned {
			t.Fatalf("pending recovery did not tombstone: %#v", finalIndex)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel))); !os.IsNotExist(err) {
			t.Fatalf("pending recovery left blob: %v", err)
		}
	})
}

// PIB-359: orphan removal is bound to both the index preimage and the
// revalidated regular-file identity, and never rewrites index.json.
func TestFeatureIntentArchiveOrphanCASAndRevalidation(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	for _, tc := range []struct {
		name            string
		changeProbeAt   int
		changeCaptureAt int
	}{
		{name: "blob-identity", changeProbeAt: 4},
		{name: "index-preimage", changeCaptureAt: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			orphanBytes := []byte("orphan cas " + tc.name + "\n")
			sum := sha256.Sum256(orphanBytes)
			hash := hex.EncodeToString(sum[:])
			writeIntentArchiveCLIFixture(t, root, slug,
				intentArchiveCLIIndex(t, slug),
				map[string][]byte{hash: orphanBytes},
			)
			indexBefore, _ := readIntentArchiveCLIIndex(t, root, slug)
			previousFactory := intentArchiveNewStorage
			var injected *intentArchiveFaultStorage
			intentArchiveNewStorage = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
				injected = &intentArchiveFaultStorage{
					IntentArchiveStorage: previousFactory(authority, root),
					changeProbeAt:        tc.changeProbeAt,
					changeCaptureAt:      tc.changeCaptureAt,
				}
				return injected
			}
			code, stdout, _, _ := runPrepare(
				t, "--path", root, "feature", "intent-archive", "purge", slug,
				"--orphans", "--yes", "--json", "--quiet",
			)
			intentArchiveNewStorage = previousFactory
			if code != 3 {
				t.Fatalf("orphan revalidation = %d probes=%d captures=%d\n%s",
					code, injected.probeCalls, injected.captureCalls, stdout)
			}
			blobRel, _ := store.IntentArchiveBlobRel(slug, hash)
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel))); err != nil {
				t.Fatalf("orphan was removed after revalidation failure: %v", err)
			}
			indexAfter, _ := readIntentArchiveCLIIndex(t, root, slug)
			if !bytes.Equal(indexBefore, indexAfter) {
				t.Fatal("orphan purge rewrote index.json")
			}
		})
	}
}

func TestFeatureIntentArchiveRealPartialBranches(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	t.Run("completion-only-between-hashes", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		firstBytes := []byte("completion first\n")
		secondBytes := []byte("completion second\n")
		first := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, firstBytes, store.IntentArchiveWireRetained)
		second := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, secondBytes, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, first, second)),
			map[string][]byte{first.ContentSHA256: firstBytes, second.ContentSHA256: secondBytes},
		)
		previousFactory := intentArchiveNewStorage
		intentArchiveNewStorage = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
			return &intentArchiveFaultStorage{
				IntentArchiveStorage: previousFactory(authority, root),
				failCASAt:            3,
			}
		}
		t.Cleanup(func() { intentArchiveNewStorage = previousFactory })
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--all", "--yes", "--json", "--quiet",
		)
		if code != 5 {
			t.Fatalf("completion-only = %d\n%s", code, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.PurgeProgress == nil ||
			report.PurgeProgress.Resume != string(store.IntentArchiveResumeCompletionOnly) ||
			report.PurgeProgress.PendingHash != "" ||
			len(report.PurgeProgress.CompletedHashes) != 1 ||
			len(report.PurgeProgress.RemainingHashes) != 1 {
			t.Fatalf("completion-only report = %#v", report)
		}
	})

	t.Run("orphan-scan-after-first-remove", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		firstBytes := []byte("orphan first\n")
		secondBytes := []byte("orphan second\n")
		firstSum := sha256.Sum256(firstBytes)
		secondSum := sha256.Sum256(secondBytes)
		firstHash := hex.EncodeToString(firstSum[:])
		secondHash := hex.EncodeToString(secondSum[:])
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug),
			map[string][]byte{firstHash: firstBytes, secondHash: secondBytes},
		)
		previousFactory := intentArchiveNewStorage
		intentArchiveNewStorage = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
			return &intentArchiveFaultStorage{
				IntentArchiveStorage: previousFactory(authority, root),
				failRemoveAt:         2,
			}
		}
		t.Cleanup(func() { intentArchiveNewStorage = previousFactory })
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--orphans", "--yes", "--json", "--quiet",
		)
		if code != 5 {
			t.Fatalf("orphan partial = %d\n%s", code, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.PurgeProgress == nil ||
			report.PurgeProgress.Resume != string(store.IntentArchiveResumeOrphanScan) ||
			report.PurgeProgress.PendingHash != "" ||
			len(report.PurgeProgress.CompletedHashes) != 1 ||
			len(report.PurgeProgress.RemainingHashes) != 1 ||
			len(report.OrphanBlobs) != 1 {
			t.Fatalf("orphan partial report = %#v", report)
		}
	})
}

func TestFeatureIntentArchivePartialDoesNotClaimRepairClassCompletion(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	root, slug := intentArchiveCLIWorkspace(t)
	mixedBytes := []byte("partial mixed class\n")
	firstOrphan := []byte("partial orphan first\n")
	secondOrphan := []byte("partial orphan second\n")
	retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, mixedBytes, store.IntentArchiveWireRetained)
	tombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, mixedBytes, store.IntentArchiveWireTombstoned)
	firstSum := sha256.Sum256(firstOrphan)
	secondSum := sha256.Sum256(secondOrphan)
	firstHash := hex.EncodeToString(firstSum[:])
	secondHash := hex.EncodeToString(secondSum[:])
	writeIntentArchiveCLIFixture(t, root, slug,
		intentArchiveCLIIndex(t, slug,
			intentArchiveCLIGeneration(t, slug, retained),
			intentArchiveCLIGeneration(t, slug, tombstoned),
		),
		map[string][]byte{
			retained.ContentSHA256: mixedBytes,
			firstHash:              firstOrphan,
			secondHash:             secondOrphan,
		},
	)
	previousFactory := intentArchiveNewStorage
	intentArchiveNewStorage = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
		return &intentArchiveFaultStorage{
			IntentArchiveStorage: previousFactory(authority, root),
			failRemoveAt:         2,
		}
	}
	t.Cleanup(func() { intentArchiveNewStorage = previousFactory })
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--yes", "--json", "--quiet",
	)
	if code != 5 {
		t.Fatalf("partial class repair = %d\n%s", code, stdout)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	if report.Outcome != string(store.IntentArchivePurgePartial) ||
		report.RemainingRepairs != nil ||
		len(report.Advisories) != 0 ||
		strings.Contains(stdout, `"remaining_repairs"`) ||
		strings.Contains(stdout, "archive-repairs-remaining") ||
		strings.Contains(stdout, "Repaired unreferenced-residue") {
		t.Fatalf("partial falsely completed repair class: %#v\n%s", report, stdout)
	}
	var human bytes.Buffer
	writeIntentArchivePurgeHuman(&human, report)
	if strings.Contains(human.String(), "Repaired class:") ||
		strings.Contains(human.String(), "Remaining repair stages:") ||
		strings.Contains(human.String(), "Repaired unreferenced-residue") {
		t.Fatalf("partial human falsely completed repair class:\n%s", human.String())
	}
}

func TestFeatureIntentArchivePartialBranchesAndDivergenceReports(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	for _, tc := range []struct {
		name        string
		selector    []string
		result      store.IntentArchivePurgeResult
		wantResume  store.IntentArchivePurgeResume
		wantPending bool
	}{
		{
			name:     "pending-recovery-then-completion",
			selector: []string{"blob"},
			result: store.IntentArchivePurgeResult{
				Outcome:         store.IntentArchivePurgePartial,
				CompletedHashes: []string{},
				PendingHash:     "set-by-fixture",
				RemainingHashes: []string{},
				Resume:          store.IntentArchiveResumePendingRecoveryThenCompletion,
				State:           store.IntentArchivePurgeStateConsistent,
				Committed:       true,
			},
			wantResume:  store.IntentArchiveResumePendingRecoveryThenCompletion,
			wantPending: true,
		},
		{
			name:     "completion-only",
			selector: []string{"blob"},
			result: store.IntentArchivePurgeResult{
				Outcome:         store.IntentArchivePurgePartial,
				CompletedHashes: []string{"set-by-fixture"},
				RemainingHashes: []string{},
				Resume:          store.IntentArchiveResumeCompletionOnly,
				State:           store.IntentArchivePurgeStateConsistent,
				Committed:       true,
			},
			wantResume: store.IntentArchiveResumeCompletionOnly,
		},
		{
			name:     "orphan-scan",
			selector: []string{"orphans"},
			result: store.IntentArchivePurgeResult{
				Outcome:         store.IntentArchivePurgePartial,
				CompletedHashes: []string{},
				RemainingHashes: []string{"set-by-fixture"},
				Resume:          store.IntentArchiveResumeOrphanScan,
				State:           store.IntentArchivePurgeStateConsistent,
				Committed:       true,
			},
			wantResume: store.IntentArchiveResumeOrphanScan,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			data := []byte("partial " + tc.name + "\n")
			sum := sha256.Sum256(data)
			hash := hex.EncodeToString(sum[:])
			var args []string
			if tc.selector[0] == "blob" {
				replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
				hash = replacement.ContentSHA256
				writeIntentArchiveCLIFixture(t, root, slug,
					intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
					map[string][]byte{hash: data},
				)
				args = []string{"--blob", hash}
			} else {
				writeIntentArchiveCLIFixture(t, root, slug,
					intentArchiveCLIIndex(t, slug),
					map[string][]byte{hash: data},
				)
				args = []string{"--orphans"}
			}
			result := tc.result
			if result.PendingHash == "set-by-fixture" {
				result.PendingHash = hash
			}
			for index := range result.CompletedHashes {
				if result.CompletedHashes[index] == "set-by-fixture" {
					result.CompletedHashes[index] = hash
				}
			}
			for index := range result.RemainingHashes {
				if result.RemainingHashes[index] == "set-by-fixture" {
					result.RemainingHashes[index] = hash
				}
			}
			previousExecute := intentArchiveExecutePurge
			intentArchiveExecutePurge = func(
				store.IntentArchiveStorage,
				store.IntentArchivePurgePlan,
			) (store.IntentArchivePurgeResult, error) {
				return result, &store.IntentArchiveError{
					Code:      store.IntentArchiveCodePurgePartial,
					Hash:      hash,
					ExitClass: 5,
					Committed: true,
				}
			}
			t.Cleanup(func() { intentArchiveExecutePurge = previousExecute })
			command := []string{
				"--path", root, "feature", "intent-archive", "purge", slug,
			}
			command = append(command, args...)
			command = append(command, "--yes", "--json", "--quiet")
			code, stdout, _, _ := runPrepare(t, command...)
			if code != 5 {
				t.Fatalf("partial branch = %d\n%s", code, stdout)
			}
			report := decodeIntentArchivePurgeReport(t, stdout)
			if report.Outcome != string(store.IntentArchivePurgePartial) ||
				report.PurgeProgress == nil ||
				report.PurgeProgress.Resume != string(tc.wantResume) ||
				(report.PurgeProgress.PendingHash != "") != tc.wantPending ||
				report.PurgeProgress.RetryCWD != store.IntentArchiveRepairCWD ||
				strings.Contains(report.PurgeProgress.Retry, root) {
				t.Fatalf("partial report = %#v", report)
			}
			if tc.wantResume != store.IntentArchiveResumePendingRecoveryThenCompletion &&
				strings.Contains(strings.ToLower(report.PurgeProgress.Retry), "recovered") {
				t.Fatalf("non-pending branch promised recovery: %#v", report.PurgeProgress)
			}
		})
	}

	t.Run("owned-divergent-blob-is-exit-six", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("divergent owned bytes\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: data},
		)
		previousExecute := intentArchiveExecutePurge
		intentArchiveExecutePurge = func(
			store.IntentArchiveStorage,
			store.IntentArchivePurgePlan,
		) (store.IntentArchivePurgeResult, error) {
			return store.IntentArchivePurgeResult{
					Outcome:         store.IntentArchivePurgePartial,
					PendingHash:     replacement.ContentSHA256,
					CompletedHashes: []string{},
					RemainingHashes: []string{},
					Resume:          store.IntentArchiveResumePendingRecoveryThenCompletion,
					Committed:       true,
				}, &store.IntentArchiveError{
					Code:      store.IntentArchiveCodePurgeEvidenceDivergent,
					Hash:      replacement.ContentSHA256,
					Detail:    "an owned blob is present but unidentifiable",
					ExitClass: 6,
					Committed: true,
				}
		}
		t.Cleanup(func() { intentArchiveExecutePurge = previousExecute })
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", replacement.ContentSHA256, "--yes", "--json", "--quiet",
		)
		if code != 6 {
			t.Fatalf("divergence = %d stderr=%q\n%s", code, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != "refused" ||
			report.Refusal == nil ||
			report.Refusal.Code != string(store.IntentArchiveCodePurgeEvidenceDivergent) ||
			report.Divergence == nil ||
			report.Divergence.Kind != "blob" ||
			!strings.HasPrefix(report.Divergence.RemoveCommand, "rm -rf -- .tpatch/features/") ||
			report.Divergence.RetryCWD != store.IntentArchiveRepairCWD ||
			strings.Contains(stdout, "--abandon-transaction") ||
			strings.Contains(stdout, root) ||
			!strings.Contains(stdout, intentArchiveHistoryDisclosure) {
			t.Fatalf("divergence report = %#v\n%s", report, stdout)
		}
	})

	t.Run("index-divergence-clears-blob-removal-shape", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("index divergence bytes\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: data},
		)
		previousExecute := intentArchiveExecutePurge
		intentArchiveExecutePurge = func(
			store.IntentArchiveStorage,
			store.IntentArchivePurgePlan,
		) (store.IntentArchivePurgeResult, error) {
			return store.IntentArchivePurgeResult{
					Outcome:         store.IntentArchivePurgePartial,
					PendingHash:     replacement.ContentSHA256,
					CompletedHashes: []string{},
					RemainingHashes: []string{},
					Committed:       true,
				}, &store.IntentArchiveError{
					Code:      store.IntentArchiveCodePurgeEvidenceDivergent,
					Hash:      replacement.ContentSHA256,
					Detail:    "index.json stopped strict-decoding after the purge transaction began",
					ExitClass: 6,
					Committed: true,
				}
		}
		t.Cleanup(func() { intentArchiveExecutePurge = previousExecute })
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", replacement.ContentSHA256, "--yes", "--json", "--quiet",
		)
		if code != 6 {
			t.Fatalf("index divergence = %d\n%s", code, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Divergence == nil ||
			report.Divergence.Kind != "index" ||
			report.Divergence.Blob != "" ||
			report.Divergence.RemoveCommand != "" ||
			len(report.Blobs) != 0 ||
			len(report.OrphanBlobs) != 0 ||
			report.RemainingRepairs != nil ||
			strings.Contains(stdout, ".blob") ||
			strings.Contains(stdout, "rm -rf") {
			t.Fatalf("index divergence leaked blob shape: %#v\n%s", report, stdout)
		}
		var human bytes.Buffer
		writeIntentArchivePurgeHuman(&human, report)
		if strings.Contains(human.String(), "\nblobs:\n") ||
			strings.Contains(human.String(), ".blob") ||
			strings.Contains(human.String(), "rm -rf") {
			t.Fatalf("index divergence human leaked blob shape:\n%s", human.String())
		}
		assertIntentArchiveTpatchRetriesHeaded(t, human.String())
	})
}

func TestFeatureIntentArchiveRealOwnedDivergence(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unsupported on this target")
	}
	root, slug := intentArchiveCLIWorkspace(t)
	expected := []byte("owned expected\n")
	replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, expected, store.IntentArchiveWireRemovalPending)
	writeIntentArchiveCLIFixture(t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
		map[string][]byte{replacement.ContentSHA256: []byte("owned wrong bytes\n")},
	)
	before := readTree(t, filepath.Join(root, ".tpatch"))
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--blob", replacement.ContentSHA256, "--yes", "--json", "--quiet",
	)
	if code != 6 {
		t.Fatalf("real owned divergence = %d\n%s", code, stdout)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	wantRetry := "tpatch feature intent-archive purge " + slug + " --blob " + replacement.ContentSHA256 + " --yes --json --quiet"
	if report.Refusal == nil ||
		report.Refusal.Code != string(store.IntentArchiveCodePurgeEvidenceDivergent) ||
		report.Refusal.Retry != "" ||
		report.Divergence == nil ||
		report.Divergence.PendingHash != replacement.ContentSHA256 ||
		report.Divergence.Retry != wantRetry ||
		report.Divergence.RemoveCommand == "" ||
		strings.Count(stdout, `"retry":`) != 1 ||
		strings.Contains(stdout, "--abandon-transaction") {
		t.Fatalf("real divergence report = %#v", report)
	}
	humanCode, human, humanErr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--blob", replacement.ContentSHA256, "--yes",
	)
	warningAt := strings.Index(human, "WARNING:")
	removeAt := strings.Index(human, "rm -rf -- ")
	humanRetry := "tpatch feature intent-archive purge " + slug + " --blob " + replacement.ContentSHA256 + " --yes"
	if humanCode != 6 || humanErr == "" ||
		!strings.HasPrefix(human, intentArchiveCommandPurge+" "+slug+": refused archive-purge-evidence-divergent\n") ||
		warningAt < 0 || removeAt <= warningAt ||
		!strings.Contains(human, "pending hash: "+replacement.ContentSHA256) ||
		strings.Count(human, prepareRetryHeader) != 1 ||
		strings.Count(human, humanRetry) != 1 ||
		!strings.Contains(human, intentArchiveHistoryDisclosure) ||
		strings.Contains(human, "--abandon-transaction") {
		t.Fatalf("real divergence human = %d stderr=%q\n%s", humanCode, humanErr, human)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("divergent recovery rewrote evidence")
	}
}

// PIB-356: availability is global by hash and every safe observation survives
// coexistence with a worse one.
func TestFeatureIntentArchiveListGlobalAvailabilityAndCompleteObservations(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	mixedBytes := []byte("mixed availability\n")
	orphanBytes := []byte("coexisting orphan\n")
	purgedBytes := []byte("purged and absent\n")
	retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, mixedBytes, store.IntentArchiveWireRetained)
	tombstonedMixed := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, mixedBytes, store.IntentArchiveWireTombstoned)
	tombstonedPurged := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, purgedBytes, store.IntentArchiveWireTombstoned)
	orphanSum := sha256.Sum256(orphanBytes)
	orphanHash := hex.EncodeToString(orphanSum[:])
	writeIntentArchiveCLIFixture(t, root, slug,
		intentArchiveCLIIndex(t, slug,
			intentArchiveCLIGeneration(t, slug, retained),
			intentArchiveCLIGeneration(t, slug, tombstonedMixed),
			intentArchiveCLIGeneration(t, slug, tombstonedPurged),
		),
		map[string][]byte{
			retained.ContentSHA256: mixedBytes,
			orphanHash:             orphanBytes,
		},
	)
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
	)
	if code != 3 {
		t.Fatalf("mixed list = %d\n%s", code, stdout)
	}
	report := decodeIntentArchiveListReport(t, stdout)
	if len(report.Generations) != 3 || len(report.Orphans) != 1 ||
		report.Refusal == nil ||
		report.Refusal.Code != string(store.IntentArchiveCodeIndexStorageInconsistent) {
		t.Fatalf("mixed list report = %#v", report)
	}
	var mixedEntry, purgedEntry *intentArchiveListEntryReport
	for generationIndex := range report.Generations {
		for entryIndex := range report.Generations[generationIndex].Entries {
			entry := &report.Generations[generationIndex].Entries[entryIndex]
			if entry.ContentSHA256 == retained.ContentSHA256 && entry.Storage == "mixed-reference" {
				mixedEntry = entry
			}
			if entry.ContentSHA256 == tombstonedPurged.ContentSHA256 {
				purgedEntry = entry
			}
		}
	}
	if mixedEntry == nil ||
		mixedEntry.Repair != "tpatch feature intent-archive purge "+slug+" --blob "+retained.ContentSHA256+" --yes" ||
		strings.Contains(mixedEntry.Repair, "--orphans") ||
		strings.Contains(mixedEntry.Availability, "not recoverable") {
		t.Fatalf("mixed entry = %#v", mixedEntry)
	}
	if purgedEntry == nil ||
		purgedEntry.Availability != "not recoverable until identical content is archived again" {
		t.Fatalf("purged entry = %#v", purgedEntry)
	}
	if report.Orphans[0].Hash != orphanHash {
		t.Fatalf("coexisting orphan suppressed: %#v", report.Orphans)
	}
	humanCode, human, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug,
	)
	if humanCode != 3 || strings.Contains(human, "repair: tpatch ") {
		t.Fatalf("mixed human = %d\n%s", humanCode, human)
	}
	assertIntentArchiveTpatchRetriesHeaded(t, human)
}

func TestFeatureIntentArchiveListIncludesUnindexedCorruptObjects(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	wrongNameSum := sha256.Sum256([]byte("expected unindexed name\n"))
	directorySum := sha256.Sum256([]byte("unindexed directory name\n"))
	wrongHash := hex.EncodeToString(wrongNameSum[:])
	directoryHash := hex.EncodeToString(directorySum[:])
	secretContent := []byte("sensitive wrong object bytes must not leak\n")
	writeIntentArchiveCLIFixture(t, root, slug,
		intentArchiveCLIIndex(t, slug),
		map[string][]byte{wrongHash: secretContent},
	)
	directoryRel, _ := store.IntentArchiveBlobRel(slug, directoryHash)
	if err := os.Mkdir(filepath.Join(root, filepath.FromSlash(directoryRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug,
		"--json", "--quiet",
	)
	if code != 3 {
		t.Fatalf("unindexed corrupt list = %d\n%s", code, stdout)
	}
	report := decodeIntentArchiveListReport(t, stdout)
	if len(report.Generations) != 0 ||
		len(report.Orphans) != 0 ||
		len(report.CorruptObjects) != 2 {
		t.Fatalf("unindexed corrupt report = %#v", report)
	}
	byHash := map[string]intentArchiveListCorruptObjectReport{}
	for _, object := range report.CorruptObjects {
		byHash[object.Hash] = object
		if object.Repair == "" || !strings.Contains(object.Repair, "rm -rf -- ") {
			t.Fatalf("unindexed corrupt object omitted prerequisite: %#v", object)
		}
	}
	if byHash[wrongHash].Kind != "regular-hash-wrong" ||
		byHash[wrongHash].Path == "" ||
		byHash[directoryHash].Kind != string(store.IntentArchiveBlobKindDirectory) ||
		byHash[directoryHash].Path != directoryRel ||
		strings.Contains(stdout, string(secretContent)) ||
		strings.Contains(stdout, root) {
		t.Fatalf("unindexed corrupt truth leaked or drifted: %#v\n%s", report.CorruptObjects, stdout)
	}
	humanCode, human, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug,
	)
	if humanCode != 3 ||
		!strings.Contains(human, wrongHash) ||
		!strings.Contains(human, directoryHash) ||
		!strings.Contains(human, "kind: regular-hash-wrong") ||
		!strings.Contains(human, "kind: directory") ||
		strings.Contains(human, string(secretContent)) ||
		strings.Contains(human, root) {
		t.Fatalf("unindexed corrupt human = %d\n%s", humanCode, human)
	}
}

func TestFeatureIntentArchiveCorruptObjectRemovalIsShellSafeAndExact(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("POSIX corrupt-object remediation is supported only on Linux and Darwin")
	}
	root, slug := intentArchiveCLIWorkspace(t)
	blobsRel, err := store.IntentArchiveBlobsRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(blobsRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	objectName := "managed object 'quoted';$(touch injected-substitution);`touch injected-backtick` $HOME wildcard*.blob"
	objectRel := blobsRel + "/" + objectName
	targetPath := filepath.Join(root, "secret-symlink-target")
	secretContent := []byte("sensitive symlink target content\n")
	if err := os.WriteFile(targetPath, secretContent, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, filepath.Join(root, filepath.FromSlash(objectRel))); err != nil {
		t.Fatal(err)
	}

	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
	)
	if code != 3 {
		t.Fatalf("shell-safe corrupt list = %d\n%s", code, stdout)
	}
	report := decodeIntentArchiveListReport(t, stdout)
	if len(report.CorruptObjects) != 1 {
		t.Fatalf("corrupt objects = %#v", report.CorruptObjects)
	}
	object := report.CorruptObjects[0]
	wantCommand := "rm -rf -- " + quoteIntentArchivePOSIXShell(objectRel)
	lines := strings.Split(object.Repair, "\n")
	command := lines[len(lines)-1]
	if object.Path != objectRel ||
		object.Kind != string(store.IntentArchiveBlobKindSymlink) ||
		command != wantCommand ||
		!strings.Contains(object.Repair, quoteIntentArchivePOSIXShell(objectRel)) ||
		strings.Contains(stdout, targetPath) ||
		strings.Contains(stdout, string(secretContent)) {
		t.Fatalf("shell-safe corrupt object = %#v\n%s", object, stdout)
	}

	shimDir := filepath.Join(root, "shell-bin")
	if err := os.Mkdir(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "rm-argv.log")
	shim := "#!/bin/sh\nprintf '%s\\n' \"$#\" \"$@\" > \"$INTENT_ARCHIVE_RM_LOG\"\n"
	if err := os.WriteFile(filepath.Join(shimDir, "rm"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	shell := exec.Command("/bin/sh", "-c", command)
	shell.Dir = root
	shell.Env = append(os.Environ(),
		"PATH="+shimDir+":/usr/bin:/bin",
		"INTENT_ARCHIVE_RM_LOG="+logPath,
	)
	if output, runErr := shell.CombinedOutput(); runErr != nil {
		t.Fatalf("rendered removal did not parse safely: %v\n%s", runErr, output)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	gotArgv := strings.Split(strings.TrimSuffix(string(logged), "\n"), "\n")
	wantArgv := []string{"3", "-rf", "--", objectRel}
	if !reflect.DeepEqual(gotArgv, wantArgv) {
		t.Fatalf("removal argv = %#v, want %#v", gotArgv, wantArgv)
	}
	for _, injected := range []string{"injected-substitution", "injected-backtick"} {
		if _, err := os.Lstat(filepath.Join(root, injected)); !os.IsNotExist(err) {
			t.Fatalf("shell injection created %q: %v", injected, err)
		}
	}
}

func TestFeatureIntentArchiveManagedBlobReportPathSafetyPredicate(t *testing.T) {
	const slug = "demo"
	blobsRel, err := store.IntentArchiveBlobsRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	safe := blobsRel + "/managed object 'quoted';$(literal) `literal` $HOME wildcard*.blob"
	if !intentArchiveManagedBlobReportPathSafe(slug, safe) {
		t.Fatalf("shell-metacharacter path was rejected: %q", safe)
	}
	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "absolute", path: "/" + blobsRel + "/object.blob"},
		{name: "traversal", path: blobsRel + "/../object.blob"},
		{name: "outside-managed-directory", path: ".tpatch/features/demo/object.blob"},
		{name: "nested-managed-path", path: blobsRel + "/nested/object.blob"},
		{name: "invalid-utf8", path: blobsRel + "/" + string([]byte{0xff})},
		{name: "c0-newline", path: blobsRel + "/object\n.blob"},
		{name: "c0-tab", path: blobsRel + "/object\t.blob"},
		{name: "c0-escape", path: blobsRel + "/object\x1b.blob"},
		{name: "del", path: blobsRel + "/object\x7f.blob"},
		{name: "c1", path: blobsRel + "/object\u0085.blob"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if intentArchiveManagedBlobReportPathSafe(slug, tc.path) {
				t.Fatalf("unsafe report path was accepted: %q", tc.path)
			}
		})
	}
}

func TestFeatureIntentArchiveControlCharacterPathsRefuseWithoutEchoOrMutation(t *testing.T) {
	const sentinel = "CONTROL-PATH-SENTINEL"
	for _, tc := range []struct {
		name    string
		control string
	}{
		{name: "newline", control: "\n"},
		{name: "tab", control: "\t"},
		{name: "escape", control: "\x1b"},
		{name: "del", control: "\x7f"},
		{name: "c1", control: "\u0085"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug), nil)
			blobsRel, err := store.IntentArchiveBlobsRel(slug)
			if err != nil {
				t.Fatal(err)
			}
			unsafeName := sentinel + tc.control + prepareRetryHeader + " forged-command.blob"
			unsafeRel := blobsRel + "/" + unsafeName
			unsafeAbs := filepath.Join(root, filepath.FromSlash(unsafeRel))
			if err := os.WriteFile(unsafeAbs, []byte("unsafe path fixture\n"), 0o600); err != nil {
				t.Skipf("filesystem does not support %s filenames: %v", tc.name, err)
			}
			before := readTree(t, filepath.Join(root, ".tpatch"))

			type invocation struct {
				name      string
				args      []string
				json      bool
				quietOnly bool
				purge     bool
			}
			invocations := []invocation{
				{name: "list-json", args: []string{"feature", "intent-archive", "list", slug, "--json", "--quiet"}, json: true},
				{name: "list-human", args: []string{"feature", "intent-archive", "list", slug}},
				{name: "list-quiet", args: []string{"feature", "intent-archive", "list", slug, "--quiet"}, quietOnly: true},
				{name: "purge-preview-json", args: []string{"feature", "intent-archive", "purge", slug, "--orphans", "--json", "--quiet"}, json: true, purge: true},
				{name: "purge-preview-human", args: []string{"feature", "intent-archive", "purge", slug, "--orphans"}, purge: true},
				{name: "purge-preview-quiet", args: []string{"feature", "intent-archive", "purge", slug, "--orphans", "--quiet"}, quietOnly: true, purge: true},
			}
			if intentlock.AuthoritySupported {
				invocations = append(invocations,
					invocation{name: "purge-confirmed-json", args: []string{"feature", "intent-archive", "purge", slug, "--orphans", "--yes", "--json", "--quiet"}, json: true, purge: true},
					invocation{name: "purge-confirmed-human", args: []string{"feature", "intent-archive", "purge", slug, "--orphans", "--yes"}, purge: true},
					invocation{name: "purge-confirmed-quiet", args: []string{"feature", "intent-archive", "purge", slug, "--orphans", "--yes", "--quiet"}, quietOnly: true, purge: true},
				)
			}

			for _, invocation := range invocations {
				t.Run(invocation.name, func(t *testing.T) {
					args := append([]string{"--path", root}, invocation.args...)
					code, stdout, stderr, _ := runPrepare(t, args...)
					if code != 3 {
						t.Fatalf("unsafe path exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
					}
					if invocation.json {
						if invocation.purge {
							report := decodeIntentArchivePurgeReport(t, stdout)
							if report.Refusal == nil ||
								report.Refusal.Code != string(store.IntentArchiveCodeIndexPathEscape) ||
								len(report.References) != 0 ||
								len(report.Blobs) != 0 ||
								report.RemainingRepairs != nil {
								t.Fatalf("unsafe purge JSON report = %#v", report)
							}
						} else {
							report := decodeIntentArchiveListReport(t, stdout)
							if report.Refusal == nil ||
								report.Refusal.Code != string(store.IntentArchiveCodeIndexPathEscape) ||
								len(report.Generations) != 0 ||
								len(report.CorruptObjects) != 0 ||
								len(report.Orphans) != 0 {
								t.Fatalf("unsafe list JSON report = %#v", report)
							}
						}
					} else if !strings.Contains(stdout, string(store.IntentArchiveCodeIndexPathEscape)) {
						t.Fatalf("closed refusal code missing from output:\n%s", stdout)
					}
					combined := stdout + stderr
					if strings.Contains(combined, sentinel) ||
						strings.Contains(combined, unsafeRel) ||
						strings.Contains(combined, prepareRetryHeader) ||
						strings.Contains(combined, "forged-command") ||
						strings.Contains(combined, "tpatch feature intent-archive purge") {
						t.Fatalf("unsafe path or forged repair leaked:\n%s", combined)
					}
					if invocation.quietOnly && strings.Count(stdout, "\n") != 1 {
						t.Fatalf("quiet refusal was line-forged: %q", stdout)
					}
					if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
						t.Fatal("unsafe observed path refusal changed the workspace")
					}
				})
			}
		})
	}
}

func TestFeatureIntentArchiveCorruptClassRemediationUsesPredictedClasses(t *testing.T) {
	t.Run("unreferenced-corrupt-needs-no-purge", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		expected := []byte("unreferenced corrupt expected\n")
		tombstoned := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, expected, store.IntentArchiveWireTombstoned)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, tombstoned)),
			map[string][]byte{tombstoned.ContentSHA256: []byte("wrong unreferenced bytes\n")},
		)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug,
			"--json", "--quiet",
		)
		if code != 3 {
			t.Fatalf("unreferenced corrupt list = %d\n%s", code, stdout)
		}
		report := decodeIntentArchiveListReport(t, stdout)
		entry := report.Generations[0].Entries[0]
		if entry.Storage != "corrupt" ||
			len(report.CorruptObjects) != 0 ||
			entry.Retry != "" ||
			strings.Contains(entry.Repair, "tpatch feature intent-archive purge") ||
			!strings.Contains(entry.Repair, "rm -rf -- "+quoteIntentArchivePOSIXShell(entry.BlobPath)) {
			t.Fatalf("unreferenced corrupt repair = %#v", entry)
		}
		humanCode, human, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug,
		)
		if humanCode != 3 ||
			strings.Contains(human, "tpatch feature intent-archive purge") ||
			strings.Contains(human, "Then run") {
			t.Fatalf("unreferenced corrupt human = %d\n%s", humanCode, human)
		}
	})

	t.Run("complete-class-then-one-total-dangling-retry", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		retainedBytes := []byte("retained corrupt expected\n")
		unreferencedBytes := []byte("unreferenced corrupt expected two\n")
		danglingBytes := []byte("existing dangling\n")
		retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, retainedBytes, store.IntentArchiveWireRetained)
		unreferenced := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, unreferencedBytes, store.IntentArchiveWireTombstoned)
		dangling := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactExploration, danglingBytes, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug,
				intentArchiveCLIGeneration(t, slug, retained),
				intentArchiveCLIGeneration(t, slug, unreferenced),
				intentArchiveCLIGeneration(t, slug, dangling),
			),
			map[string][]byte{
				retained.ContentSHA256:     []byte("wrong retained bytes\n"),
				unreferenced.ContentSHA256: []byte("wrong unreferenced bytes\n"),
			},
		)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug,
			"--json", "--quiet",
		)
		if code != 3 {
			t.Fatalf("multi-corrupt list = %d\n%s", code, stdout)
		}
		report := decodeIntentArchiveListReport(t, stdout)
		var corruptEntries []intentArchiveListEntryReport
		for _, generation := range report.Generations {
			for _, entry := range generation.Entries {
				if entry.Storage == "corrupt" {
					corruptEntries = append(corruptEntries, entry)
				}
			}
		}
		if len(corruptEntries) != 2 {
			t.Fatalf("corrupt entries = %#v", corruptEntries)
		}
		wantRetry := intentArchiveBlobRetry(slug, []string{retained.ContentSHA256, dangling.ContentSHA256})
		for _, entry := range corruptEntries {
			if !strings.Contains(entry.Repair, retained.ContentSHA256+".blob") ||
				!strings.Contains(entry.Repair, unreferenced.ContentSHA256+".blob") ||
				entry.Retry != wantRetry ||
				strings.Contains(entry.Retry, unreferenced.ContentSHA256) {
				t.Fatalf("class repair = %#v want retry %q", entry, wantRetry)
			}
		}
		humanCode, human, _, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug,
		)
		if humanCode != 3 ||
			strings.Count(human, "rm -rf -- ") != 2 ||
			strings.Count(human, wantRetry) != 1 ||
			strings.Contains(human, "Then run") {
			t.Fatalf("multi-corrupt human = %d\n%s", humanCode, human)
		}
		assertIntentArchiveTpatchRetriesHeaded(t, human)
	})
}

func TestFeatureIntentArchiveListDanglingCorruptAndOwnedTruth(t *testing.T) {
	for _, tc := range []struct {
		name        string
		wire        store.IntentArchiveWireState
		writeBlob   bool
		blobData    []byte
		wantExit    int
		wantStorage string
		wantRepair  string
		forbidden   []string
	}{
		{
			name:        "dangling",
			wire:        store.IntentArchiveWireRetained,
			wantExit:    3,
			wantStorage: "dangling",
			wantRepair:  "--blob HASH --yes",
			forbidden:   []string{"--orphans", "--abandon-transaction"},
		},
		{
			name:        "corrupt",
			wire:        store.IntentArchiveWireRetained,
			writeBlob:   true,
			blobData:    []byte("wrong bytes\n"),
			wantExit:    3,
			wantStorage: "corrupt",
			wantRepair:  "rm -rf -- '.tpatch/features/",
			forbidden:   []string{"--orphans", "--abandon-transaction"},
		},
		{
			name:        "owned-corrupt",
			wire:        store.IntentArchiveWireRemovalPending,
			writeBlob:   true,
			blobData:    []byte("wrong owned bytes\n"),
			wantExit:    0,
			wantStorage: "pending-remove",
			wantRepair:  "--blob HASH --yes",
			forbidden:   []string{`"storage": "corrupt"`, "archive-purge-evidence-divergent", "--abandon-transaction"},
		},
		{
			name:        "owned-absent",
			wire:        store.IntentArchiveWireRemovalPending,
			wantExit:    0,
			wantStorage: "pending-finalize",
			wantRepair:  "--blob HASH --yes",
			forbidden:   []string{"dangling", "not recoverable", "--abandon-transaction"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			expected := []byte("expected " + tc.name + "\n")
			replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, expected, tc.wire)
			blobs := map[string][]byte{}
			if tc.writeBlob {
				blobs[replacement.ContentSHA256] = tc.blobData
			}
			writeIntentArchiveCLIFixture(t, root, slug,
				intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
				blobs,
			)
			before := readTree(t, filepath.Join(root, ".tpatch"))
			code, stdout, _, _ := runPrepare(
				t, "--path", root, "feature", "intent-archive", "list", slug,
				"--json", "--quiet",
			)
			if code != tc.wantExit {
				t.Fatalf("list %s = %d\n%s", tc.name, code, stdout)
			}
			report := decodeIntentArchiveListReport(t, stdout)
			entry := report.Generations[0].Entries[0]
			wantRepair := strings.ReplaceAll(tc.wantRepair, "HASH", replacement.ContentSHA256)
			if entry.Storage != tc.wantStorage || !strings.Contains(entry.Repair, wantRepair) {
				t.Fatalf("%s entry = %#v", tc.name, entry)
			}
			if tc.name == "corrupt" &&
				!strings.Contains(entry.Repair, "No tpatch repair selector runs anywhere in this archive until every corrupt-object removal has happened.") {
				t.Fatalf("corrupt repair omitted its blocking-class rule: %q", entry.Repair)
			}
			for _, forbidden := range tc.forbidden {
				if strings.Contains(stdout, forbidden) {
					t.Fatalf("%s output contains forbidden %q\n%s", tc.name, forbidden, stdout)
				}
			}
			if strings.Contains(stdout, root) {
				t.Fatalf("%s leaked absolute root\n%s", tc.name, stdout)
			}
			if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
				t.Fatalf("%s list mutated the workspace", tc.name)
			}
		})
	}
}

func TestFeatureIntentArchiveZeroGitProcessesAcrossSurfaces(t *testing.T) {
	runWithTrace := func(t *testing.T, args ...string) (int, string) {
		t.Helper()
		traceDir := t.TempDir()
		trace2 := filepath.Join(traceDir, "trace2.json")
		trace := filepath.Join(traceDir, "trace.log")
		t.Setenv("GIT_TRACE2_EVENT", trace2)
		t.Setenv("GIT_TRACE", trace)
		code, stdout, _, _ := runPrepare(t, args...)
		assertIntentArchiveNoGitTrace(t, trace2)
		assertIntentArchiveNoGitTrace(t, trace)
		return code, stdout
	}

	t.Run("list", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("process-spy list\n")
		replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: data},
		)
		code, stdout := runWithTrace(
			t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("list process spy = %d\n%s", code, stdout)
		}
	})

	for _, selector := range []string{"blob", "generation", "all", "orphans"} {
		for _, confirmed := range []bool{false, true} {
			name := selector + "-preview"
			if confirmed {
				name = selector + "-confirmed"
			}
			t.Run(name, func(t *testing.T) {
				if confirmed && !intentlock.AuthoritySupported {
					t.Skip("confirmed mutation is unsupported on this target")
				}
				root, slug := intentArchiveCLIWorkspace(t)
				args := []string{"--path", root, "feature", "intent-archive", "purge", slug}
				if selector == "orphans" {
					orphanBytes := []byte("process-spy orphan\n")
					sum := sha256.Sum256(orphanBytes)
					hash := hex.EncodeToString(sum[:])
					writeIntentArchiveCLIFixture(t, root, slug,
						intentArchiveCLIIndex(t, slug),
						map[string][]byte{hash: orphanBytes},
					)
					args = append(args, "--orphans")
				} else {
					data := []byte("process-spy " + selector + "\n")
					replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
					generation := intentArchiveCLIGeneration(t, slug, replacement)
					writeIntentArchiveCLIFixture(t, root, slug,
						intentArchiveCLIIndex(t, slug, generation),
						map[string][]byte{replacement.ContentSHA256: data},
					)
					switch selector {
					case "blob":
						args = append(args, "--blob", replacement.ContentSHA256)
					case "generation":
						args = append(args, "--generation", generation.GenerationID)
					case "all":
						args = append(args, "--all")
					}
				}
				if confirmed {
					args = append(args, "--yes")
				}
				args = append(args, "--json", "--quiet")
				code, stdout := runWithTrace(t, args...)
				if code != 0 {
					t.Fatalf("%s process spy = %d\n%s", name, code, stdout)
				}
			})
		}
	}
}

// PIB-361 plus fixed-schema, registration, and quiet guards.
func TestFeatureIntentArchiveQuietHistoryPathsAndSourceGuards(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	data := []byte("quiet archive\n")
	replacement := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
	writeIntentArchiveCLIFixture(t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
		map[string][]byte{replacement.ContentSHA256: data},
	)
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--all", "--quiet",
	)
	if code != 0 || stderr != "" ||
		strings.Count(stdout, "\n") != 1 ||
		!strings.HasPrefix(stdout, intentArchiveCommandPurge+" "+slug+": planned") {
		t.Fatalf("quiet = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, jsonOut, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--all", "--json", "--quiet",
	)
	if code != 0 ||
		strings.Contains(jsonOut, root) ||
		!strings.Contains(jsonOut, intentArchiveHistoryDisclosure) {
		t.Fatalf("path/history report = %d\n%s", code, jsonOut)
	}

	reportTypes := []reflect.Type{
		reflect.TypeOf(intentArchiveListReport{}),
		reflect.TypeOf(intentArchivePurgeReport{}),
	}
	seen := map[reflect.Type]bool{}
	var rejectMap func(reflect.Type)
	rejectMap = func(typ reflect.Type) {
		if typ == nil || seen[typ] {
			return
		}
		seen[typ] = true
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() == reflect.Map {
			t.Fatalf("wire schema contains Go map type %v", typ)
		}
		if typ.Kind() != reflect.Struct {
			return
		}
		for index := 0; index < typ.NumField(); index++ {
			if typ.Field(index).PkgPath == "" {
				rejectMap(typ.Field(index).Type)
			}
		}
	}
	for _, typ := range reportTypes {
		rejectMap(typ)
	}

	rootCmd := buildRootCmd()
	feature, _, err := rootCmd.Find([]string{"feature"})
	if err != nil {
		t.Fatal(err)
	}
	archive, _, err := feature.Find([]string{"intent-archive"})
	if err != nil || archive == feature {
		t.Fatalf("intent-archive command is not registered: %v", err)
	}
	if runtime.GOOS == "windows" && !intentlock.AuthoritySupported {
		t.Log("confirmed mutation correctly remains unsupported on this target")
	}
}

//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	_ "unsafe"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

//go:linkname s7ASAfterPurgeIndexDecode github.com/tesseracode/tesserapatch/internal/store.afterPurgeIndexDecode
var s7ASAfterPurgeIndexDecode func(string)

type s7ASArchiveFixture struct {
	root         string
	slug         string
	hash         string
	generationID string
	blobRel      string
	indexRel     string
	data         []byte
}

type s7ASPendingArchiveFixture struct {
	root         string
	slug         string
	hashes       []string
	generationID string
	blobRels     map[string]string
	indexRel     string
}

type s7ASRemoveSpyStorage struct {
	store.IntentArchiveStorage
	removed            *[]string
	decodeObserved     *bool
	removeBeforeDecode *bool
}

func (spy *s7ASRemoveSpyStorage) RemoveBlob(
	blobRel string,
	expected store.IntentArchiveIdentityToken,
) (store.IntentArchiveMutationResult, error) {
	if spy.decodeObserved != nil && spy.removeBeforeDecode != nil && !*spy.decodeObserved {
		*spy.removeBeforeDecode = true
	}
	if spy.removed != nil {
		*spy.removed = append(*spy.removed, blobRel)
	}
	return spy.IntentArchiveStorage.RemoveBlob(blobRel, expected)
}

type s7ASSelectorCase struct {
	name string
	kind store.IntentArchiveSelectorKind
}

func s7ASSelectorCases() []s7ASSelectorCase {
	return []s7ASSelectorCase{
		{name: "blob", kind: store.IntentArchiveSelectorBlob},
		{name: "generation", kind: store.IntentArchiveSelectorGeneration},
		{name: "all", kind: store.IntentArchiveSelectorAll},
		{name: "orphans", kind: store.IntentArchiveSelectorOrphans},
	}
}

func s7ASSelectorArgs(selector s7ASSelectorCase, hash, generationID string) []string {
	switch selector.kind {
	case store.IntentArchiveSelectorBlob:
		return []string{"--blob", hash}
	case store.IntentArchiveSelectorGeneration:
		return []string{"--generation", generationID}
	case store.IntentArchiveSelectorAll:
		return []string{"--all"}
	case store.IntentArchiveSelectorOrphans:
		return []string{"--orphans"}
	default:
		return nil
	}
}

func s7ASRenderedRetry(
	slug string,
	selectorArgs []string,
	confirmed, asJSON, quiet bool,
) string {
	argv := []string{"tpatch", "feature", "intent-archive", "purge", slug}
	argv = append(argv, selectorArgs...)
	if confirmed {
		argv = append(argv, "--yes")
	}
	if asJSON {
		argv = append(argv, "--json")
	}
	if quiet {
		argv = append(argv, "--quiet")
	}
	return strings.Join(argv, " ")
}

func s7ASPurgeArgs(
	root, slug string,
	selectorArgs []string,
	confirmed, asJSON, quiet bool,
) []string {
	argv := []string{"--path", root, "feature", "intent-archive", "purge", slug}
	argv = append(argv, selectorArgs...)
	if confirmed {
		argv = append(argv, "--yes")
	}
	if asJSON {
		argv = append(argv, "--json")
	}
	if quiet {
		argv = append(argv, "--quiet")
	}
	return argv
}

func s7ASWriteResidueFixture(t *testing.T, root, slug string, data []byte) s7ASArchiveFixture {
	t.Helper()
	replacement := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireTombstoned,
	)
	generation := intentArchiveCLIGeneration(t, slug, replacement)
	index := intentArchiveCLIIndex(t, slug, generation)
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{replacement.ContentSHA256: data})
	blobRel, err := store.IntentArchiveBlobRel(slug, replacement.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	return s7ASArchiveFixture{
		root:         root,
		slug:         slug,
		hash:         replacement.ContentSHA256,
		generationID: generation.GenerationID,
		blobRel:      blobRel,
		indexRel:     indexRel,
		data:         append([]byte(nil), data...),
	}
}

func s7ASWriteCleanArchiveFixture(t *testing.T, root, slug string, data []byte) s7ASArchiveFixture {
	t.Helper()
	replacement := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained,
	)
	generation := intentArchiveCLIGeneration(t, slug, replacement)
	index := intentArchiveCLIIndex(t, slug, generation)
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{replacement.ContentSHA256: data})
	blobRel, err := store.IntentArchiveBlobRel(slug, replacement.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	return s7ASArchiveFixture{
		root:         root,
		slug:         slug,
		hash:         replacement.ContentSHA256,
		generationID: generation.GenerationID,
		blobRel:      blobRel,
		indexRel:     indexRel,
		data:         append([]byte(nil), data...),
	}
}

func s7ASWritePendingArchiveFixture(
	t *testing.T,
	root, slug string,
	pendingCount int,
) s7ASPendingArchiveFixture {
	t.Helper()
	artifactIDs := []store.IntentArchiveArtifactID{
		store.IntentArchiveArtifactAnalysis,
		store.IntentArchiveArtifactExploration,
		store.IntentArchiveArtifactSpec,
	}
	replacements := make([]store.IntentArchiveReplacement, 0, pendingCount)
	hashes := make([]string, 0, pendingCount)
	blobRels := make(map[string]string, pendingCount)
	blobs := make(map[string][]byte, pendingCount)
	for index := 0; index < pendingCount; index++ {
		data := []byte(fmt.Sprintf("S7 AS pending %d/%d\n", index+1, pendingCount))
		replacement := intentArchiveCLIReplacement(
			t, artifactIDs[index], data, store.IntentArchiveWireRemovalPending,
		)
		replacements = append(replacements, replacement)
		hashes = append(hashes, replacement.ContentSHA256)
		blobs[replacement.ContentSHA256] = data
		blobRel, err := store.IntentArchiveBlobRel(slug, replacement.ContentSHA256)
		if err != nil {
			t.Fatal(err)
		}
		blobRels[replacement.ContentSHA256] = blobRel
	}
	sort.Strings(hashes)
	generation := intentArchiveCLIGeneration(t, slug, replacements...)
	writeIntentArchiveCLIFixture(
		t,
		root,
		slug,
		intentArchiveCLIIndex(t, slug, generation),
		blobs,
	)
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	return s7ASPendingArchiveFixture{
		root:         root,
		slug:         slug,
		hashes:       hashes,
		generationID: generation.GenerationID,
		blobRels:     blobRels,
		indexRel:     indexRel,
	}
}

func s7ASDecodeOrderedKeys(t *testing.T, raw string) []string {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(raw))
	tokenValue, err := decoder.Token()
	if err != nil {
		t.Fatalf("read JSON opening token: %v\n%s", err, raw)
	}
	delim, ok := tokenValue.(json.Delim)
	if !ok || delim != '{' {
		t.Fatalf("JSON does not start with an object: %v\n%s", tokenValue, raw)
	}
	keys := []string{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			t.Fatalf("read JSON key: %v\n%s", err, raw)
		}
		key, ok := keyToken.(string)
		if !ok {
			t.Fatalf("JSON key token = %T (%v)", keyToken, keyToken)
		}
		keys = append(keys, key)
		var ignored json.RawMessage
		if err := decoder.Decode(&ignored); err != nil {
			t.Fatalf("discard JSON value for %s: %v\n%s", key, err, raw)
		}
	}
	closing, err := decoder.Token()
	if err != nil {
		t.Fatalf("read JSON closing token: %v\n%s", err, raw)
	}
	delim, ok = closing.(json.Delim)
	if !ok || delim != '}' {
		t.Fatalf("JSON does not end with an object: %v\n%s", closing, raw)
	}
	return keys
}

func s7ASDecodeRawObject(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &object); err != nil {
		t.Fatalf("decode JSON object: %v\n%s", err, raw)
	}
	return object
}

func s7ASDecodeRawArray(t *testing.T, raw json.RawMessage) []json.RawMessage {
	t.Helper()
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err != nil {
		t.Fatalf("decode JSON array: %v\n%s", err, string(raw))
	}
	return array
}

func s7ASAssertOrderedKeys(t *testing.T, label string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s keys = %v, want %v", label, got, want)
	}
}

func s7ASAssertRenderedArgv(t *testing.T, rendered string, want []string) {
	t.Helper()
	argv, err := s7APParseRenderedCommand(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("retry argv = %v, want %v", argv, want)
	}
}

func s7ASAssertNoLeak(t *testing.T, text, root string, forbidden ...string) {
	t.Helper()
	if strings.Contains(text, root) {
		t.Fatalf("output leaked workspace root %q\n%s", root, text)
	}
	for _, needle := range forbidden {
		if needle != "" && strings.Contains(text, needle) {
			t.Fatalf("output leaked forbidden %q\n%s", needle, text)
		}
	}
}

func s7ASOptionalTree(t *testing.T, root string) []byte {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return []byte("<absent>")
	} else if err != nil {
		t.Fatal(err)
	}
	return readTree(t, root)
}

func s7ASAuthorityArtifactPaths(t *testing.T, root string) []string {
	t.Helper()
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := strings.ToLower(entry.Name())
		if strings.Contains(name, "authority") ||
			strings.Contains(name, "lock") ||
			strings.Contains(name, "cache") {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

func s7ASAdvisoryMessage(report preparePublishReport, code string) string {
	for _, advisory := range report.Advisories {
		if advisory.Code == code {
			return advisory.Message
		}
	}
	return ""
}

func s7ASInstallDeterministicProvider(t *testing.T) {
	t.Helper()
	attempt := &s7AQRetryAttempt{}
	restore := s6InstallProviderFixture(
		t,
		&s7AQRetryProvider{attempt: attempt},
		provider.Config{
			Type:    "openai-compatible",
			BaseURL: "https://s7-as.invalid",
			Model:   "s7-as-deterministic",
		},
	)
	t.Cleanup(restore)
}

func s7ASValidateResiduePrepareReport(
	t *testing.T,
	surface string,
	code int,
	stdout string,
	report preparePublishReport,
	slug, hash string,
	mode prepareMode,
) {
	t.Helper()
	if code != 3 {
		t.Fatalf("%s exit = %d, want 3\n%s", surface, code, stdout)
	}
	if report.Mode != mode || report.Slug != slug || report.Outcome != "refused" {
		t.Fatalf("%s envelope = mode:%s slug:%q outcome:%q", surface, report.Mode, report.Slug, report.Outcome)
	}
	if report.Refusal == nil {
		t.Fatalf("%s refusal is absent", surface)
	}
	if report.Refusal.Code != string(store.IntentArchiveCodeIndexStorageInconsistent) {
		t.Fatalf("%s refusal code = %q", surface, report.Refusal.Code)
	}
	if report.Refusal.Retry != "" || report.Refusal.RetryCWD != "" {
		t.Fatalf("%s unexpected retry = %q cwd=%q", surface, report.Refusal.Retry, report.Refusal.RetryCWD)
	}
	wantRemediation := "Run tpatch feature intent-archive purge " + slug + " --orphans --yes from the workspace root."
	if report.Refusal.Remediation != wantRemediation {
		t.Fatalf("%s remediation = %q, want %q", surface, report.Refusal.Remediation, wantRemediation)
	}
	if report.Recovery != nil || report.PurgeProgress != nil || report.Archive != nil {
		t.Fatalf("%s leaked lower-precedence object: recovery=%#v progress=%#v archive=%#v",
			surface, report.Recovery, report.PurgeProgress, report.Archive)
	}
	if len(report.OrphanBlobs) != 0 && !reflect.DeepEqual(report.OrphanBlobs, []string{hash}) {
		t.Fatalf("%s orphan blobs = %v, want empty or [%s]", surface, report.OrphanBlobs, hash)
	}
	wantAdvisory := "1 orphan archive blob(s) remain; remove them with tpatch feature intent-archive purge " + slug + " --orphans --yes."
	if advisory := s7ASAdvisoryMessage(report, "archive-orphan-blobs"); advisory != "" && advisory != wantAdvisory {
		t.Fatalf("%s orphan advisory = %q, want empty or %q", surface, advisory, wantAdvisory)
	}
}

type s7ASPendingPrepareObservation struct {
	mode   prepareMode
	code   int
	stdout string
	stderr string
	report preparePublishReport
	writes int
}

func validateS7ASPendingPrepareObservation(
	observation s7ASPendingPrepareObservation,
	slug string,
	hashes []string,
) error {
	if observation.code != 3 {
		return fmt.Errorf("%s exit = %d, want 3", observation.mode, observation.code)
	}
	wantStderr := "error: prepare " + slug + ": " + string(observation.mode) + " refused recovery-pending\n"
	if observation.stderr != wantStderr {
		return fmt.Errorf("%s stderr = %q, want %q", observation.mode, observation.stderr, wantStderr)
	}
	report := observation.report
	if report.Mode != observation.mode || report.Slug != slug || report.Outcome != "refused" ||
		report.Action != "none" || report.Refusal == nil ||
		report.Refusal.Code != "recovery-pending" ||
		report.Refusal.RetryCWD != store.IntentArchiveRepairCWD ||
		report.Recovery != nil || report.PurgeProgress != nil || report.Archive != nil ||
		len(report.OrphanBlobs) != 0 || observation.writes != 0 {
		return fmt.Errorf("%s pending report = %#v writes=%d", observation.mode, report, observation.writes)
	}
	if len(report.Advisories) != 1 || report.Advisories[0].Code != "workspace-not-git" {
		return fmt.Errorf("%s advisories = %#v, want workspace-not-git only", observation.mode, report.Advisories)
	}
	if strings.Contains(report.Refusal.Retry, "--path") || strings.Contains(report.Refusal.Retry, "--all") {
		return fmt.Errorf("%s retry widened scope: %q", observation.mode, report.Refusal.Retry)
	}
	want := []string{"feature", "intent-archive", "purge", slug}
	for _, hash := range hashes {
		want = append(want, "--blob", hash)
	}
	want = append(want, "--yes")
	argv, err := s7APParseRenderedCommand(report.Refusal.Retry)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(argv, want) {
		return fmt.Errorf("%s retry argv = %v, want %v", observation.mode, argv, want)
	}
	for _, hash := range hashes {
		if !strings.Contains(observation.stdout, hash) {
			return fmt.Errorf("%s output omitted pending hash %s", observation.mode, hash)
		}
	}
	if strings.Contains(observation.stdout, `"recovery":`) || strings.Contains(observation.stdout, `"pending_purge":`) ||
		strings.Contains(observation.stdout, `"divergence":`) {
		return fmt.Errorf("%s output leaked higher-precedence archive object", observation.mode)
	}
	return nil
}

func TestS7ASUnreferencedResidueContracts(t *testing.T) {
	s7ASInstallDeterministicProvider(t)
	root, slug := intentArchiveCLIWorkspace(t)
	fixture := s7ASWriteResidueFixture(t, root, slug, []byte("PIB-521 unreferenced residue\n"))
	before := readTree(t, filepath.Join(root, ".tpatch"))

	prepareCases := []struct {
		surface string
		mode    prepareMode
		args    []string
	}{
		{
			surface: "prepare",
			mode:    prepareModeGenerate,
			args:    []string{"--path", root, "prepare", slug, "--json", "--quiet"},
		},
		{
			surface: "regenerate",
			mode:    prepareModeRegenerate,
			args:    []string{"--path", root, "prepare", slug, "--regenerate", "--json", "--quiet"},
		},
	}
	for _, tc := range prepareCases {
		code, stdout, _, _ := runPrepare(t, tc.args...)
		report := prepareS4Report(t, stdout)
		s7ASValidateResiduePrepareReport(t, tc.surface, code, stdout, report, slug, fixture.hash, tc.mode)
		s7ASAssertNoLeak(t, stdout, root, "archive-purge-evidence-divergent", "--abandon-transaction", `"pending_purge":`)
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatalf("%s mutated .tpatch", tc.surface)
		}
	}

	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("list = %d stderr=%q\n%s", code, stderr, stdout)
	}
	listReport := decodeIntentArchiveListReport(t, stdout)
	if listReport.Command != intentArchiveCommandList || listReport.Outcome != "listed" ||
		listReport.Slug != slug || listReport.Refusal != nil ||
		listReport.HistoryDisclosure != intentArchiveHistoryDisclosure ||
		len(listReport.Generations) != 1 || len(listReport.Orphans) != 1 {
		t.Fatalf("list report = %#v", listReport)
	}
	entry := listReport.Generations[0].Entries[0]
	artifactRel, err := store.IntentArchiveArtifactPath(store.IntentArchiveArtifactAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	wantEntryPath := prepareFeatureRel(slug) + "/" + artifactRel
	if entry.ArtifactID != string(store.IntentArchiveArtifactAnalysis) ||
		entry.Path != wantEntryPath ||
		entry.ContentSHA256 != fixture.hash ||
		entry.Storage != "orphan" ||
		!entry.Present || entry.BlobPath != fixture.blobRel ||
		entry.Repair != "tpatch feature intent-archive purge "+slug+" --orphans --yes" ||
		entry.Retry != entry.Repair || entry.RetryCWD != store.IntentArchiveRepairCWD ||
		entry.Availability != "present but globally unreferenced" ||
		len(entry.LiveGenerationIDs) != 0 ||
		!reflect.DeepEqual(entry.TombstoneGenerationIDs, []string{fixture.generationID}) {
		t.Fatalf("list entry = %#v", entry)
	}
	orphan := listReport.Orphans[0]
	if orphan.Hash != fixture.hash || orphan.Path != fixture.blobRel || orphan.Storage != "orphan" ||
		orphan.Repair != "tpatch feature intent-archive purge "+slug+" --orphans --yes" ||
		orphan.Retry != orphan.Repair || orphan.RetryCWD != store.IntentArchiveRepairCWD {
		t.Fatalf("list orphan = %#v", orphan)
	}
	s7ASAssertNoLeak(t, stdout, root, "archive-purge-evidence-divergent", "--abandon-transaction", `"pending_purge":`)
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("list mutated .tpatch")
	}

	structured, err := runDoctorCLI(t, root, "doctor", "--json", "--check", "D9")
	if err != nil {
		t.Fatalf("doctor D9 = %v\n%s", err, structured)
	}
	var doctorReport workflow.DoctorReport
	if err := json.Unmarshal([]byte(structured), &doctorReport); err != nil {
		t.Fatalf("decode doctor report: %v\n%s", err, structured)
	}
	if doctorReport.Summary.ChecksRun != 1 || doctorReport.Summary.Warnings != 1 ||
		len(doctorReport.Checks) != 1 || doctorReport.Checks[0].CheckID != "D9" ||
		doctorReport.Checks[0].Status != "clean" || len(doctorReport.Findings) != 1 {
		t.Fatalf("doctor report = %#v", doctorReport)
	}
	finding := doctorReport.Findings[0]
	if finding.CheckID != "D9" || finding.Code != "archive-orphan" || finding.Severity != "warning" ||
		finding.Feature != slug || finding.Tag != string(store.IntentArchiveRepairUnreferencedResidue) ||
		finding.Path != fixture.blobRel || finding.Fixable ||
		finding.Remediation != "tpatch feature intent-archive purge "+slug+" --orphans --yes" ||
		strings.Contains(finding.Message, "archive-purge-evidence-divergent") ||
		strings.Contains(finding.Message, "abandon") {
		t.Fatalf("doctor finding = %#v", finding)
	}
	s7ASAssertNoLeak(t, structured, root, "archive-purge-evidence-divergent", "--abandon-transaction", `"pending_purge":`)
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("doctor mutated .tpatch")
	}
}

func TestS7ASOrphanRepairContracts(t *testing.T) {
	s7ASInstallDeterministicProvider(t)
	root, slug := prepareS4Workspace(t, "S7 AS PIB 522")
	prepareS4WriteReadyBundle(t, root, slug, true)
	analysisPath := filepath.Join(root, ".tpatch", "features", slug, "analysis.md")
	orphanBytes := []byte("PIB-522 analysis bytes\n")
	if err := os.WriteFile(analysisPath, orphanBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := s7ASWriteResidueFixture(t, root, slug, orphanBytes)
	indexBefore, parsedBefore := readIntentArchiveCLIIndex(t, root, slug)

	decodedBeforeRemoval := false
	removeBeforeDecode := false
	removedPaths := []string{}
	previousHook := s7APBeforePurgeBlobRemove
	previousDecodeHook := s7ASAfterPurgeIndexDecode
	previousFactory := intentArchiveNewStorage
	s7ASAfterPurgeIndexDecode = func(indexRel string) {
		if indexRel == fixture.indexRel {
			decodedBeforeRemoval = true
		}
	}
	s7APBeforePurgeBlobRemove = func(blobRel string) {
		if blobRel != fixture.blobRel {
			t.Errorf("pre-removal hook path = %q, want %q", blobRel, fixture.blobRel)
		}
	}
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rootFS *os.Root,
	) store.IntentArchiveStorage {
		return &s7ASRemoveSpyStorage{
			IntentArchiveStorage: previousFactory(authority, rootFS),
			removed:              &removedPaths,
			decodeObserved:       &decodedBeforeRemoval,
			removeBeforeDecode:   &removeBeforeDecode,
		}
	}
	t.Cleanup(func() {
		s7APBeforePurgeBlobRemove = previousHook
		s7ASAfterPurgeIndexDecode = previousDecodeHook
		intentArchiveNewStorage = previousFactory
	})

	code, stdout, stderr, _ := runPrepare(
		t,
		"--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--yes", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("orphan repair = %d stderr=%q\n%s", code, stderr, stdout)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	if report.Outcome != string(store.IntentArchivePurgePurged) || report.Action != "none" ||
		report.PendingPurge != nil || report.Recovery != nil || report.Refusal != nil ||
		!reflect.DeepEqual(report.Hashes, []string{fixture.hash}) ||
		len(report.Blobs) != 1 || !report.Blobs[0].Removed || report.Blobs[0].Path != fixture.blobRel {
		t.Fatalf("orphan repair report = %#v", report)
	}
	if !decodedBeforeRemoval {
		t.Fatal("strict index decode was not observed before first removal")
	}
	if removeBeforeDecode {
		t.Fatal("blob removal ran before the post-decode seam fired")
	}
	if !reflect.DeepEqual(removedPaths, []string{fixture.blobRel}) {
		t.Fatalf("removed paths = %v, want [%s]", removedPaths, fixture.blobRel)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(fixture.blobRel))); !os.IsNotExist(err) {
		t.Fatalf("orphan blob still present: %v", err)
	}
	indexAfter, parsedAfter := readIntentArchiveCLIIndex(t, root, slug)
	if !bytes.Equal(indexBefore, indexAfter) || !reflect.DeepEqual(parsedBefore, parsedAfter) {
		t.Fatal("orphan repair rewrote index.json")
	}
	if parsedAfter.Generations[0].Replaced[0].WireState() != store.IntentArchiveWireTombstoned {
		t.Fatalf("wire state after repair = %s", parsedAfter.Generations[0].Replaced[0].WireState())
	}

	code, regenStdout, regenStderr, _ := runPrepare(
		t,
		"--path", root, "prepare", slug,
		"--regenerate", "--json", "--quiet",
	)
	regenReport := prepareS4Report(t, regenStdout)
	if code != 0 || regenStderr != "" || regenReport.Outcome != "published" {
		t.Fatalf("regenerate after repair = %d stderr=%q report=%#v\n%s", code, regenStderr, regenReport, regenStdout)
	}
	rehydrated, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fixture.blobRel)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rehydrated, orphanBytes) {
		t.Fatalf("rehydrated blob bytes = %q, want %q", rehydrated, orphanBytes)
	}
}

func TestS7ASOrphanPreviewContracts(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	fixture := s7ASWriteResidueFixture(t, root, slug, []byte("PIB-523 preview orphan\n"))
	before := readTree(t, filepath.Join(root, ".tpatch"))

	acquires := 0
	writes := 0
	previousAcquire := intentArchiveAcquireAuthority
	previousFactory := intentArchiveNewStorage
	intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
		acquires++
		return previousAcquire(path)
	}
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rootFS *os.Root,
	) store.IntentArchiveStorage {
		return &intentArchiveWriteSpyStorage{
			IntentArchiveStorage: previousFactory(authority, rootFS),
			writes:               &writes,
		}
	}
	t.Cleanup(func() {
		intentArchiveAcquireAuthority = previousAcquire
		intentArchiveNewStorage = previousFactory
	})

	code, stdout, stderr, _ := runPrepare(
		t,
		"--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("preview = %d stderr=%q\n%s", code, stderr, stdout)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	wantReport := intentArchivePurgeReport{
		SchemaVersion:     1,
		Command:           intentArchiveCommandPurge,
		Slug:              slug,
		Outcome:           string(store.IntentArchivePurgePlanned),
		Action:            "none",
		Selector:          string(store.IntentArchiveSelectorOrphans),
		Confirmed:         false,
		Hashes:            []string{fixture.hash},
		GenerationIDs:     []string{},
		References:        []intentArchivePurgeReferenceReport{},
		Blobs:             []intentArchivePurgeBlobReport{{Hash: fixture.hash, Path: fixture.blobRel, SizeBytes: int64(len(fixture.data)), Present: true, Removed: false}},
		OrphanBlobs:       []string{fixture.hash},
		Advisories:        []prepareAdvisoryReport{},
		HistoryDisclosure: intentArchiveHistoryDisclosure,
		Retry:             s7ASRenderedRetry(slug, []string{"--orphans"}, true, true, true),
		RetryCWD:          store.IntentArchiveRepairCWD,
	}
	if !reflect.DeepEqual(report, wantReport) {
		t.Fatalf("preview report mismatch\ngot:  %#v\nwant: %#v", report, wantReport)
	}
	if acquires != 0 || writes != 0 {
		t.Fatalf("preview effects: authority=%d writes=%d", acquires, writes)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("preview mutated .tpatch")
	}

	humanCode, human, humanErr, _ := runPrepare(
		t,
		"--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans",
	)
	if humanCode != 0 || humanErr != "" {
		t.Fatalf("human preview = %d stderr=%q\n%s", humanCode, humanErr, human)
	}
	wantHuman := strings.Join([]string{
		intentArchiveCommandPurge + " " + slug + ": planned",
		"selector: orphans",
		"confirmed: false",
		"hashes:",
		"  " + fixture.hash,
		"blobs:",
		fmt.Sprintf("  %s %s size=%d present=true removed=false", fixture.hash, fixture.blobRel, len(fixture.data)),
		"orphan blobs:",
		"  " + fixture.hash,
		prepareRetryHeader,
		"  " + s7ASRenderedRetry(slug, []string{"--orphans"}, true, false, false),
		intentArchiveHistoryDisclosure,
		"",
	}, "\n")
	if human != wantHuman {
		t.Fatalf("human preview mismatch\ngot:\n%s\nwant:\n%s", human, wantHuman)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("human preview mutated .tpatch")
	}
}

func TestS7ASPendingRecoveryRefusalContracts(t *testing.T) {
	for _, pendingCount := range []int{1, 2, 3} {
		root, slug := intentArchiveCLIWorkspace(t)
		fixture := s7ASWritePendingArchiveFixture(t, root, slug, pendingCount)
		before := readTree(t, filepath.Join(root, ".tpatch"))
		observations := []s7ASPendingPrepareObservation{}
		for _, tc := range []struct {
			mode prepareMode
			args []string
		}{
			{mode: prepareModeGenerate, args: []string{"--path", root, "prepare", slug, "--json", "--quiet"}},
			{mode: prepareModeManual, args: []string{"--path", root, "prepare", slug, "--manual", "--json", "--quiet"}},
			{mode: prepareModeRegenerate, args: []string{"--path", root, "prepare", slug, "--regenerate", "--json", "--quiet"}},
		} {
			writes := 0
			previousFactory := intentArchiveNewStorage
			intentArchiveNewStorage = func(
				authority *intentlock.WorkspaceAuthority,
				rootFS *os.Root,
			) store.IntentArchiveStorage {
				return &intentArchiveWriteSpyStorage{
					IntentArchiveStorage: previousFactory(authority, rootFS),
					writes:               &writes,
				}
			}
			code, stdout, stderr, _ := runPrepare(t, tc.args...)
			intentArchiveNewStorage = previousFactory
			report := prepareS4Report(t, stdout)
			observation := s7ASPendingPrepareObservation{
				mode:   tc.mode,
				code:   code,
				stdout: stdout,
				stderr: stderr,
				report: report,
				writes: writes,
			}
			if err := validateS7ASPendingPrepareObservation(observation, slug, fixture.hashes); err != nil {
				t.Fatalf("pending-%d %s: %v\n%s", pendingCount, tc.mode, err, stdout)
			}
			s7ASAssertNoLeak(t, stdout, root, "archive-purge-evidence-divergent")
			if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
				t.Fatalf("pending-%d %s mutated .tpatch", pendingCount, tc.mode)
			}
			observations = append(observations, observation)
		}

		recoveryArgv, err := s7APParseRenderedCommand(observations[0].report.Refusal.Retry)
		if err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := s7APRunFromWorkspace(t, root, recoveryArgv)
		if code != 0 || stderr != "" {
			t.Fatalf("pending-%d recovery = %d stderr=%q\n%s", pendingCount, code, stderr, stdout)
		}
		wantRetry := "tpatch feature intent-archive purge " + slug
		for _, hash := range fixture.hashes {
			wantRetry += " --blob " + hash
		}
		wantRetry += " --yes"
		if !strings.HasPrefix(stdout, intentArchiveCommandPurge+" "+slug+": recovered\n") ||
			!strings.Contains(stdout, "Recovered pending purge state. The requested selector was not processed.\n") ||
			!strings.Contains(stdout, prepareRetryHeader+"\n  "+wantRetry+"\n") {
			t.Fatalf("pending-%d recovery human =\n%s", pendingCount, stdout)
		}
		for _, hash := range fixture.hashes {
			if !strings.Contains(stdout, "  finalized "+hash+"\n") {
				t.Fatalf("pending-%d recovery omitted finalized %s\n%s", pendingCount, hash, stdout)
			}
		}
		code, stdout, stderr, _ = runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		proceed := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || proceed.Outcome != "published" {
			t.Fatalf(
				"pending-%d prepare did not proceed: exit=%d stderr=%q report=%#v\n%s",
				pendingCount, code, stderr, proceed, stdout,
			)
		}
	}
}

func TestS7ASPendingRecoveryRefusalAllSensitivity(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	fixture := s7ASWritePendingArchiveFixture(t, root, slug, 2)
	code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
	observation := s7ASPendingPrepareObservation{
		mode:   prepareModeGenerate,
		code:   code,
		stdout: stdout,
		stderr: stderr,
		report: prepareS4Report(t, stdout),
	}
	observation.report.Refusal.Retry = "tpatch feature intent-archive purge " + slug + " --all --yes"
	if err := validateS7ASPendingPrepareObservation(observation, slug, fixture.hashes); err == nil {
		t.Fatal("validator accepted a widened --all retry for pending prepare refusal")
	}
}

func TestS7ASPendingOrphanRecoveryContracts(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	pending := s7ASWritePendingArchiveFixture(t, root, slug, 1)
	orphanBytes := []byte("PIB-527 orphan bytes\n")
	orphan := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, orphanBytes, store.IntentArchiveWireTombstoned,
	)
	orphanRel, err := store.IntentArchiveBlobRel(slug, orphan.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(orphanRel)), orphanBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	pendingRel := pending.blobRels[pending.hashes[0]]

	removedPaths := []string{}
	previousFactory := intentArchiveNewStorage
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rootFS *os.Root,
	) store.IntentArchiveStorage {
		return &s7ASRemoveSpyStorage{
			IntentArchiveStorage: previousFactory(authority, rootFS),
			removed:              &removedPaths,
		}
	}
	t.Cleanup(func() { intentArchiveNewStorage = previousFactory })

	code, stdout, stderr, _ := runPrepare(
		t,
		"--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--yes", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("first orphans --yes = %d stderr=%q\n%s", code, stderr, stdout)
	}
	first := decodeIntentArchivePurgeReport(t, stdout)
	if first.Outcome != string(store.IntentArchivePurgeRecovered) || first.Recovery == nil ||
		first.Recovery.Kind != "archive-purge-finalize" ||
		!reflect.DeepEqual(first.Recovery.FinalizedHashes, pending.hashes) ||
		first.Recovery.Retry != s7ASRenderedRetry(slug, []string{"--orphans"}, true, true, true) ||
		first.Recovery.RetryCWD != store.IntentArchiveRepairCWD {
		t.Fatalf("first recovery report = %#v", first)
	}
	if !reflect.DeepEqual(removedPaths, []string{pendingRel}) {
		t.Fatalf("first removal set = %v, want [%s]", removedPaths, pendingRel)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(orphanRel))); err != nil {
		t.Fatalf("orphan was touched on first invocation: %v", err)
	}

	removedPaths = nil
	code, stdout, stderr, _ = runPrepare(
		t,
		"--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--yes", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("second orphans --yes = %d stderr=%q\n%s", code, stderr, stdout)
	}
	second := decodeIntentArchivePurgeReport(t, stdout)
	if second.Outcome != string(store.IntentArchivePurgePurged) || len(second.Blobs) != 1 ||
		!second.Blobs[0].Removed || second.Blobs[0].Path != orphanRel {
		t.Fatalf("second purge report = %#v", second)
	}
	if !reflect.DeepEqual(removedPaths, []string{orphanRel}) {
		t.Fatalf("second removal set = %v, want [%s]", removedPaths, orphanRel)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(orphanRel))); !os.IsNotExist(err) {
		t.Fatalf("orphan remained after rerun: %v", err)
	}
}

func TestS7ASPendingPurgeSchemaContracts(t *testing.T) {
	for _, selector := range s7ASSelectorCases() {
		root, slug := intentArchiveCLIWorkspace(t)
		fixture := s7ASWritePendingArchiveFixture(t, root, slug, 2)
		selectorArgs := s7ASSelectorArgs(selector, fixture.hashes[0], fixture.generationID)
		code, stdout, stderr, _ := runPrepare(
			t,
			s7ASPurgeArgs(root, slug, selectorArgs, false, true, true)...,
		)
		if code != 0 || stderr != "" {
			t.Fatalf("%s preview = %d stderr=%q\n%s", selector.name, code, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgeRecoveryRequired) || report.Action != "none" ||
			report.Selector != string(selector.kind) || report.PendingPurge == nil ||
			report.PendingPurge.Selector != string(selector.kind) || report.PendingPurge.RetryCWD != store.IntentArchiveRepairCWD ||
			report.PendingPurge.PendingHashes == nil || len(report.PendingPurge.PendingHashes) != 2 ||
			len(report.Hashes) != 0 || len(report.GenerationIDs) != 0 || len(report.References) != 0 ||
			len(report.Blobs) != 0 || len(report.OrphanBlobs) != 0 || report.Recovery != nil ||
			report.PurgeProgress != nil || report.Divergence != nil {
			t.Fatalf("%s pending schema report = %#v", selector.name, report)
		}
		wantJSONRetry := s7ASRenderedRetry(slug, selectorArgs, true, true, true)
		wantJSONArgv := []string{"feature", "intent-archive", "purge", slug}
		wantJSONArgv = append(wantJSONArgv, selectorArgs...)
		wantJSONArgv = append(wantJSONArgv, "--yes", "--json", "--quiet")
		if report.PendingPurge.Retry != wantJSONRetry {
			t.Fatalf("%s JSON retry = %q, want %q", selector.name, report.PendingPurge.Retry, wantJSONRetry)
		}
		s7ASAssertRenderedArgv(t, report.PendingPurge.Retry, wantJSONArgv)
		s7ASAssertNoLeak(t, stdout, root, "--path", `"recovery":`, `"divergence":`, `: null`)

		top := s7ASDecodeRawObject(t, stdout)
		wantTopKeys := []string{
			"schema_version", "command", "slug", "outcome", "action", "selector",
			"confirmed", "hashes", "generation_ids", "references", "blobs",
			"orphan_blobs", "advisories", "history_disclosure",
		}
		if selector.kind == store.IntentArchiveSelectorAll {
			wantTopKeys = append(wantTopKeys, "blast_radius")
		}
		wantTopKeys = append(wantTopKeys, "pending_purge")
		s7ASAssertOrderedKeys(t, selector.name+" top-level", s7ASDecodeOrderedKeys(t, stdout), wantTopKeys)
		pendingKeys := s7ASDecodeOrderedKeys(t, string(top["pending_purge"]))
		s7ASAssertOrderedKeys(t, selector.name+" pending_purge", pendingKeys,
			[]string{"recovery_required", "pending_hashes", "selector", "retry", "retry_cwd"},
		)
		pendingObject := s7ASDecodeRawObject(t, string(top["pending_purge"]))
		pendingHashes := s7ASDecodeRawArray(t, pendingObject["pending_hashes"])
		if len(pendingHashes) != 2 {
			t.Fatalf("%s pending_hashes length = %d, want 2", selector.name, len(pendingHashes))
		}
		for index, hash := range fixture.hashes {
			entry := report.PendingPurge.PendingHashes[index]
			if entry.Hash != hash || entry.Blob != fixture.blobRels[hash] || entry.Index != fixture.indexRel || entry.Plan != intentArchivePendingPlan {
				t.Fatalf("%s pending hash %d = %#v", selector.name, index, entry)
			}
			perHashRaw := s7ASDecodeRawObject(t, string(pendingHashes[index]))
			s7ASAssertOrderedKeys(t, fmt.Sprintf("%s pending hash %d", selector.name, index), s7ASDecodeOrderedKeys(t, string(pendingHashes[index])),
				[]string{"hash", "blob", "index", "plan"},
			)
			if len(perHashRaw) != 4 {
				t.Fatalf("%s pending hash %d raw object = %v", selector.name, index, perHashRaw)
			}
		}

		humanCode, human, humanErr, _ := runPrepare(
			t,
			s7ASPurgeArgs(root, slug, selectorArgs, false, false, false)...,
		)
		if humanCode != 0 || humanErr != "" {
			t.Fatalf("%s human preview = %d stderr=%q\n%s", selector.name, humanCode, humanErr, human)
		}
		wantHumanRetry := s7ASRenderedRetry(slug, selectorArgs, true, false, false)
		wantHumanBlock := "    plan:  " + intentArchivePendingPlan + "\n" + prepareRetryHeader + "\n  " + wantHumanRetry + "\n"
		if !strings.Contains(human, "A previous purge stopped with pending references. Nothing was changed.\n") ||
			!strings.Contains(human, wantHumanBlock) {
			t.Fatalf("%s human preview missing pending schema\n%s", selector.name, human)
		}
		if strings.Count(human, prepareRetryHeader) != 1 ||
			strings.Count(human, "    plan:  "+intentArchivePendingPlan+"\n") !=
				len(fixture.hashes) {
			t.Fatalf(
				"%s human heading/plan counts = %d/%d, want 1/%d\n%s",
				selector.name,
				strings.Count(human, prepareRetryHeader),
				strings.Count(human, "    plan:  "+intentArchivePendingPlan+"\n"),
				len(fixture.hashes),
				human,
			)
		}
		for _, hash := range fixture.hashes {
			if !strings.Contains(human, "  pending hash: "+hash+"\n") ||
				!strings.Contains(human, "    blob:  "+fixture.blobRels[hash]+"\n") ||
				!strings.Contains(human, "    index: "+fixture.indexRel+"\n") {
				t.Fatalf("%s human preview missing pending hash %s\n%s", selector.name, hash, human)
			}
		}
		s7ASAssertNoLeak(t, human, root, "--path")
	}
}

func TestS7ASAuthorityProcessSpyContracts(t *testing.T) {
	originalAcquire := intentArchiveAcquireAuthority
	t.Cleanup(func() { intentArchiveAcquireAuthority = originalAcquire })
	for _, fixtureName := range []string{"clean", "pending", "journal"} {
		for _, selector := range s7ASSelectorCases() {
			for _, confirmed := range []bool{false, true} {
				root, slug := intentArchiveCLIWorkspace(t)
				cleanFixture := s7ASWriteCleanArchiveFixture(t, root, slug, []byte("PIB-530 clean bytes\n"))
				pendingFixture := s7ASPendingArchiveFixture{}
				if fixtureName == "pending" {
					pendingFixture = s7ASWritePendingArchiveFixture(t, root, slug, 1)
				}
				if fixtureName == "journal" {
					s6WriteJournalFixture(t, root, slug, "journal-corrupt")
				}
				localBefore := s7ASOptionalTree(
					t, filepath.Join(root, ".tpatch", "local"),
				)
				authorityArtifactsBefore := s7ASAuthorityArtifactPaths(
					t, filepath.Join(root, ".tpatch"),
				)

				selectorArgs := s7ASSelectorArgs(selector, cleanFixture.hash, cleanFixture.generationID)
				if fixtureName == "pending" {
					selectorArgs = s7ASSelectorArgs(selector, pendingFixture.hashes[0], pendingFixture.generationID)
				}

				acquires := 0
				previousAcquire := intentArchiveAcquireAuthority
				intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
					acquires++
					return previousAcquire(path)
				}
				traceDir := t.TempDir()
				trace2 := filepath.Join(traceDir, "trace2.json")
				trace := filepath.Join(traceDir, "trace.log")
				t.Setenv("GIT_TRACE2_EVENT", trace2)
				t.Setenv("GIT_TRACE", trace)

				code, stdout, stderr, _ := runPrepare(
					t,
					s7ASPurgeArgs(root, slug, selectorArgs, confirmed, true, true)...,
				)
				intentArchiveAcquireAuthority = previousAcquire
				report := decodeIntentArchivePurgeReport(t, stdout)
				label := fmt.Sprintf("%s/%s/%t", fixtureName, selector.name, confirmed)

				wantCode := 0
				wantOutcome := string(store.IntentArchivePurgePlanned)
				wantRefusal := ""
				switch fixtureName {
				case "clean":
					if confirmed {
						wantOutcome = string(store.IntentArchivePurgePurged)
						if selector.kind == store.IntentArchiveSelectorOrphans {
							wantOutcome = string(store.IntentArchivePurgeNoOp)
						}
					}
				case "pending":
					if confirmed {
						wantOutcome = string(store.IntentArchivePurgeRecovered)
					} else {
						wantOutcome = string(store.IntentArchivePurgeRecoveryRequired)
					}
				case "journal":
					wantCode = 3
					wantOutcome = "refused"
					wantRefusal = "recovery-pending"
				}
				if code != wantCode || report.Outcome != wantOutcome {
					t.Fatalf("%s result = exit:%d report:%#v", label, code, report)
				}
				wantStderr := ""
				if fixtureName == "journal" {
					wantStderr = "error: feature intent-archive purge " + slug +
						": refused recovery-pending\n"
				}
				if stderr != wantStderr {
					t.Fatalf("%s stderr = %q, want %q", label, stderr, wantStderr)
				}
				if wantRefusal == "" {
					if report.Refusal != nil && report.Refusal.Code != "" {
						t.Fatalf("%s unexpected refusal = %#v", label, report.Refusal)
					}
				} else if report.Refusal == nil || report.Refusal.Code != wantRefusal {
					t.Fatalf("%s refusal = %#v, want %s", label, report.Refusal, wantRefusal)
				}
				if confirmed {
					if acquires != 1 {
						t.Fatalf("%s authority acquires = %d, want 1", label, acquires)
					}
				} else if acquires != 0 {
					t.Fatalf("%s preview acquires = %d, want 0", label, acquires)
				}
				assertIntentArchiveNoGitTrace(t, trace2)
				assertIntentArchiveNoGitTrace(t, trace)
				if _, err := os.Stat(filepath.Join(root, ".tpatch", "authority.lock")); err == nil {
					t.Fatalf("%s left durable authority artifact", label)
				}
				localAfter := s7ASOptionalTree(
					t, filepath.Join(root, ".tpatch", "local"),
				)
				if !bytes.Equal(localBefore, localAfter) {
					t.Fatalf("%s changed the local lane", label)
				}
				authorityArtifactsAfter := s7ASAuthorityArtifactPaths(
					t, filepath.Join(root, ".tpatch"),
				)
				if !reflect.DeepEqual(
					authorityArtifactsBefore, authorityArtifactsAfter,
				) {
					t.Fatalf(
						"%s authority/cache/lock paths changed: %v -> %v",
						label, authorityArtifactsBefore, authorityArtifactsAfter,
					)
				}
			}
		}
	}
}

//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7PIB428DanglingAndCorruptArchiveTruthAcrossSurfaces(t *testing.T) {
	for _, test := range []struct {
		name       string
		writeBlob  bool
		blobData   []byte
		wantCode   string
		wantStore  string
		wantRepair func(string, string) string
		forbidden  []string
	}{
		{
			name:      "dangling",
			wantCode:  "archive-blob-dangling",
			wantStore: "dangling",
			wantRepair: func(slug, hash string) string {
				return "tpatch feature intent-archive purge " + slug + " --blob " + hash + " --yes"
			},
			forbidden: []string{"--orphans", "--abandon-transaction", "archive-purge-evidence-divergent"},
		},
		{
			name:       "hash-wrong-corrupt",
			writeBlob:  true,
			blobData:   []byte("wrong bytes\n"),
			wantCode:   "archive-blob-corrupt",
			wantStore:  "corrupt",
			wantRepair: func(_, _ string) string { return "rm -rf -- " },
			forbidden:  []string{`"storage": "dangling"`, "--orphans", "--abandon-transaction", "archive-purge-evidence-divergent"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, slug := intentArchiveCLIWorkspace(t)
			prepareS4WriteReadyBundle(t, root, slug, false)
			expected := []byte("expected " + test.name + "\n")
			replacement := intentArchiveCLIReplacement(
				t, store.IntentArchiveArtifactAnalysis, expected, store.IntentArchiveWireRetained,
			)
			blobs := map[string][]byte{}
			if test.writeBlob {
				blobs[replacement.ContentSHA256] = test.blobData
			}
			writeIntentArchiveCLIFixture(t, root, slug,
				intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
				blobs,
			)
			before := readTree(t, filepath.Join(root, ".tpatch"))
			wantRepair := test.wantRepair(slug, replacement.ContentSHA256)

			listCode, listOut, _, _ := runPrepare(
				t, "--path", root, "feature", "intent-archive", "list", slug,
				"--json", "--quiet",
			)
			list := decodeIntentArchiveListReport(t, listOut)
			entry := list.Generations[0].Entries[0]
			if listCode != 3 || entry.Storage != test.wantStore ||
				!strings.Contains(entry.Repair, wantRepair) {
				t.Fatalf("list = exit:%d entry:%+v\n%s", listCode, entry, listOut)
			}

			doctorCode, doctorOut, _, _ := runPrepare(
				t, "--path", root, "doctor", "--check", "D9", "--json",
			)
			if doctorCode != 0 || !strings.Contains(doctorOut, test.wantCode) ||
				!strings.Contains(doctorOut, wantRepair) {
				t.Fatalf("doctor = exit:%d\n%s", doctorCode, doctorOut)
			}

			prepareCode, prepareOut, _, _ := runPrepare(
				t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
			)
			report := prepareS4Report(t, prepareOut)
			if prepareCode != 3 || report.Refusal == nil ||
				report.Refusal.Code != test.wantCode ||
				!strings.Contains(report.Refusal.Remediation+" "+report.Refusal.Retry, wantRepair) {
				t.Fatalf("prepare = exit:%d report:%+v", prepareCode, report)
			}
			allOutput := listOut + doctorOut + prepareOut
			for _, forbidden := range test.forbidden {
				if strings.Contains(allOutput, forbidden) {
					t.Fatalf("%s output contains forbidden route %q\n%s", test.name, forbidden, allOutput)
				}
			}
			if strings.Contains(allOutput, root) {
				t.Fatalf("%s surfaces leaked absolute workspace path\n%s", test.name, allOutput)
			}
			if after := readTree(t, filepath.Join(root, ".tpatch")); !bytes.Equal(after, before) {
				t.Fatalf("%s read/refusal surfaces mutated workspace", test.name)
			}
		})
	}
}

func TestS7PIB424GenerationPurgeRefusesSharedHashWithoutMutation(t *testing.T) {
	setup := s7PIB424BuildRemediationSetup(t)
	t.Run("shared-refusal-json-human", func(t *testing.T) {
		if len(setup.commandsByKind) != 3 || setup.slug == "" || setup.sharedHash == "" {
			t.Fatalf("shared remediation setup is incomplete: %+v", setup)
		}
	})
	for _, fixture := range []struct {
		name string
	}{
		{name: "blob-confirmed"},
		{name: "all-preview"},
		{name: "all-confirmed"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			command := setup.commandsByKind[fixture.name]
			if command == "" {
				t.Fatalf("missing emitted %s remediation command", fixture.name)
			}
			commandRoot, commandSlug, _, commandSharedHash := s7PIB424SharedHashFixture(t)
			if commandSlug != setup.slug || commandSharedHash != setup.sharedHash {
				t.Fatal("fresh remediation fixture changed deterministic slug or shared hash")
			}
			beforeCommand := readTree(t, filepath.Join(commandRoot, ".tpatch"))
			args, flags, err := s7PIB424ParseCommand(command)
			if err != nil {
				t.Fatal(err)
			}
			runArgs := append([]string{"--path", commandRoot}, args...)
			commandCode, commandOut, commandErr, _ := runPrepare(t, runArgs...)
			switch fixture.name {
			case "blob-confirmed":
				if commandCode != 0 ||
					!s7PIB424EqualStrings(flags, []string{"--blob", "--yes"}) {
					t.Fatalf("narrow repair = exit:%d flags:%q\n%s%s", commandCode, flags, commandOut, commandErr)
				}
				s7PIB424AssertNarrowResult(t, commandRoot, commandSlug, commandSharedHash)
			case "all-preview":
				if commandCode != 0 ||
					!s7PIB424EqualStrings(flags, []string{"--all"}) ||
					!strings.Contains(commandOut+commandErr, "tombstones every reference in every generation") {
					t.Fatalf("all preview = exit:%d flags:%q\n%s%s", commandCode, flags, commandOut, commandErr)
				}
				if after := readTree(t, filepath.Join(commandRoot, ".tpatch")); !bytes.Equal(after, beforeCommand) {
					t.Fatal("--all preview mutated the archive")
				}
			case "all-confirmed":
				if commandCode != 0 ||
					!s7PIB424EqualStrings(flags, []string{"--all", "--yes"}) {
					t.Fatalf("all confirmed = exit:%d flags:%q\n%s%s", commandCode, flags, commandOut, commandErr)
				}
				s7PIB424AssertAllResult(t, commandRoot, commandSlug)
			default:
				t.Fatalf("unexpected remediation command %q", command)
			}
		})
	}
}

type s7PIB424RemediationSetup struct {
	slug, sharedHash string
	commandsByKind   map[string]string
}

func s7PIB424BuildRemediationSetup(t *testing.T) s7PIB424RemediationSetup {
	t.Helper()
	root, slug, index, sharedHash := s7PIB424SharedHashFixture(t)
	before := readTree(t, filepath.Join(root, ".tpatch"))
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--generation", index.Generations[0].GenerationID, "--yes", "--json", "--quiet",
	)
	report := decodeIntentArchivePurgeReport(t, stdout)
	wantNarrow := "tpatch feature intent-archive purge " + slug +
		" --blob " + sharedHash + " --yes"
	wantAll := "tpatch feature intent-archive purge " + slug + " --all"
	if code != 3 || report.Refusal == nil ||
		report.Refusal.Code != "archive-blob-shared" ||
		report.Refusal.Retry != wantNarrow ||
		!strings.Contains(report.Refusal.Remediation, wantAll) ||
		!strings.Contains(report.Refusal.Remediation, wantAll+" --yes") {
		t.Fatalf("shared-generation refusal = exit:%d refusal:%+v report:%+v", code, report.Refusal, report)
	}
	if after := readTree(t, filepath.Join(root, ".tpatch")); !bytes.Equal(after, before) {
		t.Fatal("shared-generation refusal mutated the archive")
	}

	jsonCommands := append(
		[]string{report.Refusal.Retry},
		s7PIB424ExtractCommands(t, report.Refusal.Remediation)...,
	)
	jsonCommands = s7PIB424UniqueSorted(jsonCommands)
	if got, want := jsonCommands, []string{wantNarrow, wantAll, wantAll + " --yes"}; !s7PIB424EqualStrings(got, s7PIB424UniqueSorted(want)) {
		t.Fatalf("JSON remediation commands = %q, want %q", got, want)
	}
	for _, command := range jsonCommands {
		if _, _, err := s7PIB424ParseCommand(command); err != nil {
			t.Fatalf("parse JSON remediation %q: %v", command, err)
		}
	}

	humanRoot, humanSlug, humanIndex, _ := s7PIB424SharedHashFixture(t)
	humanBefore := readTree(t, filepath.Join(humanRoot, ".tpatch"))
	humanCode, humanOut, humanErr, _ := runPrepare(
		t, "--path", humanRoot, "feature", "intent-archive", "purge", humanSlug,
		"--generation", humanIndex.Generations[0].GenerationID, "--yes",
	)
	human := humanOut + humanErr
	humanCommands := s7PIB424UniqueSorted(s7PIB424ExtractCommands(t, human))
	if humanCode != 3 || !s7PIB424EqualStrings(humanCommands, jsonCommands) ||
		!strings.Contains(human, "archive-blob-shared") ||
		!strings.Contains(human, report.Refusal.Remediation) {
		t.Fatalf("human refusal = exit:%d commands:%q\n%s", humanCode, humanCommands, human)
	}
	if after := readTree(t, filepath.Join(humanRoot, ".tpatch")); !bytes.Equal(after, humanBefore) {
		t.Fatal("human shared-generation refusal mutated the archive")
	}

	commandsByKind := make(map[string]string, len(jsonCommands))
	for _, command := range jsonCommands {
		kind := s7PIB424CommandKind(command)
		if kind == "unknown" || commandsByKind[kind] != "" {
			t.Fatalf("ambiguous remediation command kind %q for %q", kind, command)
		}
		commandsByKind[kind] = command
	}
	return s7PIB424RemediationSetup{
		slug: slug, sharedHash: sharedHash, commandsByKind: commandsByKind,
	}
}

func s7PIB424SharedHashFixture(
	t *testing.T,
) (string, string, store.IntentArchiveIndex, string) {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	sharedBody := []byte("shared by generations\n")
	firstOnlyBody := []byte("first generation only\n")
	secondOnlyBody := []byte("second generation only\n")
	sharedFirst := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, sharedBody, store.IntentArchiveWireRetained,
	)
	sharedSecond := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, sharedBody, store.IntentArchiveWireRetained,
	)
	firstOnly := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, firstOnlyBody, store.IntentArchiveWireRetained,
	)
	secondOnly := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactExploration, secondOnlyBody, store.IntentArchiveWireRetained,
	)
	index := intentArchiveCLIIndex(t, slug,
		intentArchiveCLIGeneration(t, slug, sharedFirst, firstOnly),
		intentArchiveCLIGeneration(t, slug, secondOnly, sharedSecond),
	)
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{
		sharedFirst.ContentSHA256: sharedBody,
		firstOnly.ContentSHA256:   firstOnlyBody,
		secondOnly.ContentSHA256:  secondOnlyBody,
	})
	return root, slug, index, sharedFirst.ContentSHA256
}

var s7PIB424CommandPattern = regexp.MustCompile(
	`tpatch feature intent-archive purge [a-z0-9]+(?:-[a-z0-9]+)*(?: --(?:blob|generation) [a-f0-9]+| --(?:orphans|all|yes|json|quiet))+`,
)

func s7PIB424ExtractCommands(t *testing.T, text string) []string {
	t.Helper()
	commands := s7PIB424CommandPattern.FindAllString(text, -1)
	if len(commands) == 0 {
		t.Fatalf("no executable tpatch remediation command in %q", text)
	}
	return commands
}

func s7PIB424ParseCommand(command string) ([]string, []string, error) {
	if strings.TrimSpace(command) != command ||
		strings.ContainsAny(command, "\r\n\t'\"\\;$|&`()<>") {
		return nil, nil, fmt.Errorf("command contains shell syntax or control bytes")
	}
	fields := strings.Fields(command)
	if len(fields) < 6 ||
		!s7PIB424EqualStrings(fields[:4], []string{"tpatch", "feature", "intent-archive", "purge"}) ||
		!regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(fields[4]) {
		return nil, nil, fmt.Errorf("command prefix or slug is not canonical")
	}
	flags := []string{}
	selectorFamilies := 0
	for index := 5; index < len(fields); index++ {
		flag := fields[index]
		switch flag {
		case "--blob", "--generation":
			selectorFamilies++
			if index+1 >= len(fields) ||
				!regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(fields[index+1]) {
				return nil, nil, fmt.Errorf("%s lacks a canonical identifier", flag)
			}
			index++
		case "--orphans", "--all":
			selectorFamilies++
		case "--yes", "--json", "--quiet":
		default:
			return nil, nil, fmt.Errorf("unrecognized emitted flag %q", flag)
		}
		flags = append(flags, flag)
	}
	if selectorFamilies != 1 {
		return nil, nil, fmt.Errorf("command has %d selector families", selectorFamilies)
	}
	return fields[1:], flags, nil
}

func s7PIB424CommandKind(command string) string {
	switch {
	case strings.Contains(command, " --blob "):
		return "blob-confirmed"
	case strings.HasSuffix(command, " --all --yes"):
		return "all-confirmed"
	case strings.HasSuffix(command, " --all"):
		return "all-preview"
	default:
		return "unknown"
	}
}

func s7PIB424AssertNarrowResult(t *testing.T, root, slug, sharedHash string) {
	t.Helper()
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	tombstoned := 0
	retained := 0
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			switch {
			case replacement.ContentSHA256 == sharedHash &&
				replacement.WireState() == store.IntentArchiveWireTombstoned:
				tombstoned++
			case replacement.ContentSHA256 != sharedHash &&
				replacement.WireState() == store.IntentArchiveWireRetained:
				retained++
			default:
				t.Fatalf("unexpected narrow result replacement: %+v", replacement)
			}
		}
	}
	if tombstoned != 2 || retained != 2 {
		t.Fatalf("narrow result has %d shared tombstones and %d retained unique refs", tombstoned, retained)
	}
	sharedRel, _ := store.IntentArchiveBlobRel(slug, sharedHash)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(sharedRel))); !os.IsNotExist(err) {
		t.Fatalf("narrow repair left shared blob: %v", err)
	}
}

func s7PIB424AssertAllResult(t *testing.T, root, slug string) {
	t.Helper()
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	tombstoned := 0
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.WireState() != store.IntentArchiveWireTombstoned {
				t.Fatalf("--all left non-tombstoned replacement: %+v", replacement)
			}
			tombstoned++
		}
	}
	if tombstoned != 4 {
		t.Fatalf("--all tombstoned %d references, want 4", tombstoned)
	}
	blobs := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive", "blobs")
	entries, err := os.ReadDir(blobs)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("--all left %d blob objects", len(entries))
	}
}

func s7PIB424UniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func s7PIB424EqualStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestS7PIB444AvailableBytesDoNotBypassDanglingRepairRoute(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 PIB 444")
	s7PrepareInitialBundle(t, root, slug)
	bodies := s7WriteControlledIntentBundle(t, root, slug)
	retained := s7IntentArchiveReplacements(t, bodies, store.IntentArchiveWireRetained)
	generation := intentArchiveCLIGeneration(t, slug, retained...)
	blobs := map[string][]byte{}
	var dangling store.IntentArchiveReplacement
	for _, replacement := range retained {
		if replacement.ArtifactID == store.IntentArchiveArtifactAnalysis {
			dangling = replacement
		}
	}
	for _, replacement := range retained {
		if replacement.ContentSHA256 == dangling.ContentSHA256 {
			continue
		}
		blobs[replacement.ContentSHA256] = bodies[replacement.ArtifactID]
	}
	writeIntentArchiveCLIFixture(
		t, root, slug, intentArchiveCLIIndex(t, slug, generation), blobs,
	)
	before := readTree(t, filepath.Join(root, ".tpatch"))
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug,
		"--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	prepareReport := prepareS4Report(t, stdout)
	if code != 3 || prepareReport.Refusal == nil ||
		prepareReport.Refusal.Code != "archive-blob-dangling" {
		t.Fatalf("regenerate dangling refusal = exit:%d report:%+v", code, prepareReport)
	}
	code, stdout, _, _ = runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--yes", "--json", "--quiet",
	)
	archiveReport := decodeIntentArchivePurgeReport(t, stdout)
	if code != 3 || archiveReport.Refusal == nil ||
		archiveReport.Refusal.Code != "archive-blob-dangling" ||
		!bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatalf("ordinary archive mutation bypassed dangling refusal: exit=%d report=%+v", code, archiveReport)
	}

	code, stdout, _, _ = runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--blob", dangling.ContentSHA256, "--yes", "--json", "--quiet",
	)
	purge := decodeIntentArchivePurgeReport(t, stdout)
	if code != 0 || purge.Outcome != "purged" || len(purge.Blobs) != 0 ||
		len(purge.References) != 2 {
		t.Fatalf("confirmed dangling purge = exit:%d report:%+v", code, purge)
	}
	_, tombstoned := readIntentArchiveCLIIndex(t, root, slug)
	if len(tombstoned.Generations) != 1 ||
		tombstoned.Generations[0].Replaced[0].WireState() != store.IntentArchiveWireTombstoned {
		t.Fatalf("dangling repair did not tombstone retained reference: %+v", tombstoned)
	}

	code, stdout, _, _ = runPrepare(
		t, "--path", root, "prepare", slug,
		"--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	rehydratedReport := prepareS4Report(t, stdout)
	if code != 0 || rehydratedReport.Archive == nil ||
		rehydratedReport.Archive.GenerationID != generation.GenerationID {
		t.Fatalf("post-repair rehydrate = exit:%d report:%+v", code, rehydratedReport)
	}
	_, final := readIntentArchiveCLIIndex(t, root, slug)
	if len(final.Generations) != 1 ||
		final.Generations[0].Replaced[0].WireState() != store.IntentArchiveWireRetained {
		t.Fatalf("post-repair regeneration appended or failed to rehydrate: %+v", final)
	}
}

func TestS7PIB431RefusalCatalogRendererParityAndWrongInput(t *testing.T) {
	catalog, evidence, err := s6RefusalEvidence(t)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS6Refusals(catalog, catalog, evidence); err != nil {
		t.Fatal(err)
	}
	wrong := make(map[string]s6RefusalFixture, len(evidence))
	for code, fixture := range evidence {
		wrong[code] = fixture
	}
	fixture := wrong["archive-blob-dangling"]
	fixture.emittedCode = "archive-index-storage-inconsistent"
	wrong["archive-blob-dangling"] = fixture
	if err := validateS6Refusals(catalog, catalog, wrong); err == nil {
		t.Fatal("PIB-431 same production-derived validator accepted a renderer/catalog code mismatch")
	}
}

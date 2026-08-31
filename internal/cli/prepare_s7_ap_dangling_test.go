//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7APDanglingPublicWorkflows(t *testing.T) {
	t.Run("PIB-457", func(t *testing.T) {
		root, slug, hash, _ := s7APDanglingWorkspace(t)
		before := readTree(t, filepath.Join(root, ".tpatch"))
		want := "tpatch feature intent-archive purge " + slug + " --blob " + hash + " --yes"

		code, prepareOut, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		prepareReport := prepareS4Report(t, prepareOut)
		if code != 3 || stderr == "" || prepareReport.Refusal == nil ||
			prepareReport.Refusal.Code != "archive-blob-dangling" {
			t.Fatalf("PIB-457 ordinary regenerate = exit:%d stderr:%q report:%+v",
				code, stderr, prepareReport)
		}

		code, mutationOut, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--orphans", "--yes", "--json", "--quiet",
		)
		mutationReport := decodeIntentArchivePurgeReport(t, mutationOut)
		if code != 3 || stderr == "" || mutationReport.Refusal == nil ||
			mutationReport.Refusal.Code != "archive-blob-dangling" {
			t.Fatalf("PIB-457 ordinary archive mutation = exit:%d stderr:%q report:%+v",
				code, stderr, mutationReport)
		}

		code, listOut, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug,
			"--json", "--quiet",
		)
		listReport := decodeIntentArchiveListReport(t, listOut)
		if code != 3 || stderr == "" || len(listReport.Generations) != 2 {
			t.Fatalf("PIB-457 list = exit:%d stderr:%q report:%+v", code, stderr, listReport)
		}
		code, doctorOut, stderr, _ := runPrepare(
			t, "--path", root, "doctor", "--check", "D9", "--json",
		)
		if code != 0 || stderr != "" || !strings.Contains(doctorOut, "archive-blob-dangling") {
			t.Fatalf("PIB-457 doctor = exit:%d stderr:%q\n%s", code, stderr, doctorOut)
		}
		for name, output := range map[string]string{
			"prepare":  prepareOut,
			"mutation": mutationOut,
			"list":     listOut,
			"doctor":   doctorOut,
		} {
			if err := validateS7APOnlyDanglingCommand(output, want); err != nil {
				t.Fatalf("PIB-457 %s: %v\n%s", name, err, output)
			}
		}
		if after := readTree(t, filepath.Join(root, ".tpatch")); !bytes.Equal(before, after) {
			t.Fatal("PIB-457 public refusal surfaces mutated the workspace")
		}
	})

	t.Run("PIB-458", func(t *testing.T) {
		root, slug, hash, referenceCount := s7APDanglingWorkspace(t)
		oldFactory := intentArchiveNewStorage
		removals := []string{}
		intentArchiveNewStorage = func(
			authority *intentlock.WorkspaceAuthority,
			rooted *os.Root,
		) store.IntentArchiveStorage {
			return &s7APRemoveCountingStorage{
				IntentArchiveStorage: oldFactory(authority, rooted),
				removals:             &removals,
			}
		}
		t.Cleanup(func() { intentArchiveNewStorage = oldFactory })

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", hash, "--yes", "--json", "--quiet",
		)
		report := decodeIntentArchivePurgeReport(t, stdout)
		_, index := readIntentArchiveCLIIndex(t, root, slug)
		if code != 0 || stderr != "" || report.Outcome != "purged" ||
			len(removals) != 0 ||
			len(intentArchiveCLIReferencesForHash(index, hash)) != referenceCount ||
			!intentArchiveCLIHashAllState(index, hash, store.IntentArchiveWireTombstoned) {
			t.Fatalf("PIB-458 public dangling purge = exit:%d stderr:%q report:%+v removals:%v index:%+v",
				code, stderr, report, removals, index)
		}
	})

	t.Run("PIB-459", func(t *testing.T) {
		surfaces := s7APDanglingOwnedSurfaces(t)
		if err := validateS7APDanglingOwnedSurfaces(surfaces); err != nil {
			t.Fatal(err)
		}
		const prepareRel = "internal/cli/prepare.go"
		const prepareNeedle = "func prepareCmd() *cobra.Command {\n"
		wrongHelp := s7APCloneStringMap(surfaces)
		wrongHelp[prepareRel] = strings.Replace(
			wrongHelp[prepareRel],
			"This command is unrelated to ",
			"Alternatively, run tpatch prepare <slug> --regenerate. This command is unrelated to ",
			1,
		)
		if err := validateS7APDanglingOwnedSurfaces(wrongHelp); err == nil {
			t.Fatal("PIB-459 same semantic validator accepted prepareCmd help alternative")
		}
		for name, addition := range map[string]string{
			"local-const":              "\tconst s7APWrongDanglingHelp = \"Alternatively, run tpatch prepare <slug> --regenerate\"\n",
			"if-branch":                "\tif false {\n\t\t_ = \"Alternatively, run tpatch prepare <slug> --regenerate\"\n\t}\n",
			"concatenated-alternative": "\tconst s7APWrongDanglingConcat = \"Alternatively, \" + \"run tpatch prepare <slug> --regenerate\"\n",
			"exact-dangling-prepare":   "\tconst s7APWrongDanglingExact = \"For dangling references, run tpatch prepare <slug> --regenerate\"\n",
		} {
			wrong := s7APCloneStringMap(surfaces)
			wrong[prepareRel] = strings.Replace(
				wrong[prepareRel], prepareNeedle, prepareNeedle+addition, 1,
			)
			if err := validateS7APDanglingOwnedSurfaces(wrong); err == nil {
				t.Fatalf("PIB-459 same semantic validator accepted %s alternative", name)
			}
		}
		byteSupply := s7APCloneStringMap(surfaces)
		byteSupply[prepareRel] = strings.Replace(
			byteSupply[prepareRel],
			prepareNeedle,
			prepareNeedle+"\tconst s7APWrongDanglingBytes = \"For dangling references, supply the original bytes and re-run.\"\n",
			1,
		)
		if err := validateS7APDanglingOwnedSurfaces(byteSupply); err == nil {
			t.Fatal("PIB-459 closed surface inventory accepted a byte-supply alternative")
		}
		if err := s7APValidateDanglingInstruction(
			"positive",
			"For dangling references, run tpatch feature intent-archive purge demo --blob "+
				strings.Repeat("0", 64)+" --yes.",
		); err != nil {
			t.Fatalf("PIB-459 canonical command positive control failed: %v", err)
		}
	})

	t.Run("PIB-460", func(t *testing.T) {
		root, slug, hash, referenceCount := s7APDanglingWorkspace(t)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", hash, "--yes", "--json", "--quiet",
		)
		if code != 0 || stderr != "" || decodeIntentArchivePurgeReport(t, stdout).Outcome != "purged" {
			t.Fatalf("PIB-460 seed purge = exit:%d stderr:%q\n%s", code, stderr, stdout)
		}
		_, tombstoned := readIntentArchiveCLIIndex(t, root, slug)
		beforeIDs := s7GenerationIDs(tombstoned)
		if len(tombstoned.Generations) != 2 || referenceCount < 4 ||
			!intentArchiveCLIHashAllState(tombstoned, hash, store.IntentArchiveWireTombstoned) {
			t.Fatalf("PIB-460 seed purge generations=%d refs=%d did not tombstone every %s reference",
				len(tombstoned.Generations), referenceCount, hash)
		}

		oldBeforeIndex := beforeIndexRewrite
		indexCAS := []string{}
		beforeIndexRewrite = func(rel string) { indexCAS = append(indexCAS, rel) }
		t.Cleanup(func() { beforeIndexRewrite = oldBeforeIndex })
		code, stdout, stderr, _ = runPrepare(
			t, "--path", root, "prepare", slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		_, final := readIntentArchiveCLIIndex(t, root, slug)
		blobPath := filepath.Join(
			root, ".tpatch", "features", slug, "artifacts", "intent-archive",
			"blobs", hash+".blob",
		)
		blob, blobErr := os.ReadFile(blobPath)
		if code != 0 || stderr != "" || report.Archive == nil ||
			len(indexCAS) != 1 ||
			fmt.Sprint(beforeIDs) != fmt.Sprint(s7GenerationIDs(final)) ||
			len(final.Generations) != len(tombstoned.Generations) ||
			len(intentArchiveCLIReferencesForHash(final, hash)) != referenceCount ||
			!intentArchiveCLIHashAllState(final, hash, store.IntentArchiveWireRetained) ||
			blobErr != nil || !bytes.Equal(blob, []byte("shared prior intent\n")) {
			t.Fatalf(
				"PIB-460 public rehydrate = exit:%d stderr:%q report:%+v CAS:%v IDs:%v/%v refs:%d blobErr:%v",
				code, stderr, report, indexCAS, beforeIDs, s7GenerationIDs(final),
				len(intentArchiveCLIReferencesForHash(final, hash)), blobErr,
			)
		}
	})
}

type s7APRemoveCountingStorage struct {
	store.IntentArchiveStorage
	removals *[]string
}

func (storage *s7APRemoveCountingStorage) RemoveBlob(
	rel string,
	expected store.IntentArchiveIdentityToken,
) (store.IntentArchiveMutationResult, error) {
	*storage.removals = append(*storage.removals, rel)
	return storage.IntentArchiveStorage.RemoveBlob(rel, expected)
}

func s7APDanglingWorkspace(t *testing.T) (string, string, string, int) {
	t.Helper()
	root, slug := prepareS4Workspace(t, "S7 AP dangling public workflow")
	s7PrepareInitialBundle(t, root, slug)
	bodies := s7WriteControlledIntentBundle(t, root, slug)
	replacements := s7IntentArchiveReplacements(t, bodies, store.IntentArchiveWireRetained)
	firstGeneration := intentArchiveCLIGeneration(t, slug, replacements...)
	hash := ""
	referenceCount := 0
	blobs := map[string][]byte{}
	for _, replacement := range replacements {
		if bytes.Equal(bodies[replacement.ArtifactID], []byte("shared prior intent\n")) {
			hash = replacement.ContentSHA256
			referenceCount++
			continue
		}
		blobs[replacement.ContentSHA256] = bodies[replacement.ArtifactID]
	}
	if hash == "" || referenceCount != 2 {
		t.Fatalf("dangling fixture shared hash=%q references=%d", hash, referenceCount)
	}
	secondReplacements := append([]store.IntentArchiveReplacement(nil), replacements...)
	variantAdded := false
	for index, replacement := range secondReplacements {
		if replacement.ContentSHA256 == hash {
			referenceCount++
			continue
		}
		if variantAdded {
			continue
		}
		variant := []byte("S7 AP historical generation variant\n")
		secondReplacements[index] = intentArchiveCLIReplacement(
			t, replacement.ArtifactID, variant, store.IntentArchiveWireRetained,
		)
		blobs[secondReplacements[index].ContentSHA256] = variant
		variantAdded = true
	}
	if !variantAdded || referenceCount != 4 {
		t.Fatalf("dangling multi-generation fixture variant=%t references=%d", variantAdded, referenceCount)
	}
	secondGeneration := intentArchiveCLIGeneration(t, slug, secondReplacements...)
	if secondGeneration.GenerationID == firstGeneration.GenerationID {
		t.Fatal("dangling multi-generation fixture has duplicate generation IDs")
	}
	writeIntentArchiveCLIFixture(
		t, root, slug,
		intentArchiveCLIIndex(t, slug, firstGeneration, secondGeneration),
		blobs,
	)
	return root, slug, hash, referenceCount
}

var s7APDanglingCommandRE = regexp.MustCompile(
	`tpatch feature intent-archive purge [a-z0-9]+(?:-[a-z0-9]+)* --blob [0-9a-f]{64} --yes`,
)

func validateS7APOnlyDanglingCommand(output, want string) error {
	matches := s7APDanglingCommandRE.FindAllString(output, -1)
	unique := map[string]bool{}
	for _, match := range matches {
		unique[match] = true
	}
	if len(unique) != 1 || !unique[want] {
		return fmt.Errorf("dangling commands = %v, want sole %q", unique, want)
	}
	lower := strings.ToLower(output)
	for _, forbidden := range []string{
		" --all", " --generation", " --orphans",
		"tpatch prepare ", " --regenerate",
		"tpatch feature intent-archive list ",
		"supply the original bytes", "restore the original bytes",
		"rehydrate the dangling", "dangling reference by rehydrat",
	} {
		if strings.Contains(lower, forbidden) {
			return fmt.Errorf("dangling output offers alternative repair %q", forbidden)
		}
	}
	return nil
}

func intentArchiveCLIReferencesForHash(
	index store.IntentArchiveIndex,
	hash string,
) []store.IntentArchiveReplacement {
	var references []store.IntentArchiveReplacement
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == hash {
				references = append(references, replacement)
			}
		}
	}
	return references
}

func intentArchiveCLIHashAllState(
	index store.IntentArchiveIndex,
	hash string,
	state store.IntentArchiveWireState,
) bool {
	references := intentArchiveCLIReferencesForHash(index, hash)
	if len(references) == 0 {
		return false
	}
	for _, reference := range references {
		if reference.WireState() != state {
			return false
		}
	}
	return true
}

func s7APDanglingOwnedSurfaces(t *testing.T) map[string]string {
	t.Helper()
	root := avpRepoRoot(t)
	surfaces := map[string]string{}
	for _, rel := range []string{
		"internal/cli/prepare.go",
		"internal/cli/prepare_publish.go",
		"internal/cli/feature_intent_archive.go",
		"internal/workflow/doctor_d9.go",
		"docs/feature-layout.md",
	} {
		surfaces[rel] = s6RepoFile(t, rel)
	}
	err := filepath.WalkDir(filepath.Join(root, "assets"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		lower := strings.ToLower(string(raw))
		if !strings.Contains(lower, "intent-archive") &&
			!strings.Contains(lower, "archive-blob-dangling") &&
			!strings.Contains(lower, "dangling reference") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		surfaces[filepath.ToSlash(rel)] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	surfaces["docs/prds/PRD-prepare-intent-bundle.md"] =
		s7APSection(t, prd, "### 9.3 ", "### 9.8 ")
	adr := s6RepoFile(t, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
	surfaces["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"] =
		s7APSection(t, adr, "### D10 ", "### D11 ") + "\n" +
			s7APSection(t, adr, "### D16 ", "### D17 ")
	return surfaces
}

func s7APSection(t *testing.T, document, start, end string) string {
	t.Helper()
	begin := strings.Index(document, start)
	if begin < 0 {
		t.Fatalf("section %q..%q is missing", start, end)
	}
	tail := begin + len(start)
	finish := strings.Index(document[tail:], end)
	if finish < 0 {
		t.Fatalf("section %q..%q is missing", start, end)
	}
	return document[begin : tail+finish]
}

func validateS7APDanglingOwnedSurfaces(surfaces map[string]string) error {
	wantSurfaceKeys := []string{
		"assets/prompts/copilot/tessera-patch-apply.prompt.md",
		"assets/skills/claude/tessera-patch/SKILL.md",
		"assets/skills/copilot/tessera-patch/SKILL.md",
		"assets/skills/cursor/tessera-patch.mdc",
		"assets/skills/windsurf/windsurfrules",
		"assets/workflows/tessera-patch-generic.md",
		"docs/adrs/ADR-035-intent-bundle-publication-and-history.md",
		"docs/feature-layout.md",
		"docs/prds/PRD-prepare-intent-bundle.md",
		"internal/cli/feature_intent_archive.go",
		"internal/cli/prepare.go",
		"internal/cli/prepare_publish.go",
		"internal/workflow/doctor_d9.go",
	}
	gotSurfaceKeys := make([]string, 0, len(surfaces))
	for name := range surfaces {
		gotSurfaceKeys = append(gotSurfaceKeys, name)
	}
	sort.Strings(gotSurfaceKeys)
	if fmt.Sprint(gotSurfaceKeys) != fmt.Sprint(wantSurfaceKeys) {
		return fmt.Errorf("dangling owned surfaces = %v, want %v", gotSurfaceKeys, wantSurfaceKeys)
	}
	inventory, err := s7APDanglingProductionInventory(surfaces)
	if err != nil {
		return err
	}
	wantInventory := []string{
		"internal/cli/feature_intent_archive.go:const:intentArchiveHistoryDisclosure",
		"internal/cli/feature_intent_archive.go:func:buildIntentArchiveDivergence",
		"internal/cli/feature_intent_archive.go:func:buildIntentArchiveListReport",
		"internal/cli/feature_intent_archive.go:func:buildIntentArchiveRemainingRepairsReport",
		"internal/cli/feature_intent_archive.go:func:buildIntentArchiveUnindexedCorruptObjects",
		"internal/cli/feature_intent_archive.go:func:intentArchiveBlobRetry",
		"internal/cli/feature_intent_archive.go:func:intentArchiveClassRepairText",
		"internal/cli/feature_intent_archive.go:func:intentArchiveCorruptClassPredictsDangling",
		"internal/cli/feature_intent_archive.go:func:intentArchiveCorruptClassPrerequisite",
		"internal/cli/feature_intent_archive.go:func:intentArchiveCorruptRemovalText",
		"internal/cli/feature_intent_archive.go:func:intentArchiveDanglingClassRetry",
		"internal/cli/feature_intent_archive.go:func:intentArchiveListInspectionRefusal",
		"internal/cli/feature_intent_archive.go:func:intentArchiveListRepair",
		"internal/cli/feature_intent_archive.go:func:intentArchivePendingJournalRefusal",
		"internal/cli/feature_intent_archive.go:func:intentArchiveRefusalFromError",
		"internal/cli/feature_intent_archive.go:func:intentArchiveRemainingRepairsText",
		"internal/cli/feature_intent_archive.go:func:intentArchiveRepairPriority",
		"internal/cli/feature_intent_archive.go:func:intentArchiveStorageToken",
		"internal/cli/feature_intent_archive.go:func:intentArchiveWorkspace",
		"internal/cli/feature_intent_archive.go:func:intentArchiveWorkspaceRemediation",
		"internal/cli/feature_intent_archive.go:func:runFeatureIntentArchiveList",
		"internal/cli/feature_intent_archive.go:func:runFeatureIntentArchivePurgePreview",
		"internal/cli/prepare.go:func:prepareCmd",
		"internal/cli/prepare_publish.go:func:appendPrepareOrphanAdvisory",
		"internal/cli/prepare_publish.go:func:applyPrepareGenerationReport",
		"internal/cli/prepare_publish.go:func:prepareArchiveCompleteClassRoutes",
		"internal/cli/prepare_publish.go:func:prepareArchiveRepairText",
		"internal/cli/prepare_publish.go:func:preparePendingPurgeCommand",
		"internal/cli/prepare_publish.go:func:prepareRefusalText",
		"internal/cli/prepare_publish.go:func:prepareStateRemediation",
		"internal/cli/prepare_publish.go:func:prepareStoreArchiveFailure",
		"internal/cli/prepare_publish.go:func:prepareValidateArchiveSnapshot",
		"internal/cli/prepare_publish.go:func:renderPrepareFeaturesIndex",
		"internal/cli/prepare_publish.go:func:runPrepareAbandon",
		"internal/cli/prepare_publish.go:func:runPreparePublish",
		"internal/cli/prepare_publish.go:func:writePreparePublishHuman",
		"internal/workflow/doctor_d9.go:func:doctorD9ArchiveClassFinding",
		"internal/workflow/doctor_d9.go:func:doctorD9ArchiveClassRemediation",
		"internal/workflow/doctor_d9.go:func:doctorD9BlobPurgeCommand",
		"internal/workflow/doctor_d9.go:func:doctorD9ReportArchiveClasses",
		"internal/workflow/doctor_d9.go:func:runDoctorD9ArchiveEvidence",
		"internal/workflow/doctor_d9.go:func:runDoctorD9FeatureArchive",
		"internal/workflow/doctor_d9.go:func:runDoctorD9PrepareEvidence",
		"internal/workflow/doctor_d9.go:func:runDoctorD9PrepareLane",
		"internal/workflow/doctor_d9.go:var:doctorD9PersistentEvidenceCodes",
	}
	if fmt.Sprint(inventory) != fmt.Sprint(wantInventory) {
		return fmt.Errorf("dangling production inventory = %#v", inventory)
	}
	declarationHashes, err := s7APDanglingDeclarationHashes(surfaces)
	if err != nil {
		return err
	}
	surfaceHashes := map[string]string{}
	for name, body := range surfaces {
		if strings.HasSuffix(name, ".go") {
			continue
		}
		surfaceHashes[name] = s7APNormalizedContentHash(body)
	}
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`(?i)supply\s+(?:the\s+)?original bytes`),
		regexp.MustCompile(`(?i)restore\s+(?:the\s+)?original bytes`),
		regexp.MustCompile(`(?i)rehydration\s+(?:repairs|restores)\s+(?:a\s+)?dangling`),
		regexp.MustCompile(`(?i)rehydrate\s+(?:a|the)\s+dangling`),
		regexp.MustCompile(`(?i)use\s+rehydration\s+(?:as|to)\s+(?:a\s+)?dangling`),
		regexp.MustCompile(`(?i)(?:offers|provides|use)\s+(?:a\s+)?(?:second|alternative)\s+(?:dangling\s+)?repair`),
		regexp.MustCompile(`(?is)alternatively.{0,160}tpatch\s+prepare`),
		regexp.MustCompile(`(?is)(?:repair|route).{0,160}tpatch\s+prepare.{0,80}--regenerate`),
	}
	for name, body := range surfaces {
		if strings.HasSuffix(name, ".go") {
			file, err := parser.ParseFile(token.NewFileSet(), name, body, parser.ParseComments)
			if err != nil {
				return err
			}
			packageValues := map[string]string{}
			for _, declaration := range file.Decls {
				generic, ok := declaration.(*ast.GenDecl)
				if !ok {
					continue
				}
				packageValues = s7APStaticStringBindings(generic, packageValues)
			}
			for _, declaration := range file.Decls {
				if !s7APDanglingInventoryNode(declaration) {
					continue
				}
				values := s7APStaticStringBindings(declaration, packageValues)
				staticStrings := s7APMaximalInstructionStrings(append(
					s7APStaticStrings(declaration, values),
					s7APStaticStringTemplates(declaration, values)...,
				))
				for _, staticText := range staticStrings {
					for _, pattern := range forbidden {
						if pattern.MatchString(staticText) {
							return fmt.Errorf("%s inventoried declaration offers alternative dangling repair %q",
								name, pattern)
						}
					}
					if err := s7APValidateDanglingInstruction(
						name+":"+s7APDanglingDeclarationName(declaration),
						staticText,
					); err != nil {
						return err
					}
				}
			}
			continue
		}
		for _, paragraph := range regexp.MustCompile(`\n\s*\n`).Split(body, -1) {
			lower := strings.ToLower(paragraph)
			if !strings.Contains(lower, "dangling") {
				continue
			}
			for _, pattern := range forbidden {
				if pattern.MatchString(paragraph) {
					return fmt.Errorf("%s dangling prose offers alternative repair %q", name, pattern)
				}
			}
			for _, line := range strings.Split(paragraph, "\n") {
				if err := s7APValidateDanglingInstruction(name, line); err != nil {
					return err
				}
			}
		}
	}
	if !reflect.DeepEqual(declarationHashes, s7APAcceptedDanglingDeclarations) {
		return fmt.Errorf("dangling declaration content drift:\ngot  %#v\nwant %#v",
			declarationHashes, s7APAcceptedDanglingDeclarations)
	}
	if !reflect.DeepEqual(surfaceHashes, s7APAcceptedDanglingSurfaces) {
		return fmt.Errorf("dangling prose surface content drift:\ngot  %#v\nwant %#v",
			surfaceHashes, s7APAcceptedDanglingSurfaces)
	}
	return nil
}

// AQ's recovery-evidence path, AR's history disclosure, AW's complete
// repair-class routes and rev-20's selector-specific refusal text are the
// reviewed declaration drifts after AP.
var s7APAcceptedDanglingDeclarations = map[string]string{
	"internal/cli/feature_intent_archive.go:const:intentArchiveHistoryDisclosure":           "8360622adc2d49bf4dfa3467140f147d0c23d27093f10672161049d58fee33ba",
	"internal/cli/feature_intent_archive.go:func:buildIntentArchiveDivergence":              "90e96b9720f14541241ae0892030b8a82a3641aeed8f0230bb0ad582e26ee660",
	"internal/cli/feature_intent_archive.go:func:buildIntentArchiveListReport":              "bf5435ef4939a0eabf55195aa8d599deeba2af199a95e160065d8548534f2246",
	"internal/cli/feature_intent_archive.go:func:buildIntentArchiveRemainingRepairsReport":  "97c72618d563438a744b65054f1631c16091d4f4f9ee3ba946e62e2dc524039f",
	"internal/cli/feature_intent_archive.go:func:buildIntentArchiveUnindexedCorruptObjects": "52a5d21da4d4ec6f11d3d65c712cc1c527079fec0f261d162f0643e9c840e290",
	"internal/cli/feature_intent_archive.go:func:intentArchiveBlobRetry":                    "485faea30c5d1085076fc45dcebf3b7da65a71929de3a542e1661fc512ec2957",
	"internal/cli/feature_intent_archive.go:func:intentArchiveClassRepairText":              "540d9516c9dcd0d23abbf3fe9c5c6cdda6b7f5b7d8a09bf264239a4e1db6ca93",
	"internal/cli/feature_intent_archive.go:func:intentArchiveCorruptClassPredictsDangling": "55358a42da33008b0b0e8d859b6f11b22e9ebd5d139f86384533c9ac42052bb3",
	"internal/cli/feature_intent_archive.go:func:intentArchiveCorruptClassPrerequisite":     "2aefd004c3fdc4e735aef39bee20b643632e519ade699b3ec4546991fc1b0caa",
	"internal/cli/feature_intent_archive.go:func:intentArchiveCorruptRemovalText":           "011a97f93805fd05c0a96a87ed959b1705adf7a86ac830cdb12d582da6497b9a",
	"internal/cli/feature_intent_archive.go:func:intentArchiveDanglingClassRetry":           "da6b449cd8dad07a071542a4d239f80a7383a374f916ccfa357f9112782c8872",
	"internal/cli/feature_intent_archive.go:func:intentArchiveListInspectionRefusal":        "96b38ce0947f04b32482d38ea28eb92da480a5afbb336192c9c137ab660a7254",
	"internal/cli/feature_intent_archive.go:func:intentArchiveListRepair":                   "e3e58c17c62226f6d218e9980877c287e0ecfc6e157aa083d6c17489736d575d",
	"internal/cli/feature_intent_archive.go:func:intentArchivePendingJournalRefusal":        "55102086021476d5ba0c9a45ec648e4c03c4ffd8247ff8ab4855bc6d810313f7",
	"internal/cli/feature_intent_archive.go:func:intentArchiveRefusalFromError":             "a8922f510f2d610a33734a96344cd0440bb71cb433f4b46647ccb26d3ad0de91",
	"internal/cli/feature_intent_archive.go:func:intentArchiveRemainingRepairsText":         "977256aa149233e70fc46199a9a994711f8118fbb1c5131098c0facefcb975aa",
	"internal/cli/feature_intent_archive.go:func:intentArchiveRepairPriority":               "3319cffe754cfaefd7732c81c1f3804157c56c6a8ae3a0f592fa37b1d2766430",
	"internal/cli/feature_intent_archive.go:func:intentArchiveStorageToken":                 "252378708c66ae58d2fa589bf6164e34ef594d82d523b2019e554368675902af",
	"internal/cli/feature_intent_archive.go:func:intentArchiveWorkspace":                    "7a25bd9745eac07976db0ab29877dc00fb4194f1660c865a1890553a90827cbf",
	"internal/cli/feature_intent_archive.go:func:intentArchiveWorkspaceRemediation":         "7686755639c1399b273d2bb6cbf105e4a73442cd1444a229a7e8da7607ee4ac8",
	"internal/cli/feature_intent_archive.go:func:runFeatureIntentArchiveList":               "8011da4c1e6fb26766a42ae5347e57b6aae9d481a88dfdc8d54d0dd914a63178",
	"internal/cli/feature_intent_archive.go:func:runFeatureIntentArchivePurgePreview":       "6463993d19b709474fc3f8c668ed25a0b44f76a9f47b0e448b9d6284ecc528fa",
	"internal/cli/prepare.go:func:prepareCmd":                                               "f68758114e1adea915fe85640033015498c8636d6e9424760e009fcec550e504",
	"internal/cli/prepare_publish.go:func:appendPrepareOrphanAdvisory":                      "40b3d043fbbae54426dc55b18303cd4dfd8d0108d2a97ae93abeaea58ef556ce",
	"internal/cli/prepare_publish.go:func:applyPrepareGenerationReport":                     "fda949b7e12fbce78aae64213ca2c93b7317246d164527b7ecc61aa9c49c6ebd",
	"internal/cli/prepare_publish.go:func:prepareArchiveCompleteClassRoutes":                "12e758ab0675169738e89de1dd060af9d6a473c5329fc899155dc60ec33fd2d4",
	"internal/cli/prepare_publish.go:func:prepareArchiveRepairText":                         "27cd1b95b252d76e2e392721c21f79ea603bbe3faa0ac4f7e8a05297757a2f4a",
	"internal/cli/prepare_publish.go:func:preparePendingPurgeCommand":                       "82cb363ba31ee78eaa084d23d0aab576e446c816ed63dec1b71ec8005f7e417e",
	"internal/cli/prepare_publish.go:func:prepareRefusalText":                               "cea790bc9d41c5bd416fcbdc3e6bc3866a28bb12a1c3be4921658fc27834afb3",
	"internal/cli/prepare_publish.go:func:prepareStateRemediation":                          "deff3dcce09c3a0146bcbdaf5969058c1620fa2c3d82e2ef5de55eb987ae7773",
	"internal/cli/prepare_publish.go:func:prepareStoreArchiveFailure":                       "5698680a4eec61a1558f3302128aacd7e9d046a450e5ff6f75c327ed24f7958f",
	"internal/cli/prepare_publish.go:func:prepareValidateArchiveSnapshot":                   "7b24188ff4fc1747f558079b76fd9ff92259236bf91be5d841ae3ca2d4c7098d",
	"internal/cli/prepare_publish.go:func:renderPrepareFeaturesIndex":                       "2d5d1b0ee19df24bd6cc6cb1081f7718b4e71fdd640ee2b17a579d39143d3a00",
	"internal/cli/prepare_publish.go:func:runPrepareAbandon":                                "95f5bf69f4f66b172406f001541657a2dd94f546a7fc144ae301a333acdaaaf6",
	"internal/cli/prepare_publish.go:func:runPreparePublish":                                "15f1e8c5f0b80b0a2e0dea50b13e55d3df4339c4181586ffd6e2c40a7192e722",
	"internal/cli/prepare_publish.go:func:writePreparePublishHuman":                         "1f92259db76bdb11482e0853b12d534bc6c9b1be6e4f72b75ebedf6ac32dd038",
	"internal/workflow/doctor_d9.go:func:doctorD9ArchiveClassFinding":                       "38a3b8c708fdfe6b9ba4fcd41a354a40f808eea93355fb58cd6e7c320912caf2",
	"internal/workflow/doctor_d9.go:func:doctorD9ArchiveClassRemediation":                   "746e01d9e0b605a2908269565c6b1745e3d7b802cd294acb4d304170c1115634",
	"internal/workflow/doctor_d9.go:func:doctorD9BlobPurgeCommand":                          "d8a7aaa66768448a4ebf0d8ad57a0f0247c2c406d4cce830cd90b187a1724c4a",
	"internal/workflow/doctor_d9.go:func:doctorD9ReportArchiveClasses":                      "9e47c21183e3aa29489ef3c8addf9b5188377cd1de38282d8d6e13e81e33eb06",
	"internal/workflow/doctor_d9.go:func:runDoctorD9ArchiveEvidence":                        "163efe4bc0a7daa30375d31d477ac7ddd4a9f2def002b810170bdf80c446292e",
	"internal/workflow/doctor_d9.go:func:runDoctorD9FeatureArchive":                         "672995d551960de521db52cb2502570bc3f532f88362f8ed4ba5483bde293e62",
	"internal/workflow/doctor_d9.go:func:runDoctorD9PrepareEvidence":                        "35730c0fbd076a015148c607250583387f7485b02e2b6224cb34fcb87d6be606",
	"internal/workflow/doctor_d9.go:func:runDoctorD9PrepareLane":                            "21e58aff1cedc8f994d9cbb549938c8c25d53aae5394635d777f31c12b6333c8",
	"internal/workflow/doctor_d9.go:var:doctorD9PersistentEvidenceCodes":                    "0aae9410ed53791db3292e64480f6cef29867fee1f0482b76555411f04869723",
}

var s7APAcceptedDanglingSurfaces = map[string]string{
	"assets/prompts/copilot/tessera-patch-apply.prompt.md": "a0d00e4490b16e1bd62751651f36ae1e1d47145f5b35a08a4549dff0587de296",
	"assets/skills/claude/tessera-patch/SKILL.md":          "1348460eb0243d318577249ae380c2db8da9b94283e3093a8f3d7e06bc36eb4a",
	"assets/skills/copilot/tessera-patch/SKILL.md":         "ad0ef9ddd93ca3b6b17623eb36f0d3297434bdef3e1e635c4873067d3e7d13c5",
	"assets/skills/cursor/tessera-patch.mdc":               "88cb89a4aec4f3400eb654ffb545b5f377446ae69b3dc6d1bb9f66a1a05c8eea",
	"assets/skills/windsurf/windsurfrules":                 "60df5c3a9758c4d58e899621d34fdcb70eec97be4fbcd3424f1b267543f8eaae",
	"assets/workflows/tessera-patch-generic.md":            "7325c4507b67058fbe9092b4f6c49bc5d5a911712a645912f8b561c09788d735",
	// rev-19 splits D10's joint `list`/`doctor`/mutation refusal sentence into
	// the exit-3 refusal surfaces and `doctor`'s warning-only exit 0. rev-20
	// then adds D16's selector-classification vocabulary paragraph and the
	// companion PRD's `archive-selector-invalid` catalog row, so both document
	// surfaces move again; neither touches a dangling claim.
	"docs/adrs/ADR-035-intent-bundle-publication-and-history.md": "edbf901f42e7b7df24ecac8c2b6befae0a5a16136322ccee19634ce4f1af17c8",
	"docs/feature-layout.md":                                     "7065454f25457249e63d409494c7f9da25078e2d2e4c4d4c6570cdebca3b707e",
	"docs/prds/PRD-prepare-intent-bundle.md":                     "3dd28b88f34f5f5f2e523a8738681d473a6255c8bf03d9b80d3a165ab49e3cc9",
}

func s7APCloneStringMap(input map[string]string) map[string]string {
	cloned := make(map[string]string, len(input))
	for name, value := range input {
		cloned[name] = value
	}
	return cloned
}

func s7APDanglingDeclarationHashes(surfaces map[string]string) (map[string]string, error) {
	hashes := map[string]string{}
	for _, name := range []string{
		"internal/cli/feature_intent_archive.go",
		"internal/cli/prepare.go",
		"internal/cli/prepare_publish.go",
		"internal/workflow/doctor_d9.go",
	} {
		source := surfaces[name]
		fileset := token.NewFileSet()
		file, err := parser.ParseFile(fileset, name, source, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if !s7APDanglingInventoryNode(value) {
					continue
				}
				var rendered bytes.Buffer
				if err := format.Node(&rendered, fileset, value); err != nil {
					return nil, err
				}
				hashes[name+":func:"+value.Name.Name] =
					s7APNormalizedContentHash(rendered.String())
			case *ast.GenDecl:
				for _, spec := range value.Specs {
					entry, _ := spec.(*ast.ValueSpec)
					if entry == nil || !s7APDanglingInventoryNode(entry) {
						continue
					}
					var rendered bytes.Buffer
					if err := format.Node(&rendered, fileset, entry); err != nil {
						return nil, err
					}
					for _, identifier := range entry.Names {
						hashes[name+":"+strings.ToLower(value.Tok.String())+":"+identifier.Name] =
							s7APNormalizedContentHash(rendered.String())
					}
				}
			}
		}
	}
	return hashes, nil
}

func s7APDanglingDeclarationName(declaration ast.Decl) string {
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		return value.Name.Name
	case *ast.GenDecl:
		var names []string
		for _, spec := range value.Specs {
			entry, _ := spec.(*ast.ValueSpec)
			if entry == nil {
				continue
			}
			for _, name := range entry.Names {
				names = append(names, name.Name)
			}
		}
		return strings.Join(names, ",")
	default:
		return fmt.Sprintf("%T", declaration)
	}
}

func s7APNormalizedContentHash(value string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(strings.Fields(value), " "))))
}

func s7APStaticStringBindings(node ast.Node, inherited map[string]string) map[string]string {
	values := make(map[string]string, len(inherited))
	for name, value := range inherited {
		values[name] = value
	}
	for pass := 0; pass < 8; pass++ {
		changed := false
		ast.Inspect(node, func(child ast.Node) bool {
			switch value := child.(type) {
			case *ast.ValueSpec:
				for index, identifier := range value.Names {
					if index >= len(value.Values) {
						continue
					}
					if resolved, ok := s7APStaticString(value.Values[index], values); ok &&
						values[identifier.Name] != resolved {
						values[identifier.Name] = resolved
						changed = true
					}
				}
			case *ast.AssignStmt:
				for index, left := range value.Lhs {
					if index >= len(value.Rhs) {
						continue
					}
					identifier, _ := left.(*ast.Ident)
					if identifier == nil {
						continue
					}
					if resolved, ok := s7APStaticString(value.Rhs[index], values); ok &&
						values[identifier.Name] != resolved {
						values[identifier.Name] = resolved
						changed = true
					}
				}
			}
			return true
		})
		if !changed {
			break
		}
	}
	return values
}

func s7APStaticStrings(node ast.Node, values map[string]string) []string {
	var result []string
	ast.Inspect(node, func(child ast.Node) bool {
		expression, ok := child.(ast.Expr)
		if !ok {
			return true
		}
		if value, resolved := s7APStaticString(expression, values); resolved {
			result = append(result, value)
		}
		return true
	})
	return result
}

func s7APStaticStringTemplates(node ast.Node, values map[string]string) []string {
	var result []string
	builders := map[string][]string{}
	ast.Inspect(node, func(child ast.Node) bool {
		expression, ok := child.(ast.Expr)
		if ok {
			if value, resolved := s7APStaticStringTemplate(expression, values); resolved {
				result = append(result, value)
			}
		}
		call, ok := child.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		selector, _ := call.Fun.(*ast.SelectorExpr)
		if selector == nil || selector.Sel.Name != "WriteString" {
			return true
		}
		receiver, _ := selector.X.(*ast.Ident)
		if receiver == nil {
			return true
		}
		if value, resolved := s7APStaticStringTemplate(call.Args[0], values); resolved {
			builders[receiver.Name] = append(builders[receiver.Name], value)
		}
		return true
	})
	for _, fragments := range builders {
		result = append(result, strings.Join(fragments, ""))
	}
	return s7APMaximalInstructionStrings(result)
}

func s7APMaximalInstructionStrings(result []string) []string {
	filtered := make([]string, 0, len(result))
	for index, candidate := range result {
		if !strings.Contains(strings.ToLower(candidate), "tpatch") {
			filtered = append(filtered, candidate)
			continue
		}
		prefix := false
		for otherIndex, other := range result {
			if index != otherIndex && len(other) > len(candidate) &&
				strings.Contains(other, candidate) {
				prefix = true
				break
			}
		}
		if !prefix {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func s7APStaticStringTemplate(expression ast.Expr, values map[string]string) (string, bool) {
	if exact, ok := s7APStaticString(expression, values); ok {
		return exact, true
	}
	switch value := expression.(type) {
	case *ast.Ident:
		return "<value>", true
	case *ast.SelectorExpr:
		return "<value>", true
	case *ast.ParenExpr:
		return s7APStaticStringTemplate(value.X, values)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := s7APStaticStringTemplate(value.X, values)
		right, rightOK := s7APStaticStringTemplate(value.Y, values)
		return left + right, leftOK && rightOK
	case *ast.CallExpr:
		selector, _ := value.Fun.(*ast.SelectorExpr)
		var packageName *ast.Ident
		if selector != nil {
			packageName, _ = selector.X.(*ast.Ident)
		}
		if packageName != nil &&
			packageName.Name == "fmt" && selector.Sel.Name == "Sprintf" &&
			len(value.Args) != 0 {
			formatValue, ok := s7APStaticStringTemplate(value.Args[0], values)
			if !ok {
				return "", false
			}
			formatValue = regexp.MustCompile(`%[-+#0-9.]*[a-zA-Z]`).ReplaceAllString(
				formatValue, "<value>",
			)
			return formatValue, true
		}
		return "<value>", true
	default:
		return "", false
	}
}

func s7APStaticString(expression ast.Expr, values map[string]string) (string, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	case *ast.Ident:
		text, ok := values[value.Name]
		return text, ok
	case *ast.ParenExpr:
		return s7APStaticString(value.X, values)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := s7APStaticString(value.X, values)
		right, rightOK := s7APStaticString(value.Y, values)
		return left + right, leftOK && rightOK
	default:
		return "", false
	}
}

func s7APValidateDanglingInstruction(name, text string) error {
	normalized := strings.Join(strings.Fields(strings.ReplaceAll(text, "`", "")), " ")
	lower := strings.ToLower(normalized)
	starts := regexp.MustCompile(`(?i)\btpatch(?:\s+|$)`).FindAllStringIndex(normalized, -1)
	for _, span := range starts {
		contextStart := span[0] - 160
		if contextStart < 0 {
			contextStart = 0
		}
		prefix := lower[contextStart:span[0]]
		command := strings.Fields(normalized[span[0]:])
		if len(command) < 2 {
			continue
		}
		for index := range command {
			command[index] = strings.Trim(command[index], "`'\".,;:()")
		}
		hasBlob := false
		for _, argument := range command {
			hasBlob = hasBlob || argument == "--blob"
		}
		actionPrefix := regexp.MustCompile(`(?i)(?:run|retry|execute|command|use)\s*$`).MatchString(prefix)
		instructional := span[0] == 0 || actionPrefix ||
			(command[1] == "feature" && hasBlob)
		danglingContext := instructional && (strings.Contains(strings.ToLower(name), "dangling") ||
			strings.Contains(prefix, "dangling") ||
			strings.Contains(prefix, "not recoverable") ||
			strings.Contains(prefix, "no recoverable blob") ||
			(command[1] == "feature" && hasBlob))
		if !danglingContext {
			continue
		}
		yes := -1
		for index := 1; index < len(command) && index < 16; index++ {
			if command[index] == "--yes" {
				yes = index
				break
			}
		}
		if yes != 7 ||
			command[0] != "tpatch" ||
			command[1] != "feature" ||
			command[2] != "intent-archive" ||
			command[3] != "purge" ||
			command[4] == "" || strings.HasPrefix(command[4], "-") ||
			command[5] != "--blob" ||
			command[6] == "" || strings.HasPrefix(command[6], "-") {
			limit := len(command)
			if limit > 10 {
				limit = 10
			}
			return fmt.Errorf("%s has noncanonical dangling tpatch argv %v",
				name, command[:limit])
		}
		if len(command) > 8 && strings.HasPrefix(command[8], "--") {
			return fmt.Errorf("%s appends a flag to canonical dangling argv: %v",
				name, command[:9])
		}
	}
	return nil
}

func s7APDanglingProductionInventory(surfaces map[string]string) ([]string, error) {
	var inventory []string
	for _, name := range []string{
		"internal/cli/feature_intent_archive.go",
		"internal/cli/prepare.go",
		"internal/cli/prepare_publish.go",
		"internal/workflow/doctor_d9.go",
	} {
		body, ok := surfaces[name]
		if !ok {
			return nil, errors.New("missing production dangling surface " + name)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, body, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if s7APDanglingInventoryNode(value) {
					inventory = append(inventory, name+":func:"+value.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range value.Specs {
					entry, _ := spec.(*ast.ValueSpec)
					if entry == nil || !s7APDanglingInventoryNode(entry) {
						continue
					}
					for _, identifier := range entry.Names {
						inventory = append(inventory,
							name+":"+strings.ToLower(value.Tok.String())+":"+identifier.Name)
					}
				}
			}
		}
	}
	sort.Strings(inventory)
	return inventory, nil
}

func s7APDanglingInventoryNode(node ast.Node) bool {
	relevant := false
	ast.Inspect(node, func(child ast.Node) bool {
		switch value := child.(type) {
		case *ast.Ident:
			lower := strings.ToLower(value.Name)
			relevant = relevant || strings.Contains(lower, "dangling")
		case *ast.BasicLit:
			if value.Kind != token.STRING {
				return !relevant
			}
			text, _ := strconv.Unquote(value.Value)
			lower := strings.ToLower(text)
			relevant = relevant ||
				strings.Contains(lower, "dangling") ||
				strings.Contains(lower, "tpatch ")
		}
		return !relevant
	})
	return relevant
}

//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// rev20SelectorForbiddenWording is what an `archive-selector-invalid` refusal
// must never say. The shipped defect said all of it: it reported a selection
// that matched nothing as a corrupt index and told the operator to preserve
// archive bytes a feature without an archive does not have.
var rev20SelectorForbiddenWording = []string{
	"corrupt",
	"preserve",
	"restore",
	"rm -rf",
	"rm ",
	"index.json",
}

// rev20MalformedSelectorValues are the values the shipped CLI rejects as a
// usage error before any archive is read. It reuses the exact set the
// established contract test
// `TestFeatureIntentArchiveSelectorValidationAndSequentialRepair/malformed-selector`
// pins, so the two cannot state different outcomes, and adds the two
// near-miss shapes rev-20 makes it worth naming: correct length but uppercase,
// and correct alphabet but one character short.
var rev20MalformedSelectorValues = []struct {
	name  string
	flag  string
	value string
}{
	{name: "blob-not-a-hash", flag: "--blob", value: "not-a-hash"},
	{name: "blob-control-bytes", flag: "--blob", value: "bad\nselector"},
	{name: "generation-absolute-path", flag: "--generation", value: "/unsafe/selector"},
	{name: "generation-nul-byte", flag: "--generation", value: "\x00unsafe"},
	{name: "blob-uppercase-hex", flag: "--blob", value: strings.Repeat("A", 64)},
	{name: "generation-short-hex", flag: "--generation", value: strings.Repeat("a", 63)},
}

// TestRev20SelectorRefusalSurfacesItsOwnCode is the CLI half of the rev-20
// amendment (PIB-431, PIB-465). It covers the code's whole public extent: a
// well-formed lowercase 64-hex `--blob` hash the index carries no reference to,
// and a well-formed `--generation` id it records no generation for. Both
// selector families, both command forms and both archive populations must
// report the store's own classification with a truthful, selector-specific
// remediation that routes to the listing.
//
// Malformed values are deliberately *not* here: the shipped command rejects
// them at exit 1 before any archive read, and
// TestRev20MalformedSelectorIsAUsageErrorNotARefusal pins that instead.
func TestRev20SelectorRefusalSurfacesItsOwnCode(t *testing.T) {
	const unknownHash = "b1b2b3b4b5b6b7b8b9b0c1c2c3c4c5c6c7c8c9c0d1d2d3d4d5d6d7d8d9d0e1e2"
	const unknownGeneration = "a1a2a3a4a5a6a7a8a9a0f1f2f3f4f5f6f7f8f9f0e1e2e3e4e5e6e7e8e9e0d1d2"

	for _, archive := range []struct {
		name     string
		populate bool
		residue  bool
	}{
		{name: "absent-archive", populate: false},
		{name: "populated-archive", populate: true},
		{name: "archive-with-unreferenced-residue", residue: true},
	} {
		for _, selector := range []struct {
			name  string
			flag  string
			value string
		}{
			{name: "blob", flag: "--blob", value: unknownHash},
			{name: "generation", flag: "--generation", value: unknownGeneration},
		} {
			for _, confirmed := range []bool{false, true} {
				form := "preview"
				if confirmed {
					form = "confirmed"
				}
				t.Run(archive.name+"/"+selector.name+"/"+form, func(t *testing.T) {
					if confirmed && !intentlock.AuthoritySupported {
						t.Skip("real workspace authority is unsupported on this target")
					}
					root, slug := intentArchiveCLIWorkspace(t)
					if archive.residue {
						fixture := s7AVWriteRepairArchive(
							t,
							"rev20-selector-residue",
							s7AVRepairSpec{residues: 1, ready: true},
						)
						root, slug = fixture.root, fixture.slug
					} else if archive.populate {
						data := []byte("rev20 selector fixture\n")
						replacement := intentArchiveCLIReplacement(
							t, store.IntentArchiveArtifactAnalysis, data,
							store.IntentArchiveWireRetained,
						)
						writeIntentArchiveCLIFixture(t, root, slug,
							intentArchiveCLIIndex(t, slug,
								intentArchiveCLIGeneration(t, slug, replacement)),
							map[string][]byte{replacement.ContentSHA256: data},
						)
					}
					before := readTree(t, filepath.Join(root, ".tpatch"))

					trace := s7ARInstallGitSpawnSpy(t, root)

					args := []string{
						"--path", root, "feature", "intent-archive", "purge", slug,
						selector.flag, selector.value, "--json", "--quiet",
					}
					if confirmed {
						args = append(args, "--yes")
					}
					code, stdout, stderr, _ := runPrepare(t, args...)
					if code != 3 {
						t.Fatalf("selector refusal exit = %d stdout=%q stderr=%q", code, stdout, stderr)
					}
					report := decodeIntentArchivePurgeReport(t, stdout)
					if report.Outcome != "refused" || report.Refusal == nil {
						t.Fatalf("selector refusal report = %#v", report)
					}
					refusal := report.Refusal
					if refusal.Code != string(store.IntentArchiveCodeSelectorInvalid) {
						t.Fatalf("refusal code = %q, want typed selector-invalid code", refusal.Code)
					}
					if refusal.Code != "archive-selector-invalid" {
						t.Fatalf("refusal code = %q, want exact public spelling", refusal.Code)
					}
					wantValue := "Hash: " + selector.value + "."
					wantNoun := "content hash"
					if selector.name == "generation" {
						wantValue = "Generation: " + selector.value + "."
						wantNoun = "archive generation"
					}
					if !strings.Contains(refusal.Message, wantValue) ||
						!strings.Contains(refusal.Message, selector.flag) ||
						!strings.Contains(refusal.Message, wantNoun) {
						t.Fatalf("selector message is not selector-specific: %q", refusal.Message)
					}
					wantList := "tpatch feature intent-archive list " + slug
					if !strings.Contains(refusal.Remediation, wantList) ||
						!strings.Contains(refusal.Remediation, "from the workspace root") ||
						!strings.Contains(refusal.Remediation, "if that listing names no") {
						t.Fatalf("selector remediation lacks the listing route: %q", refusal.Remediation)
					}
					if strings.Contains(refusal.Remediation, "<") ||
						strings.Contains(refusal.Remediation, root) {
						t.Fatalf("selector remediation is not self-contained: %q", refusal.Remediation)
					}
					// The contract chooses remediation-only: the next step is
					// an inspection to read, not a command to paste.
					if refusal.Retry != "" || refusal.RetryCWD != "" {
						t.Fatalf("selector refusal carries a structured retry: %#v", refusal)
					}
					joined := strings.ToLower(refusal.Message + " " + refusal.Remediation)
					for _, forbidden := range rev20SelectorForbiddenWording {
						if strings.Contains(joined, forbidden) {
							t.Fatalf("selector refusal says %q: %q", forbidden, joined)
						}
					}
					if report.RemainingRepairs != nil || report.Divergence != nil ||
						report.PurgeProgress != nil || report.Recovery != nil ||
						len(report.Hashes) != 0 || len(report.GenerationIDs) != 0 ||
						len(report.References) != 0 || len(report.Blobs) != 0 ||
						len(report.OrphanBlobs) != 0 || len(report.Advisories) != 0 {
						t.Fatalf("selector refusal populated repair/selection state: %#v", report)
					}
					s7ARAssertNoGitSpawn(t, trace)
					if after := readTree(t, filepath.Join(root, ".tpatch")); !bytes.Equal(before, after) {
						t.Fatal("selector refusal mutated the workspace")
					}
					if intentlock.AuthoritySupported {
						authority, err := intentlock.Acquire(root)
						if err != nil {
							t.Fatalf("selector refusal retained the workspace authority: %v", err)
						}
						if err := authority.Release(); err != nil {
							t.Fatal(err)
						}
					}

					humanArgs := []string{
						"--path", root, "feature", "intent-archive", "purge", slug,
						selector.flag, selector.value,
					}
					if confirmed {
						humanArgs = append(humanArgs, "--yes")
					}
					humanCode, human, humanErr, _ := runPrepare(t, humanArgs...)
					combined := human + humanErr
					if humanCode != code {
						t.Fatalf("human exit = %d, JSON exit = %d", humanCode, code)
					}
					for _, want := range []string{
						refusal.Code, refusal.Message, refusal.Remediation,
					} {
						if !strings.Contains(combined, want) {
							t.Fatalf("human output lacks %q:\n%s", want, combined)
						}
					}
					if strings.Contains(combined, prepareRetryHeader) ||
						strings.Contains(combined, root) ||
						strings.Contains(combined, "orphan blobs:") {
						t.Fatalf("human selector refusal printed a retry or an absolute path:\n%s", combined)
					}
				})
			}
		}
	}
}

// TestRev20MalformedSelectorIsAUsageErrorNotARefusal pins the boundary rev-20
// must not cross. A `--blob`/`--generation` value that is not a full lowercase
// SHA-256 is rejected by the command's own selector normalization after the
// higher-precedence workspace/journal checks, so it exits 1 with an empty
// report and no refusal code — `archive-selector-invalid` is never emitted for
// it in either command form.
// This is the established shipped contract, and it is asserted here so the
// rev-20 amendment cannot quietly promote the population into the catalog.
func TestRev20MalformedSelectorIsAUsageErrorNotARefusal(t *testing.T) {
	for _, selector := range rev20MalformedSelectorValues {
		for _, confirmed := range []bool{false, true} {
			form := "preview"
			if confirmed {
				form = "confirmed"
			}
			t.Run(selector.name+"/"+form, func(t *testing.T) {
				if confirmed && !intentlock.AuthoritySupported {
					t.Skip("real workspace authority is unsupported on this target")
				}
				root, slug := intentArchiveCLIWorkspace(t)
				data := []byte("rev20 malformed fixture\n")
				replacement := intentArchiveCLIReplacement(
					t, store.IntentArchiveArtifactAnalysis, data,
					store.IntentArchiveWireRetained,
				)
				writeIntentArchiveCLIFixture(t, root, slug,
					intentArchiveCLIIndex(t, slug,
						intentArchiveCLIGeneration(t, slug, replacement)),
					map[string][]byte{replacement.ContentSHA256: data},
				)
				before := readTree(t, filepath.Join(root, ".tpatch"))

				trace := s7ARInstallGitSpawnSpy(t, root)
				previousBeforeLock := beforeLockAcquire
				lockCalls := 0
				beforeLockAcquire = func() { lockCalls++ }
				t.Cleanup(func() { beforeLockAcquire = previousBeforeLock })

				args := []string{
					"--path", root, "feature", "intent-archive", "purge", slug,
					selector.flag, selector.value, "--json", "--quiet",
				}
				if confirmed {
					args = append(args, "--yes")
				}
				code, stdout, stderr, _ := runPrepare(t, args...)
				if code != 1 {
					t.Fatalf("malformed selector exit = %d, want 1; stdout=%q stderr=%q",
						code, stdout, stderr)
				}
				if stdout != "" {
					t.Fatalf("malformed selector printed a report: %q", stdout)
				}
				combined := stdout + stderr
				if strings.Contains(combined, string(store.IntentArchiveCodeSelectorInvalid)) {
					t.Fatalf("malformed selector emitted a structured refusal code: %q", combined)
				}
				for _, structured := range []string{
					"archive-index-corrupt", "refusal", "\"outcome\"",
				} {
					if strings.Contains(combined, structured) {
						t.Fatalf("malformed selector emitted %q: %q", structured, combined)
					}
				}
				if strings.Contains(combined, selector.value) {
					t.Fatalf("malformed selector echoed the rejected value: %q", combined)
				}
				wantLockCalls := 0
				if confirmed {
					wantLockCalls = 1
				}
				if lockCalls != wantLockCalls {
					t.Fatalf("malformed selector lock attempts = %d, want %d", lockCalls, wantLockCalls)
				}

				// The human form must agree: same exit, still no report.
				humanArgs := []string{
					"--path", root, "feature", "intent-archive", "purge", slug,
					selector.flag, selector.value,
				}
				if confirmed {
					humanArgs = append(humanArgs, "--yes")
				}
				humanCode, human, humanErr, _ := runPrepare(t, humanArgs...)
				if humanCode != 1 || human != "" {
					t.Fatalf("human malformed selector = %d stdout=%q stderr=%q",
						humanCode, human, humanErr)
				}
				if strings.Contains(human+humanErr, selector.value) {
					t.Fatalf("human malformed selector echoed the rejected value: %q", human+humanErr)
				}
				if lockCalls != 2*wantLockCalls {
					t.Fatalf("malformed selector lock attempts after human parity = %d, want %d",
						lockCalls, 2*wantLockCalls)
				}

				s7ARAssertNoGitSpawn(t, trace)
				if after := readTree(t, filepath.Join(root, ".tpatch")); !bytes.Equal(before, after) {
					t.Fatal("malformed selector mutated the archive")
				}
				if intentlock.AuthoritySupported {
					authority, err := intentlock.Acquire(root)
					if err != nil {
						t.Fatalf("malformed selector retained the workspace authority: %v", err)
					}
					if err := authority.Release(); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}

	// Cross-binding, not duplication: the established contract test owns this
	// behaviour, and rev-20 must not be able to change it in one place only.
	t.Run("established-contract-still-pins-exit-1", func(t *testing.T) {
		source := rev20RepoSource(t, "internal/cli/feature_intent_archive_test.go")
		const owner = "func TestFeatureIntentArchiveSelectorValidationAndSequentialRepair("
		start := strings.Index(source, owner)
		if start < 0 {
			t.Fatal("the established selector-validation contract test is gone")
		}
		end := strings.Index(source[start:], "\n}\n")
		if end < 0 {
			t.Fatal("the established selector-validation contract test is unterminated")
		}
		body := source[start : start+end]
		malformed := strings.Index(body, `t.Run("malformed-selector"`)
		if malformed < 0 {
			t.Fatal("the established test no longer covers the malformed-selector population")
		}
		// Scope to that one subtest: its siblings legitimately assert exit 3.
		scope := body[malformed:]
		if next := strings.Index(scope[1:], "\n\tt.Run(\""); next >= 0 {
			scope = scope[:next+1]
		}
		for _, assertion := range []string{
			`if code != 1 || stdout != "" ||`,
			"strings.Contains(stdout+stderr, test.value)",
			"strings.Contains(stdout+stderr, string(store.IntentArchiveCodeSelectorInvalid))",
			"malformed selector wrote to the workspace",
			"malformed selector retained authority",
		} {
			if !strings.Contains(scope, assertion) {
				t.Fatalf("the established malformed-selector contract no longer asserts %q", assertion)
			}
		}
		for _, forbidden := range []string{"code != 3", "code == 3"} {
			if strings.Contains(scope, forbidden) {
				t.Fatalf("the established malformed-selector contract was moved to exit 3 (%q)", forbidden)
			}
		}
		// Both selector families and both command forms stay in that subtest.
		for _, shape := range []string{
			`flag: "--blob"`, `flag: "--generation"`, "confirmed: true",
		} {
			if !strings.Contains(scope, shape) {
				t.Fatalf("the established malformed-selector table lost %q", shape)
			}
		}
	})
}

// TestRev20SelectorClassificationSourceGuard is the sensitivity half: the two
// exact regressions the external review found must fail the same validator the
// shipped source passes.
func TestRev20SelectorClassificationSourceGuard(t *testing.T) {
	sources := map[string]string{
		"internal/cli/feature_intent_archive.go": rev20RepoSource(t, "internal/cli/feature_intent_archive.go"),
		"internal/store/intent_archive.go":       rev20RepoSource(t, "internal/store/intent_archive.go"),
	}
	if err := validateRev20SelectorClassification(sources); err != nil {
		t.Fatalf("the shipped selector classification failed its own guard: %v", err)
	}

	rewritten := rev20CloneSources(sources)
	rewritten["internal/cli/feature_intent_archive.go"] = strings.Replace(
		rewritten["internal/cli/feature_intent_archive.go"],
		"\tcode := string(typed.Code)\n",
		"\tcode := string(typed.Code)\n"+
			"\tif typed.Code == store.IntentArchiveCodeSelectorInvalid {\n"+
			"\t\tcode = string(store.IntentArchiveCodeIndexCorrupt)\n"+
			"\t}\n",
		1,
	)
	if rewritten["internal/cli/feature_intent_archive.go"] == sources["internal/cli/feature_intent_archive.go"] {
		t.Fatal("the CLI rewrite mutation anchor is missing")
	}
	if err := validateRev20SelectorClassification(rewritten); err == nil {
		t.Fatal("the same validator accepted a reintroduced CLI code rewrite")
	}

	reclassified := rev20CloneSources(sources)
	reclassified["internal/store/intent_archive.go"] = strings.Replace(
		reclassified["internal/store/intent_archive.go"],
		`err := intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --generation selector does not name an archive generation", 3)`,
		`err := intentArchiveError(IntentArchiveCodeIndexCorrupt, "a --generation selector does not name an archive generation", 3)`,
		1,
	)
	if reclassified["internal/store/intent_archive.go"] == sources["internal/store/intent_archive.go"] {
		t.Fatal("the store generation-branch mutation anchor is missing")
	}
	if err := validateRev20SelectorClassification(reclassified); err == nil {
		t.Fatal("the same validator accepted an unknown generation classified as index corruption")
	}

	echoed := rev20CloneSources(sources)
	echoed["internal/store/intent_archive.go"] = strings.Replace(
		echoed["internal/store/intent_archive.go"],
		`return normalized, "", intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --blob selector is not a lowercase SHA-256", 3)`,
		`malformed := intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --blob selector is not a lowercase SHA-256", 3)`+
			"\n\t\t\tmalformed.Hash = hash\n\t\t\treturn normalized, \"\", malformed",
		1,
	)
	if echoed["internal/store/intent_archive.go"] == sources["internal/store/intent_archive.go"] {
		t.Fatal("the malformed-value mutation anchor is missing")
	}
	if err := validateRev20SelectorClassification(echoed); err == nil {
		t.Fatal("the same validator accepted a malformed selector retained on the typed error")
	}

	demoted := rev20CloneSources(sources)
	demoted["internal/store/intent_archive.go"] = strings.Replace(
		demoted["internal/store/intent_archive.go"],
		`return intentArchiveError(IntentArchiveCodeIndexCorrupt, "index.json has an invalid schema version", 3)`,
		`return intentArchiveError(IntentArchiveCodeSelectorInvalid, "index.json has an invalid schema version", 3)`,
		1,
	)
	if demoted["internal/store/intent_archive.go"] == sources["internal/store/intent_archive.go"] {
		t.Fatal("the strict-decode mutation anchor is missing")
	}
	if err := validateRev20SelectorClassification(demoted); err == nil {
		t.Fatal("the same validator accepted a strict-index failure reclassified as a selector fault")
	}

	// rev-20's own boundary: a malformed value must not be able to acquire a
	// public refusal code by being routed into the report instead of returned.
	structured := rev20CloneSources(sources)
	structured["internal/cli/feature_intent_archive.go"] = strings.Replace(
		structured["internal/cli/feature_intent_archive.go"],
		"\tnormalizedOptions, normalizeErr := normalizeIntentArchivePurgeOptions(options)\n"+
			"\tif normalizeErr != nil {\n\t\treturn normalizeErr\n\t}\n",
		"\tnormalizedOptions, normalizeErr := normalizeIntentArchivePurgeOptions(options)\n"+
			"\tif normalizeErr != nil {\n"+
			"\t\treport.Outcome = \"refused\"\n"+
			"\t\treport.Refusal = intentArchiveSimpleRefusal(\n"+
			"\t\t\tstring(store.IntentArchiveCodeSelectorInvalid),\n"+
			"\t\t\t\"The purge selection is not a well-formed selector.\",\n"+
			"\t\t\t\"Use one well-formed selector.\",\n"+
			"\t\t)\n"+
			"\t\treturn emitIntentArchivePurgeReport(cmd, report, 3)\n\t}\n",
		1,
	)
	if structured["internal/cli/feature_intent_archive.go"] ==
		sources["internal/cli/feature_intent_archive.go"] {
		t.Fatal("the malformed-selector hand-back mutation anchor is missing")
	}
	if err := validateRev20SelectorClassification(structured); err == nil {
		t.Fatal("the same validator accepted a malformed selector rendered as a structured refusal")
	}

	// The CLI-side normalization must stay a plain usage error: the moment it
	// names a typed archive code the exit-1 population starts leaking one.
	typedBoundary := rev20CloneSources(sources)
	typedBoundary["internal/cli/feature_intent_archive.go"] = strings.Replace(
		typedBoundary["internal/cli/feature_intent_archive.go"],
		`return intentArchivePurgeOptions{}, errors.New("each --blob value must be 64 lowercase hexadecimal characters")`,
		`return intentArchivePurgeOptions{}, errors.New(string(store.IntentArchiveCodeSelectorInvalid))`,
		1,
	)
	if typedBoundary["internal/cli/feature_intent_archive.go"] ==
		sources["internal/cli/feature_intent_archive.go"] {
		t.Fatal("the CLI normalization mutation anchor is missing")
	}
	if err := validateRev20SelectorClassification(typedBoundary); err == nil {
		t.Fatal("the same validator accepted a typed archive code in the CLI selector normalization")
	}

	// The validator must not claim coverage it does not have: a corpus that
	// silently grows a third emitter is rejected rather than blessed.
	overreaching := rev20CloneSources(sources)
	overreaching["internal/cli/prepare_publish.go"] =
		rev20RepoSource(t, "internal/cli/prepare_publish.go")
	if err := validateRev20SelectorClassification(overreaching); err == nil {
		t.Fatal("the same validator claimed coverage over an emitter it does not analyze")
	}
}

// validateRev20SelectorClassification derives the rule from the sources rather
// than from a list of line numbers. It reads exactly two declarations' worth of
// behaviour and claims nothing beyond them:
//
//   - internal/store: every population of the purge-selector normalization is
//     classified `archive-selector-invalid` and the strict index decoder keeps
//     only `archive-index-*`. This is the store's *internal* classification —
//     the malformed and arity branches are unreachable from the shipped CLI —
//     so the only thing asserted about them is that they bind no rejected raw
//     value to the typed error.
//   - internal/cli: the malformed and arity populations are rejected by the
//     command's own plain-error normalization *before* any store call, so they
//     can never reach structured output; and the typed archive refusal
//     renderer `intentArchiveRefusalFromError` surfaces an already-public
//     catalog code unchanged.
//
// The no-relabel obligation is scoped to that one renderer on purpose. An
// internal, non-catalog transport failure — `archive-storage-failed` — is
// legitimately classified into a catalog code by the boundary that owns it,
// which is what `prepareStoreArchiveFailure` in internal/cli/prepare_publish.go
// does. That file is deliberately not in this corpus, and this validator makes
// no claim about it.
func validateRev20SelectorClassification(sources map[string]string) error {
	if len(sources) != 2 {
		return fmt.Errorf(
			"the corpus has %d files; this validator reads exactly the store selector "+
				"normalization and the CLI purge boundary, and claims nothing about any other emitter",
			len(sources),
		)
	}
	storeSource, ok := sources["internal/store/intent_archive.go"]
	if !ok {
		return fmt.Errorf("the store source is missing from the corpus")
	}
	cliSource, ok := sources["internal/cli/feature_intent_archive.go"]
	if !ok {
		return fmt.Errorf("the CLI source is missing from the corpus")
	}

	storeFile, err := parser.ParseFile(token.NewFileSet(), "intent_archive.go", storeSource, 0)
	if err != nil {
		return err
	}
	var normalized *ast.FuncDecl
	var validate *ast.FuncDecl
	for _, declaration := range storeFile.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Body == nil {
			continue
		}
		if function.Name.Name == "normalized" && function.Recv != nil {
			normalized = function
		}
		if function.Name.Name == "validateIntentArchiveIndexWire" ||
			function.Name.Name == "ValidateIntentArchiveIndex" {
			validate = function
		}
	}
	if normalized == nil {
		return fmt.Errorf("the purge-selector normalization is missing")
	}
	if validate == nil {
		return fmt.Errorf("the strict index validator is missing")
	}

	selectorCodes := rev20ArchiveErrorCodes(normalized)
	if len(selectorCodes) == 0 {
		return fmt.Errorf("the selector normalization classifies nothing")
	}
	for _, code := range selectorCodes {
		if code != "IntentArchiveCodeSelectorInvalid" {
			return fmt.Errorf(
				"the selector normalization emits %s; every selector population is archive-selector-invalid",
				code,
			)
		}
	}
	for _, code := range rev20ArchiveErrorCodes(validate) {
		if code == "IntentArchiveCodeSelectorInvalid" {
			return fmt.Errorf("the strict index validator emits the selector code")
		}
	}

	// The malformed populations must not bind the rejected raw value onto the
	// typed error; the unknown populations must bind their validated one.
	malformed := regexp.MustCompile(
		`(?s)intentArchiveError\(IntentArchiveCodeSelectorInvalid, "a --(?:blob|generation) selector is not a lowercase SHA-256", 3\)(.{0,60})`)
	for _, match := range malformed.FindAllStringSubmatch(storeSource, -1) {
		if strings.Contains(match[1], ".Hash =") || strings.Contains(match[1], ".GenerationID =") {
			return fmt.Errorf("a malformed selector retains its rejected value on the typed error")
		}
	}
	for _, wanted := range []string{
		`intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --blob selector does not name an indexed content hash", 3)`,
		`intentArchiveError(IntentArchiveCodeSelectorInvalid, "a --generation selector does not name an archive generation", 3)`,
	} {
		if !strings.Contains(storeSource, wanted) {
			return fmt.Errorf("the store no longer classifies an unknown selector: %q", wanted)
		}
	}

	cliFile, err := parser.ParseFile(token.NewFileSet(), "feature_intent_archive.go", cliSource, 0)
	if err != nil {
		return err
	}
	functions := map[string]*ast.FuncDecl{}
	for _, declaration := range cliFile.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if isFunction && function.Recv == nil {
			functions[function.Name.Name] = function
		}
	}

	// The CLI boundary: malformed values and a wrong scope-family count are
	// rejected as plain usage errors, so they exit 1 with no report at all.
	// Neither may be able to reach a typed archive code or a refusal builder.
	for _, name := range []string{
		"normalizeIntentArchivePurgeOptions",
		"validateIntentArchivePurgeScope",
	} {
		boundary := functions[name]
		if boundary == nil || boundary.Body == nil {
			return fmt.Errorf("the CLI selector boundary %s is missing", name)
		}
		if rev20MentionsArchiveErrorCode(boundary.Body) {
			return fmt.Errorf(
				"%s names a typed archive code; a malformed or mis-scoped selector must stay a plain usage error",
				name,
			)
		}
		for _, emitter := range rev20CalledFunctions(boundary) {
			switch emitter {
			case "intentArchiveRefusalFromError", "intentArchiveSimpleRefusal",
				"emitIntentArchivePurgeReport", "newIntentArchivePurgeReport":
				return fmt.Errorf(
					"%s calls %s; a malformed or mis-scoped selector must not reach structured output",
					name, emitter,
				)
			}
		}
	}

	// Both purge forms must hand that plain error straight back to the process
	// boundary. Routing it into a report is exactly how the malformed
	// population would acquire a public refusal code it does not have.
	for _, name := range []string{
		"runFeatureIntentArchivePurgePreview",
		"runFeatureIntentArchivePurgeConfirmed",
	} {
		form := functions[name]
		if form == nil || form.Body == nil {
			return fmt.Errorf("the purge form %s is missing", name)
		}
		if !rev20ReturnsBareNormalizeError(form) {
			return fmt.Errorf(
				"%s does not return the selector normalization error unchanged; "+
					"a malformed selector would reach a structured report",
				name,
			)
		}
	}

	// Inside the typed refusal renderer the classified code is already decided,
	// so the only assignment to `code` may be the forwarding one. The `list`
	// classifier at `intentArchiveListInspectionRefusal` legitimately derives a
	// code from an observation and is deliberately outside this scope, as is
	// every emitter in another file.
	renderer := functions["intentArchiveRefusalFromError"]
	if renderer == nil {
		return fmt.Errorf("the typed-error refusal renderer is missing")
	}
	assignments := 0
	var rewriteErr error
	ast.Inspect(renderer.Body, func(node ast.Node) bool {
		if rewriteErr != nil {
			return false
		}
		assignment, isAssignment := node.(*ast.AssignStmt)
		if !isAssignment || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		target, isIdent := assignment.Lhs[0].(*ast.Ident)
		if !isIdent || target.Name != "code" {
			return true
		}
		assignments++
		if rev20MentionsArchiveErrorCode(assignment.Rhs[0]) {
			rewriteErr = fmt.Errorf(
				"intentArchiveRefusalFromError assigns a store archive code to the rendered refusal code; " +
					"an already-public catalog code must reach the report unmodified",
			)
		}
		return true
	})
	if rewriteErr != nil {
		return rewriteErr
	}
	if assignments != 1 {
		return fmt.Errorf(
			"the refusal renderer assigns `code` %d times; exactly one forwarding assignment is allowed",
			assignments,
		)
	}
	if !strings.Contains(cliSource, "case store.IntentArchiveCodeSelectorInvalid:") {
		return fmt.Errorf("the CLI renders no selector-specific refusal text")
	}
	for _, wanted := range []string{
		"tpatch feature intent-archive list ",
		"The rejected value is not echoed.",
	} {
		if !strings.Contains(cliSource, wanted) {
			return fmt.Errorf("the CLI selector refusal no longer says %q", wanted)
		}
	}
	return nil
}

// rev20CalledFunctions returns the bare function names a declaration calls.
func rev20CalledFunctions(declaration *ast.FuncDecl) []string {
	var names []string
	ast.Inspect(declaration, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		if identifier, ok := call.Fun.(*ast.Ident); ok {
			names = append(names, identifier.Name)
		}
		return true
	})
	return names
}

// rev20ReturnsBareNormalizeError reports whether a purge form contains the
// `if normalizeErr != nil { return normalizeErr }` shape — the plain-error
// hand-back that keeps a malformed selector at exit 1 with no report.
func rev20ReturnsBareNormalizeError(declaration *ast.FuncDecl) bool {
	found := false
	ast.Inspect(declaration, func(node ast.Node) bool {
		if found {
			return false
		}
		branch, isIf := node.(*ast.IfStmt)
		if !isIf || branch.Else != nil || branch.Body == nil || len(branch.Body.List) != 1 {
			return true
		}
		condition, isBinary := branch.Cond.(*ast.BinaryExpr)
		if !isBinary || condition.Op != token.NEQ {
			return true
		}
		left, isIdent := condition.X.(*ast.Ident)
		right, isNil := condition.Y.(*ast.Ident)
		if !isIdent || !isNil || left.Name != "normalizeErr" || right.Name != "nil" {
			return true
		}
		returned, isReturn := branch.Body.List[0].(*ast.ReturnStmt)
		if !isReturn || len(returned.Results) != 1 {
			return true
		}
		result, isResultIdent := returned.Results[0].(*ast.Ident)
		if isResultIdent && result.Name == "normalizeErr" {
			found = true
		}
		return true
	})
	return found
}

// rev20ArchiveErrorCodes returns the `IntentArchiveCode*` identifiers a
// declaration passes to `intentArchiveError`, in source order.
func rev20ArchiveErrorCodes(declaration *ast.FuncDecl) []string {
	var codes []string
	ast.Inspect(declaration, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall || len(call.Args) == 0 {
			return true
		}
		name, isIdent := call.Fun.(*ast.Ident)
		if !isIdent || name.Name != "intentArchiveError" {
			return true
		}
		if identifier, ok := call.Args[0].(*ast.Ident); ok {
			codes = append(codes, identifier.Name)
		}
		return true
	})
	return codes
}

func rev20MentionsArchiveErrorCode(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(child ast.Node) bool {
		selector, isSelector := child.(*ast.SelectorExpr)
		if isSelector && strings.HasPrefix(selector.Sel.Name, "IntentArchiveCode") {
			found = true
		}
		return !found
	})
	return found
}

func rev20RepoSource(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(avpRepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func rev20CloneSources(sources map[string]string) map[string]string {
	clone := make(map[string]string, len(sources))
	for name, body := range sources {
		clone[name] = body
	}
	return clone
}

// TestRev20ClosedRefusalCatalogIsFiftyFour pins the closed catalog at its new
// size and order, and bites in all three directions the amendment could be
// broken: a removal, an addition, and a reclassification of the new code into
// the archive-integrity row it is deliberately not part of.
func TestRev20ClosedRefusalCatalogIsFiftyFour(t *testing.T) {
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	baseline, err := rev20CatalogFromPRD(prd)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRev20Catalog(baseline); err != nil {
		t.Fatalf("the shipped §10.4.1 catalog failed its own guard: %v", err)
	}
	if fmt.Sprint(baseline) != fmt.Sprint(s6RefusalCatalog) {
		t.Fatalf("§10.4.1 catalog = %v, want the frozen order %v", baseline, s6RefusalCatalog)
	}
	position := -1
	for index, code := range baseline {
		if code == "archive-selector-invalid" {
			position = index
		}
	}
	if position <= 0 || baseline[position-1] != "archive-purge-index-changed" ||
		baseline[position+1] != "no-pending-transaction" {
		t.Fatalf("archive-selector-invalid is at %d in %v", position, baseline)
	}

	removed := append([]string(nil), baseline[:position]...)
	removed = append(removed, baseline[position+1:]...)
	if err := validateRev20Catalog(removed); err == nil {
		t.Fatal("the same validator accepted a catalog without the selector code")
	}

	extra := append([]string(nil), baseline...)
	extra = append(extra, "archive-selector-unknown")
	if err := validateRev20Catalog(extra); err == nil {
		t.Fatal("the same validator accepted a 55-code catalog")
	}

	renamed := append([]string(nil), baseline...)
	renamed[position] = "archive-index-selector-invalid"
	if err := validateRev20Catalog(renamed); err == nil {
		t.Fatal("the same validator accepted the selector code renamed into the index family")
	}

	// Reclassification: the code exists, but §10.4.1 files it inside the
	// archive-integrity row instead of its own.
	reclassified := strings.Replace(prd,
		"| `archive-selector-invalid` | 3 |",
		"| `unused-placeholder-code` | 3 |",
		1)
	reclassified = strings.Replace(reclassified,
		"`archive-blob-shared`, `archive-purge-index-changed` | 3 | preserve bytes;",
		"`archive-blob-shared`, `archive-purge-index-changed`, `archive-selector-invalid` | 3 | preserve bytes;",
		1)
	shuffled, err := rev20CatalogFromPRD(reclassified)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(shuffled) == fmt.Sprint(baseline) {
		t.Fatal("the reclassification fixture did not move the code")
	}
	if err := validateRev20Catalog(shuffled); err == nil {
		t.Fatal("the same validator accepted the selector code folded into the archive-integrity row")
	}
}

func validateRev20Catalog(codes []string) error {
	if len(codes) != 54 {
		return fmt.Errorf("the closed refusal catalog has %d codes, want 54", len(codes))
	}
	seen := map[string]bool{}
	position := -1
	for index, code := range codes {
		if seen[code] {
			return fmt.Errorf("duplicate refusal code %q", code)
		}
		seen[code] = true
		if code == "archive-selector-invalid" {
			position = index
		}
	}
	if position < 0 {
		return fmt.Errorf("the catalog does not contain archive-selector-invalid")
	}
	if position == 0 || codes[position-1] != "archive-purge-index-changed" ||
		position+1 >= len(codes) || codes[position+1] != "no-pending-transaction" {
		return fmt.Errorf("archive-selector-invalid is not its own row between the archive and transaction blocks")
	}
	return nil
}

// rev20CatalogFromPRD reads §10.4.1's first-cell codes in document order, the
// same way the shipped S6 reader does.
func rev20CatalogFromPRD(prd string) ([]string, error) {
	start := strings.Index(prd, "### 10.4.1 Closed refusal catalog")
	if start < 0 {
		return nil, fmt.Errorf("§10.4.1 is missing")
	}
	end := strings.Index(prd[start:], "### 10.5 Precedence")
	if end < 0 {
		return nil, fmt.Errorf("§10.4.1 is unterminated")
	}
	section := prd[start : start+end]
	pattern := regexp.MustCompile("`([a-z][a-z0-9-]+)`")
	seen := map[string]bool{}
	var codes []string
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		firstCell := strings.SplitN(strings.TrimPrefix(line, "|"), "|", 2)[0]
		for _, match := range pattern.FindAllStringSubmatch(firstCell, -1) {
			if !seen[match[1]] {
				seen[match[1]] = true
				codes = append(codes, match[1])
			}
		}
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("parsed no refusal codes from §10.4.1")
	}
	return codes, nil
}

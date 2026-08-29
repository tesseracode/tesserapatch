//go:build (linux && !android) || (darwin && !ios)

package cli

// §18.53 sensitivity fixtures for the `G` rows whose wrong-input body did not
// previously exist as an independently executable target.
//
// Every test below runs the row's own validator over the shipped input (which
// must be accepted), then over a deliberately wrong version of that same input
// (which the same validator must reject). No body scans a row name, none is
// satisfied by "the scan ran", and none is a baseline-only assertion.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// ---------------------------------------------------------------------------
// Golden byte-parity family.
//
// Every fixture below captures the surfaces the *current* binary produces
// through the shipped `buildPreparePIBCurrentBinary` → `capturePreparePIBSurfaces`
// path (shared once per test binary by `preparePIBCurrentCapture`) and judges
// them with `preparePIBGoldenDelta` — the same comparator
// `TestPreparePIBPreChangeGoldens` drives through `comparePreparePIBGolden`.
// Nothing here re-reads a recorded fixture as if it were product output, and
// nothing records or overwrites a fixture: PIB-391's producer/provenance rules
// stay the only way bytes enter `testdata/prepare-pib-goldens`.
// ---------------------------------------------------------------------------

// pibDoctorGoldenNames returns the frozen D1…D8 doctor set exactly as the
// shipped golden suite enumerates it — `TestPreparePIBPreChangeGoldens` subtest
// `doctor-D1-through-D8` builds the same eight names — and proves the current
// binary produces that set and nothing else under the `doctor-` prefix. The
// prior selector `"doctor-D"` matched nothing at all, because
// `matchesGoldenSelector` only prefix-matches a selector that ends in `-`.
func pibDoctorGoldenNames(t *testing.T, captured map[string]string) []string {
	t.Helper()
	frozen := make([]string, 0, 8)
	for probe := 1; probe <= 8; probe++ {
		frozen = append(frozen, fmt.Sprintf("doctor-D%d.txt", probe))
	}
	produced := preparePIBCapturedNames(t, captured, []string{"doctor-"})
	if !reflect.DeepEqual(produced, frozen) {
		t.Fatalf("the current doctor surface set is %v, want the frozen D1…D8 set %v", produced, frozen)
	}
	return frozen
}

func TestPIBSensitivityPIB136DoctorGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := pibDoctorGoldenNames(t, captured)
	if len(names) != 8 {
		t.Fatalf("PIB-136: the doctor population is %d surfaces, want the frozen eight", len(names))
	}
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-136: the current doctor D1…D8 surfaces failed the shipped comparator: %v", err)
		}
	}
	for _, probe := range []string{names[0], names[len(names)-1]} {
		drifted := captured[probe] + "\n"
		if err := preparePIBGoldenDelta(probe, drifted); err == nil {
			t.Fatalf("PIB-136: the shipped comparator accepted a one-byte drift in %s", probe)
		}
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-136: the shipped comparator accepted an unproduced %s", names[0])
	}
}

func TestPIBSensitivityPIB186PhaseGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-186"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-186: the current automatic analyze/define/explore surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-186: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-186: the shipped comparator accepted an unproduced %s", names[0])
	}
}

func TestPIBSensitivityPIB198CheckHumanGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-198"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-198: the current --check human surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-198: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-198: the shipped comparator accepted an unproduced %s", names[0])
	}
}

func TestPIBSensitivityPIB199CheckJSONGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-199"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-199: the current --check JSON surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-199: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-199: the shipped comparator accepted an unproduced %s", names[0])
	}
}

func TestPIBSensitivityPIB207CheckPendingGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-207"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-207: the current pending-journal --check surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-207: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-207: the shipped comparator accepted an unproduced %s", names[0])
	}
}

func TestPIBSensitivityPIB208NextGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-208"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-208: the current next surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-208: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-208: the shipped comparator accepted an unproduced %s", names[0])
	}
}

func TestPIBSensitivityPIB210PhaseAutoGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-210"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-210: the current automatic phase surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-210: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-210: the shipped comparator accepted an unproduced %s", names[0])
	}
}

func TestPIBSensitivityPIB211PhaseManualGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-211"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-211: the current manual phase surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-211: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-211: the shipped comparator accepted an unproduced %s", names[0])
	}
}

func TestPIBSensitivityPIB212CompatibilityGoldenParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-212"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-212: the current compatibility surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-212: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	if err := preparePIBGoldenDelta(names[0], ""); err == nil {
		t.Fatalf("PIB-212: the shipped comparator accepted an unproduced %s", names[0])
	}
}

// TestPIBSensitivityPIB209CycleGoldenDigests owns PIB-209: the accepted cycle
// transcript is pinned by digest, and a one-byte change must break the pin.
func TestPIBSensitivityPIB209CycleGoldenDigests(t *testing.T) {
	names := []string{"cycle-skip-execute-transcript.txt", "cycle-final-state.txt"}
	current := map[string][]byte{}
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(routingGoldenDir, name))
		if err != nil {
			t.Fatal(err)
		}
		current[name] = body
	}
	if err := pibValidateGoldenDigests(current, routingGoldenSHA256); err != nil {
		t.Fatalf("PIB-209: the accepted cycle goldens failed their own digest pin: %v", err)
	}
	mutated := map[string][]byte{}
	for name, body := range current {
		mutated[name] = body
	}
	mutated[names[0]] = append(append([]byte(nil), current[names[0]]...), ' ')
	if err := pibValidateGoldenDigests(mutated, routingGoldenSHA256); err == nil {
		t.Fatal("PIB-209: the digest pin accepted a one-byte change to the cycle transcript")
	}
}

func pibValidateGoldenDigests(current map[string][]byte, pinned map[string]string) error {
	if len(current) == 0 {
		return fmt.Errorf("the digest pin was asked to compare an empty population")
	}
	for name, body := range current {
		want, ok := pinned[name]
		if !ok {
			return fmt.Errorf("no accepted digest is pinned for %s", name)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != want {
			return fmt.Errorf("%s no longer matches its accepted digest", name)
		}
	}
	return nil
}

// TestPIBSensitivityPIB286ResourceSurfaceParity owns PIB-286: the shipped
// resource subcommands stay byte-identical — judged on current product bytes by
// the shipped comparator — and prepare introduces no rescap extraction.
func TestPIBSensitivityPIB286ResourceSurfaceParity(t *testing.T) {
	captured := preparePIBCurrentCapture(t)
	names := preparePIBCapturedNames(t, captured, preparePIBRows["PIB-286"])
	for _, name := range names {
		if err := preparePIBGoldenDelta(name, captured[name]); err != nil {
			t.Fatalf("PIB-286: the current resource surface drifted from its recorded golden: %v", err)
		}
	}
	drifted := captured[names[0]] + "\n"
	if err := preparePIBGoldenDelta(names[0], drifted); err == nil {
		t.Fatalf("PIB-286: the shipped comparator accepted a one-byte drift in %s", names[0])
	}
	sources := pibGuardCLIProductionSources(t)
	if err := pibValidateNoRescapExtraction(sources); err != nil {
		t.Fatalf("PIB-286: prepare already extracts rescap: %v", err)
	}
	mutated := pibGuardClone(sources)
	mutated["internal/cli/prepare_publish.go"] += "\n// rescap.Acquire(root)\n"
	if err := pibValidateNoRescapExtraction(mutated); err == nil {
		t.Fatal("PIB-286: the guard accepted a rescap extraction inside prepare")
	}
}

func pibValidateNoRescapExtraction(sources map[string]string) error {
	if len(sources) == 0 {
		return fmt.Errorf("the rescap scan received no sources")
	}
	for name, body := range sources {
		if strings.Contains(body, "rescap.") {
			return fmt.Errorf("%s reaches into rescap", name)
		}
	}
	return nil
}

// TestPIBSensitivityPIB287RescapPlatformEnvelope owns PIB-287: the resource
// capture envelope is unchanged and its unsupported refusal stays distinct.
func TestPIBSensitivityPIB287RescapPlatformEnvelope(t *testing.T) {
	sources := pibGuardSources(t,
		"internal/rescap/lock_unix.go",
		"internal/rescap/lock_unsupported.go",
	)
	if err := pibValidateRescapEnvelope(sources); err != nil {
		t.Fatalf("PIB-287: the shipped rescap envelope failed its own guard: %v", err)
	}
	widened := pibGuardClone(sources)
	widened["internal/rescap/lock_unix.go"] = strings.Replace(
		widened["internal/rescap/lock_unix.go"],
		"//go:build linux || darwin",
		"//go:build linux || darwin || windows",
		1,
	)
	if widened["internal/rescap/lock_unix.go"] == sources["internal/rescap/lock_unix.go"] {
		t.Fatal("PIB-287: the envelope mutation anchor is missing")
	}
	if err := pibValidateRescapEnvelope(widened); err == nil {
		t.Fatal("PIB-287: the guard accepted a widened resource-capture envelope")
	}
	flipped := pibGuardClone(sources)
	flipped["internal/rescap/lock_unsupported.go"] = strings.Replace(
		flipped["internal/rescap/lock_unsupported.go"],
		"const LockSupported = false",
		"const LockSupported = true",
		1,
	)
	if err := pibValidateRescapEnvelope(flipped); err == nil {
		t.Fatal("PIB-287: the guard accepted an unsupported target claiming a real lock")
	}
}

func pibValidateRescapEnvelope(sources map[string]string) error {
	unix := sources["internal/rescap/lock_unix.go"]
	unsupported := sources["internal/rescap/lock_unsupported.go"]
	if unix == "" || unsupported == "" {
		return fmt.Errorf("the rescap envelope scan received no sources")
	}
	if !strings.Contains(unix, "//go:build linux || darwin\n") {
		return fmt.Errorf("the supported rescap envelope is no longer exactly linux || darwin")
	}
	if !strings.Contains(unix, "const LockSupported = true") {
		return fmt.Errorf("the supported rescap target no longer declares a real lock")
	}
	if !strings.Contains(unsupported, "const LockSupported = false") {
		return fmt.Errorf("the unsupported rescap target no longer refuses")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Derived-input guards.
// ---------------------------------------------------------------------------

// TestPIBSensitivityPIB142ReportsInferNoPathProvenance owns PIB-142: neither
// report schema may assert a Path A vs Path B route for a feature. The shipped
// reports are produced by the real command, and the same validator that accepts
// them must reject an injected route token.
func TestPIBSensitivityPIB142ReportsInferNoPathProvenance(t *testing.T) {
	root, slug := prepareS4Workspace(t, "PIB-142 provenance sensitivity")
	_, mutating, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
	_, checking, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
	shipped := []string{mutating, checking}
	if err := validatePrepareS5NoPathProvenanceTokens(shipped); err != nil {
		t.Fatalf("PIB-142: the shipped report schemas failed their own guard: %v", err)
	}
	routed := append([]string(nil), shipped...)
	routed[1] = strings.Replace(routed[1], `"provenance"`, `"path_a"`, 1)
	if routed[1] == shipped[1] {
		t.Fatal("PIB-142: the provenance mutation anchor is missing from the check report")
	}
	if err := validatePrepareS5NoPathProvenanceTokens(routed); err == nil {
		t.Fatal("PIB-142: the guard accepted a Path-A route in the --check report")
	}
	attributed := append([]string(nil), shipped...)
	attributed[0] = attributed[0] + "\nThe bundle was authored by the configured provider.\n"
	if err := validatePrepareS5NoPathProvenanceTokens(attributed); err == nil {
		t.Fatal("PIB-142: the guard accepted an authorship claim in the mutating report")
	}
}

// TestPIBSensitivityPIB144GeneratorKeyIsMutatingOnly owns PIB-144: the
// `generator` key exists in the mutating report schema and nowhere else.
func TestPIBSensitivityPIB144GeneratorKeyIsMutatingOnly(t *testing.T) {
	root, slug := prepareS4Workspace(t, "PIB-144 generator sensitivity")
	_, mutating, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
	_, checking, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
	fields, err := prepareS5GeneratorJSONFields(avpRepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePrepareS5GeneratorSchema(mutating, checking, fields); err != nil {
		t.Fatalf("PIB-144: the shipped report schemas failed their own guard: %v", err)
	}
	leaked := checking + "\n  \"generator\": \"tpatch\"\n"
	if err := validatePrepareS5GeneratorSchema(mutating, leaked, fields); err == nil {
		t.Fatal("PIB-144: the guard accepted a generator key on the --check schema")
	}
	renamed := strings.ReplaceAll(mutating, `"generator":`, `"produced_by":`)
	if renamed == mutating {
		t.Fatal("PIB-144: the mutating report carries no generator key to mutate")
	}
	if err := validatePrepareS5GeneratorSchema(renamed, checking, fields); err == nil {
		t.Fatal("PIB-144: the guard accepted a mutating report without its generator key")
	}
	persisted := append(append([]string(nil), fields...), "internal/store/persisted.go:persisted.Generator")
	if err := validatePrepareS5GeneratorSchema(mutating, checking, persisted); err == nil {
		t.Fatal("PIB-144: the guard accepted a generator-class key on a persistence sink")
	}
}

// TestPIBSensitivityPIB145ADR034IsNeverAWritePrecedent owns PIB-145: ADR-034 is
// never cited as precedent for persistence, in source or in docs.
func TestPIBSensitivityPIB145ADR034IsNeverAWritePrecedent(t *testing.T) {
	claims, err := prepareS5WritePrecedentClaims(avpRepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePrepareS5WritePrecedentClaims(claims); err != nil {
		t.Fatalf("PIB-145: the shipped write-precedent claims failed their own guard: %v", err)
	}
	citedInSource := clonePrepareS5Sources(claims)
	citedInSource["internal/cli/prepare_publish.go"] = append(
		citedInSource["internal/cli/prepare_publish.go"],
		[]byte("\n// ADR-034 governs persistence for prepare writes.\n")...,
	)
	if err := validatePrepareS5WritePrecedentClaims(citedInSource); err == nil {
		t.Fatal("PIB-145: the guard accepted an ADR-034 persistence precedent in production source")
	}
	citedInDocs := clonePrepareS5Sources(claims)
	citedInDocs["docs/prds/PRD-prepare-intent-bundle.md"] = append(
		citedInDocs["docs/prds/PRD-prepare-intent-bundle.md"],
		[]byte("\nADR-034 governs every write in prepare.\n")...,
	)
	if err := validatePrepareS5WritePrecedentClaims(citedInDocs); err == nil {
		t.Fatal("PIB-145: the guard accepted an ADR-034 write precedent in the PRD")
	}
}

// TestPIBSensitivityPIB146HeuristicSidecarMatchesAnalyze owns PIB-146: the
// heuristic sidecar is byte-compatible with `analyze`'s output for the same
// input and still declares heuristic mode.
func TestPIBSensitivityPIB146HeuristicSidecarMatchesAnalyze(t *testing.T) {
	fromPrepare, fromAnalyze := prepareS5HeuristicParityCorpus(t)
	if err := validatePrepareS5HeuristicSidecarParity(fromPrepare, fromAnalyze); err != nil {
		t.Fatalf("PIB-146: the shipped heuristic bundle failed its own parity guard: %v", err)
	}
	drifted := clonePrepareS5Sources(fromPrepare)
	drifted["analysis.md"] = append(drifted["analysis.md"], '\n')
	if err := validatePrepareS5HeuristicSidecarParity(drifted, fromAnalyze); err == nil {
		t.Fatal("PIB-146: the guard accepted a one-byte divergence from analyze's output")
	}
	unflagged := clonePrepareS5Sources(fromPrepare)
	unflagged["artifacts/analysis.json"] = bytes.ReplaceAll(
		unflagged["artifacts/analysis.json"],
		[]byte(`"heuristic_mode": true`),
		[]byte(`"heuristic_mode": false`),
	)
	if err := validatePrepareS5HeuristicSidecarParity(unflagged, fromAnalyze); err == nil {
		t.Fatal("PIB-146: the guard accepted a sidecar that no longer declares heuristic mode")
	}
}

// TestPIBSensitivityPIB344LandArchiveSweepIsDisclosed owns PIB-344: the human
// report and `docs/feature-layout.md` both disclose that `land` sweeps
// `intent-archive/**` into the operator's commit. The report text is rendered by
// the shipped human renderer, not paraphrased here.
func TestPIBSensitivityPIB344LandArchiveSweepIsDisclosed(t *testing.T) {
	texts := map[string]string{
		"human-report":           pibLandArchiveHumanReport(),
		"docs/feature-layout.md": s6RepoFile(t, "docs/feature-layout.md"),
	}
	if err := pibValidateLandArchiveDisclosure(texts); err != nil {
		t.Fatalf("PIB-344: the shipped land-sweep disclosure failed its own guard: %v", err)
	}
	silentReport := pibGuardClone(texts)
	silentReport["human-report"] = strings.Replace(
		silentReport["human-report"],
		"Purging the working tree does not remove committed archive bytes from Git history.",
		"Purging the working tree removes the archive everywhere.",
		1,
	)
	if silentReport["human-report"] == texts["human-report"] {
		t.Fatal("PIB-344: the human report carries no committed-bytes disclosure to mutate")
	}
	if err := pibValidateLandArchiveDisclosure(silentReport); err == nil {
		t.Fatal("PIB-344: the guard accepted a report that hides the committed archive bytes")
	}
	silentDocs := pibGuardClone(texts)
	silentDocs["docs/feature-layout.md"] = strings.Replace(
		silentDocs["docs/feature-layout.md"],
		"`tpatch land` stages\nthe archive",
		"`tpatch land` skips\nthe archive",
		1,
	)
	if err := pibValidateLandArchiveDisclosure(silentDocs); err == nil {
		t.Fatal("PIB-344: the guard accepted documentation that drops the land sweep")
	}
	unstagedDocs := pibGuardClone(texts)
	unstagedDocs["docs/feature-layout.md"] = strings.Replace(
		unstagedDocs["docs/feature-layout.md"],
		"like other `artifacts/**` files",
		"unless it is excluded",
		1,
	)
	if err := pibValidateLandArchiveDisclosure(unstagedDocs); err == nil {
		t.Fatal("PIB-344: the guard accepted documentation that drops the artifacts/** sweep scope")
	}
}

// pibLandArchiveHumanReport renders a report that carries an archive through the
// shipped human writer, so PIB-344 reads the product's own disclosure rather
// than a copy of it.
func pibLandArchiveHumanReport() string {
	report := newPreparePublishReport(prepareModeRegenerate, "pib-land-disclosure", "defined")
	report.Outcome = "published"
	report.Action = "regenerated"
	report.Archive = &prepareArchiveReport{
		GenerationID: "gen_pib344",
		BlobsDir:     ".tpatch/features/pib-land-disclosure/artifacts/intent-archive/blobs",
	}
	var human strings.Builder
	writePreparePublishHuman(&human, report)
	return human.String()
}

// pibValidateLandArchiveDisclosure is PIB-344's validator: both governed texts
// must disclose that a landed commit carries the intent archive, and that
// removing the bytes from the tree does not remove them from Git history.
func pibValidateLandArchiveDisclosure(texts map[string]string) error {
	if len(texts) != 2 {
		return fmt.Errorf("the land-sweep disclosure scan covers %d texts, want 2", len(texts))
	}
	report, ok := texts["human-report"]
	if !ok || strings.TrimSpace(report) == "" {
		return fmt.Errorf("the land-sweep scan received no human report")
	}
	if !strings.Contains(report,
		"Purging the working tree does not remove committed archive bytes from Git history.") {
		return fmt.Errorf("the human report no longer discloses that committed archive bytes survive in Git history")
	}
	layout, ok := texts["docs/feature-layout.md"]
	if !ok || strings.TrimSpace(layout) == "" {
		return fmt.Errorf("the land-sweep scan received no feature layout")
	}
	for _, anchor := range []string{
		"`tpatch land` stages\nthe archive",
		"like other `artifacts/**` files",
		"does not remove it from Git history",
	} {
		if !strings.Contains(layout, anchor) {
			return fmt.Errorf("docs/feature-layout.md no longer discloses %q", anchor)
		}
	}
	return nil
}

// TestPIBSensitivityPIB147ArchiveLayoutCarriesNoProvenance owns PIB-147.
func TestPIBSensitivityPIB147ArchiveLayoutCarriesNoProvenance(t *testing.T) {
	source := pibGuardSources(t, "internal/store/intent_archive.go")["internal/store/intent_archive.go"]
	tags := pibWireTags(source)
	if err := pibValidateNoProvenanceFields(tags); err != nil {
		t.Fatalf("PIB-147: the shipped archive wire failed its own guard: %v", err)
	}
	mutated := append(append([]string(nil), tags...), "provider")
	if err := pibValidateNoProvenanceFields(mutated); err == nil {
		t.Fatal("PIB-147: the guard accepted a provider field on the archive wire")
	}
	pathA := append(append([]string(nil), tags...), "path_a")
	if err := pibValidateNoProvenanceFields(pathA); err == nil {
		t.Fatal("PIB-147: the guard accepted a Path-A field on the archive wire")
	}
}

func pibWireTags(source string) []string {
	pattern := regexp.MustCompile("json:\"([a-z0-9_]+)")
	tags := []string{}
	for _, match := range pattern.FindAllStringSubmatch(source, -1) {
		tags = append(tags, match[1])
	}
	return tags
}

func pibValidateNoProvenanceFields(tags []string) error {
	if len(tags) == 0 {
		return fmt.Errorf("the archive wire scan found no fields")
	}
	forbidden := map[string]bool{
		"author": true, "agent": true, "model": true, "provider": true,
		"endpoint": true, "path_a": true, "path_b": true,
	}
	for _, tag := range tags {
		if forbidden[tag] {
			return fmt.Errorf("the archive wire carries the provenance field %q", tag)
		}
	}
	return nil
}

// TestPIBSensitivityPIB164JournalCarriesNoWallClockOrPhase owns PIB-164.
func TestPIBSensitivityPIB164JournalCarriesNoWallClockOrPhase(t *testing.T) {
	sources := pibGuardSources(t, "internal/intentpub/types.go")
	tags := pibWireTags(sources["internal/intentpub/types.go"])
	if err := pibValidateJournalWire(tags); err != nil {
		t.Fatalf("PIB-164: the shipped journal wire failed its own guard: %v", err)
	}
	phased := append(append([]string(nil), tags...), "phase")
	if err := pibValidateJournalWire(phased); err == nil {
		t.Fatal("PIB-164: the guard accepted a phase field on the journal wire")
	}
	clocked := append(append([]string(nil), tags...), "created_at")
	if err := pibValidateJournalWire(clocked); err == nil {
		t.Fatal("PIB-164: the guard accepted a wall-clock field on the journal wire")
	}
}

func pibValidateJournalWire(tags []string) error {
	if len(tags) == 0 {
		return fmt.Errorf("the journal wire scan found no fields")
	}
	forbidden := map[string]bool{
		"phase": true, "created_at": true, "timestamp": true,
		"updated_at": true, "started_at": true, "wall_clock": true,
	}
	for _, tag := range tags {
		if forbidden[tag] {
			return fmt.Errorf("the journal wire carries the forbidden field %q", tag)
		}
	}
	return nil
}

// TestPIBSensitivityPIB175AdvisorySelectionIsTotal owns PIB-175.
func TestPIBSensitivityPIB175AdvisorySelectionIsTotal(t *testing.T) {
	catalog, reachable, relationships, err := s6AdvisoryEvidence(t)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS6Advisories(catalog, reachable, relationships); err != nil {
		t.Fatalf("PIB-175: the shipped advisory selection failed its own guard: %v", err)
	}
	unreachable := map[string]string{}
	for code, evidence := range reachable {
		unreachable[code] = evidence
	}
	if len(catalog) == 0 {
		t.Fatal("PIB-175: the shipped advisory catalog is empty")
	}
	delete(unreachable, catalog[0])
	if err := validateS6Advisories(catalog, unreachable, relationships); err == nil {
		t.Fatal("PIB-175: the same validator accepted an advisory with no reachable fixture")
	}
}

// TestPIBSensitivityPIB180RefusalStringsCarryNoPointer owns PIB-180.
//
// The row's shorthand is "contains no `docs/`, no `.md`, no `http`". Its
// normative body is §10.7: "Every refusal names only shipped commands, shipped
// flags and repo-relative paths that exist. It must not cite a PRD path, an ADR
// path, an issue URL or any `docs/` file." A bare shipped artifact name —
// `request.md` in `Restore a non-empty regular request.md and retry.` — is a
// repo-relative path that exists, not a document pointer, and rejecting it on
// the extension alone made the shipped catalog fail its own guard. The detector
// below separates the two: a `.md` token with a directory component, or one
// whose basename is not a file the product maintains, is a pointer; a bare
// shipped artifact name is not.
func TestPIBSensitivityPIB180RefusalStringsCarryNoPointer(t *testing.T) {
	emitted := pibGuardEmittedStrings()
	permitted := pibShippedArtifactBasenames(t)
	// The shipped string this row previously mis-rejected, asserted literally.
	remediation := emitted["request-unreadable/"+string(prepareModeGenerate)+"/remediation"]
	if remediation != "Restore a non-empty regular request.md and retry." {
		t.Fatalf("PIB-180: the shipped request-unreadable remediation is %q", remediation)
	}
	if err := pibValidateNoDocumentPointer(emitted, permitted); err != nil {
		t.Fatalf("PIB-180: the shipped refusal strings failed their own guard: %v", err)
	}
	pointed := pibGuardClone(emitted)
	pointed["fixture/docs"] = "See docs/feature-layout.md for the procedure."
	if err := pibValidateNoDocumentPointer(pointed, permitted); err == nil {
		t.Fatal("PIB-180: the guard accepted a docs/ pointer in a refusal string")
	}
	linked := pibGuardClone(emitted)
	linked["fixture/http"] = "Read http://tpatch.invalid/help for the procedure."
	if err := pibValidateNoDocumentPointer(linked, permitted); err == nil {
		t.Fatal("PIB-180: the guard accepted a URL in a refusal string")
	}
	prd := pibGuardClone(emitted)
	prd["fixture/prd"] = "Follow PRD-prepare-intent-bundle.md and retry."
	if err := pibValidateNoDocumentPointer(prd, permitted); err == nil {
		t.Fatal("PIB-180: the guard accepted a PRD pointer in a refusal string")
	}
	relocated := pibGuardClone(emitted)
	relocated["fixture/relative"] = "Restore ../../notes/request.md and retry."
	if err := pibValidateNoDocumentPointer(relocated, permitted); err == nil {
		t.Fatal("PIB-180: the guard accepted a pathed pointer whose basename is a shipped artifact")
	}
	// Control: naming another bare shipped artifact is legal neighbouring
	// behaviour and must not be rejected.
	neighbour := pibGuardClone(emitted)
	neighbour["fixture/artifact"] = "Restore a non-empty regular spec.md and retry."
	if err := pibValidateNoDocumentPointer(neighbour, permitted); err != nil {
		t.Fatalf("PIB-180: the guard rejected a bare shipped artifact name: %v", err)
	}
	// Control: the detector is not vacuous — an empty permitted set turns the
	// shipped catalog's own `request.md` back into an unrecognised document.
	if err := pibValidateNoDocumentPointer(emitted, map[string]bool{"spec.md": true}); err == nil {
		t.Fatal("PIB-180: the guard admitted request.md without deriving it from the product")
	}
}

// pibShippedArtifactBasenames returns the file names the product itself
// maintains inside a feature directory, derived from the shipped path mapper
// and from the shipped request capture rather than from a copied list.
func pibShippedArtifactBasenames(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, id := range []string{"analysis", "spec", "exploration", "analysis_sidecar"} {
		rel := prepareArtifactFeaturePath(id)
		if rel == "" {
			t.Fatalf("the shipped artifact path mapper has no entry for %q", id)
		}
		base := rel
		if cut := strings.LastIndex(rel, "/"); cut >= 0 {
			base = rel[cut+1:]
		}
		names[base] = true
	}
	const requestCapture = "prepareFeatureRel(slug)+\"/request.md\""
	source := pibGuardSources(t, "internal/cli/prepare_publish.go")["internal/cli/prepare_publish.go"]
	if !strings.Contains(source, requestCapture) {
		t.Fatalf("the shipped request capture %s was not found", requestCapture)
	}
	names["request.md"] = true
	return names
}

// pibDocumentToken matches a Markdown file reference together with any path it
// carries, so `docs/feature-layout.md`, `../notes/plan.md` and a bare
// `request.md` are distinguishable from one another.
var pibDocumentToken = regexp.MustCompile(`[A-Za-z0-9_./-]*\.(?:md|markdown)\b`)

func pibValidateNoDocumentPointer(texts map[string]string, permitted map[string]bool) error {
	if len(texts) == 0 {
		return fmt.Errorf("the refusal-pointer scan received no strings")
	}
	if len(permitted) == 0 {
		return fmt.Errorf("the refusal-pointer scan received no shipped artifact names")
	}
	for name, body := range texts {
		if strings.Contains(body, "docs/") {
			return fmt.Errorf("%s cites a docs/ file", name)
		}
		if strings.Contains(strings.ToLower(body), "http") {
			return fmt.Errorf("%s cites a URL", name)
		}
		for _, token := range pibDocumentToken.FindAllString(body, -1) {
			if strings.Contains(token, "/") {
				return fmt.Errorf("%s cites the document path %q", name, token)
			}
			if !permitted[token] {
				return fmt.Errorf("%s cites the document %q", name, token)
			}
		}
	}
	return nil
}

// TestPIBSensitivityPIB194ValidationSetIsExactlySix owns PIB-194.
func TestPIBSensitivityPIB194ValidationSetIsExactlySix(t *testing.T) {
	sources := pibGuardSources(t, "internal/intentpub/types.go", "internal/intentpub/transaction.go")
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	if err := pibValidateStagedValidationSet(sources, prd); err != nil {
		t.Fatalf("PIB-194: the shipped validation set failed its own guard: %v", err)
	}
	widened := pibGuardClone(sources)
	widened["internal/intentpub/transaction.go"] += "\nfunc pibHeadingCheck() bool { return true }\n"
	if err := pibValidateStagedValidationSet(widened, prd); err == nil {
		t.Fatal("PIB-194: the guard accepted a heading inspection in the validation set")
	}
	seventh := prd + "\n| V7 | topicality | reject |\n"
	if err := pibValidateStagedValidationSet(sources, seventh); err == nil {
		t.Fatal("PIB-194: the guard accepted a seventh validation")
	}
}

func pibValidateStagedValidationSet(sources map[string]string, prd string) error {
	if len(sources) == 0 {
		return fmt.Errorf("the validation-set scan received no sources")
	}
	for name, body := range sources {
		lower := strings.ToLower(body)
		for _, token := range []string{"heading", "topical", "wordcount", "word count"} {
			if strings.Contains(lower, token) {
				return fmt.Errorf("%s inspects %q, which V1…V6 do not", name, token)
			}
		}
	}
	for index := 1; index <= 6; index++ {
		if !regexp.MustCompile(fmt.Sprintf(`\bV%d\b`, index)).MatchString(prd) {
			return fmt.Errorf("the PRD no longer enumerates V%d", index)
		}
	}
	if regexp.MustCompile(`\bV7\b`).MatchString(prd) {
		return fmt.Errorf("the PRD enumerates a seventh validation")
	}
	return nil
}

// TestPIBSensitivityPIB221PlatformEnvelopeIsSourceGuarded owns PIB-221.
//
// The row's Windows clause is "mutating prepare is source-guarded to refuse".
// The guard below proves the dominance property the accepted PIB-222 body
// asserts for two named functions, over the *derived* route set instead: every
// function that acquires the workspace mutation authority must consult
// `prepareMutationAuthoritySupported` first, and the code it refuses with
// between those two points must be `prepare-unsupported-platform`. A token
// scan could not see this — the shipped source carries the string four times,
// so renaming the one occurrence that dominates a route left three behind and
// the scan passed.
func TestPIBSensitivityPIB221PlatformEnvelopeIsSourceGuarded(t *testing.T) {
	sources := pibGuardCLIProductionSources(t)
	if err := pibValidateMutatingPlatformGuard(sources); err != nil {
		t.Fatalf("PIB-221: the shipped platform guard failed its own guard: %v", err)
	}
	removed := pibGuardClone(sources)
	removed["internal/cli/prepare_publish.go"] = strings.Replace(
		removed["internal/cli/prepare_publish.go"],
		"prepare-unsupported-platform",
		"prepare-platform-advisory",
		1,
	)
	if removed["internal/cli/prepare_publish.go"] == sources["internal/cli/prepare_publish.go"] {
		t.Fatal("PIB-221: the platform-guard mutation anchor is missing")
	}
	if err := pibValidateMutatingPlatformGuard(removed); err == nil {
		t.Fatal("PIB-221: the guard accepted a mutating path without its platform refusal")
	}
	added := pibGuardClone(sources)
	added["internal/cli/prepare_publish.go"] += "\nfunc pibUnguardedMutatingRoute(repoRoot string) error {\n" +
		"\tauthority, err := prepareAcquireAuthority(repoRoot)\n" +
		"\tif err != nil {\n\t\treturn err\n\t}\n\treturn authority.Release()\n}\n"
	if err := pibValidateMutatingPlatformGuard(added); err == nil {
		t.Fatal("PIB-221: the guard accepted a new mutating route with no platform refusal")
	}
	reached := pibGuardClone(sources)
	reached["internal/cli/prepare.go"] = strings.Replace(
		reached["internal/cli/prepare.go"],
		"	report := intent.Inspect(intent.NewRootOps(root), slug, scratch)",
		"	_, _ = prepareAcquireAuthority(repoRoot)\n	report := intent.Inspect(intent.NewRootOps(root), slug, scratch)",
		1,
	)
	if reached["internal/cli/prepare.go"] == sources["internal/cli/prepare.go"] {
		t.Fatal("PIB-221: the read-only mutation anchor is missing")
	}
	if err := pibValidateMutatingPlatformGuard(reached); err == nil {
		t.Fatal("PIB-221: the guard accepted an authority acquisition on the accepted read-only path")
	}
	// Control: a new function that consults the support constant but never
	// acquires the authority is legal neighbouring behaviour.
	neighbour := pibGuardClone(sources)
	neighbour["internal/cli/prepare_publish.go"] += "\nfunc pibPlatformProbe() bool {\n" +
		"\treturn prepareMutationAuthoritySupported()\n}\n"
	if err := pibValidateMutatingPlatformGuard(neighbour); err != nil {
		t.Fatalf("PIB-221: the guard rejected a non-mutating platform probe: %v", err)
	}
}

// pibValidateMutatingPlatformGuard derives the mutating route set from the
// shipped source — every function that calls `prepareAcquireAuthority` — and
// requires each one to be dominated by the platform refusal.
func pibValidateMutatingPlatformGuard(sources map[string]string) error {
	publish := sources["internal/cli/prepare_publish.go"]
	command := sources["internal/cli/prepare.go"]
	if publish == "" || command == "" {
		return fmt.Errorf("the platform-guard scan received no mutating source")
	}
	fileset := token.NewFileSet()
	parsed, err := parser.ParseFile(fileset, "prepare_publish.go", publish, 0)
	if err != nil {
		return fmt.Errorf("the mutating source does not parse: %v", err)
	}
	routes := 0
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		acquisitions := pibCallSites(function.Body, "prepareAcquireAuthority")
		if len(acquisitions) == 0 {
			continue
		}
		routes++
		gates := pibCallSites(function.Body, "prepareMutationAuthoritySupported")
		if len(gates) == 0 {
			return fmt.Errorf(
				"%s acquires the mutation authority without consulting the support constant",
				function.Name.Name)
		}
		gateOffset := fileset.Position(gates[0].Pos()).Offset
		acquireOffset := fileset.Position(acquisitions[0].Pos()).Offset
		if gateOffset > acquireOffset {
			return fmt.Errorf(
				"%s consults the support constant after acquiring the mutation authority",
				function.Name.Name)
		}
		if !strings.Contains(publish[gateOffset:acquireOffset], "\"prepare-unsupported-platform\"") {
			return fmt.Errorf(
				"%s does not refuse with prepare-unsupported-platform before acquiring the authority",
				function.Name.Name)
		}
	}
	if routes < 2 {
		return fmt.Errorf(
			"the derived mutating route set has %d members, want the publish and abandon routes", routes)
	}
	checkParsed, err := parser.ParseFile(token.NewFileSet(), "prepare.go", command, 0)
	if err != nil {
		return fmt.Errorf("the accepted read-only source does not parse: %v", err)
	}
	for _, declaration := range checkParsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil || function.Name.Name != "runPrepareCheck" {
			continue
		}
		if len(pibCallSites(function.Body, "prepareAcquireAuthority")) != 0 {
			return fmt.Errorf("the accepted read-only check acquires the mutation authority")
		}
	}
	if !strings.Contains(publish, "prepareMutationAuthoritySupported") {
		return fmt.Errorf("the mutating path no longer consults the authority-support constant")
	}
	return nil
}

// TestPIBSensitivityPIB295CrashTableHasNoPermanentRefusal owns PIB-295.
//
// The authoritative domain is the §7.10 table, not the document. Scanning the
// whole PRD for "refuses forever" labelled two real sentences as crash-phase
// outcomes that are not: §R29's risk statement (which describes the failure the
// design avoids) and this row's own acceptance-matrix line (which quotes the
// forbidden phrase in order to forbid it). The parser below reads the §7.10
// rows, requires the frozen phase domain, and then distinguishes a named
// terminal route — CP9's `recovery-divergent` refusal that names §6.6, CP14's
// exit-6 verification failure, CP13's named purge repair — from an outcome with
// no route out at all.
func TestPIBSensitivityPIB295CrashTableHasNoPermanentRefusal(t *testing.T) {
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	if err := pibValidateCrashPointTable(prd); err != nil {
		t.Fatalf("PIB-295: the shipped CP table failed its own guard: %v", err)
	}
	stuck := strings.Replace(prd,
		"\n| CP14 |",
		"\n| CP99 | injected | nothing | refuses forever |\n| CP14 |",
		1,
	)
	if stuck == prd {
		t.Fatal("PIB-295: the permanent-refusal row anchor is missing")
	}
	if err := pibValidateCrashPointTable(stuck); err == nil {
		t.Fatal("PIB-295: the guard accepted a CP row whose outcome is a permanent refusal")
	}
	blocked := strings.Replace(prd,
		"name them and name §6.6 |",
		"the slug stays blocked and no command clears it |",
		1,
	)
	if blocked == prd {
		t.Fatal("PIB-295: the CP9 outcome anchor is missing")
	}
	if err := pibValidateCrashPointTable(blocked); err == nil {
		t.Fatal("PIB-295: the guard accepted a real CP row rewritten into permanent blockage")
	}
	truncated := strings.Replace(prd, "| CP10 |", "| CPXX |", 1)
	if truncated == prd {
		t.Fatal("PIB-295: the phase-domain mutation anchor is missing")
	}
	if err := pibValidateCrashPointTable(truncated); err == nil {
		t.Fatal("PIB-295: the guard accepted a CP table missing a numbered phase")
	}
	silent := strings.Replace(prd,
		"| CP0 | before the lock exists | nothing | nothing to do |",
		"| CP0 | before the lock exists | nothing | the operator is on their own |",
		1,
	)
	if silent == prd {
		t.Fatal("PIB-295: the CP0 outcome anchor is missing")
	}
	if err := pibValidateCrashPointTable(silent); err == nil {
		t.Fatal("PIB-295: the guard accepted a CP row whose outcome names no route")
	}
	// Control: the forbidden phrase outside §7.10 — which is exactly where the
	// shipped §R29 risk statement uses it — must still be accepted.
	elsewhere := prd + "\n| R99 | every ordinary mutation refuses forever. | mitigated |\n"
	if err := pibValidateCrashPointTable(elsewhere); err != nil {
		t.Fatalf("PIB-295: the guard rejected the phrase outside the crash table: %v", err)
	}
}

// pibCrashPhaseDomain is the frozen §7.10 phase set, in table order.
func pibCrashPhaseDomain() []string {
	return []string{
		"CP0", "CP1", "CP2", "CP3", "CP4", "CP5", "CP6", "CP7",
		"CP8", "CP9", "CP10", "CP11", "CP12", "CP12a", "CP13", "CP14",
	}
}

// pibCrashRouteMarkers name the ways a §7.10 outcome discloses its route out:
// it completes, it recovers, it proceeds, or it refuses while naming a section
// or a command the operator can run.
var pibCrashRouteMarkers = []string{
	"nothing to do", "nothing to recover", "nothing to restore", "nothing is terminal",
	"proceed", "recovered", "reconverged", "exit", "§", "purge", "tpatch ",
}

// pibPermanentBlockagePhrases are outcomes with no route out. A refusal that
// names its route — CP9, CP12, CP13, CP14 — is not one of these.
var pibPermanentBlockagePhrases = []string{
	"refuses forever", "refuse forever", "refused forever", "never recovers",
	"permanently blocked", "stays blocked", "remains blocked", "unrecoverable",
	"cannot be recovered", "no route out", "no way out", "blocked forever",
}

// pibCrashPointSection returns the §7.10 body, bounded by the next heading.
func pibCrashPointSection(prd string) (string, error) {
	start := strings.Index(prd, "\n### 7.10 ")
	if start < 0 {
		return "", fmt.Errorf("§7.10 is not in the document")
	}
	section := prd[start+1:]
	if end := strings.Index(section[1:], "\n### "); end >= 0 {
		section = section[:end+1]
	}
	return section, nil
}

func pibValidateCrashPointTable(prd string) error {
	section, err := pibCrashPointSection(prd)
	if err != nil {
		return err
	}
	flat := regexp.MustCompile(`\s+`).ReplaceAllString(section, " ")
	if !strings.Contains(flat,
		"there is no stale-lock analysis and no row in which the slug stays blocked.") {
		return fmt.Errorf("§7.10 no longer states that no row leaves the slug blocked")
	}
	observed := []string{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| CP") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), " | ")
		if len(cells) != 4 {
			return fmt.Errorf("the CP row %q has %d cells, the table fixes four",
				strings.TrimSpace(cells[0]), len(cells))
		}
		phase := strings.TrimSpace(cells[0])
		outcome := strings.ToLower(strings.TrimSpace(cells[3]))
		observed = append(observed, phase)
		if outcome == "" {
			return fmt.Errorf("%s names no recovery outcome", phase)
		}
		named := false
		for _, marker := range pibCrashRouteMarkers {
			if strings.Contains(outcome, marker) {
				named = true
				break
			}
		}
		for _, blockage := range pibPermanentBlockagePhrases {
			if strings.Contains(outcome, blockage) {
				return fmt.Errorf("%s's outcome is a permanent refusal: %q", phase, blockage)
			}
		}
		if !named {
			return fmt.Errorf("%s's outcome names no route out: %q", phase, outcome)
		}
	}
	frozen := pibCrashPhaseDomain()
	if len(observed) != len(frozen) {
		return fmt.Errorf("the CP table has %d rows, §7.10 fixes %d", len(observed), len(frozen))
	}
	for index, phase := range frozen {
		if observed[index] != phase {
			return fmt.Errorf("the CP table names %q where §7.10 fixes %q", observed[index], phase)
		}
	}
	return nil
}

// TestPIBSensitivityPIB305JournalBindListIsComplete owns PIB-305.
func TestPIBSensitivityPIB305JournalBindListIsComplete(t *testing.T) {
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	if err := pibValidateJournalBindList(prd); err != nil {
		t.Fatalf("PIB-305: the shipped J1–J10 bind list failed its own guard: %v", err)
	}
	removed := strings.Replace(prd, "J7", "JX", -1)
	if removed == prd {
		t.Fatal("PIB-305: the bind-list mutation anchor is missing")
	}
	if err := pibValidateJournalBindList(removed); err == nil {
		t.Fatal("PIB-305: the guard accepted a bind list with one bind removed")
	}
}

func pibValidateJournalBindList(prd string) error {
	for index := 1; index <= 10; index++ {
		if !regexp.MustCompile(fmt.Sprintf(`\bJ%d\b`, index)).MatchString(prd) {
			return fmt.Errorf("the bind list no longer names J%d", index)
		}
	}
	return nil
}

// TestPIBSensitivityPIB309RenameScanBites owns PIB-309.
func TestPIBSensitivityPIB309RenameScanBites(t *testing.T) {
	sources := pibGuardSources(t, "internal/intentpub/transaction.go", "internal/intentpub/ops.go")
	if err := pibValidateNoRawRename(sources); err != nil {
		t.Fatalf("PIB-309: the shipped publication path failed its own guard: %v", err)
	}
	injected := pibGuardClone(sources)
	injected["internal/intentpub/transaction.go"] += "\nfunc pibRawRename() error { return os.Rename(\"a\", \"b\") }\n"
	if err := pibValidateNoRawRename(injected); err == nil {
		t.Fatal("PIB-309: the guard accepted one injected os.Rename call")
	}
}

func pibValidateNoRawRename(sources map[string]string) error {
	if len(sources) == 0 {
		return fmt.Errorf("the rename scan received no sources")
	}
	for name, body := range sources {
		if strings.Contains(body, "os.Rename(") {
			return fmt.Errorf("%s renames outside the rooted writer", name)
		}
	}
	return nil
}

// TestPIBSensitivityPIB356AvailabilityClaimsAreExact owns PIB-356.
//
// The claim table is *derived* by calling the shipped mapper over the shipped
// disposition domain, which is the single source `list`, `doctor` D9 and the
// purge report all render through. Scanning the file that holds the mapper for
// one sentence could not bite: the sentence occurs twice, so replacing one
// occurrence left the other and the scan still passed. The row's two named
// wrong inputs — a live hash reported as absent, and `--orphans --yes` offered
// for a live blob — are both applied to that derived table.
func TestPIBSensitivityPIB356AvailabilityClaimsAreExact(t *testing.T) {
	claims := pibAvailabilityClaims()
	repairs := pibAvailabilityRepairs("demo-feature")
	if err := pibValidateAvailabilityClaims(claims, repairs); err != nil {
		t.Fatalf("PIB-356: the shipped availability claims failed their own guard: %v", err)
	}
	if claims["dangling"] != pibAvailabilitySentence {
		t.Fatalf("PIB-356: the dangling claim is %q, want the exact availability sentence", claims["dangling"])
	}
	unavailable := map[string]string{}
	for token, claim := range claims {
		unavailable[token] = claim
	}
	unavailable["present"] = "unavailable"
	if err := pibValidateAvailabilityClaims(unavailable, repairs); err == nil {
		t.Fatal("PIB-356: the guard accepted an unavailability claim for a retained hash")
	}
	pending := map[string]string{}
	for token, claim := range claims {
		pending[token] = claim
	}
	pending["pending-remove"] = pibAvailabilitySentence
	if err := pibValidateAvailabilityClaims(pending, repairs); err == nil {
		t.Fatal("PIB-356: the guard accepted an unavailability claim for a removal-pending hash")
	}
	weakened := map[string]string{}
	for token, claim := range claims {
		weakened[token] = claim
	}
	weakened["dangling"] = "unavailable"
	if err := pibValidateAvailabilityClaims(weakened, repairs); err == nil {
		t.Fatal("PIB-356: the guard accepted a dangling claim that drops the exact sentence")
	}
	orphaned := map[string]string{}
	for token, repair := range repairs {
		orphaned[token] = repair
	}
	orphaned["present"] = "tpatch feature intent-archive purge demo-feature --orphans --yes"
	if err := pibValidateAvailabilityClaims(claims, orphaned); err == nil {
		t.Fatal("PIB-356: the guard accepted --orphans --yes as the repair for a live blob")
	}
	// Control: rewording a live hash's claim while it still reports the bytes
	// as reachable is legal neighbouring behaviour.
	reworded := map[string]string{}
	for token, claim := range claims {
		reworded[token] = claim
	}
	reworded["present"] = "recoverable from the present archive blob in this generation"
	if err := pibValidateAvailabilityClaims(reworded, repairs); err != nil {
		t.Fatalf("PIB-356: the guard rejected a reworded availability claim: %v", err)
	}
}

const pibAvailabilitySentence = "not recoverable until identical content is archived again"

// pibArchiveDispositions is the closed shipped disposition domain.
func pibArchiveDispositions() []store.IntentArchiveDisposition {
	return []store.IntentArchiveDisposition{
		store.IntentArchiveDispositionHealthyRetained,
		store.IntentArchiveDispositionHealthyPurged,
		store.IntentArchiveDispositionPendingRemove,
		store.IntentArchiveDispositionPendingFinalize,
		store.IntentArchiveDispositionDanglingReference,
		store.IntentArchiveDispositionResidue,
		store.IntentArchiveDispositionMixedReference,
		store.IntentArchiveDispositionCorruptObject,
	}
}

// pibAvailabilityClaims calls the shipped availability mapper over the shipped
// domain, so the table under test is the product's own.
func pibAvailabilityClaims() map[string]string {
	claims := map[string]string{}
	for _, disposition := range pibArchiveDispositions() {
		token := intentArchiveStorageToken(disposition)
		claims[token] = intentArchiveAvailability(token)
	}
	return claims
}

// pibAvailabilityRepairs calls the shipped `list` repair selector over the same
// domain and returns the command text each disposition routes to.
func pibAvailabilityRepairs(slug string) map[string]string {
	repairs := map[string]string{}
	for _, disposition := range pibArchiveDispositions() {
		presentation := intentArchiveListRepair(
			slug,
			store.IntentArchiveInspection{},
			store.IntentArchiveHashReport{Hash: "0"},
			store.IntentArchiveReferenceReport{Disposition: disposition},
		)
		repairs[intentArchiveStorageToken(disposition)] =
			presentation.Repair + "\n" + presentation.Retry
	}
	return repairs
}

// pibLiveStorageTokens are the dispositions under which a retained or
// removal-pending reference to the hash still exists somewhere.
var pibLiveStorageTokens = []string{
	"present", "pending-remove", "pending-finalize", "mixed-reference", "corrupt", "orphan",
}

// pibAbsentStorageTokens are the dispositions under which the bytes really are
// gone, and are the only ones entitled to the exact availability sentence.
var pibAbsentStorageTokens = []string{"purged", "dangling"}

func pibValidateAvailabilityClaims(claims map[string]string, repairs map[string]string) error {
	if len(claims) == 0 || len(repairs) == 0 {
		return fmt.Errorf("the availability scan received an empty claim table")
	}
	for _, token := range pibLiveStorageTokens {
		claim, covered := claims[token]
		if !covered {
			return fmt.Errorf("the claim table does not cover the live disposition %q", token)
		}
		lower := strings.ToLower(claim)
		if strings.Contains(claim, pibAvailabilitySentence) {
			return fmt.Errorf("%q reports a live hash as never recoverable: %q", token, claim)
		}
		for _, absent := range []string{"unavailable", "is absent", "is gone", "no longer exists"} {
			if strings.Contains(lower, absent) {
				return fmt.Errorf("%q reports a live hash as absent: %q", token, claim)
			}
		}
	}
	for _, token := range pibAbsentStorageTokens {
		claim, covered := claims[token]
		if !covered {
			return fmt.Errorf("the claim table does not cover the absent disposition %q", token)
		}
		if claim != pibAvailabilitySentence {
			return fmt.Errorf("%q does not carry the exact availability sentence: %q", token, claim)
		}
	}
	for token, repair := range repairs {
		if !strings.Contains(repair, "--orphans --yes") {
			continue
		}
		if token != "orphan" {
			return fmt.Errorf("the %q repair names --orphans --yes for a live blob: %q",
				token, strings.TrimSpace(repair))
		}
	}
	if !strings.Contains(repairs["orphan"], "--orphans --yes") {
		return fmt.Errorf("the unreferenced-residue repair no longer names its own selector")
	}
	return nil
}

// TestPIBSensitivityPIB361CommittedBlobDisclosure owns PIB-361.
func TestPIBSensitivityPIB361CommittedBlobDisclosure(t *testing.T) {
	texts := map[string]string{
		"internal/cli/feature_intent_archive.go": pibGuardSources(
			t, "internal/cli/feature_intent_archive.go",
		)["internal/cli/feature_intent_archive.go"],
		"docs/feature-layout.md": s6RepoFile(t, "docs/feature-layout.md"),
	}
	if err := pibValidateCommittedBlobDisclosure(texts); err != nil {
		t.Fatalf("PIB-361: the shipped disclosure failed its own guard: %v", err)
	}
	silent := pibGuardClone(texts)
	silent["docs/feature-layout.md"] = strings.ReplaceAll(
		silent["docs/feature-layout.md"], "Git history", "the working tree",
	)
	if err := pibValidateCommittedBlobDisclosure(silent); err == nil {
		t.Fatal("PIB-361: the guard accepted documentation that drops the Git-history disclosure")
	}
	overreaching := pibGuardClone(texts)
	overreaching["internal/cli/feature_intent_archive.go"] += "\n// tpatch rewrites Git history on request.\n"
	if err := pibValidateCommittedBlobDisclosure(overreaching); err == nil {
		t.Fatal("PIB-361: the guard accepted a history-rewriting claim")
	}
}

func pibValidateCommittedBlobDisclosure(texts map[string]string) error {
	if len(texts) != 2 {
		return fmt.Errorf("the disclosure scan covers %d texts, want 2", len(texts))
	}
	for name, body := range texts {
		if !strings.Contains(body, "Git history") {
			return fmt.Errorf("%s does not state that a committed blob remains in Git history", name)
		}
		if strings.Contains(body, "tpatch rewrites Git history") {
			return fmt.Errorf("%s claims tpatch rewrites Git history", name)
		}
	}
	return nil
}

// TestPIBSensitivityPIB362ExitSixJournalPopulation owns PIB-362.
func TestPIBSensitivityPIB362ExitSixJournalPopulation(t *testing.T) {
	population := []string{
		"undo-cas-mismatch", "recovery-divergent", "journal-corrupt",
		"journal-version-mismatch", "journal-foreign", "journal-path-escape",
		"journal-forged", "post-publication-divergence",
		"workspace-root-replaced-after-publication",
	}
	if err := pibValidateExitSixJournalPopulation(population); err != nil {
		t.Fatalf("PIB-362: the shipped exit-6 journal population failed its own guard: %v", err)
	}
	widened := append(append([]string(nil), population...), "archive-purge-evidence-divergent")
	if err := pibValidateExitSixJournalPopulation(widened); err == nil {
		t.Fatal("PIB-362: the guard accepted archive-purge-evidence-divergent in the journal population")
	}
	narrowed := append([]string(nil), population[:len(population)-1]...)
	if err := pibValidateExitSixJournalPopulation(narrowed); err == nil {
		t.Fatal("PIB-362: the guard accepted an eight-code population")
	}
}

func pibValidateExitSixJournalPopulation(codes []string) error {
	if len(codes) != 9 {
		return fmt.Errorf("the exit-6 journal population has %d codes, §10.4 fixes nine", len(codes))
	}
	for _, code := range codes {
		if code == "archive-purge-evidence-divergent" {
			return fmt.Errorf("%s is not part of the journal/publication population", code)
		}
		_, remediation := prepareRefusalText(code, prepareModeGenerate, "demo-feature", "")
		if !strings.Contains(remediation, "--abandon-transaction") {
			return fmt.Errorf("%s does not route to the abandon mode: %q", code, remediation)
		}
	}
	archiveMessage, _ := prepareRefusalText(
		"archive-purge-evidence-divergent", prepareModeGenerate, "demo-feature", "",
	)
	journalMessage, _ := prepareRefusalText(codes[0], prepareModeGenerate, "demo-feature", "")
	if archiveMessage == journalMessage {
		return fmt.Errorf("the archive divergence code shares the journal population's message")
	}
	return nil
}

// TestPIBSensitivityPIB376NoRetryPersistenceOutsideTemp owns PIB-376.
func TestPIBSensitivityPIB376NoRetryPersistenceOutsideTemp(t *testing.T) {
	sources := pibGuardSources(t,
		"internal/cli/prepare_publish.go",
		"internal/workflow/generate_spec.go",
		"internal/workflow/generate_analysis.go",
		"internal/workflow/generate_exploration.go",
	)
	if err := pibValidateNoRetryPersistence(sources); err != nil {
		t.Fatalf("PIB-376: the shipped generation path failed its own guard: %v", err)
	}
	stored := pibGuardClone(sources)
	stored["internal/workflow/generate_spec.go"] += "\ntype pibRetryStore struct{ attempts []string }\n"
	if err := pibValidateNoRetryPersistence(stored); err == nil {
		t.Fatal("PIB-376: the guard accepted a retry store")
	}
	sunk := pibGuardClone(sources)
	sunk["internal/cli/prepare_publish.go"] += "\n// transcriptSink writes raw attempts.\n"
	if err := pibValidateNoRetryPersistence(sunk); err == nil {
		t.Fatal("PIB-376: the guard accepted a raw-attempt transcript sink")
	}
}

func pibValidateNoRetryPersistence(sources map[string]string) error {
	if len(sources) == 0 {
		return fmt.Errorf("the retry-persistence scan received no sources")
	}
	forbidden := []string{"retrystore", "rawattempt", "transcriptsink", "attemptlog"}
	for name, body := range sources {
		lower := strings.ToLower(body)
		for _, token := range forbidden {
			if strings.Contains(lower, token) {
				return fmt.Errorf("%s persists provider attempts through %q", name, token)
			}
		}
	}
	return nil
}

// TestPIBSensitivityPIB378NoAuthorshipEvidenceClaim owns PIB-378.
func TestPIBSensitivityPIB378NoAuthorshipEvidenceClaim(t *testing.T) {
	corpus := pibGuardClaimCorpus(t)
	if err := pibValidateNoAuthorshipClaim(corpus); err != nil {
		t.Fatalf("PIB-378: the shipped strings and docs failed their own guard: %v", err)
	}
	notes := pibGuardClone(corpus)
	notes["SPEC.md"] += "\nstatus.json notes record who authored each artifact.\n"
	if err := pibValidateNoAuthorshipClaim(notes); err == nil {
		t.Fatal("PIB-378: the guard accepted a notes-as-authorship-evidence claim")
	}
	archive := pibGuardClone(corpus)
	archive["SPEC.md"] += "\nThe intent archive is evidence of who authored an artifact.\n"
	if err := pibValidateNoAuthorshipClaim(archive); err == nil {
		t.Fatal("PIB-378: the guard accepted an archive-as-authorship-evidence claim")
	}
}

func pibValidateNoAuthorshipClaim(texts map[string]string) error {
	if len(texts) == 0 {
		return fmt.Errorf("the authorship scan received no text")
	}
	for name, body := range texts {
		lower := strings.ToLower(body)
		for _, claim := range []string{
			"who authored", "record who authored", "evidence of who",
		} {
			if strings.Contains(lower, claim) {
				return fmt.Errorf("%s claims %q", name, claim)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Direct per-row S7 sensitivity targets whose category guard body carries the
// wrong-input fixtures but no mutation seam the registry can bind to.
// ---------------------------------------------------------------------------

// TestPIBSensitivityPIB546RecoveryOrderingDirect owns PIB-546's wrong input.
func TestPIBSensitivityPIB546RecoveryOrderingDirect(t *testing.T) {
	sources := s7AVRepoSources(t,
		s7AVStoreArchiveSource, s7AVCLIArchiveSource, s7AVCLIPrepareSource,
	)
	if err := s7AVValidateRecoveryOrdering(sources); err != nil {
		t.Fatalf("PIB-546: the shipped control flow failed its own guard: %v", err)
	}
	mutated := map[string]string{}
	for name, body := range sources {
		mutated[name] = body
	}
	mutated[s7AVCLIPrepareSource] = strings.Replace(
		sources[s7AVCLIPrepareSource],
		"\tif len(pending) != 0 {\n\t\tretry := preparePendingPurgeCommand(slug, pending)\n",
		"\tif len(pending) != 0 {\n"+
			"\t\tif _, recoverErr := store.RecoverPendingPurge(archiveStorage, slug); recoverErr != nil {\n"+
			"\t\t\treport = prepareStoreArchiveFailure(report, recoverErr, true)\n"+
			"\t\t\t_ = release()\n"+
			"\t\t\treturn emitPreparePublishReport(cmd, report, prepareArchiveExit(recoverErr, 3))\n"+
			"\t\t}\n"+
			"\t\tretry := preparePendingPurgeCommand(slug, pending)\n",
		1,
	)
	if mutated[s7AVCLIPrepareSource] == sources[s7AVCLIPrepareSource] {
		t.Fatal("PIB-546: the ordering mutation anchor is missing")
	}
	if err := s7AVValidateRecoveryOrdering(mutated); err == nil {
		t.Fatal("PIB-546: the same validator accepted a mutating prepare granted the recovery exception")
	}
}

// TestPIBSensitivityPIB549AdmissionPredicateDirect owns PIB-549's wrong input.
func TestPIBSensitivityPIB549AdmissionPredicateDirect(t *testing.T) {
	sources := s7AVRepoSources(t, s7AVStoreArchiveSource)
	if err := s7AVValidateAdmissionPredicate(sources); err != nil {
		t.Fatalf("PIB-549: the shipped admission predicate failed its own guard: %v", err)
	}
	mutated := map[string]string{}
	for name, body := range sources {
		mutated[name] = body
	}
	mutated[s7AVStoreArchiveSource] = strings.Replace(
		sources[s7AVStoreArchiveSource],
		"if report != nil && equalStringSets(selected, report.Hashes) {",
		"if report != nil && len(selected) != 0 {",
		1,
	)
	if mutated[s7AVStoreArchiveSource] == sources[s7AVStoreArchiveSource] {
		t.Fatal("PIB-549: the admission mutation anchor is missing")
	}
	if err := s7AVValidateAdmissionPredicate(mutated); err == nil {
		t.Fatal("PIB-549: the same validator accepted partial class coverage")
	}
}

// TestPIBSensitivityPIB508ExitSixRouteMapDirect owns PIB-508's wrong input.
// The shipped `TestS7ARExitSixRouteGuard/PIB-508` body mutates through a
// helper call, so it carries no seam the registry can bind to; this body drives
// the same validator over the same document with an explicit substitution.
func TestPIBSensitivityPIB508ExitSixRouteMapDirect(t *testing.T) {
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	if err := validateS7ARExitSixRoutes(prd); err != nil {
		t.Fatalf("PIB-508: the shipped exit-6 route map failed its own guard: %v", err)
	}
	misdefined := strings.ReplaceAll(prd, "§9.7.2", "§6.6")
	if misdefined == prd {
		t.Fatal("PIB-508: the route-definition mutation anchor is missing")
	}
	if err := validateS7ARExitSixRoutes(misdefined); err == nil {
		t.Fatal("PIB-508: the same validator accepted the archive code defined outside §9.7.2")
	}
	unrouted := strings.ReplaceAll(
		prd, "`tpatch prepare <slug> --abandon-transaction`", "`tpatch prepare <slug>`",
	)
	if unrouted == prd {
		t.Fatal("PIB-508: the abandon-route mutation anchor is missing")
	}
	if err := validateS7ARExitSixRoutes(unrouted); err == nil {
		t.Fatal("PIB-508: the same validator accepted a journal population with no abandon route")
	}
}

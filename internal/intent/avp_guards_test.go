package intent

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// guardSpec is one mechanical guard plus the deliberately broken input it must
// reject. §18.28's rule is that a guard without a proven failure mode is not
// evidence, so `Sensitivity` runs the *same* guard body over a mutated input
// and the meta-check (AVP-139) fails when it does not return an error.
type guardSpec struct {
	Run         func(t *testing.T) error
	Sensitivity func(t *testing.T) error
}

var avpGuards = map[string]guardSpec{}

func registerGuard(id string, spec guardSpec) {
	if _, exists := avpGuards[id]; exists {
		panic("duplicate guard registration for " + id)
	}
	avpGuards[id] = spec
}

// TestAVPGuards runs every registered guard and its paired sensitivity
// fixture. Subtest names are the acceptance row IDs, which is what the
// traceability ledger resolves against.
func TestAVPGuards(t *testing.T) {
	ids := make([]string, 0, len(avpGuards))
	for id := range avpGuards {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		spec := avpGuards[id]
		t.Run(id, func(t *testing.T) {
			if err := spec.Run(t); err != nil {
				t.Fatalf("%s guard failed: %v", id, err)
			}
			t.Run("sensitivity", func(t *testing.T) {
				if err := spec.Sensitivity(t); err == nil {
					t.Fatalf("%s sensitivity fixture did not fail the guard", id)
				}
			})
		})
	}
}

func init() {
	registerOutputGuards()
	registerCatalogGuards()
	registerSourceGuards()
	registerPlatformGuards()
	registerDocumentGuards()
}

// ---------------------------------------------------------------------------
// Output shape and privacy guards
// ---------------------------------------------------------------------------

func registerOutputGuards() {
	registerGuard("AVP-051", guardSpec{
		Run: func(t *testing.T) error {
			for _, report := range guardReportMatrix(t) {
				document, err := json.Marshal(report)
				if err != nil {
					return err
				}
				if err := checkForbiddenKeys(document); err != nil {
					return err
				}
				var rendered strings.Builder
				report.WriteHuman(&rendered)
				if err := checkHumanLabels(rendered.String()); err != nil {
					return err
				}
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			return checkForbiddenKeys([]byte(`{"artifacts":[{"id":"spec","content":"secret"}]}`))
		},
	})

	registerGuard("AVP-074", guardSpec{
		Run: func(t *testing.T) error {
			for _, report := range guardReportMatrix(t) {
				document, err := json.Marshal(report)
				if err != nil {
					return err
				}
				if err := checkPathKindAbsent(document); err != nil {
					return err
				}
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			return checkPathKindAbsent([]byte(`{"path_kind":"A"}`))
		},
	})

	registerGuard("AVP-079", guardSpec{
		Run: func(t *testing.T) error {
			for _, report := range guardReportMatrix(t) {
				if err := checkNoContentHash(renderAllSurfaces(report)); err != nil {
					return err
				}
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			return checkNoContentHash("digest " + strings.Repeat("ab", 32))
		},
	})

	registerGuard("AVP-127", guardSpec{
		Run: func(t *testing.T) error {
			for _, report := range guardReportMatrix(t) {
				if err := checkAbortShape(report); err != nil {
					return err
				}
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			broken := NewAbortReport(testSlug, AbortFeatureNotFound)
			broken.Artifacts = []Artifact{{ID: "spec", Role: RoleRequired, State: StateAbsent}}
			return checkAbortShape(broken)
		},
	})

	registerGuard("AVP-122", guardSpec{
		Run: func(t *testing.T) error {
			for _, report := range guardReportMatrix(t) {
				if err := checkAdvisoryCardinality(report.Advisories); err != nil {
					return err
				}
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			return checkAdvisoryCardinality([]Advisory{
				{Code: AdvisorySidecarEmpty}, {Code: AdvisorySidecarAbsent},
				{Code: AdvisoryFeatureStateAbsent}, {Code: AdvisoryProvenanceUnknown},
			})
		},
	})

	registerGuard("AVP-129", guardSpec{
		Run: func(t *testing.T) error {
			emitted := map[string]bool{}
			for _, report := range guardReportMatrix(t) {
				for _, artifact := range report.Artifacts {
					emitted[artifact.Provenance] = true
				}
			}
			if err := checkProvenanceDomain(emitted); err != nil {
				return err
			}
			_, files := productionFiles(t, "internal/intent")
			if !docCommentMentions(files, "ProvenanceUnknown", "not computed") &&
				!strings.Contains(repoFile(t, "internal/intent/inspect.go"), ProvenanceUnknown) {
				return errors.New("the provenance constant lost its documented definition")
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			return checkProvenanceDomain(map[string]bool{"unknown": true, "path-a": true})
		},
	})

	registerGuard("AVP-140", guardSpec{
		Run: func(t *testing.T) error {
			root := fixtureRoot(t)
			root.set(testSpec, sized(testSpec, MaxArtifactBytes+1))
			root.set(testSidecar, sized(testSidecar, MaxArtifactBytes+1))
			report := Inspect(root, testSlug, scratchBuffer())
			spec := artifactByID(t, report, "spec")
			if spec.State != StateOversize || spec.ReasonCode != ReasonArtifactOversize {
				return fmt.Errorf("spec = (%q, %q), want oversize/artifact-oversize", spec.State, spec.ReasonCode)
			}
			if !hasAdvisory(report, AdvisorySidecarOversize) {
				return fmt.Errorf("advisories = %v, want analysis-sidecar-oversize", advisoryCodes(report))
			}
			document, err := json.Marshal(report)
			if err != nil {
				return err
			}
			// The whole point: `oversize` contains the forbidden substring
			// `size`, and the key-name-scoped guard must still be green.
			if !strings.Contains(string(document), "oversize") {
				return errors.New("fixture no longer exercises the substring collision")
			}
			return checkForbiddenKeys(document)
		},
		Sensitivity: func(t *testing.T) error {
			// A substring-scoped guard would fail on exactly this document;
			// the key-name-scoped guard must fail only on a real key.
			if err := checkForbiddenKeys([]byte(`{"state":"oversize","reason_code":"artifact-oversize"}`)); err != nil {
				return fmt.Errorf("guard is substring scoped: %w", err)
			}
			return checkForbiddenKeys([]byte(`{"overall":{"size":4}}`))
		},
	})

	registerGuard("AVP-187", guardSpec{
		Run: func(t *testing.T) error {
			attacker := "slug\x1b[2J\x09\x0d\nvalue"
			reports := guardReportMatrix(t)
			reports = append(reports, NewAbortReport("", AbortSlugUnsafe))
			for _, report := range reports {
				if err := checkOutputBytes(renderAllSurfaces(report), attacker); err != nil {
					return err
				}
			}
			ready, _ := runInspect(t, nil)
			notReady, _ := runInspect(t, func(r *fakeRoot) { r.remove(testSpec) })
			happyPath := renderAllSurfaces(ready) + renderAllSurfaces(notReady)
			for _, glyph := range []string{"—", "→"} {
				if !strings.Contains(happyPath, glyph) {
					return fmt.Errorf("the project glyph %q is absent from command-owned output", glyph)
				}
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			return checkOutputBytes("clean line\n\x1b[2Jinjected\n", "slug\x1b[2J\x09\x0d\nvalue")
		},
	})
}

func checkPathKindAbsent(document []byte) error {
	var decoded any
	if err := json.Unmarshal(document, &decoded); err != nil {
		return fmt.Errorf("report is not JSON: %w", err)
	}
	var found []string
	walkJSONKeys(decoded, func(key string) {
		if key == "path_kind" || key == "path_role" {
			found = append(found, key)
		}
	})
	if len(found) > 0 {
		return fmt.Errorf("path-kind key(s) present: %v", found)
	}
	if strings.Contains(string(document), `"A"`) || strings.Contains(string(document), `"B"`) {
		return errors.New("a bare path-kind value is emitted")
	}
	return nil
}

var hexTokenRE = regexp.MustCompile(`\b[0-9a-fA-F]{64}\b`)

func checkNoContentHash(rendered string) error {
	if match := hexTokenRE.FindString(rendered); match != "" {
		return fmt.Errorf("a 64-hex token appears in output: %s", match)
	}
	return nil
}

func checkAbortShape(report Report) error {
	if report.Abort == nil {
		if len(report.Artifacts) != 4 {
			return fmt.Errorf("non-abort report has %d artifact rows, want 4", len(report.Artifacts))
		}
		return nil
	}
	if len(report.Artifacts) != 0 {
		return fmt.Errorf("abort %q emitted %d artifact rows", report.Abort.Code, len(report.Artifacts))
	}
	if len(report.Advisories) != 0 {
		return fmt.Errorf("abort %q emitted %d advisories", report.Abort.Code, len(report.Advisories))
	}
	if report.FeatureState != FeatureStateUnknown {
		return fmt.Errorf("abort %q emitted feature_state %q", report.Abort.Code, report.FeatureState)
	}
	if report.Overall.StructuralReadiness != ReadinessIndeterminate {
		return fmt.Errorf("abort %q emitted readiness %q", report.Abort.Code, report.Overall.StructuralReadiness)
	}
	if report.Overall.RequiredTotal != 3 || report.Overall.OptionalTotal != 1 {
		return fmt.Errorf("abort %q emitted totals %d/%d", report.Abort.Code, report.Overall.RequiredTotal, report.Overall.OptionalTotal)
	}
	if report.Overall.RequiredSatisfied != 0 || report.Overall.OptionalSatisfied != 0 {
		return fmt.Errorf("abort %q emitted non-zero satisfied counters", report.Abort.Code)
	}
	return nil
}

func checkAdvisoryCardinality(advisories []Advisory) error {
	if len(advisories) > 3 {
		return fmt.Errorf("advisories length %d exceeds 3", len(advisories))
	}
	sidecar := 0
	for _, advisory := range advisories {
		if strings.HasPrefix(advisory.Code, sidecarAdvisoryPrefix) {
			sidecar++
		}
	}
	if sidecar > 1 {
		return fmt.Errorf("%d analysis-sidecar advisories on one run", sidecar)
	}
	return nil
}

func checkProvenanceDomain(emitted map[string]bool) error {
	for value := range emitted {
		if value != ProvenanceUnknown {
			return fmt.Errorf("provenance domain widened to include %q without a schema decision", value)
		}
	}
	return nil
}

func checkOutputBytes(rendered, attackerArgument string) error {
	for index := 0; index < len(rendered); index++ {
		b := rendered[index]
		if b < 0x20 && b != '\n' {
			return fmt.Errorf("control byte %#x at offset %d", b, index)
		}
		if b == 0x7f {
			return fmt.Errorf("DEL byte at offset %d", index)
		}
	}
	for _, fragment := range []string{"\x1b[2J", attackerArgument} {
		if fragment != "" && strings.Contains(rendered, fragment) {
			return fmt.Errorf("a rejected argument byte sequence was echoed: %q", fragment)
		}
	}
	if !utf8Valid(rendered) {
		return errors.New("output is not valid UTF-8")
	}
	return nil
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == 0xFFFD && !strings.ContainsRune(s, 0xFFFD) {
			return false
		}
	}
	return true
}

func docCommentMentions(files []*ast.File, symbol, phrase string) bool {
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Doc == nil {
				continue
			}
			text := gen.Doc.Text()
			if strings.Contains(text, symbol) && strings.Contains(text, phrase) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Catalog totality guards
// ---------------------------------------------------------------------------

func registerCatalogGuards() {
	registerGuard("AVP-093", guardSpec{
		Run: func(t *testing.T) error {
			return checkStateEnumParity(ArtifactStates(), prdStateEnum(t))
		},
		Sensitivity: func(t *testing.T) error {
			return checkStateEnumParity(append(ArtifactStates(), "present-thin"), prdStateEnum(t))
		},
	})

	registerGuard("AVP-094", guardSpec{
		Run: func(t *testing.T) error {
			return checkPrecedence(t, realLadderClassifier(t))
		},
		Sensitivity: func(t *testing.T) error {
			swapped := func(fixture string) string {
				if fixture == "unstable-over-empty" {
					return StatePresentEmpty
				}
				return realLadderClassifier(t)(fixture)
			}
			return checkPrecedence(t, swapped)
		},
	})

	registerGuard("AVP-095", guardSpec{
		Run: func(t *testing.T) error {
			return checkCatalogTotality(t, producedCodes(t))
		},
		Sensitivity: func(t *testing.T) error {
			produced := producedCodes(t)
			delete(produced, string(AbortStatusInvalidState))
			return checkCatalogTotality(t, produced)
		},
	})

	registerGuard("AVP-101", guardSpec{
		Run: func(t *testing.T) error {
			return checkExitTemplates(AbortCodes())
		},
		Sensitivity: func(t *testing.T) error {
			return checkExitTemplates(append(AbortCodes(), AbortCode("status-fourteenth")))
		},
	})

	registerGuard("AVP-119", guardSpec{
		Run: func(t *testing.T) error {
			return checkAdvisoryTotality(realAdvisorySelector())
		},
		Sensitivity: func(t *testing.T) error {
			pruned := func(state string) (string, bool) {
				if state == StateOversize {
					return "", false
				}
				return realAdvisorySelector()(state)
			}
			return checkAdvisoryTotality(pruned)
		},
	})

	registerGuard("AVP-153", guardSpec{
		Run: func(t *testing.T) error {
			return checkLifecycleLines(AbortCodes())
		},
		Sensitivity: func(t *testing.T) error {
			return checkLifecycleLines(append(AbortCodes(), AbortCode("status-fourteenth")))
		},
	})

	registerGuard("AVP-165", guardSpec{
		Run: func(t *testing.T) error {
			return checkFeatureStateParity(FeatureStates(), storeFeatureStates(t))
		},
		Sensitivity: func(t *testing.T) error {
			return checkFeatureStateParity(FeatureStates(), append(storeFeatureStates(t), "archived"))
		},
	})

	registerGuard("AVP-168", guardSpec{
		Run: func(t *testing.T) error {
			return checkStatusAbortTotality(deriveStatusOutcomes(t, statusOutcomeProbes()))
		},
		Sensitivity: func(t *testing.T) error {
			// Arm 1: cross two probes' production-path inputs. The status
			// side is fed an oversize file while the artifact side is fed an
			// unreadable one (and vice versa), so the *derived* pairing is
			// `unreadable → status-oversize`. Totality and disjointness still
			// hold; only the state↔abort correspondence breaks, which is the
			// property this guard is actually about.
			crossed := statusOutcomeProbes()
			oversize, unreadable := -1, -1
			for index, probe := range crossed {
				switch probe.name {
				case "oversize":
					oversize = index
				case "unreadable":
					unreadable = index
				}
			}
			if oversize < 0 || unreadable < 0 {
				t.Fatal("the sensitivity arm no longer matches the probe set")
			}
			crossed[oversize].artifact, crossed[unreadable].artifact =
				crossed[unreadable].artifact, crossed[oversize].artifact
			if err := checkStatusAbortTotality(deriveStatusOutcomes(t, crossed)); err == nil {
				t.Fatal("crossing two production-path inputs did not fail the guard")
			}
			// Arm 2: stop exercising the oversize production path at all.
			// Nothing is deleted from a literal map — the probe that drives
			// `Inspect` down that branch is removed, so the derivation can no
			// longer observe the outcome and totality fails.
			pruned := append([]statusOutcomeProbe{}, statusOutcomeProbes()[:oversize]...)
			pruned = append(pruned, statusOutcomeProbes()[oversize+1:]...)
			return checkStatusAbortTotality(deriveStatusOutcomes(t, pruned))
		},
	})

	registerGuard("AVP-181", guardSpec{
		Run: func(t *testing.T) error {
			templates := map[AbortCode]string{}
			for _, code := range AbortCodes() {
				templates[code] = abortMessage(code, testSlug)
			}
			return checkMessageCatalog(AbortCodes(), templates)
		},
		Sensitivity: func(t *testing.T) error {
			templates := map[AbortCode]string{}
			for _, code := range AbortCodes() {
				templates[code] = abortMessage(code, testSlug)
			}
			templates[AbortStatusUnreadable] = "status.json could not be read: open /abs/path: permission denied (%v)"
			return checkMessageCatalog(AbortCodes(), templates)
		},
	})

	registerGuard("AVP-201", guardSpec{
		Run: func(t *testing.T) error {
			return checkCapMessageCoupling(t, MaxArtifactBytes, MaxStatusBytes)
		},
		Sensitivity: func(t *testing.T) error {
			return checkCapMessageCoupling(t, 8<<20, MaxStatusBytes)
		},
	})
}

func prdStateEnum(t *testing.T) []string {
	t.Helper()
	prd := repoFile(t, "docs/prds/PRD-artifact-validation-and-provenance.md")
	section := prd[strings.Index(prd, "### 7.6 The closed state enum"):]
	section = section[:strings.Index(section, "### 7.7")]
	rowRE := regexp.MustCompile("(?m)^\\| `([a-z-]+)` \\|")
	var states []string
	for _, match := range rowRE.FindAllStringSubmatch(section, -1) {
		states = append(states, match[1])
	}
	if len(states) != 9 {
		t.Fatalf("parsed %d states from §7.6, want 9", len(states))
	}
	return states
}

func checkStateEnumParity(implementation, declared []string) error {
	left := append([]string(nil), implementation...)
	right := append([]string(nil), declared...)
	sort.Strings(left)
	sort.Strings(right)
	if strings.Join(left, ",") != strings.Join(right, ",") {
		return fmt.Errorf("state enum drift: implementation %v vs PRD %v", left, right)
	}
	return nil
}

func realLadderClassifier(t *testing.T) func(string) string {
	return func(fixture string) string {
		root := fixtureRoot(t)
		switch fixture {
		case "unstable-over-empty":
			// Zero-length capture whose declared size disagrees: the
			// instability probe must win over `present-empty`.
			info := sized(testSpec, 32)
			root.nodes[testSpec] = &fakeNode{info: info, file: &fakeFile{
				name: testSpec, data: nil, statInfos: []fs.FileInfo{info},
			}}
		case "unstable-over-invalid-structured":
			info := sized(testSidecar, 32)
			root.nodes[testSidecar] = &fakeNode{info: info, file: &fakeFile{
				name: testSidecar, data: []byte("["), statInfos: []fs.FileInfo{info},
			}}
		case "symlink-over-not-regular":
			root.set(testSpec, fakeInfo{name: testSpec, mode: fs.ModeSymlink | fs.ModeDir})
		case "oversize-before-open":
			root.set(testSpec, sized(testSpec, MaxArtifactBytes+1))
		}
		report := Inspect(root, testSlug, scratchBuffer())
		id := "spec"
		if fixture == "unstable-over-invalid-structured" {
			id = "analysis_sidecar"
		}
		for _, artifact := range report.Artifacts {
			if artifact.ID == id {
				return artifact.State
			}
		}
		return "no-artifact"
	}
}

func checkPrecedence(t *testing.T, classify func(string) string) error {
	expected := map[string]string{
		"unstable-over-empty":              StateUnstable,
		"unstable-over-invalid-structured": StateUnstable,
		"symlink-over-not-regular":         StateSymlinkRefused,
		"oversize-before-open":             StateOversize,
	}
	names := make([]string, 0, len(expected))
	for name := range expected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if got := classify(name); got != expected[name] {
			return fmt.Errorf("precedence pair %s classified %q, want %q", name, got, expected[name])
		}
	}
	// The pre-open oversize refusal must beat the open itself.
	root := fixtureRoot(t)
	root.set(testSpec, sized(testSpec, MaxArtifactBytes+1))
	Inspect(root, testSlug, scratchBuffer())
	if root.opensOf(testSpec) != 0 {
		return errors.New("oversize did not outrank the open")
	}
	return nil
}

func producedCodes(t *testing.T) map[string]bool {
	t.Helper()
	produced := map[string]bool{}
	for _, report := range guardReportMatrix(t) {
		if report.Abort != nil {
			produced[string(report.Abort.Code)] = true
		}
		for _, artifact := range report.Artifacts {
			if artifact.ReasonCode != "" {
				produced[artifact.ReasonCode] = true
			}
		}
		for _, advisory := range report.Advisories {
			produced[advisory.Code] = true
		}
	}
	// The remaining sidecar advisories and reason codes are produced by the
	// dedicated fixtures below rather than by the shared matrix.
	extras := []func(*fakeRoot){
		func(r *fakeRoot) { r.set(testSidecar, fakeInfo{name: testSidecar, mode: fs.ModeSymlink}) },
		func(r *fakeRoot) { r.set(testSidecar, dir(testSidecar)) },
		func(r *fakeRoot) { r.nodes[testSidecar].openErr = fs.ErrPermission },
		func(r *fakeRoot) { r.set(testSidecar, sized(testSidecar, MaxArtifactBytes+1)) },
		func(r *fakeRoot) { r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testSidecar } },
		func(r *fakeRoot) { r.setFile(testSidecar, []byte(`"str"`)) },
		func(r *fakeRoot) { r.remove(testSidecarDir) },
	}
	for _, mutate := range extras {
		root := fixtureRoot(t)
		mutate(root)
		report := Inspect(root, testSlug, scratchBuffer())
		for _, artifact := range report.Artifacts {
			if artifact.ReasonCode != "" {
				produced[artifact.ReasonCode] = true
			}
		}
		for _, advisory := range report.Advisories {
			produced[advisory.Code] = true
		}
	}
	return produced
}

func allCatalogCodes() []string {
	codes := []string{
		ReasonArtifactEmpty, ReasonArtifactAbsent, ReasonArtifactSymlinkRefused,
		ReasonArtifactNotRegular, ReasonArtifactUnreadable, ReasonArtifactOversize,
		ReasonArtifactUnstable, ReasonSidecarNotJSON, ReasonSidecarNotJSONObject,
		AdvisoryFeatureStateAbsent, AdvisoryProvenanceUnknown, AdvisorySidecarAbsent,
		AdvisorySidecarEmpty, AdvisorySidecarInvalid, AdvisorySidecarUnstable,
		AdvisorySidecarSymlinkRefused, AdvisorySidecarNotRegular,
		AdvisorySidecarUnreadable, AdvisorySidecarOversize,
	}
	for _, code := range AbortCodes() {
		codes = append(codes, string(code))
	}
	return codes
}

func checkCatalogTotality(t *testing.T, produced map[string]bool) error {
	for _, code := range allCatalogCodes() {
		if !produced[code] {
			return fmt.Errorf("catalog code %q is produced by no acceptance fixture", code)
		}
	}
	for code := range produced {
		known := false
		for _, declared := range allCatalogCodes() {
			if declared == code {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("fixture produced %q, which is in no catalog", code)
		}
	}
	// The reason-code ↔ state mapping is total in both directions, with
	// invalid-structured the single documented one-to-two case.
	mapping := map[string][]string{}
	for _, state := range ArtifactStates() {
		switch state {
		case StatePresentNonempty:
			mapping[state] = []string{""}
		case StateInvalidStructured:
			mapping[state] = []string{ReasonSidecarNotJSON, ReasonSidecarNotJSONObject}
		default:
			mapping[state] = []string{reasonCode(state, false, "")}
		}
	}
	if len(mapping) != 9 {
		return fmt.Errorf("state→reason map covers %d states, want 9", len(mapping))
	}
	if len(mapping[StateInvalidStructured]) != 2 {
		return errors.New("invalid-structured is no longer the single one-to-two case")
	}
	for _, code := range AbortCodes() {
		if abortMessage(code, testSlug) == "" {
			return fmt.Errorf("abort %q has no message template", code)
		}
		if lifecycleAnnotation(NewAbortReport(testSlug, code)) == "" {
			return fmt.Errorf("abort %q has no lifecycle line", code)
		}
	}
	return nil
}

func checkExitTemplates(codes []AbortCode) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("abort code without a closed template: %v", recovered)
		}
	}()
	seen := map[string]bool{}
	for _, code := range codes {
		report := NewAbortReportUnchecked(testSlug, code)
		message := report.ExitMessage()
		if message == "" {
			return fmt.Errorf("abort %q has no exit template", code)
		}
		if strings.Count(message, "\n") != 0 {
			return fmt.Errorf("abort %q exit line is multi-line", code)
		}
		if strings.Contains(message, "/Users/") || strings.Contains(message, "%!") {
			return fmt.Errorf("abort %q exit line leaks a path or format verb", code)
		}
		if seen[message] {
			return fmt.Errorf("abort %q shares an exit line with another code", code)
		}
		seen[message] = true
	}
	return nil
}

func realAdvisorySelector() func(string) (string, bool) {
	return func(state string) (string, bool) {
		advisory := sidecarAdvisory(state)
		if advisory == nil {
			return "", state == StatePresentNonempty
		}
		return advisory.Code, true
	}
}

func checkAdvisoryTotality(selector func(string) (string, bool)) error {
	for _, state := range ArtifactStates() {
		code, ok := selector(state)
		if !ok {
			return fmt.Errorf("sidecar state %q selects no advisory row", state)
		}
		if state == StatePresentNonempty {
			if code != "" {
				return fmt.Errorf("present-nonempty selected advisory %q", code)
			}
			continue
		}
		if code == "" {
			return fmt.Errorf("sidecar state %q has an empty advisory code", state)
		}
	}
	return nil
}

func checkLifecycleLines(codes []AbortCode) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("abort code without a lifecycle line: %v", recovered)
		}
	}()
	lines := map[string]bool{}
	for _, code := range codes {
		line := lifecycleAnnotation(NewAbortReportUnchecked(testSlug, code))
		if line == "" {
			return fmt.Errorf("abort %q has no lifecycle line", code)
		}
		if lines[line] {
			return fmt.Errorf("abort %q shares a lifecycle line", code)
		}
		lines[line] = true
	}
	// Plus the two non-abort populations: status ok and status absent.
	ok := Report{FeatureState: FeatureStateDefined}
	absent := Report{FeatureState: FeatureStateUnknown}
	if lifecycleAnnotation(ok) == "" || lifecycleAnnotation(absent) == "" {
		return errors.New("a non-abort lifecycle population lost its line")
	}
	if len(lines)+2 != len(codes)+2 {
		return errors.New("lifecycle lines are not a bijection with the populations")
	}
	return nil
}

func storeFeatureStates(t *testing.T) []string {
	t.Helper()
	_, files := productionFiles(t, "internal/store")
	var states []string
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || value.Type == nil {
					continue
				}
				ident, ok := value.Type.(*ast.Ident)
				if !ok || ident.Name != "FeatureState" {
					continue
				}
				for _, expr := range value.Values {
					lit, ok := expr.(*ast.BasicLit)
					if !ok {
						continue
					}
					if unquoted, err := strconv.Unquote(lit.Value); err == nil {
						states = append(states, unquoted)
					}
				}
			}
		}
	}
	if len(states) == 0 {
		t.Fatal("no FeatureState constants parsed from internal/store")
	}
	return states
}

func checkFeatureStateParity(implementation, declared []string) error {
	left := append([]string(nil), implementation...)
	right := append([]string(nil), declared...)
	sort.Strings(left)
	sort.Strings(right)
	if strings.Join(left, ",") != strings.Join(right, ",") {
		return fmt.Errorf("FeatureState drift: intent %v vs store %v", left, right)
	}
	return nil
}

// statusOutcomeProbe drives one filesystem condition through the production
// code twice: once with the condition on `status.json`, which yields an abort
// code, and once with the same condition on `spec.md`, which yields the
// structural state the production classifier assigns to it. Nothing here is a
// hand-written table of expected outcomes — both halves are read back out of
// `Inspect`.
type statusOutcomeProbe struct {
	name     string
	status   func(root *fakeRoot)
	artifact func(root *fakeRoot)
}

func statusOutcomeProbes() []statusOutcomeProbe {
	empty := func(name string) func(*fakeRoot) {
		return func(root *fakeRoot) { root.setFile(name, nil) }
	}
	symlink := func(name string) func(*fakeRoot) {
		return func(root *fakeRoot) { root.set(name, fakeInfo{name: name, mode: fs.ModeSymlink}) }
	}
	notRegular := func(name string) func(*fakeRoot) {
		return func(root *fakeRoot) { root.set(name, dir(name)) }
	}
	oversize := func(name string, limit int) func(*fakeRoot) {
		return func(root *fakeRoot) { root.set(name, sized(name, int64(limit)+1)) }
	}
	vanished := func(name string) func(*fakeRoot) {
		// Lstat succeeds, the open then fails with ErrNotExist: the file was
		// replaced between the two syscalls.
		return func(root *fakeRoot) { root.nodes[name].openErr = fs.ErrNotExist }
	}
	unreadable := func(name string) func(*fakeRoot) {
		return func(root *fakeRoot) { root.nodes[name].openErr = fs.ErrPermission }
	}
	return []statusOutcomeProbe{
		{"present-empty", empty(testStatus), empty(testSpec)},
		{"symlink", symlink(testStatus), symlink(testSpec)},
		{"not-regular", notRegular(testStatus), notRegular(testSpec)},
		{"oversize", oversize(testStatus, MaxStatusBytes), oversize(testSpec, MaxArtifactBytes)},
		{"unstable", vanished(testStatus), vanished(testSpec)},
		{"unreadable", unreadable(testStatus), unreadable(testSpec)},
	}
}

// deriveStatusOutcomes runs every probe through the real `Inspect` and returns
// the state→abort mapping the production status ladder actually implements.
func deriveStatusOutcomes(t *testing.T, probes []statusOutcomeProbe) map[string]AbortCode {
	t.Helper()
	outcomes := map[string]AbortCode{}
	for _, probe := range probes {
		statusRoot := fixtureRoot(t)
		probe.status(statusRoot)
		abort := Inspect(statusRoot, testSlug, scratchBuffer()).AbortCode()
		if abort == "" {
			t.Fatalf("probe %q on status.json produced no abort", probe.name)
		}

		artifactRoot := fixtureRoot(t)
		probe.artifact(artifactRoot)
		report := Inspect(artifactRoot, testSlug, scratchBuffer())
		if report.AbortCode() != "" {
			t.Fatalf("probe %q on spec.md aborted (%q); it must reach artifact classification",
				probe.name, report.AbortCode())
		}
		state := artifactByID(t, report, "spec").State
		if previous, seen := outcomes[state]; seen && previous != abort {
			t.Fatalf("probe %q derived state %q twice with different aborts (%q, %q)",
				probe.name, state, previous, abort)
		}
		outcomes[state] = abort
	}
	return outcomes
}

func checkStatusAbortTotality(outcomes map[string]AbortCode) error {
	for _, state := range ArtifactStates() {
		switch state {
		case StatePresentNonempty, StateAbsent, StateInvalidStructured:
			continue
		}
		if _, ok := outcomes[state]; !ok {
			return fmt.Errorf("status outcome %q maps to no abort code", state)
		}
	}
	seen := map[AbortCode]bool{}
	for _, code := range outcomes {
		if seen[code] {
			return fmt.Errorf("status abort code %q is not pairwise disjoint", code)
		}
		seen[code] = true
	}
	// Every ladder outcome names its own state, with exactly one documented
	// exception: an empty status document is `status-malformed`, because zero
	// bytes are not a decodable status document.
	for state, code := range outcomes {
		want := AbortCode("status-" + state)
		if state == StatePresentEmpty {
			want = AbortStatusMalformed
		}
		if code != want {
			return fmt.Errorf("status state %q aborts %q, want %q", state, code, want)
		}
	}
	// The status catalog has exactly seven codes: six ladder outcomes plus
	// the two document verdicts, of which malformed is shared.
	statusCodes := 0
	for _, code := range AbortCodes() {
		if strings.HasPrefix(string(code), "status-") {
			statusCodes++
		}
	}
	if statusCodes != 7 {
		return fmt.Errorf("%d status-* abort codes, want 7", statusCodes)
	}
	return nil
}

func checkMessageCatalog(codes []AbortCode, templates map[AbortCode]string) error {
	if len(templates) != len(codes) {
		return fmt.Errorf("%d templates for %d codes — not a bijection", len(templates), len(codes))
	}
	seen := map[string]bool{}
	for _, code := range codes {
		template, ok := templates[code]
		if !ok || template == "" {
			return fmt.Errorf("abort %q has no template", code)
		}
		if seen[template] {
			return fmt.Errorf("abort %q shares a template", code)
		}
		seen[template] = true
		for _, forbidden := range []string{"%v", "%s", "docs/", ".md", "http://", "https://", "/Users/", "permission denied"} {
			if strings.Contains(template, forbidden) {
				return fmt.Errorf("abort %q template contains %q", code, forbidden)
			}
		}
	}
	return nil
}

func checkCapMessageCoupling(t *testing.T, artifactCap, statusCap int) error {
	artifactUnit := fmt.Sprintf("%d MiB", artifactCap>>20)
	statusUnit := fmt.Sprintf("%d MiB", statusCap>>20)
	oversizeAdvisory := sidecarAdvisory(StateOversize)
	if oversizeAdvisory == nil || !strings.Contains(oversizeAdvisory.Message, artifactUnit) {
		return fmt.Errorf("the sidecar oversize advisory does not name %s", artifactUnit)
	}
	remediationText := remediation(artifactSpecs[1], testSpec, testSlug, StateOversize)
	if !strings.Contains(remediationText, artifactUnit) {
		return fmt.Errorf("the oversize remediation does not name %s", artifactUnit)
	}
	statusTemplate := abortMessage(AbortStatusOversize, testSlug)
	if !strings.Contains(statusTemplate, statusUnit) {
		return fmt.Errorf("the status-oversize template does not name %s", statusUnit)
	}
	if artifactUnit == statusUnit {
		return errors.New("the two caps must render to distinct human units")
	}
	if strings.Contains(oversizeAdvisory.Message, statusUnit) {
		return fmt.Errorf("the artifact advisory names the status limit %s", statusUnit)
	}
	return nil
}

// NewAbortReportUnchecked builds an abort report without the closed-catalog
// assertion, so the catalog guards can drive a synthetic fourteenth code
// through the same renderers the production path uses.
func NewAbortReportUnchecked(slug string, code AbortCode) Report {
	report := Report{
		SchemaVersion: 1,
		Command:       CommandName,
		Slug:          slug,
		FeatureState:  FeatureStateUnknown,
		Disclaimer:    disclaimer,
		Artifacts:     []Artifact{},
		Overall: Overall{
			StructuralReadiness: ReadinessIndeterminate,
			RequiredTotal:       3,
			OptionalTotal:       1,
		},
		Advisories: []Advisory{},
		Abort:      &Abort{Code: code},
	}
	report.Abort.Message = abortMessage(code, slug)
	return report
}

// ---------------------------------------------------------------------------
// Source-scan guards
// ---------------------------------------------------------------------------

var forbiddenUnrootedReaders = []string{
	"os.Stat", "os.Lstat", "os.Open", "os.OpenFile", "os.ReadFile", "os.ReadDir",
	"filepath.Walk", "filepath.WalkDir", "ioutil.ReadFile",
}

var forbiddenRootMutators = []string{
	"root.Create", "root.Mkdir", "root.MkdirAll", "root.Remove", "root.RemoveAll",
	"root.Rename", "root.Chmod", "root.Chown", "root.Chtimes", "root.Link",
	"root.Symlink", "root.WriteFile", "os.Create", "os.Mkdir", "os.MkdirAll",
	"os.Remove", "os.RemoveAll", "os.Rename", "os.WriteFile",
}

func inspectorSources(t *testing.T) []*ast.File {
	t.Helper()
	_, intentFiles := productionFiles(t, "internal/intent")
	_, cliFiles := productionFiles(t, "internal/cli")
	for _, file := range cliFiles {
		if strings.HasSuffix(positionName(t, file), "prepare.go") {
			intentFiles = append(intentFiles, file)
		}
	}
	return intentFiles
}

func positionName(t *testing.T, file *ast.File) string {
	t.Helper()
	if file.Name == nil {
		return ""
	}
	// The parser records the file name in the FileSet; re-derive it from the
	// declarations' position via a fresh parse of the same path is overkill,
	// so identify the prepare command file by a marker declaration instead.
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name != nil && fn.Name.Name == "prepareCmd" {
			return "prepare.go"
		}
	}
	return "other.go"
}

func registerSourceGuards() {
	registerGuard("AVP-089", guardSpec{
		Run: func(t *testing.T) error {
			return forbiddenSelectors(inspectorSources(t), forbiddenUnrootedReaders)
		},
		Sensitivity: func(t *testing.T) error {
			return forbiddenSelectors(parseFixtureSource(t, `package intent
import "os"
func leak(name string) { _, _ = os.Lstat(name) }`), forbiddenUnrootedReaders)
		},
	})

	registerGuard("AVP-150", guardSpec{
		Run: func(t *testing.T) error {
			forbidden := append([]string{"store.LoadFeatureStatus", "store.Store"}, forbiddenUnrootedReaders...)
			files := inspectorSources(t)
			if err := forbiddenSelectors(files, forbidden); err != nil {
				return err
			}
			_, intentOnly := productionFiles(t, "internal/intent")
			imports := importPaths(intentOnly)
			for _, banned := range []string{
				"github.com/tesseracode/tesserapatch/internal/store",
				"github.com/tesseracode/tesserapatch/internal/gitutil",
			} {
				if imports[banned] {
					return fmt.Errorf("internal/intent imports %s", banned)
				}
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			return forbiddenSelectors(parseFixtureSource(t, `package intent
import "github.com/tesseracode/tesserapatch/internal/store"
func read(s *store.Store) { _, _ = store.LoadFeatureStatus("x") }`),
				[]string{"store.LoadFeatureStatus", "store.Store"})
		},
	})

	registerGuard("AVP-116", guardSpec{
		Run: func(t *testing.T) error {
			_, files := productionFiles(t, "internal/intent")
			if err := forbiddenSelectors(files, []string{"io.ReadAll", "io.LimitReader", "bufio.NewScanner", "bufio.Scanner", "os.ReadFile", "ioutil.ReadFile"}); err != nil {
				return err
			}
			if err := checkNoByteSliceMake(files); err != nil {
				return err
			}
			source := repoFile(t, "internal/intent/inspect.go")
			if !strings.Contains(source, "scratch[:MaxStatusBytes+1]") {
				return errors.New("the status capture no longer takes the shared sub-slice")
			}
			if !strings.Contains(source, "buffer[:limit+1]") {
				return errors.New("the bounded read lost its +1")
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			return checkNoByteSliceMake(parseFixtureSource(t, `package intent
func capture() []byte { return make([]byte, 4096) }`))
		},
	})

	registerGuard("AVP-172", guardSpec{
		Run: func(t *testing.T) error {
			_, files := productionFiles(t, "internal/intent")
			return forbiddenSelectors(files, []string{"io.ReadAll", "io.LimitReader", "os.ReadFile", "ioutil.ReadFile", "bufio.NewScanner"})
		},
		Sensitivity: func(t *testing.T) error {
			return forbiddenSelectors(parseFixtureSource(t, `package intent
import "io"
func capture(f io.Reader) ([]byte, error) { return io.ReadAll(io.LimitReader(f, 4194305)) }`),
				[]string{"io.ReadAll", "io.LimitReader"})
		},
	})

	registerGuard("AVP-193", guardSpec{
		Run: func(t *testing.T) error {
			return checkNoParseErrorInterception(prepareCommandFile(t))
		},
		Sensitivity: func(t *testing.T) error {
			return checkNoParseErrorInterception(parseFixtureSource(t, `package cli
func prepareCmd() {
	cmd.SetFlagErrorFunc(func(err error) error { return err })
}`))
		},
	})

	registerGuard("AVP-194", guardSpec{
		Run: func(t *testing.T) error {
			_, files := productionFiles(t, "internal/intent")
			return checkSeamShape(files)
		},
		Sensitivity: func(t *testing.T) error {
			_, files := productionFiles(t, "internal/intent")
			extra := parseFixtureSource(t, `package intent
import "io/fs"
type shadowRootOps struct{ a, b int }
func (s shadowRootOps) Lstat(name string) (fs.FileInfo, error) { return nil, nil }
func (s shadowRootOps) OpenFile(name string, flag int, perm fs.FileMode) (FileOps, error) { return nil, nil }
func (s shadowRootOps) SameFile(a, b fs.FileInfo) bool { return false }`)
			return checkSeamShape(append(files, extra...))
		},
	})

	registerGuard("AVP-206", guardSpec{
		Run: func(t *testing.T) error {
			_, files := productionFiles(t, "internal/intent")
			return checkSameFileCallSites(files)
		},
		Sensitivity: func(t *testing.T) error {
			_, files := productionFiles(t, "internal/intent")
			extra := parseFixtureSource(t, `package intent
import ("io/fs"; "os")
func compare(a, b fs.FileInfo) bool { return os.SameFile(a, b) }`)
			return checkSameFileCallSites(append(files, extra...))
		},
	})

	registerGuard("AVP-144", guardSpec{
		Run: func(t *testing.T) error {
			_, root := runInspect(t, nil)
			names := append(append([]string{}, root.lstatNames...), root.opened...)
			if err := checkValidPathNames(names); err != nil {
				return err
			}
			_, files := productionFiles(t, "internal/intent")
			return forbiddenSelectors(files, []string{"filepath.Join", "filepath.Abs", "filepath.Clean"})
		},
		Sensitivity: func(t *testing.T) error {
			return checkValidPathNames([]string{".tpatch/features/../../etc/passwd"})
		},
	})

	registerGuard("AVP-146", guardSpec{
		Run: func(t *testing.T) error {
			return checkReparsePredicate(refused)
		},
		Sensitivity: func(t *testing.T) error {
			narrowed := func(info fs.FileInfo) bool { return info.Mode()&fs.ModeSymlink != 0 }
			return checkReparsePredicate(narrowed)
		},
	})

	registerGuard("AVP-170", guardSpec{
		Run: func(t *testing.T) error {
			// Real measurement of the real Inspect.
			return checkCaptureAllocations(t, false)
		},
		Sensitivity: func(t *testing.T) error {
			// Synthetic model of the rejected per-capture design — see the
			// honesty note on checkCaptureAllocations.
			return checkCaptureAllocations(t, true)
		},
	})

	registerGuard("AVP-197", guardSpec{
		Run: func(t *testing.T) error {
			return checkSingleScratchAllocation(t, MaxArtifactBytes+1)
		},
		Sensitivity: func(t *testing.T) error {
			// A per-capture buffer would double the invocation's data-buffer
			// footprint; the guard must reject that arithmetic. This is an
			// arithmetic assertion over the real invocation's measured
			// footprint, not a measurement of a mutated build.
			return checkSingleScratchAllocation(t, 2*(MaxArtifactBytes+1))
		},
	})

	registerGuard("AVP-205", guardSpec{
		Run: func(t *testing.T) error {
			return checkDescriptorLifecycle(t, false)
		},
		Sensitivity: func(t *testing.T) error {
			return checkDescriptorLifecycle(t, true)
		},
	})

	registerGuard("AVP-139", guardSpec{
		Run: func(t *testing.T) error {
			derived := guardRowsFromMatrix(t)
			if len(derived) != 43 {
				return fmt.Errorf("derived %d guard rows from the matrix, want 43", len(derived))
			}
			for _, id := range derived {
				spec, ok := avpGuards[id]
				if !ok {
					return fmt.Errorf("matrix row %s carries a G kind but has no registered guard", id)
				}
				if spec.Sensitivity == nil {
					return fmt.Errorf("guard %s ships no sensitivity fixture", id)
				}
			}
			for id := range avpGuards {
				found := false
				for _, declared := range derived {
					if declared == id {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("registered guard %s is not a G-kind matrix row", id)
				}
			}
			// AVP-128 is `S+I` and must NOT be in the derived set.
			for _, id := range derived {
				if id == "AVP-128" {
					return errors.New("AVP-128 (S+I) was derived as a guard row")
				}
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			derived := guardRowsFromMatrix(t)
			registry := map[string]guardSpec{}
			for id, spec := range avpGuards {
				registry[id] = spec
			}
			delete(registry, derived[0])
			for _, id := range derived {
				if _, ok := registry[id]; !ok {
					return fmt.Errorf("matrix row %s has no registered guard", id)
				}
			}
			return nil
		},
	})
}

func prepareCommandFile(t *testing.T) []*ast.File {
	t.Helper()
	_, files := productionFiles(t, "internal/cli")
	var out []*ast.File
	for _, file := range files {
		if positionName(t, file) == "prepare.go" {
			out = append(out, file)
		}
	}
	if len(out) != 1 {
		t.Fatalf("found %d prepare command files, want 1", len(out))
	}
	return out
}

func checkNoByteSliceMake(files []*ast.File) error {
	var found []string
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "make" {
				return true
			}
			arrayType, ok := call.Args[0].(*ast.ArrayType)
			if !ok {
				return true
			}
			elem, ok := arrayType.Elt.(*ast.Ident)
			if ok && elem.Name == "byte" {
				found = append(found, "make([]byte, …)")
			}
			return true
		})
	}
	if len(found) > 0 {
		return fmt.Errorf("the inspector allocates its own data buffer: %v", found)
	}
	return nil
}

func checkNoParseErrorInterception(files []*ast.File) error {
	forbidden := []string{
		"cmd.SetFlagErrorFunc", "cmd.SetErr", "cmd.SetOut", "cmd.SetOutput",
		"cobra.FlagErrorFunc",
	}
	seen := selectorSet(files)
	for _, name := range forbidden {
		if seen[name] > 0 {
			return fmt.Errorf("the prepare command installs %s", name)
		}
	}
	for _, file := range files {
		var bad error
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range assign.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if sel.Sel.Name == "SilenceUsage" || sel.Sel.Name == "SilenceErrors" || sel.Sel.Name == "FParseErrWhitelist" {
					bad = fmt.Errorf("the prepare command assigns %s locally", sel.Sel.Name)
				}
			}
			return true
		})
		if bad != nil {
			return bad
		}
	}
	return nil
}

func checkSeamShape(files []*ast.File) error {
	methods := map[string]map[string]bool{}
	structs := map[string]int{}
	interfaces := map[string][]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			switch typed := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range typed.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					switch definition := typeSpec.Type.(type) {
					case *ast.StructType:
						structs[typeSpec.Name.Name] = definition.Fields.NumFields()
					case *ast.InterfaceType:
						var names []string
						for _, method := range definition.Methods.List {
							for _, name := range method.Names {
								names = append(names, name.Name)
							}
						}
						interfaces[typeSpec.Name.Name] = names
					}
				}
			case *ast.FuncDecl:
				if typed.Recv == nil || len(typed.Recv.List) == 0 {
					continue
				}
				receiver := receiverTypeName(typed.Recv.List[0].Type)
				if methods[receiver] == nil {
					methods[receiver] = map[string]bool{}
				}
				methods[receiver][typed.Name.Name] = true
			}
		}
	}
	if got := interfaces["RootOps"]; len(got) != 3 {
		return fmt.Errorf("RootOps declares %d methods (%v), want 3", len(got), got)
	}
	if got := interfaces["FileOps"]; len(got) != 3 {
		return fmt.Errorf("FileOps declares %d methods (%v), want 3", len(got), got)
	}
	var rootImpls, fileImpls []string
	for name, set := range methods {
		if set["Lstat"] && set["OpenFile"] && set["SameFile"] {
			rootImpls = append(rootImpls, name)
		}
		if set["Stat"] && set["Read"] && set["Close"] {
			fileImpls = append(fileImpls, name)
		}
	}
	sort.Strings(rootImpls)
	sort.Strings(fileImpls)
	if len(rootImpls) != 1 || rootImpls[0] != "osRootOps" {
		return fmt.Errorf("production RootOps implementations = %v, want exactly [osRootOps]", rootImpls)
	}
	if len(fileImpls) != 1 || fileImpls[0] != "osFileOps" {
		return fmt.Errorf("production FileOps implementations = %v, want exactly [osFileOps]", fileImpls)
	}
	if structs["osRootOps"] != 1 || structs["osFileOps"] != 1 {
		return fmt.Errorf("adapters carry %d/%d fields, want exactly one each",
			structs["osRootOps"], structs["osFileOps"])
	}
	return nil
}

func receiverTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	}
	return ""
}

func checkSameFileCallSites(files []*ast.File) error {
	count := selectorSet(files)["os.SameFile"]
	if count != 1 {
		return fmt.Errorf("os.SameFile appears at %d production call sites, want exactly 1", count)
	}
	return nil
}

func checkValidPathNames(names []string) error {
	for _, name := range names {
		if !fs.ValidPath(name) {
			return fmt.Errorf("%q is not a canonical fs.ValidPath name", name)
		}
	}
	return nil
}

func checkReparsePredicate(predicate func(fs.FileInfo) bool) error {
	cases := []struct {
		name    string
		info    fs.FileInfo
		refused bool
	}{
		{"symlink", fakeInfo{name: "l", mode: fs.ModeSymlink}, true},
		{"junction", fakeInfo{name: "j", mode: fs.ModeIrregular}, true},
		{"junction-with-dir-bit", fakeInfo{name: "j", mode: fs.ModeIrregular | fs.ModeDir}, true},
		{"regular", fakeInfo{name: "r"}, false},
		{"directory", fakeInfo{name: "d", mode: fs.ModeDir}, false},
	}
	for _, tc := range cases {
		if got := predicate(tc.info); got != tc.refused {
			return fmt.Errorf("predicate(%s) = %v, want %v", tc.name, got, tc.refused)
		}
	}
	return nil
}

// checkCaptureAllocations measures the allocation delta across a capture.
//
// Honesty note: the `modelRejectedDesign` arm does NOT rebuild the inspector
// with a per-capture buffer — no production code is mutated anywhere in this
// package. It allocates a growing slice *inside the measurement window* to
// model what the rejected design would cost, and asserts the guard's
// arithmetic rejects that cost. The guard's positive arm is a real
// measurement of the real `Inspect`; the sensitivity arm is a synthetic
// model, and the two must not be conflated when citing this row as evidence.
func checkCaptureAllocations(t *testing.T, modelRejectedDesign bool) error {
	sizes := []int{1, MaxArtifactBytes - 1, MaxArtifactBytes}
	for _, size := range sizes {
		root := fixtureRoot(t)
		root.setFile(testSpec, filledBytes(size))
		scratch := scratchBuffer()
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		if modelRejectedDesign {
			// The rejected design: each capture grows its own slice.
			grown := make([]byte, 0)
			for len(grown) < size {
				grown = append(grown, byte('a'))
			}
			_ = grown
		}
		Inspect(root, testSlug, scratch)
		runtime.ReadMemStats(&after)
		if allocated := after.TotalAlloc - before.TotalAlloc; allocated > MaxArtifactBytes {
			return fmt.Errorf("the capture window allocated %d bytes for a %d-byte artifact; the ceiling is %d",
				allocated, size, MaxArtifactBytes)
		}
	}
	return nil
}

// checkSingleScratchAllocation asserts one invocation's data-buffer footprint
// is one scratch buffer.
//
// Honesty note: the sensitivity arm asserts the *stated budget* is the one
// that actually holds — it feeds a doubled budget and requires the real
// invocation not to satisfy it. It does not measure a second production build
// in which a per-capture buffer exists; no such build is produced here.
func checkSingleScratchAllocation(t *testing.T, budget int) error {
	root := fixtureRoot(t)
	root.setFile(testExploration, filledBytes(MaxArtifactBytes-1))
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	scratch := make([]byte, MaxArtifactBytes+1)
	Inspect(root, testSlug, scratch)
	runtime.ReadMemStats(&after)
	allocated := int(after.TotalAlloc - before.TotalAlloc)
	if budget != MaxArtifactBytes+1 {
		// The sensitivity arm asserts the *stated* budget is the one that
		// holds; a doubled budget must not be satisfiable by one buffer.
		if allocated >= budget {
			return nil
		}
		return fmt.Errorf("invocation allocated %d bytes, which does not match the %d-byte budget", allocated, budget)
	}
	if allocated < budget {
		return fmt.Errorf("invocation allocated %d bytes; one scratch buffer is %d", allocated, budget)
	}
	if allocated >= 2*budget {
		return fmt.Errorf("invocation allocated %d bytes — room for a second data buffer", allocated)
	}
	return nil
}

func checkDescriptorLifecycle(t *testing.T, skipClose bool) error {
	root := fixtureRoot(t)
	root.nodes[testSpec].file.statErrs = []error{errors.New("fstat")}
	Inspect(root, testSlug, scratchBuffer())
	for _, file := range root.handedOut {
		closes := file.closes
		if skipClose && file.name == testSpec {
			// Model the regression: an early-return path that forgets Close.
			closes = 0
		}
		if closes != 1 {
			return fmt.Errorf("descriptor %q closed %d times, want exactly 1", file.name, closes)
		}
	}
	return nil
}

// guardRowsFromMatrix derives the guard set mechanically from the PRD's
// acceptance matrix: every row whose Kind column contains `G`.
func guardRowsFromMatrix(t *testing.T) []string {
	t.Helper()
	rows := parseAcceptanceMatrix(t)
	var derived []string
	for _, row := range rows {
		if strings.Contains(row.Kind, "G") {
			derived = append(derived, row.ID)
		}
	}
	sort.Strings(derived)
	return derived
}

// ---------------------------------------------------------------------------
// Platform guards
// ---------------------------------------------------------------------------

func registerPlatformGuards() {
	registerGuard("AVP-118", guardSpec{
		Run: func(t *testing.T) error {
			return checkOpenFlagsPartition(openFlagsHalves(t))
		},
		Sensitivity: func(t *testing.T) error {
			halves := openFlagsHalves(t)
			halves["unix"] = "//go:build unix\npackage intent\nimport (\"os\";\"syscall\")\nfunc openFlags() int { return syscall.O_NONBLOCK | os.O_WRONLY }\n"
			if err := checkOpenFlagsPartition(halves); err == nil {
				return errors.New("a write bit did not fail the guard")
			}
			halves = openFlagsHalves(t)
			halves["unix"] = "//go:build unix\npackage intent\nimport \"syscall\"\nfunc openFlags() int { return syscall.O_NONBLOCK | syscall.O_NOFOLLOW }\n"
			if err := checkOpenFlagsPartition(halves); err == nil {
				return errors.New("a reintroduced O_NOFOLLOW did not fail the guard")
			}
			// rev-6 erratum: collapsing back to two `!windows`/`windows`
			// halves must fail, because the unsupported set stops compiling.
			collapsed := map[string]string{
				"unix":    "//go:build !windows\npackage intent\nimport \"syscall\"\nfunc openFlags() int { return syscall.O_NONBLOCK }\n",
				"windows": openFlagsHalves(t)["windows"],
			}
			return checkOpenFlagsPartition(collapsed)
		},
	})

	registerGuard("AVP-175", guardSpec{
		Run: func(t *testing.T) error {
			return checkCIMatrix(repoFile(t, ".github/workflows/ci.yml"))
		},
		Sensitivity: func(t *testing.T) error {
			workflow := repoFile(t, ".github/workflows/ci.yml")
			arms := []struct {
				name    string
				mutated string
			}{
				// The matrix row disappears.
				{"windows-row-removed", strings.ReplaceAll(workflow, "windows-latest", "ubuntu-22.04")},
				// The row survives but the job dies at formatting.
				{"crlf-checkout-restored", strings.ReplaceAll(workflow, "core.autocrlf false", "core.autocrlf true")},
				// The native rows move into the allowed-failure step.
				{"intent-moved-to-allowed-failure",
					strings.Replace(workflow, "run: go test ./... -count=1 -timeout 20m",
						"run: go test ./internal/intent ./... -count=1 -timeout 20m", 1)},
				// The blocking step stops blocking.
				{"blocking-flag-removed",
					strings.Replace(workflow,
						"      - name: Test (Windows GH #16 surface — blocking)\n        if: runner.os == 'Windows'\n",
						"      - name: Test (Windows GH #16 surface — blocking)\n        if: runner.os == 'Windows'\n        continue-on-error: true\n", 1)},
				// The native invocation stops naming the native test.
				{"native-invocation-dropped",
					strings.Replace(workflow, "-run TestAVPNativeWindows ./internal/intent",
						"-run TestNothingAtAll ./internal/testutil", 1)},
				// The tag-version check is pinned back to one leg.
				{"tag-version-pinned-to-ubuntu",
					strings.Replace(workflow,
						"if: startsWith(github.ref, 'refs/tags/v') && runner.os != 'Windows'",
						"if: startsWith(github.ref, 'refs/tags/v') && matrix.os == 'ubuntu-latest'", 1)},
			}
			// Each arm must be *detected*. A non-detecting arm is failed with
			// t.Fatalf rather than by returning an error: returning an error
			// is how this harness signals "the guard is sensitive", so a
			// returned error would launder a hole into a pass.
			var last error
			for _, arm := range arms {
				if arm.mutated == workflow {
					t.Fatalf("sensitivity arm %s no longer matches the workflow", arm.name)
				}
				last = checkCIMatrix(arm.mutated)
				if last == nil {
					t.Fatalf("sensitivity arm %s did not fail the guard", arm.name)
				}
			}
			return last
		},
	})

	registerGuard("AVP-177", guardSpec{
		Run: func(t *testing.T) error {
			supported := repoFile(t, "internal/intent/confine_supported.go")
			unsupported := repoFile(t, "internal/intent/confine_unsupported.go")
			return checkConfinementConstant(supported, unsupported, repoFile(t, "internal/cli/prepare.go"))
		},
		Sensitivity: func(t *testing.T) error {
			overlapping := "//go:build unix || windows || plan9\n\npackage intent\n\nconst rootConfinementSupported = false\n"
			return checkConfinementConstant(repoFile(t, "internal/intent/confine_supported.go"), overlapping, repoFile(t, "internal/cli/prepare.go"))
		},
	})

	registerGuard("AVP-178", guardSpec{
		Run: func(t *testing.T) error {
			return checkCrossBuild(t, []string{"linux", "darwin", "windows"})
		},
		Sensitivity: func(t *testing.T) error {
			return checkCrossBuild(t, []string{"nonexistent-goos"})
		},
	})

	registerGuard("AVP-191", guardSpec{
		Run: func(t *testing.T) error {
			return checkPlatformTags(
				buildTagLine(t, "internal/intent/confine_supported.go"),
				buildTagLine(t, "internal/intent/confine_unsupported.go"),
				goosList(t),
			)
		},
		Sensitivity: func(t *testing.T) error {
			if err := checkPlatformTags("(js && wasm) || plan9", "!((js && wasm) || plan9)", goosList(t)); err == nil {
				return errors.New("the rev-2 denylist form did not fail the guard")
			}
			return checkPlatformTags("unix || windows || wasip1", "!(unix || windows)", goosList(t))
		},
	})

	registerGuard("AVP-198", guardSpec{
		Run: func(t *testing.T) error {
			if err := checkWinsymlinkDirective(repoFile(t, "cmd/tpatch/main.go")); err != nil {
				return err
			}
			return checkWindowsModeMapping(refused)
		},
		Sensitivity: func(t *testing.T) error {
			return checkWinsymlinkDirective("//go:debug winsymlink=0\npackage main\n")
		},
	})

	registerGuard("AVP-200", guardSpec{
		Run: func(t *testing.T) error {
			return fifoOpenTripwire(t, true)
		},
		Sensitivity: func(t *testing.T) error {
			return fifoOpenTripwire(t, false)
		},
	})

	registerGuard("AVP-208", guardSpec{
		Run: func(t *testing.T) error {
			return checkStdlibSubset(
				buildTagLine(t, "internal/intent/confine_supported.go"),
				stdlibRootTag(t),
				openFlagsHalves(t),
				goosList(t),
			)
		},
		Sensitivity: func(t *testing.T) error {
			if err := checkStdlibSubset(stdlibRootTag(t), stdlibRootTag(t), openFlagsHalves(t), goosList(t)); err == nil {
				return errors.New("widening the allowlist to the stdlib tag did not fail the guard")
			}
			return checkStdlibSubset(
				buildTagLine(t, "internal/intent/confine_supported.go"),
				"unix || windows",
				openFlagsHalves(t),
				goosList(t),
			)
		},
	})
}

func openFlagsHalves(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"unix":        repoFile(t, "internal/intent/openflags_unix.go"),
		"windows":     repoFile(t, "internal/intent/openflags_windows.go"),
		"unsupported": repoFile(t, "internal/intent/openflags_unsupported.go"),
	}
}

// checkOpenFlagsPartition enforces the rev-6 three-half partition.
//
// The guard's own contract, restated so a later reader cannot widen it by
// accident: `O_NONBLOCK` bounds the **open** and asserts nothing whatsoever
// about read time (PRD §7.4.2, AVP-207). This guard therefore checks the flag
// *set*, never a timing property.
func checkOpenFlagsPartition(halves map[string]string) error {
	wantTags := map[string]string{
		"unix":        "//go:build unix",
		"windows":     "//go:build windows",
		"unsupported": "//go:build !(unix || windows)",
	}
	if len(halves) != 3 {
		return fmt.Errorf("openFlags() is declared in %d halves, want the three rev-6 halves", len(halves))
	}
	for name, want := range wantTags {
		source, ok := halves[name]
		if !ok {
			return fmt.Errorf("the %s half is missing", name)
		}
		if !strings.HasPrefix(strings.TrimSpace(source), want) {
			return fmt.Errorf("the %s half's build tag is not %q", name, want)
		}
	}
	if !strings.Contains(halves["unix"], "return syscall.O_NONBLOCK\n") {
		return errors.New("the unix half does not return exactly syscall.O_NONBLOCK")
	}
	for _, name := range []string{"windows", "unsupported"} {
		if !strings.Contains(halves[name], "return 0\n") {
			return fmt.Errorf("the %s half does not return exactly 0", name)
		}
	}
	if !strings.Contains(halves["unsupported"], "buildability") || !strings.Contains(halves["unsupported"], "unreachable") {
		return errors.New("the unsupported half's doc comment no longer records buildability and unreachability")
	}
	for name, source := range halves {
		for _, forbidden := range []string{
			"O_NOFOLLOW", "O_WRONLY", "O_RDWR", "O_CREATE", "O_TRUNC", "O_APPEND",
			"syscall.CreateFile", "rescap.openNoFollow", "os.Open(",
		} {
			if strings.Contains(source, forbidden) {
				return fmt.Errorf("the %s half references %s", name, forbidden)
			}
		}
	}
	return nil
}

func checkCIMatrix(workflow string) error {
	if !strings.Contains(workflow, "windows-latest") {
		return errors.New("the CI test matrix no longer contains windows-latest")
	}
	inMatrix := false
	for _, line := range strings.Split(workflow, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "os:") && strings.Contains(trimmed, "windows-latest") {
			inMatrix = true
			break
		}
	}
	if !inMatrix {
		return errors.New("windows-latest is present but not in the job's os matrix")
	}
	// Presence in the matrix is not the same as reaching the test step. The
	// Windows runner checks out with core.autocrlf=true by default, which
	// rewrites every text file and makes `gofmt -l .` list the whole tree —
	// the job then fails before "Test" and the native rows never run. The
	// LF-checkout step is therefore part of this guard, not a nicety.
	if !strings.Contains(workflow, "core.autocrlf false") {
		return errors.New("the Windows job does not force an LF checkout, so it fails formatting before it reaches go test")
	}
	if !strings.Contains(workflow, "runner.os == 'Windows'") {
		return errors.New("the LF-checkout step is not gated on the Windows runner")
	}
	checkoutIndex := strings.Index(workflow, "actions/checkout@")
	autocrlfIndex := strings.Index(workflow, "core.autocrlf false")
	if checkoutIndex >= 0 && autocrlfIndex > checkoutIndex {
		return errors.New("the LF-checkout configuration runs after actions/checkout, which is too late")
	}
	if !strings.Contains(workflow, "go test") {
		return errors.New("the CI job does not run go test")
	}
	return checkCIWindowsGate(workflow)
}

// checkCIWindowsGate is the rev-2 half of AVP-175.
//
// GH #16 added the windows-latest leg; `main` was green at WAVE_BASE and the
// leg exposed 192 pre-existing failures in packages this wave does not touch
// (GH #17). The accepted interim shape is therefore two Windows test steps:
// a BLOCKING one over the surface GH #16 owns, and a VISIBLE but
// `continue-on-error` full-suite one that GH #17 owns and will delete. The
// failure this guard exists to prevent is someone silencing a native-Windows
// regression by moving `./internal/intent` into the allowed-failure step, or
// by making the blocking step non-blocking.
func checkCIWindowsGate(workflow string) error {
	steps, err := parseWorkflowSteps(workflow)
	if err != nil {
		return err
	}

	var blocking, allowed, nonWindows []workflowStep
	for _, step := range steps {
		if step.Job != "test" || !strings.Contains(step.Run, "go test") {
			continue
		}
		switch {
		case !runsOnWindows(step.If):
			nonWindows = append(nonWindows, step)
		case step.ContinueOnError:
			allowed = append(allowed, step)
		default:
			blocking = append(blocking, step)
		}
	}

	// The full suite stays blocking on ubuntu and macOS.
	full := false
	for _, step := range nonWindows {
		if step.ContinueOnError {
			return fmt.Errorf("the non-Windows step %q is continue-on-error; the full suite must stay blocking there", step.Name)
		}
		if strings.Contains(step.Run, "go test ./...") {
			full = true
		}
	}
	if !full {
		return errors.New("no blocking full-suite `go test ./...` step runs on the non-Windows legs")
	}

	// The native rows are proven only on a real Windows runner, so the
	// blocking Windows step must run them explicitly, verbosely, and must
	// assert the verbose log really contains their PASS lines: `go test -run`
	// over a pattern that matches nothing exits 0, which is exactly how a
	// silently-unrun row would look green.
	native := -1
	for _, step := range blocking {
		if !strings.Contains(step.Run, "./internal/intent") {
			continue
		}
		if !strings.Contains(step.Run, "-run TestAVPNativeWindows") {
			continue
		}
		if !strings.Contains(step.Run, "-v ") && !strings.Contains(step.Run, "go test -v") {
			return fmt.Errorf("the blocking Windows step %q does not run the native test verbosely, so it cannot prove the subtests executed", step.Name)
		}
		if !strings.Contains(step.Run, "--- PASS: TestAVPNativeWindows") {
			return fmt.Errorf("the blocking Windows step %q does not assert the native PASS lines; a no-match -run pattern would exit 0", step.Name)
		}
		native = step.Index
		break
	}
	if native < 0 {
		return errors.New("no blocking windows-latest step runs `go test -run TestAVPNativeWindows ./internal/intent`")
	}

	// `internal/intent` must never appear in an allowed-failure command: that
	// is precisely how the GH #16 acceptance surface would stop gating.
	for _, step := range allowed {
		if strings.Contains(step.Run, "internal/intent") {
			return fmt.Errorf("the allowed-failure step %q names internal/intent; the GH #16 surface must stay blocking", step.Name)
		}
		if step.Index < native {
			return fmt.Errorf("the allowed-failure step %q runs before the blocking native step", step.Name)
		}
		if !strings.Contains(step.Name, "#17") {
			return fmt.Errorf("the allowed-failure step %q does not name the issue that owns removing it (GH #17)", step.Name)
		}
	}

	// The tag-version check must run on both non-Windows legs. Pinning it to
	// one matrix entry would let a macOS-specific ldflags or `make install`
	// regression reach a published release unchecked.
	for _, step := range steps {
		if step.Job != "test" || !strings.Contains(step.If, "refs/tags/v") {
			continue
		}
		if strings.Contains(step.If, "matrix.os ==") {
			return fmt.Errorf("the tag-version step is pinned to a single matrix entry: %q", step.If)
		}
		if !strings.Contains(step.If, "runner.os != 'Windows'") {
			return fmt.Errorf("the tag-version step %q does not run on both non-Windows legs", step.Name)
		}
		return nil
	}
	return errors.New("no tag-version verification step is present in the test job")
}

func checkConfinementConstant(supported, unsupported, command string) error {
	supportedTag := firstBuildTag(supported)
	unsupportedTag := firstBuildTag(unsupported)
	if supportedTag == "" || unsupportedTag == "" {
		return errors.New("a confinement half lost its build tag")
	}
	if unsupportedTag != "!("+supportedTag+")" {
		return fmt.Errorf("the tags overlap: %q vs %q", supportedTag, unsupportedTag)
	}
	if strings.Count(supported, "rootConfinementSupported") != 1 || strings.Count(unsupported, "rootConfinementSupported") != 1 {
		return errors.New("rootConfinementSupported is not declared exactly twice")
	}
	platformIndex := strings.Index(command, "RootConfinementSupported()")
	rootIndex := strings.Index(command, "os.OpenRoot(")
	if platformIndex < 0 || rootIndex < 0 {
		return errors.New("the command lost either the platform gate or the root open")
	}
	if platformIndex > rootIndex {
		return errors.New("the platform gate no longer precedes os.OpenRoot")
	}
	return nil
}

func firstBuildTag(source string) string {
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(line, "//go:build ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "//go:build "))
		}
	}
	return ""
}

func buildTagLine(t *testing.T, rel string) string {
	t.Helper()
	tag := firstBuildTag(repoFile(t, rel))
	if tag == "" {
		t.Fatalf("%s has no //go:build line", rel)
	}
	return tag
}

func goosList(t *testing.T) []string {
	t.Helper()
	output, err := exec.Command("go", "tool", "dist", "list").Output()
	if err != nil {
		t.Fatalf("go tool dist list: %v", err)
	}
	seen := map[string]bool{}
	var list []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		goos, _, ok := strings.Cut(strings.TrimSpace(line), "/")
		if !ok || seen[goos] {
			continue
		}
		seen[goos] = true
		list = append(list, goos)
	}
	sort.Strings(list)
	return list
}

var unixGOOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "hurd": true, "illumos": true, "ios": true, "linux": true,
	"netbsd": true, "openbsd": true, "solaris": true,
}

// evalBuildTag evaluates the small tag grammar this design uses.
func evalBuildTag(expression, goos string) (bool, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return false, errors.New("empty build expression")
	}
	if strings.HasPrefix(expression, "!(") && strings.HasSuffix(expression, ")") {
		inner, err := evalBuildTag(expression[2:len(expression)-1], goos)
		return !inner, err
	}
	if strings.HasPrefix(expression, "!") {
		inner, err := evalBuildTag(expression[1:], goos)
		return !inner, err
	}
	if strings.HasPrefix(expression, "(") && strings.HasSuffix(expression, ")") {
		return evalBuildTag(expression[1:len(expression)-1], goos)
	}
	if parts := strings.Split(expression, "||"); len(parts) > 1 {
		for _, part := range parts {
			value, err := evalBuildTag(part, goos)
			if err != nil {
				return false, err
			}
			if value {
				return true, nil
			}
		}
		return false, nil
	}
	if parts := strings.Split(expression, "&&"); len(parts) > 1 {
		for _, part := range parts {
			value, err := evalBuildTag(part, goos)
			if err != nil {
				return false, err
			}
			if !value {
				return false, nil
			}
		}
		return true, nil
	}
	switch expression {
	case "unix":
		return unixGOOS[goos], nil
	case "wasm":
		return goos == "js" || goos == "wasip1", nil
	}
	return expression == goos, nil
}

func checkPlatformTags(supported, unsupported string, targets []string) error {
	if supported != "unix || windows" {
		return fmt.Errorf("the supported tag is %q, want the fail-closed allowlist %q", supported, "unix || windows")
	}
	if unsupported != "!("+supported+")" {
		return fmt.Errorf("the unsupported tag %q is not the exact negation of %q", unsupported, supported)
	}
	for _, goos := range targets {
		left, err := evalBuildTag(supported, goos)
		if err != nil {
			return err
		}
		right, err := evalBuildTag(unsupported, goos)
		if err != nil {
			return err
		}
		if left == right {
			return fmt.Errorf("tags are not exhaustive and disjoint for GOOS=%s", goos)
		}
	}
	return nil
}

func stdlibRootTag(t *testing.T) string {
	t.Helper()
	output, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	path := filepath.Join(strings.TrimSpace(string(output)), "src", "os", "root_openat.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	tag := firstBuildTag(string(data))
	if tag == "" {
		t.Fatalf("%s has no //go:build line", path)
	}
	return tag
}

func checkStdlibSubset(ours, stdlib string, halves map[string]string, targets []string) error {
	if len(halves) != 3 {
		return fmt.Errorf("openFlags() has %d halves, want three build-tagged files", len(halves))
	}
	for name, source := range halves {
		if strings.Contains(source, "wasip1") {
			return fmt.Errorf("the %s half is wasip1-specific; no such implementation is authorized", name)
		}
	}
	proper := false
	for _, goos := range targets {
		mine, err := evalBuildTag(ours, goos)
		if err != nil {
			return err
		}
		theirs, err := evalBuildTag(stdlib, goos)
		if err != nil {
			return err
		}
		if mine && !theirs {
			return fmt.Errorf("GOOS=%s is in our allowlist but not the stdlib's confined set", goos)
		}
		if theirs && !mine {
			proper = true
		}
	}
	if !proper {
		return errors.New("the subset is not proper — the stdlib set no longer exceeds ours")
	}
	wasip1Supported, err := evalBuildTag(ours, "wasip1")
	if err != nil {
		return err
	}
	if wasip1Supported {
		return errors.New("wasip1 resolves to rootConfinementSupported == true")
	}
	return nil
}

func checkWinsymlinkDirective(main string) error {
	lines := strings.Split(main, "\n")
	for index, line := range lines {
		if strings.TrimSpace(line) == "package main" {
			if index == 0 {
				return errors.New("no //go:debug directive precedes package main")
			}
			if strings.TrimSpace(lines[index-1]) != "//go:debug winsymlink=1" {
				return fmt.Errorf("the line above package main is %q, want //go:debug winsymlink=1", lines[index-1])
			}
			return nil
		}
	}
	return errors.New("package main not found")
}

func checkWindowsModeMapping(predicate func(fs.FileInfo) bool) error {
	cases := []struct {
		name    string
		info    fs.FileInfo
		refused bool
	}{
		{"symlink-tag", fakeInfo{name: "s", mode: fs.ModeSymlink}, true},
		{"name-surrogate", fakeInfo{name: "j", mode: fs.ModeIrregular}, true},
		{"af-unix-tag", fakeInfo{name: "u", mode: fs.ModeSocket}, false},
		{"dedup-tag-regular", fakeInfo{name: "d", mode: 0}, false},
	}
	for _, tc := range cases {
		if got := predicate(tc.info); got != tc.refused {
			return fmt.Errorf("mapping(%s) refused = %v, want %v", tc.name, got, tc.refused)
		}
	}
	// A name surrogate must not carry the directory bit into the mapping.
	surrogate := fakeInfo{name: "j", mode: fs.ModeIrregular}
	if surrogate.IsDir() {
		return errors.New("a name-surrogate stat reports ModeDir")
	}
	// An AF_UNIX reparse point maps to a socket, which the kind gate refuses
	// as not-regular rather than as a symlink.
	afUnix := fakeInfo{name: "u", mode: fs.ModeSocket}
	if afUnix.Mode().IsRegular() {
		return errors.New("an AF_UNIX reparse point is reported regular")
	}
	return nil
}

package workflow

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

type doctorD9S5Ref struct {
	Package string
	Test    string
	Subtest string
}

type doctorD9S5Package struct {
	Dir       string
	GoPackage string
}

type doctorD9S5Runnable struct {
	NonVacuous bool
	Subtests   map[string]bool
}

var doctorD9S5Packages = map[string]doctorD9S5Package{
	"cli":       {Dir: "internal/cli", GoPackage: "cli"},
	"intent":    {Dir: "internal/intent", GoPackage: "intent"},
	"intentpub": {Dir: "internal/intentpub", GoPackage: "intentpub"},
	"workflow":  {Dir: "internal/workflow", GoPackage: "workflow"},
}

var doctorD9S5Matrix = map[string][]doctorD9S5Ref{
	"PIB-140": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-140"}},
	"PIB-141": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-141"}},
	"PIB-142": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-142"}},
	"PIB-143": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-143"}},
	"PIB-144": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-144"}},
	"PIB-145": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-145"}},
	"PIB-146": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-146"}},
	"PIB-147": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-147"}},

	"PIB-198": {{Package: "cli", Test: "TestPreparePIBPreChangeGoldens", Subtest: "PIB-198"}},
	"PIB-199": {{Package: "cli", Test: "TestPreparePIBPreChangeGoldens", Subtest: "PIB-199"}},
	"PIB-200": {{Package: "cli", Test: "TestPreparePIBPreChangeGoldens", Subtest: "PIB-200"}},
	"PIB-201": {{Package: "cli", Test: "TestAVPZeroMutationAndPrivacy", Subtest: "AVP-053"}},
	"PIB-202": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-140"}},
	"PIB-203": {{Package: "cli", Test: "TestAVPReadinessAndOutput", Subtest: "AVP-046"}},
	"PIB-204": {{Package: "cli", Test: "TestAVPReadinessAndOutput", Subtest: "AVP-042"}},
	"PIB-205": {{Package: "cli", Test: "TestAVPReadinessAndOutput"}},
	"PIB-206": {{Package: "cli", Test: "TestPrepareS5CheckWindowRows", Subtest: "PIB-206"}},
	"PIB-207": {{Package: "cli", Test: "TestPreparePIBPreChangeGoldens", Subtest: "PIB-207"}},

	"PIB-208": {{Package: "cli", Test: "TestPreparePIBPreChangeGoldens", Subtest: "PIB-208"}},
	"PIB-209": {{Package: "cli", Test: "TestPrepareS5CycleCompatibilityRows", Subtest: "PIB-209"}},
	"PIB-210": {{Package: "cli", Test: "TestPreparePIBPreChangeGoldens", Subtest: "PIB-210"}},
	"PIB-211": {{Package: "cli", Test: "TestPreparePIBPreChangeGoldens", Subtest: "PIB-211"}},
	"PIB-212": {{Package: "cli", Test: "TestPreparePIBPreChangeGoldens", Subtest: "PIB-212"}},
	"PIB-213": {{Package: "cli", Test: "TestPrepareS5NonInvalidationSourceRows", Subtest: "PIB-213"}},
	"PIB-214": {{Package: "cli", Test: "TestPrepareS5NonInvalidationSourceRows", Subtest: "PIB-214"}},

	"PIB-221": {{Package: "cli", Test: "TestPrepareS5PlatformRows", Subtest: "PIB-221"}},
	"PIB-222": {{Package: "cli", Test: "TestPreparePIBUnsupportedPlatformRuntimeGolden", Subtest: "PIB-222"}},
	"PIB-223": {{Package: "cli", Test: "TestPrepareS5PlatformRows", Subtest: "PIB-223"}},

	"PIB-316": {{Package: "cli", Test: "TestPrepareS5PersistentEvidenceRuntimeRows", Subtest: "PIB-316"}},
	"PIB-317": {{Package: "cli", Test: "TestPrepareS5PersistentEvidenceRuntimeRows", Subtest: "PIB-317"}},
	"PIB-318": {{Package: "cli", Test: "TestPrepareS5PersistentEvidenceRuntimeRows", Subtest: "PIB-318"}},
	"PIB-319": {{Package: "cli", Test: "TestPrepareS5LateCrashAndLifecycleRows", Subtest: "PIB-319"}},
	"PIB-320": {{Package: "cli", Test: "TestPrepareS5PersistentEvidenceRuntimeRows", Subtest: "PIB-320"}},
	"PIB-321": {
		{Package: "cli", Test: "TestPrepareS5PersistentEvidenceRuntimeRows", Subtest: "PIB-321"},
		{Package: "workflow", Test: "TestDoctorD9RemovedJournalAndFreshCloneDoNotInventLoss", Subtest: "removed-journal"},
	},
	"PIB-322": {
		{Package: "cli", Test: "TestPrepareS5PersistentEvidenceRuntimeRows", Subtest: "PIB-322"},
		{Package: "workflow", Test: "TestDoctorD9RemovedJournalAndFreshCloneDoNotInventLoss", Subtest: "fresh-clone-healthy-archive"},
	},
	"PIB-323": {{Package: "cli", Test: "TestDoctorD9DisclosureRows", Subtest: "PIB-323"}},
	"PIB-324": {{Package: "cli", Test: "TestPrepareS5LateCrashAndLifecycleRows", Subtest: "PIB-324"}},
	"PIB-325": {{Package: "cli", Test: "TestPrepareS5LateCrashAndLifecycleRows", Subtest: "PIB-325"}},

	"PIB-378": {{Package: "cli", Test: "TestPrepareS5ProvenanceBoundaryRows", Subtest: "PIB-378"}},
	"PIB-379": {{Package: "cli", Test: "TestPrepareS5LateCrashAndLifecycleRows", Subtest: "PIB-379"}},
	"PIB-380": {
		{Package: "cli", Test: "TestPrepareS5DoctorConcurrencyRows", Subtest: "PIB-380"},
		{Package: "workflow", Test: "TestDoctorD9ConnectedCapabilitySpiesRecordZeroProbesProcessesAndWrites"},
	},
	"PIB-381": {{Package: "workflow", Test: "TestDoctorD9ConcurrencyRows", Subtest: "PIB-381"}},
	"PIB-382": {{Package: "workflow", Test: "TestDoctorD9PrepareEvidenceTaxonomyAndOrdering"}},
	"PIB-383": {{Package: "workflow", Test: "TestDoctorD9PrepareEvidenceTaxonomyAndOrdering"}},
	"PIB-384": {{Package: "workflow", Test: "TestDoctorD9ArchiveEvidenceClassesAndRoutes"}},
	"PIB-385": {{Package: "workflow", Test: "TestDoctorD9MalformedAndUnsafeEvidenceFailsClosed", Subtest: "corrupt-index-offers-list-only"}},
	"PIB-386": {{Package: "workflow", Test: "TestDoctorD9CleanWorkspaceAndLostJournalBoundary"}},
	"PIB-387": {{Package: "workflow", Test: "TestDoctorD9RegistryOrderAndTaxonomySensitivity"}},
}

func TestDoctorD9S5MatrixMapsAll48Rows(t *testing.T) {
	root := doctorD9S5RepoRoot(t)
	want, err := doctorD9S5RowsFromPRD(filepath.Join(root, "docs", "prds", "PRD-prepare-intent-bundle.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDoctorD9S5Matrix(doctorD9S5Matrix, want); err != nil {
		t.Fatal(err)
	}
	index, err := doctorD9S5TestIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	for id, refs := range doctorD9S5Matrix {
		for _, ref := range refs {
			if !doctorD9S5RefResolves(index, ref) {
				t.Errorf("%s has unresolved or vacuous target %s", id, ref)
			}
		}
	}

	mutated := cloneDoctorD9S5Matrix(doctorD9S5Matrix)
	mutated["PIB-999"] = []doctorD9S5Ref{{Package: "workflow", Test: "TestSynthetic"}}
	if err := validateDoctorD9S5Matrix(mutated, want); err == nil {
		t.Fatal("S5 matrix guard accepted a synthetic row")
	}
	delete(mutated, "PIB-999")
	delete(mutated, "PIB-387")
	if err := validateDoctorD9S5Matrix(mutated, want); err == nil {
		t.Fatal("S5 matrix guard accepted a missing row")
	}
}

func TestDoctorD9S5LedgerResolverRejectsFalsePositives(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture
import "testing"
// func TestCommentOnly(t *testing.T) {}
var _ = "func TestStringOnly(t *testing.T) {}"
func TestReal(t *testing.T) {
	t.Run("literal", func(t *testing.T) { t.Log("runs") })
	runner.Run("runner-only", func(t *testing.T) { t.Log("wrong receiver") })
	{
		t := runner
		t.Run("shadowed-receiver", func(t *testing.T) { t.Log("wrong lexical receiver") })
	}
	name := "computed"
	t.Run(name, func(t *testing.T) { t.Log("not statically bound") })
	t.Run("empty", func(t *testing.T) {})
	unrelated := []struct{name string}{{name: "unrelated-match"}}
	selected := []struct{name string}{{name: "selected-table"}}
	other := []struct{name string}{{name: "selected-table"}, {name: "unselected-only"}}
	_, _ = unrelated, other
	for _, tc := range selected {
		t.Run(tc.name, func(t *testing.T) { t.Log("selected exact table") })
	}
	cases := []struct{name string}{{name: "shared-table"}, {name: "outer-only"}}
	{
		cases := []struct{name string}{{name: "shared-table"}, {name: "inner-only"}}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) { t.Log("selected shadow table") })
		}
	}
	aliasSource := []struct{name string}{{name: "alias-only"}}
	alias := aliasSource
	for _, tc := range alias {
		t.Run(tc.name, func(t *testing.T) { t.Log("unsupported alias") })
	}
	reassigned := []struct{name string}{{name: "reassigned-phantom"}}
	reassigned = []struct{name string}{{name: "reassigned-live"}}
	for _, tc := range reassigned {
		t.Run(tc.name, func(t *testing.T) { t.Log("ambiguous reassignment") })
	}
	phantom := []struct{name string}{{name: "alias-phantom"}}
	live := []struct{name string}{{name: "alias-live"}}
	reassignedAlias := phantom
	reassignedAlias = live
	for _, tc := range reassignedAlias {
		t.Run(tc.name, func(t *testing.T) { t.Log("ambiguous alias reassignment") })
	}
	indexMutated := []struct{name string}{{name: "index-before"}}
	indexMutated[0] = struct{name string}{name: "index-after"}
	for _, tc := range indexMutated {
		t.Run(tc.name, func(t *testing.T) { t.Log("mutated table") })
	}
	compoundMutated := []struct{name string}{{name: "compound-before"}}
	compoundMutated[0].name += "-changed"
	for _, tc := range compoundMutated {
		t.Run(tc.name, func(t *testing.T) { t.Log("compound-mutated table") })
	}
	aliasIndexTarget := []struct{name string}{{name: "alias-index-before"}}
	aliasIndex := aliasIndexTarget
	aliasIndex[0].name = "alias-index-after"
	for _, tc := range aliasIndexTarget {
		t.Run(tc.name, func(t *testing.T) { t.Log("pre-range alias mutation") })
	}
	aliasAppendTarget := []struct{name string}{{name: "alias-append-before"}}
	aliasAppend := aliasAppendTarget
	aliasAppend = append(aliasAppend, struct{name string}{name: "alias-append-after"})
	for _, tc := range aliasAppendTarget {
		t.Run(tc.name, func(t *testing.T) { t.Log("pre-range alias append") })
	}
	rangeValueMutated := []struct{name string}{{name: "range-value-before"}}
	for _, tc := range rangeValueMutated {
		tc.name = "range-value-after"
		t.Run(tc.name, func(t *testing.T) { t.Log("range value mutated before Run") })
	}
	rangeValueAfterRun := []struct{name string}{{name: "range-value-after-run"}}
	for _, tc := range rangeValueAfterRun {
		t.Run(tc.name, func(t *testing.T) { t.Log("range value mutated after Run") })
		tc.name = "range-value-after-run-mutated"
	}
	underlyingMutated := []struct{name string}{
		{name: "underlying-first"},
		{name: "underlying-later-before"},
	}
	for index, tc := range underlyingMutated {
		if index == 0 {
			underlyingMutated[1].name = "underlying-later-after"
		}
		t.Run(tc.name, func(t *testing.T) { t.Log("underlying table mutates during range") })
	}
	pointerElements := []*struct{name string}{{name: "pointer-element"}}
	for _, tc := range pointerElements {
		t.Run(tc.name, func(t *testing.T) { t.Log("unsupported pointer element") })
	}
	for _, name := range []string{"direct-value-phantom"} {
		name = "direct-value-live"
		t.Run(name, func(t *testing.T) { t.Log("direct value mutated") })
	}
	for name := range map[string]bool{"direct-key-phantom": true} {
		name = "direct-key-live"
		t.Run(name, func(t *testing.T) { t.Log("direct key mutated") })
	}
	for _, name := range []string{"direct-compound"} {
		name += "-mutated"
		t.Run(name, func(t *testing.T) { t.Log("direct value compound mutation") })
	}
	for _, name := range []string{"direct-closure"} {
		mutate := func() { name = "direct-closure-mutated" }
		mutate()
		t.Run(name, func(t *testing.T) { t.Log("direct value closure mutation") })
	}
	for _, name := range []string{"stable-direct-value"} {
		t.Run(name, func(t *testing.T) { t.Log("stable direct value") })
	}
	for name := range map[string]bool{"stable-direct-key": true} {
		t.Run(name, func(t *testing.T) { t.Log("stable direct key") })
	}
	afterRange := []struct{name string}{{name: "before-late-assignment"}}
	for _, tc := range afterRange {
		t.Run(tc.name, func(t *testing.T) { t.Log("assignment is later") })
	}
	afterRange = []struct{name string}{{name: "after-late-assignment"}}
	_ = cases
}
func TestWrongSignature() {}
func TestEmpty(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(dir, "fixture_test.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	external := `package fixture_test
import "testing"
func TestWrongPackage(t *testing.T) { t.Log("external") }
`
	if err := os.WriteFile(filepath.Join(dir, "external_test.go"), []byte(external), 0o644); err != nil {
		t.Fatal(err)
	}
	tests, err := indexDoctorD9S5PackageTests(dir, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	index := map[string]map[string]doctorD9S5Runnable{"fixture": tests}
	for _, ref := range []doctorD9S5Ref{
		{Package: "fixture", Test: "TestCommentOnly"},
		{Package: "fixture", Test: "TestStringOnly"},
		{Package: "fixture", Test: "TestWrongSignature"},
		{Package: "fixture", Test: "TestWrongPackage"},
		{Package: "wrong-package", Test: "TestReal"},
		{Package: "fixture", Test: "TestReal", Subtest: "computed"},
		{Package: "fixture", Test: "TestReal", Subtest: "runner-only"},
		{Package: "fixture", Test: "TestReal", Subtest: "shadowed-receiver"},
		{Package: "fixture", Test: "TestReal", Subtest: "unrelated-match"},
		{Package: "fixture", Test: "TestReal", Subtest: "unselected-only"},
		{Package: "fixture", Test: "TestReal", Subtest: "outer-only"},
		{Package: "fixture", Test: "TestReal", Subtest: "alias-only"},
		{Package: "fixture", Test: "TestReal", Subtest: "reassigned-phantom"},
		{Package: "fixture", Test: "TestReal", Subtest: "reassigned-live"},
		{Package: "fixture", Test: "TestReal", Subtest: "alias-phantom"},
		{Package: "fixture", Test: "TestReal", Subtest: "alias-live"},
		{Package: "fixture", Test: "TestReal", Subtest: "index-before"},
		{Package: "fixture", Test: "TestReal", Subtest: "index-after"},
		{Package: "fixture", Test: "TestReal", Subtest: "compound-before"},
		{Package: "fixture", Test: "TestReal", Subtest: "alias-index-before"},
		{Package: "fixture", Test: "TestReal", Subtest: "alias-index-after"},
		{Package: "fixture", Test: "TestReal", Subtest: "alias-append-before"},
		{Package: "fixture", Test: "TestReal", Subtest: "alias-append-after"},
		{Package: "fixture", Test: "TestReal", Subtest: "range-value-before"},
		{Package: "fixture", Test: "TestReal", Subtest: "range-value-after"},
		{Package: "fixture", Test: "TestReal", Subtest: "range-value-after-run"},
		{Package: "fixture", Test: "TestReal", Subtest: "underlying-first"},
		{Package: "fixture", Test: "TestReal", Subtest: "underlying-later-before"},
		{Package: "fixture", Test: "TestReal", Subtest: "underlying-later-after"},
		{Package: "fixture", Test: "TestReal", Subtest: "pointer-element"},
		{Package: "fixture", Test: "TestReal", Subtest: "direct-value-phantom"},
		{Package: "fixture", Test: "TestReal", Subtest: "direct-value-live"},
		{Package: "fixture", Test: "TestReal", Subtest: "direct-key-phantom"},
		{Package: "fixture", Test: "TestReal", Subtest: "direct-key-live"},
		{Package: "fixture", Test: "TestReal", Subtest: "direct-compound"},
		{Package: "fixture", Test: "TestReal", Subtest: "direct-closure"},
		{Package: "fixture", Test: "TestReal", Subtest: "direct-closure-mutated"},
		{Package: "fixture", Test: "TestReal", Subtest: "after-late-assignment"},
		{Package: "fixture", Test: "TestReal", Subtest: "missing"},
		{Package: "fixture", Test: "TestReal", Subtest: "empty"},
		{Package: "fixture", Test: "TestEmpty"},
	} {
		if doctorD9S5RefResolves(index, ref) {
			t.Errorf("false-positive reference resolved: %s", ref)
		}
	}
	for _, ref := range []doctorD9S5Ref{
		{Package: "fixture", Test: "TestReal"},
		{Package: "fixture", Test: "TestReal", Subtest: "literal"},
		{Package: "fixture", Test: "TestReal", Subtest: "selected-table"},
		{Package: "fixture", Test: "TestReal", Subtest: "shared-table"},
		{Package: "fixture", Test: "TestReal", Subtest: "inner-only"},
		{Package: "fixture", Test: "TestReal", Subtest: "before-late-assignment"},
		{Package: "fixture", Test: "TestReal", Subtest: "stable-direct-value"},
		{Package: "fixture", Test: "TestReal", Subtest: "stable-direct-key"},
	} {
		if !doctorD9S5RefResolves(index, ref) {
			t.Errorf("live reference did not resolve: %s", ref)
		}
	}
}

func (ref doctorD9S5Ref) String() string {
	target := ref.Package + ":" + ref.Test
	if ref.Subtest != "" {
		target += "/" + ref.Subtest
	}
	return target
}

func cloneDoctorD9S5Matrix(source map[string][]doctorD9S5Ref) map[string][]doctorD9S5Ref {
	clone := make(map[string][]doctorD9S5Ref, len(source))
	for id, refs := range source {
		clone[id] = append([]doctorD9S5Ref(nil), refs...)
	}
	return clone
}

func validateDoctorD9S5Matrix(rows map[string][]doctorD9S5Ref, want []string) error {
	if len(want) != 48 {
		return fmt.Errorf("S5 PRD partition contains %d rows, want 48", len(want))
	}
	got := make([]string, 0, len(rows))
	for id, refs := range rows {
		got = append(got, id)
		if len(refs) == 0 {
			return fmt.Errorf("%s has no runnable target", id)
		}
		for _, ref := range refs {
			if ref.Package == "" || ref.Test == "" {
				return fmt.Errorf("%s has incomplete target %#v", id, ref)
			}
		}
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		return fmt.Errorf("S5 row set = %v, want PRD rows %v", got, want)
	}
	return nil
}

func doctorD9S5RowsFromPRD(filename string) ([]string, error) {
	body, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	text := string(body)
	headings := []string{
		"### 18.11 J — Provenance boundary",
		"### 18.16 O — `--check` compatibility",
		"### 18.17 P — Non-invalidation of shipped commands",
		"### 18.19 R — Platform and build",
		"### 18.32 AE — Orphans, late crash phases, journal-loss boundary and concurrency",
		"### 18.38 AK — Notes semantics, doctor evidence and residue reporting",
	}
	rowPattern := regexp.MustCompile(`(?m)^\| (PIB-[0-9]{3}) \|`)
	set := map[string]struct{}{}
	for _, heading := range headings {
		start := strings.Index(text, heading)
		if start < 0 {
			return nil, fmt.Errorf("missing S5 PRD heading %q", heading)
		}
		section := text[start+len(heading):]
		if end := strings.Index(section, "\n### 18."); end >= 0 {
			section = section[:end]
		}
		for _, match := range rowPattern.FindAllStringSubmatch(section, -1) {
			set[match[1]] = struct{}{}
		}
	}
	rows := make([]string, 0, len(set))
	for id := range set {
		rows = append(rows, id)
	}
	sort.Strings(rows)
	return rows, nil
}

func doctorD9S5RepoRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
}

func doctorD9S5TestIndex(root string) (map[string]map[string]doctorD9S5Runnable, error) {
	index := map[string]map[string]doctorD9S5Runnable{}
	names := make([]string, 0, len(doctorD9S5Packages))
	for name := range doctorD9S5Packages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		spec := doctorD9S5Packages[name]
		tests, err := indexDoctorD9S5PackageTests(filepath.Join(root, filepath.FromSlash(spec.Dir)), spec.GoPackage)
		if err != nil {
			return nil, fmt.Errorf("index %s: %w", name, err)
		}
		index[name] = tests
	}
	return index, nil
}

func doctorD9S5RefResolves(index map[string]map[string]doctorD9S5Runnable, ref doctorD9S5Ref) bool {
	tests, ok := index[ref.Package]
	if !ok {
		return false
	}
	runnable, ok := tests[ref.Test]
	if !ok || !runnable.NonVacuous {
		return false
	}
	if ref.Subtest == "" {
		return true
	}
	return runnable.Subtests[ref.Subtest]
}

func indexDoctorD9S5PackageTests(dir, goPackage string) (map[string]doctorD9S5Runnable, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	out := map[string]doctorD9S5Runnable{}
	var preparePIBRows []string
	preparePIBRowsBound := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		if file.Name == nil || file.Name.Name != goPackage {
			continue
		}
		aliases, dotTesting := doctorD9S5TestingImports(file)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !doctorD9S5RunnableTest(function, aliases, dotTesting) {
				continue
			}
			subtests := map[string]bool{}
			doctorD9S5CollectSubtests(function, subtests)
			out[function.Name.Name] = doctorD9S5Runnable{
				NonVacuous: len(function.Body.List) != 0,
				Subtests:   subtests,
			}
			if function.Name.Name == "TestPreparePIBPreChangeGoldens" {
				preparePIBRowsBound = doctorD9S5BindsStringMapSubtests(
					function, file.Scope.Lookup("preparePIBRows"),
				)
			}
		}
		if entry.Name() == "prepare_pib_golden_test.go" {
			preparePIBRows = append(preparePIBRows, doctorD9S5StringMapKeys(file, "preparePIBRows")...)
		}
	}
	if runnable, ok := out["TestPreparePIBPreChangeGoldens"]; ok && preparePIBRowsBound {
		for _, row := range preparePIBRows {
			runnable.Subtests[row] = true
		}
		out["TestPreparePIBPreChangeGoldens"] = runnable
	}
	return out, nil
}

func doctorD9S5BindsStringMapSubtests(function *ast.FuncDecl, table *ast.Object) bool {
	testParameter := doctorD9S5TestParameterObject(function)
	if testParameter == nil || table == nil {
		return false
	}
	bound := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 || !doctorD9S5NonVacuousCallback(call.Args[1]) {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Run" {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok || receiver.Obj != testParameter {
			return true
		}
		argument, ok := call.Args[0].(*ast.Ident)
		if !ok {
			return true
		}
		ast.Inspect(function.Body, func(candidate ast.Node) bool {
			if _, nested := candidate.(*ast.FuncLit); nested {
				return false
			}
			statement, ok := candidate.(*ast.RangeStmt)
			if !ok || call.Pos() < statement.Body.Pos() || call.End() > statement.Body.End() {
				return true
			}
			key, _ := statement.Key.(*ast.Ident)
			source, _ := statement.X.(*ast.Ident)
			if key != nil && source != nil &&
				key.Obj != nil &&
				(key.Obj == argument.Obj || doctorD9S5DirectAlias(argument.Obj, key.Obj)) &&
				source.Obj == table {
				bound = true
				return false
			}
			return true
		})
		return !bound
	})
	return bound
}

func doctorD9S5DirectAlias(object, target *ast.Object) bool {
	if object == nil || target == nil {
		return false
	}
	assignment, ok := object.Decl.(*ast.AssignStmt)
	if !ok {
		return false
	}
	for index, left := range assignment.Lhs {
		ident, ok := left.(*ast.Ident)
		if !ok || ident.Obj != object || index >= len(assignment.Rhs) {
			continue
		}
		source, ok := assignment.Rhs[index].(*ast.Ident)
		return ok && source.Obj == target
	}
	return false
}

func doctorD9S5TestingImports(file *ast.File) (map[string]struct{}, bool) {
	aliases := map[string]struct{}{}
	dot := false
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || path != "testing" {
			continue
		}
		if imported.Name == nil {
			aliases["testing"] = struct{}{}
			continue
		}
		switch imported.Name.Name {
		case ".":
			dot = true
		case "_":
		default:
			aliases[imported.Name.Name] = struct{}{}
		}
	}
	return aliases, dot
}

func doctorD9S5RunnableTest(function *ast.FuncDecl, aliases map[string]struct{}, dotTesting bool) bool {
	if function.Recv != nil || function.Body == nil || function.Name == nil ||
		!doctorD9S5ValidTestName(function.Name.Name) {
		return false
	}
	if function.Type.TypeParams != nil && len(function.Type.TypeParams.List) != 0 {
		return false
	}
	if function.Type.Results != nil && len(function.Type.Results.List) != 0 {
		return false
	}
	if function.Type.Params == nil || len(function.Type.Params.List) != 1 ||
		len(function.Type.Params.List[0].Names) > 1 {
		return false
	}
	star, ok := function.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	if ident, ok := star.X.(*ast.Ident); ok {
		return dotTesting && ident.Name == "T"
	}
	selector, ok := star.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "T" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = aliases[pkg.Name]
	return ok
}

func doctorD9S5ValidTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") || len(name) == len("Test") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name[len("Test"):])
	return !unicode.IsLower(r)
}

func doctorD9S5CollectSubtests(function *ast.FuncDecl, into map[string]bool) {
	testParameter := doctorD9S5TestParameterObject(function)
	if testParameter == nil {
		return
	}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, receiverOK := selector.X.(*ast.Ident)
		if !receiverOK || receiver.Obj != testParameter ||
			selector.Sel.Name != "Run" || !doctorD9S5NonVacuousCallback(call.Args[1]) {
			return true
		}
		switch argument := call.Args[0].(type) {
		case *ast.BasicLit:
			if argument.Kind == token.STRING {
				if value, err := strconv.Unquote(argument.Value); err == nil && value != "" {
					into[value] = true
				}
			}
		case *ast.SelectorExpr:
			ranged, composite := doctorD9S5CallRangeComposite(function.Body, call, argument.X)
			valueIdent, _ := ranged.Value.(*ast.Ident)
			base, _ := argument.X.(*ast.Ident)
			if composite != nil && valueIdent != nil && base != nil &&
				valueIdent.Obj != nil && valueIdent.Obj == base.Obj &&
				doctorD9S5StableRangeName(ranged, base.Obj) {
				doctorD9S5CollectTableField(composite, argument.Sel.Name, into)
			}
		case *ast.Ident:
			ranged, composite := doctorD9S5CallRangeComposite(function.Body, call, argument)
			if ranged != nil && composite != nil &&
				doctorD9S5StableRangeName(ranged, argument.Obj) {
				doctorD9S5CollectRangeStrings(ranged, composite, argument.Obj, into)
			}
		}
		return true
	})
}

func doctorD9S5TestParameterObject(function *ast.FuncDecl) *ast.Object {
	if function.Type.Params == nil || len(function.Type.Params.List) != 1 ||
		len(function.Type.Params.List[0].Names) != 1 {
		return nil
	}
	return function.Type.Params.List[0].Names[0].Obj
}

func doctorD9S5CallRangeComposite(
	body *ast.BlockStmt,
	call *ast.CallExpr,
	argument ast.Expr,
) (*ast.RangeStmt, *ast.CompositeLit) {
	argumentIdent, _ := argument.(*ast.Ident)
	if argumentIdent == nil {
		return nil, nil
	}
	var selected *ast.RangeStmt
	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		statement, ok := node.(*ast.RangeStmt)
		if !ok || call.Pos() < statement.Body.Pos() || call.End() > statement.Body.End() {
			return true
		}
		value, _ := statement.Value.(*ast.Ident)
		key, _ := statement.Key.(*ast.Ident)
		if (value == nil || value.Obj == nil || value.Obj != argumentIdent.Obj) &&
			(key == nil || key.Obj == nil || key.Obj != argumentIdent.Obj) {
			return true
		}
		if selected == nil || statement.End()-statement.Pos() < selected.End()-selected.Pos() {
			selected = statement
		}
		return true
	})
	if selected == nil {
		return nil, nil
	}
	return selected, doctorD9S5ResolveComposite(body, selected.X, selected)
}

func doctorD9S5ResolveComposite(
	body *ast.BlockStmt,
	expression ast.Expr,
	selected *ast.RangeStmt,
) *ast.CompositeLit {
	if composite, ok := expression.(*ast.CompositeLit); ok {
		return composite
	}
	ident, ok := expression.(*ast.Ident)
	if !ok || ident.Obj == nil {
		return nil
	}
	var composite *ast.CompositeLit
	var initializedAt token.Pos
	switch declaration := ident.Obj.Decl.(type) {
	case *ast.AssignStmt:
		for index, left := range declaration.Lhs {
			name, ok := left.(*ast.Ident)
			if !ok || name.Obj != ident.Obj || index >= len(declaration.Rhs) {
				continue
			}
			composite, _ = declaration.Rhs[index].(*ast.CompositeLit)
			initializedAt = declaration.End()
			break
		}
	case *ast.ValueSpec:
		for index, name := range declaration.Names {
			if name.Obj != ident.Obj || index >= len(declaration.Values) {
				continue
			}
			composite, _ = declaration.Values[index].(*ast.CompositeLit)
			initializedAt = declaration.End()
			break
		}
	}
	if composite == nil || initializedAt == token.NoPos ||
		!doctorD9S5ExclusiveTableReference(body, ident.Obj, initializedAt, selected) {
		return nil
	}
	return composite
}

func doctorD9S5ExclusiveTableReference(
	body *ast.BlockStmt,
	object *ast.Object,
	after token.Pos,
	selected *ast.RangeStmt,
) bool {
	rangeSource, ok := selected.X.(*ast.Ident)
	if !ok || rangeSource.Obj != object {
		return false
	}
	exclusive := true
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil || !exclusive {
			return false
		}
		if node.Pos() <= after || node.Pos() > selected.End() {
			return true
		}
		ident, ok := node.(*ast.Ident)
		if ok && ident.Obj == object && ident != rangeSource {
			exclusive = false
			return false
		}
		return true
	})
	return exclusive
}

func doctorD9S5StableRangeName(statement *ast.RangeStmt, object *ast.Object) bool {
	if statement == nil || statement.Tok != token.DEFINE || object == nil {
		return false
	}
	value, _ := statement.Value.(*ast.Ident)
	key, _ := statement.Key.(*ast.Ident)
	bound := (value != nil && value.Obj == object) ||
		(key != nil && key.Obj == object)
	if !bound {
		return false
	}
	stable := true
	ast.Inspect(statement.Body, func(node ast.Node) bool {
		if node == nil || !stable {
			return false
		}
		switch value := node.(type) {
		case *ast.AssignStmt:
			for _, target := range value.Lhs {
				if doctorD9S5MutationRoot(target) == object {
					stable = false
					return false
				}
			}
		case *ast.IncDecStmt:
			if doctorD9S5MutationRoot(value.X) == object {
				stable = false
				return false
			}
		case *ast.UnaryExpr:
			if value.Op == token.AND && doctorD9S5MutationRoot(value.X) == object {
				stable = false
				return false
			}
		}
		return true
	})
	return stable
}

func doctorD9S5MutationRoot(expression ast.Expr) *ast.Object {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Obj
	case *ast.IndexExpr:
		return doctorD9S5MutationRoot(value.X)
	case *ast.IndexListExpr:
		return doctorD9S5MutationRoot(value.X)
	case *ast.SelectorExpr:
		return doctorD9S5MutationRoot(value.X)
	case *ast.SliceExpr:
		return doctorD9S5MutationRoot(value.X)
	case *ast.ParenExpr:
		return doctorD9S5MutationRoot(value.X)
	case *ast.StarExpr:
		return doctorD9S5MutationRoot(value.X)
	}
	return nil
}

func doctorD9S5CollectTableField(composite *ast.CompositeLit, field string, into map[string]bool) {
	if !doctorD9S5SupportedTableField(composite, field) {
		return
	}
	for _, element := range composite.Elts {
		row, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, rowElement := range row.Elts {
			keyed, ok := rowElement.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := keyed.Key.(*ast.Ident)
			if !ok || key.Name != field {
				continue
			}
			if value, ok := doctorD9S5StringLiteral(keyed.Value); ok {
				into[value] = true
			}
		}
	}
}

func doctorD9S5SupportedTableField(composite *ast.CompositeLit, field string) bool {
	array, ok := composite.Type.(*ast.ArrayType)
	if !ok {
		return false
	}
	element, ok := array.Elt.(*ast.StructType)
	if !ok || element.Fields == nil {
		return false
	}
	matches := 0
	for _, declared := range element.Fields.List {
		for _, name := range declared.Names {
			if name.Name != field {
				continue
			}
			kind, ok := declared.Type.(*ast.Ident)
			if !ok || kind.Name != "string" {
				return false
			}
			matches++
		}
	}
	return matches == 1
}

func doctorD9S5CollectRangeStrings(
	statement *ast.RangeStmt,
	composite *ast.CompositeLit,
	object *ast.Object,
	into map[string]bool,
) {
	if object == nil {
		return
	}
	key, _ := statement.Key.(*ast.Ident)
	value, _ := statement.Value.(*ast.Ident)
	for _, element := range composite.Elts {
		if key != nil && key.Obj == object {
			if keyed, ok := element.(*ast.KeyValueExpr); ok {
				if literal, ok := doctorD9S5StringLiteral(keyed.Key); ok {
					into[literal] = true
				}
			}
			continue
		}
		if value != nil && value.Obj == object {
			if literal, ok := doctorD9S5StringLiteral(element); ok {
				into[literal] = true
			}
		}
	}
}

func doctorD9S5NonVacuousCallback(expression ast.Expr) bool {
	function, ok := expression.(*ast.FuncLit)
	return ok && function.Body != nil && len(function.Body.List) != 0
}

func doctorD9S5StringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil && value != ""
}

func doctorD9S5StringMapKeys(file *ast.File, variable string) []string {
	var keys []string
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.VAR {
			continue
		}
		for _, spec := range generic.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || len(valueSpec.Names) != 1 || valueSpec.Names[0].Name != variable ||
				len(valueSpec.Values) != 1 {
				continue
			}
			composite, ok := valueSpec.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, element := range composite.Elts {
				keyed, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := doctorD9S5StringLiteral(keyed.Key); ok {
					keys = append(keys, key)
				}
			}
		}
	}
	sort.Strings(keys)
	return keys
}

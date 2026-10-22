package workflow

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func rocS0ValidateChangedPaths(paths []string) error {
	allowed := map[string]bool{
		"docs/handoff/CURRENT.md":                                   true,
		"docs/handoff/HISTORY.md":                                   true,
		"docs/ROADMAP.md":                                           true,
		"docs/supervisor/LOG.md":                                    true,
		"internal/workflow/operation_candidate_s0_test.go":          true,
		"internal/workflow/operation_candidate_s0_guards_test.go":   true,
		"internal/workflow/operation_candidate_s0_fixtures_test.go": true,
		"internal/cli/operation_candidate_s0_test.go":               true,
		"internal/store/operation_candidate_s0_test.go":             true,
	}
	for _, path := range paths {
		if path != "" && filepath.ToSlash(filepath.Clean(path)) != path {
			return fmt.Errorf("non-canonical S0 path: %q", path)
		}
		if path == "" || allowed[path] {
			continue
		}
		if strings.HasPrefix(path, "internal/workflow/testdata/roc-s0/") {
			continue
		}
		return fmt.Errorf("S0 changed a non-test/public-contract path: %s", path)
	}
	return nil
}

func rocS0AssertTestOnlyScope(t *testing.T) {
	t.Helper()
	root := rgaS0RepoRoot(t)
	changed := git(t, root, "diff", "--no-renames", "--name-only", rocS0Baseline, "--")
	untracked := git(t, root, "ls-files", "--others", "--exclude-standard")
	paths := strings.Split(changed+"\n"+untracked, "\n")
	var ordinary []string
	for _, path := range paths {
		if path != rocS0MemoScopeFile {
			ordinary = append(ordinary, path)
			continue
		}
		before := gitOutput(t, root, "show", rocS0Baseline+":"+path)
		after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if err := rocS0ValidateMemoScopeChange(before, string(after)); err != nil {
			t.Fatal(err)
		}
	}
	if err := rocS0ValidateChangedPaths(ordinary); err != nil {
		t.Fatal(err)
	}
}

func TestROCS0NoProductionChanges(t *testing.T) {
	rocS0AssertTestOnlyScope(t)
}

func TestROCS0ScopeGuardSensitivity(t *testing.T) {
	for _, path := range []string{
		"internal/workflow/operation_candidate_s0_test.go",
		"internal/cli/operation_candidate_s0_test.go",
		"internal/store/operation_candidate_s0_test.go",
		"internal/workflow/testdata/roc-s0/phase1-confirmed.golden",
		"docs/handoff/CURRENT.md",
		"docs/handoff/HISTORY.md",
		"docs/ROADMAP.md",
		"docs/supervisor/LOG.md",
	} {
		if err := rocS0ValidateChangedPaths([]string{path}); err != nil {
			t.Fatalf("valid S0 path rejected: %v", err)
		}
	}
	for _, path := range []string{
		"internal/workflow/reconcile.go",
		"internal/workflow/new_candidate.go",
		"internal/workflow/unregistered_test.go",
		"internal/workflow/reconcile_evidence_integration_test.go",
		"internal/workflow/golden_reconcile_test.go",
		"internal/workflow/reconcile_canonical_safety_test.go",
		"internal/workflow/reconcile_test.go",
		"internal/workflow/resolver_test.go",
		"internal/cli/recipe_authority_s5_cli_test.go",
		"internal/cli/cobra_test.go",
		"internal/cli/prepare_s7_ar_guard_test.go",
		"internal/cli/prepare_s6_cache_scope_test.go",
		"internal/cli/prepare_s6_contract_test.go",
		"internal/workflow/testdata/rga-s0/adjacent-cli-args-conflict-2026-08/base.command.go.txt",
		"internal/cli/testdata/roc-s0/unregistered.json",
		"internal/store/types.go",
		"internal/cli/cobra.go",
		"cmd/tpatch/main.go",
		"assets/SKILL.md",
		"SPEC.md",
		"CHANGELOG.md",
		"go.mod",
		"go.sum",
		".github/workflows/ci.yml",
		"scripts/wave-close-test-shards.sh",
		"docs/adrs/ADR-037-reconcile-operation-replay-candidate-authority.md",
		"internal/workflow/testdata/roc-s0/../../reconcile.go",
	} {
		t.Run(path, func(t *testing.T) {
			if err := rocS0ValidateChangedPaths([]string{path}); err == nil {
				t.Fatalf("scope guard accepted %s", path)
			}
		})
	}
}

func rocS0ValidateCaseInventory(names []string) error {
	want := []string{
		"phase1-confirmed", "phase15-confirmed", "phase2-legacy-rejected",
		"phase2-conflict-fallthrough", "phase2-applicable-only", "phase2-mixed",
		"phase3-rejected", "phase35-shadow-awaiting", "phase4-strict",
		"phase4-threeway-clean", "phase4-conflicts", "multi-slug", "cumulative-legacy",
	}
	got := append([]string(nil), names...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("S0 runtime fixture inventory changed: %v", names)
	}
	return nil
}

func TestROCS0RuntimeInventorySensitivity(t *testing.T) {
	if err := rocS0ValidateCaseInventory(rocS0RuntimeCases); err != nil {
		t.Fatal(err)
	}
	for _, mutant := range [][]string{
		nil,
		rocS0RuntimeCases[1:],
		append(append([]string(nil), rocS0RuntimeCases...), rocS0RuntimeCases[0]),
		append(append([]string(nil), rocS0RuntimeCases[1:]...), "unregistered"),
	} {
		if err := rocS0ValidateCaseInventory(mutant); err == nil {
			t.Fatalf("inventory guard accepted %v", mutant)
		}
	}
}

func TestROCS0GoldenByteGuardSensitivity(t *testing.T) {
	for _, name := range rocS0RuntimeCases {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("testdata", "roc-s0", name+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			if len(body) == 0 {
				t.Fatal("empty baseline golden")
			}
			if err := rocS0GoldenDifference(body, body); err != nil {
				t.Fatal(err)
			}
			for _, mutant := range [][]byte{
				nil,
				body[1:],
				append(append([]byte(nil), body...), '\n'),
				[]byte(strings.ReplaceAll(string(body), `"phase"`, `"wrong_phase"`)),
				[]byte(strings.ReplaceAll(string(body), `"outcome"`, `"wrong_outcome"`)),
				[]byte(strings.ReplaceAll(string(body), `"evidence_kind"`, `"wrong_evidence_kind"`)),
			} {
				if string(mutant) == string(body) {
					t.Fatal("mutation did not affect the baseline")
				}
				if err := rocS0GoldenDifference(body, mutant); err == nil {
					t.Fatal("byte comparison accepted a changed baseline")
				}
			}
		})
	}
}

func TestROCS0NormalizationSensitivity(t *testing.T) {
	root, shadow := "/fixture", "/fixture/.tpatch/shadow/feature-2026-10-11T01-02-03.123456Z"
	first := `{"attempted_at":"2026-10-11T01:02:03Z","outcome":"blocked","confidence":"low","phase":"phase-2","upstream_commit":"abcdef","base_commit":"123456","attempt_id":"re_123","shadow_path":"/fixture/.tpatch/shadow/feature-2026-10-11T01-02-03.123456Z","resolve_session_id":"rec-2026-10-11T01-02-03Z-012345abcdef"}`
	second := strings.ReplaceAll(first, "2026-10-11T01:02:03Z", "2026-10-12T04:05:06Z")
	second = strings.ReplaceAll(second, "rec-2026-10-11T01-02-03Z-012345abcdef", "rec-2026-10-12T04-05-06Z-fedcba543210")
	normalized := rocS0Normalize(first, root, shadow, "rec-2026-10-11T01-02-03Z-012345abcdef")
	if got := rocS0Normalize(second, root, shadow, "rec-2026-10-12T04-05-06Z-fedcba543210"); got != normalized {
		t.Fatalf("declared runtime values did not normalize:\n%s\n%s", normalized, got)
	}
	for _, value := range []string{`"blocked"`, `"low"`, `"phase-2"`, `"abcdef"`, `"123456"`, `"re_123"`} {
		mutant := strings.Replace(first, value, `"changed"`, 1)
		if got := rocS0Normalize(mutant, root, shadow, "rec-2026-10-11T01-02-03Z-012345abcdef"); got == normalized {
			t.Fatalf("normalization erased semantic change %s", value)
		}
	}

}

func TestROCS0ShadowNormalizationScope(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	valid := filepath.Join(root, ".tpatch", "shadow", "feature-2026-10-11T01-02-03.123456Z")
	if err := rocS0ValidateShadowPath(root, "feature", valid); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, ".tpatch", "candidate", filepath.Base(valid)),
		filepath.Join(root, ".tpatch", "shadow", "other-2026-10-11T01-02-03.123456Z"),
		filepath.Join(root, ".tpatch", "shadow", "feature"),
		filepath.Join(root, ".tpatch", "shadow", "feature-not-a-time"),
	} {
		if err := rocS0ValidateShadowPath(root, "feature", path); err == nil {
			t.Fatalf("normalization would hide an invalid shadow path: %s", path)
		}
	}
}

func TestROCS0OperationCohortSensitivity(t *testing.T) {
	for _, name := range []string{"phase4-strict", "phase2-applicable-only", "phase2-mixed", "phase2-conflict-fallthrough"} {
		t.Run(name, func(t *testing.T) {
			rocS0PinGit(t)
			f := rocS0Fixture(t, name)
			body, err := os.ReadFile(filepath.Join(f.store.Root, ".tpatch", "features", f.slugs[0], "artifacts", "apply-recipe.json"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := rocS0ValidateOperationFixture(f.store.Root, name, body, err == nil); err != nil {
				t.Fatal(err)
			}
			if name == "phase4-strict" {
				if err := rocS0ValidateOperationFixture(f.store.Root, name, []byte(`{"operations":[]}`), true); err == nil {
					t.Fatal("plain strict fixture accepted a phase-2 recipe")
				}
				return
			}
			if err := rocS0ValidateOperationFixture(f.store.Root, name, nil, false); err == nil {
				t.Fatal("phase-2 fixture accepted a missing recipe")
			}
			var recipe ApplyRecipe
			if err := json.Unmarshal(body, &recipe); err != nil {
				t.Fatal(err)
			}
			recipe.Operations = recipe.Operations[:len(recipe.Operations)-1]
			mutant, err := json.Marshal(recipe)
			if err != nil {
				t.Fatal(err)
			}
			if err := rocS0ValidateOperationFixture(f.store.Root, name, mutant, true); err == nil {
				t.Fatal("operation cohort accepted a dropped operation")
			}
		})
	}
}

const rocS0MemoScopeFile = "internal/cli/prepare_s7_ar_guard_test.go"

var rocS0MemoScopeTests = []string{
	"TestS7ARRev21ReviewerRepros",
	"TestS7ARRev23ScheduledClosureAliasRepros",
	"TestS7ARRev24ScheduledSelfCaptureRepros",
	"TestS7ARRev24StaticChallenges",
	"TestS7ARRev23StaticChallenges",
	"TestS7ARRev21StaticChallenges",
}

// The authorized maintenance exception permits only twelve exact insertions;
// shared fixture builders and every other byte remain pinned to the S0 base.
func rocS0ExpectedMemoScopeChange(before string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rocS0MemoScopeFile, before, 0)
	if err != nil {
		return "", err
	}
	pending := make(map[string]bool, len(rocS0MemoScopeTests))
	for _, name := range rocS0MemoScopeTests {
		pending[name] = true
	}
	var expected strings.Builder
	cursor, changed := 0, 0
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || !pending[fn.Name.Name] {
			continue
		}
		start, end := fset.Position(fn.Pos()).Offset, fset.Position(fn.End()).Offset
		body := before[start:end]
		header := "func " + fn.Name.Name + "(t *testing.T) {\n"
		const helper = "\t\tt.Helper()\n"
		if !strings.HasPrefix(body, header) || strings.Count(body, helper) != 1 ||
			strings.Contains(body, "fixtureMemoScope") || strings.Contains(body, "t.Parallel(") {
			return "", fmt.Errorf("unexpected frozen memo-scope fixture shape: %s", fn.Name.Name)
		}
		body = strings.Replace(body, header, header+"\tfixtureMemoScope := s6SerialAnalyzerMemoScope(t)\n", 1)
		body = strings.Replace(body, helper, helper+"\t\tfixtureMemoScope(t)\n", 1)
		expected.WriteString(before[cursor:start])
		expected.WriteString(body)
		cursor = end
		delete(pending, fn.Name.Name)
		changed++
	}
	if changed != 6 || len(pending) != 0 {
		return "", fmt.Errorf("expected six frozen memo-scope roots, found %d", changed)
	}
	expected.WriteString(before[cursor:])
	return expected.String(), nil
}

func rocS0ValidateMemoScopeChange(before, after string) error {
	expected, err := rocS0ExpectedMemoScopeChange(before)
	if err != nil {
		return err
	}
	if after != expected {
		return fmt.Errorf("%s differs beyond the twelve authorized memo-scope insertions", rocS0MemoScopeFile)
	}
	return nil
}

func TestROCS0MemoScopeMaintenanceSensitivity(t *testing.T) {
	root := rgaS0RepoRoot(t)
	before := gitOutput(t, root, "show", rocS0Baseline+":"+rocS0MemoScopeFile)
	after := rgaS0ReadRepoFile(t, rocS0MemoScopeFile)
	if err := rocS0ValidateMemoScopeChange(before, after); err != nil {
		t.Fatal(err)
	}
	if err := rocS0ValidateMemoScopeChange(before, before); err == nil {
		t.Fatal("missing all scope registrations was accepted")
	}
	for _, name := range rocS0MemoScopeTests {
		t.Run(name, func(t *testing.T) {
			start := strings.Index(after, "func "+name+"(t *testing.T) {")
			if start < 0 {
				t.Fatal("missing mutation target")
			}
			end := len(after)
			if next := strings.Index(after[start+1:], "\nfunc "); next >= 0 {
				end = start + 1 + next
			}
			body := after[start:end]
			for _, mutation := range [][2]string{
				{"\tfixtureMemoScope := s6SerialAnalyzerMemoScope(t)\n", ""},
				{"\t\tfixtureMemoScope(t)\n", ""},
				{"\t\tfixtureMemoScope(t)\n", "\t\tfixtureMemoScope(t)\n\t\tfixtureMemoScope(t)\n"},
				{"\t\tfixtureMemoScope(t)\n", "\t\tt.Parallel()\n\t\tfixtureMemoScope(t)\n"},
				{"\tfixtureMemoScope := s6SerialAnalyzerMemoScope(t)\n", "\tt.Parallel()\n\tfixtureMemoScope := s6SerialAnalyzerMemoScope(t)\n"},
				{"!counted || actual != 4", "!counted || actual != 3"},
			} {
				if strings.Count(body, mutation[0]) != 1 {
					t.Fatalf("non-unique mutation anchor %q", mutation[0])
				}
				mutant := after[:start] + strings.Replace(body, mutation[0], mutation[1], 1) + after[end:]
				if err := rocS0ValidateMemoScopeChange(before, mutant); err == nil {
					t.Fatalf("scope validator accepted mutation %q", mutation[1])
				}
			}
		})
	}
	for _, mutant := range []string{
		after + "\n",
		strings.Replace(after, "TestS7ARRev19ReviewerRepros", "TestS7ARRev19UnexpectedRename", 1),
		strings.Replace(after, "const storeCall", "// unrelated edit\n\tconst storeCall", 1),
	} {
		if mutant == after {
			t.Fatal("mutation did not change the file")
		}
		if err := rocS0ValidateMemoScopeChange(before, mutant); err == nil {
			t.Fatal("unrelated edit passed the exact-delta exception")
		}
	}
	for _, malformed := range []string{
		"package cli\nfunc broken(",
		strings.Replace(before, "TestS7ARRev21ReviewerRepros", "TestS7ARRev21MissingRoot", 1),
		after,
	} {
		if _, err := rocS0ExpectedMemoScopeChange(malformed); err == nil {
			t.Fatal("invalid frozen baseline accepted")
		}
	}
}

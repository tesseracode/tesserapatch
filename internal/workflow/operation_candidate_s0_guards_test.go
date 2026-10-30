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

var rocS0CaptureAllowedPaths = map[string]bool{
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

var rocS1AllowedPaths = map[string]bool{
	"docs/adrs/ADR-037-reconcile-operation-replay-candidate-authority.md": true,
	"docs/adrs/ADR-044-candidate-caller-held-lock-capabilities.md":        true,
	"docs/prds/PRD-reconcile-operation-replay-candidate.md":               true,
	"docs/handoff/CURRENT.md":                                             true,
	"docs/handoff/HISTORY.md":                                             true,
	"docs/ROADMAP.md":                                                     true,
	"docs/supervisor/LOG.md":                                              true,
	".github/workflows/ci.yml":                                            true,
	"internal/workflow/operation_candidate_s0_test.go":                    true,
	"internal/workflow/operation_candidate_s0_guards_test.go":             true,
	"internal/workflow/operation_candidate_s0_fixtures_test.go":           true,
	"internal/cli/operation_candidate_s0_test.go":                         true,
	"internal/store/operation_candidate_s0_test.go":                       true,
	"internal/candidate/root.go":                                          true,
	"internal/candidate/readflags_other.go":                               true,
	"internal/candidate/readflags_unix.go":                                true,
	"internal/candidate/root_test.go":                                     true,
	"internal/candidate/root_unix_test.go":                                true,
	"internal/candidate/source_guard_test.go":                             true,
	"internal/candidatelock/error.go":                                     true,
	"internal/candidatelock/lock_common.go":                               true,
	"internal/candidatelock/lock_unix.go":                                 true,
	"internal/candidatelock/lock_windows.go":                              true,
	"internal/candidatelock/lock_unsupported.go":                          true,
	"internal/candidatelock/filesystem_linux.go":                          true,
	"internal/candidatelock/filesystem_darwin.go":                         true,
	"internal/candidatelock/lock_supported_test.go":                       true,
	"internal/candidatelock/lock_windows_native_test.go":                  true,
	"internal/candidatelock/lock_unsupported_test.go":                     true,
	"internal/candidatelock/source_guard_test.go":                         true,
	"internal/candidatelock/lock_unix_safety_test.go":                     true,
	"internal/candidatelock/filesystem_linux_test.go":                     true,
	"internal/candidatelock/filesystem_darwin_test.go":                    true,
	"internal/candidatelock/filesystem_windows_test.go":                   true,
	"internal/gitutil/tree_hash.go":                                       true,
	"internal/gitutil/tree_hash_test.go":                                  true,
	"internal/gitutil/hardened_env.go":                                    true,
	"internal/cli/prepare_s7_platform_guard_test.go":                      true,
	"internal/cli/prepare_avp2_test.go":                                   true,
	"internal/cli/prepare_s6_cache_scope_test.go":                         true,
	"internal/cli/prepare_s7_ci_timeout_guard_test.go":                    true,
	"scripts/wave-close-test-shards.sh":                                   true,
	"scripts/profile-validation.py":                                       true,
	"scripts/profile-validation-test.py":                                  true,
}

var rocS2AllowedPaths = map[string]bool{
	"internal/cli/prepare_s7_ap_guards_test.go":                   true,
	"internal/gitutil/recipe_authority_s0_pi12_test.go":           true,
	"internal/workflow/recipe_authority_s0_source_guards_test.go": true,
	"internal/workflow/operation_candidate_eligibility.go":        true,
	"internal/workflow/operation_candidate_s2_test.go":            true,
	"internal/workflow/operation_candidate_s2_guards_test.go":     true,
	"internal/candidate/reference.go":                             true,
	"internal/candidate/reference_test.go":                        true,
	"internal/patchobs/gitread.go":                                true,
	"internal/patchobs/reconstruct.go":                            true,
	"internal/store/types.go":                                     true,
	"internal/store/store.go":                                     true,
	"internal/store/roundtrip_test.go":                            true,
}

var rocS3AllowedPaths = map[string]bool{
	"docs/supervisor/OVERNIGHT-2026-10-29.md":                       true,
	"docs/adrs/ADR-045-ephemeral-operation-derivation-snapshots.md": true,
	"internal/candidate/alignment.go":                               true,
	"internal/candidate/alignment_test.go":                          true,
	"internal/candidate/derivation.go":                              true,
	"internal/candidate/derivation_test.go":                         true,
	"internal/candidate/derivation_guard_test.go":                   true,
	"internal/candidate/snapshot.go":                                true,
	"internal/gitutil/patch_insertions.go":                          true,
	"internal/gitutil/patch_insertions_test.go":                     true,
	"internal/gitutil/patch_reconstruct.go":                         true,
}

func rocS3ValidateStageChangedPaths(paths []string) error {
	for _, path := range paths {
		if rocS3AllowedPaths[path] {
			continue
		}
		if err := rocS2ValidateStageChangedPaths([]string{path}); err != nil {
			return err
		}
	}
	return nil
}

func TestROCS3ScopeTransition(t *testing.T) {
	for path := range rocS3AllowedPaths {
		if err := rocS3ValidateStageChangedPaths([]string{path}); err != nil {
			t.Fatal(err)
		}
		if err := rocS2ValidateStageChangedPaths([]string{path}); err == nil {
			t.Fatalf("S2 accepted S3 path %s", path)
		}
		if err := rocS0ValidateCaptureChangedPaths([]string{path}); err == nil {
			t.Fatalf("capture accepted S3 path %s", path)
		}
	}
	for _, path := range []string{"internal/candidate/execute.go", "internal/workflow/reconcile.go", "internal/store/candidate.go", "internal/cli/cobra.go", "internal/candidate/./alignment.go"} {
		if err := rocS3ValidateStageChangedPaths([]string{path}); err == nil {
			t.Fatalf("S3 accepted later/unregistered surface %s", path)
		}
	}
}

func rocS2ValidateStageChangedPaths(paths []string) error {
	for _, path := range paths {
		if rocS2AllowedPaths[path] {
			continue
		}
		if err := rocS1ValidateStageChangedPaths([]string{path}); err != nil {
			return err
		}
	}
	return nil
}

func TestROCS2ScopeTransition(t *testing.T) {
	for path := range rocS2AllowedPaths {
		if err := rocS2ValidateStageChangedPaths([]string{path}); err != nil {
			t.Fatal(err)
		}
		if err := rocS1ValidateStageChangedPaths([]string{path}); err == nil {
			t.Fatalf("S1 unexpectedly allowed S2 path %s", path)
		}
		if err := rocS0ValidateCaptureChangedPaths([]string{path}); err == nil {
			t.Fatalf("S0 capture allowed S2 path %s", path)
		}
	}
	for _, path := range []string{
		"internal/workflow/reconcile.go", "internal/cli/cobra.go",
		"internal/workflow/operation_candidate_execute.go", "internal/workflow/operation_candidate_alignment.go",
		"internal/candidate/third_root.go", "internal/patchobs/unregistered.go",
		"internal/store/./types.go", "SPEC.md", "go.mod", "assets/SKILL.md",
	} {
		if err := rocS2ValidateStageChangedPaths([]string{path}); err == nil {
			t.Fatalf("S2 accepted unregistered or later-slice surface %s", path)
		}
	}
}

func rocS0ValidateCaptureChangedPaths(paths []string) error {
	for _, path := range paths {
		if path != "" && filepath.ToSlash(filepath.Clean(path)) != path {
			return fmt.Errorf("non-canonical S0 path: %q", path)
		}
		if path == "" || rocS0CaptureAllowedPaths[path] {
			continue
		}
		if strings.HasPrefix(path, "internal/workflow/testdata/roc-s0/") {
			continue
		}
		return fmt.Errorf("S0 golden capture changed a non-baseline path: %s", path)
	}
	return nil
}

func rocS1ValidateStageChangedPaths(paths []string) error {
	for _, path := range paths {
		if path != "" && filepath.ToSlash(filepath.Clean(path)) != path {
			return fmt.Errorf("non-canonical S1 path: %q", path)
		}
		if path == "" || rocS1AllowedPaths[path] {
			continue
		}
		if strings.HasPrefix(path, "internal/workflow/testdata/roc-s0/") {
			continue
		}
		return fmt.Errorf("S1 scope includes an unregistered path: %s", path)
	}
	return nil
}

func rocS0AssertChangedPaths(t *testing.T, validator func([]string) error, memoValidator func(string, string) error) {
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
		if err := memoValidator(before, string(after)); err != nil {
			t.Fatal(err)
		}
	}
	if err := validator(ordinary); err != nil {
		t.Fatal(err)
	}
}

func rocS0AssertCaptureOnlyScope(t *testing.T) {
	t.Helper()
	rocS0AssertChangedPaths(t, rocS0ValidateCaptureChangedPaths, rocS0ValidateMemoScopeChange)
}

func rocS0AssertStageScope(t *testing.T) {
	t.Helper()
	rocS0AssertChangedPaths(t, rocS3ValidateStageChangedPaths, rocS1ValidateMemoScopeChange)
}

func TestROCS0NoProductionChanges(t *testing.T) {
	rocS0AssertStageScope(t)
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
		"docs/adrs/ADR-037-reconcile-operation-replay-candidate-authority.md",
		"docs/adrs/ADR-044-candidate-caller-held-lock-capabilities.md",
		"docs/prds/PRD-reconcile-operation-replay-candidate.md",
		".github/workflows/ci.yml",
		"internal/candidate/root.go",
		"internal/candidate/root_test.go",
		"internal/candidate/source_guard_test.go",
		"internal/candidatelock/lock_unix.go",
		"internal/candidatelock/lock_windows_native_test.go",
		"internal/gitutil/tree_hash.go",
		"internal/gitutil/tree_hash_test.go",
		"internal/gitutil/hardened_env.go",
		"internal/cli/prepare_s6_cache_scope_test.go",
		"internal/cli/prepare_s7_ci_timeout_guard_test.go",
		"scripts/wave-close-test-shards.sh",
		"scripts/profile-validation.py",
		"scripts/profile-validation-test.py",
	} {
		if err := rocS1ValidateStageChangedPaths([]string{path}); err != nil {
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
		"scripts/unregistered-shards.sh",
		"docs/adrs/ADR-045-unapproved.md",
		"internal/workflow/testdata/roc-s0/../../reconcile.go",
		"internal/candidatex/root.go",
		"internal/candidatelockx/lock_unix.go",
		"internal/gitutil/treehash.go",
	} {
		t.Run(path, func(t *testing.T) {
			if err := rocS1ValidateStageChangedPaths([]string{path}); err == nil {
				t.Fatalf("scope guard accepted %s", path)
			}
		})
	}
}

func TestROCS0CaptureScopeRejectsS1ProductionPaths(t *testing.T) {
	for _, path := range []string{
		"docs/adrs/ADR-037-reconcile-operation-replay-candidate-authority.md",
		"docs/adrs/ADR-044-candidate-caller-held-lock-capabilities.md",
		"docs/prds/PRD-reconcile-operation-replay-candidate.md",
		".github/workflows/ci.yml",
		"internal/candidate/root.go",
		"internal/candidate/source_guard_test.go",
		"internal/candidatelock/lock_unix.go",
		"internal/gitutil/tree_hash.go",
		"internal/cli/prepare_s7_platform_guard_test.go",
		"internal/cli/prepare_s6_cache_scope_test.go",
		"internal/cli/prepare_s7_ci_timeout_guard_test.go",
		"scripts/wave-close-test-shards.sh",
		"scripts/profile-validation.py",
		"scripts/profile-validation-test.py",
	} {
		if err := rocS0ValidateCaptureChangedPaths([]string{path}); err == nil {
			t.Fatalf("capture scope accepted S1 production path %s", path)
		}
	}
	for _, path := range []string{
		"internal/workflow/operation_candidate_s0_test.go",
		"internal/workflow/operation_candidate_s0_guards_test.go",
		"internal/workflow/testdata/roc-s0/phase1-confirmed.golden",
	} {
		if err := rocS0ValidateCaptureChangedPaths([]string{path}); err != nil {
			t.Fatalf("capture scope rejected baseline path %s: %v", path, err)
		}
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
	return rocExpectedNamedMemoScopes(before, rocS0MemoScopeTests)
}

func rocExpectedNamedMemoScopes(before string, names []string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rocS0MemoScopeFile, before, 0)
	if err != nil {
		return "", err
	}
	pending := make(map[string]bool, len(names))
	for _, name := range names {
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
	if changed != len(names) || len(pending) != 0 {
		return "", fmt.Errorf("expected %d frozen memo-scope roots, found %d", len(names), changed)
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

func rocS1ExpectedMemoScopeChange(before string) (string, error) {
	expected, err := rocS0ExpectedMemoScopeChange(before)
	if err != nil {
		return "", err
	}
	expected, err = rocExpectedNamedMemoScopes(expected, []string{
		"TestS7ARRev17ReviewerRepros", "TestS7ARRev18ReviewerRepros",
	})
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rocS0MemoScopeFile, expected, 0)
	if err != nil {
		return "", err
	}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "TestS7ARPurgeProgressGuard" {
			continue
		}
		start, end := fset.Position(fn.Pos()).Offset, fset.Position(fn.End()).Offset
		body := expected[start:end]
		if strings.Contains(body, "fixtureMemoDone") || strings.Contains(body, "t.Parallel(") {
			return "", fmt.Errorf("unexpected frozen flat-fixture scope")
		}
		for _, insertion := range []struct{ anchor, added string }{
			{"\tt.Run(\"PIB-518\", func(t *testing.T) {\n", "\t\tfixtureMemoDone := s6SerialAnalyzerMemoBatch(t)\n"},
			{"\t\t\t\t\t\tfixture.name, got, ok, fixture.wantActualWrites)\n\t\t\t\t}\n\t\t\t}\n", "\t\t\tfixtureMemoDone()\n"},
			{"\t\t\t\tt.Fatalf(\"PIB-518 %s created a false inventory: %v\", fixture.name, err)\n\t\t\t}\n", "\t\t\tfixtureMemoDone()\n"},
		} {
			if strings.Count(body, insertion.anchor) != 1 {
				return "", fmt.Errorf("flat-fixture insertion anchor is not unique")
			}
			body = strings.Replace(body, insertion.anchor, insertion.anchor+insertion.added, 1)
		}
		return expected[:start] + body + expected[end:], nil
	}
	return "", fmt.Errorf("missing frozen purge guard")
}

func rocS1ValidateMemoScopeChange(before, after string) error {
	expected, err := rocS1ExpectedMemoScopeChange(before)
	if err != nil {
		return err
	}
	if after != expected {
		return fmt.Errorf("%s differs beyond the nineteen authorized memo-scope insertions", rocS0MemoScopeFile)
	}
	return nil
}

func TestROCS0ShardingMemoScopeSensitivity(t *testing.T) {
	root := rgaS0RepoRoot(t)
	before := gitOutput(t, root, "show", rocS0Baseline+":"+rocS0MemoScopeFile)
	after := rgaS0ReadRepoFile(t, rocS0MemoScopeFile)
	if err := rocS1ValidateMemoScopeChange(before, after); err != nil {
		t.Fatal(err)
	}
	legacy, err := rocS0ExpectedMemoScopeChange(before)
	if err != nil {
		t.Fatal(err)
	}
	if err := rocS1ValidateMemoScopeChange(before, legacy); err == nil {
		t.Fatal("missing new memo scopes accepted")
	}
	if err := rocS0ValidateMemoScopeChange(before, after); err == nil {
		t.Fatal("golden capture accepted post-S0 memory maintenance")
	}
	for _, anchor := range []string{
		"\tfixtureMemoScope := s6SerialAnalyzerMemoScope(t)\n",
		"\t\tfixtureMemoScope(t)\n",
		"\t\tfixtureMemoDone := s6SerialAnalyzerMemoBatch(t)\n",
		"\t\t\tfixtureMemoDone()\n",
	} {
		for offset := 0; ; {
			index := strings.Index(after[offset:], anchor)
			if index < 0 {
				break
			}
			index += offset
			for _, replacement := range []string{"", anchor + anchor, anchor + "\t\tt.Parallel()\n"} {
				mutant := after[:index] + replacement + after[index+len(anchor):]
				if err := rocS1ValidateMemoScopeChange(before, mutant); err == nil {
					t.Fatal("omitted, duplicated or parallel memo scope accepted")
				}
			}
			offset = index + len(anchor)
		}
	}
	mutant := strings.Replace(after, "fixture.wantActualWrites)", "fixture.wantActualWrites+1)", 1)
	if mutant == after {
		t.Fatal("assertion mutation did not change source")
	}
	if err := rocS1ValidateMemoScopeChange(before, mutant); err == nil {
		t.Fatal("changed purge assertion accepted")
	}
}
func TestROCS0MemoScopeMaintenanceSensitivity(t *testing.T) {
	root := rgaS0RepoRoot(t)
	before := gitOutput(t, root, "show", rocS0Baseline+":"+rocS0MemoScopeFile)
	after, err := rocS0ExpectedMemoScopeChange(before)
	if err != nil {
		t.Fatal(err)
	}
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

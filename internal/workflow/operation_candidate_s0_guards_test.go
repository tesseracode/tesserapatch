package workflow

import (
	"encoding/json"
	"fmt"
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
	if err := rocS0ValidateChangedPaths(strings.Split(changed+"\n"+untracked, "\n")); err != nil {
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

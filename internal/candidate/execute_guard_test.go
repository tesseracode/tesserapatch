package candidate

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestS4WriterRegistryIsClosed(t *testing.T) {
	var keys []string
	for key := range candidateWriterRegistry {
		keys = append(keys, key)
	}
	if err := validateWriterRegistryKeys(keys); err != nil {
		t.Fatal(err)
	}
	if _, err := candidateWriterForKind("unknown"); refusalCode(err) != "candidate-operation-kind-unsupported" {
		t.Fatalf("unknown writer kind did not fail closed: %v", err)
	}
	for _, mutant := range [][]string{
		{"create-file"},
		{"create-file", "insert-by-boundary", "append"},
		{"insert-by-boundary", "create-file", "insert-by-boundary"},
	} {
		if err := validateWriterRegistryKeys(mutant); err == nil {
			t.Fatalf("registry validator accepted %v", mutant)
		}
	}
}

func validateWriterRegistryKeys(keys []string) error {
	want := []string{"create-file", "insert-by-boundary"}
	got := append([]string(nil), keys...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("candidate writer registry changed: got %v want %v", got, want)
	}
	return nil
}

func TestS4GenerationScopeGuardAndSensitivity(t *testing.T) {
	allow := generationScopeAllowPrefixes("demo")
	if err := validateGenerationScopeAllowlist(allow); err != nil {
		t.Fatal(err)
	}
	if err := validateGenerationWritePaths([]string{
		".tpatch/reconcile-candidates/demo/rc_deadbeef/worktree/file.txt",
		".tpatch/locks/reconcile/demo.lock",
		".tpatch/features/demo/status.json",
		".tpatch/features/demo/reconcile-evidence.jsonl",
		".git/worktrees/demo-worktree/gitdir",
	}, allow); err != nil {
		t.Fatal(err)
	}
	if err := validateGenerationWritePaths([]string{"tracked.txt"}, allow); err == nil {
		t.Fatal("generation scope accepted live tracked write")
	}
	for _, mutant := range [][]string{
		allow[:3],
		append(append([]string(nil), allow...), ".tpatch/cache/"),
	} {
		if err := validateGenerationScopeAllowlist(mutant); err == nil {
			t.Fatalf("allowlist validator accepted mutation %v", mutant)
		}
	}
}

func generationScopeAllowPrefixes(slug string) []string {
	return []string{
		".tpatch/reconcile-candidates/" + slug + "/",
		".tpatch/locks/",
		".tpatch/features/" + slug + "/",
		".git/worktrees/",
	}
}

func validateGenerationScopeAllowlist(prefixes []string) error {
	if len(prefixes) != 4 {
		return fmt.Errorf("generation allowlist must contain exactly four write sets")
	}
	required := []string{".tpatch/reconcile-candidates/", ".tpatch/locks/", ".tpatch/features/", ".git/worktrees/"}
	for _, needle := range required {
		found := false
		for _, prefix := range prefixes {
			if strings.Contains(prefix, needle) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("generation allowlist is missing required prefix %s", needle)
		}
	}
	return nil
}

func validateGenerationWritePaths(paths, allowPrefixes []string) error {
	if err := validateGenerationScopeAllowlist(allowPrefixes); err != nil {
		return err
	}
	for _, path := range paths {
		allowed := false
		for _, prefix := range allowPrefixes {
			if strings.HasPrefix(path, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("generation wrote outside the authorized scope: %s", path)
		}
	}
	return nil
}

func TestS4NoTestCommandExecutionBoundary(t *testing.T) {
	files, err := candidateProductionFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		body, err := os.ReadFile(file.path)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateNoGenerationTestExecution(string(body)); err != nil {
			t.Fatalf("%s: %v", file.path, err)
		}
	}
	source := "package candidate\nfunc bad() { RunTestCommandInShadow(nil,nil,nil,nil) }\n"
	if err := validateNoGenerationTestExecution(source); err == nil {
		t.Fatal("test-command boundary accepted a generation test invocation")
	}
}

func validateNoGenerationTestExecution(source string) error {
	for _, forbidden := range []string{"RunTestCommandInShadow", "test_command"} {
		if strings.Contains(source, forbidden) {
			return fmt.Errorf("generation path invokes acceptance-time test command primitive")
		}
	}
	return nil
}

func TestS4SecondPassAllEffectsGuardSensitivity(t *testing.T) {
	body, err := os.ReadFile("validation.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if err := validateSecondPassAllEffectsSource(source); err != nil {
		t.Fatal(err)
	}
	mutant := strings.Replace(source, "validateSecondPassPlan(secondPlan, len(effects))", "validateSecondPassPlan(secondPlan, len(plan.operations))", 1)
	if mutant == source {
		t.Fatal("second-pass mutation anchor missing")
	}
	if err := validateSecondPassAllEffectsSource(mutant); err == nil {
		t.Fatal("second-pass guard accepted operation-only idempotency")
	}
}

func validateSecondPassAllEffectsSource(source string) error {
	if !strings.Contains(source, "secondPlan, err := derive(reference, candidateRoot, patch, effects)") {
		return fmt.Errorf("second pass no longer reuses production derivation over all effects")
	}
	if !strings.Contains(source, "validateSecondPassPlan(secondPlan, len(effects))") {
		return fmt.Errorf("second pass no longer validates the full original effect set")
	}
	return nil
}

func TestS4ModeledContentLifetimeGuardAndSensitivity(t *testing.T) {
	body, err := os.ReadFile("validation.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if err := validateModeledContentLifetimeSource(source); err != nil {
		t.Fatal(err)
	}
	retentionMutant := `func validateExpectedModeledContent(upstream *ReferenceRoot, root *CandidateRoot, plan Plan) error {
		expected := map[string][]byte{}
		return nil
	}`
	if err := validateModeledContentLifetimeSource(retentionMutant); err == nil {
		t.Fatal("modeled-content lifetime guard accepted retained multi-path body map")
	}
}

func validateModeledContentLifetimeSource(source string) error {
	if strings.Contains(source, "expected := map[string][]byte{}") {
		return fmt.Errorf("modeled-content validation must not retain full-body map across paths")
	}
	if !strings.Contains(source, "opsByPath := map[string][]operation{}") {
		return fmt.Errorf("modeled-content validation no longer groups operations per path")
	}
	if !strings.Contains(source, "sort.Strings(paths)") {
		return fmt.Errorf("modeled-content validation no longer processes paths in sorted deterministic order")
	}
	return nil
}

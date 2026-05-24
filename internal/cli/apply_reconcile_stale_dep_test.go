package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func setupParentGenerationStaleFixture(t *testing.T, depKind string) (tmpDir, parentSlug, childSlug, currentParentGenerationID string) {
	t.Helper()
	tmpDir, baseSha, shaA, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Parent")
	parentSlug = "parent"
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, parentSlug, "--from", baseSha, "--to", shaA, "--lenient"); code != 0 {
		t.Fatalf("parent record failed: %s", stderr)
	}
	runCmd("add", "--path", tmpDir, "Child")
	childSlug = "child"
	s, err := store.Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.FeaturesDependencies = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	child, err := s.LoadFeatureStatus(childSlug)
	if err != nil {
		t.Fatal(err)
	}
	child.DependsOn = []store.Dependency{{Slug: parentSlug, Kind: depKind}}
	if err := s.SaveFeatureStatus(child); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, childSlug, "--from", baseSha, "--lenient", "--files", "noise.txt"); code != 0 {
		t.Fatalf("child record failed: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "a.txt"), []byte("parent refresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("feature", "patch", "refresh", "--path", tmpDir, parentSlug, "--reason", "parent update"); code != 0 {
		t.Fatalf("parent refresh failed: %s", stderr)
	}
	parentManifest := loadPatchGenerationsForTest(t, tmpDir, parentSlug)
	currentParentGenerationID = parentManifest.Generations[len(parentManifest.Generations)-1].GenerationID
	return tmpDir, parentSlug, childSlug, currentParentGenerationID
}

func writeStaleDepApplyRecipe(t *testing.T, tmpDir, childSlug, fileName string) {
	t.Helper()
	artDir := filepath.Join(tmpDir, ".tpatch", "features", childSlug, "artifacts")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	recipe := `{
  "feature": "` + childSlug + `",
  "operations": [
    {"type": "write-file", "path": "` + fileName + `", "content": "ok\n"}
  ]
}
`
	if err := os.WriteFile(filepath.Join(artDir, "apply-recipe.json"), []byte(recipe), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertStaleDepDiagnostic(t *testing.T, stderr, parentSlug, currentParentGenerationID string) {
	t.Helper()
	for _, want := range []string{
		"parent-generation-stale",
		parentSlug,
		"snapshot parent_generation=1",
		currentParentGenerationID,
		"tpatch feature patch refresh",
		"tpatch reconcile child",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected diagnostic to contain %q, got %q", want, stderr)
		}
	}
}

func TestApplyHardParentGenerationStaleRefuses(t *testing.T) {
	tmpDir, parentSlug, childSlug, currentParentGenerationID := setupParentGenerationStaleFixture(t, store.DependencyKindHard)
	writeStaleDepApplyRecipe(t, tmpDir, childSlug, "hard-should-not-exist.txt")

	_, stderr, code := runCmdWithError("apply", "--path", tmpDir, childSlug, "--mode", "execute")
	if code == 0 {
		t.Fatalf("expected hard stale apply refusal; stderr=%q", stderr)
	}
	assertStaleDepDiagnostic(t, stderr, parentSlug, currentParentGenerationID)
	if _, err := os.Stat(filepath.Join(tmpDir, "hard-should-not-exist.txt")); err == nil {
		t.Fatalf("apply mutated working tree despite hard stale refusal")
	}
}

func TestApplySoftParentGenerationStaleWarnsAndProceeds(t *testing.T) {
	tmpDir, parentSlug, childSlug, currentParentGenerationID := setupParentGenerationStaleFixture(t, store.DependencyKindSoft)
	writeStaleDepApplyRecipe(t, tmpDir, childSlug, "soft-should-exist.txt")

	_, stderr, code := runCmdWithError("apply", "--path", tmpDir, childSlug, "--mode", "execute")
	if code != 0 {
		t.Fatalf("expected soft stale apply to proceed; stderr=%q", stderr)
	}
	assertStaleDepDiagnostic(t, stderr, parentSlug, currentParentGenerationID)
	if _, err := os.Stat(filepath.Join(tmpDir, "soft-should-exist.txt")); err != nil {
		t.Fatalf("expected apply to proceed and write file: %v", err)
	}
}

func TestApplyParentGenerationStaleFlagOffBypassesGate(t *testing.T) {
	tmpDir, _, childSlug, _ := setupParentGenerationStaleFixture(t, store.DependencyKindHard)
	writeStaleDepApplyRecipe(t, tmpDir, childSlug, "flag-off-should-exist.txt")
	s, err := store.Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.FeaturesDependencies = false
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runCmdWithError("apply", "--path", tmpDir, childSlug, "--mode", "execute")
	if code != 0 {
		t.Fatalf("expected flag-off stale apply to proceed; stderr=%q", stderr)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "flag-off-should-exist.txt")); err != nil {
		t.Fatalf("expected flag-off apply to execute recipe and write file: %v", err)
	}
}

func TestReconcileHardParentGenerationStaleRefuses(t *testing.T) {
	tmpDir, parentSlug, childSlug, currentParentGenerationID := setupParentGenerationStaleFixture(t, store.DependencyKindHard)

	_, stderr, code := runCmdWithError("reconcile", "--path", tmpDir, childSlug)
	if code == 0 {
		t.Fatalf("expected hard stale reconcile refusal; stderr=%q", stderr)
	}
	assertStaleDepDiagnostic(t, stderr, parentSlug, currentParentGenerationID)
}

func TestReconcileSoftParentGenerationStaleWarnsAndProceeds(t *testing.T) {
	tmpDir, parentSlug, childSlug, currentParentGenerationID := setupParentGenerationStaleFixture(t, store.DependencyKindSoft)

	_, stderr, code := runCmdWithError("reconcile", "--path", tmpDir, "--preflight", "--allow-dirty", childSlug)
	if code != 0 {
		t.Fatalf("expected soft stale reconcile preflight to proceed; stderr=%q", stderr)
	}
	assertStaleDepDiagnostic(t, stderr, parentSlug, currentParentGenerationID)
}

func TestReconcileParentGenerationStaleFlagOffBypassesGate(t *testing.T) {
	tmpDir, _, childSlug, _ := setupParentGenerationStaleFixture(t, store.DependencyKindHard)
	s, err := store.Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.FeaturesDependencies = false
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runCmdWithError("reconcile", "--path", tmpDir, "--preflight", "--allow-dirty", childSlug)
	if code != 0 {
		t.Fatalf("expected flag-off stale reconcile preflight to proceed; stderr=%q", stderr)
	}
	if strings.Contains(stderr, "parent-generation-stale") {
		t.Fatalf("expected flag-off reconcile to bypass parent-generation-stale gate; stderr=%q", stderr)
	}
}

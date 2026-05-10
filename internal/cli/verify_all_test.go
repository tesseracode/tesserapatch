package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runVerifyAllForExitCode invokes `tpatch verify --all` through the
// real cobra root and returns the unwrapped error so tests can assert
// on *ExitCodeError directly. Mirrors `runVerifyForExitCode` in
// verify_test.go (Slice C) — same reason: the package-level `runCmd`
// helper collapses every error to exit code 1, masking the typed exit
// codes Slice D commits to.
func runVerifyAllForExitCode(args ...string) (stdout, stderr string, err error) {
	root := buildRootCmd()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"verify"}, args...))
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func setupAppliedFeatureForVerifyAll(t *testing.T, tmp, slug string) {
	t.Helper()
	if _, _, code := runCmd("add", "--path", tmp, "--slug", slug, slug); code != 0 {
		t.Fatalf("add %s: %d", slug, code)
	}
	// Mark applied + write minimal verifiable artifacts.
	statusPath := filepath.Join(tmp, ".tpatch", "features", slug, "status.json")
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	// Patch state in-place rather than re-marshalling the whole shape.
	patched := bytes.Replace(raw, []byte(`"state": "requested"`), []byte(`"state": "applied"`), 1)
	if err := os.WriteFile(statusPath, patched, 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	dir := filepath.Join(tmp, ".tpatch", "features", slug)
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("intent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exploration.md"), []byte("expl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artDir := filepath.Join(dir, "artifacts")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	recipe := `{"feature":"` + slug + `","operations":[{"type":"ensure-directory","path":"src/"}]}`
	if err := os.WriteFile(filepath.Join(artDir, "apply-recipe.json"), []byte(recipe), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestVerifyAll_NoFeatures_ExitsZero — empty repo with `--all` emits a
// zero-summary aggregate and exits 0.
func TestVerifyAll_NoFeatures_ExitsZero(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	stdout, _, err := runVerifyAllForExitCode("--path", tmp, "--all")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(stdout, "verify --all (0 feature") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

// TestVerifyAll_AllPassing_ExitsZero — two healthy applied features →
// exit 0.
func TestVerifyAll_AllPassing_ExitsZero(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	setupAppliedFeatureForVerifyAll(t, tmp, "alpha")
	setupAppliedFeatureForVerifyAll(t, tmp, "beta")

	stdout, _, err := runVerifyAllForExitCode("--path", tmp, "--all")
	if err != nil {
		t.Fatalf("expected nil error, got %v; stdout=%q", err, stdout)
	}
	if !strings.Contains(stdout, "Summary:") {
		t.Errorf("missing summary footer in %q", stdout)
	}
}

// TestVerifyAll_MalformedFeature_ExitsTwo — present-but-malformed
// recipe on one feature flips the aggregate exit to 2 (PRD §6 Q7) and
// the typed *ExitCodeError carries Code=2. Mirrors the carryover
// matrix lesson from Slice C.
func TestVerifyAll_MalformedFeature_ExitsTwo(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	setupAppliedFeatureForVerifyAll(t, tmp, "healthy")
	setupAppliedFeatureForVerifyAll(t, tmp, "broken")
	// Overwrite broken's recipe with malformed JSON.
	if err := os.WriteFile(filepath.Join(tmp, ".tpatch", "features", "broken", "artifacts", "apply-recipe.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := runVerifyAllForExitCode("--path", tmp, "--all")
	if err == nil {
		t.Fatalf("expected ExitCodeError, got nil")
	}
	var ec *ExitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("expected *ExitCodeError, got %T: %v", err, err)
	}
	if ec.Code != 2 {
		t.Errorf("malformed feature must exit 2; got %d", ec.Code)
	}
}

// TestVerifyAll_RejectsSlugArg — `verify --all <slug>` is a misuse.
func TestVerifyAll_RejectsSlugArg(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	_, _, err := runVerifyAllForExitCode("--path", tmp, "--all", "extra")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var ec *ExitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("expected *ExitCodeError, got %T: %v", err, err)
	}
	if ec.Code != 2 {
		t.Errorf("misuse must exit 2; got %d", ec.Code)
	}
}

// TestVerifyAll_JSONShape — `--json --all` emits the documented
// envelope on stdout.
func TestVerifyAll_JSONShape(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	setupAppliedFeatureForVerifyAll(t, tmp, "alpha")

	stdout, _, err := runVerifyAllForExitCode("--path", tmp, "--all", "--json")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if _, ok := parsed["features"].([]any); !ok {
		t.Errorf("features array missing")
	}
	if _, ok := parsed["summary"].(map[string]any); !ok {
		t.Errorf("summary object missing")
	}
}

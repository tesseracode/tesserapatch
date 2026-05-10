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

// TestVerifyAll_UnreadableStatusJSON_ExitsTwoAndIncludesFeature pins
// the revision-1 contract at the CLI surface: a feature whose
// status.json is unreadable must appear in `--json` output as an
// error row, summary.error must increment, and exit code must be 2.
// The bug at 19271f7 silently dropped such features, producing a
// false-green exit 0 with the bad feature ABSENT from output.
func TestVerifyAll_UnreadableStatusJSON_ExitsTwoAndIncludesFeature(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	setupAppliedFeatureForVerifyAll(t, tmp, "good")
	setupAppliedFeatureForVerifyAll(t, tmp, "bad")
	// Corrupt bad's status.json — the external supervisor's exact repro.
	badStatusPath := filepath.Join(tmp, ".tpatch", "features", "bad", "status.json")
	if err := os.WriteFile(badStatusPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runVerifyAllForExitCode("--path", tmp, "--all", "--json")
	if err == nil {
		t.Fatalf("expected ExitCodeError, got nil; stdout=%q", stdout)
	}
	var ec *ExitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("expected *ExitCodeError, got %T: %v", err, err)
	}
	if ec.Code != 2 {
		t.Errorf("unreadable status.json must exit 2; got %d", ec.Code)
	}
	var parsed map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &parsed); jerr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", jerr, stdout)
	}
	feats, ok := parsed["features"].([]any)
	if !ok {
		t.Fatalf("features array missing")
	}
	if len(feats) != 2 {
		t.Errorf("expected 2 features in aggregate, got %d (the bug omitted bad)", len(feats))
	}
	var sawBadAsError bool
	for _, raw := range feats {
		f, _ := raw.(map[string]any)
		if f["slug"] == "bad" && f["status"] == "error" {
			sawBadAsError = true
		}
	}
	if !sawBadAsError {
		t.Errorf("bad feature missing or not status=error in features array: %v", feats)
	}
	summary, _ := parsed["summary"].(map[string]any)
	if v, _ := summary["error"].(float64); v != 1 {
		t.Errorf("summary.error=%v want 1; full summary=%v", summary["error"], summary)
	}
}

// TestVerifyAll_UnstattableStatusJSON_ExitsTwoAndIncludesFeature pins
// the revision-2 contract at the CLI surface: a feature whose
// status.json cannot even be stat-ed (parent dir chmod 000) must
// appear in `--json` output as an error row, summary.error must
// increment, and exit code must be 2. The bug at 67730de silently
// dropped such features, producing a false-green exit 0 with the
// locked feature ABSENT from output (the external supervisor's exact
// chmod-000 repro).
func TestVerifyAll_UnstattableStatusJSON_ExitsTwoAndIncludesFeature(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user (root bypasses permission checks)")
	}
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	setupAppliedFeatureForVerifyAll(t, tmp, "good")
	setupAppliedFeatureForVerifyAll(t, tmp, "locked")

	lockedDir := filepath.Join(tmp, ".tpatch", "features", "locked")
	if err := os.Chmod(lockedDir, 0o000); err != nil {
		t.Fatalf("chmod 000 locked: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockedDir, 0o700) })

	stdout, _, err := runVerifyAllForExitCode("--path", tmp, "--all", "--json")
	if err == nil {
		t.Fatalf("expected ExitCodeError, got nil; stdout=%q", stdout)
	}
	var ec *ExitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("expected *ExitCodeError, got %T: %v", err, err)
	}
	if ec.Code != 2 {
		t.Errorf("unstattable status.json must exit 2; got %d", ec.Code)
	}
	var parsed map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &parsed); jerr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", jerr, stdout)
	}
	feats, ok := parsed["features"].([]any)
	if !ok {
		t.Fatalf("features array missing")
	}
	if len(feats) != 2 {
		t.Errorf("expected 2 features in aggregate, got %d (the bug omitted locked)", len(feats))
	}
	var sawLockedAsError bool
	var lockedReason string
	for _, raw := range feats {
		f, _ := raw.(map[string]any)
		if f["slug"] == "locked" && f["status"] == "error" {
			sawLockedAsError = true
			lockedReason, _ = f["reason"].(string)
		}
	}
	if !sawLockedAsError {
		t.Errorf("locked feature missing or not status=error in features array: %v", feats)
	}
	if !strings.Contains(lockedReason, "stat status.json") {
		t.Errorf("locked reason=%q must mention stat failure", lockedReason)
	}
	summary, _ := parsed["summary"].(map[string]any)
	if v, _ := summary["error"].(float64); v != 1 {
		t.Errorf("summary.error=%v want 1; full summary=%v", summary["error"], summary)
	}
}

// TestVerifyAll_FeaturesDirMissing_ExitsTwo pins the revision-3
// contract at the CLI surface: a workspace where `.tpatch/` exists
// but `.tpatch/features/` was removed must exit 2 with an error
// message naming "features" and "workspace"/"corruption". The bug
// at e7f8661 returned exit 0 with an empty aggregate (the external
// supervisor's exact `rm -rf .tpatch/features` repro).
func TestVerifyAll_FeaturesDirMissing_ExitsTwo(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	if err := os.RemoveAll(filepath.Join(tmp, ".tpatch", "features")); err != nil {
		t.Fatalf("remove features dir: %v", err)
	}

	stdout, _, err := runVerifyAllForExitCode("--path", tmp, "--all", "--json")
	if err == nil {
		t.Fatalf("expected ExitCodeError, got nil; stdout=%q", stdout)
	}
	var ec *ExitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("expected *ExitCodeError, got %T: %v", err, err)
	}
	if ec.Code != 2 {
		t.Errorf("missing features dir must exit 2; got %d", ec.Code)
	}
	msg := ec.Message
	if !strings.Contains(msg, "features") {
		t.Errorf("error message must name features; got %q", msg)
	}
	if !strings.Contains(msg, "workspace") && !strings.Contains(msg, "corruption") {
		t.Errorf("error message must name workspace or corruption; got %q", msg)
	}
}

// TestVerifyAll_TpatchDirUnstattable_ExitsTwo pins the revision-4
// contract at the CLI surface: an exotic non-ENOENT FS error on the
// `.tpatch/` path itself (here: a symlink loop → ELOOP) must flip
// exit to 2 with an error message naming the workspace or the
// underlying FS failure. The bug at d390322 silently treated *any*
// non-nil non-ENOENT stat error as "uninitialized" and exited 0 with
// an empty aggregate (false green).
//
// See the matching workflow-level test for the empirical note about
// which branch this exercises in practice.
func TestVerifyAll_TpatchDirUnstattable_ExitsTwo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user")
	}
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init: %d", code)
	}
	dotp := filepath.Join(tmp, ".tpatch")
	if err := os.RemoveAll(dotp); err != nil {
		t.Fatalf("remove .tpatch: %v", err)
	}
	loopA := filepath.Join(tmp, "loopA")
	loopB := filepath.Join(tmp, "loopB")
	if err := os.Symlink(loopB, loopA); err != nil {
		t.Fatalf("symlink loopA: %v", err)
	}
	if err := os.Symlink(loopA, loopB); err != nil {
		t.Fatalf("symlink loopB: %v", err)
	}
	if err := os.Symlink(loopA, dotp); err != nil {
		t.Fatalf("symlink .tpatch: %v", err)
	}

	stdout, _, err := runVerifyAllForExitCode("--path", tmp, "--all")
	if err == nil {
		t.Fatalf("expected ExitCodeError; got stdout=%q", stdout)
	}
	var ec *ExitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("expected *ExitCodeError, got %T: %v", err, err)
	}
	if ec.Code != 2 {
		t.Errorf("unstattable .tpatch must exit 2; got %d", ec.Code)
	}
	msg := ec.Message
	if !strings.Contains(msg, ".tpatch") && !strings.Contains(msg, "symbolic links") && !strings.Contains(msg, "workspace") {
		t.Errorf("error message must reference workspace or underlying FS error; got %q", msg)
	}
}

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

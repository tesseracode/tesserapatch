// CLI-surface tests for `tpatch feature resource ...` and
// `tpatch record --resources` (PRD §3, §11; ADR-033).
//
// These exercise the taxonomy end to end: every refusal must reach the
// process boundary with its binding exit code, not merely be returned
// as an error inside the package.

package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/rescap"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// resourceTestRepo builds an initialized tpatch workspace with one
// feature and an ignored config file.
func resourceTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	if _, _, code := runCmdExit("init", "--path", dir); code != 0 {
		t.Fatalf("tpatch init failed with code %d", code)
	}
	if _, _, code := runCmdExit("add", "--path", dir, "Model picker"); code != 0 {
		t.Fatalf("tpatch add failed with code %d", code)
	}
	writeResourceFile(t, dir, "config/local-secrets.env.template", "A=1\n")
	appendResourceGitignore(t, dir, "config/local-secrets.env.template", "config/dir-selector/")
	gitCommitAll(t, dir, "ignore config")
	return dir
}

func writeResourceFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func appendResourceGitignore(t *testing.T, root string, lines ...string) {
	t.Helper()
	path := filepath.Join(root, ".gitignore")
	existing, _ := os.ReadFile(path)
	body := string(existing) + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
}

// removeGitignoreLine drops one exact rule while preserving every
// other line — notably the `.tpatch/local/` rule tpatch init installs,
// which the mutator gate depends on.
func removeGitignoreLine(t *testing.T, root, rule string) {
	t.Helper()
	path := filepath.Join(root, ".gitignore")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	var kept []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) == rule {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
}

func gitCommitAll(t *testing.T, root, message string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

// TestResourceAddListRemoveClearRoundTrip covers the declaration
// quartet's shapes, including the "no such feature" refusal shared with
// feature claim.
func TestResourceAddListRemoveClearRoundTrip(t *testing.T) {
	dir := resourceTestRepo(t)

	stdout, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "ignored-file", "--selector", "config/local-secrets.env.template")
	if code != 0 {
		t.Fatalf("add exit %d: %s", code, stdout)
	}
	if !strings.Contains(stdout, "res_79f5ac5dca13") {
		t.Fatalf("add should report the golden vector's id: %s", stdout)
	}

	// Idempotent re-add: exit 0, no second entry.
	stdout, _, code = runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "ignored-file", "--selector", "config/local-secrets.env.template")
	if code != 0 || !strings.Contains(stdout, "already-declared") {
		t.Fatalf("duplicate add should be idempotent: code=%d out=%s", code, stdout)
	}

	stdout, _, code = runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "git-metadata", "--selector", "head")
	if code != 0 {
		t.Fatalf("git-metadata add exit %d: %s", code, stdout)
	}

	stdout, _, code = runCmdExit("feature", "resource", "list", "--path", dir, "model-picker", "--json")
	if code != 0 {
		t.Fatalf("list exit %d", code)
	}
	var listing struct {
		Feature   string `json:"feature"`
		Resources []struct {
			ResourceID string `json:"resource_id"`
			State      string `json:"state"`
		} `json:"resources"`
	}
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil {
		t.Fatalf("list --json: %v\n%s", err, stdout)
	}
	if len(listing.Resources) != 2 {
		t.Fatalf("want 2 resources, got %d", len(listing.Resources))
	}
	for _, r := range listing.Resources {
		if r.State != "no-capture-yet" {
			t.Fatalf("state = %s, want no-capture-yet", r.State)
		}
	}

	// Prefix removal.
	_, _, code = runCmdExit("feature", "resource", "remove", "--path", dir, "model-picker", "79f5ac5")
	if code != 0 {
		t.Fatalf("prefix remove exit %d", code)
	}
	stdout, _, _ = runCmdExit("feature", "resource", "list", "--path", dir, "model-picker")
	if strings.Contains(stdout, "res_79f5ac5dca13") {
		t.Fatal("the removed resource is still listed")
	}

	_, _, code = runCmdExit("feature", "resource", "clear", "--path", dir, "model-picker")
	if code != 0 {
		t.Fatalf("clear exit %d", code)
	}
	stdout, _, _ = runCmdExit("feature", "resource", "list", "--path", dir, "model-picker")
	if !strings.Contains(stdout, "(none)") {
		t.Fatalf("clear should leave an empty manifest: %s", stdout)
	}
	// clear keeps the file with resources: [].
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	body, err := os.ReadFile(s.ResourcesPath("model-picker"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !strings.Contains(string(body), `"resources": []`) {
		t.Fatalf("manifest should keep an empty array: %s", body)
	}

	_, _, code = runCmdExit("feature", "resource", "list", "--path", dir, "no-such-feature")
	if code == 0 {
		t.Fatal("an unknown feature must be refused")
	}
}

// TestResourcePrefixAmbiguityAndUnknownTarget covers the shared
// resolution rules for remove/trust-dolt/capture/diff.
func TestResourcePrefixAmbiguityAndUnknownTarget(t *testing.T) {
	dir := resourceTestRepo(t)
	if _, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "ignored-file", "--selector", "config/local-secrets.env.template"); code != 0 {
		t.Fatal("add failed")
	}
	_, stderr, code := runCmdExit("feature", "resource", "remove", "--path", dir, "model-picker", "zzzzzzzzz")
	if code != rescap.ExitValidation {
		t.Fatalf("unknown target exit = %d, want 2: %s", code, stderr)
	}
	if !strings.Contains(stderr, rescap.ReasonNoSuchResource) {
		t.Fatalf("stderr = %s", stderr)
	}

	// Force an ambiguous prefix by hand-writing two entries whose IDs
	// share a prefix, using their real, self-consistent identities.
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	m, err := store.LoadResources(s, "model-picker")
	if err != nil {
		t.Fatalf("LoadResources: %v", err)
	}
	if len(m.Resources) != 1 {
		t.Fatalf("want 1 resource, got %d", len(m.Resources))
	}
	// A one-character prefix is ambiguous against itself plus any
	// second entry; add a second real declaration and use the empty
	// common prefix.
	second := store.Resource{
		Kind: store.ResourceKindGitMetadata, Selector: "head", Args: []store.ResourceArg{},
	}
	second.ResourceID = store.ComputeResourceID("model-picker", second.Kind, second.Selector, "", "", nil)
	m.Resources = append(m.Resources, second)
	if err := store.SaveResources(s, m); err != nil {
		t.Fatalf("SaveResources: %v", err)
	}
	// "res_" strips to the empty needle, which is below the minimum
	// prefix length and therefore reports "no such resource" rather
	// than ambiguity; a shared real prefix is what triggers ambiguity.
	shared := commonPrefix(m.Resources[0].ResourceID, m.Resources[1].ResourceID)
	if len(strings.TrimPrefix(shared, store.ResourceIDPrefix)) >= store.ResourceIDPrefixMin {
		_, stderr, code = runCmdExit("feature", "resource", "remove", "--path", dir, "model-picker", shared)
		if code != rescap.ExitValidation || !strings.Contains(stderr, rescap.ReasonAmbiguousResourcePrefix) {
			t.Fatalf("ambiguous prefix should be exit 2: code=%d stderr=%s", code, stderr)
		}
	}
}

func commonPrefix(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// TestResourceExitCodeTaxonomy walks the CLI-reachable refusals and
// asserts the exact process exit code for each.
func TestResourceExitCodeTaxonomy(t *testing.T) {
	dir := resourceTestRepo(t)
	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantName string
	}{
		{
			name:     "unknown-kind",
			args:     []string{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "reflog", "--selector", "x"},
			wantCode: rescap.ExitValidation, wantName: rescap.ReasonInvalidDeclaration,
		},
		{
			name:     "not-ignored",
			args:     []string{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "ignored-file", "--selector", "README.md"},
			wantCode: rescap.ExitRefusal, wantName: rescap.ReasonNotIgnored,
		},
		{
			name:     "path-outside-repo",
			args:     []string{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "ignored-file", "--selector", "../escape.env"},
			wantCode: rescap.ExitRefusal, wantName: rescap.ReasonPathOutsideRepo,
		},
		{
			name: "dolt-trust-flag-required",
			args: []string{"feature", "resource", "add", "--path", dir, "model-picker",
				"--kind", "adapter-snapshot", "--adapter", "dolt", "--selector", "dolt:diff-summary:users",
				"--arg", "contract=dolt-diff-summary-v1", "--arg", "db_path=config", "--arg", "table=users",
				"--arg", "from=main", "--arg", "to=HEAD"},
			wantCode: rescap.ExitValidation, wantName: rescap.ReasonDoltTrustFlagRequired,
		},
		{
			name: "dolt-contract-unsupported",
			args: []string{"feature", "resource", "add", "--path", dir, "model-picker",
				"--kind", "adapter-snapshot", "--adapter", "dolt", "--selector", "dolt:diff-summary:users",
				"--arg", "contract=dolt-diff-summary-v2", "--arg", "db_path=config", "--arg", "table=users",
				"--arg", "from=main", "--arg", "to=HEAD", "--trust-current-dolt"},
			wantCode: rescap.ExitValidation, wantName: rescap.ReasonDoltContractUnsupported,
		},
		{
			name: "dolt-argument-refused-working",
			args: []string{"feature", "resource", "add", "--path", dir, "model-picker",
				"--kind", "adapter-snapshot", "--adapter", "dolt", "--selector", "dolt:diff-summary:users",
				"--arg", "contract=dolt-diff-summary-v1", "--arg", "db_path=config", "--arg", "table=users",
				"--arg", "from=WORKING", "--arg", "to=HEAD", "--trust-current-dolt"},
			wantCode: rescap.ExitValidation, wantName: rescap.ReasonDoltArgumentRefused,
		},
		{
			name: "duplicate-arg",
			args: []string{"feature", "resource", "add", "--path", dir, "model-picker",
				"--kind", "adapter-snapshot", "--adapter", "dolt", "--selector", "dolt:diff-summary:users",
				"--arg", "table=users", "--arg", "table=users"},
			wantCode: rescap.ExitValidation, wantName: rescap.ReasonInvalidDeclaration,
		},
		{
			name: "git-metadata-bad-config-key",
			args: []string{"feature", "resource", "add", "--path", dir, "model-picker",
				"--kind", "git-metadata", "--capability", "config", "--selector", "user.email"},
			wantCode: rescap.ExitValidation, wantName: rescap.ReasonInvalidDeclaration,
		},
		{
			name: "git-metadata-index-entry-missing",
			args: []string{"feature", "resource", "add", "--path", dir, "model-picker",
				"--kind", "git-metadata", "--capability", "index-entry", "--selector", "not/in/index.txt"},
			wantCode: rescap.ExitValidation, wantName: rescap.ReasonInvalidDeclaration,
		},
		{
			name:     "trust-dolt-bad-hex",
			args:     []string{"feature", "resource", "trust-dolt", "--path", dir, "model-picker", "res_79f5ac5dca13", "--binary-sha256", "nothex"},
			wantCode: rescap.ExitValidation, wantName: rescap.ReasonInvalidDeclaration,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runCmdExit(tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d (stderr: %s)", code, tc.wantCode, stderr)
			}
			if !strings.Contains(stderr, tc.wantName) {
				t.Fatalf("stderr = %q, want it to name %s", stderr, tc.wantName)
			}
		})
	}
}

// TestTrustDoltRePinsWithoutTouchingIdentityOrHistory covers §12.6 at
// the CLI boundary.
func TestTrustDoltRePinsWithoutTouchingIdentityOrHistory(t *testing.T) {
	dir := resourceTestRepo(t)
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	// Hand-declare a Dolt resource so the test never needs a real Dolt
	// binary on PATH.
	args := []store.ResourceArg{
		{Key: "contract", Value: store.DoltContractDiffSummary1},
		{Key: "db_path", Value: "config"},
		{Key: "from", Value: "main"},
		{Key: "table", Value: "users"},
		{Key: "to", Value: "HEAD"},
	}
	entry := store.Resource{
		Kind: store.ResourceKindAdapterSnapshot, Selector: "dolt:diff-summary:users",
		Adapter: "dolt", Capability: "diff-summary", Args: args,
		Trust:              &store.ResourceTrust{BinarySHA256: strings.Repeat("3", 64)},
		AddedByToolVersion: "tpatch/test",
	}
	entry.ResourceID = store.ComputeResourceID("model-picker", entry.Kind, entry.Selector, entry.Adapter, entry.Capability, args)
	m := store.ResourcesManifest{Version: store.ResourcesManifestVersion, Feature: "model-picker", Resources: []store.Resource{entry}}
	if err := store.SaveResources(s, m); err != nil {
		t.Fatalf("SaveResources: %v", err)
	}

	newPin := strings.Repeat("6", 64)
	_, stderr, code := runCmdExit("feature", "resource", "trust-dolt", "--path", dir, "model-picker", entry.ResourceID, "--binary-sha256", newPin)
	if code != 0 {
		t.Fatalf("trust-dolt exit %d: %s", code, stderr)
	}
	reloaded, err := store.LoadResources(s, "model-picker")
	if err != nil {
		t.Fatalf("LoadResources: %v", err)
	}
	got := reloaded.Resources[0]
	if got.ResourceID != entry.ResourceID {
		t.Fatalf("resource_id changed: %s -> %s", entry.ResourceID, got.ResourceID)
	}
	if got.Trust == nil || got.Trust.BinarySHA256 != newPin {
		t.Fatalf("pin = %+v, want %s", got.Trust, newPin)
	}
	if got.AddedByToolVersion != "tpatch/test" {
		t.Fatal("trust-dolt must not touch added_by_tool_version")
	}
	if _, err := os.Stat(s.ResourceCurrentPath("model-picker")); !os.IsNotExist(err) {
		t.Fatal("trust-dolt must never create current.json")
	}
}

// TestTrustDoltRefusesNonDoltResource covers the
// resource-not-dolt-adapter refusal.
func TestTrustDoltRefusesNonDoltResource(t *testing.T) {
	dir := resourceTestRepo(t)
	if _, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "git-metadata", "--selector", "head"); code != 0 {
		t.Fatal("add failed")
	}
	_, stderr, code := runCmdExit("feature", "resource", "trust-dolt", "--path", dir, "model-picker",
		"res_acc91dc23a8b", "--binary-sha256", strings.Repeat("a", 64))
	if code != rescap.ExitValidation || !strings.Contains(stderr, rescap.ReasonResourceNotDoltAdapter) {
		t.Fatalf("want resource-not-dolt-adapter exit 2, got code=%d stderr=%s", code, stderr)
	}
}

// TestCaptureAndDiffLifecycle covers the full publish/read cycle,
// including dry-run's "nothing is written" guarantee and the
// content-addressed re-publish.
func TestCaptureAndDiffLifecycle(t *testing.T) {
	dir := resourceTestRepo(t)
	for _, args := range [][]string{
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "ignored-file", "--selector", "config/local-secrets.env.template"},
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "git-metadata", "--selector", "head"},
	} {
		if _, stderr, code := runCmdExit(args...); code != 0 {
			t.Fatalf("setup add failed: %d %s", code, stderr)
		}
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	// diff before any capture: exit 0, "no capture yet".
	stdout, _, code := runCmdExit("feature", "resource", "diff", "--path", dir, "model-picker")
	if code != 0 || !strings.Contains(stdout, "no-capture-yet") {
		t.Fatalf("pre-capture diff: code=%d out=%s", code, stdout)
	}

	// --dry-run writes no tracked batch or pointer, and leaves no
	// ephemeral scratch behind.
	stdout, stderr, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker", "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "dry-run") {
		t.Fatalf("dry-run output = %s", stdout)
	}
	if _, err := os.Stat(s.ResourceCurrentPath("model-picker")); !os.IsNotExist(err) {
		t.Fatal("--dry-run must not write current.json")
	}
	if _, err := os.Stat(s.ResourceBatchesDir("model-picker")); err == nil {
		entries, _ := os.ReadDir(s.ResourceBatchesDir("model-picker"))
		if len(entries) != 0 {
			t.Fatalf("--dry-run wrote %d batch files", len(entries))
		}
	}
	assertNoEphemeralScratch(t, dir)
	// The persistent .lock file is explicitly NOT part of the "nothing
	// persists" guarantee.
	if _, err := os.Stat(filepath.Join(rescap.ScratchRoot(dir, "model-picker"), ".lock")); err != nil {
		t.Fatalf("--dry-run should still create/keep the persistent lock: %v", err)
	}

	stdout, stderr, code = runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker", "--json")
	if code != 0 {
		t.Fatalf("capture exit %d: %s", code, stderr)
	}
	var captured struct {
		BatchID    string   `json:"batch_id"`
		Resources  []string `json:"resources"`
		WroteBatch bool     `json:"wrote_batch"`
	}
	if err := json.Unmarshal([]byte(stdout), &captured); err != nil {
		t.Fatalf("capture --json: %v\n%s", err, stdout)
	}
	if !captured.WroteBatch || len(captured.Resources) != 2 {
		t.Fatalf("unexpected capture result: %+v", captured)
	}
	if !strings.HasPrefix(captured.BatchID, "rb_") || len(captured.BatchID) != len("rb_")+64 {
		t.Fatalf("batch_id must be rb_ plus the full 64-hex digest: %s", captured.BatchID)
	}
	assertNoEphemeralScratch(t, dir)

	// A re-capture of unchanged content writes zero new batch bytes.
	stdout, _, code = runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker", "--json")
	if code != 0 {
		t.Fatalf("re-capture exit %d", code)
	}
	var recaptured struct {
		BatchID    string `json:"batch_id"`
		WroteBatch bool   `json:"wrote_batch"`
	}
	if err := json.Unmarshal([]byte(stdout), &recaptured); err != nil {
		t.Fatalf("re-capture --json: %v", err)
	}
	if recaptured.BatchID != captured.BatchID || recaptured.WroteBatch {
		t.Fatalf("an unchanged re-capture must reuse the batch: %+v", recaptured)
	}
	entries, _ := os.ReadDir(s.ResourceBatchesDir("model-picker"))
	if len(entries) != 1 {
		t.Fatalf("batches directory holds %d files, want exactly 1", len(entries))
	}

	stdout, _, code = runCmdExit("feature", "resource", "diff", "--path", dir, "model-picker", "--json")
	if code != 0 {
		t.Fatalf("diff exit %d", code)
	}
	if !strings.Contains(stdout, `"status": "unchanged"`) {
		t.Fatalf("diff should report unchanged: %s", stdout)
	}

	// Change the ignored file: diff names the changed field, and a new
	// capture produces a new batch. Reverting repoints at the original
	// batch without creating a third file.
	writeResourceFile(t, dir, "config/local-secrets.env.template", "A=2\n")
	stdout, _, code = runCmdExit("feature", "resource", "diff", "--path", dir, "model-picker", "--json")
	if code != 0 {
		t.Fatalf("diff exit %d", code)
	}
	if !strings.Contains(stdout, "hash differs") {
		t.Fatalf("diff should name the changed field: %s", stdout)
	}
	if _, _, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker"); code != 0 {
		t.Fatalf("second capture exit %d", code)
	}
	entries, _ = os.ReadDir(s.ResourceBatchesDir("model-picker"))
	if len(entries) != 2 {
		t.Fatalf("changed content should add one batch, got %d", len(entries))
	}
	writeResourceFile(t, dir, "config/local-secrets.env.template", "A=1\n")
	if _, _, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker"); code != 0 {
		t.Fatalf("third capture exit %d", code)
	}
	entries, _ = os.ReadDir(s.ResourceBatchesDir("model-picker"))
	if len(entries) != 2 {
		t.Fatalf("reverting must repoint rather than write a third batch, got %d", len(entries))
	}
	pointer, err := s.LoadCurrentPointer("model-picker")
	if err != nil {
		t.Fatalf("LoadCurrentPointer: %v", err)
	}
	if pointer.CurrentBatchID != captured.BatchID {
		t.Fatalf("the pointer should name the original batch again: %s", pointer.CurrentBatchID)
	}
}

// assertNoEphemeralScratch proves every es_<id>/ directory was removed.
func assertNoEphemeralScratch(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(rescap.ScratchRoot(dir, "model-picker"))
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "es_") {
			t.Fatalf("ephemeral scratch %s survived the invocation", e.Name())
		}
	}
}

// TestCaptureSubsetTargeting covers --resource.
func TestCaptureSubsetTargeting(t *testing.T) {
	dir := resourceTestRepo(t)
	for _, args := range [][]string{
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "ignored-file", "--selector", "config/local-secrets.env.template"},
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "git-metadata", "--selector", "head"},
	} {
		if _, _, code := runCmdExit(args...); code != 0 {
			t.Fatal("setup add failed")
		}
	}
	stdout, _, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker", "--resource", "res_acc91dc23a8b", "--json")
	if code != 0 {
		t.Fatalf("subset capture exit %d", code)
	}
	var captured struct {
		Resources []string `json:"resources"`
	}
	if err := json.Unmarshal([]byte(stdout), &captured); err != nil {
		t.Fatalf("capture --json: %v", err)
	}
	if len(captured.Resources) != 1 || captured.Resources[0] != "res_acc91dc23a8b" {
		t.Fatalf("subset targeting failed: %+v", captured)
	}
}

// TestCaptureAllOrNothingStaging covers §7.3 step 1: one resource's
// staging failure aborts the whole invocation with that refusal's own
// code, and no batch is written for the other, unaffected resources.
func TestCaptureAllOrNothingStaging(t *testing.T) {
	dir := resourceTestRepo(t)
	for _, args := range [][]string{
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "ignored-file", "--selector", "config/local-secrets.env.template"},
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "git-metadata", "--selector", "head"},
	} {
		if _, _, code := runCmdExit(args...); code != 0 {
			t.Fatal("setup add failed")
		}
	}
	// Make the ignored file fail its gate at capture time by removing
	// only its own ignore rule; every other rule, including the
	// .tpatch/local/ one the mutator gate depends on, is preserved.
	removeGitignoreLine(t, dir, "config/local-secrets.env.template")
	_, stderr, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker")
	if code != rescap.ExitRefusal {
		t.Fatalf("want exit 3, got code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "error: "+rescap.ReasonNotIgnored+":") {
		t.Fatalf("want the not-ignored refusal specifically, got %s", stderr)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if _, err := os.Stat(s.ResourceCurrentPath("model-picker")); !os.IsNotExist(err) {
		t.Fatal("an aborted staging must publish nothing at all")
	}
	assertNoEphemeralScratch(t, dir)
}

// TestCaptureRedactionRefusal covers the hard refusal at the CLI
// boundary, including that no batch is written for other resources.
func TestCaptureRedactionRefusal(t *testing.T) {
	dir := resourceTestRepo(t)
	writeResourceFile(t, dir, "config/local-secrets.env.template", "PGURL=postgres://u:p@h:5432/db\n")
	if _, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "ignored-file", "--selector", "config/local-secrets.env.template"); code != 0 {
		t.Fatal("add failed")
	}
	_, stderr, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker")
	if code != rescap.ExitRefusal || !strings.Contains(stderr, rescap.ReasonRedactionRefused) {
		t.Fatalf("want redaction-refused exit 3, got code=%d stderr=%s", code, stderr)
	}
}

// TestDirectorySelectorCapture covers the directory result variant end
// to end at the CLI boundary.
func TestDirectorySelectorCapture(t *testing.T) {
	dir := resourceTestRepo(t)
	writeResourceFile(t, dir, "config/dir-selector/a.txt", "")
	writeResourceFile(t, dir, "config/dir-selector/sub/b.sh", "#!/bin/sh\necho hi\n")
	if err := os.Chmod(filepath.Join(dir, "config", "dir-selector", "sub", "b.sh"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if _, stderr, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "ignored-file", "--selector", "config/dir-selector"); code != 0 {
		t.Fatalf("add exit %d: %s", code, stderr)
	}
	if _, stderr, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker"); code != 0 {
		t.Fatalf("capture exit %d: %s", code, stderr)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	pointer, err := s.LoadCurrentPointer("model-picker")
	if err != nil {
		t.Fatalf("LoadCurrentPointer: %v", err)
	}
	batch, err := s.LoadBatch("model-picker", pointer.CurrentBatchID)
	if err != nil {
		t.Fatalf("LoadBatch: %v", err)
	}
	result := batch.Results[0].Result
	count, _ := result.Field("file_count")
	if count.Uint != 2 {
		t.Fatalf("file_count = %d, want 2", count.Uint)
	}
	files, _ := result.Field("files")
	if len(files.Array) != 2 {
		t.Fatalf("files[] length = %d, want 2", len(files.Array))
	}

	// A chmod-only change is reported as a per-file mode difference.
	if err := os.Chmod(filepath.Join(dir, "config", "dir-selector", "a.txt"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	stdout, _, code := runCmdExit("feature", "resource", "diff", "--path", dir, "model-picker")
	if code != 0 {
		t.Fatalf("diff exit %d", code)
	}
	if !strings.Contains(stdout, "file mode differs") {
		t.Fatalf("a chmod-only change must be named: %s", stdout)
	}
}

// TestTrackedBatchMissingIsExitOne covers §4.1 at the CLI boundary.
func TestTrackedBatchMissingIsExitOne(t *testing.T) {
	dir := resourceTestRepo(t)
	if _, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "git-metadata", "--selector", "head"); code != 0 {
		t.Fatal("add failed")
	}
	if _, _, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker"); code != 0 {
		t.Fatal("capture failed")
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	entries, err := os.ReadDir(s.ResourceBatchesDir("model-picker"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one batch: %v %v", entries, err)
	}
	if err := os.Remove(filepath.Join(s.ResourceBatchesDir("model-picker"), entries[0].Name())); err != nil {
		t.Fatalf("remove: %v", err)
	}
	_, stderr, code := runCmdExit("feature", "resource", "list", "--path", dir, "model-picker")
	if code != rescap.ExitInternal || !strings.Contains(stderr, rescap.ReasonTrackedBatchMissing) {
		t.Fatalf("want tracked-batch-missing exit 1, got code=%d stderr=%s", code, stderr)
	}
	_, stderr, code = runCmdExit("feature", "resource", "diff", "--path", dir, "model-picker")
	if code != rescap.ExitInternal || !strings.Contains(stderr, rescap.ReasonTrackedBatchMissing) {
		t.Fatalf("diff: want tracked-batch-missing exit 1, got code=%d stderr=%s", code, stderr)
	}
}

// TestResourcesFileCorruptIsExitThree covers the load-time corruption
// refusal at the CLI boundary.
func TestResourcesFileCorruptIsExitThree(t *testing.T) {
	dir := resourceTestRepo(t)
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	path := s.ResourcesPath("model-picker")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	body := `{"version":1,"feature":"model-picker","resources":[{"resource_id":"res_deadbeef0000","kind":"git-metadata","selector":"head","adapter":"","capability":"","args":[],"trust":null,"added_by_tool_version":"tpatch/test"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, stderr, code := runCmdExit("feature", "resource", "list", "--path", dir, "model-picker")
	if code != rescap.ExitRefusal || !strings.Contains(stderr, store.ReasonResourcesFileCorrupt) {
		t.Fatalf("want resources-file-corrupt exit 3, got code=%d stderr=%s", code, stderr)
	}
}

// TestLocalPathTrackedRefusal covers §10.3 step 2 for every mutator.
func TestLocalPathTrackedRefusal(t *testing.T) {
	dir := resourceTestRepo(t)
	// Declare a resource first so `capture` reaches the gate rather
	// than short-circuiting on the zero-resource preflight.
	if _, stderr, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "git-metadata", "--selector", "head"); code != 0 {
		t.Fatalf("setup add failed: %d %s", code, stderr)
	}
	writeResourceFile(t, dir, ".tpatch/local/oops.txt", "x")
	cmd := exec.Command("git", "add", "-f", ".tpatch/local/oops.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	mutators := [][]string{
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "git-metadata", "--selector", "head"},
		{"feature", "resource", "clear", "--path", dir, "model-picker"},
		{"feature", "resource", "capture", "--path", dir, "model-picker"},
		{"feature", "resource", "trust-dolt", "--path", dir, "model-picker", "res_acc91dc23a8b", "--binary-sha256", strings.Repeat("a", 64)},
		{"feature", "resource", "remove", "--path", dir, "model-picker", "res_acc91dc23a8b"},
	}
	for _, args := range mutators {
		verb := args[2]
		t.Run(verb, func(t *testing.T) {
			_, stderr, code := runCmdExit(args...)
			if code != rescap.ExitRefusal || !strings.Contains(stderr, rescap.ReasonLocalPathTracked) {
				t.Fatalf("%s: want local-path-tracked exit 3, got code=%d stderr=%s", verb, code, stderr)
			}
		})
	}
	// list and diff never run the gate: they are pure reads.
	if _, _, code := runCmdExit("feature", "resource", "list", "--path", dir, "model-picker"); code != 0 {
		t.Fatal("list must not run the mutator gate")
	}
	if _, _, code := runCmdExit("feature", "resource", "diff", "--path", dir, "model-picker"); code != 0 {
		t.Fatal("diff must not run the mutator gate")
	}
}

// TestRecordResourcesTwoDomainOrdering covers §11: the zero-resource
// preflight, the Git-gated publish, and the partial-domain refusal.
func TestRecordResourcesTwoDomainOrdering(t *testing.T) {
	t.Run("zero-resource-preflight", func(t *testing.T) {
		dir := resourceTestRepo(t)
		_, stderr, code := runCmdExit("record", "--path", dir, "model-picker", "--resources")
		if code != rescap.ExitInternal || !strings.Contains(stderr, rescap.ReasonNoResourcesDeclared) {
			t.Fatalf("want no-resources-declared exit 1, got code=%d stderr=%s", code, stderr)
		}
	})

	t.Run("git-failure-discards-the-candidate", func(t *testing.T) {
		dir := resourceTestRepo(t)
		if _, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
			"--kind", "git-metadata", "--selector", "head"); code != 0 {
			t.Fatal("add failed")
		}
		// Nothing to capture: record's own Git-side failure fires.
		_, _, code := runCmdExit("record", "--path", dir, "model-picker", "--resources")
		if code == 0 {
			t.Fatal("an empty Git capture should fail record")
		}
		s, err := store.Open(dir)
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		if _, err := os.Stat(s.ResourceCurrentPath("model-picker")); !os.IsNotExist(err) {
			t.Fatal("no tracked resource write may happen before Git succeeds")
		}
	})

	t.Run("git-success-publishes", func(t *testing.T) {
		dir := resourceTestRepo(t)
		if _, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
			"--kind", "git-metadata", "--selector", "head"); code != 0 {
			t.Fatal("add failed")
		}
		writeResourceFile(t, dir, "src/feature.txt", "hello\n")
		stdout, stderr, code := runCmdExit("record", "--path", dir, "model-picker", "--resources")
		if code != 0 {
			t.Fatalf("record --resources exit %d: %s", code, stderr)
		}
		if !strings.Contains(stdout, "Recorded patch for model-picker") {
			t.Fatalf("the Git-side output must be preserved verbatim: %s", stdout)
		}
		if !strings.Contains(stdout, "published rb_") {
			t.Fatalf("the resource domain must publish after Git success: %s", stdout)
		}
		s, err := store.Open(dir)
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		if _, err := os.Stat(s.ResourceCurrentPath("model-picker")); err != nil {
			t.Fatalf("current.json should exist: %v", err)
		}
	})

	t.Run("partial-domain-when-staging-fails-after-git-success", func(t *testing.T) {
		dir := resourceTestRepo(t)
		if _, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
			"--kind", "ignored-file", "--selector", "config/local-secrets.env.template"); code != 0 {
			t.Fatal("add failed")
		}
		// Break only this selector's ignore rule so staging fails,
		// while leaving a real Git-side change so the Git domain
		// succeeds and the partial-domain path is exercised.
		removeGitignoreLine(t, dir, "config/local-secrets.env.template")
		writeResourceFile(t, dir, "src/feature.txt", "hello\n")
		stdout, stderr, code := runCmdExit("record", "--path", dir, "model-picker", "--resources")
		if code != rescap.ExitInternal {
			t.Fatalf("want resource-domain-incomplete exit 1, got %d: %s", code, stderr)
		}
		if !strings.Contains(stderr, rescap.ReasonResourceDomainIncomplete) {
			t.Fatalf("stderr = %s", stderr)
		}
		if !strings.Contains(stderr, "canonical patch recorded successfully") {
			t.Fatalf("the partial-domain message must be exact: %s", stderr)
		}
		if !strings.Contains(stderr, "tpatch feature resource capture model-picker") {
			t.Fatalf("the retry guidance must name the exact command: %s", stderr)
		}
		if !strings.Contains(stdout, "Recorded patch for model-picker") {
			t.Fatalf("the Git side must still have succeeded: %s", stdout)
		}
	})

	t.Run("without-the-flag-record-is-unchanged", func(t *testing.T) {
		dir := resourceTestRepo(t)
		writeResourceFile(t, dir, "src/feature.txt", "hello\n")
		stdout, stderr, code := runCmdExit("record", "--path", dir, "model-picker")
		if code != 0 {
			t.Fatalf("plain record exit %d: %s", code, stderr)
		}
		if strings.Contains(stdout, "published rb_") {
			t.Fatal("record without --resources must not touch the resource domain")
		}
	})
}

// TestCaptureLockContentionAcrossProcesses proves the flock genuinely
// serializes two concurrent mutators, using a second process rather
// than a second in-process acquire.
func TestCaptureLockContentionAcrossProcesses(t *testing.T) {
	dir := resourceTestRepo(t)
	if _, _, code := runCmdExit("feature", "resource", "add", "--path", dir, "model-picker",
		"--kind", "git-metadata", "--selector", "head"); code != 0 {
		t.Fatal("add failed")
	}
	scratchRoot := rescap.ScratchRoot(dir, "model-picker")
	held, err := rescap.AcquireLock(scratchRoot, dir)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer func() { _ = held.Release() }()

	_, stderr, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker")
	if code != rescap.ExitRefusal || !strings.Contains(stderr, rescap.ReasonCaptureInProgress) {
		t.Fatalf("want capture-in-progress exit 3, got code=%d stderr=%s", code, stderr)
	}
	if err := held.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, stderr, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker"); code != 0 {
		t.Fatalf("capture after release exit %d: %s", code, stderr)
	}
}

// TestNoTimestampsInTrackedResourceArtifacts proves the whole tracked
// tree stays timestamp-free.
func TestNoTimestampsInTrackedResourceArtifacts(t *testing.T) {
	dir := resourceTestRepo(t)
	for _, args := range [][]string{
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "ignored-file", "--selector", "config/local-secrets.env.template"},
		{"feature", "resource", "add", "--path", dir, "model-picker", "--kind", "git-metadata", "--selector", "head"},
	} {
		if _, _, code := runCmdExit(args...); code != 0 {
			t.Fatal("setup add failed")
		}
	}
	if _, _, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker"); code != 0 {
		t.Fatal("capture failed")
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	forbidden := []string{"timestamp", "created_at", "captured_at", "recorded_at", "latest_batch_id"}
	roots := []string{s.ResourcesPath("model-picker"), s.ResourceCapturesDir("model-picker")}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			for _, f := range forbidden {
				if strings.Contains(string(body), f) {
					t.Errorf("%s contains the forbidden field %q", path, f)
				}
			}
			return nil
		})
	}
}

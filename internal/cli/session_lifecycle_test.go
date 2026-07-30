package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// runSessionCmd is a runCmd variant that ALSO surfaces the error text
// returned by cobra Execute — the root command has SilenceErrors=true,
// so error strings never reach the caller through the stderr buffer.
// Sessions rely on rich refusal messages carrying all six D6 mandates,
// which arrive via the returned error rather than the stream.
func runSessionCmd(args ...string) (stdout, errMsg string, code int) {
	var outBuf, errBuf bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		combined := errBuf.String()
		if combined != "" && !strings.HasSuffix(combined, "\n") {
			combined += "\n"
		}
		combined += err.Error()
		return outBuf.String(), combined, 1
	}
	return outBuf.String(), errBuf.String(), 0
}

// initGitRepo makes `dir` a working tree with a default identity so
// `git check-ignore` can answer. Skips the enclosing test if git is
// missing. Mirrors the workflow-package fixture.
func initGitRepoForCLI(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH:", err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, string(out))
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "test")
	run("config", "commit.gpgsign", "false")
}

// setupSessionRepo initializes a git repo, runs `tpatch init`, and
// adds a feature so `tpatch session start <slug>` has a target.
// Returns the resolved tmp dir and the slug.
func setupSessionRepo(t *testing.T, title string) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	initGitRepoForCLI(t, tmp)
	if _, stderr, code := runSessionCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("tpatch init failed: %s", stderr)
	}
	if _, stderr, code := runSessionCmd("add", "--path", tmp, title); code != 0 {
		t.Fatalf("tpatch add failed: %s", stderr)
	}
	// Slug is the deterministic kebab-case of the title per store rules.
	slug := deriveSlugForTest(t, tmp)
	return tmp, slug
}

// deriveSlugForTest lists features and returns the first slug found.
func deriveSlugForTest(t *testing.T, root string) string {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	features, err := s.ListFeatures()
	if err != nil {
		t.Fatalf("list features: %v", err)
	}
	if len(features) == 0 {
		t.Fatalf("no features created")
	}
	return features[0].Slug
}

// TestSessionInitAppendsGitignore proves `tpatch init` installs the
// `.tpatch/local/` rule per PRD §4 D6 mandate 1 + Rule 19.
func TestSessionInitAppendsGitignore(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoForCLI(t, tmp)
	if _, stderr, code := runSessionCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("tpatch init failed: %s", stderr)
	}
	giBytes, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(giBytes), ".tpatch/local/") {
		t.Fatalf("expected .gitignore to contain %q; got:\n%s", workflow.LocalIgnoreRule, string(giBytes))
	}
}

// TestSessionStartRefusesWithoutGitignore verifies mandate 3+4+5:
// session start refuses when .tpatch/local/ is not effectively
// ignored. We drive this by deleting .gitignore after init so
// `check-ignore` reports the path is not ignored.
func TestSessionStartRefusesWithoutGitignore(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Fix session start")
	// Remove the .gitignore file so mandate 5 (effective check) fails.
	if err := os.Remove(filepath.Join(tmp, ".gitignore")); err != nil {
		t.Fatalf("rm .gitignore: %v", err)
	}
	_, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug)
	if code == 0 {
		t.Fatalf("expected session start to refuse; got success")
	}
	if !strings.Contains(stderr, ".tpatch/local/") {
		t.Fatalf("expected refusal message to name .tpatch/local/; got %q", stderr)
	}
	// The rendered message MUST enumerate all six mandates.
	for i := 1; i <= 6; i++ {
		needle := "  " + itoa(i) + "."
		if !strings.Contains(stderr, needle) {
			t.Errorf("refusal message must enumerate mandate %d; got:\n%s", i, stderr)
		}
	}
}

// TestSessionStartIdempotent verifies PRD §3 D1.5: starting twice on
// the same feature returns the existing cs_<12hex> and writes no new
// buffer.
func TestSessionStartIdempotent(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Session idempotence")
	out1, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug)
	if code != 0 {
		t.Fatalf("first start failed: %s / %s", out1, stderr)
	}
	// Second start MUST succeed and emit an idempotent notice.
	out2, stderr2, code2 := runSessionCmd("session", "start", "--path", tmp, slug)
	if code2 != 0 {
		t.Fatalf("second start failed: %s / %s", out2, stderr2)
	}
	if !strings.Contains(out2, "already active") {
		t.Fatalf("expected 'already active' idempotence notice on second start; got %q", out2)
	}
	// The single active session should be listed.
	listOut, _, listCode := runSessionCmd("session", "list", "--path", tmp, "--json")
	if listCode != 0 {
		t.Fatalf("list failed: %s", listOut)
	}
	var payload SessionListJSON
	if err := json.Unmarshal([]byte(listOut), &payload); err != nil {
		t.Fatalf("list JSON parse: %v (%s)", err, listOut)
	}
	activeCount := 0
	for _, item := range payload.Sessions {
		if item.Feature == slug && item.State == string(store.SessionActive) {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active session after idempotent start, got %d (%+v)", activeCount, payload.Sessions)
	}
}

// TestSessionStopTransitions verifies PRD §3 D4: active -> closed.
// Repeated stop is idempotent per PRD §8.11.
func TestSessionStopTransitions(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Session stop transitions")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start failed: %s", stderr)
	}
	out, stderr, code := runSessionCmd("session", "stop", "--path", tmp, slug)
	if code != 0 {
		t.Fatalf("stop failed: %s / %s", out, stderr)
	}
	if !strings.Contains(out, "closed session") {
		t.Fatalf("expected 'closed session' output; got %q", out)
	}
	// Second stop is idempotent.
	out2, stderr2, code2 := runSessionCmd("session", "stop", "--path", tmp, slug)
	if code2 != 0 {
		t.Fatalf("second stop failed: %s / %s", out2, stderr2)
	}
	if !strings.Contains(out2, "already closed") && !strings.Contains(out2, "no eligible") {
		// After close, no active sessions remain — pickSessionForOp
		// returns "no eligible sessions" which is a valid idempotent
		// signal here.
		t.Fatalf("expected idempotence on second stop; got %q", out2)
	}
}

// TestSessionListJSONDeterministic verifies PRD §6 D14 + §8.15
// deterministic output: two runs produce byte-identical JSON.
func TestSessionListJSONDeterministic(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Session list determinism")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start failed: %s", stderr)
	}
	first, _, code := runSessionCmd("session", "list", "--path", tmp, "--json")
	if code != 0 {
		t.Fatalf("first list failed")
	}
	second, _, code2 := runSessionCmd("session", "list", "--path", tmp, "--json")
	if code2 != 0 {
		t.Fatalf("second list failed")
	}
	if first != second {
		t.Fatalf("session list --json not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	// Schema version must be present.
	if !strings.Contains(first, `"schema_version": "session/v1"`) {
		t.Fatalf("expected schema_version in list JSON; got %s", first)
	}
}

// TestSessionPurgeDryRunDefault verifies PRD §6 D14: purge defaults
// to a dry-run unless --yes is passed.
func TestSessionPurgeDryRunDefault(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Session purge default")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start failed: %s", stderr)
	}
	// Default is dry-run — pass no --yes.
	out, _, code := runSessionCmd("session", "purge", "--path", tmp, slug)
	if code != 0 {
		t.Fatalf("purge default failed: %s", out)
	}
	if !strings.Contains(out, "would remove") {
		t.Fatalf("expected 'would remove' in dry-run purge output; got %q", out)
	}
	// Session must still exist on disk.
	listOut, _, _ := runSessionCmd("session", "list", "--path", tmp, "--json")
	var payload SessionListJSON
	if err := json.Unmarshal([]byte(listOut), &payload); err != nil {
		t.Fatalf("list JSON parse: %v", err)
	}
	found := false
	for _, item := range payload.Sessions {
		if item.Feature == slug {
			found = true
		}
	}
	if !found {
		t.Fatalf("dry-run purge must not delete; session missing after purge")
	}

	// Now purge with --yes — session should disappear.
	out2, errMsg2, code2 := runSessionCmd("session", "purge", "--path", tmp, slug, "--yes")
	if code2 != 0 {
		t.Fatalf("purge --yes failed: out=%q err=%q", out2, errMsg2)
	}
	if !strings.Contains(out2, "removed") {
		t.Fatalf("expected 'removed' output on purge --yes; got %q", out2)
	}
	listOut2, _, _ := runSessionCmd("session", "list", "--path", tmp, "--json")
	var payload2 SessionListJSON
	if err := json.Unmarshal([]byte(listOut2), &payload2); err != nil {
		t.Fatalf("list JSON parse: %v", err)
	}
	for _, item := range payload2.Sessions {
		if item.Feature == slug {
			t.Fatalf("session still present after purge --yes: %+v", item)
		}
	}
}

// TestSessionStartRefusesUnknownFeature verifies session start
// refuses when the feature does not exist per PRD §3 D1.1.
func TestSessionStartRefusesUnknownFeature(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoForCLI(t, tmp)
	if _, stderr, code := runSessionCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("tpatch init failed: %s", stderr)
	}
	_, stderr, code := runSessionCmd("session", "start", "--path", tmp, "nonexistent-slug")
	if code == 0 {
		t.Fatalf("expected refusal for unknown feature; got success")
	}
	if !strings.Contains(stderr, "nonexistent-slug") {
		t.Fatalf("expected refusal to name slug; got %q", stderr)
	}
}

// TestSessionStartCrossFeatureIsolation proves feature-A session
// cannot observe feature-B's buffer (PRD §7 D18). Regression fixture:
// start session for slug A; ensure `session list <slug-B>` returns
// nothing.
func TestSessionStartCrossFeatureIsolation(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoForCLI(t, tmp)
	if _, stderr, code := runSessionCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("tpatch init failed: %s", stderr)
	}
	if _, stderr, code := runSessionCmd("add", "--path", tmp, "Feature A cross iso"); code != 0 {
		t.Fatalf("add A failed: %s", stderr)
	}
	if _, stderr, code := runSessionCmd("add", "--path", tmp, "Feature B cross iso"); code != 0 {
		t.Fatalf("add B failed: %s", stderr)
	}
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	features, err := s.ListFeatures()
	if err != nil {
		t.Fatalf("list features: %v", err)
	}
	if len(features) < 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}
	slugA, slugB := features[0].Slug, features[1].Slug

	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slugA); code != 0 {
		t.Fatalf("start A failed: %s", stderr)
	}

	// List filtered by slug B — must be empty.
	out, _, code := runSessionCmd("session", "list", "--path", tmp, "--json", slugB)
	if code != 0 {
		t.Fatalf("list B failed: %s", out)
	}
	var payload SessionListJSON
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("list JSON parse: %v (%s)", err, out)
	}
	if len(payload.Sessions) != 0 {
		t.Fatalf("expected zero sessions under slug B; got %+v", payload.Sessions)
	}

	// Directly loading slugA's session under slugB must refuse per D18.
	entries, err := s.ListSessions(slugA)
	if err != nil {
		t.Fatalf("list A sessions: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 session under A, got %d", len(entries))
	}
	csID := entries[0].SessionID
	if _, err := s.LoadSession(slugB, csID); err == nil {
		t.Fatalf("expected LoadSession(B, %s) to refuse; got success", csID)
	}
}

// TestSessionSummarizeInvalidFlagPairs verifies mutually exclusive
// --dry-run + --write and the --promote-requires-write constraint.
func TestSessionSummarizeInvalidFlagPairs(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Session summarize flags")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start failed: %s", stderr)
	}
	// --dry-run + --write must refuse.
	_, stderr, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--dry-run", "--write")
	if code == 0 {
		t.Fatalf("expected refusal for --dry-run + --write; got success")
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected 'mutually exclusive' in stderr; got %q", stderr)
	}
	// --promote without --write must refuse.
	_, stderr2, code2 := runSessionCmd("session", "summarize", "--path", tmp, slug, "--promote")
	if code2 == 0 {
		t.Fatalf("expected refusal for --promote without --write; got success")
	}
	if !strings.Contains(stderr2, "requires --write") {
		t.Fatalf("expected 'requires --write' in stderr; got %q", stderr2)
	}
}

// itoa is a small no-fmt-import helper for the mandate loop above.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [3]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

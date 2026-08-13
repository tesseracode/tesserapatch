package gitutil

// Rev-3 apply-probe classifier suite — v0.15.1 Wave C, adjudication P1.
//
// Rev-2 admitted any non-answer exit whose stderr merely CONTAINED a
// broad English fragment (`already exists`, `new file`, `deleted
// file`, …), so a wrapper failure, a signalled process or a translated
// diagnostic could be promoted to a patch answer. These tests pin the
// narrow, anchored, C-locale-only grammar that replaced it, and the
// real-Git diagnostics it was measured against.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ── Pure table ───────────────────────────────────────────────────────────

func TestApplyProbeAnswered_Table(t *testing.T) {
	const noValidPatches = `error: No valid patches in input (allow with "--allow-empty")`
	const corruptPatch = `error: corrupt patch at ../p.patch:5`
	const corruptBinary = `error: corrupt binary patch at ../p.patch:6: NOTBASE85`
	const fragment = `error: patch fragment without header at ../p.patch:1: @@ -1,3 +1,3 @@`
	const missingObject = `error: failed to read src/app.txt`
	const network = `fatal: 'git://127.0.0.1:1/dead' does not appear to be a git repository`

	cases := []struct {
		name     string
		exit     int
		ok       bool
		stderr   string
		answered bool
	}{
		// Unconditional answers.
		{"success", 0, true, "", true},
		{"success-with-verbose-noise", 0, true, "Checking patch f.txt...", true},
		{"exit-1-does-not-apply", 1, false, "error: f.txt: patch does not apply", true},
		{"exit-1-already-exists", 1, false, "error: f.txt: already exists in index", true},
		{"exit-1-does-not-exist", 1, false, "error: gone.txt: does not exist in index", true},
		{"exit-1-empty-stderr", 1, false, "", true},
		{"exit-1-unknown-stderr", 1, false, "something entirely unexpected", true},

		// Exit 128 with a measured malformed-patch diagnostic.
		{"128-no-valid-patches", 128, false, noValidPatches, true},
		{"128-corrupt-patch-path-line", 128, false, corruptPatch, true},
		{"128-corrupt-patch-at-line", 128, false, "error: corrupt patch at line 5", true},
		{"128-patch-fragment-without-header", 128, false, fragment, true},
		{"128-patch-with-only-garbage", 128, false, "error: patch with only garbage at line 7", true},
		{"128-corrupt-binary-paired", 128, false, corruptBinary + "\n" + noValidPatches, true},
		{"128-trailing-newline", 128, false, noValidPatches + "\n", true},
		{"128-verbose-informational-plus-diagnostic", 128, false, "Checking patch f.txt...\n" + corruptPatch, true},

		// Exit 128 that must NOT be admitted.
		{"128-broad-phrase-spoof-already-exists", 128, false, "error: f.txt: already exists in index", false},
		{"128-broad-phrase-spoof-new-file", 128, false, "wrapper: new file mode 100644", false},
		{"128-broad-phrase-spoof-deleted-file", 128, false, "deleted file mode 100644", false},
		{"128-broad-phrase-spoof-does-not-apply", 128, false, "error: f.txt: patch does not apply", false},
		{"128-fatal-spoof", 128, false, `fatal: No valid patches in input (allow with "--allow-empty")`, false},
		{"128-unknown", 128, false, "fatal: internal error: object database offline", false},
		{"128-empty-stderr", 128, false, "", false},
		{"128-whitespace-only", 128, false, "   \n\t\n", false},
		{"128-mixed-recognised-plus-missing-object", 128, false, noValidPatches + "\n" + missingObject, false},
		{"128-mixed-recognised-plus-network", 128, false, corruptPatch + "\n" + network, false},
		{"128-mixed-recognised-plus-wrapper", 128, false, corruptPatch + "\nwrapper: shim intercepted this call", false},
		{"128-informational-only", 128, false, "Checking patch f.txt...", false},
		{"128-uppercased-diagnostic", 128, false, strings.ToUpper(noValidPatches), false},
		{"128-translated-diagnostic", 128, false, "erreur : aucun correctif valide en entrée", false},
		{"128-leading-space", 128, false, " " + noValidPatches, false},

		// Every other exit is a failure even with a recognised phrase.
		{"signalled-negative", -1, false, noValidPatches, false},
		{"exit-2", 2, false, noValidPatches, false},
		{"exit-126-not-executable", 126, false, noValidPatches, false},
		{"exit-127-not-found", 127, false, noValidPatches, false},
		{"exit-129", 129, false, noValidPatches, false},
		{"exit-137-sigkill", 137, false, noValidPatches, false},
		{"exit-255", 255, false, corruptPatch, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ApplyProbeAnswered(tc.exit, tc.ok, tc.stderr); got != tc.answered {
				t.Fatalf("ApplyProbeAnswered(exit=%d ok=%v stderr=%q) = %v, want %v",
					tc.exit, tc.ok, tc.stderr, got, tc.answered)
			}
		})
	}
}

// TestIsMalformedPatchDiagnostic_RequiresEveryLine pins the "all lines
// must match, at least one diagnostic" rule directly.
func TestIsMalformedPatchDiagnostic_RequiresEveryLine(t *testing.T) {
	const ok = `error: corrupt patch at ../p.patch:5`
	if !IsMalformedPatchDiagnostic(ok) {
		t.Errorf("a single recognised diagnostic must be admitted")
	}
	if IsMalformedPatchDiagnostic("") {
		t.Errorf("empty stderr carries no diagnostic")
	}
	if IsMalformedPatchDiagnostic("Checking patch f.txt...") {
		t.Errorf("an informational line alone must not be admitted")
	}
	if IsMalformedPatchDiagnostic(ok + "\nunexpected trailing line") {
		t.Errorf("one unrecognised line must reject the whole diagnostic")
	}
	if IsMalformedPatchDiagnostic("unexpected leading line\n" + ok) {
		t.Errorf("one unrecognised line must reject the whole diagnostic")
	}
}

// ── Real-Git goldens ─────────────────────────────────────────────────────

// gitApplyProbe runs the real `git apply --check` against a fixture and
// returns the measured exit code and stderr, under the same offline +
// C-locale environment the evidence reader uses.
func gitApplyProbe(t *testing.T, repo, patchBody string, extraArgs ...string) (int, string) {
	t.Helper()
	patch := filepath.Join(t.TempDir(), "p.patch")
	if err := os.WriteFile(patch, []byte(patchBody), 0o644); err != nil {
		t.Fatal(err)
	}
	args := append([]string{"apply", "--check"}, extraArgs...)
	args = append(args, patch)
	res := RunOfflineGitInResult(repo, args...)
	return res.ExitCode, res.Stderr
}

func newApplyFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", "."},
		{"config", "user.email", "t@e.com"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "seed"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

// TestRealGit_MalformedPatchesExit128AndAreAdmitted measures the actual
// diagnostics — no mocking — and asserts the grammar admits each one.
func TestRealGit_MalformedPatchesExit128AndAreAdmitted(t *testing.T) {
	repo := newApplyFixtureRepo(t)
	cases := map[string]string{
		"empty":                   "",
		"garbage":                 "this is not a diff at all\n",
		"prose":                   "Subject: hello\n\nsome prose\nmore prose\n",
		"truncated-hunk":          "diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,3 +1,3 @@\n",
		"garbage-hunk":            "diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,3 +1,3 @@\n!!!garbage!!!\n",
		"fragment-without-header": "@@ -1,3 +1,3 @@\n-a\n+A\n",
		"corrupt-binary":          "diff --git a/b.bin b/b.bin\nnew file mode 100644\nindex 0000000..1111111\nGIT binary patch\nliteral 4\nNOTBASE85\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			exit, stderr := gitApplyProbe(t, repo, body, "--cached")
			if exit != 128 {
				t.Skipf("this git exits %d for %s (%q); the grammar covers 128 only", exit, name, stderr)
			}
			if !ApplyProbeAnswered(exit, false, stderr) {
				t.Fatalf("a real malformed-patch diagnostic was rejected:\nexit=%d stderr=%q", exit, stderr)
			}
			if !IsMalformedPatchDiagnostic(stderr) {
				t.Fatalf("grammar did not match the measured diagnostic: %q", stderr)
			}
		})
	}
}

// TestRealGit_OrdinaryConflictsExitOne asserts the conflict forms rev-2
// wrongly needed a stderr grammar for are decided by the EXIT CODE, and
// that they are NOT admitted by the malformed-patch grammar.
func TestRealGit_OrdinaryConflictsExitOne(t *testing.T) {
	repo := newApplyFixtureRepo(t)
	cases := map[string]string{
		"new-file-already-exists": "diff --git a/f.txt b/f.txt\nnew file mode 100644\n--- /dev/null\n+++ b/f.txt\n@@ -0,0 +1 @@\n+x\n",
		"delete-missing-file":     "diff --git a/gone.txt b/gone.txt\ndeleted file mode 100644\n--- a/gone.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n",
		"context-mismatch":        "diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,3 +1,3 @@\n-ZZZ\n+A\n b\n c\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			exit, stderr := gitApplyProbe(t, repo, body, "--cached")
			if exit != 1 {
				t.Fatalf("%s exited %d, want 1 (stderr=%q)", name, exit, stderr)
			}
			if !ApplyProbeAnswered(exit, false, stderr) {
				t.Fatalf("an exit-1 conflict must be an answer regardless of stderr")
			}
			if IsMalformedPatchDiagnostic(stderr) {
				t.Fatalf("an ordinary conflict must NOT match the malformed-patch grammar: %q", stderr)
			}
		})
	}
}

// TestRealGit_MissingObjectIsNeverAnAnswer measures the missing-object
// form and asserts it is rejected at every exit code.
func TestRealGit_MissingObjectIsNeverAnAnswer(t *testing.T) {
	const missing = "error: failed to read src/app.txt"
	if !IsMissingObjectError(missing) {
		t.Fatalf("the measured missing-object form is not recognised")
	}
	for _, exit := range []int{128, 1, -1, 2} {
		if exit == 1 {
			// Exit 1 is an answer by contract; the classifier is not
			// consulted. Documented, not asserted as a rejection.
			continue
		}
		if ApplyProbeAnswered(exit, false, missing) {
			t.Errorf("a missing-object diagnostic was admitted at exit %d", exit)
		}
	}
}

// ── Locale determinism ───────────────────────────────────────────────────

// TestEvidenceCommandsForceCLocale starts from a NON-C ambient locale and
// asserts every evidence command sees `LC_ALL=C` (and keeps
// `GIT_NO_LAZY_FETCH=1`), through a PATH wrapper that records each
// invocation's environment.
func TestEvidenceCommandsForceCLocale(t *testing.T) {
	repo := newApplyFixtureRepo(t)

	// A foreign ambient locale that must be overridden.
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	t.Setenv("LANG", "fr_FR.UTF-8")
	t.Setenv("LC_MESSAGES", "fr_FR.UTF-8")

	logPath := installEnvRecordingGit(t)

	// One of every classified command family.
	if _, err := ReadRepoFacts(repo); err != nil {
		t.Fatalf("ReadRepoFacts: %v", err)
	}
	if _, err := EnumerateCommitTrailers(repo); err != nil {
		t.Fatalf("EnumerateCommitTrailers: %v", err)
	}
	if _, err := HeadCommitOffline(repo); err != nil {
		t.Fatalf("HeadCommitOffline: %v", err)
	}
	idx, err := NewTempIndex(repo, filepath.Join(repo, ".git", "tpatch-verify"))
	if err != nil {
		t.Fatalf("NewTempIndex: %v", err)
	}
	if err := idx.ReadTree("HEAD"); err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	patch := filepath.Join(t.TempDir(), "p.patch")
	if err := os.WriteFile(patch, []byte("this is not a diff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx.ApplyCheck(ApplyCheckOptions{PatchPath: patch, Context: IntPtr(0), Verbose: true})
	_ = idx.Close()
	head, _ := HeadCommitOffline(repo)
	if _, _, _, err := BlobAtTree(repo, head, "f.txt"); err != nil {
		t.Fatalf("BlobAtTree: %v", err)
	}
	if _, err := IsAncestorOffline(repo, head, "HEAD"); err != nil {
		t.Fatalf("IsAncestorOffline: %v", err)
	}
	_ = RunOfflineGitInResult(repo, "status", "--porcelain")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read the wrapper log: %v", err)
	}
	subcommands := map[string]bool{}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("no git invocations recorded")
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\x1f")
		var argv []string
		env := map[string]string{}
		inEnv := false
		for _, f := range fields {
			if f == "ENV" {
				inEnv = true
				continue
			}
			if inEnv {
				if kv := strings.SplitN(f, "=", 2); len(kv) == 2 {
					env[kv[0]] = kv[1]
				}
				continue
			}
			argv = append(argv, f)
		}
		for _, a := range argv {
			if !strings.HasPrefix(a, "-") {
				subcommands[a] = true
				break
			}
		}
		if env["LC_ALL"] != "C" {
			t.Errorf("git %s ran with LC_ALL=%q, want C", strings.Join(argv, " "), env["LC_ALL"])
		}
		if env["GIT_NO_LAZY_FETCH"] != "1" {
			t.Errorf("git %s lost GIT_NO_LAZY_FETCH=1", strings.Join(argv, " "))
		}
	}
	for _, want := range []string{"rev-parse", "log", "read-tree", "apply", "cat-file", "merge-base"} {
		if !subcommands[want] {
			t.Errorf("no %s invocation was exercised; the locale assertion would be vacuous for it", want)
		}
	}
}

// installEnvRecordingGit records argv plus the classified environment of
// every git invocation, then forwards to the real git.
func installEnvRecordingGit(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	// POSIX printf uses octal escapes; Ubuntu's /bin/sh (dash) does not
	// interpret \xHH and would log the four literal bytes "\x1f".
	script := "#!/bin/sh\n{\n" +
		"  for a in \"$@\"; do printf '%s\\037' \"$a\"; done\n" +
		"  printf 'ENV\\037LC_ALL=%s\\037GIT_NO_LAZY_FETCH=%s\\n' \"${LC_ALL-}\" \"${GIT_NO_LAZY_FETCH-}\"\n" +
		"} >> " + logPath + "\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// TestEvidenceEnvOrdering asserts the mandatory entries are appended
// LAST, so they win over both the inherited environment and any
// caller-supplied extra.
func TestEvidenceEnvOrdering(t *testing.T) {
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	env := evidenceEnv("LC_ALL=de_DE.UTF-8", "GIT_NO_LAZY_FETCH=0")
	last := map[string]string{}
	for _, kv := range env {
		if parts := strings.SplitN(kv, "=", 2); len(parts) == 2 {
			last[parts[0]] = parts[1]
		}
	}
	if last["LC_ALL"] != "C" {
		t.Errorf("LC_ALL resolved to %q, want C", last["LC_ALL"])
	}
	if last["GIT_NO_LAZY_FETCH"] != "1" {
		t.Errorf("GIT_NO_LAZY_FETCH resolved to %q, want 1", last["GIT_NO_LAZY_FETCH"])
	}
}

// TestUnrelatedCallersKeepTheirEnvironment asserts the locale entry does
// not leak into helpers that deliberately inherit the ambient
// environment (a nil extra means "unchanged").
func TestUnrelatedCallersKeepTheirEnvironment(t *testing.T) {
	if env := shadowEnv(nil); env != nil {
		t.Fatalf("a nil extra must leave the environment untouched, got %d entries", len(env))
	}
	env := shadowEnv([]string{NoLazyFetchEnv, CLocaleEnv})
	if len(env) == 0 {
		t.Fatalf("an explicit extra must produce a concrete environment")
	}
	found := 0
	for _, kv := range env {
		if kv == NoLazyFetchEnv || kv == CLocaleEnv {
			found++
		}
	}
	if found != 2 {
		t.Errorf("the explicit entries were not appended: %d/2", found)
	}
}

// ── Regression proof against the rev-2 predicate ─────────────────────────

// rev2Patterns and rev2ApplyProbeAnswered are the SHIPPED rev-2 logic,
// reproduced verbatim so the improvement is demonstrable in-tree rather
// than only by checking out an older commit.
var rev2Patterns = []string{
	"no valid patches in input", "unrecognized input", "corrupt patch at line",
	"patch fragment without header", "patch with only garbage", "patch does not apply",
	"does not exist in index", "already exists", "cannot apply binary patch",
	"new file", "deleted file",
}

func rev2IsPatchInputError(stderr string) bool {
	low := strings.ToLower(stderr)
	for _, p := range rev2Patterns {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

func rev2ApplyProbeAnswered(exitCode int, ok bool, stderr string) bool {
	if ok || exitCode == 1 {
		return true
	}
	if IsMissingObjectError(stderr) || IsNetworkFetchError(stderr) {
		return false
	}
	return rev2IsPatchInputError(stderr)
}

// TestRev3ClassifierFixesRev2Misclassifications enumerates the outcomes
// the rev-2 predicate promoted to a patch ANSWER and the rev-3 predicate
// correctly rejects. Each row is a way a wrapper, a signalled process or
// a foreign locale could have been reported as R5/R11 patch drift.
func TestRev3ClassifierFixesRev2Misclassifications(t *testing.T) {
	cases := []struct {
		name   string
		exit   int
		stderr string
	}{
		{"wrapper-echoing-new-file", 128, "wrapper: new file mode 100644"},
		{"wrapper-echoing-deleted-file", 128, "deleted file mode 100644"},
		{"wrapper-echoing-already-exists", 128, "shim: already exists"},
		{"fatal-spoof-of-no-valid-patches", 128, `fatal: No valid patches in input (allow with "--allow-empty")`},
		{"signalled-with-recognised-phrase", -1, `error: No valid patches in input (allow with "--allow-empty")`},
		{"sigkill-137-with-recognised-phrase", 137, "error: corrupt patch at line 5"},
		{"exit-127-not-found-with-phrase", 127, "error: patch with only garbage at line 3"},
		{"exit-2-with-phrase", 2, "error: patch fragment without header at line 1: @@"},
		{"mixed-recognised-plus-wrapper-noise", 128, "error: patch with only garbage at line 3\nwrapper: intercepted"},
		{"conflict-phrase-at-128", 128, "error: f.txt: patch does not apply"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !rev2ApplyProbeAnswered(tc.exit, false, tc.stderr) {
				t.Fatalf("precondition: rev-2 must have admitted this outcome")
			}
			if ApplyProbeAnswered(tc.exit, false, tc.stderr) {
				t.Fatalf("rev-3 still admits a non-answer as a patch verdict")
			}
		})
	}
}

// TestRev3ClassifierKeepsEveryRealAnswer is the other direction: every
// outcome rev-2 correctly admitted for a REAL git run is still admitted.
func TestRev3ClassifierKeepsEveryRealAnswer(t *testing.T) {
	repo := newApplyFixtureRepo(t)
	bodies := []string{
		"",
		"this is not a diff at all\n",
		"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,3 +1,3 @@\n",
		"diff --git a/f.txt b/f.txt\nnew file mode 100644\n--- /dev/null\n+++ b/f.txt\n@@ -0,0 +1 @@\n+x\n",
		"diff --git a/gone.txt b/gone.txt\ndeleted file mode 100644\n--- a/gone.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n",
		"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,3 +1,3 @@\n-a\n+A\n b\n c\n",
	}
	for i, body := range bodies {
		exit, stderr := gitApplyProbe(t, repo, body, "--cached")
		if !rev2ApplyProbeAnswered(exit, exit == 0, stderr) {
			continue // rev-2 rejected it too; nothing to preserve
		}
		if !ApplyProbeAnswered(exit, exit == 0, stderr) {
			t.Errorf("case %d: rev-3 rejected a real git answer (exit=%d stderr=%q)", i, exit, stderr)
		}
	}
}

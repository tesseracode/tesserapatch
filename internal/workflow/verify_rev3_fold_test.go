package workflow

// Rev-3 fold regressions — v0.15.1 Wave C, adjudication P1 (broad
// locale-dependent apply classifier).
//
// The classifier itself is unit-tested in internal/gitutil; these tests
// prove the BEHAVIOUR change end-to-end: a probe that fails with a
// broad English fragment is no longer promoted to a patch verdict, a
// missing-object diagnostic still reaches R22, and a genuinely
// malformed patch still gets its patch-level answer.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestRev3_HistoricalV8BroadPhraseFailureIsUnavailable injects a shadow
// `git apply` failure whose stderr carries the broad `new file` fragment
// rev-2 accepted. It must classify as `unavailable` + R10, never as an
// R5 patch/attestation disagreement.
func TestRev3_HistoricalV8BroadPhraseFailureIsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		exit   string
		stderr string
	}{
		{"exit-128-new-file", "128", "wrapper: new file mode 100644"},
		{"exit-128-already-exists", "128", "shim: already exists"},
		{"exit-137-signalled", "137", `error: No valid patches in input (allow with "--allow-empty")`},
		{"exit-127-not-found", "127", "error: corrupt patch at line 5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLadderFixture(t)
			installShadowApplyFailureGitExit(t, tc.stderr, tc.exit)
			r := f.Verify()
			if r.LandingEvidence.State != EvidenceUnavailable {
				t.Fatalf("evidence=%q want unavailable (reason=%q)", r.LandingEvidence.State, r.LandingEvidence.Reason)
			}
			v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
			if strings.Contains(v8.Remediation, "the patch and the landing attestation disagree") {
				t.Fatalf("a failed probe was promoted to an R5 patch verdict: %q", v8.Remediation)
			}
			v7 := checkByID(t, r, CheckRecipeReplayClean)
			if !strings.Contains(v7.Remediation, "verify requires git >= 2.36") {
				t.Errorf("expected R10; got %q", v7.Remediation)
			}
			assertNoShadowLeak(t, f.Root, f.Slug)
			assertNoTempIndexLeak(t, f.Root)
		})
	}
}

// TestRev3_HistoricalV8MissingObjectStillReachesR22 pins that the
// narrowed grammar did not cost the missing-object classification.
func TestRev3_HistoricalV8MissingObjectStillReachesR22(t *testing.T) {
	f := newLadderFixture(t)
	installShadowApplyFailureGitExit(t, "error: failed to read src/app.txt", "128")
	r := f.Verify()
	if r.LandingEvidence.State != EvidenceHistoryIncomplete {
		t.Fatalf("evidence=%q want history-incomplete", r.LandingEvidence.State)
	}
	if got, want := checkByID(t, r, CheckRecipeReplayClean).Remediation, remediationR22(f.Slug); got != want {
		t.Errorf("R22 not verbatim:\n got %q\nwant %q", got, want)
	}
	assertNoShadowLeak(t, f.Root, f.Slug)
	assertNoTempIndexLeak(t, f.Root)
}

// TestRev3_MalformedPatchStillGetsAPatchAnswer runs the REAL git against
// a genuinely malformed canonical patch (no shim) and asserts the run
// still produces a patch-level answer rather than a reader failure.
func TestRev3_MalformedPatchStillGetsAPatchAnswer(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("target", store.StateApplied)
	c.Artifacts("target", "this is not a diff at all\n", ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
		{Type: "append-file", Path: "src/parent.txt", Content: "T\n"},
	}})
	r := c.Verify("target")
	if r.LandingEvidence.State == EvidenceUnavailable || r.LandingEvidence.State == EvidenceHistoryIncomplete {
		t.Fatalf("a malformed patch must stay a patch-level answer; evidence=%q", r.LandingEvidence.State)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if v8.Passed {
		t.Fatalf("a malformed patch must fail V8")
	}
	if !strings.Contains(v8.Remediation, "no longer applies to closure-replayed baseline") {
		t.Errorf("expected the shipped V8 remediation; got %q", v8.Remediation)
	}
}

// TestRev3_LadderBroadPhraseFailureIsNotAnAnswer drives the isolated
// index ladder (`--cached`) rather than the shadow, so the anchor-C and
// qualification probes are the ones failing.
func TestRev3_LadderBroadPhraseFailureIsNotAnAnswer(t *testing.T) {
	f := newLadderFixture(t)
	installFailingCachedApplyGitExit(t, "wrapper: deleted file mode 100644", "128")
	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("verdict=%s want failed", r.Verdict)
	}
	if r.FailedAt == FailedAtLandedContentAbsent {
		t.Fatalf("a failed ladder probe was reported as absent content")
	}
	if r.LandingEvidence.State != EvidenceUnavailable {
		t.Fatalf("evidence=%q want unavailable (reason=%q)", r.LandingEvidence.State, r.LandingEvidence.Reason)
	}
	assertNoTempIndexLeak(t, f.Root)
}

// TestRev3_LadderMissingObjectStillReachesR22 is the ladder's
// missing-object half.
func TestRev3_LadderMissingObjectStillReachesR22(t *testing.T) {
	f := newLadderFixture(t)
	installFailingCachedApplyGitExit(t, "error: failed to read src/app.txt", "128")
	r := f.Verify()
	if r.LandingEvidence.State != EvidenceHistoryIncomplete {
		t.Fatalf("evidence=%q want history-incomplete", r.LandingEvidence.State)
	}
	if got, want := checkByID(t, r, CheckRecipeReplayClean).Remediation, remediationR22(f.Slug); got != want {
		t.Errorf("R22 not verbatim:\n got %q\nwant %q", got, want)
	}
	assertNoTempIndexLeak(t, f.Root)
}

// TestRev3_VerifyRunsUnderCLocale starts from a foreign ambient locale
// and asserts every classified verify command still sees LC_ALL=C, while
// a below-floor run still issues only `--version`.
func TestRev3_VerifyRunsUnderCLocale(t *testing.T) {
	t.Run("every-command-is-c-locale", func(t *testing.T) {
		f := newLadderFixture(t)
		t.Setenv("LC_ALL", "fr_FR.UTF-8")
		t.Setenv("LANG", "fr_FR.UTF-8")
		w := installGitWrapper(t)
		w.Reset()
		f.Verify()
		calls := w.Calls()
		if len(calls) == 0 {
			t.Fatalf("no git calls recorded")
		}
		checked := 0
		for _, c := range calls {
			if !gitSubcommandsUnderTest[c.Subcommand()] {
				continue
			}
			checked++
			if c.Env["LC_ALL"] != "C" {
				t.Errorf("git %s ran with LC_ALL=%q, want C", c.Joined(), c.Env["LC_ALL"])
			}
			if c.Env["GIT_NO_LAZY_FETCH"] != "1" {
				t.Errorf("git %s lost GIT_NO_LAZY_FETCH=1", c.Joined())
			}
		}
		if checked == 0 {
			t.Fatalf("no classified command was exercised")
		}
	})
	t.Run("below-floor-still-only-version", func(t *testing.T) {
		f := newLadderFixture(t)
		t.Setenv("LC_ALL", "fr_FR.UTF-8")
		w := installFakeVersionGit(t, "git version 2.30.2")
		w.Reset()
		r := f.Verify()
		for _, c := range w.Calls() {
			if c.Subcommand() != "--version" && !c.Has("--version") {
				t.Errorf("below-floor run issued git %s", c.Joined())
			}
		}
		if r.LandingEvidence.State != EvidenceUnavailable {
			t.Errorf("evidence=%q want unavailable", r.LandingEvidence.State)
		}
	})
}

// ── helpers ──────────────────────────────────────────────────────────────

// installShadowApplyFailureGitExit fails the shadow-side
// `git apply --check` (no `--cached`) with a chosen exit code.
func installShadowApplyFailureGitExit(t *testing.T, msg, exit string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nisapply=0\ncached=0\nfor a in \"$@\"; do\n" +
		"  [ \"$a\" = \"apply\" ] && isapply=1\n" +
		"  [ \"$a\" = \"--cached\" ] && cached=1\ndone\n" +
		"if [ \"$isapply\" = \"1\" ] && [ \"$cached\" = \"0\" ]; then\n" +
		"  echo " + rev1ShellQuote(msg) + " >&2\n  exit " + exit + "\nfi\n" +
		"exec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// installFailingCachedApplyGitExit fails the isolated-index probes with a
// chosen exit code.
func installFailingCachedApplyGitExit(t *testing.T, msg, exit string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nisapply=0\ncached=0\nfor a in \"$@\"; do\n" +
		"  [ \"$a\" = \"apply\" ] && isapply=1\n" +
		"  [ \"$a\" = \"--cached\" ] && cached=1\ndone\n" +
		"if [ \"$isapply\" = \"1\" ] && [ \"$cached\" = \"1\" ]; then\n" +
		"  echo " + rev1ShellQuote(msg) + " >&2\n  exit " + exit + "\nfi\n" +
		"exec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertNoTempIndexLeak(t *testing.T, root string) {
	t.Helper()
	if leaks := listTempIndexes(t, root); len(leaks) != 0 {
		t.Errorf("temp index leaked: %v", leaks)
	}
	dir := filepath.Join(root, ".git", "tpatch-verify")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "verify-index-") {
			t.Errorf("temp index leaked: %s", e.Name())
		}
	}
}
